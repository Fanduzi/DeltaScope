// Package tidbparser verifies the R1 expression-structure facts: the
// top-level root form classification, the authored-root parens wrapper
// fact, and the authored-parenthesis syntax fact bound through the
// same-width supplement.
// input: real-parser statement texts over the R1 expression shapes
// output: exact TopForm classes, RootParenthesized flags, CallCount node
// counts, DefaultParenthesized flags, and unchanged literal/bare controls
// pos: T07-R1 parser-side fact pins beside the catalog expression suite
// note: if this file changes, update this header and module README.md.
package tidbparser

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// r1Statements parses and extracts one SQL string into spec statements.
func r1Statements(t *testing.T, sql string) []spec.Statement {
	t.Helper()
	parser := New()
	result, err := parser.Parse(context.Background(), sql)
	if err != nil {
		t.Fatalf("parse %q: %v", sql, err)
	}
	wrapped := WrapStatements(result.Statements, result.Warnings)
	statements := make([]spec.Statement, 0, len(wrapped))
	for _, item := range wrapped {
		statement, err := item.Extractor.Extract(spec.DialectTiDB, item.RawSQL)
		if err != nil {
			t.Fatalf("extract %q: %v", sql, err)
		}
		statements = append(statements, statement)
	}
	return statements
}

// TestT07R1ExpressionTopForm pins the outer-root classification for every
// observed shape family.
func TestT07R1ExpressionTopForm(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		sql         string
		wantForm    string
		wantWrapped bool
		wantCalls   int
	}{
		{"call", "ALTER TABLE t ALTER COLUMN v SET DEFAULT (uuid())", spec.ExpressionTopCall, false, 1},
		{"parens_call", "ALTER TABLE t ALTER COLUMN v SET DEFAULT ((uuid()))", spec.ExpressionTopCall, true, 1},
		{"arithmetic_constant", "ALTER TABLE t ALTER COLUMN v SET DEFAULT (1 + 2)", spec.ExpressionTopArithmetic, false, 0},
		{"arithmetic_wrapped", "ALTER TABLE t ALTER COLUMN v SET DEFAULT ((1 + 2))", spec.ExpressionTopArithmetic, true, 0},
		{"arithmetic_column", "ALTER TABLE t ALTER COLUMN v SET DEFAULT (x + 1)", spec.ExpressionTopArithmetic, false, 0},
		{"arith_function", "ALTER TABLE t ALTER COLUMN v SET DEFAULT (uuid() + 1)", spec.ExpressionTopArithmetic, false, 1},
		{"case", "ALTER TABLE t ALTER COLUMN v SET DEFAULT (CASE WHEN 1 THEN uuid() ELSE 'x' END)", spec.ExpressionTopCase, false, 1},
		{"unary", "ALTER TABLE t ALTER COLUMN v SET DEFAULT (-x)", spec.ExpressionTopUnary, false, 0},
		{"column", "ALTER TABLE t ALTER COLUMN v SET DEFAULT (x)", spec.ExpressionTopColumn, false, 0},
		// The deduplicated Functions set cannot count call nodes — nested
		// same-callee and mixed-callee roots both carry CallCount > 1.
		{"nested_same_callee", "ALTER TABLE t ALTER COLUMN v SET DEFAULT (uuid(uuid()))", spec.ExpressionTopCall, false, 2},
		{"nested_mixed_calls", "ALTER TABLE t ALTER COLUMN v SET DEFAULT (json_object(uuid()))", spec.ExpressionTopCall, false, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			statements := r1Statements(t, tc.sql)
			if len(statements) != 1 {
				t.Fatalf("statements = %d, want 1", len(statements))
			}
			def := statements[0].DDL.Alter[0].Column.Definition
			if def.DefaultExpression == nil {
				t.Fatalf("DefaultExpression = nil for %q", tc.sql)
			}
			if def.DefaultExpression.TopForm != tc.wantForm {
				t.Fatalf("TopForm = %q, want %q", def.DefaultExpression.TopForm, tc.wantForm)
			}
			if def.DefaultExpression.RootParenthesized != tc.wantWrapped {
				t.Fatalf("RootParenthesized = %v for %q, want %v",
					def.DefaultExpression.RootParenthesized, tc.sql, tc.wantWrapped)
			}
			if def.DefaultExpression.CallCount != tc.wantCalls {
				t.Fatalf("CallCount = %d for %q, want %d",
					def.DefaultExpression.CallCount, tc.sql, tc.wantCalls)
			}
			if !def.DefaultExpression.Known {
				t.Fatalf("Known = false for %q", tc.sql)
			}
		})
	}
}

// TestT07R1ColumnRefTopForm pins a bare parenthesized column reference: it
// reaches the facts as TopColumn with one local reference — distinct from
// an arithmetic wrap.
func TestT07R1ColumnRefTopForm(t *testing.T) {
	t.Parallel()
	statements := r1Statements(t, "ALTER TABLE t ALTER COLUMN v SET DEFAULT (x)")
	facts := statements[0].DDL.Alter[0].Column.Definition.DefaultExpression
	if facts == nil || facts.TopForm != spec.ExpressionTopColumn {
		t.Fatalf("facts = %+v, want TopColumn", facts)
	}
	locals := facts.LocalReferences()
	if len(locals) != 1 || locals[0].Name != "x" {
		t.Fatalf("LocalReferences = %+v, want one local ref to x", locals)
	}
}

// TestT07R1SupplementSitesCarryColumn proves the default-parens site binds
// its column identity across CREATE and ALTER SET sources — including a
// mixed literal site in the same statement.
func TestT07R1SupplementSitesCarryColumn(t *testing.T) {
	t.Parallel()
	statements := r1Statements(t,
		"CREATE TABLE t (a INT DEFAULT (1 + 2), b INT DEFAULT 7, c VARCHAR(36) DEFAULT (uuid()))")
	cols := statements[0].DDL.Columns
	if len(cols) != 3 {
		t.Fatalf("columns = %+v, want 3", cols)
	}
	checks := []struct {
		name       string
		parens     bool
		expr       bool
		defaultVal string
	}{
		// Expression defaults carry the authored inner text as their
		// default value evidence alongside the typed expression facts.
		{"a", true, true, "1 + 2"},
		{"b", false, false, "7"},
		{"c", true, true, "uuid()"},
	}
	for i, want := range checks {
		got := cols[i]
		if got.Name != want.name || got.DefaultParenthesized != want.parens ||
			(got.DefaultExpression != nil) != want.expr || got.DefaultValue != want.defaultVal {
			t.Fatalf("column[%d] = %+v, want name=%s parens=%v expr=%v default=%q",
				i, got, want.name, want.parens, want.expr, want.defaultVal)
		}
	}
}
