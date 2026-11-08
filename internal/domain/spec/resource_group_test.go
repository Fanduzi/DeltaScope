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

// A non-reset QUERY_LIMIT member list that omits ACTION or omits every
// threshold member is a form fixed TiDB v8.5.0 refuses at run time
// (ERROR 1105 "unknown resource group runaway action" / "please set at
// least one field"). Until the audit models that completeness refusal
// explicitly, the whole option stays bounded unmodeled evidence — never
// a claimed-clean clause — matching the issue #101 T22 native probe.
func TestResourceGroupUnmodeledIncompleteRunaway(t *testing.T) {
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
	}
	for i, runaway := range incomplete {
		decl := ResourceGroupDeclaration{Options: []ResourceGroupOption{
			{Kind: ResourceGroupOptionQueryLimit, Runaway: &runaway},
		}}
		got := decl.UnmodeledOptionKinds()
		if len(got) != 1 || got[0] != ResourceGroupOptionQueryLimit {
			t.Fatalf("case %d: UnmodeledOptionKinds = %v, want [query_limit]", i, got)
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
