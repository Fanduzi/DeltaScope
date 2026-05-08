// Package dml defines Tier-1 DML rules.
// input: insert Statement specs and per-rule policy values
// output: insert findings for row-count, replace, insert-select, and on-duplicate checks
// pos: DML rule implementations for insert-family guardrails
// note: if this file changes, update this header and module README.md.
package dml

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type insertRowsMaxCountRule struct {
	limit int
	level rule.Level
}

func newInsertRowsMaxCountRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	limit, err := intParam(ruleIDInsertRowsMaxCount, cfg, "limit", 100)
	if err != nil {
		return nil, err
	}
	if limit < 1 {
		return nil, fmt.Errorf("rule %s param %q must be >= 1, got %d", ruleIDInsertRowsMaxCount, "limit", limit)
	}

	return insertRowsMaxCountRule{
		limit: limit,
		level: configuredLevel(cfg, rule.LevelWarning),
	}, nil
}

func (r insertRowsMaxCountRule) ID() string { return ruleIDInsertRowsMaxCount }

func (r insertRowsMaxCountRule) AppliesTo(statement spec.Statement) bool {
	return appliesToInsert(statement)
}

func (r insertRowsMaxCountRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(statement) || statement.DML.InsertRows == 0 || statement.DML.InsertRows <= r.limit {
		return nil, nil
	}

	return []rule.Finding{{
		Level:      r.level,
		Message:    fmt.Sprintf("INSERT statements must not include more than %d row(s)", r.limit),
		Suggestion: fmt.Sprintf("split the INSERT into batches of %d row(s) or fewer", r.limit),
		Metadata: map[string]any{
			"limit":  r.limit,
			"actual": statement.DML.InsertRows,
		},
	}}, nil
}

type replaceForbiddenRule struct {
	forbid bool
	level  rule.Level
}

func newReplaceForbiddenRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	forbid, err := boolParam(ruleIDReplaceForbid, cfg, "forbid", true)
	if err != nil {
		return nil, err
	}

	return replaceForbiddenRule{
		forbid: forbid,
		level:  configuredLevel(cfg, rule.LevelBlocker),
	}, nil
}

func (r replaceForbiddenRule) ID() string { return ruleIDReplaceForbid }

func (r replaceForbiddenRule) AppliesTo(statement spec.Statement) bool {
	return r.forbid && appliesToInsert(statement)
}

func (r replaceForbiddenRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(statement) || !statement.DML.IsReplace {
		return nil, nil
	}

	return []rule.Finding{{
		Level:      r.level,
		Message:    "REPLACE statements are forbidden",
		Suggestion: "use INSERT or UPDATE explicitly instead of REPLACE",
	}}, nil
}

type insertSelectForbiddenRule struct {
	forbid bool
	level  rule.Level
}

func newInsertSelectForbiddenRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	forbid, err := boolParam(ruleIDInsertSelectForbid, cfg, "forbid", true)
	if err != nil {
		return nil, err
	}

	return insertSelectForbiddenRule{
		forbid: forbid,
		level:  configuredLevel(cfg, rule.LevelBlocker),
	}, nil
}

func (r insertSelectForbiddenRule) ID() string { return ruleIDInsertSelectForbid }

func (r insertSelectForbiddenRule) AppliesTo(statement spec.Statement) bool {
	return r.forbid && appliesToInsert(statement)
}

func (r insertSelectForbiddenRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(statement) || !statement.DML.IsInsertSelect {
		return nil, nil
	}

	return []rule.Finding{{
		Level:      r.level,
		Message:    "INSERT ... SELECT statements are forbidden",
		Suggestion: "materialize the source rows before issuing INSERT statements",
	}}, nil
}

type onDuplicateForbiddenRule struct {
	forbid bool
	level  rule.Level
}

func newOnDuplicateForbiddenRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	forbid, err := boolParam(ruleIDOnDuplicateForbid, cfg, "forbid", true)
	if err != nil {
		return nil, err
	}

	return onDuplicateForbiddenRule{
		forbid: forbid,
		level:  configuredLevel(cfg, rule.LevelBlocker),
	}, nil
}

func (r onDuplicateForbiddenRule) ID() string { return ruleIDOnDuplicateForbid }

func (r onDuplicateForbiddenRule) AppliesTo(statement spec.Statement) bool {
	return r.forbid && appliesToInsert(statement)
}

func (r onDuplicateForbiddenRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !r.AppliesTo(statement) || !statement.DML.HasOnDuplicate {
		return nil, nil
	}

	return []rule.Finding{{
		Level:      r.level,
		Message:    "INSERT ... ON DUPLICATE KEY UPDATE is forbidden",
		Suggestion: "use a separate INSERT or UPDATE workflow instead of ON DUPLICATE KEY",
	}}, nil
}
