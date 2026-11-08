package spec

import (
	"math"
	"strconv"
	"strings"
)

// auto_random.go is the shared TiDB AUTO_RANDOM declaration verdict —
// the static refusal matrix upstream runs in setTableAutoRandomBits
// (v8.5.0 pkg/ddl/create_table.go) plus the NewShardIDFormat bit math
// (pkg/meta/autoid/autoid.go). The validity rule maps the verdict into
// findings and gaps; the ordered-state publisher maps it into the
// create's outcome — one matrix, two consumers, so a proven refusal can
// never publish a shape the engine never built.

const (
	AutoRandomDefaultShardBits   = 5
	AutoRandomMinShardBits       = 1
	AutoRandomMaxShardBits       = 15
	AutoRandomDefaultRangeBits   = 64
	AutoRandomMinRangeBits       = 32
	AutoRandomMaxRangeBits       = 64
	AutoRandomMinIncrementalBits = 27
)

// AutoRandomVerdict is the three-state answer for one declared column.
type AutoRandomVerdict int

const (
	// AutoRandomOK: the declaration is provably applicable.
	AutoRandomOK AutoRandomVerdict = iota
	// AutoRandomRefused: a fixed upstream refusal — the statement never
	// executes, whatever the rest of the batch proves.
	AutoRandomRefused
	// AutoRandomUndecidable: only the clustered-premise is unprovable —
	// the unwritten PK type defers to unobserved session facts. Never a
	// guessed pass or refusal.
	AutoRandomUndecidable
)

// AutoRandomRefusalReason tokens stay stable for finding messages and
// ordered-state evidence; they are not display strings.
const (
	AutoRandomReasonNonBigint         = "non_bigint"
	AutoRandomReasonWithAutoIncrement = "with_auto_increment"
	AutoRandomReasonWithDefault       = "with_default"
	AutoRandomReasonShardBits         = "shard_bits_out_of_range"
	AutoRandomReasonRangeBits         = "range_bits_out_of_range"
	AutoRandomReasonIncrementalBits   = "incremental_bits_under_min"
	AutoRandomReasonNoPrimaryKey      = "no_primary_key"
	AutoRandomReasonNotClustered      = "primary_key_not_clustered"
	AutoRandomReasonNotFirstMember    = "not_first_primary_key_member"
)

// AutoRandomEffectiveBits resolves the authored-or-default shard and range
// bit counts, mirroring the upstream normalization (5/64). An authored
// value is never rewritten — 0 stays 0 and fails the range check like an
// explicit SQL declaration would (the NewShardIDFormat range=0 encoding
// detail is not a license for an authored AUTO_RANDOM(5,0)).
func AutoRandomEffectiveBits(column Column) (shard, rangeBits int64) {
	shard = AutoRandomDefaultShardBits
	if column.AutoRandomShardBits != nil {
		shard = *column.AutoRandomShardBits
	}
	rangeBits = AutoRandomDefaultRangeBits
	if column.AutoRandomRangeBits != nil {
		rangeBits = *column.AutoRandomRangeBits
	}
	return shard, rangeBits
}

// AutoRandomColumnVerdict adjudicates one AUTO_RANDOM column declaration
// inside its CREATE TABLE context. pk is the declared primary key (nil
// allowed), columns the full declared column set, tidbClusteredMode the
// observed tidb_enable_clustered_index value (nil = unobserved), and
// tidbAlterPK the observed alter-primary-key fact (nil = unobserved).
// The verdict and its reason token are returned; Undecidable always
// carries an empty reason.
func AutoRandomColumnVerdict(column Column, pk *Index, columns []Column, tidbClusteredMode *string, tidbAlterPK *bool) (AutoRandomVerdict, string) {
	if baseColumnTypeName(column.Type) != "bigint" {
		return AutoRandomRefused, AutoRandomReasonNonBigint
	}
	if column.AutoIncrement {
		return AutoRandomRefused, AutoRandomReasonWithAutoIncrement
	}
	if column.HasDefault {
		return AutoRandomRefused, AutoRandomReasonWithDefault
	}
	shard, rangeBits := AutoRandomEffectiveBits(column)
	if column.AutoRandomShardBits != nil && (shard < AutoRandomMinShardBits || shard > AutoRandomMaxShardBits) {
		return AutoRandomRefused, AutoRandomReasonShardBits
	}
	if column.AutoRandomRangeBits != nil && (rangeBits < AutoRandomMinRangeBits || rangeBits > AutoRandomMaxRangeBits) {
		return AutoRandomRefused, AutoRandomReasonRangeBits
	}
	// Upstream NewShardIDFormat: incremental = range − shard minus the
	// sign bit a signed BIGINT spends.
	incremental := rangeBits - shard
	if !column.Unsigned {
		incremental--
	}
	if incremental < AutoRandomMinIncrementalBits {
		return AutoRandomRefused, AutoRandomReasonIncrementalBits
	}
	if pk == nil {
		return AutoRandomRefused, AutoRandomReasonNoPrimaryKey
	}
	if len(pk.Columns) == 0 || !strings.EqualFold(pk.Columns[0], column.Name) {
		return AutoRandomRefused, AutoRandomReasonNotFirstMember
	}
	clustered := PrimaryKeyClustered(DialectTiDB, pk, columns, tidbClusteredMode, tidbAlterPK)
	if clustered == nil {
		return AutoRandomUndecidable, ""
	}
	if !*clustered {
		return AutoRandomRefused, AutoRandomReasonNotClustered
	}
	return AutoRandomOK, ""
}

// Create-option refusal reasons — stable tokens both layers switch on.
const (
	CreateOptionReasonAutoIDCacheOverflow = "auto_id_cache_overflow"
	CreateOptionReasonShardRowIDClustered = "shard_row_id_clustered"
	CreateOptionReasonPreSplitTemporary   = "pre_split_regions_temporary"
	CreateOptionReasonValueUnproven       = "option_value_unproven"
	CreateOptionReasonClusteredUnproven   = "clustered_premise_unproven"
)

// CreateTableOptionOutcome applies the fixed TiDB v8.5.0 CREATE-time table
// option verdicts (pkg/ddl create_table.go handleTableOptions) to the
// declared option map. Tri-state like the column verdict: a proven
// refusal shares the statement's atomic fate, an unprovable premise keeps
// the outcome undecidable, and a legal declaration is applied — declared
// values stay authored (source clamps like shard>15→15 are effective-
// state normalization, never refusal or authored-value rewrites).
// Non-TiDB dialects return applied: those options never reach here.
func CreateTableOptionOutcome(dialect Dialect, ddl *DDL, tidbClusteredMode *string, tidbAlterPrimaryKey *bool) (verdict AutoRandomVerdict, reason string) {
	if dialect != DialectTiDB || ddl == nil {
		return AutoRandomOK, ""
	}
	undecidable := ""
	if value, ok := ddl.Options["auto_id_cache"]; ok {
		n, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
		if err != nil {
			undecidable = CreateOptionReasonValueUnproven
		} else if n > math.MaxInt64 {
			return AutoRandomRefused, CreateOptionReasonAutoIDCacheOverflow
		}
	}
	if value, ok := ddl.Options["shard_row_id_bits"]; ok {
		n, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
		if err != nil {
			undecidable = CreateOptionReasonValueUnproven
		} else if n > 0 {
			// Upstream refuses a nonzero shard on a clustered primary —
			// the same tri-state premise the column verdict consumes.
			clustered := PrimaryKeyClustered(dialect, ddl.PrimaryKey, ddl.Columns, tidbClusteredMode, tidbAlterPrimaryKey)
			switch {
			case clustered == nil:
				undecidable = CreateOptionReasonClusteredUnproven
			case *clustered:
				return AutoRandomRefused, CreateOptionReasonShardRowIDClustered
			}
		}
	}
	if _, ok := ddl.Options["pre_split_regions"]; ok && ddl.TemporaryScope != "" {
		return AutoRandomRefused, CreateOptionReasonPreSplitTemporary
	}
	if undecidable != "" {
		return AutoRandomUndecidable, undecidable
	}
	return AutoRandomOK, ""
}
