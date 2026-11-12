package audit

// R2 supplementary rows: the frozen line-level witnesses the R1 pass
// collapsed or skipped — real GEOMETRY parse failures, positive
// witnesses on the boundary/different-SRID rows, request/API layer
// version-gate and error paths, full serialization redaction, and the
// SERVER privilege-isolation witness.
//
// Read-shape columns follow the contract's R/Qexist/Qdeps/Qo/L/Vd order.

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// --- R2-C.1: E8.3/E8.3b — the real GEOMETRY parser refusal --------------------

// Both frozen rows run the authored SQL unchanged: the vendored parser
// rejects GEOMETRY at the Type position (no identifier fallback — the
// accepted Type-subtree proof), producing a parseFailure while the
// following DROP still extracts; authored ordering then contaminates
// the DROP into arm B with every SRS read seam at zero.
func TestT26SRSR2GeometryTypeRefusal(t *testing.T) {
	for _, sqlText := range []string{
		"CREATE TABLE t_new (g GEOMETRY SRID 4326); DROP SPATIAL REFERENCE SYSTEM 4326", // E8.3
		"CREATE TABLE t_new (g GEOMETRY); DROP SPATIAL REFERENCE SYSTEM 4326",           // E8.3b
	} {
		fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{version: "8.0.36"}, nil)
		parsed, _ := parseSQL(context.Background(), sqlText, spec.DialectMySQL)
		if len(parsed.failures) != 1 || len(parsed.Statements) != 1 {
			t.Fatalf("%q: failures=%d statements=%d want 1/1", sqlText, len(parsed.failures), len(parsed.Statements))
		}
		failure := parsed.failures[0]
		if failure.Line != 1 || failure.Column != 1 {
			t.Fatalf("%q: failure at (%d,%d) want (1,1) — GEOMETRY rejected at the type position", sqlText, failure.Line, failure.Column)
		}
		statements, err := Extract(context.Background(), parsed)
		if err != nil || len(statements) != 1 {
			t.Fatalf("%q: extract err=%v statements=%d want the DROP only", sqlText, err, len(statements))
		}
		state := newSRSState(provider)
		enriched, err := state.enrichStatements(context.Background(),
			&MetadataRequest{Schema: "golden", Provider: provider},
			statements, parsed.failures, state.instanceFacts, auditLimits{orderedStatements: defaultOrderedStatementLimit})
		if err != nil {
			t.Fatalf("%q: enrich: %v", sqlText, err)
		}
		if !state.contaminated {
			t.Fatalf("%q: the authored failure must contaminate", sqlText)
		}
		if arm := srsArm(t, enriched[0]); arm != spec.SRSDropArmBlocked {
			t.Fatalf("%q: arm=%v want blocked", sqlText, arm)
		}
		if got := srsOwed(enriched[0]); len(got) != 1 || got[0] != spec.SRSFactBatchEffects {
			t.Fatalf("%q: owed=%v want [B]", sqlText, got)
		}
		assertReads(t, provider, fixture, 0, 0, 0, 0, 0)
		result := evaluateEnriched(t, enriched)
		if len(result.Statements[0].Findings) != 1 ||
			result.Statements[0].Findings[0].RuleID != "ddl.mysql.srs.drop.reserved" {
			t.Fatalf("%q: findings=%+v want only the blocked-arm reserved co-fire", sqlText, result.Statements[0].Findings)
		}
	}
}

// --- R2-C.2: E7.2 — boundary CREATE + different SRID with a positive
// dependency witness refuses -----------------------------------------------

func TestT26SRSR2BoundaryDifferentSRIDRefuses(t *testing.T) {
	fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{100001: "proj"},
		deps:    []srsDepRow{{schema: "s", table: "t", column: "g", srid: 100001}},
	}, nil)
	enriched := enrichSRS(t, newSRSState(provider), provider,
		"CREATE OR REPLACE SPATIAL REFERENCE SYSTEM 4326 DEFINITION 'x'; DROP SPATIAL REFERENCE SYSTEM 100001")
	if enriched[0].Unsupported == nil {
		t.Fatal("stmt1 must carry the implementation boundary")
	}
	if arm := srsArm(t, enriched[1]); arm != spec.SRSDropArmRefusedDependency {
		t.Fatalf("stmt2 arm=%v want refused_dependency — the boundary write mints staleness only on SRID 4326", arm)
	}
	assertReads(t, provider, fixture, 2, 2, 2, 2, 0)
	result := evaluateEnriched(t, enriched)
	if len(result.Statements[1].Findings) != 1 ||
		result.Statements[1].Findings[0].RuleID != "ddl.mysql.srs.drop.dependency" {
		t.Fatalf("stmt2 findings=%+v want the dependency blocker", result.Statements[1].Findings)
	}
}

// --- R2-C.3/R3-D: E8.2a positive control + E8.2b inert-CREATE ---------

// E8.2a: the inert CREATE leaves the positive dependency witness fully
// consumable — the tbl-u fixture's s.u.g row drives refused_dependency.
func TestT26SRSR2InertCreatePositiveControl(t *testing.T) {
	fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{4326: "EPSG 4326"},
		deps:    []srsDepRow{{schema: "s", table: "u", column: "g", srid: 4326}},
	}, tblTFixture())
	state := newSRSState(provider)
	enriched := enrichSRS(t, state, provider,
		"CREATE TABLE s.t_new (i INT); DROP SPATIAL REFERENCE SYSTEM 4326")
	if arm := srsArm(t, enriched[1]); arm != spec.SRSDropArmRefusedDependency {
		t.Fatalf("stmt2 arm=%v want refused_dependency — the positive s.u.g witness survives the inert CREATE", arm)
	}
	if got := srsOwed(enriched[1]); len(got) != 0 {
		t.Fatalf("owed=%v want empty — refusal owes nothing", got)
	}
	if state.srsDepsStale {
		t.Fatal("inert CREATE minted dependency staleness")
	}
	result := evaluateEnriched(t, enriched)
	stmt := result.Statements[1]
	if len(stmt.Findings) != 1 || stmt.Findings[0].RuleID != "ddl.mysql.srs.drop.dependency" {
		t.Fatalf("findings=%+v want only the dependency blocker", stmt.Findings)
	}
	if len(stmt.EvidenceGaps) != 0 || stmt.Coverage.Status != report.CoverageComplete {
		t.Fatalf("gaps=%+v coverage=%v want none/complete", stmt.EvidenceGaps, stmt.Coverage.Status)
	}
	assertReads(t, provider, fixture, 1, 1, 1, 1, 0)
}

// E8.2b: inert CREATE then an empty-incomplete SRID stays S2 — the
// unproven result is caused by the empty deps answer itself, never by
// stale state (srsDepsStale asserted false at the SRS bind point).
func TestT26SRSR2InertCreateEmptyIncomplete(t *testing.T) {
	fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{4326: "EPSG 4326"},
	}, tblTFixture())
	state := newSRSState(provider)
	var depsStaleAtSRSBind *bool
	state.afterBindObserve = func(i int, _ *spec.Statement) {
		if i == 1 {
			flag := state.srsDepsStale
			depsStaleAtSRSBind = &flag
		}
	}
	enriched := enrichSRS(t, state, provider,
		"CREATE TABLE s.t_new (i INT); DROP SPATIAL REFERENCE SYSTEM 4326")
	if depsStaleAtSRSBind == nil || *depsStaleAtSRSBind {
		t.Fatalf("srsDepsStale at SRS bind=%v want false — inert CREATE writes nothing", depsStaleAtSRSBind)
	}
	if arm := srsArm(t, enriched[1]); arm != spec.SRSDropArmUnproven {
		t.Fatalf("stmt2 arm=%v want unproven — inert CREATE on tbl_u keeps the SRID unshadowed", arm)
	}
	wantOwed := []string{spec.SRSFactColumnDependencies, spec.SRSFactDropPrivilege}
	if got := srsOwed(enriched[1]); !reflect.DeepEqual(got, wantOwed) {
		t.Fatalf("owed=%v want exactly %v — empty deps is incomplete, never proven-absent", got, wantOwed)
	}
	result := evaluateEnriched(t, enriched)
	stmt := result.Statements[1]
	if len(stmt.EvidenceGaps) != 1 || !reflect.DeepEqual(stmt.EvidenceGaps[0].RequiredFacts, wantOwed) {
		t.Fatalf("public gap=%+v want exactly the %v evidence gap", stmt.EvidenceGaps, wantOwed)
	}
	reserved := 0
	for _, f := range stmt.Findings {
		if f.RuleID == "ddl.mysql.srs.drop.reserved" {
			reserved++
		}
	}
	if reserved != 1 {
		t.Fatalf("reserved notices=%d want exactly 1 — 4326 is reserved", reserved)
	}
	assertReads(t, provider, fixture, 1, 1, 1, 1, 0)
}

// --- R2-C.4: E5.2 — actual IF EXISTS 0 lands S0 ------------------------------

func TestT26SRSR2IfExistsSRID0(t *testing.T) {
	fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{0: "0"},
	}, nil)
	enriched := enrichSRS(t, newSRSState(provider), provider, "DROP SPATIAL REFERENCE SYSTEM IF EXISTS 0")
	if arm := srsArm(t, enriched[0]); arm != spec.SRSDropArmRefusedSRID0 {
		t.Fatalf("arm=%v want refused_srid0 — IF EXISTS never enters adjudication", arm)
	}
	assertReads(t, provider, fixture, 1, 1, 1, 1, 0)
	result := evaluateEnriched(t, enriched)
	if len(result.Statements[0].Findings) != 1 ||
		result.Statements[0].Findings[0].RuleID != "ddl.mysql.srs.drop.srid0" {
		t.Fatalf("findings=%+v want the srid0 blocker", result.Statements[0].Findings)
	}
}

// --- R2-C.5: E5.3/E5.6/E5.11 statement-level reserved range cells ------------

func TestT26SRSR2ReservedStatementCells(t *testing.T) {
	cells := []struct {
		srid    uint32
		advised bool
	}{
		{0, true},      // E5.6a — advised but refused arm suppresses it
		{1, true},      // E5.6b
		{32767, true},  // E5.4
		{32768, false}, // E5.5
		{59999999, false},
		{60000000, true}, // E5.6c
		{69999999, true}, // E5.6d
		{70000000, false},
		{1999999999, false},
		{2000000000, true}, // E5.11 lower bound
		{2147483647, true}, // E5.6e
		{2147483648, false},
	}
	for _, cell := range cells {
		fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{
			version: "8.0.36",
			srs:     map[uint32]string{cell.srid: "x"},
		}, nil)
		enriched := enrichSRS(t, newSRSState(provider), provider,
			"DROP SPATIAL REFERENCE SYSTEM "+strconv.FormatUint(uint64(cell.srid), 10))
		result := evaluateEnriched(t, enriched)
		stmt := result.Statements[0]
		arm := srsArm(t, enriched[0])
		if cell.srid == 0 {
			if arm != spec.SRSDropArmRefusedSRID0 {
				t.Fatalf("srid 0: arm=%v want refused_srid0", arm)
			}
			for _, f := range stmt.Findings {
				if f.RuleID == "ddl.mysql.srs.drop.reserved" {
					t.Fatalf("srid 0: advisory suppressed inside the refusal arm")
				}
			}
			continue
		}
		if arm != spec.SRSDropArmUnproven {
			t.Fatalf("srid %d: arm=%v want unproven (exists, empty deps)", cell.srid, arm)
		}
		if got := srsOwed(enriched[0]); len(got) != 2 || got[0] != spec.SRSFactColumnDependencies || got[1] != spec.SRSFactDropPrivilege {
			t.Fatalf("srid %d: owed=%v want [D,P]", cell.srid, got)
		}
		reserved := 0
		for _, f := range stmt.Findings {
			if f.RuleID == "ddl.mysql.srs.drop.reserved" {
				reserved++
				assertReservedAdvisoryText(t, f)
			}
		}
		if cell.advised && reserved != 1 {
			t.Fatalf("srid %d: reserved findings=%d want 1", cell.srid, reserved)
		}
		if !cell.advised && reserved != 0 {
			t.Fatalf("srid %d: unexpected advisory", cell.srid)
		}
		if stmt.Coverage.Status != report.CoverageUnverified {
			t.Fatalf("srid %d: coverage=%v want unverified", cell.srid, stmt.Coverage.Status)
		}
		assertReads(t, provider, fixture, 1, 1, 1, 1, 0)
	}
}

// E5.11: 2000000000 is inside a reserved block AND can still hold a
// positive witness — the refusal arm suppresses the advisory.
func TestT26SRSR2ReservedSuppressedInRefusal(t *testing.T) {
	fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{2000000000: "x"},
		deps:    []srsDepRow{{schema: "s", table: "t", column: "g", srid: 2000000000}},
	}, nil)
	enriched := enrichSRS(t, newSRSState(provider), provider,
		"DROP SPATIAL REFERENCE SYSTEM 2000000000")
	if arm := srsArm(t, enriched[0]); arm != spec.SRSDropArmRefusedDependency {
		t.Fatalf("arm=%v want refused_dependency", arm)
	}
	result := evaluateEnriched(t, enriched)
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.mysql.srs.drop.reserved" {
			t.Fatal("reserved advisory must be suppressed inside the refusal arm")
		}
	}
	assertReads(t, provider, fixture, 1, 1, 1, 1, 0)
}

// --- R2-B: request/API version-gate rows -------------------------------------

// E10.5: real TiDB discovery → vendor boundary, not the version gate.
func TestT26SRSR2TiDBVendorBoundary(t *testing.T) {
	result, err := AuditSQL(context.Background(), Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM 4326", Dialect: spec.DialectTiDB,
	})
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("err=%v want ErrUnsupportedStatement", err)
	}
	if len(result.Unsupported) != 1 ||
		result.Unsupported[0].Reason != spec.UnsupportedVendorBoundaryReason ||
		result.Unsupported[0].Feature != "drop_spatial_reference_system" ||
		result.Unsupported[0].Metadata["boundary"] != "vendor" {
		t.Fatalf("unsupported=%+v want the vendor boundary", result.Unsupported)
	}
}

// E10.6: malformed pin takes the typed input exit before any boundary work.
func TestT26SRSR2MalformedPin(t *testing.T) {
	result, err := AuditSQL(context.Background(), Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM 100001", Dialect: spec.DialectMySQL, TargetVersion: "8.0.x",
	})
	if !errors.Is(err, spec.ErrInvalidTargetVersion) {
		t.Fatalf("err=%v want ErrInvalidTargetVersion", err)
	}
	if len(result.Statements) != 0 {
		t.Fatalf("result=%+v want zero statements on the typed exit", result.Statements)
	}
}

// E10.7/E10.8: discovery-observed product/version mismatches propagate —
// the statement never reaches the SRS seams.
func TestT26SRSR2DiscoveryMismatch(t *testing.T) {
	// E10.7: the provider's observed version disagrees with the pin.
	fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{version: "5.7.44"}, nil)
	result, err := AuditSQL(context.Background(), Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM 100001", Dialect: spec.DialectMySQL,
		TargetVersion: "8.0.36", MetadataProvider: provider,
	})
	if !errors.Is(err, ErrTargetVersionMismatch) {
		t.Fatalf("err=%v want ErrTargetVersionMismatch", err)
	}
	if len(result.Statements) != 0 {
		t.Fatalf("statements=%d want 0", len(result.Statements))
	}
	if len(provider.forwards) != 0 || srsQueryCount(fixture, "st_spatial_reference_systems") != 0 {
		t.Fatal("version mismatch must stop before any SRS seam")
	}
}

// --- R2-C.6: E3.2 request-layer zero-row + empty-incomplete -------------------

func TestT26SRSR2RequestZeroRow(t *testing.T) {
	fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{version: "8.0.36"}, nil)
	result, err := AuditSQL(context.Background(), Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM 4326", Dialect: spec.DialectMySQL,
		TargetVersion: "8.0.36", MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	stmt := result.Statements[0]
	if len(stmt.EvidenceGaps) != 1 || stmt.EvidenceGaps[0].ReasonCode != "unknown_srs_state" {
		t.Fatalf("gaps=%+v want one unknown_srs_state", stmt.EvidenceGaps)
	}
	want := []string{spec.SRSFactExistence, spec.SRSFactColumnDependencies, spec.SRSFactDropPrivilege}
	for i, f := range want {
		if stmt.EvidenceGaps[0].RequiredFacts[i] != f {
			t.Fatalf("required_facts=%v want ordered %v", stmt.EvidenceGaps[0].RequiredFacts, want)
		}
	}
	if stmt.Coverage.Status != report.CoverageUnverified || result.Verdict != report.VerdictReview {
		t.Fatalf("coverage=%v verdict=%v", stmt.Coverage.Status, result.Verdict)
	}
	assertReads(t, provider, fixture, 1, 1, 1, 1, 0)
}

// --- R2-E: E11 full seam accounting + bounded message --------------------------

func TestT26SRSR2ErrorSeamAccounting(t *testing.T) {
	// E11.1: resolver-forward failure — the message is the bounded
	// public text, the sentinel stays wrapped for errors.Is but never
	// surfaces; one R attempt, zero catalog seams, no adjudication.
	sentinel := errors.New("srs-sentinel-r2-1")
	fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{version: "8.0.36"}, nil)
	provider.resolveErr = sentinel
	state := newSRSState(provider)
	statements, failures := parseT26(t, "DROP SPATIAL REFERENCE SYSTEM 4326")
	_, err := state.enrichStatements(context.Background(),
		&MetadataRequest{Schema: "golden", Provider: provider},
		statements, failures, state.instanceFacts, auditLimits{orderedStatements: defaultOrderedStatementLimit})
	if err == nil || !errors.Is(err, sentinel) {
		t.Fatalf("err=%v want errors.Is(sentinel)", err)
	}
	if err.Error() != "metadata object read failed" {
		t.Fatalf("public text=%q want the bounded message — sentinel must not leak", err.Error())
	}
	assertReads(t, provider, fixture, 1, 0, 0, 0, 0) // R=1 attempt, zero later

	// E11.3: existence row answers, deps query fails — the existence
	// attempt counted, deps counted as attempt, error propagates.
	sentinelD := errors.New("srs-deps-r2-3")
	fixture, provider = newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{4326: "EPSG 4326"},
		failOn:  map[string]error{"st_geometry_columns": sentinelD},
	}, nil)
	state = newSRSState(provider)
	_, err = state.enrichStatements(context.Background(),
		&MetadataRequest{Schema: "golden", Provider: provider},
		statements, failures, state.instanceFacts, auditLimits{orderedStatements: defaultOrderedStatementLimit})
	if err == nil || !errors.Is(err, sentinelD) {
		t.Fatalf("err=%v want errors.Is(deps sentinel)", err)
	}
	if err.Error() != "metadata object read failed" {
		t.Fatalf("deps public text=%q", err.Error())
	}
	// R=1, Qexist=1 (answered), Qdeps=1 (failed attempt), Qo=1.
	assertReads(t, provider, fixture, 1, 1, 1, 1, 0)
	// Failed bind must not attach an adjudication arm or mint flags —
	// enrichment aborted before own apply.
	if len(state.srsExistenceStale) != 0 {
		t.Fatalf("failed bind minted stale: %+v", state.srsExistenceStale)
	}

	// E11.2: the existence query itself fails after the loader's
	// version read — deps never issued; own bind terminates.
	sentinelE := errors.New("srs-exist-r2-2")
	fixture, provider = newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36",
		failOn:  map[string]error{"st_spatial_reference_systems": sentinelE},
	}, nil)
	state = newSRSState(provider)
	var bindObserved, applyObserved bool
	state.afterBindObserve = func(int, *spec.Statement) { bindObserved = true }
	state.afterApplyObserve = func(int, *spec.Statement) { applyObserved = true }
	_, err = state.enrichStatements(context.Background(),
		&MetadataRequest{Schema: "golden", Provider: provider},
		statements, failures, state.instanceFacts, auditLimits{orderedStatements: defaultOrderedStatementLimit})
	if err == nil || !errors.Is(err, sentinelE) {
		t.Fatalf("err=%v want errors.Is(exist sentinel)", err)
	}
	if err.Error() != "metadata object read failed" {
		t.Fatalf("exist public text=%q", err.Error())
	}
	if bindObserved || applyObserved {
		t.Fatal("failed own bind must not reach the bind/apply observe seams — adjudication never ran")
	}
	assertReads(t, provider, fixture, 1, 1, 0, 1, 0) // exist attempt counted, deps never issued

	// E11.4: the loader's own select version() fails before any
	// catalog read — the R attempt happened, zero downstream reads.
	sentinelV := errors.New("srs-ver-r2-4")
	fixture, provider = newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36",
		failOn:  map[string]error{"version()": sentinelV},
	}, nil)
	state = newSRSState(provider)
	_, err = state.enrichStatements(context.Background(),
		&MetadataRequest{Schema: "golden", Provider: provider},
		statements, failures, state.instanceFacts, auditLimits{orderedStatements: defaultOrderedStatementLimit})
	if err == nil || !errors.Is(err, sentinelV) {
		t.Fatalf("err=%v want errors.Is(version sentinel)", err)
	}
	if err.Error() != "metadata object read failed" {
		t.Fatalf("version public text=%q", err.Error())
	}
	assertReads(t, provider, fixture, 1, 0, 0, 1, 0) // Qo attempt counted, nothing downstream
	if len(state.srsExistenceStale) != 0 || state.srsDepsStale {
		t.Fatal("failed bind minted SRS flags")
	}
}

// E11.x sibling isolation: a mid-sequence bind failure leaves already-
// applied sibling state untouched and never reaches later statements.
func TestT26SRSR2ErrorSiblingIsolation(t *testing.T) {
	sentinel := errors.New("srs-deps-r2-sib")
	_, provider := newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{4326: "EPSG 4326"},
		deps:    []srsDepRow{},
		failOn:  map[string]error{"st_geometry_columns": sentinel},
	}, tblTFixture())
	state := newSRSState(provider)
	var afterFirstApply map[string]any
	state.afterApplyObserve = func(i int, _ *spec.Statement) {
		if i == 0 {
			afterFirstApply = srsChannels(t, state)
		}
	}
	parsed, _ := parseSQL(context.Background(),
		"DROP TABLE s.t_new; DROP SPATIAL REFERENCE SYSTEM 4326; DROP TABLE s.t", spec.DialectMySQL)
	stmts, err := Extract(context.Background(), parsed)
	if err != nil || len(stmts) != 3 {
		t.Fatalf("extract: %v %d", err, len(stmts))
	}
	_, err = state.enrichStatements(context.Background(),
		&MetadataRequest{Schema: "golden", Provider: provider},
		stmts, parsed.failures, state.instanceFacts, auditLimits{orderedStatements: defaultOrderedStatementLimit})
	if err == nil || !errors.Is(err, sentinel) {
		t.Fatalf("err=%v want errors.Is(sentinel)", err)
	}
	// Statement 1's own apply already committed its legitimate write
	// (depsStale minted by the touching DROP TABLE); statement 3 never
	// ran — the failure terminated the loop. The post-error state must
	// be value-identical to the post-statement-1 state: the failed bind
	// wrote nothing.
	after := srsChannels(t, state)
	if !reflect.DeepEqual(after, afterFirstApply) {
		t.Fatalf("failed bind mutated state: after=%+v want %+v", after, afterFirstApply)
	}
	if len(state.srsExistenceStale) != 0 {
		t.Fatalf("failed bind minted existence stale: %+v", state.srsExistenceStale)
	}
	// The error was not laundered into an evidence gap: no statement
	// carries an adjudication.
	for _, s := range stmts {
		if s.Metadata != nil && s.Metadata.SRSDrop != nil {
			t.Fatalf("statement %q carries an adjudication despite the failed bind", s.RawSQL)
		}
	}
}

// E11.5: the LoadInstanceFacts sentinel — request dies before the
// enrichment loop; zero-value result, Vd counted, all SRS seams zero.
func TestT26SRSR2DiscoverySentinel(t *testing.T) {
	sentinel := errors.New("srs-discover-r2-5")
	fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{version: "8.0.36"}, nil)
	provider.discoverErr = sentinel
	result, err := AuditSQL(context.Background(), Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM 4326", Dialect: spec.DialectMySQL,
		MetadataProvider: provider,
	})
	if err == nil || !errors.Is(err, sentinel) {
		t.Fatalf("err=%v want errors.Is(discovery sentinel)", err)
	}
	if !strings.Contains(err.Error(), "load instance facts") {
		t.Fatalf("wrapper text %q must keep the load-instance-facts prefix", err.Error())
	}
	if len(result.Statements) != 0 || len(result.Unsupported) != 0 {
		t.Fatalf("result=%+v want a complete zero value", result)
	}
	if len(provider.forwards) != 0 {
		t.Fatalf("SRS seams touched: forwards=%+v", provider.forwards)
	}
	if srsQueryCount(fixture, "version()") != 0 {
		t.Fatal("Qo fired before enrichment — discovery precedes everything")
	}
}

// --- R2-D: E12 deep-value snapshots ------------------------------------------

// srsStaleSet deep-copies the batch's per-SRID existence-stale key set
// so before/after captures compare complete sets by value (comma-ok —
// never a bare map-membership probe).
func srsStaleSet(state *batchState) map[uint32]struct{} {
	set := map[uint32]struct{}{}
	for srid, v := range state.srsExistenceStale {
		if _, ok := state.srsExistenceStale[srid]; !ok {
			panic("unreachable")
		}
		set[srid] = v
	}
	return set
}

// srsChannels captures the full deep value of every sibling channel the
// SRS own apply could conceivably touch — the table/schema registry,
// the SERVER channel, and both SRS flags.
func srsChannels(t *testing.T, state *batchState) map[string]any {
	t.Helper()
	return map[string]any{
		"table":  snapshotTableSchemaState(t, state),
		"server": snapshotServerChannel(state),
		"deps":   state.srsDepsStale,
		"stale":  srsStaleSet(state),
	}
}

// E12.3: frozen initial state — a tracked s.t snapshot carrying the
// geometry column, a SERVER OrderedPresent entry, clean SRS flags;
// the SRS bind gets an existence row and empty dependencies → a real
// S2 proven by facts, owed exactly [D,P]. Independent deep snapshots
// taken at the bind and apply observe seams prove the own apply's only
// write is existenceStale ∪ {4326}.
func TestT26SRSR2UnprovenOwnApplySnapshot(t *testing.T) {
	_, provider := newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{4326: "EPSG 4326"},
	}, nil)
	state := newSRSState(provider)
	// Seed the frozen non-empty initial state: tracked s.t table
	// snapshot with g:geometry and a SERVER OrderedPresent entry.
	key := batchTableKey{dialect: spec.DialectMySQL, schema: "s", table: "t"}
	state.entries[key] = &batchTableEntry{
		state: tablePresent,
		shape: &spec.TableSnapshot{
			Schema: "s", Table: &spec.Table{Schema: "s", Name: "t"}, Exists: true,
			Columns: []spec.Column{{Name: "g", Type: "geometry"}},
		},
		displaySchema: "s", displayTable: "t", notView: true,
	}
	state.serverEntryFor("s1").present = true // OrderedPresent
	state.srsDepsStale = false

	before := srsChannels(t, state)
	var atBind, atApply map[string]any
	state.afterBindObserve = func(_ int, _ *spec.Statement) { atBind = srsChannels(t, state) }
	state.afterApplyObserve = func(_ int, _ *spec.Statement) { atApply = srsChannels(t, state) }
	enriched := enrichSRS(t, state, provider, "DROP SPATIAL REFERENCE SYSTEM 4326")
	if arm := srsArm(t, enriched[0]); arm != spec.SRSDropArmUnproven {
		t.Fatalf("arm=%v want unproven", arm)
	}
	wantOwed := []string{spec.SRSFactColumnDependencies, spec.SRSFactDropPrivilege}
	if got := srsOwed(enriched[0]); !reflect.DeepEqual(got, wantOwed) {
		t.Fatalf("owed=%v want exactly %v — existence row discharges E, empty deps stays unproven", got, wantOwed)
	}
	if atBind == nil || atApply == nil {
		t.Fatal("observe hooks did not fire — bind/apply witnesses missing")
	}
	// The bind performs reads only: every channel value-identical.
	if !reflect.DeepEqual(atBind, before) {
		t.Fatalf("own bind mutated state: before=%+v atBind=%+v", before, atBind)
	}
	// The own apply writes exactly one key and nothing else.
	if !reflect.DeepEqual(atApply["table"], before["table"]) {
		t.Fatal("S2 own apply touched the table/schema channel")
	}
	if !reflect.DeepEqual(atApply["server"], before["server"]) {
		t.Fatal("S2 own apply touched the SERVER channel")
	}
	if atApply["deps"] != false {
		t.Fatal("S2 own apply must not mint dependency staleness")
	}
	if _, ok := state.srsExistenceStale[4326]; !ok {
		t.Fatalf("comma-ok: existenceStale missing 4326: %+v", state.srsExistenceStale)
	}
	if !reflect.DeepEqual(atApply["stale"], map[uint32]struct{}{4326: {}}) {
		t.Fatalf("existenceStale=%+v want exactly {4326} — prior keys ∪ {4326}", atApply["stale"])
	}
}

// E12.4: two own applies observed separately — the table statement's
// own apply mints srsDepsStale (false→true), leaves existenceStale and
// the SERVER channel value-identical, and writes its legitimate table
// change; the SRS S2's own apply then adds only {4326}.
func TestT26SRSR2TouchingApplyThenSRSMint(t *testing.T) {
	_, provider := newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{4326: "EPSG 4326"},
	}, tblTFixture())
	state := newSRSState(provider)
	before := srsChannels(t, state)
	var afterTableApply, afterSRSApply map[string]any
	state.afterApplyObserve = func(i int, _ *spec.Statement) {
		switch i {
		case 0:
			afterTableApply = srsChannels(t, state)
		case 1:
			afterSRSApply = srsChannels(t, state)
		}
	}
	enriched := enrichSRS(t, state, provider,
		"DROP TABLE s.t; DROP SPATIAL REFERENCE SYSTEM 4326")
	if afterTableApply == nil || afterSRSApply == nil {
		t.Fatal("apply observe hooks did not fire for both statements")
	}
	// Table statement's own apply: depsStale false→true; existenceStale
	// and SERVER channel exactly unchanged; the s.t entry legitimately
	// changed (asserted separately, not folded into the flag check).
	if afterTableApply["deps"] != true {
		t.Fatal("table apply must mint srsDepsStale false→true")
	}
	if !reflect.DeepEqual(afterTableApply["stale"], before["stale"]) {
		t.Fatalf("table apply touched existenceStale: %+v want %+v", afterTableApply["stale"], before["stale"])
	}
	if !reflect.DeepEqual(afterTableApply["server"], before["server"]) {
		t.Fatal("table apply must not touch the SERVER channel")
	}
	key := batchTableKey{dialect: spec.DialectMySQL, schema: "s", table: "t"}
	entry, ok := state.entries[key]
	if !ok {
		t.Fatalf("table apply produced no s.t entry: %+v", state.entries)
	}
	if entry.state == tablePresent {
		t.Fatal("DROP TABLE must write its own table state — the legitimate change")
	}
	// SRS S2's own apply: adds only {4326}; depsStale, table/schema and
	// SERVER all identical to the post-table-apply point.
	if arm := srsArm(t, enriched[1]); arm != spec.SRSDropArmUnproven {
		t.Fatalf("stmt2 arm=%v want unproven — stale deps witness demotes, existence consumed", arm)
	}
	if got := srsOwed(enriched[1]); !reflect.DeepEqual(got, []string{spec.SRSFactColumnDependencies, spec.SRSFactDropPrivilege}) {
		t.Fatalf("owed=%v want [D,P]", got)
	}
	if _, ok := state.srsExistenceStale[4326]; !ok {
		t.Fatalf("comma-ok: existenceStale missing 4326: %+v", state.srsExistenceStale)
	}
	if !reflect.DeepEqual(afterSRSApply["stale"], map[uint32]struct{}{4326: {}}) {
		t.Fatalf("existenceStale=%+v want exactly {4326}", afterSRSApply["stale"])
	}
	if afterSRSApply["deps"] != true {
		t.Fatal("SRS own apply must leave depsStale as the table write left it")
	}
	if !reflect.DeepEqual(afterSRSApply["table"], afterTableApply["table"]) {
		t.Fatal("SRS own apply touched the table/schema channel")
	}
	if !reflect.DeepEqual(afterSRSApply["server"], afterTableApply["server"]) {
		t.Fatal("SRS own apply touched the SERVER channel")
	}
}

// B-arm own apply is a no-op — both blocked sources compared by full
// deep state: whole-batch contamination and a pre-existing same-SRID
// existence-stale flag.
func TestT26SRSR2BlockedOwnApplyNoop(t *testing.T) {
	// Source 1: whole-batch contamination (tail-sweep seeded).
	_, provider := newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{4326: "EPSG 4326"},
	}, nil)
	state := newSRSState(provider)
	state.contaminated = true
	before := srsChannels(t, state)
	var atApply map[string]any
	state.afterApplyObserve = func(_ int, _ *spec.Statement) { atApply = srsChannels(t, state) }
	enriched := enrichSRS(t, state, provider, "DROP SPATIAL REFERENCE SYSTEM 4326")
	if arm := srsArm(t, enriched[0]); arm != spec.SRSDropArmBlocked {
		t.Fatalf("contamination arm=%v want blocked", arm)
	}
	if got := srsOwed(enriched[0]); !reflect.DeepEqual(got, []string{spec.SRSFactBatchEffects}) {
		t.Fatalf("contamination owed=%v want [B]", got)
	}
	if !reflect.DeepEqual(atApply, before) {
		t.Fatalf("blocked own apply must write nothing: before=%+v after=%+v", before, atApply)
	}

	// Source 2: pre-existing same-SRID existence stale.
	_, provider = newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{4326: "EPSG 4326"},
	}, nil)
	state = newSRSState(provider)
	state.srsExistenceStale[4326] = struct{}{}
	before = srsChannels(t, state)
	atApply = nil
	state.afterApplyObserve = func(_ int, _ *spec.Statement) { atApply = srsChannels(t, state) }
	enriched = enrichSRS(t, state, provider, "DROP SPATIAL REFERENCE SYSTEM 4326")
	if arm := srsArm(t, enriched[0]); arm != spec.SRSDropArmBlocked {
		t.Fatalf("stale arm=%v want blocked", arm)
	}
	if got := srsOwed(enriched[0]); !reflect.DeepEqual(got, []string{spec.SRSFactBatchEffects}) {
		t.Fatalf("stale owed=%v want [B]", got)
	}
	if !reflect.DeepEqual(atApply, before) {
		t.Fatalf("blocked own apply must write nothing: before=%+v after=%+v", before, atApply)
	}
}

// E12.2: serialized output never carries catalog identities — full
// scan over the JSON-encoded result for the injected secrets.
func TestT26SRSR2SerializationScan(t *testing.T) {
	_, provider := newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{4326: "EPSG 4326"},
		deps: []srsDepRow{
			{schema: "secret_schema", table: "secret_tbl", column: "secret_col", srid: 4326},
			{schema: "secret_schema2", table: "secret_tbl2", column: "secret_col2", srid: 4326},
		},
	}, nil)
	enriched := enrichSRS(t, newSRSState(provider), provider, "DROP SPATIAL REFERENCE SYSTEM 4326")
	if arm := srsArm(t, enriched[0]); arm != spec.SRSDropArmRefusedDependency {
		t.Fatalf("arm=%v want refused_dependency — the witness set drives the count", arm)
	}
	result := evaluateEnriched(t, enriched)
	blob, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, secret := range []string{"secret_schema", "secret_tbl", "secret_col"} {
		if strings.Contains(string(blob), secret) {
			t.Fatalf("serialized result leaks %q", secret)
		}
	}
	f := result.Statements[0].Findings[0]
	if f.Metadata["dependency_witness_count"] != 2 {
		t.Fatalf("count=%v want 2 (the privilege-filtered visible set)", f.Metadata["dependency_witness_count"])
	}
	if _, bad := f.Metadata["conditional"]; bad {
		t.Fatal("conditional marker must stay absent from this slice's projection")
	}
}

// E12.5: the full request path under observed_account_new_session —
// the privilege loader fires once for the request, the SERVER drop's
// premise is consumable, and the following SRS drop still lands S2
// with its own privilege fact owed. The SRS statement never re-triggers
// the loader (L stays 1 at the request seam) and inherits nothing.
func TestT26SRSR2ServerPrivilegeIsolation(t *testing.T) {
	fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{
		version: "8.0.36",
		srs:     map[uint32]string{4326: "EPSG 4326"},
		servers: map[string]bool{"s1": true},
	}, nil)
	provider.privObservation = &spec.MySQLServerDropPrivilegeObservation{
		Account:    "auditor@localhost",
		GrantLines: []string{"GRANT SUPER ON *.* TO 'auditor'@'localhost'"},
	}
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "DROP SERVER s1; DROP SPATIAL REFERENCE SYSTEM 4326",
		Dialect: spec.DialectMySQL, TargetVersion: "8.0.36",
		MetadataProvider:   provider,
		PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 2 {
		t.Fatalf("statements=%d want 2", len(result.Statements))
	}
	// Request L=1: the loader ran once and the SERVER premise is consumable.
	if provider.privCalls != 1 {
		t.Fatalf("L=%d want 1 — the request-level loader fired once", provider.privCalls)
	}
	srv := result.Statements[0]
	for _, gap := range srv.EvidenceGaps {
		for _, fact := range gap.RequiredFacts {
			if fact == spec.ServerFactDropPrivilege {
				t.Fatalf("SERVER premise must be consumable: gaps=%+v", srv.EvidenceGaps)
			}
		}
	}
	srsStmt := result.Statements[1]
	if len(srsStmt.EvidenceGaps) != 1 || srsStmt.EvidenceGaps[0].ReasonCode != "unknown_srs_state" {
		t.Fatalf("SRS gaps=%+v want one unknown_srs_state (S2)", srsStmt.EvidenceGaps)
	}
	want := []string{spec.SRSFactColumnDependencies, spec.SRSFactDropPrivilege}
	if len(srsStmt.EvidenceGaps[0].RequiredFacts) != 2 {
		t.Fatalf("required_facts=%v want [D,P]", srsStmt.EvidenceGaps[0].RequiredFacts)
	}
	for i, f := range want {
		if srsStmt.EvidenceGaps[0].RequiredFacts[i] != f {
			t.Fatalf("required_facts=%v want ordered [D,P] — SRS keeps its own privilege debt", srsStmt.EvidenceGaps[0].RequiredFacts)
		}
	}
	// 4326 is reserved — the advisory co-fires on the unproven arm.
	if len(srsStmt.Findings) != 1 || srsStmt.Findings[0].RuleID != "ddl.mysql.srs.drop.reserved" {
		t.Fatalf("SRS findings=%+v want only the reserved notice", srsStmt.Findings)
	}
	assertReservedAdvisoryText(t, srsStmt.Findings[0])
	// No conditional marker may leak into the SRS projection.
	blob, err := json.Marshal(srsStmt)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(blob), "conditional") || strings.Contains(string(blob), "auditor@localhost") || strings.Contains(string(blob), "SUPER") {
		t.Fatalf("SRS statement leaks the SERVER privilege evidence: %s", blob)
	}
	assertReads(t, provider, fixture, 1, 1, 1, 1, 1) // R counts SRS forwards only; the server forward is a separate fact family's seam — L=1 once per request
}

var _ = rule.LevelBlocker // keep rule import live for finders

// --- R2-C.9: D3 discriminating inputs — the frozen predicate edges -----------

// Table-driven at the model layer; inputs no legal SQL can produce are
// model-unit cells, not parser-integration witnesses.
func TestT26SRSR2D3Discriminating(t *testing.T) {
	state := newSRSState(nil)
	cells := []struct {
		name     string
		build    func() *spec.DDL
		touching bool
	}{
		// Inert op wins over a non-empty TableTargets (checked first).
		{"inert_op_beats_targets", func() *spec.DDL {
			return &spec.DDL{Operation: spec.DDLOperationCreateIndex, Table: &spec.Table{Name: "t"}}
		}, false},
		// Refer-table discriminators.
		{"has_refer", func() *spec.DDL {
			return &spec.DDL{Operation: spec.DDLOperationCreateTable, HasReferTable: true,
				ReferTable: &spec.Table{Name: "t"}, Columns: []spec.Column{{Name: "i", Type: "int"}}}
		}, true},
		{"refer_no_flag", func() *spec.DDL {
			return &spec.DDL{Operation: spec.DDLOperationCreateTable,
				ReferTable: &spec.Table{Name: "t"}, Columns: []spec.Column{{Name: "i", Type: "int"}}}
		}, true},
		// Select discriminators.
		{"has_select", func() *spec.DDL {
			return &spec.DDL{Operation: spec.DDLOperationCreateTable, HasSelect: true,
				SelectSources: []spec.Table{{Name: "src"}}, Columns: []spec.Column{{Name: "i", Type: "int"}}}
		}, true},
		{"sources_no_flag", func() *spec.DDL {
			return &spec.DDL{Operation: spec.DDLOperationCreateTable,
				SelectSources: []spec.Table{{Name: "src"}}, Columns: []spec.Column{{Name: "i", Type: "int"}}}
		}, true},
		// Column-list completeness.
		{"empty_columns", func() *spec.DDL {
			return &spec.DDL{Operation: spec.DDLOperationCreateTable}
		}, true},
		{"complete_columns", func() *spec.DDL {
			return &spec.DDL{Operation: spec.DDLOperationCreateTable,
				Columns: []spec.Column{{Name: "i", Type: "int"}, {Name: "v", Type: "varchar"}}}
		}, false},
		{"column_unextracted", func() *spec.DDL {
			return &spec.DDL{Operation: spec.DDLOperationCreateTable,
				Columns: []spec.Column{{Name: "i", Type: "int", UnextractedOptions: []string{"auto"}}}}
		}, true},
		{"ddl_unextracted", func() *spec.DDL {
			return &spec.DDL{Operation: spec.DDLOperationCreateTable,
				Columns: []spec.Column{{Name: "i", Type: "int"}}, UnextractedOptions: []string{"row_format"}}
		}, true},
		{"spatial_family", func() *spec.DDL {
			return &spec.DDL{Operation: spec.DDLOperationCreateTable,
				Columns: []spec.Column{{Name: "g", Type: "geometry"}}}
		}, true},
		// ALTER: action vs payload are distinct exclusion axes.
		{"alter_engine_only", func() *spec.DDL {
			return &spec.DDL{Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Name: "t"},
				Alter: []spec.Alter{{Action: "table_option", Options: map[string]string{"engine": "InnoDB"}}}}
		}, false},
		{"alter_engine_plus", func() *spec.DDL {
			return &spec.DDL{Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Name: "t"},
				Alter: []spec.Alter{{Action: "table_option", Options: map[string]string{"engine": "x", "comment": "c"}}}}
		}, true},
		{"alter_unextracted_payload", func() *spec.DDL {
			return &spec.DDL{Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Name: "t"},
				Alter: []spec.Alter{{Action: "table_option", Options: map[string]string{"engine": "x"}, UnextractedOptions: []string{"compression"}}}}
		}, true},
		{"alter_unknown_action", func() *spec.DDL {
			return &spec.DDL{Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Name: "t"},
				Alter: []spec.Alter{{Action: "add_columns"}}}
		}, true},
		// Multi-member lists: a clean head never launders a dirty tail.
		{"alter_clean_head_dirty_tail", func() *spec.DDL {
			return &spec.DDL{Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Name: "t"},
				Alter: []spec.Alter{
					{Action: "table_option", Options: map[string]string{"engine": "x"}},
					{Action: "add_columns"},
				}}
		}, true},
		{"alter_two_clean", func() *spec.DDL {
			return &spec.DDL{Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Name: "t"},
				Alter: []spec.Alter{
					{Action: "table_option", Options: map[string]string{"engine": "x"}},
					{Action: "table_option", Options: map[string]string{"engine": "y"}},
				}}
		}, false},
	}
	for _, cell := range cells {
		stmt := spec.Statement{Dialect: spec.DialectMySQL, Kind: spec.KindDDL, DDL: cell.build()}
		if got := state.spatialTouching(stmt); got != cell.touching {
			t.Fatalf("%s: touching=%v want %v", cell.name, got, cell.touching)
		}
	}
}

// --- R2-C.7: E9.2 — the authored coordinates are the witness ------------------

func TestT26SRSR2AuthoredCoordinates(t *testing.T) {
	parsed, _ := parseSQL(context.Background(), "GARBAGE; DROP SPATIAL REFERENCE SYSTEM 4326", spec.DialectMySQL)
	if len(parsed.failures) != 1 || len(parsed.Statements) != 1 {
		t.Fatalf("failures=%d statements=%d want 1/1", len(parsed.failures), len(parsed.Statements))
	}
	failure := parsed.failures[0]
	drop := parsed.Statements[0]
	if failure.Line != 1 || failure.Column != 1 {
		t.Fatalf("failure at (%d,%d) want (1,1)", failure.Line, failure.Column)
	}
	if drop.Line != 1 || drop.Column != 10 {
		t.Fatalf("DROP at (%d,%d) want (1,10)", drop.Line, drop.Column)
	}
	if !failureBefore(failure.Line, failure.Column, drop.Line, drop.Column) {
		t.Fatal("failure (1,1) must precede DROP (1,10) — authored ordering governs")
	}
}

// --- R2-B.4: E10.3d — the inclusive floor fires the full audited projection ---

func TestT26SRSR2FloorFullProjection(t *testing.T) {
	fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{version: "8.0.4"}, nil)
	result, err := AuditSQL(context.Background(), Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM 4326", Dialect: spec.DialectMySQL,
		TargetVersion: "8.0.4", MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("8.0.4 audit: %v", err)
	}
	stmt := result.Statements[0]
	if len(stmt.EvidenceGaps) != 1 || len(stmt.EvidenceGaps[0].RequiredFacts) != 3 {
		t.Fatalf("gaps=%+v want one gap [E,D,P]", stmt.EvidenceGaps)
	}
	// 4326 is reserved — the notice must co-fire on the unproven arm.
	if len(stmt.Findings) != 1 || stmt.Findings[0].RuleID != "ddl.mysql.srs.drop.reserved" {
		t.Fatalf("findings=%+v want the reserved notice at the inclusive floor", stmt.Findings)
	}
	assertReads(t, provider, fixture, 1, 1, 1, 1, 0)
}

// E10.8: a TiDB-product observation under the MySQL dialect path is a
// product mismatch — the statement dies before the SRS seams.
func TestT26SRSR2ProductMismatch(t *testing.T) {
	fixture, provider := newSRSSQLHarness(t, &srsSQLFixture{version: "5.7.44-TiDB-v8.5.0"}, nil)
	result, err := AuditSQL(context.Background(), Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM 100001", Dialect: spec.DialectMySQL,
		TargetVersion: "8.0.36", MetadataProvider: provider,
	})
	if !errors.Is(err, ErrDialectProductMismatch) {
		t.Fatalf("err=%v want ErrDialectProductMismatch on the product banner", err)
	}
	if len(result.Statements) != 0 {
		t.Fatalf("statements=%d want 0", len(result.Statements))
	}
	if len(provider.forwards) != 0 || srsQueryCount(fixture, "st_spatial_reference_systems") != 0 {
		t.Fatal("product mismatch must stop before any SRS seam")
	}
}
