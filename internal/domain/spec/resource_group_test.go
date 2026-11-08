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
