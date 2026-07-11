// Package httpapi verifies HTTP query access request binding and response mapping.
// input: synthetic HTTP requests against the DeltaScope HTTP adapter for query access analysis
// output: focused coverage for query access analysis success, error handling, and JSON response shape
// pos: interface adapter test coverage for the HTTP query access surface
// note: if this file changes, update this header and module README.md.
package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

func TestHandlerQueryAccessReturnsJSONResult(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/query-access/analyze", bytes.NewBufferString(`{"sql":"SELECT id, name FROM users WHERE id = 1","dialect":"mysql","mode":"strict"}`))
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
	if payload["dialect"] != "mysql" {
		t.Fatalf("expected mysql dialect, got %#v", payload["dialect"])
	}
	if payload["mode"] != "strict" {
		t.Fatalf("expected strict mode, got %#v", payload["mode"])
	}
	if payload["read_classification"] != "read_only" {
		t.Fatalf("expected read_only classification, got %#v", payload["read_classification"])
	}
	if payload["admission"] != "admissible" {
		t.Fatalf("expected admissible admission, got %#v", payload["admission"])
	}
}

func TestHandlerQueryAccessRejectsInvalidJSON(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/query-access/analyze", bytes.NewBufferString(`{"sql":`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"invalid_json"`)) {
		t.Fatalf("expected invalid_json code, got %q", rec.Body.String())
	}
}

func TestHandlerQueryAccessRejectsEmptySQL(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/query-access/analyze", bytes.NewBufferString(`{"sql":"   "}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"bad_request"`)) {
		t.Fatalf("expected bad_request code, got %q", rec.Body.String())
	}
}

func TestHandlerQueryAccessDefaultDialectMySQL(t *testing.T) {
	var capturedRequest deltascope.QueryAccessRequest
	previous := analyzeQueryAccess
	analyzeQueryAccess = func(_ context.Context, req deltascope.QueryAccessRequest) (*deltascope.QueryAccessResult, error) {
		capturedRequest = req
		return &deltascope.QueryAccessResult{
			Dialect:            string(req.Dialect),
			Mode:               req.Mode,
			ReadClassification: deltascope.QueryAccessReadOnly,
			Admission:          deltascope.QueryAccessAdmissible,
		}, nil
	}
	t.Cleanup(func() { analyzeQueryAccess = previous })

	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/query-access/analyze", bytes.NewBufferString(`{"sql":"SELECT 1"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if capturedRequest.Dialect != deltascope.DialectMySQL {
		t.Fatalf("expected default mysql dialect, got %q", capturedRequest.Dialect)
	}
}

func TestHandlerQueryAccessDefaultModeStrict(t *testing.T) {
	var capturedRequest deltascope.QueryAccessRequest
	previous := analyzeQueryAccess
	analyzeQueryAccess = func(_ context.Context, req deltascope.QueryAccessRequest) (*deltascope.QueryAccessResult, error) {
		capturedRequest = req
		return &deltascope.QueryAccessResult{
			Dialect:            string(req.Dialect),
			Mode:               req.Mode,
			ReadClassification: deltascope.QueryAccessReadOnly,
			Admission:          deltascope.QueryAccessAdmissible,
		}, nil
	}
	t.Cleanup(func() { analyzeQueryAccess = previous })

	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/query-access/analyze", bytes.NewBufferString(`{"sql":"SELECT 1"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if capturedRequest.Mode != deltascope.QueryAccessModeStrict {
		t.Fatalf("expected default strict mode, got %q", capturedRequest.Mode)
	}
}

func TestHandlerQueryAccessNoAuditFieldLeakage(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/query-access/analyze", bytes.NewBufferString(`{"sql":"SELECT id FROM users","dialect":"mysql"}`))
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

	forbiddenFields := []string{"verdict", "summary", "statements", "global_findings", "findings", "level", "rule_id", "context"}
	for _, field := range forbiddenFields {
		if _, ok := payload[field]; ok {
			t.Errorf("forbidden field %q found in query access HTTP response", field)
		}
	}
}

func TestHandlerQueryAccessCapabilitiesIncludesEndpoint(t *testing.T) {
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

	endpoints, ok := payload["endpoints"].([]any)
	if !ok {
		t.Fatalf("expected endpoints array, got %#v", payload["endpoints"])
	}
	found := false
	for _, ep := range endpoints {
		if ep == "POST /v1/query-access/analyze" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected POST /v1/query-access/analyze in endpoints, got %#v", endpoints)
	}
}
