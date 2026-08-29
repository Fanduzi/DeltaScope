// Package dml defines Tier-1 DML rules.
// input: metadata-enriched MySQL/TiDB DML Statement specs and policy values
// output: blocker findings for definitively absent DML target tables
// pos: metadata-backed DML table-existence guardrail
// note: if this file changes, update this header and module README.md.
package dml

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type tableExistenceRule struct {
	level rule.Level
}

func newTableExistenceRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return tableExistenceRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r tableExistenceRule) ID() string { return ruleIDTableExistsRequire }

func (r tableExistenceRule) AppliesTo(statement spec.Statement) bool {
	return (statement.Dialect == spec.DialectMySQL || statement.Dialect == spec.DialectTiDB) &&
		statement.Kind == spec.KindDML &&
		statement.DML != nil &&
		(statement.DML.Operation == spec.DMLOperationInsert ||
			statement.DML.Operation == spec.DMLOperationUpdate ||
			statement.DML.Operation == spec.DMLOperationDelete) &&
		len(statement.DML.Tables) > 0
}

func (r tableExistenceRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(statement) || statement.Metadata == nil || statement.Metadata.TargetTable == nil || statement.Metadata.TargetTable.Exists {
		return nil, nil
	}

	table := statement.DML.Tables[0]
	return []rule.Finding{{
		Level:      r.level,
		Message:    fmt.Sprintf("table %q does not exist in the target schema", table.Name),
		Suggestion: "create the table first or run the audit without metadata mode if live schema checks are unavailable",
		Metadata: map[string]any{
			"table":  table.Name,
			"exists": false,
		},
	}}, nil
}
