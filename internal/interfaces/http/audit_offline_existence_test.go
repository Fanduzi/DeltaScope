// Package httpapi verifies HTTP offline existence caveats on audit context.
// input: offline ALTER DROP COLUMN HTTP audit requests
// output: pass verdict plus JSON context.note / context.unproven matching the CLI caveat
// pos: HTTP contract coverage for issue #28
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

func TestHandlerOfflineDropColumnStatesExistenceNotChecked(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"alter table users drop column not_a_col"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["verdict"] != "pass" {
		t.Fatalf("expected verdict pass, got %#v", payload["verdict"])
	}
	assertJSONContextExistenceCaveat(t, payload)
	if strings.Contains(rec.Body.String(), "existing column") {
		t.Fatalf("HTTP notice must not claim the column exists, got %s", rec.Body.String())
	}
}

func TestHandlerCapabilitiesListsOfflineExistenceContextFields(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/capabilities", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	fields, ok := payload["context_fields"].([]any)
	if !ok {
		t.Fatalf("expected context_fields array, got %#v", payload["context_fields"])
	}
	got := make([]string, 0, len(fields))
	for _, item := range fields {
		text, _ := item.(string)
		got = append(got, text)
	}
	joined := strings.Join(got, ",")
	if !strings.Contains(joined, "note") || !strings.Contains(joined, "unproven") {
		t.Fatalf("expected context_fields to advertise note and unproven, got %#v", fields)
	}
}

func assertJSONContextExistenceCaveat(t *testing.T, payload map[string]any) {
	t.Helper()
	contextValue, ok := payload["context"].(map[string]any)
	if !ok {
		t.Fatalf("expected context object, got %#v", payload["context"])
	}
	if contextValue["mode"] != "offline" {
		t.Fatalf("expected offline mode, got %#v", contextValue)
	}
	if contextValue["note"] != "existence not checked (no database connection)" {
		t.Fatalf("expected context.note existence caveat, got %#v", contextValue["note"])
	}
	unproven, ok := contextValue["unproven"].([]any)
	if !ok {
		t.Fatalf("expected context.unproven array, got %#v", contextValue["unproven"])
	}
	got := make([]string, 0, len(unproven))
	for _, item := range unproven {
		text, _ := item.(string)
		got = append(got, text)
	}
	if strings.Join(got, ",") != "column_exists,table_exists" {
		t.Fatalf("expected unproven [column_exists table_exists], got %#v", unproven)
	}
}
