// Package dml defines Tier-1 DML rules.
// input: DML statements plus policy-backed schema/table denylist entries
// output: findings that block DML against protected schemas or tables
// pos: DML object-scope denylist guardrails for audit completion
// note: if this file changes, update this header and module README.md.
package dml

import (
	"context"
	"fmt"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type tableDenylistRule struct {
	level           rule.Level
	schemas         map[string]struct{}
	tables          map[string]struct{}
	qualifiedTables map[string]struct{}
}

func newTableDenylistRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	schemas, err := normalizedStringSetParam(ruleIDTableDenylistForbid, cfg, "schemas", nil)
	if err != nil {
		return nil, err
	}
	tables, err := normalizedStringSetParam(ruleIDTableDenylistForbid, cfg, "tables", nil)
	if err != nil {
		return nil, err
	}
	qualifiedTables, err := normalizedStringSetParam(ruleIDTableDenylistForbid, cfg, "qualified_tables", nil)
	if err != nil {
		return nil, err
	}
	return tableDenylistRule{
		level:           configuredLevel(cfg, rule.LevelBlocker),
		schemas:         schemas,
		tables:          tables,
		qualifiedTables: qualifiedTables,
	}, nil
}

func (r tableDenylistRule) ID() string { return ruleIDTableDenylistForbid }

func (r tableDenylistRule) AppliesTo(statement spec.Statement) bool {
	return statement.Kind == spec.KindDML && statement.DML != nil && len(statement.DML.Tables) > 0
}

func (r tableDenylistRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil { return nil, err }
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	schemaName := normalizedMetadataSchema(statement)
	findings := make([]rule.Finding, 0)
	for _, table := range statement.DML.Tables {
		tableName := strings.ToLower(strings.TrimSpace(table.Name))
		if !r.matches(schemaName, tableName) {
			continue
		}
		target := qualifiedTableName(schemaName, tableName)
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    fmt.Sprintf("DML target %q is blocked by the table denylist policy", target),
			Suggestion: "run the statement against an allowed table or relax the denylist policy intentionally",
			Metadata: map[string]any{
				"schema":    schemaName,
				"table":     tableName,
				"operation": statement.DML.Operation,
			},
		})
	}
	return findings, nil
}

func (r tableDenylistRule) matches(schemaName, tableName string) bool {
	if tableName == "" {
		return false
	}
	if _, ok := r.tables[tableName]; ok {
		return true
	}
	if schemaName != "" {
		if _, ok := r.schemas[schemaName]; ok {
			return true
		}
		if _, ok := r.qualifiedTables[qualifiedTableName(schemaName, tableName)]; ok {
			return true
		}
	}
	return false
}

func normalizedMetadataSchema(statement spec.Statement) string {
	if statement.Metadata == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(statement.Metadata.Schema))
}

func qualifiedTableName(schemaName, tableName string) string {
	if schemaName == "" {
		return tableName
	}
	return schemaName + "." + tableName
}
