//go:build postgresql

package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlerAuditPostgreSQLCreateSchemaRendersNotice(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"CREATE SCHEMA app;","dialect":"postgresql"}`))
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
	statements, ok := payload["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one statement, got %#v", payload["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	findings, ok := statement["findings"].([]any)
	if !ok || len(findings) < 1 {
		t.Fatalf("expected at least one finding, got %#v", statement["findings"])
	}

	found := false
	for _, item := range findings {
		finding, ok := item.(map[string]any)
		if !ok {
			continue
		}
		ruleID, _ := finding["rule_id"].(string)
		if ruleID == "ddl.pg.create_schema.notice" {
			found = true
		}
		for _, forbidden := range []string{"ddl.database.create.notice", "ddl.database.drop.warn"} {
			if ruleID == forbidden {
				t.Fatalf("PG HTTP audit must not emit MySQL-family database rule %q", forbidden)
			}
		}
	}
	if !found {
		t.Fatalf("expected rule_id ddl.pg.create_schema.notice, got %#v", findings)
	}
}
