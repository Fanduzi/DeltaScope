package spec

import "strings"

// split_compact_r8.go implements the T23-R8 outcome adjudication for the
// two standalone TiDB physical-layout statements that remained
// recognized-but-unmodeled after R7: SPLIT [REGION FOR] [PARTITION] TABLE
// and ALTER TABLE ... COMPACT. Verdicts share the AutoRandomVerdict
// vocabulary — refused arms emit findings, unproven premises emit
// evidence gaps, and neither statement ever publishes a post-state shape
// because neither moves a catalog fact (issue #102).
//
// Upstream anchors (v8.5.0):
//   - SPLIT: pkg/planner/core/planbuilder.go buildSplitRegion /
//     buildSplitIndexRegion / buildSplitTableRegion +
//     convertValueListToData / convertValue, then exec-time partition
//     resolution in pkg/executor/split.go (FindPartitionByName).
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
	LayoutReasonSplitIndexUnproven    = "split_index_members_unproven"
	LayoutReasonSplitHandleUnproven   = "split_handle_columns_unproven"
	LayoutReasonSplitMemberUnproven   = "split_member_type_unproven"
	LayoutReasonSplitValueExpr        = "split_value_expr_unproven"
	LayoutReasonSplitNumLimitUnproven = "split_num_limit_unproven"
	LayoutReasonCompactReplicaKind    = "compact_replica_kind_unsupported"
	LayoutReasonCompactNonPartitioned = "compact_on_nonpartitioned"
	LayoutReasonCompactPartMissing    = "compact_partition_not_found"
	LayoutReasonCompactNotTiKV        = "compact_not_tikv_storage"
	LayoutReasonCompactStoreUnproven  = "compact_tikv_store_unproven"
	LayoutReasonCompactNoReplicaWarn  = "compact_no_tiflash_replica_noop"
)

// SplitValueClass classifies one authored value position. The literal
// payload itself never enters the model — only the verdict-relevant
// class.
type SplitValueClass uint8

const (
	// SplitValueLiteral is an authored literal whose conversion to the
	// member column type is a bounded boundary — the outcome does not
	// re-simulate datum conversion, so it stays a plain literal class.
	SplitValueLiteral SplitValueClass = iota
	// SplitValueNull is an authored NULL literal — refused on the table
	// path (ErrBadNull), unproven on the index path.
	SplitValueNull
	// SplitValueNonNumeric is a string literal that cannot convert to an
	// integer datum — refused against an integer member upstream.
	SplitValueNonNumeric
	// SplitValueExpr is a non-literal expression — constant foldability
	// is unproven without an evaluator.
	SplitValueExpr
)

// SplitBound carries the authored BETWEEN/lower-upper or BY value tuples
// as positional classes — never the literal payloads.
type SplitBound struct {
	// Arities holds the authored expression count per BY tuple; nil for
	// the BETWEEN form.
	Arities []int
	// Classes aligns with Arities: per tuple, per position.
	Classes [][]SplitValueClass
	// LowerArity/UpperArity and LowerClasses/UpperClasses carry the
	// BETWEEN bounds; empty arities are refused upstream.
	LowerArity   int
	UpperArity   int
	LowerClasses []SplitValueClass
	UpperClasses []SplitValueClass
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
	// member type the snapshot could not vouch.
	HandleInt        []bool
	HandleKnown      bool
	HandleTypesKnown bool
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
// path (ErrBadNull) but unproven on the index path (no NULL gate
// upstream — conversion behavior is not modeled), and a provably
// non-numeric string is refused against an integer member. Member type
// unknown keeps the conversion undecidable.
func splitValueCheck(classes []SplitValueClass, memberInt []bool, typesKnown, tablePath bool) (AutoRandomVerdict, string) {
	for pos, class := range classes {
		switch class {
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
			if i < len(decl.Bound.Classes) {
				if verdict, reason := splitValueCheck(decl.Bound.Classes[i], memberInt, typesKnown, tablePath); verdict != AutoRandomOK {
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
		if verdict, reason := splitValueCheck(decl.Bound.LowerClasses, memberInt, typesKnown, tablePath); verdict != AutoRandomOK {
			return verdict, reason
		}
		if verdict, reason := splitValueCheck(decl.Bound.UpperClasses, memberInt, typesKnown, tablePath); verdict != AutoRandomOK {
			return verdict, reason
		}
	}
	if verdict, reason := splitNumCheck(decl, facts); verdict != AutoRandomOK {
		return verdict, reason
	}
	return splitPartitionCheck(decl.PartitionNames, LayoutReasonSplitPartMissing, facts)
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
	switch {
	case shape.PrimaryKeyIsHandle:
		// PKIsHandle is a single integer primary column.
		facts.HandleKnown = true
		facts.HandleInt = []bool{true}
		facts.HandleTypesKnown = true
	case shape.PrimaryKeyClustered && shape.PrimaryKey != nil && !shape.PrimaryKeyUnknown:
		// Common-handle tables split on the primary member tuple.
		facts.HandleKnown = true
		facts.HandleTypesKnown = true
		facts.HandleInt = make([]bool, len(shape.PrimaryKey.Columns))
		for i, name := range shape.PrimaryKey.Columns {
			intTyped, observed := columnInt(name)
			facts.HandleInt[i] = intTyped
			if !observed {
				facts.HandleTypesKnown = false
			}
		}
	case shape.PrimaryKeyHandleUnknown || shape.PrimaryKeyClusteredUnknown || shape.PrimaryKeyUnknown:
		facts.HandleKnown = false
	default:
		// Nonclustered and keyless tables split on the synthetic
		// _tidb_rowid handle — a single integer member.
		facts.HandleKnown = true
		facts.HandleInt = []bool{true}
		facts.HandleTypesKnown = true
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
	// the compact storage check only fails on mock/unistore backends
	// that never serve a production connection. An observed TiDB
	// version proves the store; unproven version leaves it unknown.
	if instance != nil && strings.Contains(strings.ToLower(instance.Version), "-tidb-") {
		facts.TiKVStoreKnown = true
		facts.TiKVStore = true
	}
	return facts
}
