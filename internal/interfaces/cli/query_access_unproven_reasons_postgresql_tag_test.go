//go:build postgresql

// Package cli verifies CLI passthrough of unproven-effect reason codes.
// input: CLI query-access analyze for PostgreSQL unproven effects
// output: JSON reason_codes, indeterminate admission exit code, no severity/SQL leak
// pos: CLI surface coverage for Query Access T4 reason explanation
// note: if this file changes, update this header and module README.md.
package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestQueryAccessAnalyzePostgreSQLUnprovenReasons(t *testing.T) {
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
			var stdout, stderr bytes.Buffer
			exitCode := Execute(t.Context(), []string{
				"query-access", "analyze",
				"--sql", tc.sql,
				"--dialect", "postgresql",
			}, &bytes.Buffer{}, &stdout, &stderr)

			if exitCode != exitQueryAccessIndeterminate {
				t.Fatalf("expected exit %d (indeterminate), got %d: %s", exitQueryAccessIndeterminate, exitCode, stderr.String())
			}

			var result map[string]any
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
				t.Fatalf("decode: %v body=%s", err, stdout.String())
			}
			if result["read_classification"] != "indeterminate" {
				t.Errorf("classification: got %#v", result["read_classification"])
			}
			if result["admission"] != "indeterminate" {
				t.Errorf("admission: got %#v", result["admission"])
			}
			codes, _ := result["reason_codes"].([]any)
			found := false
			for _, c := range codes {
				s, _ := c.(string)
				if s == tc.want {
					found = true
				}
			}
			if !found {
				t.Errorf("expected reason %q in %#v", tc.want, codes)
			}
			body := stdout.String()
			for _, bad := range []string{"severity", "password", "postgres://"} {
				if strings.Contains(body, bad) {
					t.Errorf("CLI output leaked %q", bad)
				}
			}
			// Do not require absence of SQL fragments from --sql echo in stderr; stdout result must not re-embed raw SQL as free text fields.
			if _, ok := result["sql"]; ok {
				t.Error("result must not include sql field")
			}
			if _, ok := result["severity"]; ok {
				t.Error("result must not include severity")
			}
			if _, ok := result["effect_candidates"]; ok {
				t.Error("CLI JSON must not include effect_candidates")
			}
			// Candidate-internal spellings must not appear as free-standing public fields.
			for _, bad := range []string{"NamePath", "name_path", "TargetTypePath"} {
				if strings.Contains(body, bad) {
					t.Errorf("CLI output must not contain %q", bad)
				}
			}
		})
	}
}
