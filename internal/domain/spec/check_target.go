// Package spec — CHECK target binding. Shared by the rule layer
// (Evaluate/EvidenceGaps) and the ordered-state machine so a name-keyed
// ALTER|DROP CHECK|CONSTRAINT answers identically everywhere: proven
// target, proven refusal class, or unproven name space — never a guessed
// first-match.
package spec

import "strings"

// CheckTargetBinding classifies what an authored name binds on the proven
// collections under the action's spelling — the three honest answers a
// name-keyed CHECK|CONSTRAINT operation can give.
type CheckTargetBinding int

const (
	// CheckTargetProven means the name binds exactly one executable
	// target under the action's spelling — the operation provably lands.
	CheckTargetProven CheckTargetBinding = iota
	// CheckTargetMissing means the name binds nothing in the applicable
	// namespace — a proven refusal (3821/3940).
	CheckTargetMissing
	// CheckTargetWrongKind means the name binds members the action cannot
	// operate on — a proven refusal (3821 on DROP CHECK, 3941 on ALTER
	// CONSTRAINT against a non-check target).
	CheckTargetWrongKind
	// CheckTargetAmbiguous means a cross-type same-name set provably
	// refuses on MySQL (3939) — the generic keyword cannot pick.
	CheckTargetAmbiguous
	// CheckTargetUnproven means a name space the binding consults cannot
	// vouch, or the authored spelling was unrecoverable — the evidence
	// gap is owed, never a fabricated answer either way.
	CheckTargetUnproven
)

// CheckTargetKind names the member kind a proven binding resolved to.
type CheckTargetKind string

const (
	CheckTargetKindCheck      CheckTargetKind = "check"
	CheckTargetKindUnique     CheckTargetKind = "unique"
	CheckTargetKindPrimary    CheckTargetKind = "primary"
	CheckTargetKindForeignKey CheckTargetKind = "foreign_key"
)

// CheckTarget carries a resolved binding plus the member it provably
// names — Constraint points into the snapshot's Constraints slice for
// check/foreign_key kinds, Index into Indexes for unique/primary members,
// and PKObject marks the primary-key object (which lives outside both
// member collections in the model).
type CheckTarget struct {
	Binding    CheckTargetBinding
	Kind       CheckTargetKind
	Constraint *Constraint
	Index      *Index
	PKObject   bool
}

// genericConstraintKinds returns the distinct kinds an authored name
// binds in the server's generic CONSTRAINT namespace {check, unique, pk,
// fk} — secondary indexes are never members (probed: the generic keyword
// reports 3940 "does not exist" on a secondary-index name on 8.0.46).
// indexKindUnproven marks a proven-named index member whose stored kind
// is unproven: it may hide a unique binding the generic keyword would
// remove, so callers can never answer Missing against it.
func genericConstraintKinds(snapshot *TableSnapshot, name string) (kinds map[CheckTargetKind]bool, indexKindUnproven bool) {
	kinds = map[CheckTargetKind]bool{}
	if snapshot == nil || name == "" {
		return kinds, false
	}
	if snapshot.FindCheckConstraint(name) != nil {
		kinds[CheckTargetKindCheck] = true
	}
	if snapshot.FindForeignKeyConstraint(name) != nil {
		kinds[CheckTargetKindForeignKey] = true
	}
	if member := snapshot.FindProvenIndex(name); member != nil {
		switch member.Kind {
		case IndexKindUnique:
			kinds[CheckTargetKindUnique] = true
		case IndexKindPrimary:
			kinds[CheckTargetKindPrimary] = true
		case IndexKindSecondary:
			// Provably outside the constraint namespace (probed 3940).
		default:
			indexKindUnproven = true
		}
	}
	if pkName, proven := PrimaryKeyObjectName(snapshot); proven && strings.EqualFold(pkName, name) {
		kinds[CheckTargetKindPrimary] = true
	}
	if snapshot.PrimaryKey != nil && strings.EqualFold(name, "primary") {
		kinds[CheckTargetKindPrimary] = true
	}
	return kinds, indexKindUnproven
}

// checkTargetNameSpaceUnvouched reports whether the collections the
// action's spelling consults cannot all vouch. A CHECK-spelled action
// only consults the CHECK name space — an unrelated index member's
// unproven name can never drag its proven absence into a gap — while the
// generic and unproven spellings consult the full constraint/index
// member sets.
func checkTargetNameSpaceUnvouched(snapshot *TableSnapshot, spelling string) bool {
	if snapshot.CheckNameSpaceUnvouched() {
		return true
	}
	if spelling == CheckSpellingCheck {
		return false
	}
	return snapshot.ForeignKeyNameSpaceUnvouched() || snapshot.IndexNameSpaceUnvouched()
}

// checkTargetMember fills t with the single kind that provably bound —
// caller guarantees exactly one kind matched.
func checkTargetMember(snapshot *TableSnapshot, name string, kind CheckTargetKind, t *CheckTarget) {
	t.Kind = kind
	switch kind {
	case CheckTargetKindCheck:
		t.Constraint = snapshot.FindCheckConstraint(name)
	case CheckTargetKindForeignKey:
		t.Constraint = snapshot.FindForeignKeyConstraint(name)
	case CheckTargetKindUnique:
		t.Index = snapshot.FindProvenIndex(name)
	case CheckTargetKindPrimary:
		if member := snapshot.FindProvenIndex(name); member != nil && member.Kind == IndexKindPrimary {
			t.Index = member
		} else {
			t.PKObject = true
		}
	}
}

// CheckDropTarget resolves what a DROP CHECK|CONSTRAINT name provably
// binds. Probed semantics (8.0.46/8.4.10, TiDB v8.5.0 flag-ON):
//   - CHECK spelling binds CHECK members only — a non-check or missing
//     name refuses 3821.
//   - Generic CONSTRAINT spelling binds the union namespace {check,
//     unique, pk, fk}: exactly one member drops (rc=0 verified per kind),
//     zero members refuse 3940, a cross-type same-name pair refuses 3939.
//   - TiDB flag-ON binds the CHECK member first under either spelling and
//     never detects the cross-type ambiguity — a non-check-only match is
//     unprobed there and stays unproven.
//   - Unproven spelling answers only where both spellings agree.
func CheckDropTarget(dialect Dialect, snapshot *TableSnapshot, spelling, name string) CheckTarget {
	t := CheckTarget{}
	if checkTargetNameSpaceUnvouched(snapshot, spelling) {
		t.Binding = CheckTargetUnproven
		return t
	}
	kinds, indexKindUnproven := genericConstraintKinds(snapshot, name)
	checkFound := kinds[CheckTargetKindCheck]
	delete(kinds, CheckTargetKindCheck)
	if dialect == DialectTiDB {
		switch {
		case checkFound:
			t.Binding = CheckTargetProven
			checkTargetMember(snapshot, name, CheckTargetKindCheck, &t)
		case len(kinds) > 0 || indexKindUnproven:
			t.Binding = CheckTargetUnproven
		default:
			t.Binding = CheckTargetMissing
		}
		return t
	}
	sole := func() CheckTargetKind {
		if checkFound {
			return CheckTargetKindCheck
		}
		for kind := range kinds {
			return kind
		}
		return ""
	}
	switch spelling {
	case CheckSpellingCheck:
		// The CHECK keyword never consults the index namespace — an
		// unproven index kind cannot move its answer.
		if checkFound {
			t.Binding = CheckTargetProven
			checkTargetMember(snapshot, name, CheckTargetKindCheck, &t)
		} else {
			t.Binding = CheckTargetMissing
		}
	case CheckSpellingConstraint:
		switch {
		case indexKindUnproven:
			// A hidden unique may bind here — undecidable.
			t.Binding = CheckTargetUnproven
		case checkFound && len(kinds) > 0:
			t.Binding = CheckTargetAmbiguous
		case checkFound || len(kinds) == 1:
			t.Binding = CheckTargetProven
			checkTargetMember(snapshot, name, sole(), &t)
		case len(kinds) > 1:
			t.Binding = CheckTargetAmbiguous
		default:
			t.Binding = CheckTargetMissing
		}
	default:
		// Unproven spelling: a lone CHECK member drops under either
		// keyword; a name neither spelling binds refuses under both.
		// Everything else diverges — undecidable.
		switch {
		case checkFound && len(kinds) == 0 && !indexKindUnproven:
			t.Binding = CheckTargetProven
			checkTargetMember(snapshot, name, CheckTargetKindCheck, &t)
		case !checkFound && len(kinds) == 0 && !indexKindUnproven:
			t.Binding = CheckTargetMissing
		default:
			t.Binding = CheckTargetUnproven
		}
	}
	return t
}

// CheckAlterTarget resolves what an ALTER CHECK|CONSTRAINT name provably
// binds. The operation is only executable on a CHECK member:
//   - CHECK spelling: member → proven; anything else → 3821 missing.
//   - Generic spelling: lone CHECK member → proven; CHECK-only absence
//     with other kinds bound → 3941 wrong-kind refusal; cross-type pair
//     → 3939 ambiguous refusal; nothing bound → 3940 missing.
//   - TiDB flag-ON binds CHECK first; non-check-only matches unprobed.
func CheckAlterTarget(dialect Dialect, snapshot *TableSnapshot, spelling, name string) CheckTarget {
	t := CheckTarget{}
	if checkTargetNameSpaceUnvouched(snapshot, spelling) {
		t.Binding = CheckTargetUnproven
		return t
	}
	kinds, indexKindUnproven := genericConstraintKinds(snapshot, name)
	checkFound := kinds[CheckTargetKindCheck]
	delete(kinds, CheckTargetKindCheck)
	if dialect == DialectTiDB {
		switch {
		case checkFound:
			t.Binding = CheckTargetProven
			checkTargetMember(snapshot, name, CheckTargetKindCheck, &t)
		case len(kinds) > 0 || indexKindUnproven:
			t.Binding = CheckTargetUnproven
		default:
			t.Binding = CheckTargetMissing
		}
		return t
	}
	switch spelling {
	case CheckSpellingCheck:
		if checkFound {
			t.Binding = CheckTargetProven
			checkTargetMember(snapshot, name, CheckTargetKindCheck, &t)
		} else {
			t.Binding = CheckTargetMissing
		}
	case CheckSpellingConstraint:
		switch {
		case indexKindUnproven:
			// A hidden unique may be the refused-or-bound target —
			// undecidable.
			t.Binding = CheckTargetUnproven
		case checkFound && len(kinds) > 0:
			t.Binding = CheckTargetAmbiguous
		case checkFound:
			t.Binding = CheckTargetProven
			checkTargetMember(snapshot, name, CheckTargetKindCheck, &t)
		case len(kinds) > 0:
			t.Binding = CheckTargetWrongKind
		default:
			t.Binding = CheckTargetMissing
		}
	default:
		if checkFound && len(kinds) == 0 && !indexKindUnproven {
			t.Binding = CheckTargetProven
			checkTargetMember(snapshot, name, CheckTargetKindCheck, &t)
		} else if !checkFound && len(kinds) == 0 && !indexKindUnproven {
			t.Binding = CheckTargetMissing
		} else {
			t.Binding = CheckTargetUnproven
		}
	}
	return t
}
