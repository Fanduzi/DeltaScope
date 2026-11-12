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
//     unknown_srs_state reason.
//   - ddl.mysql.srs.drop.reserved is an advisory that co-fires on the
//     blocked and unproven arms when the SRID sits in a registered
//     reserved range — never inside a refusal arm's own finding.
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
// fixed unknown_srs_state reason.
func (r mysqlSRSDropDependencyRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	owed := srsDropOwed(statement)
	if len(owed) == 0 {
		return nil
	}
	return []rule.EvidenceGap{{
		ReasonCode:    "unknown_srs_state",
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
// co-fires on the blocked and unproven arms only and carries exactly
// {srid} — never arm, outcome, or gap data.
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
		(adjudication.Arm != spec.SRSDropArmBlocked && adjudication.Arm != spec.SRSDropArmUnproven) {
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
			Why:        "The authored SRID sits inside a documented MySQL reserved range; a native drop of a reserved SRID is refused regardless of catalog state.",
			Risk:       "The statement would be refused if it lands — the advisory does not claim it executed.",
			Suggestion: "Review whether the reserved SRID was intended; pick a non-reserved identifier for new spatial reference systems.",
		},
		Metadata: map[string]any{
			"srid": srid,
		},
	}}, nil
}
