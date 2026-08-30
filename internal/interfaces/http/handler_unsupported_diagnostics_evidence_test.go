// Package httpapi verifies HTTP diagnostic result contracts.
// input: offline HTTP audit requests containing parser-error and valid SQL statements
// output: bounded error envelopes that preserve review-floored partial audit results and diagnostic evidence
// pos: HTTP interface parser-diagnostic and partial-result regression coverage
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

func TestUnsupportedDiagnosticsEvidenceHTTPParserError(t *testing.T) {
	t.Parallel()

	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"CREATE FUNCTION hello() RETURNS VARCHAR(20) RETURN 'secret_body_value'","dialect":"mysql"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Fatalf("expected non-200 status for parser-error SQL, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if diags, ok := payload["diagnostics"].([]any); ok && len(diags) > 0 {
		first, ok := diags[0].(map[string]any)
		if !ok {
			t.Fatalf("expected diagnostic object, got %T", diags[0])
		}
		classification, _ := first["classification"].(string)
		if classification != "parser_error" {
			t.Fatalf("expected classification parser_error, got %q", classification)
		}
		reason, _ := first["reason"].(string)
		if !strings.Contains(strings.ToLower(reason), "not audited") {
			t.Fatalf("expected reason containing 'not audited', got %q", reason)
		}
		actionHint, _ := first["action_hint"].(string)
		if !strings.Contains(strings.ToLower(actionHint), "verify the selected dialect") {
			t.Fatalf("expected action_hint, got %q", actionHint)
		}
		audited, _ := first["audited"].(bool)
		if audited {
			t.Fatal("expected audited=false")
		}
		dialect, _ := first["dialect"].(string)
		if dialect != "mysql" {
			t.Fatalf("expected dialect mysql, got %q", dialect)
		}
		guidanceCode, _ := first["guidance_code"].(string)
		if guidanceCode != "parser_upgrade_candidate" {
			t.Fatalf("expected guidance_code parser_upgrade_candidate, got %q", guidanceCode)
		}
		evidenceRef, _ := first["evidence_ref"].(string)
		if !strings.HasPrefix(evidenceRef, "https://github.com/Fanduzi/DeltaScope/") {
			t.Fatalf("expected GitHub evidence_ref URL, got %q", evidenceRef)
		}
	} else {
		errPayload, ok := payload["error"].(map[string]any)
		if !ok {
			t.Fatalf("expected error envelope or diagnostics, got %#v", payload)
		}
		message, _ := errPayload["message"].(string)
		lower := strings.ToLower(message)
		if !strings.Contains(lower, "not audited") && !strings.Contains(lower, "parser_error") {
			t.Fatalf("expected not-audited or parser_error in error message, got %q", message)
		}
		if !strings.Contains(lower, "action_hint") && !strings.Contains(lower, "verify the selected dialect") {
			t.Fatalf("expected action_hint or verify-the-dialect guidance in error message, got %q", message)
		}
		if !strings.Contains(lower, "classification") && !strings.Contains(lower, "parser_error") {
			t.Fatalf("expected classification field or parser_error mention, got %q", message)
		}
	}

	body := strings.ToLower(rec.Body.String())
	if strings.Contains(body, "secret_body_value") {
		t.Fatalf("HTTP response leaked forbidden payload in %q", rec.Body.String())
	}
	if strings.Contains(body, "near ") {
		t.Fatalf("HTTP response leaked raw parser fragment in %q", rec.Body.String())
	}
}

func TestHTTPParserErrorResponsePreservesPartialAuditResult(t *testing.T) {
	t.Parallel()

	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"ALTER TABLE users DROP COLUMN email;\nCREATE INDEX CONCURRENTLY idx_users_name ON users(name);\nDELETE FROM users WHERE id = 1;","dialect":"mysql"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Fatalf("expected parser-error HTTP status, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["verdict"] != "review" {
		t.Fatalf("expected parser-error review floor, got %s", rec.Body.String())
	}
	summary, _ := payload["summary"].(map[string]any)
	if summary["statements"] != float64(2) {
		t.Fatalf("expected two audited statements, got %s", rec.Body.String())
	}
	statements, _ := payload["statements"].([]any)
	if len(statements) != 2 || !strings.Contains(rec.Body.String(), "ddl.alter.drop_column.notice") {
		t.Fatalf("expected valid statements and DROP COLUMN notice, got %s", rec.Body.String())
	}
	diagnostics, _ := payload["diagnostics"].([]any)
	if len(diagnostics) != 1 || diagnostics[0].(map[string]any)["line"] != float64(2) {
		t.Fatalf("expected one line-2 parser diagnostic, got %s", rec.Body.String())
	}
	runContext, _ := payload["context"].(map[string]any)
	if runContext["mode"] != "offline" || runContext["dialect"] != "mysql" {
		t.Fatalf("expected normal offline context on partial result, got %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "idx_users_name") {
		t.Fatalf("HTTP diagnostic response leaked invalid SQL text: %s", rec.Body.String())
	}
}
