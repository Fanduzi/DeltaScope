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

// resourceGroupStatementRefused reports whether the statement provably
// cannot take effect under fixed TiDB 8.5 semantics: a proven-OFF
// resource-control gate, a statically-invalid runaway clause, or a
// proven existence/reserved-target refusal. It reads only shared facts —
// spec declarations, ordered state and observed metadata — never the
// verdict, another rule's findings, or whether the valid rule is
// enabled. Declared-effect warnings and success notices consume it so a
// refused statement never claims its authored config will apply.
func resourceGroupStatementRefused(statement spec.Statement) bool {
	if statement.Metadata != nil && statement.Metadata.Instance != nil &&
		statement.Metadata.Instance.TiDBEnableResourceControlKnown &&
		!statement.Metadata.Instance.TiDBEnableResourceControl {
		return true
	}
	if statement.DDL == nil || statement.DDL.ResourceGroup == nil {
		return false
	}
	if resourceGroupRunawayInvalidReason(statement.DDL.Operation,
		statement.DDL.ResourceGroup) != "" {
		return true
	}
	return resourceGroupProvenRefusal(statement, resourceGroupBinding(statement))
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

	var findings []rule.Finding
	if reason := resourceGroupRunawayInvalidReason(statement.DDL.Operation, decl); reason != "" {
		findings = append(findings, resourceGroupFinding(r.level,
			fmt.Sprintf("%s on %q declares a runaway clause TiDB 8.5 statically refuses: %s.", operation, decl.Name, reason),
			"declare at least one positive EXEC_ELAPSED/PROCESSED_KEYS/RU threshold and an ACTION", operation, state))
	}
	if state == nil || !state.Proven() {
		return findings, nil
	}
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
	provenOff := instance != nil && instance.TiDBEnableResourceControlKnown &&
		!instance.TiDBEnableResourceControl
	if instance == nil || !instance.TiDBEnableResourceControlKnown {
		if !resourceGroupProvenRefusal(statement, state) {
			gaps = append(gaps, rule.EvidenceGap{
				ReasonCode:    resourceGroupGapUnknownCapability,
				RequiredFacts: []string{resourceGroupFactCapability},
			})
		}
	}
	// A proven-OFF gate already concluded this check with a refusal —
	// target existence is no longer a required premise, so no residual
	// state gap follows (the object itself may stay unknown internally;
	// only the gap that a completed check no longer needs is dropped).
	if !provenOff && (state == nil || !state.Proven()) {
		gaps = append(gaps, rule.EvidenceGap{
			ReasonCode:    resourceGroupGapUnknownState,
			RequiredFacts: []string{resourceGroupFactState},
		})
	}
	return gaps
}

// resourceGroupRunawayInvalidReason reports the fixed v8.5.0 static
// refusal for a complete, fully-recognized non-reset QUERY_LIMIT clause:
// NewGroupFromOptions rejects a runaway rule whose thresholds are all
// zero before rejecting a missing ACTION, so the same order applies.
// Only the two proven conditions are adjudicated — an unrecognized
// member stays a capability gap (the whole option is unmodeled), a reset
// clause is a legal clear, and QUERY_LIMIT member values are never
// simulated. Returns "" for CREATE/ALTER without an offending clause and
// always for DROP.
func resourceGroupRunawayInvalidReason(operation spec.DDLOperation, decl *spec.ResourceGroupDeclaration) string {
	if decl == nil || (operation != spec.DDLOperationCreateResourceGroup &&
		operation != spec.DDLOperationAlterResourceGroup) {
		return ""
	}
	for _, option := range decl.Options {
		if option.Kind != spec.ResourceGroupOptionQueryLimit ||
			option.Runaway == nil || option.Runaway.Reset {
			continue
		}
		recognized := true
		for _, member := range option.Runaway.Members {
			switch member.Kind {
			case spec.ResourceGroupRunawayExecElapsed, spec.ResourceGroupRunawayProcessedKeys,
				spec.ResourceGroupRunawayRU, spec.ResourceGroupRunawayAction,
				spec.ResourceGroupRunawayWatch:
			default:
				recognized = false
			}
		}
		if !recognized {
			continue
		}
		if !option.Runaway.HasEffectiveThreshold() {
			return "every runaway threshold is zero"
		}
		if option.Runaway.Action() == "" {
			return "no runaway ACTION is declared"
		}
	}
	return ""
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
	// The authored declaration describes a config the statement will not
	// land once the statement itself is a proven refusal — suppress that
	// branch only. The proven catalog flag is a live fact independent of
	// this statement's outcome and still earns its warning.
	enabled := declared && burstable.Enabled() && !resourceGroupStatementRefused(statement)
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
	// The warning describes the declared action's applied effect — a
	// proven refusal lands nothing, so it must stay silent.
	if resourceGroupStatementRefused(statement) {
		return nil, nil
	}
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
// the valid rule reads. Under TiDB a proven refusal or guard no-op
// suppresses it (B1 — never a false success notice); under MySQL the
// MySQL CPU family adjudicates against its own ordered state and
// version premise with the same truth table (T24/#103), while any
// remaining shape falls back to the generic declarative path.
func resourceGroupNoticeAdjudicator(operation, message, suggestion string) func(rule.Level, spec.Statement) ([]rule.Finding, bool) {
	return func(level rule.Level, statement spec.Statement) ([]rule.Finding, bool) {
		if statement.Dialect == spec.DialectMySQL && statement.DDL != nil &&
			statement.DDL.MySQLResourceGroup != nil {
			return mysqlResourceGroupNoticeOutcome(level, operation, message, suggestion, statement)
		}
		if statement.Dialect != spec.DialectTiDB {
			return nil, false
		}
		// A proven refusal — the OFF gate, a statically-invalid runaway
		// clause or existence adjudication — suppresses the success
		// notice even when the target stays unproven (issue #101 T22
		// Profile A native finding).
		if resourceGroupStatementRefused(statement) {
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

// mysqlResourceGroupNoticeOutcome is the MySQL arm of the shared notice
// adjudicator — the same ordered MySQLResourceGroupState the valid rule
// reads, under the version capability premise. A proven refusal on any
// ground (version, duplicate kind, priority bound, existence, reserved
// target) suppresses the success notice; MySQL's grammar has no IF
// guards, so no rescued no-op path exists.
func mysqlResourceGroupNoticeOutcome(level rule.Level, operation, message, suggestion string, statement spec.Statement) ([]rule.Finding, bool) {
	// A proven refusal lands nothing — never a false success notice,
	// even when the target stays unproven (same B1 contract as the TiDB
	// arm). An undecidable outcome is equally no claim of success: a
	// refusing branch exists, so the statement may land nothing.
	outcome := mysqlResourceGroupOutcome(statement)
	if outcome.Refused || outcome.Undecidable {
		return []rule.Finding{}, true
	}
	state := mysqlResourceGroupBinding(statement)
	if state != nil && state.Proven() {
		suppress := false
		switch statement.DDL.Operation {
		case spec.DDLOperationCreateResourceGroup:
			suppress = state.Exists
		case spec.DDLOperationAlterResourceGroup, spec.DDLOperationDropResourceGroup:
			suppress = state.Reserved || !state.Exists
		}
		if suppress {
			return []rule.Finding{}, true
		}
	}
	name := statement.DDL.MySQLResourceGroup.Name
	if state != nil && state.Name != "" {
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
