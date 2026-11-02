// Package ddl defines Tier-1 DDL rules.
package ddl

import (
	"context"
	"fmt"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// tidbTTLWarnRule is the TiDB automatic-deletion risk rule. Authored TTL
// syntax alone is not enough: OFF definitions, REMOVE TTL, atomic TiDB
// refusals, and proven no-TTL member mutations must not be narrated as
// enabling cleanup. Prior state is consulted only where the resulting
// enabled bit depends on it.
type tidbTTLWarnRule struct {
	level rule.Level
}

func newTiDBTTLWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return tidbTTLWarnRule{level: configuredLevel(cfg, rule.LevelWarning)}, nil
}

func (r tidbTTLWarnRule) ID() string { return ruleIDTiDBTTLWarn }

func (r tidbTTLWarnRule) AppliesTo(statement spec.Statement) bool {
	if statement.Dialect != spec.DialectTiDB || statement.Kind != spec.KindDDL || statement.DDL == nil {
		return false
	}
	if len(statement.DDL.TTL) > 0 {
		return true
	}
	for _, alter := range statement.DDL.Alter {
		if len(alter.TTL) > 0 || alter.Action == "remove_ttl" {
			return true
		}
	}
	return false
}

func (r tidbTTLWarnRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	outcome := r.outcome(statement)
	if !outcome.warning {
		return nil, nil
	}
	target := primaryTarget(statement)
	message := fmt.Sprintf("TTL on table %q enables automatic deletion", target.Name)
	if outcome.modifyEnabled {
		message = fmt.Sprintf("TTL change on table %q modifies an enabled automatic deletion schedule", target.Name)
	}
	return []rule.Finding{{
		RuleID:     r.ID(),
		Level:      r.level,
		Message:    message,
		Suggestion: "verify TTL cleanup cadence, retention interval, and operational safeguards intentionally",
		Metadata: map[string]any{
			"table":          target.Name,
			"ttl":            true,
			"enabled":        true,
			"modify_enabled": outcome.modifyEnabled,
		},
	}}, nil
}

func (r tidbTTLWarnRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	outcome := r.outcome(statement)
	if len(outcome.facts) == 0 {
		return nil
	}
	return []rule.EvidenceGap{{
		ReasonCode:    gapReasonUnknownTableState,
		RequiredFacts: outcome.facts,
	}}
}

type ttlRuleOutcome struct {
	warning       bool
	modifyEnabled bool
	facts         []string
}

func (r tidbTTLWarnRule) outcome(statement spec.Statement) ttlRuleOutcome {
	ddl := statement.DDL
	if ddl == nil {
		return ttlRuleOutcome{}
	}
	if len(ddl.TTL) > 0 {
		return createTTLOutcome(statement)
	}
	if len(ddl.Alter) != 1 {
		// TiDB refuses TTL combined with any other schema action atomically.
		return ttlRuleOutcome{}
	}
	alter := ddl.Alter[0]
	if alter.Action == "remove_ttl" {
		return ttlRuleOutcome{}
	}
	if len(alter.TTL) != 1 {
		return ttlRuleOutcome{}
	}
	return alterTTLOutcome(statement, alter.TTL[0])
}

func createTTLOutcome(statement spec.Statement) ttlRuleOutcome {
	ddl := statement.DDL
	var definition *spec.TTLDeclaration
	enabled := true
	count := 0
	for i := range ddl.TTL {
		declaration := &ddl.TTL[i]
		if len(declaration.Unmodeled) > 0 || declaration.Source == spec.TTLClauseSourceUnproven {
			return ttlRuleOutcome{}
		}
		switch declaration.Member {
		case "ttl":
			definition = declaration
			count++
		case "ttl_enable":
			if declaration.Enabled == nil {
				return ttlRuleOutcome{}
			}
			enabled = *declaration.Enabled
		case "ttl_job_interval":
			if declaration.JobInterval == "" {
				return ttlRuleOutcome{}
			}
		default:
			return ttlRuleOutcome{}
		}
	}
	if definition == nil || count != 1 || !enabled {
		return ttlRuleOutcome{}
	}
	var facts *spec.InstanceFacts
	var likeSource *spec.TableSnapshot
	effectiveSchema := ""
	if statement.Metadata != nil {
		facts = statement.Metadata.Instance
		likeSource = statement.Metadata.SourceTable
		effectiveSchema = statement.Metadata.Schema
	}
	if ddl.Table != nil && strings.TrimSpace(ddl.Table.Schema) != "" {
		effectiveSchema = ddl.Table.Schema
	}
	switch spec.TTLCreateOutcome(statement.Dialect, ddl, effectiveSchema, facts, likeSource) {
	case spec.TTLDefinitionApplied:
		return ttlRuleOutcome{warning: true}
	case spec.TTLDefinitionUnproven:
		return ttlRuleOutcome{facts: []string{"target_table.ttl"}}
	default:
		return ttlRuleOutcome{}
	}
}

func alterTTLOutcome(statement spec.Statement, declaration spec.TTLDeclaration) ttlRuleOutcome {
	if len(declaration.Unmodeled) > 0 || declaration.Source == spec.TTLClauseSourceUnproven {
		return ttlRuleOutcome{}
	}
	snapshot, facts := ttlBoundSnapshot(statement)
	if len(facts) > 0 {
		return ttlRuleOutcome{facts: facts}
	}
	if snapshot == nil || !snapshot.Exists || !snapshot.TTLKnown {
		return ttlRuleOutcome{facts: []string{"target_table.ttl"}}
	}
	prior := snapshot.TTL
	switch declaration.Member {
	case "ttl":
		switch spec.TTLDefinitionLandingOutcome(snapshot, declaration) {
		case spec.TTLDefinitionRefused:
			return ttlRuleOutcome{}
		case spec.TTLDefinitionUnproven:
			return ttlRuleOutcome{facts: []string{"target_table.ttl"}}
		}
		if prior == nil {
			return ttlRuleOutcome{warning: true}
		}
		if prior.Enabled == nil {
			return ttlRuleOutcome{facts: []string{"target_table.ttl_enable"}}
		}
		return ttlRuleOutcome{warning: *prior.Enabled, modifyEnabled: *prior.Enabled}
	case "ttl_enable":
		if declaration.Enabled == nil || prior == nil || !*declaration.Enabled {
			return ttlRuleOutcome{}
		}
		return ttlRuleOutcome{warning: true}
	case "ttl_job_interval":
		if prior == nil || prior.Enabled == nil {
			return ttlRuleOutcome{facts: []string{"target_table.ttl_enable"}}
		}
		return ttlRuleOutcome{warning: *prior.Enabled, modifyEnabled: *prior.Enabled}
	default:
		return ttlRuleOutcome{}
	}
}

func ttlBoundSnapshot(statement spec.Statement) (*spec.TableSnapshot, []string) {
	bindings := boundTargetBindings(statement)
	if len(bindings) != 1 || bindings[0].Ambiguous || bindings[0].Snapshot == nil {
		return nil, []string{"target_table.ttl"}
	}
	return bindings[0].Snapshot, nil
}
