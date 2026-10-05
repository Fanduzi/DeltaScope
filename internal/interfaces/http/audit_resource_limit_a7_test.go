// Package httpapi verifies the T05-A7 ordered-state budget at the HTTP seam.
// input: a 1025-statement MySQL /v1/audit request through the real handler
// output: HTTP 400 diagnostic envelope carrying the retained partial result and the exact bounded resource-limit projection
// pos: HTTP transport contract test for the shared ordered-state admission budget (issue #84/T05-A7)
// note: if this file changes, update this header and module README.md.
package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestHandlerT05A7OrderedStatementBudget(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	sql := strings.Repeat("SELECT 1;\n", 1025)
	body, err := json.Marshal(map[string]any{"sql": sql, "dialect": "mysql"})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	if len(body) >= 1<<20 {
		t.Fatalf("request body must stay under 1 MiB, got %d bytes", len(body))
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for the over-budget audit, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if errBody, _ := payload["error"].(map[string]any); errBody == nil || errBody["code"] != "bad_request" {
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

	if payload["verdict"] != "review" {
		t.Fatalf("verdict = %#v, want review", payload["verdict"])
	}
	if c, _ := payload["coverage"].(map[string]any); c["status"] != "incomplete" {
		t.Fatalf("coverage = %#v, want incomplete", payload["coverage"])
	}
	statements, _ := payload["statements"].([]any)
	if len(statements) != 1025 {
		t.Fatalf("statements = %d, want 1025 retained", len(statements))
	}
	for i := 0; i < 1024; i++ {
		statement, _ := statements[i].(map[string]any)
		sc, _ := statement["coverage"].(map[string]any)
		if statement["index"] != float64(i) || sc["status"] != "complete" {
			t.Fatalf("admitted statement %d = %#v, want complete", i, statement)
		}
		if statement["findings"] != nil || statement["evidence_gaps"] != nil || statement["impact"] != nil {
			t.Fatalf("admitted statement %d must not carry findings, gaps, or impact, got %#v", i, statement)
		}
	}
	if payload["global_findings"] != nil {
		t.Fatalf("global findings = %#v, want none", payload["global_findings"])
	}
	if summary, _ := payload["summary"].(map[string]any); summary["blockers"] != float64(0) || summary["warnings"] != float64(0) || summary["notices"] != float64(0) {
		t.Fatalf("finding counters = %#v, want zero", payload["summary"])
	}
	last, _ := statements[1024].(map[string]any)
	lc, _ := last["coverage"].(map[string]any)
	normalized, _ := last["normalized_sql"].(string)
	if last["index"] != float64(1024) || last["kind"] != "unknown" || strings.TrimSpace(last["raw_sql"].(string)) != "SELECT 1;" || normalized == "" {
		t.Fatalf("blocked statement identity = %#v, want index 1024 kind unknown raw SELECT 1;", last)
	}
	if lc["status"] != "incomplete" || last["findings"] != nil || last["evidence_gaps"] != nil || last["impact"] != nil {
		t.Fatalf("blocked statement = %#v, want incomplete with no findings, gaps, or impact", last)
	}
	unsupported, _ := payload["unsupported"].([]any)
	if len(unsupported) != 1 {
		t.Fatalf("unsupported entries = %d, want 1: %#v", len(unsupported), payload["unsupported"])
	}
	item, _ := unsupported[0].(map[string]any)
	if item["feature"] != "audit.resource_limit" || item["reason"] != "ordered-state statement budget exhausted" {
		t.Fatalf("unsupported entry = %#v, want the audit.resource_limit budget projection", item)
	}
	if item["index"] != float64(1024) || item["sql"] != last["raw_sql"] {
		t.Fatalf("unsupported identity = %#v, want index 1024 bound to the blocked statement", item)
	}
	wantMeta := map[string]any{
		"phase":    "ordered_state",
		"resource": "statements",
		"limit":    float64(1024),
		"consumed": float64(1024),
		"line":     float64(1025),
		"column":   float64(1),
	}
	if metadata, _ := item["metadata"].(map[string]any); !reflect.DeepEqual(metadata, wantMeta) {
		t.Fatalf("resource metadata = %#v, want %#v", item["metadata"], wantMeta)
	}
}
