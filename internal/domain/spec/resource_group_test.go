package spec

import "testing"

// UnmodeledOptionKinds must surface every option the audit surface
// cannot fully model — including a member nested inside an otherwise
// recognized QUERY_LIMIT clause, so the runaway group is never
// half-audited (issue #101 T22 R2-B).
func TestResourceGroupUnmodeledOptionKinds(t *testing.T) {
	modeled := ResourceGroupDeclaration{Options: []ResourceGroupOption{
		{Kind: ResourceGroupOptionRURate},
		{Kind: ResourceGroupOptionBurstable, Burstable: ResourceGroupBurstableEnabled},
		{Kind: ResourceGroupOptionQueryLimit, Runaway: &ResourceGroupRunawayDeclaration{
			Members: []ResourceGroupRunawayMember{
				{Kind: ResourceGroupRunawayExecElapsed, TextValue: "10s"},
				{Kind: ResourceGroupRunawayProcessedKeys},
				{Kind: ResourceGroupRunawayRU},
				{Kind: ResourceGroupRunawayAction, TextValue: "kill"},
				{Kind: ResourceGroupRunawayWatch, TextValue: "exact:1m"},
			},
		}},
	}}
	if got := modeled.UnmodeledOptionKinds(); len(got) != 0 {
		t.Fatalf("fully modeled declaration reported unmodeled kinds %v", got)
	}

	unmodeled := ResourceGroupDeclaration{Options: []ResourceGroupOption{
		{Kind: ResourceGroupOptionRURate},
		{Kind: ResourceGroupOptionBackground},
		{Kind: ResourceGroupOptionUnrecognized},
		{Kind: ResourceGroupOptionBurstable, Burstable: ResourceGroupBurstableUnsupportedForm},
		{Kind: ResourceGroupOptionQueryLimit, Runaway: &ResourceGroupRunawayDeclaration{
			Members: []ResourceGroupRunawayMember{
				{Kind: ResourceGroupRunawayExecElapsed, TextValue: "10s"},
				{Kind: "unrecognized"},
			},
		}},
	}}
	got := unmodeled.UnmodeledOptionKinds()
	want := []ResourceGroupOptionKind{
		ResourceGroupOptionBackground,
		ResourceGroupOptionUnrecognized,
		ResourceGroupOptionBurstable,
		ResourceGroupOptionQueryLimit,
	}
	if len(got) != len(want) {
		t.Fatalf("UnmodeledOptionKinds = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("UnmodeledOptionKinds[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// A complete, fully-recognized QUERY_LIMIT list that omits ACTION or has
// no positive threshold is adjudicated by the valid rule as a static
// refusal (fixed v8.5.0 NewGroupFromOptions) — it is NOT unmodeled
// evidence. Only an unrecognized member bounds the aspect unmodeled.
func TestResourceGroupRecognizedIncompleteRunawayModeled(t *testing.T) {
	incomplete := []ResourceGroupRunawayDeclaration{
		{Members: []ResourceGroupRunawayMember{
			{Kind: ResourceGroupRunawayExecElapsed, TextValue: "30s"},
		}},
		{Members: []ResourceGroupRunawayMember{
			{Kind: ResourceGroupRunawayAction, TextValue: "kill"},
		}},
		{Members: []ResourceGroupRunawayMember{
			{Kind: ResourceGroupRunawayWatch, TextValue: "exact:1m"},
		}},
		{Members: []ResourceGroupRunawayMember{
			{Kind: ResourceGroupRunawayRU, UintValue: func() *uint64 { v := uint64(0); return &v }()},
			{Kind: ResourceGroupRunawayAction, TextValue: "kill"},
		}},
	}
	for i, runaway := range incomplete {
		decl := ResourceGroupDeclaration{Options: []ResourceGroupOption{
			{Kind: ResourceGroupOptionQueryLimit, Runaway: &runaway},
		}}
		if got := decl.UnmodeledOptionKinds(); len(got) != 0 {
			t.Fatalf("case %d: UnmodeledOptionKinds = %v, want [] — recognized static refusal stays modeled", i, got)
		}
	}

	complete := ResourceGroupDeclaration{Options: []ResourceGroupOption{
		{Kind: ResourceGroupOptionQueryLimit, Runaway: &ResourceGroupRunawayDeclaration{
			Members: []ResourceGroupRunawayMember{
				{Kind: ResourceGroupRunawayExecElapsed, TextValue: "30s"},
				{Kind: ResourceGroupRunawayAction, TextValue: "kill"},
			},
		}},
	}}
	if got := complete.UnmodeledOptionKinds(); len(got) != 0 {
		t.Fatalf("threshold+action list reported unmodeled kinds %v", got)
	}

	reset := ResourceGroupDeclaration{Options: []ResourceGroupOption{
		{Kind: ResourceGroupOptionQueryLimit, Runaway: &ResourceGroupRunawayDeclaration{Reset: true}},
	}}
	if got := reset.UnmodeledOptionKinds(); len(got) != 0 {
		t.Fatalf("reset runaway reported unmodeled kinds %v", got)
	}
}

// HasEffectiveThreshold follows the fixed v8.5.0 all-zero refusal:
// declared-but-zero still counts as zero; a positive value in any one of
// the three threshold kinds suffices; an unparseable duration is not
// positive.
func TestResourceGroupRunawayEffectiveThreshold(t *testing.T) {
	zero := uint64(0)
	positive := uint64(10)
	cases := []struct {
		name    string
		members []ResourceGroupRunawayMember
		want    bool
	}{
		{"no threshold members", []ResourceGroupRunawayMember{
			{Kind: ResourceGroupRunawayAction, TextValue: "kill"},
		}, false},
		{"declared zero ru", []ResourceGroupRunawayMember{
			{Kind: ResourceGroupRunawayRU, UintValue: &zero},
		}, false},
		{"positive ru", []ResourceGroupRunawayMember{
			{Kind: ResourceGroupRunawayRU, UintValue: &positive},
		}, true},
		{"positive elapsed", []ResourceGroupRunawayMember{
			{Kind: ResourceGroupRunawayExecElapsed, TextValue: "30s"},
		}, true},
		{"zero elapsed", []ResourceGroupRunawayMember{
			{Kind: ResourceGroupRunawayExecElapsed, TextValue: "0s"},
		}, false},
		{"unparseable elapsed", []ResourceGroupRunawayMember{
			{Kind: ResourceGroupRunawayExecElapsed, TextValue: "not-a-duration"},
		}, false},
		{"positive keys", []ResourceGroupRunawayMember{
			{Kind: ResourceGroupRunawayProcessedKeys, UintValue: &positive},
		}, true},
	}
	for _, tc := range cases {
		decl := ResourceGroupRunawayDeclaration{Members: tc.members}
		if got := decl.HasEffectiveThreshold(); got != tc.want {
			t.Fatalf("%s: HasEffectiveThreshold = %v, want %v", tc.name, got, tc.want)
		}
	}
}
