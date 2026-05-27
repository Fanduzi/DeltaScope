package ddl

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type alterActionNoticeRule struct {
	ruleID  string
	level   rule.Level
	action  string
	label   string
	message string
}

func newAlterActionNoticeRule(ruleID, action, label string, level rule.Level, message string, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return alterActionNoticeRule{
		ruleID:  ruleID,
		level:   configuredLevel(cfg, level),
		action:  action,
		label:   label,
		message: message,
	}, nil
}

func (r alterActionNoticeRule) ID() string { return r.ruleID }

func (r alterActionNoticeRule) AppliesTo(statement spec.Statement) bool {
	if statement.Dialect != spec.DialectMySQL && statement.Dialect != spec.DialectTiDB {
		return false
	}
	return appliesToAlterActions(statement, r.action)
}

func (r alterActionNoticeRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	alters := matchingAlterActions(statement, r.action)
	findings := make([]rule.Finding, 0, len(alters))
	for _, alter := range alters {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		message := fmt.Sprintf(r.message, statement.DDL.Table.Name)
		if alter.Name != "" {
			message = fmt.Sprintf("%s for %q", message, alter.Name)
		}
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    message,
			Suggestion: fmt.Sprintf("review %s before applying", r.label),
			Metadata: map[string]any{
				"action": r.action,
				"table":  statement.DDL.Table.Name,
				"name":   alter.Name,
			},
		})
	}
	return findings, nil
}

func newAlterAddColumnNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newAlterActionNoticeRule(
		ruleIDAlterAddColumnNotice, "add_columns", "add column", rule.LevelNotice,
		"ALTER TABLE %s ADD COLUMN adds a new column",
		cfg,
	)
}

func newAlterAddConstraintNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newAlterActionNoticeRule(
		ruleIDAlterAddConstraintNotice, "add_constraint", "add constraint", rule.LevelNotice,
		"ALTER TABLE %s ADD CONSTRAINT adds a new constraint",
		cfg,
	)
}

func newAlterDropColumnNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newAlterActionNoticeRule(
		ruleIDAlterDropColumnNotice, "drop_column", "drop column", rule.LevelNotice,
		"ALTER TABLE %s DROP COLUMN removes an existing column",
		cfg,
	)
}

func newAlterModifyColumnNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newAlterActionNoticeRule(
		ruleIDAlterModifyColumnNotice, "modify_column", "modify column", rule.LevelNotice,
		"ALTER TABLE %s MODIFY COLUMN changes a column definition",
		cfg,
	)
}

func newAlterDropIndexNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newAlterActionNoticeRule(
		ruleIDAlterDropIndexNotice, "drop_index", "drop index", rule.LevelNotice,
		"ALTER TABLE %s DROP INDEX removes an existing index",
		cfg,
	)
}

func newAlterDropForeignKeyNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newAlterActionNoticeRule(
		ruleIDAlterDropForeignKeyNotice, "drop_foreign_key", "drop foreign key", rule.LevelNotice,
		"ALTER TABLE %s DROP FOREIGN KEY removes a referential constraint",
		cfg,
	)
}

// tidbOnlyAlterActionRule wraps alterActionNoticeRule restricting to TiDB dialect.
type tidbOnlyAlterActionRule struct {
	alterActionNoticeRule
}

func (r tidbOnlyAlterActionRule) AppliesTo(statement spec.Statement) bool {
	if statement.Dialect != spec.DialectTiDB {
		return false
	}
	return appliesToAlterActions(statement, r.action)
}

func newTiDBAlterTablePlacementPolicyNoticeRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	base, err := newAlterActionNoticeRule(
		ruleIDTiDBAlterTablePlacementPolicyNotice, "placement_policy", "placement policy assignment", rule.LevelNotice,
		"ALTER TABLE %s PLACEMENT POLICY assigns a placement policy",
		cfg,
	)
	if err != nil {
		return nil, err
	}
	return tidbOnlyAlterActionRule{alterActionNoticeRule: base.(alterActionNoticeRule)}, nil
}
