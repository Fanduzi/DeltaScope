// Package spec: T26-SRS-DROP-EXECUTION-01 — the audited MySQL DROP
// SPATIAL REFERENCE SYSTEM [IF EXISTS] <uint32 SRID> first slice
// (accepted contract docs/verification/t26-srs-drop-contract-01,
// R10 candidate d68f4ab7).
//
// One shared adjudication is the single truth the rule projection and
// the batch own-apply both read — the rule never recomputes premises
// and apply never re-derives an arm. The arm table is the contract's
// first-match order B → S0 → S1 → S2; owed facts follow the arm, never
// a fixed per-row rule. Findings, gaps, coverage, and the stale flags
// this slice mints are all projections of this one result. Internal
// facts, flags, and this structure never enter a serialized payload.
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
	// SRSFactDropPrivilege is always owed on the unproven arm — this
	// slice ships no privilege witness, so the premise stays
	// unproven on every qualified input that reaches it.
	SRSFactDropPrivilege = "srs.drop.privilege"
)

// SRSDropPrivilege is this statement's derived privilege premise
// projection. Only the unproven value is reachable in this slice — no
// privilege witness exists yet — but the field is carried so the
// adjudication shape stays the one the contract fixes.
type SRSDropPrivilege int

const (
	// SRSDropPrivilegeUnproven is the only reachable premise this
	// slice: no consumable SRS drop-privilege witness exists, so the
	// premise is owed wherever the arm leaves it.
	SRSDropPrivilegeUnproven SRSDropPrivilege = iota
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
	// Privilege is this statement's derived premise projection —
	// unproven only this slice.
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

// SRSDropInReservedRange reports whether the SRID falls inside a
// registered reserved range — the documented MySQL reserved blocks the
// drop would refuse natively. The reserved notice is an advisory that
// co-fires on the blocked and unproven arms; it never refires inside a
// refusal arm's own finding.
func SRSDropInReservedRange(srid uint32) bool {
	return srid <= 32767 ||
		(srid >= 60000000 && srid <= 69999999) ||
		(srid >= 2000000000 && srid <= 2147483647)
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
//     and no dependency staleness — refused, owes nothing.
//   - S2: everything else — owes column dependencies + privilege
//     always, plus existence first while unproven.
func AdjudicateSRSDrop(state *SpatialReferenceSystemState, srid uint32, depsStale bool) SRSDropAdjudication {
	adjudication := SRSDropAdjudication{Privilege: SRSDropPrivilegeUnproven}
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
	default:
		adjudication.Arm = SRSDropArmUnproven
		if state == nil || !state.Read || !state.Exists {
			adjudication.Owed = append(adjudication.Owed, SRSFactExistence)
		}
		adjudication.Owed = append(adjudication.Owed, SRSFactColumnDependencies, SRSFactDropPrivilege)
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
