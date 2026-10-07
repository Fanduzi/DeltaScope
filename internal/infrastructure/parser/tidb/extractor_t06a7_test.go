// Package tidbparser verifies the T06-A7 charset/collation fact contract:
// column-level CHARACTER SET lands on the FieldType while COLLATE arrives as a
// named column option, both stay explicit-declaration facts (no inheritance is
// inferred), and the table-level COLLATE clause lands verbatim in
// DDL.Options["collate"].
// input: MySQL and TiDB CREATE TABLE inputs covering the S1–S4 baseline shapes plus a bare control
// output: pinned AST FieldType/column-option and spec.Charset/Collation/Options fields for each shape
// pos: infrastructure parser extractor test coverage for issue #85 T06-A7
// note: if this file changes, update this header and module README.md.
package tidbparser

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/pingcap/tidb/pkg/parser/ast"
)

// t06a7ColumnAST returns the raw FieldType charset/collate plus the named
// column option enum set of the named column, keeping the AST-level channels
// visible next to the extracted spec.Column fields.
func t06a7ColumnAST(t *testing.T, sql, name string) (charset, collate string, hasCollateOption bool) {
	t.Helper()
	result, err := New().Parse(context.Background(), sql)
	if err != nil {
		t.Fatalf("parse %q: %v", sql, err)
	}
	create, ok := result.Statements[0].(*ast.CreateTableStmt)
	if !ok {
		t.Fatalf("statement type = %T, want *ast.CreateTableStmt", result.Statements[0])
	}
	for _, col := range create.Cols {
		if col.Name.Name.L != name {
			continue
		}
		for _, option := range col.Options {
			if option != nil && option.Tp == ast.ColumnOptionCollate {
				hasCollateOption = true
			}
		}
		return col.Tp.GetCharset(), col.Tp.GetCollate(), hasCollateOption
	}
	t.Fatalf("column %q not found in %q", name, sql)
	return "", "", false
}

// TestT06A7DeclaredCharsetCollationFields pins the two declaration channels:
// an explicit column CHARACTER SET lands on the FieldType and spec.Charset,
// an explicit COLLATE lands via ast.ColumnOptionCollate and spec.Collation,
// and nothing is filled when undeclared.
func TestT06A7DeclaredCharsetCollationFields(t *testing.T) {
	cases := []struct {
		name          string
		sql           string
		wantFTCharset string
		wantCollOpt   bool
		wantSpecChs   string
		wantSpecColl  string
	}{
		{"pair", "CREATE TABLE t (c VARCHAR(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin);", "utf8mb4", true, "utf8mb4", "utf8mb4_bin"},
		{"collate_only", "CREATE TABLE t (c VARCHAR(16) COLLATE utf8mb4_bin);", "", true, "", "utf8mb4_bin"},
		{"charset_only", "CREATE TABLE t (c VARCHAR(16) CHARACTER SET utf8mb4);", "utf8mb4", false, "utf8mb4", ""},
		{"mismatched", "CREATE TABLE t (c VARCHAR(16) CHARACTER SET utf8mb4 COLLATE latin1_swedish_ci);", "utf8mb4", true, "utf8mb4", "latin1_swedish_ci"},
		{"undeclared", "CREATE TABLE t (c VARCHAR(16));", "", false, "", ""},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, tc := range cases {
			t.Run(string(dialect)+"/"+tc.name, func(t *testing.T) {
				ftCharset, _, hasCollOpt := t06a7ColumnAST(t, tc.sql, "c")
				if ftCharset != tc.wantFTCharset || hasCollOpt != tc.wantCollOpt {
					t.Fatalf("AST = charset %q collate-option %v, want %q/%v",
						ftCharset, hasCollOpt, tc.wantFTCharset, tc.wantCollOpt)
				}
				column := t06a3Column(t, t06a2Extract(t, dialect, tc.sql), "c")
				if column.Charset != tc.wantSpecChs || column.Collation != tc.wantSpecColl {
					t.Fatalf("spec = {%q %q}, want {%q %q}",
						column.Charset, column.Collation, tc.wantSpecChs, tc.wantSpecColl)
				}
			})
		}
	}
}

// TestT06A7TableCollateOptionField pins the table-level channel: a declared
// table COLLATE clause lands verbatim in DDL.Options["collate"], while an
// undeclared table leaves the key absent — no server default is inferred.
func TestT06A7TableCollateOptionField(t *testing.T) {
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, tc := range []struct {
			name string
			sql  string
			want string
		}{
			{"declared", "CREATE TABLE t (c INT) COLLATE=utf8mb4_bin;", "utf8mb4_bin"},
			{"undeclared", "CREATE TABLE t (c INT);", ""},
		} {
			t.Run(string(dialect)+"/"+tc.name, func(t *testing.T) {
				statement := t06a2Extract(t, dialect, tc.sql)
				got, present := statement.DDL.Options["collate"]
				if present != (tc.want != "") || got != tc.want {
					t.Fatalf("options.collate = (%q,%v), want (%q,%v)",
						got, present, tc.want, tc.want != "")
				}
			})
		}
	}
}
