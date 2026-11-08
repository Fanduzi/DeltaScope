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
// validity rule. It adjudicates only what the statement and observed
// instance facts prove: the column-intrinsic constraints upstream checks
// in setTableAutoRandomBits (v8.5.0 pkg/ddl/create_table.go) and the
// clustered-primary-key position premise. An unobserved
// tidb_enable_clustered_index (or the alter_primary_key fact int_only
// defers to) is an evidence gap, never a guessed pass or refusal.

const (
	autoRandomDefaultShardBits    = 5
	autoRandomMinShardBits        = 1
	autoRandomMaxShardBits        = 15
	autoRandomDefaultRangeBits    = 64
	autoRandomMinRangeBits        = 32
	autoRandomMaxRangeBits        = 64
	autoRandomMinIncrementalBits  = 27
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

func (r autoRandomValidRule) finding(statement spec.Statement, column spec.Column, reason, suggestion string) rule.Finding {
	return rule.Finding{
		Level:      r.level,
		Message:    fmt.Sprintf("auto_random column %q %s", column.Name, reason),
		Suggestion: suggestion,
		Metadata: map[string]any{
			"table":  statement.DDL.Table.Name,
			"column": column.Name,
		},
	}
}

// autoRandomEffectiveBits resolves the authored-or-default shard and range
// bit counts, mirroring the upstream normalization (defaults 5 and 64).
func autoRandomEffectiveBits(column spec.Column) (shard, rangeBits int64) {
	shard = autoRandomDefaultShardBits
	if column.AutoRandomShardBits != nil {
		shard = *column.AutoRandomShardBits
	}
	rangeBits = autoRandomDefaultRangeBits
	if column.AutoRandomRangeBits != nil {
		rangeBits = *column.AutoRandomRangeBits
	}
	return shard, rangeBits
}

func (r autoRandomValidRule) Evaluate(ctx context.Context, statement spec.Statement) ([]rule.Finding, error) {
	if !r.AppliesTo(statement) {
		return nil, nil
	}
	findings := make([]rule.Finding, 0)
	pk := statement.DDL.PrimaryKey
	for _, column := range autoRandomColumns(statement) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if baseType(column) != "bigint" {
			findings = append(findings, r.finding(statement, column,
				"must be BIGINT: TiDB supports AUTO_RANDOM only on BIGINT columns",
				"change the column type to BIGINT"))
			continue
		}
		if column.AutoIncrement {
			findings = append(findings, r.finding(statement, column,
				"cannot combine AUTO_RANDOM with AUTO_INCREMENT",
				"remove AUTO_INCREMENT from the column"))
			continue
		}
		if column.HasDefault {
			findings = append(findings, r.finding(statement, column,
				"cannot combine AUTO_RANDOM with an explicit DEFAULT",
				"remove the DEFAULT clause from the column"))
			continue
		}
		shard, rangeBits := autoRandomEffectiveBits(column)
		if column.AutoRandomShardBits != nil && (shard < autoRandomMinShardBits || shard > autoRandomMaxShardBits) {
			findings = append(findings, r.finding(statement, column,
				fmt.Sprintf("has shard bits %d outside the allowed range %d..%d", shard, autoRandomMinShardBits, autoRandomMaxShardBits),
				"choose AUTO_RANDOM shard bits between 1 and 15"))
			continue
		}
		if column.AutoRandomRangeBits != nil && (rangeBits < autoRandomMinRangeBits || rangeBits > autoRandomMaxRangeBits) {
			findings = append(findings, r.finding(statement, column,
				fmt.Sprintf("has range bits %d outside the allowed range %d..%d", rangeBits, autoRandomMinRangeBits, autoRandomMaxRangeBits),
				"choose AUTO_RANDOM range bits between 32 and 64"))
			continue
		}
		// Upstream NewShardIDFormat: incremental = range - shard minus the
		// sign bit a signed BIGINT spends (v8.5.0 pkg/meta/autoid).
		incremental := rangeBits - shard
		if !column.Unsigned {
			incremental--
		}
		if incremental < autoRandomMinIncrementalBits {
			findings = append(findings, r.finding(statement, column,
				fmt.Sprintf("leaves only %d incremental bits; TiDB requires at least %d", incremental, autoRandomMinIncrementalBits),
				"reduce the shard bits or raise the range bits"))
			continue
		}
		if pk == nil {
			findings = append(findings, r.finding(statement, column,
				"requires a clustered primary key, but the table declares none",
				"declare a primary key that includes this column"))
			continue
		}
		if len(pk.Columns) == 0 || !strings.EqualFold(pk.Columns[0], column.Name) {
			findings = append(findings, r.finding(statement, column,
				"must be the first primary key column (the row handle on TiDB)",
				"list this column first in the primary key"))
			continue
		}
		// Position is provably satisfied; clustered storage is the
		// remaining premise. A proven non-clustered PK refuses natively;
		// an unproven one is the EvidenceGaps boundary, not a finding.
		clustered := spec.PrimaryKeyClustered(statement.Dialect, pk, statement.DDL.Columns,
			autoRandomClusteredMode(statement), autoRandomAlterPrimaryKey(statement))
		if clustered != nil && !*clustered {
			findings = append(findings, r.finding(statement, column,
				"requires a clustered primary key, but this primary key is not clustered",
				"use a clustered primary key or remove AUTO_RANDOM"))
		}
	}
	return findings, nil
}

// EvidenceGaps surfaces the unresolved clustered-mode premise: the column
// is a provable first PK member whose clustered answer defers to an
// unobserved tidb_enable_clustered_index — and, under int_only, to the
// unobserved alter-primary-key fact.
func (r autoRandomValidRule) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	if !r.AppliesTo(statement) {
		return nil
	}
	pk := statement.DDL.PrimaryKey
	var facts []string
	for _, column := range autoRandomColumns(statement) {
		if baseType(column) != "bigint" || column.AutoIncrement || column.HasDefault {
			continue
		}
		shard, rangeBits := autoRandomEffectiveBits(column)
		if (column.AutoRandomShardBits != nil && (shard < autoRandomMinShardBits || shard > autoRandomMaxShardBits)) ||
			(column.AutoRandomRangeBits != nil && (rangeBits < autoRandomMinRangeBits || rangeBits > autoRandomMaxRangeBits)) {
			continue
		}
		incremental := rangeBits - shard
		if !column.Unsigned {
			incremental--
		}
		if incremental < autoRandomMinIncrementalBits {
			continue
		}
		if pk == nil || len(pk.Columns) == 0 || !strings.EqualFold(pk.Columns[0], column.Name) {
			continue
		}
		if spec.PrimaryKeyClustered(statement.Dialect, pk, statement.DDL.Columns,
			autoRandomClusteredMode(statement), autoRandomAlterPrimaryKey(statement)) == nil {
			facts = append(facts, autoRandomFactClusteredMode, autoRandomFactAlterPrimaryKey)
		}
	}
	if len(facts) == 0 {
		return nil
	}
	return []rule.EvidenceGap{{
		ReasonCode:    gapReasonMissingInstanceFact,
		RequiredFacts: facts,
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
