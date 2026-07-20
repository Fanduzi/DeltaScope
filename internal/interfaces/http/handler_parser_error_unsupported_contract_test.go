package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerAuditParserErrorUnsupportedContractMySQL(t *testing.T) {
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

	errPayload, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error envelope, got %#v", payload)
	}
	message, _ := errPayload["message"].(string)
	if message == "" {
		t.Fatalf("expected non-empty error message, got %#v", errPayload)
	}

	lower := strings.ToLower(message)
	if !strings.Contains(lower, "not audited") && !strings.Contains(lower, "parse") && !strings.Contains(lower, "invalid") {
		t.Fatalf("expected not-audited, parse, or invalid diagnostic in error message, got %q", message)
	}
	if !strings.Contains(lower, "audit") && !strings.Contains(lower, "invalid") {
		t.Fatalf("expected audit or invalid semantics in error message, got %q", message)
	}
	if strings.Contains(message, "secret_body_value") {
		t.Fatalf("HTTP response leaked forbidden payload in %q", message)
	}
	if strings.Contains(lower, "near ") {
		t.Fatalf("HTTP response leaked raw parser fragment in %q", message)
	}

	// Verify no findings payload.
	if statements, ok := payload["statements"].([]any); ok && len(statements) != 0 {
		t.Fatalf("parser-error SQL must not produce statement findings: %#v", statements)
	}
}

func TestHandlerAuditParserErrorUnsupportedContractTiDB(t *testing.T) {
	t.Parallel()

	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"ALTER TABLE users LOCALITY = 'region=us-east-1'","dialect":"tidb"}`))
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

	errPayload, ok := payload["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error envelope, got %#v", payload)
	}
	message, _ := errPayload["message"].(string)
	if message == "" {
		t.Fatalf("expected non-empty error message, got %#v", errPayload)
	}

	lower := strings.ToLower(message)
	if !strings.Contains(lower, "not audited") && !strings.Contains(lower, "parse") && !strings.Contains(lower, "invalid") {
		t.Fatalf("expected not-audited, parse, or invalid diagnostic in error message, got %q", message)
	}
	if strings.Contains(message, "us-east-1") {
		t.Fatalf("HTTP response leaked forbidden payload in %q", message)
	}
	if strings.Contains(lower, "near ") {
		t.Fatalf("HTTP response leaked raw parser fragment in %q", message)
	}
}
