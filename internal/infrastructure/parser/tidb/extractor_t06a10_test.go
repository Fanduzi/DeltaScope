// Package tidbparser verifies T06-A10 AUTO_INCREMENT declaration extraction.
// input: the eight frozen CREATE TABLE inputs plus a table-level single-PK
// control, parsed through the real TiDB parser and extractor under both dialects
// output: Column.AutoIncrement, ordered PrimaryKey members, and the
// Options["auto_increment"] declaration key pinned to the actual fields
// pos: infrastructure parser extractor regression for issue #85 T06-A10
// note: if this file changes, update this header and module README.md.
package tidbparser

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// extractT06A10 parses and extracts with the given dialect so both the mysql
// and tidb stamps exercise the real parser/extractor path.
func extractT06A10(t *testing.T, dialect spec.Dialect, sql string) spec.Statement {
	t.Helper()

	parser := New()
	result, err := parser.Parse(context.Background(), sql)
	if err != nil {
		t.Fatalf("parse sql: %v", err)
	}
	wrapped := WrapStatements(result.Statements, result.Warnings)
	if len(wrapped) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(wrapped))
	}
	statement, err := wrapped[0].Extractor.Extract(dialect, wrapped[0].RawSQL)
	if err != nil {
		t.Fatalf("extract statement: %v", err)
	}
	return statement
}

// t06A10Inputs are the frozen baseline inputs. Wanted fields: column-level
// AUTO_INCREMENT flag per declared column, the bound PK member list, and the
// declared table option key — present only when the statement writes it.
var t06A10Inputs = []struct {
	name        string
	sql         string
	columns     []string
	autoFlags   []bool
	pkColumns   []string
	optionKey   bool
	optionValue string
}{
	{name: "pk-auto",
		sql:     "CREATE TABLE t (id BIGINT AUTO_INCREMENT PRIMARY KEY);",
		columns: []string{"id"}, autoFlags: []bool{true},
		pkColumns: []string{"id"}},
	{name: "pk-no-auto",
		sql:     "CREATE TABLE t (id BIGINT PRIMARY KEY);",
		columns: []string{"id"}, autoFlags: []bool{false},
		pkColumns: []string{"id"}},
	{name: "composite",
		sql:     "CREATE TABLE t (id BIGINT NOT NULL, tenant_id INT NOT NULL, PRIMARY KEY (id, tenant_id));",
		columns: []string{"id", "tenant_id"}, autoFlags: []bool{false, false},
		pkColumns: []string{"id", "tenant_id"}},
	{name: "pk-off",
		sql:     "CREATE TABLE t (id BIGINT PRIMARY KEY);",
		columns: []string{"id"}, autoFlags: []bool{false},
		pkColumns: []string{"id"}},
	{name: "init-match",
		sql:     "CREATE TABLE t (id BIGINT AUTO_INCREMENT PRIMARY KEY) AUTO_INCREMENT=8;",
		columns: []string{"id"}, autoFlags: []bool{true},
		pkColumns: []string{"id"},
		optionKey: true, optionValue: "8"},
	{name: "init-mismatch",
		sql:     "CREATE TABLE t (id BIGINT AUTO_INCREMENT PRIMARY KEY) AUTO_INCREMENT=9;",
		columns: []string{"id"}, autoFlags: []bool{true},
		pkColumns: []string{"id"},
		optionKey: true, optionValue: "9"},
	{name: "init-omitted",
		sql:     "CREATE TABLE t (id BIGINT AUTO_INCREMENT PRIMARY KEY);",
		columns: []string{"id"}, autoFlags: []bool{true},
		pkColumns: []string{"id"}},
	{name: "init-off",
		sql:     "CREATE TABLE t (id BIGINT AUTO_INCREMENT PRIMARY KEY) AUTO_INCREMENT=9;",
		columns: []string{"id"}, autoFlags: []bool{true},
		pkColumns: []string{"id"},
		optionKey: true, optionValue: "9"},
	// Table-level single-member PK control: the member binds the same id
	// column as the inline form, keeping AutoIncrement on the column.
	{name: "table-level-pk-auto",
		sql:     "CREATE TABLE t (id BIGINT AUTO_INCREMENT NOT NULL, PRIMARY KEY (id));",
		columns: []string{"id"}, autoFlags: []bool{true},
		pkColumns: []string{"id"}},
}

func TestExtractorT06A10AutoIncrementFacts(t *testing.T) {
	t.Parallel()
	for _, tc := range t06A10Inputs {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
				stmt := extractT06A10(t, dialect, tc.sql)
				if stmt.Dialect != dialect {
					t.Fatalf("dialect stamp = %q, want %q", stmt.Dialect, dialect)
				}
				if stmt.DDL == nil || stmt.DDL.Table == nil || stmt.DDL.Table.Name != "t" {
					t.Fatalf("[%s] not a create-table on t: %#v", dialect, stmt.DDL)
				}
				if len(stmt.DDL.Columns) != len(tc.columns) {
					t.Fatalf("[%s] columns = %#v, want %v", dialect, stmt.DDL.Columns, tc.columns)
				}
				for i, name := range tc.columns {
					column := stmt.DDL.Columns[i]
					if column.Name != name || column.AutoIncrement != tc.autoFlags[i] {
						t.Fatalf("[%s] column %d = %#v, want name=%s auto_increment=%v",
							dialect, i, column, name, tc.autoFlags[i])
					}
				}
				if stmt.DDL.PrimaryKey == nil {
					t.Fatalf("[%s] missing primary key in %#v", dialect, stmt.DDL)
				}
				if len(stmt.DDL.PrimaryKey.Columns) != len(tc.pkColumns) {
					t.Fatalf("[%s] pk members = %#v, want %v", dialect, stmt.DDL.PrimaryKey.Columns, tc.pkColumns)
				}
				for i, name := range tc.pkColumns {
					if stmt.DDL.PrimaryKey.Columns[i] != name {
						t.Fatalf("[%s] pk member %d = %q, want %q", dialect, i, stmt.DDL.PrimaryKey.Columns[i], name)
					}
				}
				got, present := stmt.DDL.Options["auto_increment"]
				if present != tc.optionKey {
					t.Fatalf("[%s] options = %#v, auto_increment present=%v want %v",
						dialect, stmt.DDL.Options, present, tc.optionKey)
				}
				if tc.optionKey && got != tc.optionValue {
					t.Fatalf("[%s] options[auto_increment] = %q, want %q", dialect, got, tc.optionValue)
				}
				// The column attribute and the table option are distinct facts:
				// the declared table value must never appear as a column flag,
				// and an omitted option must never materialize a key.
			}
		})
	}
}
