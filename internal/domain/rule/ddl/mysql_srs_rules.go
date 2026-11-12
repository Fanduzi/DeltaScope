// Package ddl: the audited MySQL DROP SPATIAL REFERENCE SYSTEM slice —
// T26-SRS-DROP-EXECUTION-01 over the accepted contract
// docs/verification/t26-srs-drop-contract-01 (R10 candidate d68f4ab7).
// Three rules read one truth: spec.AdjudicateSRSDrop's arm plus its
// derived owed-fact set, never a recomputation.
//
//   - ddl.mysql.srs.drop.srid0 fires the fixed SRID-0 blocker on the
//     refused_srid0 arm.
//   - ddl.mysql.srs.drop.dependency fires the dependency blocker on
//     refused_dependency and carries the evidence gap on every arm
//     whose premises stayed owed (blocked, unproven) under the fixed
//     unknown_srs_state reason — and the absent_conditional arm's
//     exactly-[srs.drop.privilege] debt under unproven_privilege
//     (T26-SRS-DROP-PROVEN-ABSENT-EXECUTION-01).
//   - ddl.mysql.srs.drop.reserved is an advisory that co-fires on the
//     blocked, unproven, absent_noop, and absent_conditional arms when
//     the SRID sits in a registered reserved range — never inside a
//     refusal arm's own finding.
//   - ddl.mysql.srs.drop.privilege is the family's conditional notice —
//     it fires only when the shared adjudication says this statement
//     actually consumed its own privilege premise (unproven arm plus a
//     consumable premise) and carries exactly {srid, conditional_audit}
//     (T26-SRS-DROP-PRIVILEGE-EXECUTION-01).
//   - ddl.mysql.srs.drop.absent fires the refused_absent blocker on the
//     proven-absence plain drop (T26-SRS-DROP-PROVEN-ABSENT-
//     EXECUTION-01).
//   - ddl.mysql.srs.drop.absent_noop carries the absent_noop notice —
//     including the conditional_audit marker — on the IF EXISTS
//     proven-absence arm with a consumable premise; the same marker is
//     a finding metadata key, not an arm outcome of its own.
//
// Public surfaces stay bounded: finding metadata carries the numeric
// SRID, the arm's outcome token, and the dependency witness count —
// never an authored name, attribute payload, or schema/table/column
// identity; the reserved notice carries only {srid}.
package ddl

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// srsDropStatement reports whether the statement is an audited MySQL
// DROP SPATIAL REFERENCE SYSTEM — dialect, family, operation, and no
// unsupported marker.
func srsDropStatement(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectMySQL &&
		statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.SpatialReference != nil &&
		statement.DDL.SpatialReference.Kind == spec.SRSKindDrop &&
		statement.DDL.Operation == spec.DDLOperationDropSpatialReferenceSystem &&
		statement.Unsupported == nil
}

// srsDropAdjudicationFor returns the shared adjudication the
// enrichment attached — nil outside the audited slice (a legacy
// statement shape or a bypassed enrichment), where the rules fall
// back to the pre-execution contract: every premise owed.
func srsDropAdjudicationFor(statement spec.Statement) *spec.SRSDropAdjudication {
	if statement.Metadata == nil {
		return nil
	}
	return statement.Metadata.SRSDrop
}

// srsDropSRID returns the authored numeric identity — the provable
// catalog key for this statement.
func srsDropSRID(statement spec.Statement) uint32 {
	return statement.DDL.SpatialReference.SRID
}

// srsDropOwed returns the adjudication's owed set — the one truth the
// dependency rule publishes verbatim. Outside the audited slice the
// defensive fallback owes every premise.
func srsDropOwed(statement spec.Statement) []string {
	if adjudication := srsDropAdjudicationFor(statement); adjudication != nil {
		return adjudication.Owed
	}
	return []string{spec.SRSFactExistence, spec.SRSFactColumnDependencies, spec.SRSFactDropPrivilege}
}

// ddl.mysql.srs.drop.srid0 — the fixed SRID-0 refusal blocker.
type mysqlSRSDropSRID0Rule struct {
	id    string
	level rule.Level
}

func newMySQLSRSDropSRID0Rule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return mysqlSRSDropSRID0Rule{
		id:    ruleIDMySQLSRSDropSRID0,
		level: configuredLevel(cfg, rule.LevelBlocker),
	}, nil
}

func (r mysqlSRSDropSRID0Rule) ID() string { return r.id }

func (r mysqlSRSDropSRID0Rule) AppliesTo(statement spec.Statement) bool {
	return srsDropStatement(statement)
}

func (r mysqlSRSDropSRID0Rule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	adjudication := srsDropAdjudicationFor(statement)
	if adjudication == nil || adjudication.Arm != spec.SRSDropArmRefusedSRID0 {
		return nil, nil
	}
	return []rule.Finding{{
		Level:   r.level,
		Message: "dropping spatial reference system SRID 0 is not allowed",
		Explanation: &rule.FindingExplanation{
			Why:        "MySQL reserves SRID 0 and refuses to drop it regardless of catalog state.",
			Risk:       "The statement cannot land — the refusal is fixed by the authored numeric identity alone.",
			Suggestion: "Remove the statement or target a non-zero SRID.",
		},
		Metadata: map[string]any{
			"srid":    0,
			"outcome": "refused_srid0",
		},
	}}, nil
}

// ddl.mysql.srs.drop.dependency — the dependency refusal blocker plus
// the family's evidence-gap carrier.
type mysqlSRSDropDependencyRule struct {
	id    string
	level rule.Level
}

func newMySQLSRSDropDependencyRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return mysqlSRSDropDependencyRule{
		id:    ruleIDMySQLSRSDropDependency,
		level: configuredLevel(cfg, rule.LevelBlocker),
	}, nil
}

func (r mysqlSRSDropDependencyRule) ID() string { return r.id }

func (r mysqlSRSDropDependencyRule) AppliesTo(statement spec.Statement) bool {
	return srsDropStatement(statement)
}

// EvidenceGaps carries the adjudication's owed facts on every arm whose
// premises stayed unproven — the blocked and unproven arms — under the
// fixed unknown_srs_state reason. The absent_conditional arm is the one
// registered exception (T26-SRS-DROP-PROVEN-ABSENT-EXECUTION-01): the
// object's absence IS proven, so the remaining debt is exactly this
// statement's own privilege premise under unproven_privilege — never
// the catalog-state reason.
func (r mysqlSRSDropDependencyRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	owed := srsDropOwed(statement)
	if len(owed) == 0 {
		return nil
	}
	reason := "unknown_srs_state"
	if adjudication := srsDropAdjudicationFor(statement); adjudication != nil &&
		adjudication.Arm == spec.SRSDropArmAbsentConditional {
		reason = "unproven_privilege"
	}
	return []rule.EvidenceGap{{
		ReasonCode:    reason,
		RequiredFacts: owed,
	}}
}

func (r mysqlSRSDropDependencyRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	adjudication := srsDropAdjudicationFor(statement)
	if adjudication == nil || adjudication.Arm != spec.SRSDropArmRefusedDependency {
		return nil, nil
	}
	srid := srsDropSRID(statement)
	witnesses := 0
	if statement.Metadata != nil && statement.Metadata.SpatialReference != nil &&
		statement.Metadata.SpatialReference.Dependencies != nil {
		witnesses = len(statement.Metadata.SpatialReference.Dependencies.Columns)
	}
	return []rule.Finding{{
		Level:   r.level,
		Message: fmt.Sprintf("spatial reference system %d is used by %d visible geometry column(s)", srid, witnesses),
		Explanation: &rule.FindingExplanation{
			Why:        "MySQL refuses to drop a spatial reference system while a geometry column declared on it exists; the catalog witness read returned visible dependent columns for this SRID.",
			Risk:       "The statement cannot land while any visible dependent stands — the witness count is the privilege-filtered visible set, never a proven instance-wide total.",
			Suggestion: "Drop or redeclare the dependent geometry columns before dropping this spatial reference system.",
		},
		Metadata: map[string]any{
			"srid":                     srid,
			"outcome":                  "refused_dependency",
			"dependency_witness_count": witnesses,
		},
	}}, nil
}

// ddl.mysql.srs.drop.reserved — the reserved-range advisory. It
// co-fires on the blocked, unproven, absent_noop, and
// absent_conditional arms and carries exactly {srid} — never arm,
// outcome, or gap data. The absent-refusal arm is a refusal: its own
// blocker finding already settles the outcome, so no advisory
// co-fires (T26-SRS-DROP-PROVEN-ABSENT-EXECUTION-01).
type mysqlSRSDropReservedRule struct {
	id    string
	level rule.Level
}

func newMySQLSRSDropReservedRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return mysqlSRSDropReservedRule{
		id:    ruleIDMySQLSRSDropReserved,
		level: configuredLevel(cfg, rule.LevelNotice),
	}, nil
}

func (r mysqlSRSDropReservedRule) ID() string { return r.id }

func (r mysqlSRSDropReservedRule) AppliesTo(statement spec.Statement) bool {
	return srsDropStatement(statement)
}

func (r mysqlSRSDropReservedRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	adjudication := srsDropAdjudicationFor(statement)
	if adjudication == nil ||
		(adjudication.Arm != spec.SRSDropArmBlocked && adjudication.Arm != spec.SRSDropArmUnproven &&
			adjudication.Arm != spec.SRSDropArmAbsentNoop && adjudication.Arm != spec.SRSDropArmAbsentConditional) {
		return nil, nil
	}
	srid := srsDropSRID(statement)
	if !spec.SRSDropInReservedRange(srid) {
		return nil, nil
	}
	return []rule.Finding{{
		Level:   r.level,
		Message: fmt.Sprintf("spatial reference system %d is in a reserved SRID range", srid),
		Explanation: &rule.FindingExplanation{
			Why:        "The authored SRID sits inside a documented MySQL reserved range; the advisory reports that membership only.",
			Risk:       "The advisory carries no claim about whether a native drop would refuse, succeed, or warn — the audit outcome on this arm is already settled by the shared adjudication.",
			Suggestion: "Review whether the reserved SRID was intended; pick a non-reserved identifier for new spatial reference systems.",
		},
		Metadata: map[string]any{
			"srid": srid,
		},
	}}, nil
}

// ddl.mysql.srs.drop.privilege — the family's conditional privilege
// notice. It fires exactly when the shared adjudication reports the
// statement consumed its own privilege premise: audited drop,
// adjudication attached, unproven arm, consumable premise. Metadata
// stays bounded to {srid, conditional_audit} — never an outcome token,
// account, grant text, session detail, or observation payload
// (T26-SRS-DROP-PRIVILEGE-EXECUTION-01).
type mysqlSRSDropPrivilegeRule struct {
	id    string
	level rule.Level
}

func newMySQLSRSDropPrivilegeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return mysqlSRSDropPrivilegeRule{
		id:    ruleIDMySQLSRSDropPrivilege,
		level: configuredLevel(cfg, rule.LevelNotice),
	}, nil
}

func (r mysqlSRSDropPrivilegeRule) ID() string { return r.id }

func (r mysqlSRSDropPrivilegeRule) AppliesTo(statement spec.Statement) bool {
	return srsDropStatement(statement)
}

func (r mysqlSRSDropPrivilegeRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	adjudication := srsDropAdjudicationFor(statement)
	if adjudication == nil ||
		adjudication.Arm != spec.SRSDropArmUnproven ||
		adjudication.Privilege != spec.SRSDropPrivilegeConsumable {
		return nil, nil
	}
	srid := srsDropSRID(statement)
	return []rule.Finding{{
		Level:   r.level,
		Message: fmt.Sprintf("spatial reference system %d drop privilege premise discharged under the observed_account_new_session conditional audit", srid),
		Explanation: &rule.FindingExplanation{
			Why:        "The shared request-level privilege observation bound a positive direct-global SUPER-or-equivalent witness for the observed account, and the ordered scope was still open at this statement — this statement's own privilege premise is discharged.",
			Risk:       "The witness binds the observed account under a conditional audit: execution is assumed on a new session established after the observation on the same target, with the relied-upon grant still applying when that session loads privileges — never a verified future executor or a pooled connection's live privileges.",
			Suggestion: "Keep the observed account's direct global SUPER-or-equivalent grant intact for the session that executes the drop.",
		},
		Metadata: map[string]any{
			"srid":              srid,
			"conditional_audit": string(spec.PrivilegeAuditModeObservedAccountNewSession),
		},
	}}, nil
}

// ddl.mysql.srs.drop.absent — the proven-absence refusal blocker
// (T26-SRS-DROP-PROVEN-ABSENT-EXECUTION-01). Fires only on the shared
// adjudication's absent_refused arm: a plain DROP against a catalog
// answer the registered completeness proof certified as absent.
// Metadata stays {srid:uint32, outcome:"refused_absent"} — never an
// authored name, catalog payload, or premise detail.
type mysqlSRSDropAbsentRule struct {
	id    string
	level rule.Level
}

func newMySQLSRSDropAbsentRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return mysqlSRSDropAbsentRule{
		id:    ruleIDMySQLSRSDropAbsent,
		level: configuredLevel(cfg, rule.LevelBlocker),
	}, nil
}

func (r mysqlSRSDropAbsentRule) ID() string { return r.id }

func (r mysqlSRSDropAbsentRule) AppliesTo(statement spec.Statement) bool {
	return srsDropStatement(statement)
}

func (r mysqlSRSDropAbsentRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	adjudication := srsDropAdjudicationFor(statement)
	if adjudication == nil || adjudication.Arm != spec.SRSDropArmAbsentRefused {
		return nil, nil
	}
	srid := srsDropSRID(statement)
	return []rule.Finding{{
		Level:   r.level,
		Message: fmt.Sprintf("spatial reference system %d does not exist", srid),
		Explanation: &rule.FindingExplanation{
			Why:        "The bounded ST_SPATIAL_REFERENCE_SYSTEMS read returned a clean zero-observed-row answer under the registered completeness proof — the authored SRID is proven absent, so the drop cannot land.",
			Risk:       "The statement names an object the catalog proves absent; a plain DROP without IF EXISTS is refused.",
			Suggestion: "Remove the statement, author the SRID first, or add IF EXISTS if an absent SRID is acceptable.",
		},
		Metadata: map[string]any{
			"srid":    srid,
			"outcome": "refused_absent",
		},
	}}, nil
}

// ddl.mysql.srs.drop.absent_noop — the proven-absence IF EXISTS
// notice (T26-SRS-DROP-PROVEN-ABSENT-EXECUTION-01). Fires only on the
// shared adjudication's absent_noop arm: an IF EXISTS drop whose
// proven-absent object makes the statement a no-op while the
// statement's own privilege premise was consumable — the conditional
// marker rides this finding's metadata, never a sibling rule.
// Metadata stays exactly {srid:uint32, outcome:"absent_noop",
// conditional_audit:"observed_account_new_session"}.
type mysqlSRSDropAbsentNoopRule struct {
	id    string
	level rule.Level
}

func newMySQLSRSDropAbsentNoopRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return mysqlSRSDropAbsentNoopRule{
		id:    ruleIDMySQLSRSDropAbsentNoop,
		level: configuredLevel(cfg, rule.LevelNotice),
	}, nil
}

func (r mysqlSRSDropAbsentNoopRule) ID() string { return r.id }

func (r mysqlSRSDropAbsentNoopRule) AppliesTo(statement spec.Statement) bool {
	return srsDropStatement(statement)
}

func (r mysqlSRSDropAbsentNoopRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	adjudication := srsDropAdjudicationFor(statement)
	if adjudication == nil || adjudication.Arm != spec.SRSDropArmAbsentNoop {
		return nil, nil
	}
	srid := srsDropSRID(statement)
	return []rule.Finding{{
		Level:   r.level,
		Message: fmt.Sprintf("spatial reference system %d does not exist; IF EXISTS drop is a no-op", srid),
		Explanation: &rule.FindingExplanation{
			Why:        "The bounded ST_SPATIAL_REFERENCE_SYSTEMS read returned a clean zero-observed-row answer under the registered completeness proof — the authored SRID is proven absent and the authored IF EXISTS guard makes the statement a no-op.",
			Risk:       "The no-op's privilege premise was discharged under the observed_account_new_session conditional audit: a new session is assumed to hold the observed account's grant when it loads privileges — never a verified future executor.",
			Suggestion: "Keep the statement only if the absent SRID is acceptable; otherwise author the SRID first.",
		},
		Metadata: map[string]any{
			"srid":              srid,
			"outcome":           "absent_noop",
			"conditional_audit": string(spec.PrivilegeAuditModeObservedAccountNewSession),
		},
	}}, nil
}
