// Package dml defines Tier-1 DML rules.
// input: update/delete Statement specs and per-rule policy values
// output: mutation findings for WHERE, LIMIT, ORDER BY, subquery, and JOIN ... ON checks
// pos: DML rule implementations for update/delete guardrails
// note: if this file changes, update this header and module README.md.
package dml

import (
	"context"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type whereRequiredRule struct {
	required bool
	level    rule.Level
}

func newWhereRequiredRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	required, err := boolParam(ruleIDWhereRequire, cfg, "required", true)
	if err != nil {
		return nil, err
	}

	return whereRequiredRule{
		required: required,
		level:    configuredLevel(cfg, rule.LevelBlocker),
	}, nil
}

func (r whereRequiredRule) ID() string { return ruleIDWhereRequire }

func (r whereRequiredRule) AppliesTo(statement spec.Statement) bool {
	return r.required && appliesToMutation(statement)
}

func (r whereRequiredRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil { return nil, err }
	if !r.AppliesTo(statement) || statement.DML.HasWhere {
		return nil, nil
	}

	return []rule.Finding{{
		Level:      r.level,
		Message:    "UPDATE and DELETE statements must include a WHERE clause",
		Suggestion: "add a WHERE clause that narrows the affected rows",
		Metadata: map[string]any{
			"operation": statement.DML.Operation,
		},
	}}, nil
}

type limitForbiddenRule struct {
	forbid bool
	level  rule.Level
}

func newLimitForbiddenRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	forbid, err := boolParam(ruleIDLimitForbid, cfg, "forbid", true)
	if err != nil {
		return nil, err
	}

	return limitForbiddenRule{
		forbid: forbid,
		level:  configuredLevel(cfg, rule.LevelWarning),
	}, nil
}

func (r limitForbiddenRule) ID() string { return ruleIDLimitForbid }

func (r limitForbiddenRule) AppliesTo(statement spec.Statement) bool {
	return r.forbid && appliesToMutation(statement)
}

func (r limitForbiddenRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil { return nil, err }
	if !r.AppliesTo(statement) || !statement.DML.HasLimit {
		return nil, nil
	}

	return []rule.Finding{{
		Level:      r.level,
		Message:    "UPDATE and DELETE statements must not include LIMIT",
		Suggestion: "remove LIMIT and use a precise WHERE clause instead",
		Metadata: map[string]any{
			"operation": statement.DML.Operation,
		},
	}}, nil
}

type orderByForbiddenRule struct {
	forbid bool
	level  rule.Level
}

func newOrderByForbiddenRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	forbid, err := boolParam(ruleIDOrderByForbid, cfg, "forbid", true)
	if err != nil {
		return nil, err
	}

	return orderByForbiddenRule{
		forbid: forbid,
		level:  configuredLevel(cfg, rule.LevelWarning),
	}, nil
}

func (r orderByForbiddenRule) ID() string { return ruleIDOrderByForbid }

func (r orderByForbiddenRule) AppliesTo(statement spec.Statement) bool {
	return r.forbid && appliesToMutation(statement)
}

func (r orderByForbiddenRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil { return nil, err }
	if !r.AppliesTo(statement) || !statement.DML.HasOrderBy {
		return nil, nil
	}

	return []rule.Finding{{
		Level:      r.level,
		Message:    "UPDATE and DELETE statements must not include ORDER BY",
		Suggestion: "remove ORDER BY and rely on a precise WHERE clause instead",
		Metadata: map[string]any{
			"operation": statement.DML.Operation,
		},
	}}, nil
}

type subqueryForbiddenRule struct {
	forbid bool
	level  rule.Level
}

func newSubqueryForbiddenRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	forbid, err := boolParam(ruleIDSubqueryForbid, cfg, "forbid", true)
	if err != nil {
		return nil, err
	}

	return subqueryForbiddenRule{
		forbid: forbid,
		level:  configuredLevel(cfg, rule.LevelBlocker),
	}, nil
}

func (r subqueryForbiddenRule) ID() string { return ruleIDSubqueryForbid }

func (r subqueryForbiddenRule) AppliesTo(statement spec.Statement) bool {
	return r.forbid && appliesToMutation(statement)
}

func (r subqueryForbiddenRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil { return nil, err }
	if !r.AppliesTo(statement) || !statement.DML.HasSubquery {
		return nil, nil
	}

	return []rule.Finding{{
		Level:      r.level,
		Message:    "UPDATE and DELETE statements must not include subqueries",
		Suggestion: "rewrite the statement to avoid nested SELECTs in DML",
		Metadata: map[string]any{
			"operation": statement.DML.Operation,
		},
	}}, nil
}

type joinOnRequiredRule struct {
	required bool
	level    rule.Level
}

func newJoinOnRequiredRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	required, err := boolParam(ruleIDJoinOnRequire, cfg, "required", true)
	if err != nil {
		return nil, err
	}

	return joinOnRequiredRule{
		required: required,
		level:    configuredLevel(cfg, rule.LevelBlocker),
	}, nil
}

func (r joinOnRequiredRule) ID() string { return ruleIDJoinOnRequire }

func (r joinOnRequiredRule) AppliesTo(statement spec.Statement) bool {
	return r.required && appliesToMutation(statement)
}

func (r joinOnRequiredRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if err := ctx.Err(); err != nil { return nil, err }
	if !r.AppliesTo(statement) || !statement.DML.HasJoin || statement.DML.HasJoinOn {
		return nil, nil
	}

	return []rule.Finding{{
		Level:      r.level,
		Message:    "UPDATE and DELETE joins must include an ON clause",
		Suggestion: "add an ON clause for every joined table in the DML statement",
		Metadata: map[string]any{
			"operation": statement.DML.Operation,
		},
	}}, nil
}
