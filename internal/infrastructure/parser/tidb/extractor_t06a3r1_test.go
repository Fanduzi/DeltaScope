// Package tidbparser verifies the T06-A3-R1 per-option DEFAULT contract: every
// DEFAULT clause re-derives the typed facts from its own expression, so a
// later spelling can never inherit an earlier NULL flag.
// input: MySQL and TiDB CREATE/ALTER inputs with repeated DEFAULT options through the real parser and extractor
// output: HasDefault/DefaultValue/DefaultIsNull bound to the last written DEFAULT expression on every path
// pos: infrastructure parser extractor test coverage for issue #85 T06-A3-R1
// note: if this file changes, update this header and module README.md.
package tidbparser

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/pingcap/tidb/pkg/parser/ast"
)

// TestT06A3R1DuplicateDefaultFieldMatrix pins the frozen per-option contract:
// the final DEFAULT expression alone determines DefaultValue and
// DefaultIsNull — never an accumulated "a NULL appeared once" flag.
func TestT06A3R1DuplicateDefaultFieldMatrix(t *testing.T) {
	cases := []struct {
		name       string
		defaults   string
		hasDefault bool
		value      string
		isNull     bool
	}{
		// Group A: final spelling is non-NULL — the stale flag must clear.
		{"null_then_text_null", "DEFAULT NULL DEFAULT 'NULL'", true, "'NULL'", false},
		{"null_then_text_nil", "DEFAULT NULL DEFAULT '<nil>'", true, "'<nil>'", false},
		{"null_then_zero", "DEFAULT NULL DEFAULT 0", true, "0", false},
		{"null_then_empty", "DEFAULT NULL DEFAULT ''", true, "''", false},
		// Group B: final spelling is SQL NULL — the flag must be set.
		{"text_null_then_null", "DEFAULT 'NULL' DEFAULT NULL", true, "NULL", true},
		{"null_then_null", "DEFAULT NULL DEFAULT NULL", true, "NULL", true},
		{"triple_ending_null", "DEFAULT NULL DEFAULT 'NULL' DEFAULT NULL", true, "NULL", true},
		// Group C: single-option controls stay unchanged.
		{"single_null", "DEFAULT NULL", true, "NULL", true},
		{"single_text_null", "DEFAULT 'NULL'", true, "'NULL'", false},
		{"single_text_nil", "DEFAULT '<nil>'", true, "'<nil>'", false},
		{"no_default", "", false, "", false},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, tc := range cases {
			t.Run(string(dialect)+"/"+tc.name, func(t *testing.T) {
				sql := "CREATE TABLE t (c VARCHAR(8) " + tc.defaults + ");"
				column := t06a3Column(t, t06a2Extract(t, dialect, sql), "c")
				t06a3CheckDefault(t, column, tc.hasDefault, tc.value, tc.isNull)
			})
		}
	}
}

// TestT06A3R1DuplicateDefaultIsolation pins the non-crossing controls: a NULL
// flag on one column never leaks into a sibling, and non-DEFAULT options do
// not rewrite the already-recorded default facts.
func TestT06A3R1DuplicateDefaultIsolation(t *testing.T) {
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			statement := t06a2Extract(t, dialect,
				"CREATE TABLE t (a VARCHAR(8) DEFAULT NULL DEFAULT 'NULL', "+
					"b VARCHAR(8) DEFAULT NULL, c INT DEFAULT 7 COMMENT 'x' DEFAULT NULL);")
			t06a3CheckDefault(t, t06a3Column(t, statement, "a"), true, "'NULL'", false)
			t06a3CheckDefault(t, t06a3Column(t, statement, "b"), true, "NULL", true)
			c := t06a3Column(t, statement, "c")
			t06a3CheckDefault(t, c, true, "NULL", true)
			if c.Comment != "'x'" {
				t.Fatalf("c.Comment = %q, want COMMENT preserved across a later DEFAULT", c.Comment)
			}
			// A non-DEFAULT option after DEFAULT must not clear the recorded
			// default (NOT NULL written after DEFAULT NULL).
			later := t06a2Extract(t, dialect,
				"CREATE TABLE t (d VARCHAR(8) DEFAULT NULL NOT NULL);")
			d := t06a3Column(t, later, "d")
			t06a3CheckDefault(t, d, true, "NULL", true)
			if !d.NotNull {
				t.Fatalf("d = %+v, want the written NOT NULL kept", d)
			}
		})
	}
}

// TestT06A3R1AlterDefinitionDuplicateDefault verifies the shared ALTER
// definition path re-derives the flag per option too.
func TestT06A3R1AlterDefinitionDuplicateDefault(t *testing.T) {
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, sql := range []string{
			"ALTER TABLE t MODIFY COLUMN c VARCHAR(8) DEFAULT NULL DEFAULT 'NULL';",
			"ALTER TABLE t CHANGE COLUMN c c VARCHAR(8) DEFAULT NULL DEFAULT 'NULL';",
			"ALTER TABLE t ADD COLUMN c VARCHAR(8) DEFAULT NULL DEFAULT 'NULL';",
		} {
			t.Run(string(dialect)+"/"+sql[:25], func(t *testing.T) {
				statement := t06a2Extract(t, dialect, sql)
				definition := statement.DDL.Alter[0].Column.Definition
				if definition == nil {
					t.Fatalf("definition = %+v, want a target column", statement.DDL.Alter[0].Column)
				}
				t06a3CheckDefault(t, *definition, true, "'NULL'", false)
			})
		}
	}
}

// TestT06A3R1HelperBoundariesPreserved re-pins the private recognizer edges:
// these AST shapes cannot come from real SQL in this contract, so they are
// labeled helper controls built with the parser's constructors.
func TestT06A3R1HelperBoundariesPreserved(t *testing.T) {
	if exprIsNullLiteral(nil) {
		t.Fatal("nil ExprNode must not be SQL NULL")
	}
	if exprIsNullLiteral(ast.NewParamMarkerExpr(0)) {
		t.Fatal("param marker '?' must not be SQL NULL")
	}
	fn := &ast.FuncCallExpr{FnName: ast.NewCIStr("now")}
	if exprIsNullLiteral(fn) {
		t.Fatal("function call must not be SQL NULL")
	}
}
