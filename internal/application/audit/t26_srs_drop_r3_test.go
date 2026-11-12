package audit_test

// Package audit_test: T26-SRS-DROP-EXECUTION-01 R3 — product-provider
// witnesses. The R1/R2 harness could not import mysqlmeta (its compile
// assertion on audit.MetadataProvider closes an import cycle inside
// `package audit`); this external test package imports both sides and
// drives the REAL product loader — ResolveObject/LoadInstanceFacts/
// LoadMySQLServerDropPrivilege all delegate to
// mysqlmeta.NewProvider(db) over a stub database/sql driver, so every
// Qexist/Qdeps/Qo count is a query the product loader actually issued.
// The r3Provider wrapper only counts calls and injects sentinels at
// the named seams (R3-A.2); it never re-implements loader SQL.
//
// Seam columns follow the contract order R/Qexist/Qdeps/Qo/L with Vd
// (LoadInstanceFacts call count) reported separately — Vd counts method
// calls, never claims to be a database query (R3-A.5).
//
// Frozen expectations: docs/verification/t26-srs-drop-contract-01/case-matrix.md
// (accepted supplement included), candidate chain
// d68f4ab7 → 3c74445c → 35d41f5b → f3a70987.

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	audit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	mysqlmeta "github.com/Fanduzi/DeltaScope/internal/infrastructure/metadata/mysql"
)

// --- stub driver -----------------------------------------------------------

type r3DepRow struct {
	schema, table, column string
	srid                  uint64
}

// r3Fixture scripts every SQL answer the product loader can issue and
// counts attempts per seam. failOn maps a seam kind to its sentinel;
// a failing query is still counted as an attempt.
type r3Fixture struct {
	version     string            // select version() banner (Qo)
	discoverVer string            // 'version' row inside SHOW VARIABLES
	srs         map[uint32]string // srid -> srs_name
	deps        []r3DepRow
	servers     map[string]bool
	account     string
	grants      []string
	failOn      map[string]error
	count       map[string]int
	// depsScript overrides deps per "deps"-kind query — the nth deps
	// query (0-indexed, counting full-path AND direct-reader reads
	// alike) serves depsScript[n] filtered by SRID; indices past the
	// script answer empty. Ordered-publication freshness rows script
	// changing witness answers across one batch (OP-4a).
	depsScript [][]r3DepRow
	depsCalls  int
}

// r3Classify maps each product-loader query to its seam kind. Order
// matters: "show grants for current_user()" contains current_user().
func r3Classify(query string) string {
	switch {
	case strings.Contains(query, "show grants"):
		return "grants"
	case strings.Contains(query, "current_user()"):
		return "user"
	case strings.Contains(query, "version()"):
		return "ver"
	case strings.Contains(query, "st_geometry_columns"):
		return "deps"
	case strings.Contains(query, "st_spatial_reference_systems"):
		return "exist"
	case strings.Contains(query, "mysql.servers"):
		return "srv"
	case strings.Contains(query, "variables"):
		return "vars"
	default:
		return "other"
	}
}

type r3Driver struct{}
type r3Connector struct{ fx *r3Fixture }
type r3Conn struct{ fx *r3Fixture }
type r3Rows struct {
	cols []string
	rows [][]driver.Value
	i    int
}

func (r3Driver) Open(string) (driver.Conn, error) {
	return nil, errors.New("r3 driver: connections come from r3Connector")
}

func (c *r3Connector) Connect(context.Context) (driver.Conn, error) {
	return &r3Conn{fx: c.fx}, nil
}

func (c *r3Connector) Driver() driver.Driver { return r3Driver{} }

func (c *r3Conn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (c *r3Conn) Close() error                        { return nil }
func (c *r3Conn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }

func (c *r3Conn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	kind := r3Classify(query)
	c.fx.count[kind]++
	if sentinel, ok := c.fx.failOn[kind]; ok {
		return nil, sentinel
	}
	switch kind {
	case "ver":
		return &r3Rows{cols: []string{"version()"}, rows: [][]driver.Value{{c.fx.version}}}, nil
	case "vars":
		var rows [][]driver.Value
		if c.fx.discoverVer != "" {
			rows = append(rows, []driver.Value{"version", c.fx.discoverVer})
		}
		return &r3Rows{cols: []string{"Variable_name", "Value"}, rows: rows}, nil
	case "exist":
		srid := args[0].Value.(int64)
		if name, ok := c.fx.srs[uint32(srid)]; ok {
			return &r3Rows{cols: []string{"srs_id", "srs_name"}, rows: [][]driver.Value{{srid, name}}}, nil
		}
		return &r3Rows{cols: []string{"srs_id", "srs_name"}}, nil
	case "deps":
		srid := args[0].Value.(int64)
		deps := c.fx.deps
		if c.fx.depsScript != nil {
			i := c.fx.depsCalls
			c.fx.depsCalls++
			if i < len(c.fx.depsScript) {
				deps = c.fx.depsScript[i]
			} else {
				deps = nil
			}
		}
		var rows [][]driver.Value
		for _, dep := range deps {
			if dep.srid == uint64(srid) {
				rows = append(rows, []driver.Value{dep.schema, dep.table, dep.column})
			}
		}
		return &r3Rows{cols: []string{"table_schema", "table_name", "column_name"}, rows: rows}, nil
	case "srv":
		name, _ := args[0].Value.(string)
		if c.fx.servers[name] {
			return &r3Rows{cols: []string{"server_name"}, rows: [][]driver.Value{{name}}}, nil
		}
		return &r3Rows{cols: []string{"server_name"}}, nil
	case "user":
		if c.fx.account == "" {
			// P4b: a successful-but-empty current_user() answer —
			// zero rows, no account bound.
			return &r3Rows{cols: []string{"current_user()"}}, nil
		}
		return &r3Rows{cols: []string{"current_user()"}, rows: [][]driver.Value{{c.fx.account}}}, nil
	case "grants":
		var rows [][]driver.Value
		for _, g := range c.fx.grants {
			rows = append(rows, []driver.Value{g})
		}
		return &r3Rows{cols: []string{"Grants"}, rows: rows}, nil
	}
	return &r3Rows{cols: []string{"x"}}, nil
}

func (r *r3Rows) Columns() []string { return r.cols }
func (r *r3Rows) Close() error      { return nil }
func (r *r3Rows) Next(dest []driver.Value) error {
	if r.i >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.i])
	r.i++
	return nil
}

// --- product provider wrapper ------------------------------------------------

// r3ProductProvider delegates every metadata method to the real
// mysqlmeta.Provider — the "product loader path" — and only counts
// calls / injects sentinels around it (R3-A.2).
type r3ProductProvider struct {
	inner       *mysqlmeta.Provider
	forwards    []spec.ObjectLookupRequest // full-forward R seam, per request type
	depsForward int                        // direct deps-reader R seam (incl. failures)
	vd          int                        // LoadInstanceFacts calls
	privCalls   int                        // L seam
	resolveErr  error
	discoverErr error
}

func (p *r3ProductProvider) LoadInstanceFacts(ctx context.Context, dialect spec.Dialect, schema string) (*spec.InstanceFacts, error) {
	p.vd++
	if p.discoverErr != nil {
		return nil, p.discoverErr
	}
	return p.inner.LoadInstanceFacts(ctx, dialect, schema)
}

func (p *r3ProductProvider) LoadTableSnapshot(ctx context.Context, dialect spec.Dialect, schema, table string) (*spec.TableSnapshot, error) {
	return p.inner.LoadTableSnapshot(ctx, dialect, schema, table)
}

func (p *r3ProductProvider) ResolveObject(ctx context.Context, dialect spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	p.forwards = append(p.forwards, req)
	if p.resolveErr != nil {
		return nil, p.resolveErr
	}
	return p.inner.ResolveObject(ctx, dialect, req)
}

func (p *r3ProductProvider) LoadMySQLServerDropPrivilege(ctx context.Context, dialect spec.Dialect) (*spec.MySQLServerDropPrivilegeObservation, error) {
	p.privCalls++
	return p.inner.LoadMySQLServerDropPrivilege(ctx, dialect)
}

// ResolveMySQLSRSDependencies forwards the ordered channel's deps-only
// capability to the real product provider. The forwarding attempt itself
// is counted on the R seam (including failed attempts): the contract's R
// is measured full-forwards + direct deps-forwards, while the Qo/Qdeps
// SQL reads still land on the fixture's independent seam counters and
// NO ResolveObject forward is recorded (the direct method bypasses the
// objectCache seam, exactly as the contract requires).
func (p *r3ProductProvider) ResolveMySQLSRSDependencies(ctx context.Context, srid uint32) (*spec.SRSDependencyFact, error) {
	p.depsForward++
	return p.inner.ResolveMySQLSRSDependencies(ctx, srid)
}

// r3Harness builds the real product provider over the stub driver.
func r3Harness(t *testing.T, fx *r3Fixture) *r3ProductProvider {
	t.Helper()
	if fx.count == nil {
		fx.count = map[string]int{}
	}
	db := sql.OpenDB(&r3Connector{fx: fx})
	t.Cleanup(func() { db.Close() })
	return &r3ProductProvider{inner: mysqlmeta.NewProvider(db)}
}

// r3AssertReads checks the ordered R/Qexist/Qdeps/Qo/L vector plus Vd.
// wantVd < 0 means N/A (providerless row — no discovery surface).
func r3AssertReads(t *testing.T, p *r3ProductProvider, fx *r3Fixture, wantR, wantExist, wantDeps, wantQo, wantL, wantVd int) {
	t.Helper()
	r := p.depsForward
	for _, req := range p.forwards {
		if req.Type == "mysql_spatial_reference_system" {
			r++
		}
	}
	if r != wantR || fx.count["exist"] != wantExist || fx.count["deps"] != wantDeps ||
		fx.count["ver"] != wantQo || p.privCalls != wantL {
		t.Fatalf("reads R/Qexist/Qdeps/Qo/L=%d/%d/%d/%d/%d want %d/%d/%d/%d/%d (queries=%v)",
			r, fx.count["exist"], fx.count["deps"], fx.count["ver"], p.privCalls,
			wantR, wantExist, wantDeps, wantQo, wantL, fx.count)
	}
	if wantVd >= 0 && p.vd != wantVd {
		t.Fatalf("Vd=%d want %d", p.vd, wantVd)
	}
}

func r3Facts(stmt report.StatementResult) []string {
	var out []string
	for _, gap := range stmt.EvidenceGaps {
		out = append(out, gap.RequiredFacts...)
	}
	return out
}

// r3Boundary asserts the frozen boundary-row shape: ErrUnsupportedStatement,
// exactly one unsupported entry, exactly one statement carrying
// incomplete coverage with ZERO findings and ZERO gaps, aggregate
// incomplete, review verdict.
func r3Boundary(t *testing.T, result report.Result, err error, object string) {
	t.Helper()
	if !errors.Is(err, audit.ErrUnsupportedStatement) {
		t.Fatalf("err=%v want ErrUnsupportedStatement", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("statements=%d want exactly 1 on a version-boundary row", len(result.Statements))
	}
	stmt := result.Statements[0]
	if len(stmt.Findings) != 0 {
		t.Fatalf("findings=%+v want ZERO on a boundary row", stmt.Findings)
	}
	if len(stmt.EvidenceGaps) != 0 {
		t.Fatalf("gaps=%+v want ZERO on a boundary row", stmt.EvidenceGaps)
	}
	if stmt.Coverage.Status != report.CoverageIncomplete {
		t.Fatalf("statement coverage=%v want incomplete", stmt.Coverage.Status)
	}
	if len(result.Unsupported) != 1 {
		t.Fatalf("unsupported=%+v want exactly one entry", result.Unsupported)
	}
	entry := result.Unsupported[0]
	if entry.Reason != spec.UnsupportedUnauditedReason ||
		entry.Metadata["boundary"] != "unaudited" ||
		entry.Metadata["object"] != object ||
		entry.Metadata["gate"] != "version" {
		t.Fatalf("unsupported=%+v want the frozen unaudited/version gate for %s", entry, object)
	}
	if result.Coverage.Status != report.CoverageIncomplete {
		t.Fatalf("aggregate coverage=%v want incomplete", result.Coverage.Status)
	}
	if result.Verdict != report.VerdictReview {
		t.Fatalf("verdict=%v want review", result.Verdict)
	}
}

// r3S2Statement asserts the exact statement-level unproven projection:
// one dependency-owned unknown_srs_state gap carrying the exact ordered
// required_facts; findings strictly counted — zero when the authored
// SRID is not reserved (wantReservedSRID=0), else exactly one frozen
// reserved notice with RuleID/notice-level/exact message/authored SRID
// and never a refusal prediction; statement coverage unverified.
func r3S2Statement(t *testing.T, stmt report.StatementResult, wantFacts []string, wantReservedSRID uint32) {
	t.Helper()
	if len(stmt.EvidenceGaps) != 1 {
		t.Fatalf("gaps=%+v want exactly 1", stmt.EvidenceGaps)
	}
	g := stmt.EvidenceGaps[0]
	if g.RuleID != "ddl.mysql.srs.drop.dependency" || g.ReasonCode != "unknown_srs_state" {
		t.Fatalf("gap=%+v want dependency-owned unknown_srs_state", g)
	}
	if !reflect.DeepEqual(g.RequiredFacts, wantFacts) {
		t.Fatalf("gap facts=%v want ordered %v", g.RequiredFacts, wantFacts)
	}
	wantFindings := 0
	if wantReservedSRID != 0 {
		wantFindings = 1
	}
	if len(stmt.Findings) != wantFindings {
		t.Fatalf("findings=%+v want exactly %d (reserved SRID %d)", stmt.Findings, wantFindings, wantReservedSRID)
	}
	if wantReservedSRID != 0 {
		f := stmt.Findings[0]
		if f.RuleID != "ddl.mysql.srs.drop.reserved" || f.Level != rule.LevelNotice {
			t.Fatalf("finding=%+v want the frozen reserved notice", f)
		}
		if srid, ok := f.Metadata["srid"]; !ok || srid != wantReservedSRID {
			t.Fatalf("finding metadata=%+v want srid=%d present", f.Metadata, wantReservedSRID)
		}
		wantMessage := fmt.Sprintf("spatial reference system %d is in a reserved SRID range", wantReservedSRID)
		if f.Message != wantMessage {
			t.Fatalf("reserved message=%q want exact %q", f.Message, wantMessage)
		}
	}
	if stmt.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("statement coverage=%v want unverified", stmt.Coverage.Status)
	}
}

// r3S2 asserts the exact unproven public projection at statement AND
// aggregate level: the r3S2Statement shape plus aggregate unverified
// coverage and review verdict. For rows inside multi-statement batches
// use r3S2Statement on the SRS statement alone.
func r3S2(t *testing.T, result report.Result, stmt report.StatementResult, wantFacts []string, wantReservedSRID uint32) {
	t.Helper()
	r3S2Statement(t, stmt, wantFacts, wantReservedSRID)
	if result.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("aggregate coverage=%v want unverified", result.Coverage.Status)
	}
	if result.Verdict != report.VerdictReview {
		t.Fatalf("verdict=%v want review", result.Verdict)
	}
}

const r3SQL = "DROP SPATIAL REFERENCE SYSTEM 4326;"

// r3PrivilegeNotice is the strict conditional-marker check the
// privilege contract fixes (T26-SRS-DROP-PRIVILEGE-CONTRACT-01 §3):
// exactly one ddl.mysql.srs.drop.privilege finding on the statement —
// notice level, the exact discharged-premise message, and typed
// metadata carrying EXACTLY {srid:uint32, conditional_audit:string}.
// A missing key, nil, non-string, empty, or wrong token all fail;
// "!= \"\"" is never the criterion.
func r3PrivilegeNotice(t *testing.T, stmt report.StatementResult, srid uint32) {
	t.Helper()
	var notice *rule.Finding
	for i := range stmt.Findings {
		if stmt.Findings[i].RuleID == "ddl.mysql.srs.drop.privilege" {
			if notice != nil {
				t.Fatalf("duplicate privilege notice: %+v", stmt.Findings)
			}
			notice = &stmt.Findings[i]
		}
	}
	if notice == nil {
		t.Fatalf("no ddl.mysql.srs.drop.privilege finding: %+v", stmt.Findings)
	}
	if notice.Level != rule.LevelNotice {
		t.Fatalf("privilege notice level=%v want notice", notice.Level)
	}
	wantMessage := fmt.Sprintf("spatial reference system %d drop privilege premise discharged under the observed_account_new_session conditional audit", srid)
	if notice.Message != wantMessage {
		t.Fatalf("privilege message=%q want exact %q", notice.Message, wantMessage)
	}
	if len(notice.Metadata) != 2 {
		t.Fatalf("privilege metadata=%+v want exactly {srid, conditional_audit}", notice.Metadata)
	}
	gotSRID, ok := notice.Metadata["srid"].(uint32)
	if !ok || gotSRID != srid {
		t.Fatalf("privilege metadata srid=%v(%T) want typed uint32 %d", notice.Metadata["srid"], notice.Metadata["srid"], srid)
	}
	marker, ok := notice.Metadata["conditional_audit"].(string)
	if !ok || marker != string(spec.PrivilegeAuditModeObservedAccountNewSession) {
		t.Fatalf("conditional_audit=%v(%T) want exact string observed_account_new_session",
			notice.Metadata["conditional_audit"], notice.Metadata["conditional_audit"])
	}
}

// r3S2Consumed asserts the full consumed-S2 statement projection
// (T26-SRS-DROP-PRIVILEGE-EXECUTION-01): exactly one dependency-owned
// unknown_srs_state gap with the exact ordered facts; strictly counted
// findings — the family's privilege notice always, plus the frozen
// reserved advisory only when wantReserved; statement coverage stays
// unverified (consumption never upgrades unproven).
func r3S2Consumed(t *testing.T, stmt report.StatementResult, wantFacts []string, srid uint32, wantReserved bool) {
	t.Helper()
	if len(stmt.EvidenceGaps) != 1 {
		t.Fatalf("gaps=%+v want exactly 1", stmt.EvidenceGaps)
	}
	g := stmt.EvidenceGaps[0]
	if g.RuleID != "ddl.mysql.srs.drop.dependency" || g.ReasonCode != "unknown_srs_state" {
		t.Fatalf("gap=%+v want dependency-owned unknown_srs_state", g)
	}
	if !reflect.DeepEqual(g.RequiredFacts, wantFacts) {
		t.Fatalf("gap facts=%v want ordered %v", g.RequiredFacts, wantFacts)
	}
	wantFindings := 1
	if wantReserved {
		wantFindings = 2
	}
	if len(stmt.Findings) != wantFindings {
		t.Fatalf("findings=%+v want exactly %d (privilege notice + reserved=%v)", stmt.Findings, wantFindings, wantReserved)
	}
	r3PrivilegeNotice(t, stmt, srid)
	if wantReserved {
		var reserved *rule.Finding
		for i := range stmt.Findings {
			if stmt.Findings[i].RuleID == "ddl.mysql.srs.drop.reserved" {
				reserved = &stmt.Findings[i]
			}
		}
		if reserved == nil {
			t.Fatalf("findings=%+v want the reserved advisory present", stmt.Findings)
		}
		if fmt.Sprintf("%v", reserved.Metadata["srid"]) != fmt.Sprintf("%d", srid) ||
			reserved.Message != fmt.Sprintf("spatial reference system %d is in a reserved SRID range", srid) {
			t.Fatalf("reserved advisory=%+v want the frozen shape", *reserved)
		}
	}
	if stmt.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("statement coverage=%v want unverified", stmt.Coverage.Status)
	}
}

// r3RefusedDependency asserts the complete refused_dependency public
// projection (R5-B.3 — reused verbatim on every dependency-refusal row):
// exactly one dependency blocker carrying outcome=refused_dependency,
// the authored SRID, dependency_witness_count and the frozen message;
// zero evidence gaps; statement+aggregate complete; reject verdict.
func r3RefusedDependency(t *testing.T, result report.Result, stmt report.StatementResult, srid uint32, wantWitnesses int) {
	t.Helper()
	if len(stmt.Findings) != 1 {
		t.Fatalf("findings=%+v want exactly one", stmt.Findings)
	}
	f := stmt.Findings[0]
	if f.RuleID != "ddl.mysql.srs.drop.dependency" || f.Level != rule.LevelBlocker {
		t.Fatalf("finding=%+v want the dependency blocker", f)
	}
	if f.Metadata["outcome"] != "refused_dependency" || f.Metadata["dependency_witness_count"] != wantWitnesses {
		t.Fatalf("metadata=%+v want outcome=refused_dependency witness_count=%d", f.Metadata, wantWitnesses)
	}
	wantMessage := fmt.Sprintf("spatial reference system %d is used by %d visible geometry column(s)", srid, wantWitnesses)
	if fmt.Sprintf("%v", f.Metadata["srid"]) != fmt.Sprintf("%d", srid) || f.Message != wantMessage {
		t.Fatalf("finding srid=%v message=%q want the frozen refused_dependency shape", f.Metadata["srid"], f.Message)
	}
	if len(stmt.EvidenceGaps) != 0 {
		t.Fatalf("gaps=%+v want absent — positive witness discharges existence", stmt.EvidenceGaps)
	}
	if stmt.Coverage.Status != report.CoverageComplete || result.Coverage.Status != report.CoverageComplete {
		t.Fatalf("coverage=%v/%v want complete/complete", stmt.Coverage.Status, result.Coverage.Status)
	}
	if result.Verdict != report.VerdictReject {
		t.Fatalf("verdict=%v want reject", result.Verdict)
	}
}

// --- R3-B.A: request/API basic rows ------------------------------------------

// E1.1/E4.1: positive dependency witness through the product loader →
// refused_dependency, exact public projection, complete coverage.
func TestT26SRSR3RefusedDependencyRequest(t *testing.T) {
	fx := &r3Fixture{
		version: "8.0.36", discoverVer: "8.0.36",
		srs:  map[uint32]string{4326: "WGS 84"},
		deps: []r3DepRow{{schema: "s", table: "t", column: "g", srid: 4326}},
	}
	p := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: r3SQL, Dialect: spec.DialectMySQL, TargetVersion: "8.0.36", MetadataProvider: p,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	stmt := result.Statements[0]
	r3RefusedDependency(t, result, stmt, 4326, 1)
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 0, 1)
}

// E4.1: zero-row existence + a positive dependency witness still
// refuses — the witness alone discharges existence (no [E] in owed).
func TestT26SRSR3ZeroRowPositiveDepsRefuses(t *testing.T) {
	fx := &r3Fixture{
		version: "8.0.36", discoverVer: "8.0.36",
		deps: []r3DepRow{{schema: "s", table: "t", column: "g", srid: 4326}},
	}
	p := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: r3SQL, Dialect: spec.DialectMySQL, TargetVersion: "8.0.36", MetadataProvider: p,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	stmt := result.Statements[0]
	r3RefusedDependency(t, result, stmt, 4326, 1)
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 0, 1)
}

// E1.2 + E12.1 half: IF EXISTS keeps the refusal arm; normalized_sql
// keeps the authored IF EXISTS; no if_exists key exists anywhere.
func TestT26SRSR3IfExistsRequest(t *testing.T) {
	fx := &r3Fixture{
		version: "8.0.36", discoverVer: "8.0.36",
		srs:  map[uint32]string{4326: "WGS 84"},
		deps: []r3DepRow{{schema: "s", table: "t", column: "g", srid: 4326}},
	}
	p := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL:     "DROP SPATIAL REFERENCE SYSTEM IF EXISTS 4326;",
		Dialect: spec.DialectMySQL, TargetVersion: "8.0.36", MetadataProvider: p,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	stmt := result.Statements[0]
	r3RefusedDependency(t, result, stmt, 4326, 1)
	if stmt.NormalizedSQL != "DROP SPATIAL REFERENCE SYSTEM IF EXISTS 4326" {
		t.Fatalf("normalized_sql=%q want authored IF EXISTS form", stmt.NormalizedSQL)
	}
	blob, _ := json.Marshal(result)
	if strings.Contains(string(blob), "if_exists") {
		t.Fatal("serialized result carries an if_exists key")
	}
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 0, 1)
}

// E2.1: exists + empty deps → unproven; exactly one reserved notice in
// the frozen shape; gaps [D,P]; statement+aggregate unverified; review.
func TestT26SRSR3UnprovenReservedRequest(t *testing.T) {
	fx := &r3Fixture{
		version: "8.0.36", discoverVer: "8.0.36",
		srs: map[uint32]string{4326: "WGS 84"},
	}
	p := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: r3SQL, Dialect: spec.DialectMySQL, TargetVersion: "8.0.36", MetadataProvider: p,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	stmt := result.Statements[0]
	r3S2(t, result, stmt, []string{spec.SRSFactColumnDependencies, spec.SRSFactDropPrivilege}, 4326)
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 0, 1)
}

// E3.1: providerless S2 — [E,D,P], findings absent, review, zero reads.
func TestT26SRSR3UnprovenProviderless(t *testing.T) {
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: "DROP SPATIAL REFERENCE SYSTEM 100001;", Dialect: spec.DialectMySQL, TargetVersion: "8.0.36",
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	stmt := result.Statements[0]
	r3S2(t, result, stmt,
		[]string{spec.SRSFactExistence, spec.SRSFactColumnDependencies, spec.SRSFactDropPrivilege}, 0)
}

// E3.2 strengthened (R3-B.2): zero-row existence + empty deps through
// the product loader → [E,D,P] exact length+order, exactly one reserved
// notice, aggregate unverified. The Qo banner 8.0.3 is supported
// (major >= 8) but outside the proven-absence domain (patch < 4), so
// the clean zero-row answer stays unknown — T26-SRS-DROP-PROVEN-
// ABSENT-EXECUTION-01.
func TestT26SRSR3ZeroRowProductLoader(t *testing.T) {
	fx := &r3Fixture{version: "8.0.3-log", discoverVer: "8.0.36"}
	p := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: r3SQL, Dialect: spec.DialectMySQL, TargetVersion: "8.0.36", MetadataProvider: p,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	stmt := result.Statements[0]
	r3S2(t, result, stmt,
		[]string{spec.SRSFactExistence, spec.SRSFactColumnDependencies, spec.SRSFactDropPrivilege}, 4326)
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 0, 1)
}

// E5.1 + E5.2: SRID 0 refuses through the request layer — exact finding
// shape, advisory suppressed, IF EXISTS identical.
func TestT26SRSR3SRID0Request(t *testing.T) {
	for _, sqlText := range []string{
		"DROP SPATIAL REFERENCE SYSTEM 0;",
		"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 0;",
	} {
		fx := &r3Fixture{version: "8.0.36", discoverVer: "8.0.36", srs: map[uint32]string{0: "0"}}
		p := r3Harness(t, fx)
		result, err := audit.AuditSQL(context.Background(), audit.Request{
			SQL: sqlText, Dialect: spec.DialectMySQL, TargetVersion: "8.0.36", MetadataProvider: p,
		})
		if err != nil {
			t.Fatalf("%q audit: %v", sqlText, err)
		}
		stmt := result.Statements[0]
		if len(stmt.Findings) != 1 {
			t.Fatalf("%q findings=%+v want exactly one", sqlText, stmt.Findings)
		}
		f := stmt.Findings[0]
		if f.RuleID != "ddl.mysql.srs.drop.srid0" || f.Level != rule.LevelBlocker ||
			f.Metadata["outcome"] != "refused_srid0" {
			t.Fatalf("%q finding=%+v want the srid0 blocker", sqlText, f)
		}
		if fmt.Sprintf("%v", f.Metadata["srid"]) != "0" || f.Message != "dropping spatial reference system SRID 0 is not allowed" {
			t.Fatalf("%q finding srid=%v message=%q want the frozen E5.1 shape", sqlText, f.Metadata["srid"], f.Message)
		}
		if len(stmt.EvidenceGaps) != 0 {
			t.Fatalf("%q gaps=%+v want absent", sqlText, stmt.EvidenceGaps)
		}
		if stmt.Coverage.Status != report.CoverageComplete || result.Coverage.Status != report.CoverageComplete ||
			result.Verdict != report.VerdictReject {
			t.Fatalf("%q coverage=%v/%v verdict=%v want complete/complete/reject",
				sqlText, stmt.Coverage.Status, result.Coverage.Status, result.Verdict)
		}
		r3AssertReads(t, p, fx, 1, 1, 1, 1, 0, 1)
	}
}

// E5.3: the authored max-uint32 SRID through the real loader —
// exist(empty) deps([]) → unproven [E,D,P], findings absent.
// Qo 8.0.3 keeps the zero-row answer outside the D-proof.
func TestT26SRSR3MaxSRIDRequest(t *testing.T) {
	fx := &r3Fixture{version: "8.0.3-log", discoverVer: "8.0.36"}
	p := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL:     "DROP SPATIAL REFERENCE SYSTEM 4294967295;",
		Dialect: spec.DialectMySQL, TargetVersion: "8.0.36", MetadataProvider: p,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	stmt := result.Statements[0]
	r3S2(t, result, stmt,
		[]string{spec.SRSFactExistence, spec.SRSFactColumnDependencies, spec.SRSFactDropPrivilege}, 0)
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 0, 1)
}

// E5.4/E5.5/E5.6a–h (frozen exist(empty)/deps([]) form): every
// reserved-range cell and boundary neighbour through the product
// loader → unproven [E,D,P] with advisory present/absent per cell.
func TestT26SRSR3ReservedCellsEmptyEvidence(t *testing.T) {
	cells := []struct {
		srid    uint32
		advised bool
	}{
		{32767, true}, {32768, false}, // E5.4 / E5.5
		{60000000, true}, {59999999, false}, {69999999, true}, {70000000, false}, // E5.6a–d
		{2000000000, true}, {1999999999, false}, {2147483647, true}, {2147483648, false}, // E5.6e–h
	}
	for _, cell := range cells {
		// Qo 8.0.3 — supported read outside the proven-absence domain —
		// keeps every zero-row cell on the unproven arm.
		fx := &r3Fixture{version: "8.0.3-log", discoverVer: "8.0.36"}
		p := r3Harness(t, fx)
		result, err := audit.AuditSQL(context.Background(), audit.Request{
			SQL:     fmt.Sprintf("DROP SPATIAL REFERENCE SYSTEM %d;", cell.srid),
			Dialect: spec.DialectMySQL, TargetVersion: "8.0.36", MetadataProvider: p,
		})
		if err != nil {
			t.Fatalf("srid %d audit: %v", cell.srid, err)
		}
		stmt := result.Statements[0]
		var wantReserved uint32
		if cell.advised {
			wantReserved = cell.srid
		}
		r3S2(t, result, stmt,
			[]string{spec.SRSFactExistence, spec.SRSFactColumnDependencies, spec.SRSFactDropPrivilege},
			wantReserved)
		r3AssertReads(t, p, fx, 1, 1, 1, 1, 0, 1)
	}
}

// E5.11 request layer: reserved SRID with a positive witness → refusal
// suppresses the advisory.
func TestT26SRSR3ReservedRefusalRequest(t *testing.T) {
	fx := &r3Fixture{
		version: "8.0.36", discoverVer: "8.0.36",
		srs:  map[uint32]string{2000000000: "x"},
		deps: []r3DepRow{{schema: "s", table: "t", column: "g", srid: 2000000000}},
	}
	p := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL:     "DROP SPATIAL REFERENCE SYSTEM 2000000000;",
		Dialect: spec.DialectMySQL, TargetVersion: "8.0.36", MetadataProvider: p,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	stmt := result.Statements[0]
	r3RefusedDependency(t, result, stmt, 2000000000, 1)
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 0, 1)
}

// --- R3-B.B: Group 10 --------------------------------------------------------

// E10.1: observed banner unproven → qualification gate fires while the
// product loader's own ver("8.0.36") still opens its catalog-read path.
func TestT26SRSR3E10ObservedUnknown(t *testing.T) {
	fx := &r3Fixture{version: "8.0.36", discoverVer: "unknown"}
	p := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: r3SQL, Dialect: spec.DialectMySQL, MetadataProvider: p,
	})
	r3Boundary(t, result, err, "spatial_reference_system")
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 0, 1)
}

// E10.2: healthy observed + loader ver → audited unproven [E,D,P] +
// exactly one reserved notice; unverified; review. Qo 8.0.3 keeps the
// zero-row answer outside the proven-absence domain.
func TestT26SRSR3E10HealthyObserved(t *testing.T) {
	fx := &r3Fixture{version: "8.0.3-log", discoverVer: "8.0.36-log"}
	p := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: r3SQL, Dialect: spec.DialectMySQL, MetadataProvider: p,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	stmt := result.Statements[0]
	r3S2(t, result, stmt,
		[]string{spec.SRSFactExistence, spec.SRSFactColumnDependencies, spec.SRSFactDropPrivilege}, 4326)
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 0, 1)
}

// E10.3: proven 5.7 → version boundary AND the loader's own version
// gate skips catalog reads entirely.
func TestT26SRSR3E10Proven57(t *testing.T) {
	fx := &r3Fixture{version: "5.7.0-m14", discoverVer: "5.7.0-m14"}
	p := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: r3SQL, Dialect: spec.DialectMySQL, MetadataProvider: p,
	})
	r3Boundary(t, result, err, "spatial_reference_system")
	r3AssertReads(t, p, fx, 1, 0, 0, 1, 0, 1)
}

// E10.3b: proven 8.0.3 → audit gate fires (patch < 4) but the loader's
// catalog path (major >= 8) still issues existence+deps.
func TestT26SRSR3E10Proven803(t *testing.T) {
	fx := &r3Fixture{version: "8.0.3-log", discoverVer: "8.0.3-log"}
	p := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: r3SQL, Dialect: spec.DialectMySQL, MetadataProvider: p,
	})
	r3Boundary(t, result, err, "spatial_reference_system")
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 0, 1)
}

// E10.3c/E10.4: providerless pins outside the domain → boundary, all
// seams zero, Vd N/A.
func TestT26SRSR3E10ProviderlessBoundary(t *testing.T) {
	for _, pin := range []string{"8.0.3", "5.7.44"} {
		result, err := audit.AuditSQL(context.Background(), audit.Request{
			SQL: r3SQL, Dialect: spec.DialectMySQL, TargetVersion: pin,
		})
		r3Boundary(t, result, err, "spatial_reference_system")
	}
}

// E10.3d: providerless pin 8.0.4 (floor inclusive) → audited S2.
func TestT26SRSR3E10FloorProviderless(t *testing.T) {
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: r3SQL, Dialect: spec.DialectMySQL, TargetVersion: "8.0.4",
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	stmt := result.Statements[0]
	r3S2(t, result, stmt,
		[]string{spec.SRSFactExistence, spec.SRSFactColumnDependencies, spec.SRSFactDropPrivilege}, 4326)
	if len(result.Unsupported) != 0 {
		t.Fatalf("unsupported=%+v want absent on an audited row", result.Unsupported)
	}
	// declared target-version projection
	if result.Version == nil || result.Version.Product != spec.VersionProductMySQL ||
		result.Version.Version != "8.0.4" || result.Version.Source != "target" {
		t.Fatalf("version=%+v want mysql 8.0.4 source=target", result.Version)
	}
}

// E10.5: dialect tidb + real TiDB discovery banner → vendor boundary;
// zero SRS seams; Vd=1.
func TestT26SRSR3E10TiDBVendorBoundary(t *testing.T) {
	fx := &r3Fixture{discoverVer: "8.0.11-TiDB-v8.5.0"}
	p := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: r3SQL, Dialect: spec.DialectTiDB, MetadataProvider: p,
	})
	if !errors.Is(err, audit.ErrUnsupportedStatement) {
		t.Fatalf("err=%v want ErrUnsupportedStatement", err)
	}
	if len(result.Unsupported) != 1 ||
		result.Unsupported[0].Reason != spec.UnsupportedVendorBoundaryReason ||
		result.Unsupported[0].Metadata["boundary"] != "vendor" {
		t.Fatalf("unsupported=%+v want the vendor boundary", result.Unsupported)
	}
	// statement-level zero findings/gaps + incomplete coverage (R4-B.1)
	// — explicitly one statement row before reading its fields (R5-B.2)
	if len(result.Statements) != 1 {
		t.Fatalf("statements=%d want exactly 1 boundary row", len(result.Statements))
	}
	for _, s := range result.Statements {
		if len(s.Findings) != 0 || len(s.EvidenceGaps) != 0 {
			t.Fatalf("boundary row carries findings/gaps: %+v", s)
		}
		if s.Coverage.Status != report.CoverageIncomplete {
			t.Fatalf("statement coverage=%v want incomplete", s.Coverage.Status)
		}
	}
	// declared TiDB product attribution (R4-B.6)
	if result.Version == nil || result.Version.Product != spec.VersionProductTiDB {
		t.Fatalf("version=%+v want resolved product tidb", result.Version)
	}
	if result.Coverage.Status != report.CoverageIncomplete || result.Verdict != report.VerdictReview {
		t.Fatalf("coverage=%v verdict=%v", result.Coverage.Status, result.Verdict)
	}
	r3AssertReads(t, p, fx, 0, 0, 0, 0, 0, 1)
}

// E10.6: malformed pin → typed input exit; no audit result; Vd N/A.
func TestT26SRSR3E10MalformedPin(t *testing.T) {
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: r3SQL, Dialect: spec.DialectMySQL, TargetVersion: "8.0.x",
	})
	if !errors.Is(err, spec.ErrInvalidTargetVersion) {
		t.Fatalf("err=%v want ErrInvalidTargetVersion", err)
	}
	if len(result.Statements) != 0 || len(result.Unsupported) != 0 {
		t.Fatalf("result=%+v want zero statements", result)
	}
}

// E10.7: dialect mysql + real TiDB discovery banner → product mismatch.
func TestT26SRSR3E10ProductMismatch(t *testing.T) {
	fx := &r3Fixture{discoverVer: "8.0.11-TiDB-v8.5.0"}
	p := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: r3SQL, Dialect: spec.DialectMySQL, MetadataProvider: p,
	})
	if !errors.Is(err, audit.ErrDialectProductMismatch) {
		t.Fatalf("err=%v want ErrDialectProductMismatch", err)
	}
	if len(result.Statements) != 0 {
		t.Fatalf("statements=%d want 0", len(result.Statements))
	}
	r3AssertReads(t, p, fx, 0, 0, 0, 0, 0, 1)
}

// E10.8: pin vs observed mismatch → typed exit, zero SRS seams, Vd=1.
func TestT26SRSR3E10VersionMismatch(t *testing.T) {
	fx := &r3Fixture{discoverVer: "8.0.37-log"}
	p := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: r3SQL, Dialect: spec.DialectMySQL, TargetVersion: "8.0.36", MetadataProvider: p,
	})
	if !errors.Is(err, audit.ErrTargetVersionMismatch) {
		t.Fatalf("err=%v want ErrTargetVersionMismatch", err)
	}
	if len(result.Statements) != 0 {
		t.Fatalf("statements=%d want 0", len(result.Statements))
	}
	r3AssertReads(t, p, fx, 0, 0, 0, 0, 0, 1)
}

// --- R3-E: error paths through the product loader ----------------------------

// r3ErrorRow runs one E11 request-level witness and checks the frozen
// contract: sentinel via errors.Is, exact bounded public text, the full
// attempt vector, and no gap substitute.
func r3ErrorRow(t *testing.T, fx *r3Fixture, resolveSentinel error, name string,
	wantR, wantExist, wantDeps, wantQo int) {
	t.Helper()
	p := r3Harness(t, fx)
	p.resolveErr = resolveSentinel
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: r3SQL, Dialect: spec.DialectMySQL, TargetVersion: "8.0.36", MetadataProvider: p,
	})
	var sentinel error
	for _, s := range fx.failOn {
		sentinel = s
	}
	if resolveSentinel != nil {
		sentinel = resolveSentinel
	}
	if err == nil || !errors.Is(err, sentinel) {
		t.Fatalf("%s: err=%v want errors.Is(sentinel)", name, err)
	}
	if err.Error() != "metadata object read failed" {
		t.Fatalf("%s: public text=%q want the bounded message — sentinel %q must not leak",
			name, err.Error(), sentinel.Error())
	}
	if strings.Contains(err.Error(), sentinel.Error()) {
		t.Fatalf("%s: bounded error leaks sentinel text", name)
	}
	if len(result.Statements) != 0 || len(result.Unsupported) != 0 {
		t.Fatalf("%s: result=%+v want empty — own bind terminated before adjudication", name, result)
	}
	r3AssertReads(t, p, fx, wantR, wantExist, wantDeps, wantQo, 0, 1)
}

// E11.1: resolver forward itself fails → 1(att)/0/0/0/0.
func TestT26SRSR3E11ResolverFailure(t *testing.T) {
	sentinel := errors.New("srs-sentinel-r3-1")
	r3ErrorRow(t, &r3Fixture{version: "8.0.36", discoverVer: "8.0.36"},
		sentinel, "E11.1", 1, 0, 0, 0)
}

// E11.2: product loader's existence query fails → 1/1(att)/0/1/0.
func TestT26SRSR3E11ExistenceFailure(t *testing.T) {
	fx := &r3Fixture{version: "8.0.36", discoverVer: "8.0.36",
		failOn: map[string]error{"exist": errors.New("srs-exist-r3-2")}}
	r3ErrorRow(t, fx, nil, "E11.2", 1, 1, 0, 1)
}

// E11.3: deps query fails after existence answered → 1/1/1(att)/1/0.
func TestT26SRSR3E11DepsFailure(t *testing.T) {
	fx := &r3Fixture{
		version: "8.0.36", discoverVer: "8.0.36",
		srs:    map[uint32]string{4326: "WGS 84"},
		failOn: map[string]error{"deps": errors.New("srs-deps-r3-3")},
	}
	r3ErrorRow(t, fx, nil, "E11.3", 1, 1, 1, 1)
}

// E11.4: the loader's own select version() fails before any catalog
// read → 1/0/0/1(att)/0.
func TestT26SRSR3E11LoaderVersionFailure(t *testing.T) {
	fx := &r3Fixture{discoverVer: "8.0.36",
		failOn: map[string]error{"ver": errors.New("srs-ver-r3-4")}}
	r3ErrorRow(t, fx, nil, "E11.4", 1, 0, 0, 1)
}

// E11.5: discovery sentinel — Vd=1(att), %w text carries the sentinel,
// zero-value report.Result{}, all SRS seams zero.
func TestT26SRSR3E11DiscoverySentinel(t *testing.T) {
	sentinel := errors.New("srs-discover-r3-5")
	fx := &r3Fixture{version: "8.0.36", discoverVer: "8.0.36"}
	p := r3Harness(t, fx)
	p.discoverErr = sentinel
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: r3SQL, Dialect: spec.DialectMySQL, MetadataProvider: p,
	})
	if err == nil || !errors.Is(err, sentinel) {
		t.Fatalf("err=%v want errors.Is(discovery sentinel)", err)
	}
	if err.Error() != "load instance facts: "+sentinel.Error() {
		t.Fatalf("text=%q want exact %q — E11.5 is a %%w wrap, not the bounded text",
			err.Error(), "load instance facts: "+sentinel.Error())
	}
	if !reflect.DeepEqual(result, report.Result{}) {
		t.Fatalf("result=%+v want zero-value report.Result{}", result)
	}
	if p.vd != 1 {
		t.Fatalf("Vd=%d want 1(att)", p.vd)
	}
	r3AssertReads(t, p, fx, 0, 0, 0, 0, 0, 1)
}

// --- R3-D: E12.1/E12.2 request-layer + E12.5 premise-consumable ----------------

// E12.1: raw/normalized SQL carry the authored spellings; no if_exists
// key anywhere in the serialized result.
func TestT26SRSR3SerializedSQLForms(t *testing.T) {
	for _, tc := range []struct{ sql, normalized string }{
		{"DROP SPATIAL REFERENCE SYSTEM 4326;", "DROP SPATIAL REFERENCE SYSTEM 4326"},
		{"DROP SPATIAL REFERENCE SYSTEM IF EXISTS 4326;", "DROP SPATIAL REFERENCE SYSTEM IF EXISTS 4326"},
	} {
		fx := &r3Fixture{
			version: "8.0.36", discoverVer: "8.0.36",
			srs:  map[uint32]string{4326: "WGS 84"},
			deps: []r3DepRow{{schema: "s", table: "t", column: "g", srid: 4326}},
		}
		p := r3Harness(t, fx)
		result, err := audit.AuditSQL(context.Background(), audit.Request{
			SQL: tc.sql, Dialect: spec.DialectMySQL, TargetVersion: "8.0.36", MetadataProvider: p,
		})
		if err != nil {
			t.Fatalf("%q audit: %v", tc.sql, err)
		}
		stmt := result.Statements[0]
		// frozen: raw_sql equals the semicolon-free authored string exactly
		if stmt.RawSQL != strings.TrimSuffix(tc.sql, ";") {
			t.Fatalf("raw_sql=%q want %q", stmt.RawSQL, strings.TrimSuffix(tc.sql, ";"))
		}
		if stmt.NormalizedSQL != tc.normalized {
			t.Fatalf("normalized_sql=%q want %q", stmt.NormalizedSQL, tc.normalized)
		}
		blob, _ := json.Marshal(result)
		if strings.Contains(string(blob), "if_exists") {
			t.Fatal("serialized result carries an if_exists key")
		}
	}
}

// E12.2 request layer: a real refused_dependency fixture through the
// product loader, then a full serialized scan — no catalog identities,
// only the witness count.
func TestT26SRSR3SerializationScanRequest(t *testing.T) {
	fx := &r3Fixture{
		version: "8.0.36", discoverVer: "8.0.36",
		srs: map[uint32]string{4326: "WGS 84"},
		deps: []r3DepRow{
			{schema: "secret_schema", table: "secret_tbl", column: "secret_col", srid: 4326},
		},
	}
	p := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL: r3SQL, Dialect: spec.DialectMySQL, TargetVersion: "8.0.36", MetadataProvider: p,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	f := result.Statements[0].Findings[0]
	if f.RuleID != "ddl.mysql.srs.drop.dependency" || f.Metadata["dependency_witness_count"] != 1 {
		t.Fatalf("finding=%+v want the refusal with witness count 1", f)
	}
	blob, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, secret := range []string{"secret_schema", "secret_tbl", "secret_col"} {
		if strings.Contains(string(blob), secret) {
			t.Fatalf("serialized result leaks %q", secret)
		}
	}
}

// E12.5 request layer: observed_account_new_session loads the SERVER
// privilege observation through the product loader (L=1: current_user
// + show grants both issued) — the consumable premise is proven by the
// SUPER grant reaching the SERVER adjudication — while the sibling SRS
// statement keeps its own [D,P] debt and never triggers a privilege
// load for itself.
func TestT26SRSR3ServerPrivilegeIsolationProduct(t *testing.T) {
	fx := &r3Fixture{
		version: "8.0.36", discoverVer: "8.0.36",
		srs:     map[uint32]string{4326: "WGS 84"},
		servers: map[string]bool{"s1": true},
		account: "app@%",
		grants:  []string{"GRANT SELECT, SUPER ON *.* TO `app`@`%`"},
	}
	p := r3Harness(t, fx)
	result, err := audit.AuditSQL(context.Background(), audit.Request{
		SQL:                "DROP SERVER s1; DROP SPATIAL REFERENCE SYSTEM 4326;",
		Dialect:            spec.DialectMySQL,
		TargetVersion:      "8.0.36",
		MetadataProvider:   p,
		PrivilegeAuditMode: spec.PrivilegeAuditModeObservedAccountNewSession,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if p.privCalls != 1 {
		t.Fatalf("L=%d want exactly 1 — one privilege load for the batch", p.privCalls)
	}
	if fx.count["user"] != 1 || fx.count["grants"] != 1 {
		t.Fatalf("privilege loader queries user/grants=%d/%d want 1/1 — product loader really ran",
			fx.count["user"], fx.count["grants"])
	}
	srv := result.Statements[0]
	// positive consumability proof (R5-A): the SUPER witness was consumed
	// by the SERVER adjudication — ONE ddl.mysql.server.drop.notice finding
	// carries the full conditional projection on the same finding:
	// outcome=conditional_drop and conditional_audit=
	// observed_account_new_session as a string. A missing key, nil,
	// non-string, empty string, or wrong token all fail this proof —
	// absence of a server.drop_privilege debt alone does not count.
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
	for _, gap := range srv.EvidenceGaps {
		for _, fact := range gap.RequiredFacts {
			if fact == "server.drop_privilege" {
				t.Fatal("SERVER still owes drop_privilege — the SUPER witness was not consumed")
			}
		}
	}
	if !serverConsumed {
		t.Fatalf("no ddl.mysql.server.drop.notice finding carries outcome=conditional_drop + conditional_audit=observed_account_new_session — privilege premise not proven consumable: %+v", srv.Findings)
	}
	srs := result.Statements[1]
	// T26-SRS-DROP-PRIVILEGE-EXECUTION-01 registered change (P8): the
	// SRS statement now projects its own consumable premise — owed
	// drops from historical [D,P] to [D], one dependency-owned
	// unknown_srs_state gap remains, and the statement carries the
	// family's own privilege notice plus the reserved advisory.
	r3S2Consumed(t, srs, []string{spec.SRSFactColumnDependencies}, 4326, true)
	// Marker isolation is checked per statement/finding (P8-G): the
	// SRS notice's conditional_audit is its own designed public field —
	// the raw SERVER observation payload is what must never cross.
	srsBlob, _ := json.Marshal(srs)
	for _, forbidden := range []string{"app@%", "current_user", "GRANT SELECT", "GRANT SUPER", "grant_lines"} {
		if strings.Contains(string(srsBlob), forbidden) {
			t.Fatalf("SRS statement leaks SERVER observation payload %q", forbidden)
		}
	}
	// R4-A.5: independent read vector for THIS request — R/Qexist/Qdeps/
	// Qo/L/Vd all exactly 1 (SERVER never borrows SRS seams).
	r3AssertReads(t, p, fx, 1, 1, 1, 1, 1, 1)
	// conditional_audit is a designed public projection on the SERVER
	// finding; the raw observation payload is not — the serialized
	// result must never carry the observed account or grant text.
	blob, _ := json.Marshal(result)
	for _, forbidden := range []string{"app@%", "current_user", "GRANT SELECT", "grant_lines"} {
		if strings.Contains(string(blob), forbidden) {
			t.Fatalf("serialized result leaks %q", forbidden)
		}
	}
}
