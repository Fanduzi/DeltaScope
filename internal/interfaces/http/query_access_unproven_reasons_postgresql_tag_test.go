//go:build postgresql

// Package httpapi verifies HTTP passthrough of unproven-effect reason codes.
// input: POST /v1/query-access/analyze for PostgreSQL unproven effects
// output: JSON reason_codes, indeterminate admission, no severity/SQL leak
// pos: HTTP surface coverage for Query Access T4 reason explanation
// note: if this file changes, update this header and module README.md.
package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerQueryAccessPostgreSQLUnprovenReasons(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

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
			body, _ := json.Marshal(map[string]string{
				"sql":     tc.sql,
				"dialect": "postgresql",
				"mode":    "strict",
			})
			req := httptest.NewRequest(http.MethodPost, "/v1/query-access/analyze", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
			}
			var payload map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if payload["read_classification"] != "indeterminate" {
				t.Errorf("classification: got %#v", payload["read_classification"])
			}
			if payload["admission"] != "indeterminate" {
				t.Errorf("admission: got %#v", payload["admission"])
			}
			codes, _ := payload["reason_codes"].([]any)
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
			raw := rec.Body.String()
			for _, bad := range []string{"severity", "password", "postgres://"} {
				if strings.Contains(raw, bad) {
					t.Errorf("HTTP body leaked %q", bad)
				}
			}
			if _, ok := payload["severity"]; ok {
				t.Error("severity field must not exist")
			}
			if _, ok := payload["sql"]; ok {
				t.Error("sql field must not exist")
			}
		})
	}
}
