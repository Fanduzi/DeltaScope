// Package ddl defines Tier-1 DDL rules.
// input: alter-table Statement specs with normalized alter actions and per-rule policy values
// output: findings for forbidden ALTER TABLE actions
// pos: DDL rule implementations for action-level alter restrictions
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type forbiddenAlterActionRule struct {
	ruleID string
	action string
	label  string
	level  rule.Level
	forbid bool
}

func newForbiddenAlterActionRule(ruleID, action, label string, fallbackLevel rule.Level, cfg policy.RulePolicy) (rule.StatementRule, error) {
	forbid, err := boolParam(ruleID, cfg, "forbid", true)
	if err != nil {
		return nil, err
	}
	return forbiddenAlterActionRule{
		ruleID: ruleID,
		action: action,
		label:  label,
		level:  configuredLevel(cfg, fallbackLevel),
		forbid: forbid,
	}, nil
}

func (r forbiddenAlterActionRule) ID() string { return r.ruleID }

func (r forbiddenAlterActionRule) AppliesTo(statement spec.Statement) bool {
	return r.forbid && appliesToAlterTable(statement)
}

func (r forbiddenAlterActionRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	for _, alter := range statement.DDL.Alter {
		if alter.Action != r.action {
			continue
		}
		message := fmt.Sprintf("ALTER TABLE %s is forbidden", r.label)
		suggestion := fmt.Sprintf("avoid %s in this change or relax the policy intentionally", r.label)
		if alter.Name != "" {
			message = fmt.Sprintf("ALTER TABLE %s is forbidden for %q", r.label, alter.Name)
		}
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    message,
			Suggestion: suggestion,
			Metadata: map[string]any{
				"table":  statement.DDL.Table.Name,
				"action": alter.Action,
				"name":   alter.Name,
			},
		})
	}
	return findings, nil
}
