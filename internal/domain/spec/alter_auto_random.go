package spec

import (
	"math"
	"strconv"
	"strings"
)

// alter_auto_random.go is the shared ALTER-side TiDB AUTO_RANDOM and
// layout-option verdict — the static matrix upstream runs in
// checkAutoRandom (v8.5.0 pkg/ddl/modify_column.go), the ADD COLUMN
// refusal (pkg/ddl/add_column.go), and the AlterTableOption dispatch
// (pkg/ddl/executor.go) with ShardRowID / AlterTableAutoIDCache /
// RebaseAutoID target functions. The rule layer maps verdicts into
// findings and gaps; the ordered-state publisher maps them into refusal
// preservation, unvouched facts, or projected post-state — one matrix,
// two consumers.

// ALTER-side refusal reason tokens — stable for findings and ordered
// state, never display strings.
const (
	AlterReasonAddColumnAutoRandom      = "add_column_auto_random"
	AlterReasonRemoveAutoRandom         = "remove_auto_random"
	AlterReasonDecreaseShardBits        = "decrease_shard_bits"
	AlterReasonChangeFromAutoIncrement  = "change_from_non_auto_increment"
	AlterReasonModifyColumnType         = "modify_column_type"
	AlterReasonNonBigint                = "non_bigint"
	AlterReasonWithAutoIncrement        = "with_auto_increment"
	AlterReasonWithDefault              = "with_default"
	AlterReasonRangeBitsChanged         = "range_bits_changed"
	AlterReasonShardBits                = "shard_bits_out_of_range"
	AlterReasonRangeBits                = "range_bits_out_of_range"
	AlterReasonRuntimeAllocatorUnproven = "runtime_allocator_unproven"
)

// alterAutoRandomNewBits normalises the authored AUTO_RANDOM arguments of
// the declared new column definition the way
// extractAutoRandomBitsFromColDef does: an absent attribute yields (0,0),
// a bare attribute the defaults (5,64), and an authored value outside the
// declared bounds a refusal reason. Authoring a boundary-invalid value is
// a proven refusal, never a silent clamp.
func alterAutoRandomNewBits(column *Column) (shard, rangeBits int64, reason string) {
	if !column.AutoRandom {
		return 0, 0, ""
	}
	shard = AutoRandomDefaultShardBits
	if column.AutoRandomShardBits != nil {
		shard = *column.AutoRandomShardBits
		if shard < AutoRandomMinShardBits || shard > AutoRandomMaxShardBits {
			return 0, 0, AlterReasonShardBits
		}
	}
	rangeBits = AutoRandomDefaultRangeBits
	if column.AutoRandomRangeBits != nil {
		rangeBits = *column.AutoRandomRangeBits
		if rangeBits < AutoRandomMinRangeBits || rangeBits > AutoRandomMaxRangeBits {
			return 0, 0, AlterReasonRangeBits
		}
	}
	return shard, rangeBits, ""
}

// AlterAutoRandomColumnVerdict adjudicates one MODIFY COLUMN or CHANGE
// COLUMN action against checkAutoRandom. The caller resolves the old
// facts: oldShardBits/oldRangeBits are the table's effective AUTO_RANDOM
// bits when the old column is a clustered-primary-key member
// (isClusteredPKColumn) and zero otherwise; oldBitsKnown=false marks the
// member or bits answer unproven. oldIsPKHandle carries
// IsPKHandleColumn — the PKIsHandle member the AUTO_INCREMENT→AUTO_RANDOM
// conversion accepts. A static pass still owes the runtime allocator
// overflow check (checkNewAutoRandomBits), so an applicable newBits>0
// outcome is Undecidable — only a truly untouched declaration is OK.
func AlterAutoRandomColumnVerdict(oldShardBits, oldRangeBits int64, oldBitsKnown bool,
	oldType string, oldAutoIncrement, oldIsPKHandle bool, newColumn *Column) (AutoRandomVerdict, string) {
	if newColumn == nil {
		return AutoRandomUndecidable, ""
	}
	newShard, newRange, newReason := alterAutoRandomNewBits(newColumn)
	if newReason != "" {
		return AutoRandomRefused, newReason
	}
	if !oldBitsKnown {
		return AutoRandomUndecidable, ""
	}
	switch {
	case oldShardBits == newShard:
	case oldShardBits < newShard:
		if oldShardBits == 0 && !(oldAutoIncrement && oldIsPKHandle) {
			return AutoRandomRefused, AlterReasonChangeFromAutoIncrement
		}
	default: // oldShardBits > newShard
		if newShard == 0 {
			return AutoRandomRefused, AlterReasonRemoveAutoRandom
		}
		return AutoRandomRefused, AlterReasonDecreaseShardBits
	}
	if oldShardBits > 0 || newShard > 0 {
		// Upstream compares the field-type bytes, not display strings —
		// "bigint(20)" and "bigint" are the same TypeLonglong.
		if !strings.EqualFold(baseColumnTypeName(newColumn.Type), baseColumnTypeName(oldType)) {
			return AutoRandomRefused, AlterReasonModifyColumnType
		}
		if !strings.EqualFold(baseColumnTypeName(oldType), "bigint") {
			return AutoRandomRefused, AlterReasonNonBigint
		}
		if newColumn.AutoIncrement {
			return AutoRandomRefused, AlterReasonWithAutoIncrement
		}
		if newColumn.HasDefault {
			return AutoRandomRefused, AlterReasonWithDefault
		}
	}
	// rangeBitsIsChanged normalises 0 to the 64 default before comparing.
	oldRangeEffective, newRangeEffective := oldRangeBits, newRange
	if oldRangeEffective == 0 {
		oldRangeEffective = AutoRandomDefaultRangeBits
	}
	if newRangeEffective == 0 {
		newRangeEffective = AutoRandomDefaultRangeBits
	}
	if oldRangeEffective != newRangeEffective {
		return AutoRandomRefused, AlterReasonRangeBitsChanged
	}
	if newShard > 0 {
		// checkNewAutoRandomBits reads the live allocator's used
		// incremental bits — a runtime premise offline audit cannot
		// prove, so the applicable branch stays undecidable.
		return AutoRandomUndecidable, AlterReasonRuntimeAllocatorUnproven
	}
	return AutoRandomOK, ""
}

// AlterAutoRandomOldFacts resolves the old side of checkAutoRandom from
// a proven table shape: whether the named old column is a
// clustered-primary-key member (isClusteredPKColumn — PKIsHandle members
// by flag, common-handle members by primary-index membership), the
// table's effective AUTO_RANDOM shard/range bits when it is, the
// IsPKHandleColumn answer, and whether the table carries any AUTO_RANDOM
// column at all. memberKnown=false marks the clustered premise unproven —
// the member question can then swing the bits or the handle answer, so
// callers fold it into undecidable except where the outcome is vacuous
// (a table provably without AUTO_RANDOM reads old bits of zero either
// way). Both the rule layer (metadata snapshots) and the ordered-state
// publisher (batch-derived shapes) consume this one resolver.
func AlterAutoRandomOldFacts(shape *TableSnapshot, oldName string) (
	member, memberKnown bool, shardBits, rangeBits int64, isPKHandle, hasAuto bool) {
	if shape == nil {
		return false, false, 0, 0, false, false
	}
	for _, column := range shape.Columns {
		if column.AutoRandom {
			hasAuto = true
			break
		}
	}
	if shape.PrimaryKey == nil {
		if shape.PrimaryKeyUnknown {
			return false, false, 0, 0, false, hasAuto
		}
		return false, true, 0, 0, false, hasAuto
	}
	for _, column := range shape.PrimaryKey.Columns {
		if strings.EqualFold(strings.TrimSpace(column), oldName) {
			member = true
			break
		}
	}
	if !member {
		return false, true, 0, 0, false, hasAuto
	}
	switch {
	case shape.PrimaryKeyIsHandle:
		isPKHandle = true
	case shape.PrimaryKeyClusteredUnknown:
		// The member could be a clustered member (bits matter) or the
		// PK handle (add arm) — neither answer is proven.
		return true, false, 0, 0, false, hasAuto
	case !shape.PrimaryKeyClustered:
		return false, true, 0, 0, false, hasAuto
	}
	if hasAuto {
		for _, column := range shape.Columns {
			if column.AutoRandom {
				shardBits, rangeBits = AutoRandomEffectiveBits(column)
				break
			}
		}
	}
	return true, true, shardBits, rangeBits, isPKHandle, hasAuto
}

// AlterAutoRandomAddColumnVerdict mirrors the ADD COLUMN refusal: an
// AUTO_RANDOM attribute on an added column is a proven refusal upstream
// (AutoRandomAlterAddColumn), whatever the rest of the batch proves.
func AlterAutoRandomAddColumnVerdict(newColumn *Column) (AutoRandomVerdict, string) {
	if newColumn != nil && newColumn.AutoRandom {
		return AutoRandomRefused, AlterReasonAddColumnAutoRandom
	}
	return AutoRandomOK, ""
}

// ALTER table-option refusal reason tokens.
const (
	AlterOptionReasonPreSplitUnsupported  = "pre_split_regions_unsupported_alter"
	AlterOptionReasonAutoIDCacheOverflow  = "auto_id_cache_overflow"
	AlterOptionReasonAutoIDCacheBoundary  = "auto_id_cache_1_boundary"
	AlterOptionReasonShardTemporary       = "shard_row_id_bits_temporary"
	AlterOptionReasonShardClustered       = "shard_row_id_bits_clustered"
	AlterOptionReasonRebaseNotApplicable  = "auto_random_base_not_applicable"
	AlterOptionReasonRebaseOverflow       = "auto_random_base_overflow"
	AlterOptionReasonOldValueUnproven     = "old_option_value_unproven"
	AlterOptionReasonClusteredUnproven    = "clustered_premise_unproven"
	AlterOptionReasonAutoRandomBitsUnprov = "auto_random_bits_unproven"
	AlterOptionReasonRuntimeRowsUnproven  = "runtime_row_data_unproven"
	AlterOptionReasonValueUnproven        = "option_value_unproven"
)

// AlterLayoutFacts carries the target facts an ALTER layout-option
// verdict needs. Options is the catalog projection (shape.Options) —
// absent keys are unobserved, never a proven zero. Temporary, Clustered,
// and the AUTO_RANDOM layout record what the entry proves; nil/*Known
// bits mark the unproven answers.
type AlterLayoutFacts struct {
	Options                 map[string]string
	Temporary               bool
	Clustered               *bool
	AutoRandomBits          int64
	AutoRandomRangeBits     int64
	AutoRandomBitsKnown     bool
	AutoRandomPKHasUnsigned bool
	HasPKColumn             bool
}

// AlterLayoutOptionOutcome adjudicates one extracted ALTER TABLE option
// name/value pair under TiDB against the fixed dispatch: pre_split_regions
// hits the unsupported-option default; auto_id_cache refuses int64
// overflow and the 1↔non-1 boundary; shard_row_id_bits refuses temporary
// and clustered targets and owes the runtime row-scan for real changes;
// auto_random_base refuses non-AUTO_RANDOM targets and overflow bases and
// owes the runtime-adjusted base without FORCE. The caller supplies the
// option's authored value and the proven target facts.
func AlterLayoutOptionOutcome(name, value string, facts AlterLayoutFacts) (AutoRandomVerdict, string) {
	switch name {
	case "pre_split_regions":
		// Not in the AlterTableOption dispatch — ErrUnsupportedAlterTableOption.
		return AutoRandomRefused, AlterOptionReasonPreSplitUnsupported
	case "auto_id_cache":
		n, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
		if err != nil {
			return AutoRandomUndecidable, AlterOptionReasonValueUnproven
		}
		if n > math.MaxInt64 {
			return AutoRandomRefused, AlterOptionReasonAutoIDCacheOverflow
		}
		old, ok := facts.Options["auto_id_cache"]
		if !ok {
			return AutoRandomUndecidable, AlterOptionReasonOldValueUnproven
		}
		oldN, err := strconv.ParseUint(strings.TrimSpace(old), 10, 64)
		if err != nil {
			return AutoRandomUndecidable, AlterOptionReasonOldValueUnproven
		}
		if (n == 1) != (oldN == 1) {
			return AutoRandomRefused, AlterOptionReasonAutoIDCacheBoundary
		}
		return AutoRandomOK, ""
	case "shard_row_id_bits":
		n, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
		if err != nil {
			return AutoRandomUndecidable, AlterOptionReasonValueUnproven
		}
		if facts.Temporary {
			return AutoRandomRefused, AlterOptionReasonShardTemporary
		}
		if n > createOptionMaxShardRowIDBits {
			n = createOptionMaxShardRowIDBits
		}
		if old, ok := facts.Options["shard_row_id_bits"]; ok {
			if oldN, err := strconv.ParseUint(strings.TrimSpace(old), 10, 64); err == nil && oldN == n {
				// uVal == tbInfo.ShardRowIDBits is an upstream no-op.
				return AutoRandomOK, ""
			}
		}
		if n > 0 {
			if facts.Clustered == nil {
				return AutoRandomUndecidable, AlterOptionReasonClusteredUnproven
			}
			if *facts.Clustered {
				return AutoRandomRefused, AlterOptionReasonShardClustered
			}
		}
		// verifyNoOverflowShardBits scans the live rows — a runtime
		// premise the offline audit cannot prove.
		return AutoRandomUndecidable, AlterOptionReasonRuntimeRowsUnproven
	case "auto_random_base":
		n, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
		if err != nil {
			return AutoRandomUndecidable, AlterOptionReasonValueUnproven
		}
		if !facts.AutoRandomBitsKnown {
			return AutoRandomUndecidable, AlterOptionReasonAutoRandomBitsUnprov
		}
		if facts.AutoRandomBits == 0 || !facts.HasPKColumn {
			return AutoRandomRefused, AlterOptionReasonRebaseNotApplicable
		}
		// IncrementalMask()&newBase != newBase refuses — incremental bits
		// are range minus shard minus the sign bit a signed BIGINT spends.
		incremental := facts.AutoRandomRangeBits - facts.AutoRandomBits
		if !facts.AutoRandomPKHasUnsigned {
			incremental--
		}
		if incremental > 0 && incremental < 63 {
			mask := (uint64(1) << uint(incremental)) - 1
			if n&mask != n {
				return AutoRandomRefused, AlterOptionReasonRebaseOverflow
			}
		}
		return AutoRandomOK, ""
	}
	return AutoRandomUndecidable, AlterOptionReasonValueUnproven
}
