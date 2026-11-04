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
	// Every authored binding applies — direct binds, member definitions,
	// alter member defs and partition-scheme defs; DEFAULT retained so an
	// unbind still reaches its premise checks (T20-R4).
	return len(spec.CollectPlacementBindings(ddl)) > 0
}

func (r tidbPlacementWarnRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	outcome := r.outcome(statement)
	var findings []rule.Finding
	for _, warn := range outcome.warnings {
		metadata := map[string]any{
			"policy": warn.policy,
			"action": warn.action,
		}
		if warn.target != "" {
			metadata["target"] = warn.target
		}
		if warn.targetKind != "" {
			metadata["target_kind"] = warn.targetKind
		}
		if warn.schema != "" {
			metadata["schema"] = warn.schema
		}
		if warn.table != "" {
			metadata["table"] = warn.table
		}
		if warn.partition != "" {
			metadata["partition"] = warn.partition
		}
		if warn.bindingSource != "" && warn.actionIndex >= 0 {
			metadata["action_index"] = warn.actionIndex
		}
		if warn.bindingSource != "" && warn.bindingSource != "direct" {
			metadata["binding_source"] = warn.bindingSource
		}
		findings = append(findings, rule.Finding{
			RuleID:     r.ID(),
			Level:      r.level,
			Message:    warn.message,
			Suggestion: "verify the placement policy's region constraints, replica counts, and PD/TiKV topology prerequisites intentionally — a bound policy reassigns the object's physical placement",
			Metadata:   metadata,
		})
	}
	return findings, nil
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

// placementWarnRecord is one applicable binding occurrence's warning
// payload — bounded identities only, never raw regions or constraints.
type placementWarnRecord struct {
	policy        string
	action        string
	target        string
	targetKind    string
	message       string
	schema        string
	table         string
	partition     string
	bindingSource string
	actionIndex   int
}

// placementRuleOutcome aggregates a statement's binding warnings (one per
// applied occurrence), the deduplicated missing evidence, and whether any
// applied bind owes the single statement-level topology gap (T20-R4).
type placementRuleOutcome struct {
	warnings []placementWarnRecord
	topology bool
	facts    []string
}

func (r tidbPlacementWarnRule) outcome(statement spec.Statement) placementRuleOutcome {
	ddl := statement.DDL
	if ddl == nil {
		return placementRuleOutcome{}
	}
	switch ddl.Operation {
	case spec.DDLOperationCreatePlacementPolicy:
		exists, _ := placementLifecyclePremise(statement, ddl.ObjectName)
		switch spec.PlacementCreateOutcome(statement.Dialect,
			ddl.Options["or_replace"] == "true", ddl.Options["if_not_exists"] == "true", exists) {
		case spec.PlacementApplied:
			outcome := placementRuleOutcome{
				warnings: []placementWarnRecord{{
					policy:  strings.ToLower(ddl.ObjectName),
					action:  "create_placement_policy",
					message: fmt.Sprintf("CREATE PLACEMENT POLICY %q declares a topology policy objects can bind to", ddl.ObjectName),
				}},
				// OR REPLACE on a proven-existing policy rewrites the
				// definition every bound object deploys — the same live
				// reassignment an ALTER POLICY performs.
				topology: ddl.Options["or_replace"] == "true" && exists != nil && *exists,
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
		exists, _ := placementLifecyclePremise(statement, ddl.ObjectName)
		switch spec.PlacementAlterOutcome(statement.Dialect, exists) {
		case spec.PlacementApplied:
			return placementRuleOutcome{
				warnings: []placementWarnRecord{{
					policy:  strings.ToLower(ddl.ObjectName),
					action:  "alter_placement_policy",
					message: fmt.Sprintf("ALTER PLACEMENT POLICY %q changes the topology contract of every bound object", ddl.ObjectName),
				}},
				topology: true,
			}
		case spec.PlacementUnproven:
			return placementRuleOutcome{facts: []string{"placement_policy.catalog"}}
		}
	case spec.DDLOperationDropPlacementPolicy:
		exists, bound := placementLifecyclePremise(statement, ddl.ObjectName)
		switch spec.PlacementDropOutcome(statement.Dialect,
			ddl.Options["if_exists"] == "true", exists, bound) {
		case spec.PlacementApplied:
			out := placementRuleOutcome{}
			// An IF EXISTS no-op over proven absence removes nothing —
			// the warning only fires when a policy provably existed.
			if exists != nil && *exists {
				out.warnings = []placementWarnRecord{{
					policy:  strings.ToLower(ddl.ObjectName),
					action:  "drop_placement_policy",
					message: fmt.Sprintf("DROP PLACEMENT POLICY %q removes a topology policy", ddl.ObjectName),
				}}
			}
			return out
		case spec.PlacementUnproven:
			// Attribute the missing premise precisely on the per-target
			// proof: unknown existence owes the catalog fact; a bound set
			// never vouched owes bound_targets. An unvouched catalog is
			// itself missing evidence — the bound uncertainty stems from
			// it — so it is owed alongside (T20-R4).
			var facts []string
			if exists == nil || placementPolicyFacts(statement) == nil {
				facts = append(facts, "placement_policy.catalog")
			}
			if bound == nil {
				facts = append(facts, "placement_policy.bound_targets")
			}
			return placementRuleOutcome{facts: facts}
		}
	case spec.DDLOperationAlterRangePlacement:
		if spec.PlacementRangeOutcome(statement.Dialect, ddl.PlacementRange) != spec.PlacementApplied {
			return placementRuleOutcome{}
		}
		// The range's own binding rides the same per-record assessment —
		// collected as a range target — so a subsequently-unknown policy
		// can never read as a false scalar absence (T20-R4).
		return r.bindingOutcome(statement)
	default:
		return r.bindingOutcome(statement)
	}
	return placementRuleOutcome{}
}

// bindingOutcome consumes the per-statement adjudicated placement facts:
// every collected binding — not just the first — so a later member def's
// missing policy still suppresses the warning and owes no topology gap.
// Aggregate or mutation refusal stays silent; an unproven aggregate owes
// the catalog fact; an unproven mutation owes its bounded static facts;
// otherwise each applied named binding earns its own warning while the
// topology gap stays deduplicated at statement level (T20-R4).
func (r tidbPlacementWarnRule) bindingOutcome(statement spec.Statement) placementRuleOutcome {
	facts := placementFactsFor(statement)
	if facts == nil {
		return placementRuleOutcome{}
	}
	outcome := placementRuleOutcome{}
	mutation := spec.PlacementOutcome(-1)
	if facts.MutationOutcome != nil {
		mutation = *facts.MutationOutcome
	}
	// DEFAULT-only bindings carry no placement premise — no warn and no
	// gap, regardless of the mutation outcome (T20-R4).
	named := false
	for _, item := range facts.Bindings {
		if !item.Target.Binding.Default {
			named = true
			break
		}
	}
	if !named {
		return outcome
	}
	switch {
	case facts.Outcome == spec.PlacementRefused || mutation == spec.PlacementRefused:
		return outcome
	case facts.Outcome == spec.PlacementUnproven:
		outcome.facts = []string{"placement_policy.catalog"}
		return outcome
	case mutation == spec.PlacementUnproven:
		outcome.facts = facts.RequiredFacts
		return outcome
	}
	for _, binding := range facts.Bindings {
		if binding.Outcome != spec.PlacementApplied || binding.Target.Binding.Default {
			continue
		}
		target := binding.Target
		display := target.Target
		if display == "" {
			display = target.Table
		}
		outcome.warnings = append(outcome.warnings, placementWarnRecord{
			policy:        target.Binding.Policy,
			action:        target.Action,
			target:        display,
			targetKind:    target.TargetKind,
			schema:        target.Schema,
			table:         target.Table,
			partition:     target.Partition,
			bindingSource: target.Source,
			actionIndex:   target.ActionIndex,
			message: fmt.Sprintf("If applied, PLACEMENT POLICY %q would bind %s %q to a topology policy",
				target.Binding.Policy, target.TargetKind, display),
		})
		outcome.topology = true
	}
	return outcome
}

// placementFactsFor returns the enrichment-attached adjudicated facts or
// builds the same assessment over the statement's catalog projection —
// the fallback keeps bare rule-unit fixtures working without enrichment.
func placementFactsFor(statement spec.Statement) *spec.PlacementFacts {
	if statement.DDL == nil {
		return nil
	}
	if statement.Metadata != nil && statement.Metadata.Placement != nil {
		return statement.Metadata.Placement
	}
	policies := placementPolicyFacts(statement)
	return spec.AssessPlacementBindings(statement.Dialect,
		spec.CollectPlacementBindings(statement.DDL),
		func(name string) *bool { return placementPolicyExists(policies, name) })
}

// placementLifecyclePremise prefers the lifecycle target's own per-statement
// premise when it names the same policy — a proven-positive bound list
// refuses DROP without laundering a partial catalog into a complete one.
func placementLifecyclePremise(statement spec.Statement, name string) (*bool, *[]string) {
	if statement.Metadata != nil && statement.Metadata.Placement != nil &&
		statement.Metadata.Placement.Policy != nil &&
		strings.EqualFold(statement.Metadata.Placement.Policy.Name, name) {
		premise := statement.Metadata.Placement.Policy
		return premise.Exists, premise.BoundTargets
	}
	policies := placementPolicyFacts(statement)
	return placementPolicyExists(policies, name), placementBoundTargets(policies, name)
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
	ddl := statement.DDL
	switch ddl.Operation {
	case spec.DDLOperationCreatePlacementPolicy:
		exists, _ := placementLifecyclePremise(statement, ddl.ObjectName)
		if spec.PlacementCreateOutcome(statement.Dialect,
			ddl.Options["or_replace"] == "true", ddl.Options["if_not_exists"] == "true",
			exists) == spec.PlacementRefused {
			return r.finding(fmt.Sprintf("placement policy %q already exists", ddl.ObjectName),
				"create the policy with OR REPLACE or IF NOT EXISTS, or choose a different name",
				map[string]any{"policy": strings.ToLower(ddl.ObjectName), "action": "create_placement_policy"})
		}
		return nil, nil
	case spec.DDLOperationAlterPlacementPolicy:
		exists, _ := placementLifecyclePremise(statement, ddl.ObjectName)
		if spec.PlacementAlterOutcome(statement.Dialect, exists) == spec.PlacementRefused {
			return r.missingPolicyFinding(ddl.ObjectName, "alter_placement_policy", "", "")
		}
		return nil, nil
	case spec.DDLOperationDropPlacementPolicy:
		return r.dropFinding(statement, ddl)
	case spec.DDLOperationAlterRangePlacement:
		// A refused range name is a range premise, not a policy-existence
		// premise — this rule stays silent there; the binding itself is
		// adjudicated through the same per-record proof as every other
		// authored assignment (T20-R4).
		if spec.PlacementRangeOutcome(statement.Dialect, ddl.PlacementRange) != spec.PlacementApplied {
			return nil, nil
		}
		return r.bindingFindings(statement)
	default:
		return r.bindingFindings(statement)
	}
}

// bindingFindings walks every collected binding record and emits one
// missing-policy finding per refused named occurrence — per-object
// registry proof, never a scalar read of a possibly-stale catalog map
// (T20-R4).
func (r tidbPlacementExistsRequireRule) bindingFindings(statement spec.Statement) ([]rule.Finding, error) {
	facts := placementFactsFor(statement)
	if facts == nil {
		return nil, nil
	}
	var findings []rule.Finding
	for _, binding := range facts.Bindings {
		if binding.Outcome != spec.PlacementRefused || binding.Target.Binding.Default {
			continue
		}
		found, err := r.missingPolicyFinding(binding.Target.Binding.Policy,
			binding.Target.Action, binding.Target.Target, binding.Target.TargetKind,
			binding.Target)
		if err != nil {
			return nil, err
		}
		findings = append(findings, found...)
	}
	return findings, nil
}

func (r tidbPlacementExistsRequireRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	ddl := statement.DDL
	switch ddl.Operation {
	case spec.DDLOperationCreatePlacementPolicy, spec.DDLOperationAlterPlacementPolicy:
		if ddl.Operation == spec.DDLOperationCreatePlacementPolicy &&
			(ddl.Options["or_replace"] == "true" || ddl.Options["if_not_exists"] == "true") {
			return nil
		}
		// The lifecycle target's own proof decides — an unread
		// existence premise owes the catalog fact; it never falls
		// through silently (T20-R4).
		if exists, _ := placementLifecyclePremise(statement, ddl.ObjectName); exists == nil {
			return []rule.EvidenceGap{{
				ReasonCode:    gapReasonUnknownTableState,
				RequiredFacts: []string{"placement_policy.catalog"},
			}}
		}
		return nil
	case spec.DDLOperationDropPlacementPolicy:
		// IF EXISTS disarms only the existence premise — the bound set
		// stays a live premise because a proven-bound policy still
		// refuses under IF EXISTS (native 8241 ignores the clause).
		// The per-target premise — not the global map — decides which
		// fact is owed (T20-R4).
		exists, bound := placementLifecyclePremise(statement, ddl.ObjectName)
		var facts []string
		if exists == nil || placementPolicyFacts(statement) == nil {
			facts = append(facts, "placement_policy.catalog")
		}
		if bound == nil {
			facts = append(facts, "placement_policy.bound_targets")
		}
		if len(facts) > 0 {
			return []rule.EvidenceGap{{
				ReasonCode:    gapReasonUnknownTableState,
				RequiredFacts: facts,
			}}
		}
		return nil
	case spec.DDLOperationAlterRangePlacement:
		if spec.PlacementRangeOutcome(statement.Dialect, ddl.PlacementRange) != spec.PlacementApplied ||
			ddl.PlacementBinding == nil || ddl.PlacementBinding.Default {
			return nil
		}
		return r.bindingGaps(statement)
	default:
		return r.bindingGaps(statement)
	}
}

// bindingGaps reports the binding records' missing premise: a refused
// statement carries no gap (the blocker finding explains it), an
// unproven aggregate owes the catalog fact, and a decided statement
// owes nothing — the per-object proof replaces the old global-map nil
// test that could lose a gap after a complete seed plus unknown DROP
// (T20-R4).
func (r tidbPlacementExistsRequireRule) bindingGaps(statement spec.Statement) []rule.EvidenceGap {
	facts := placementFactsFor(statement)
	if facts == nil || facts.Outcome == spec.PlacementRefused ||
		(facts.MutationOutcome != nil && *facts.MutationOutcome == spec.PlacementRefused) {
		return nil
	}
	if facts.Outcome == spec.PlacementUnproven {
		return []rule.EvidenceGap{{
			ReasonCode:    gapReasonUnknownTableState,
			RequiredFacts: []string{"placement_policy.catalog"},
		}}
	}
	return nil
}

// dropFinding adjudicates the drop's catalog premise: a bare drop of a
// proven-absent policy refuses the missing policy, while a proven-bound
// policy refuses the inbound references (native 8241).
func (r tidbPlacementExistsRequireRule) dropFinding(statement spec.Statement, ddl *spec.DDL) ([]rule.Finding, error) {
	exists, bound := placementLifecyclePremise(statement, ddl.ObjectName)
	if spec.PlacementDropOutcome(statement.Dialect,
		ddl.Options["if_exists"] == "true", exists, bound) != spec.PlacementRefused {
		return nil, nil
	}
	if exists != nil && !*exists {
		return r.missingPolicyFinding(ddl.ObjectName, "drop_placement_policy", "", "")
	}
	var boundTargets []string
	if bound != nil {
		boundTargets = append([]string(nil), (*bound)...)
	}
	return r.finding(
		fmt.Sprintf("placement policy %q still has %d bound object(s)", ddl.ObjectName, len(boundTargets)),
		"unbind the dependent objects with PLACEMENT POLICY=DEFAULT first, or drop the bound objects",
		map[string]any{
			"policy":        strings.ToLower(ddl.ObjectName),
			"action":        "drop_placement_policy",
			"bound_targets": boundTargets,
		})
}

func (r tidbPlacementExistsRequireRule) missingPolicyFinding(policy, action, target, targetKind string, source ...spec.PlacementBindingTarget) ([]rule.Finding, error) {
	metadata := map[string]any{"policy": strings.ToLower(policy), "action": action}
	if target != "" {
		metadata["target"] = target
	}
	if targetKind != "" {
		metadata["target_kind"] = targetKind
	}
	if len(source) > 0 {
		t := source[0]
		if t.Schema != "" {
			metadata["schema"] = t.Schema
		}
		if t.Table != "" {
			metadata["table"] = t.Table
		}
		if t.Partition != "" {
			metadata["partition"] = t.Partition
		}
		if t.ActionIndex >= 0 {
			metadata["action_index"] = t.ActionIndex
		}
		if t.Source != "" && t.Source != "direct" {
			metadata["binding_source"] = t.Source
		}
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
