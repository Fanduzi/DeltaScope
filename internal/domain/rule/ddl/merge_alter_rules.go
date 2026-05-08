// Package ddl defines Tier-1 DDL rules.
// input: statement batches plus dialect-specific merge-alter policy values
// output: global findings when repeated alter-table statements should be merged
// pos: DDL global-rule implementations for cross-statement alter governance
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type mergeAlterRule struct {
	ruleID  string
	dialect spec.Dialect
	level   rule.Level
}

func newMergeAlterRule(ruleID string, dialect spec.Dialect, fallbackLevel rule.Level, cfg policy.RulePolicy) (rule.GlobalRule, error) {
	required, err := boolParam(ruleID, cfg, "required", true)
	if err != nil {
		return nil, err
	}
	if !required {
		return mergeAlterRule{ruleID: ruleID}, nil
	}
	return mergeAlterRule{
		ruleID:  ruleID,
		dialect: dialect,
		level:   configuredLevel(cfg, fallbackLevel),
	}, nil
}

func (r mergeAlterRule) ID() string { return r.ruleID }

func (r mergeAlterRule) EvaluateAll(ctx context.Context, statements []spec.Statement) ([]rule.Finding, error) {
	if r.dialect == "" {
		return nil, nil
	}

	counts := make(map[string]int)
	for _, statement := range statements {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if statement.Kind != spec.KindDDL || statement.Dialect != r.dialect || !appliesToAlterTable(statement) {
			continue
		}
		if statement.DDL.Table == nil || statement.DDL.Table.Name == "" {
			continue
		}
		counts[statement.DDL.Table.Name]++
	}

	findings := make([]rule.Finding, 0)
	for table, count := range counts {
		if count < 2 {
			continue
		}
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    fmt.Sprintf("multiple ALTER TABLE statements target %q under %s mode", table, r.dialect),
			Suggestion: "merge repeated alter statements on the same table into a single ALTER TABLE when the dialect policy requires it",
			Metadata: map[string]any{
				"table":       table,
				"dialect":     r.dialect.String(),
				"alter_count": count,
			},
		})
	}

	return findings, nil
}
