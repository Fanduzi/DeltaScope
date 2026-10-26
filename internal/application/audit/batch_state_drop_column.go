// Package audit publishes the T05-A6 dependency-free DROP COLUMN post-state.
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
	local := dropColumnLocalVerdict(entry.shape, source.Name)
	if local == dropColumnLocalRefused {
		// A provably-multi-column CHECK referencing this column refuses
		// the drop natively (3959, probed MySQL 8.0.46/8.4.10 and TiDB
		// flag-ON) — the statement fails, so column, constraint, and
		// dependents all keep their pre-state.
		return modifyPublication{}, false
	}
	if dropTargetColumnSpecial(source) || local == dropColumnLocalUnsafe || len(dependents) > 0 {
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
	// The column's referencing CHECK members are silently dropped with it —
	// a proven cascade, not a refusal.
	dropColumnCascadedChecks(shape, source.Name)
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

// dropColumnLocalOutcome classifies what loaded local facts prove about
// the column's disappearance.
type dropColumnLocalOutcome int

const (
	// dropColumnLocalSafe means no loaded member provably references the
	// column and every vouched list is a complete set of unrelated names.
	dropColumnLocalSafe dropColumnLocalOutcome = iota
	// dropColumnLocalUnsafe means some fact cannot vouch the drop —
	// publication stays withheld rather than guessing.
	dropColumnLocalUnsafe
	// dropColumnLocalRefused means a provably-multi-column CHECK
	// references the column — the drop natively refuses 3959 and the
	// whole statement fails, so the table keeps its pre-state.
	dropColumnLocalRefused
)

// dropColumnLocalVerdict reports loaded facts about whether the column can
// disappear without rewriting a member. Ordinary indexes are not safe
// merely because a type change could keep them: any reference, or any
// list that is not a complete set of unrelated names, refuses the precise
// template. A CHECK member is the one modeled exception — and only when
// its vouched reference set is exactly the dropped column (the probed
// silent cascade); a reference set that provably reaches other columns
// makes the drop itself a proven refusal.
func dropColumnLocalVerdict(shape *spec.TableSnapshot, columnName string) dropColumnLocalOutcome {
	if shape == nil || shape.PrimaryKeyUnknown || shape.IndexesUnknown || shape.ConstraintsUnknown {
		return dropColumnLocalUnsafe
	}
	if !dropPrimaryKeyProvesUnrelated(shape, columnName) {
		return dropColumnLocalUnsafe
	}
	for _, index := range shape.Indexes {
		if !dropIndexProvesUnrelated(index, columnName) {
			return dropColumnLocalUnsafe
		}
	}
	var table spec.Table
	if shape.Table != nil {
		table = *shape.Table
	}
	dependency := spec.ColumnDependents(table, shape.Columns, columnName)
	if !dependency.Known || len(dependency.Dependents) > 0 {
		return dropColumnLocalUnsafe
	}
	for _, column := range shape.Columns {
		if strings.EqualFold(column.Name, columnName) {
			continue
		}
		if dropOtherColumnMayReference(column) {
			return dropColumnLocalUnsafe
		}
	}
	outcome := dropColumnLocalSafe
	for _, constraint := range shape.Constraints {
		if primaryKeyConstraint(constraint.Type) {
			continue
		}
		if constraint.Type == "check" {
			if checkMayReferenceUnvouched(constraint) {
				return dropColumnLocalUnsafe
			}
			if !checkMemberReferences(constraint, columnName) {
				continue
			}
			if len(constraint.Columns) == 1 {
				// Probed on 8.0.46/8.4.10/TiDB-ON: dropping the column
				// silently drops a check that references only it — a
				// modeled cascade, not an unsafe dependency.
				continue
			}
			// The vouched reference set reaches other columns — the
			// drop refuses 3959 natively. A refusal outranks other
			// unsafe facts only as a proven answer: keep scanning, an
			// unvouched member still makes the outcome undecidable.
			outcome = dropColumnLocalRefused
			continue
		}
		if !dropConstraintProvesUnrelated(shape, constraint, columnName) {
			return dropColumnLocalUnsafe
		}
	}
	return outcome
}

// checkMayReferenceUnvouched reports whether a check member cannot vouch
// its reference set — a withheld expression fact or a member with no
// modeled references at all — so it may reference the dropped column and
// the drop is unsafe to model.
func checkMayReferenceUnvouched(constraint spec.Constraint) bool {
	if constraint.UnmodeledParts > 0 {
		// Unmodeled fragments may carry a reference the column list does
		// not vouch for.
		return true
	}
	if constraint.Expression != nil && !constraint.Expression.Known {
		return true
	}
	return constraint.Expression == nil && len(constraint.Columns) == 0
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

// dropOtherColumnMayReference refuses a sibling carrying an unmodeled fact
// that could still reference the dropped column — a PostgreSQL generation
// attribute, an unextracted generated/reference/check option, or a default
// whose text was never parsed into expression facts. Modeled Generated and
// DefaultExpression records are already answered by the shared
// ColumnDependents computation above. Default text is not searched for the
// column name.
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
	return column.HasDefault && !column.DefaultIsNull && !column.DefaultIsCurrentTimestamp && column.DefaultExpression == nil
}
