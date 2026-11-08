package spec

import "testing"

// split_compact_r8_test.go pins the T23-R8/R9 SPLIT/COMPACT outcome
// matrix against the upstream v8.5.0 anchors cited in
// split_compact_r8.go: temporary/identity arms, handle vs index member
// arity, NULL and non-numeric conversion refusals, the REGIONS bound,
// exec-time partition-member resolution, the int-handle BETWEEN
// relation/step arm (upper<=lower, step<1000), the encoded-compare
// capability boundary on common-handle/index BETWEEN, and the COMPACT
// replica-kind/tikv-store order plus the no-TiFlash warning no-op
// (issue #102).

func baseSplitFacts() SplitCompactFacts {
	return SplitCompactFacts{
		Partitioned:         false,
		PartitionedKnown:    true,
		ClusteredKnown:      true,
		PrimaryKnown:        true,
		PrimaryExist:        false,
		HandleKnown:         true,
		HandleTypesKnown:    true,
		HandleInt:           []bool{true},
		HandleUnsigned:      []bool{false},
		HandleUnsignedKnown: true,
		MaxRegionsKnown:     true,
		MaxRegions:          1000,
		TiKVStore:           true,
		TiKVStoreKnown:      true,
		ReplicaKnown:        true,
	}
}

// bound helpers build typed bound values the way the extractor does.
func boundInt(v int64) SplitBoundValue {
	return SplitBoundValue{Class: SplitValueLiteral, Int: SplitIntSigned, Int64: v}
}

func boundUint(v uint64) SplitBoundValue {
	return SplitBoundValue{Class: SplitValueLiteral, Int: SplitIntUnsigned, Uint64: v}
}

func boundLit() SplitBoundValue {
	return SplitBoundValue{Class: SplitValueLiteral}
}

func boundClass(c SplitValueClass) SplitBoundValue {
	return SplitBoundValue{Class: c}
}

func splitBetween(lower, upper int, num int64, lowerVals, upperVals []SplitBoundValue) *SplitRegionDecl {
	return &SplitRegionDecl{
		Bound: SplitBound{
			LowerArity: lower,
			UpperArity: upper,
			Lower:      lowerVals,
			Upper:      upperVals,
		},
		Num: &num,
	}
}

// splitBetweenInt builds the common single-int BETWEEN declaration.
func splitBetweenInt(lower, upper, num int64) *SplitRegionDecl {
	return splitBetween(1, 1, num, []SplitBoundValue{boundInt(lower)}, []SplitBoundValue{boundInt(upper)})
}

func splitBy(tuples [][]SplitBoundValue) *SplitRegionDecl {
	decl := &SplitRegionDecl{}
	for _, tuple := range tuples {
		decl.Bound.Arities = append(decl.Bound.Arities, len(tuple))
		decl.Bound.Values = append(decl.Bound.Values, tuple)
	}
	return decl
}

func TestT23R8SplitTableOutcome(t *testing.T) {
	lit := []SplitBoundValue{boundLit()}
	intBound := []SplitBoundValue{boundInt(0)}
	intBoundUp := []SplitBoundValue{boundInt(10000)}
	cases := []struct {
		name    string
		decl    *SplitRegionDecl
		mutate  func(*SplitCompactFacts)
		verdict AutoRandomVerdict
		reason  string
	}{
		{"between_ok", splitBetween(1, 1, 4, intBound, intBoundUp), nil, AutoRandomOK, ""},
		{"by_ok", splitBy([][]SplitBoundValue{lit, lit}), nil, AutoRandomOK, ""},
		{"temporary_refused", splitBetweenInt(0, 10000, 4), func(f *SplitCompactFacts) { f.Temporary = true }, AutoRandomRefused, LayoutReasonSplitTemporary},
		{"view_object_undecidable", splitBetweenInt(0, 10000, 4), func(f *SplitCompactFacts) { f.ViewOrSequence = true }, AutoRandomUndecidable, LayoutReasonSplitNonTableObject},
		{"partition_clause_nonpartitioned", func() *SplitRegionDecl {
			d := splitBetweenInt(0, 10000, 4)
			d.HasPartitionClause = true
			return d
		}(), nil, AutoRandomRefused, LayoutReasonSplitNonPartitioned},
		{"partition_clause_unproven", func() *SplitRegionDecl {
			d := splitBetweenInt(0, 10000, 4)
			d.HasPartitionClause = true
			return d
		}(), func(f *SplitCompactFacts) { f.PartitionedKnown = false }, AutoRandomUndecidable, LayoutReasonPartitionedUnproven},
		{"handle_unproven", splitBetweenInt(0, 10000, 4), func(f *SplitCompactFacts) { f.HandleKnown = false }, AutoRandomUndecidable, LayoutReasonSplitHandleUnproven},
		{"table_arity_mismatch", splitBetween(2, 2, 4, []SplitBoundValue{lit[0], lit[0]}, []SplitBoundValue{lit[0], lit[0]}), nil, AutoRandomRefused, LayoutReasonSplitArityTable},
		{"bound_empty_lower", splitBetween(0, 1, 4, nil, lit), nil, AutoRandomRefused, LayoutReasonSplitBoundEmpty},
		{"bound_empty_upper", splitBetween(1, 0, 4, lit, nil), nil, AutoRandomRefused, LayoutReasonSplitBoundEmpty},
		// NULL bounds on the table path refuse — upstream
		// convertValueListToData maps a converted NULL to ErrBadNull
		// for BETWEEN and BY alike. The earlier R8-era claim that the
		// refusal was a false direction is withdrawn: the ConvertTo→0
		// path is real, but the post-conversion IsNull gate fires
		// first (v8.5.0 planbuilder.go).
		{"null_table_path", splitBetween(1, 1, 4, []SplitBoundValue{boundClass(SplitValueNull)}, lit), nil, AutoRandomRefused, LayoutReasonSplitNullValue},
		{"null_upper_table_path", splitBetween(1, 1, 4, lit, []SplitBoundValue{boundClass(SplitValueNull)}), nil, AutoRandomRefused, LayoutReasonSplitNullValue},
		{"expr_undecidable", splitBetween(1, 1, 4, []SplitBoundValue{boundClass(SplitValueExpr)}, lit), nil, AutoRandomUndecidable, LayoutReasonSplitValueExpr},
		{"nonnumeric_int_member", splitBetween(1, 1, 4, []SplitBoundValue{boundClass(SplitValueNonNumeric)}, lit), nil, AutoRandomRefused, LayoutReasonSplitNonNumeric},
		{"nonnumeric_type_unproven", splitBetween(1, 1, 4, []SplitBoundValue{boundClass(SplitValueNonNumeric)}, lit), func(f *SplitCompactFacts) { f.HandleTypesKnown = false }, AutoRandomUndecidable, LayoutReasonSplitMemberUnproven},
		{"regions_below_one", splitBetweenInt(0, 10000, 0), nil, AutoRandomRefused, LayoutReasonSplitNumBelowOne},
		{"regions_over_limit", splitBetweenInt(0, 10000, 1001), nil, AutoRandomRefused, LayoutReasonSplitNumOverLimit},
		{"regions_limit_unproven", splitBetweenInt(0, 10000, 4), func(f *SplitCompactFacts) { f.MaxRegionsKnown = false }, AutoRandomUndecidable, LayoutReasonSplitNumLimitUnproven},
		// Partition names on a non-partitioned target are ignored by the
		// upstream executor — never refused.
		{"names_on_nonpartitioned_ignored", func() *SplitRegionDecl {
			d := splitBetweenInt(0, 10000, 4)
			d.PartitionNames = []string{"p0"}
			return d
		}(), nil, AutoRandomOK, ""},
		{"names_partitioned_missing", func() *SplitRegionDecl {
			d := splitBetweenInt(0, 10000, 4)
			d.PartitionNames = []string{"p9"}
			return d
		}(), func(f *SplitCompactFacts) {
			f.Partitioned = true
			f.MembersKnown = true
			f.Members = map[string]bool{"p0": true}
		}, AutoRandomRefused, LayoutReasonSplitPartMissing},
		{"names_partitioned_unproven_members", func() *SplitRegionDecl {
			d := splitBetweenInt(0, 10000, 4)
			d.PartitionNames = []string{"p0"}
			return d
		}(), func(f *SplitCompactFacts) { f.Partitioned = true }, AutoRandomUndecidable, LayoutReasonMembersUnproven},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			facts := baseSplitFacts()
			if tc.mutate != nil {
				tc.mutate(&facts)
			}
			verdict, reason := SplitRegionOutcome(DialectTiDB, tc.decl, facts)
			if verdict != tc.verdict || reason != tc.reason {
				t.Fatalf("got (%v,%q) want (%v,%q)", verdict, reason, tc.verdict, tc.reason)
			}
		})
	}
}

// TestT23R9SplitBoundCompare pins the int-handle BETWEEN arm added in
// R9: upper<=lower refuses, a step under the fixed 1000 refuses, the
// step boundary 1000 passes, and literal domains outside the column's
// stay honestly undecidable rather than guessed (issue #102).
func TestT23R9SplitBoundCompare(t *testing.T) {
	cases := []struct {
		name    string
		decl    *SplitRegionDecl
		mutate  func(*SplitCompactFacts)
		verdict AutoRandomVerdict
		reason  string
	}{
		{"range_ok_step2500", splitBetweenInt(0, 10000, 4), nil, AutoRandomOK, ""},
		{"step_25_refused", splitBetweenInt(0, 100, 4), nil, AutoRandomRefused, LayoutReasonSplitStepTooSmall},
		{"bounds_equal_refused", splitBetweenInt(100, 100, 4), nil, AutoRandomRefused, LayoutReasonSplitBoundNotIncreasing},
		{"bounds_reversed_refused", splitBetweenInt(100, 0, 4), nil, AutoRandomRefused, LayoutReasonSplitBoundNotIncreasing},
		{"step_boundary_1000_ok", splitBetweenInt(0, 4000, 4), nil, AutoRandomOK, ""},
		{"step_999_refused", splitBetweenInt(0, 3999, 4), nil, AutoRandomRefused, LayoutReasonSplitStepTooSmall},
		{"negative_range_ok", splitBetweenInt(-10000, 10000, 4), nil, AutoRandomOK, ""},
		{"negative_step_small", splitBetweenInt(-100, 100, 4), nil, AutoRandomRefused, LayoutReasonSplitStepTooSmall},
		{"signed_min_max_range_refused", splitBetweenInt(-9223372036854775808, 9223372036854775807, 1),
			nil, AutoRandomRefused, LayoutReasonSplitStepTooSmall},
		// Unsigned arm: the same bounds compare in the uint64 domain.
		{"unsigned_wide_ok", splitBetween(1, 1, 4,
			[]SplitBoundValue{boundInt(0)}, []SplitBoundValue{boundUint(18446744073709551615)}),
			func(f *SplitCompactFacts) {
				f.HandlePKIsHandle = true
				f.HandleUnsigned = []bool{true}
			}, AutoRandomOK, ""},
		// Upstream casts the uint64 step back to int64 — a step beyond
		// MaxInt64 wraps negative and refuses identically.
		{"unsigned_step_wrap_refused", splitBetween(1, 1, 1,
			[]SplitBoundValue{boundInt(0)}, []SplitBoundValue{boundUint(18446744073709551615)}),
			func(f *SplitCompactFacts) {
				f.HandlePKIsHandle = true
				f.HandleUnsigned = []bool{true}
			}, AutoRandomRefused, LayoutReasonSplitStepTooSmall},
		{"unsigned_equal_refused", splitBetween(1, 1, 4,
			[]SplitBoundValue{boundUint(100)}, []SplitBoundValue{boundUint(100)}),
			func(f *SplitCompactFacts) {
				f.HandlePKIsHandle = true
				f.HandleUnsigned = []bool{true}
			}, AutoRandomRefused, LayoutReasonSplitBoundNotIncreasing},
		// Domain mismatches: the conversion outcome depends on the
		// session sql_mode — honest undecidable, never guessed.
		{"negative_on_unsigned_unproven", splitBetween(1, 1, 4,
			[]SplitBoundValue{boundInt(-1)}, []SplitBoundValue{boundUint(10000)}),
			func(f *SplitCompactFacts) {
				f.HandlePKIsHandle = true
				f.HandleUnsigned = []bool{true}
			}, AutoRandomUndecidable, LayoutReasonSplitBoundConvertUnproven},
		{"overflow_on_signed_unproven", splitBetween(1, 1, 4,
			[]SplitBoundValue{boundInt(0)}, []SplitBoundValue{boundUint(18446744073709551615)}),
			nil, AutoRandomUndecidable, LayoutReasonSplitBoundConvertUnproven},
		// A literal without an integer payload (float/decimal/numeric
		// non-integer string) cannot fold — undecidable, not guessed.
		{"non_integer_literal_unproven", splitBetween(1, 1, 4,
			[]SplitBoundValue{boundLit()}, []SplitBoundValue{boundInt(10000)}),
			nil, AutoRandomUndecidable, LayoutReasonSplitBoundConvertUnproven},
		// Signedness unproven keeps the arm-domain choice undecidable.
		{"unsigned_flag_unproven", splitBetweenInt(0, 10000, 4),
			func(f *SplitCompactFacts) { f.HandleUnsignedKnown = false },
			AutoRandomUndecidable, LayoutReasonSplitMemberUnproven},
		// Member types unproven precede any arm selection.
		{"member_types_unproven", splitBetweenInt(0, 10000, 4),
			func(f *SplitCompactFacts) { f.HandleTypesKnown = false },
			AutoRandomUndecidable, LayoutReasonSplitMemberUnproven},
		// Multi-member and non-int handles compare on encoded handles —
		// a disclosed capability boundary, never a fabricated complete.
		{"common_handle_compare_gap", splitBetween(2, 2, 4,
			[]SplitBoundValue{boundInt(0), boundLit()}, []SplitBoundValue{boundInt(100), boundLit()}),
			func(f *SplitCompactFacts) {
				f.HandleInt = []bool{true, false}
			}, AutoRandomUndecidable, LayoutReasonSplitBoundCompareUnsupported},
		// A single-int common handle still compares encoded handles
		// upstream — the int arm is only PKIsHandle / rowid.
		{"single_int_common_handle_gap", splitBetweenInt(0, 10000, 4),
			func(f *SplitCompactFacts) {
				f.HandleCommonHandle = true
			}, AutoRandomUndecidable, LayoutReasonSplitBoundCompareUnsupported},
		{"non_int_handle_compare_gap", splitBetweenInt(0, 10000, 4),
			func(f *SplitCompactFacts) {
				f.HandleInt = []bool{false}
			}, AutoRandomUndecidable, LayoutReasonSplitBoundCompareUnsupported},
		// Existing refusals still precede the new arm: an invalid
		// REGIONS count cannot be swallowed by the bound checks.
		{"num_zero_still_refused", splitBetweenInt(0, 10000, 0), nil, AutoRandomRefused, LayoutReasonSplitNumBelowOne},
		{"null_still_refused", splitBetween(1, 1, 4,
			[]SplitBoundValue{boundClass(SplitValueNull)}, []SplitBoundValue{boundInt(10000)}),
			nil, AutoRandomRefused, LayoutReasonSplitNullValue},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			facts := baseSplitFacts()
			if tc.mutate != nil {
				tc.mutate(&facts)
			}
			verdict, reason := SplitRegionOutcome(DialectTiDB, tc.decl, facts)
			if verdict != tc.verdict || reason != tc.reason {
				t.Fatalf("got (%v,%q) want (%v,%q)", verdict, reason, tc.verdict, tc.reason)
			}
		})
	}
}

// TestT23R9SplitByKeepsNoRelation proves the BY form never gains the
// BETWEEN range/step checks: reversed, equal and unordered tuples all
// pass the modeled premises, while NULL still refuses via the shared
// table-path conversion gate.
func TestT23R9SplitByKeepsNoRelation(t *testing.T) {
	ints := func(vs ...int64) []SplitBoundValue {
		out := make([]SplitBoundValue, len(vs))
		for i, v := range vs {
			out[i] = boundInt(v)
		}
		return out
	}
	cases := []struct {
		name    string
		decl    *SplitRegionDecl
		verdict AutoRandomVerdict
		reason  string
	}{
		{"by_descending_ok", splitBy([][]SplitBoundValue{ints(100), ints(50), ints(0)}), AutoRandomOK, ""},
		{"by_equal_ok", splitBy([][]SplitBoundValue{ints(7), ints(7)}), AutoRandomOK, ""},
		{"by_null_refused", splitBy([][]SplitBoundValue{{boundClass(SplitValueNull)}, ints(20)}), AutoRandomRefused, LayoutReasonSplitNullValue},
		{"by_expr_undecidable", splitBy([][]SplitBoundValue{{boundClass(SplitValueExpr)}, ints(20)}), AutoRandomUndecidable, LayoutReasonSplitValueExpr},
	}
	facts := baseSplitFacts()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			verdict, reason := SplitRegionOutcome(DialectTiDB, tc.decl, facts)
			if verdict != tc.verdict || reason != tc.reason {
				t.Fatalf("got (%v,%q) want (%v,%q)", verdict, reason, tc.verdict, tc.reason)
			}
		})
	}
}

func TestT23R8SplitIndexOutcome(t *testing.T) {
	lit := []SplitBoundValue{boundLit()}
	intBound := []SplitBoundValue{boundInt(0)}
	intBoundUp := []SplitBoundValue{boundInt(10000)}
	indexFacts := func(mutate func(*SplitCompactFacts)) SplitCompactFacts {
		facts := baseSplitFacts()
		facts.IndexMembersKnown = true
		facts.IndexMembers = map[string][]bool{"ix": {true, true}}
		facts.IndexMemberTypesKnown = map[string]bool{"ix": true}
		if mutate != nil {
			mutate(&facts)
		}
		return facts
	}
	decl := func(name string, bound *SplitRegionDecl) *SplitRegionDecl {
		bound.IndexName = name
		return bound
	}
	cases := []struct {
		name    string
		decl    *SplitRegionDecl
		facts   SplitCompactFacts
		verdict AutoRandomVerdict
		reason  string
	}{
		{"index_by_partial_ok", decl("ix", splitBy([][]SplitBoundValue{lit})), indexFacts(nil), AutoRandomOK, ""},
		{"index_missing_refused", decl("ghost", splitBetween(1, 1, 4, intBound, intBoundUp)), indexFacts(nil), AutoRandomRefused, LayoutReasonSplitIndexMissing},
		{"index_members_unproven", decl("ix", splitBetween(1, 1, 4, intBound, intBoundUp)), indexFacts(func(f *SplitCompactFacts) { f.IndexMembersKnown = false }), AutoRandomUndecidable, LayoutReasonSplitIndexUnproven},
		{"index_over_arity", decl("ix", splitBetween(3, 3, 4, []SplitBoundValue{lit[0], lit[0], lit[0]}, []SplitBoundValue{lit[0], lit[0], lit[0]})), indexFacts(nil), AutoRandomRefused, LayoutReasonSplitArityIndex},
		{"index_null_undecidable", decl("ix", splitBetween(1, 1, 4, []SplitBoundValue{boundClass(SplitValueNull)}, lit)), indexFacts(nil), AutoRandomUndecidable, LayoutReasonSplitValueExpr},
		{"index_between_compare_gap", decl("ix", splitBetween(1, 1, 4, intBound, intBoundUp)), indexFacts(nil), AutoRandomUndecidable, LayoutReasonSplitBoundCompareUnsupported},
		{"primary_clustered_refused", decl("primary", splitBetween(1, 1, 4, intBound, intBoundUp)), indexFacts(func(f *SplitCompactFacts) {
			f.Clustered = true
			f.PrimaryExist = true
			f.IndexMembers["primary"] = []bool{true}
			f.IndexMemberTypesKnown["primary"] = true
		}), AutoRandomRefused, LayoutReasonSplitPrimaryClustered},
		{"primary_unproven", decl("primary", splitBetween(1, 1, 4, intBound, intBoundUp)), indexFacts(func(f *SplitCompactFacts) { f.ClusteredKnown = false }), AutoRandomUndecidable, LayoutReasonSplitIndexUnproven},
		{"primary_absent_refused", decl("primary", splitBetween(1, 1, 4, intBound, intBoundUp)), indexFacts(nil), AutoRandomRefused, LayoutReasonSplitIndexMissing},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			verdict, reason := SplitRegionOutcome(DialectTiDB, tc.decl, tc.facts)
			if verdict != tc.verdict || reason != tc.reason {
				t.Fatalf("got (%v,%q) want (%v,%q)", verdict, reason, tc.verdict, tc.reason)
			}
		})
	}
}

func TestT23R8CompactOutcome(t *testing.T) {
	decl := func(kind string, names []string) *CompactTableDecl {
		return &CompactTableDecl{ReplicaKind: kind, PartitionNames: names}
	}
	cases := []struct {
		name    string
		decl    *CompactTableDecl
		mutate  func(*SplitCompactFacts)
		verdict AutoRandomVerdict
		reason  string
	}{
		{"tiflash_ok", decl("tiflash", nil), nil, AutoRandomOK, ""},
		{"all_ok", decl("all", nil), nil, AutoRandomOK, ""},
		{"bare_ok", decl("", nil), nil, AutoRandomOK, ""},
		{"tikv_refused", decl("tikv", nil), nil, AutoRandomRefused, LayoutReasonCompactReplicaKind},
		{"unknown_kind_refused", decl("rocksdb", nil), nil, AutoRandomRefused, LayoutReasonCompactReplicaKind},
		{"store_unproven", decl("tiflash", nil), func(f *SplitCompactFacts) { f.TiKVStoreKnown = false }, AutoRandomUndecidable, LayoutReasonCompactStoreUnproven},
		{"store_not_tikv", decl("tiflash", nil), func(f *SplitCompactFacts) { f.TiKVStore = false }, AutoRandomRefused, LayoutReasonCompactNotTiKV},
		{"names_on_nonpartitioned", decl("tiflash", []string{"p0"}), nil, AutoRandomRefused, LayoutReasonCompactNonPartitioned},
		{"names_partitioned_missing", decl("tiflash", []string{"p9"}), func(f *SplitCompactFacts) {
			f.Partitioned = true
			f.MembersKnown = true
			f.Members = map[string]bool{"p0": true}
		}, AutoRandomRefused, LayoutReasonCompactPartMissing},
		{"names_partitioned_ok", decl("tiflash", []string{"p0"}), func(f *SplitCompactFacts) {
			f.Partitioned = true
			f.MembersKnown = true
			f.Members = map[string]bool{"p0": true}
		}, AutoRandomOK, ""},
		{"no_tiflash_replica_noop", decl("tiflash", nil), func(f *SplitCompactFacts) { f.ReplicaZero = true }, AutoRandomOK, LayoutReasonCompactNoReplicaWarn},
		{"replica_unproven_no_warn", decl("tiflash", nil), func(f *SplitCompactFacts) { f.ReplicaKnown = false }, AutoRandomOK, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			facts := baseSplitFacts()
			if tc.mutate != nil {
				tc.mutate(&facts)
			}
			verdict, reason := CompactTableOutcome(DialectTiDB, tc.decl, facts)
			if verdict != tc.verdict || reason != tc.reason {
				t.Fatalf("got (%v,%q) want (%v,%q)", verdict, reason, tc.verdict, tc.reason)
			}
		})
	}
}

func TestT23R8DialectAndNilDecl(t *testing.T) {
	facts := baseSplitFacts()
	if verdict, _ := SplitRegionOutcome(DialectMySQL, splitBetweenInt(0, 10000, 4), facts); verdict != AutoRandomRefused {
		t.Fatal("split on mysql must refuse")
	}
	if verdict, _ := CompactTableOutcome(DialectMySQL, &CompactTableDecl{ReplicaKind: "tiflash"}, facts); verdict != AutoRandomRefused {
		t.Fatal("compact on mysql must refuse")
	}
	if verdict, reason := SplitRegionOutcome(DialectTiDB, nil, facts); verdict != AutoRandomUndecidable || reason == "" {
		t.Fatalf("nil split decl must stay undecidable, got (%v,%q)", verdict, reason)
	}
	if verdict, reason := CompactTableOutcome(DialectTiDB, nil, facts); verdict != AutoRandomUndecidable || reason == "" {
		t.Fatalf("nil compact decl must stay undecidable, got (%v,%q)", verdict, reason)
	}
}

func TestT23R8FactsForSnapshot(t *testing.T) {
	t.Run("nil snapshot leaves every fact unproven", func(t *testing.T) {
		facts := SplitCompactFactsForSnapshot(nil, nil)
		if facts.HandleKnown || facts.IndexMembersKnown || facts.PartitionedKnown ||
			facts.MaxRegionsKnown || facts.TiKVStoreKnown || facts.ReplicaKnown {
			t.Fatalf("nil snapshot must not vouch facts, got %+v", facts)
		}
	})
	t.Run("pk_is_handle splits on a single int member", func(t *testing.T) {
		facts := SplitCompactFactsForSnapshot(&TableSnapshot{PrimaryKeyIsHandle: true}, nil)
		if !facts.HandleKnown || len(facts.HandleInt) != 1 || !facts.HandleInt[0] || !facts.HandleTypesKnown {
			t.Fatalf("pk-is-handle handle fact wrong: %+v", facts)
		}
	})
	t.Run("keyless table splits on the rowid handle", func(t *testing.T) {
		facts := SplitCompactFactsForSnapshot(&TableSnapshot{}, nil)
		if !facts.HandleKnown || len(facts.HandleInt) != 1 || !facts.HandleInt[0] {
			t.Fatalf("rowid handle fact wrong: %+v", facts)
		}
		if !facts.HandleUnsignedKnown || len(facts.HandleUnsigned) != 1 || facts.HandleUnsigned[0] || facts.HandlePKIsHandle {
			t.Fatalf("rowid arm must be the signed domain: %+v", facts)
		}
	})
	t.Run("pk_is_handle unsigned flag selects the arm domain", func(t *testing.T) {
		facts := SplitCompactFactsForSnapshot(&TableSnapshot{
			PrimaryKeyIsHandle: true,
			PrimaryKey:         &Index{Columns: []string{"id"}},
			Columns:            []Column{{Name: "id", Type: "bigint unsigned"}},
		}, nil)
		if !facts.HandlePKIsHandle || !facts.HandleUnsignedKnown || !facts.HandleUnsigned[0] {
			t.Fatalf("unsigned pk-is-handle fact wrong: %+v", facts)
		}
	})
	t.Run("pk_is_handle unobserved column keeps flag unproven", func(t *testing.T) {
		facts := SplitCompactFactsForSnapshot(&TableSnapshot{PrimaryKeyIsHandle: true}, nil)
		if facts.HandleUnsignedKnown {
			t.Fatalf("unobserved pk column type must keep the flag unproven: %+v", facts)
		}
	})
	t.Run("common handle carries pk member types", func(t *testing.T) {
		snap := &TableSnapshot{
			PrimaryKeyClustered: true,
			PrimaryKey:          &Index{Columns: []string{"a", "b"}},
			Columns:             []Column{{Name: "a", Type: "bigint"}, {Name: "b", Type: "varchar(10)"}},
		}
		facts := SplitCompactFactsForSnapshot(snap, nil)
		if !facts.HandleKnown || len(facts.HandleInt) != 2 || !facts.HandleInt[0] || facts.HandleInt[1] {
			t.Fatalf("common-handle member types wrong: %+v", facts)
		}
		if facts.HandlePKIsHandle || !facts.HandleCommonHandle {
			t.Fatalf("common handle must mark the encoded-compare arm: %+v", facts)
		}
	})
	t.Run("untyped member keeps conversion undecidable", func(t *testing.T) {
		snap := &TableSnapshot{
			PrimaryKeyClustered: true,
			PrimaryKey:          &Index{Columns: []string{"a"}},
			Columns:             []Column{{Name: "a"}},
		}
		facts := SplitCompactFactsForSnapshot(snap, nil)
		if !facts.HandleKnown || facts.HandleTypesKnown {
			t.Fatalf("unobserved member type must stay unproven: %+v", facts)
		}
	})
	t.Run("unknown pk handle stays unproven", func(t *testing.T) {
		facts := SplitCompactFactsForSnapshot(&TableSnapshot{PrimaryKeyHandleUnknown: true}, nil)
		if facts.HandleKnown {
			t.Fatalf("unknown handle must not publish a shape: %+v", facts)
		}
	})
	t.Run("index members carry per-index type knowledge", func(t *testing.T) {
		snap := &TableSnapshot{
			Indexes: []Index{
				{Name: "ix", Columns: []string{"a"}},
				{Name: "iy", Columns: []string{"missing"}},
			},
			Columns: []Column{{Name: "a", Type: "int"}},
		}
		facts := SplitCompactFactsForSnapshot(snap, nil)
		if !facts.IndexMembersKnown || len(facts.IndexMembers["ix"]) != 1 || !facts.IndexMembers["ix"][0] {
			t.Fatalf("index member fact wrong: %+v", facts)
		}
		if facts.IndexMemberTypesKnown["iy"] {
			t.Fatalf("unresolved member column must keep types unproven: %+v", facts)
		}
	})
	t.Run("tiflash replica options project the no-op arm", func(t *testing.T) {
		for value, wantZero := range map[string]bool{"0": true, "": true, "2": false} {
			facts := SplitCompactFactsForSnapshot(&TableSnapshot{Options: map[string]string{"tiflash_replica_count": value}}, nil)
			if !facts.ReplicaKnown || facts.ReplicaZero != wantZero {
				t.Fatalf("replica %q: got %+v", value, facts)
			}
		}
	})
	t.Run("instance facts publish only when observed", func(t *testing.T) {
		facts := SplitCompactFactsForSnapshot(&TableSnapshot{}, &InstanceFacts{
			Version:                    "8.0.11-TiDB-v8.5.0",
			TiDBSplitRegionMaxNum:      500,
			TiDBSplitRegionMaxNumKnown: true,
		})
		if !facts.TiKVStoreKnown || !facts.TiKVStore || !facts.MaxRegionsKnown || facts.MaxRegions != 500 {
			t.Fatalf("instance fact projection wrong: %+v", facts)
		}
		facts = SplitCompactFactsForSnapshot(&TableSnapshot{}, &InstanceFacts{Version: "8.0.40"})
		if facts.TiKVStoreKnown {
			t.Fatalf("non-tidb version must not prove tikv storage: %+v", facts)
		}
	})
}
