// Package ddl: T13 destructive/data-moving partition alter rules.
//
// DROP/TRUNCATE PARTITION are governed by forbid rules (default blocker);
// EXCHANGE PARTITION is governed by a data-move warning. All three fire on
// the authored action alone — partition existence, partition type, and
// exchange compatibility are unmodeled metadata facts and are never
// fabricated into a finding or a gap.
package ddl

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type partitionForbidRule struct {
	ruleID string
	action string
	label  string
	level  rule.Level
	forbid bool
}

func newPartitionForbidRule(ruleID, action, label string, cfg policy.RulePolicy) (rule.StatementRule, error) {
	forbid, err := boolParam(ruleID, cfg, "forbid", true)
	if err != nil {
		return nil, err
	}
	return partitionForbidRule{
		ruleID: ruleID,
		action: action,
		label:  label,
		level:  configuredLevel(cfg, rule.LevelBlocker),
		forbid: forbid,
	}, nil
}

func newDropPartitionForbidRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionForbidRule(ruleIDAlterDropPartitionForbid, "drop_partition", "drop partition", cfg)
}

func newTruncatePartitionForbidRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return newPartitionForbidRule(ruleIDAlterTruncatePartitionForbid, "truncate_partition", "truncate partition", cfg)
}

func (r partitionForbidRule) ID() string { return r.ruleID }

func (r partitionForbidRule) AppliesTo(statement spec.Statement) bool {
	return r.forbid && appliesToAlterTable(statement)
}

func (r partitionForbidRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	findings := make([]rule.Finding, 0)
	for _, alter := range matchingForbiddenAlterActions(statement, r.action) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if alter.Action != r.action {
			continue
		}
		findings = append(findings, partitionActionFindings(r, statement, alter)...)
	}
	return findings, nil
}

func partitionActionFindings(r partitionForbidRule, statement spec.Statement, alter spec.Alter) []rule.Finding {
	// The ALL form is one finding with no fabricated partition name.
	if alter.AllPartitions {
		return []rule.Finding{{
			Level:      r.level,
			Message:    fmt.Sprintf("ALTER TABLE %s is forbidden for ALL partitions", r.label),
			Suggestion: fmt.Sprintf("avoid %s in this change or relax the policy intentionally", r.label),
			Metadata:   partitionMetadata(statement, alter, map[string]any{"all_partitions": "true"}),
		}}
	}
	if len(alter.Partitions) == 0 {
		// No name could be attributed — still flag the action itself rather
		// than silently passing a destructive operation.
		return []rule.Finding{{
			Level:      r.level,
			Message:    fmt.Sprintf("ALTER TABLE %s is forbidden", r.label),
			Suggestion: fmt.Sprintf("avoid %s in this change or relax the policy intentionally", r.label),
			Metadata:   partitionMetadata(statement, alter, nil),
		}}
	}
	findings := make([]rule.Finding, 0, len(alter.Partitions))
	for _, name := range alter.Partitions {
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    fmt.Sprintf("ALTER TABLE %s is forbidden for partition %q", r.label, name),
			Suggestion: fmt.Sprintf("avoid %s in this change or relax the policy intentionally", r.label),
			Metadata:   partitionMetadata(statement, alter, map[string]any{"partition": name}),
		})
	}
	return findings
}

func partitionMetadata(statement spec.Statement, alter spec.Alter, extra map[string]any) map[string]any {
	metadata := map[string]any{"action": alter.Action}
	if statement.DDL.Table != nil && statement.DDL.Table.Name != "" {
		metadata["table"] = statement.DDL.Table.Name
	}
	for key, value := range extra {
		metadata[key] = value
	}
	return metadata
}

// exchangePartitionWarnRule flags the data move between a partition and a
// plain table. The warning reports the authored clause and both object
// identities; it never claims the engine validated compatibility.
type exchangePartitionWarnRule struct {
	ruleID string
	level  rule.Level
}

func newExchangePartitionWarnRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return exchangePartitionWarnRule{
		ruleID: ruleIDAlterExchangePartitionWarn,
		level:  configuredLevel(cfg, rule.LevelWarning),
	}, nil
}

func (r exchangePartitionWarnRule) ID() string { return r.ruleID }

func (r exchangePartitionWarnRule) AppliesTo(statement spec.Statement) bool {
	return appliesToAlterTable(statement)
}

func (r exchangePartitionWarnRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	findings := make([]rule.Finding, 0)
	for _, alter := range matchingForbiddenAlterActions(statement, "exchange_partition") {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if alter.Action != "exchange_partition" {
			continue
		}
		metadata := partitionMetadata(statement, alter, nil)
		if len(alter.Partitions) > 0 {
			metadata["partition"] = alter.Partitions[0]
		}
		if alter.ExchangeTable != nil {
			metadata["exchange_table"] = alter.ExchangeTable.Name
			metadata["exchange_schema"] = alter.ExchangeTable.Schema
		}
		metadata["validation"] = alter.ExchangeValidation
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    fmt.Sprintf("ALTER TABLE exchange partition swaps rows between partition %q and table %q", alter.Name, exchangeTableName(alter)),
			Suggestion: "verify the exchange table is not partitioned, its rows satisfy the partition bounds, and WITH/WITHOUT VALIDATION matches your intent",
			Metadata:   metadata,
		})
	}
	return findings, nil
}

func exchangeTableName(alter spec.Alter) string {
	if alter.ExchangeTable == nil {
		return ""
	}
	if alter.ExchangeTable.Schema == "" {
		return alter.ExchangeTable.Name
	}
	return alter.ExchangeTable.Schema + "." + alter.ExchangeTable.Name
}
