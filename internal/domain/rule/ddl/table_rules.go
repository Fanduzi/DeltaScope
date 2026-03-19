// Package ddl defines Tier-1 DDL rules.
// input: create-table Statement specs and per-rule policy values
// output: table-level findings for missing comments and oversized names
// pos: DDL rule implementations for table naming and comments
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"fmt"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type tableCommentRequiredRule struct {
	required bool
	level    rule.Level
}

func newTableCommentRequiredRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	required, err := boolParam(ruleIDTableCommentRequired, cfg, "required", true)
	if err != nil {
		return nil, err
	}

	return tableCommentRequiredRule{
		required: required,
		level:    configuredLevel(cfg, rule.LevelWarning),
	}, nil
}

func (r tableCommentRequiredRule) ID() string {
	return ruleIDTableCommentRequired
}

func (r tableCommentRequiredRule) AppliesTo(statement spec.Statement) bool {
	return r.required && appliesToCreateTable(statement)
}

func (r tableCommentRequiredRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	if strings.TrimSpace(statement.DDL.Table.Comment) != "" {
		return nil, nil
	}

	return []rule.Finding{{
		Level:      r.level,
		Message:    "table comment is required",
		Suggestion: "add a COMMENT clause describing the table purpose",
		Metadata: map[string]any{
			"table": statement.DDL.Table.Name,
		},
	}}, nil
}

type tableNameMaxLengthRule struct {
	limit int
	level rule.Level
}

func newTableNameMaxLengthRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	limit, err := intParam(ruleIDTableNameMaxLength, cfg, "limit", 64)
	if err != nil {
		return nil, err
	}
	if limit < 1 {
		return nil, fmt.Errorf("rule %s param %q must be >= 1, got %d", ruleIDTableNameMaxLength, "limit", limit)
	}

	return tableNameMaxLengthRule{
		limit: limit,
		level: configuredLevel(cfg, rule.LevelBlocker),
	}, nil
}

func (r tableNameMaxLengthRule) ID() string {
	return ruleIDTableNameMaxLength
}

func (r tableNameMaxLengthRule) AppliesTo(statement spec.Statement) bool {
	return appliesToCreateTable(statement)
}

func (r tableNameMaxLengthRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	actual := len(statement.DDL.Table.Name)
	if actual <= r.limit {
		return nil, nil
	}

	return []rule.Finding{{
		Level:      r.level,
		Message:    fmt.Sprintf("table name must not exceed %d characters", r.limit),
		Suggestion: fmt.Sprintf("rename the table to fit within %d characters", r.limit),
		Metadata: map[string]any{
			"table":  statement.DDL.Table.Name,
			"limit":  r.limit,
			"actual": actual,
		},
	}}, nil
}
