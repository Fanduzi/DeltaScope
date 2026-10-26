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

	// Whole-statement premise scan: an ALTER fails atomically, so a
	// provably refused action at any position keeps the table's pre-state
	// exactly — earlier siblings never partially publish (probed 8.0.46:
	// a 1064/3959 action leaves every sibling column and member
	// untouched). The CHECK-referenced rename/change premise is evaluated
	// on the pre-state — the refusal does not depend on sibling order.
	for i := range ddl.Alter {
		alter := &ddl.Alter[i]
		if multiActionCheckActionRefused(s, alter) ||
			multiActionIdentityCheckRefused(s, alter, entry.shape) {
			return modifyPublication{replaceKey: primary, replace: entry}
		}
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
		if !multiActionPremiseHolds(alter, validation) {
			return invalidate(primary)
		}
		switch multiActionApply(s, alter, working) {
		case multiActionApplyRefused:
			return modifyPublication{replaceKey: primary, replace: entry}
		case multiActionApplyUnprovable:
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
func multiActionPremiseHolds(alter *spec.Alter, validation *spec.TableSnapshot) bool {
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

// multiActionVerdict is the three-way outcome of applying one action to
// the working shape: applied, provably refused (the whole statement keeps
// its pre-state), or unprovable (the bounded successor is withheld).
type multiActionVerdict int

const (
	multiActionApplyOK multiActionVerdict = iota
	multiActionApplyRefused
	multiActionApplyUnprovable
)

// multiActionIdentityCheckRefused reports whether a rename/change action
// inside a multi-action statement provably refuses through the CHECK
// identity premise — a proven member references the renamed column and
// the arm answers 3959 (MySQL always; TiDB refuses RENAME but cascades
// CHANGE). The premise is a statement-level validation read on the
// pre-state — the same shape the server validates against before any
// action executes — so sibling order cannot weaken it. Only the proven
// refusal counts here; an unvouched member set or an arm that cannot
// decide stays with the ordinary unprovable path.
func multiActionIdentityCheckRefused(s *batchState, alter *spec.Alter, shape *spec.TableSnapshot) bool {
	if alter.Action != "rename_column" && alter.Action != "change_column" {
		return false
	}
	oldName, newName, ok := columnIdentityNames(alter)
	if !ok {
		return false
	}
	return s.checkColumnIdentityOutcome(shape, alter.Action, oldName, newName) == checkIdentityRefused
}

// multiActionCheckActionRefused reports whether a CHECK|CONSTRAINT
// alter/drop action inside a multi-action statement is a proven syntax
// refusal on the resolved version — the CONSTRAINT spelling on MySQL
// 8.0.16–8.0.18 refuses 1064 and fails the whole statement atomically.
// Supported and unproven actions stay outside the bounded template and
// keep their invalidate answer.
func multiActionCheckActionRefused(s *batchState, alter *spec.Alter) bool {
	if alter.Action != "drop_check" && alter.Action != "alter_check" {
		return false
	}
	return spec.CheckAlterSyntaxOutcome(s.dialect, s.resolvedVersion, alter.Options["check_spelling"]) == spec.CheckSyntaxRefused
}

// multiActionApply applies one provable action to the working shape. Every
// safety template the single-action publications enforce is re-checked on
// the working shape — a proven refusal keeps the pre-state, an unprovable
// template withholds the successor, never a skipped action.
func multiActionApply(s *batchState, alter *spec.Alter, working *spec.TableSnapshot) multiActionVerdict {
	applied := func(ok bool) multiActionVerdict {
		if !ok {
			return multiActionApplyUnprovable
		}
		return multiActionApplyOK
	}
	switch alter.Action {
	case "add_columns":
		def := alter.Column.Definition
		if modifyColumnIndex(working.Columns, def.Name) >= 0 {
			// TiDB validates the add premise against the pre-state, so a
			// name a sibling already materialized still passes there —
			// the server then refuses the duplicate. Never publish it.
			return multiActionApplyUnprovable
		}
		next := cloneColumn(*def)
		if alter.Position == nil {
			working.Columns = append(working.Columns, next)
			return multiActionApplyOK
		}
		if alter.Position.Kind == "first" {
			working.Columns = insertColumnAt(working.Columns, next, 0)
			return multiActionApplyOK
		}
		idx, ok := positionResolveIndex(working.Columns, alter.Position, def.Name)
		if !ok || idx < 0 {
			return multiActionApplyUnprovable
		}
		working.Columns = insertColumnAt(working.Columns, next, idx+1)
		return multiActionApplyOK
	case "modify_column":
		return applied(multiActionModify(s, alter, working))
	case "change_column":
		return multiActionChange(s, alter, working)
	case "rename_column":
		return multiActionRename(s, alter, working)
	case "drop_column":
		return applied(multiActionDrop(alter, working))
	case "set_column_default", "drop_column_default":
		return applied(multiActionSetter(s, alter, working))
	case "set_column_visibility":
		return applied(multiActionVisibility(alter, working))
	default:
		return multiActionApplyUnprovable
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

// multiActionChangeRenameVerdict runs the shared CHECK identity gate for a
// rename/change action inside a multi-action statement — the same premise
// the single-action publication enforces. A proven 3959 refuses the whole
// statement atomically; an unvouched member set withholds the successor.
func multiActionChangeRenameVerdict(s *batchState, alter *spec.Alter, working *spec.TableSnapshot, oldName, newName string) multiActionVerdict {
	switch s.checkColumnIdentityOutcome(working, alter.Action, oldName, newName) {
	case checkIdentityRefused:
		return multiActionApplyRefused
	case checkIdentityUnsafe:
		return multiActionApplyUnprovable
	case checkIdentityCascade:
		// TiDB flag-ON CHANGE silently drops every referencing member —
		// single- and multi-column references alike (probed v8.5.0).
		dropIdentityCascadedChecks(working, oldName)
	}
	return multiActionApplyOK
}

func multiActionChange(s *batchState, alter *spec.Alter, working *spec.TableSnapshot) multiActionVerdict {
	oldName, newName, _ := columnIdentityNames(alter)
	index := modifyColumnIndex(working.Columns, oldName)
	if index < 0 || columnIdentityConflict(working.Columns, index, oldName, newName) {
		return multiActionApplyUnprovable
	}
	if verdict := multiActionChangeRenameVerdict(s, alter, working, oldName, newName); verdict != multiActionApplyOK {
		return verdict
	}
	if columnIdentityLocalUnsafe(working, oldName, newName) {
		return multiActionApplyUnprovable
	}
	definition := alter.Column.Definition
	if strings.TrimSpace(definition.Type) == "" {
		return multiActionApplyUnprovable
	}
	source := working.Columns[index]
	inPK, pkKnown := primaryKeyMembership(working, source.Name)
	if !modifyTypeTemplate(source, *definition) ||
		modifyDeclaresExtraMember(alter, *definition) ||
		modifyTiDBPrimaryKeySignedness(s.dialect, inPK, source, *definition) ||
		modifyTiDBAutoIncrementRemoval(s.dialect, source, *definition) ||
		(!pkKnown && !modifyExplicitNotNull(alter, *definition)) ||
		(inPK && modifyExplicitNull(alter, *definition)) {
		return multiActionApplyUnprovable
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
			return multiActionApplyUnprovable
		}
		working.Columns = moved
	}
	return multiActionApplyOK
}

func multiActionRename(s *batchState, alter *spec.Alter, working *spec.TableSnapshot) multiActionVerdict {
	oldName, newName, _ := columnIdentityNames(alter)
	index := modifyColumnIndex(working.Columns, oldName)
	if index < 0 || columnIdentityConflict(working.Columns, index, oldName, newName) {
		return multiActionApplyUnprovable
	}
	if verdict := multiActionChangeRenameVerdict(s, alter, working, oldName, newName); verdict != multiActionApplyOK {
		return verdict
	}
	if columnIdentityLocalUnsafe(working, oldName, newName) {
		return multiActionApplyUnprovable
	}
	if !spec.RenameColumnVersionSupportFor(s.resolvedVersion).Supported {
		return multiActionApplyUnprovable
	}
	next := cloneColumn(working.Columns[index])
	next.Name = newName
	working.Columns[index] = next
	rewriteIdentityColumnRefs(working, oldName, newName)
	return multiActionApplyOK
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
