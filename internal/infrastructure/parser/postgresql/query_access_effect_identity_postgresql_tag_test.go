//go:build postgresql

// Package postgresql AST characterizations for pure-read effect identity (T3).
// input: SQL spanning structural BoolExpr, comparison candidates, count, rejected forms
// output: QueryAccessFacts classification freeze (no identity proof / promotion)
// pos: infrastructure-level characterization tests for query access extractor
// note: tests only; does not implement effect candidate projection or OID resolution.
package postgresql

import (
	"context"
	"testing"

	domain "github.com/Fanduzi/DeltaScope/internal/domain/queryaccess"
)

func extractQA(t *testing.T, sql string) *QueryAccessFacts {
	t.Helper()
	extractor := &QueryAccessExtractor{}
	facts, err := extractor.ExtractQueryAccess(context.Background(), sql, "postgresql", "public")
	if err != nil {
		t.Fatalf("extract %q: %v", sql, err)
	}
	return facts
}

// TestQueryAccessAST_StructuralBoolExprIsNotOperatorPresence confirms BoolExpr
// AND/OR/NOT alone do not trip operator/cast presence classification. They are
// structural AST nodes, not catalog identity proofs.
func TestQueryAccessAST_StructuralBoolExprIsNotOperatorPresence(t *testing.T) {
	t.Parallel()

	cases := []string{
		"SELECT id FROM users WHERE active",
		"SELECT id FROM users WHERE active AND inactive",
		"SELECT id FROM users WHERE active OR inactive",
		"SELECT id FROM users WHERE NOT active",
	}
	for _, sql := range cases {
		sql := sql
		t.Run(sql, func(t *testing.T) {
			t.Parallel()
			facts := extractQA(t, sql)
			if facts.ReadClassification != string(domain.ReadOnly) {
				t.Errorf("classification: got %q, want %q (structural BoolExpr only)",
					facts.ReadClassification, domain.ReadOnly)
			}
		})
	}
}

// TestQueryAccessAST_CandidateComparisonsIndeterminate freezes A_Expr comparison
// presence as indeterminate (T2 candidate set; not yet identity-proven).
func TestQueryAccessAST_CandidateComparisonsIndeterminate(t *testing.T) {
	t.Parallel()

	cases := []string{
		"SELECT id FROM users WHERE id = 1",
		"SELECT id FROM users WHERE id <> 1",
		"SELECT id FROM users WHERE id < 1",
		"SELECT id FROM users WHERE id > 1",
		"SELECT id FROM users WHERE id <= 1",
		"SELECT id FROM users WHERE id >= 1",
		"SELECT id FROM users WHERE name = 'alice'",
		"SELECT id FROM users WHERE active = true",
		"SELECT id FROM users WHERE id = 1 AND active",
		"SELECT id FROM users WHERE id OPERATOR(pg_catalog.=) 1",
	}
	for _, sql := range cases {
		sql := sql
		t.Run(sql, func(t *testing.T) {
			t.Parallel()
			facts := extractQA(t, sql)
			if facts.ReadClassification != string(domain.Indeterminate) {
				t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.Indeterminate)
			}
		})
	}
}

// TestQueryAccessAST_CountCandidatesIndeterminate freezes COUNT forms as
// FuncCall presence → indeterminate (manifest+proof required later).
func TestQueryAccessAST_CountCandidatesIndeterminate(t *testing.T) {
	t.Parallel()

	for _, sql := range []string{
		"SELECT COUNT(*) FROM users",
		"SELECT COUNT(id) FROM users",
	} {
		sql := sql
		t.Run(sql, func(t *testing.T) {
			t.Parallel()
			facts := extractQA(t, sql)
			if facts.ReadClassification != string(domain.Indeterminate) {
				t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.Indeterminate)
			}
		})
	}
}

// TestQueryAccessAST_RejectedLedgerIndeterminate freezes rejected-ledger shapes
// at the extractor: session helpers, file readers, UDFs, casts.
func TestQueryAccessAST_RejectedLedgerIndeterminate(t *testing.T) {
	t.Parallel()

	cases := []string{
		"SELECT current_setting('search_path')",
		"SELECT set_config('a','b',true)",
		"SELECT pg_get_userbyid(1)",
		"SELECT pg_read_file('/tmp/x')",
		"SELECT my_udf(id) FROM users",
		"SELECT app.my_func(id) FROM users",
		"SELECT id::text FROM users",
		"SELECT CAST(id AS text) FROM users",
		"SELECT id FROM users WHERE id = 'x'",
		"SELECT id FROM users WHERE id = 1.5",
	}
	for _, sql := range cases {
		sql := sql
		t.Run(sql, func(t *testing.T) {
			t.Parallel()
			facts := extractQA(t, sql)
			if facts.ReadClassification != string(domain.Indeterminate) {
				t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.Indeterminate)
			}
		})
	}
}

// TestQueryAccessAST_TypeCastIsOperatorPresence documents that TypeCast is
// treated as operator-expression presence today (not a trusted cast identity).
func TestQueryAccessAST_TypeCastIsOperatorPresence(t *testing.T) {
	t.Parallel()
	facts := extractQA(t, "SELECT id::int4 FROM users")
	if facts.ReadClassification != string(domain.Indeterminate) {
		t.Errorf("classification: got %q, want %q", facts.ReadClassification, domain.Indeterminate)
	}
}
