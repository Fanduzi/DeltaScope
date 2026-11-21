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
	for _, tc := range []struct {
		sql          string
		buildWarning bool
	}{
		{"ALTER TABLE t ADD PRIMARY KEY(id)", true},
		{"ALTER TABLE t ADD PRIMARY KEY USING INDEX idx", false},
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
				} `json:"findings"`
				Gaps []struct {
					ID     string   `json:"rule_id"`
					Reason string   `json:"reason_code"`
					Facts  []string `json:"required_facts"`
				} `json:"evidence_gaps"`
			} `json:"statements"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Verdict != "review" || len(body.Statements) != 1 {
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
		if found != tc.buildWarning {
			t.Fatal("lost shared declaration-mode semantics")
		}
		gaps := body.Statements[0].Gaps
		wantRule, wantReason := "ddl.pg.alter.constraint_using_index.valid", "unproven_constraint_using_index"
		if tc.buildWarning {
			wantRule, wantReason = "ddl.pg.alter.constraint_columns.valid", "unproven_pg_constraint_columns"
		}
		if len(gaps) != 1 || gaps[0].ID != wantRule || gaps[0].Reason != wantReason || len(gaps[0].Facts) != 1 || gaps[0].Facts[0] != "instance.version" {
			t.Fatalf("unexpected constraint gaps: %+v", gaps)
		}
	}
}
