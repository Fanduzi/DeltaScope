package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlerRoutineSecurityWarningParity(t *testing.T) {
	handler, err := NewHandler("", "test-build")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		sql    string
		status int
	}{
		{"CREATE PROCEDURE p() SET @x=1", http.StatusOK},
		{"CREATE FUNCTION f() RETURNS INT RETURN 1", http.StatusBadRequest},
	} {
		raw, err := json.Marshal(map[string]string{"sql": tc.sql, "dialect": "mysql"})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != tc.status {
			t.Fatalf("status=%d: %s", rec.Code, rec.Body.String())
		}
		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload["verdict"] != "review" {
			t.Fatalf("verdict=%v", payload["verdict"])
		}
		stmts := payload["statements"].([]any)
		found := false
		for _, value := range stmts[0].(map[string]any)["findings"].([]any) {
			f := value.(map[string]any)
			if f["rule_id"] == "ddl.routine.security.definer.warn" {
				found = true
				if f["level"] != "warning" || f["metadata"].(map[string]any)["security_source"] != "create_default" {
					t.Fatalf("finding: %+v", f)
				}
			}
		}
		if !found {
			t.Fatal("missing security warning")
		}
	}
}
