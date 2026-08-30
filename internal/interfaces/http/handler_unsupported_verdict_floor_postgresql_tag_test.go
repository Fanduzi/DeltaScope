//go:build postgresql

// Package httpapi verifies HTTP unsupported-statement result contracts.
// input: offline HTTP audit requests containing structured unsupported PostgreSQL statements
// output: non-success envelopes that serialize the review-floored unsupported result
// pos: HTTP adapter regression coverage for the unsupported-statement verdict floor
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

func TestHTTPUnsupportedStatementAppliesReviewFloor(t *testing.T) {
	t.Parallel()

	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"SELECT 1","dialect":"postgresql"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Fatalf("expected non-success HTTP status for unsupported SQL, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["verdict"] != "review" {
		t.Fatalf("expected unsupported review floor, got %s", rec.Body.String())
	}
	summary, _ := payload["summary"].(map[string]any)
	if summary["statements"] != float64(0) {
		t.Fatalf("expected zero audited statements, got %s", rec.Body.String())
	}
	unsupported, _ := payload["unsupported"].([]any)
	if len(unsupported) != 1 {
		t.Fatalf("expected one unsupported detail, got %s", rec.Body.String())
	}
	item, _ := unsupported[0].(map[string]any)
	if item["feature"] != "select" {
		t.Fatalf("expected unsupported feature select, got %#v", item)
	}
	diagnostics, _ := payload["diagnostics"].([]any)
	if len(diagnostics) != 1 {
		t.Fatalf("expected one unsupported diagnostic, got %s", rec.Body.String())
	}
	diagnostic, _ := diagnostics[0].(map[string]any)
	if diagnostic["classification"] != "unsupported_statement" || diagnostic["audited"] != false {
		t.Fatalf("expected unaudited unsupported_statement diagnostic, got %#v", diagnostic)
	}
	reason, _ := diagnostic["reason"].(string)
	actionHint, _ := diagnostic["action_hint"].(string)
	if strings.Contains(reason+actionHint, "SELECT 1") {
		t.Fatalf("HTTP diagnostic leaked SQL text: %s", rec.Body.String())
	}
}
