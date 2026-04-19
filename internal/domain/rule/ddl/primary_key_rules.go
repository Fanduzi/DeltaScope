// Package ddl defines Tier-1 DDL rules.
// input: create-table Statement specs and per-rule policy values
// output: primary-key findings for missing or oversized primary-key definitions
// pos: DDL rule implementations for primary-key requirements and shape
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type primaryKeyRequiredRule struct {
	required bool
	level    rule.Level
}

func newPrimaryKeyRequiredRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	required, err := boolParam(ruleIDPrimaryKeyRequired, cfg, "required", true)
	if err != nil {
		return nil, err
	}

	return primaryKeyRequiredRule{
		required: required,
		level:    configuredLevel(cfg, rule.LevelBlocker),
	}, nil
}

func (r primaryKeyRequiredRule) ID() string {
	return ruleIDPrimaryKeyRequired
}

func (r primaryKeyRequiredRule) AppliesTo(statement spec.Statement) bool {
	return r.required && appliesToCreateTable(statement)
}

func (r primaryKeyRequiredRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	if statement.DDL.PrimaryKey != nil {
		return nil, nil
	}

	return []rule.Finding{{
		Level:      r.level,
		Message:    "primary key is required",
		Suggestion: "add an explicit PRIMARY KEY declaration",
		Metadata: map[string]any{
			"table": statement.DDL.Table.Name,
		},
	}}, nil
}

type primaryKeyColumnCountRule struct {
	limit int
	level rule.Level
}

func newPrimaryKeyColumnCountRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	limit, err := intParam(ruleIDPrimaryKeyColumnsMaxCount, cfg, "limit", 1)
	if err != nil {
		return nil, err
	}
	if limit < 1 {
		return nil, fmt.Errorf("rule %s param %q must be >= 1, got %d", ruleIDPrimaryKeyColumnsMaxCount, "limit", limit)
	}

	return primaryKeyColumnCountRule{
		limit: limit,
		level: configuredLevel(cfg, rule.LevelWarning),
	}, nil
}

func (r primaryKeyColumnCountRule) ID() string {
	return ruleIDPrimaryKeyColumnsMaxCount
}

func (r primaryKeyColumnCountRule) AppliesTo(statement spec.Statement) bool {
	return appliesToCreateTable(statement) || appliesToAlterAddConstraintPrimaryKey(statement)
}

func (r primaryKeyColumnCountRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	var actual int
	if statement.DDL.PrimaryKey != nil {
		actual = len(statement.DDL.PrimaryKey.Columns)
	} else {
		for _, alter := range statement.DDL.Alter {
			if alter.Action == "add_constraint" && alter.Options["constraint_type"] == "primary_key" {
				if cols := splitAlterConstraintColumns(alter.Options["columns"]); len(cols) > 0 {
					actual = len(cols)
				}
			}
		}
	}
	if actual == 0 || actual <= r.limit {
		return nil, nil
	}

	return []rule.Finding{{
		Level:      r.level,
		Message:    fmt.Sprintf("primary key must not contain more than %d columns", r.limit),
		Suggestion: fmt.Sprintf("reduce the PRIMARY KEY to %d column(s) or fewer", r.limit),
		Metadata: map[string]any{
			"table":  statement.DDL.Table.Name,
			"limit":  r.limit,
			"actual": actual,
		},
	}}, nil
}
