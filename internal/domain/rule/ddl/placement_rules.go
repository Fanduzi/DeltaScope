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

// tidbPlacementWarnRule is the TiDB placement-policy binding risk rule.
// An authored PLACEMENT POLICY clause that provably lands produces a
// bounded warning — the binding reassigns the object's region topology.
// Refused premises (missing policy 8239, bound-policy drop 8241,
// unsupported range 1105, MySQL vendor boundaries) stay silent: nothing
// landed. Unproven premises surface evidence gaps, never findings.
// Findings only ever carry bounded identities — policy name, target
// kind/name, action — never constraint bodies or region configuration.
type tidbPlacementWarnRule struct {
	level rule.Level
}

func newTiDBPlacementWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return tidbPlacementWarnRule{level: configuredLevel(cfg, rule.LevelWarning)}, nil
}

func (r tidbPlacementWarnRule) ID() string { return ruleIDTiDBPlacementWarn }

func (r tidbPlacementWarnRule) AppliesTo(statement spec.Statement) bool {
	if statement.Dialect != spec.DialectTiDB || statement.Kind != spec.KindDDL || statement.DDL == nil {
		return false
	}
	ddl := statement.DDL
	switch ddl.Operation {
	case spec.DDLOperationCreatePlacementPolicy,
		spec.DDLOperationAlterPlacementPolicy,
		spec.DDLOperationDropPlacementPolicy,
		spec.DDLOperationAlterRangePlacement:
		return true
	}
	if ddl.PlacementBinding != nil {
		return true
	}
	for i := range ddl.Alter {
		if ddl.Alter[i].Action == "placement_policy" && ddl.Alter[i].Placement != nil {
			return true
		}
	}
	for i := range ddl.PartitionDefs {
		if ddl.PartitionDefs[i].Placement != nil && !ddl.PartitionDefs[i].Placement.Default {
			return true
		}
	}
	return false
}

func (r tidbPlacementWarnRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
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
	metadata := map[string]any{
		"policy": outcome.policy,
		"action": outcome.action,
	}
	if outcome.target != "" {
		metadata["target"] = outcome.target
	}
	if outcome.targetKind != "" {
		metadata["target_kind"] = outcome.targetKind
	}
	return []rule.Finding{{
		RuleID:     r.ID(),
		Level:      r.level,
		Message:    outcome.message,
		Suggestion: "verify the placement policy's region constraints, replica counts, and PD/TiKV topology prerequisites intentionally — a bound policy reassigns the object's physical placement",
		Metadata:   metadata,
	}}, nil
}

func (r tidbPlacementWarnRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
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

type placementRuleOutcome struct {
	warning    bool
	message    string
	policy     string
	action     string
	target     string
	targetKind string
	facts      []string
}

func (r tidbPlacementWarnRule) outcome(statement spec.Statement) placementRuleOutcome {
	ddl := statement.DDL
	if ddl == nil {
		return placementRuleOutcome{}
	}
	policies := placementPolicyFacts(statement)
	switch ddl.Operation {
	case spec.DDLOperationCreatePlacementPolicy:
		exists := placementPolicyExists(policies, ddl.ObjectName)
		switch spec.PlacementCreateOutcome(statement.Dialect,
			ddl.Options["or_replace"] == "true", ddl.Options["if_not_exists"] == "true", exists) {
		case spec.PlacementApplied:
			return placementRuleOutcome{
				warning: true,
				policy:  strings.ToLower(ddl.ObjectName),
				action:  "create_placement_policy",
				message: fmt.Sprintf("CREATE PLACEMENT POLICY %q declares a topology policy objects can bind to", ddl.ObjectName),
			}
		case spec.PlacementUnproven:
			return placementRuleOutcome{facts: []string{"placement_policy.catalog"}}
		}
	case spec.DDLOperationAlterPlacementPolicy:
		exists := placementPolicyExists(policies, ddl.ObjectName)
		switch spec.PlacementAlterOutcome(statement.Dialect, exists) {
		case spec.PlacementApplied:
			return placementRuleOutcome{
				warning: true,
				policy:  strings.ToLower(ddl.ObjectName),
				action:  "alter_placement_policy",
				message: fmt.Sprintf("ALTER PLACEMENT POLICY %q changes the topology contract of every bound object", ddl.ObjectName),
			}
		case spec.PlacementUnproven:
			return placementRuleOutcome{facts: []string{"placement_policy.catalog"}}
		}
	case spec.DDLOperationDropPlacementPolicy:
		exists := placementPolicyExists(policies, ddl.ObjectName)
		bound := placementBoundTargets(policies, ddl.ObjectName)
		switch spec.PlacementDropOutcome(statement.Dialect,
			ddl.Options["if_exists"] == "true", exists, bound) {
		case spec.PlacementApplied:
			return placementRuleOutcome{
				warning: true,
				policy:  strings.ToLower(ddl.ObjectName),
				action:  "drop_placement_policy",
				message: fmt.Sprintf("DROP PLACEMENT POLICY %q removes a topology policy", ddl.ObjectName),
			}
		case spec.PlacementUnproven:
			return placementRuleOutcome{facts: []string{"placement_policy.catalog", "placement_policy.bound_targets"}}
		}
	case spec.DDLOperationAlterRangePlacement:
		if spec.PlacementRangeOutcome(statement.Dialect, ddl.PlacementRange) != spec.PlacementApplied {
			return placementRuleOutcome{}
		}
		return r.bindOutcome(statement, ddl.PlacementBinding, policies, "range:"+strings.ToLower(ddl.PlacementRange), "range", "alter_range_placement")
	default:
		if ddl.PlacementBinding != nil {
			kind := "table"
			target := ""
			if ddl.Table != nil {
				target = ddl.Table.Name
			}
			if ddl.Operation == spec.DDLOperationCreateSchema || ddl.Operation == spec.DDLOperationAlterSchema {
				kind = "schema"
				target = ddl.ObjectName
			}
			return r.bindOutcome(statement, ddl.PlacementBinding, policies, target, kind, strings.ToLower(string(ddl.Operation)))
		}
		for i := range ddl.Alter {
			alter := &ddl.Alter[i]
			if alter.Action != "placement_policy" || alter.Placement == nil {
				continue
			}
			target := ""
			if ddl.Table != nil {
				target = ddl.Table.Name
			}
			kind := "table"
			if len(alter.Partitions) > 0 {
				kind = "partition"
			}
			return r.bindOutcome(statement, alter.Placement, policies, target, kind, "placement_policy")
		}
		for i := range ddl.PartitionDefs {
			def := &ddl.PartitionDefs[i]
			if def.Placement != nil && !def.Placement.Default {
				target := ""
				if ddl.Table != nil {
					target = ddl.Table.Name
				}
				return r.bindOutcome(statement, def.Placement, policies, target, "partition", "create_table")
			}
		}
	}
	return placementRuleOutcome{}
}

// bindOutcome adjudicates one authored assignment through the shared
// tri-state outcome against the same catalog fact the ordered state
// consumed. DEFAULT unbinds and missing-policy refusals stay silent —
// nothing risky lands. A proven named bind warns; an unproven premise
// reports the catalog gap.
func (r tidbPlacementWarnRule) bindOutcome(statement spec.Statement, binding *spec.PlacementBinding, policies map[string]*spec.PlacementPolicyFact, target, targetKind, action string) placementRuleOutcome {
	if binding == nil || binding.Default {
		return placementRuleOutcome{}
	}
	exists := placementPolicyExists(policies, binding.Policy)
	switch spec.PlacementBindOutcome(statement.Dialect, *binding, exists) {
	case spec.PlacementApplied:
		return placementRuleOutcome{
			warning:    true,
			policy:     binding.Policy,
			action:     action,
			target:     target,
			targetKind: targetKind,
			message:    fmt.Sprintf("PLACEMENT POLICY %q binds %s %q to a topology policy", binding.Policy, targetKind, target),
		}
	case spec.PlacementUnproven:
		return placementRuleOutcome{facts: []string{"placement_policy.catalog"}}
	default:
		return placementRuleOutcome{}
	}
}

// placementPolicyFacts reads the ordered pre-state catalog the enrich pass
// attached. A nil map is the unvouched answer — no existence claim either
// way.
func placementPolicyFacts(statement spec.Statement) map[string]*spec.PlacementPolicyFact {
	if statement.Metadata == nil {
		return nil
	}
	return statement.Metadata.PlacementPolicies
}

func placementPolicyExists(policies map[string]*spec.PlacementPolicyFact, name string) *bool {
	if policies == nil {
		return nil
	}
	name = strings.ToLower(strings.TrimSpace(name))
	_, ok := policies[name]
	return &ok
}

// placementBoundTargets answers the proven inbound-reference list for one
// policy, or nil while the bound set is unvouched (policy absent from the
// catalog or the fact itself carries no proven set).
func placementBoundTargets(policies map[string]*spec.PlacementPolicyFact, name string) *[]string {
	if policies == nil {
		return nil
	}
	fact := policies[strings.ToLower(strings.TrimSpace(name))]
	if fact == nil || fact.BoundTargets == nil {
		if fact == nil {
			empty := []string{}
			return &empty
		}
		return nil
	}
	return &fact.BoundTargets
}
