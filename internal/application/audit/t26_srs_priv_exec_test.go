package audit_test

// Package audit_test: T26-SRS-DROP-PRIVILEGE-EXECUTION-01 R1 — the
// request/API-layer matrix witnesses. Every provider-backed row drives
// the REAL product chain: scripted database/sql driver →
// mysqlmeta.NewProvider(db) → product discovery, privilege loader, and
// SRS object loader; the r3 wrapper only counts calls and injects
// sentinels at the named seams. The counting vector follows the
// contract order R/Qexist/Qdeps/Qo/L with Vd separate; the SERVER
// family counts its own forward/mysql.servers/internal-version seam.
//
// Frozen expectations: docs/verification/t26-srs-drop-privilege-
// contract-01/case-matrix.md (R3, accepted at 2f7231b5) — P1–P10 with
// default fixture mysql, pin/observed/loader 8.0.36, mode
// observed_account_new_session, account app@%, direct-global
// SELECT,SUPER, scope open, clean state, E=row, D=[].

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

// r3PrivilegeFixture is the default positive-witness fixture — every
// consumable row derives from it by field override.
func r3PrivilegeFixture() *r3Fixture {
	return &r3Fixture{
		version:     "8.0.36",
		discoverVer: "8.0.36",
		srs:         map[uint32]string{100001: "SRID 100001"},
		account:     "app@%",
		grants:      []string{"GRANT SELECT, SUPER ON *.* TO `app`@`%`"},
	}
}

// r3PrivilegeRequest runs the real request path under
// observed_account_new_session for the given SQL.
func r3PrivilegeRequest(t *testing.T, p *r3ProductProvider, sqlText string) (report.Result, error) {
	t.Helper()
	return audit.AuditSQL(context.Background(), audit.Request{
		SQL: sqlText, Dialect: spec.DialectMySQL, TargetVersion: "8.0.36",
		MetadataProvider:   p,
		PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
}

// r3ExactGap is the frozen strict gap assertion (contract §R2-C):
// exactly one evidence gap on the statement, carrying exactly the
// dependency-owned rule id, the unknown_srs_state reason, and the
// row's frozen ORDERED required-facts list. Flattening several gaps
// into facts alone can never prove this shape.
func r3ExactGap(t *testing.T, stmt report.StatementResult, wantFacts []string) {
	t.Helper()
	if len(stmt.EvidenceGaps) != 1 {
		t.Fatalf("gaps=%+v want exactly one", stmt.EvidenceGaps)
	}
	gap := stmt.EvidenceGaps[0]
	if gap.RuleID != "ddl.mysql.srs.drop.dependency" || gap.ReasonCode != "unknown_srs_state" {
		t.Fatalf("gap=%+v want owner ddl.mysql.srs.drop.dependency / unknown_srs_state", gap)
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

// r3Aggregate asserts the frozen whole-request projection for the
// rows that declare it: review verdict plus the aggregate coverage.
func r3Aggregate(t *testing.T, result report.Result, wantCoverage report.CoverageStatus) {
	t.Helper()
	if result.Verdict != report.VerdictReview {
		t.Fatalf("verdict=%v want review", result.Verdict)
	}
	if result.Coverage.Status != wantCoverage {
		t.Fatalf("aggregate coverage=%v want %v", result.Coverage.Status, wantCoverage)
	}
}

// r3OnlyFinding asserts the statement carries exactly one finding —
// the named rule — with the exact fixed message and the exact
// ORDERED metadata key/value set (no extra keys, no conditional_audit
// leak). want is the literal metadata map; every value is compared by
// typed equality against the emitted metadata.
func r3OnlyFinding(t *testing.T, stmt report.StatementResult, ruleID, wantMessage string, wantMetadata map[string]any) {
	t.Helper()
	if len(stmt.Findings) != 1 {
		t.Fatalf("findings=%+v want exactly one %s finding", stmt.Findings, ruleID)
	}
	f := stmt.Findings[0]
	if f.RuleID != ruleID {
		t.Fatalf("finding rule=%q want %q", f.RuleID, ruleID)
	}
	if wantMessage != "" && f.Message != wantMessage {
		t.Fatalf("finding message=%q want exact %q", f.Message, wantMessage)
	}
	if len(f.Metadata) != len(wantMetadata) {
		t.Fatalf("finding metadata=%+v want exactly %d keys %v", f.Metadata, len(wantMetadata), wantMetadata)
	}
	for k, want := range wantMetadata {
		got, ok := f.Metadata[k]
		if !ok {
			t.Fatalf("finding metadata missing key %q: %+v", k, f.Metadata)
		}
		if got != want {
			t.Fatalf("finding metadata[%q]=%v(%T) want %v(%T)", k, got, got, want, want)
		}
	}
	if _, leaked := f.Metadata["conditional_audit"]; leaked {
		t.Fatalf("conditional_audit leaked into %s metadata: %+v", ruleID, f.Metadata)
	}
}

// r3TableFixtureProvider delegates every provider method to the real
// mysqlmeta.NewProvider chain except LoadTableSnapshot — the P6a
// request/API twin's accepted test-only table seam (tblTFixture shape:
// s.t Exists=true, Columns {g:geometry}). Discovery, the privilege
// loader, and the SRS resolver still ride the real product code.
type r3TableFixtureProvider struct {
	p      *r3ProductProvider
	tables map[string]*spec.TableSnapshot
}

func (w *r3TableFixtureProvider) LoadInstanceFacts(ctx context.Context, dialect spec.Dialect, schema string) (*spec.InstanceFacts, error) {
	return w.p.LoadInstanceFacts(ctx, dialect, schema)
}

func (w *r3TableFixtureProvider) LoadTableSnapshot(_ context.Context, _ spec.Dialect, schema, table string) (*spec.TableSnapshot, error) {
	return w.tables[schema+"."+table], nil
}

func (w *r3TableFixtureProvider) ResolveObject(ctx context.Context, dialect spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	return w.p.ResolveObject(ctx, dialect, req)
}

func (w *r3TableFixtureProvider) LoadMySQLServerDropPrivilege(ctx context.Context, dialect spec.Dialect) (*spec.MySQLServerDropPrivilegeObservation, error) {
	return w.p.LoadMySQLServerDropPrivilege(ctx, dialect)
}

// r3ServerSeam returns the SERVER family's separate counts —
// resolver forwards, mysql.servers query attempts, and internal
// version-query attempts attributable to the SERVER loader (none: all
// counted version queries belong to the SRS catalog seam, so the
// SERVER count is always zero and asserted as such).
func r3ServerSeam(p *r3ProductProvider, fx *r3Fixture) (forwards, servers int) {
	for _, req := range p.forwards {
		if req.Type == "mysql_server" {
			forwards++
		}
	}
	return forwards, fx.count["srv"]
}

// --- P1: three distinct unavailable reasons --------------------------------

// P1a (ER1): providerless under mode — the pin is conflict-check
// only; with no observation the statement takes the version boundary
// (unproven) and the loader surface never exists.
func TestT26SRSPrivP1aProviderless(t *testing.T) {
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM 100001;", Dialect: spec.DialectMySQL, TargetVersion: "8.0.36",
		PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	if err == nil || len(result.Unsupported) != 1 {
		t.Fatalf("err=%v unsupported=%+v want the version boundary", err, result.Unsupported)
	}
	u := result.Unsupported[0]
	if u.Metadata["gate"] != "version" || u.Metadata["version_reason"] != spec.MetadataUnreadVersionUnproven {
		t.Fatalf("unsupported=%+v want gate=version reason=version_unproven", u)
	}
}

// P1b: mode off — the SRS catalog still reads; the privilege premise
// is unproven because the observation never ran.
func TestT26SRSPrivP1bModeOff(t *testing.T) {
	fx := &r3Fixture{
		version: "8.0.36", discoverVer: "8.0.36",
		srs: map[uint32]string{100001: "SRID 100001"},
	}
	p := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM 100001;", Dialect: spec.DialectMySQL, TargetVersion: "8.0.36",
		MetadataProvider: p,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	r3S2(t, result, result.Statements[0],
		[]string{spec.SRSFactColumnDependencies, spec.SRSFactDropPrivilege}, 0)
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 0, 1)
}

// r3NoPrivilegeLoader forwards the three provider methods but never
// implements MySQLServerDropPrivilegeLoader — the P1c capability seam.
type r3NoPrivilegeLoader struct {
	p *r3ProductProvider
}

func (w *r3NoPrivilegeLoader) LoadInstanceFacts(ctx context.Context, dialect spec.Dialect, schema string) (*spec.InstanceFacts, error) {
	return w.p.LoadInstanceFacts(ctx, dialect, schema)
}

func (w *r3NoPrivilegeLoader) LoadTableSnapshot(ctx context.Context, dialect spec.Dialect, schema, table string) (*spec.TableSnapshot, error) {
	return w.p.LoadTableSnapshot(ctx, dialect, schema, table)
}

func (w *r3NoPrivilegeLoader) ResolveObject(ctx context.Context, dialect spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	return w.p.ResolveObject(ctx, dialect, req)
}

// P1c: provider lacks the loader capability — silently unproven, never
// a loader error.
func TestT26SRSPrivP1cMissingLoaderCapability(t *testing.T) {
	fx := &r3Fixture{
		version: "8.0.36", discoverVer: "8.0.36",
		srs: map[uint32]string{100001: "SRID 100001"},
	}
	p := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM 100001;", Dialect: spec.DialectMySQL, TargetVersion: "8.0.36",
		MetadataProvider:   &r3NoPrivilegeLoader{p: p},
		PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	if err != nil {
		t.Fatalf("audit: %v — a missing loader capability is not a loader error", err)
	}
	r3S2(t, result, result.Statements[0],
		[]string{spec.SRSFactColumnDependencies, spec.SRSFactDropPrivilege}, 0)
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 0, 1)
}

// --- P2/P3: real positive consumption ---------------------------------------

// P2: E=row — S2 consumable [D], the family's own privilege notice,
// own-apply mints the SRID's existence stale (internal companion).
// The serialized public result and seam counts are captured for the
// archive (T26_PRIV_EVIDENCE env path when set).
func TestT26SRSPrivP2Consumable(t *testing.T) {
	fx := r3PrivilegeFixture()
	p := r3Harness(t, fx)
	result, err := r3PrivilegeRequest(t, p, "DROP SPATIAL REFERENCE SYSTEM 100001;")
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	stmt := result.Statements[0]
	r3S2Consumed(t, stmt, []string{spec.SRSFactColumnDependencies}, 100001, false)
	r3Aggregate(t, result, report.CoverageUnverified)
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 1, 1)
	// The JSON layer separately asserts numeric srid, the exact key set,
	// and the exact token — decoded numbers are never mistaken for the
	// original typed uint32.
	blob, err := json.Marshal(stmt)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded struct {
		Findings []struct {
			RuleID   string         `json:"rule_id"`
			Metadata map[string]any `json:"metadata"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(blob, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(decoded.Findings) != 1 || decoded.Findings[0].RuleID != "ddl.mysql.srs.drop.privilege" {
		t.Fatalf("decoded findings=%+v want the single privilege notice", decoded.Findings)
	}
	md := decoded.Findings[0].Metadata
	if len(md) != 2 || md["srid"] != float64(100001) || md["conditional_audit"] != "observed_account_new_session" {
		t.Fatalf("decoded metadata=%+v want {srid:100001, conditional_audit:observed_account_new_session}", md)
	}
	if dir := os.Getenv("T26_PRIV_EVIDENCE"); dir != "" {
		full, _ := json.MarshalIndent(result, "", "  ")
		if werr := os.WriteFile(filepath.Join(dir, "p2-public-result.json"), full, 0o644); werr != nil {
			t.Fatalf("write p2 evidence: %v", werr)
		}
		// R2-C6: serialize the REAL measured counters — never the
		// expected vector. Each field reads the live seam the same way
		// r3AssertReads does (resolver forwards by object type, SQL
		// attempts by query text, loader/discovery by call counters).
		var forwards int
		for _, req := range p.forwards {
			if req.Type == "mysql_spatial_reference_system" {
				forwards++
			}
		}
		reads, _ := json.Marshal(map[string]int{
			"R":      forwards,
			"Qexist": fx.count["exist"],
			"Qdeps":  fx.count["deps"],
			"Qo":     fx.count["ver"],
			"L":      p.privCalls,
			"Vd":     p.vd,
		})
		if werr := os.WriteFile(filepath.Join(dir, "p2-seam-counts.json"), reads, 0o644); werr != nil {
			t.Fatalf("write p2 seam counts: %v", werr)
		}
	}
	t.Logf("P2 serialized public result: %s", blob)
}

// P3: E=zero rows — consumable still discharges P; [E,D]. The Qo
// banner 8.0.3 stays outside the proven-absence domain so the zero-row
// answer remains unknown — T26-SRS-DROP-PROVEN-ABSENT-EXECUTION-01.
func TestT26SRSPrivP3ConsumableZeroRowExistence(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.srs = nil             // zero-row existence answer
	fx.version = "8.0.3-log" // Qo outside the D-proof
	p := r3Harness(t, fx)
	result, err := r3PrivilegeRequest(t, p, "DROP SPATIAL REFERENCE SYSTEM 100001;")
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	r3S2Consumed(t, result.Statements[0],
		[]string{spec.SRSFactExistence, spec.SRSFactColumnDependencies}, 100001, false)
	r3Aggregate(t, result, report.CoverageUnverified)
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 1, 1)
}

// --- P4: witness and error branches -----------------------------------------

// P4a: a complete parsed global SELECT grant without SUPER — parsed
// shape, insufficient privilege, not malformed syntax.
func TestT26SRSPrivP4aGlobalSelectNoSuper(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.grants = []string{"GRANT SELECT ON *.* TO `app`@`%`"}
	p := r3Harness(t, fx)
	result, err := r3PrivilegeRequest(t, p, "DROP SPATIAL REFERENCE SYSTEM 100001;")
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	r3S2(t, result, result.Statements[0],
		[]string{spec.SRSFactColumnDependencies, spec.SRSFactDropPrivilege}, 0)
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 1, 1)
	if fx.count["user"] != 1 || fx.count["grants"] != 1 {
		t.Fatalf("privilege queries user/grants=%d/%d want 1/1", fx.count["user"], fx.count["grants"])
	}
}

// P4b: current_user() answers zero rows — Account stays unbound; the
// grant lines still return but no positive witness can bind.
func TestT26SRSPrivP4bUnboundAccount(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.account = "" // zero-row current_user answer
	p := r3Harness(t, fx)
	result, err := r3PrivilegeRequest(t, p, "DROP SPATIAL REFERENCE SYSTEM 100001;")
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	r3S2(t, result, result.Statements[0],
		[]string{spec.SRSFactColumnDependencies, spec.SRSFactDropPrivilege}, 0)
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 1, 1)
}

// P4c: real role-assignment SHOW GRANTS shape — no ON clause, never a
// privilege list.
func TestT26SRSPrivP4cRoleAssignmentLine(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.grants = []string{"GRANT `r1`@`%` TO `app`@`%`"}
	p := r3Harness(t, fx)
	result, err := r3PrivilegeRequest(t, p, "DROP SPATIAL REFERENCE SYSTEM 100001;")
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	r3S2(t, result, result.Statements[0],
		[]string{spec.SRSFactColumnDependencies, spec.SRSFactDropPrivilege}, 0)
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 1, 1)
}

// P4d: a legal non-global ALL PRIVILEGES grant — the global witness
// stays unproven.
func TestT26SRSPrivP4dNonGlobalAll(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.grants = []string{"GRANT ALL PRIVILEGES ON `s`.* TO `app`@`%`"}
	p := r3Harness(t, fx)
	result, err := r3PrivilegeRequest(t, p, "DROP SPATIAL REFERENCE SYSTEM 100001;")
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	r3S2(t, result, result.Statements[0],
		[]string{spec.SRSFactColumnDependencies, spec.SRSFactDropPrivilege}, 0)
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 1, 1)
}

// P4e: global ALL PRIVILEGES ON *.* — the registered static-set proof
// makes the premise consumable.
func TestT26SRSPrivP4eGlobalAllImpliesSuper(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.grants = []string{"GRANT ALL PRIVILEGES ON *.* TO `app`@`%`"}
	p := r3Harness(t, fx)
	result, err := r3PrivilegeRequest(t, p, "DROP SPATIAL REFERENCE SYSTEM 100001;")
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	r3S2Consumed(t, result.Statements[0],
		[]string{spec.SRSFactColumnDependencies}, 100001, false)
	r3Aggregate(t, result, report.CoverageUnverified)
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 1, 1)
}

// P4f: current_user QueryContext sentinel — attempts 1/0, wrapped
// error preserves the cause under the bounded public text.
func TestT26SRSPrivP4fCurrentUserFailure(t *testing.T) {
	sentinel := errors.New("t26-p4f-sentinel")
	fx := r3PrivilegeFixture()
	fx.failOn = map[string]error{"user": sentinel}
	p := r3Harness(t, fx)
	result, err := r3PrivilegeRequest(t, p, "DROP SPATIAL REFERENCE SYSTEM 100001;")
	if err == nil {
		t.Fatal("want the bounded privilege read error")
	}
	if err.Error() != "server drop privilege observation failed" {
		t.Fatalf("err=%q want exact bounded text", err.Error())
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("err=%v must unwrap to the injected sentinel", err)
	}
	if fx.count["user"] != 1 || fx.count["grants"] != 0 {
		t.Fatalf("attempts user/grants=%d/%d want 1/0 — grants never issued", fx.count["user"], fx.count["grants"])
	}
	if len(result.Statements) != 0 {
		t.Fatalf("result=%+v want N/A — no bind/adjudicate/apply ran", result.Statements)
	}
	r3AssertReads(t, p, fx, 0, 0, 0, 0, 1, 1)
}

// P4g: SHOW GRANTS QueryContext sentinel — attempts 1/1.
func TestT26SRSPrivP4gGrantsFailure(t *testing.T) {
	sentinel := errors.New("t26-p4g-sentinel")
	fx := r3PrivilegeFixture()
	fx.failOn = map[string]error{"grants": sentinel}
	p := r3Harness(t, fx)
	result, err := r3PrivilegeRequest(t, p, "DROP SPATIAL REFERENCE SYSTEM 100001;")
	if err == nil {
		t.Fatal("want the bounded privilege read error")
	}
	if err.Error() != "server drop privilege observation failed" {
		t.Fatalf("err=%q want exact bounded text", err.Error())
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("err=%v must unwrap to the injected sentinel", err)
	}
	if fx.count["user"] != 1 || fx.count["grants"] != 1 {
		t.Fatalf("attempts user/grants=%d/%d want 1/1", fx.count["user"], fx.count["grants"])
	}
	if len(result.Statements) != 0 {
		t.Fatalf("result=%+v want N/A — no bind/adjudicate/apply ran", result.Statements)
	}
	r3AssertReads(t, p, fx, 0, 0, 0, 0, 1, 1)
}

// P4h: synthetic SHOW GRANTS response — a complete, correctly shaped
// global grant whose privilege token is SUPERUSER, not SUPER. This is
// the substring-matching discriminator: a product implementation
// matching "SUPER" as a substring would wrongly consume here; the
// exact-token collector must not (review-registered erratum: this row
// is the request/API-layer discriminator, not a spec-only unit).
func TestT26SRSPrivP4hSuperuserExactToken(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.grants = []string{"GRANT SUPERUSER ON *.* TO `app`@`%`"}
	p := r3Harness(t, fx)
	result, err := r3PrivilegeRequest(t, p, "DROP SPATIAL REFERENCE SYSTEM 100001;")
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	// The grant parses (global scope, well-formed) — the premise stays
	// unproven purely because SUPERUSER != SUPER.
	r3S2(t, result, result.Statements[0],
		[]string{spec.SRSFactColumnDependencies, spec.SRSFactDropPrivilege}, 0)
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 1, 1)
	if fx.count["user"] != 1 || fx.count["grants"] != 1 {
		t.Fatalf("the real loader must issue user/grants 1/1, got %d/%d", fx.count["user"], fx.count["grants"])
	}
}

// --- P5: consumable premise on non-consuming arms ----------------------------

// P5a: a real leading parse failure contaminates — the DROP lands B;
// premise consumable but never consumed; [B], 0 findings/1 gap. The
// request still returns the audited statement inside the result under
// the parser-failure diagnostic contract.
func TestT26SRSPrivP5aParseFailureBlocked(t *testing.T) {
	fx := r3PrivilegeFixture()
	p := r3Harness(t, fx)
	result, err := r3PrivilegeRequest(t, p, "GARBAGE; DROP SPATIAL REFERENCE SYSTEM 100001;")
	if err == nil || !strings.Contains(err.Error(), "could not parse") {
		t.Fatalf("err=%v want the parser-failure diagnostic error", err)
	}
	if len(result.Diagnostics) == 0 {
		t.Fatal("the parser-failure diagnostic must be attached")
	}
	if len(result.Statements) != 1 {
		t.Fatalf("statements=%d want exactly the audited DROP", len(result.Statements))
	}
	stmt := result.Statements[0]
	if len(stmt.Findings) != 0 {
		t.Fatalf("findings=%+v want 0 — B arm emits no privilege notice", stmt.Findings)
	}
	r3ExactGap(t, stmt, []string{spec.SRSFactBatchEffects})
	if stmt.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("coverage=%v want unverified", stmt.Coverage.Status)
	}
	r3AssertReads(t, p, fx, 0, 0, 0, 0, 1, 1)
}

// P5b: same SRID dropped twice — stmt1 consumes and mints existence
// stale; stmt2 lands B with its premise still consumable, unconsumed.
func TestT26SRSPrivP5bSameSRIDTwice(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.srs = map[uint32]string{100001: "SRID 100001"}
	p := r3Harness(t, fx)
	result, err := r3PrivilegeRequest(t, p,
		"DROP SPATIAL REFERENCE SYSTEM 100001; DROP SPATIAL REFERENCE SYSTEM 100001;")
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 2 {
		t.Fatalf("statements=%d want 2", len(result.Statements))
	}
	r3S2Consumed(t, result.Statements[0], []string{spec.SRSFactColumnDependencies}, 100001, false)
	stmt2 := result.Statements[1]
	if len(stmt2.Findings) != 0 {
		t.Fatalf("stmt2 findings=%+v want 0 — B arm carries no privilege notice", stmt2.Findings)
	}
	r3ExactGap(t, stmt2, []string{spec.SRSFactBatchEffects})
	if stmt2.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("stmt2 coverage=%v want unverified", stmt2.Coverage.Status)
	}
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 1, 1)
	if result.Verdict != report.VerdictReview {
		t.Fatalf("verdict=%v want review — whole-request observation declared", result.Verdict)
	}
}

// P5c/P5d: SRID 0 plain and IF EXISTS — refused_srid0 arm; premise
// consumable but the arm never consumes; zero owed; the fixed blocker.
func TestT26SRSPrivP5cdSRID0(t *testing.T) {
	for _, sqlText := range []string{
		"DROP SPATIAL REFERENCE SYSTEM 0;",
		"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 0;",
	} {
		fx := r3PrivilegeFixture()
		fx.srs = map[uint32]string{0: "0"}
		p := r3Harness(t, fx)
		result, err := r3PrivilegeRequest(t, p, sqlText)
		if err != nil {
			t.Fatalf("audit %q: %v", sqlText, err)
		}
		stmt := result.Statements[0]
		r3OnlyFinding(t, stmt, "ddl.mysql.srs.drop.srid0",
			"dropping spatial reference system SRID 0 is not allowed",
			map[string]any{"srid": 0, "outcome": "refused_srid0"})
		// R3-B4: the exact shape check alone never proves the severity —
		// the refused arm's finding must be the blocker level.
		if stmt.Findings[0].Level != rule.LevelBlocker {
			t.Fatalf("%q level=%v want blocker", sqlText, stmt.Findings[0].Level)
		}
		if len(stmt.EvidenceGaps) != 0 {
			t.Fatalf("%q gaps=%+v want 0 — refused arm owes nothing", sqlText, stmt.EvidenceGaps)
		}
		if stmt.Coverage.Status != report.CoverageComplete || result.Coverage.Status != report.CoverageComplete {
			t.Fatalf("%q coverage=%v/%v want complete", sqlText, stmt.Coverage.Status, result.Coverage.Status)
		}
		if result.Verdict != report.VerdictReject {
			t.Fatalf("%q verdict=%v want reject", sqlText, result.Verdict)
		}
		r3AssertReads(t, p, fx, 1, 1, 1, 1, 1, 1)
	}
}

// P5e: a valid positive dependency — S1; premise consumable but the
// refused arm never consumes; one dependency blocker, zero gaps.
func TestT26SRSPrivP5eDependencyRefusal(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.deps = []r3DepRow{{schema: "s", table: "t", column: "g", srid: 100001}}
	p := r3Harness(t, fx)
	result, err := r3PrivilegeRequest(t, p, "DROP SPATIAL REFERENCE SYSTEM 100001;")
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	r3RefusedDependency(t, result, result.Statements[0], 100001, 1)
	stmt := result.Statements[0]
	r3OnlyFinding(t, stmt, "ddl.mysql.srs.drop.dependency",
		"spatial reference system 100001 is used by 1 visible geometry column(s)",
		map[string]any{"srid": uint32(100001), "outcome": "refused_dependency", "dependency_witness_count": 1})
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 1, 1)
}

// --- P6a-bis: authored GEOMETRY parse rejection → B ---------------------------

// P6a-bis: the authored CREATE TABLE (g GEOMETRY) is a real
// type-position parse rejection — the DROP lands B; premise
// consumable, unconsumed.
func TestT26SRSPrivP6aBisGeometryParseBlocked(t *testing.T) {
	fx := r3PrivilegeFixture()
	p := r3Harness(t, fx)
	result, err := r3PrivilegeRequest(t, p,
		"CREATE TABLE s.x (g GEOMETRY); DROP SPATIAL REFERENCE SYSTEM 100001;")
	if err == nil || !strings.Contains(err.Error(), "could not parse") {
		t.Fatalf("err=%v want the parser-failure diagnostic error", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("statements=%d want exactly the audited DROP", len(result.Statements))
	}
	stmt := result.Statements[0]
	if len(stmt.Findings) != 0 {
		t.Fatalf("findings=%+v want 0 on B", stmt.Findings)
	}
	r3ExactGap(t, stmt, []string{spec.SRSFactBatchEffects})
	if stmt.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("coverage=%v want unverified", stmt.Coverage.Status)
	}
	r3AssertReads(t, p, fx, 0, 0, 0, 0, 1, 1)
}

// --- P7: two SRIDs + real scope terminator -----------------------------------

// P7: REVOKE ends the observed scope — stmt1 consumable, stmt3
// suppressed → [D,P], 0 findings.
func TestT26SRSPrivP7ScopeTerminator(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.srs = map[uint32]string{100001: "SRID 100001", 100002: "SRID 100002"}
	p := r3Harness(t, fx)
	result, err := r3PrivilegeRequest(t, p,
		"DROP SPATIAL REFERENCE SYSTEM 100001; REVOKE SELECT ON *.* FROM 'r1'@'%'; DROP SPATIAL REFERENCE SYSTEM 100002;")
	// REVOKE is a recognized-but-unsupported statement — the request
	// still carries every audited statement's projection.
	if !errors.Is(err, audit.ErrUnsupportedStatement) {
		t.Fatalf("err=%v want ErrUnsupportedStatement (REVOKE is unsupported)", err)
	}
	if len(result.Statements) != 3 {
		t.Fatalf("statements=%d want 3", len(result.Statements))
	}
	r3S2Consumed(t, result.Statements[0], []string{spec.SRSFactColumnDependencies}, 100001, false)
	// stmt3: the scope end suppresses the witness — premise stays owed
	// exactly like unproven; zero findings including no privilege notice.
	stmt3 := result.Statements[2]
	if len(stmt3.Findings) != 0 {
		t.Fatalf("stmt3 findings=%+v want 0 — suppressed premise emits no notice", stmt3.Findings)
	}
	r3ExactGap(t, stmt3, []string{spec.SRSFactColumnDependencies, spec.SRSFactDropPrivilege})
	if stmt3.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("stmt3 coverage=%v want unverified", stmt3.Coverage.Status)
	}
	r3AssertReads(t, p, fx, 2, 2, 2, 2, 1, 1)
}

// --- P8: mixed families, both orders -----------------------------------------

// P8a: DROP SERVER → DROP SRS — the shared observation feeds each
// family's own projection; markers stay per-finding.
func TestT26SRSPrivP8aServerThenSRS(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.servers = map[string]bool{"s1": true}
	p := r3Harness(t, fx)
	result, err := r3PrivilegeRequest(t, p,
		"DROP SERVER s1; DROP SPATIAL REFERENCE SYSTEM 100001;")
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	r3MixedFamily(t, result, p, fx)
}

// P8b: DROP SRS → DROP SERVER — same shared observation, reversed order.
func TestT26SRSPrivP8bSRSThenServer(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.servers = map[string]bool{"s1": true}
	p := r3Harness(t, fx)
	result, err := r3PrivilegeRequest(t, p,
		"DROP SPATIAL REFERENCE SYSTEM 100001; DROP SERVER s1;")
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	r3MixedFamily(t, result, p, fx)
}

// r3MixedFamily asserts the shared-observation projections for either
// statement order: the SERVER notice carries outcome=conditional_drop
// AND conditional_audit=observed_account_new_session on the same
// finding; the SRS row carries its own consumed [D] projection.
func r3MixedFamily(t *testing.T, result report.Result, p *r3ProductProvider, fx *r3Fixture) {
	t.Helper()
	if len(result.Statements) != 2 {
		t.Fatalf("statements=%d want 2", len(result.Statements))
	}
	var srv, srs report.StatementResult
	for _, stmt := range result.Statements {
		if strings.Contains(strings.ToUpper(stmt.RawSQL), "DROP SERVER") {
			srv = stmt
		} else {
			srs = stmt
		}
	}
	serverConsumed := false
	for _, f := range srv.Findings {
		if f.RuleID != "ddl.mysql.server.drop.notice" {
			continue
		}
		marker, isString := f.Metadata["conditional_audit"].(string)
		if isString && marker == string(spec.PrivilegeAuditModeObservedAccountNewSession) &&
			f.Metadata["outcome"] == "conditional_drop" {
			serverConsumed = true
		}
	}
	if !serverConsumed {
		t.Fatalf("SERVER findings=%+v want outcome=conditional_drop + conditional_audit marker on one finding", srv.Findings)
	}
	r3S2Consumed(t, srs, []string{spec.SRSFactColumnDependencies}, 100001, false)
	// Reads: SRS 1/1/1/1; SERVER forward/mysql.servers/internal-version
	// =1/1/0 — every counted version query belongs to the SRS seam.
	srvFwd, srvQueries := r3ServerSeam(p, fx)
	if srvFwd != 1 || srvQueries != 1 || fx.count["ver"] != 1 {
		t.Fatalf("SERVER seam forward/mysql.servers/version=%d/%d/%d want 1/1/0 (ver belongs to SRS)", srvFwd, srvQueries, fx.count["ver"]-1)
	}
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 1, 1)
	if result.Verdict != report.VerdictReview {
		t.Fatalf("verdict=%v want review — whole-request observation", result.Verdict)
	}
}

// --- P9: qualification and typed exits ---------------------------------------

// P9a: the floor — resolved 8.0.4 qualifies, S2 consumable [D].
func TestT26SRSPrivP9aFloor(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.version = "8.0.4"
	fx.discoverVer = "8.0.4"
	p := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM 100001;", Dialect: spec.DialectMySQL, TargetVersion: "8.0.4",
		MetadataProvider:   p,
		PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	r3S2Consumed(t, result.Statements[0], []string{spec.SRSFactColumnDependencies}, 100001, false)
	r3Aggregate(t, result, report.CoverageUnverified)
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 1, 1)
}

// P9b: below-floor — mysql, no pin, observed+loader banner 8.0.3-log.
// The audit gate fails but the provider's Major>=8 catalog gate still
// reads: boundary report with zero findings/gaps; 1/1/1/1/0, Vd=1.
// The boundary apply mints srsExistenceStale (internal companion).
func TestT26SRSPrivP9bBelowFloorBoundary(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.version = "8.0.3-log"
	fx.discoverVer = "8.0.3-log"
	p := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM 100001;", Dialect: spec.DialectMySQL,
		MetadataProvider:   p,
		PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	r3Boundary(t, result, err, "spatial_reference_system")
	// Unified gate: zero catalog reads on the version boundary.
	r3AssertReads(t, p, fx, 0, 0, 0, 0, 0, 1)
}

// P9c: tidb dialect, no pin, recognized TiDB banner — vendor boundary;
// the MySQL SRS path never enters: loader-internal version N/A, all
// reads zero, Vd=1.
// R3-A4 attribution correction registered: the R2 internal "zero mint"
// observation came from a MySQL-parsed statement fed into a TiDB
// batchState — a mixed-dialect fixture, never a real TiDB
// vendor-boundary row and not a contract counterexample. The real
// TiDB parse path is proven by the internal companion
// TestT26SRSPrivP9cInternalVendorBoundaryMint (R3-A); the R2
// archive/logs/counterexample text stay preserved as historical.
func TestT26SRSPrivP9cTiDBVendorBoundary(t *testing.T) {
	fx := &r3Fixture{
		version:     "8.0.11-TiDB-v8.5.0",
		discoverVer: "8.0.11-TiDB-v8.5.0",
		srs:         map[uint32]string{100001: "SRID 100001"},
		account:     "app@%",
		grants:      []string{"GRANT SELECT, SUPER ON *.* TO `app`@`%`"},
	}
	p := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM 100001;", Dialect: spec.DialectTiDB,
		MetadataProvider:   p,
		PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	if err == nil || !errors.Is(err, audit.ErrUnsupportedStatement) {
		t.Fatalf("err=%v want ErrUnsupportedStatement (vendor boundary)", err)
	}
	if len(result.Statements) != 1 || len(result.Unsupported) != 1 {
		t.Fatalf("result=%+v want one statement + one unsupported entry", result)
	}
	entry := result.Unsupported[0]
	// The reason token and the metadata boundary value are separate
	// frozen fields — assert both (R2-A3).
	if entry.Reason != spec.UnsupportedVendorBoundaryReason {
		t.Fatalf("unsupported=%+v want the vendor-boundary reason", entry)
	}
	if entry.Metadata["boundary"] != "vendor" {
		t.Fatalf("unsupported metadata=%+v want boundary=vendor", entry.Metadata)
	}
	if result.Version == nil || result.Version.Product != spec.VersionProductTiDB {
		t.Fatalf("result.Version=%+v want resolved product TiDB", result.Version)
	}
	stmt := result.Statements[0]
	if len(stmt.Findings) != 0 || len(stmt.EvidenceGaps) != 0 {
		t.Fatalf("findings=%+v gaps=%+v want 0/0", stmt.Findings, stmt.EvidenceGaps)
	}
	if stmt.Coverage.Status != report.CoverageIncomplete || result.Coverage.Status != report.CoverageIncomplete ||
		result.Verdict != report.VerdictReview {
		t.Fatalf("coverage/verdict=%v/%v/%v want incomplete/incomplete/review", stmt.Coverage.Status, result.Coverage.Status, result.Verdict)
	}
	r3AssertReads(t, p, fx, 0, 0, 0, 0, 0, 1)
}

// P9d: an unregistered mode exits before discovery — typed error,
// everything N/A, Vd=0.
func TestT26SRSPrivP9dInvalidMode(t *testing.T) {
	fx := r3PrivilegeFixture()
	p := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM 100001;", Dialect: spec.DialectMySQL, TargetVersion: "8.0.36",
		MetadataProvider:   p,
		PrivilegeAuditMode: "bogus_mode",
	})
	if !errors.Is(err, audit.ErrInvalidPrivilegeMode) {
		t.Fatalf("err=%v want ErrInvalidPrivilegeMode", err)
	}
	if len(result.Statements) != 0 {
		t.Fatalf("result=%+v want N/A", result.Statements)
	}
	r3AssertReads(t, p, fx, 0, 0, 0, 0, 0, 0)
}

// P9e: malformed pin — typed error before discovery, Vd=0.
func TestT26SRSPrivP9eMalformedPin(t *testing.T) {
	fx := r3PrivilegeFixture()
	p := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM 100001;", Dialect: spec.DialectMySQL, TargetVersion: "8.0.x",
		MetadataProvider:   p,
		PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	if !errors.Is(err, audit.ErrInvalidTargetVersion) {
		t.Fatalf("err=%v want ErrInvalidTargetVersion", err)
	}
	if len(result.Statements) != 0 {
		t.Fatalf("result=%+v want N/A", result.Statements)
	}
	r3AssertReads(t, p, fx, 0, 0, 0, 0, 0, 0)
}

// P9f: mysql dialect, no pin, recognized TiDB banner — product
// mismatch typed exit after discovery: Vd=1, everything else zero.
// R2-A2: the request carries NO TargetVersion — the helper's hardcoded
// 8.0.36 pin would falsify the frozen input.
func TestT26SRSPrivP9fProductMismatch(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.discoverVer = "8.0.11-TiDB-v8.5.0"
	p := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM 100001;", Dialect: spec.DialectMySQL,
		MetadataProvider:   p,
		PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	if !errors.Is(err, audit.ErrDialectProductMismatch) {
		t.Fatalf("err=%v want ErrDialectProductMismatch", err)
	}
	if len(result.Statements) != 0 {
		t.Fatalf("result=%+v want N/A", result.Statements)
	}
	r3AssertReads(t, p, fx, 0, 0, 0, 0, 0, 1)
}

// P9g: pin 8.0.36 vs parsed observed 8.0.37-log — target-version
// mismatch typed exit after discovery, before resolvedVersion binds:
// Vd=1, everything else zero.
func TestT26SRSPrivP9gVersionMismatch(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.discoverVer = "8.0.37-log"
	p := r3Harness(t, fx)
	result, err := r3PrivilegeRequest(t, p, "DROP SPATIAL REFERENCE SYSTEM 100001;")
	if !errors.Is(err, audit.ErrTargetVersionMismatch) {
		t.Fatalf("err=%v want ErrTargetVersionMismatch", err)
	}
	if len(result.Statements) != 0 {
		t.Fatalf("result=%+v want N/A", result.Statements)
	}
	r3AssertReads(t, p, fx, 0, 0, 0, 0, 0, 1)
}

// --- P10: public carrier and configuration ----------------------------------

// P10a: reserved SRID consumed — exactly 2 findings (privilege +
// reserved), 1 gap [D]. R2-C3: the reserved advisory carries only
// {srid} — no conditional_audit or outcome leaks onto its carrier.
func TestT26SRSPrivP10aReservedConsumed(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.srs = map[uint32]string{4326: "WGS 84"}
	p := r3Harness(t, fx)
	result, err := r3PrivilegeRequest(t, p, "DROP SPATIAL REFERENCE SYSTEM 4326;")
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	r3S2Consumed(t, result.Statements[0], []string{spec.SRSFactColumnDependencies}, 4326, true)
	var reserved *rule.Finding
	for i := range result.Statements[0].Findings {
		if result.Statements[0].Findings[i].RuleID == "ddl.mysql.srs.drop.reserved" {
			reserved = &result.Statements[0].Findings[i]
		}
	}
	if reserved == nil {
		t.Fatal("the reserved advisory must co-fire on the consumed S2")
	}
	if reserved.Level != "notice" ||
		reserved.Message != "spatial reference system 4326 is in a reserved SRID range" {
		t.Fatalf("reserved finding=%+v want notice + fixed membership text", reserved)
	}
	if len(reserved.Metadata) != 1 || reserved.Metadata["srid"] != uint32(4326) {
		t.Fatalf("reserved metadata=%+v want exactly {srid}", reserved.Metadata)
	}
	r3Aggregate(t, result, report.CoverageUnverified)
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 1, 1)
}

// P10b: reserved SRID unproven — exactly 1 reserved finding, 1 gap
// [D,P], no privilege marker anywhere.
func TestT26SRSPrivP10bReservedUnproven(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.srs = map[uint32]string{4326: "WGS 84"}
	fx.grants = []string{"GRANT SELECT ON *.* TO `app`@`%`"}
	p := r3Harness(t, fx)
	result, err := r3PrivilegeRequest(t, p, "DROP SPATIAL REFERENCE SYSTEM 4326;")
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	stmt := result.Statements[0]
	r3S2(t, result, stmt,
		[]string{spec.SRSFactColumnDependencies, spec.SRSFactDropPrivilege}, 4326)
	// R2-C3: the unproven reserved advisory keeps its exact key set —
	// strict no-extra; no conditional_audit or outcome anywhere.
	var reserved *rule.Finding
	for i := range stmt.Findings {
		if stmt.Findings[i].RuleID == "ddl.mysql.srs.drop.reserved" {
			reserved = &stmt.Findings[i]
		}
	}
	if reserved == nil {
		t.Fatal("the reserved advisory must fire on the unproven S2")
	}
	if len(reserved.Metadata) != 1 || reserved.Metadata["srid"] != uint32(4326) ||
		reserved.Level != "notice" ||
		reserved.Message != "spatial reference system 4326 is in a reserved SRID range" {
		t.Fatalf("reserved finding=%+v want notice, fixed text, exactly {srid}", reserved)
	}
	blob, _ := json.Marshal(stmt)
	if strings.Contains(string(blob), "conditional_audit") {
		t.Fatal("unproven SRS statement carries conditional_audit")
	}
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 1, 1)
}

// r3PolicyFile writes a minimal rules-only policy for the given levels
// (empty string disables the rule outright).
func r3PolicyFile(t *testing.T, privilegeLevel string) string {
	t.Helper()
	level := privilegeLevel
	if level == "" {
		level = "notice"
	}
	body := fmt.Sprintf(`rules:
  ddl.mysql.srs.drop.srid0:
    enabled: true
    level: blocker
  ddl.mysql.srs.drop.dependency:
    enabled: true
    level: blocker
  ddl.mysql.srs.drop.reserved:
    enabled: true
    level: notice
  ddl.mysql.srs.drop.privilege:
    enabled: %v
    level: %s
`, privilegeLevel != "", level)
	path := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write policy: %v", err)
	}
	return path
}

// P10c: the new rule disabled — zero findings; the premise is still
// consumable internally, the gap stays [D], loader/apply unchanged.
func TestT26SRSPrivP10cRuleDisabled(t *testing.T) {
	fx := r3PrivilegeFixture()
	p := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM 100001;", Dialect: spec.DialectMySQL, TargetVersion: "8.0.36",
		MetadataProvider:   p,
		PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
		ConfigPath:         r3PolicyFile(t, ""),
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	stmt := result.Statements[0]
	if len(stmt.Findings) != 0 {
		t.Fatalf("findings=%+v want 0 — disabling the notice suppresses only the finding", stmt.Findings)
	}
	r3ExactGap(t, stmt, []string{spec.SRSFactColumnDependencies})
	if stmt.Coverage.Status != report.CoverageUnverified || result.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("coverage=%v/%v want unverified/unverified — the flag never upgrades coverage", stmt.Coverage.Status, result.Coverage.Status)
	}
	if result.Verdict != report.VerdictReview {
		t.Fatalf("verdict=%v want review", result.Verdict)
	}
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 1, 1)
}

// P10d: warning rewrite — same text/metadata, review verdict.
func TestT26SRSPrivP10dWarningRewrite(t *testing.T) {
	fx := r3PrivilegeFixture()
	p := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM 100001;", Dialect: spec.DialectMySQL, TargetVersion: "8.0.36",
		MetadataProvider:   p,
		PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
		ConfigPath:         r3PolicyFile(t, "warning"),
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	stmt := result.Statements[0]
	if len(stmt.Findings) != 1 || stmt.Findings[0].Level != "warning" {
		t.Fatalf("findings=%+v want one warning-level privilege notice", stmt.Findings)
	}
	assertJSONMarker(t, stmt, 100001)
	// R2-C4: the level rewrite must not re-owe P — the exact [D] gap
	// and unverified coverage hold at both scopes.
	r3ExactGap(t, stmt, []string{spec.SRSFactColumnDependencies})
	if stmt.Coverage.Status != report.CoverageUnverified || result.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("coverage=%v/%v want unverified/unverified", stmt.Coverage.Status, result.Coverage.Status)
	}
	if result.Verdict != report.VerdictReview {
		t.Fatalf("verdict=%v want review", result.Verdict)
	}
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 1, 1)
}

// P10e: blocker rewrite — same premise/Owed; reject; statement
// coverage stays unverified.
func TestT26SRSPrivP10eBlockerRewrite(t *testing.T) {
	fx := r3PrivilegeFixture()
	p := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM 100001;", Dialect: spec.DialectMySQL, TargetVersion: "8.0.36",
		MetadataProvider:   p,
		PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
		ConfigPath:         r3PolicyFile(t, "blocker"),
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	stmt := result.Statements[0]
	if len(stmt.Findings) != 1 || stmt.Findings[0].Level != "blocker" {
		t.Fatalf("findings=%+v want one blocker-level privilege notice", stmt.Findings)
	}
	assertJSONMarker(t, stmt, 100001)
	r3ExactGap(t, stmt, []string{spec.SRSFactColumnDependencies})
	if stmt.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("coverage=%v want unverified — blocker rewrite never upgrades", stmt.Coverage.Status)
	}
	if result.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("aggregate coverage=%v want unverified", result.Coverage.Status)
	}
	if result.Verdict != report.VerdictReject {
		t.Fatalf("verdict=%v want reject", result.Verdict)
	}
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 1, 1)
}

// assertJSONMarker verifies the privilege finding survives the JSON
// layer with the exact key set and token — the decoded numeric srid is
// checked as a number, never re-typed as uint32.
func assertJSONMarker(t *testing.T, stmt report.StatementResult, srid uint32) {
	t.Helper()
	blob, err := json.Marshal(stmt)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded struct {
		Findings []struct {
			RuleID   string         `json:"rule_id"`
			Level    string         `json:"level"`
			Message  string         `json:"message"`
			Metadata map[string]any `json:"metadata"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(blob, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	var found bool
	for _, f := range decoded.Findings {
		if f.RuleID != "ddl.mysql.srs.drop.privilege" {
			continue
		}
		found = true
		if len(f.Metadata) != 2 || f.Metadata["srid"] != float64(srid) ||
			f.Metadata["conditional_audit"] != "observed_account_new_session" {
			t.Fatalf("decoded metadata=%+v want exactly {srid:%d, conditional_audit:observed_account_new_session}", f.Metadata, srid)
		}
		wantMessage := fmt.Sprintf("spatial reference system %d drop privilege premise discharged under the observed_account_new_session conditional audit", srid)
		if f.Message != wantMessage {
			t.Fatalf("decoded message=%q want exact %q", f.Message, wantMessage)
		}
	}
	if !found {
		t.Fatalf("decoded findings=%+v want the privilege notice present", decoded.Findings)
	}
}

// P10f: the mode arrives only through the legacy request.Metadata bag —
// same consumable projection as P2.
func TestT26SRSPrivP10fLegacyMetadataMode(t *testing.T) {
	fx := r3PrivilegeFixture()
	p := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM 100001;", Dialect: spec.DialectMySQL, TargetVersion: "8.0.36",
		MetadataProvider: p,
		Metadata:         &audit.MetadataRequest{PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession},
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	r3S2Consumed(t, result.Statements[0], []string{spec.SRSFactColumnDependencies}, 100001, false)
	r3Aggregate(t, result, report.CoverageUnverified)
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 1, 1)
}

// P10g: IF EXISTS — same premise/arm/Owed/notice semantics as P2.
func TestT26SRSPrivP10gIfExists(t *testing.T) {
	fx := r3PrivilegeFixture()
	p := r3Harness(t, fx)
	result, err := r3PrivilegeRequest(t, p, "DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001;")
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	stmt := result.Statements[0]
	r3S2Consumed(t, stmt, []string{spec.SRSFactColumnDependencies}, 100001, false)
	if stmt.NormalizedSQL != "DROP SPATIAL REFERENCE SYSTEM IF EXISTS 100001" {
		t.Fatalf("normalized_sql=%q want authored IF EXISTS form", stmt.NormalizedSQL)
	}
	r3Aggregate(t, result, report.CoverageUnverified)
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 1, 1)
}

// --- P6a request/API twin -----------------------------------------------------

// P6a request/API twin (R2-A1): `DROP TABLE s.t; DROP SRS 100001` —
// the DROP TABLE binds through the accepted test-only LoadTableSnapshot
// seam (tblTFixture shape: s.t Exists=true, Columns {g:geometry}) while
// discovery, the privilege loader, and the SRS resolver ride the real
// mysqlmeta.NewProvider chain. DROP TABLE's own apply mints
// srsDepsStale; the positive dependency witness is demoted to S2 with
// the premise still consumable → the family's own notice + one [D]
// dependency-owned gap. Non-SRS reads and the DROP TABLE's own public
// output stay outside the frozen vector's declared scope.
func TestT26SRSPrivP6aRequestStaleDemotion(t *testing.T) {
	fx := r3PrivilegeFixture()
	fx.deps = []r3DepRow{{schema: "s", table: "t", column: "g", srid: 100001}}
	p := r3Harness(t, fx)
	provider := &r3TableFixtureProvider{p: p, tables: map[string]*spec.TableSnapshot{
		"s.t": {Schema: "s", Table: &spec.Table{Schema: "s", Name: "t"}, Exists: true,
			Columns: []spec.Column{{Name: "g", Type: "geometry"}}},
	}}
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL:     "DROP TABLE s.t; DROP SPATIAL REFERENCE SYSTEM 100001;",
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
	stmt := result.Statements[1]
	// R3-B1: the fixed notice shape — exactly one privilege finding
	// carrying level=notice, the exact discharged-premise message, and
	// the typed two-key metadata {srid:uint32, conditional_audit:string};
	// dependency-owned [D] gap; unverified coverage. r3S2Consumed
	// asserts all of it — count+RuleID alone is insufficient.
	r3S2Consumed(t, stmt, []string{spec.SRSFactColumnDependencies}, 100001, false)
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 1, 1)
}

var _ = fmt.Sprintf // keep fmt live for helpers
