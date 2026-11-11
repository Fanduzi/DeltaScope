// T26-SERVER-DROP-EXECUTION-01 R1-E: the vertical chain — a stub
// database/sql driver feeds the real MySQL metadata Provider, whose
// built-in LoadMySQLServerDropPrivilege supplies the conditional
// premise to the real AuditSQL consumer. Every claim is observed on
// the actual result: gaps, coverage, verdict, finding shape, the
// OrderedAbsent consumer, and the layered query counts.
package deltascope

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	mysqlmeta "github.com/Fanduzi/DeltaScope/internal/infrastructure/metadata/mysql"
)

// t26vProvider adapts the real internal mysqlmeta.Provider to the
// public MetadataProvider surface — the same adapter shape the SDK
// itself applies at the boundary; the loader, the resolver, and the
// driver underneath are the production implementations.
type t26vProvider struct{ inner *mysqlmeta.Provider }

func (p *t26vProvider) LoadInstanceFacts(ctx context.Context, dialect Dialect, schema string) (*InstanceFacts, error) {
	return p.inner.LoadInstanceFacts(ctx, spec.Dialect(dialect), schema)
}

func (p *t26vProvider) LoadTableSnapshot(ctx context.Context, dialect Dialect, schema, table string) (*TableSnapshot, error) {
	return p.inner.LoadTableSnapshot(ctx, spec.Dialect(dialect), schema, table)
}

// ResolveObject carries the internal spec.Dialect signature — the
// public wrapper discovers the object resolver seam by asserting the
// internal signature, not the public Dialect surface.
func (p *t26vProvider) ResolveObject(ctx context.Context, dialect spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	return p.inner.ResolveObject(ctx, dialect, req)
}

func (p *t26vProvider) LoadMySQLServerDropPrivilege(ctx context.Context, dialect Dialect) (*MySQLServerDropPrivilegeObservation, error) {
	return p.inner.LoadMySQLServerDropPrivilege(ctx, spec.Dialect(dialect))
}

// t26vFixture is the per-test query surface — exact fragment matching,
// query recording for the layered-count assertions.
type t26vFixture struct {
	servers    map[string]bool
	account    string
	grantLines []string
	queries    []string
}

var (
	t26vOnce   sync.Once
	t26vFixies sync.Map
)

func openT26VDB(t *testing.T, fixture *t26vFixture) *sql.DB {
	t.Helper()
	t26vOnce.Do(func() { sql.Register("t26v-mysqlmeta", t26vDriver{}) })
	name := "t26v-" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	t26vFixies.Store(name, fixture)
	db, err := sql.Open("t26v-mysqlmeta", name)
	if err != nil {
		t.Fatalf("open driver db: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		t26vFixies.Delete(name)
	})
	return db
}

type t26vDriver struct{}
type t26vConn struct{ fixture *t26vFixture }
type t26vRows struct {
	cols  []string
	rows  [][]driver.Value
	index int
}

func (t26vDriver) Open(name string) (driver.Conn, error) {
	fixture, _ := t26vFixies.Load(name)
	conn, _ := fixture.(*t26vFixture)
	return &t26vConn{fixture: conn}, nil
}

func (c *t26vConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (c *t26vConn) Close() error                        { return nil }
func (c *t26vConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }

func (c *t26vConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.fixture.queries = append(c.fixture.queries, query)
	switch {
	case strings.Contains(query, "show variables"):
		return &t26vRows{cols: []string{"Variable_name", "Value"}, rows: [][]driver.Value{{"version", "8.0.36"}}}, nil
	case strings.Contains(query, "select version()"):
		return &t26vRows{cols: []string{"version()"}, rows: [][]driver.Value{{"8.0.36"}}}, nil
	case strings.Contains(query, "select current_user()"):
		if c.fixture.account == "" {
			return &t26vRows{cols: []string{"current_user()"}}, nil
		}
		return &t26vRows{cols: []string{"current_user()"}, rows: [][]driver.Value{{c.fixture.account}}}, nil
	case strings.Contains(query, "show grants for current_user()"):
		rows := make([][]driver.Value, 0, len(c.fixture.grantLines))
		for _, line := range c.fixture.grantLines {
			rows = append(rows, []driver.Value{line})
		}
		return &t26vRows{cols: []string{"Grants"}, rows: rows}, nil
	case strings.Contains(query, "mysql.servers"):
		if len(args) > 0 {
			if name, ok := args[0].Value.(string); ok && c.fixture.servers[name] {
				return &t26vRows{cols: []string{"server_name"}, rows: [][]driver.Value{{name}}}, nil
			}
		}
		return &t26vRows{cols: []string{"server_name"}}, nil
	}
	return &t26vRows{}, nil
}

func (r *t26vRows) Columns() []string { return r.cols }
func (r *t26vRows) Close() error      { return nil }
func (r *t26vRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

// serversQueries counts mysql.servers resolver reads; grantQueries
// counts the one-shot loader's grants read.
func serversQueries(fixture *t26vFixture) int {
	count := 0
	for _, q := range fixture.queries {
		if strings.Contains(q, "mysql.servers") {
			count++
		}
	}
	return count
}

func grantsQueries(fixture *t26vFixture) int {
	count := 0
	for _, q := range fixture.queries {
		if strings.Contains(q, "show grants for current_user()") {
			count++
		}
	}
	return count
}

// Vertical chain A: consumable witness end-to-end. Statement 1 is the
// conditional proven drop; statement 2 is the refusal the published
// OrderedAbsent answers without any new resolver read; statement 3 is
// the guarded_absent arm consuming that same published absent (not a
// convergent publish). Layered counts: request
// total = 1 servers query, first statement issued it, the two
// consumers issued zero new ones; the loader ran exactly once.
func TestT26VerticalConsumableChain(t *testing.T) {
	fixture := &t26vFixture{
		servers:    map[string]bool{"s1": true},
		account:    "audited@%",
		grantLines: []string{"GRANT SELECT, SUPER ON *.* TO 'audited'@'%'"},
	}
	db := openT26VDB(t, fixture)
	provider := &t26vProvider{inner: mysqlmeta.NewProvider(db)}
	result, err := Audit(context.Background(), Request{
		SQL:     "DROP SERVER s1; DROP SERVER s1; DROP SERVER IF EXISTS s1",
		Dialect: DialectMySQL, Schema: "golden",
		MetadataProvider: provider, PrivilegeAuditMode: PrivilegeAuditModeObservedAccountNewSession,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 3 {
		t.Fatalf("statements = %d", len(result.Statements))
	}
	if result.Verdict != VerdictReject {
		t.Fatalf("a proven refusal in the batch must reject: %+v", result.Verdict)
	}
	for i, want := range []string{"conditional_drop", "refused_absent", "guarded_absent"} {
		stmt := result.Statements[i]
		if len(stmt.EvidenceGaps) != 0 {
			t.Fatalf("statement %d gaps = %+v, want none", i, stmt.EvidenceGaps)
		}
		if len(stmt.Findings) != 1 {
			t.Fatalf("statement %d findings = %+v", i, stmt.Findings)
		}
		finding := stmt.Findings[0]
		if finding.Metadata["outcome"] != want {
			t.Fatalf("statement %d outcome = %v, want %s", i, finding.Metadata["outcome"], want)
		}
		if finding.Metadata["conditional_audit"] != string(PrivilegeAuditModeObservedAccountNewSession) {
			t.Fatalf("statement %d must keep the conditional marker (own consumable or conditional absent): %+v", i, finding.Metadata)
		}
	}
	if result.Statements[1].Findings[0].Level != LevelBlocker {
		t.Fatalf("refusal level = %+v, want blocker", result.Statements[1].Findings[0].Level)
	}
	// Two-level coverage: every audited statement gap-free means each
	// statement and the aggregate stay complete; the blocker alone
	// drives the top-level reject.
	for i, stmt := range result.Statements {
		if stmt.Coverage.Status != CoverageComplete {
			t.Fatalf("statement %d coverage = %v, want complete", i, stmt.Coverage.Status)
		}
	}
	if result.Coverage.Status != CoverageComplete {
		t.Fatalf("aggregate coverage = %v, want complete", result.Coverage.Status)
	}
	// Layered counts: one loader pass for the whole request; one
	// resolver read by statement 1; statements 2-3 added none.
	if grantsQueries(fixture) != 1 || serversQueries(fixture) != 1 {
		t.Fatalf("queries = grants:%d servers:%d, want 1 each — the OrderedAbsent consumers issued no new reads",
			grantsQueries(fixture), serversQueries(fixture))
	}
	// Protected detail never surfaces anywhere in the public result.
	blob := ""
	for _, stmt := range result.Statements {
		for _, f := range stmt.Findings {
			blob += f.Message + f.Explanation.Why + f.Explanation.Risk + f.Explanation.Suggestion
		}
	}
	for _, protected := range []string{"audited@%", "GRANT SELECT", "SHOW GRANTS", "s1"} {
		if strings.Contains(blob, protected) {
			t.Fatalf("protected detail %q leaked: %q", protected, blob)
		}
	}
}

// Vertical chain B — relation boundary: the published OrderedAbsent on
// s1 answers nothing for the relation-unproven identity s2 directly,
// but the ordered scan still precedes the resolver — s2's access is
// shadowed by the unproven relation to a recorded write, so the
// catalog is never queried for it (the conservative batch_effects
// barrier, no barrier bit required on the entry). The third drop on
// s1 is shadowed the same way — s2's unadjudicated write taints the
// whole channel — and reads blocked_batch_effects with zero new
// resolver reads.
func TestT26VerticalRelationBoundary(t *testing.T) {
	fixture := &t26vFixture{
		servers:    map[string]bool{"s1": true, "s2": true},
		account:    "audited@%",
		grantLines: []string{"GRANT SUPER ON *.* TO 'audited'@'%'"},
	}
	db := openT26VDB(t, fixture)
	provider := &t26vProvider{inner: mysqlmeta.NewProvider(db)}
	result, err := Audit(context.Background(), Request{
		SQL:     "DROP SERVER s1; DROP SERVER s2; DROP SERVER s1",
		Dialect: DialectMySQL, Schema: "golden",
		MetadataProvider: provider, PrivilegeAuditMode: PrivilegeAuditModeObservedAccountNewSession,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if finding := result.Statements[0].Findings[0]; finding.Metadata["outcome"] != "conditional_drop" {
		t.Fatalf("s1 drop outcome = %v, want conditional_drop", finding.Metadata["outcome"])
	}
	// s2: the unproven relation to s1's recorded drop shadows the
	// catalog answer — blocked_batch_effects, owes the barrier fact
	// only (consumable premise discharged), and NO resolver read.
	barrier := result.Statements[1]
	if finding := barrier.Findings[0]; finding.Metadata["outcome"] != "blocked_batch_effects" {
		t.Fatalf("s2 outcome = %v, want blocked_batch_effects", finding.Metadata["outcome"])
	}
	if len(barrier.EvidenceGaps) != 1 {
		t.Fatalf("s2 gaps = %+v, want the batch_effects fact only", barrier.EvidenceGaps)
	}
	if facts := barrier.EvidenceGaps[0].RequiredFacts; len(facts) != 1 || facts[0] != "server.batch_effects" {
		t.Fatalf("s2 owed = %v, want server.batch_effects only", facts)
	}
	// s1 re-drop: s2's own arm stayed unproven (barrier) — an
	// undischarged unknown effect on an unproven-related identity
	// shadows s1's proven absent too, so the re-drop reads the same
	// barrier rather than the stale absent. Still zero new resolver
	// reads: every ordered-channel answer precedes the resolver.
	shadowed := result.Statements[2]
	if finding := shadowed.Findings[0]; finding.Metadata["outcome"] != "blocked_batch_effects" {
		t.Fatalf("s1 re-drop outcome = %v, want blocked_batch_effects under the shadowed absent", finding.Metadata["outcome"])
	}
	if facts := shadowed.EvidenceGaps[0].RequiredFacts; len(facts) != 1 || facts[0] != "server.batch_effects" {
		t.Fatalf("s1 re-drop owed = %v, want server.batch_effects only", facts)
	}
	// Two-level coverage: the gap-free first statement stays complete;
	// both shadowed statements carry the batch_effects gap and read
	// unverified; the aggregate follows them; no blocker means the
	// verdict floor is review.
	if result.Statements[0].Coverage.Status != CoverageComplete {
		t.Fatalf("s1 drop coverage = %v, want complete", result.Statements[0].Coverage.Status)
	}
	for i := 1; i <= 2; i++ {
		if result.Statements[i].Coverage.Status != CoverageUnverified {
			t.Fatalf("statement %d coverage = %v, want unverified under the shadowed channel", i, result.Statements[i].Coverage.Status)
		}
	}
	if result.Coverage.Status != CoverageUnverified {
		t.Fatalf("aggregate coverage = %v, want unverified", result.Coverage.Status)
	}
	if result.Verdict != VerdictReview {
		t.Fatalf("verdict = %+v, want review under the evidence-gap floor", result.Verdict)
	}
	if serversQueries(fixture) != 1 {
		t.Fatalf("resolver reads = %d, want exactly 1 — the relation scan answered s2 before the resolver and the shadowed channel answered the re-drop", serversQueries(fixture))
	}
}

// Vertical chain C — near-token negatives: SUPERSET and ALL PRIVILEGED
// are near-shaped, never the registered tokens; the premise stays owed
// and the verdict degrades to review, never a denial or a discharge.
func TestT26VerticalNearTokenNegatives(t *testing.T) {
	for _, grant := range []string{
		"GRANT SUPERSET ON *.* TO 'audited'@'%'",
		"GRANT ALL PRIVILEGED ON *.* TO 'audited'@'%'",
	} {
		fixture := &t26vFixture{
			servers:    map[string]bool{"s1": true},
			account:    "audited@%",
			grantLines: []string{grant},
		}
		db := openT26VDB(t, fixture)
		provider := &t26vProvider{inner: mysqlmeta.NewProvider(db)}
		result, err := Audit(context.Background(), Request{
			SQL: "DROP SERVER s1", Dialect: DialectMySQL, Schema: "golden",
			MetadataProvider: provider, PrivilegeAuditMode: PrivilegeAuditModeObservedAccountNewSession,
		})
		if err != nil {
			t.Fatalf("%s: audit: %v", grant, err)
		}
		if result.Verdict != VerdictReview {
			t.Fatalf("%s: near token must degrade to review, got %+v", grant, result.Verdict)
		}
		stmt := result.Statements[0]
		if len(stmt.EvidenceGaps) != 1 {
			t.Fatalf("%s: gaps = %+v, want exactly the privilege gap", grant, stmt.EvidenceGaps)
		}
		facts := stmt.EvidenceGaps[0].RequiredFacts
		if len(facts) != 1 || facts[0] != "server.drop_privilege" {
			t.Fatalf("%s: owed = %v, want server.drop_privilege only", grant, facts)
		}
		if stmt.Findings[0].Metadata["outcome"] != "unverified" {
			t.Fatalf("%s: outcome = %v, want unverified", grant, stmt.Findings[0].Metadata["outcome"])
		}
		if stmt.Coverage.Status != CoverageUnverified || result.Coverage.Status != CoverageUnverified {
			t.Fatalf("%s: coverage = stmt:%v agg:%v, want unverified at both levels", grant, stmt.Coverage.Status, result.Coverage.Status)
		}
		if grantsQueries(fixture) != 1 {
			t.Fatalf("%s: the loader still ran once — unproven, never a denial", grant)
		}
	}
}

// Vertical chain D — the real convergent arm through the public result:
// a clean zero-row catalog answer (never proven absent), unproven
// pre-state + consumable privilege + IF EXISTS publishes the
// conditional absent the second guarded statement consumes. Distinct
// from chain A's third statement, which consumes an OrderedAbsent a
// proven drop already published — this first statement IS the
// conditional_absent publish.
func TestT26VerticalConvergentPublish(t *testing.T) {
	fixture := &t26vFixture{
		servers:    map[string]bool{},
		account:    "audited@%",
		grantLines: []string{"GRANT SUPER ON *.* TO 'audited'@'%'"},
	}
	db := openT26VDB(t, fixture)
	provider := &t26vProvider{inner: mysqlmeta.NewProvider(db)}
	result, err := Audit(context.Background(), Request{
		SQL:     "DROP SERVER IF EXISTS s1; DROP SERVER IF EXISTS s1",
		Dialect: DialectMySQL, Schema: "golden",
		MetadataProvider: provider, PrivilegeAuditMode: PrivilegeAuditModeObservedAccountNewSession,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 2 {
		t.Fatalf("statements = %d", len(result.Statements))
	}
	for i, want := range []string{"conditional_absent", "guarded_absent"} {
		stmt := result.Statements[i]
		if len(stmt.EvidenceGaps) != 0 {
			t.Fatalf("statement %d gaps = %+v, want none", i, stmt.EvidenceGaps)
		}
		finding := stmt.Findings[0]
		if finding.Metadata["outcome"] != want {
			t.Fatalf("statement %d outcome = %v, want %s", i, finding.Metadata["outcome"], want)
		}
		if finding.Metadata["conditional_audit"] != string(PrivilegeAuditModeObservedAccountNewSession) {
			t.Fatalf("statement %d marker missing: %+v", i, finding.Metadata)
		}
		if stmt.Coverage.Status != CoverageComplete {
			t.Fatalf("statement %d coverage = %v, want complete", i, stmt.Coverage.Status)
		}
	}
	if result.Coverage.Status != CoverageComplete {
		t.Fatalf("aggregate coverage = %v, want complete", result.Coverage.Status)
	}
	if result.Verdict != VerdictPass {
		t.Fatalf("convergent publish + guard must pass: %+v", result.Verdict)
	}
	// The zero-row read answered existence unknown — never absent —
	// and the second statement consumed the convergent publish with
	// no new resolver read. Total servers queries stay one.
	if serversQueries(fixture) != 1 {
		t.Fatalf("resolver reads = %d, want exactly 1 — the second guard consumed the first's publish", serversQueries(fixture))
	}
}
