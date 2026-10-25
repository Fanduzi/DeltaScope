// Package ddl defines Tier-1 DDL rules.
package ddl

import (
	"context"
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

func (r forbiddenAlterActionRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}

	findings := make([]rule.Finding, 0)
	// Standalone index operations carry the target table too — standalone is
	// an operation property, not the absence of DDL.Table. The standalone
	// branch resolves names from Index.Definition/OldName when alter.Name is
	// empty.
	standalone := !appliesToAlterTable(statement)
	alters := matchingForbiddenAlterActions(statement, r.action)
	if standalone {
		alters = matchingStandaloneDDLActions(statement, r.action)
	}
	for _, alter := range alters {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if alter.Action != r.action {
			continue
		}
		name := alterIndexSubjectName(alter)
		if name == "" {
			name = alterIndexTargetName(alter)
		}
		message := fmt.Sprintf("ALTER TABLE %s is forbidden", r.label)
		suggestion := fmt.Sprintf("avoid %s in this change or relax the policy intentionally", r.label)
		if !standalone && name != "" {
			message = fmt.Sprintf("ALTER TABLE %s is forbidden for %q", r.label, name)
		}
		metadata := map[string]any{
			"action": alter.Action,
			"name":   name,
		}
		if statement.DDL.Table != nil && statement.DDL.Table.Name != "" {
			metadata["table"] = statement.DDL.Table.Name
		}
		if standalone {
			if name != "" {
				message = fmt.Sprintf("DDL %s is forbidden for %q", r.label, name)
			} else {
				message = fmt.Sprintf("DDL %s is forbidden", r.label)
			}
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
