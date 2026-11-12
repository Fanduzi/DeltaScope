// Package spec: T26-SRS-CREATE-PUBLICATION-01 — the audited MySQL
// CREATE [OR REPLACE] SPATIAL REFERENCE SYSTEM slice over the
// reviewer-frozen contract (Phase 0 evidence candidate accepted; the
// frozen clause set F-1..F-11 and U1..U7 rulings live in
// docs/verification/t26-srs-create-publication-01/README.md).
//
// One shared adjudication is the single truth the rule projection and
// the batch own-apply both read — the rule never recomputes premises
// and apply never re-derives an arm. The first-match arm order mirrors
// the observed native refusal order (Phase 0 native probes on
// mysql:8.0 / 8.0.46): batch-effects block, SRID-0 refusal, duplicate
// authored attribute (3709), missing mandatory attribute (3708),
// existence layer (3712 / 3713-warning), NAME unique-key conflict
// (1062), ORGANIZATION+ID unique-key conflict (1062), then the
// outcome arms. Findings, gaps, coverage, and the stale/present flags
// this slice mints are all projections of this one result. Internal
// facts, flags, and this structure never enter a serialized payload.
//
// Conservative-by-design (F-6): a conflict read that returns empty is
// "read, zero rows, unknown" — never a proven "no conflict". Every
// audited create whose authored NAME (or ORGANIZATION pair) carries a
// provable identity owes that conflict premise wherever no positive
// witness discharged it; the statement stays coverage-incomplete
// rather than silently passing.
package spec

// Owed-fact vocabulary for the audited SRS create — the existence and
// batch-effects tokens reuse the family's shared vocabulary; the
// privilege token is CREATE's own projection (never
// srs.drop.privilege — the frozen contract forbids borrowing the drop
// premise's semantic identity); the two conflict tokens are new.
const (
	// SRSFactCreatePrivilege is the SUPER execution premise for
	// CREATE [OR REPLACE] SPATIAL REFERENCE SYSTEM — owed on every
	// non-terminal arm whose own premise is not consumable. A
	// satisfied srs.drop.privilege is never borrowed: each statement
	// projects its own premise.
	SRSFactCreatePrivilege = "srs.create.privilege"
	// SRSFactNameConflict is owed while the authored NAME's unique-key
	// conflict premise stayed unresolved — a positive witness on
	// another SRID would have refused; an empty or unread answer is
	// unknown, never a proven "no conflict".
	SRSFactNameConflict = "srs.name_conflict"
	// SRSFactOrganizationConflict is owed while the authored
	// ORGANIZATION + IDENTIFIED BY pair's unique-key conflict premise
	// stayed unresolved — same witness contract as the NAME read.
	SRSFactOrganizationConflict = "srs.organization_conflict"
)

// SRSCreatePrivilege is one CREATE statement's derived privilege
// premise projection — a per-statement verdict on the shared
// request-level SUPER observation, never a borrowed drop premise.
type SRSCreatePrivilege int

const (
	// SRSCreatePrivilegeUnproven means the premise is not consumable:
	// no positive witness was established for this request's shared
	// observation — the premise is owed wherever the arm leaves it.
	SRSCreatePrivilegeUnproven SRSCreatePrivilege = iota
	// SRSCreatePrivilegeConsumable means the positive witness and the
	// still-open ordered scope let this statement actually discharge
	// its privilege premise.
	SRSCreatePrivilegeConsumable
	// SRSCreatePrivilegeSuppressed means a positive witness was
	// established but the ordered scope ended before this statement —
	// the premise stays owed exactly like an unproven one, never a
	// denial.
	SRSCreatePrivilegeSuppressed
)

// SRSCreateArm is the adjudicated arm of one audited CREATE
// SPATIAL REFERENCE SYSTEM — the single classification driving
// findings, gaps, coverage, verdict, and the own-apply mints.
type SRSCreateArm int

const (
	// SRSCreateArmBlocked (B): the bind was blocked before the
	// catalog/cache answer could be consumed — whole-batch
	// contamination or the same SRID's monotonic existence stale
	// flag. Owes exactly {srs.batch_effects}; the own apply writes
	// nothing.
	SRSCreateArmBlocked SRSCreateArm = iota
	// SRSCreateArmRefusedSRID0 (S0): the authored SRID is 0 — SRID 0
	// is not modifiable (native 3714). Owes nothing; one blocker.
	SRSCreateArmRefusedSRID0
	// SRSCreateArmRefusedDuplicateAttribute: an authored attribute
	// appears twice (native 3709 — fires before the mandatory-
	// attribute check, so it outranks A2). Owes nothing; one blocker.
	SRSCreateArmRefusedDuplicateAttribute
	// SRSCreateArmRefusedMissingAttribute: NAME or DEFINITION absent
	// from the authored attributes (native 3708 — fires before the
	// existence check). Owes nothing; one blocker.
	SRSCreateArmRefusedMissingAttribute
	// SRSCreateArmRefusedExistence: a plain CREATE (no IF NOT EXISTS)
	// against a proven-present SRID (native 3712). Owes nothing; one
	// blocker.
	SRSCreateArmRefusedExistence
	// SRSCreateArmExistsNoop: a CREATE ... IF NOT EXISTS against a
	// proven-present SRID under a consumable privilege premise
	// (native warning 3713) — the statement is a no-op, never a
	// conflict finding (frozen U2; T26-SRS-CREATE-GUARD-CORRECTION-01
	// C1: the only arm allowed to publish exists_noop). Owes nothing;
	// carries the conditional_audit marker.
	SRSCreateArmExistsNoop
	// SRSCreateArmOutcomeUnproven (T26-SRS-CREATE-GUARD-CORRECTION-01
	// C2): a CREATE ... IF NOT EXISTS against a proven-present SRID
	// whose privilege premise is unproven or suppressed. The catalog
	// fact — object present — stands, but the native order checks
	// privilege BEFORE the existence guard, so the guarded no-op arm
	// is unproven: the statement outcome is unproven, never a claimed
	// Warning-3713 no-op and never a denial. Owes exactly
	// {srs.create.privilege}; the own apply writes nothing — no new
	// success publication and no overwrite of the proven-present
	// state.
	SRSCreateArmOutcomeUnproven
	// SRSCreateArmRefusedNameConflict: a provable authored NAME whose
	// conflict read returned a witness SRID other than this
	// statement's own (native 1062 on SRS_NAME). Owes nothing; one
	// blocker.
	SRSCreateArmRefusedNameConflict
	// SRSCreateArmRefusedOrgConflict: a provable authored
	// ORGANIZATION + IDENTIFIED BY pair whose conflict read returned
	// a witness SRID other than this statement's own (native 1062 on
	// ORGANIZATION_AND_ID). Owes nothing; one blocker.
	SRSCreateArmRefusedOrgConflict
	// SRSCreateArmCreate: a catalog-certified absence (ValidAbsence)
	// on the authored SRID — plain, IF NOT EXISTS, and OR REPLACE all
	// take create-result semantics here (frozen F-4: OR REPLACE on an
	// absent target is a create). Owed carries every unresolved
	// conflict premise and the unproven privilege premise; the own
	// apply mints present only when the owed set is empty, else the
	// existence stale flag.
	SRSCreateArmCreate
	// SRSCreateArmReplace: OR REPLACE against a proven-present SRID —
	// replacement-result semantics. The object exists before the
	// statement and exists after whatever the statement does, so the
	// own apply mints present unconditionally; the owed set still
	// carries unresolved conflict premises and the unproven privilege
	// premise.
	SRSCreateArmReplace
	// SRSCreateArmUnproven: every remaining qualified input —
	// existence stayed unproven and no positive witness refused. Owed
	// follows the arm's actual premises: srs.existence while
	// unproven, each unresolved authored conflict premise, and
	// srs.create.privilege while not consumable.
	SRSCreateArmUnproven
)

// SRSCreateAdjudication is the shared per-statement adjudication —
// the one truth the rule projection and the own apply both read.
type SRSCreateAdjudication struct {
	// Arm is the adjudicated arm.
	Arm SRSCreateArm
	// Privilege is this statement's own derived premise projection —
	// unproven, consumable, or scope-suppressed.
	Privilege SRSCreatePrivilege
	// Owed lists the bounded gap facts this adjudication still owes —
	// derived from the arm's actual premises, never a fixed rule.
	Owed []string
	// MissingAttribute names the mandatory attribute whose absence
	// refused the statement — "NAME" or "DEFINITION"; only set on the
	// refused_missing_attribute arm.
	MissingAttribute string
	// DuplicateAttribute names the authored attribute defined twice —
	// only set on the refused_duplicate_attribute arm.
	DuplicateAttribute string
	// ConditionalAudit marks that the conditional privilege scope
	// actually fed this adjudication's conclusion — the statement's
	// own consumable premise reached an arm that discharged it. It is
	// the one fact finding metadata projects as conditional_audit.
	ConditionalAudit bool
}

// AdjudicateSRSCreate computes the shared adjudication for one
// audited qualified MySQL CREATE [OR REPLACE] SPATIAL REFERENCE
// SYSTEM. The caller guarantees the audited slice — MySQL dialect, a
// create kind, no unsupported marker, resolved version inside the
// audited domain — so the arm table walks the contract's first-match
// order:
//
//   - B: blocked bind (UnreadReason = batch_effects) — owes
//     {srs.batch_effects}.
//   - S0: authored SRID 0 — refused.
//   - A1: an authored attribute defined twice — refused (native 3709
//     fires before the mandatory check).
//   - A2: NAME or DEFINITION absent — refused (native 3708 fires
//     before the existence check).
//   - E: proven presence — from the existence read or an own-SRID
//     conflict witness (a matched own SRID proves the row exists). A
//     plain create refuses (3712); IF NOT EXISTS takes the no-op
//     marker arm (3713 — never a blocker, frozen U2) only under a
//     consumable privilege premise, else the outcome_unproven arm
//     (the privilege gate precedes the existence guard natively —
//     T26-SRS-CREATE-GUARD-CORRECTION-01 C2); OR REPLACE continues
//     to the conflict layer.
//   - C1/C2: a provable authored NAME or ORGANIZATION+ID pair whose
//     conflict read returned a witness on another SRID — refused
//     (native 1062). A match on the statement's own SRID is never a
//     conflict: for OR REPLACE it is the pair the row already owns
//     (native probe C4); for every kind it is existence evidence that
//     already fed the E layer.
//   - N1/R1/U: a certified absence takes create-result semantics; OR
//     REPLACE on proven presence takes replacement-result semantics;
//     everything else is unproven — owed follows the arm's actual
//     premises (existence, each unresolved authored conflict premise,
//     privilege while not consumable).
func AdjudicateSRSCreate(state *SpatialReferenceSystemState, decl *SpatialReferenceDeclaration, privilege SRSCreatePrivilege) SRSCreateAdjudication {
	adjudication := SRSCreateAdjudication{Privilege: privilege}
	consumable := privilege == SRSCreatePrivilegeConsumable
	orReplace := decl.Kind == SRSKindCreateOrReplace

	if state != nil && state.UnreadReason == MetadataUnreadBatchEffects {
		adjudication.Arm = SRSCreateArmBlocked
		adjudication.Owed = []string{SRSFactBatchEffects}
		return adjudication
	}
	if decl.SRID == 0 {
		adjudication.Arm = SRSCreateArmRefusedSRID0
		return adjudication
	}
	if len(decl.DuplicateAttrs) > 0 {
		adjudication.Arm = SRSCreateArmRefusedDuplicateAttribute
		adjudication.DuplicateAttribute = decl.DuplicateAttrs[0]
		return adjudication
	}
	if !decl.Attrs.HasName {
		adjudication.Arm = SRSCreateArmRefusedMissingAttribute
		adjudication.MissingAttribute = "NAME"
		return adjudication
	}
	if !decl.Attrs.HasDefinition {
		adjudication.Arm = SRSCreateArmRefusedMissingAttribute
		adjudication.MissingAttribute = "DEFINITION"
		return adjudication
	}

	// Conflict witnesses: a match on another SRID is a refusal; a
	// match on this statement's own SRID is existence evidence — the
	// row exists — which feeds the presence layer below and, for OR
	// REPLACE, discharges that conflict premise (the pair is the one
	// the object already owns).
	nameWitness, nameOwn := srsConflictWitness(state, true, decl.SRID)
	orgWitness, orgOwn := srsConflictWitness(state, false, decl.SRID)

	present := (state != nil && state.Read && state.Exists) || nameOwn || orgOwn
	switch {
	case present && !orReplace && !decl.IfNotExists:
		adjudication.Arm = SRSCreateArmRefusedExistence
		return adjudication
	case present && !orReplace:
		// G10/C1-C2 (T26-SRS-CREATE-GUARD-CORRECTION-01 over the
		// frozen oracle): the privilege gate precedes the existence
		// guard natively, so only a consumable premise proves the
		// guarded no-op. An unproven or suppressed premise leaves the
		// outcome unproven — the catalog-present fact stands but the
		// statement never claims the Warning-3713 no-op.
		if consumable {
			adjudication.Arm = SRSCreateArmExistsNoop
			adjudication.ConditionalAudit = true
		} else {
			adjudication.Arm = SRSCreateArmOutcomeUnproven
			adjudication.Owed = []string{SRSFactCreatePrivilege}
		}
		return adjudication
	}

	// The conflict layer fires on every remaining path — OR REPLACE on
	// presence, certified absence, and unknown alike: a positive
	// witness is usable evidence wherever the existence answer landed.
	if decl.Attrs.HasName && nameWitness {
		adjudication.Arm = SRSCreateArmRefusedNameConflict
		return adjudication
	}
	if decl.Attrs.HasOrganization && decl.Attrs.HasIdentifiedBy && orgWitness {
		adjudication.Arm = SRSCreateArmRefusedOrgConflict
		return adjudication
	}

	switch {
	case orReplace && present:
		// A proven presence under OR REPLACE takes replacement-result
		// semantics — the own-SRID conflict witness outranks a
		// simultaneously empty existence answer: the row is there.
		adjudication.Arm = SRSCreateArmReplace
	case ValidAbsence(state, decl.SRID):
		adjudication.Arm = SRSCreateArmCreate
	default:
		adjudication.Arm = SRSCreateArmUnproven
		if !srsCreateExistenceKnown(state) {
			adjudication.Owed = append(adjudication.Owed, SRSFactExistence)
		}
	}
	// Unresolved authored conflict premises ride the outcome arm's
	// owed set — an empty or unread answer is unknown, never a proven
	// "no conflict" (frozen F-6).
	if decl.Attrs.HasName && !nameWitness && !nameOwn {
		adjudication.Owed = append(adjudication.Owed, SRSFactNameConflict)
	}
	if decl.Attrs.HasOrganization && decl.Attrs.HasIdentifiedBy && !orgWitness && !orgOwn {
		adjudication.Owed = append(adjudication.Owed, SRSFactOrganizationConflict)
	}
	if !consumable {
		adjudication.Owed = append(adjudication.Owed, SRSFactCreatePrivilege)
	} else {
		adjudication.ConditionalAudit = true
	}
	return adjudication
}

// srsConflictWitness splits one conflict read's matched SRIDs into
// the other-SRID witness (a refusal) and the own-SRID witness
// (existence evidence / OR REPLACE exemption). A nil fact answers
// false/false — the premise stays unresolved.
func srsConflictWitness(state *SpatialReferenceSystemState, name bool, srid uint32) (witness bool, own bool) {
	if state == nil {
		return false, false
	}
	var fact *SRSConflictFact
	if name {
		fact = state.NameConflict
	} else {
		fact = state.OrgConflict
	}
	if fact == nil {
		return false, false
	}
	for _, matched := range fact.MatchedSRIDs {
		if matched == srid {
			own = true
		} else {
			witness = true
		}
	}
	return witness, own
}

// srsCreateExistenceKnown reports whether the bound state's existence
// answer is proven either way — read-and-present or a certified
// absence. Read-and-not-exists without the absence mark stays
// unproven.
func srsCreateExistenceKnown(state *SpatialReferenceSystemState) bool {
	if state == nil || !state.Read {
		return false
	}
	return state.Exists || state.Absence
}
