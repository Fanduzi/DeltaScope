//go:build postgresql

// Package deltascope verifies SDK surface passthrough of unproven-effect reason codes.
// input: PostgreSQL SQL via public AnalyzeQueryAccess
// output: reason_codes machine ids, no severity, no SQL leakage, admission freeze
// pos: public API coverage for Query Access T4 reason explanation
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestAnalyzeQueryAccess_PostgreSQLUnprovenReasons(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		sql  string
		want string
	}{
		{name: "operator", sql: "SELECT id FROM users WHERE id = 1", want: "unproven_operator_effect"},
		{name: "function", sql: "SELECT COUNT(*) FROM users", want: "unproven_function_effect"},
		{name: "cast", sql: "SELECT id::text FROM users", want: "unproven_cast_effect"},
		{name: "limit_function", sql: "SELECT id FROM users LIMIT length('a')", want: "unproven_function_effect"},
		{name: "values_function", sql: "VALUES (length('a'))", want: "unproven_function_effect"},
		{name: "window_partition", sql: "SELECT row_number() OVER (PARTITION BY length(name)) FROM users", want: "unproven_function_effect"},
		{name: "agg_filter", sql: "SELECT count(*) FILTER (WHERE length(name) > 0) FROM users", want: "unproven_function_effect"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := AnalyzeQueryAccess(context.Background(), QueryAccessRequest{
				SQL:     tc.sql,
				Dialect: DialectPostgreSQL,
				Mode:    QueryAccessModeStrict,
			})
			if err != nil {
				t.Fatalf("analyze: %v", err)
			}
			if result.ReadClassification != QueryAccessIndeterminate {
				t.Errorf("classification: got %q, want indeterminate", result.ReadClassification)
			}
			if result.Admission != QueryAccessIndeterminateAdmission {
				t.Errorf("admission: got %q, want indeterminate", result.Admission)
			}
			found := false
			for _, rc := range result.ReasonCodes {
				if rc == tc.want {
					found = true
				}
				if strings.ContainsAny(rc, " \t'\"()") {
					t.Errorf("reason not machine-like: %q", rc)
				}
			}
			if !found {
				t.Errorf("expected reason %q in %v", tc.want, result.ReasonCodes)
			}

			data, err := json.Marshal(result)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			raw := string(data)
			for _, bad := range []string{"severity", "password", "postgres://", "COUNT(*)", "id::text"} {
				if strings.Contains(raw, bad) {
					t.Errorf("JSON leaked %q: %s", bad, raw)
				}
			}
		})
	}
}

func TestAnalyzeQueryAccess_PostgreSQLNoCandidateFieldsInJSON(t *testing.T) {
	t.Parallel()

	result, err := AnalyzeQueryAccess(context.Background(), QueryAccessRequest{
		SQL:     "SELECT COUNT(*) FROM users WHERE id = 1 LIMIT length('a')",
		Dialect: DialectPostgreSQL,
		Mode:    QueryAccessModeStrict,
	})
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	raw := string(data)
	for _, bad := range []string{
		"effect_candidates", "EffectCandidates", "NamePath", "name_path",
		"TargetTypePath", "severity", "COUNT", "length", "password",
	} {
		if strings.Contains(raw, bad) {
			t.Errorf("public JSON must not contain %q; got %s", bad, raw)
		}
	}
	// Public contract: still indeterminate with unproven reasons only.
	if result.Admission != QueryAccessIndeterminateAdmission {
		t.Errorf("admission: got %q", result.Admission)
	}
	if len(result.ReasonCodes) == 0 {
		t.Error("expected unproven reason codes on public result")
	}
}

func TestAnalyzeQueryAccess_PostgreSQLReasonOrderDeterministic(t *testing.T) {
	t.Parallel()

	sql := "SELECT COUNT(id)::text FROM users WHERE id = 1"
	var first []string
	for i := 0; i < 3; i++ {
		result, err := AnalyzeQueryAccess(context.Background(), QueryAccessRequest{
			SQL: sql, Dialect: DialectPostgreSQL, Mode: QueryAccessModeStrict,
		})
		if err != nil {
			t.Fatalf("analyze: %v", err)
		}
		if i == 0 {
			first = append([]string(nil), result.ReasonCodes...)
			continue
		}
		if len(result.ReasonCodes) != len(first) {
			t.Fatalf("len mismatch %v vs %v", result.ReasonCodes, first)
		}
		for j := range first {
			if result.ReasonCodes[j] != first[j] {
				t.Fatalf("order mismatch %v vs %v", result.ReasonCodes, first)
			}
		}
	}
}
