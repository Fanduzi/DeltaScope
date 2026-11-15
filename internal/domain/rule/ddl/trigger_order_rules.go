package ddl

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// trigger_order_rules.go — FOLLOWS/PRECEDES ordering is a MySQL 5.7.2+
// feature (WL#3253): multiple triggers per event arrived there, so the
// ordering clause cannot exist earlier. The rule is a determined
// negative on a known-incompatible version and an evidence gap when the
// target version is unproven — never a silent pass (T29/#108).

type triggerOrderVersionRule struct {
	ruleID   string
	required bool
	level    rule.Level
}

func newTriggerOrderVersionRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	required, err := boolParam(ruleIDCreateTriggerOrderVersionRequire, cfg, "required", true)
	if err != nil {
		return nil, err
	}
	return triggerOrderVersionRule{
		ruleID:   ruleIDCreateTriggerOrderVersionRequire,
		required: required,
		level:    configuredLevel(cfg, rule.LevelBlocker),
	}, nil
}

func (r triggerOrderVersionRule) ID() string { return r.ruleID }

func (r triggerOrderVersionRule) AppliesTo(statement spec.Statement) bool {
	return r.required && statement.Dialect == spec.DialectMySQL &&
		statement.DDL != nil && statement.DDL.Trigger != nil &&
		statement.DDL.Trigger.Kind == spec.TriggerKindCreate &&
		statement.DDL.Trigger.Order != nil
}

func (r triggerOrderVersionRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	version := statementVersionIdentity(statement)
	support := spec.TriggerOrderVersionSupportFor(version)
	if !support.Incompatible || version == nil {
		return nil, nil
	}
	order := statement.DDL.Trigger.Order
	return []rule.Finding{{
		Level:      r.level,
		Message:    fmt.Sprintf("FOLLOWS/PRECEDES trigger ordering requires MySQL %s or newer; %s is not supported", spec.TriggerOrderMinimumSupportedVersion, version.Version),
		Suggestion: "drop the ordering clause or retarget the audit at MySQL 5.7.2 or newer",
		Metadata: map[string]any{
			"operation":                 "create_trigger",
			"ordering":                  string(order.Kind),
			"sibling_trigger":           order.Name,
			"product":                   version.Product,
			"target_version":            version.Version,
			"minimum_supported_version": spec.TriggerOrderMinimumSupportedVersion,
		},
	}}, nil
}

func (r triggerOrderVersionRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	support := spec.TriggerOrderVersionSupportFor(statementVersionIdentity(statement))
	if support.GapReason == "" {
		return nil
	}
	return []rule.EvidenceGap{{
		ReasonCode:    support.GapReason,
		RequiredFacts: append([]string(nil), support.GapFacts...),
	}}
}
