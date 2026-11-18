//go:build postgresql

package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/pkg/deltascope"
)

func TestHandlerPGColumnTuning(t *testing.T) {
	for _, tc := range []struct {
		sql, id, verdict string
		unknown          bool
	}{
		{"ALTER COLUMN id SET STATISTICS -2", "ddl.pg.alter.column.statistics.target.valid", "reject", false},
		{"ALTER COLUMN id SET STATISTICS 10001", "ddl.pg.alter.column.statistics.clamp.warn", "review", false},
		{"ALTER COLUMN id SET(n_distinct=-1.1)", "ddl.pg.alter.column.options.valid", "reject", false},
		{"DROP COLUMN IF EXISTS missing", "", "pass", false},
		{"DROP COLUMN missing", "ddl.alter.drop_column.exists.require", "reject", false},
		{"DROP COLUMN IF EXISTS missing", "ddl.pg.alter.drop_column.advisory", "review", true},
	} {
		catalog := &deltascope.PGColumnCatalog{Ordinary: true, Columns: []deltascope.PGColumnState{{Name: "id"}}}
		if tc.unknown {
			catalog = nil
		}
		h, err := NewHandler("", "test", WithAuditFunc(func(ctx context.Context, r deltascope.Request) (deltascope.Result, error) {
			r.MetadataProvider = pgColumnGenerationHTTPProvider{catalog}
			return deltascope.Audit(ctx, r)
		}))
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(map[string]string{"sql": "ALTER TABLE app.t " + tc.sql, "dialect": "postgresql", "schema": "app"})
		req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		var body struct {
			Verdict  string `json:"verdict"`
			Coverage struct {
				Status string `json:"status"`
			} `json:"coverage"`
			Statements []struct {
				Findings []struct {
					ID       string         `json:"rule_id"`
					Metadata map[string]any `json:"metadata"`
				} `json:"findings"`
				Gaps []struct {
					ID string `json:"rule_id"`
				} `json:"evidence_gaps"`
			} `json:"statements"`
		}
		if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &body) != nil || body.Verdict != tc.verdict || len(body.Statements) != 1 {
			t.Fatal(rec.Body.String())
		}
		found := false
		for _, f := range body.Statements[0].Findings {
			if f.ID == tc.id {
				found = true
			}
			b, _ := json.Marshal(f.Metadata)
			if strings.Contains(string(b), "10001") || strings.Contains(string(b), "-1.1") || strings.Contains(string(b), "PGColumns") {
				t.Fatal("private facts leaked", string(b))
			}
			if tc.id == "" && (strings.HasPrefix(f.ID, "ddl.pg.alter.column.") || f.ID == "ddl.pg.alter.drop_column.advisory") {
				t.Fatal("no-op finding", f)
			}
		}
		if found != (tc.id != "") || tc.unknown && (body.Coverage.Status != "unverified" || len(body.Statements[0].Gaps) != 1) {
			t.Fatal(rec.Body.String())
		}
	}
}
