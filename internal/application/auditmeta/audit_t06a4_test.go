// Package auditmeta verifies the T06-A4 provider default-identity contract
// through the shared AuditSQL path with the real mysqlmeta.Provider: the
// provider's output feeds the ordered state exactly as production wiring
// does. A controlled database/sql driver supplies raw information_schema
// rows — the provider, its queries, and its field mapping stay 100%
// production code (never a hand-built spec.Column, never a copied
// loadColumns). Package audit cannot host this test because mysqlmeta
// already asserts the audit.MetadataProvider interface (import cycle).
// input: two-statement ALTER DROP COLUMN + CREATE INDEX batches against driver-fed catalog rows
// output: review/unverified for string-literal siblings (A/B), complete/pass for stored SQL NULL (C)
// pos: application-layer real-provider contract tests for issue #85 T06-A4
// note: if this file changes, update this header and module README.md.
package auditmeta

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	appaudit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	domainpolicy "github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	mysqlmeta "github.com/Fanduzi/DeltaScope/internal/infrastructure/metadata/mysql"
)

const (
	t06a4AlterRequire = "ddl.table.exists.alter.require"
	t06a4DropExists   = "ddl.alter.drop_column.exists.require"
	t06a4IndexColumns = "ddl.create_index.columns.exists.require"
	t06a4DefaultRule  = "ddl.column.default.require"
	t06a4Table        = "golden.t06_a4_defaults"

	t06a4DropDSQL = "ALTER TABLE t06_a4_defaults DROP COLUMN d;\nCREATE INDEX idx_b ON t06_a4_defaults(b);"
	t06a4DropCSQL = "ALTER TABLE t06_a4_defaults DROP COLUMN c;\nCREATE INDEX idx_b ON t06_a4_defaults(b);"
)

// --- controlled driver feeding only the queries the real provider issues ---

type t06a4Result struct {
	cols []string
	rows [][]driver.Value
	err  error
}

type t06a4Fixture struct {
	// cDefault is c's raw COLUMN_DEFAULT: nil = stored SQL NULL (C shape),
	// "NULL" = the stored byte text of DEFAULT 'NULL' (A/B shape).
	cDefault  driver.Value
	columnsEr error
}

var (
	t06a4Once  sync.Once
	t06a4State sync.Map
)

type t06a4Driver struct{}
type t06a4Conn struct{ fixture t06a4Fixture }
type t06a4Rows struct {
	cols  []string
	rows  [][]driver.Value
	index int
}

func openT06A4DB(t *testing.T, fixture t06a4Fixture) *sql.DB {
	t.Helper()
	t06a4Once.Do(func() { sql.Register("t06a4-mysqlmeta", t06a4Driver{}) })
	name := "t06a4-" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	t06a4State.Store(name, fixture)
	db, err := sql.Open("t06a4-mysqlmeta", name)
	if err != nil {
		t.Fatalf("open driver db: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		t06a4State.Delete(name)
	})
	return db
}

func (t06a4Driver) Open(name string) (driver.Conn, error) {
	value, _ := t06a4State.Load(name)
	fixture, _ := value.(t06a4Fixture)
	return t06a4Conn{fixture: fixture}, nil
}

func (t06a4Conn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (t06a4Conn) Close() error                        { return nil }
func (t06a4Conn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }

func (c t06a4Conn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	result := c.answer(query)
	if result.err != nil {
		return nil, result.err
	}
	return &t06a4Rows{cols: result.cols, rows: result.rows}, nil
}

func (c t06a4Conn) answer(query string) t06a4Result {
	switch {
	case strings.Contains(query, "show variables"):
		return t06a4Result{
			cols: []string{"Variable_name", "Value"},
			rows: [][]driver.Value{{"version", "8.4.10"}},
		}
	case strings.Contains(query, "information_schema.columns"):
		if c.fixture.columnsEr != nil {
			return t06a4Result{err: c.fixture.columnsEr}
		}
		// Raw catalog rows for the frozen fixture:
		// id INT PRIMARY KEY, a/b VARCHAR(8) with SQL NULL defaults,
		// c VARCHAR(8) carrying the variant default, d DEFAULT '<nil>'.
		return t06a4Result{
			cols: []string{"column_name", "column_type", "character_set_name", "collation_name", "column_comment", "column_default", "is_nullable", "extra"},
			rows: [][]driver.Value{
				{"id", "int", nil, nil, "", nil, "NO", ""},
				{"a", "varchar(8)", nil, nil, "", nil, "YES", ""},
				{"b", "varchar(8)", nil, nil, "", nil, "YES", ""},
				{"c", "varchar(8)", nil, nil, "", c.fixture.cDefault, "YES", ""},
				{"d", "varchar(8)", nil, nil, "", "<nil>", "YES", ""},
			},
		}
	case strings.Contains(query, "information_schema.statistics"):
		return t06a4Result{
			cols: []string{"index_name", "non_unique", "index_type", "column_name", "cardinality"},
			rows: [][]driver.Value{{"PRIMARY", int64(0), "BTREE", "id", nil}},
		}
	case strings.Contains(query, "information_schema.tables"):
		return t06a4Result{
			cols: []string{"engine", "table_collation", "table_comment", "auto_increment", "row_format", "table_rows"},
			rows: [][]driver.Value{{"InnoDB", "utf8mb4_general_ci", "", nil, "Dynamic", int64(0)}},
		}
	}
	return t06a4Result{err: sql.ErrNoRows}
}

func (r *t06a4Rows) Columns() []string { return r.cols }
func (r *t06a4Rows) Close() error      { return nil }
func (r *t06a4Rows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

// t06a4CountingProvider delegates to the real provider while recording
// snapshot loads; it never rewrites produced fields.
type t06a4CountingProvider struct {
	inner *mysqlmeta.Provider
	calls []string
}

func (p *t06a4CountingProvider) LoadInstanceFacts(ctx context.Context, dialect spec.Dialect, schema string) (*spec.InstanceFacts, error) {
	return p.inner.LoadInstanceFacts(ctx, dialect, schema)
}

func (p *t06a4CountingProvider) LoadTableSnapshot(ctx context.Context, dialect spec.Dialect, schema, table string) (*spec.TableSnapshot, error) {
	p.calls = append(p.calls, schema+"."+table)
	return p.inner.LoadTableSnapshot(ctx, dialect, schema, table)
}

// t06a4AbsentProvider answers exists:false for the parsed-batch contrast
// path (batch-derived state instead of provider-derived).
type t06a4AbsentProvider struct{}

func (t06a4AbsentProvider) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	return &spec.InstanceFacts{}, nil
}

func (t06a4AbsentProvider) LoadTableSnapshot(_ context.Context, _ spec.Dialect, schema, table string) (*spec.TableSnapshot, error) {
	return &spec.TableSnapshot{Exists: false, Table: &spec.Table{Schema: schema, Name: table}}, nil
}

func t06a4Policy(t *testing.T, enabled map[string]string) string {
	t.Helper()
	var builder strings.Builder
	builder.WriteString("rules:\n")
	ruleIDs := make([]string, 0, len(domainpolicy.Default().Rules))
	for id := range domainpolicy.Default().Rules {
		ruleIDs = append(ruleIDs, id)
	}
	sort.Strings(ruleIDs)
	for _, id := range ruleIDs {
		if _, keep := enabled[id]; keep {
			continue
		}
		fmt.Fprintf(&builder, "  %s:\n    enabled: false\n", strconv.Quote(id))
	}
	for id, params := range enabled {
		fmt.Fprintf(&builder, "  %s:\n    enabled: true\n    level: blocker\n", strconv.Quote(id))
		if params != "" {
			builder.WriteString("    params:\n")
			builder.WriteString(params)
		}
	}
	path := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatalf("write policy: %v", err)
	}
	return path
}

// t06a4IsolatedPolicy is the frozen t06-a4-provider-defaults-isolated
// profile: three blockers, index rule required:true, everything else off.
func t06a4IsolatedPolicy(t *testing.T) string {
	t.Helper()
	return t06a4Policy(t, map[string]string{
		t06a4AlterRequire: "",
		t06a4DropExists:   "",
		t06a4IndexColumns: "      required: true\n",
	})
}

func t06a4Audit(t *testing.T, sqlText, policy string, provider appaudit.MetadataProvider) report.Result {
	t.Helper()
	result, err := appaudit.AuditSQL(context.Background(), appaudit.Request{
		SQL: sqlText, Dialect: spec.DialectMySQL, Schema: "golden",
		ConfigPath: policy, MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if len(result.Statements) != 2 {
		t.Fatalf("statements = %d, want 2", len(result.Statements))
	}
	for i, statement := range result.Statements {
		if statement.Index != i {
			t.Fatalf("statement %d index = %d", i, statement.Index)
		}
	}
	return result
}

func t06a4GapsByRule(result report.Result, statementIndex int, ruleID string) []rule.EvidenceGap {
	out := make([]rule.EvidenceGap, 0)
	for _, gap := range result.Statements[statementIndex].EvidenceGaps {
		if gap.RuleID == ruleID {
			out = append(out, gap)
		}
	}
	return out
}

// t06a4WantConservative asserts the shared A/B contract through the real
// provider: statement 0 complete, statement 1 unverified with exactly one
// frozen unknown_table_state gap, aggregate review/unverified, one snapshot
// read, error nil.
func t06a4WantConservative(t *testing.T, result report.Result, provider *t06a4CountingProvider) {
	t.Helper()
	if result.Verdict != report.VerdictReview || result.Coverage.Status != report.CoverageUnverified {
		t.Fatalf("aggregate = %s/%s, want review/unverified", result.Verdict, result.Coverage.Status)
	}
	first := result.Statements[0]
	if first.Coverage.Status != report.CoverageComplete || len(first.Findings) != 0 || len(first.EvidenceGaps) != 0 {
		t.Fatalf("statement 0 = %s findings %#v gaps %#v, want clean", first.Coverage.Status, first.Findings, first.EvidenceGaps)
	}
	second := result.Statements[1]
	if second.Coverage.Status != report.CoverageUnverified || len(second.Findings) != 0 {
		t.Fatalf("statement 1 = %s findings %#v, want unverified with no findings", second.Coverage.Status, second.Findings)
	}
	gaps := t06a4GapsByRule(result, 1, t06a4IndexColumns)
	if len(gaps) != 1 || gaps[0].ReasonCode != "unknown_table_state" ||
		!reflect.DeepEqual(gaps[0].RequiredFacts, []string{"target_table.columns", "target_table.existence"}) {
		t.Fatalf("statement 1 gaps = %#v, want exactly one frozen unknown_table_state", second.EvidenceGaps)
	}
	if len(provider.calls) != 1 || provider.calls[0] != t06a4Table {
		t.Fatalf("provider calls = %#v, want exactly one %s load", provider.calls, t06a4Table)
	}
}

// TestT06A4RealProviderTextNullDropStaysConservative is path A: the stored
// 'NULL' byte text on sibling c must not read as a SQL NULL default, so
// DROP d cannot publish a precise post-state.
func TestT06A4RealProviderTextNullDropStaysConservative(t *testing.T) {
	db := openT06A4DB(t, t06a4Fixture{cDefault: "NULL"})
	provider := &t06a4CountingProvider{inner: mysqlmeta.NewProvider(db)}
	result := t06a4Audit(t, t06a4DropDSQL, t06a4IsolatedPolicy(t), provider)
	t06a4WantConservative(t, result, provider)
}

// TestT06A4RealProviderTextNilDropStaysConservative is path B: dropping c
// leaves sibling d's stored '<nil>' literal — already conservative, must
// stay so.
func TestT06A4RealProviderTextNilDropStaysConservative(t *testing.T) {
	db := openT06A4DB(t, t06a4Fixture{cDefault: "NULL"})
	provider := &t06a4CountingProvider{inner: mysqlmeta.NewProvider(db)}
	result := t06a4Audit(t, t06a4DropCSQL, t06a4IsolatedPolicy(t), provider)
	t06a4WantConservative(t, result, provider)
}

// TestT06A4RealProviderNullDropStaysPrecise is path C: a stored SQL NULL
// default on c keeps the precise path — the fix must not invalidate every
// metadata-backed DROP.
func TestT06A4RealProviderNullDropStaysPrecise(t *testing.T) {
	db := openT06A4DB(t, t06a4Fixture{cDefault: nil})
	provider := &t06a4CountingProvider{inner: mysqlmeta.NewProvider(db)}
	result := t06a4Audit(t, t06a4DropDSQL, t06a4IsolatedPolicy(t), provider)
	if result.Verdict != report.VerdictPass || result.Coverage.Status != report.CoverageComplete {
		t.Fatalf("aggregate = %s/%s, want pass/complete", result.Verdict, result.Coverage.Status)
	}
	for i, statement := range result.Statements {
		if statement.Coverage.Status != report.CoverageComplete || len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 {
			t.Fatalf("statement %d = %s findings %#v gaps %#v, want clean", i, statement.Coverage.Status, statement.Findings, statement.EvidenceGaps)
		}
	}
	if len(provider.calls) != 1 || provider.calls[0] != t06a4Table {
		t.Fatalf("provider calls = %#v, want exactly one load", provider.calls)
	}
}

// TestT06A4SourceContrastSameOutcome pins the cross-source contract: the
// stored 'NULL' literal read by the provider and the same literal parsed
// in-batch must both keep the conservative drop.
func TestT06A4SourceContrastSameOutcome(t *testing.T) {
	db := openT06A4DB(t, t06a4Fixture{cDefault: "NULL"})
	providerResult := t06a4Audit(t, t06a4DropDSQL, t06a4IsolatedPolicy(t),
		&t06a4CountingProvider{inner: mysqlmeta.NewProvider(db)})

	parsedBatch := "CREATE TABLE t06_a4_defaults (id INT PRIMARY KEY, a VARCHAR(8), b VARCHAR(8) DEFAULT NULL, c VARCHAR(8) DEFAULT 'NULL', d VARCHAR(8) DEFAULT '<nil>');\n" +
		"ALTER TABLE t06_a4_defaults DROP COLUMN d;\n" +
		"CREATE INDEX idx_b ON t06_a4_defaults(b);"
	result, err := appaudit.AuditSQL(context.Background(), appaudit.Request{
		SQL: parsedBatch, Dialect: spec.DialectMySQL, Schema: "golden",
		ConfigPath: t06a4IsolatedPolicy(t), MetadataProvider: t06a4AbsentProvider{},
	})
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	parsedResult := result

	if providerResult.Verdict != parsedResult.Verdict || providerResult.Coverage.Status != parsedResult.Coverage.Status {
		t.Fatalf("provider %s/%s vs parsed %s/%s, want identical conservative outcome",
			providerResult.Verdict, providerResult.Coverage.Status, parsedResult.Verdict, parsedResult.Coverage.Status)
	}
	if providerResult.Verdict != report.VerdictReview {
		t.Fatalf("both sources = %s, want review", providerResult.Verdict)
	}
}

// TestT06A4RealProviderErrorPropagates keeps the provider error channel
// honest through AuditSQL: a columns-query failure surfaces as an error
// (errors.Is), never as an unknown table or an evidence gap.
func TestT06A4RealProviderErrorPropagates(t *testing.T) {
	sentinel := errors.New("columns access denied")
	db := openT06A4DB(t, t06a4Fixture{cDefault: "NULL", columnsEr: sentinel})
	_, err := appaudit.AuditSQL(context.Background(), appaudit.Request{
		SQL: t06a4DropDSQL, Dialect: spec.DialectMySQL, Schema: "golden",
		ConfigPath: t06a4IsolatedPolicy(t), MetadataProvider: mysqlmeta.NewProvider(db),
	})
	if err == nil || !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want wrapped %v", err, sentinel)
	}
}

// TestT06A4DefaultRequireReadsOnlySubmittedDDL pins group 5: provider facts
// (b's stored SQL NULL surfacing as HasDefault=false) must not produce a
// default-require finding — the rule reads only submitted CREATE TABLE
// columns.
func TestT06A4DefaultRequireReadsOnlySubmittedDDL(t *testing.T) {
	policy := t06a4Policy(t, map[string]string{
		t06a4AlterRequire: "",
		t06a4DropExists:   "",
		t06a4IndexColumns: "      required: true\n",
		t06a4DefaultRule:  "      required: true\n",
	})
	db := openT06A4DB(t, t06a4Fixture{cDefault: "NULL"})
	result := t06a4Audit(t, t06a4DropDSQL, policy, &t06a4CountingProvider{inner: mysqlmeta.NewProvider(db)})
	for i, statement := range result.Statements {
		for _, finding := range statement.Findings {
			if finding.RuleID == t06a4DefaultRule {
				t.Fatalf("statement %d produced a default-require finding %#v from provider facts", i, finding)
			}
		}
	}
}

// TestT06A4DisabledRulesStaySilent pins the negative control: all rules off
// must not fabricate findings, gaps, or capability errors on this path.
func TestT06A4DisabledRulesStaySilent(t *testing.T) {
	db := openT06A4DB(t, t06a4Fixture{cDefault: "NULL"})
	result := t06a4Audit(t, t06a4DropDSQL, t06a4Policy(t, map[string]string{}),
		&t06a4CountingProvider{inner: mysqlmeta.NewProvider(db)})
	for i, statement := range result.Statements {
		if len(statement.Findings) != 0 {
			t.Fatalf("statement %d findings = %#v, want none with all rules off", i, statement.Findings)
		}
	}
}
