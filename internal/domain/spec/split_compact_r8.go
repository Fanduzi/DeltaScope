package spec

import (
	"math"
	"strings"
)

// split_compact_r8.go implements the T23-R8/R9 outcome adjudication for
// the two standalone TiDB physical-layout statements: SPLIT [REGION
// FOR] [PARTITION] TABLE and ALTER TABLE ... COMPACT. Verdicts share
// the AutoRandomVerdict vocabulary — refused arms emit findings,
// unproven premises emit evidence gaps, and neither statement ever
// publishes a post-state shape because neither moves a catalog fact
// (issue #102).
//
// Upstream anchors (v8.5.0):
//   - SPLIT: pkg/planner/core/planbuilder.go buildSplitIndexRegion /
//     buildSplitTableRegion + convertValueListToData (NULL → ErrBadNull
//     on the table path) / convertValue2ColumnType (index path encodes
//     NULL rather than refusing it), num bound in the planner, then
//     exec-time partition resolution (FindPartitionByName) and the
//     bound checks in pkg/executor/split.go: calculateIntBoundValue
//     (upper<=lower refused, step<minRegionStepValue=1000 refused),
//     common-handle BuildHandleByDatums + Compare, index GenIndexKey +
//     bytes.Compare.
//   - COMPACT: pkg/planner/core/planbuilder.go buildCompactTable (priv +
//     resolve only), then pkg/executor/builder.go buildCompactTable
//     (replica-kind → tikv-storage → partition-name order) and
//     compact_table.go doCompact (no-replica warning no-op).

// Reason codes for the split/compact adjudication — stable vocabulary
// shared by the rule findings, the evidence gaps, and any ordered-state
// consumer.
const (
	LayoutReasonSplitTemporary        = "split_temporary_table"
	LayoutReasonSplitNonPartitioned   = "split_partition_clause_on_nonpartitioned"
	LayoutReasonSplitPrimaryClustered = "split_primary_clustered"
	LayoutReasonSplitIndexMissing     = "split_index_missing"
	LayoutReasonSplitArityIndex       = "split_index_arity_mismatch"
	LayoutReasonSplitArityTable       = "split_table_arity_mismatch"
	LayoutReasonSplitBoundEmpty       = "split_bound_value_empty"
	LayoutReasonSplitNullValue        = "split_null_value"
	LayoutReasonSplitNonNumeric       = "split_non_numeric_value"
	LayoutReasonSplitNumBelowOne      = "split_num_below_one"
	LayoutReasonSplitNumOverLimit     = "split_num_over_limit"
	LayoutReasonSplitPartMissing      = "split_partition_not_found"
	LayoutReasonSplitNonTableObject   = "split_non_table_object"
	// LayoutReasonSplitBoundNotIncreasing is the int-handle BETWEEN arm's
	// upper<=lower refusal (upstream ErrInvalidSplitRegionRanges).
	LayoutReasonSplitBoundNotIncreasing = "split_bound_not_increasing"
	// LayoutReasonSplitStepTooSmall is the int-handle BETWEEN arm's
	// step<minRegionStepValue refusal; step is computed in the arm's
	// 64-bit domain and an int64-overflowing step refuses identically
	// (upstream casts the step back to int64 before comparing to 1000).
	LayoutReasonSplitStepTooSmall = "split_step_too_small"
	// LayoutReasonSplitBoundCompareUnsupported marks the common-handle
	// and index BETWEEN arms whose encoded-key comparison is outside
	// this product's static capability — an honest boundary, never a
	// fabricated complete.
	LayoutReasonSplitBoundCompareUnsupported = "split_bound_compare_unsupported"
	// LayoutReasonSplitBoundConvertUnproven marks a bound position the
	// product cannot fold or convert statically — a float/expression
	// residue, or a literal outside the member column's domain whose
	// upstream outcome depends on the session sql_mode.
	LayoutReasonSplitBoundConvertUnproven = "split_bound_convert_unproven"
	LayoutReasonSplitIndexUnproven        = "split_index_members_unproven"
	LayoutReasonSplitHandleUnproven       = "split_handle_columns_unproven"
	LayoutReasonSplitMemberUnproven       = "split_member_type_unproven"
	LayoutReasonSplitValueExpr            = "split_value_expr_unproven"
	LayoutReasonSplitNumLimitUnproven     = "split_num_limit_unproven"
	LayoutReasonCompactReplicaKind        = "compact_replica_kind_unsupported"
	LayoutReasonCompactNonPartitioned     = "compact_on_nonpartitioned"
	LayoutReasonCompactPartMissing        = "compact_partition_not_found"
	LayoutReasonCompactNotTiKV            = "compact_not_tikv_storage"
	LayoutReasonCompactStoreUnproven      = "compact_tikv_store_unproven"
	LayoutReasonCompactNoReplicaWarn      = "compact_no_tiflash_replica_noop"
)

// SplitValueClass classifies one authored value position — the
// verdict-relevant class, never a verbatim payload echo.
type SplitValueClass uint8

const (
	// SplitValueLiteral is an authored literal whose conversion to the
	// member column type is a bounded boundary — the outcome does not
	// re-simulate datum conversion beyond the integer payload below.
	SplitValueLiteral SplitValueClass = iota
	// SplitValueNull is an authored NULL literal — refused on the table
	// path (upstream convertValueListToData → ErrBadNull), unproven on
	// the index path where convertValue2ColumnType encodes it instead.
	SplitValueNull
	// SplitValueNonNumeric is a string literal that cannot convert to an
	// integer datum — refused against an integer member upstream.
	SplitValueNonNumeric
	// SplitValueExpr is a non-literal expression — constant foldability
	// is unproven without an evaluator.
	SplitValueExpr
)

// SplitBoundInt marks which 64-bit domain a foldable integer payload
// belongs to; SplitIntNone means the position carries no payload at all
// (a NULL, an expression, or a literal that does not fold to an
// integer). A payload is never implied by a zero value — the class
// field decides.
type SplitBoundInt uint8

const (
	SplitIntNone SplitBoundInt = iota
	// SplitIntSigned carries Int64.
	SplitIntSigned
	// SplitIntUnsigned carries Uint64.
	SplitIntUnsigned
)

// SplitBoundValue is one authored bound position: the value class plus
// the foldable 64-bit integer payload the int-handle range/step arm
// needs (issue #102 T23-R9). Floats and non-integer numerics never
// carry the payload; the raw literal text and any encoded form stay
// out of the model entirely.
type SplitBoundValue struct {
	Class  SplitValueClass
	Int    SplitBoundInt
	Int64  int64
	Uint64 uint64
}

// SplitBound carries the authored BETWEEN/lower-upper or BY value tuples
// as positional bound values — class plus typed integer payload where
// statically foldable, never the raw expression text.
type SplitBound struct {
	// Arities holds the authored expression count per BY tuple; nil for
	// the BETWEEN form.
	Arities []int
	// Values aligns with Arities: per tuple, per position.
	Values [][]SplitBoundValue
	// LowerArity/UpperArity carry the authored BETWEEN bound widths;
	// empty arities are refused upstream. Lower/Upper hold the typed
	// bound values aligned with them.
	LowerArity int
	UpperArity int
	Lower      []SplitBoundValue
	Upper      []SplitBoundValue
}

// SplitRegionDecl is the authored SPLIT statement — object, member,
// parameter, and clause facts, with no runtime or catalog state.
type SplitRegionDecl struct {
	// IndexName is the authored index target, lower-cased; "" selects
	// the table (handle) path.
	IndexName string
	// HasPartitionClause marks the `SPLIT [REGION FOR] PARTITION TABLE`
	// keyword-level variant — refused on a non-partitioned target.
	HasPartitionClause bool
	// PartitionNames lists authored member names, lower-cased; resolved
	// only on a partitioned target and ignored upstream otherwise.
	PartitionNames []string
	// Bound carries BY tuples (Arities set) or BETWEEN bounds plus the
	// authored REGIONS count.
	Bound SplitBound
	// Num is the authored REGIONS count on the BETWEEN form; nil on BY.
	Num *int64
}

// CompactTableDecl is the authored COMPACT statement.
type CompactTableDecl struct {
	// ReplicaKind is the normalized authored replica kind — "tiflash"
	// or "all" (bare COMPACT). The grammar never emits "tikv"; the
	// outcome still guards it so a hand-built model stays refused.
	ReplicaKind string
	// PartitionNames lists authored member names, lower-cased; nil for
	// the whole-table form.
	PartitionNames []string
}

// SplitCompactFacts carries the catalog/service facts the R8 outcomes
// adjudicate against. Known flags separate proven answers from
// unobserved ones — a zero value is never evidence.
type SplitCompactFacts struct {
	Temporary      bool
	ViewOrSequence bool
	// Partitioned is the proven partitioning answer.
	Partitioned      bool
	PartitionedKnown bool
	// Members is the proven partition-member name set (lower-cased).
	Members      map[string]bool
	MembersKnown bool
	// Clustered/PrimaryExist answer the INDEX PRIMARY arm:
	// clustered-primary refuses; a proven nonclustered primary index
	// splits its own regions; absent primary misses the index lookup.
	Clustered      bool
	ClusteredKnown bool
	PrimaryExist   bool
	PrimaryKnown   bool
	// IndexMembers maps a proven index name (lower-cased, "primary"
	// included when a real index object exists) to per-member
	// int-typed flags; len() is the member count.
	IndexMembers      map[string][]bool
	IndexMembersKnown bool
	// IndexMemberTypesKnown marks the indexes whose member column
	// types were all observed — a false entry keeps the conversion arm
	// undecidable instead of guessing a member type.
	IndexMemberTypesKnown map[string]bool
	// HandleInt carries the handle member types for the table path:
	// PKIsHandle→[int], common-handle→pk member types, otherwise the
	// synthetic rowid handle [int]. HandleTypesKnown=false marks a
	// member type the snapshot could not vouch. HandleCommonHandle
	// marks the clustered non-PKIsHandle arm — upstream compares its
	// bounds through BuildHandleByDatums encodings (never
	// calculateIntBoundValue), so even a single-int common handle is a
	// capability boundary, not an int-arithmetic arm.
	HandleInt          []bool
	HandleKnown        bool
	HandleTypesKnown   bool
	HandleCommonHandle bool
	// HandleUnsigned carries the per-member unsigned flag for the int
	// handle arm — the conversion domain for bound values. Upstream
	// checks the unsigned flag only for PKIsHandle (the signed arm is
	// used even on an unsigned common-handle int column), so
	// HandlePKIsHandle records that provenance for the arm-domain
	// selection. HandleUnsignedKnown=false when the pk column type was
	// unobserved.
	HandleUnsigned      []bool
	HandleUnsignedKnown bool
	HandlePKIsHandle    bool
	// MaxRegions is the proven split-region-max-num instance config.
	MaxRegions      int64
	MaxRegionsKnown bool
	// TiKVStore is the proven "server runs tikv-compatible storage"
	// answer — a real connected TiDB always is; unproven offline.
	TiKVStore      bool
	TiKVStoreKnown bool
	// ReplicaZero reports a proven absent TiFlash replica declaration,
	// which upstream answers with a warning no-op, never a refusal.
	ReplicaZero  bool
	ReplicaKnown bool
}

// splitValueCheck runs one authored tuple against the upstream
// conversion checks shared by BY items and BETWEEN bounds: every
// non-literal stays unproven, a NULL literal is refused on the table
// path (upstream convertValueListToData → ErrBadNull) but unproven on
// the index path (convertValue2ColumnType has no NULL gate — encoded
// comparison is outside our capability), and a provably non-numeric
// string is refused against an integer member. Member type unknown
// keeps the conversion undecidable.
func splitValueCheck(values []SplitBoundValue, memberInt []bool, typesKnown, tablePath bool) (AutoRandomVerdict, string) {
	for pos, value := range values {
		switch value.Class {
		case SplitValueExpr:
			return AutoRandomUndecidable, LayoutReasonSplitValueExpr
		case SplitValueNull:
			if tablePath {
				return AutoRandomRefused, LayoutReasonSplitNullValue
			}
			return AutoRandomUndecidable, LayoutReasonSplitValueExpr
		case SplitValueNonNumeric:
			if pos >= len(memberInt) {
				return AutoRandomUndecidable, LayoutReasonSplitMemberUnproven
			}
			if !typesKnown {
				return AutoRandomUndecidable, LayoutReasonSplitMemberUnproven
			}
			if memberInt[pos] {
				return AutoRandomRefused, LayoutReasonSplitNonNumeric
			}
		case SplitValueLiteral:
			if pos >= len(memberInt) {
				return AutoRandomUndecidable, LayoutReasonSplitMemberUnproven
			}
		}
	}
	return AutoRandomOK, ""
}

// splitMinRegionStep is the executor's fixed minimum step between two
// split points — a compile-time upstream constant (v8.5.0
// pkg/executor/split.go minRegionStepValue), not a server config.
const splitMinRegionStep int64 = 1000

// boundArmBits converts one proven literal position to the member
// column's domain bits — the datum's 64-bit storage after upstream
// convertValue. A value outside the column domain resolves to
// sql_mode-dependent upstream behavior (strict error vs clipped
// warning), so it stays unconvertible rather than guessed.
func boundArmBits(value SplitBoundValue, colUnsigned bool) (uint64, bool) {
	if value.Class != SplitValueLiteral {
		return 0, false
	}
	switch value.Int {
	case SplitIntSigned:
		if colUnsigned && value.Int64 < 0 {
			return 0, false
		}
		return uint64(value.Int64), true
	case SplitIntUnsigned:
		if !colUnsigned && value.Uint64 > math.MaxInt64 {
			return 0, false
		}
		return value.Uint64, true
	}
	return 0, false
}

// splitBoundCompare applies the executor-time bound checks that run
// after partition resolution for the BETWEEN form. Only the
// single-int-handle arm is adjudicated statically — upstream compares
// record IDs in calculateIntBoundValue there. The common-handle and
// index arms compare encoded keys, a capability boundary this product
// discloses instead of completing. BY tuples carry no relation or
// step semantics upstream and never reach this function.
func splitBoundCompare(decl *SplitRegionDecl, facts SplitCompactFacts, memberInt []bool, typesKnown, tablePath bool) (AutoRandomVerdict, string) {
	if !typesKnown {
		return AutoRandomUndecidable, LayoutReasonSplitMemberUnproven
	}
	if !tablePath {
		// Index bounds compare on GenIndexKey encodings.
		return AutoRandomUndecidable, LayoutReasonSplitBoundCompareUnsupported
	}
	if facts.HandleCommonHandle || len(memberInt) != 1 || !memberInt[0] {
		// Every common handle — single int member included — compares on
		// BuildHandleByDatums encodings upstream; the int-arithmetic arm
		// is only the PKIsHandle / synthetic rowid handle.
		return AutoRandomUndecidable, LayoutReasonSplitBoundCompareUnsupported
	}
	if !facts.HandleUnsignedKnown || len(facts.HandleUnsigned) != 1 {
		return AutoRandomUndecidable, LayoutReasonSplitMemberUnproven
	}
	if decl.Num == nil || len(decl.Bound.Lower) == 0 || len(decl.Bound.Upper) == 0 {
		return AutoRandomUndecidable, "split_declaration_unproven"
	}
	colUnsigned := facts.HandleUnsigned[0]
	lower, ok := boundArmBits(decl.Bound.Lower[0], colUnsigned)
	if !ok {
		return AutoRandomUndecidable, LayoutReasonSplitBoundConvertUnproven
	}
	upper, ok := boundArmBits(decl.Bound.Upper[0], colUnsigned)
	if !ok {
		return AutoRandomUndecidable, LayoutReasonSplitBoundConvertUnproven
	}
	// The arm domain is unsigned only under PKIsHandle — upstream's
	// isUnsigned flag never reads a common-handle column's flag, so an
	// unsigned single-int common handle still compares signed int64.
	if facts.HandlePKIsHandle && colUnsigned {
		if upper <= lower {
			return AutoRandomRefused, LayoutReasonSplitBoundNotIncreasing
		}
		step := (upper - lower) / uint64(*decl.Num)
		if int64(step) < splitMinRegionStep {
			return AutoRandomRefused, LayoutReasonSplitStepTooSmall
		}
		return AutoRandomOK, ""
	}
	lowerSigned := int64(lower)
	upperSigned := int64(upper)
	if upperSigned <= lowerSigned {
		return AutoRandomRefused, LayoutReasonSplitBoundNotIncreasing
	}
	// int64 subtraction wraps but uint64() of the wrapped result equals
	// the mathematical diff whenever upper>lower — the same arithmetic
	// upstream performs before casting the step back to int64.
	step := uint64(upperSigned-lowerSigned) / uint64(*decl.Num)
	if int64(step) < splitMinRegionStep {
		return AutoRandomRefused, LayoutReasonSplitStepTooSmall
	}
	return AutoRandomOK, ""
}

// splitNumCheck applies the shared REGIONS bound: <1 refuses, >proven
// limit refuses, and an unproven limit keeps the acceptance undecidable.
func splitNumCheck(decl *SplitRegionDecl, facts SplitCompactFacts) (AutoRandomVerdict, string) {
	if decl.Num == nil {
		return AutoRandomOK, ""
	}
	if *decl.Num < 1 {
		return AutoRandomRefused, LayoutReasonSplitNumBelowOne
	}
	if !facts.MaxRegionsKnown {
		return AutoRandomUndecidable, LayoutReasonSplitNumLimitUnproven
	}
	if *decl.Num > facts.MaxRegions {
		return AutoRandomRefused, LayoutReasonSplitNumOverLimit
	}
	return AutoRandomOK, ""
}

// splitPartitionCheck applies the exec-time member resolution:
// non-partitioned targets ignore the clause outright, named members on
// a partitioned target refuse when absent, and an unproven member space
// keeps the outcome undecidable.
func splitPartitionCheck(names []string, missingReason string, facts SplitCompactFacts) (AutoRandomVerdict, string) {
	if len(names) == 0 {
		return AutoRandomOK, ""
	}
	if facts.PartitionedKnown && !facts.Partitioned {
		// Upstream executor skips member resolution entirely on a
		// non-partitioned table — the names are ignored, not refused.
		return AutoRandomOK, ""
	}
	if !facts.PartitionedKnown {
		return AutoRandomUndecidable, LayoutReasonPartitionedUnproven
	}
	if !facts.MembersKnown {
		return AutoRandomUndecidable, LayoutReasonMembersUnproven
	}
	for _, name := range names {
		if !facts.Members[name] {
			return AutoRandomRefused, missingReason
		}
	}
	return AutoRandomOK, ""
}

// SplitRegionOutcome adjudicates a SPLIT statement in upstream order:
// temporary → SPLIT PARTITION TABLE clause on non-partitioned → index
// arms (clustered primary, index existence) → value tuples → REGIONS
// bound → exec-time partition member resolution. The verdict never
// claims a topology change — OK means the statement clears the modeled
// premises; the actual region scatter is out of scope.
func SplitRegionOutcome(dialect Dialect, decl *SplitRegionDecl, facts SplitCompactFacts) (AutoRandomVerdict, string) {
	if dialect != DialectTiDB {
		return AutoRandomRefused, "split_not_mysql"
	}
	if decl == nil {
		return AutoRandomUndecidable, "split_declaration_unproven"
	}
	if facts.Temporary {
		return AutoRandomRefused, LayoutReasonSplitTemporary
	}
	if decl.HasPartitionClause {
		if facts.PartitionedKnown && !facts.Partitioned {
			return AutoRandomRefused, LayoutReasonSplitNonPartitioned
		}
		if !facts.PartitionedKnown {
			return AutoRandomUndecidable, LayoutReasonPartitionedUnproven
		}
	}
	var memberInt []bool
	typesKnown := true
	if decl.IndexName != "" {
		if decl.IndexName == "primary" {
			if !facts.ClusteredKnown || !facts.PrimaryKnown {
				return AutoRandomUndecidable, LayoutReasonSplitIndexUnproven
			}
			if facts.Clustered {
				return AutoRandomRefused, LayoutReasonSplitPrimaryClustered
			}
			if !facts.PrimaryExist {
				return AutoRandomRefused, LayoutReasonSplitIndexMissing
			}
		}
		if !facts.IndexMembersKnown {
			return AutoRandomUndecidable, LayoutReasonSplitIndexUnproven
		}
		members, ok := facts.IndexMembers[decl.IndexName]
		if !ok {
			if decl.IndexName != "primary" {
				return AutoRandomRefused, LayoutReasonSplitIndexMissing
			}
			return AutoRandomUndecidable, LayoutReasonSplitIndexUnproven
		}
		memberInt = members
		typesKnown = facts.IndexMemberTypesKnown[decl.IndexName]
	} else {
		if facts.ViewOrSequence {
			return AutoRandomUndecidable, LayoutReasonSplitNonTableObject
		}
		if !facts.HandleKnown {
			return AutoRandomUndecidable, LayoutReasonSplitHandleUnproven
		}
		memberInt = facts.HandleInt
		typesKnown = facts.HandleTypesKnown
	}
	tablePath := decl.IndexName == ""
	if len(decl.Bound.Arities) > 0 {
		for i, arity := range decl.Bound.Arities {
			if tablePath {
				if arity != len(memberInt) {
					return AutoRandomRefused, LayoutReasonSplitArityTable
				}
			} else if arity > len(memberInt) {
				return AutoRandomRefused, LayoutReasonSplitArityIndex
			}
			if i < len(decl.Bound.Values) {
				if verdict, reason := splitValueCheck(decl.Bound.Values[i], memberInt, typesKnown, tablePath); verdict != AutoRandomOK {
					return verdict, reason
				}
			}
		}
	} else {
		if decl.Bound.LowerArity == 0 || decl.Bound.UpperArity == 0 {
			return AutoRandomRefused, LayoutReasonSplitBoundEmpty
		}
		if tablePath {
			if decl.Bound.LowerArity != len(memberInt) || decl.Bound.UpperArity != len(memberInt) {
				return AutoRandomRefused, LayoutReasonSplitArityTable
			}
		} else {
			if decl.Bound.LowerArity > len(memberInt) || decl.Bound.UpperArity > len(memberInt) {
				return AutoRandomRefused, LayoutReasonSplitArityIndex
			}
		}
		if verdict, reason := splitValueCheck(decl.Bound.Lower, memberInt, typesKnown, tablePath); verdict != AutoRandomOK {
			return verdict, reason
		}
		if verdict, reason := splitValueCheck(decl.Bound.Upper, memberInt, typesKnown, tablePath); verdict != AutoRandomOK {
			return verdict, reason
		}
	}
	if verdict, reason := splitNumCheck(decl, facts); verdict != AutoRandomOK {
		return verdict, reason
	}
	if verdict, reason := splitPartitionCheck(decl.PartitionNames, LayoutReasonSplitPartMissing, facts); verdict != AutoRandomOK {
		return verdict, reason
	}
	if len(decl.Bound.Arities) == 0 {
		// BETWEEN only: the executor's bound-relation and minimum-step
		// checks run after partition resolution. BY tuples have no
		// relation or step semantics upstream.
		return splitBoundCompare(decl, facts, memberInt, typesKnown, tablePath)
	}
	return AutoRandomOK, ""
}

// CompactTableOutcome adjudicates ALTER TABLE ... COMPACT in upstream
// executor-builder order: replica kind → tikv-compatible storage →
// partition-member resolution. A proven absent TiFlash replica is a
// warning no-op upstream, surfaced as OK with the no-op reason so the
// rule can raise the bounded notice.
func CompactTableOutcome(dialect Dialect, decl *CompactTableDecl, facts SplitCompactFacts) (AutoRandomVerdict, string) {
	if dialect != DialectTiDB {
		return AutoRandomRefused, "compact_not_mysql"
	}
	if decl == nil {
		return AutoRandomUndecidable, "compact_declaration_unproven"
	}
	switch decl.ReplicaKind {
	case "tiflash", "all", "":
		// "" covers a hand-built model without a kind; the grammar
		// always emits one and never emits "tikv" — that arm is still
		// guarded below so an unexpected kind stays refused.
	case "tikv":
		return AutoRandomRefused, LayoutReasonCompactReplicaKind
	default:
		return AutoRandomRefused, LayoutReasonCompactReplicaKind
	}
	if !facts.TiKVStoreKnown {
		return AutoRandomUndecidable, LayoutReasonCompactStoreUnproven
	}
	if !facts.TiKVStore {
		return AutoRandomRefused, LayoutReasonCompactNotTiKV
	}
	if decl.PartitionNames != nil {
		if facts.PartitionedKnown && !facts.Partitioned {
			return AutoRandomRefused, LayoutReasonCompactNonPartitioned
		}
		if !facts.PartitionedKnown {
			return AutoRandomUndecidable, LayoutReasonPartitionedUnproven
		}
		if !facts.MembersKnown {
			return AutoRandomUndecidable, LayoutReasonMembersUnproven
		}
		for _, name := range decl.PartitionNames {
			if !facts.Members[name] {
				return AutoRandomRefused, LayoutReasonCompactPartMissing
			}
		}
	}
	if facts.ReplicaKnown && facts.ReplicaZero {
		return AutoRandomOK, LayoutReasonCompactNoReplicaWarn
	}
	return AutoRandomOK, ""
}

// intTypedName reports whether a column type string names an integer
// family — the member-type class for the non-numeric conversion arm.
func splitMemberInt(columnType string) bool {
	t := strings.ToLower(strings.TrimSpace(columnType))
	switch t {
	case "tinyint", "smallint", "mediumint", "int", "bigint", "integer", "year":
		return true
	}
	return false
}

// SplitCompactFactsForSnapshot projects a bound snapshot into the
// adjudication facts — mirroring AlterLayoutR6FactsForSnapshot's known
// flags so a withheld collection stays unproven rather than absent.
func SplitCompactFactsForSnapshot(shape *TableSnapshot, instance *InstanceFacts) SplitCompactFacts {
	facts := SplitCompactFacts{}
	if shape == nil {
		return facts
	}
	facts.Temporary = shape.Temporary || shape.GlobalTemporary
	facts.ViewOrSequence = shape.RelationKind == "view" ||
		shape.RelationKind == "sequence" || shape.View != nil
	if shape.Partitioned != nil {
		facts.PartitionedKnown = true
		facts.Partitioned = *shape.Partitioned
	}
	if facts.Partitioned && shape.PartitionNames != nil {
		facts.MembersKnown = true
		facts.Members = map[string]bool{}
		for _, name := range shape.PartitionNames {
			facts.Members[strings.ToLower(name)] = true
		}
	}
	if !shape.PrimaryKeyClusteredUnknown {
		facts.ClusteredKnown = true
		facts.Clustered = shape.PrimaryKeyClustered || shape.PrimaryKeyIsHandle
	}
	if !shape.PrimaryKeyUnknown {
		facts.PrimaryKnown = true
		facts.PrimaryExist = shape.PrimaryKey != nil
	}
	columnInt := func(name string) (intTyped, observed bool) {
		for _, column := range shape.Columns {
			if strings.EqualFold(column.Name, name) {
				if column.Type == "" {
					return false, false
				}
				return splitMemberInt(column.Type), true
			}
		}
		return false, false
	}
	if !shape.IndexesUnknown {
		facts.IndexMembersKnown = true
		facts.IndexMembers = map[string][]bool{}
		facts.IndexMemberTypesKnown = map[string]bool{}
		memberInts := func(columns []string) ([]bool, bool) {
			out := make([]bool, len(columns))
			known := true
			for i, name := range columns {
				intTyped, observed := columnInt(name)
				out[i] = intTyped
				if !observed {
					known = false
				}
			}
			return out, known
		}
		for _, index := range shape.Indexes {
			ints, known := memberInts(index.Columns)
			name := strings.ToLower(index.Name)
			facts.IndexMembers[name] = ints
			facts.IndexMemberTypesKnown[name] = known
		}
		if shape.PrimaryKey != nil {
			ints, known := memberInts(shape.PrimaryKey.Columns)
			facts.IndexMembers["primary"] = ints
			facts.IndexMemberTypesKnown["primary"] = known
		}
	}
	columnUnsigned := func(name string) (unsigned, observed bool) {
		for _, column := range shape.Columns {
			if strings.EqualFold(column.Name, name) {
				if column.Type == "" {
					return false, false
				}
				return strings.Contains(strings.ToLower(column.Type), "unsigned"), true
			}
		}
		return false, false
	}
	switch {
	case shape.PrimaryKeyIsHandle:
		// PKIsHandle is a single integer primary column; its unsigned
		// flag selects the unsigned arm domain upstream.
		facts.HandleKnown = true
		facts.HandleInt = []bool{true}
		facts.HandleTypesKnown = true
		facts.HandlePKIsHandle = true
		if shape.PrimaryKey != nil && len(shape.PrimaryKey.Columns) == 1 {
			unsigned, observed := columnUnsigned(shape.PrimaryKey.Columns[0])
			facts.HandleUnsignedKnown = observed
			facts.HandleUnsigned = []bool{unsigned}
		}
	case shape.PrimaryKeyClustered && shape.PrimaryKey != nil && !shape.PrimaryKeyUnknown:
		// Common-handle tables split on the primary member tuple —
		// compared through BuildHandleByDatums encodings upstream even
		// for a single int member, so the arm is marked common rather
		// than int-arithmetic. The unsigned flags are still projected
		// for the member-type consumers.
		facts.HandleKnown = true
		facts.HandleTypesKnown = true
		facts.HandleCommonHandle = true
		facts.HandleInt = make([]bool, len(shape.PrimaryKey.Columns))
		facts.HandleUnsigned = make([]bool, len(shape.PrimaryKey.Columns))
		facts.HandleUnsignedKnown = true
		for i, name := range shape.PrimaryKey.Columns {
			intTyped, observed := columnInt(name)
			facts.HandleInt[i] = intTyped
			unsigned, flagObserved := columnUnsigned(name)
			facts.HandleUnsigned[i] = unsigned
			if !observed || !flagObserved {
				facts.HandleTypesKnown = false
				facts.HandleUnsignedKnown = false
			}
		}
	case shape.PrimaryKeyHandleUnknown || shape.PrimaryKeyClusteredUnknown || shape.PrimaryKeyUnknown:
		facts.HandleKnown = false
	default:
		// Nonclustered and keyless tables split on the synthetic
		// _tidb_rowid handle — a single signed integer member.
		facts.HandleKnown = true
		facts.HandleInt = []bool{true}
		facts.HandleTypesKnown = true
		facts.HandleUnsigned = []bool{false}
		facts.HandleUnsignedKnown = true
	}
	if count, ok := shape.Options["tiflash_replica_count"]; ok {
		facts.ReplicaKnown = true
		facts.ReplicaZero = count == "" || count == "0"
	}
	if instance != nil && instance.TiDBSplitRegionMaxNumKnown {
		facts.MaxRegionsKnown = true
		facts.MaxRegions = int64(instance.TiDBSplitRegionMaxNum)
	}
	// A real connected TiDB server is tikv-compatible by construction —
	// this product only ever observes servers through the MySQL
	// protocol, and the only deployment reaching that path is a real
	// TiDB node. (Upstream's tikv.Storage assertion also holds for
	// unistore test backends, which embed a real *tikv.KVStore — but no
	// unistore backend ever serves a MySQL-protocol connection here.)
	// An observed TiDB version proves the store; an unproven version
	// leaves it unknown. The version-string check is a provenance gate
	// for the servers this product can actually reach, not a claim
	// that any string containing "-tidb-" is trustworthy.
	if instance != nil && strings.Contains(strings.ToLower(instance.Version), "-tidb-") {
		facts.TiKVStoreKnown = true
		facts.TiKVStore = true
	}
	return facts
}
