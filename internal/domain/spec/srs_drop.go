// Package spec: T26-SRS-DROP-EXECUTION-01 — the audited MySQL DROP
// SPATIAL REFERENCE SYSTEM [IF EXISTS] <uint32 SRID> first slice
// (accepted contract docs/verification/t26-srs-drop-contract-01,
// R10 candidate d68f4ab7).
//
// One shared adjudication is the single truth the rule projection and
// the batch own-apply both read — the rule never recomputes premises
// and apply never re-derives an arm. The arm table is the contract's
// first-match order B → S0 → S1 → A → S2; owed facts follow the arm,
// never a fixed per-row rule. Findings, gaps, coverage, and the stale
// flags this slice mints are all projections of this one result.
// Internal facts, flags, and this structure never enter a serialized
// payload.
//
// T26-SRS-DROP-PROVEN-ABSENT-EXECUTION-01 adds the A family between S1
// and S2: a proven absence (the supplier's clean zero-observed-row
// answer under the registered completeness proof, bound to the
// authored SRID) refuses a plain DROP, no-ops an IF EXISTS drop under
// a consumable privilege premise, and stays conditional while that
// premise is unproven or scope-suppressed.
package spec

// Owed-fact vocabulary for the audited SRS drop — stable internal
// tokens the rule surface publishes verbatim in evidence gaps.
const (
	// SRSFactBatchEffects is owed on the blocked arm — an earlier batch
	// effect (whole-batch contamination or the same SRID's stale
	// existence flag) shadowed the catalog, so the read never issued.
	SRSFactBatchEffects = "srs.batch_effects"
	// SRSFactExistence is owed first on the unproven arm only while
	// the catalog existence answer stayed unproven.
	SRSFactExistence = "srs.existence"
	// SRSFactColumnDependencies is always owed on the unproven arm —
	// a consumable dependency witness would have refused the drop;
	// without one the dependency premise is undischarged.
	SRSFactColumnDependencies = "srs.column_dependencies"
	// SRSFactDropPrivilege is owed on the unproven arm while this
	// statement's own privilege premise is not consumable — an
	// unproven or scope-suppressed witness stays a debt; only a
	// consumable premise actually discharges it (T26-SRS-DROP-
	// PRIVILEGE-CONTRACT-01).
	SRSFactDropPrivilege = "srs.drop.privilege"
)

// SRSDropPrivilege is this statement's derived privilege premise
// projection — the SRS family's independent read of the shared
// request-level privilege observation (T26-SRS-DROP-PRIVILEGE-
// CONTRACT-01). The projection never borrows a SERVER statement's
// adjudication, marker, or outcome and never derives privilege
// evidence from the SRS catalog reads.
type SRSDropPrivilege int

const (
	// SRSDropPrivilegeUnproven means the premise is not consumable:
	// no positive witness was established for this request's shared
	// observation — the premise is owed wherever the arm leaves it.
	SRSDropPrivilegeUnproven SRSDropPrivilege = iota
	// SRSDropPrivilegeConsumable means the positive witness and the
	// still-open ordered scope let this statement actually discharge
	// its privilege premise — the unproven arm discharges it through
	// the S2-specific privilege notice and A-IE✓ discharges it
	// through the absent_noop marker; blocked/refused arms may carry
	// the projection without using it.
	SRSDropPrivilegeConsumable
	// SRSDropPrivilegeSuppressed means a positive witness was
	// established but the ordered scope ended before this statement —
	// the premise stays owed exactly like an unproven one, never a
	// denial. A scope end without an established witness keeps the
	// premise unproven, never suppressed.
	SRSDropPrivilegeSuppressed
)

// SRSDropArm is the adjudicated arm of one audited DROP SPATIAL
// REFERENCE SYSTEM — the single classification driving findings,
// gaps, coverage, verdict, and the own-apply stale flags.
type SRSDropArm int

const (
	// SRSDropArmBlocked (B): the bind was blocked before the
	// catalog/cache answer could be consumed — whole-batch
	// contamination or the same SRID's monotonic existence stale
	// flag. Owes exactly {srs.batch_effects}; the own apply preserves
	// the entry state exactly.
	SRSDropArmBlocked SRSDropArm = iota
	// SRSDropArmRefusedSRID0 (S0): the authored SRID is 0 — dropping
	// SRID 0 is never allowed. Owes nothing; one blocker finding.
	SRSDropArmRefusedSRID0
	// SRSDropArmRefusedDependency (S1): a valid positive dependency
	// witness (issued read, non-nil fact, non-empty columns, every
	// column SRID equal to the declared SRID, clean channel, no
	// staleness) proves a visible geometry column uses the SRID.
	// Owes nothing; one blocker finding.
	SRSDropArmRefusedDependency
	// SRSDropArmAbsentRefused (A-plain): a proven absence — ValidAbsence
	// holds — on a plain DROP without IF EXISTS. Refused, owes nothing,
	// never consumes the privilege premise; one blocker finding.
	SRSDropArmAbsentRefused
	// SRSDropArmAbsentNoop (A-IE✓): a proven absence on an IF EXISTS
	// drop while this statement's privilege premise is consumable —
	// the no-op drop discharges it. Owes nothing; one notice finding
	// carrying the conditional-audit marker.
	SRSDropArmAbsentNoop
	// SRSDropArmAbsentConditional (A-IE?): a proven absence on an
	// IF EXISTS drop while the privilege premise is unproven or
	// scope-suppressed — never consumed. Owes exactly
	// {srs.drop.privilege} under the unproven_privilege gap reason.
	SRSDropArmAbsentConditional
	// SRSDropArmUnproven (S2): every remaining qualified input —
	// owed facts follow the arm: srs.column_dependencies and
	// srs.drop.privilege always, plus srs.existence first only while
	// existence stayed unproven.
	SRSDropArmUnproven
)

// SRSDropAdjudication is the shared per-statement adjudication — the
// one truth the rule projection and the own apply both read.
type SRSDropAdjudication struct {
	// Arm is the adjudicated arm.
	Arm SRSDropArm
	// Privilege is this statement's own derived premise projection —
	// unproven, consumable, or scope-suppressed.
	Privilege SRSDropPrivilege
	// Owed lists the bounded gap facts this adjudication still owes —
	// derived from the arm's actual premises, never a fixed rule.
	Owed []string
}

// MySQLSRSDropAuditedDomain is the registered applicability domain for
// the audited SRS drop slice: a resolved, validated MySQL 8.0.4+
// version. The gate reads the resolved version — the presence or
// absence of a caller pin is never the criterion. Unknown or
// unparseable versions, proven pre-8.0.4, non-MySQL products, and
// unvalidated ranges all stay boundary — never inferred feature
// absence.
func MySQLSRSDropAuditedDomain(version *VersionIdentity) bool {
	return version != nil &&
		version.Product == VersionProductMySQL &&
		version.ValidatedRange &&
		version.Major == 8 && version.Minor == 0 && version.Patch >= 4
}

// SRSFactVersionApplicability is the named version-dependent
// applicability premise (T26-SRS-VERSION-APPLICABILITY-01 frozen V4):
// when the resolved version cannot certify the audited domain, the gap
// is reported under this fact id with one of the four version reasons —
// never folded into a generic unknown.
const SRSFactVersionApplicability = "srs.version_applicability"

// MySQLSRSVersionGateReason is the SRS family's single version verdict
// (frozen V1): one request-level version fact decides BOTH audit
// admission (the unaudited boundary marker) and catalog-read admission
// (whether the provider's SRS queries may fire at all). It returns ""
// only inside the registered 8.0.4+ domain; otherwise the stable reason
// token the public marker and the bound state both carry:
//
//   - nil version, no observation ever ran → version_unproven
//   - nil version, an observation answered without a usable banner fact
//     → version_observation_error
//   - identity outside the validated series → version_out_of_series
//   - proven in-series but outside the audited domain (5.7, 8.0.0–8.0.3,
//     8.4) → version_inapplicable
//
// `observed` reports whether a provider observation was performed this
// request — it is what separates "never observed" from "observation
// failed" (frozen V2/V10).
func MySQLSRSVersionGateReason(version *VersionIdentity, observed bool) string {
	if MySQLSRSDropAuditedDomain(version) {
		return ""
	}
	switch {
	case version == nil && !observed:
		return MetadataUnreadVersionUnproven
	case version == nil:
		return MetadataUnreadVersionObservationError
	case !version.ValidatedRange:
		return MetadataUnreadVersionOutOfSeries
	default:
		return MetadataUnreadVersionInapplicable
	}
}

// SRSDropInReservedRange reports whether the SRID falls inside a
// registered reserved range — the documented MySQL reserved blocks.
// The reserved notice is an advisory that
// co-fires on the blocked and unproven arms; it never refires inside a
// refusal arm's own finding.
func SRSDropInReservedRange(srid uint32) bool {
	return srid <= 32767 ||
		(srid >= 60000000 && srid <= 69999999) ||
		(srid >= 2000000000 && srid <= 2147483647)
}

// ValidAbsence is the consumer-side proven-absence predicate
// (T26-SRS-DROP-PROVEN-ABSENT-EXECUTION-01): the bound state carries a
// supplier-certified absence ONLY for the statement's own authored
// SRID — a foreign-SRID snapshot is never consumed as this object's
// absence. Read && !Exists alone stays unknown: the zero value of
// Absence is the "no absence proof" answer.
func ValidAbsence(state *SpatialReferenceSystemState, authoredSRID uint32) bool {
	return state != nil && state.SRID == authoredSRID &&
		state.Read && !state.Exists && state.Absence
}

// OrderedAbsentConsumed is the ordered channel's single cross-layer
// predicate (T26-SRS-DROP-ORDERED-PUBLICATION-EXECUTION-01 contract
// §4.5): the bind writes the marker only after certifying an active
// same-SRID entry (absent && !barrier) — the barrier question is
// decided at bind, so the adjudication consumes marker + authored-SRID
// equality verbatim with no second check. `state.SRID` is always
// assigned the declaration's authored SRID at bind; the channel's map
// key never substitutes for this field-level contract.
func OrderedAbsentConsumed(state *SpatialReferenceSystemState, authoredSRID uint32) bool {
	return state != nil && state.OrderedAbsent && state.SRID == authoredSRID
}

// AdjudicateSRSDrop computes the shared adjudication for one audited
// qualified MySQL DROP SPATIAL REFERENCE SYSTEM. The caller guarantees
// the audited slice — MySQL dialect, Kind == drop, no unsupported
// marker, resolved version inside the audited domain — so the arm
// table only walks the contract's first-match order:
//
//   - B: the bound state's unread marker carries the batch-effects
//     reason — set by the bind for whole-batch contamination or the
//     same SRID's existence stale flag. Owes {srs.batch_effects}.
//   - S0: authored SRID 0 — refused outright, owes nothing.
//   - S1: a valid positive dependency witness under a clean channel
//     and no dependency staleness — refused, owes nothing. The
//     positive witness outranks a simultaneously proven absence: the
//     SRID's visible dependents exist regardless.
//   - A: ValidAbsence holds for the authored SRID — a catalog-certified
//     absence — or OrderedAbsentConsumed holds: the batch's ordered
//     channel supplied a proven absence fact an earlier statement
//     certified (T26-SRS-DROP-ORDERED-PUBLICATION-EXECUTION-01). A
//     plain drop is refused_absent (owes nothing, never consumes the
//     premise); an IF EXISTS drop is absent_noop while the premise is
//     consumable (discharges it), else absent_conditional owing exactly
//     {srs.drop.privilege} — never consumed.
//   - S2: everything else — owes column dependencies always,
//     privilege while this statement's own premise is not consumable,
//     plus existence first while unproven. A consumable premise
//     reaching this arm is discharged here (the S2-specific privilege
//     notice carries that consumption); A-IE✓ discharges the same
//     premise through its own absent_noop marker. Every remaining
//     arm carries (or lacks) the projection without consuming it.
func AdjudicateSRSDrop(state *SpatialReferenceSystemState, srid uint32, depsStale bool, privilege SRSDropPrivilege, ifExists bool) SRSDropAdjudication {
	adjudication := SRSDropAdjudication{Privilege: privilege}
	switch {
	case state != nil && state.UnreadReason == MetadataUnreadBatchEffects:
		// The bind blocked before the catalog/cache answer could be
		// consumed — contamination or the same SRID's stale flag both
		// carry the batch-effects unread vocabulary.
		adjudication.Arm = SRSDropArmBlocked
		adjudication.Owed = []string{SRSFactBatchEffects}
		return adjudication
	case srid == 0:
		adjudication.Arm = SRSDropArmRefusedSRID0
		return adjudication
	case !depsStale && srsDropDependencyWitness(state, srid):
		// A stale dependency flag demotes the witness to non-
		// consumable; a still-valid existence fact is untouched and
		// stays consumed in the unproven arm's owed set.
		adjudication.Arm = SRSDropArmRefusedDependency
		return adjudication
	case ValidAbsence(state, srid) || OrderedAbsentConsumed(state, srid):
		switch {
		case !ifExists:
			adjudication.Arm = SRSDropArmAbsentRefused
		case privilege == SRSDropPrivilegeConsumable:
			adjudication.Arm = SRSDropArmAbsentNoop
		default:
			adjudication.Arm = SRSDropArmAbsentConditional
			adjudication.Owed = []string{SRSFactDropPrivilege}
		}
		return adjudication
	default:
		adjudication.Arm = SRSDropArmUnproven
		if state == nil || !state.Read || !state.Exists {
			adjudication.Owed = append(adjudication.Owed, SRSFactExistence)
		}
		adjudication.Owed = append(adjudication.Owed, SRSFactColumnDependencies)
		if privilege != SRSDropPrivilegeConsumable {
			adjudication.Owed = append(adjudication.Owed, SRSFactDropPrivilege)
		}
		return adjudication
	}
}

// srsDropDependencyWitness reports whether the bound state carries a
// valid positive dependency witness for this SRID: the read actually
// issued (non-nil fact), the column list is non-empty, and every
// visible dependent column declares the statement's own SRID. A
// clean zero-row fact is unproven, never a refusal and never a
// proven-absent answer for the unproven arm.
func srsDropDependencyWitness(state *SpatialReferenceSystemState, srid uint32) bool {
	if state == nil || state.Dependencies == nil || len(state.Dependencies.Columns) == 0 {
		return false
	}
	for _, column := range state.Dependencies.Columns {
		if column.SRID != srid {
			return false
		}
	}
	return true
}
