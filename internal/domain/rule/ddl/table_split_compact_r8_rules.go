package ddl

import (
	"context"
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// table_split_compact_r8_rules.go implements the T23-R8 TiDB
// physical-layout rules for the standalone SPLIT and COMPACT
// statements. Verdicts live in spec.SplitRegionOutcome and
// spec.CompactTableOutcome — the same functions the ordered-state
// publisher consults for post-state decisions (issue #102).

const (
	layoutR8FactPartitioned  = "table.partitioned"
	layoutR8FactMembers      = "table.partition_names"
	layoutR8FactIndexMembers = "table.index_members"
	layoutR8FactHandle       = "table.handle_columns"
	layoutR8FactMemberTypes  = "table.member_column_types"
	layoutR8FactValueExpr    = "statement.split_value_foldability"
	layoutR8FactBoundCompare = "statement.split_bound_relation"
	layoutR8FactBoundConvert = "statement.split_bound_conversion"
	layoutR8FactNumLimit     = "instance.split_region_max_num"
	layoutR8FactTiKVStore    = "instance.tikv_compatible_storage"
)

func splitCompactSnapshot(statement spec.Statement) *spec.TableSnapshot {
	if statement.Metadata != nil {
		return statement.Metadata.TargetTable
	}
	return nil
}

func splitCompactInstance(statement spec.Statement) *spec.InstanceFacts {
	if statement.Metadata != nil {
		return statement.Metadata.Instance
	}
	return nil
}

func splitCompactFacts(statement spec.Statement) spec.SplitCompactFacts {
	return spec.SplitCompactFactsForSnapshot(
		splitCompactSnapshot(statement), splitCompactInstance(statement))
}

func splitCompactApplies(statement spec.Statement, operation spec.DDLOperation) bool {
	return statement.Dialect == spec.DialectTiDB &&
		statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Operation == operation &&
		statement.DDL.Table != nil
}

func splitRefusalText(reason string) (message, suggestion string) {
	switch reason {
	case spec.LayoutReasonSplitTemporary:
		return "cannot split regions on a temporary table",
			"split regions on a base table only"
	case spec.LayoutReasonSplitNonPartitioned:
		return "cannot use the PARTITION clause form on a non-partitioned table",
			"drop PARTITION from the statement or split named members with SPLIT TABLE ... PARTITION (names)"
	case spec.LayoutReasonSplitPrimaryClustered:
		return "cannot split the primary index regions of a clustered-index table",
			"split the table regions instead of the primary index"
	case spec.LayoutReasonSplitIndexMissing:
		return "references an index that does not exist on the target table",
			"check the index name or split the table regions directly"
	case spec.LayoutReasonSplitArityIndex:
		return "lists more value members than the index has columns",
			"match the value tuple arity to the index member count or fewer"
	case spec.LayoutReasonSplitArityTable:
		return "lists a value tuple arity that does not match the table handle width",
			"match the tuple arity to the primary-key member count (or 1 for the rowid handle)"
	case spec.LayoutReasonSplitBoundEmpty:
		return "declares an empty lower or upper bound",
			"provide at least one value for both BETWEEN bounds"
	case spec.LayoutReasonSplitNullValue:
		return "contains a NULL literal in a split value tuple",
			"use non-NULL boundary values"
	case spec.LayoutReasonSplitNonNumeric:
		return "contains a string literal that cannot convert to the integer handle/index member",
			"use numeric boundary values for integer members"
	case spec.LayoutReasonSplitNumBelowOne:
		return "declares a REGIONS count below 1",
			"request at least one region"
	case spec.LayoutReasonSplitNumOverLimit:
		return "declares a REGIONS count above the server split-region-max-num limit",
			"lower the region count or raise the server configuration"
	case spec.LayoutReasonSplitPartMissing:
		return "names a partition that does not exist on the target table",
			"check the partition member names"
	case spec.LayoutReasonSplitBoundNotIncreasing:
		return "declares an upper bound that is not greater than the lower bound",
			"order the BETWEEN bounds so the upper value is strictly greater"
	case spec.LayoutReasonSplitStepTooSmall:
		return "declares a BETWEEN range whose per-region step is below the fixed minimum",
			"widen the BETWEEN range or lower the REGIONS count"
	default:
		return "is statically refused by TiDB",
			"adjust the SPLIT declaration"
	}
}

func compactRefusalText(reason string) (message, suggestion string) {
	switch reason {
	case spec.LayoutReasonCompactReplicaKind:
		return "declares an unsupported replica kind — TiDB compacts TIFLASH or ALL replicas only",
			"use COMPACT TIFLASH REPLICA or the bare COMPACT form"
	case spec.LayoutReasonCompactNonPartitioned:
		return "names partitions on a non-partitioned table",
			"drop the PARTITION clause or compact the whole table"
	case spec.LayoutReasonCompactPartMissing:
		return "names a partition that does not exist on the target table",
			"check the partition member names"
	case spec.LayoutReasonCompactNotTiKV:
		return "requires tikv-compatible storage, which the target server does not run",
			"run the statement only on a TiKV-backed TiDB deployment"
	default:
		return "is statically refused by TiDB",
			"adjust the COMPACT declaration"
	}
}

// --- ddl.tidb.table.split_region.valid ---

type tableSplitRegionValidRule struct{ level rule.Level }

func newTableSplitRegionValidRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return tableSplitRegionValidRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r tableSplitRegionValidRule) ID() string { return ruleIDTiDBTableSplitRegionValid }

func (r tableSplitRegionValidRule) AppliesTo(statement spec.Statement) bool {
	return splitCompactApplies(statement, spec.DDLOperationSplitRegion)
}

func (r tableSplitRegionValidRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	facts := splitCompactFacts(statement)
	verdict, reason := spec.SplitRegionOutcome(statement.Dialect, statement.DDL.SplitRegion, facts)
	if verdict != spec.AutoRandomRefused {
		return nil, nil
	}
	message, suggestion := splitRefusalText(reason)
	return []rule.Finding{{
		Level:      r.level,
		Message:    fmt.Sprintf("split table region on %q %s", statement.DDL.Table.Name, message),
		Suggestion: suggestion,
		Metadata: map[string]any{
			"table":  statement.DDL.Table.Name,
			"index":  statement.DDL.SplitRegion.IndexName,
			"reason": reason,
		},
	}}, nil
}

func (r tableSplitRegionValidRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	facts := splitCompactFacts(statement)
	verdict, reason := spec.SplitRegionOutcome(statement.Dialect, statement.DDL.SplitRegion, facts)
	if verdict != spec.AutoRandomUndecidable {
		return nil
	}
	var fact string
	switch reason {
	case spec.LayoutReasonPartitionedUnproven:
		fact = layoutR8FactPartitioned
	case spec.LayoutReasonMembersUnproven:
		fact = layoutR8FactMembers
	case spec.LayoutReasonSplitIndexUnproven:
		fact = layoutR8FactIndexMembers
	case spec.LayoutReasonSplitHandleUnproven:
		fact = layoutR8FactHandle
	case spec.LayoutReasonSplitMemberUnproven:
		fact = layoutR8FactMemberTypes
	case spec.LayoutReasonSplitValueExpr:
		// Expression foldability is a statement-side premise the
		// analyzer cannot establish — a statement fact, not a missing
		// instance fact.
		return []rule.EvidenceGap{{ReasonCode: gapReasonUnprovenStatementFact, RequiredFacts: []string{layoutR8FactValueExpr}}}
	case spec.LayoutReasonSplitNumLimitUnproven:
		fact = layoutR8FactNumLimit
	case spec.LayoutReasonSplitBoundCompareUnsupported:
		// The encoded-key comparison on common-handle/index BETWEEN is
		// a statement-side analysis boundary, not a missing instance
		// fact — disclose the unproven premise as such.
		return []rule.EvidenceGap{{ReasonCode: gapReasonUnprovenStatementFact, RequiredFacts: []string{layoutR8FactBoundCompare}}}
	case spec.LayoutReasonSplitBoundConvertUnproven:
		return []rule.EvidenceGap{{ReasonCode: gapReasonUnprovenStatementFact, RequiredFacts: []string{layoutR8FactBoundConvert}}}
	case spec.LayoutReasonSplitNonTableObject:
		// The executor's behavior on a non-table target is an unmodeled
		// product boundary, not a missing catalog fact — bounded
		// undecidable evidence without a required-fact name.
		fact = ""
	default:
		fact = "table.split_region_premise"
	}
	required := []string{}
	if fact != "" {
		required = []string{fact}
	}
	return []rule.EvidenceGap{{ReasonCode: gapReasonMissingInstanceFact, RequiredFacts: required}}
}

// --- ddl.tidb.table.compact.valid ---

type tableCompactValidRule struct{ level rule.Level }

func newTableCompactValidRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return tableCompactValidRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r tableCompactValidRule) ID() string { return ruleIDTiDBTableCompactValid }

func (r tableCompactValidRule) AppliesTo(statement spec.Statement) bool {
	return splitCompactApplies(statement, spec.DDLOperationCompactTable)
}

func (r tableCompactValidRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	facts := splitCompactFacts(statement)
	verdict, reason := spec.CompactTableOutcome(statement.Dialect, statement.DDL.CompactTable, facts)
	switch verdict {
	case spec.AutoRandomRefused:
		message, suggestion := compactRefusalText(reason)
		return []rule.Finding{{
			Level:      r.level,
			Message:    fmt.Sprintf("compact table on %q %s", statement.DDL.Table.Name, message),
			Suggestion: suggestion,
			Metadata: map[string]any{
				"table":  statement.DDL.Table.Name,
				"reason": reason,
			},
		}}, nil
	case spec.AutoRandomOK:
		if reason == spec.LayoutReasonCompactNoReplicaWarn {
			// Upstream emits a warning and completes a no-op — the
			// statement is legal but performs no work.
			return []rule.Finding{{
				Level:      rule.LevelWarning,
				Message:    fmt.Sprintf("compact table on %q is a no-op: the table declares no tiflash replica", statement.DDL.Table.Name),
				Suggestion: "set a TiFlash replica first, or drop the COMPACT statement",
				Metadata: map[string]any{
					"table":  statement.DDL.Table.Name,
					"reason": reason,
				},
			}}, nil
		}
	}
	return nil, nil
}

func (r tableCompactValidRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	facts := splitCompactFacts(statement)
	verdict, reason := spec.CompactTableOutcome(statement.Dialect, statement.DDL.CompactTable, facts)
	if verdict != spec.AutoRandomUndecidable {
		return nil
	}
	var fact string
	switch reason {
	case spec.LayoutReasonPartitionedUnproven:
		fact = layoutR8FactPartitioned
	case spec.LayoutReasonMembersUnproven:
		fact = layoutR8FactMembers
	case spec.LayoutReasonCompactStoreUnproven:
		fact = layoutR8FactTiKVStore
	default:
		fact = "table.compact_premise"
	}
	return []rule.EvidenceGap{{ReasonCode: gapReasonMissingInstanceFact, RequiredFacts: []string{fact}}}
}
