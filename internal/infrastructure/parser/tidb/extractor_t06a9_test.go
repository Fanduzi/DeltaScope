// Package tidbparser verifies the T06-A9 explicit audit-time-column fact
// contract: the parser normalizes every accepted CURRENT_TIMESTAMP spelling
// (NOW(), LOCALTIME, LOCALTIMESTAMP, parenthesized and fractional-second
// forms) to one FuncCallExpr name, and the extractor lands the typed
// DefaultIsCurrentTimestamp/OnUpdateCurrentTimestamp flags the role rule
// consumes — never depending on the expression text, which is empty for
// function defaults.
// input: MySQL and TiDB CREATE TABLE inputs with current-timestamp spellings and counter shapes
// output: Column.HasDefault/DefaultIsCurrentTimestamp/OnUpdateCurrentTimestamp facts per declared option
// pos: parser-layer contract tests for issue #85 T06-A9
// note: if this file changes, update this header and module README.md.
package tidbparser

import (
	"testing"

	tidbast "github.com/pingcap/tidb/pkg/parser/ast"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// TestT06A9TimestampSpellingNormalization pins the parser-side synonym map:
// every accepted current-timestamp spelling reaches the extractor as a
// FuncCallExpr named "current_timestamp", so the typed flags do not depend on
// string comparison. This is the fact behind the baseline E role passing.
func TestT06A9TimestampSpellingNormalization(t *testing.T) {
	cases := []struct {
		name    string
		clause  string
		wantDCT bool
		wantOCT bool
	}{
		{"current_timestamp", "DEFAULT CURRENT_TIMESTAMP", true, false},
		{"current_timestamp_parens", "DEFAULT CURRENT_TIMESTAMP()", true, false},
		{"now", "DEFAULT NOW()", true, false},
		{"localtime", "DEFAULT LOCALTIME", true, false},
		{"localtimestamp", "DEFAULT LOCALTIMESTAMP", true, false},
		{"on_update_now", "DEFAULT NOW() ON UPDATE NOW()", true, true},
		{"literal_default", "DEFAULT '2020-01-01 00:00:00'", false, false},
		{"numeric_default", "DEFAULT 0", false, false},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, tc := range cases {
			t.Run(string(dialect)+"/"+tc.name, func(t *testing.T) {
				statement := t06a2Extract(t, dialect,
					"CREATE TABLE t (c DATETIME NOT NULL "+tc.clause+");")
				if len(statement.DDL.Columns) != 1 {
					t.Fatalf("columns = %+v, want 1", statement.DDL.Columns)
				}
				c := statement.DDL.Columns[0]
				if !c.HasDefault {
					t.Fatalf("HasDefault = false, want declared default present")
				}
				if c.DefaultIsCurrentTimestamp != tc.wantDCT ||
					c.OnUpdateCurrentTimestamp != tc.wantOCT {
					t.Fatalf("flags = DCT:%v OCT:%v, want %v/%v",
						c.DefaultIsCurrentTimestamp, c.OnUpdateCurrentTimestamp,
						tc.wantDCT, tc.wantOCT)
				}
			})
		}
	}
}

// TestT06A9TimestampASTNodeKind pins the AST node shape directly — the
// normalization is a parser fact (FuncCallExpr with fn="current_timestamp"),
// not an extractor convention. FuncCallExpr.Text() stays empty in this parse
// path, which is why DefaultValue carries "" for function defaults and the
// rule must consume the typed flags instead.
func TestT06A9TimestampASTNodeKind(t *testing.T) {
	parser := New()
	res, err := parser.Parse(t.Context(),
		"CREATE TABLE t (a DATETIME NOT NULL DEFAULT NOW(), b DATETIME NOT NULL DEFAULT NOW(3) ON UPDATE NOW(3));")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	ct, ok := res.Statements[0].(*tidbast.CreateTableStmt)
	if !ok {
		t.Fatalf("statement type = %T, want CreateTableStmt", res.Statements[0])
	}
	want := []struct {
		column string
		defFn  string
		defArg int
		updFn  string
		updArg int
	}{
		{"a", "current_timestamp", 0, "", -1},
		{"b", "current_timestamp", 1, "current_timestamp", 1},
	}
	for i, cd := range ct.Cols {
		w := want[i]
		if cd.Name.Name.L != w.column {
			t.Fatalf("column %d = %q, want %q", i, cd.Name.Name.L, w.column)
		}
		for _, opt := range cd.Options {
			call, isCall := opt.Expr.(*tidbast.FuncCallExpr)
			switch opt.Tp {
			case tidbast.ColumnOptionDefaultValue:
				if !isCall || call.FnName.L != w.defFn || len(call.Args) != w.defArg {
					t.Fatalf("column %s default expr = %T/%v, want FuncCallExpr %s args=%d",
						w.column, opt.Expr, opt.Expr, w.defFn, w.defArg)
				}
			case tidbast.ColumnOptionOnUpdate:
				if !isCall || call.FnName.L != w.updFn || len(call.Args) != w.updArg {
					t.Fatalf("column %s on-update expr = %T/%v, want FuncCallExpr %s args=%d",
						w.column, opt.Expr, opt.Expr, w.updFn, w.updArg)
				}
			}
		}
	}
}

// TestT06A9SpecRoleFlags pins the extracted column flags for each frozen
// input role so the rule's source facts are locked at the extraction seam.
func TestT06A9SpecRoleFlags(t *testing.T) {
	cases := []struct {
		name string
		sql  string
		want map[string][3]bool // column -> {HasDefault, DCT, OCT}
	}{
		{"A-complete-pair",
			"CREATE TABLE t (id INT PRIMARY KEY, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP);",
			map[string][3]bool{
				"id":         {false, false, false},
				"created_at": {true, true, false},
				"updated_at": {true, true, true},
			}},
		{"B-missing-created",
			"CREATE TABLE t (id INT PRIMARY KEY, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP);",
			map[string][3]bool{
				"id":         {false, false, false},
				"updated_at": {true, true, true},
			}},
		{"C-missing-updated",
			"CREATE TABLE t (id INT PRIMARY KEY, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);",
			map[string][3]bool{
				"id":         {false, false, false},
				"created_at": {true, true, false},
			}},
		{"E-now-spelling",
			"CREATE TABLE t (id INT PRIMARY KEY, created_at DATETIME NOT NULL DEFAULT NOW(), updated_at DATETIME NOT NULL DEFAULT NOW() ON UPDATE NOW());",
			map[string][3]bool{
				"id":         {false, false, false},
				"created_at": {true, true, false},
				"updated_at": {true, true, true},
			}},
		{"fsp-preserved",
			"CREATE TABLE t (c DATETIME(3) DEFAULT NOW(3) ON UPDATE NOW(3));",
			map[string][3]bool{
				"c": {true, true, true},
			}},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, tc := range cases {
			t.Run(string(dialect)+"/"+tc.name, func(t *testing.T) {
				statement := t06a2Extract(t, dialect, tc.sql)
				for _, c := range statement.DDL.Columns {
					w, ok := tc.want[c.Name]
					if !ok {
						t.Fatalf("unexpected column %q in %s", c.Name, tc.name)
					}
					got := [3]bool{c.HasDefault, c.DefaultIsCurrentTimestamp, c.OnUpdateCurrentTimestamp}
					if got != w {
						t.Fatalf("column %s flags = %v, want %v", c.Name, got, w)
					}
				}
				if len(statement.DDL.Columns) != len(tc.want) {
					t.Fatalf("columns = %d, want %d", len(statement.DDL.Columns), len(tc.want))
				}
			})
		}
	}
}
