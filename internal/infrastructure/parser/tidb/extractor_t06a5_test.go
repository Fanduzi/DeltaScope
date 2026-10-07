// Package tidbparser verifies the T06-A5 declared-length sentinel contract:
// the extractor carries the TiDB FieldType Flen verbatim into spec.Column.Length
// for both dialects, so an omitted CHAR length lands as the parser's
// UnspecifiedLength (-1) and an explicit CHAR(0) lands as 0 — two distinct
// facts that must never merge. Bare VARCHAR and the CHAR BYTE alias record
// the real parser-refusal controls for this slice.
// input: MySQL and TiDB CREATE TABLE inputs through the real parser and extractor
// output: pinned Flen→Length mappings for declared, omitted, and zero lengths plus parser-error controls
// pos: infrastructure parser extractor test coverage for issue #85 T06-A5
// note: if this file changes, update this header and module README.md.
package tidbparser

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/pingcap/tidb/pkg/parser/ast"
	tidbtypes "github.com/pingcap/tidb/pkg/parser/types"
)

// t06a5Flen returns the raw FieldType Flen of the named column, keeping the
// AST-level fact visible next to the extracted spec.Column.Length.
func t06a5Flen(t *testing.T, sql, name string) int {
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
		if col.Name.Name.L == name {
			return col.Tp.GetFlen()
		}
	}
	t.Fatalf("column %q not found in %q", name, sql)
	return 0
}

// TestT06A5DeclaredLengthSentinelMatrix pins the Flen→Length mapping for the
// declared, omitted, and explicit-zero forms: the parser's UnspecifiedLength
// (-1) is preserved verbatim for a bare CHAR (whose rendered Type already
// canonicalizes to char(1), matching MySQL CHAR≡CHAR(1)) and CHAR(0) stays
// 0 — the two share only the rule-level outcome (both under any positive
// limit), never the underlying fact.
func TestT06A5DeclaredLengthSentinelMatrix(t *testing.T) {
	cases := []struct {
		name     string
		sql      string
		wantFlen int
		wantType string
	}{
		{"char_declared", "CREATE TABLE t (c CHAR(7));", 7, "char(7)"},
		{"char_zero", "CREATE TABLE t (c CHAR(0));", 0, "char(0)"},
		{"char_bare_unspecified", "CREATE TABLE t (c CHAR);", tidbtypes.UnspecifiedLength, "char(1)"},
		{"varchar_declared", "CREATE TABLE t (c VARCHAR(7));", 7, "varchar(7)"},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, tc := range cases {
			t.Run(string(dialect)+"/"+tc.name, func(t *testing.T) {
				flen := t06a5Flen(t, tc.sql, "c")
				if flen != tc.wantFlen {
					t.Fatalf("%s raw Flen = %d, want %d", dialect, flen, tc.wantFlen)
				}
				column := t06a3Column(t, t06a2Extract(t, dialect, tc.sql), "c")
				if column.Length != tc.wantFlen || column.Type != tc.wantType {
					t.Fatalf("%s column = {Type:%q Length:%d}, want {%q %d}",
						dialect, column.Type, column.Length, tc.wantType, tc.wantFlen)
				}
			})
		}
	}
}

// TestT06A5LengthRequiringFormsReject pins the real parser-error controls:
// bare VARCHAR and the CHAR BYTE alias are refused before extraction, so no
// fabricated length ever reaches the rules — parser errors stay parser
// errors, never silent pass or a forged zero.
func TestT06A5LengthRequiringFormsReject(t *testing.T) {
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, sql := range []string{
			"CREATE TABLE t (c VARCHAR);",
			"CREATE TABLE t (c CHAR BYTE);",
		} {
			t.Run(string(dialect)+"/"+sql, func(t *testing.T) {
				if _, err := New().Parse(context.Background(), sql); err == nil {
					t.Fatalf("%s %q parsed successfully, want parser error", dialect, sql)
				}
			})
		}
	}
}
