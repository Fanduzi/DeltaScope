package ddl

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// table_layout_r6_rules.go implements the T23-R6 TiDB physical-layout
// rules: table CACHE/NOCACHE, table/partition ATTRIBUTES, and SET
// TIFLASH REPLICA. The verdicts live in spec.AlterTableCacheOutcome /
// spec.AlterAttributesOutcome / spec.AlterTiFlashReplicaOutcome — the
// same functions the ordered-state publisher consumes (issue #102).

const (
	layoutR6FactCacheStatus = "table.cache_status"
	layoutR6FactTableSize   = "runtime.cache_table_size"
	layoutR6FactPartitioned = "table.partitioned"
	layoutR6FactMembers     = "table.partition_names"
	layoutR6FactCharset     = "table.columns.charset"
	layoutR6FactOldReplica  = "table.tiflash_replica"
	layoutR6FactStores      = "runtime.tiflash_store_count"
)

func layoutR6Targets(statement spec.Statement) []*spec.Alter {
	if statement.DDL == nil {
		return nil
	}
	var targets []*spec.Alter
	for i := range statement.DDL.Alter {
		targets = append(targets, &statement.DDL.Alter[i])
	}
	return targets
}

func layoutR6Applies(statement spec.Statement, action string) bool {
	return statement.Dialect == spec.DialectTiDB &&
		statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Operation == spec.DDLOperationAlterTable &&
		action != ""
}

func layoutR6Snapshot(statement spec.Statement) *spec.TableSnapshot {
	if statement.Metadata != nil {
		return statement.Metadata.TargetTable
	}
	return nil
}

func layoutR6Facts(statement spec.Statement) spec.AlterLayoutR6Facts {
	if snapshot := layoutR6Snapshot(statement); snapshot != nil {
		return spec.AlterLayoutR6FactsForSnapshot(snapshot)
	}
	return spec.AlterLayoutR6Facts{}
}

// --- ddl.tidb.table.cache.valid ---

type tableCacheValidRule struct{ level rule.Level }

func newTableCacheValidRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return tableCacheValidRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r tableCacheValidRule) ID() string { return ruleIDTiDBTableCacheValid }

// cacheGuardCoveredStatement reports whether the statement runs the
// upstream `TableCacheStatusType != Disable` gate: every ALTER spec
// except a single CACHE/NOCACHE reaches it inside ALTER, and the
// standalone DROP/TRUNCATE/RENAME/index operations evaluate the same
// guard unconditionally (issue #102 R7).
func cacheGuardCoveredStatement(statement spec.Statement) bool {
	if statement.DDL == nil {
		return false
	}
	switch statement.DDL.Operation {
	case spec.DDLOperationDropTable, spec.DDLOperationTruncateTable,
		spec.DDLOperationRenameTable, spec.DDLOperationCreateIndex,
		spec.DDLOperationDropIndex:
		return true
	}
	return false
}

// AppliesTo covers every alter spec — the cached-table guard refuses
// non-cache actions on a proven cache-enabled table too — plus the
// standalone mutation operations that share the same upstream gate.
func (r tableCacheValidRule) AppliesTo(statement spec.Statement) bool {
	if statement.Dialect != spec.DialectTiDB || statement.Kind != spec.KindDDL {
		return false
	}
	if cacheGuardCoveredStatement(statement) {
		return true
	}
	return layoutR6Applies(statement, "any") && len(layoutR6Targets(statement)) > 0
}

func cacheRefusalText(reason string) (message, suggestion string) {
	switch reason {
	case spec.LayoutReasonCachedTableGuard:
		return "cannot run on a cache-enabled table: TiDB allows only ALTER TABLE ... CACHE|NOCACHE there",
			"run ALTER TABLE ... NOCACHE first, then the structural change"
	case spec.LayoutReasonCacheSysTable:
		return "cannot cache a system or memory database table",
			"cache only user-schema tables"
	case spec.LayoutReasonCacheTemporary:
		return "cannot cache a temporary table",
			"cache only base tables"
	case spec.LayoutReasonCachePartitioned:
		return "cannot cache a partitioned table",
			"cache only non-partitioned tables"
	default:
		return "is statically refused by TiDB",
			"adjust the CACHE declaration"
	}
}

func (r tableCacheValidRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	facts := layoutR6Facts(statement)
	targets := layoutR6Targets(statement)
	findings := make([]rule.Finding, 0)
	// Standalone covered operations evaluate the `!= Disable` gate
	// unconditionally — a proven non-Disable status is a native refusal.
	if cacheGuardCoveredStatement(statement) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if verdict, reason := spec.CacheGuardOutcome(facts); verdict == spec.AutoRandomRefused {
			message, suggestion := cacheRefusalText(reason)
			findings = append(findings, rule.Finding{
				Level:      r.level,
				Message:    fmt.Sprintf("%s on table %q %s", statement.DDL.Operation, statement.DDL.Table.Name, message),
				Suggestion: suggestion,
				Metadata: map[string]any{
					"table":  statement.DDL.Table.Name,
					"action": string(statement.DDL.Operation),
					"reason": reason,
				},
			})
		}
		return findings, nil
	}
	specCount := spec.CacheGuardSpecCount(statement.DDL.Alter)
	for _, alter := range targets {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var verdict spec.AutoRandomVerdict
		var reason string
		switch alter.Action {
		case "cache", "nocache":
			verdict, reason = spec.AlterTableCacheOutcome(alter.Action, facts)
		default:
			if spec.CachedTableGuardRefused(alter.Action, specCount, facts) {
				verdict, reason = spec.AutoRandomRefused, spec.LayoutReasonCachedTableGuard
			}
		}
		if verdict != spec.AutoRandomRefused {
			continue
		}
		message, suggestion := cacheRefusalText(reason)
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    fmt.Sprintf("alter action %q on table %q %s", alter.Action, statement.DDL.Table.Name, message),
			Suggestion: suggestion,
			Metadata: map[string]any{
				"table":  statement.DDL.Table.Name,
				"action": alter.Action,
				"reason": reason,
			},
		})
	}
	return findings, nil
}

func (r tableCacheValidRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	facts := layoutR6Facts(statement)
	required := map[string]bool{}
	// Covered operations need the status premise unconditionally; an
	// unobserved status is the precise missing fact (issue #102 R7).
	if cacheGuardCoveredStatement(statement) {
		if !facts.CacheStatusKnown {
			required[layoutR6FactCacheStatus] = true
		}
	} else {
		specCount := spec.CacheGuardSpecCount(statement.DDL.Alter)
		guardPremise := false
		for i := range statement.DDL.Alter {
			if spec.CacheGuardPremiseNeeded(statement.DDL.Alter[i].Action, specCount) {
				guardPremise = true
				break
			}
		}
		if guardPremise && !facts.CacheStatusKnown {
			required[layoutR6FactCacheStatus] = true
		}
		for _, alter := range layoutR6Targets(statement) {
			if alter.Action != "cache" && alter.Action != "nocache" {
				continue
			}
			verdict, reason := spec.AlterTableCacheOutcome(alter.Action, facts)
			if verdict != spec.AutoRandomUndecidable {
				continue
			}
			switch reason {
			case spec.LayoutReasonCacheSizeUnproven:
				required[layoutR6FactTableSize] = true
			case spec.LayoutReasonCacheStatusUnproven:
				required[layoutR6FactCacheStatus] = true
			case spec.LayoutReasonPartitionedUnproven:
				required[layoutR6FactPartitioned] = true
			}
		}
	}
	if len(required) == 0 {
		return nil
	}
	list := make([]string, 0, len(required))
	for fact := range required {
		list = append(list, fact)
	}
	return []rule.EvidenceGap{{ReasonCode: gapReasonMissingInstanceFact, RequiredFacts: list}}
}

// --- ddl.tidb.table.attributes.valid ---

type tableAttributesValidRule struct{ level rule.Level }

func newTableAttributesValidRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return tableAttributesValidRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r tableAttributesValidRule) ID() string { return ruleIDTiDBTableAttributesValid }

func attributesTargets(statement spec.Statement) []*spec.Alter {
	var targets []*spec.Alter
	for _, alter := range layoutR6Targets(statement) {
		if alter.Action == "attributes" || alter.Action == "partition_attributes" {
			targets = append(targets, alter)
		}
	}
	return targets
}

func (r tableAttributesValidRule) AppliesTo(statement spec.Statement) bool {
	return layoutR6Applies(statement, "attributes") && len(attributesTargets(statement)) > 0
}

func attributesRefusalText(reason string) (message, suggestion string) {
	switch reason {
	case spec.LayoutReasonAttributesInvalid:
		return "carries an ATTRIBUTES spec TiDB cannot parse as a label list",
			"use a comma-separated key=value list, or ATTRIBUTES=DEFAULT to clear"
	case spec.LayoutReasonAttributesNonPart:
		return "cannot set partition ATTRIBUTES on a non-partitioned table",
			"drop the PARTITION clause or partition the table first"
	case spec.LayoutReasonAttributesPartMiss:
		return "names a partition that does not exist",
			"name an existing partition member"
	default:
		return "is statically refused by TiDB",
			"adjust the ATTRIBUTES declaration"
	}
}

func (r tableAttributesValidRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	facts := layoutR6Facts(statement)
	findings := make([]rule.Finding, 0)
	for _, alter := range attributesTargets(statement) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		verdict, reason := spec.AlterAttributesOutcome(alter.Attributes, alter.Partitions, facts)
		if verdict != spec.AutoRandomRefused {
			continue
		}
		message, suggestion := attributesRefusalText(reason)
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    fmt.Sprintf("alter action %q on table %q %s", alter.Action, statement.DDL.Table.Name, message),
			Suggestion: suggestion,
			Metadata: map[string]any{
				"table":  statement.DDL.Table.Name,
				"action": alter.Action,
				"reason": reason,
			},
		})
	}
	return findings, nil
}

func (r tableAttributesValidRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	facts := layoutR6Facts(statement)
	required := map[string]bool{}
	for _, alter := range attributesTargets(statement) {
		verdict, reason := spec.AlterAttributesOutcome(alter.Attributes, alter.Partitions, facts)
		if verdict != spec.AutoRandomUndecidable {
			continue
		}
		switch reason {
		case spec.LayoutReasonPartitionedUnproven:
			required[layoutR6FactPartitioned] = true
		case spec.LayoutReasonMembersUnproven:
			required[layoutR6FactMembers] = true
		}
	}
	if len(required) == 0 {
		return nil
	}
	list := make([]string, 0, len(required))
	for fact := range required {
		list = append(list, fact)
	}
	return []rule.EvidenceGap{{ReasonCode: gapReasonMissingInstanceFact, RequiredFacts: list}}
}

// --- ddl.tidb.table.tiflash.valid ---

type tableTiFlashValidRule struct{ level rule.Level }

func newTableTiFlashValidRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return tableTiFlashValidRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r tableTiFlashValidRule) ID() string { return ruleIDTiDBTableTiFlashValid }

func tiflashTargets(statement spec.Statement) []*spec.Alter {
	var targets []*spec.Alter
	for _, alter := range layoutR6Targets(statement) {
		if alter.Action == "set_tiflash_replica" {
			targets = append(targets, alter)
		}
	}
	return targets
}

func (r tableTiFlashValidRule) AppliesTo(statement spec.Statement) bool {
	return layoutR6Applies(statement, "set_tiflash_replica") && len(tiflashTargets(statement)) > 0
}

func tiflashRefusalText(reason string) (message, suggestion string) {
	switch reason {
	case spec.LayoutReasonTiFlashSysTable:
		return "cannot set a TiFlash replica on a system or memory database table",
			"set TiFlash replicas only on user-schema tables"
	case spec.LayoutReasonTiFlashTemporary:
		return "cannot set a TiFlash replica on a temporary table",
			"set TiFlash replicas only on base tables"
	case spec.LayoutReasonTiFlashViewOrSeq:
		return "cannot set a TiFlash replica on a view or sequence",
			"set TiFlash replicas only on base tables"
	case spec.LayoutReasonTiFlashCharset:
		return "has a column charset TiFlash does not support",
			"use a TiFlash-supported column charset (utf8, utf8mb4, ascii, latin1, binary)"
	default:
		return "is statically refused by TiDB",
			"adjust the TiFlash replica declaration"
	}
}

func (r tableTiFlashValidRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	facts := layoutR6Facts(statement)
	findings := make([]rule.Finding, 0)
	for _, alter := range tiflashTargets(statement) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		verdict, reason := spec.AlterTiFlashReplicaOutcome(alter.TiFlashReplica, facts)
		if verdict != spec.AutoRandomRefused {
			continue
		}
		message, suggestion := tiflashRefusalText(reason)
		findings = append(findings, rule.Finding{
			Level:      r.level,
			Message:    fmt.Sprintf("alter action %q on table %q %s", alter.Action, statement.DDL.Table.Name, message),
			Suggestion: suggestion,
			Metadata: map[string]any{
				"table":  statement.DDL.Table.Name,
				"action": alter.Action,
				"reason": reason,
			},
		})
	}
	return findings, nil
}

func (r tableTiFlashValidRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	facts := layoutR6Facts(statement)
	required := map[string]bool{}
	for _, alter := range tiflashTargets(statement) {
		verdict, reason := spec.AlterTiFlashReplicaOutcome(alter.TiFlashReplica, facts)
		if verdict != spec.AutoRandomUndecidable {
			continue
		}
		switch reason {
		case spec.LayoutReasonTiFlashStoreUnproven:
			required[layoutR6FactStores] = true
		case spec.LayoutReasonOldReplicaUnproven:
			required[layoutR6FactOldReplica] = true
		case spec.LayoutReasonCharsetUnproven:
			required[layoutR6FactCharset] = true
		}
	}
	if len(required) == 0 {
		return nil
	}
	list := make([]string, 0, len(required))
	for fact := range required {
		list = append(list, fact)
	}
	return []rule.EvidenceGap{{ReasonCode: gapReasonMissingInstanceFact, RequiredFacts: list}}
}
