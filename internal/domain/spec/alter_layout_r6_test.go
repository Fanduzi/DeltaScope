package spec

import "testing"

// alter_layout_r6_test.go pins the T23-R6 outcome matrix against the
// upstream v8.5.0 executor semantics: CACHE/NOCACHE ordering, the
// cached-table guard, ATTRIBUTES spec/member adjudication, and the
// TiFlash replica declaration arms (issue #102).

func baseR6Facts() AlterLayoutR6Facts {
	false_ := false
	return AlterLayoutR6Facts{
		Partitioned:      false_,
		PartitionedKnown: true,
		CacheStatus:      "disable",
		CacheStatusKnown: true,
		CharsetKnown:     true,
		ReplicaKnown:     true,
	}
}

func TestT23R6CacheOutcome(t *testing.T) {
	for _, tc := range []struct {
		name    string
		action  string
		mutate  func(*AlterLayoutR6Facts)
		verdict AutoRandomVerdict
		reason  string
	}{
		{"cache_on_disable_undecidable_size", "cache", nil, AutoRandomUndecidable, LayoutReasonCacheSizeUnproven},
		{"cache_on_enable_noop", "cache", func(f *AlterLayoutR6Facts) { f.CacheStatus = "enable" }, AutoRandomOK, LayoutReasonCacheAlreadyCached},
		{"cache_sysdb_refused", "cache", func(f *AlterLayoutR6Facts) { f.SysDB = true }, AutoRandomRefused, LayoutReasonCacheSysTable},
		{"cache_temporary_refused", "cache", func(f *AlterLayoutR6Facts) { f.Temporary = true }, AutoRandomRefused, LayoutReasonCacheTemporary},
		{"cache_partitioned_refused", "cache", func(f *AlterLayoutR6Facts) { f.Partitioned = true }, AutoRandomRefused, LayoutReasonCachePartitioned},
		{"cache_partitioned_unproven", "cache", func(f *AlterLayoutR6Facts) { f.PartitionedKnown = false }, AutoRandomUndecidable, LayoutReasonPartitionedUnproven},
		{"nocache_on_enable_applied", "nocache", func(f *AlterLayoutR6Facts) { f.CacheStatus = "enable" }, AutoRandomOK, ""},
		{"nocache_on_disable_noop", "nocache", nil, AutoRandomOK, LayoutReasonNoCacheNotCached},
		// R7: every legal old status converges to disable — unobserved
		// status still publishes the converged post-state, no status gap.
		{"nocache_status_unknown_converges", "nocache", func(f *AlterLayoutR6Facts) { f.CacheStatusKnown = false }, AutoRandomOK, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			facts := baseR6Facts()
			if tc.mutate != nil {
				tc.mutate(&facts)
			}
			verdict, reason := AlterTableCacheOutcome(tc.action, facts)
			if verdict != tc.verdict || reason != tc.reason {
				t.Fatalf("got (%v,%q) want (%v,%q)", verdict, reason, tc.verdict, tc.reason)
			}
		})
	}
}

func TestT23R6CachedTableGuard(t *testing.T) {
	facts := baseR6Facts()
	facts.CacheStatus = "enable"
	if !CachedTableGuardRefused("modify_column", 1, facts) {
		t.Fatal("cached table must refuse a structural alter")
	}
	if !CachedTableGuardRefused("cache", 2, facts) {
		t.Fatal("cached table must refuse multi-spec forms")
	}
	if CachedTableGuardRefused("cache", 1, facts) || CachedTableGuardRefused("nocache", 1, facts) {
		t.Fatal("cache/nocache stay allowed on a cached table")
	}
	facts.CacheStatusKnown = false
	if CachedTableGuardRefused("modify_column", 1, facts) {
		t.Fatal("unproven cache status cannot refuse")
	}
}

func TestT23R6AttributesOutcome(t *testing.T) {
	decl := &AlterAttributesDecl{Value: "merge_option=allow,x=1"}
	for _, tc := range []struct {
		name    string
		decl    *AlterAttributesDecl
		parts   []string
		mutate  func(*AlterLayoutR6Facts)
		verdict AutoRandomVerdict
		reason  string
	}{
		{"table_level_valid", decl, nil, nil, AutoRandomOK, ""},
		{"table_level_default", &AlterAttributesDecl{Default: true}, nil, nil, AutoRandomOK, ""},
		{"table_level_invalid", &AlterAttributesDecl{Value: "notakeyvalue"}, nil, nil, AutoRandomRefused, LayoutReasonAttributesInvalid},
		{"partition_member_ok", decl, []string{"p0"}, func(f *AlterLayoutR6Facts) {
			f.Partitioned, f.MembersKnown = true, true
			f.Members = map[string]bool{"p0": true}
		}, AutoRandomOK, ""},
		{"partition_member_missing", decl, []string{"p9"}, func(f *AlterLayoutR6Facts) {
			f.Partitioned, f.MembersKnown = true, true
			f.Members = map[string]bool{"p0": true}
		}, AutoRandomRefused, LayoutReasonAttributesPartMiss},
		{"partition_on_nonpartitioned", decl, []string{"p0"}, nil, AutoRandomRefused, LayoutReasonAttributesNonPart},
		{"partition_unproven", decl, []string{"p0"}, func(f *AlterLayoutR6Facts) { f.PartitionedKnown = false }, AutoRandomUndecidable, LayoutReasonPartitionedUnproven},
		{"members_unproven", decl, []string{"p0"}, func(f *AlterLayoutR6Facts) { f.Partitioned = true }, AutoRandomUndecidable, LayoutReasonMembersUnproven},
	} {
		t.Run(tc.name, func(t *testing.T) {
			facts := baseR6Facts()
			if tc.mutate != nil {
				tc.mutate(&facts)
			}
			verdict, reason := AlterAttributesOutcome(tc.decl, tc.parts, facts)
			if verdict != tc.verdict || reason != tc.reason {
				t.Fatalf("got (%v,%q) want (%v,%q)", verdict, reason, tc.verdict, tc.reason)
			}
		})
	}
}

func TestT23R6AttributesSpecParser(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  bool
	}{
		{"merge_option=allow", true},
		{"merge_option=allow,x=1", true},
		// R7: real YAML scan — `key='a,b'` splits mid-quote into `key='a`
		// + `b'`; the unpaired `b'` refuses upstream the same way.
		{"merge_option=deny, key='a,b'", false},
		{`key="v,with,commas"`, false},
		{"nokeyvalue", false},
		{"=novaluekey", false},
		{"a=1,,b=2", false},
		// YAML's quote recovery still yields a `a='unclosed` key=value
		// item — upstream accepts it through the same engine.
		{"a='unclosed", true},
		{"[a=1]", false},
		{"key=val ue", true},
		// R7 B2 permanent refusals — upstream NewLabel/Add semantics.
		{"a=", false},      // empty value refused
		{"a=b=c", false},   // Split("=") must yield exactly 2 parts
		{"a=1,a=2", false}, // same key conflicting values refused
		{"a=1,a=1", true},  // same key same value dedups (no-op)
		{"a=1,", true},     // trailing comma tolerated by YAML flow
		{"", true},         // empty spec parses to the empty label list
		{"a={x:1}", false}, // non-string YAML item refused
		{`"a=1","b=2"`, true},
	} {
		if got := ParseAttributesSpecValue(tc.value); got != tc.want {
			t.Fatalf("ParseAttributesSpecValue(%q)=%v want %v", tc.value, got, tc.want)
		}
	}
}

func TestT23R6TiFlashOutcome(t *testing.T) {
	for _, tc := range []struct {
		name    string
		decl    *TiFlashReplicaDecl
		mutate  func(*AlterLayoutR6Facts)
		verdict AutoRandomVerdict
		reason  string
	}{
		{"count_positive_store_premise", &TiFlashReplicaDecl{Count: 2}, nil, AutoRandomUndecidable, LayoutReasonTiFlashStoreUnproven},
		{"count_zero_applies", &TiFlashReplicaDecl{Count: 0}, nil, AutoRandomOK, ""},
		{"same_def_noop", &TiFlashReplicaDecl{Count: 2}, func(f *AlterLayoutR6Facts) { f.ReplicaCount = 2 }, AutoRandomOK, LayoutReasonTiFlashSameDef},
		{"different_labels_not_noop", &TiFlashReplicaDecl{Count: 2, Labels: []string{"a"}}, func(f *AlterLayoutR6Facts) {
			f.ReplicaCount = 2
			f.ReplicaLabels = []string{"b"}
		}, AutoRandomUndecidable, LayoutReasonTiFlashStoreUnproven},
		{"sysdb_refused", &TiFlashReplicaDecl{Count: 0}, func(f *AlterLayoutR6Facts) { f.SysDB = true }, AutoRandomRefused, LayoutReasonTiFlashSysTable},
		{"temporary_refused", &TiFlashReplicaDecl{Count: 0}, func(f *AlterLayoutR6Facts) { f.Temporary = true }, AutoRandomRefused, LayoutReasonTiFlashTemporary},
		{"view_refused", &TiFlashReplicaDecl{Count: 0}, func(f *AlterLayoutR6Facts) { f.ViewOrSequence = true }, AutoRandomRefused, LayoutReasonTiFlashViewOrSeq},
		{"charset_refused", &TiFlashReplicaDecl{Count: 0}, func(f *AlterLayoutR6Facts) { f.CharsetUnsupported = true }, AutoRandomRefused, LayoutReasonTiFlashCharset},
		{"charset_unproven_first", &TiFlashReplicaDecl{Count: 0}, func(f *AlterLayoutR6Facts) { f.CharsetKnown = false }, AutoRandomUndecidable, LayoutReasonCharsetUnproven},
		{"hypo_session_only", &TiFlashReplicaDecl{Count: 2, Hypo: true}, nil, AutoRandomOK, LayoutReasonTiFlashHypo},
		{"old_replica_unproven", &TiFlashReplicaDecl{Count: 2}, func(f *AlterLayoutR6Facts) { f.ReplicaKnown = false }, AutoRandomUndecidable, LayoutReasonOldReplicaUnproven},
		{"count_zero_old_unproven_ok", &TiFlashReplicaDecl{Count: 0}, func(f *AlterLayoutR6Facts) { f.ReplicaKnown = false }, AutoRandomOK, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			facts := baseR6Facts()
			if tc.mutate != nil {
				tc.mutate(&facts)
			}
			verdict, reason := AlterTiFlashReplicaOutcome(tc.decl, facts)
			if verdict != tc.verdict || reason != tc.reason {
				t.Fatalf("got (%v,%q) want (%v,%q)", verdict, reason, tc.verdict, tc.reason)
			}
		})
	}
}
