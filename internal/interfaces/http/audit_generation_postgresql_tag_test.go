//go:build postgresql

package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlerPGGenerationWarningParity(t *testing.T) {
	h, err := NewHandler("", "test")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"ALTER TABLE t ADD COLUMN id bigint GENERATED ALWAYS AS IDENTITY(CYCLE)","dialect":"postgresql"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d %s", rec.Code, rec.Body.String())
	}
	var r struct {
		Verdict    string `json:"verdict"`
		Statements []struct {
			Findings []struct {
				ID    string `json:"rule_id"`
				Level string `json:"level"`
			}
		} `json:"statements"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	if r.Verdict != "review" {
		t.Fatalf("verdict=%s", r.Verdict)
	}
	for _, s := range r.Statements {
		for _, f := range s.Findings {
			if f.ID == "ddl.pg.column.identity.cycle.warn" && f.Level == "warning" {
				return
			}
		}
	}
	t.Fatal("missing shared security-independent cycling warning")
}
