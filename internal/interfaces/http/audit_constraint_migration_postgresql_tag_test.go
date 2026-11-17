//go:build postgresql

package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlerPGConstraintMigration(t *testing.T) {
	h, err := NewHandler("", "test")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ sql, verdict string }{
		{"ALTER TABLE t ADD PRIMARY KEY(id)", "review"},
		{"ALTER TABLE t ADD PRIMARY KEY USING INDEX idx", "pass"},
	} {
		raw, err := json.Marshal(map[string]string{"sql": tc.sql, "dialect": "postgresql"})
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		var body struct {
			Verdict    string `json:"verdict"`
			Statements []struct {
				Findings []struct {
					ID    string `json:"rule_id"`
					Level string `json:"level"`
				}
			} `json:"statements"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Verdict != tc.verdict {
			t.Fatalf("verdict=%s", body.Verdict)
		}
		found := false
		for _, s := range body.Statements {
			for _, f := range s.Findings {
				if f.ID == "ddl.pg.alter.add_primary_key.concurrent_index.warn" && f.Level == "warning" {
					found = true
				}
			}
		}
		if found != (tc.verdict == "review") {
			t.Fatal("lost shared declaration-mode semantics")
		}
	}
}
