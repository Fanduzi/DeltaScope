// Package httpapi verifies the T06-A1 primary-key-presence contract at the HTTP seam.
// input: MySQL/TiDB CREATE TABLE /v1/audit requests with the isolated ddl.table.primary_key.require policy
// output: HTTP 200 reject verdict carrying the blocker finding, and HTTP 200 pass when a PK is declared
// pos: HTTP transport contract test for the frozen T06-A1 oracle (issue #85)
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
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule/catalog"
)

func t06A1HTTPPolicy(t *testing.T) string {
	t.Helper()
	if _, ok := catalog.Lookup("ddl.table.primary_key.require"); !ok {
		t.Fatal("missing catalog rule ddl.table.primary_key.require")
	}
	var text strings.Builder
	text.WriteString("rules:\n")
	for _, entry := range catalog.All() {
		fmt.Fprintf(&text, "  %q:\n    enabled: %t\n", entry.RuleID,
			entry.RuleID == "ddl.table.primary_key.require")
		if entry.RuleID == "ddl.table.primary_key.require" {
			text.WriteString("    level: blocker\n")
			text.WriteString("    params:\n      required: true\n")
		}
	}
	path := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(path, []byte(text.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func postT06A1Audit(t *testing.T, dialect, sql string) (int, map[string]any) {
	t.Helper()
	handler, err := NewHandler(t06A1HTTPPolicy(t), "test-build")
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

func TestHandlerT06A1PrimaryKeyPresence(t *testing.T) {
	cases := []struct {
		name    string
		dialect string
		sql     string
		verdict string
	}{
		{"mysql_no_pk", "mysql", "CREATE TABLE t (id INT);", "reject"},
		{"mysql_table_pk", "mysql", "CREATE TABLE t (id INT, PRIMARY KEY (id));", "pass"},
		{"tidb_no_pk", "tidb", "CREATE TABLE t (id INT);", "reject"},
		{"tidb_table_pk", "tidb", "CREATE TABLE t (id INT, PRIMARY KEY (id));", "pass"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, payload := postT06A1Audit(t, tc.dialect, tc.sql)
			if code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %#v", code, payload)
			}
			if payload["verdict"] != tc.verdict {
				t.Fatalf("verdict = %#v, want %s", payload["verdict"], tc.verdict)
			}
			if payload["error"] != nil {
				t.Fatalf("unexpected error envelope %#v", payload["error"])
			}
			if coverage, _ := payload["coverage"].(map[string]any); coverage["status"] != "complete" {
				t.Fatalf("coverage = %#v, want complete", payload["coverage"])
			}
			statements, _ := payload["statements"].([]any)
			if len(statements) != 1 {
				t.Fatalf("statements = %d, want 1", len(statements))
			}
			statement, _ := statements[0].(map[string]any)
			if statement["index"] != float64(0) || statement["kind"] != "ddl" ||
				statement["raw_sql"] != tc.sql ||
				statement["normalized_sql"] != strings.TrimSuffix(tc.sql, ";") ||
				statement["impact"] != nil {
				t.Fatalf("statement identity = %#v", statement)
			}
			statementCoverage, _ := statement["coverage"].(map[string]any)
			if statementCoverage["status"] != "complete" || statement["evidence_gaps"] != nil {
				t.Fatalf("statement coverage = %#v gaps = %#v", statementCoverage, statement["evidence_gaps"])
			}
			findings, _ := statement["findings"].([]any)
			if tc.verdict == "reject" {
				if len(findings) != 1 {
					t.Fatalf("findings = %#v, want exactly one", findings)
				}
				finding, _ := findings[0].(map[string]any)
				metadata, _ := finding["metadata"].(map[string]any)
				location, _ := finding["location"].(map[string]any)
				if finding["rule_id"] != "ddl.table.primary_key.require" ||
					finding["level"] != "blocker" ||
					finding["message"] != "primary key is required" ||
					finding["statement_kind"] != "ddl" ||
					location["line"] != float64(1) || location["column"] != float64(1) ||
					metadata["table"] != "t" {
					t.Fatalf("finding = %#v, want the primary-key blocker on t at 1:1", finding)
				}
				if summary, _ := payload["summary"].(map[string]any); summary["blockers"] != float64(1) ||
					summary["warnings"] != float64(0) || summary["notices"] != float64(0) {
					t.Fatalf("summary = %#v, want exactly one blocker", payload["summary"])
				}
			} else if len(findings) != 0 {
				t.Fatalf("findings = %#v, want none", findings)
			}
			if unsupported, _ := payload["unsupported"].([]any); len(unsupported) != 0 {
				t.Fatalf("unsupported = %#v, want none", payload["unsupported"])
			}
			if payload["global_findings"] != nil || payload["diagnostics"] != nil {
				t.Fatalf("global_findings/diagnostics = %#v/%#v, want none",
					payload["global_findings"], payload["diagnostics"])
			}
		})
	}
}
