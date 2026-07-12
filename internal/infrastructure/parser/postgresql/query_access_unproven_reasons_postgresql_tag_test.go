//go:build postgresql

// Package postgresql tests bounded unproven-effect reason emission for Query Access.
// input: PostgreSQL SQL with operator / function / cast presence
// output: presence-only reason codes without effect spellings or SQL leakage
// pos: parser-level coverage for T4 unproven effect reason codes
// note: if this file changes, update this header and module README.md.
package postgresql

import (
	"context"
	"strings"
	"testing"
)

func TestExtractQueryAccess_UnprovenEffectReasonCodes(t *testing.T) {
	t.Parallel()
	e := &QueryAccessExtractor{}

	cases := []struct {
		name string
		sql  string
		want []string
	}{
		{name: "operator_eq", sql: "SELECT id FROM users WHERE id = 1", want: []string{"unproven_operator_effect"}},
		{name: "operator_ge", sql: "SELECT id FROM users WHERE id >= 1", want: []string{"unproven_operator_effect"}},
		{name: "function_count_star", sql: "SELECT COUNT(*) FROM users", want: []string{"unproven_function_effect"}},
		{name: "function_count_col", sql: "SELECT COUNT(id) FROM users", want: []string{"unproven_function_effect"}},
		{name: "function_current_setting", sql: "SELECT current_setting('search_path')", want: []string{"unproven_function_effect"}},
		{name: "function_udf", sql: "SELECT my_udf(id) FROM users", want: []string{"unproven_function_effect"}},
		{name: "function_pg_get", sql: "SELECT pg_get_userbyid(10)", want: []string{"unproven_function_effect"}},
		{name: "cast_double_colon", sql: "SELECT id::text FROM users", want: []string{"unproven_cast_effect"}},
		{name: "cast_function_form", sql: "SELECT CAST(id AS text) FROM users", want: []string{"unproven_cast_effect"}},
		{name: "function_and_operator", sql: "SELECT COUNT(*) FROM users WHERE id = 1", want: []string{"unproven_function_effect", "unproven_operator_effect"}},
		{name: "structural_bool_only", sql: "SELECT id FROM users WHERE active AND inactive", want: nil},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			facts, err := e.ExtractQueryAccess(context.Background(), tc.sql, "postgresql", "public")
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			if len(facts.ReasonCodes) != len(tc.want) {
				t.Fatalf("reasons: got %v, want %v", facts.ReasonCodes, tc.want)
			}
			for i := range tc.want {
				if facts.ReasonCodes[i] != tc.want[i] {
					t.Fatalf("reasons: got %v, want %v", facts.ReasonCodes, tc.want)
				}
			}
			// No-leak: reason codes and fact dump must not include SQL spellings or secrets.
			blob := strings.Join(facts.ReasonCodes, ",")
			for _, bad := range []string{"SELECT", "COUNT", "current_setting", "::text", "my_udf", "search_path", "password", "severity"} {
				if strings.Contains(blob, bad) {
					t.Errorf("reason codes must not contain %q; got %v", bad, facts.ReasonCodes)
				}
			}
		})
	}
}

func TestExtractQueryAccess_UnprovenReasons_DeterministicOrder(t *testing.T) {
	t.Parallel()
	e := &QueryAccessExtractor{}
	sql := "SELECT COUNT(id)::text FROM users WHERE id = 1"
	var first []string
	for i := 0; i < 5; i++ {
		facts, err := e.ExtractQueryAccess(context.Background(), sql, "postgresql", "public")
		if err != nil {
			t.Fatalf("extract: %v", err)
		}
		if i == 0 {
			first = append([]string(nil), facts.ReasonCodes...)
			continue
		}
		if len(facts.ReasonCodes) != len(first) {
			t.Fatalf("iteration %d: len mismatch %v vs %v", i, facts.ReasonCodes, first)
		}
		for j := range first {
			if facts.ReasonCodes[j] != first[j] {
				t.Fatalf("iteration %d: order mismatch %v vs %v", i, facts.ReasonCodes, first)
			}
		}
	}
	// Presence of cast + function + operator
	wantSet := map[string]bool{
		"unproven_cast_effect":     true,
		"unproven_function_effect": true,
		"unproven_operator_effect": true,
	}
	if len(first) != 3 {
		t.Fatalf("expected 3 reasons, got %v", first)
	}
	for _, c := range first {
		if !wantSet[c] {
			t.Errorf("unexpected reason %q in %v", c, first)
		}
	}
}
