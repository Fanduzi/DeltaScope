// Package audit publishes the T05-A6 dependency-free DROP COLUMN post-state.
// input: one ALTER statement, its drop_column action, and the table identities it names
// output: either a precise column removal with unrelated members kept and the accepted statistic set cleared, or one tombstone publication of every named identity and its loaded dependents
// pos: ordered-state successor for a single ordinary DROP COLUMN; multi-action and dependency-bearing forms stay conservative
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// applyDropColumn publishes the frozen ordinary DROP COLUMN post-state.
// The affected set is fixed before any write. Cancellation before publication
// leaves every entry unchanged. A refused template tombstones every named
// table identity, including one this request has not loaded, together with
// already-loaded dependents of those identities.
func (s *batchState) applyDropColumn(ctx context.Context, statement spec.Statement, alter *spec.Alter, targets []spec.Table) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	publication, unbound := s.prepareDropColumn(statement, alter, targets)
	if err := ctx.Err(); err != nil {
		return err
	}
	if unbound {
		s.contaminated = true
		return nil
	}
	publication.apply(s)
	return nil
}

func (s *batchState) prepareDropColumn(statement spec.Statement, alter *spec.Alter, targets []spec.Table) (modifyPublication, bool) {
	targetKeys := s.modifyTargetKeys(targets)
	if len(targetKeys) == 0 {
		return modifyPublication{}, true
	}
	primary := targetKeys[0]
	precise := dropColumnTemplate(s, statement, alter, targets)
	var dependents []batchTableKey
	if precise {
		dependents = s.modifyDependentKeys(primary, []string{alter.Name})
	} else {
		dependents = s.modifyIdentityDependentKeys(targetKeys)
	}
	invalidate := func(extra ...batchTableKey) modifyPublication {
		keys := append([]batchTableKey{}, dependents...)
		keys = append(keys, extra...)
		return modifyPublication{invalidate: keys}
	}
	if !precise {
		return invalidate(targetKeys...), false
	}
	entry := s.entries[primary]
	if entry == nil || entry.state == tableUnknown || entry.state == tableAbsent {
		return invalidate(primary), false
	}
	// A withheld column set is not a one-column table. DROP does not keep a
	// partial snapshot: the whole target becomes unknown.
	if entry.shape == nil || entry.shape.Columns == nil {
		return invalidate(primary), false
	}
	index, count := dropColumnMatch(entry.shape.Columns, alter.Name)
	if count != 1 || len(entry.shape.Columns) < 2 {
		return invalidate(primary), false
	}
	source := entry.shape.Columns[index]
	if dropTargetColumnSpecial(source) || dropColumnLocalUnsafe(entry.shape, source.Name) || len(dependents) > 0 {
		return invalidate(primary), false
	}
	shape := cloneTableSnapshot(entry.shape)
	remaining := make([]spec.Column, 0, len(shape.Columns)-1)
	for i, column := range shape.Columns {
		if i == index {
			continue
		}
		remaining = append(remaining, column)
	}
	shape.Columns = remaining
	clearModifyAffectedStats(shape)
	return modifyPublication{
		replaceKey: primary,
		replace: &batchTableEntry{
			state:         tablePresent,
			shape:         shape,
			displaySchema: entry.displaySchema,
			displayTable:  entry.displayTable,
		},
	}, false
}

// firstDropColumnAlter returns the first DROP COLUMN action. A statement with
// several actions is still not a success template; the caller uses it only to
// reach the shared affected-set publication.
func firstDropColumnAlter(ddl *spec.DDL) *spec.Alter {
	if ddl == nil {
		return nil
	}
	for i := range ddl.Alter {
		if ddl.Alter[i].Action == "drop_column" {
			return &ddl.Alter[i]
		}
	}
	return nil
}

func dropColumnStatement(statement spec.Statement) bool {
	return statement.DDL != nil && statement.DDL.Operation == spec.DDLOperationAlterTable && firstDropColumnAlter(statement.DDL) != nil
}

// dropColumnTemplate reports whether this single DROP COLUMN is the frozen
// ordinary removal: one bounded target, matching identities, and no
// conditional or positional modifier. The column definition is not a new column.
func dropColumnTemplate(s *batchState, statement spec.Statement, alter *spec.Alter, targets []spec.Table) bool {
	ddl := statement.DDL
	if !orderedStateDialect(s.dialect) || ddl == nil || ddl.Table == nil || len(ddl.Alter) != 1 || len(targets) != 1 || alter == nil {
		return false
	}
	if !fullyAuditedStatement(s.dialect, statement) {
		return false
	}
	if s.keyFor(s.schema, targets[0]) != s.keyFor(s.schema, *ddl.Table) {
		return false
	}
	if alter.Action != "drop_column" || alter.HasColumnPosition {
		return false
	}
	if alter.Options["if_exists"] == "true" || alter.Options["if_not_exists"] == "true" {
		return false
	}
	if len(alter.UnextractedOptions) > 0 || alter.Column == nil {
		return false
	}
	name := strings.TrimSpace(alter.Name)
	oldName := strings.TrimSpace(alter.Column.OldName)
	return name != "" && oldName != "" && strings.EqualFold(name, oldName)
}

func dropColumnMatch(columns []spec.Column, name string) (int, int) {
	index := -1
	count := 0
	for i := range columns {
		if strings.EqualFold(columns[i].Name, name) {
			count++
			if index < 0 {
				index = i
			}
		}
	}
	return index, count
}

func dropTargetColumnSpecial(column spec.Column) bool {
	if column.AutoIncrement || column.AutoRandom || column.IsIdentity || strings.TrimSpace(column.GeneratedWhen) != "" {
		return true
	}
	return len(column.UnextractedOptions) > 0
}

// dropColumnLocalUnsafe reports loaded facts that do not prove the column can
// disappear without rewriting a member. Ordinary indexes are not safe merely
// because a type change could keep them: any reference, or any list that is
// not a complete set of unrelated names, refuses the precise template.
func dropColumnLocalUnsafe(shape *spec.TableSnapshot, columnName string) bool {
	if shape == nil || shape.PrimaryKeyUnknown || shape.IndexesUnknown || shape.ConstraintsUnknown {
		return true
	}
	if !dropPrimaryKeyProvesUnrelated(shape, columnName) {
		return true
	}
	for _, index := range shape.Indexes {
		if !dropIndexProvesUnrelated(index, columnName) {
			return true
		}
	}
	for _, column := range shape.Columns {
		if strings.EqualFold(column.Name, columnName) {
			continue
		}
		if dropOtherColumnMayReference(column) {
			return true
		}
	}
	for _, constraint := range shape.Constraints {
		if primaryKeyConstraint(constraint.Type) {
			continue
		}
		if !dropConstraintProvesUnrelated(shape, constraint, columnName) {
			return true
		}
	}
	return false
}

func dropPrimaryKeyProvesUnrelated(shape *spec.TableSnapshot, columnName string) bool {
	if shape.PrimaryKey != nil && !dropIndexProvesUnrelated(*shape.PrimaryKey, columnName) {
		return false
	}
	for _, constraint := range shape.Constraints {
		if !primaryKeyConstraint(constraint.Type) {
			continue
		}
		if !columnListProvesUnrelated(constraint.Columns, columnName, constraint.UnmodeledParts) {
			return false
		}
	}
	return true
}

func dropIndexProvesUnrelated(index spec.Index, columnName string) bool {
	switch index.Kind {
	case spec.IndexKindPrimary, spec.IndexKindSecondary, spec.IndexKindUnique:
	default:
		return false
	}
	if index.HasExpressionKeys || index.ExpressionCount > 0 || index.PrefixParts > 0 || index.HasPredicate || index.DescParts > 0 || index.Global || len(index.UnmodeledOptions) > 0 {
		return false
	}
	if !columnListProvesUnrelated(index.Columns, columnName, 0) {
		return false
	}
	if len(index.IncludedColumns) > 0 && !columnListProvesUnrelated(index.IncludedColumns, columnName, 0) {
		return false
	}
	return true
}

func dropConstraintProvesUnrelated(shape *spec.TableSnapshot, constraint spec.Constraint, columnName string) bool {
	if constraint.UnmodeledParts > 0 || constraint.UnmodeledReferencedParts > 0 {
		return false
	}
	if !columnListProvesUnrelated(constraint.Columns, columnName, 0) {
		return false
	}
	if shape == nil || shape.Table == nil || strings.TrimSpace(constraint.ReferencedTable) == "" {
		return true
	}
	if !strings.EqualFold(constraint.ReferencedTable, shape.Table.Name) {
		return true
	}
	if refSchema := strings.TrimSpace(constraint.ReferencedSchema); refSchema != "" &&
		strings.TrimSpace(shape.Schema) != "" && !strings.EqualFold(refSchema, shape.Schema) {
		return true
	}
	return columnListProvesUnrelated(constraint.ReferencedColumns, columnName, 0)
}

// dropOtherColumnMayReference refuses a sibling whose stored expression or
// default cannot be proved free of the dropped column. Default text is not
// searched for the column name.
func dropOtherColumnMayReference(column spec.Column) bool {
	if strings.TrimSpace(column.GeneratedWhen) != "" {
		return true
	}
	for _, option := range column.UnextractedOptions {
		switch option {
		case "generated", "reference", "check":
			return true
		}
	}
	return column.HasDefault && !column.DefaultIsNull && !column.DefaultIsCurrentTimestamp
}
