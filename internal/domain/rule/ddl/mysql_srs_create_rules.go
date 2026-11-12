// Package ddl: the audited MySQL CREATE [OR REPLACE] SPATIAL
// REFERENCE SYSTEM slice — T26-SRS-CREATE-PUBLICATION-01 over the
// reviewer-frozen contract (Phase 0 evidence + frozen clauses at
// docs/verification/t26-srs-create-publication-01/README.md). Every
// rule reads one truth: spec.AdjudicateSRSCreate's arm plus its
// derived owed-fact set, never a recomputation.
//
//   - ddl.mysql.srs.create.srid0 fires the fixed SRID-0 blocker on the
//     refused_srid0 arm (native 3714).
//   - ddl.mysql.srs.create.duplicate_attribute fires the blocker on
//     the refused_duplicate_attribute arm (native 3709).
//   - ddl.mysql.srs.create.missing_attribute fires the blocker on the
//     refused_missing_attribute arm (native 3708); metadata names the
//     missing member — NAME or DEFINITION.
//   - ddl.mysql.srs.create.existence fires the refusal blocker on the
//     refused_existence arm (native 3712) and carries the evidence
//     gap on every arm whose premises stayed owed — blocked,
//     exists_noop-with-debt, create, replace, unproven — under
//     unknown_srs_state, or unproven_privilege when the only debt is
//     this statement's own privilege premise.
//   - ddl.mysql.srs.create.exists_noop carries the IF NOT EXISTS
//     no-op notice on the exists_noop arm (native warning 3713 —
//     frozen U2: never a blocker, never a claimed re-create); its
//     metadata carries conditional_audit when the premise discharged.
//   - ddl.mysql.srs.create.name_conflict fires the unique-key
//     blocker on the refused_name_conflict arm (native 1062).
//   - ddl.mysql.srs.create.organization_conflict fires the unique-key
//     blocker on the refused_org_conflict arm (native 1062).
//   - ddl.mysql.srs.create.privilege is the family's conditional
//     notice — it fires only when the shared adjudication says this
//     statement consumed its own privilege premise on an outcome arm
//     (create, replace, or unproven) and carries exactly
//     {srid, conditional_audit} (frozen F-7).
//
// Public surfaces stay bounded: finding metadata carries the numeric
// SRID, the arm's outcome token, and the attribute name — never
// DEFINITION or DESCRIPTION payload text (frozen F-9).
package ddl

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// srsCreateStatement reports whether the statement is an audited MySQL
// CREATE [OR REPLACE] SPATIAL REFERENCE SYSTEM — dialect, family,
// operation, a create kind, and no unsupported marker.
func srsCreateStatement(statement spec.Statement) bool {
	if statement.Dialect != spec.DialectMySQL ||
		statement.Kind != spec.KindDDL ||
		statement.DDL == nil ||
		statement.DDL.SpatialReference == nil ||
		statement.Unsupported != nil {
		return false
	}
	kind := statement.DDL.SpatialReference.Kind
	return kind == spec.SRSKindCreate || kind == spec.SRSKindCreateOrReplace
}

// srsCreateAdjudicationFor returns the shared adjudication the
// enrichment attached — nil outside the audited slice, where the rules
// fall back to the defensive contract: every premise owed.
func srsCreateAdjudicationFor(statement spec.Statement) *spec.SRSCreateAdjudication {
	if statement.Metadata == nil {
		return nil
	}
	return statement.Metadata.SRSCreate
}

func srsCreateSRID(statement spec.Statement) uint32 {
	return statement.DDL.SpatialReference.SRID
}

// srsCreateOwed returns the adjudication's owed set — the one truth the
// gap carrier publishes verbatim. Outside the audited slice the
// defensive fallback owes every premise.
func srsCreateOwed(statement spec.Statement) []string {
	if adjudication := srsCreateAdjudicationFor(statement); adjudication != nil {
		return adjudication.Owed
	}
	return []string{spec.SRSFactExistence, spec.SRSFactNameConflict, spec.SRSFactOrganizationConflict, spec.SRSFactCreatePrivilege}
}

// ddl.mysql.srs.create.srid0 — the fixed SRID-0 refusal blocker.
type mysqlSRSCreateSRID0Rule struct {
	id    string
	level rule.Level
}

func newMySQLSRSCreateSRID0Rule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return mysqlSRSCreateSRID0Rule{
		id:    ruleIDMySQLSRSCreateSRID0,
		level: configuredLevel(cfg, rule.LevelBlocker),
	}, nil
}

func (r mysqlSRSCreateSRID0Rule) ID() string { return r.id }

func (r mysqlSRSCreateSRID0Rule) AppliesTo(statement spec.Statement) bool {
	return srsCreateStatement(statement)
}

func (r mysqlSRSCreateSRID0Rule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	adjudication := srsCreateAdjudicationFor(statement)
	if adjudication == nil || adjudication.Arm != spec.SRSCreateArmRefusedSRID0 {
		return nil, nil
	}
	return []rule.Finding{{
		Level:   r.level,
		Message: "creating spatial reference system SRID 0 is not allowed",
		Explanation: &rule.FindingExplanation{
			Why:        "MySQL reserves SRID 0 and refuses to create it regardless of catalog state or authored attributes.",
			Risk:       "The statement cannot land — the refusal is fixed by the authored numeric identity alone.",
			Suggestion: "Remove the statement or target a non-zero SRID.",
		},
		Metadata: map[string]any{
			"srid":    0,
			"outcome": "refused_srid0",
		},
	}}, nil
}

// ddl.mysql.srs.create.duplicate_attribute — the authored-input
// refusal blocker for a member defined twice (native 3709).
type mysqlSRSCreateDuplicateAttributeRule struct {
	id    string
	level rule.Level
}

func newMySQLSRSCreateDuplicateAttributeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return mysqlSRSCreateDuplicateAttributeRule{
		id:    ruleIDMySQLSRSCreateDuplicateAttribute,
		level: configuredLevel(cfg, rule.LevelBlocker),
	}, nil
}

func (r mysqlSRSCreateDuplicateAttributeRule) ID() string { return r.id }

func (r mysqlSRSCreateDuplicateAttributeRule) AppliesTo(statement spec.Statement) bool {
	return srsCreateStatement(statement)
}

func (r mysqlSRSCreateDuplicateAttributeRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	adjudication := srsCreateAdjudicationFor(statement)
	if adjudication == nil || adjudication.Arm != spec.SRSCreateArmRefusedDuplicateAttribute {
		return nil, nil
	}
	srid := srsCreateSRID(statement)
	attr := adjudication.DuplicateAttribute
	return []rule.Finding{{
		Level:   r.level,
		Message: fmt.Sprintf("spatial reference system %d defines attribute %s twice", srid, attr),
		Explanation: &rule.FindingExplanation{
			Why:        "MySQL refuses a CREATE SPATIAL REFERENCE SYSTEM whose attribute list defines the same member twice; the authored input itself settles the refusal — no catalog fact is needed.",
			Risk:       "The statement cannot land — the duplicate member is a deterministic authored-input error.",
			Suggestion: "Keep each attribute at most once in the statement.",
		},
		Metadata: map[string]any{
			"srid":      srid,
			"outcome":   "refused_duplicate_attribute",
			"attribute": attr,
		},
	}}, nil
}

// ddl.mysql.srs.create.missing_attribute — the authored-input refusal
// blocker for a missing mandatory member (native 3708).
type mysqlSRSCreateMissingAttributeRule struct {
	id    string
	level rule.Level
}

func newMySQLSRSCreateMissingAttributeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return mysqlSRSCreateMissingAttributeRule{
		id:    ruleIDMySQLSRSCreateMissingAttribute,
		level: configuredLevel(cfg, rule.LevelBlocker),
	}, nil
}

func (r mysqlSRSCreateMissingAttributeRule) ID() string { return r.id }

func (r mysqlSRSCreateMissingAttributeRule) AppliesTo(statement spec.Statement) bool {
	return srsCreateStatement(statement)
}

func (r mysqlSRSCreateMissingAttributeRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	adjudication := srsCreateAdjudicationFor(statement)
	if adjudication == nil || adjudication.Arm != spec.SRSCreateArmRefusedMissingAttribute {
		return nil, nil
	}
	srid := srsCreateSRID(statement)
	attr := adjudication.MissingAttribute
	return []rule.Finding{{
		Level:   r.level,
		Message: fmt.Sprintf("spatial reference system %d is missing mandatory attribute %s", srid, attr),
		Explanation: &rule.FindingExplanation{
			Why:        "MySQL requires the NAME and DEFINITION attributes on CREATE SPATIAL REFERENCE SYSTEM; the authored statement omits one, so the refusal is settled by the authored input alone.",
			Risk:       "The statement cannot land — the mandatory member is missing before any catalog fact applies.",
			Suggestion: "Add the missing mandatory attribute to the statement.",
		},
		Metadata: map[string]any{
			"srid":      srid,
			"outcome":   "refused_missing_attribute",
			"attribute": attr,
		},
	}}, nil
}

// ddl.mysql.srs.create.existence — the existence refusal blocker plus
// the family's evidence-gap carrier. The gap carries the
// adjudication's owed facts on every arm whose premises stayed owed —
// blocked, exists_noop-with-debt, create, replace, and unproven —
// under the fixed unknown_srs_state reason, or unproven_privilege
// when this statement's own privilege premise is the only debt.
type mysqlSRSCreateExistenceRule struct {
	id    string
	level rule.Level
}

func newMySQLSRSCreateExistenceRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return mysqlSRSCreateExistenceRule{
		id:    ruleIDMySQLSRSCreateExistence,
		level: configuredLevel(cfg, rule.LevelBlocker),
	}, nil
}

func (r mysqlSRSCreateExistenceRule) ID() string { return r.id }

func (r mysqlSRSCreateExistenceRule) AppliesTo(statement spec.Statement) bool {
	return srsCreateStatement(statement)
}

// EvidenceGaps carries the adjudication's owed facts verbatim on the
// non-refusal arms. When the owed set is exactly this statement's own
// privilege premise — the exists_noop arm's only possible debt — the
// reason narrows to unproven_privilege; otherwise the generic
// unknown_srs_state reason holds.
func (r mysqlSRSCreateExistenceRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	owed := srsCreateOwed(statement)
	if len(owed) == 0 {
		return nil
	}
	reason := "unknown_srs_state"
	privilegeOnly := true
	for _, fact := range owed {
		if fact != spec.SRSFactCreatePrivilege {
			privilegeOnly = false
			break
		}
	}
	if privilegeOnly {
		reason = "unproven_privilege"
	}
	return []rule.EvidenceGap{{
		ReasonCode:    reason,
		RequiredFacts: owed,
	}}
}

func (r mysqlSRSCreateExistenceRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	adjudication := srsCreateAdjudicationFor(statement)
	if adjudication == nil || adjudication.Arm != spec.SRSCreateArmRefusedExistence {
		return nil, nil
	}
	srid := srsCreateSRID(statement)
	return []rule.Finding{{
		Level:   r.level,
		Message: fmt.Sprintf("spatial reference system %d already exists", srid),
		Explanation: &rule.FindingExplanation{
			Why:        "The bound catalog answer — or a conflict witness on this same SRID — proves the object is present; a plain CREATE without IF NOT EXISTS is refused, matching the native 3712 refusal.",
			Risk:       "The statement cannot land — the authored SRID is taken.",
			Suggestion: "Pick a different SRID, or add IF NOT EXISTS if an existing object is acceptable.",
		},
		Metadata: map[string]any{
			"srid":    srid,
			"outcome": "refused_existence",
		},
	}}, nil
}

// ddl.mysql.srs.create.exists_noop — the IF NOT EXISTS no-op notice on
// a proven-present SRID (native warning 3713, frozen U2): the
// statement is not judged a failure for the target existing, and the
// object is never claimed re-created. Metadata stays {srid, outcome}
// plus conditional_audit when this statement's own privilege premise
// was consumable — never an authored name or attribute payload.
type mysqlSRSCreateExistsNoopRule struct {
	id    string
	level rule.Level
}

func newMySQLSRSCreateExistsNoopRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return mysqlSRSCreateExistsNoopRule{
		id:    ruleIDMySQLSRSCreateExistsNoop,
		level: configuredLevel(cfg, rule.LevelNotice),
	}, nil
}

func (r mysqlSRSCreateExistsNoopRule) ID() string { return r.id }

func (r mysqlSRSCreateExistsNoopRule) AppliesTo(statement spec.Statement) bool {
	return srsCreateStatement(statement)
}

func (r mysqlSRSCreateExistsNoopRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	adjudication := srsCreateAdjudicationFor(statement)
	if adjudication == nil || adjudication.Arm != spec.SRSCreateArmExistsNoop {
		return nil, nil
	}
	srid := srsCreateSRID(statement)
	metadata := map[string]any{
		"srid":    srid,
		"outcome": "exists_noop",
	}
	if adjudication.ConditionalAudit {
		metadata["conditional_audit"] = string(spec.PrivilegeAuditModeObservedAccountNewSession)
	}
	return []rule.Finding{{
		Level:   r.level,
		Message: fmt.Sprintf("spatial reference system %d already exists; IF NOT EXISTS create is a no-op", srid),
		Explanation: &rule.FindingExplanation{
			Why:        "The bound catalog answer proves the object is present; the authored IF NOT EXISTS guard makes the statement a no-op — matching the native warning instead of a failure.",
			Risk:       "The statement leaves the existing object untouched — it is not re-created and none of the authored attributes apply.",
			Suggestion: "Remove the statement if the existing object was expected to differ, or keep it as an idempotent guard.",
		},
		Metadata: metadata,
	}}, nil
}

// ddl.mysql.srs.create.name_conflict — the NAME unique-key refusal
// blocker (native 1062). Fires only when the authored NAME's conflict
// read returned a witness SRID other than the statement's own.
type mysqlSRSCreateNameConflictRule struct {
	id    string
	level rule.Level
}

func newMySQLSRSCreateNameConflictRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return mysqlSRSCreateNameConflictRule{
		id:    ruleIDMySQLSRSCreateNameConflict,
		level: configuredLevel(cfg, rule.LevelBlocker),
	}, nil
}

func (r mysqlSRSCreateNameConflictRule) ID() string { return r.id }

func (r mysqlSRSCreateNameConflictRule) AppliesTo(statement spec.Statement) bool {
	return srsCreateStatement(statement)
}

func (r mysqlSRSCreateNameConflictRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	adjudication := srsCreateAdjudicationFor(statement)
	if adjudication == nil || adjudication.Arm != spec.SRSCreateArmRefusedNameConflict {
		return nil, nil
	}
	srid := srsCreateSRID(statement)
	witnesses := 0
	if statement.Metadata != nil && statement.Metadata.SpatialReference != nil &&
		statement.Metadata.SpatialReference.NameConflict != nil {
		witnesses = len(statement.Metadata.SpatialReference.NameConflict.MatchedSRIDs)
	}
	return []rule.Finding{{
		Level:   r.level,
		Message: fmt.Sprintf("spatial reference system %d takes a NAME already held by another visible SRID", srid),
		Explanation: &rule.FindingExplanation{
			Why:        "The SRS_NAME unique-key witness read returned another SRID holding the authored name — matching the native 1062 refusal.",
			Risk:       "The statement cannot land — the authored NAME belongs to a different spatial reference system.",
			Suggestion: "Choose a NAME no other spatial reference system holds, or drop the conflicting object first.",
		},
		Metadata: map[string]any{
			"srid":               srid,
			"outcome":            "refused_name_conflict",
			"name_witness_count": witnesses,
		},
	}}, nil
}

// ddl.mysql.srs.create.organization_conflict — the ORGANIZATION +
// ORGANIZATION_COORDSYS_ID unique-key refusal blocker (native 1062).
// Fires only when the authored pair's conflict read returned a witness
// SRID other than the statement's own — a match on the statement's own
// SRID is the pair the object already owns, never a conflict.
type mysqlSRSCreateOrgConflictRule struct {
	id    string
	level rule.Level
}

func newMySQLSRSCreateOrgConflictRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return mysqlSRSCreateOrgConflictRule{
		id:    ruleIDMySQLSRSCreateOrgConflict,
		level: configuredLevel(cfg, rule.LevelBlocker),
	}, nil
}

func (r mysqlSRSCreateOrgConflictRule) ID() string { return r.id }

func (r mysqlSRSCreateOrgConflictRule) AppliesTo(statement spec.Statement) bool {
	return srsCreateStatement(statement)
}

func (r mysqlSRSCreateOrgConflictRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	adjudication := srsCreateAdjudicationFor(statement)
	if adjudication == nil || adjudication.Arm != spec.SRSCreateArmRefusedOrgConflict {
		return nil, nil
	}
	srid := srsCreateSRID(statement)
	witnesses := 0
	if statement.Metadata != nil && statement.Metadata.SpatialReference != nil &&
		statement.Metadata.SpatialReference.OrgConflict != nil {
		witnesses = len(statement.Metadata.SpatialReference.OrgConflict.MatchedSRIDs)
	}
	return []rule.Finding{{
		Level:   r.level,
		Message: fmt.Sprintf("spatial reference system %d takes an ORGANIZATION + IDENTIFIED BY pair already held by another visible SRID", srid),
		Explanation: &rule.FindingExplanation{
			Why:        "The ORGANIZATION_AND_ID unique-key witness read returned another SRID holding the authored pair — matching the native 1062 refusal.",
			Risk:       "The statement cannot land — the authored ORGANIZATION + IDENTIFIED BY pair belongs to a different spatial reference system.",
			Suggestion: "Choose an ORGANIZATION + IDENTIFIED BY pair no other spatial reference system holds.",
		},
		Metadata: map[string]any{
			"srid":              srid,
			"outcome":           "refused_organization_conflict",
			"org_witness_count": witnesses,
		},
	}}, nil
}

// ddl.mysql.srs.create.privilege — the family's conditional privilege
// notice (frozen F-7). It fires exactly when the shared adjudication
// reports the statement consumed its own privilege premise on an
// outcome arm — create, replace, or unproven — and carries exactly
// {srid, conditional_audit}: never an outcome token, account, grant
// text, session detail, or observation payload. The exists_noop arm
// discharges the same premise through its own marker instead.
type mysqlSRSCreatePrivilegeRule struct {
	id    string
	level rule.Level
}

func newMySQLSRSCreatePrivilegeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return mysqlSRSCreatePrivilegeRule{
		id:    ruleIDMySQLSRSCreatePrivilege,
		level: configuredLevel(cfg, rule.LevelNotice),
	}, nil
}

func (r mysqlSRSCreatePrivilegeRule) ID() string { return r.id }

func (r mysqlSRSCreatePrivilegeRule) AppliesTo(statement spec.Statement) bool {
	return srsCreateStatement(statement)
}

func (r mysqlSRSCreatePrivilegeRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	adjudication := srsCreateAdjudicationFor(statement)
	if adjudication == nil ||
		(adjudication.Arm != spec.SRSCreateArmCreate &&
			adjudication.Arm != spec.SRSCreateArmReplace &&
			adjudication.Arm != spec.SRSCreateArmUnproven) ||
		adjudication.Privilege != spec.SRSCreatePrivilegeConsumable {
		return nil, nil
	}
	srid := srsCreateSRID(statement)
	return []rule.Finding{{
		Level:   r.level,
		Message: fmt.Sprintf("spatial reference system %d create privilege premise discharged under the observed_account_new_session conditional audit", srid),
		Explanation: &rule.FindingExplanation{
			Why:        "The shared request-level privilege observation bound a positive direct-global SUPER-or-equivalent witness for the observed account, and the ordered scope was still open at this statement — this statement's own privilege premise is discharged.",
			Risk:       "The witness binds the observed account under a conditional audit: execution is assumed on a new session established after the observation on the same target, with the relied-upon grant still applying when that session loads privileges — never a verified future executor or a pooled connection's live privileges.",
			Suggestion: "Keep the observed account's direct global SUPER-or-equivalent grant intact for the session that executes the create.",
		},
		Metadata: map[string]any{
			"srid":              srid,
			"conditional_audit": string(spec.PrivilegeAuditModeObservedAccountNewSession),
		},
	}}, nil
}
