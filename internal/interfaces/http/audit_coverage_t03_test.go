// Package httpapi verifies the T03 incomplete-coverage HTTP contract.
// input: offline HTTP audit requests containing recognized-but-unsupported DDL
// output: HTTP 400 envelope with the partial audit result and coverage fields
// pos: HTTP transport contract tests for incomplete audit results (issue #82)
// note: if this file changes, update this header and module README.md.
package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Recognized-but-unsupported statements return HTTP 400 with the partial audit
// body: retained statements, per-statement coverage, and bounded unsupported
// evidence.
func TestHandlerT03IncompleteCoverageReturnsBadRequest(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"CREATE SEQUENCE golden_seq START WITH 1; ALTER TABLE t ADD COLUMN c INT;","dialect":"mysql"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for incomplete coverage, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["verdict"] != "review" {
		t.Fatalf("expected verdict review, got %#v", payload["verdict"])
	}
	if c, _ := payload["coverage"].(map[string]any); c["status"] != "incomplete" {
		t.Fatalf("expected aggregate coverage incomplete, got %#v", payload["coverage"])
	}
	stmts, _ := payload["statements"].([]any)
	if len(stmts) != 2 {
		t.Fatalf("expected 2 retained statements, got %#v", payload["statements"])
	}
	unsupported, _ := payload["unsupported"].([]any)
	if len(unsupported) != 1 {
		t.Fatalf("expected 1 unsupported detail, got %#v", payload["unsupported"])
	}
	if detail, _ := unsupported[0].(map[string]any); detail["feature"] != "create_sequence" {
		t.Fatalf("expected create_sequence feature, got %#v", detail["feature"])
	}
}
