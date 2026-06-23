package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlerAuditMySQLReturningEmitsUnsupportedNotice(t *testing.T) {
	t.Parallel()
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"insert into users(id) values (1) returning id;","dialect":"mysql"}`))
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
	findings, ok := payload["global_findings"].([]any)
	if !ok {
		t.Fatalf("expected global_findings array, got %#v", payload["global_findings"])
	}
	var hasMySQLReturning, hasPostgreSQL bool
	for _, item := range findings {
		finding, ok := item.(map[string]any)
		if !ok {
			continue
		}
		switch finding["rule_id"] {
		case "dialect.mysql.returning.unsupported.notice":
			hasMySQLReturning = true
		case "dialect.postgresql.syntax.detected.notice":
			hasPostgreSQL = true
		}
	}
	if !hasMySQLReturning {
		t.Fatalf("expected mysql returning unsupported notice, got %#v", findings)
	}
	if hasPostgreSQL {
		t.Fatalf("did not expect postgresql syntax notice for mysql returning, got %#v", findings)
	}
}
