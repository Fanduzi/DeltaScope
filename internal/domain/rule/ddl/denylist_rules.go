// Package ddl defines Tier-1 DDL rules.
// input: DDL statements plus policy-backed schema/table denylist entries
// output: findings that block DDL against protected schemas or tables
// pos: DDL object-scope denylist guardrails for audit completion
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"fmt"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type tableDenylistRule struct {
	ruleID          string
	level           rule.Level
	schemas         map[string]struct{}
	tables          map[string]struct{}
	qualifiedTables map[string]struct{}
}

func newTableDenylistRule(ruleID string, fallbackLevel rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	schemas, err := normalizedStringSetParam(ruleID, cfg, "schemas", nil)
	if err != nil {
		return nil, err
	}
	tables, err := normalizedStringSetParam(ruleID, cfg, "tables", nil)
	if err != nil {
		return nil, err
	}
	qualifiedTables, err := normalizedStringSetParam(ruleID, cfg, "qualified_tables", nil)
	if err != nil {
		return nil, err
	}
	return tableDenylistRule{
		ruleID:          ruleID,
		level:           configuredLevel(cfg, fallbackLevel),
		schemas:         schemas,
		tables:          tables,
		qualifiedTables: qualifiedTables,
	}, nil
}

func (r tableDenylistRule) ID() string { return r.ruleID }

func (r tableDenylistRule) AppliesTo(statement spec.Statement) bool {
	return statement.Kind == spec.KindDDL && statement.DDL != nil && len(statement.DDL.TableTargets()) > 0
}

func (r tableDenylistRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	metadataSchema := normalizedMetadataSchema(statement)
	seen := make(map[spec.Table]struct{})
	findings := make([]rule.Finding, 0)
	for _, table := range statement.DDL.TableTargets() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		tableName := strings.ToLower(strings.TrimSpace(table.Name))
		schemaName := strings.ToLower(strings.TrimSpace(table.Schema))
		if schemaName == "" {
			schemaName = metadataSchema
		}
		if !r.matches(schemaName, tableName) {
			continue
		}
		identity := spec.Table{Schema: schemaName, Name: tableName}
		if _, duplicate := seen[identity]; duplicate {
			continue
		}
		seen[identity] = struct{}{}
		target := qualifiedTableName(schemaName, tableName)
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    fmt.Sprintf("DDL target %q is blocked by the table denylist policy", target),
			Suggestion: "run the change against an allowed table or relax the denylist policy intentionally",
			Metadata: map[string]any{
				"schema": schemaName,
				"table":  tableName,
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
