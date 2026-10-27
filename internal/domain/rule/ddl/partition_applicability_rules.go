// Package ddl: T13-R1 partition applicability rules.
//
// The AST risk rules (partition_rules.go) fire on the authored action
// alone; the rules here bind the action to proven partition metadata:
// primary partitioned state, partition method, named-member existence,
// and EXCHANGE counterpart identity/compatibility. The verdict table is
// evidence-bounded:
//   - DROP PARTITION executes on range|list types and refuses on
//     hash|key types (native 1512).
//   - Any partition action on a known non-partitioned primary refuses
//     (native 1505).
//   - A named partition member missing from an observed member set
//     refuses (native 1507 DROP, 1735 TRUNCATE/EXCHANGE).
//   - EXCHANGE counterpart missing (1146), partitioned (1732), or
//     definition-mismatched (1736) refuses.
//
// Facts that were never observed stay unknown and surface as evidence
// gaps on the owning rule — never fabricated into a refusal finding.
package ddl

import (
	"context"
	"fmt"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// Reason codes emitted as finding metadata on proven applicability cells.
const (
	partitionReasonNotPartitioned         = "not_partitioned"
	partitionReasonTypeInapplicable       = "partition_type_inapplicable"
	partitionReasonPartitionAbsent        = "partition_absent"
	partitionReasonCounterpartAbsent      = "counterpart_absent"
	partitionReasonCounterpartPartitioned = "counterpart_partitioned"
	partitionReasonDefinitionMismatch     = "counterpart_definition_mismatch"
)

// gapReasonPartitionFacts reuses the shared unknown-table-state reason:
// the partition family owes bounded catalog facts the probe never
// produced — the fact names carry the partition-specific identity.
const gapReasonPartitionFacts = gapReasonUnknownTableState

// partitionActionApplicableTypes lists the partition methods that allow
// DROP PARTITION on both anchors (1512 on hash|key families). TRUNCATE and
// EXCHANGE execute on every observed method, so only drop carries the
// gate.
var partitionActionApplicableTypes = map[string]map[string]bool{
	"drop_partition": {
		"range": true, "range columns": true,
		"list": true, "list columns": true,
	},
}

// partitionApplicabilityRule gates one partition action on the primary
// table's observed partition state and method.
type partitionApplicabilityRule struct {
	ruleID  string
	action  string
	label   string
	enabled bool
	level   rule.Level
}

func newPartitionApplicabilityRule(ruleID, action, label string, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return partitionApplicabilityRule{
		ruleID:  ruleID,
		action:  action,
		label:   label,
		enabled: cfg.Enabled,
		level:   configuredLevel(cfg, rule.LevelBlocker),
	}, nil
}

func newDropPartitionApplicabilityRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionApplicabilityRule(ruleIDAlterDropPartitionApplicability, "drop_partition", "drop partition", cfg)
}

func newTruncatePartitionApplicabilityRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionApplicabilityRule(ruleIDAlterTruncatePartitionApplicability, "truncate_partition", "truncate partition", cfg)
}

func newExchangePartitionApplicabilityRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionApplicabilityRule(ruleIDAlterExchangePartitionApplicability, "exchange_partition", "exchange partition", cfg)
}

func (r partitionApplicabilityRule) ID() string { return r.ruleID }

func (r partitionApplicabilityRule) AppliesTo(statement spec.Statement) bool {
	return r.enabled && orderedGapDialect(statement.Dialect) &&
		len(matchingAlterActions(statement, r.action)) > 0
}

func (r partitionApplicabilityRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	snapshot, ok := targetTableSnapshot(statement)
	// Unvouched or known-absent primaries own no finding here — the gap
	// path reports the missing fact and the existence rules own the
	// absent-table cell.
	if !ok || snapshot == nil || !snapshot.Exists || snapshot.Partitioned == nil {
		return nil, nil
	}
	if !*snapshot.Partitioned {
		return partitionApplicabilityFindings(r, statement, snapshot,
			partitionReasonNotPartitioned, "1505",
			fmt.Sprintf("ALTER TABLE %s on a non-partitioned table cannot execute (native 1505)", r.label)), nil
	}
	allowed, typeGated := partitionActionApplicableTypes[r.action]
	if typeGated {
		if snapshot.PartitionType == nil {
			return nil, nil
		}
		if !allowed[*snapshot.PartitionType] {
			return partitionApplicabilityFindings(r, statement, snapshot,
				partitionReasonTypeInapplicable, "1512",
				fmt.Sprintf("ALTER TABLE %s is not allowed on %s-partitioned tables (native 1512)", r.label, *snapshot.PartitionType)), nil
		}
	}
	return nil, ctx.Err()
}

func partitionApplicabilityFindings(r partitionApplicabilityRule, statement spec.Statement, snapshot *spec.TableSnapshot, reasonCode, nativeError, message string) []rule.Finding {
	findings := make([]rule.Finding, 0, len(statement.DDL.Alter))
	for _, alter := range statement.DDL.Alter {
		if alter.Action != r.action {
			continue
		}
		metadata := partitionMetadata(statement, alter, map[string]any{
			"reason_code":  reasonCode,
			"native_error": nativeError,
			"exists":       "true",
			"partitioned":  fmt.Sprintf("%t", *snapshot.Partitioned),
		})
		if snapshot.PartitionType != nil {
			metadata["partition_type"] = *snapshot.PartitionType
		}
		if len(alter.Partitions) > 0 {
			metadata["partition"] = alter.Partitions[0]
		}
		if alter.ExchangeTable != nil {
			metadata["exchange_table"] = alter.ExchangeTable.Name
			metadata["exchange_schema"] = alter.ExchangeTable.Schema
		}
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    message,
			Suggestion: fmt.Sprintf("verify the target actually carries %s-compatible partitioning before shipping", r.label),
			Metadata:   metadata,
		})
	}
	return findings
}

// EvidenceGaps reports the partition-state facts the rule needed but the
// projection never answered. Unknown primary state reports the partitioned
// fact; a known-partitioned primary that never answered PARTITION_METHOD
// under a type-gated action reports the type fact. Facts the rule does
// not consume (engine, members, counterpart) are never owed here.
func (r partitionApplicabilityRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || snapshot == nil {
		return []rule.EvidenceGap{{
			ReasonCode:    gapReasonPartitionFacts,
			RequiredFacts: []string{"target_table.partitioned"},
		}}
	}
	if !snapshot.Exists {
		return nil
	}
	if snapshot.Partitioned == nil {
		return []rule.EvidenceGap{{
			ReasonCode:    gapReasonPartitionFacts,
			RequiredFacts: []string{"target_table.partitioned"},
		}}
	}
	if _, typeGated := partitionActionApplicableTypes[r.action]; typeGated &&
		*snapshot.Partitioned && snapshot.PartitionType == nil {
		return []rule.EvidenceGap{{
			ReasonCode:    gapReasonPartitionFacts,
			RequiredFacts: []string{"target_table.partition_type"},
		}}
	}
	return nil
}

// partitionTargetRule checks each named partition member against the
// observed member set of a known-partitioned primary.
type partitionTargetRule struct {
	ruleID      string
	action      string
	label       string
	nativeError string
	enabled     bool
	level       rule.Level
}

func newPartitionTargetRule(ruleID, action, label, nativeError string, cfg policy.RulePolicy) (rule.StatementRule, error) {
	return partitionTargetRule{
		ruleID:      ruleID,
		action:      action,
		label:       label,
		nativeError: nativeError,
		enabled:     cfg.Enabled,
		level:       configuredLevel(cfg, rule.LevelBlocker),
	}, nil
}

func newDropPartitionTargetRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionTargetRule(ruleIDAlterDropPartitionTarget, "drop_partition", "drop partition", "1507", cfg)
}

func newTruncatePartitionTargetRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionTargetRule(ruleIDAlterTruncatePartitionTarget, "truncate_partition", "truncate partition", "1735", cfg)
}

func newExchangePartitionTargetRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionTargetRule(ruleIDAlterExchangePartitionTarget, "exchange_partition", "exchange partition", "1735", cfg)
}

// newPartitionTargetExistsRule dispatches the member-existence rule for
// one authored partition action — the shared factory the registration
// table and tests bind by action name.
func newPartitionTargetExistsRule(action string, cfg policy.RulePolicy) (rule.StatementRule, error) {
	switch action {
	case "drop_partition":
		return newDropPartitionTargetRule(cfg)
	case "truncate_partition":
		return newTruncatePartitionTargetRule(cfg)
	case "exchange_partition":
		return newExchangePartitionTargetRule(cfg)
	default:
		return nil, fmt.Errorf("no partition target rule for action %q", action)
	}
}

func (r partitionTargetRule) ID() string { return r.ruleID }

func (r partitionTargetRule) AppliesTo(statement spec.Statement) bool {
	return r.enabled && orderedGapDialect(statement.Dialect) &&
		len(matchingAlterActions(statement, r.action)) > 0
}

func (r partitionTargetRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || snapshot == nil || !snapshot.Exists ||
		snapshot.Partitioned == nil || !*snapshot.Partitioned ||
		snapshot.PartitionNames == nil {
		return nil, nil
	}
	members := make(map[string]bool, len(snapshot.PartitionNames))
	for _, name := range snapshot.PartitionNames {
		members[strings.ToLower(name)] = true
	}
	findings := make([]rule.Finding, 0)
	for _, alter := range matchingAlterActions(statement, r.action) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if alter.AllPartitions {
			// The ALL form names no member — nothing to check.
			continue
		}
		for _, name := range alter.Partitions {
			if members[strings.ToLower(name)] {
				continue
			}
			findings = append(findings, rule.Finding{
				Level:      r.level,
				Message:    fmt.Sprintf("ALTER TABLE %s names partition %q, which is not a member of the table (native %s)", r.label, name, r.nativeError),
				Suggestion: "verify the partition name against the observed member set",
				Metadata: partitionMetadata(statement, alter, map[string]any{
					"partition":    name,
					"exists":       "false",
					"reason_code":  partitionReasonPartitionAbsent,
					"native_error": r.nativeError,
				}),
			})
		}
	}
	return findings, nil
}

// EvidenceGaps reports the member-set fact the rule needed. The gap is
// owed only when the member check itself could run — a primary whose
// partitioned state is unproven or proven-false does not owe the member
// fact (the applicability rule owns that arm).
func (r partitionTargetRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || snapshot == nil || !snapshot.Exists ||
		snapshot.Partitioned == nil || !*snapshot.Partitioned {
		return nil
	}
	namesConsumed := false
	for _, alter := range matchingAlterActions(statement, r.action) {
		if !alter.AllPartitions && len(alter.Partitions) > 0 {
			namesConsumed = true
			break
		}
	}
	if !namesConsumed {
		// The ALL form names no member — the member set is never
		// consulted, so no fact is owed.
		return nil
	}
	if snapshot.PartitionNames == nil {
		return []rule.EvidenceGap{{
			ReasonCode:    gapReasonPartitionFacts,
			RequiredFacts: []string{"target_table.partitions"},
		}}
	}
	return nil
}

// exchangeCounterpartRule binds the EXCHANGE counterpart identity to its
// observed pre-state: known-absent (1146), known-partitioned (1732), or
// structurally incompatible on the bounded column/PK compare (1736) are
// proven refusals. The rule never claims the exchange validated — a clean
// bounded compare stays silent.
type exchangeCounterpartRule struct {
	ruleID  string
	enabled bool
	level   rule.Level
}

func newExchangeCounterpartCompatibilityRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return exchangeCounterpartRule{
		ruleID:  ruleIDAlterExchangePartitionCounterpart,
		enabled: cfg.Enabled,
		level:   configuredLevel(cfg, rule.LevelBlocker),
	}, nil
}

func (r exchangeCounterpartRule) ID() string { return r.ruleID }

func (r exchangeCounterpartRule) AppliesTo(statement spec.Statement) bool {
	return r.enabled && orderedGapDialect(statement.Dialect) &&
		len(matchingAlterActions(statement, "exchange_partition")) > 0
}

func (r exchangeCounterpartRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	counterpart := counterpartSnapshot(statement)
	if counterpart == nil {
		return nil, nil
	}
	findings := make([]rule.Finding, 0)
	for _, alter := range matchingAlterActions(statement, "exchange_partition") {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !counterpart.Exists {
			findings = append(findings, r.counterpartFinding(statement, alter,
				partitionReasonCounterpartAbsent, "1146",
				fmt.Sprintf("ALTER TABLE exchange partition counterpart %q does not exist (native 1146)", exchangeTableName(alter)), "false"))
			continue
		}
		if counterpart.Partitioned != nil && *counterpart.Partitioned {
			findings = append(findings, r.counterpartFinding(statement, alter,
				partitionReasonCounterpartPartitioned, "1732",
				fmt.Sprintf("ALTER TABLE exchange partition counterpart %q is itself partitioned (native 1732)", exchangeTableName(alter)), "true"))
			continue
		}
		if counterpart.Partitioned == nil {
			continue
		}
		primary, _ := targetTableSnapshot(statement)
		if primary == nil || !primary.Exists {
			continue
		}
		if mismatch, comparable := exchangeDefinitionMismatch(primary, counterpart); comparable && mismatch {
			findings = append(findings, r.counterpartFinding(statement, alter,
				partitionReasonDefinitionMismatch, "1736",
				fmt.Sprintf("ALTER TABLE exchange partition counterpart %q has a different definition (native 1736)", exchangeTableName(alter)), "true"))
		}
	}
	return findings, nil
}

func (r exchangeCounterpartRule) counterpartFinding(statement spec.Statement, alter spec.Alter, reasonCode, nativeError, message, exists string) rule.Finding {
	metadata := partitionMetadata(statement, alter, map[string]any{
		"exists":       exists,
		"reason_code":  reasonCode,
		"native_error": nativeError,
	})
	if len(alter.Partitions) > 0 {
		metadata["partition"] = alter.Partitions[0]
	}
	if alter.ExchangeTable != nil {
		metadata["exchange_table"] = alter.ExchangeTable.Name
		metadata["exchange_schema"] = alter.ExchangeTable.Schema
	}
	return rule.Finding{
		Level:      r.level,
		Message:    message,
		Suggestion: "verify the counterpart is a non-partitioned table with a matching definition",
		Metadata:   metadata,
	}
}

// EvidenceGaps reports the counterpart and compare facts the rule needed
// but the projection never answered. Each missing premise names its own
// fact — an unresolved counterpart identity reports existence; a resolved
// one reports partitioned state, then the bounded compare inputs.
func (r exchangeCounterpartRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) || !hasExchangeCounterpart(statement) {
		return nil
	}
	counterpart := counterpartSnapshot(statement)
	if counterpart == nil {
		return []rule.EvidenceGap{{
			ReasonCode:    gapReasonPartitionFacts,
			RequiredFacts: []string{"exchange_table.existence"},
		}}
	}
	if !counterpart.Exists {
		return nil
	}
	if counterpart.Partitioned == nil {
		return []rule.EvidenceGap{{
			ReasonCode:    gapReasonPartitionFacts,
			RequiredFacts: []string{"exchange_table.partitioned"},
		}}
	}
	if *counterpart.Partitioned {
		return nil
	}
	primary, _ := targetTableSnapshot(statement)
	var missing []string
	if primary == nil || !primary.Exists || primary.Columns == nil {
		missing = append(missing, "target_table.columns")
	}
	if counterpart.Columns == nil {
		missing = append(missing, "exchange_table.columns")
	}
	if primary == nil || !primary.Exists || primary.PrimaryKeyUnknown {
		missing = append(missing, "target_table.primary_key")
	}
	if counterpart.PrimaryKeyUnknown {
		missing = append(missing, "exchange_table.primary_key")
	}
	if len(missing) == 0 {
		return nil
	}
	return []rule.EvidenceGap{{
		ReasonCode:    gapReasonPartitionFacts,
		RequiredFacts: missing,
	}}
}

// counterpartSnapshot returns the resolved EXCHANGE counterpart projection,
// or nil when the statement carries no resolvable counterpart identity.
func counterpartSnapshot(statement spec.Statement) *spec.TableSnapshot {
	if statement.Metadata == nil {
		return nil
	}
	return statement.Metadata.ExchangeTable
}

// hasExchangeCounterpart reports whether the statement's exchange clause
// named a counterpart table — the bound identity fact gaps can be owed
// for. An unbound clause owes nothing here.
func hasExchangeCounterpart(statement spec.Statement) bool {
	for _, alter := range matchingAlterActions(statement, "exchange_partition") {
		if alter.ExchangeTable != nil && strings.TrimSpace(alter.ExchangeTable.Name) != "" {
			return true
		}
	}
	return false
}

// exchangeDefinitionMismatch compares the bounded structural facts the
// exchange requires to match: column name/type sequence and primary-key
// identity. comparable=false means at least one side could not vouch the
// compare inputs — never read as matching.
func exchangeDefinitionMismatch(primary, counterpart *spec.TableSnapshot) (mismatch, comparable bool) {
	if primary == nil || counterpart == nil ||
		primary.Columns == nil || counterpart.Columns == nil ||
		primary.PrimaryKeyUnknown || counterpart.PrimaryKeyUnknown {
		return false, false
	}
	if len(primary.Columns) != len(counterpart.Columns) {
		return true, true
	}
	for i := range primary.Columns {
		if !exchangeColumnEqual(primary.Columns[i], counterpart.Columns[i]) {
			return true, true
		}
	}
	pkPresent, pkMembers, pkOK := primaryKeyIdentityFacts(primary)
	cpPresent, cpMembers, cpOK := primaryKeyIdentityFacts(counterpart)
	if !pkOK || !cpOK {
		return false, false
	}
	if pkPresent != cpPresent {
		return true, true
	}
	if pkPresent && !equalFoldedNames(pkMembers, cpMembers) {
		return true, true
	}
	return false, true
}

// exchangeColumnEqual compares the bounded column-definition facts the
// exchange validates: name, normalized type family, declared length, and
// signedness.
func exchangeColumnEqual(a, b spec.Column) bool {
	return strings.EqualFold(a.Name, b.Name) &&
		baseType(a) == baseType(b) &&
		a.Length == b.Length &&
		a.Unsigned == b.Unsigned
}

// primaryKeyIdentityFacts resolves PK presence and member columns from the
// vouched member facts. ok=false means the fact could not be vouched —
// never read as "no PK".
func primaryKeyIdentityFacts(snapshot *spec.TableSnapshot) (present bool, members []string, ok bool) {
	if snapshot.PrimaryKey != nil {
		return true, snapshot.PrimaryKey.Columns, true
	}
	if snapshot.ConstraintsUnknown {
		return false, nil, false
	}
	for _, constraint := range snapshot.Constraints {
		if constraint.Type == "primary_key" {
			return true, constraint.Columns, true
		}
	}
	return false, nil, true
}

func equalFoldedNames(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !strings.EqualFold(a[i], b[i]) {
			return false
		}
	}
	return true
}
