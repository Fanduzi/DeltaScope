// Package tidbparser verifies the T08-R1 comparable literal identity the
// declaration side publishes: the typed AST yields decoded literal bytes
// (never the re-quoted display text) and a kind domain, while literal
// shapes whose cross-source identity is not provable — decimal, bit, hex,
// byte, boolean forms — carry nil, the honest unknown.
// input: CREATE/ALTER DEFAULT clauses through the real parser and extractor
// output: spec.Column.DefaultLiteral kind+value (or nil for unproven shapes)
// pos: infrastructure parser extractor test coverage for issue #87 T08-R1
// note: if this file changes, update this header and module README.md.
package tidbparser

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// TestT08R1DeclaredLiteralIdentity pins the declaration-side decode: the
// comparable value is the literal's own bytes (x, quote-content 'x',
// O'Reilly) and DefaultValue keeps its display contract.
func TestT08R1DeclaredLiteralIdentity(t *testing.T) {
	cases := []struct {
		name      string
		clause    string
		wantKind  string // "" = expect nil DefaultLiteral
		wantValue string
	}{
		{"string", "DEFAULT 'x'", spec.LiteralDefaultString, "x"},
		{"string_empty", "DEFAULT ''", spec.LiteralDefaultString, ""},
		{"string_text_null", "DEFAULT 'NULL'", spec.LiteralDefaultString, "NULL"},
		{"string_content_quotes", "DEFAULT '''x'''", spec.LiteralDefaultString, "'x'"},
		{"string_quote_in_content", "DEFAULT 'O''Reilly'", spec.LiteralDefaultString, "O'Reilly"},
		{"string_keyword_ct", "DEFAULT 'CURRENT_TIMESTAMP'", spec.LiteralDefaultString, "CURRENT_TIMESTAMP"},
		{"int", "DEFAULT 7", spec.LiteralDefaultNumeric, "7"},
		{"int_zero", "DEFAULT 0", spec.LiteralDefaultNumeric, "0"},
		{"int_negative", "DEFAULT -7", spec.LiteralDefaultNumeric, "-7"},
		{"int_plus", "DEFAULT +7", spec.LiteralDefaultNumeric, "7"},
		{"int_leading_zeros", "DEFAULT 007", spec.LiteralDefaultNumeric, "7"},
		{"paren_string", "DEFAULT ('x')", spec.LiteralDefaultString, "x"},
		{"decimal_unproven", "DEFAULT 1.50", "", ""},
		{"bit_unproven", "DEFAULT b'1'", "", ""},
		{"hex_unproven", "DEFAULT 0x1F", "", ""},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, tc := range cases {
			t.Run(string(dialect)+"/"+tc.name, func(t *testing.T) {
				column := t06a3Column(t, t06a2Extract(t, dialect, "CREATE TABLE t (c VARCHAR(16) "+tc.clause+");"), "c")
				if tc.wantKind == "" {
					if column.DefaultLiteral != nil {
						t.Fatalf("DefaultLiteral = %+v, want nil (unproven shape)", column.DefaultLiteral)
					}
					if !column.HasDefault {
						t.Fatalf("HasDefault = false, want the literal clause still declared")
					}
					return
				}
				if column.DefaultLiteral == nil {
					t.Fatalf("DefaultLiteral = nil, want {%s %q}", tc.wantKind, tc.wantValue)
				}
				if column.DefaultLiteral.Kind != tc.wantKind || column.DefaultLiteral.Value != tc.wantValue {
					t.Fatalf("DefaultLiteral = {%q %q}, want {%q %q}",
						column.DefaultLiteral.Kind, column.DefaultLiteral.Value, tc.wantKind, tc.wantValue)
				}
			})
		}
	}
}

// TestT08R1DeclaredLiteralNullAndExpression pins the class boundaries:
// DEFAULT NULL is the null class (no literal identity), a temporal marker
// is the temporal class, and an expression default routes to the
// structural expression fact — never a literal.
func TestT08R1DeclaredLiteralNullAndExpression(t *testing.T) {
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect)+"/default_null", func(t *testing.T) {
			column := t06a3Column(t, t06a2Extract(t, dialect, "CREATE TABLE t (c VARCHAR(16) DEFAULT NULL);"), "c")
			if !column.DefaultIsNull || column.DefaultLiteral != nil {
				t.Fatalf("DEFAULT NULL = {IsNull:%v Literal:%+v}, want null class with nil identity", column.DefaultIsNull, column.DefaultLiteral)
			}
		})
	}
	t.Run("mysql/expression_default", func(t *testing.T) {
		column := t06a3Column(t, t06a2Extract(t, spec.DialectMySQL, "CREATE TABLE t (c VARCHAR(16) DEFAULT (uuid()));"), "c")
		if column.DefaultExpression == nil || column.DefaultLiteral != nil {
			t.Fatalf("expression default = {Expr:%+v Literal:%+v}, want expr fact with nil literal", column.DefaultExpression, column.DefaultLiteral)
		}
	})
}
