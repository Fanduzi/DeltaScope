// Package deltascope verifies the T06-A4 provider default-identity contract at
// the public SDK seam with the real mysqlmeta.Provider behind a controlled
// database/sql driver — one conservative A representative and one precise C
// representative; the full matrix lives in the shared layer and Golden runs.
// input: provider-fed ALTER DROP COLUMN + CREATE INDEX audits through Audit
// output: review/unverified for the stored 'NULL' literal; pass/complete for stored SQL NULL
// pos: public SDK contract test for issue #85 T06-A4
// note: if this file changes, update this header and module README.md.
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

// t06a4SDKProvider adapts the real mysql provider to the public
// MetadataProvider interface; it never rewrites produced fields.
type t06a4SDKProvider struct {
	inner *mysqlmeta.Provider
	calls []string
}

func (p *t06a4SDKProvider) LoadInstanceFacts(ctx context.Context, _ Dialect, schema string) (*InstanceFacts, error) {
	return p.inner.LoadInstanceFacts(ctx, spec.DialectMySQL, schema)
}

func (p *t06a4SDKProvider) LoadTableSnapshot(ctx context.Context, _ Dialect, schema, table string) (*TableSnapshot, error) {
	p.calls = append(p.calls, schema+"."+table)
	return p.inner.LoadTableSnapshot(ctx, spec.DialectMySQL, schema, table)
}

// --- controlled driver: only the queries the real provider issues ---

var (
	t06a4SDKOnce  sync.Once
	t06a4SDKState sync.Map
)

type t06a4SDKDriver struct{}
type t06a4SDKConn struct{ cDefault driver.Value }
type t06a4SDKRows struct {
	cols  []string
	rows  [][]driver.Value
	index int
}

func openT06A4SDKDB(t *testing.T, cDefault driver.Value) *sql.DB {
	t.Helper()
	t06a4SDKOnce.Do(func() { sql.Register("t06a4-sdk-mysqlmeta", t06a4SDKDriver{}) })
	name := "t06a4-sdk-" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	t06a4SDKState.Store(name, cDefault)
	db, err := sql.Open("t06a4-sdk-mysqlmeta", name)
	if err != nil {
		t.Fatalf("open driver db: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		t06a4SDKState.Delete(name)
	})
	return db
}

func (t06a4SDKDriver) Open(name string) (driver.Conn, error) {
	value, _ := t06a4SDKState.Load(name)
	cDefault, _ := value.(driver.Value)
	return t06a4SDKConn{cDefault: cDefault}, nil
}

func (t06a4SDKConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (t06a4SDKConn) Close() error                        { return nil }
func (t06a4SDKConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }

func (c t06a4SDKConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	switch {
	case strings.Contains(query, "show variables"):
		return &t06a4SDKRows{cols: []string{"Variable_name", "Value"}, rows: [][]driver.Value{{"version", "8.4.10"}}}, nil
	case strings.Contains(query, "information_schema.columns"):
		return &t06a4SDKRows{
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
		return &t06a4SDKRows{
			cols: []string{"index_name", "non_unique", "index_type", "column_name", "cardinality"},
			rows: [][]driver.Value{{"PRIMARY", int64(0), "BTREE", "id", nil}},
		}, nil
	case strings.Contains(query, "information_schema.tables"):
		return &t06a4SDKRows{
			cols: []string{"engine", "table_collation", "table_comment", "auto_increment", "row_format", "table_rows"},
			rows: [][]driver.Value{{"InnoDB", "utf8mb4_general_ci", "", nil, "Dynamic", int64(0)}},
		}, nil
	}
	return nil, sql.ErrNoRows
}

func (r *t06a4SDKRows) Columns() []string { return r.cols }
func (r *t06a4SDKRows) Close() error      { return nil }
func (r *t06a4SDKRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

const t06a4SDKSQL = "ALTER TABLE t06_a4_defaults DROP COLUMN d;\nCREATE INDEX idx_b ON t06_a4_defaults(b);"

func t06a4SDKPolicy(t *testing.T) string {
	t.Helper()
	return t06A3SDKPolicy(t, map[string]bool{
		"ddl.table.exists.alter.require":          true,
		"ddl.alter.drop_column.exists.require":    true,
		"ddl.create_index.columns.exists.require": true,
	})
}

// TestT06A4ProviderTextNullStaysConservative is the A representative at the
// SDK seam: the stored 'NULL' literal on c keeps the drop inside the A6
// conservative boundary — review/unverified with one index gap.
func TestT06A4ProviderTextNullStaysConservative(t *testing.T) {
	db := openT06A4SDKDB(t, "NULL")
	provider := &t06a4SDKProvider{inner: mysqlmeta.NewProvider(db)}
	result, err := Audit(context.Background(), Request{
		SQL: t06a4SDKSQL, Dialect: DialectMySQL, Schema: "golden",
		ConfigPath: t06a4SDKPolicy(t), MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if result.Verdict != VerdictReview || result.Coverage.Status != "unverified" {
		t.Fatalf("aggregate = %s/%s, want review/unverified", result.Verdict, result.Coverage.Status)
	}
	if len(provider.calls) != 1 || provider.calls[0] != "golden.t06_a4_defaults" {
		t.Fatalf("provider calls = %#v, want exactly one load", provider.calls)
	}
}

// TestT06A4ProviderNullStaysPrecise is the C representative: the stored SQL
// NULL default keeps the precise drop path — pass/complete, err nil.
func TestT06A4ProviderNullStaysPrecise(t *testing.T) {
	db := openT06A4SDKDB(t, nil)
	provider := &t06a4SDKProvider{inner: mysqlmeta.NewProvider(db)}
	result, err := Audit(context.Background(), Request{
		SQL: t06a4SDKSQL, Dialect: DialectMySQL, Schema: "golden",
		ConfigPath: t06a4SDKPolicy(t), MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if result.Verdict != VerdictPass || result.Coverage.Status != "complete" {
		t.Fatalf("aggregate = %s/%s, want pass/complete", result.Verdict, result.Coverage.Status)
	}
	if len(provider.calls) != 1 || provider.calls[0] != "golden.t06_a4_defaults" {
		t.Fatalf("provider calls = %#v, want exactly one load", provider.calls)
	}
}
