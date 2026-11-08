package spec

import "testing"

// split_compact_r8_test.go pins the T23-R8 SPLIT/COMPACT outcome matrix
// against the upstream v8.5.0 anchors cited in split_compact_r8.go:
// temporary/identity arms, handle vs index member arity, NULL and
// non-numeric conversion refusals, the REGIONS bound, exec-time
// partition-member resolution, the COMPACT replica-kind/tikv-store
// order, and the no-TiFlash warning no-op (issue #102).

func baseSplitFacts() SplitCompactFacts {
	return SplitCompactFacts{
		Partitioned:      false,
		PartitionedKnown: true,
		ClusteredKnown:   true,
		PrimaryKnown:     true,
		PrimaryExist:     false,
		HandleKnown:      true,
		HandleTypesKnown: true,
		HandleInt:        []bool{true},
		MaxRegionsKnown:  true,
		MaxRegions:       1000,
		TiKVStore:        true,
		TiKVStoreKnown:   true,
		ReplicaKnown:     true,
	}
}

func splitBetween(lower, upper int, num int64, lowerClass, upperClass []SplitValueClass) *SplitRegionDecl {
	return &SplitRegionDecl{
		Bound: SplitBound{
			LowerArity:   lower,
			UpperArity:   upper,
			LowerClasses: lowerClass,
			UpperClasses: upperClass,
		},
		Num: &num,
	}
}

func splitBy(classes [][]SplitValueClass) *SplitRegionDecl {
	decl := &SplitRegionDecl{}
	for _, tuple := range classes {
		decl.Bound.Arities = append(decl.Bound.Arities, len(tuple))
		decl.Bound.Classes = append(decl.Bound.Classes, tuple)
	}
	return decl
}

func TestT23R8SplitTableOutcome(t *testing.T) {
	lit := []SplitValueClass{SplitValueLiteral}
	cases := []struct {
		name    string
		decl    *SplitRegionDecl
		mutate  func(*SplitCompactFacts)
		verdict AutoRandomVerdict
		reason  string
	}{
		{"between_ok", splitBetween(1, 1, 4, lit, lit), nil, AutoRandomOK, ""},
		{"by_ok", splitBy([][]SplitValueClass{lit, lit}), nil, AutoRandomOK, ""},
		{"temporary_refused", splitBetween(1, 1, 4, lit, lit), func(f *SplitCompactFacts) { f.Temporary = true }, AutoRandomRefused, LayoutReasonSplitTemporary},
		{"view_object_undecidable", splitBetween(1, 1, 4, lit, lit), func(f *SplitCompactFacts) { f.ViewOrSequence = true }, AutoRandomUndecidable, LayoutReasonSplitNonTableObject},
		{"partition_clause_nonpartitioned", func() *SplitRegionDecl {
			d := splitBetween(1, 1, 4, lit, lit)
			d.HasPartitionClause = true
			return d
		}(), nil, AutoRandomRefused, LayoutReasonSplitNonPartitioned},
		{"partition_clause_unproven", func() *SplitRegionDecl {
			d := splitBetween(1, 1, 4, lit, lit)
			d.HasPartitionClause = true
			return d
		}(), func(f *SplitCompactFacts) { f.PartitionedKnown = false }, AutoRandomUndecidable, LayoutReasonPartitionedUnproven},
		{"handle_unproven", splitBetween(1, 1, 4, lit, lit), func(f *SplitCompactFacts) { f.HandleKnown = false }, AutoRandomUndecidable, LayoutReasonSplitHandleUnproven},
		{"table_arity_mismatch", splitBetween(2, 2, 4, []SplitValueClass{lit[0], lit[0]}, []SplitValueClass{lit[0], lit[0]}), nil, AutoRandomRefused, LayoutReasonSplitArityTable},
		{"bound_empty_lower", splitBetween(0, 1, 4, nil, lit), nil, AutoRandomRefused, LayoutReasonSplitBoundEmpty},
		{"bound_empty_upper", splitBetween(1, 0, 4, lit, nil), nil, AutoRandomRefused, LayoutReasonSplitBoundEmpty},
		{"null_table_path", splitBetween(1, 1, 4, []SplitValueClass{SplitValueNull}, lit), nil, AutoRandomRefused, LayoutReasonSplitNullValue},
		{"expr_undecidable", splitBetween(1, 1, 4, []SplitValueClass{SplitValueExpr}, lit), nil, AutoRandomUndecidable, LayoutReasonSplitValueExpr},
		{"nonnumeric_int_member", splitBetween(1, 1, 4, []SplitValueClass{SplitValueNonNumeric}, lit), nil, AutoRandomRefused, LayoutReasonSplitNonNumeric},
		{"nonnumeric_type_unproven", splitBetween(1, 1, 4, []SplitValueClass{SplitValueNonNumeric}, lit), func(f *SplitCompactFacts) { f.HandleTypesKnown = false }, AutoRandomUndecidable, LayoutReasonSplitMemberUnproven},
		{"regions_below_one", splitBetween(1, 1, 0, lit, lit), nil, AutoRandomRefused, LayoutReasonSplitNumBelowOne},
		{"regions_over_limit", splitBetween(1, 1, 1001, lit, lit), nil, AutoRandomRefused, LayoutReasonSplitNumOverLimit},
		{"regions_limit_unproven", splitBetween(1, 1, 4, lit, lit), func(f *SplitCompactFacts) { f.MaxRegionsKnown = false }, AutoRandomUndecidable, LayoutReasonSplitNumLimitUnproven},
		// Partition names on a non-partitioned target are ignored by the
		// upstream executor — never refused.
		{"names_on_nonpartitioned_ignored", func() *SplitRegionDecl {
			d := splitBetween(1, 1, 4, lit, lit)
			d.PartitionNames = []string{"p0"}
			return d
		}(), nil, AutoRandomOK, ""},
		{"names_partitioned_missing", func() *SplitRegionDecl {
			d := splitBetween(1, 1, 4, lit, lit)
			d.PartitionNames = []string{"p9"}
			return d
		}(), func(f *SplitCompactFacts) {
			f.Partitioned = true
			f.MembersKnown = true
			f.Members = map[string]bool{"p0": true}
		}, AutoRandomRefused, LayoutReasonSplitPartMissing},
		{"names_partitioned_unproven_members", func() *SplitRegionDecl {
			d := splitBetween(1, 1, 4, lit, lit)
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

func TestT23R8SplitIndexOutcome(t *testing.T) {
	lit := []SplitValueClass{SplitValueLiteral}
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
		{"index_by_partial_ok", decl("ix", splitBy([][]SplitValueClass{lit})), indexFacts(nil), AutoRandomOK, ""},
		{"index_missing_refused", decl("ghost", splitBetween(1, 1, 4, lit, lit)), indexFacts(nil), AutoRandomRefused, LayoutReasonSplitIndexMissing},
		{"index_members_unproven", decl("ix", splitBetween(1, 1, 4, lit, lit)), indexFacts(func(f *SplitCompactFacts) { f.IndexMembersKnown = false }), AutoRandomUndecidable, LayoutReasonSplitIndexUnproven},
		{"index_over_arity", decl("ix", splitBetween(3, 3, 4, []SplitValueClass{lit[0], lit[0], lit[0]}, []SplitValueClass{lit[0], lit[0], lit[0]})), indexFacts(nil), AutoRandomRefused, LayoutReasonSplitArityIndex},
		{"index_null_undecidable", decl("ix", splitBetween(1, 1, 4, []SplitValueClass{SplitValueNull}, lit)), indexFacts(nil), AutoRandomUndecidable, LayoutReasonSplitValueExpr},
		{"primary_clustered_refused", decl("primary", splitBetween(1, 1, 4, lit, lit)), indexFacts(func(f *SplitCompactFacts) {
			f.Clustered = true
			f.PrimaryExist = true
			f.IndexMembers["primary"] = []bool{true}
			f.IndexMemberTypesKnown["primary"] = true
		}), AutoRandomRefused, LayoutReasonSplitPrimaryClustered},
		{"primary_unproven", decl("primary", splitBetween(1, 1, 4, lit, lit)), indexFacts(func(f *SplitCompactFacts) { f.ClusteredKnown = false }), AutoRandomUndecidable, LayoutReasonSplitIndexUnproven},
		{"primary_absent_refused", decl("primary", splitBetween(1, 1, 4, lit, lit)), indexFacts(nil), AutoRandomRefused, LayoutReasonSplitIndexMissing},
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
	if verdict, _ := SplitRegionOutcome(DialectMySQL, splitBetween(1, 1, 4, nil, nil), facts); verdict != AutoRandomRefused {
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
