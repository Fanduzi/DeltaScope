// Package httpapi verifies the T06-A7 table-collation upgrade at the HTTP
// seam — the declared table COLLATE is an ordinary audit result (HTTP 200),
// a policy rejection stays a populated response, never a transport error.
// input: POST /v1/audit bodies carrying declared table COLLATE clauses under isolated policies
// output: HTTP 200 with pass/reject audit payloads; no unsupported-statement diagnostics for collate
// pos: HTTP transport contract test for issue #85 T06-A7
// note: if this file changes, update this header and module README.md.
package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule/catalog"
	"github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

const t06a7HTTPRuleID = "ddl.table.collation.allowlist"

// t06a7HTTPPolicy writes an isolated policy for the table-collation reps.
func t06a7HTTPPolicy(t *testing.T, enabled map[string]string) string {
	t.Helper()
	var text strings.Builder
	text.WriteString("rules:\n")
	for _, entry := range catalog.All() {
		if params, keep := enabled[entry.RuleID]; keep {
			fmt.Fprintf(&text, "  %q:\n    enabled: true\n    level: blocker\n", entry.RuleID)
			if params != "" {
				text.WriteString("    params:\n" + params)
			}
		} else {
			fmt.Fprintf(&text, "  %q:\n    enabled: false\n", entry.RuleID)
		}
	}
	path := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(path, []byte(text.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func postT06A7Audit(t *testing.T, policy, sql string) (int, map[string]any) {
	t.Helper()
	handler, err := NewHandler(policy, "test-build", WithAuditFunc(func(ctx context.Context, request deltascope.Request) (deltascope.Result, error) {
		return deltascope.Audit(ctx, request)
	}))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	body, err := json.Marshal(map[string]any{"sql": sql, "dialect": "mysql"})
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

// TestHandlerT06A7TableCollateAllOff is the S4 representative: the declared
// table COLLATE now yields a complete/pass payload at HTTP 200 — previously
// this path surfaced the unsupported-statement diagnostic.
func TestHandlerT06A7TableCollateAllOff(t *testing.T) {
	policy := t06a7HTTPPolicy(t, map[string]string{})
	code, payload := postT06A7Audit(t, policy, "CREATE TABLE t (c INT) COLLATE=utf8mb4_bin;")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %#v", code, payload)
	}
	if payload["verdict"] != "pass" || payload["error"] != nil {
		t.Fatalf("verdict = %#v error = %#v, want pass", payload["verdict"], payload["error"])
	}
	if coverage, _ := payload["coverage"].(map[string]any); coverage["status"] != "complete" {
		t.Fatalf("coverage = %#v, want complete", payload["coverage"])
	}
	if unsupported, _ := payload["unsupported"].([]any); len(unsupported) != 0 {
		t.Fatalf("unsupported = %#v, want none", payload["unsupported"])
	}
}

// TestHandlerT06A7TableCollateDenied is the policy representative: a denied
// collation is verdict data, not a 400.
func TestHandlerT06A7TableCollateDenied(t *testing.T) {
	policy := t06a7HTTPPolicy(t, map[string]string{
		t06a7HTTPRuleID: "      values: [utf8mb4_bin]\n      require_explicit: false\n",
	})
	code, payload := postT06A7Audit(t, policy, "CREATE TABLE t (c INT) COLLATE=utf8mb4_general_ci;")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %#v", code, payload)
	}
	if payload["verdict"] != "reject" {
		t.Fatalf("verdict = %#v, want reject", payload["verdict"])
	}
	statements, _ := payload["statements"].([]any)
	if len(statements) != 1 {
		t.Fatalf("statements = %#v, want 1", statements)
	}
	findings, _ := statements[0].(map[string]any)["findings"].([]any)
	if len(findings) != 1 || findings[0].(map[string]any)["rule_id"] != t06a7HTTPRuleID {
		t.Fatalf("findings = %#v, want one %s blocker", findings, t06a7HTTPRuleID)
	}
}
