// Package ddl defines Tier-1 DDL rules.
// input: create-table Statement specs with timestamp/default metadata and policy values
// output: findings for missing audit timestamp column patterns
// pos: DDL rule implementations for audit-column governance
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type tableAuditColumnsRequiredRule struct {
	required bool
	level    rule.Level
}

func newTableAuditColumnsRequiredRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	required, err := boolParam(ruleIDTableAuditColumnsRequire, cfg, "required", true)
	if err != nil {
		return nil, err
	}

	return tableAuditColumnsRequiredRule{
		required: required,
		level:    configuredLevel(cfg, rule.LevelWarning),
	}, nil
}

func (r tableAuditColumnsRequiredRule) ID() string { return ruleIDTableAuditColumnsRequire }

func (r tableAuditColumnsRequiredRule) AppliesTo(statement spec.Statement) bool {
	return r.required && appliesToCreateTableColumns(statement)
}

func (r tableAuditColumnsRequiredRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	var hasCreatedAudit bool
	var hasUpdatedAudit bool
	requiresUpdatedAudit := statement.Dialect != spec.DialectPostgreSQL
	for _, column := range statement.DDL.Columns {
		if !isTimeLike(column) || !column.DefaultIsCurrentTimestamp {
			continue
		}
		if column.OnUpdateCurrentTimestamp {
			hasUpdatedAudit = true
			continue
		}
		hasCreatedAudit = true
	}

	findings := make([]rule.Finding, 0, 2)
	if !hasCreatedAudit {
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    "table should include a created-time audit column with DEFAULT CURRENT_TIMESTAMP",
			Suggestion: "add a datetime/timestamp column with DEFAULT CURRENT_TIMESTAMP",
			Metadata: map[string]any{
				"table": statement.DDL.Table.Name,
				"kind":  "created",
			},
		})
	}
	if requiresUpdatedAudit && !hasUpdatedAudit {
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    "table should include an updated-time audit column with DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP",
			Suggestion: "add a datetime/timestamp column with DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP",
			Metadata: map[string]any{
				"table": statement.DDL.Table.Name,
				"kind":  "updated",
			},
		})
	}

	return findings, nil
}
