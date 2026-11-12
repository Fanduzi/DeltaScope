package audit

// T26-SERVER-ORDERED-ABSENCE-01 EXECUTION-01 — the frozen S1/S6/S7/S8
// contract on the SERVER ordered channel: the OrderedAbsent data model
// carries the publishing statement's index and conditional provenance;
// same-identity writes invalidate, provably-distinct identities
// preserve, unprovable-relation writes shadow; the ASCII canonical
// domain stops at the trailing-space and non-ASCII boundaries.

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// S1: a proven drop publishes OrderedAbsent carrying its own statement
// index; a later same-identity drop consumes it as the terminal
// refusal, and an IF EXISTS consumes it as the guarded arm — both see
// the source index of the publishing statement.
func TestT26OrderedAbsentSourceIndex(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	enriched := enrichT26Premise(t,
		"DROP SERVER s1; DROP SERVER s1; DROP SERVER IF EXISTS s1",
		provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	if len(provider.calls) != 1 {
		t.Fatalf("resolver calls = %+v — OrderedAbsent must answer before the provider seam", provider.calls)
	}
	second := enriched[1].Metadata.Server
	if second == nil || !second.OrderedAbsent || !second.OrderedAbsentSourceSet || second.OrderedAbsentSourceIndex != 0 {
		t.Fatalf("second drop must bind OrderedAbsent sourced at index 0: %+v", second)
	}
	if !second.OrderedAbsentConditional {
		t.Fatalf("proven-arm provenance must mark the conditional scope: %+v", second)
	}
	third := enriched[2].Metadata.Server
	if third == nil || !third.OrderedAbsent || third.OrderedAbsentSourceIndex != 0 {
		t.Fatalf("guarded drop must read the same sourced absence: %+v", third)
	}
	if adj := enriched[1].Metadata.ServerDrop; adj == nil || adj.Arm != spec.ServerDropArmTerminalRefusal {
		t.Fatalf("second drop arm = %+v, want terminal refusal on sourced absence", adj)
	}
	if adj := enriched[2].Metadata.ServerDrop; adj == nil || adj.Arm != spec.ServerDropArmGuardedAbsent {
		t.Fatalf("third drop arm = %+v, want guarded absent on sourced absence", adj)
	}
}

// S6 same-identity invalidation: a proven create on the same canonical
// identity replaces the published absent — the trailing drop consumes
// the ordered present, never the stale absence.
func TestT26OrderedAbsentSameKeyInvalidatedByCreate(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	enriched := enrichT26Premise(t,
		"DROP SERVER s1; CREATE SERVER s1 FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', DATABASE 'd', USER 'u'); DROP SERVER s1",
		provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	first := enriched[0].Metadata.ServerDrop
	if first == nil || first.Arm != spec.ServerDropArmProven || !first.PublishAbsent {
		t.Fatalf("first drop = %+v, want proven publish", first)
	}
	third := enriched[2].Metadata.ServerDrop
	if third == nil || third.Arm == spec.ServerDropArmTerminalRefusal {
		t.Fatalf("third drop = %+v, must not consume the invalidated absence", third)
	}
	// The trailing drop either re-proves through the ordered present
	// or the create's own arm left a barrier — never a stale refusal.
	thirdState := enriched[2].Metadata.Server
	if thirdState != nil && thirdState.OrderedAbsent && thirdState.OrderedAbsentSourceIndex == 0 {
		t.Fatalf("stale sourced absence survived the same-key create: %+v", thirdState)
	}
}

// S6 boundary-write invalidation: an unaudited literal-name CREATE on
// the same identity taints the channel — the trailing drop binds the
// barrier, never the stale absence.
func TestT26OrderedAbsentLiteralCreateTaints(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	enriched := enrichT26Premise(t,
		"DROP SERVER s1; CREATE SERVER 's1' FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h'); DROP SERVER s1",
		provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	if enriched[1].Unsupported == nil {
		t.Fatalf("literal create must stay the unaudited boundary")
	}
	third := enriched[2].Metadata.ServerDrop
	if third == nil || third.Arm != spec.ServerDropArmBarrier {
		t.Fatalf("third drop = %+v, want barrier — literal write tainted the channel", third)
	}
}

// S6 provably-distinct preservation: a create on a fold-differing
// space-clean ASCII name leaves the published absent standing — the
// trailing drop consumes it with the original source index.
func TestT26OrderedAbsentDistinctKeyPreserved(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	enriched := enrichT26Premise(t,
		"DROP SERVER s1; CREATE SERVER s2 FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', DATABASE 'd', USER 'u'); DROP SERVER s1",
		provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	third := enriched[2].Metadata.ServerDrop
	if third == nil || third.Arm != spec.ServerDropArmTerminalRefusal {
		t.Fatalf("third drop = %+v, want terminal refusal on the preserved absence", third)
	}
	thirdState := enriched[2].Metadata.Server
	if thirdState == nil || !thirdState.OrderedAbsent || thirdState.OrderedAbsentSourceIndex != 0 {
		t.Fatalf("preserved absence must keep its source index: %+v", thirdState)
	}
}

// S6 boundary-write preservation: an unaudited create (non-mysql
// wrapper stays the audited boundary) on a provably-distinct ASCII
// identity must not shadow the ordered absent either.
func TestT26OrderedAbsentDistinctBoundaryWritePreserved(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	enriched := enrichT26Premise(t,
		"DROP SERVER s1; CREATE SERVER s2 FOREIGN DATA WRAPPER pgsql OPTIONS (HOST 'h'); DROP SERVER s1",
		provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	if enriched[1].Unsupported == nil {
		t.Fatalf("pgsql-wrapper create must stay the unaudited boundary")
	}
	third := enriched[2].Metadata.ServerDrop
	if third == nil || third.Arm != spec.ServerDropArmTerminalRefusal {
		t.Fatalf("third drop = %+v, want terminal refusal — distinct boundary write preserved the absence", third)
	}
}

// S6 identity-unprovable write: a non-ASCII audited create's unknown
// relation to every ordered identity retires them all — the trailing
// drop binds the barrier instead of a stale absence (channel taint
// through the relation scan, S8/D4).
func TestT26OrderedAbsentNonASCIIWriteTaints(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	enriched := enrichT26Premise(t,
		"DROP SERVER s1; CREATE SERVER `sérv` FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', DATABASE 'd', USER 'u'); DROP SERVER s1",
		provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	third := enriched[2].Metadata.ServerDrop
	if third == nil || third.Arm != spec.ServerDropArmBarrier {
		t.Fatalf("third drop = %+v, want barrier — unprovable-relation write retires the ordered answer", third)
	}
}

// S7/S8 relation boundary table: the ASCII provable-distinct domain is
// exactly fold-differing space-clean pairs; case-only and byte-equal
// stay same; trailing space and non-ASCII stay unproven.
func TestT26ServerRelationDomainTable(t *testing.T) {
	cases := []struct {
		a, b string
		want serverRelation
	}{
		{"s1", "s1", serverRelSame},
		{"s1", "S1", serverRelSame},
		{"s1", "s2", serverRelDistinct},
		{"srv", "s1", serverRelDistinct},
		{"s1", "s1 ", serverRelUnproven},   // PAD SPACE could equate them
		{"s1 ", "s1  ", serverRelUnproven}, // pad-equated too — fold differs, stay conservative
		{"sérv", "serv", serverRelUnproven},
		{"sérv", "s1", serverRelUnproven},
		{" s1", "s1", serverRelDistinct}, // leading space: PAD governs trailing only
	}
	for _, tc := range cases {
		if got := serverRel(tc.a, tc.b); got != tc.want {
			t.Fatalf("serverRel(%q,%q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

// The trailing-space write stays out of the distinct domain: a
// boundary create on 's1 ' is an unproven relation to s1 — it may
// have recreated s1's row, so s1's ordered answer must not survive
// it. (An ALTER 's1 ' cannot exercise this path — ALTER is
// presence-preserving and mints nothing on its non-publish arms.)
func TestT26OrderedAbsentTrailingSpaceWriteUnproven(t *testing.T) {
	provider := &t26Provider{servers: map[string]bool{"s1": true}, version: "8.0.36", privilege: superObservation()}
	enriched := enrichT26Premise(t,
		"DROP SERVER s1; CREATE SERVER `s1 ` FOREIGN DATA WRAPPER pgsql OPTIONS (HOST 'h'); DROP SERVER s1",
		provider, spec.PrivilegeAuditModeObservedAccountNewSession)
	if enriched[1].Unsupported == nil {
		t.Fatalf("pgsql-wrapper create must stay the unaudited boundary")
	}
	third := enriched[2].Metadata.ServerDrop
	if third == nil || third.Arm != spec.ServerDropArmBarrier {
		t.Fatalf("third drop = %+v, want barrier — trailing-space write is unprovable relation", third)
	}
}
