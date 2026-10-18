// Package auditmeta verifies the T06-A17 provider-source carry contract: a
// real mysqlmeta.Provider reading constructed catalog rows feeds AuditSQL's
// ordered pre-state, and the faithful COLUMN_TYPE text (DECIMAL scale,
// ENUM/SET member case) reaches the compatibility finding's source_type echo
// — never a truncated base keyword. This is "real provider + constructed DB
// rows", not a live anchor.
// input: ALTER MODIFY batches against driver-fed catalog rows for t06_a17_type_facts
// output: compatibility findings carrying verbatim provider Type in source_type
// pos: application-layer real-provider contract tests for issue #85 T06-A17
// note: if this file changes, update this header and module README.md.
package auditmeta

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"strings"
	"sync"
	"testing"

	appaudit "github.com/Fanduzi/DeltaScope/internal/application/audit"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	mysqlmeta "github.com/Fanduzi/DeltaScope/internal/infrastructure/metadata/mysql"
)

const (
	t06a17CompatRule = "ddl.alter.modify_column.compatibility.require"
	t06a17Table      = "golden.t06_a17_type_facts"
)

var (
	t06a17Once  sync.Once
	t06a17State sync.Map
)

type t06a17Driver struct{}
type t06a17Conn struct{ rows [][]driver.Value }
type t06a17Rows struct {
	cols  []string
	rows  [][]driver.Value
	index int
}

// t06a17Catalog is the constructed catalog fixture: five target-type columns
// whose parameters/members must survive the provider verbatim.
var t06a17Catalog = [][]driver.Value{
	{"d", "decimal(12,2)", nil, nil, "", nil, "YES", ""},
	{"e", "enum('Ready','Closed')", "utf8mb4", "utf8mb4_bin", "", nil, "YES", ""},
	{"s", "set('Write','Read')", "utf8mb4", "utf8mb4_bin", "", nil, "YES", ""},
}

func openT06A17DB(t *testing.T) *sql.DB {
	t.Helper()
	t06a17Once.Do(func() { sql.Register("t06a17-mysqlmeta", t06a17Driver{}) })
	name := "t06a17-" + strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	t06a17State.Store(name, t06a17Catalog)
	db, err := sql.Open("t06a17-mysqlmeta", name)
	if err != nil {
		t.Fatalf("open driver db: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		t06a17State.Delete(name)
	})
	return db
}

func (t06a17Driver) Open(name string) (driver.Conn, error) {
	value, _ := t06a17State.Load(name)
	rows, _ := value.([][]driver.Value)
	return t06a17Conn{rows: rows}, nil
}

func (t06a17Conn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (t06a17Conn) Close() error                        { return nil }
func (t06a17Conn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }

func (c t06a17Conn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	switch {
	case strings.Contains(query, "show variables"):
		return &t06a17Rows{
			cols: []string{"Variable_name", "Value"},
			rows: [][]driver.Value{{"version", "8.4.10"}},
		}, nil
	case strings.Contains(query, "information_schema.columns"):
		return &t06a17Rows{
			cols: []string{"column_name", "column_type", "character_set_name", "collation_name", "column_comment", "column_default", "is_nullable", "extra"},
			rows: c.rows,
		}, nil
	case strings.Contains(query, "information_schema.statistics"):
		return &t06a17Rows{
			cols: []string{"index_name", "non_unique", "index_type", "column_name", "cardinality"},
		}, nil
	case strings.Contains(query, "information_schema.tables"):
		return &t06a17Rows{
			cols: []string{"engine", "table_collation", "table_comment", "auto_increment", "row_format", "table_rows"},
			rows: [][]driver.Value{{"InnoDB", "utf8mb4_general_ci", "", nil, "Dynamic", int64(0)}},
		}, nil
	}
	return nil, sql.ErrNoRows
}

func (r *t06a17Rows) Columns() []string { return r.cols }
func (r *t06a17Rows) Close() error      { return nil }
func (r *t06a17Rows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

// TestT06A17ProviderTypeReachesPreState pins the observable echo: MODIFY
// across a type-family boundary reports the provider's verbatim source_type.
// One audit per column because a cross-family MODIFY publishes a conservative
// tombstone for the following statement's pre-state by design.
func TestT06A17ProviderTypeReachesPreState(t *testing.T) {
	db := openT06A17DB(t)
	provider := mysqlmeta.NewProvider(db)
	policy := t06a4Policy(t, map[string]string{t06a17CompatRule: ""})
	for _, tc := range []struct {
		column, wantSource string
	}{
		{"d", "decimal(12,2)"},
		{"e", "enum('Ready','Closed')"},
		{"s", "set('Write','Read')"},
	} {
		t.Run(tc.column, func(t *testing.T) {
			result, err := appaudit.AuditSQL(context.Background(), appaudit.Request{
				SQL:     "ALTER TABLE t06_a17_type_facts MODIFY " + tc.column + " VARCHAR(9);",
				Dialect: spec.DialectMySQL, Schema: "golden",
				ConfigPath: policy, MetadataProvider: provider,
			})
			if err != nil {
				t.Fatalf("AuditSQL: %v", err)
			}
			if len(result.Statements) != 1 {
				t.Fatalf("statements = %d, want 1", len(result.Statements))
			}
			var found bool
			for _, finding := range result.Statements[0].Findings {
				if finding.RuleID != t06a17CompatRule {
					continue
				}
				found = true
				if finding.Metadata["source_type"] != tc.wantSource {
					t.Fatalf("source_type = %v, want %q", finding.Metadata["source_type"], tc.wantSource)
				}
			}
			if !found {
				t.Fatalf("no compatibility finding: %+v", result.Statements[0])
			}
		})
	}
}
