// Package httpapi verifies the T06-A8 comment fidelity upgrade at the HTTP
// seam — an empty column COMMENT and a rune-counted table comment are
// ordinary audit payloads at HTTP 200, never transport errors.
// input: POST /v1/audit bodies carrying declared comments under isolated policies
// output: HTTP 200 with reject/pass audit payloads
// pos: HTTP transport contract test for issue #85 T06-A8
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

// t06A8HTTPPolicy writes an isolated policy for the comment representatives.
func t06A8HTTPPolicy(t *testing.T, enabled map[string]string) string {
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

func postT06A8Audit(t *testing.T, policy, sql string) (int, map[string]any) {
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

// TestHandlerT06A8ColumnEmptyComment posts an explicitly empty column COMMENT
// — a policy reject stays a populated 200 audit result.
func TestHandlerT06A8ColumnEmptyComment(t *testing.T) {
	policy := t06A8HTTPPolicy(t, map[string]string{
		"ddl.column.comment.require": "      required: true\n",
	})
	code, payload := postT06A8Audit(t, policy, "CREATE TABLE t (c INT COMMENT '');")
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
	if len(findings) != 1 || findings[0].(map[string]any)["rule_id"] != "ddl.column.comment.require" {
		t.Fatalf("findings = %#v, want one column comment require blocker", findings)
	}
}

// TestHandlerT06A8TableCommentRuneLength posts a 3-code-point comment under
// limit=8 — the rune count passes where a byte count would have rejected.
func TestHandlerT06A8TableCommentRuneLength(t *testing.T) {
	policy := t06A8HTTPPolicy(t, map[string]string{
		"ddl.table.comment.max_length": "      limit: 8\n",
	})
	code, payload := postT06A8Audit(t, policy, "CREATE TABLE t (c INT) COMMENT='中文注';")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %#v", code, payload)
	}
	if payload["verdict"] != "pass" {
		t.Fatalf("verdict = %#v, want pass", payload["verdict"])
	}
}
