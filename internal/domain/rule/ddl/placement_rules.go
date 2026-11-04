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
	return appliesToTiDBPlacementSurface(statement)
}

// appliesToTiDBPlacementSurface is the shared statement filter for TiDB
// placement checks: policy lifecycle operations, ALTER RANGE placement,
// and any authored named-or-default binding clause.
func appliesToTiDBPlacementSurface(statement spec.Statement) bool {
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
	var gaps []rule.EvidenceGap
	if len(outcome.facts) > 0 {
		gaps = append(gaps, rule.EvidenceGap{
			ReasonCode:    gapReasonUnknownTableState,
			RequiredFacts: outcome.facts,
		})
	}
	// A proven applied bind or policy ALTER reassigns live physical
	// placement — the binding-risk warning lands, yet whether the new
	// placement can actually deploy stays unverified while the product
	// holds no topology facts (#99: 拓扑未知给gap而非猜测可部署). The
	// suggestion text alone is not a substitute for the structured gap.
	if outcome.topology && placementTopologyFacts(statement) == nil {
		gaps = append(gaps, rule.EvidenceGap{
			ReasonCode:    gapReasonUnknownTableState,
			RequiredFacts: []string{"placement_policy.topology"},
		})
	}
	return gaps
}

// placementTopologyFacts reads the proven topology-prerequisite facts the
// enrich pass attached — nil means topology was never read.
func placementTopologyFacts(statement spec.Statement) *spec.PlacementTopologyFacts {
	if statement.Metadata == nil {
		return nil
	}
	return statement.Metadata.PlacementTopology
}

type placementRuleOutcome struct {
	warning    bool
	topology   bool
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
			outcome := placementRuleOutcome{
				warning: true,
				// OR REPLACE on a proven-existing policy rewrites the
				// definition every bound object deploys — the same live
				// reassignment an ALTER POLICY performs.
				topology: ddl.Options["or_replace"] == "true" && exists != nil && *exists,
				policy:   strings.ToLower(ddl.ObjectName),
				action:   "create_placement_policy",
				message:  fmt.Sprintf("CREATE PLACEMENT POLICY %q declares a topology policy objects can bind to", ddl.ObjectName),
			}
			if ddl.Options["or_replace"] == "true" && exists == nil {
				// Whether the replace rewrote live bound deployments is
				// undecidable while the catalog is unvouched — surface
				// the premise instead of staying silent.
				outcome.facts = []string{"placement_policy.catalog"}
			}
			return outcome
		case spec.PlacementUnproven:
			return placementRuleOutcome{facts: []string{"placement_policy.catalog"}}
		}
	case spec.DDLOperationAlterPlacementPolicy:
		exists := placementPolicyExists(policies, ddl.ObjectName)
		switch spec.PlacementAlterOutcome(statement.Dialect, exists) {
		case spec.PlacementApplied:
			return placementRuleOutcome{
				warning:  true,
				topology: true,
				policy:   strings.ToLower(ddl.ObjectName),
				action:   "alter_placement_policy",
				message:  fmt.Sprintf("ALTER PLACEMENT POLICY %q changes the topology contract of every bound object", ddl.ObjectName),
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
				// An IF EXISTS no-op over proven absence removes nothing —
				// the warning only fires when a policy provably existed.
				warning: exists != nil && *exists,
				policy:  strings.ToLower(ddl.ObjectName),
				action:  "drop_placement_policy",
				message: fmt.Sprintf("DROP PLACEMENT POLICY %q removes a topology policy", ddl.ObjectName),
			}
		case spec.PlacementUnproven:
			// Attribute the missing premise precisely: an unvouched
			// catalog owes the catalog fact; a proven-existing policy
			// whose bound set was never read owes bound_targets alone.
			if policies == nil {
				return placementRuleOutcome{facts: []string{"placement_policy.catalog", "placement_policy.bound_targets"}}
			}
			return placementRuleOutcome{facts: []string{"placement_policy.bound_targets"}}
		}
	case spec.DDLOperationAlterRangePlacement:
		if spec.PlacementRangeOutcome(statement.Dialect, ddl.PlacementRange) != spec.PlacementApplied {
			return placementRuleOutcome{}
		}
		return r.bindOutcome(statement, ddl.PlacementBinding, policies, "range:"+strings.ToLower(ddl.PlacementRange), "range", "alter_range_placement")
	default:
		binding, action, target, targetKind := firstNamedPlacementBinding(ddl)
		if binding == nil {
			return placementRuleOutcome{}
		}
		return r.bindOutcome(statement, binding, policies, target, targetKind, action)
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
			topology:   true,
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

// tidbPlacementExistsRequireRule is the catalog-premise check for TiDB
// placement statements: a named policy reference must provably exist and a
// policy DROP must provably have no bound objects. Proven refusals surface
// as blocker findings — the statement cannot land (#99: 不存在策略给对应
// 错误); unvouched catalog or bound-target facts surface evidence gaps
// with their own fact identifiers. DEFAULT unbinds carry no premise;
// IF EXISTS disarms only the existence premise — the bound set stays live
// because native 8241 refuses a bound policy under IF EXISTS too.
// Findings only ever carry bounded identities — policy name, action,
// target kind/name, bound-target identities — never constraint bodies.
type tidbPlacementExistsRequireRule struct {
	level rule.Level
}

func newTiDBPlacementExistsRequireRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return tidbPlacementExistsRequireRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r tidbPlacementExistsRequireRule) ID() string { return ruleIDTiDBPlacementExistsRequire }

func (r tidbPlacementExistsRequireRule) AppliesTo(statement spec.Statement) bool {
	return appliesToTiDBPlacementSurface(statement)
}

func (r tidbPlacementExistsRequireRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	policies := placementPolicyFacts(statement)
	if policies == nil {
		return nil, nil // unvouched catalog — evidence gap carries it
	}
	ddl := statement.DDL
	switch ddl.Operation {
	case spec.DDLOperationCreatePlacementPolicy:
		exists := placementPolicyExists(policies, ddl.ObjectName)
		if spec.PlacementCreateOutcome(statement.Dialect,
			ddl.Options["or_replace"] == "true", ddl.Options["if_not_exists"] == "true",
			exists) == spec.PlacementRefused {
			return r.finding(fmt.Sprintf("placement policy %q already exists", ddl.ObjectName),
				"create the policy with OR REPLACE or IF NOT EXISTS, or choose a different name",
				map[string]any{"policy": strings.ToLower(ddl.ObjectName), "action": "create_placement_policy"})
		}
		return nil, nil
	case spec.DDLOperationAlterPlacementPolicy:
		exists := placementPolicyExists(policies, ddl.ObjectName)
		if spec.PlacementAlterOutcome(statement.Dialect, exists) == spec.PlacementRefused {
			return r.missingPolicyFinding(ddl.ObjectName, "alter_placement_policy", "", "")
		}
		return nil, nil
	case spec.DDLOperationDropPlacementPolicy:
		return r.dropFinding(statement, ddl, policies)
	case spec.DDLOperationAlterRangePlacement:
		// A refused range name is a range premise, not a policy-existence
		// premise — this rule stays silent there.
		if spec.PlacementRangeOutcome(statement.Dialect, ddl.PlacementRange) != spec.PlacementApplied {
			return nil, nil
		}
		return r.bindFinding(statement, ddl.PlacementBinding, policies,
			"range:"+strings.ToLower(ddl.PlacementRange), "range", "alter_range_placement")
	default:
		binding, action, target, targetKind := firstNamedPlacementBinding(ddl)
		if binding == nil {
			return nil, nil
		}
		return r.bindFinding(statement, binding, policies, target, targetKind, action)
	}
}

// firstNamedPlacementBinding returns the first authored named (non-DEFAULT)
// placement binding the statement carries — the shared scan both placement
// rules use: statement-level binding, alter action, partition def. A
// DEFAULT clause is an unbind, never a named reference, so the scan skips
// it and keeps looking (a CREATE TABLE can legitimately carry a table-level
// = DEFAULT beside named partition binds).
func firstNamedPlacementBinding(ddl *spec.DDL) (binding *spec.PlacementBinding, action, target, targetKind string) {
	target = ""
	targetKind = "table"
	if ddl.Table != nil {
		target = ddl.Table.Name
	}
	if ddl.Operation == spec.DDLOperationCreateSchema || ddl.Operation == spec.DDLOperationAlterSchema {
		targetKind = "schema"
		target = ddl.ObjectName
	}
	if ddl.PlacementBinding != nil && !ddl.PlacementBinding.Default {
		return ddl.PlacementBinding, strings.ToLower(string(ddl.Operation)), target, targetKind
	}
	for i := range ddl.Alter {
		alter := &ddl.Alter[i]
		if alter.Action == "placement_policy" && alter.Placement != nil && !alter.Placement.Default {
			if len(alter.Partitions) > 0 {
				targetKind = "partition"
			}
			return alter.Placement, "placement_policy", target, targetKind
		}
	}
	for i := range ddl.PartitionDefs {
		def := &ddl.PartitionDefs[i]
		if def.Placement != nil && !def.Placement.Default {
			return def.Placement, "create_table", target, "partition"
		}
	}
	return nil, "", "", ""
}

func (r tidbPlacementExistsRequireRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	policies := placementPolicyFacts(statement)
	ddl := statement.DDL
	catalogGap := rule.EvidenceGap{
		ReasonCode:    gapReasonUnknownTableState,
		RequiredFacts: []string{"placement_policy.catalog"},
	}
	switch ddl.Operation {
	case spec.DDLOperationCreatePlacementPolicy:
		// OR REPLACE / IF NOT EXISTS disarm the existence premise.
		if ddl.Options["or_replace"] == "true" || ddl.Options["if_not_exists"] == "true" {
			return nil
		}
	case spec.DDLOperationAlterPlacementPolicy:
		// always consults the catalog
	case spec.DDLOperationDropPlacementPolicy:
		// IF EXISTS disarms only the existence premise — the bound set
		// stays a live premise because a proven-bound policy still
		// refuses under IF EXISTS (native 8241 ignores the clause).
		if policies == nil {
			return []rule.EvidenceGap{{
				ReasonCode:    gapReasonUnknownTableState,
				RequiredFacts: []string{"placement_policy.catalog", "placement_policy.bound_targets"},
			}}
		}
		exists := placementPolicyExists(policies, ddl.ObjectName)
		if *exists && placementBoundTargets(policies, ddl.ObjectName) == nil {
			return []rule.EvidenceGap{{
				ReasonCode:    gapReasonUnknownTableState,
				RequiredFacts: []string{"placement_policy.bound_targets"},
			}}
		}
		return nil
	case spec.DDLOperationAlterRangePlacement:
		if spec.PlacementRangeOutcome(statement.Dialect, ddl.PlacementRange) != spec.PlacementApplied ||
			ddl.PlacementBinding == nil || ddl.PlacementBinding.Default {
			return nil
		}
	default:
		binding, _, _, _ := firstNamedPlacementBinding(ddl)
		if binding == nil {
			return nil
		}
	}
	if policies == nil {
		return []rule.EvidenceGap{catalogGap}
	}
	return nil
}

// bindFinding surfaces the proven-absent policy behind a refused named
// binding. A DEFAULT unbind and an unproven premise stay silent.
func (r tidbPlacementExistsRequireRule) bindFinding(statement spec.Statement, binding *spec.PlacementBinding, policies map[string]*spec.PlacementPolicyFact, target, targetKind, action string) ([]rule.Finding, error) {
	if binding == nil || binding.Default {
		return nil, nil
	}
	exists := placementPolicyExists(policies, binding.Policy)
	if spec.PlacementBindOutcome(statement.Dialect, *binding, exists) != spec.PlacementRefused {
		return nil, nil
	}
	return r.missingPolicyFinding(binding.Policy, action, target, targetKind)
}

// dropFinding adjudicates the drop's catalog premise: a bare drop of a
// proven-absent policy refuses the missing policy, while a proven-bound
// policy refuses the inbound references (native 8241).
func (r tidbPlacementExistsRequireRule) dropFinding(statement spec.Statement, ddl *spec.DDL, policies map[string]*spec.PlacementPolicyFact) ([]rule.Finding, error) {
	exists := placementPolicyExists(policies, ddl.ObjectName)
	bound := placementBoundTargets(policies, ddl.ObjectName)
	if spec.PlacementDropOutcome(statement.Dialect,
		ddl.Options["if_exists"] == "true", exists, bound) != spec.PlacementRefused {
		return nil, nil
	}
	if exists != nil && !*exists {
		return r.missingPolicyFinding(ddl.ObjectName, "drop_placement_policy", "", "")
	}
	metadata := map[string]any{
		"policy": strings.ToLower(ddl.ObjectName),
		"action": "drop_placement_policy",
	}
	if bound != nil {
		metadata["bound_targets"] = append([]string(nil), (*bound)...)
	}
	return r.finding(
		fmt.Sprintf("placement policy %q still has %d bound object(s)", ddl.ObjectName, len(*bound)),
		"unbind the dependent objects with PLACEMENT POLICY=DEFAULT first, or drop the bound objects",
		metadata)
}

func (r tidbPlacementExistsRequireRule) missingPolicyFinding(policy, action, target, targetKind string) ([]rule.Finding, error) {
	metadata := map[string]any{"policy": strings.ToLower(policy), "action": action}
	if target != "" {
		metadata["target"] = target
	}
	if targetKind != "" {
		metadata["target_kind"] = targetKind
	}
	return r.finding(
		fmt.Sprintf("placement policy %q does not exist", policy),
		"create the placement policy first or correct the policy reference",
		metadata)
}

func (r tidbPlacementExistsRequireRule) finding(message, suggestion string, metadata map[string]any) ([]rule.Finding, error) {
	return []rule.Finding{{
		RuleID:     r.ID(),
		Level:      r.level,
		Message:    message,
		Suggestion: suggestion,
		Metadata:   metadata,
	}}, nil
}

// placementBoundTargets answers the proven inbound-reference list for one
// policy, or nil while the bound set is unvouched (policy absent from the
// catalog or the fact itself carries no proven set).
func placementBoundTargets(policies map[string]*spec.PlacementPolicyFact, name string) *[]string {
	if policies == nil {
		return nil
	}
	fact := policies[strings.ToLower(strings.TrimSpace(name))]
	if fact == nil {
		empty := []string{}
		return &empty // proven-absent policy has a proven-empty bound set
	}
	if fact.BoundTargets == nil {
		return nil // bound set unvouched
	}
	return &fact.BoundTargets
}
