package audit_test

// T26-SRS-VERSION-APPLICABILITY-01 EXECUTION-01 — the frozen V1 contract
// test matrix: one request-level version fact controls BOTH the audited
// domain and every SRS catalog read. Each cell asserts the boundary
// shape AND the zero-read witness; the registered 8.0.4+ domain keeps
// the full path. Provider SQL, grammar, and adjudication semantics are
// untouched.

import (
	"context"
	"errors"
	"testing"

	appaudit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// vgMarker asserts the single unsupported marker's metadata: the frozen
// gate:"version" shape plus the V11 reason subdivision and the named
// srs.version_applicability gap fact (V4).
func vgMarker(t *testing.T, unsupported []spec.UnsupportedDetail, wantReason string) {
	t.Helper()
	if len(unsupported) != 1 {
		t.Fatalf("unsupported=%+v want exactly the SRS version-boundary marker", unsupported)
	}
	entry := unsupported[0]
	if entry.Reason != spec.UnsupportedUnauditedReason ||
		entry.Metadata["boundary"] != "unaudited" ||
		entry.Metadata["object"] != "spatial_reference_system" ||
		entry.Metadata["gate"] != "version" {
		t.Fatalf("unsupported=%+v want unaudited/spatial_reference_system/version boundary", entry)
	}
	if entry.Metadata["version_reason"] != wantReason {
		t.Fatalf("version_reason=%v want %q", entry.Metadata["version_reason"], wantReason)
	}
	if entry.Metadata["gap_fact"] != spec.SRSFactVersionApplicability {
		t.Fatalf("gap_fact=%v want %q", entry.Metadata["gap_fact"], spec.SRSFactVersionApplicability)
	}
}

const vgSQL = "DROP SPATIAL REFERENCE SYSTEM 4326;"

// Cases 1–10: proven-version cells and unproven cells — every
// out-of-domain cell must show boundary + zero catalog reads + the
// correct reason; the two in-domain cells keep the full read path.
func TestT26SRSVersionGateMatrix(t *testing.T) {
	cases := []struct {
		name       string
		banner     string // O1 request-level discovery banner
		loaderVer  string // O2 provider-internal select version() banner
		wantReason string // "" means in-domain (full read path)
	}{
		{name: "mysql57", banner: "5.7.44", loaderVer: "5.7.44", wantReason: "version_inapplicable"},
		{name: "mysql800", banner: "8.0.0", loaderVer: "8.0.0", wantReason: "version_inapplicable"},
		{name: "mysql802", banner: "8.0.2", loaderVer: "8.0.2", wantReason: "version_inapplicable"},
		{name: "mysql803", banner: "8.0.3", loaderVer: "8.0.3", wantReason: "version_inapplicable"},
		{name: "mysql804", banner: "8.0.4", loaderVer: "8.0.4"},
		{name: "mysql8046", banner: "8.0.46", loaderVer: "8.0.46"},
		{name: "mysql8410", banner: "8.4.10", loaderVer: "8.4.10", wantReason: "version_inapplicable"},
		{name: "unparseable-banner", banner: "unknown-banner", loaderVer: "8.0.46", wantReason: "version_observation_error"},
		{name: "out-of-series", banner: "9.1.0", loaderVer: "9.1.0", wantReason: "version_out_of_series"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx := &r3Fixture{version: tc.loaderVer, discoverVer: tc.banner}
			p := r3Harness(t, fx)
			result, err := appaudit.AuditSQL(context.Background(), appaudit.Request{
				SQL: vgSQL, Dialect: spec.DialectMySQL, MetadataProvider: p,
			})
			if tc.wantReason == "" {
				// In-domain: the registered read path runs (R=1,
				// exist+deps, Qo=1 loader-internal check, Vd=1).
				if err != nil {
					t.Fatalf("in-domain audit must not error: %v", err)
				}
				r3AssertReads(t, p, fx, 1, 1, 1, 1, 0, 1)
				return
			}
			r3Boundary(t, result, err, "spatial_reference_system")
			vgMarker(t, result.Unsupported, tc.wantReason)
			// Frozen V5–V8: zero catalog reads — resolver, exist,
			// deps, and even the loader-internal version check.
			r3AssertReads(t, p, fx, 0, 0, 0, 0, 0, 1)
		})
	}
}

// Case 8: version nil — providerless offline audit, no pin. The
// boundary marker carries version_unproven and no catalog channel ever
// existed.
func TestT26SRSVersionGateUnprovenProviderless(t *testing.T) {
	result, err := appaudit.AuditSQL(context.Background(), appaudit.Request{
		SQL: vgSQL, Dialect: spec.DialectMySQL,
	})
	r3Boundary(t, result, err, "spatial_reference_system")
	vgMarker(t, result.Unsupported, spec.MetadataUnreadVersionUnproven)
}

// Case 8b (ER1): providerless WITH a valid in-domain pin — the pin
// cannot substitute for an observation (frozen V1): the statement
// takes the version boundary as unproven. No catalog read can fire
// without a provider, so zero reads holds by construction.
func TestT26SRSVersionGatePinNoProvider(t *testing.T) {
	result, err := appaudit.AuditSQL(context.Background(), appaudit.Request{
		SQL: vgSQL, Dialect: spec.DialectMySQL, TargetVersion: "8.0.46",
	})
	// ER1 regression: a caller pin is conflict-check only and can never
	// substitute for an observation — even a valid in-domain pin takes
	// the version boundary as unproven. No provider means no read seams
	// exist at all, so zero reads holds by construction.
	r3Boundary(t, result, err, "spatial_reference_system")
	vgMarker(t, result.Unsupported, spec.MetadataUnreadVersionUnproven)
}

// Case 10: observation channel itself failed — the audit surfaces the
// bounded load error and zero SRS reads fire.
func TestT26SRSVersionGateObservationError(t *testing.T) {
	fx := &r3Fixture{version: "8.0.46", discoverVer: "8.0.46"}
	p := r3Harness(t, fx)
	p.discoverErr = errors.New("simulated discovery failure")
	_, err := appaudit.AuditSQL(context.Background(), appaudit.Request{
		SQL: vgSQL, Dialect: spec.DialectMySQL, MetadataProvider: p,
	})
	if err == nil || !errors.Is(err, p.discoverErr) {
		t.Fatalf("err=%v want the preserved discovery error", err)
	}
	r3AssertReads(t, p, fx, 0, 0, 0, 0, 0, 1)
}

// Case 11: TiDB dialect — vendor boundary; the MySQL SRS path never
// enters (frozen V9). zero reads.
func TestT26SRSVersionGateTiDB(t *testing.T) {
	fx := &r3Fixture{version: "8.0.11-TiDB-v8.5.0", discoverVer: "8.0.11-TiDB-v8.5.0"}
	p := r3Harness(t, fx)
	result, err := appaudit.AuditSQL(context.Background(), appaudit.Request{
		SQL: vgSQL, Dialect: spec.DialectTiDB, MetadataProvider: p,
	})
	if err == nil {
		t.Fatal("TiDB SRS must surface an unsupported boundary")
	}
	for _, u := range result.Unsupported {
		if u.Metadata["object"] == "spatial_reference_system" && u.Metadata["gate"] == "version" {
			t.Fatalf("TiDB must be a VENDOR boundary, never a version gate: %+v", u)
		}
	}
	r3AssertReads(t, p, fx, 0, 0, 0, 0, 0, 1)
}

// Case 12: caller target_version mismatched to the observed banner —
// the typed conflict error survives unchanged (frozen V1 pin rule).
func TestT26SRSVersionGateTargetMismatch(t *testing.T) {
	fx := &r3Fixture{version: "8.0.46", discoverVer: "8.0.46"}
	p := r3Harness(t, fx)
	_, err := appaudit.AuditSQL(context.Background(), appaudit.Request{
		SQL: vgSQL, Dialect: spec.DialectMySQL, MetadataProvider: p,
		TargetVersion: "8.0.45",
	})
	if !errors.Is(err, appaudit.ErrTargetVersionMismatch) {
		t.Fatalf("err=%v want ErrTargetVersionMismatch", err)
	}
	r3AssertReads(t, p, fx, 0, 0, 0, 0, 0, 1)
}

// Cases 13–14: static findings survive under unknown version (frozen
// V3/V4-B4 — skipped catalog reads never erase proven static facts).
func TestT26SRSVersionGateStaticPreserved(t *testing.T) {
	// 13: SRID above the u32 ceiling is a static refusal — no version
	// fact is needed to prove it.
	result, err := appaudit.AuditSQL(context.Background(), appaudit.Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM 4294967296;", Dialect: spec.DialectMySQL,
	})
	if result.Unsupported == nil && err == nil {
		t.Fatal("a statically-invalid SRID must surface under no version observation")
	}
	// 14 (ER3): CREATE missing the mandatory paired attribute
	// (ORGANIZATION without IDENTIFIED BY) is a static violation the
	// parser itself reports — it must still be identified with NO
	// version observation: a preserved parser_error diagnostic, never
	// silently swallowed by the version gate.
	result2, err2 := appaudit.AuditSQL(context.Background(), appaudit.Request{
		SQL: "CREATE SPATIAL REFERENCE SYSTEM 4326 ORGANIZATION 'EPSG';", Dialect: spec.DialectMySQL,
	})
	if err2 == nil {
		t.Fatal("a missing-mandatory-attribute CREATE must surface under no version observation")
	}
	if len(result2.Diagnostics) != 1 {
		t.Fatalf("diagnostics=%+v want exactly the static parser diagnostic", result2.Diagnostics)
	}
	d := result2.Diagnostics[0]
	if d.Classification != spec.DiagnosticParserError || d.Audited {
		t.Fatalf("diagnostic=%+v want parser_error audited=false", d)
	}
	if len(result2.Unsupported) != 0 {
		t.Fatalf("unsupported=%+v want none — the static error precedes the version gate", result2.Unsupported)
	}
}

// --- D-section: own-apply / batch effects under version variance (frozen V12) ---

// vgOutOfDomain seeds a proven out-of-domain identity (8.4.10 — the
// registered domain is 8.0.4+ only).
func vgOutOfDomain(state *appaudit.TestBatchState) {
	state.SeedResolvedVersion(&spec.VersionIdentity{
		Product: spec.VersionProductMySQL, Version: "8.4.10",
		Major: 8, Minor: 4, Patch: 10, ValidatedRange: true, Source: spec.VersionSourceObserved})
}

// D1: an out-of-domain CREATE mints NO present fact — it is a boundary
// write that may have landed, so only the stale flag is honest.
func TestT26SRSVersionGateOwnApplyCreate(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.version = "8.4.10"
	fx.discoverVer = "8.4.10"
	provider, state := paRealState(t, fx)
	vgOutOfDomain(state)
	enriched := appaudit.EnrichSRSPrivForTest(t, state, provider,
		"CREATE SPATIAL REFERENCE SYSTEM 100001 NAME 'a' DEFINITION 'x';")
	if enriched[0].Unsupported == nil || enriched[0].Unsupported.Metadata["gate"] != "version" {
		t.Fatalf("must take the version boundary: %+v", enriched[0].Unsupported)
	}
	if got := appaudit.BatchStatePresentSet(state); len(got) != 0 {
		t.Fatalf("present=%v want EMPTY — boundary CREATE never mints presence", got)
	}
	paSameStale(t, appaudit.BatchStateStaleSet(state), map[uint32]bool{100001: true}, "boundary CREATE mints stale")
	r3AssertReads(t, provider, fx, 0, 0, 0, 0, 0, -1)
}

// D2: an out-of-domain DROP mints NO ordered absence entry; the stale
// mint flips an existing same-SRID entry's barrier (provenance kept).
func TestT26SRSVersionGateOwnApplyDrop(t *testing.T) {
	fx := opAbsentFixture(opGrantConsumable)
	provider, state := paRealState(t, fx)
	// First an in-domain proven-absence DROP publishes the entry.
	enriched := appaudit.EnrichSRSPrivForTest(t, state, provider,
		"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;")
	if len(state.SRSOrderedSRIDs()) != 1 {
		t.Fatalf("in-domain absent drop must publish: %+v", state.SRSOrderedSRIDs())
	}
	// Now a boundary write on the same SRID: stale mint + barrier flip.
	vgOutOfDomain(state)
	enriched = appaudit.EnrichSRSPrivForTest(t, state, provider,
		"DROP SPATIAL REFERENCE SYSTEM 100001;")
	if enriched[0].Unsupported == nil || enriched[0].Unsupported.Metadata["gate"] != "version" {
		t.Fatalf("must take the version boundary: %+v", enriched[0].Unsupported)
	}
	paSameStale(t, appaudit.BatchStateStaleSet(state), map[uint32]bool{100001: true}, "boundary DROP mints stale")
	entry := opEntry(t, state, 100001)
	if !entry.Barrier {
		t.Fatalf("entry=%+v want barrier flipped by the boundary write", entry)
	}
}

// D3: a later same-SRID statement never consumes a stale positive fact
// — the ordered entry is barrier'd and the stale flag short-circuits
// the bind even for an in-domain reader.
func TestT26SRSVersionGateNoStaleConsumption(t *testing.T) {
	fx := r3PrivilegeFixture()
	provider, state := paRealState(t, fx)
	appaudit.EnrichSRSPrivForTest(t, state, provider,
		"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;")
	vgOutOfDomain(state)
	appaudit.EnrichSRSPrivForTest(t, state, provider,
		"CREATE OR REPLACE SPATIAL REFERENCE SYSTEM 100001 NAME 'a' DEFINITION 'x';")
	// Back in-domain: the stale flag blocks the bind before any read.
	state.SeedResolvedVersion(&spec.VersionIdentity{
		Product: spec.VersionProductMySQL, Version: "8.0.46",
		Major: 8, Minor: 0, Patch: 46, ValidatedRange: true, Source: spec.VersionSourceObserved})
	enriched := appaudit.EnrichSRSPrivForTest(t, state, provider,
		"DROP SPATIAL REFERENCE SYSTEM 100001;")
	bound := enriched[0].Metadata.SpatialReference
	if bound == nil || bound.UnreadReason != spec.MetadataUnreadBatchEffects {
		t.Fatalf("bound=%+v want batch_effects — the minted stale flag must not be consumed", bound)
	}
	if enriched[0].Metadata.SRSDrop == nil || enriched[0].Metadata.SRSDrop.Arm != spec.SRSDropArmBlocked {
		t.Fatalf("arm=%+v want blocked (batch_effects)", enriched[0].Metadata.SRSDrop)
	}
}

// D4: a different SRID stays unpolluted by the boundary write.
func TestT26SRSVersionGateDifferentSRIDUnpolluted(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.version = "8.4.10"
	fx.discoverVer = "8.4.10"
	provider, state := paRealState(t, fx)
	vgOutOfDomain(state)
	appaudit.EnrichSRSPrivForTest(t, state, provider,
		"DROP SPATIAL REFERENCE SYSTEM 100001;")
	paSameStale(t, appaudit.BatchStateStaleSet(state), map[uint32]bool{100001: true}, "only the authored SRID")
}

// D5: a second enrich call does not re-mint (idempotent stale flag).
func TestT26SRSVersionGateSecondEnrich(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.version = "8.4.10"
	fx.discoverVer = "8.4.10"
	provider, state := paRealState(t, fx)
	vgOutOfDomain(state)
	appaudit.EnrichSRSPrivForTest(t, state, provider, "DROP SPATIAL REFERENCE SYSTEM 100001;")
	first := appaudit.BatchStateStaleSet(state)
	appaudit.EnrichSRSPrivForTest(t, state, provider, "DROP SPATIAL REFERENCE SYSTEM 100001;")
	paSameStale(t, appaudit.BatchStateStaleSet(state), first, "second enrich re-mints nothing new")
}

// D6: the boundary write never touches the batch dependency flag —
// D3-0's SRS family exclusion is a separate frozen contract.
func TestT26SRSVersionGateNoDepsStale(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.version = "8.4.10"
	fx.discoverVer = "8.4.10"
	provider, state := paRealState(t, fx)
	vgOutOfDomain(state)
	appaudit.EnrichSRSPrivForTest(t, state, provider,
		"CREATE SPATIAL REFERENCE SYSTEM 100001 NAME 'a' DEFINITION 'x';")
	if state.SRSDepsStale() {
		t.Fatal("boundary SRS write must not mint srsDepsStale — D3-0 stays closed this round")
	}
}
