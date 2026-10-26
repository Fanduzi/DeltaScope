package audit

import (
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// T12 CHECK ordered-state transitions. The arm classifier
// (spec.ResolveCheckArm) shares the rule layer's answer: parse+ignore arms
// (MySQL <8.0.16, TiDB flag OFF) land and mutate nothing; unobserved
// governing facts leave the affected fact undecidable rather than guessing.

// checkArmFor resolves the statement's CHECK arm from the batch state's
// observed facts — the same classification the rules read.
func (s *batchState) checkArmFor() spec.CheckArm {
	return spec.ResolveCheckArm(s.dialect, s.resolvedVersion, s.instanceFacts)
}

// firstAlterCheckAlter returns the sole alter_check action when the
// statement is exactly one ALTER CHECK|CONSTRAINT — both authored keywords
// share the normalized action, and both flip the same check member on every
// probed arm, so the keyword split never matters.
func firstAlterCheckAlter(ddl *spec.DDL) *spec.Alter {
	if ddl == nil || len(ddl.Alter) != 1 {
		return nil
	}
	alter := &ddl.Alter[0]
	if alter.Action != "alter_check" {
		return nil
	}
	return alter
}

// applyAlterCheck publishes the bounded post-state of ALTER CHECK|CONSTRAINT
// x [NOT] ENFORCED.
//
// NOT ENFORCED lands ENFORCED=NO unconditionally and idempotently
// (probed 8.0.46/8.4.10) — a deterministic post-state. ENFORCED is
// data-conditional (3819 refuses when violating rows exist), so the
// member survives with its enforcement unvouched. The authored keyword
// decides the provable refusal class: CHECK spelling needs a CHECK member
// (else 3821), the generic CONSTRAINT spelling needs a lone check member
// (else 3940 missing / 3941 wrong-kind / 3939 cross-type ambiguity) —
// every refusal publishes unchanged. MySQL gates the keywords separately
// (CHECK 8.0.16, generic 8.0.19; probed on the 8.0.17 band anchor); an
// unproven gate leaves the member's enforcement undecidable. TiDB flag ON
// binds the CHECK member first under either spelling; flag OFF is a
// parse-only no-op.
func (s *batchState) applyAlterCheck(_ spec.Statement, alter *spec.Alter, targets []spec.Table) {
	if len(targets) != 1 {
		s.invalidateAll(targets)
		return
	}
	key := s.keyFor(s.schema, targets[0])
	entry := s.entries[key]
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		return
	}
	arm := s.checkArmFor()
	if arm == spec.CheckArmIgnored {
		return
	}
	name := strings.TrimSpace(alter.Name)
	if name == "" {
		return
	}
	// unvouchEnforced records the member casualty when execution itself is
	// undecidable — the member survives either way (a refusal changes
	// nothing) but its enforcement may have flipped.
	unvouchEnforced := func(member *spec.Constraint) {
		idx := memberIndex(entry.shape, member)
		s.withClonedShape(key, func(shape *spec.TableSnapshot) {
			shape.Constraints[idx].Enforced = ""
		})
	}
	if s.dialect == spec.DialectMySQL {
		switch spec.CheckAlterSyntaxOutcome(s.dialect, s.resolvedVersion, alter.Options["check_spelling"]) {
		case spec.CheckSyntaxRefused:
			// The authored spelling provably refuses (pre-8.0.16, or the
			// generic keyword before 8.0.19) — provably unchanged.
			return
		case spec.CheckSyntaxUnproven:
			// Execution undecidable: a bound CHECK member may have
			// flipped or refused; an unvouched name space cannot even
			// vouch the target.
			if entry.shape.CheckNameSpaceUnvouched() {
				s.markConstraintsUnknown(key)
				return
			}
			if member := entry.shape.FindCheckConstraint(name); member != nil {
				unvouchEnforced(member)
			}
			return
		}
	}
	if arm == spec.CheckArmUnknown {
		// The statement may have flipped, no-oped, or refused — the
		// member's enforcement state is the only decidable casualty, and
		// only when the member itself is proven. An unvouched name space
		// cannot even vouch the target, so the collection unvouches.
		if entry.shape.CheckNameSpaceUnvouched() {
			s.markConstraintsUnknown(key)
			return
		}
		if member := entry.shape.FindCheckConstraint(name); member != nil {
			unvouchEnforced(member)
		}
		return
	}
	// Enforcing arm with proven syntax: the statement reaches binding.
	res := spec.CheckAlterTarget(s.dialect, entry.shape, alter.Options["check_spelling"], name)
	switch res.Binding {
	case spec.CheckTargetMissing, spec.CheckTargetWrongKind, spec.CheckTargetAmbiguous:
		// Proven refusal (3821/3940/3941/3939) — unchanged.
		return
	case spec.CheckTargetUnproven:
		// A consulted name space cannot vouch the target, or TiDB's
		// unprobed generic binding leaves the member fate undecidable.
		if entry.shape.CheckNameSpaceUnvouched() ||
			entry.shape.ForeignKeyNameSpaceUnvouched() ||
			entry.shape.IndexNameSpaceUnvouched() {
			s.markConstraintsUnknown(key)
			return
		}
		if member := entry.shape.FindCheckConstraint(name); member != nil {
			unvouchEnforced(member)
		}
		return
	}
	member := res.Constraint
	if member == nil {
		return
	}
	idx := memberIndex(entry.shape, member)
	if alter.Options["check_enforced"] == "true" {
		// ENFORCED validates existing data — the flip may refuse, so the
		// surviving member's enforcement is unproven.
		s.withClonedShape(key, func(shape *spec.TableSnapshot) {
			shape.Constraints[idx].Enforced = ""
		})
		return
	}
	s.withClonedShape(key, func(shape *spec.TableSnapshot) {
		shape.Constraints[idx].Enforced = "not_enforced"
	})
}

// applyAddCheck publishes a declared ADD CONSTRAINT ... CHECK member — or
// its provable non-landing on ignored arms. Landing is arm-gated: MySQL
// ≥8.0.16 stores it with the declared enforcement (default ENFORCED); an
// authored name colliding with a proven check member refuses 3822 and
// changes nothing. Unobserved arms cannot answer landing, so the member
// collection unvouches rather than fabricating presence or absence.
func (s *batchState) applyAddCheck(_ spec.Statement, alter *spec.Alter, targets []spec.Table) {
	if len(targets) != 1 || alter.Constraint == nil {
		s.invalidateAll(targets)
		return
	}
	key := s.keyFor(s.schema, targets[0])
	entry := s.entries[key]
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		return
	}
	switch s.checkArmFor() {
	case spec.CheckArmIgnored:
		return
	case spec.CheckArmUnknown:
		s.markConstraintsUnknown(key)
		return
	}
	declared := *alter.Constraint
	name := strings.TrimSpace(declared.Name)
	if name != "" && entry.shape.FindCheckConstraint(name) != nil {
		// Duplicate check name refuses 3822 — provably unchanged.
		return
	}
	if entry.shape.CheckNameSpaceUnvouched() {
		s.markConstraintsUnknown(key)
		return
	}
	// Referenced-column premise: the expression binds against the CURRENT
	// table — a provably-missing referenced column refuses the whole
	// statement (1054, probed on 8.0.46 and TiDB-ON), so nothing lands. A
	// column set or reference set that cannot vouch leaves landing
	// undecidable — the collection unvouches rather than guessing either
	// landing or refusal.
	if checkMayReferenceUnvouched(declared) || entry.shape.Columns == nil {
		s.markConstraintsUnknown(key)
		return
	}
	for _, column := range declared.Columns {
		if entry.shape.FindColumn(column) == nil {
			return
		}
	}
	if name == "" {
		// The server derives t_chk_N — never authored, so name-keyed
		// lookups must never bind it.
		declared.NameUnknown = true
	}
	s.withClonedShape(key, func(shape *spec.TableSnapshot) {
		shape.Constraints = append(shape.Constraints, cloneConstraint(declared))
	})
}

// inColumnCheckOutcome classifies whether an ADD COLUMN statement
// carrying inline CHECK members can publish its column at all — the
// column and its inline members share ONE statement result (probed: a
// colliding check name refuses the whole statement 3822 and the new
// column never lands).
type inColumnCheckOutcome int

const (
	// inColumnCheckLand means the column may publish; member landing is
	// decided per arm afterwards.
	inColumnCheckLand inColumnCheckOutcome = iota
	// inColumnCheckRefused means a proven collision or missing referenced
	// column refuses the whole statement — nothing lands.
	inColumnCheckRefused
	// inColumnCheckUndecidable means the statement may land or refuse —
	// the column's fate is unprovable.
	inColumnCheckUndecidable
)

// inColumnChecksOutcome evaluates the atomic premise before the column
// publishes: a proven check-name collision (3822) or a provably-missing
// referenced column (1054) refuses the statement; an unvouched check name
// space, unvouched column set, or unvouched reference set makes landing
// undecidable. The ignored arm parses the check away — the column always
// lands. The unknown arm stays landable only while every member's name is
// provably fresh (a collision would refuse under the ON reading).
func (s *batchState) inColumnChecksOutcome(shape *spec.TableSnapshot, columnName string, checks []spec.Constraint) inColumnCheckOutcome {
	checksOnly := 0
	for i := range checks {
		if checks[i].Type == "check" {
			checksOnly++
		}
	}
	if checksOnly == 0 {
		return inColumnCheckLand
	}
	arm := s.checkArmFor()
	for _, member := range checks {
		if member.Type == "check" && spec.CheckExplicitClauseRefused(s.dialect, arm, member) {
			// MySQL's ignored arm refuses an authored enforcement clause
			// outright (1064) — the column shares the refusal and never
			// lands. TiDB's ignored arm accepts and ignores it.
			return inColumnCheckRefused
		}
	}
	switch arm {
	case spec.CheckArmIgnored:
		return inColumnCheckLand
	case spec.CheckArmUnknown:
		if shape.CheckNameSpaceUnvouched() {
			return inColumnCheckUndecidable
		}
		seen := map[string]bool{}
		for _, member := range checks {
			if member.Type != "check" {
				continue
			}
			name := strings.TrimSpace(member.Name)
			if name == "" || seen[strings.ToLower(name)] {
				continue
			}
			seen[strings.ToLower(name)] = true
			if shape.FindCheckConstraint(name) != nil {
				return inColumnCheckUndecidable
			}
		}
		return inColumnCheckLand
	}
	// Enforcing arm: MySQL refuses a colliding name (3822) or a missing
	// referenced column (1054) atomically — nothing lands. TiDB flag-ON
	// diverges (probed): a colliding in-column name is silently ignored
	// while the column still lands; its missing-reference answer is
	// unprobed, so only the name collision is provably landable there.
	if shape.CheckNameSpaceUnvouched() || shape.Columns == nil {
		return inColumnCheckUndecidable
	}
	seen := map[string]bool{}
	for _, member := range checks {
		if member.Type != "check" {
			continue
		}
		name := strings.TrimSpace(member.Name)
		if name != "" {
			if s.dialect == spec.DialectMySQL &&
				(seen[strings.ToLower(name)] || shape.FindCheckConstraint(name) != nil) {
				return inColumnCheckRefused
			}
			seen[strings.ToLower(name)] = true
		}
		if checkMayReferenceUnvouched(member) {
			return inColumnCheckUndecidable
		}
		for _, ref := range member.Columns {
			if strings.EqualFold(ref, columnName) {
				continue
			}
			if shape.FindColumn(ref) == nil {
				if s.dialect == spec.DialectMySQL {
					return inColumnCheckRefused
				}
				return inColumnCheckUndecidable
			}
		}
	}
	return inColumnCheckLand
}

// applyInColumnChecks lands CHECK members declared inside a column
// definition that itself just landed (ADD/MODIFY/CHANGE COLUMN) — the same
// arm gating as applyAddCheck.
func (s *batchState) applyInColumnChecks(key batchTableKey, checks []spec.Constraint) {
	if len(checks) == 0 {
		return
	}
	entry := s.entries[key]
	if entry == nil || entry.state != tablePresent || entry.shape == nil {
		return
	}
	switch s.checkArmFor() {
	case spec.CheckArmIgnored:
		return
	case spec.CheckArmUnknown:
		s.markConstraintsUnknown(key)
		return
	}
	s.withClonedShape(key, func(shape *spec.TableSnapshot) {
		for _, declared := range checks {
			member := cloneConstraint(declared)
			name := strings.TrimSpace(member.Name)
			if name != "" && shape.FindCheckConstraint(name) != nil {
				if s.dialect == spec.DialectTiDB {
					// TiDB flag-ON probed: a colliding in-column name is
					// silently ignored — the member never lands while the
					// column does.
					continue
				}
				// Duplicate check name would refuse the whole statement —
				// cannot happen on a landed column; keep it unvouched.
				shape.ConstraintsUnknown = true
				return
			}
			if name == "" {
				member.NameUnknown = true
			}
			shape.Constraints = append(shape.Constraints, member)
		}
	})
}

// dropColumnCascadedChecks filters out CHECK members whose vouched
// reference set is exactly the dropped column — the only shape the probed
// silent cascade takes (8.0.46/8.4.10/TiDB-ON). A member that references
// other columns too makes the drop refuse 3959 upstream and never
// reaches publication; an unvouched reference set is gated by the unsafe
// path before this runs.
func dropColumnCascadedChecks(shape *spec.TableSnapshot, columnName string) {
	if shape == nil || len(shape.Constraints) == 0 {
		return
	}
	kept := shape.Constraints[:0]
	for _, member := range shape.Constraints {
		if member.Type == "check" && len(member.Columns) == 1 && strings.EqualFold(member.Columns[0], columnName) {
			continue
		}
		kept = append(kept, member)
	}
	for i := len(kept); i < len(shape.Constraints); i++ {
		shape.Constraints[i] = spec.Constraint{}
	}
	shape.Constraints = kept
}

// checkMemberReferences reports whether a check member's modeled column
// list provably references the column — the only silent-cascade arm.
func checkMemberReferences(member spec.Constraint, columnName string) bool {
	if member.Expression != nil && !member.Expression.Known {
		// Withheld expression facts cannot vouch the reference set — the
		// member is not safe to cascade (the unsafe gate handles it).
		return false
	}
	for _, col := range member.Columns {
		if strings.EqualFold(col, columnName) {
			return true
		}
	}
	return false
}

// checkIdentityOutcome answers what an ordered RENAME/CHANGE COLUMN does to
// proven CHECK members on the resolved arm — the probed arms diverge:
// MySQL 8.x refuses the rename outright (3959), TiDB flag-ON refuses
// RENAME COLUMN (3959) but lets CHANGE COLUMN land while silently dropping
// the referencing member. Members whose reference set cannot be vouched
// keep the whole transition unsafe.
type checkIdentityOutcome int

const (
	// checkIdentityProceed means every proven member ignores the renamed
	// column (or names do not change) — the generic path decides the rest.
	checkIdentityProceed checkIdentityOutcome = iota
	// checkIdentityRefused means a proven member's reference makes the
	// statement natively refuse 3959 — the table provably keeps its
	// pre-state.
	checkIdentityRefused
	// checkIdentityCascade means CHANGE COLUMN on TiDB flag-ON silently
	// drops the referencing members while the rename itself lands.
	checkIdentityCascade
	// checkIdentityUnsafe means a member's reference set is unvouched, or
	// the arm cannot decide refuse/drop/proceed — publication must stay
	// withheld.
	checkIdentityUnsafe
)

// checkColumnIdentityOutcome classifies the check-member arm of a
// RENAME/CHANGE COLUMN. A same-name declaration never rebinds references.
// A member present on MySQL is self-vouching enforcement (storage itself
// proves the ≥8.0.16 arm) so its 3959 refusal is proven even when the
// version fact is unobserved; TiDB needs the observed flag arm because a
// member under an OFF flag is contradictory and an unobserved flag cannot
// decide between refusal and silent drop.
func (s *batchState) checkColumnIdentityOutcome(shape *spec.TableSnapshot, action, oldName, newName string) checkIdentityOutcome {
	if shape == nil || strings.EqualFold(oldName, newName) {
		return checkIdentityProceed
	}
	outcome := checkIdentityProceed
	for i := range shape.Constraints {
		member := &shape.Constraints[i]
		if member.Type != "check" {
			continue
		}
		if checkMayReferenceUnvouched(*member) {
			return checkIdentityUnsafe
		}
		if !checkMemberReferences(*member, oldName) {
			continue
		}
		if s.dialect == spec.DialectMySQL {
			return checkIdentityRefused
		}
		if s.checkArmFor() != spec.CheckArmEnforced {
			return checkIdentityUnsafe
		}
		if action == "change_column" {
			outcome = checkIdentityCascade
			continue
		}
		return checkIdentityRefused
	}
	return outcome
}

// memberIndex resolves a FindCheckConstraint-style slice pointer back to
// its index for a mutation inside withClonedShape.
func memberIndex(shape *spec.TableSnapshot, member *spec.Constraint) int {
	for i := range shape.Constraints {
		if &shape.Constraints[i] == member {
			return i
		}
	}
	return -1
}
