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
// MySQL ≥8.0.19 boundary (probed 8.0.46/8.4.10): NOT ENFORCED lands
// ENFORCED=NO unconditionally and idempotently — a deterministic
// post-state. ENFORCED is data-conditional (3819 refuses when violating
// rows exist), so the member survives with its enforcement unvouched.
// Missing and non-check names refuse 3821 — provably unchanged. MySQL
// <8.0.19 rejects the syntax (5.7: 1064) — provably unchanged. TiDB flag ON
// mirrors MySQL (3940 refusal class); flag OFF is a parse-only no-op; an
// unobserved flag leaves the member's enforced state undecidable.
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
	version := s.resolvedVersion
	if arm == spec.CheckArmIgnored {
		return
	}
	if s.dialect == spec.DialectMySQL && !spec.CheckAlterSyntaxSupported(s.dialect, version) && arm == spec.CheckArmEnforced {
		// Enforcing but pre-8.0.19 — the syntax refuses; unchanged.
		return
	}
	name := strings.TrimSpace(alter.Name)
	if name == "" {
		return
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
			idx := memberIndex(entry.shape, member)
			s.withClonedShape(key, func(shape *spec.TableSnapshot) {
				shape.Constraints[idx].Enforced = ""
			})
		}
		return
	}
	// Enforcing arm: the statement reaches target binding.
	if entry.shape.CheckNameSpaceUnvouched() {
		// The target may hide an unproven member — removal/flip is
		// unprovable either way.
		s.markConstraintsUnknown(key)
		return
	}
	member := entry.shape.FindCheckConstraint(name)
	if member == nil {
		// Missing or proven non-check name: refused (3821/3940) —
		// provably unchanged.
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
	if name == "" {
		// The server derives t_chk_N — never authored, so name-keyed
		// lookups must never bind it.
		declared.NameUnknown = true
	}
	s.withClonedShape(key, func(shape *spec.TableSnapshot) {
		shape.Constraints = append(shape.Constraints, cloneConstraint(declared))
	})
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

// dropColumnCascadedChecks filters out CHECK members whose column list
// references the dropped column — the native silent cascade probed on
// 8.0.46/8.4.10/TiDB-ON. Members whose reference set cannot vouch are
// handled by the unsafe path before this runs.
func dropColumnCascadedChecks(shape *spec.TableSnapshot, columnName string) {
	if shape == nil || len(shape.Constraints) == 0 {
		return
	}
	kept := shape.Constraints[:0]
	for _, member := range shape.Constraints {
		if member.Type == "check" && checkMemberReferences(member, columnName) {
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
