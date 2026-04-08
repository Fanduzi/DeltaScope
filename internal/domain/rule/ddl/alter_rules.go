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
	ruleID           string
	action           string
	label            string
	level            rule.Level
	forbid           bool
	dialectAllowlist []spec.Dialect
}

type forbiddenAlterActionOption func(*forbiddenAlterActionRule)

func withDialectAllowlist(dialects ...spec.Dialect) forbiddenAlterActionOption {
	return func(r *forbiddenAlterActionRule) {
		r.dialectAllowlist = dialects
	}
}

func newForbiddenAlterActionRule(ruleID, action, label string, fallbackLevel rule.Level, cfg policy.RulePolicy, opts ...forbiddenAlterActionOption) (rule.StatementRule, error) {
	forbid, err := boolParam(ruleID, cfg, "forbid", true)
	if err != nil {
		return nil, err
	}
	r := forbiddenAlterActionRule{
		ruleID: ruleID,
		action: action,
		label:  label,
		level:  configuredLevel(cfg, fallbackLevel),
		forbid: forbid,
	}
	for _, opt := range opts {
		opt(&r)
	}
	return r, nil
}

func (r forbiddenAlterActionRule) ID() string { return r.ruleID }

func (r forbiddenAlterActionRule) AppliesTo(statement spec.Statement) bool {
	if !r.forbid || (!appliesToAlterTable(statement) && !appliesToStandaloneDDLAction(statement, r.action)) {
		return false
	}
	if len(r.dialectAllowlist) > 0 {
		for _, d := range r.dialectAllowlist {
			if statement.Dialect == d {
				return true
			}
		}
		return false
	}
	return true
}

func (r forbiddenAlterActionRule) Evaluate(statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	alters := matchingForbiddenAlterActions(statement, r.action)
	standalone := statement.DDL.Table == nil
	if standalone {
		alters = matchingStandaloneDDLActions(statement, r.action)
	}
	for _, alter := range alters {
		if alter.Action != r.action {
			continue
		}
		message := fmt.Sprintf("ALTER TABLE %s is forbidden", r.label)
		suggestion := fmt.Sprintf("avoid %s in this change or relax the policy intentionally", r.label)
		if alter.Name != "" {
			message = fmt.Sprintf("ALTER TABLE %s is forbidden for %q", r.label, alter.Name)
		}
		metadata := map[string]any{
			"action": alter.Action,
			"name":   alter.Name,
		}
		if !standalone {
			metadata["table"] = statement.DDL.Table.Name
		} else if alter.Name != "" {
			message = fmt.Sprintf("DDL %s is forbidden for %q", r.label, alter.Name)
		} else {
			message = fmt.Sprintf("DDL %s is forbidden", r.label)
		}
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    message,
			Suggestion: suggestion,
			Metadata:   metadata,
		})
	}
	return findings, nil
}

func matchingForbiddenAlterActions(statement spec.Statement, action string) []spec.Alter {
	if action == "drop_primary_key" {
		return matchingDropPrimaryKeyActions(statement)
	}
	return matchingAlterActions(statement, action)
}
