// Package ddl: the audited MySQL ALTER INSTANCE slice —
// T26-ALTER-INSTANCE-PUBLICATION-01 over the reviewer-frozen contract
// (Phase 0 evidence + frozen clauses D1..D10 in the work-order
// record). Every rule reads one truth: spec.AdjudicateAlterInstance's
// arm plus its derived owed-fact set, never a recomputation.
//
//   - ddl.mysql.alter_instance.reload_tls is the audited marker and
//     evidence-gap carrier for the RELOAD TLS form — the
//     no_rollback_on_error variant rides the same rule (frozen D7:
//     the modifier is a variant member, not a separate feature id).
//   - ddl.mysql.alter_instance.rotate_master_key is the audited
//     marker and evidence-gap carrier for ROTATE INNODB MASTER KEY.
//   - ddl.mysql.alter_instance.privilege is the family's conditional
//     notice — it fires only when the shared adjudication says this
//     statement consumed its own privilege premise on the outcome arm
//     and carries exactly {form, conditional_audit} (frozen D3).
//
// Public surfaces stay bounded: finding metadata carries the form
// token, the variant member flag, the arm outcome, and the
// conditional-audit marker — never certificate material, key text,
// keyring contents, or grant payload text (frozen D10). A successful
// return code is never rendered as a proven reload or rotation.
package ddl

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// alterInstanceStatement reports whether the statement is an audited
// MySQL ALTER INSTANCE — dialect, kind, typed declaration, and no
// unsupported marker.
func alterInstanceStatement(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectMySQL &&
		statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.AlterInstance != nil &&
		statement.Unsupported == nil
}

// alterInstanceAdjudicationFor returns the shared adjudication the
// enrichment attached — nil outside the audited slice, where the rules
// fall back to the defensive contract: every premise owed.
func alterInstanceAdjudicationFor(statement spec.Statement) *spec.AlterInstanceAdjudication {
	if statement.Metadata == nil {
		return nil
	}
	return statement.Metadata.AlterInstance
}

// alterInstanceOwed returns the adjudication's owed set — the one
// truth the gap carrier publishes verbatim. Outside the audited slice
// the defensive fallback owes every premise the form's arm could
// need.
func alterInstanceOwed(statement spec.Statement) []string {
	if adjudication := alterInstanceAdjudicationFor(statement); adjudication != nil {
		return adjudication.Owed
	}
	decl := statement.DDL.AlterInstance
	return []string{spec.AlterInstanceFactPrivilege, decl.InstancePremise()}
}

// alterInstanceOutcome maps the adjudicated arm to the serialized
// outcome token — the bounded vocabulary findings publish.
func alterInstanceOutcome(adjudication *spec.AlterInstanceAdjudication) string {
	if adjudication != nil && adjudication.Arm == spec.AlterInstanceArmBlocked {
		return "blocked"
	}
	return "unproven"
}

// alterInstanceMetadata builds the bounded finding metadata — form,
// outcome, the variant flag when authored, and the conditional-audit
// marker when this statement's own premise discharged (frozen D10:
// never payload text, never grant text).
func alterInstanceMetadata(statement spec.Statement, adjudication *spec.AlterInstanceAdjudication) map[string]any {
	decl := statement.DDL.AlterInstance
	metadata := map[string]any{
		"form":    alterInstanceFormToken(decl.Form),
		"outcome": alterInstanceOutcome(adjudication),
	}
	if decl.NoRollbackOnError {
		metadata["no_rollback_on_error"] = true
	}
	if adjudication != nil && adjudication.ConditionalAudit {
		metadata["conditional_audit"] = string(spec.PrivilegeAuditModeObservedAccountNewSession)
	}
	return metadata
}

func alterInstanceFormToken(form spec.AlterInstanceForm) string {
	if form == spec.AlterInstanceFormRotateInnodbMasterKey {
		return "rotate_innodb_master_key"
	}
	return "reload_tls"
}

// alterInstanceEvaluate is the shared per-form evaluation: the blocked
// arm fires a notice that an earlier batch statement contaminated the
// premise surface; the unproven arm fires the bounded-outcome notice —
// the statement is audited, its success is never claimed proven, and
// the owed facts publish through EvidenceGaps.
func alterInstanceEvaluate(statement spec.Statement, level rule.Level, form spec.AlterInstanceForm) []rule.Finding {
	adjudication := alterInstanceAdjudicationFor(statement)
	metadata := alterInstanceMetadata(statement, adjudication)
	name := "ALTER INSTANCE RELOAD TLS"
	if statement.DDL.AlterInstance.NoRollbackOnError {
		name = "ALTER INSTANCE RELOAD TLS NO ROLLBACK ON ERROR"
	}
	if form == spec.AlterInstanceFormRotateInnodbMasterKey {
		name = "ALTER INSTANCE ROTATE INNODB MASTER KEY"
	}
	if adjudication != nil && adjudication.Arm == spec.AlterInstanceArmBlocked {
		return []rule.Finding{{
			Level:   level,
			Message: name + " blocked — an earlier statement may have mutated the instance surface",
			Explanation: &rule.FindingExplanation{
				Why:        "A preceding batch statement ran the ALTER INSTANCE apply, so the TLS-context or keyring premise this statement would read may be stale — the bind is blocked before reading it.",
				Risk:       "The outcome cannot be bounded against the pre-state — the batch-effects evidence is owed.",
				Suggestion: "Audit the statement in its own batch so the instance premise is read before any mutation.",
			},
			Metadata: metadata,
		}}
	}
	return []rule.Finding{{
		Level:   level,
		Message: name + " audited — outcome unproven",
		Explanation: &rule.FindingExplanation{
			Why:        "The instance premise this form needs (a usable TLS context / a loaded keyring backend) has no proven supplier, and a successful return code never proves the operation took effect — the NO ROLLBACK variant makes even that signal weaker.",
			Risk:       "The statement may have run, failed silently, or taken partial effect — the outcome stays bounded by the owed facts.",
			Suggestion: "Confirm the result against the server's own observables (TLS status / keyring state); the audit reports only what is provable.",
		},
		Metadata: metadata,
	}}
}

// alterInstanceEvidenceGaps carries the adjudication's owed facts
// verbatim on every arm — blocked or unproven. When the owed set is
// exactly this statement's own privilege premise, the reason narrows
// to unproven_privilege; otherwise the generic
// unknown_instance_state reason holds.
func alterInstanceEvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	owed := alterInstanceOwed(statement)
	if len(owed) == 0 {
		return nil
	}
	reason := "unknown_instance_state"
	privilegeOnly := true
	for _, fact := range owed {
		if fact != spec.AlterInstanceFactPrivilege {
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

// ddl.mysql.alter_instance.reload_tls — the audited marker and gap
// carrier for RELOAD TLS and its NO ROLLBACK ON ERROR variant.
type mysqlAlterInstanceReloadTLSRule struct {
	id    string
	level rule.Level
}

func newMySQLAlterInstanceReloadTLSRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return mysqlAlterInstanceReloadTLSRule{
		id:    ruleIDMySQLAlterInstanceReloadTLS,
		level: configuredLevel(cfg, rule.LevelNotice),
	}, nil
}

func (r mysqlAlterInstanceReloadTLSRule) ID() string { return r.id }

func (r mysqlAlterInstanceReloadTLSRule) AppliesTo(statement spec.Statement) bool {
	return alterInstanceStatement(statement) &&
		statement.DDL.AlterInstance.Form == spec.AlterInstanceFormReloadTLS
}

func (r mysqlAlterInstanceReloadTLSRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	return alterInstanceEvidenceGaps(statement)
}

func (r mysqlAlterInstanceReloadTLSRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	return alterInstanceEvaluate(statement, r.level, spec.AlterInstanceFormReloadTLS), nil
}

// ddl.mysql.alter_instance.rotate_master_key — the audited marker and
// gap carrier for ROTATE INNODB MASTER KEY.
type mysqlAlterInstanceRotateKeyRule struct {
	id    string
	level rule.Level
}

func newMySQLAlterInstanceRotateKeyRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return mysqlAlterInstanceRotateKeyRule{
		id:    ruleIDMySQLAlterInstanceRotateKey,
		level: configuredLevel(cfg, rule.LevelNotice),
	}, nil
}

func (r mysqlAlterInstanceRotateKeyRule) ID() string { return r.id }

func (r mysqlAlterInstanceRotateKeyRule) AppliesTo(statement spec.Statement) bool {
	return alterInstanceStatement(statement) &&
		statement.DDL.AlterInstance.Form == spec.AlterInstanceFormRotateInnodbMasterKey
}

func (r mysqlAlterInstanceRotateKeyRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	return alterInstanceEvidenceGaps(statement)
}

func (r mysqlAlterInstanceRotateKeyRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	return alterInstanceEvaluate(statement, r.level, spec.AlterInstanceFormRotateInnodbMasterKey), nil
}

// ddl.mysql.alter_instance.privilege — the family's conditional
// notice: fires only when the shared adjudication reports this
// statement consumed its own privilege premise on the outcome arm
// (frozen D3). Metadata stays {form, conditional_audit} — never the
// observed account or grant text.
type mysqlAlterInstancePrivilegeRule struct {
	id    string
	level rule.Level
}

func newMySQLAlterInstancePrivilegeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return mysqlAlterInstancePrivilegeRule{
		id:    ruleIDMySQLAlterInstancePrivilege,
		level: configuredLevel(cfg, rule.LevelNotice),
	}, nil
}

func (r mysqlAlterInstancePrivilegeRule) ID() string { return r.id }

func (r mysqlAlterInstancePrivilegeRule) AppliesTo(statement spec.Statement) bool {
	return alterInstanceStatement(statement)
}

func (r mysqlAlterInstancePrivilegeRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	adjudication := alterInstanceAdjudicationFor(statement)
	if adjudication == nil || !adjudication.ConditionalAudit {
		return nil, nil
	}
	return []rule.Finding{{
		Level:   r.level,
		Message: "alter instance audited under the observed-account privilege scope",
		Metadata: map[string]any{
			"form":              alterInstanceFormToken(statement.DDL.AlterInstance.Form),
			"conditional_audit": string(spec.PrivilegeAuditModeObservedAccountNewSession),
		},
	}}, nil
}
