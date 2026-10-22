// Package auditmeta verifies the T08-R1 default-comparison contract through
// the real mysqlmeta.Provider and shared AuditSQL path: a controlled
// database/sql driver feeds raw information_schema rows, so the provider's
// query, scan, and field mapping stay 100% production code. The matrix pins
// the knowledge boundary — a stored literal's decoded value bytes are the
// comparable identity, never the source-shaped DefaultValue text — across
// provider-sourced and CREATE-derived pre-state, MODIFY and CHANGE actions,
// and both MySQL/TiDB dialects. Package audit cannot host the real-provider
// cells because mysqlmeta already asserts audit.MetadataProvider.
// input: string-family and numeric-family NULL columns with provider-catalog
// or derived defaults, exercised through MODIFY/CHANGE under the isolated
// compatibility policy, including YEAR cells whose MySQL catalog normalizes
// the declared short-year integer to the full year
// output: zero findings for proven-same literals, one transition finding for
// proven-different ones inside a comparable domain, bounded evidence gaps
// for unproven or cross-encoding identities (including year-type literal
// pairs whose recorded text is not a proven final encoding, T08-R3), and an
// independent type-family finding while the identical default identity stays
// silent
// pos: application-layer real-provider regression tests for issue #87 T08-R1
// note: if this file changes, update this header and module README.md.
package auditmeta

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"

	appaudit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	mysqlmeta "github.com/Fanduzi/DeltaScope/internal/infrastructure/metadata/mysql"
)

const (
	t08r1ModifyRule = "ddl.alter.modify_column.compatibility.require"
	t08r1ChangeRule = "ddl.alter.change_column.compatibility.require"
	t08r1Table      = "golden.r1_defaults"
)

// --- controlled driver feeding the real provider's query set -------------

type t08r1Fixture struct {
	// version is the @@version banner the provider observes — the dialect
	// cross-check keys off the -TiDB- marker, so each cell declares its own
	// product like a real anchor would.
	version string
	// columns renders one information_schema.columns row per entry:
	// name, column_type, column_default (nil = stored SQL NULL), extra.
	columns []t08r1Column
}

type t08r1Column struct {
	name    string
	colType string
	def     driver.Value
	extra   string
}

func t08r1FixtureFor(dialect spec.Dialect, columns []t08r1Column) t08r1Fixture {
	version := "8.4.10"
	if dialect == spec.DialectTiDB {
		version = "8.0.11-TiDB-v8.5.0"
	}
	return t08r1Fixture{version: version, columns: columns}
}

var (
	t08r1Once  sync.Once
	t08r1State sync.Map
)

type t08r1Driver struct{}
type t08r1Conn struct{ fixture t08r1Fixture }
type t08r1Rows struct {
	cols  []string
	rows  [][]driver.Value
	index int
}

func openT08R1DB(t *testing.T, fixture t08r1Fixture) *sql.DB {
	t.Helper()
	t08r1Once.Do(func() { sql.Register("t08r1-mysqlmeta", t08r1Driver{}) })
	name := "t08r1-" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	t08r1State.Store(name, fixture)
	db, err := sql.Open("t08r1-mysqlmeta", name)
	if err != nil {
		t.Fatalf("open driver db: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		t08r1State.Delete(name)
	})
	return db
}

func (t08r1Driver) Open(name string) (driver.Conn, error) {
	value, _ := t08r1State.Load(name)
	fixture, _ := value.(t08r1Fixture)
	return t08r1Conn{fixture: fixture}, nil
}

func (t08r1Conn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (t08r1Conn) Close() error                        { return nil }
func (t08r1Conn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }

func (c t08r1Conn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	switch {
	case strings.Contains(query, "show variables"):
		version := c.fixture.version
		if version == "" {
			version = "8.4.10"
		}
		return &t08r1Rows{cols: []string{"Variable_name", "Value"}, rows: [][]driver.Value{{"version", version}}}, nil
	case strings.Contains(query, "show config"):
		return &t08r1Rows{
			cols: []string{"Type", "Instance", "Name", "Value"},
			rows: [][]driver.Value{{"tidb", "127.0.0.1:4000", "max-index-length", "3072"}},
		}, nil
	case strings.Contains(query, "information_schema.columns"):
		rows := make([][]driver.Value, 0, len(c.fixture.columns))
		for _, col := range c.fixture.columns {
			rows = append(rows, []driver.Value{col.name, col.colType, nil, nil, "", col.def, "YES", col.extra, ""})
		}
		return &t08r1Rows{
			cols: []string{"column_name", "column_type", "character_set_name", "collation_name", "column_comment", "column_default", "is_nullable", "extra", "generation_expression"},
			rows: rows,
		}, nil
	case strings.Contains(query, "information_schema.statistics"):
		return &t08r1Rows{
			cols: []string{"index_name", "non_unique", "index_type", "column_name", "cardinality", "sub_part", "collation"},
			rows: [][]driver.Value{{"PRIMARY", int64(0), "BTREE", "id", nil, nil, "A"}},
		}, nil
	case strings.Contains(query, "information_schema.tables"):
		return &t08r1Rows{
			cols: []string{"engine", "table_collation", "table_comment", "auto_increment", "row_format", "table_rows"},
			rows: [][]driver.Value{{"InnoDB", "utf8mb4_general_ci", "", nil, "Dynamic", int64(0)}},
		}, nil
	}
	return nil, sql.ErrNoRows
}

func (r *t08r1Rows) Columns() []string { return r.cols }
func (r *t08r1Rows) Close() error      { return nil }
func (r *t08r1Rows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

// t08r1CountingProvider delegates to the real provider while recording
// snapshot loads; it never rewrites produced fields. The returned snapshot
// pointer is captured for the post-audit immutability check.
type t08r1CountingProvider struct {
	inner  *mysqlmeta.Provider
	calls  []string
	loaded []*spec.TableSnapshot
}

func (p *t08r1CountingProvider) LoadInstanceFacts(ctx context.Context, dialect spec.Dialect, schema string) (*spec.InstanceFacts, error) {
	return p.inner.LoadInstanceFacts(ctx, dialect, schema)
}

func (p *t08r1CountingProvider) LoadTableSnapshot(ctx context.Context, dialect spec.Dialect, schema, table string) (*spec.TableSnapshot, error) {
	p.calls = append(p.calls, schema+"."+table)
	snapshot, err := p.inner.LoadTableSnapshot(ctx, dialect, schema, table)
	if err == nil && snapshot != nil {
		p.loaded = append(p.loaded, snapshot)
	}
	return snapshot, err
}

func t08r1Policy(t *testing.T, ruleID string) string {
	t.Helper()
	return t06a4Policy(t, map[string]string{ruleID: "      required: true\n"})
}

func t08r1Audit(t *testing.T, sqlText, policy string, dialect spec.Dialect, provider appaudit.MetadataProvider, wantStatements int) report.Result {
	t.Helper()
	result, err := appaudit.AuditSQL(context.Background(), appaudit.Request{
		SQL: sqlText, Dialect: dialect, Schema: "golden",
		ConfigPath: policy, MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if len(result.Statements) != wantStatements {
		t.Fatalf("statements = %d, want %d", len(result.Statements), wantStatements)
	}
	return result
}

// t08r1WantDefaultFindings asserts the compatibility findings on one
// statement: exactly wantCount findings carrying the bound rule ID, and —
// when present — the frozen transition metadata (class names only, never
// literal bytes).
func t08r1WantDefaultFindings(t *testing.T, stmt report.StatementResult, ruleID string, wantCount int) {
	t.Helper()
	matched := 0
	for _, finding := range stmt.Findings {
		if finding.RuleID == ruleID {
			matched++
			if !strings.Contains(finding.Message, "changes its default value") {
				t.Fatalf("compatibility finding message = %q, want the default transition", finding.Message)
			}
			if finding.Metadata["source_default"] == nil || finding.Metadata["target_default"] == nil {
				t.Fatalf("finding metadata missing default classes: %#v", finding.Metadata)
			}
		}
	}
	if matched != wantCount {
		t.Fatalf("default findings under %s = %d, want %d (all: %#v)", ruleID, matched, wantCount, stmt.Findings)
	}
}

// TestT08R1ProviderLiteralMatrix is path A: the stored COLUMN_DEFAULT bytes
// feed the real provider, and the MODIFY/CHANGE declaration feeds the real
// parser. Same-value cells must produce zero compatibility findings —
// source representation is never a semantic identity — while different and
// unproven cells keep their determined and bounded outcomes.
func TestT08R1ProviderLiteralMatrix(t *testing.T) {
	cells := []struct {
		name         string
		catalog      driver.Value
		extra        string
		colType      string
		targetClause string
		wantFindings int
	}{
		{"s1_same_string", "x", "", "varchar(16)", "DEFAULT 'x'", 0},
		{"s2_same_empty", "", "", "varchar(16)", "DEFAULT ''", 0},
		{"s3_same_text_null", "NULL", "", "varchar(16)", "DEFAULT 'NULL'", 0},
		{"s4_content_quotes", "'x'", "", "varchar(16)", "DEFAULT 'x'", 1},
		{"s5_content_quotes_reverse", "x", "", "varchar(16)", "DEFAULT '''x'''", 1},
		{"s6_different", "x", "", "varchar(16)", "DEFAULT 'y'", 1},
		{"s7_quote_in_content", "O'Reilly", "", "varchar(16)", "DEFAULT 'O''Reilly'", 0},
		{"keyword_string_stays_literal", "CURRENT_TIMESTAMP", "", "varchar(16)", "DEFAULT 'CURRENT_TIMESTAMP'", 0},
		{"numeric_same", "7", "", "int", "DEFAULT 7", 0},
		{"numeric_diff", "7", "", "int", "DEFAULT 8", 1},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		for _, action := range []struct {
			name   string
			sql    func(col, clause string) string
			ruleID string
		}{
			{"modify", func(col, clause string) string {
				return fmt.Sprintf("ALTER TABLE r1_defaults MODIFY COLUMN c %s NULL %s;", col, clause)
			}, t08r1ModifyRule},
			{"change", func(col, clause string) string {
				return fmt.Sprintf("ALTER TABLE r1_defaults CHANGE COLUMN c c %s NULL %s;", col, clause)
			}, t08r1ChangeRule},
		} {
			action := action
			for _, cell := range cells {
				cell := cell
				t.Run(fmt.Sprintf("%s/%s/%s", dialect, action.name, cell.name), func(t *testing.T) {
					fixture := t08r1FixtureFor(dialect, []t08r1Column{
						{name: "id", colType: "int"},
						{name: "c", colType: cell.colType, def: cell.catalog, extra: cell.extra},
					})
					db := openT08R1DB(t, fixture)
					provider := &t08r1CountingProvider{inner: mysqlmeta.NewProvider(db)}
					result := t08r1Audit(t, action.sql(cell.colType, cell.targetClause), t08r1Policy(t, action.ruleID), dialect, provider, 1)
					stmt := result.Statements[0]
					t08r1WantDefaultFindings(t, stmt, action.ruleID, cell.wantFindings)
					if len(provider.calls) != 1 || provider.calls[0] != t08r1Table {
						t.Fatalf("provider calls = %#v, want exactly one %s load", provider.calls, t08r1Table)
					}
					if cell.wantFindings == 0 && len(stmt.EvidenceGaps) != 0 {
						t.Fatalf("proven cell emitted gaps %#v", stmt.EvidenceGaps)
					}
				})
			}
		}
	}
}

// TestT08R2NumericComparableDomain pins the tightened numeric contract:
// kind=numeric marks the value domain, not a proven encoding — catalog
// scale-normalized text (1.00, -1.00) is not the AST integer's canonical
// text, so a representation-only difference is unproven (a bounded
// unproven_default_identity gap over both facts), never a determined
// change. Canonical integers keep comparing directly.
func TestT08R2NumericComparableDomain(t *testing.T) {
	unprovenPair := []string{"source_column.default", "target_column.default"}
	cells := []struct {
		name         string
		catalog      driver.Value
		colType      string
		targetClause string
		wantFindings int
		wantGap      []string // facts required under unproven_default_identity; nil = no gaps
	}{
		{"n1_decimal_scale", "1.00", "decimal(10,2)", "DEFAULT 1", 0, unprovenPair},
		{"n2_decimal_zero", "0.00", "decimal(10,2)", "DEFAULT 0", 0, unprovenPair},
		{"n3_decimal_negative", "-1.00", "decimal(10,2)", "DEFAULT -1", 0, unprovenPair},
		{"i1_int_same", "7", "int", "DEFAULT 7", 0, nil},
		{"i2_int_diff", "7", "int", "DEFAULT 8", 1, nil},
		{"s1_string_same", "x", "varchar(16)", "DEFAULT 'x'", 0, nil},
		// d1: a scale-0 decimal column stores the canonical integer text
		// itself — the encoding, not the type family, is the boundary, so
		// catalog `1` pairs against AST `1` as proven-same.
		{"d1_decimal_scale0_same", "1", "decimal(10,0)", "DEFAULT 1", 0, nil},
		// d2: the same effective value declared as a decimal literal carries
		// no comparable identity on the declaration side — a bounded
		// target-side unproven gap, never a determined change or same.
		{"d2_decimal_scale0_declared_scale", "1", "decimal(10,0)", "DEFAULT 1.0", 0, []string{"target_column.default"}},
		// y1/y3 (T08-R3, natively confirmed on mysql84 8.4.10): the MySQL
		// catalog normalizes a short YEAR input — declared 1 stores as
		// 2001 — so a year column's integer text is not a proven final
		// encoding; a different-text numeric pair touching a year column
		// stays unproven instead of convicting an identical stored default.
		{"y1_year_normalized", "2001", "year", "DEFAULT 1", 0, unprovenPair},
		{"y2_year_full_same", "2001", "year", "DEFAULT 2001", 0, nil},
		{"y3_year_diff_unproven", "2001", "year", "DEFAULT 2002", 0, unprovenPair},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		for _, action := range []struct {
			name   string
			sql    func(col, clause string) string
			ruleID string
		}{
			{"modify", func(col, clause string) string {
				return fmt.Sprintf("ALTER TABLE r1_defaults MODIFY COLUMN c %s NULL %s;", col, clause)
			}, t08r1ModifyRule},
			{"change", func(col, clause string) string {
				return fmt.Sprintf("ALTER TABLE r1_defaults CHANGE COLUMN c c %s NULL %s;", col, clause)
			}, t08r1ChangeRule},
		} {
			action := action
			for _, cell := range cells {
				cell := cell
				t.Run(fmt.Sprintf("%s/%s/%s", dialect, action.name, cell.name), func(t *testing.T) {
					fixture := t08r1FixtureFor(dialect, []t08r1Column{
						{name: "id", colType: "int"},
						{name: "c", colType: cell.colType, def: cell.catalog},
					})
					db := openT08R1DB(t, fixture)
					provider := &t08r1CountingProvider{inner: mysqlmeta.NewProvider(db)}
					result := t08r1Audit(t, action.sql(cell.colType, cell.targetClause), t08r1Policy(t, action.ruleID), dialect, provider, 1)
					stmt := result.Statements[0]
					t08r1WantDefaultFindings(t, stmt, action.ruleID, cell.wantFindings)
					if len(provider.calls) != 1 || provider.calls[0] != t08r1Table {
						t.Fatalf("provider calls = %#v, want exactly one %s load", provider.calls, t08r1Table)
					}
					if cell.wantGap == nil {
						if len(stmt.EvidenceGaps) != 0 {
							t.Fatalf("proven cell emitted gaps %#v", stmt.EvidenceGaps)
						}
						return
					}
					unprovenFacts := map[string]bool{}
					for _, gap := range stmt.EvidenceGaps {
						if gap.ReasonCode != "unproven_default_identity" {
							t.Fatalf("unexpected gap reason %q, want only unproven_default_identity", gap.ReasonCode)
						}
						for _, fact := range gap.RequiredFacts {
							unprovenFacts[fact] = true
						}
					}
					for _, fact := range cell.wantGap {
						if !unprovenFacts[fact] {
							t.Fatalf("unproven pair missing %q under unproven_default_identity, got %#v", fact, stmt.EvidenceGaps)
						}
					}
				})
			}
		}
	}
}

// TestT08R2DerivedNumericDomain proves the same comparable-domain boundary
// on the CREATE-derived path: a declared integer is canonical-integer text
// (provable across sources), while a declared decimal literal yields no
// comparable identity on the derived side either — the domain check cannot
// be bypassed by switching which source produced the pre-state.
func TestT08R2DerivedNumericDomain(t *testing.T) {
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		for _, cell := range []struct {
			name         string
			colType      string
			createClause string
			alterClause  string
			wantFindings int
			wantGap      bool
		}{
			{"derived_int_same", "DECIMAL(10,2)", "DEFAULT 1", "DEFAULT 1", 0, false},
			{"derived_int_diff", "DECIMAL(10,2)", "DEFAULT 1", "DEFAULT 2", 1, false},
			{"derived_decimal_unproven", "DECIMAL(10,2)", "DEFAULT 1.00", "DEFAULT 1", 0, true},
			{"derived_decimal_diff_unproven", "DECIMAL(10,2)", "DEFAULT 1.00", "DEFAULT 2.00", 0, true},
			// YEAR normalization (T08-R3): the derived source carries the
			// declared integer just like the catalog carries its bytes —
			// the provider/declaration split may not diverge, so the same
			// year gate applies on the CREATE-derived path.
			{"derived_year_same", "YEAR", "DEFAULT 1", "DEFAULT 1", 0, false},
			{"derived_year_normalized_unproven", "YEAR", "DEFAULT 1", "DEFAULT 2001", 0, true},
			{"derived_year_full_same", "YEAR", "DEFAULT 2001", "DEFAULT 2001", 0, false},
		} {
			cell := cell
			t.Run(fmt.Sprintf("%s/%s", dialect, cell.name), func(t *testing.T) {
				sqlText := fmt.Sprintf("CREATE TABLE r1_defaults (id INT PRIMARY KEY, c %s NULL %s);\n", cell.colType, cell.createClause) +
					fmt.Sprintf("ALTER TABLE r1_defaults MODIFY COLUMN c %s NULL %s;", cell.colType, cell.alterClause)
				result := t08r1Audit(t, sqlText, t08r1Policy(t, t08r1ModifyRule), dialect, t06a4AbsentProvider{}, 2)
				stmt := result.Statements[1]
				t08r1WantDefaultFindings(t, stmt, t08r1ModifyRule, cell.wantFindings)
				if cell.wantGap && len(stmt.EvidenceGaps) == 0 {
					t.Fatalf("unproven derived cell emitted no gaps, want bounded unproven")
				}
				if !cell.wantGap && len(stmt.EvidenceGaps) != 0 {
					t.Fatalf("proven derived cell emitted gaps %#v", stmt.EvidenceGaps)
				}
			})
		}
	}
}

// TestT08R2TypeChangeKeepsDefaultVerdict separates the default comparison
// from the independent type finding: identical literal identity (same kind,
// same canonical text) under different source/target column types stays
// silent on the default attribute while the type-family transition still
// fires — the type finding does not launder a default verdict either way.
func TestT08R2TypeChangeKeepsDefaultVerdict(t *testing.T) {
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		for _, action := range []struct {
			name   string
			sql    string
			ruleID string
		}{
			{"modify", "ALTER TABLE r1_defaults MODIFY COLUMN c DECIMAL(10,0) NULL DEFAULT 7;", t08r1ModifyRule},
			{"change", "ALTER TABLE r1_defaults CHANGE COLUMN c c DECIMAL(10,0) NULL DEFAULT 7;", t08r1ChangeRule},
		} {
			action := action
			t.Run(fmt.Sprintf("%s/%s", dialect, action.name), func(t *testing.T) {
				fixture := t08r1FixtureFor(dialect, []t08r1Column{
					{name: "id", colType: "int"},
					{name: "c", colType: "int", def: "7"},
				})
				db := openT08R1DB(t, fixture)
				provider := &t08r1CountingProvider{inner: mysqlmeta.NewProvider(db)}
				result := t08r1Audit(t, action.sql, t08r1Policy(t, action.ruleID), dialect, provider, 1)
				stmt := result.Statements[0]
				defaultFindings, typeFindings := 0, 0
				for _, finding := range stmt.Findings {
					if finding.RuleID != action.ruleID {
						continue
					}
					if strings.Contains(finding.Message, "default value") {
						defaultFindings++
					}
					if strings.Contains(finding.Message, "changes type family") {
						typeFindings++
					}
				}
				if defaultFindings != 0 {
					t.Fatalf("default transition findings = %d, want 0 (all: %#v)", defaultFindings, stmt.Findings)
				}
				if typeFindings != 1 {
					t.Fatalf("type-family findings = %d, want 1 (all: %#v)", typeFindings, stmt.Findings)
				}
				if len(stmt.EvidenceGaps) != 0 {
					t.Fatalf("silent default verdict emitted gaps %#v", stmt.EvidenceGaps)
				}
			})
		}
	}
}

// TestT08R1ChangeRenameReadsOldName pins the CHANGE source identity: the
// comparison must read the old-named catalog column (c, default 'x'), never
// borrow the destination-named sibling (c2, default 'z').
func TestT08R1ChangeRenameReadsOldName(t *testing.T) {
	fixture := t08r1FixtureFor(spec.DialectMySQL, []t08r1Column{
		{name: "id", colType: "int"},
		{name: "c", colType: "varchar(16)", def: "x"},
		{name: "c2", colType: "varchar(16)", def: "z"},
	})
	db := openT08R1DB(t, fixture)
	provider := &t08r1CountingProvider{inner: mysqlmeta.NewProvider(db)}
	result := t08r1Audit(t, "ALTER TABLE r1_defaults CHANGE COLUMN c c2 VARCHAR(16) NULL DEFAULT 'x';",
		t08r1Policy(t, t08r1ChangeRule), spec.DialectMySQL, provider, 1)
	t08r1WantDefaultFindings(t, result.Statements[0], t08r1ChangeRule, 0)
}

// TestT08R1ExpressionIdentityMatrix pins the knowledge boundary: equal
// fingerprints prove same; a source expression the catalog text could not
// decode is unproven; a declared literal the AST cannot decode into a
// comparable identity (b'1' has no cross-source text identity) is unproven
// on the target side. Unproven is always a bounded evidence gap, never a
// determined transition finding.
func TestT08R1ExpressionIdentityMatrix(t *testing.T) {
	cells := []struct {
		name         string
		catalog      driver.Value
		extra        string
		targetClause string
		wantFindings int
		wantGapFact  string // "" = expect no gap
	}{
		{"expr_same", "(uuid())", "DEFAULT_GENERATED", "DEFAULT (uuid())", 0, ""},
		{"expr_diff", "(uuid())", "DEFAULT_GENERATED", "DEFAULT (1 + 2)", 1, ""},
		{"expr_unknown_source", "", "DEFAULT_GENERATED", "DEFAULT (uuid())", 0, "source_column.default"},
		{"literal_unknown_target", "x", "", "DEFAULT b'1'", 0, "target_column.default"},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		for _, cell := range cells {
			cell := cell
			t.Run(fmt.Sprintf("%s/%s", dialect, cell.name), func(t *testing.T) {
				fixture := t08r1FixtureFor(dialect, []t08r1Column{
					{name: "id", colType: "int"},
					{name: "c", colType: "varchar(16)", def: cell.catalog, extra: cell.extra},
				})
				db := openT08R1DB(t, fixture)
				provider := &t08r1CountingProvider{inner: mysqlmeta.NewProvider(db)}
				result := t08r1Audit(t, "ALTER TABLE r1_defaults MODIFY COLUMN c VARCHAR(16) NULL "+cell.targetClause+";",
					t08r1Policy(t, t08r1ModifyRule), dialect, provider, 1)
				stmt := result.Statements[0]
				t08r1WantDefaultFindings(t, stmt, t08r1ModifyRule, cell.wantFindings)
				if cell.wantGapFact == "" {
					if len(stmt.EvidenceGaps) != 0 {
						t.Fatalf("proven cell emitted gaps %#v", stmt.EvidenceGaps)
					}
					return
				}
				found := false
				for _, gap := range stmt.EvidenceGaps {
					for _, fact := range gap.RequiredFacts {
						if fact == cell.wantGapFact {
							found = true
						}
					}
				}
				if !found {
					t.Fatalf("unproven cell missing %q gap, got %#v", cell.wantGapFact, stmt.EvidenceGaps)
				}
			})
		}
	}
}

// TestT08R1DerivedVsProviderSameOutcome is path B: the identical logical
// pre-state created by an earlier CREATE must judge exactly like the
// provider-sourced one — the source of the default facts is not the
// identity.
func TestT08R1DerivedVsProviderSameOutcome(t *testing.T) {
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		for _, cell := range []struct {
			name         string
			createClause string
			alterClause  string
			wantFindings int
		}{
			{"same", "DEFAULT 'x'", "DEFAULT 'x'", 0},
			{"diff", "DEFAULT 'x'", "DEFAULT 'y'", 1},
			{"cleared", "DEFAULT 'x'", "", 1},
		} {
			cell := cell
			t.Run(fmt.Sprintf("%s/%s", dialect, cell.name), func(t *testing.T) {
				sqlText := fmt.Sprintf("CREATE TABLE r1_defaults (id INT PRIMARY KEY, c VARCHAR(16) NULL %s);\n", cell.createClause) +
					fmt.Sprintf("ALTER TABLE r1_defaults MODIFY COLUMN c VARCHAR(16) NULL %s;", cell.alterClause)
				result := t08r1Audit(t, sqlText, t08r1Policy(t, t08r1ModifyRule), dialect, t06a4AbsentProvider{}, 2)
				stmt := result.Statements[1]
				if cell.wantFindings == 0 {
					t08r1WantDefaultFindings(t, stmt, t08r1ModifyRule, 0)
					if len(stmt.Findings) != 0 || len(stmt.EvidenceGaps) != 0 || stmt.Coverage.Status != report.CoverageComplete {
						t.Fatalf("same-value derived cell = findings %#v gaps %#v coverage %s, want clean/complete",
							stmt.Findings, stmt.EvidenceGaps, stmt.Coverage.Status)
					}
					return
				}
				matched := 0
				for _, finding := range stmt.Findings {
					if finding.RuleID == t08r1ModifyRule {
						matched++
					}
				}
				if matched != cell.wantFindings {
					t.Fatalf("derived findings = %d, want %d (all: %#v)", matched, cell.wantFindings, stmt.Findings)
				}
			})
		}
	}
}

// TestT08R1DerivedStateChains pins the ordered-state contract: a setter
// must publish the new comparable identity (never leave the provider's
// stale one), DROP DEFAULT must clear it (a follow-up MODIFY sees a proven
// none→literal gain, not a silent false-same), and CHANGE must carry the
// identity under the new name.
func TestT08R1DerivedStateChains(t *testing.T) {
	cells := []struct {
		name        string
		catalog     driver.Value
		colType     string
		batch       string // %s = column type
		wantMessage string // "" = no default finding expected
	}{
		{
			name:    "set_default_publishes_new_identity",
			catalog: "x", colType: "varchar(16)",
			batch: "ALTER TABLE r1_defaults ALTER COLUMN c SET DEFAULT 'y';\n" +
				"ALTER TABLE r1_defaults MODIFY COLUMN c %s NULL DEFAULT 'y';",
			wantMessage: "",
		},
		{
			name:    "drop_default_clears_identity",
			catalog: "x", colType: "varchar(16)",
			batch: "ALTER TABLE r1_defaults ALTER COLUMN c DROP DEFAULT;\n" +
				"ALTER TABLE r1_defaults MODIFY COLUMN c %s NULL DEFAULT 'x';",
			wantMessage: "gains a default value",
		},
		{
			name:    "change_rename_carries_identity",
			catalog: "x", colType: "varchar(16)",
			batch: "ALTER TABLE r1_defaults CHANGE COLUMN c c2 %s NULL DEFAULT 'x';\n" +
				"ALTER TABLE r1_defaults MODIFY COLUMN c2 %s NULL DEFAULT 'x';",
			wantMessage: "",
		},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		for _, cell := range cells {
			cell := cell
			t.Run(fmt.Sprintf("%s/%s", dialect, cell.name), func(t *testing.T) {
				fixture := t08r1FixtureFor(dialect, []t08r1Column{
					{name: "id", colType: "int"},
					{name: "c", colType: cell.colType, def: cell.catalog},
				})
				db := openT08R1DB(t, fixture)
				provider := &t08r1CountingProvider{inner: mysqlmeta.NewProvider(db)}
				args := []any{cell.colType}
				if cell.name == "change_rename_carries_identity" {
					args = append(args, cell.colType)
				}
				sqlText := fmt.Sprintf(cell.batch, args...)
				result := t08r1Audit(t, sqlText, t08r1Policy(t, t08r1ModifyRule), dialect, provider, 2)
				stmt := result.Statements[1]
				matched := 0
				for _, finding := range stmt.Findings {
					if finding.RuleID == t08r1ModifyRule &&
						(finding.Metadata["source_default"] != nil || finding.Metadata["target_default"] != nil) {
						matched++
						if cell.wantMessage == "" || !strings.Contains(finding.Message, cell.wantMessage) {
							t.Fatalf("default finding message = %q, want %q", finding.Message, cell.wantMessage)
						}
					}
				}
				wantFindings := 0
				if cell.wantMessage != "" {
					wantFindings = 1
				}
				if matched != wantFindings {
					t.Fatalf("second-statement default findings = %d, want %d (all: %#v)", matched, wantFindings, stmt.Findings)
				}
				if len(provider.calls) != 1 || provider.calls[0] != t08r1Table {
					t.Fatalf("provider calls = %#v, want exactly one %s load", provider.calls, t08r1Table)
				}
			})
		}
	}
}

// TestT08R1ExplicitPolicySeparation keeps the two rule families honest:
// independently enabling the explicit-default-change forbid still reports
// the authored operation on a same-value MODIFY, and the compatibility rule
// must not add a false transition beside it.
func TestT08R1ExplicitPolicySeparation(t *testing.T) {
	policy := t06a4Policy(t, map[string]string{
		t08r1ModifyRule: "      required: true\n",
		"ddl.alter.modify_column.explicit_default_change.forbid": "",
	})
	fixture := t08r1FixtureFor(spec.DialectMySQL, []t08r1Column{
		{name: "id", colType: "int"},
		{name: "c", colType: "varchar(16)", def: "x"},
	})
	db := openT08R1DB(t, fixture)
	provider := &t08r1CountingProvider{inner: mysqlmeta.NewProvider(db)}
	result := t08r1Audit(t, "ALTER TABLE r1_defaults MODIFY COLUMN c VARCHAR(16) NULL DEFAULT 'x';",
		policy, spec.DialectMySQL, provider, 1)
	stmt := result.Statements[0]
	explicit := 0
	compat := 0
	for _, finding := range stmt.Findings {
		switch finding.RuleID {
		case "ddl.alter.modify_column.explicit_default_change.forbid":
			explicit++
		case t08r1ModifyRule:
			compat++
		}
	}
	if explicit != 1 {
		t.Fatalf("explicit default-change findings = %d, want 1 (all: %#v)", explicit, stmt.Findings)
	}
	if compat != 0 {
		t.Fatalf("same-value compat findings = %d, want 0 beside the explicit blocker (all: %#v)", compat, stmt.Findings)
	}
}

// TestT08R1ProviderSnapshotUntouched proves the audit never mutates the
// provider-returned snapshot: the loaded object must byte-compare equal to
// a second load of the same fixture after the audit ran.
func TestT08R1ProviderSnapshotUntouched(t *testing.T) {
	fixture := t08r1FixtureFor(spec.DialectMySQL, []t08r1Column{
		{name: "id", colType: "int"},
		{name: "c", colType: "varchar(16)", def: "x"},
	})
	db := openT08R1DB(t, fixture)
	provider := &t08r1CountingProvider{inner: mysqlmeta.NewProvider(db)}
	result := t08r1Audit(t, "ALTER TABLE r1_defaults MODIFY COLUMN c VARCHAR(16) NULL DEFAULT 'y';",
		t08r1Policy(t, t08r1ModifyRule), spec.DialectMySQL, provider, 1)
	_ = result
	if len(provider.loaded) == 0 {
		t.Fatal("provider snapshot never captured")
	}
	fresh, err := mysqlmeta.NewProvider(db).LoadTableSnapshot(context.Background(), spec.DialectMySQL, "golden", "r1_defaults")
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !reflect.DeepEqual(provider.loaded[0], fresh) {
		t.Fatalf("provider snapshot mutated by audit:\nfirst %+v\nfresh %+v", provider.loaded[0], fresh)
	}
}
