// Package tidb verifies the T06-A13 extracted facts the five type-family
// forbid policies consume.
package tidbparser

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const t06A13MixedSQL = "CREATE TABLE t (c_blob BLOB, c_text TEXT, c_json JSON, c_bit BIT(1), c_float FLOAT, c_double DOUBLE, c_timestamp TIMESTAMP NULL, c_datetime DATETIME NULL, c_int INT);"
const t06A13SafeSQL = "CREATE TABLE t (c_int INT, c_string VARCHAR(8), c_datetime DATETIME NULL);"

// t06A13Column pins the extracted-fact want for one declared column. Type is
// the parser-rendered text verbatim (BLOB renders `blob binary`, BIT keeps
// its declared width `bit(1)`, a bare timestamp/datetime carries flen 19 as
// the parser's display width — none of these is a user-declared fsp or a
// server COLUMN_TYPE oracle). charset/collation are the spec fields, which
// stay empty when the AST-level binary charset does not surface.
type t06A13Column struct {
	name      string
	typeText  string
	length    int
	charset   string
	collation string
	notNull   bool
}

var t06A13Inputs = []struct {
	name    string
	sql     string
	columns []t06A13Column
}{
	{name: "mixed", sql: t06A13MixedSQL, columns: []t06A13Column{
		{name: "c_blob", typeText: "blob binary", length: -1},
		{name: "c_text", typeText: "text", length: -1},
		{name: "c_json", typeText: "json", length: -1},
		{name: "c_bit", typeText: "bit(1)", length: 1},
		{name: "c_float", typeText: "float", length: -1},
		{name: "c_double", typeText: "double", length: -1},
		{name: "c_timestamp", typeText: "timestamp", length: 19},
		{name: "c_datetime", typeText: "datetime", length: 19},
		{name: "c_int", typeText: "int(11)", length: -1},
	}},
	{name: "safe", sql: t06A13SafeSQL, columns: []t06A13Column{
		{name: "c_int", typeText: "int(11)", length: -1},
		{name: "c_string", typeText: "varchar(8)", length: 8},
		{name: "c_datetime", typeText: "datetime", length: 19},
	}},
}

// TestExtractorT06A13ColumnFacts pins the extracted per-column facts the
// five forbid policies match on: declaration order, name, rendered Type,
// Length, Charset/Collation, NotNull — plus the absence of primary key,
// indexes, and any default/on-update semantics the input never declared.
func TestExtractorT06A13ColumnFacts(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, tc := range t06A13Inputs {
			tc := tc
			t.Run(string(dialect)+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				_, statement := parseExtractT06A12(t, dialect, tc.sql)
				if statement.Kind != spec.KindDDL || statement.DDL == nil {
					t.Fatalf("statement kind = %v, want ddl", statement.Kind)
				}
				if len(statement.DDL.Columns) != len(tc.columns) {
					t.Fatalf("columns = %#v, want %d", statement.DDL.Columns, len(tc.columns))
				}
				for i, want := range tc.columns {
					column := statement.DDL.Columns[i]
					if column.Name != want.name || column.Type != want.typeText ||
						column.Length != want.length || column.NotNull != want.notNull ||
						column.Charset != want.charset || column.Collation != want.collation {
						t.Fatalf("column[%d] = %#v, want %s %s len=%d charset=%q collation=%q notNull=%v",
							i, column, want.name, want.typeText, want.length,
							want.charset, want.collation, want.notNull)
					}
					if column.HasDefault || column.DefaultIsCurrentTimestamp ||
						column.OnUpdateCurrentTimestamp {
						t.Fatalf("column[%d] = %#v carries undeclared default/update semantics", i, column)
					}
				}
				if statement.DDL.PrimaryKey != nil || len(statement.DDL.Indexes) != 0 ||
					len(statement.DDL.Constraints) != 0 {
					t.Fatalf("pk/indexes/constraints = %#v/%#v/%#v, want none",
						statement.DDL.PrimaryKey, statement.DDL.Indexes, statement.DDL.Constraints)
				}
			})
		}
	}
}

// TestExtractorT06A13BinaryCharsetBoundary pins the representational
// boundary the forbid findings inherit: the AST carries a built-in binary
// charset for BLOB and JSON, but tidbtypes.HasCharset does not surface it,
// so the spec's explicit Charset stays "" — while the type text itself
// keeps the parser's `blob binary`/`json` rendering. TEXT, by contrast, is
// charset-capable and still reports "" because no charset was declared.
func TestExtractorT06A13BinaryCharsetBoundary(t *testing.T) {
	t.Parallel()
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			create, statement := parseExtractT06A12(t, dialect, t06A13MixedSQL)
			astByName := map[string]int{}
			for i, col := range create.Cols {
				astByName[col.Name.Name.L] = i
			}
			cases := []struct {
				column      string
				wantASTCS   string // FieldType-reported charset, observed only
				wantSpecCS  string
				wantTypeStr string
			}{
				{"c_blob", "binary", "", "blob binary"},
				{"c_json", "binary", "", "json"},
				{"c_text", "", "", "text"},
				{"c_bit", "", "", "bit(1)"},
			}
			for _, tc := range cases {
				i, ok := astByName[tc.column]
				if !ok {
					t.Fatalf("column %s missing from ast", tc.column)
				}
				if got := create.Cols[i].Tp.GetCharset(); got != tc.wantASTCS {
					t.Fatalf("%s ast charset = %q, want %q", tc.column, got, tc.wantASTCS)
				}
				specCol := statement.DDL.Columns[i]
				if specCol.Name != tc.column || specCol.Charset != tc.wantSpecCS {
					t.Fatalf("spec column[%d] = %#v, want name=%s charset=%q",
						i, specCol, tc.column, tc.wantSpecCS)
				}
				if specCol.Type != tc.wantTypeStr {
					t.Fatalf("spec column[%d] type = %q, want %q", i, specCol.Type, tc.wantTypeStr)
				}
			}
		})
	}
}
