package ddl

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// mysql_resource_group_rules.go implements the T24/#103 MySQL CPU
// resource-group lifecycle rules. They adjudicate only what the ordered
// state, the authored declaration, and the resolved version prove:
// existence, the reserved builtin pair, the version capability premise,
// name length, and the authored priority/type bounds. CPU affinity
// effects, achieved priorities, and workload enforcement are
// configuration facts — never runtime measurements.

const (
	mysqlResourceGroupGapUnknownState     = "unknown_resource_group_state"
	mysqlResourceGroupGapUnknownVersion   = "unknown_resource_group_version"
	mysqlResourceGroupGapUnknownHostVCPU  = "unknown_host_vcpu_count"
	mysqlResourceGroupGapUnknownPrivilege = "unknown_resource_group_privilege"
	mysqlResourceGroupFactState           = "resource_group.state"
	mysqlResourceGroupFactVersion         = "instance.version"
	mysqlResourceGroupFactHostVCPU        = "host.vcpu_count"
	mysqlResourceGroupFactPrivilege       = "privilege.resource_group_admin"
	mysqlResourceGroupMaxNameChars        = 64
)

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

// mysqlResourceGroupVersionRefused reports whether the resolved version
// proves the feature absent — MySQL 8.0 is the introduction version, so
// a proven older product version refuses every lifecycle statement
// before existence adjudication. An unproven version never claims a
// refusal.
func mysqlResourceGroupVersionRefused(statement spec.Statement) bool {
	if statement.Metadata == nil || statement.Metadata.Version == nil {
		return false
	}
	version := statement.Metadata.Version
	return version.ValidatedRange && version.Product == spec.VersionProductMySQL && version.Major < 8
}

// mysqlResourceGroupPriorityRefusal reports whether the authored
// THREAD_PRIORITY exceeds the group type's permitted range — system
// groups take -20..0, user groups 0..19. On CREATE the authored TYPE
// resolves the check; on ALTER the proven catalog type does; an
// unproven type never adjudicates.
func mysqlResourceGroupPriorityRefusal(operation spec.DDLOperation, decl *spec.MySQLResourceGroupDeclaration, state *spec.MySQLResourceGroupState) string {
	value, ok := decl.DeclaredThreadPriority()
	if !ok || value == nil {
		return ""
	}
	priority := *value
	var groupType spec.MySQLResourceGroupType
	typeKnown := false
	if operation == spec.DDLOperationCreateResourceGroup {
		groupType, typeKnown = decl.DeclaredType()
	} else if state != nil && state.Proven() && state.Exists &&
		state.Definition != nil && state.Definition.GroupType != "" {
		groupType = state.Definition.GroupType
		typeKnown = true
	}
	if !typeKnown {
		// With no proven type only the absolute -20..19 window is
		// adjudicable.
		if priority < -20 || priority > 19 {
			return fmt.Sprintf("THREAD_PRIORITY %d is outside the -20..19 range", priority)
		}
		return ""
	}
	switch groupType {
	case spec.MySQLResourceGroupTypeSystem:
		if priority < -20 || priority > 0 {
			return fmt.Sprintf("THREAD_PRIORITY %d is outside the SYSTEM range -20..0", priority)
		}
	case spec.MySQLResourceGroupTypeUser:
		if priority < 0 || priority > 19 {
			return fmt.Sprintf("THREAD_PRIORITY %d is outside the USER range 0..19", priority)
		}
	}
	return ""
}

// mysqlResourceGroupProvenRefusal reports whether the ordered state
// already decides the statement's outcome without the version premise —
// a proven refusal keeps the version gap silent since the fact cannot
// change the verdict.
func mysqlResourceGroupProvenRefusal(statement spec.Statement, state *spec.MySQLResourceGroupState) bool {
	if state == nil || !state.Proven() || statement.DDL == nil || statement.DDL.MySQLResourceGroup == nil {
		return false
	}
	switch statement.DDL.Operation {
	case spec.DDLOperationCreateResourceGroup:
		// MySQL has no IF NOT EXISTS — a present target always refuses.
		return state.Exists
	case spec.DDLOperationAlterResourceGroup, spec.DDLOperationDropResourceGroup:
		return state.Reserved || !state.Exists
	}
	return false
}

// mysqlResourceGroupStatementRefused reports whether the statement
// provably cannot take effect: a proven version refusal, an over-long
// name, a static priority/type violation, or a proven existence/reserved
// refusal. Declared-effect warnings, success notices, and premise-gap
// emission consume it so a refused statement never claims its authored
// config will apply or owes moot evidence.
func mysqlResourceGroupStatementRefused(statement spec.Statement) bool {
	if mysqlResourceGroupVersionRefused(statement) {
		return true
	}
	if statement.DDL == nil || statement.DDL.MySQLResourceGroup == nil {
		return false
	}
	decl := statement.DDL.MySQLResourceGroup
	if utf8.RuneCountInString(decl.Name) > mysqlResourceGroupMaxNameChars {
		return true
	}
	if mysqlResourceGroupPriorityRefusal(statement.DDL.Operation, decl,
		mysqlResourceGroupBinding(statement)) != "" {
		return true
	}
	return mysqlResourceGroupProvenRefusal(statement, mysqlResourceGroupBinding(statement))
}

// mysqlResourceGroupValidRule adjudicates the existence and premise
// contracts of MySQL resource-group lifecycle statements: the version
// capability premise, the reserved builtin pair, duplicate option
// kinds, name length, the authored priority/type bounds, and proven
// existence outcomes. Unproven facts emit bounded gaps — never
// fabricated answers.
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

	// The version premise is a proven-refusal gate: resource groups
	// exist only on MySQL 8.0+, so a proven older version refuses
	// before anything else is adjudicated.
	if mysqlResourceGroupVersionRefused(statement) {
		return []rule.Finding{mysqlResourceGroupFinding(r.level,
			fmt.Sprintf("%s on %q requires MySQL 8.0 or later; the observed version does not support resource groups.", operation, decl.Name),
			"upgrade to MySQL 8.0+ or remove the statement", operation, state)}, nil
	}

	var findings []rule.Finding
	if utf8.RuneCountInString(decl.Name) > mysqlResourceGroupMaxNameChars {
		findings = append(findings, mysqlResourceGroupFinding(r.level,
			fmt.Sprintf("%s names a resource group longer than %d characters.", operation, mysqlResourceGroupMaxNameChars),
			"shorten the group name to 64 characters or fewer", operation, state))
	}
	if reason := mysqlResourceGroupPriorityRefusal(statement.DDL.Operation, decl, state); reason != "" {
		findings = append(findings, mysqlResourceGroupFinding(r.level,
			fmt.Sprintf("%s on %q declares a priority MySQL refuses: %s.", operation, decl.Name, reason),
			"keep THREAD_PRIORITY inside the group type's permitted range", operation, state))
	}
	if state == nil || !state.Proven() {
		return findings, nil
	}
	switch statement.DDL.Operation {
	case spec.DDLOperationCreateResourceGroup:
		if state.Exists {
			findings = append(findings, mysqlResourceGroupFinding(r.level,
				fmt.Sprintf("CREATE RESOURCE GROUP names the existing group %q.", state.Name),
				"alter the existing group instead — MySQL has no CREATE RESOURCE GROUP IF NOT EXISTS", operation, state))
		}
	case spec.DDLOperationAlterResourceGroup:
		if state.Reserved {
			findings = append(findings, mysqlResourceGroupFinding(r.level,
				fmt.Sprintf("ALTER RESOURCE GROUP names the reserved builtin group %q, whose attributes cannot be modified.", state.Name),
				"the builtin groups are managed by the server — create a named group instead", operation, state))
		} else if !state.Exists {
			findings = append(findings, mysqlResourceGroupFinding(r.level,
				fmt.Sprintf("ALTER RESOURCE GROUP names the absent group %q.", state.Name),
				"create the resource group first — MySQL has no ALTER RESOURCE GROUP IF EXISTS", operation, state))
		}
	case spec.DDLOperationDropResourceGroup:
		if state.Reserved {
			findings = append(findings, mysqlResourceGroupFinding(r.level,
				fmt.Sprintf("DROP RESOURCE GROUP names the reserved builtin group %q, which cannot be dropped.", state.Name),
				"the builtin groups are managed by the server — they cannot be dropped", operation, state))
		} else if !state.Exists {
			findings = append(findings, mysqlResourceGroupFinding(r.level,
				fmt.Sprintf("DROP RESOURCE GROUP names the absent group %q.", state.Name),
				"verify the group name — MySQL has no DROP RESOURCE GROUP IF EXISTS", operation, state))
		}
	}
	return findings, nil
}

// EvidenceGaps reports the bounded missing-fact contracts this rule
// owns: the version premise, the unproven target existence, the host
// CPU capacity a declared VCPU clause needs, and the
// RESOURCE_GROUP_ADMIN privilege every lifecycle statement requires.
// Gaps stay silent when the outcome is already a proven refusal on any
// ground — version, static, or existence — since the missing facts
// cannot change the verdict.
func (r mysqlResourceGroupValidRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	state := mysqlResourceGroupBinding(statement)
	var gaps []rule.EvidenceGap
	provenRefusal := mysqlResourceGroupStatementRefused(statement)
	if !provenRefusal {
		version := (*spec.VersionIdentity)(nil)
		if statement.Metadata != nil {
			version = statement.Metadata.Version
		}
		if version == nil || !version.ValidatedRange || version.Product != spec.VersionProductMySQL {
			gaps = append(gaps, rule.EvidenceGap{
				ReasonCode:    mysqlResourceGroupGapUnknownVersion,
				RequiredFacts: []string{mysqlResourceGroupFactVersion},
			})
		}
	} else {
		return nil
	}
	if state == nil || !state.Proven() {
		gaps = append(gaps, rule.EvidenceGap{
			ReasonCode:    mysqlResourceGroupGapUnknownState,
			RequiredFacts: []string{mysqlResourceGroupFactState},
		})
	}
	decl := statement.DDL.MySQLResourceGroup
	if _, ok := decl.DeclaredVCPU(); ok {
		gaps = append(gaps, rule.EvidenceGap{
			ReasonCode:    mysqlResourceGroupGapUnknownHostVCPU,
			RequiredFacts: []string{mysqlResourceGroupFactHostVCPU},
		})
	}
	gaps = append(gaps, rule.EvidenceGap{
		ReasonCode:    mysqlResourceGroupGapUnknownPrivilege,
		RequiredFacts: []string{mysqlResourceGroupFactPrivilege},
	})
	return gaps
}

// mysqlResourceGroupForceWarnRule reports an authored FORCE member —
// ALTER ... FORCE or DROP ... FORCE relocates threads bound to the
// group (or allows the drop while bound). The warning names only the
// declared relocation, never a claim it fired.
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
		"FORCE relocates threads bound to the resource group while altering or dropping it.",
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
