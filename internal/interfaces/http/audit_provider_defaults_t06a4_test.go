// Package http verifies the T06-A4 provider default-identity contract at the
// HTTP seam with the real mysqlmeta.Provider behind a controlled
// database/sql driver — one conservative A representative and one precise C
// representative; the full matrix lives in the shared layer and Golden runs.
// input: provider-fed ALTER DROP COLUMN + CREATE INDEX audits through /v1/audit
// output: review/unverified for the stored 'NULL' literal; pass/complete for stored SQL NULL
// pos: HTTP transport contract test for issue #85 T06-A4
// note: if this file changes, update this header and module README.md.
package httpapi

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	mysqlmeta "github.com/Fanduzi/DeltaScope/internal/infrastructure/metadata/mysql"
	"github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

// t06a4HTTPProvider adapts the real mysql provider to the public
// MetadataProvider interface; it never rewrites produced fields.
type t06a4HTTPProvider struct {
	inner *mysqlmeta.Provider
	calls []string
}

func (p *t06a4HTTPProvider) LoadInstanceFacts(ctx context.Context, _ deltascope.Dialect, schema string) (*deltascope.InstanceFacts, error) {
	return p.inner.LoadInstanceFacts(ctx, spec.DialectMySQL, schema)
}

func (p *t06a4HTTPProvider) LoadTableSnapshot(ctx context.Context, _ deltascope.Dialect, schema, table string) (*deltascope.TableSnapshot, error) {
	p.calls = append(p.calls, schema+"."+table)
	return p.inner.LoadTableSnapshot(ctx, spec.DialectMySQL, schema, table)
}

// --- controlled driver: only the queries the real provider issues ---

var (
	t06a4HTTPOnce  sync.Once
	t06a4HTTPState sync.Map
)

type t06a4HTTPDriver struct{}
type t06a4HTTPConn struct{ cDefault driver.Value }
type t06a4HTTPRows struct {
	cols  []string
	rows  [][]driver.Value
	index int
}

func openT06A4HTTPDB(t *testing.T, cDefault driver.Value) *sql.DB {
	t.Helper()
	t06a4HTTPOnce.Do(func() { sql.Register("t06a4-http-mysqlmeta", t06a4HTTPDriver{}) })
	name := "t06a4-http-" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	t06a4HTTPState.Store(name, cDefault)
	db, err := sql.Open("t06a4-http-mysqlmeta", name)
	if err != nil {
		t.Fatalf("open driver db: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		t06a4HTTPState.Delete(name)
	})
	return db
}

func (t06a4HTTPDriver) Open(name string) (driver.Conn, error) {
	value, _ := t06a4HTTPState.Load(name)
	cDefault, _ := value.(driver.Value)
	return t06a4HTTPConn{cDefault: cDefault}, nil
}

func (t06a4HTTPConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (t06a4HTTPConn) Close() error                        { return nil }
func (t06a4HTTPConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }

func (c t06a4HTTPConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	switch {
	case strings.Contains(query, "show variables"):
		return &t06a4HTTPRows{cols: []string{"Variable_name", "Value"}, rows: [][]driver.Value{{"version", "8.4.10"}}}, nil
	case strings.Contains(query, "information_schema.columns"):
		return &t06a4HTTPRows{
			cols: []string{"column_name", "column_type", "character_set_name", "collation_name", "column_comment", "column_default", "is_nullable", "extra"},
			rows: [][]driver.Value{
				{"id", "int", nil, nil, "", nil, "NO", ""},
				{"a", "varchar(8)", nil, nil, "", nil, "YES", ""},
				{"b", "varchar(8)", nil, nil, "", nil, "YES", ""},
				{"c", "varchar(8)", nil, nil, "", c.cDefault, "YES", ""},
				{"d", "varchar(8)", nil, nil, "", "<nil>", "YES", ""},
			},
		}, nil
	case strings.Contains(query, "information_schema.statistics"):
		return &t06a4HTTPRows{
			cols: []string{"index_name", "non_unique", "index_type", "column_name", "cardinality"},
			rows: [][]driver.Value{{"PRIMARY", int64(0), "BTREE", "id", nil}},
		}, nil
	case strings.Contains(query, "information_schema.tables"):
		return &t06a4HTTPRows{
			cols: []string{"engine", "table_collation", "table_comment", "auto_increment", "row_format", "table_rows"},
			rows: [][]driver.Value{{"InnoDB", "utf8mb4_general_ci", "", nil, "Dynamic", int64(0)}},
		}, nil
	}
	return nil, sql.ErrNoRows
}

func (r *t06a4HTTPRows) Columns() []string { return r.cols }
func (r *t06a4HTTPRows) Close() error      { return nil }
func (r *t06a4HTTPRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

const t06a4HTTPSQL = "ALTER TABLE t06_a4_defaults DROP COLUMN d;\nCREATE INDEX idx_b ON t06_a4_defaults(b);"

func t06a4HTTPPolicy(t *testing.T) string {
	t.Helper()
	return t06A3HTTPPolicy(t, map[string]bool{
		"ddl.table.exists.alter.require":          true,
		"ddl.alter.drop_column.exists.require":    true,
		"ddl.create_index.columns.exists.require": true,
	})
}

// TestHandlerT06A4ProviderTextNullStaysConservative is the A representative
// at the HTTP seam: a stored 'NULL' literal keeps the drop inside the
// conservative boundary — 200, review/unverified.
func TestHandlerT06A4ProviderTextNullStaysConservative(t *testing.T) {
	db := openT06A4HTTPDB(t, "NULL")
	provider := &t06a4HTTPProvider{inner: mysqlmeta.NewProvider(db)}
	code, payload := postT06A3Audit(t, t06a4HTTPPolicy(t), "mysql", t06a4HTTPSQL, provider)
	if code != http.StatusOK {
		t.Fatalf("http code = %d payload = %v, want 200", code, payload)
	}
	if payload["verdict"] != "review" {
		t.Fatalf("verdict = %v, want review", payload["verdict"])
	}
	if len(provider.calls) != 1 || provider.calls[0] != "golden.t06_a4_defaults" {
		t.Fatalf("provider calls = %#v, want exactly one load", provider.calls)
	}
}

// TestHandlerT06A4ProviderNullStaysPrecise is the C representative: stored
// SQL NULL keeps the precise drop path — 200, pass.
func TestHandlerT06A4ProviderNullStaysPrecise(t *testing.T) {
	db := openT06A4HTTPDB(t, nil)
	provider := &t06a4HTTPProvider{inner: mysqlmeta.NewProvider(db)}
	code, payload := postT06A3Audit(t, t06a4HTTPPolicy(t), "mysql", t06a4HTTPSQL, provider)
	if code != http.StatusOK {
		t.Fatalf("http code = %d payload = %v, want 200", code, payload)
	}
	if payload["verdict"] != "pass" {
		t.Fatalf("verdict = %v, want pass", payload["verdict"])
	}
}
