// Package ddl defines Tier-1 DDL rules.
// input: create-table Statement specs with primary-key declarations and enriched column metadata
// output: findings for primary-key semantic requirements beyond simple presence/count checks
// pos: DDL rule implementations for primary-key semantic governance
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type singlePrimaryKeyColumnRule struct {
	ruleID    string
	level     rule.Level
	required  bool
	predicate func(spec.Column) bool
	message   string
	suggest   string
}

func newSinglePrimaryKeyColumnRule(ruleID string, fallbackLevel rule.Level, message, suggest string, predicate func(spec.Column) bool, cfg policy.RulePolicy) (rule.StatementRule, error) {
	required, err := boolParam(ruleID, cfg, "required", true)
	if err != nil {
		return nil, err
	}
	return singlePrimaryKeyColumnRule{
		ruleID:    ruleID,
		level:     configuredLevel(cfg, fallbackLevel),
		required:  required,
		predicate: predicate,
		message:   message,
		suggest:   suggest,
	}, nil
}

func (r singlePrimaryKeyColumnRule) ID() string { return r.ruleID }

func (r singlePrimaryKeyColumnRule) AppliesTo(statement spec.Statement) bool {
	return r.required && (appliesToCreateTable(statement) && statement.DDL.PrimaryKey != nil ||
		appliesToAlterAddConstraintPrimaryKey(statement))
}

func (r singlePrimaryKeyColumnRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	columns := primaryKeyColumnSpecs(statement)
	if len(columns) != 1 {
		return nil, nil
	}
	if r.predicate(columns[0]) {
		return nil, nil
	}
	return []rule.Finding{{
		Level:      r.level,
		Message:    fmt.Sprintf("primary key column %q %s", columns[0].Name, r.message),
		Suggestion: r.suggest,
		Metadata: map[string]any{
			"table":  statement.DDL.Table.Name,
			"column": columns[0].Name,
			"type":   columns[0].Type,
		},
	}}, nil
}

type primaryKeyNotNullRule struct {
	required bool
	level    rule.Level
}

func newPrimaryKeyNotNullRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	required, err := boolParam(ruleIDPrimaryKeyNotNullRequire, cfg, "required", true)
	if err != nil {
		return nil, err
	}
	return primaryKeyNotNullRule{required: required, level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r primaryKeyNotNullRule) ID() string { return ruleIDPrimaryKeyNotNullRequire }

func (r primaryKeyNotNullRule) AppliesTo(statement spec.Statement) bool {
	return r.required && appliesToCreateTable(statement) && statement.DDL.PrimaryKey != nil
}

func (r primaryKeyNotNullRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	findings := make([]rule.Finding, 0)
	for _, column := range primaryKeyColumnSpecs(statement) {
		if column.NotNull {
			continue
		}
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    fmt.Sprintf("primary key column %q must be NOT NULL", column.Name),
			Suggestion: "add an explicit NOT NULL constraint to the primary key column",
			Metadata: map[string]any{
				"table":  statement.DDL.Table.Name,
				"column": column.Name,
			},
		})
	}
	return findings, nil
}
