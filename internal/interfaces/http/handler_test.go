// Package httpapi verifies HTTP request binding and response mapping.
// input: synthetic HTTP requests against the DeltaScope HTTP adapter
// output: focused coverage for health, version, audit success, and structured error responses
// pos: interface adapter test coverage for the HTTP service surface
// note: if this file changes, update this header and module README.md.
package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

func TestHandlerHealthz(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() == "" {
		t.Fatalf("expected health body")
	}
}

func TestHandlerVersion(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("test-build")) {
		t.Fatalf("expected version payload, got %q", rec.Body.String())
	}
}

func TestHandlerAuditReturnsJSONResult(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"delete from users","dialect":"mysql"}`))
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
	if payload["verdict"] == "" {
		t.Fatalf("expected verdict in response, got %+v", payload)
	}
}

func TestHandlerAuditReturnsFindingExplanationFields(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"delete from users","dialect":"mysql"}`))
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

	finding, explanation := firstFindingByRuleID(t, payload, "dml.where.require")
	if finding["rule_id"] != "dml.where.require" {
		t.Fatalf("expected canonical where-rule finding, got %#v", finding["rule_id"])
	}
	assertExplanationFieldString(t, explanation, "summary")
	assertExplanationFieldString(t, explanation, "why")
	assertExplanationFieldString(t, explanation, "risk")
	assertExplanationFieldString(t, explanation, "suggestion")
}

func TestWriteJSONSerializesFindingExplanationMetadataFields(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, deltascope.Result{
		Statements: []deltascope.StatementResult{{
			Index: 0,
			Kind:  "ddl",
			Findings: []deltascope.Finding{{
				RuleID:  "ddl.table.exists.create.forbid",
				Level:   deltascope.LevelBlocker,
				Message: "table already exists",
				Explanation: &deltascope.FindingExplanation{
					Summary:    "Forbid create existing table",
					Why:        "Live metadata says the target table already exists.",
					Risk:       "Re-running the statement can fail or mask drift.",
					Suggestion: "Use IF NOT EXISTS or reconcile with the live schema first.",
					Metadata: &deltascope.ExplanationMetadata{
						Status: "limited",
						Note:   "metadata unavailable",
					},
				},
			}},
		}},
	})

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	_, explanation := firstFindingByRuleID(t, payload, "ddl.table.exists.create.forbid")
	metadata, ok := explanation["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("expected explanation metadata object, got %#v", explanation["metadata"])
	}
	assertExplanationFieldString(t, metadata, "status")
	assertExplanationFieldString(t, metadata, "note")
}

func TestWriteJSONOmitsFindingExplanationWhenAbsent(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, deltascope.Result{
		Statements: []deltascope.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Findings: []deltascope.Finding{{
				RuleID:  "custom.rule",
				Level:   deltascope.LevelWarning,
				Message: "custom finding",
			}},
		}},
	})

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	finding := firstFindingWithoutExplanationByRuleID(t, payload, "custom.rule")
	if _, ok := finding["explanation"]; ok {
		t.Fatalf("expected explanation to be omitted, got %#v", finding["explanation"])
	}
}

func TestHandlerAuditRejectsInvalidJSON(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"invalid_json"`)) {
		t.Fatalf("expected invalid_json code, got %q", rec.Body.String())
	}
}

func TestHandlerAuditRejectsEmptySQL(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"   "}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"bad_request"`)) {
		t.Fatalf("expected bad_request code, got %q", rec.Body.String())
	}
}

func TestHandlerAuditRejectsOversizedRequestBody(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	oversizedSQL := strings.Repeat("a", 1<<20)
	body := bytes.NewBufferString(`{"sql":"` + oversizedSQL + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for oversized body, got %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"invalid_json"`)) {
		t.Fatalf("expected invalid_json code, got %q", rec.Body.String())
	}
}

func TestNewHandlerRejectsInvalidConfigPath(t *testing.T) {
	if _, err := NewHandler("/tmp/deltascope-missing-config.yaml", "test-build"); err == nil {
		t.Fatalf("expected invalid config path to fail")
	}
}

func firstFindingByRuleID(t *testing.T, payload map[string]any, ruleID string) (map[string]any, map[string]any) {
	t.Helper()
	statements, ok := payload["statements"].([]any)
	if !ok {
		t.Fatalf("expected statements array, got %#v", payload["statements"])
	}
	for _, rawStatement := range statements {
		statement, ok := rawStatement.(map[string]any)
		if !ok {
			continue
		}
		findings, ok := statement["findings"].([]any)
		if !ok {
			continue
		}
		for _, rawFinding := range findings {
			finding, ok := rawFinding.(map[string]any)
			if !ok {
				continue
			}
			if finding["rule_id"] != ruleID {
				continue
			}
			explanation, ok := finding["explanation"].(map[string]any)
			if !ok {
				t.Fatalf("expected explanation on rule %q, got %#v", ruleID, finding["explanation"])
			}
			return finding, explanation
		}
	}
	t.Fatalf("expected finding for rule %q, got %#v", ruleID, payload)
	return nil, nil
}

func firstFindingWithoutExplanationByRuleID(t *testing.T, payload map[string]any, ruleID string) map[string]any {
	t.Helper()
	statements, ok := payload["statements"].([]any)
	if !ok {
		t.Fatalf("expected statements array, got %#v", payload["statements"])
	}
	for _, rawStatement := range statements {
		statement, ok := rawStatement.(map[string]any)
		if !ok {
			continue
		}
		findings, ok := statement["findings"].([]any)
		if !ok {
			continue
		}
		for _, rawFinding := range findings {
			finding, ok := rawFinding.(map[string]any)
			if !ok {
				continue
			}
			if finding["rule_id"] == ruleID {
				return finding
			}
		}
	}
	t.Fatalf("expected finding for rule %q, got %#v", ruleID, payload)
	return nil
}

func assertExplanationFieldString(t *testing.T, object map[string]any, key string) string {
	t.Helper()
	value, ok := object[key].(string)
	if !ok || value == "" {
		t.Fatalf("expected non-empty %q field, got %#v", key, object[key])
	}
	return value
}
