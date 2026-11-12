// T26-SRS-CREATE-PUBLICATION-01 execution matrix — the frozen
// F-1..F-11 / U1–U7 contract proven against the real provider
// adapter: audited arms, conflict premises, post-state minting, the
// public finding/gap shapes, and the regression invariants (DROP
// ordered channel untouched, TiDB vendor boundary held).
package audit_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/application/audit"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// srsCreateAdj returns the statement's SRSCreate adjudication — the
// shared one-truth record for the CREATE family.
func srsCreateAdj(t *testing.T, stmt spec.Statement) *spec.SRSCreateAdjudication {
	t.Helper()
	if stmt.Metadata == nil || stmt.Metadata.SRSCreate == nil {
		t.Fatalf("statement missing SRSCreate adjudication: %+v", stmt.Metadata)
	}
	return stmt.Metadata.SRSCreate
}

// createGap asserts the public gap the create family owes: exactly
// one, carrying the frozen reason and the ordered owed facts.
func createGap(t *testing.T, stmt report.StatementResult, reason string, wantFacts []string) {
	t.Helper()
	if len(stmt.EvidenceGaps) != 1 {
		t.Fatalf("gaps=%+v want exactly one", stmt.EvidenceGaps)
	}
	gap := stmt.EvidenceGaps[0]
	if gap.RuleID != "ddl.mysql.srs.create.existence" || gap.ReasonCode != reason {
		t.Fatalf("gap=%+v want owner ddl.mysql.srs.create.existence / %s", gap, reason)
	}
	if len(gap.RequiredFacts) != len(wantFacts) {
		t.Fatalf("gap facts=%v want ordered %v", gap.RequiredFacts, wantFacts)
	}
	for i := range wantFacts {
		if gap.RequiredFacts[i] != wantFacts[i] {
			t.Fatalf("gap facts[%d]=%q want ordered %v", i, gap.RequiredFacts[i], wantFacts)
		}
	}
}

// createFinding asserts exactly one finding with the wanted rule and
// returns it for the caller's shape assertions.
func createFinding(t *testing.T, stmt report.StatementResult, ruleID string) rule.Finding {
	t.Helper()
	var found *rule.Finding
	for i := range stmt.Findings {
		if stmt.Findings[i].RuleID == ruleID {
			if found != nil {
				t.Fatalf("duplicate %s findings: %+v", ruleID, stmt.Findings)
			}
			found = &stmt.Findings[i]
		}
	}
	if found == nil {
		t.Fatalf("findings=%+v want one %s", stmt.Findings, ruleID)
	}
	return *found
}

// --- C-ARM: existence + form matrix ------------------------------------

// C1: plain CREATE on proven-absent takes the create arm; the empty
// conflict read stays owed (never a proven "no conflict").
func TestT26SRSCreatePlainAbsent(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	provider, state := paRealState(t, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"CREATE SPATIAL REFERENCE SYSTEM 900001 NAME 'n1' DEFINITION 'd'")
	adj := srsCreateAdj(t, enriched[0])
	if adj.Arm != spec.SRSCreateArmCreate {
		t.Fatalf("arm=%v want create", adj.Arm)
	}
	if len(adj.Owed) != 1 || adj.Owed[0] != spec.SRSFactNameConflict {
		t.Fatalf("owed=%v want exactly [srs.name_conflict] — empty conflict is unknown, not no-conflict", adj.Owed)
	}
	if !adj.ConditionalAudit {
		t.Fatal("consumable premise must mark conditional_audit")
	}
}

// C2: plain CREATE on proven-present refuses existence.
func TestT26SRSCreatePlainPresent(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	fx.srs = map[uint32]string{4326: "WGS 84"}
	provider, state := paRealState(t, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"CREATE SPATIAL REFERENCE SYSTEM 4326 NAME 'n1' DEFINITION 'd'")
	if arm := srsCreateAdj(t, enriched[0]).Arm; arm != spec.SRSCreateArmRefusedExistence {
		t.Fatalf("arm=%v want refused_existence", arm)
	}
}

// C3: IF NOT EXISTS on present is the no-op arm — non-blocking — and
// discharges the privilege notice only when consumable.
func TestT26SRSCreateIfNotExistsPresent(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	fx.srs = map[uint32]string{4326: "WGS 84"}
	provider, state := paRealState(t, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"CREATE SPATIAL REFERENCE SYSTEM IF NOT EXISTS 4326 NAME 'n1' DEFINITION 'd'")
	adj := srsCreateAdj(t, enriched[0])
	if adj.Arm != spec.SRSCreateArmExistsNoop {
		t.Fatalf("arm=%v want exists_noop", adj.Arm)
	}
	if len(adj.Owed) != 0 || !adj.ConditionalAudit {
		t.Fatalf("owed=%v conditional=%v want [] + conditional_audit", adj.Owed, adj.ConditionalAudit)
	}
}

// C4: IF NOT EXISTS on absent continues the create path.
func TestT26SRSCreateIfNotExistsAbsent(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	provider, state := paRealState(t, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"CREATE SPATIAL REFERENCE SYSTEM IF NOT EXISTS 900001 NAME 'n1' DEFINITION 'd'")
	if arm := srsCreateAdj(t, enriched[0]).Arm; arm != spec.SRSCreateArmCreate {
		t.Fatalf("arm=%v want create", arm)
	}
}

// C5: OR REPLACE on absent takes create-result semantics.
func TestT26SRSCreateOrReplaceAbsent(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	provider, state := paRealState(t, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"CREATE OR REPLACE SPATIAL REFERENCE SYSTEM 900001 NAME 'n1' DEFINITION 'd'")
	if arm := srsCreateAdj(t, enriched[0]).Arm; arm != spec.SRSCreateArmCreate {
		t.Fatalf("arm=%v want create", arm)
	}
}

// C6: OR REPLACE on present takes replacement-result semantics.
func TestT26SRSCreateOrReplacePresent(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	fx.srs = map[uint32]string{4326: "WGS 84"}
	provider, state := paRealState(t, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"CREATE OR REPLACE SPATIAL REFERENCE SYSTEM 4326 NAME 'n1' DEFINITION 'd'")
	adj := srsCreateAdj(t, enriched[0])
	if adj.Arm != spec.SRSCreateArmReplace {
		t.Fatalf("arm=%v want replace", adj.Arm)
	}
}

// C7: unknown existence — an unparseable Qo banner strips the absence
// proof, so the arm is unproven with the existence premise owed.
func TestT26SRSCreateUnknownExistence(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	fx.version = "bogus"
	provider, state := paRealState(t, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"CREATE SPATIAL REFERENCE SYSTEM 900001 NAME 'n1' DEFINITION 'd'")
	adj := srsCreateAdj(t, enriched[0])
	if adj.Arm != spec.SRSCreateArmUnproven {
		t.Fatalf("arm=%v want unproven", adj.Arm)
	}
	if len(adj.Owed) != 2 || adj.Owed[0] != spec.SRSFactExistence || adj.Owed[1] != spec.SRSFactNameConflict {
		t.Fatalf("owed=%v want [srs.existence srs.name_conflict]", adj.Owed)
	}
}

// --- C-CONF: NAME / ORG conflict premises ------------------------------

// C8: an authored NAME owned by another SRID refuses name conflict.
func TestT26SRSCreateNameConflict(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	fx.nameOwners = map[string]uint32{"taken": 4327}
	provider, state := paRealState(t, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"CREATE SPATIAL REFERENCE SYSTEM 900001 NAME 'taken' DEFINITION 'd'")
	if arm := srsCreateAdj(t, enriched[0]).Arm; arm != spec.SRSCreateArmRefusedNameConflict {
		t.Fatalf("arm=%v want refused_name_conflict", arm)
	}
	if fx.count["name"] != 1 {
		t.Fatalf("name-conflict reads=%d want exactly one qualified read", fx.count["name"])
	}
}

// C9: an authored ORGANIZATION+IDENTIFIED BY pair owned by another
// SRID refuses organization conflict.
func TestT26SRSCreateOrgConflict(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	fx.orgOwners = map[string]uint32{"EPSG|1234": 4327}
	provider, state := paRealState(t, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"CREATE SPATIAL REFERENCE SYSTEM 900002 NAME 'n1' DEFINITION 'd' ORGANIZATION 'EPSG' IDENTIFIED BY 1234")
	if arm := srsCreateAdj(t, enriched[0]).Arm; arm != spec.SRSCreateArmRefusedOrgConflict {
		t.Fatalf("arm=%v want refused_org_conflict", arm)
	}
	if fx.count["orgc"] != 1 {
		t.Fatalf("org-conflict reads=%d want exactly one qualified read", fx.count["orgc"])
	}
}

// C10: a conflict witness matching the statement's OWN SRID is not a
// conflict — under OR REPLACE it is existence evidence and the arm is
// replace.
func TestT26SRSCreateOrReplaceOwnConflict(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	fx.nameOwners = map[string]uint32{"own": 4326}
	provider, state := paRealState(t, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"CREATE OR REPLACE SPATIAL REFERENCE SYSTEM 4326 NAME 'own' DEFINITION 'd'")
	adj := srsCreateAdj(t, enriched[0])
	if adj.Arm != spec.SRSCreateArmReplace {
		t.Fatalf("arm=%v want replace — own-SRID conflict is existence evidence, not a conflict", adj.Arm)
	}
}

// C11: the same own-SRID witness under a plain CREATE is presence
// evidence — refused_existence, never name conflict.
func TestT26SRSCreatePlainOwnConflict(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	fx.nameOwners = map[string]uint32{"own": 4326}
	provider, state := paRealState(t, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"CREATE SPATIAL REFERENCE SYSTEM 4326 NAME 'own' DEFINITION 'd'")
	if arm := srsCreateAdj(t, enriched[0]).Arm; arm != spec.SRSCreateArmRefusedExistence {
		t.Fatalf("arm=%v want refused_existence — the own witness proves presence", arm)
	}
}

// --- C-ATTR: authored deterministic attributes -------------------------

// C12: a duplicate authored attribute refuses deterministically.
func TestT26SRSCreateDuplicateAttribute(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	provider, state := paRealState(t, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"CREATE SPATIAL REFERENCE SYSTEM 900001 NAME 'a' NAME 'b' DEFINITION 'd'")
	adj := srsCreateAdj(t, enriched[0])
	if adj.Arm != spec.SRSCreateArmRefusedDuplicateAttribute || adj.DuplicateAttribute != "name" {
		t.Fatalf("adj=%+v want refused_duplicate_attribute/name", adj)
	}
}

// C13: missing mandatory attributes refuse — NAME and DEFINITION each.
func TestT26SRSCreateMissingAttribute(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	provider, state := paRealState(t, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"CREATE SPATIAL REFERENCE SYSTEM 900001 DEFINITION 'd';"+
			"CREATE SPATIAL REFERENCE SYSTEM 900002 NAME 'n1'")
	if arm := srsCreateAdj(t, enriched[0]).Arm; arm != spec.SRSCreateArmRefusedMissingAttribute ||
		srsCreateAdj(t, enriched[0]).MissingAttribute != "NAME" {
		t.Fatalf("stmt1 adj=%+v want refused_missing/NAME", enriched[0].Metadata.SRSCreate)
	}
	if arm := srsCreateAdj(t, enriched[1]).Arm; arm != spec.SRSCreateArmRefusedMissingAttribute ||
		srsCreateAdj(t, enriched[1]).MissingAttribute != "DEFINITION" {
		t.Fatalf("stmt2 adj=%+v want refused_missing/DEFINITION", enriched[1].Metadata.SRSCreate)
	}
}

// C14: SRID 0 refuses deterministically before any attribute check.
func TestT26SRSCreateSRID0(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	provider, state := paRealState(t, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"CREATE SPATIAL REFERENCE SYSTEM 0 NAME 'n1' DEFINITION 'd'")
	if arm := srsCreateAdj(t, enriched[0]).Arm; arm != spec.SRSCreateArmRefusedSRID0 {
		t.Fatalf("arm=%v want refused_srid0", arm)
	}
}

// --- C-PRIV: privilege premise projection -------------------------------

// C15: a non-consumable premise rides the outcome arm's owed set as
// srs.create.privilege — the create identity, never the drop's.
func TestT26SRSCreatePrivilegeUnproven(t *testing.T) {
	fx := opAbsentFixture(opGrantNegative)
	provider, state := paRealState(t, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"CREATE SPATIAL REFERENCE SYSTEM 900001 NAME 'n1' DEFINITION 'd'")
	adj := srsCreateAdj(t, enriched[0])
	if adj.Arm != spec.SRSCreateArmCreate {
		t.Fatalf("arm=%v want create", adj.Arm)
	}
	want := []string{spec.SRSFactNameConflict, spec.SRSFactCreatePrivilege}
	if len(adj.Owed) != len(want) {
		t.Fatalf("owed=%v want %v", adj.Owed, want)
	}
	for i := range want {
		if adj.Owed[i] != want[i] {
			t.Fatalf("owed[%d]=%q want %v", i, adj.Owed[i], want)
		}
	}
	if adj.ConditionalAudit {
		t.Fatal("unproven premise must not mark conditional_audit")
	}
}

// C16: IF NOT EXISTS on present with an unproven premise owes the
// privilege premise — the suppressed-conflict no-op still audits its
// own privilege gate.
func TestT26SRSCreateNoopPrivilegeUnproven(t *testing.T) {
	fx := opAbsentFixture(opGrantNegative)
	fx.srs = map[uint32]string{4326: "WGS 84"}
	provider, state := paRealState(t, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"CREATE SPATIAL REFERENCE SYSTEM IF NOT EXISTS 4326 NAME 'n1' DEFINITION 'd'")
	adj := srsCreateAdj(t, enriched[0])
	if adj.Arm != spec.SRSCreateArmExistsNoop {
		t.Fatalf("arm=%v want exists_noop", adj.Arm)
	}
	if len(adj.Owed) != 1 || adj.Owed[0] != spec.SRSFactCreatePrivilege {
		t.Fatalf("owed=%v want exactly [srs.create.privilege]", adj.Owed)
	}
}

// --- C-POST: own-apply post-state (F-8) ---------------------------------

// C17: OR REPLACE on present mints proven presence — a later
// same-SRID DROP binds Exists from the mint with zero further
// catalog reads, and publishes its ordered entry (the mint is
// consumed by reads, never by the ordered channel).
func TestT26SRSCreateReplaceMintsPresent(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	fx.srs = map[uint32]string{4326: "WGS 84"}
	provider, state := paRealState(t, fx)
	var vecs [][5]int
	var staleAt [2]map[uint32]bool
	state.SetAfterApplyObserve(func(i int, stmt *spec.Statement) {
		vecs = append(vecs, opReads(provider, fx))
		staleAt[i] = audit.BatchStateStaleSet(state)
	})
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"CREATE OR REPLACE SPATIAL REFERENCE SYSTEM 4326 NAME 'n1' DEFINITION 'd';"+
			"DROP SPATIAL REFERENCE SYSTEM 4326")
	if arm := srsCreateAdj(t, enriched[0]).Arm; arm != spec.SRSCreateArmReplace {
		t.Fatalf("stmt1 arm=%v want replace", arm)
	}
	adj := paAdj(t, enriched[1])
	if adj.Arm != spec.SRSDropArmUnproven {
		t.Fatalf("stmt2 arm=%v want S2 — a proven presence has no absent arm", adj.Arm)
	}
	for _, fact := range adj.Owed {
		if fact == spec.SRSFactExistence {
			t.Fatalf("owed=%v must not owe srs.existence — the present mint answered it", adj.Owed)
		}
	}
	if len(state.SRSOrderedSRIDs()) != 0 {
		t.Fatalf("ordered entries=%v want none — S2 never publishes", state.SRSOrderedSRIDs())
	}
	// stmt1 issues its full resolver pass; stmt2 consumes the mint —
	// zero reads.
	opWantDelta(t, "stmt2", opDelta(vecs[0], vecs[1]), [5]int{0, 0, 0, 0, 0})
	// Post-apply timeline: the replace minted NO staleness (presence
	// only); the tail's own unproven apply then mints it — the drop
	// may have deleted the row.
	paSameStale(t, staleAt[0], map[uint32]bool{}, "post-replace — presence mint only")
	paSameStale(t, staleAt[1], map[uint32]bool{4326: true}, "post-unproven-drop — its own apply mints stale")
}

// C18: an uncertain create arm (unproven existence) mints staleness —
// the same-SRID DROP tail blocks with [B] and zero reads.
func TestT26SRSCreateUnprovenMintsStale(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	fx.version = "bogus"
	provider, state := paRealState(t, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"CREATE SPATIAL REFERENCE SYSTEM 900001 NAME 'n1' DEFINITION 'd';"+
			"DROP SPATIAL REFERENCE SYSTEM 900001")
	if arm := srsCreateAdj(t, enriched[0]).Arm; arm != spec.SRSCreateArmUnproven {
		t.Fatalf("stmt1 arm=%v want unproven", arm)
	}
	if arm := paAdj(t, enriched[1]).Arm; arm != spec.SRSDropArmBlocked {
		t.Fatalf("stmt2 arm=%v want blocked — uncertainty minted staleness", arm)
	}
	paSameStale(t, audit.BatchStateStaleSet(state), map[uint32]bool{900001: true}, "unproven minted stale")
}

// C19: refused and no-op arms write nothing — no stale mint, no
// presence mint, no ordered entry.
func TestT26SRSCreateRefusedWritesNothing(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	fx.srs = map[uint32]string{4326: "WGS 84"}
	provider, state := paRealState(t, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"CREATE SPATIAL REFERENCE SYSTEM 4326 NAME 'n1' DEFINITION 'd';"+
			"CREATE SPATIAL REFERENCE SYSTEM IF NOT EXISTS 4326 NAME 'n2' DEFINITION 'd';"+
			"CREATE SPATIAL REFERENCE SYSTEM 0 NAME 'n3' DEFINITION 'd'")
	if len(state.SRSOrderedSRIDs()) != 0 {
		t.Fatalf("ordered entries=%v want none — CREATE never produces ordered publication", state.SRSOrderedSRIDs())
	}
	if got := audit.BatchStateStaleSet(state); len(got) != 0 {
		t.Fatalf("stale=%v want empty — refused/no-op arms mint nothing", got)
	}
	if len(enriched) != 3 {
		t.Fatalf("enriched=%d want 3", len(enriched))
	}
}

// C20: CREATE is never an ordered-publication producer — even its
// successful arms mint no entry and never touch the DROP channel.
func TestT26SRSCreateNeverPublishesOrdered(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	fx.srs = map[uint32]string{4326: "WGS 84"}
	provider, state := paRealState(t, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"CREATE OR REPLACE SPATIAL REFERENCE SYSTEM 4326 NAME 'n1' DEFINITION 'd'")
	if arm := srsCreateAdj(t, enriched[0]).Arm; arm != spec.SRSCreateArmReplace {
		t.Fatalf("arm=%v want replace", arm)
	}
	if len(state.SRSOrderedSRIDs()) != 0 {
		t.Fatalf("ordered entries=%v want none — CREATE is not an ordered producer", state.SRSOrderedSRIDs())
	}
	if opSRS(t, enriched[0]).OrderedAbsent {
		t.Fatal("a create row never carries the ordered marker")
	}
}

// --- C-PUB: public projection shapes -------------------------------------

// C21: the public finding/gap vocabulary — existence blocker,
// suppressed-conflict notice, attribute blockers, conflict blockers,
// privilege notice, owed-fact gaps.
func TestT26SRSCreatePublicShapes(t *testing.T) {
	newFixture := func(mutate func(*r3Fixture)) *r3Fixture {
		fx := r3PrivilegeFixture()
		fx.srs = nil
		if mutate != nil {
			mutate(fx)
		}
		return fx
	}

	t.Run("existence blocker", func(t *testing.T) {
		fx := newFixture(func(fx *r3Fixture) { fx.srs = map[uint32]string{4326: "WGS 84"} })
		p := r3Harness(t, fx)
		result, err := r3PrivilegeRequest(t, p,
			"CREATE SPATIAL REFERENCE SYSTEM 4326 NAME 'n1' DEFINITION 'd'")
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		f := createFinding(t, result.Statements[0], "ddl.mysql.srs.create.existence")
		if f.Level != rule.LevelBlocker || f.Metadata["srid"] != uint32(4326) {
			t.Fatalf("finding=%+v want blocker with srid metadata", f)
		}
	})

	t.Run("name conflict blocker carries the witness srid", func(t *testing.T) {
		fx := newFixture(func(fx *r3Fixture) { fx.nameOwners = map[string]uint32{"taken": 4327} })
		p := r3Harness(t, fx)
		result, err := r3PrivilegeRequest(t, p,
			"CREATE SPATIAL REFERENCE SYSTEM 900001 NAME 'taken' DEFINITION 'd'")
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		f := createFinding(t, result.Statements[0], "ddl.mysql.srs.create.name_conflict")
		if f.Level != rule.LevelBlocker || f.Metadata["name_witness_count"] != 1 ||
			f.Metadata["outcome"] != "refused_name_conflict" || f.Metadata["srid"] != uint32(900001) {
			t.Fatalf("finding=%+v want blocker with srid/name_witness_count/outcome metadata", f)
		}
	})

	t.Run("org conflict blocker carries the witness srid", func(t *testing.T) {
		fx := newFixture(func(fx *r3Fixture) { fx.orgOwners = map[string]uint32{"EPSG|1234": 4327} })
		p := r3Harness(t, fx)
		result, err := r3PrivilegeRequest(t, p,
			"CREATE SPATIAL REFERENCE SYSTEM 900002 NAME 'n1' DEFINITION 'd' ORGANIZATION 'EPSG' IDENTIFIED BY 1234")
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		f := createFinding(t, result.Statements[0], "ddl.mysql.srs.create.organization_conflict")
		if f.Level != rule.LevelBlocker || f.Metadata["org_witness_count"] != 1 ||
			f.Metadata["outcome"] != "refused_organization_conflict" || f.Metadata["srid"] != uint32(900002) {
			t.Fatalf("finding=%+v want blocker with srid/org_witness_count/outcome metadata", f)
		}
	})

	t.Run("missing attribute blocker names the attribute", func(t *testing.T) {
		fx := newFixture(nil)
		p := r3Harness(t, fx)
		result, err := r3PrivilegeRequest(t, p,
			"CREATE SPATIAL REFERENCE SYSTEM 900001 DEFINITION 'd'")
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		f := createFinding(t, result.Statements[0], "ddl.mysql.srs.create.missing_attribute")
		if f.Level != rule.LevelBlocker || f.Metadata["attribute"] != "NAME" {
			t.Fatalf("finding=%+v want blocker with attribute=NAME", f)
		}
	})

	t.Run("duplicate attribute blocker names the attribute", func(t *testing.T) {
		fx := newFixture(nil)
		p := r3Harness(t, fx)
		result, err := r3PrivilegeRequest(t, p,
			"CREATE SPATIAL REFERENCE SYSTEM 900001 NAME 'a' NAME 'b' DEFINITION 'd'")
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		f := createFinding(t, result.Statements[0], "ddl.mysql.srs.create.duplicate_attribute")
		if f.Level != rule.LevelBlocker || f.Metadata["attribute"] != "name" {
			t.Fatalf("finding=%+v want blocker with attribute=name", f)
		}
	})

	t.Run("srid0 blocker", func(t *testing.T) {
		fx := newFixture(nil)
		p := r3Harness(t, fx)
		result, err := r3PrivilegeRequest(t, p,
			"CREATE SPATIAL REFERENCE SYSTEM 0 NAME 'n1' DEFINITION 'd'")
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		f := createFinding(t, result.Statements[0], "ddl.mysql.srs.create.srid0")
		if f.Level != rule.LevelBlocker {
			t.Fatalf("finding=%+v want blocker", f)
		}
	})

	t.Run("exists_noop notice — the suppressed conflict marker", func(t *testing.T) {
		fx := newFixture(func(fx *r3Fixture) { fx.srs = map[uint32]string{4326: "WGS 84"} })
		p := r3Harness(t, fx)
		result, err := r3PrivilegeRequest(t, p,
			"CREATE SPATIAL REFERENCE SYSTEM IF NOT EXISTS 4326 NAME 'n1' DEFINITION 'd'")
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		f := createFinding(t, result.Statements[0], "ddl.mysql.srs.create.exists_noop")
		if f.Level != rule.LevelNotice {
			t.Fatalf("finding=%+v want the notice-level suppressed-conflict marker", f)
		}
	})

	t.Run("owed premises project as gaps", func(t *testing.T) {
		fx := newFixture(func(fx *r3Fixture) {
			fx.grants = []string{"GRANT SELECT ON *.* TO 'app'@'%'"}
		})
		p := r3Harness(t, fx)
		result, err := r3PrivilegeRequest(t, p,
			"CREATE SPATIAL REFERENCE SYSTEM 900001 NAME 'n1' DEFINITION 'd'")
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		createGap(t, result.Statements[0], "unknown_srs_state",
			[]string{spec.SRSFactNameConflict, spec.SRSFactCreatePrivilege})
	})

	t.Run("privilege-only owed narrows to unproven_privilege", func(t *testing.T) {
		fx := newFixture(func(fx *r3Fixture) {
			fx.srs = map[uint32]string{4326: "WGS 84"}
			fx.grants = []string{"GRANT SELECT ON *.* TO 'app'@'%'"}
		})
		p := r3Harness(t, fx)
		result, err := r3PrivilegeRequest(t, p,
			"CREATE SPATIAL REFERENCE SYSTEM IF NOT EXISTS 4326 NAME 'n1' DEFINITION 'd'")
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		createGap(t, result.Statements[0], "unproven_privilege",
			[]string{spec.SRSFactCreatePrivilege})
	})
}

// C22: the TiDB vendor boundary is untouched — CREATE SRS under TiDB
// stays Unsupported with the vendor marker, never an audited arm.
func TestT26SRSCreateTiDBBoundary(t *testing.T) {
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL:           "CREATE SPATIAL REFERENCE SYSTEM 4326 NAME 'n1' DEFINITION 'd'",
		Dialect:       spec.DialectTiDB,
		TargetVersion: "8.0.36",
	})
	if !errors.Is(err, audit.ErrUnsupportedStatement) {
		t.Fatalf("err=%v want ErrUnsupportedStatement", err)
	}
	if len(result.Unsupported) != 1 ||
		result.Unsupported[0].Metadata["boundary"] != "vendor" {
		t.Fatalf("unsupported=%+v want exactly the TiDB vendor boundary", result.Unsupported)
	}
	if len(result.Statements[0].Findings) != 0 {
		t.Fatalf("vendor-boundary findings=%+v want none — no MySQL semantic finding leaks", result.Statements[0].Findings)
	}
}

// C23: privacy — the authored DEFINITION and DESCRIPTION payloads
// never surface in findings, gaps, messages, or metadata.
func TestT26SRSCreatePrivacyNoPayload(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	fx.srs = map[uint32]string{4326: "WGS 84"}
	p := r3Harness(t, fx)
	result, err := r3PrivilegeRequest(t, p,
		"CREATE SPATIAL REFERENCE SYSTEM 4326 NAME 'n1' DEFINITION 'PAYLOAD_MARKER_DEF' DESCRIPTION 'PAYLOAD_MARKER_DESC';"+
			"CREATE SPATIAL REFERENCE SYSTEM 900001 NAME 'n2' DEFINITION 'PAYLOAD_MARKER_DEF' DESCRIPTION 'PAYLOAD_MARKER_DESC'")
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	for i, stmt := range result.Statements {
		for _, f := range stmt.Findings {
			blob := f.Message + " " + f.Suggestion + " " + fmt.Sprintf("%v", f.Metadata)
			if strings.Contains(blob, "PAYLOAD_MARKER") {
				t.Fatalf("stmt%d finding leaks payload text: %+v", i, f)
			}
		}
		for _, g := range stmt.EvidenceGaps {
			blob := g.ReasonCode
			for _, fact := range g.RequiredFacts {
				blob += " " + fact
			}
			if strings.Contains(blob, "PAYLOAD_MARKER") {
				t.Fatalf("stmt%d gap leaks payload text: %+v", i, g)
			}
		}
	}
}

// C24: the version gate's lower bound — a resolved 5.7 keeps the
// CREATE family a boundary (inherited gate, F-2/F-10): never an
// audited arm, and the boundary lifecycle still mints staleness.
func TestT26SRSCreateVersionGateBelow(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	fx.version = "5.7.44"
	fx.discoverVer = "5.7.44"
	provider, state := paRealState(t, fx)
	state.SeedResolvedVersion(spec.ParseObservedVersion("5.7.44"))
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"CREATE SPATIAL REFERENCE SYSTEM 900001 NAME 'n1' DEFINITION 'd';"+
			"DROP SPATIAL REFERENCE SYSTEM 900001")
	if enriched[0].Unsupported == nil {
		t.Fatalf("stmt1 unsupported=nil want boundary — 5.7 is outside the audited domain")
	}
	if enriched[0].Metadata != nil && enriched[0].Metadata.SRSCreate != nil {
		t.Fatal("a version-gated row never carries an audited adjudication")
	}
	// The whole family is gated under 5.7 — the tail DROP is a
	// boundary too, never an audited arm.
	if enriched[1].Unsupported == nil {
		t.Fatal("stmt2 unsupported=nil want boundary — the DROP family is equally gated")
	}
	if enriched[1].Metadata != nil && enriched[1].Metadata.SRSDrop != nil {
		t.Fatal("a version-gated DROP never adjudicates")
	}
	// The gated write's lifecycle still mints authored stale.
	paSameStale(t, audit.BatchStateStaleSet(state), map[uint32]bool{900001: true}, "gated write minted stale")
}

// C25: a dependency witness never gates CREATE — U7: the family has
// no dependency premise, so a visible geometry column on the SRID
// leaves the create arm untouched.
func TestT26SRSCreateDependencyNotPremise(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	fx.deps = []r3DepRow{{schema: "s", table: "t", column: "g", srid: 900001}}
	provider, state := paRealState(t, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"CREATE SPATIAL REFERENCE SYSTEM 900001 NAME 'n1' DEFINITION 'd'")
	adj := srsCreateAdj(t, enriched[0])
	if adj.Arm != spec.SRSCreateArmCreate {
		t.Fatalf("arm=%v want create — a dependency witness is no CREATE premise (U7)", adj.Arm)
	}
	for _, fact := range adj.Owed {
		if fact == spec.SRSFactColumnDependencies {
			t.Fatalf("owed=%v must never carry the dependency premise", adj.Owed)
		}
	}
}

// C26: an incomplete ORGANIZATION pair is a whole-input rejection —
// no audited arm, no partial semantics.
func TestT26SRSCreateIncompleteOrgPair(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	provider, state := paRealState(t, fx)
	enriched := audit.EnrichSRSPrivForTest(t, state, provider,
		"CREATE SPATIAL REFERENCE SYSTEM 900001 NAME 'n1' DEFINITION 'd' ORGANIZATION 'EPSG'")
	if len(enriched) != 0 {
		t.Fatalf("enriched=%d want 0 — an incomplete ORGANIZATION pair rejects the whole input before any semantic evaluation", len(enriched))
	}
}
