// T26-SERVER-ALTER-EXECUTION-01 R2-F: vertical witnesses through the
// real mysqlmeta.Provider + stub driver — the independent L/R/S
// layered counts the application-layer stubs cannot see (L = loader
// seam invocations on the provider wrapper, R = provider-facing
// ResolveObject forwards, S = actual mysql.servers SQL on the
// driver), plus the Group 9 request/version/dialect boundary rows
// and the two pre-SQL error sentinels.
package deltascope

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	mysqlmeta "github.com/Fanduzi/DeltaScope/internal/infrastructure/metadata/mysql"
)

func newT26AlterProvider(t *testing.T, fixture *t26vFixture) *t26vProvider {
	t.Helper()
	db := openT26VDB(t, fixture)
	return &t26vProvider{inner: mysqlmeta.NewProvider(db)}
}

// AM-9.3c: MySQL dialect, no pin, observed 8.0.36, positive
// consumable witness — the audited A3 alters. Layered counts:
// L=1 (one loader seam call at enrich entry), R=1 (one provider-
// facing resolver forward), S=1 (one mysql.servers query on the
// driver), plus the loader's own grants SQL recorded separately.
func TestT26AlterAM93cPositiveWitness(t *testing.T) {
	fixture := &t26vFixture{
		servers:    map[string]bool{"s1": true},
		account:    "audited@%",
		grantLines: []string{"GRANT SELECT, SUPER ON *.* TO 'audited'@'%'"},
	}
	provider := newT26AlterProvider(t, fixture)
	result, err := Audit(context.Background(), Request{
		SQL:     "ALTER SERVER s1 OPTIONS (USER 'u')",
		Dialect: DialectMySQL, Schema: "golden",
		MetadataProvider: provider, PrivilegeAuditMode: PrivilegeAuditModeObservedAccountNewSession,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	stmt := result.Statements[0]
	if len(stmt.Findings) != 2 {
		t.Fatalf("findings = %+v, want main + advisory", stmt.Findings)
	}
	if got := stmt.Findings[0].Metadata["outcome"]; got != "conditional_alter" {
		t.Fatalf("outcome = %v, want conditional_alter", got)
	}
	if stmt.Findings[0].Metadata["conditional_audit"] == "" {
		t.Fatalf("A3 must carry the conditional marker: %+v", stmt.Findings[0].Metadata)
	}
	if provider.loaderCalls != 1 || provider.resolverCalls != 1 {
		t.Fatalf("L=%d R=%d, want 1/1", provider.loaderCalls, provider.resolverCalls)
	}
	if serversQueries(fixture) != 1 || grantsQueries(fixture) != 1 {
		t.Fatalf("S servers=%d grants=%d, want 1/1", serversQueries(fixture), grantsQueries(fixture))
	}
}

// AM-9.1b: request/API, no pin, observed 5.7.44 — the public version
// boundary. The bind still runs (clean channel, cache miss,
// identifier, provider executes): L=0/R=1/S=1.
func TestT26AlterAM91bOutOfDomain(t *testing.T) {
	fixture := &t26vFixture{
		servers:    map[string]bool{"s1": true},
		account:    "audited@%",
		grantLines: []string{"GRANT SUPER ON *.* TO 'audited'@'%'"},
		version:    "5.7.44",
	}
	provider := newT26AlterProvider(t, fixture)
	result, err := Audit(context.Background(), Request{
		SQL:     "ALTER SERVER s1 OPTIONS (USER 'u')",
		Dialect: DialectMySQL, Schema: "golden",
		MetadataProvider: provider, PrivilegeAuditMode: PrivilegeAuditModeObservedAccountNewSession,
	})
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("err = %v, want ErrUnsupportedStatement", err)
	}
	found := false
	for _, u := range result.Unsupported {
		if u.Metadata != nil && u.Metadata["gate"] == "version" {
			found = true
		}
	}
	if !found {
		t.Fatalf("unsupported = %+v, want the version gate", result.Unsupported)
	}
	if provider.loaderCalls != 0 {
		t.Fatalf("L = %d, want 0 — version gate precedes the loader", provider.loaderCalls)
	}
	if provider.resolverCalls != 1 || serversQueries(fixture) != 1 {
		t.Fatalf("R=%d S=%d, want 1/1 — the bind still issued the catalog read", provider.resolverCalls, serversQueries(fixture))
	}
}

// AM-9.3a: pin = 5.7.44 and observed = 5.7.44 — matching, but
// out-of-domain. Same version boundary; L=0/R=1/S=1.
func TestT26AlterAM93aPinnedOutOfDomain(t *testing.T) {
	fixture := &t26vFixture{
		servers:    map[string]bool{"s1": true},
		account:    "audited@%",
		grantLines: []string{"GRANT SUPER ON *.* TO 'audited'@'%'"},
		version:    "5.7.44",
	}
	provider := newT26AlterProvider(t, fixture)
	_, err := Audit(context.Background(), Request{
		SQL:     "ALTER SERVER s1 OPTIONS (USER 'u')",
		Dialect: DialectMySQL, Schema: "golden", TargetVersion: "5.7.44",
		MetadataProvider: provider, PrivilegeAuditMode: PrivilegeAuditModeObservedAccountNewSession,
	})
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("err = %v, want ErrUnsupportedStatement (matching but out-of-domain)", err)
	}
	if provider.loaderCalls != 0 || provider.resolverCalls != 1 || serversQueries(fixture) != 1 {
		t.Fatalf("L=%d R=%d S=%d, want 0/1/1", provider.loaderCalls, provider.resolverCalls, serversQueries(fixture))
	}
}

// AM-9.3b: TiDB dialect, providerless, no pin, mode off — the
// request/API vendor boundary. No provider exists to count on:
// L/R/S = 0 by construction.
func TestT26AlterAM93bTiDBVendorBoundary(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "ALTER SERVER s1 OPTIONS (USER 'u')",
		Dialect: DialectTiDB, Schema: "golden",
	})
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("err = %v, want ErrUnsupportedStatement", err)
	}
	if len(result.Unsupported) == 0 {
		t.Fatalf("unsupported = %+v, want the vendor boundary detail", result.Unsupported)
	}
}

// AM-9.3d: MySQL dialect with a TiDB-observed product —
// ErrDialectProductMismatch before any arm; audit-layer L/R/S = 0
// (the instance-facts version read is not a server-channel query).
// R3-C note: the frozen row's literal `postgres` observed product is
// unexpressible in the current observed-product vocabulary —
// spec.ParseObservedVersion derives product from the banner itself
// and yields only {mysql, tidb}; a postgres-versioned banner parses
// to nil or mysql, never postgres. The TiDB-observed row is the
// row's real expressible evidence; the postgres-banner behavior is
// registered honestly in TestT26AlterAM93dPostgresBannerBoundary
// below, and the contract gap is recorded in the execution errata
// for reviewer disposition — never re-labeled to match the frozen
// literal.
func TestT26AlterAM93dProductMismatch(t *testing.T) {
	fixture := &t26vFixture{servers: map[string]bool{}, account: "audited@%", version: "8.0.11-TiDB-v8.5.0"}
	provider := newT26AlterProvider(t, fixture)
	_, err := Audit(context.Background(), Request{
		SQL:     "ALTER SERVER s1 OPTIONS (USER 'u')",
		Dialect: DialectMySQL, Schema: "golden",
		MetadataProvider: provider, PrivilegeAuditMode: PrivilegeAuditModeObservedAccountNewSession,
	})
	if !errors.Is(err, ErrDialectProductMismatch) {
		t.Fatalf("err = %v, want ErrDialectProductMismatch", err)
	}
	if provider.loaderCalls != 0 || provider.resolverCalls != 0 || serversQueries(fixture) != 0 {
		t.Fatalf("L=%d R=%d S=%d, want 0/0/0 — typed error precedes the arm", provider.loaderCalls, provider.resolverCalls, serversQueries(fixture))
	}
}

// AM-9.3d supplement (R3-C): a postgres-spelling version banner is
// documented REAL behavior, not the frozen row — ParseObservedVersion
// yields no postgres product, so no dialect-product mismatch exists
// to fire: the banner is an unestablished version fact, the ALTER
// exits at the version gate (unaudited boundary), and the loader
// never runs. Registered so the contract's unreachable literal is
// distinguished from this reachable boundary evidence.
func TestT26AlterAM93dPostgresBannerBoundary(t *testing.T) {
	fixture := &t26vFixture{servers: map[string]bool{}, account: "audited@%", version: "PostgreSQL 16.1"}
	provider := newT26AlterProvider(t, fixture)
	result, err := Audit(context.Background(), Request{
		SQL:     "ALTER SERVER s1 OPTIONS (USER 'u')",
		Dialect: DialectMySQL, Schema: "golden",
		MetadataProvider: provider, PrivilegeAuditMode: PrivilegeAuditModeObservedAccountNewSession,
	})
	if errors.Is(err, ErrDialectProductMismatch) {
		t.Fatalf("postgres banner must not mismatch — no postgres product exists: %v", err)
	}
	gated := false
	for _, u := range result.Unsupported {
		if u.Metadata != nil && u.Metadata["gate"] == "version" {
			gated = true
		}
	}
	if !gated {
		t.Fatalf("unsupported = %+v, want the version gate — the postgres banner is an unestablished fact", result.Unsupported)
	}
	if provider.loaderCalls != 0 {
		t.Fatalf("L = %d, want 0 — version gate precedes the loader", provider.loaderCalls)
	}
}

// AM-9.3e: pin 8.0.36 with observed 8.4.0 — ErrTargetVersionMismatch
// before any arm; audit-layer L/R/S = 0.
func TestT26AlterAM93eVersionMismatch(t *testing.T) {
	fixture := &t26vFixture{servers: map[string]bool{}, account: "audited@%", version: "8.4.0"}
	provider := newT26AlterProvider(t, fixture)
	_, err := Audit(context.Background(), Request{
		SQL:     "ALTER SERVER s1 OPTIONS (USER 'u')",
		Dialect: DialectMySQL, Schema: "golden", TargetVersion: "8.0.36",
		MetadataProvider: provider, PrivilegeAuditMode: PrivilegeAuditModeObservedAccountNewSession,
	})
	if !errors.Is(err, ErrTargetVersionMismatch) {
		t.Fatalf("err = %v, want ErrTargetVersionMismatch", err)
	}
	if provider.loaderCalls != 0 || provider.resolverCalls != 0 || serversQueries(fixture) != 0 {
		t.Fatalf("L=%d R=%d S=%d, want 0/0/0", provider.loaderCalls, provider.resolverCalls, serversQueries(fixture))
	}
}

// AM-9.4: in-domain observed version — the sentinel is thrown at the
// ResolveObject forward seam BEFORE any SQL: L=1/R=1/S=0. The error
// chain preserves the sentinel via errors.Is, and no arm/gap
// substitution happens.
func TestT26AlterAM94ResolverSentinel(t *testing.T) {
	// The injected payload IS the checked text — a distinguishable
	// sensitive sentinel, never an unrelated string (R3-E.1).
	sentinel := errors.New("resolver secret payload pw-sentinel-455")
	fixture := &t26vFixture{
		servers:    map[string]bool{"s1": true},
		account:    "audited@%",
		grantLines: []string{"GRANT SUPER ON *.* TO 'audited'@'%'"},
	}
	db := openT26VDB(t, fixture)
	provider := &t26vProvider{inner: mysqlmeta.NewProvider(db), resolverErr: sentinel}
	_, err := Audit(context.Background(), Request{
		SQL:     "ALTER SERVER s1 OPTIONS (USER 'u')",
		Dialect: DialectMySQL, Schema: "golden",
		MetadataProvider: provider, PrivilegeAuditMode: PrivilegeAuditModeObservedAccountNewSession,
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want errors.Is(sentinel)", err)
	}
	if strings.Contains(err.Error(), "pw-sentinel-455") {
		t.Fatalf("bounded public error echoed the injected payload: %v", err)
	}
	if !strings.Contains(err.Error(), "metadata object read failed") {
		t.Fatalf("bounded category message missing: %v", err)
	}
	if provider.loaderCalls != 1 || provider.resolverCalls != 1 || serversQueries(fixture) != 0 {
		t.Fatalf("L=%d R=%d S=%d, want 1/1/0 — sentinel at the forward seam before SQL", provider.loaderCalls, provider.resolverCalls, serversQueries(fixture))
	}
}

// AM-9.5: the loader throws its sentinel at enrich entry —
// serverPrivilegeReadError class, errors.Is preserves the cause, and
// no statement bind/adjudication follows: L=1/R=0/S=0.
func TestT26AlterAM95LoaderSentinel(t *testing.T) {
	sentinel := errors.New("loader-sentinel")
	fixture := &t26vFixture{
		servers:    map[string]bool{"s1": true},
		account:    "audited@%",
		grantLines: []string{"GRANT SUPER ON *.* TO 'audited'@'%'"},
	}
	db := openT26VDB(t, fixture)
	provider := &t26vProvider{inner: mysqlmeta.NewProvider(db), loaderErr: sentinel}
	_, err := Audit(context.Background(), Request{
		SQL:     "ALTER SERVER s1 OPTIONS (USER 'u')",
		Dialect: DialectMySQL, Schema: "golden",
		MetadataProvider: provider, PrivilegeAuditMode: PrivilegeAuditModeObservedAccountNewSession,
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want errors.Is(sentinel)", err)
	}
	if provider.loaderCalls != 1 || provider.resolverCalls != 0 || serversQueries(fixture) != 0 {
		t.Fatalf("L=%d R=%d S=%d, want 1/0/0 — loader failed before any bind", provider.loaderCalls, provider.resolverCalls, serversQueries(fixture))
	}
}

// S-layer on the audited chains: ALTER → ALTER on catalog-present
// s1 issues exactly one mysql.servers read across the batch — the
// ordered channel answers the second statement (R=1/S=1 while the
// resolver forward count stays one).
func TestT26AlterVerticalOrderedChainCounts(t *testing.T) {
	fixture := &t26vFixture{
		servers:    map[string]bool{"s1": true},
		account:    "audited@%",
		grantLines: []string{"GRANT SUPER ON *.* TO 'audited'@'%'"},
	}
	provider := newT26AlterProvider(t, fixture)
	_, err := Audit(context.Background(), Request{
		SQL:     "ALTER SERVER s1 OPTIONS (USER 'u'); ALTER SERVER s1 OPTIONS (HOST 'h')",
		Dialect: DialectMySQL, Schema: "golden",
		MetadataProvider: provider, PrivilegeAuditMode: PrivilegeAuditModeObservedAccountNewSession,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if provider.resolverCalls != 1 || serversQueries(fixture) != 1 || grantsQueries(fixture) != 1 {
		t.Fatalf("R=%d S=%d loaderSQL=%d, want 1/1/1", provider.resolverCalls, serversQueries(fixture), grantsQueries(fixture))
	}
}

// Boundary unsupported form carries the fixed redacted skeleton and
// never reaches the provider: the literal name still forwards the
// resolver call but issues no mysql.servers SQL — R=1/S=0.
func TestT26AlterVerticalLiteralBoundary(t *testing.T) {
	fixture := &t26vFixture{
		servers:    map[string]bool{},
		account:    "audited@%",
		grantLines: []string{"GRANT SUPER ON *.* TO 'audited'@'%'"},
	}
	provider := newT26AlterProvider(t, fixture)
	result, err := Audit(context.Background(), Request{
		SQL:     "ALTER SERVER 'lit9' OPTIONS (USER 'u')",
		Dialect: DialectMySQL, Schema: "golden",
		MetadataProvider: provider,
	})
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("err = %v, want ErrUnsupportedStatement", err)
	}
	if result.Statements[0].RawSQL != "ALTER SERVER <redacted>" {
		t.Fatalf("boundary skeleton = %q", result.Statements[0].RawSQL)
	}
	if provider.resolverCalls != 1 || serversQueries(fixture) != 0 {
		t.Fatalf("R=%d S=%d, want 1/0 — literal forward issued no catalog SQL", provider.resolverCalls, serversQueries(fixture))
	}
}

var _ = spec.DialectMySQL // keep spec import used if helpers drift

// R3-D: the independent L/R/S counts measured on the SAME fixtures at
// their own seams — L on the provider wrapper's loader seam, R on the
// ResolveObject forward seam, S on real mysql.servers driver SQL.
// grants SQL stays a separate count, never mixed into the
// server-channel S. One shared fixture/driver shape per row; the
// AM-8.1 row is the single-request form of the two-call fixture, so
// its tail ALTER contaminates the same batch and the S total proves
// exactly one catalog read ran.
func TestT26AlterR3DMeasuredLayerCounts(t *testing.T) {
	observed := PrivilegeAuditModeObservedAccountNewSession
	grants := []string{"GRANT SUPER ON *.* TO 'audited'@'%'"}
	for _, tc := range []struct {
		am      string
		sql     string
		mode    PrivilegeAuditMode
		servers map[string]bool
		l, r, s int
	}{
		// ALTER→DROP alias: zero catalog, each verbatim spelling reads.
		{"AM-7.2", "ALTER SERVER s9 OPTIONS (USER 'u'); DROP SERVER S9", observed, map[string]bool{}, 1, 2, 2},
		// sé present + se blocked by the relation scan — the DROP never
		// reaches a read: one catalog read total.
		{"AM-7.3a", "ALTER SERVER `sé` OPTIONS (USER 'u'); DROP SERVER se", observed, map[string]bool{"sé": true}, 1, 1, 1},
		// Both zero: the DROP's own fresh verbatim read — two catalog reads.
		{"AM-7.3b", "ALTER SERVER `sé` OPTIONS (USER 'u'); DROP SERVER se", observed, map[string]bool{}, 1, 2, 2},
		// 65-rune overlong identifier boundary forwards AND executes —
		// measured against the real provider: R=1/S=1.
		{"AM-7.4", "ALTER SERVER " + strings.Repeat("x", 65) + " OPTIONS (USER 'u'); DROP SERVER s9", observed, map[string]bool{}, 1, 1, 1},
		// Mixed identifier/literal chain: s1's catalog read + the
		// literal's declined forward; the tainted DROP reads nothing.
		{"AM-7.5", "ALTER SERVER s1 OPTIONS (USER 'u'); ALTER SERVER 'lit' OPTIONS (USER 'u'); DROP SERVER S1", observed, map[string]bool{"s1": true}, 1, 2, 1},
		// sé→se→DROP sé final-consumer chain: L=1/R=1/S=1.
		{"AM-7.6", "ALTER SERVER `sé` OPTIONS (USER 'u'); ALTER SERVER se OPTIONS (USER 'v'); DROP SERVER `sé`", observed, map[string]bool{"sé": true}, 1, 1, 1},
		// Single-request form of the two-call contamination fixture:
		// the prefix ALTER reads once; the tail binds contaminated —
		// total S=1 proves the second ALTER issued no SQL.
		{"AM-8.1", "ALTER SERVER s1 OPTIONS (USER 'u'); GARBAGE(; ALTER SERVER s1 OPTIONS (HOST 'h')", observed, map[string]bool{"s1": true}, 1, 1, 1},
		// Literal write → tainted DROP, mode off: R=1/S=0.
		{"AM-10.4", "ALTER SERVER 'lit9' OPTIONS (USER 'u'); DROP SERVER s9", "", map[string]bool{}, 0, 1, 0},
	} {
		fixture := &t26vFixture{servers: tc.servers, account: "audited@%", grantLines: grants}
		provider := newT26AlterProvider(t, fixture)
		_, err := Audit(context.Background(), Request{
			SQL: tc.sql, Dialect: DialectMySQL, Schema: "golden",
			MetadataProvider: provider, PrivilegeAuditMode: tc.mode,
		})
		if provider.loaderCalls != tc.l || provider.resolverCalls != tc.r || serversQueries(fixture) != tc.s {
			t.Fatalf("%s: L=%d R=%d S=%d, want %d/%d/%d (err=%v)", tc.am, provider.loaderCalls, provider.resolverCalls, serversQueries(fixture), tc.l, tc.r, tc.s, err)
		}
		t.Logf("%s measured L=%d R=%d S=%d grants=%d", tc.am, provider.loaderCalls, provider.resolverCalls, serversQueries(fixture), grantsQueries(fixture))
	}
}
