package ddl

import (
	"context"
	"fmt"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// mysql_resource_group_rules.go implements the T24/#103 MySQL CPU
// resource-group lifecycle rules. Every verdict rides the shared
// spec.MySQLResourceGroupStatementOutcome truth table — the same
// upstream prepare→execute replay the ordered-state transition
// consumes, so a blocker finding, a premise gap, and a withheld
// post-state can never diverge. CPU affinity effects, achieved
// priorities, and workload enforcement are configuration facts —
// never runtime measurements.

func mysqlResourceGroupApplies(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectMySQL &&
		statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.MySQLResourceGroup != nil
}

// mysqlResourceGroupBinding returns the statement's bound group state —
// the ordered pre-state keyed by lowercase name (global, case-insensitive
// namespace).
func mysqlResourceGroupBinding(statement spec.Statement) *spec.MySQLResourceGroupState {
	if statement.Metadata == nil || statement.DDL.MySQLResourceGroup == nil {
		return nil
	}
	return statement.Metadata.MySQLResourceGroups[strings.ToLower(strings.TrimSpace(statement.DDL.MySQLResourceGroup.Name))]
}

// mysqlResourceGroupEvidence bundles the premise inputs the shared
// outcome replays: the resolved version, the deployment facts, and the
// ordered pre-state — exactly what the transition consults.
func mysqlResourceGroupEvidence(statement spec.Statement) (*spec.VersionIdentity, *spec.InstanceFacts, *spec.MySQLResourceGroupState) {
	var version *spec.VersionIdentity
	var facts *spec.InstanceFacts
	if statement.Metadata != nil {
		version = statement.Metadata.Version
		facts = statement.Metadata.Instance
	}
	return version, facts, mysqlResourceGroupBinding(statement)
}

// mysqlResourceGroupOutcome evaluates the shared truth table for one
// statement — the single adjudication the rules and the transition
// share.
func mysqlResourceGroupOutcome(statement spec.Statement) spec.MySQLResourceGroupOutcome {
	version, facts, state := mysqlResourceGroupEvidence(statement)
	return spec.MySQLResourceGroupStatementOutcome(statement.DDL.Operation,
		statement.DDL.MySQLResourceGroup, version, facts, state)
}

// mysqlResourceGroupStatementRefused reports whether the statement
// provably cannot take effect in every consistent world — declared-
// effect warnings, success notices, and premise-gap emission consume it
// so a refused statement never claims its authored config will apply or
// owes moot evidence.
func mysqlResourceGroupStatementRefused(statement spec.Statement) bool {
	return mysqlResourceGroupOutcome(statement).Refused
}

func mysqlResourceGroupFinding(level rule.Level, message, suggestion, operation string, state *spec.MySQLResourceGroupState) rule.Finding {
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

// mysqlResourceGroupValidRule adjudicates the proven refusals of MySQL
// resource-group lifecycle statements through the shared outcome: the
// version and deployment capability premises, name bound, privilege,
// reserved builtin pair, existence, FORCE pairing, inverted VCPU
// ranges, host-capacity endpoints, capability-bound priorities, and
// DROP occupancy. Unproven facts emit bounded gaps — never fabricated
// answers.
type mysqlResourceGroupValidRule struct {
	level rule.Level
}

func newMySQLResourceGroupValidRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return mysqlResourceGroupValidRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r mysqlResourceGroupValidRule) ID() string { return ruleIDMySQLResourceGroupValid }

func (r mysqlResourceGroupValidRule) AppliesTo(statement spec.Statement) bool {
	return mysqlResourceGroupApplies(statement)
}

func (r mysqlResourceGroupValidRule) Evaluate(_ context.Context, statement spec.Statement) ([]rule.Finding, error) {
	decl := statement.DDL.MySQLResourceGroup
	state := mysqlResourceGroupBinding(statement)
	operation := string(statement.DDL.Operation)
	outcome := mysqlResourceGroupOutcome(statement)
	findings := make([]rule.Finding, 0, len(outcome.Reasons))
	for _, reason := range outcome.Reasons {
		findings = append(findings, mysqlResourceGroupFinding(r.level,
			fmt.Sprintf("%s on %q is refused: %s.", operation, decl.Name, reason),
			mysqlResourceGroupRefusalSuggestion(reason), operation, state))
	}
	return findings, nil
}

// mysqlResourceGroupRefusalSuggestion pairs each refusal reason with
// the actionable hint — the reasons are the shared outcome's upstream-
// ordered verdicts.
func mysqlResourceGroupRefusalSuggestion(reason string) string {
	switch {
	case strings.Contains(reason, "8.0.3"):
		return "upgrade to MySQL 8.0.3+ or remove the statement"
	case strings.Contains(reason, "does not support resource groups"):
		return "the deployment lacks the resource-group feature — remove the statement"
	case strings.Contains(reason, "64-byte"):
		return "shorten the group name to 64 bytes or fewer"
	case strings.Contains(reason, "RESOURCE_GROUP_ADMIN"):
		return "grant RESOURCE_GROUP_ADMIN or remove the statement"
	case strings.Contains(reason, "reserved builtin"):
		return "the builtin groups are managed by the server — use a named group instead"
	case strings.Contains(reason, "already exists"):
		return "alter the existing group instead — MySQL has no CREATE RESOURCE GROUP IF NOT EXISTS"
	case strings.Contains(reason, "does not exist"):
		return "create the resource group first — MySQL has no IF EXISTS guard on this surface"
	case strings.Contains(reason, "FORCE is valid only"):
		return "pair FORCE with DISABLE or remove it"
	case strings.Contains(reason, "inverted"):
		return "order the VCPU range low-to-high"
	case strings.Contains(reason, "CPU count"):
		return "keep VCPU ids inside the host's CPU range"
	case strings.Contains(reason, "permitted range"):
		return "keep THREAD_PRIORITY inside the group type's permitted range"
	case strings.Contains(reason, "threads are bound"):
		return "unbind the threads first or add FORCE to relocate them to a default group"
	}
	return "resolve the refusal before applying the statement"
}

// EvidenceGaps reports the bounded missing-fact contracts the shared
// outcome left open — each gap is a premise the verdict still depends
// on. A proven refusal emits none: the missing facts cannot change the
// verdict.
func (r mysqlResourceGroupValidRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	outcome := mysqlResourceGroupOutcome(statement)
	gaps := make([]rule.EvidenceGap, 0, len(outcome.Gaps))
	for _, gap := range outcome.Gaps {
		gaps = append(gaps, rule.EvidenceGap{
			ReasonCode:    gap.ReasonCode,
			RequiredFacts: gap.RequiredFacts,
		})
	}
	return gaps
}

// mysqlResourceGroupForceWarnRule reports an authored FORCE member —
// ALTER ... DISABLE FORCE or DROP ... FORCE relocates threads bound to
// the group to the default groups. The warning names only the declared
// relocation, never a claim it fired.
type mysqlResourceGroupForceWarnRule struct {
	level rule.Level
}

func newMySQLResourceGroupForceWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return mysqlResourceGroupForceWarnRule{level: configuredLevel(cfg, rule.LevelWarning)}, nil
}

func (r mysqlResourceGroupForceWarnRule) ID() string { return ruleIDMySQLResourceGroupForceWarn }

func (r mysqlResourceGroupForceWarnRule) AppliesTo(statement spec.Statement) bool {
	return mysqlResourceGroupApplies(statement) &&
		(statement.DDL.Operation == spec.DDLOperationAlterResourceGroup ||
			statement.DDL.Operation == spec.DDLOperationDropResourceGroup) &&
		statement.DDL.MySQLResourceGroup.DeclaredForce()
}

func (r mysqlResourceGroupForceWarnRule) Evaluate(_ context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if mysqlResourceGroupStatementRefused(statement) {
		return nil, nil
	}
	state := mysqlResourceGroupBinding(statement)
	return []rule.Finding{mysqlResourceGroupFinding(r.level,
		"FORCE relocates threads bound to the resource group to the default groups while altering or dropping it.",
		"verify bound workloads tolerate relocation to a default group", "force", state)}, nil
}

// mysqlResourceGroupAffinityWarnRule reports an authored VCPU clause —
// CPU affinity constrains the group to a declared CPU set. The warning
// names only the declared pinning, never achieved placement.
type mysqlResourceGroupAffinityWarnRule struct {
	level rule.Level
}

func newMySQLResourceGroupAffinityWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return mysqlResourceGroupAffinityWarnRule{level: configuredLevel(cfg, rule.LevelWarning)}, nil
}

func (r mysqlResourceGroupAffinityWarnRule) ID() string { return ruleIDMySQLResourceGroupAffinityWarn }

func (r mysqlResourceGroupAffinityWarnRule) AppliesTo(statement spec.Statement) bool {
	if !mysqlResourceGroupApplies(statement) ||
		(statement.DDL.Operation != spec.DDLOperationCreateResourceGroup &&
			statement.DDL.Operation != spec.DDLOperationAlterResourceGroup) {
		return false
	}
	_, ok := statement.DDL.MySQLResourceGroup.DeclaredVCPU()
	return ok
}

func (r mysqlResourceGroupAffinityWarnRule) Evaluate(_ context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if mysqlResourceGroupStatementRefused(statement) {
		return nil, nil
	}
	state := mysqlResourceGroupBinding(statement)
	return []rule.Finding{mysqlResourceGroupFinding(r.level,
		"VCPU pins the resource group to a declared CPU set; threads in the group cannot use other CPUs.",
		"verify the pinned set matches the host's CPU capacity and the workload's needs", "vcpu", state)}, nil
}

// mysqlResourceGroupPriorityWarnRule reports a declared nonzero
// THREAD_PRIORITY under a proven-unavailable platform capability —
// upstream's ER_ATTRIBUTE_IGNORED: the server warns, ignores the
// authored value, and stores zero. The warning names only the declared
// value's fate under that proven capability, never an achieved
// priority.
type mysqlResourceGroupPriorityWarnRule struct {
	level rule.Level
}

func newMySQLResourceGroupPriorityWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return mysqlResourceGroupPriorityWarnRule{level: configuredLevel(cfg, rule.LevelWarning)}, nil
}

func (r mysqlResourceGroupPriorityWarnRule) ID() string {
	return ruleIDMySQLResourceGroupPriorityWarn
}

func (r mysqlResourceGroupPriorityWarnRule) AppliesTo(statement spec.Statement) bool {
	if !mysqlResourceGroupApplies(statement) ||
		(statement.DDL.Operation != spec.DDLOperationCreateResourceGroup &&
			statement.DDL.Operation != spec.DDLOperationAlterResourceGroup) {
		return false
	}
	value, ok := statement.DDL.MySQLResourceGroup.DeclaredThreadPriority()
	return ok && value != nil && spec.MySQLResourceGroupEffectivePriority(*value) != 0
}

func (r mysqlResourceGroupPriorityWarnRule) Evaluate(_ context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if mysqlResourceGroupStatementRefused(statement) {
		return nil, nil
	}
	facts := (*spec.InstanceFacts)(nil)
	if statement.Metadata != nil {
		facts = statement.Metadata.Instance
	}
	if facts == nil || !facts.MySQLResourceGroupPriorityCapKnown || facts.MySQLResourceGroupPriorityCapable {
		// The ignore is certain only under a proven-off capability — an
		// unproven one owes the premise gap instead.
		return nil, nil
	}
	state := mysqlResourceGroupBinding(statement)
	return []rule.Finding{mysqlResourceGroupFinding(r.level,
		"THREAD_PRIORITY is ignored on this platform — the deployment cannot apply thread priorities, so the effective value stores zero.",
		"remove the declared priority or verify the platform's thread-priority support", "thread_priority", state)}, nil
}
