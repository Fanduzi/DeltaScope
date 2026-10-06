// Package tidbparser verifies the T06-A3 typed SQL NULL literal recognition for
// column DEFAULT extraction and the preserved literal/alter fact contract.
// input: MySQL and TiDB CREATE/ALTER inputs through the real parser and extractor
// output: typed HasDefault/DefaultValue/DefaultIsNull facts and unchanged literal/comment/alter fields
// pos: infrastructure parser extractor test coverage for issue #85 T06-A3
// note: if this file changes, update this header and module README.md.
package tidbparser

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/pingcap/tidb/pkg/parser/ast"
)

// t06a3Column returns the named column from an extracted CREATE TABLE.
func t06a3Column(t *testing.T, statement spec.Statement, name string) spec.Column {
	t.Helper()
	for _, column := range statement.DDL.Columns {
		if column.Name == name {
			return column
		}
	}
	t.Fatalf("column %q not found in %+v", name, statement.DDL.Columns)
	return spec.Column{}
}

// t06a3CheckDefault pins the typed default triple (HasDefault, DefaultValue,
// DefaultIsNull) — DefaultIsNull binds the SQL NULL literal, never the text.
func t06a3CheckDefault(t *testing.T, column spec.Column, hasDefault bool, value string, isNull bool) {
	t.Helper()
	if column.HasDefault != hasDefault || column.DefaultValue != value || column.DefaultIsNull != isNull {
		t.Fatalf("column %q = {HasDefault:%t DefaultValue:%q DefaultIsNull:%t}, want {%t %q %t}",
			column.Name, column.HasDefault, column.DefaultValue, column.DefaultIsNull, hasDefault, value, isNull)
	}
}

// TestT06A3DefaultNullFieldMatrix pins the frozen four-input contract: an
// absent clause, a real SQL NULL literal, and the look-alike string literals
// 'NULL' / '<nil>' must stay distinct on both dialects.
func TestT06A3DefaultNullFieldMatrix(t *testing.T) {
	cases := []struct {
		name       string
		sql        string
		hasDefault bool
		value      string
		isNull     bool
	}{
		{"no_clause", "CREATE TABLE t (c VARCHAR(8));", false, "", false},
		{"sql_null", "CREATE TABLE t (c VARCHAR(8) DEFAULT NULL);", true, "NULL", true},
		{"text_null", "CREATE TABLE t (c VARCHAR(8) DEFAULT 'NULL');", true, "'NULL'", false},
		{"text_nil", "CREATE TABLE t (c VARCHAR(8) DEFAULT '<nil>');", true, "'<nil>'", false},
		{"text_null_lower", "CREATE TABLE t (c VARCHAR(8) DEFAULT 'null');", true, "'null'", false},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, tc := range cases {
			t.Run(string(dialect)+"/"+tc.name, func(t *testing.T) {
				column := t06a3Column(t, t06a2Extract(t, dialect, tc.sql), "c")
				t06a3CheckDefault(t, column, tc.hasDefault, tc.value, tc.isNull)
			})
		}
	}
}

// TestT06A3NullLiteralHelperBoundaries pins the private recognizer: nil nodes,
// param markers (whose embedded datum also reports a nil value), and
// non-literal expressions must never be read as SQL NULL.
func TestT06A3NullLiteralHelperBoundaries(t *testing.T) {
	if exprIsNullLiteral(nil) {
		t.Fatal("nil ExprNode must not be SQL NULL")
	}
	marker := ast.NewParamMarkerExpr(0)
	if marker == nil {
		t.Fatal("test_driver param marker constructor unavailable")
	}
	if exprIsNullLiteral(marker) {
		t.Fatal("param marker '?' must not be SQL NULL — its embedded datum has a nil value too")
	}
	// A function call is a non-literal expression: whatever it may evaluate to
	// at runtime, the parsed default is not a NULL literal.
	fn := &ast.FuncCallExpr{FnName: ast.NewCIStr("current_timestamp")}
	if exprIsNullLiteral(fn) {
		t.Fatal("function call must not be SQL NULL")
	}
	// Quoted literals stay non-NULL through the same extraction site the
	// DEFAULT clause uses.
	statement := t06a2Extract(t, spec.DialectMySQL, "CREATE TABLE t (c VARCHAR(8) DEFAULT 'NULL');")
	t06a3CheckDefault(t, t06a3Column(t, statement, "c"), true, "'NULL'", false)
}

// TestT06A3DefaultLiteralPreservation pins the unchanged contract for
// literal, timestamp, and comment facts around the NULL fix.
func TestT06A3DefaultLiteralPreservation(t *testing.T) {
	statement := t06a2Extract(t, spec.DialectMySQL,
		"CREATE TABLE t (a INT DEFAULT 0, b VARCHAR(4) DEFAULT '', c TIMESTAMP DEFAULT CURRENT_TIMESTAMP, d INT COMMENT 'note');")
	columns := statement.DDL.Columns
	t06a3CheckDefault(t, columns[0], true, "0", false)
	t06a3CheckDefault(t, columns[1], true, "''", false)
	if !columns[2].HasDefault || !columns[2].DefaultIsCurrentTimestamp || columns[2].DefaultIsNull {
		t.Fatalf("timestamp column = %+v, want CURRENT_TIMESTAMP kept non-null default", columns[2])
	}
	if columns[3].Comment != "'note'" || columns[3].HasDefault {
		t.Fatalf("comment column = %+v, want COMMENT preserved without a default", columns[3])
	}
}

// TestT06A3AlterDefinitionDefaultNull verifies ALTER column definitions share
// the corrected default facts while nullability/rename/PK markers stay intact.
func TestT06A3AlterDefinitionDefaultNull(t *testing.T) {
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			statement := t06a2Extract(t, dialect, "ALTER TABLE t MODIFY COLUMN c VARCHAR(8) NULL DEFAULT NULL;")
			if statement.DDL == nil || len(statement.DDL.Alter) != 1 {
				t.Fatalf("alters = %+v, want 1", statement.DDL)
			}
			definition := statement.DDL.Alter[0].Column.Definition
			if definition == nil {
				t.Fatalf("alter definition = %+v, want a target column", statement.DDL.Alter[0].Column)
			}
			t06a3CheckDefault(t, *definition, true, "NULL", true)
			if definition.NotNull {
				t.Fatalf("definition = %+v, want the written NULL nullability untouched by the default fix", definition)
			}
			add := t06a2Extract(t, dialect, "ALTER TABLE t ADD COLUMN c VARCHAR(8) DEFAULT 'NULL';")
			addDef := add.DDL.Alter[0].Column.Definition
			if addDef == nil {
				t.Fatalf("add definition = %+v", add.DDL.Alter[0].Column)
			}
			t06a3CheckDefault(t, *addDef, true, "'NULL'", false)
		})
	}
}
