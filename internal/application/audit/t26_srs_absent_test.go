package audit_test

// T26-SRS-DROP-PROVEN-ABSENT-EXECUTION-01 request/public rows — the
// api and public halves of the frozen case matrix
// (docs/verification/t26-srs-drop-proven-absent-contract-01
// case-matrix PA-1, PA-2, PA-3a, PA-4b/PA-4c, PA-6, PA-7b, PA-9d,
// PA-10). Every row rides the real request path (AuditSQL → parse →
// enrich → rules → projection) over the r3 product-provider stub;
// no arm is recomputed in-test.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	audit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// paGap asserts exactly one evidence gap with the frozen owner,
// reason, and ORDERED required-facts list — the PA-2c/2d/7b/10c
// unproven_privilege shape and the ordinary unknown_srs_state shape.
func paGap(t *testing.T, stmt report.StatementResult, reason string, wantFacts []string) {
	t.Helper()
	if len(stmt.EvidenceGaps) != 1 {
		t.Fatalf("gaps=%+v want exactly one", stmt.EvidenceGaps)
	}
	gap := stmt.EvidenceGaps[0]
	if gap.RuleID != "ddl.mysql.srs.drop.dependency" || gap.ReasonCode != reason {
		t.Fatalf("gap=%+v want owner ddl.mysql.srs.drop.dependency / %s", gap, reason)
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

// paAbsentNoop asserts the frozen A-IE✓ public projection: exactly
// one ddl.mysql.srs.drop.absent_noop notice with the exact metadata
// {srid:uint32, outcome:"absent_noop",
// conditional_audit:"observed_account_new_session"} — the marker
// legitimately rides THIS finding, so the generic r3OnlyFinding leak
// guard does not apply.
func paAbsentNoop(t *testing.T, stmt report.StatementResult, srid uint32) {
	t.Helper()
	if len(stmt.Findings) != 1 {
		t.Fatalf("findings=%+v want exactly one absent_noop finding", stmt.Findings)
	}
	f := stmt.Findings[0]
	if f.RuleID != "ddl.mysql.srs.drop.absent_noop" || f.Level != rule.LevelNotice {
		t.Fatalf("finding=%+v want the absent_noop notice", f)
	}
	wantMessage := fmt.Sprintf("spatial reference system %d does not exist; IF EXISTS drop is a no-op", srid)
	if f.Message != wantMessage {
		t.Fatalf("finding message=%q want exact %q", f.Message, wantMessage)
	}
	want := map[string]any{
		"srid":              srid,
		"outcome":           "absent_noop",
		"conditional_audit": string(spec.PrivilegeAuditModeObservedAccountNewSession),
	}
	if len(f.Metadata) != len(want) {
		t.Fatalf("metadata=%+v want exactly %v", f.Metadata, want)
	}
	for k, v := range want {
		if f.Metadata[k] != v {
			t.Fatalf("metadata[%q]=%v(%T) want %v(%T)", k, f.Metadata[k], f.Metadata[k], v, v)
		}
	}
}

// paPolicyFile writes a minimal rule-override config for the
// PA-10d/PA-10e policy rows — every unlisted rule keeps its default.
func paPolicyFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write policy: %v", err)
	}
	return path
}

// --- PA-1: absence supply discriminant ---------------------------------------

// PA-1a: providerless — pin admits D-qualify but no existence read
// ever runs, so the statement stays S2 with the full unproven debt.
func TestT26SRSAbsentProviderlessStaysUnproven(t *testing.T) {
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM 100001;", Dialect: spec.DialectMySQL,
		TargetVersion: "8.0.36",
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	stmt := result.Statements[0]
	paGap(t, stmt, "unknown_srs_state",
		[]string{spec.SRSFactExistence, spec.SRSFactColumnDependencies, spec.SRSFactDropPrivilege})
	if len(stmt.Findings) != 0 {
		t.Fatalf("findings=%+v want none — nothing discharges an unproven premise", stmt.Findings)
	}
	r3Aggregate(t, result, report.CoverageUnverified)
}

// PA-1b/PA-2a: plain DROP against a proven absence — the absent
// blocker is the only finding, metadata exactly
// {srid:uint32, outcome:"refused_absent"}; complete; reject.
func TestT26SRSAbsentPlainRefused(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.srs = nil // clean zero observed rows under the in-domain Qo
	p := r3Harness(t, fx)
	result, err := r3PrivilegeRequest(t, p, "DROP SPATIAL REFERENCE SYSTEM 100001;")
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	stmt := result.Statements[0]
	r3OnlyFinding(t, stmt, "ddl.mysql.srs.drop.absent",
		"spatial reference system 100001 does not exist",
		map[string]any{"srid": uint32(100001), "outcome": "refused_absent"})
	if len(stmt.EvidenceGaps) != 0 {
		t.Fatalf("gaps=%+v want none — absence owes nothing", stmt.EvidenceGaps)
	}
	if stmt.Coverage.Status != report.CoverageComplete || result.Coverage.Status != report.CoverageComplete {
		t.Fatalf("coverage=%v/%v want complete/complete", stmt.Coverage.Status, result.Coverage.Status)
	}
	if result.Verdict != report.VerdictReject {
		t.Fatalf("verdict=%v want reject", result.Verdict)
	}
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 1, 1)
}

// PA-1c/PA-8b: the ONLY changed input is the Qo banner — an
// unparseable answer can never carry the completeness proof, so the
// clean zero-row answer stays unknown: S2 [E,D] + privilege notice.
func TestT26SRSAbsentUnprovenQoStaysUnproven(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.srs = nil
	fx.version = "unparseable"
	p := r3Harness(t, fx)
	result, err := r3PrivilegeRequest(t, p, "DROP SPATIAL REFERENCE SYSTEM 100001;")
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	r3S2Consumed(t, result.Statements[0],
		[]string{spec.SRSFactExistence, spec.SRSFactColumnDependencies}, 100001, false)
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 1, 1)
}

// --- PA-2: plain / IF EXISTS × premise ----------------------------------------

// PA-2b: IF EXISTS on a proven absence under the consumable premise —
// absent_noop notice carrying the conditional_audit marker; complete;
// pass.
func TestT26SRSAbsentIfExistsNoop(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.srs = nil
	p := r3Harness(t, fx)
	result, err := r3PrivilegeRequest(t, p, "DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;")
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	stmt := result.Statements[0]
	paAbsentNoop(t, stmt, 100001)
	if !strings.Contains(strings.ToLower(stmt.NormalizedSQL), "if exists") {
		t.Fatalf("normalized_sql=%q must keep the authored IF EXISTS", stmt.NormalizedSQL)
	}
	if len(stmt.EvidenceGaps) != 0 {
		t.Fatalf("gaps=%+v want none", stmt.EvidenceGaps)
	}
	if stmt.Coverage.Status != report.CoverageComplete || result.Verdict != report.VerdictPass {
		t.Fatalf("coverage=%v verdict=%v want complete/pass", stmt.Coverage.Status, result.Verdict)
	}
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 1, 1)
}

// PA-2c: IF EXISTS on a proven absence under an UNPROVEN premise —
// exactly the [srs.drop.privilege] debt under unproven_privilege.
func TestT26SRSAbsentIfExistsConditional(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.srs = nil
	fx.grants = []string{"GRANT SELECT ON *.* TO `app`@`%`"}
	p := r3Harness(t, fx)
	result, err := r3PrivilegeRequest(t, p, "DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;")
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	stmt := result.Statements[0]
	paGap(t, stmt, "unproven_privilege", []string{spec.SRSFactDropPrivilege})
	if len(stmt.Findings) != 0 {
		t.Fatalf("findings=%+v want none — the conditional arm publishes no notice", stmt.Findings)
	}
	r3Aggregate(t, result, report.CoverageUnverified)
}

// PA-2d: a prefix REVOKE ends the observed privilege scope — the
// suppressed premise lands the same conditional arm and gap.
func TestT26SRSAbsentIfExistsSuppressed(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.srs = nil
	p := r3Harness(t, fx)
	result, err := r3PrivilegeRequest(t, p,
		"REVOKE SELECT ON *.* FROM 'app'@'%'; DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;")
	// REVOKE is recognized-but-unsupported — the request reports it
	// while still carrying every audited statement's projection.
	if !errors.Is(err, audit.ErrUnsupportedStatement) {
		t.Fatalf("err=%v want ErrUnsupportedStatement", err)
	}
	if len(result.Statements) != 2 {
		t.Fatalf("statements=%d want 2", len(result.Statements))
	}
	stmt := result.Statements[1]
	paGap(t, stmt, "unproven_privilege", []string{spec.SRSFactDropPrivilege})
	if len(stmt.Findings) != 0 {
		t.Fatalf("findings=%+v want none", stmt.Findings)
	}
}

// --- PA-3a: S2 control on a present object ------------------------------------

// PA-3a: existence proven present + no visible dependents → S2
// owing [D] with the consumable premise's privilege notice.
func TestT26SRSAbsentExistsStaysUnproven(t *testing.T) {
	fx := r3PrivilegeFixture()
	p := r3Harness(t, fx)
	result, err := r3PrivilegeRequest(t, p, "DROP SPATIAL REFERENCE SYSTEM 100001;")
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	r3S2Consumed(t, result.Statements[0], []string{spec.SRSFactColumnDependencies}, 100001, false)
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 1, 1)
}

// --- PA-4b/4c: S1 precedence variants ------------------------------------------

// PA-4b: IF EXISTS keeps the refusal arm — a valid positive witness
// refuses regardless of the guard.
func TestT26SRSAbsentIfExistsDependencyRefused(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.srs = nil
	fx.deps = []r3DepRow{{schema: "s", table: "t", column: "g", srid: 100001}}
	p := r3Harness(t, fx)
	result, err := r3PrivilegeRequest(t, p, "DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;")
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	r3RefusedDependency(t, result, result.Statements[0], 100001, 1)
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 1, 1)
}

// PA-4c: a real prefix-minted srsDepsStale (DROP TABLE s.t through
// the accepted table-fixture seam) demotes the witness — ValidAbsence
// governs → absent_refused, NOT refused_dependency.
func TestT26SRSAbsentDepsStaleDemotesWitness(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.srs = nil
	fx.deps = []r3DepRow{{schema: "s", table: "t", column: "g", srid: 100001}}
	inner := r3Harness(t, fx)
	p := &r3TableFixtureProvider{p: inner, tables: map[string]*spec.TableSnapshot{
		"s.t": {Schema: "s", Table: &spec.Table{Schema: "s", Name: "t"}, Exists: true,
			Columns: []spec.Column{{Name: "g", Type: "geometry"}}},
	}}
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL:     "DROP TABLE s.t; DROP SPATIAL REFERENCE SYSTEM 100001;",
		Dialect: spec.DialectMySQL, TargetVersion: "8.0.36",
		MetadataProvider:   p,
		PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 2 {
		t.Fatalf("statements=%d want 2", len(result.Statements))
	}
	stmt := result.Statements[1]
	r3OnlyFinding(t, stmt, "ddl.mysql.srs.drop.absent",
		"spatial reference system 100001 does not exist",
		map[string]any{"srid": uint32(100001), "outcome": "refused_absent"})
	if len(stmt.EvidenceGaps) != 0 {
		t.Fatalf("gaps=%+v want none", stmt.EvidenceGaps)
	}
}

// --- PA-6: ordering and the per-request cache ---------------------------------

// PA-6a: two identical drops in one request share the per-request
// object cache — stmt2 is a cache hit, both absent_refused.
func TestT26SRSAbsentSameSRIDCacheHit(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.srs = nil
	p := r3Harness(t, fx)
	result, err := r3PrivilegeRequest(t, p,
		"DROP SPATIAL REFERENCE SYSTEM 100001; DROP SPATIAL REFERENCE SYSTEM 100001;")
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 2 {
		t.Fatalf("statements=%d want 2", len(result.Statements))
	}
	for i := range result.Statements {
		r3OnlyFinding(t, result.Statements[i], "ddl.mysql.srs.drop.absent",
			"spatial reference system 100001 does not exist",
			map[string]any{"srid": uint32(100001), "outcome": "refused_absent"})
	}
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 1, 1)
}

// PA-6b: a different SRID is an independent cache key — the second
// resolve re-reads the catalog.
func TestT26SRSAbsentDifferentSRIDRereads(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.srs = nil
	p := r3Harness(t, fx)
	result, err := r3PrivilegeRequest(t, p,
		"DROP SPATIAL REFERENCE SYSTEM 100001; DROP SPATIAL REFERENCE SYSTEM 100002;")
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 2 {
		t.Fatalf("statements=%d want 2", len(result.Statements))
	}
	r3OnlyFinding(t, result.Statements[1], "ddl.mysql.srs.drop.absent",
		"spatial reference system 100002 does not exist",
		map[string]any{"srid": uint32(100002), "outcome": "refused_absent"})
	r3AssertReads(t, p, fx, 2, 2, 2, 2, 1, 1)
}

// PA-6c: a boundary CREATE OR REPLACE mints the authored SRID stale;
// the same-batch DROP's bind short-circuits — B, zero extra reads.
func TestT26SRSAbsentBoundaryPrefixBlocks(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.srs = nil
	p := r3Harness(t, fx)
	result, err := r3PrivilegeRequest(t, p,
		"CREATE OR REPLACE SPATIAL REFERENCE SYSTEM 100001 DEFINITION 'x';"+
			"DROP SPATIAL REFERENCE SYSTEM 100001;")
	// The boundary CREATE marks the batch Unsupported — reported as an
	// error while the audited statements still project.
	if !errors.Is(err, audit.ErrUnsupportedStatement) {
		t.Fatalf("err=%v want ErrUnsupportedStatement", err)
	}
	if len(result.Statements) != 2 {
		t.Fatalf("statements=%d want 2", len(result.Statements))
	}
	stmt := result.Statements[1]
	paGap(t, stmt, "unknown_srs_state", []string{spec.SRSFactBatchEffects})
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 1, 1)
}

// PA-6e: no pin + observed 8.0.3 — both statements stay version
// boundary (never adjudicated); stmt2's zero extra reads come from
// stmt1's boundary own-apply stale mint, not from "reads nothing".
func TestT26SRSAbsentBoundaryVersionPair(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.version = "8.0.3"
	fx.discoverVer = "8.0.3"
	fx.srs = nil
	p := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL:     "DROP SPATIAL REFERENCE SYSTEM 100001; DROP SPATIAL REFERENCE SYSTEM 100001;",
		Dialect: spec.DialectMySQL, MetadataProvider: p,
		PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	if !errors.Is(err, audit.ErrUnsupportedStatement) {
		t.Fatalf("err=%v want ErrUnsupportedStatement — both statements sit at the version boundary", err)
	}
	if len(result.Statements) != 2 {
		t.Fatalf("statements=%d want 2", len(result.Statements))
	}
	if len(result.Unsupported) != 2 {
		t.Fatalf("unsupported=%+v want the version-boundary marker on both statements", result.Unsupported)
	}
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 0, 1)
}

// --- PA-7b: REVOKE scoping — public twin ---------------------------------------

// PA-7b: consumable → REVOKE → suppressed at the public projection.
func TestT26SRSAbsentRevokeScopingPublic(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.srs = nil
	p := r3Harness(t, fx)
	result, err := r3PrivilegeRequest(t, p,
		"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;"+
			"REVOKE SELECT ON *.* FROM 'app'@'%';"+
			"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100002;")
	if !errors.Is(err, audit.ErrUnsupportedStatement) {
		t.Fatalf("err=%v want ErrUnsupportedStatement", err)
	}
	if len(result.Statements) != 3 {
		t.Fatalf("statements=%d want 3", len(result.Statements))
	}
	paAbsentNoop(t, result.Statements[0], 100001)
	paGap(t, result.Statements[2], "unproven_privilege", []string{spec.SRSFactDropPrivilege})
	r3AssertReads(t, p, fx, 2, 2, 2, 2, 1, 1)
}

// --- PA-9d: capability minus ObjectResolver -------------------------------------

// paNoResolverProvider carries every capability except the object
// resolver — the SRS bind stays unissued and the answer stays S2.
type paNoResolverProvider struct{}

func (paNoResolverProvider) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{Version: "8.0.36"}, nil
}

func (paNoResolverProvider) LoadTableSnapshot(context.Context, spec.Dialect, string, string) (*spec.TableSnapshot, error) {
	return nil, nil
}

func (paNoResolverProvider) LoadMySQLServerDropPrivilege(context.Context, spec.Dialect) (*spec.MySQLServerDropPrivilegeObservation, error) {
	return &spec.MySQLServerDropPrivilegeObservation{
		Account:    "app@%",
		GrantLines: []string{"GRANT SELECT, SUPER ON *.* TO `app`@`%`"},
	}, nil
}

// PA-9d: no ObjectResolver — the snapshot stays Unavailable, the
// consumable premise still projects its S2 privilege notice.
func TestT26SRSAbsentNoResolverStaysUnproven(t *testing.T) {
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM 100001;", Dialect: spec.DialectMySQL,
		TargetVersion: "8.0.36", MetadataProvider: paNoResolverProvider{},
		PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	r3S2Consumed(t, result.Statements[0],
		[]string{spec.SRSFactExistence, spec.SRSFactColumnDependencies}, 100001, false)
}

// --- PA-10: public shape & policy ------------------------------------------------

// PA-10a: the reserved-range advisory NEVER co-fires on absent_refused
// — the blocker alone carries the outcome.
func TestT26SRSAbsentRefusedNoReservedAdvisory(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.srs = nil
	p := r3Harness(t, fx)
	result, err := r3PrivilegeRequest(t, p, "DROP SPATIAL REFERENCE SYSTEM 4326;")
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	r3OnlyFinding(t, result.Statements[0], "ddl.mysql.srs.drop.absent",
		"spatial reference system 4326 does not exist",
		map[string]any{"srid": uint32(4326), "outcome": "refused_absent"})
}

// PA-10b: IF EXISTS on a reserved absent SRID — absent_noop plus
// exactly one membership-only reserved advisory.
func TestT26SRSAbsentNoopReservedAdvisory(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.srs = nil
	p := r3Harness(t, fx)
	result, err := r3PrivilegeRequest(t, p, "DROP SPATIAL REFERENCE SYSTEM IF EXISTS 4326;")
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	stmt := result.Statements[0]
	if len(stmt.Findings) != 2 {
		t.Fatalf("findings=%+v want absent_noop + reserved", stmt.Findings)
	}
	var noop, reserved *rule.Finding
	for i := range stmt.Findings {
		switch stmt.Findings[i].RuleID {
		case "ddl.mysql.srs.drop.absent_noop":
			noop = &stmt.Findings[i]
		case "ddl.mysql.srs.drop.reserved":
			reserved = &stmt.Findings[i]
		}
	}
	if noop == nil || reserved == nil {
		t.Fatalf("findings=%+v want absent_noop + reserved", stmt.Findings)
	}
	if len(reserved.Metadata) != 1 || fmt.Sprintf("%v", reserved.Metadata["srid"]) != "4326" {
		t.Fatalf("reserved metadata=%+v want exactly {srid:4326}", reserved.Metadata)
	}
	if len(stmt.EvidenceGaps) != 0 {
		t.Fatalf("gaps=%+v want none", stmt.EvidenceGaps)
	}
}

// PA-10c: the conditional arm keeps the reserved advisory while the
// privilege debt publishes under unproven_privilege — no marker.
func TestT26SRSAbsentConditionalReservedAdvisory(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.srs = nil
	fx.grants = []string{"GRANT SELECT ON *.* TO `app`@`%`"}
	p := r3Harness(t, fx)
	result, err := r3PrivilegeRequest(t, p, "DROP SPATIAL REFERENCE SYSTEM IF EXISTS 4326;")
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	stmt := result.Statements[0]
	if len(stmt.Findings) != 1 || stmt.Findings[0].RuleID != "ddl.mysql.srs.drop.reserved" {
		t.Fatalf("findings=%+v want exactly the reserved advisory", stmt.Findings)
	}
	paGap(t, stmt, "unproven_privilege", []string{spec.SRSFactDropPrivilege})
}

// PA-10d: disabling the absent rule removes the finding only —
// adjudication/apply are untouched, coverage stays complete.
func TestT26SRSAbsentDisabledRuleSilent(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.srs = nil
	p := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM 100001;", Dialect: spec.DialectMySQL,
		TargetVersion: "8.0.36", MetadataProvider: p,
		ConfigPath:         paPolicyFile(t, "rules:\n  \"ddl.mysql.srs.drop.absent\":\n    enabled: false\n"),
		PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	stmt := result.Statements[0]
	if len(stmt.Findings) != 0 || len(stmt.EvidenceGaps) != 0 {
		t.Fatalf("findings=%+v gaps=%+v want both empty", stmt.Findings, stmt.EvidenceGaps)
	}
	if stmt.Coverage.Status != report.CoverageComplete || result.Verdict != report.VerdictPass {
		t.Fatalf("coverage=%v verdict=%v want complete/pass", stmt.Coverage.Status, result.Verdict)
	}
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 1, 1)
}

// PA-10e: the absent_noop rule at warning level — verdict review;
// the conditional_audit marker is preserved verbatim.
func TestT26SRSAbsentNoopWarningLevel(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.srs = nil
	p := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;", Dialect: spec.DialectMySQL,
		TargetVersion: "8.0.36", MetadataProvider: p,
		ConfigPath:         paPolicyFile(t, "rules:\n  \"ddl.mysql.srs.drop.absent_noop\":\n    enabled: true\n    level: warning\n"),
		PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	stmt := result.Statements[0]
	if len(stmt.Findings) != 1 || stmt.Findings[0].Level != rule.LevelWarning {
		t.Fatalf("findings=%+v want one warning-level absent_noop", stmt.Findings)
	}
	if marker := stmt.Findings[0].Metadata["conditional_audit"]; marker != string(spec.PrivilegeAuditModeObservedAccountNewSession) {
		t.Fatalf("conditional_audit=%v want the marker preserved", marker)
	}
	if result.Verdict != report.VerdictReview {
		t.Fatalf("verdict=%v want review", result.Verdict)
	}
}

// PA-10f: sentinel scans — catalog names, dependent identities, and
// the observed account/grant text never reach the serialized result.
func TestT26SRSAbsentSentinelScan(t *testing.T) {
	run := func(t *testing.T, fx *r3Fixture, sqlText string) string {
		p := r3Harness(t, fx)
		result, err := r3PrivilegeRequest(t, p, sqlText)
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		blob, err := json.Marshal(result)
		if err != nil {
			t.Fatalf("marshal result: %v", err)
		}
		return string(blob)
	}
	// PA-3a-variant: the catalog srs_name sentinel.
	fx := r3PrivilegeFixture()
	fx.srs = map[uint32]string{100001: "SRS_ZZTEST"}
	if out := run(t, fx, "DROP SPATIAL REFERENCE SYSTEM 100001;"); strings.Contains(out, "SRS_ZZTEST") {
		t.Fatalf("serialized result leaks the catalog name sentinel: %s", out)
	}
	// PA-4a-variant: dependent schema/table/column sentinels.
	fx = r3PrivilegeFixture()
	fx.srs = nil
	fx.deps = []r3DepRow{{schema: "zz_schema", table: "zz_table", column: "zz_col", srid: 100001}}
	out := run(t, fx, "DROP SPATIAL REFERENCE SYSTEM 100001;")
	for _, sentinel := range []string{"zz_schema", "zz_table", "zz_col"} {
		if strings.Contains(out, sentinel) {
			t.Fatalf("serialized result leaks the dependency sentinel %q: %s", sentinel, out)
		}
	}
	// PA-2b-variant: the observed account and grant text sentinels.
	fx = r3PrivilegeFixture()
	fx.srs = nil
	fx.account = "zz_acct@%"
	fx.grants = []string{"GRANT SUPER ON *.* TO `zz_acct`@`%`"}
	out = run(t, fx, "DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;")
	for _, sentinel := range []string{"zz_acct", "GRANT SUPER"} {
		if strings.Contains(out, sentinel) {
			t.Fatalf("serialized result leaks the privilege sentinel %q: %s", sentinel, out)
		}
	}
}
