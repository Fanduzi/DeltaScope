//go:build postgresql

// Package httpapi verifies PostgreSQL offline impact rendering.
// input: HTTP audit requests selecting PostgreSQL without a connection
// output: public impact fields in the HTTP JSON response
// pos: PostgreSQL-tagged HTTP adapter regression coverage
// note: if this file changes, update this header and module README.md.
package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlerAuditPostgreSQLOfflineIDEqualityImpact(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"delete from users where id = 42","dialect":"postgresql"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	statements, ok := decoded["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one statement, got %#v", decoded["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	impact, ok := statement["impact"].(map[string]any)
	if !ok {
		t.Fatalf("expected impact object, got %#v", statement["impact"])
	}
	if impact["estimated_rows"] != float64(1) || impact["risk_level"] != "low" || impact["confidence"] != "high" || impact["source"] != "shape" {
		t.Fatalf("unexpected impact object: %#v", impact)
	}
	reasonCodes, ok := impact["reason_codes"].([]any)
	if !ok || len(reasonCodes) != 1 || reasonCodes[0] != "pk_equality" {
		t.Fatalf("reason codes = %#v, want [pk_equality]", impact["reason_codes"])
	}
}
