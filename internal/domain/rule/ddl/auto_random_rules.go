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

// auto_random_rules.go implements the T23 TiDB AUTO_RANDOM declaration
// validity rule. The refusal matrix itself lives in
// spec.AutoRandomColumnVerdict — shared with the ordered-state publisher
// so a proven-invalid CREATE can never publish a shape and an
// undecidable clustered premise is an evidence gap, never a guess.

const (
	autoRandomFactClusteredMode   = "instance.tidb_enable_clustered_index"
	autoRandomFactAlterPrimaryKey = "instance.alter_primary_key"
)

type autoRandomValidRule struct {
	level rule.Level
}

func newAutoRandomValidRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return autoRandomValidRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r autoRandomValidRule) ID() string { return ruleIDTiDBTableAutoRandomValid }

func (r autoRandomValidRule) AppliesTo(statement spec.Statement) bool {
	return statement.Dialect == spec.DialectTiDB &&
		statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Operation == spec.DDLOperationCreateTable
}

// autoRandomColumns returns the declared CREATE TABLE columns carrying a
// parsed AUTO_RANDOM attribute.
func autoRandomColumns(statement spec.Statement) []spec.Column {
	var columns []spec.Column
	for _, column := range statement.DDL.Columns {
		if column.AutoRandom {
			columns = append(columns, column)
		}
	}
	return columns
}

// autoRandomRefusalText maps the stable spec reason token into the
// finding's message fragment and suggestion.
func autoRandomRefusalText(reason string, column spec.Column) (message, suggestion string) {
	shard, rangeBits := spec.AutoRandomEffectiveBits(column)
	switch reason {
	case spec.AutoRandomReasonNonBigint:
		return "must be BIGINT: TiDB supports AUTO_RANDOM only on BIGINT columns",
			"change the column type to BIGINT"
	case spec.AutoRandomReasonWithAutoIncrement:
		return "cannot combine AUTO_RANDOM with AUTO_INCREMENT",
			"remove AUTO_INCREMENT from the column"
	case spec.AutoRandomReasonWithDefault:
		return "cannot combine AUTO_RANDOM with an explicit DEFAULT",
			"remove the DEFAULT clause from the column"
	case spec.AutoRandomReasonShardBits:
		return fmt.Sprintf("has shard bits %d outside the allowed range %d..%d", shard, spec.AutoRandomMinShardBits, spec.AutoRandomMaxShardBits),
			"choose AUTO_RANDOM shard bits between 1 and 15"
	case spec.AutoRandomReasonRangeBits:
		return fmt.Sprintf("has range bits %d outside the allowed range %d..%d", rangeBits, spec.AutoRandomMinRangeBits, spec.AutoRandomMaxRangeBits),
			"choose AUTO_RANDOM range bits between 32 and 64"
	case spec.AutoRandomReasonIncrementalBits:
		incremental := rangeBits - shard
		if !column.Unsigned {
			incremental--
		}
		return fmt.Sprintf("leaves only %d incremental bits; TiDB requires at least %d", incremental, spec.AutoRandomMinIncrementalBits),
			"reduce the shard bits or raise the range bits"
	case spec.AutoRandomReasonNoPrimaryKey:
		return "requires a clustered primary key, but the table declares none",
			"declare a primary key that includes this column"
	case spec.AutoRandomReasonNotClustered:
		return "requires a clustered primary key, but this primary key is not clustered",
			"use a clustered primary key or remove AUTO_RANDOM"
	case spec.AutoRandomReasonNotFirstMember:
		return "must be the first primary key column (the row handle on TiDB)",
			"list this column first in the primary key"
	default:
		return "is statically refused by TiDB",
			"adjust the AUTO_RANDOM declaration"
	}
}

func (r autoRandomValidRule) finding(statement spec.Statement, column spec.Column, reason string) rule.Finding {
	message, suggestion := autoRandomRefusalText(reason, column)
	return rule.Finding{
		Level:      r.level,
		Message:    fmt.Sprintf("auto_random column %q %s", column.Name, message),
		Suggestion: suggestion,
		Metadata: map[string]any{
			"table":  statement.DDL.Table.Name,
			"column": column.Name,
		},
	}
}

func (r autoRandomValidRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	findings := make([]rule.Finding, 0)
	for _, column := range autoRandomColumns(statement) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		verdict, reason := spec.AutoRandomColumnVerdict(column, statement.DDL.PrimaryKey, statement.DDL.Columns,
			autoRandomClusteredMode(statement), autoRandomAlterPrimaryKey(statement))
		if verdict == spec.AutoRandomRefused {
			findings = append(findings, r.finding(statement, column, reason))
		}
	}
	return findings, nil
}

// EvidenceGaps surfaces the unresolved clustered-mode premise: a column
// whose only pending premise is the unobserved session fact owes exactly
// that fact — an observed int_only mode narrows it to alter_primary_key,
// an unobserved or unrecognized mode to tidb_enable_clustered_index.
func (r autoRandomValidRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	var facts []string
	mode := autoRandomClusteredMode(statement)
	for _, column := range autoRandomColumns(statement) {
		verdict, _ := spec.AutoRandomColumnVerdict(column, statement.DDL.PrimaryKey, statement.DDL.Columns,
			mode, autoRandomAlterPrimaryKey(statement))
		if verdict != spec.AutoRandomUndecidable {
			continue
		}
		if mode == nil || !strings.EqualFold(strings.TrimSpace(*mode), "int_only") {
			facts = append(facts, autoRandomFactClusteredMode)
		} else {
			facts = append(facts, autoRandomFactAlterPrimaryKey)
		}
	}
	if len(facts) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(facts))
	required := facts[:0]
	for _, fact := range facts {
		if !seen[fact] {
			seen[fact] = true
			required = append(required, fact)
		}
	}
	return []rule.EvidenceGap{{
		ReasonCode:    gapReasonMissingInstanceFact,
		RequiredFacts: required,
	}}
}

// autoRandomClusteredMode returns the request-observed
// tidb_enable_clustered_index mode — nil when unobserved, so the clustered
// answer withholds rather than guessing the product default.
func autoRandomClusteredMode(statement spec.Statement) *string {
	if statement.Metadata == nil || statement.Metadata.Instance == nil ||
		statement.Metadata.Instance.TiDBEnableClusteredIndex == "" {
		return nil
	}
	mode := statement.Metadata.Instance.TiDBEnableClusteredIndex
	return &mode
}

// autoRandomAlterPrimaryKey returns the observed alter-primary-key fact —
// nil when unobserved, withholding the int_only handle answer.
func autoRandomAlterPrimaryKey(statement spec.Statement) *bool {
	if statement.Metadata == nil || statement.Metadata.Instance == nil ||
		!statement.Metadata.Instance.TiDBAlterPrimaryKeyKnown {
		return nil
	}
	alterPK := statement.Metadata.Instance.TiDBAlterPrimaryKey
	return &alterPK
}

// t23TableOptionKeys are the declared create-time table options the shared
// option verdict audits (v8.5.0 handleTableOptions).
var t23TableOptionKeys = []string{"auto_random_base", "auto_id_cache", "shard_row_id_bits", "pre_split_regions"}

type autoRandomOptionValidRule struct {
	level rule.Level
}

func newAutoRandomOptionValidRule(cfg policy.RulePolicy) (rule.StatementRule, error) {
	return autoRandomOptionValidRule{level: configuredLevel(cfg, rule.LevelBlocker)}, nil
}

func (r autoRandomOptionValidRule) ID() string { return ruleIDTiDBTableOptionsValid }

func (r autoRandomOptionValidRule) AppliesTo(statement spec.Statement) bool {
	if statement.Dialect != spec.DialectTiDB || statement.Kind != spec.KindDDL ||
		statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationCreateTable {
		return false
	}
	for _, key := range t23TableOptionKeys {
		if _, ok := statement.DDL.Options[key]; ok {
			return true
		}
	}
	return false
}

func autoRandomOptionRefusalText(reason string) (message, suggestion string) {
	switch reason {
	case spec.CreateOptionReasonAutoIDCacheOverflow:
		return "AUTO_ID_CACHE exceeds int64; TiDB refuses the table option",
			"choose an AUTO_ID_CACHE value at or below 9223372036854775807"
	case spec.CreateOptionReasonShardRowIDClustered:
		return "SHARD_ROW_ID_BITS > 0 on a clustered primary key is refused by TiDB",
			"use a nonclustered primary key or drop SHARD_ROW_ID_BITS"
	case spec.CreateOptionReasonPreSplitTemporary:
		return "PRE_SPLIT_REGIONS on a temporary table is refused by TiDB",
			"remove PRE_SPLIT_REGIONS or create a permanent table"
	default:
		return "is statically refused by TiDB",
			"adjust the declared table option"
	}
}

func (r autoRandomOptionValidRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	verdict, reason := spec.CreateTableOptionOutcome(statement.Dialect, statement.DDL,
		autoRandomClusteredMode(statement), autoRandomAlterPrimaryKey(statement))
	if verdict != spec.AutoRandomRefused {
		return nil, ctx.Err()
	}
	message, suggestion := autoRandomOptionRefusalText(reason)
	return []rule.Finding{{
		Level:      r.level,
		Message:    fmt.Sprintf("table %q option %s", statement.DDL.Table.Name, message),
		Suggestion: suggestion,
		Metadata:   map[string]any{"table": statement.DDL.Table.Name, "reason": reason},
	}}, nil
}

// EvidenceGaps surfaces an unresolved clustered premise owed by a nonzero
// SHARD_ROW_ID_BITS — the same minimal instance-fact minimization as the
// column verdict — or an unprovable authored numeric value.
func (r autoRandomOptionValidRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	verdict, reason := spec.CreateTableOptionOutcome(statement.Dialect, statement.DDL,
		autoRandomClusteredMode(statement), autoRandomAlterPrimaryKey(statement))
	if verdict != spec.AutoRandomUndecidable {
		return nil
	}
	if reason == spec.CreateOptionReasonValueUnproven {
		return []rule.EvidenceGap{{
			ReasonCode:    gapReasonMissingInstanceFact,
			RequiredFacts: []string{"table.options.value"},
		}}
	}
	mode := autoRandomClusteredMode(statement)
	fact := autoRandomFactClusteredMode
	if mode != nil && strings.EqualFold(strings.TrimSpace(*mode), "int_only") {
		fact = autoRandomFactAlterPrimaryKey
	}
	return []rule.EvidenceGap{{
		ReasonCode:    gapReasonMissingInstanceFact,
		RequiredFacts: []string{fact},
	}}
}
