package ddl

import (
	"context"
	"fmt"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// resource_group_rules.go implements the T22/#101 TiDB RU resource-group
// lifecycle rules. They adjudicate only what the ordered state and the
// instance capability fact prove: existence, the reserved builtin group,
// and the resource-control gate. Definitions, quota values, and workload
// effects are configuration facts — the rules never claim throughput,
// latency, or runtime enforcement.

const (
	resourceGroupGapUnknownState      = "unknown_resource_group_state"
	resourceGroupGapUnknownCapability = "unknown_resource_group_capability"
	resourceGroupFactState            = "resource_group.state"
	resourceGroupFactCapability       = "tidb_enable_resource_control"
)

func resourceGroupApplies(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectTiDB &&
		statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.ResourceGroup != nil
}

// resourceGroupBinding returns the statement's bound group state — the
// ordered pre-state keyed by lowercase name (global namespace).
func resourceGroupBinding(statement spec.Statement) *spec.ResourceGroupState {
	if statement.Metadata == nil || statement.DDL.ResourceGroup == nil {
		return nil
	}
	return statement.Metadata.ResourceGroups[strings.ToLower(strings.TrimSpace(statement.DDL.ResourceGroup.Name))]
}

func resourceGroupFinding(level rule.Level, message, suggestion, operation string, state *spec.ResourceGroupState) rule.Finding {
	name := ""
	if state != nil {
		name = state.Name
	}
	return rule.Finding{
		Level:      level,
		Message:    message,
		Suggestion: suggestion,
		Metadata: map[string]any{
			"operation":   operation,
			"object_type": "resource_group",
			"object_name": name,
		},
	}
}

// resourceGroupProvenRefusal reports whether the ordered state already
// decides the statement's outcome without the capability premise — a
// proven refusal or a proven guard no-op keeps the capability gap
// silent since the fact cannot change the verdict.
func resourceGroupProvenRefusal(statement spec.Statement, state *spec.ResourceGroupState) bool {
	if state == nil || !state.Proven() || statement.DDL == nil || statement.DDL.ResourceGroup == nil {
		return false
	}
	decl := statement.DDL.ResourceGroup
	switch statement.DDL.Operation {
	case spec.DDLOperationCreateResourceGroup:
		return state.Exists && !decl.IfNotExists
	case spec.DDLOperationAlterResourceGroup:
		return !state.Exists && !decl.IfExists
	case spec.DDLOperationDropResourceGroup:
		return state.Reserved || (!state.Exists && !decl.IfExists)
	}
	return false
}

// resourceGroupValidRule adjudicates the existence and premise contracts
// of resource-group lifecycle statements: the proven resource-control
// gate, the reserved builtin group, proven duplicates, and proven-absent
// targets. Unproven facts emit bounded gaps — never fabricated answers.
type resourceGroupValidRule struct {
	level rule.Level
}

func newTiDBResourceGroupValidRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return resourceGroupValidRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r resourceGroupValidRule) ID() string { return ruleIDTiDBResourceGroupValid }

func (r resourceGroupValidRule) AppliesTo(statement spec.Statement) bool {
	return resourceGroupApplies(statement)
}

func (r resourceGroupValidRule) Evaluate(_ context.Context, statement spec.Statement) ([]rule.Finding, error) {
	decl := statement.DDL.ResourceGroup
	state := resourceGroupBinding(statement)
	operation := string(statement.DDL.Operation)

	// The resource-control gate is a proven-refusal premise: TiDB 8.5
	// refuses every resource-group DDL while tidb_enable_resource_control
	// is OFF, before existence adjudication.
	var instance *spec.InstanceFacts
	if statement.Metadata != nil {
		instance = statement.Metadata.Instance
	}
	if instance != nil &&
		instance.TiDBEnableResourceControlKnown && !instance.TiDBEnableResourceControl {
		return []rule.Finding{resourceGroupFinding(r.level,
			fmt.Sprintf("%s on %q refuses while tidb_enable_resource_control is OFF.", operation, decl.Name),
			"enable tidb_enable_resource_control or remove the statement", operation, state)}, nil
	}

	if state == nil || !state.Proven() {
		return nil, nil
	}
	var findings []rule.Finding
	switch statement.DDL.Operation {
	case spec.DDLOperationCreateResourceGroup:
		if state.Exists && !decl.IfNotExists {
			findings = append(findings, resourceGroupFinding(r.level,
				fmt.Sprintf("CREATE RESOURCE GROUP names the existing group %q.", state.Name),
				"use CREATE RESOURCE GROUP IF NOT EXISTS or alter the existing group", operation, state))
		}
	case spec.DDLOperationAlterResourceGroup:
		if !state.Exists && !decl.IfExists {
			findings = append(findings, resourceGroupFinding(r.level,
				fmt.Sprintf("ALTER RESOURCE GROUP names the absent group %q.", state.Name),
				"create the resource group first or use ALTER RESOURCE GROUP IF EXISTS", operation, state))
		}
	case spec.DDLOperationDropResourceGroup:
		if state.Reserved {
			findings = append(findings, resourceGroupFinding(r.level,
				fmt.Sprintf("DROP RESOURCE GROUP names the reserved builtin group %q, which cannot be dropped.", state.Name),
				"the default group is managed by TiDB — alter its options instead of dropping", operation, state))
		} else if !state.Exists && !decl.IfExists {
			findings = append(findings, resourceGroupFinding(r.level,
				fmt.Sprintf("DROP RESOURCE GROUP names the absent group %q.", state.Name),
				"verify the group name or use DROP RESOURCE GROUP IF EXISTS", operation, state))
		}
	}
	return findings, nil
}

// EvidenceGaps reports the bounded missing-fact contracts this rule owns:
// the resource-control premise (required for every lifecycle statement —
// a disabled gate refuses before anything else) and the unproven target
// existence. The capability gap stays silent when the outcome is already
// a proven refusal on other grounds.
func (r resourceGroupValidRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	state := resourceGroupBinding(statement)
	var gaps []rule.EvidenceGap
	var instance *spec.InstanceFacts
	if statement.Metadata != nil {
		instance = statement.Metadata.Instance
	}
	if instance == nil || !instance.TiDBEnableResourceControlKnown {
		if !resourceGroupProvenRefusal(statement, state) {
			gaps = append(gaps, rule.EvidenceGap{
				ReasonCode:    resourceGroupGapUnknownCapability,
				RequiredFacts: []string{resourceGroupFactCapability},
			})
		}
	}
	if state == nil || !state.Proven() {
		gaps = append(gaps, rule.EvidenceGap{
			ReasonCode:    resourceGroupGapUnknownState,
			RequiredFacts: []string{resourceGroupFactState},
		})
	}
	return gaps
}

// resourceGroupBurstableWarnRule reports a declared-or-proven burst
// allowance — the group may exceed its RU_PER_SEC budget under load. The
// warning names only the proven enablement, never a performance claim.
type resourceGroupBurstableWarnRule struct {
	level rule.Level
}

func newTiDBResourceGroupBurstableWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return resourceGroupBurstableWarnRule{level: configuredLevel(cfg, rule.LevelWarning)}, nil
}

func (r resourceGroupBurstableWarnRule) ID() string { return ruleIDTiDBResourceGroupBurstableWarn }

func (r resourceGroupBurstableWarnRule) AppliesTo(statement spec.Statement) bool {
	return resourceGroupApplies(statement) &&
		(statement.DDL.Operation == spec.DDLOperationCreateResourceGroup ||
			statement.DDL.Operation == spec.DDLOperationAlterResourceGroup)
}

func (r resourceGroupBurstableWarnRule) Evaluate(_ context.Context, statement spec.Statement) ([]rule.Finding, error) {
	decl := statement.DDL.ResourceGroup
	state := resourceGroupBinding(statement)
	burstable, declared := decl.DeclaredBurstable()
	enabled := declared && burstable.Enabled()
	if !declared && statement.DDL.Operation == spec.DDLOperationAlterResourceGroup &&
		state != nil && state.Proven() && state.Exists && state.Definition != nil && state.Definition.Burstable != nil {
		// ALTER without a BURSTABLE member keeps the proven catalog flag.
		enabled = *state.Definition.Burstable
	}
	if !enabled {
		return nil, nil
	}
	return []rule.Finding{resourceGroupFinding(r.level,
		"BURSTABLE lets the resource group exceed its RU_PER_SEC budget when capacity is available.",
		"verify the workload tolerates quota overflow before enabling burstable", "burstable", state)}, nil
}

// resourceGroupRunawayWarnRule reports a declared QUERY_LIMIT with a
// forcible action — KILL terminates matching queries and SWITCH_GROUP
// moves them to another group. Passive actions (DRYRUN, COOLDOWN) and a
// reset clause never warn.
type resourceGroupRunawayWarnRule struct {
	level rule.Level
}

func newTiDBResourceGroupRunawayWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return resourceGroupRunawayWarnRule{level: configuredLevel(cfg, rule.LevelWarning)}, nil
}

func (r resourceGroupRunawayWarnRule) ID() string { return ruleIDTiDBResourceGroupRunawayWarn }

func (r resourceGroupRunawayWarnRule) AppliesTo(statement spec.Statement) bool {
	if !resourceGroupApplies(statement) ||
		(statement.DDL.Operation != spec.DDLOperationCreateResourceGroup &&
			statement.DDL.Operation != spec.DDLOperationAlterResourceGroup) {
		return false
	}
	runaway := statement.DDL.ResourceGroup.DeclaredRunaway()
	return runaway != nil && !runaway.Reset
}

func (r resourceGroupRunawayWarnRule) Evaluate(_ context.Context, statement spec.Statement) ([]rule.Finding, error) {
	state := resourceGroupBinding(statement)
	switch action := statement.DDL.ResourceGroup.DeclaredRunaway().Action(); {
	case action == "kill":
		return []rule.Finding{resourceGroupFinding(r.level,
			"QUERY_LIMIT ACTION=KILL terminates queries that exceed the runaway rule.",
			"verify killed statements are retryable or expected before enabling", "query_limit", state)}, nil
	case strings.HasPrefix(action, "switch_group"):
		return []rule.Finding{resourceGroupFinding(r.level,
			"QUERY_LIMIT ACTION=SWITCH_GROUP moves runaway queries to another resource group.",
			"verify the target group exists and accepts the demoted load", "query_limit", state)}, nil
	}
	return nil, nil
}

// resourceGroupNoticeAdjudicator is the shared refusal/no-op truth table
// the lifecycle notices consume — the same ordered ResourceGroupState
// the valid rule reads. Under MySQL the statement carries no binding, so
// the notice falls back to the generic declarative path; under TiDB a
// proven refusal or guard no-op suppresses it (B1 — never a false
// success notice).
func resourceGroupNoticeAdjudicator(operation, message, suggestion string) func(rule.Level, spec.Statement) ([]rule.Finding, bool) {
	return func(level rule.Level, statement spec.Statement) ([]rule.Finding, bool) {
		if statement.Dialect != spec.DialectTiDB {
			return nil, false
		}
		// A proven-OFF resource-control gate refuses before existence
		// adjudication — the same dominance that suppresses notices on
		// proven refusals applies even when the target stays unproven
		// (issue #101 T22 Profile A native finding).
		if statement.Metadata != nil && statement.Metadata.Instance != nil &&
			statement.Metadata.Instance.TiDBEnableResourceControlKnown &&
			!statement.Metadata.Instance.TiDBEnableResourceControl {
			return []rule.Finding{}, true
		}
		state := resourceGroupBinding(statement)
		if state == nil {
			return nil, false
		}
		if state.Proven() {
			suppress := false
			switch statement.DDL.Operation {
			case spec.DDLOperationCreateResourceGroup:
				// Existing target: refusal without the guard, no-op with
				// it — neither is a successful create.
				suppress = state.Exists
			case spec.DDLOperationAlterResourceGroup:
				// Absent target: refusal without IF EXISTS, no-op with it.
				suppress = !state.Exists
			case spec.DDLOperationDropResourceGroup:
				suppress = state.Reserved || !state.Exists
			}
			if suppress {
				return []rule.Finding{}, true
			}
		}
		name := statement.DDL.ResourceGroup.Name
		if state.Name != "" {
			name = state.Name
		}
		return []rule.Finding{{
			Level:      level,
			Message:    fmt.Sprintf(message, name),
			Suggestion: suggestion,
			Metadata: map[string]any{
				"operation":   operation,
				"object_type": "resource_group",
				"object_name": name,
			},
		}}, true
	}
}
