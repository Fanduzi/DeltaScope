// Package audit orchestrates audit use cases at the application layer.
// input: one fully parsed CHANGE COLUMN or RENAME COLUMN statement, the request-local pre-state, and the already-resolved version identity
// output: one publication that either migrates the source column in place or tombstones the affected tables and loaded dependents; a loaded empty member collection stays empty, and a CHANGE declaration outside the ordinary template tombstones before a withheld column set is kept
// pos: T05-A5 column-identity transition beside the ordinary MODIFY replacement; it reuses A4 definition helpers and does not rewrite foreign keys
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// applyColumnIdentity publishes the frozen single-column CHANGE or RENAME
// COLUMN post-state. The write set is fixed before replacement. Cancellation
// before publication leaves every entry unchanged.
func (s *batchState) applyColumnIdentity(ctx context.Context, statement spec.Statement, alter *spec.Alter, targets []spec.Table) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	publication := s.prepareColumnIdentity(statement, alter, targets)
	if err := ctx.Err(); err != nil {
		return err
	}
	publication.apply(s)
	return nil
}

func (s *batchState) prepareColumnIdentity(statement spec.Statement, alter *spec.Alter, targets []spec.Table) modifyPublication {
	targetKeys := s.modifyTargetKeys(targets)
	var primary batchTableKey
	if len(targetKeys) > 0 {
		primary = targetKeys[0]
	}
	oldName, newName, namesOK := columnIdentityNames(alter)
	precise := namesOK && columnIdentityTemplate(s, statement, alter, targets)
	var dependents []batchTableKey
	if precise {
		dependents = s.modifyDependentKeys(primary, []string{oldName, newName})
	} else {
		dependents = s.modifyIdentityDependentKeys(targetKeys)
	}
	invalidate := func(extra ...batchTableKey) modifyPublication {
		keys := append([]batchTableKey{}, dependents...)
		keys = append(keys, extra...)
		return modifyPublication{invalidate: keys}
	}
	if !precise {
		return invalidate(targetKeys...)
	}
	// Version applicability is a state gate. Disabling the version rule must
	// not publish a renamed column on a missing or incompatible version.
	if alter.Action == "rename_column" && !spec.RenameColumnVersionSupportFor(s.resolvedVersion).Supported {
		return invalidate(primary)
	}
	entry := s.entries[primary]
	if entry == nil || entry.state == tableUnknown || entry.state == tableAbsent {
		return invalidate(primary)
	}
	if entry.shape == nil {
		return invalidate()
	}
	if entry.shape.Columns == nil {
		// A declaration can be outside the ordinary template without the old
		// column definition. That check has to win before this branch keeps a
		// known-empty primary key or index set.
		if alter.Action == "change_column" && alter.Column != nil && alter.Column.Definition != nil && modifyDeclaresExtraMember(alter, *alter.Column.Definition) {
			return invalidate(primary)
		}
		shape := cloneTableSnapshot(entry.shape)
		scrubColumnIdentityMembers(shape, oldName, newName)
		return modifyPublication{
			invalidate: dependents,
			replaceKey: primary,
			replace: &batchTableEntry{
				state:         tablePresent,
				shape:         shape,
				displaySchema: entry.displaySchema,
				displayTable:  entry.displayTable,
			},
		}
	}
	index := modifyColumnIndex(entry.shape.Columns, oldName)
	if index < 0 || columnIdentityConflict(entry.shape.Columns, index, oldName, newName) {
		return invalidate(primary)
	}
	if columnIdentityLocalUnsafe(entry.shape, oldName, newName) || len(dependents) > 0 {
		return invalidate(primary)
	}
	source := entry.shape.Columns[index]
	shape := cloneTableSnapshot(entry.shape)
	if alter.Action == "change_column" {
		definition := alter.Column.Definition
		inPK, pkKnown := primaryKeyMembership(entry.shape, source.Name)
		if !modifyTypeTemplate(source, *definition) ||
			modifyDeclaresExtraMember(alter, *definition) ||
			modifyTiDBPrimaryKeySignedness(s.dialect, inPK, source, *definition) ||
			(!pkKnown && !modifyExplicitNotNull(alter, *definition)) ||
			(inPK && modifyExplicitNull(alter, *definition)) {
			return invalidate(primary)
		}
		next := cloneColumn(*definition)
		if inPK {
			next.NotNull = true
		}
		applyModifyColumnDefaults(&next, shape)
		shape.Columns[index] = next
		rewriteIdentityColumnRefs(shape, oldName, newName)
		clearModifyAffectedStats(shape)
	} else {
		next := cloneColumn(source)
		next.Name = newName
		shape.Columns[index] = next
		rewriteIdentityColumnRefs(shape, oldName, newName)
	}
	return modifyPublication{
		replaceKey: primary,
		replace: &batchTableEntry{
			state:         tablePresent,
			shape:         shape,
			displaySchema: entry.displaySchema,
			displayTable:  entry.displayTable,
		},
	}
}

// firstColumnIdentityAlter returns the first CHANGE or RENAME COLUMN action.
// A statement with several actions is still not a success template; the
// caller uses it only to reach the shared affected-set publication.
func firstColumnIdentityAlter(ddl *spec.DDL) *spec.Alter {
	if ddl == nil {
		return nil
	}
	for i := range ddl.Alter {
		switch ddl.Alter[i].Action {
		case "change_column", "rename_column":
			return &ddl.Alter[i]
		}
	}
	return nil
}

// columnIdentityTemplate reports whether this single action is the frozen
// identity migration: one bounded target, matching old-name identities, no
// conditional or positional modifier. CHANGE also needs a non-empty new type.
// RENAME COLUMN does not.
func columnIdentityTemplate(s *batchState, statement spec.Statement, alter *spec.Alter, targets []spec.Table) bool {
	ddl := statement.DDL
	if ddl == nil || ddl.Table == nil || len(ddl.Alter) != 1 || len(targets) != 1 || alter == nil {
		return false
	}
	if !fullyAuditedStatement(s.dialect, statement) {
		return false
	}
	if s.keyFor(s.schema, targets[0]) != s.keyFor(s.schema, *ddl.Table) {
		return false
	}
	if alter.Action != "change_column" && alter.Action != "rename_column" {
		return false
	}
	if alter.HasColumnPosition {
		return false
	}
	if alter.Options["if_exists"] == "true" || alter.Options["if_not_exists"] == "true" {
		return false
	}
	if len(alter.UnextractedOptions) > 0 {
		return false
	}
	if _, _, ok := columnIdentityNames(alter); !ok {
		return false
	}
	if alter.Action == "change_column" && strings.TrimSpace(alter.Column.Definition.Type) == "" {
		return false
	}
	return true
}

// columnIdentityNames returns the source name and the destination name when
// Alter.Name and Column.OldName are the same identity and Definition.Name is set.
func columnIdentityNames(alter *spec.Alter) (string, string, bool) {
	if alter == nil || alter.Column == nil || alter.Column.Definition == nil {
		return "", "", false
	}
	oldName := strings.TrimSpace(alter.Name)
	columnOld := strings.TrimSpace(alter.Column.OldName)
	newName := strings.TrimSpace(alter.Column.Definition.Name)
	if oldName == "" || columnOld == "" || newName == "" || !strings.EqualFold(oldName, columnOld) {
		return "", "", false
	}
	return oldName, newName, true
}

// columnIdentityConflict reports a destination that is a different existing
// column. The source column itself is not a conflict, including old==new.
func columnIdentityConflict(columns []spec.Column, self int, oldName, newName string) bool {
	if strings.EqualFold(oldName, newName) {
		return false
	}
	for i, column := range columns {
		if i == self {
			continue
		}
		if strings.EqualFold(column.Name, newName) {
			return true
		}
	}
	return false
}

func columnIdentityLocalUnsafe(shape *spec.TableSnapshot, oldName, newName string) bool {
	if modifyLocalUnsafe(shape, oldName) {
		return true
	}
	return !strings.EqualFold(oldName, newName) && modifyLocalUnsafe(shape, newName)
}

// rewriteIdentityColumnRefs renames column references on the ordinary primary
// key, primary-key constraints, and ordinary secondary or unique indexes.
// Index names and key order stay. Foreign keys and CHECK constraints are left
// untouched; callers withhold publication when those cannot be recomputed.
func rewriteIdentityColumnRefs(shape *spec.TableSnapshot, oldName, newName string) {
	if shape == nil || strings.EqualFold(oldName, newName) {
		return
	}
	if shape.PrimaryKey != nil {
		shape.PrimaryKey.Columns = rewriteColumnRefs(shape.PrimaryKey.Columns, oldName, newName)
	}
	for i := range shape.Constraints {
		if primaryKeyConstraint(shape.Constraints[i].Type) {
			shape.Constraints[i].Columns = rewriteColumnRefs(shape.Constraints[i].Columns, oldName, newName)
		}
	}
	for i := range shape.Indexes {
		index := shape.Indexes[i]
		if index.HasExpressionKeys || index.ExpressionCount > 0 || modifyIndexSpecial(index) {
			continue
		}
		shape.Indexes[i].Columns = rewriteColumnRefs(index.Columns, oldName, newName)
	}
}

func rewriteColumnRefs(columns []string, oldName, newName string) []string {
	out := append([]string(nil), columns...)
	for i, column := range out {
		if strings.EqualFold(strings.TrimSpace(column), oldName) {
			out[i] = newName
		}
	}
	return out
}

// scrubColumnIdentityMembers drops member payloads that still name the old or
// new column, or that cannot prove they ignore both. Proven-unrelated members
// stay. A nil collection is left unchanged: Unknown=false is a loaded empty
// set, and Unknown=true stays unknown. It is not replaced with a known empty
// list, and a known absence is not marked unknown.
func scrubColumnIdentityMembers(shape *spec.TableSnapshot, oldName, newName string) {
	if shape.PrimaryKey != nil && !indexProvesUnrelated(*shape.PrimaryKey, oldName, newName) {
		shape.PrimaryKey = nil
		shape.PrimaryKeyUnknown = true
	}
	scrubIndexList(&shape.Indexes, &shape.IndexesUnknown, oldName, newName)
	scrubConstraintList(shape, oldName, newName)
}

func indexProvesUnrelated(index spec.Index, oldName, newName string) bool {
	if index.HasExpressionKeys || index.ExpressionCount > 0 {
		return false
	}
	if !columnListProvesUnrelated(index.Columns, oldName, 0) || !columnListProvesUnrelated(index.Columns, newName, 0) {
		return false
	}
	if len(index.IncludedColumns) > 0 &&
		(!columnListProvesUnrelated(index.IncludedColumns, oldName, 0) || !columnListProvesUnrelated(index.IncludedColumns, newName, 0)) {
		return false
	}
	return true
}

func scrubIndexList(indexes *[]spec.Index, unknown *bool, oldName, newName string) {
	if indexes == nil || *indexes == nil {
		return
	}
	kept := make([]spec.Index, 0, len(*indexes))
	dropped := false
	for _, index := range *indexes {
		if indexProvesUnrelated(index, oldName, newName) {
			kept = append(kept, index)
			continue
		}
		dropped = true
	}
	if unknown != nil && (*unknown || dropped) {
		*unknown = true
		if len(kept) == 0 {
			*indexes = nil
			return
		}
	}
	*indexes = kept
}

func scrubConstraintList(shape *spec.TableSnapshot, oldName, newName string) {
	if shape.Constraints == nil {
		return
	}
	kept := make([]spec.Constraint, 0, len(shape.Constraints))
	dropped := false
	for _, constraint := range shape.Constraints {
		if constraintProvesUnrelated(constraint, oldName, newName) {
			kept = append(kept, constraint)
			continue
		}
		dropped = true
	}
	if shape.ConstraintsUnknown || dropped {
		shape.ConstraintsUnknown = true
		if len(kept) == 0 {
			shape.Constraints = nil
			return
		}
	}
	shape.Constraints = kept
}

func constraintProvesUnrelated(constraint spec.Constraint, oldName, newName string) bool {
	if !columnListProvesUnrelated(constraint.Columns, oldName, constraint.UnmodeledParts) ||
		!columnListProvesUnrelated(constraint.Columns, newName, constraint.UnmodeledParts) {
		return false
	}
	if strings.TrimSpace(constraint.ReferencedTable) != "" || len(constraint.ReferencedColumns) > 0 || constraint.UnmodeledReferencedParts > 0 {
		if !columnListProvesUnrelated(constraint.ReferencedColumns, oldName, constraint.UnmodeledReferencedParts) ||
			!columnListProvesUnrelated(constraint.ReferencedColumns, newName, constraint.UnmodeledReferencedParts) {
			return false
		}
	}
	return true
}
