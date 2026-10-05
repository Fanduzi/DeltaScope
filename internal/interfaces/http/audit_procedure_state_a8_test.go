// Package httpapi verifies the T05-A8 procedure lifecycle at the HTTP seam.
// input: four-statement TiDB CREATE PROCEDURE and MySQL DROP PROCEDURE /v1/audit requests through the real handler
// output: HTTP 400 vendor-boundary envelope versus HTTP 200 supported MySQL DROP result
// pos: HTTP transport contract test for procedure table-state isolation (T05-A8)
// note: if this file changes, update this header and module README.md.
package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule/catalog"
)

var t05A8HTTPLines = []string{
	"CREATE TABLE t (id INT PRIMARY KEY);",
	"",
	"ALTER TABLE t ADD COLUMN c INT;",
	"CREATE INDEX idx_c ON t(c);",
}

func t05A8HTTPPolicy(t *testing.T) string {
	t.Helper()
	enabled := map[string]string{
		"ddl.table.exists.create.forbid":          "",
		"ddl.table.exists.alter.require":          "",
		"ddl.alter.add_column.exists.forbid":      "",
		"ddl.create_index.columns.exists.require": "      required: true\n",
	}
	for id := range enabled {
		if _, ok := catalog.Lookup(id); !ok {
			t.Fatalf("missing catalog rule %s", id)
		}
	}
	var text strings.Builder
	text.WriteString("rules:\n")
	for _, entry := range catalog.All() {
		params, on := enabled[entry.RuleID]
		fmt.Fprintf(&text, "  %q:\n    enabled: %t\n", entry.RuleID, on)
		if on {
			text.WriteString("    level: blocker\n")
			if params != "" {
				text.WriteString("    params:\n" + params)
			}
		}
	}
	path := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(path, []byte(text.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func postT05A8Audit(t *testing.T, dialect, sql string) (int, map[string]any) {
	t.Helper()
	handler, err := NewHandler(t05A8HTTPPolicy(t), "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	body, err := json.Marshal(map[string]any{"sql": sql, "dialect": dialect, "schema": "golden"})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return rec.Code, payload
}

func TestHandlerT05A8ProcedureOuterState(t *testing.T) {
	cases := []struct {
		name      string
		dialect   string
		procedure string
		wantCode  int
		coverage  string
		boundary  bool
	}{
		{"tidb_create", "tidb", "CREATE PROCEDURE p() SELECT 1;", http.StatusBadRequest, "incomplete", true},
		{"mysql_drop", "mysql", "DROP PROCEDURE p;", http.StatusOK, "unverified", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lines := append([]string(nil), t05A8HTTPLines...)
			lines[1] = tc.procedure
			code, payload := postT05A8Audit(t, tc.dialect, strings.Join(lines, "\n"))
			if code != tc.wantCode {
				t.Fatalf("status = %d, want %d: %#v", code, tc.wantCode, payload)
			}
			if payload["verdict"] != "review" {
				t.Fatalf("verdict = %#v, want review", payload["verdict"])
			}
			if tc.wantCode == http.StatusBadRequest {
				errBody, _ := payload["error"].(map[string]any)
				if errBody == nil || errBody["code"] != "bad_request" {
					t.Fatalf("expected the existing bad_request error envelope, got %#v", payload["error"])
				}
				diagnosticSeen := false
				for _, raw := range payload["diagnostics"].([]any) {
					if diagnostic, _ := raw.(map[string]any); diagnostic["classification"] == "unsupported_statement" {
						diagnosticSeen = true
					}
				}
				if !diagnosticSeen {
					t.Fatalf("expected the unsupported_statement diagnostic, got %#v", payload["diagnostics"])
				}
			} else if payload["error"] != nil {
				t.Fatalf("unexpected error envelope %#v", payload["error"])
			}
			if coverage, _ := payload["coverage"].(map[string]any); coverage["status"] != tc.coverage {
				t.Fatalf("coverage = %#v, want %s", payload["coverage"], tc.coverage)
			}
			statements, _ := payload["statements"].([]any)
			if len(statements) != 4 {
				t.Fatalf("statements = %d, want 4", len(statements))
			}
			for i, raw := range statements {
				statement, _ := raw.(map[string]any)
				normalized, ok := statement["normalized_sql"].(string)
				if !ok || normalized == "" {
					t.Fatalf("statement %d normalized_sql = %#v, want nonempty string", i, statement["normalized_sql"])
				}
				if statement["index"] != float64(i) || statement["kind"] != "ddl" ||
					statement["raw_sql"] != lines[i] || normalized != strings.TrimSuffix(lines[i], ";") {
					t.Fatalf("statement %d identity = %#v", i, statement)
				}
				if statement["findings"] != nil || statement["impact"] != nil {
					t.Fatalf("statement %d = %#v, want no findings/impact", i, statement)
				}
			}
			first, _ := statements[0].(map[string]any)
			firstCoverage, _ := first["coverage"].(map[string]any)
			if firstCoverage["status"] != "unverified" {
				t.Fatalf("first coverage = %#v, want unverified", firstCoverage)
			}
			gaps, _ := first["evidence_gaps"].([]any)
			if len(gaps) != 1 {
				t.Fatalf("first gaps = %#v, want exactly one", first["evidence_gaps"])
			}
			gap, _ := gaps[0].(map[string]any)
			if gap["rule_id"] != "ddl.table.exists.create.forbid" || gap["reason_code"] != "unknown_table_state" ||
				!reflect.DeepEqual(gap["required_facts"], []any{"target_table.existence"}) {
				t.Fatalf("first gap = %#v, want unknown_table_state create-existence", gap)
			}
			procedure, _ := statements[1].(map[string]any)
			procedureCoverage, _ := procedure["coverage"].(map[string]any)
			wantProcedureCoverage := "complete"
			if tc.boundary {
				wantProcedureCoverage = "incomplete"
			}
			if procedureCoverage["status"] != wantProcedureCoverage || procedure["evidence_gaps"] != nil {
				t.Fatalf("procedure = %#v, want %s with no gaps", procedure, wantProcedureCoverage)
			}
			for i := 2; i < 4; i++ {
				follower, _ := statements[i].(map[string]any)
				followerCoverage, _ := follower["coverage"].(map[string]any)
				if followerCoverage["status"] != "complete" || follower["evidence_gaps"] != nil {
					t.Fatalf("follower %d = %#v, want complete with no gaps", i, follower)
				}
			}
			unsupported, _ := payload["unsupported"].([]any)
			wantUnsupported := 0
			if tc.boundary {
				wantUnsupported = 1
			}
			if len(unsupported) != wantUnsupported {
				t.Fatalf("unsupported = %#v, want %d", payload["unsupported"], wantUnsupported)
			}
			if tc.boundary {
				item, _ := unsupported[0].(map[string]any)
				if item["index"] != float64(1) || item["feature"] != "create_procedure" ||
					item["reason"] != "parsed by the shared parser but outside the supported statement surface for this dialect" ||
					item["sql"] != procedure["raw_sql"] ||
					!reflect.DeepEqual(item["metadata"], map[string]any{"boundary": "vendor"}) {
					t.Fatalf("boundary = %#v, want create_procedure vendor at index 1 bound to raw_sql", item)
				}
			}
			if payload["global_findings"] != nil {
				t.Fatalf("global findings = %#v, want none", payload["global_findings"])
			}
			if summary, _ := payload["summary"].(map[string]any); summary["blockers"] != float64(0) || summary["warnings"] != float64(0) || summary["notices"] != float64(0) {
				t.Fatalf("finding counters = %#v, want zero", summary)
			}
		})
	}
}
