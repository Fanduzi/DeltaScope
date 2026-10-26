// Package audit derives the bounded multi-action ALTER TABLE post-state.
package audit

import (
	"context"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// applyMultiActionAlter publishes the bounded net-effect successor for a
// multi-action ALTER TABLE, or the shared tombstone.
func (s *batchState) applyMultiActionAlter(ctx context.Context, statement spec.Statement, targets []spec.Table) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	publication := s.prepareMultiAction(statement, targets)
	if err := ctx.Err(); err != nil {
		return err
	}
	publication.apply(s)
	return nil
}

func (s *batchState) prepareMultiAction(statement spec.Statement, targets []spec.Table) modifyPublication {
	targetKeys := s.modifyTargetKeys(targets)
	dependents := s.modifyIdentityDependentKeys(targetKeys)
	invalidate := func(extra ...batchTableKey) modifyPublication {
		keys := append([]batchTableKey{}, dependents...)
		keys = append(keys, extra...)
		return modifyPublication{invalidate: keys}
	}
	// The multi-action successor is bounded to exactly one table identity
	// matching the statement's own target, a fully audited statement, and a
	// complete known column set — column order is a modeled fact here.
	ddl := statement.DDL
	if len(targets) != 1 || ddl == nil || ddl.Table == nil ||
		!orderedStateDialect(s.dialect) || !fullyAuditedStatement(s.dialect, statement) ||
		len(ddl.Alter) < 2 {
		return invalidate(targetKeys...)
	}
	primary, singleBound := boundSingleTarget(s, ddl, targets)
	if !singleBound {
		return invalidate(targetKeys...)
	}
	entry := s.entries[primary]
	if entry == nil || entry.state == tableUnknown || entry.state == tableAbsent {
		return invalidate(primary)
	}
	if entry.shape == nil || entry.shape.Columns == nil {
		return invalidate(primary)
	}

	// Primary-object ledger: two actions operating on the same column
	// identity are a known failure on both dialect families (MySQL 1054,
	// TiDB 8200) and are never modeled.
	touched := map[string]bool{}
	working := cloneTableSnapshot(entry.shape)
	for i := range ddl.Alter {
		alter := &ddl.Alter[i]
		object, ok := multiActionPrimaryObject(alter)
		if !ok || touched[object] {
			return invalidate(primary)
		}
		validation := working
		if s.dialect == spec.DialectTiDB {
			// TiDB validates every premise against the pre-state.
			validation = entry.shape
		}
		if !multiActionPremiseHolds(s, alter, validation) {
			return invalidate(primary)
		}
		if !multiActionApply(s, statement, alter, working) {
			return invalidate(primary)
		}
		touched[object] = true
	}
	clearModifyAffectedStats(working)
	return modifyPublication{
		invalidate: dependents,
		replaceKey: primary,
		replace: &batchTableEntry{
			state:         tablePresent,
			shape:         working,
			displaySchema: entry.displaySchema,
			displayTable:  entry.displayTable,
		},
	}
}

// multiActionPrimaryObject returns the column identity an action operates on
// inside the bounded template set — the destination name for additions, the
// pre-change name for every other action. An action outside the set, or one
// whose identity cannot be read, is not provable.
func multiActionPrimaryObject(alter *spec.Alter) (string, bool) {
	if alter == nil || alter.HasColumnPosition && (alter.Position == nil || (alter.Position.Kind != "first" && alter.Position.Kind != "after")) {
		return "", false
	}
	if alter.Options["if_exists"] == "true" || alter.Options["if_not_exists"] == "true" {
		return "", false
	}
	if len(alter.UnextractedOptions) > 0 {
		return "", false
	}
	switch alter.Action {
	case "add_columns":
		if alter.Column == nil || alter.Column.Definition == nil {
			return "", false
		}
		if len(alter.Constraints) > 0 {
			// Inline CHECK members share the statement's atomic fate
			// (3822/1054 premise classes the working copy cannot model)
			// — a multi-action add carrying them is unprovable.
			return "", false
		}
		name := strings.TrimSpace(alter.Column.Definition.Name)
		return name, name != ""
	case "modify_column", "drop_column", "set_column_default", "drop_column_default", "set_column_visibility":
		name := strings.TrimSpace(alter.Name)
		return name, name != ""
	case "change_column", "rename_column":
		if _, _, ok := columnIdentityNames(alter); !ok {
			return "", false
		}
		return strings.TrimSpace(alter.Name), true
	default:
		return "", false
	}
}

// multiActionPremiseHolds checks the action's object/position premises
// against the dialect's validation set: every named reference must resolve
// the way the server resolves it, and destination names must not collide.
func multiActionPremiseHolds(s *batchState, alter *spec.Alter, validation *spec.TableSnapshot) bool {
	afterOK := func() bool {
		if alter.Position == nil || alter.Position.Kind != "after" {
			return true
		}
		idx, _ := positionResolveIndex(validation.Columns, alter.Position, "")
		return idx >= 0
	}
	find := func(name string) bool {
		return modifyColumnIndex(validation.Columns, name) >= 0
	}
	switch alter.Action {
	case "add_columns":
		name := strings.TrimSpace(alter.Column.Definition.Name)
		return !find(name) && afterOK()
	case "modify_column":
		return find(alter.Name) && afterOK()
	case "drop_column":
		return find(alter.Name)
	case "set_column_default", "drop_column_default", "set_column_visibility":
		return find(alter.Name)
	case "change_column", "rename_column":
		oldName, newName, _ := columnIdentityNames(alter)
		if !find(oldName) {
			return false
		}
		if !strings.EqualFold(oldName, newName) && find(newName) {
			// A rename/change destination colliding with an existing column
			// is a failed premise.
			return false
		}
		return afterOK()
	default:
		return false
	}
}

// multiActionApply applies one provable action to the working shape. Every
// safety template the single-action publications enforce is re-checked on
// the working shape — a refused template is a tombstone, not a skipped
// action.
func multiActionApply(s *batchState, statement spec.Statement, alter *spec.Alter, working *spec.TableSnapshot) bool {
	switch alter.Action {
	case "add_columns":
		def := alter.Column.Definition
		if modifyColumnIndex(working.Columns, def.Name) >= 0 {
			// TiDB validates the add premise against the pre-state, so a
			// name a sibling already materialized still passes there —
			// the server then refuses the duplicate. Never publish it.
			return false
		}
		next := cloneColumn(*def)
		if alter.Position == nil {
			working.Columns = append(working.Columns, next)
			return true
		}
		if alter.Position.Kind == "first" {
			working.Columns = insertColumnAt(working.Columns, next, 0)
			return true
		}
		idx, ok := positionResolveIndex(working.Columns, alter.Position, def.Name)
		if !ok || idx < 0 {
			return false
		}
		working.Columns = insertColumnAt(working.Columns, next, idx+1)
		return true
	case "modify_column":
		return multiActionModify(s, alter, working)
	case "change_column":
		return multiActionChange(s, alter, working)
	case "rename_column":
		return multiActionRename(s, alter, working)
	case "drop_column":
		return multiActionDrop(alter, working)
	case "set_column_default", "drop_column_default":
		return multiActionSetter(s, alter, working)
	case "set_column_visibility":
		return multiActionVisibility(alter, working)
	default:
		return false
	}
}

func multiActionModify(s *batchState, alter *spec.Alter, working *spec.TableSnapshot) bool {
	index := modifyColumnIndex(working.Columns, alter.Name)
	if index < 0 || !modifyColumnIdentity(alter) {
		return false
	}
	source := working.Columns[index]
	definition := alter.Column.Definition
	if strings.TrimSpace(definition.Type) == "" {
		return false
	}
	inPK, pkKnown := primaryKeyMembership(working, source.Name)
	if !modifyTypeTemplate(source, *definition) ||
		modifyDeclaresExtraMember(alter, *definition) ||
		modifyTiDBPrimaryKeySignedness(s.dialect, inPK, source, *definition) ||
		modifyTiDBAutoIncrementRemoval(s.dialect, source, *definition) ||
		(!pkKnown && !modifyExplicitNotNull(alter, *definition)) ||
		(inPK && modifyExplicitNull(alter, *definition)) ||
		modifyLocalUnsafe(working, source.Name) {
		return false
	}
	next := cloneColumn(*definition)
	if inPK {
		next.NotNull = true
	}
	applyModifyColumnDefaults(&next, working)
	working.Columns[index] = next
	if alter.Position != nil {
		moved, ok := applyColumnPosition(working.Columns, index, alter.Position, next.Name)
		if !ok {
			return false
		}
		working.Columns = moved
	}
	return true
}

func multiActionChange(s *batchState, alter *spec.Alter, working *spec.TableSnapshot) bool {
	oldName, newName, _ := columnIdentityNames(alter)
	index := modifyColumnIndex(working.Columns, oldName)
	if index < 0 || columnIdentityConflict(working.Columns, index, oldName, newName) ||
		columnIdentityLocalUnsafe(working, oldName, newName) {
		return false
	}
	definition := alter.Column.Definition
	if strings.TrimSpace(definition.Type) == "" {
		return false
	}
	source := working.Columns[index]
	inPK, pkKnown := primaryKeyMembership(working, source.Name)
	if !modifyTypeTemplate(source, *definition) ||
		modifyDeclaresExtraMember(alter, *definition) ||
		modifyTiDBPrimaryKeySignedness(s.dialect, inPK, source, *definition) ||
		modifyTiDBAutoIncrementRemoval(s.dialect, source, *definition) ||
		(!pkKnown && !modifyExplicitNotNull(alter, *definition)) ||
		(inPK && modifyExplicitNull(alter, *definition)) {
		return false
	}
	next := cloneColumn(*definition)
	if inPK {
		next.NotNull = true
	}
	applyModifyColumnDefaults(&next, working)
	working.Columns[index] = next
	rewriteIdentityColumnRefs(working, oldName, newName)
	if alter.Position != nil {
		moved, ok := applyColumnPosition(working.Columns, index, alter.Position, newName)
		if !ok {
			return false
		}
		working.Columns = moved
	}
	return true
}

func multiActionRename(s *batchState, alter *spec.Alter, working *spec.TableSnapshot) bool {
	oldName, newName, _ := columnIdentityNames(alter)
	index := modifyColumnIndex(working.Columns, oldName)
	if index < 0 || columnIdentityConflict(working.Columns, index, oldName, newName) ||
		columnIdentityLocalUnsafe(working, oldName, newName) {
		return false
	}
	if !spec.RenameColumnVersionSupportFor(s.resolvedVersion).Supported {
		return false
	}
	next := cloneColumn(working.Columns[index])
	next.Name = newName
	working.Columns[index] = next
	rewriteIdentityColumnRefs(working, oldName, newName)
	return true
}

func multiActionDrop(alter *spec.Alter, working *spec.TableSnapshot) bool {
	index, count := dropColumnMatch(working.Columns, alter.Name)
	if count != 1 || len(working.Columns) < 2 {
		return false
	}
	source := working.Columns[index]
	if dropTargetColumnSpecial(source) || dropColumnLocalVerdict(working, source.Name) != dropColumnLocalSafe {
		return false
	}
	working.Columns = removeColumnAt(working.Columns, index)
	dropColumnCascadedChecks(working, source.Name)
	return true
}

func multiActionSetter(s *batchState, alter *spec.Alter, working *spec.TableSnapshot) bool {
	if alter.Column == nil || alter.Column.Definition == nil {
		return false
	}
	index := modifyColumnIndex(working.Columns, alter.Column.OldName)
	if index < 0 {
		return false
	}
	next := cloneColumn(working.Columns[index])
	if !s.publishColumnDefault(&next, alter) {
		return false
	}
	working.Columns[index] = next
	return true
}

func multiActionVisibility(alter *spec.Alter, working *spec.TableSnapshot) bool {
	if alter.Column == nil || alter.Column.Definition == nil {
		return false
	}
	visibility := alter.Column.Definition.Visibility
	if visibility != spec.ColumnVisibilityVisible && visibility != spec.ColumnVisibilityInvisible {
		return false
	}
	index := modifyColumnIndex(working.Columns, alter.Column.OldName)
	if index < 0 {
		return false
	}
	next := cloneColumn(working.Columns[index])
	next.Visibility = visibility
	next.VisibilityExplicit = false
	working.Columns[index] = next
	return true
}
