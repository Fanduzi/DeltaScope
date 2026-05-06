// Package ddl defines Tier-1 DDL rules.
// input: normalized Statement specs for PostgreSQL table privilege DCL operations
// output: findings for PostgreSQL GRANT/REVOKE table privilege risks
// pos: PostgreSQL-specific table privilege rule implementations
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type pgTablePrivilegeRule struct {
	id       string
	level    rule.Level
	operation spec.DDLOperation
	optionKey string
	optionValue string
	requireOption bool
	message  string
	why      string
	risk     string
	suggestion string
}

func newPGTablePrivilegeRule(id string, level rule.Level, operation spec.DDLOperation, optionKey, optionValue string, requireOption bool, message, why, risk, suggestion string, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return pgTablePrivilegeRule{
		id:            id,
		level:         configuredLevel(cfg, level),
		operation:     operation,
		optionKey:     optionKey,
		optionValue:   optionValue,
		requireOption: requireOption,
		message:       message,
		why:           why,
		risk:          risk,
		suggestion:    suggestion,
	}, nil
}

func (r pgTablePrivilegeRule) ID() string { return r.id }

func (r pgTablePrivilegeRule) AppliesTo(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectPostgreSQL &&
		statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Operation == r.operation
}

func (r pgTablePrivilegeRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	if r.requireOption && statement.DDL.Options[r.optionKey] != r.optionValue {
		return nil, nil
	}
	if !r.requireOption && r.optionKey != "" && statement.DDL.Options[r.optionKey] != r.optionValue {
		return nil, nil
	}

	objectName := statement.DDL.ObjectName
	message := fmt.Sprintf(r.message, objectName)

	metadata := map[string]any{
		"operation":   string(statement.DDL.Operation),
		"object_type": statement.DDL.ObjectType,
		"object_name": objectName,
	}
	for _, key := range []string{"privileges", "all_privileges", "grantees", "schema", "cascade"} {
		if val := statement.DDL.Options[key]; val != "" {
			metadata[key] = val
		}
	}

	return []rule.Finding{{
		Level:   r.level,
		Message: message,
		Explanation: &rule.FindingExplanation{
			Why:        r.why,
			Risk:       r.risk,
			Suggestion: r.suggestion,
		},
		Metadata: metadata,
	}}, nil
}

func newGrantTablePrivilegeNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGTablePrivilegeRule(
		ruleIDPGGrantTablePrivilegeNotice, rule.LevelNotice, spec.DDLOperationGrantTable,
		"", "", false,
		"GRANT on table %q — review table privilege assignment on PostgreSQL",
		"GRANT assigns table-level privileges to roles, changing who can read, write, or manage table data.",
		"Overly broad grants may allow unauthorized data access or modification if roles are shared or compromised.",
		"Follow least-privilege: grant only the privileges each role needs and review periodically.",
		cfg,
	)
}

func newGrantTablePrivilegeAllWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGTablePrivilegeRule(
		ruleIDPGGrantTablePrivilegeAllWarn, rule.LevelWarning, spec.DDLOperationGrantTable,
		"all_privileges", "true", true,
		"GRANT ALL PRIVILEGES on table %q grants unrestricted table access on PostgreSQL",
		"ALL PRIVILEGES grants SELECT, INSERT, UPDATE, DELETE, TRUNCATE, REFERENCES, and TRIGGER in a single statement.",
		"Granting all privileges is rarely necessary and makes it harder to audit or restrict access later.",
		"Prefer granting only the specific privileges needed. If ALL PRIVILEGES is intentional, document the reason.",
		cfg,
	)
}

func newRevokeTablePrivilegeNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGTablePrivilegeRule(
		ruleIDPGRevokeTablePrivilegeNotice, rule.LevelNotice, spec.DDLOperationRevokeTable,
		"", "", false,
		"REVOKE on table %q — table privilege revoked on PostgreSQL",
		"REVOKE removes table-level privileges from roles, changing who can access or modify table data.",
		"Revoking privileges may break application queries or reporting pipelines that depend on the removed access.",
		"Before revoking, verify no application or service relies on the privileges being removed.",
		cfg,
	)
}

func newRevokeTablePrivilegeCascadeWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPGTablePrivilegeRule(
		ruleIDPGRevokeTablePrivilegeCascadeWarn, rule.LevelWarning, spec.DDLOperationRevokeTable,
		"cascade", "true", true,
		"REVOKE on table %q CASCADE revokes dependent grants on PostgreSQL",
		"CASCADE tells PostgreSQL to also revoke privileges that were granted by the target grantees to others.",
		"Cascading revocation can silently remove access from roles that received grants through the target grantees, causing unexpected permission failures.",
		"Prefer RESTRICT during review. If CASCADE is intentional, enumerate dependent grants and confirm the blast radius.",
		cfg,
	)
}
