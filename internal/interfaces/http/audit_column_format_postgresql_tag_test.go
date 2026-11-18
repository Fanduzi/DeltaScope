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

func TestHandlerPGColumnFormat(t *testing.T) {
	for _, tc := range []struct{ clause, storage, id, verdict string }{
		{"ALTER COLUMN id SET STORAGE EXTERNAL", "p", "ddl.pg.alter.column.storage.valid", "reject"},
		{"ALTER COLUMN id SET STORAGE DEFAULT", "p", "", "pass"},
		{"ALTER COLUMN id SET COMPRESSION pglz", "p", "ddl.pg.alter.column.compression.valid", "reject"},
		{"ALTER COLUMN id SET COMPRESSION pglz", "x", "", "pass"},
		{"ALTER COLUMN id SET COMPRESSION default", "p", "", "pass"},
		{`ALTER COLUMN id SET COMPRESSION "private_method_payload"`, "x", "ddl.pg.alter.column.compression.valid", "reject"},
		{"ALTER COLUMN id SET COMPRESSION lz4", "x", "", "review"},
		{"ALTER COLUMN ctid SET STORAGE PLAIN", "p", "ddl.pg.alter.column.storage.valid", "reject"},
		{"ALTER COLUMN ctid SET COMPRESSION default", "p", "ddl.pg.alter.column.compression.valid", "reject"},
		{"ALTER COLUMN ctid SET STATISTICS DEFAULT", "p", "ddl.pg.alter.column.statistics.target.valid", "reject"},
		{"DROP COLUMN IF EXISTS ctid", "p", "ddl.alter.drop_column.exists.require", "reject"},
	} {
		catalog := &deltascope.PGColumnCatalog{Ordinary: true, Columns: []deltascope.PGColumnState{{Name: "id", TypeStorage: tc.storage}}, SystemColumns: []deltascope.PGColumnState{{Name: "ctid", System: true, TypeStorage: "p"}}}
		h, err := NewHandler("", "test", WithAuditFunc(func(ctx context.Context, r deltascope.Request) (deltascope.Result, error) {
			r.MetadataProvider = pgColumnGenerationHTTPProvider{catalog}
			return deltascope.Audit(ctx, r)
		}))
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(map[string]string{"sql": "ALTER TABLE app.t " + tc.clause, "dialect": "postgresql", "schema": "app"})
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
			if strings.Contains(tc.clause, "ctid") && f.ID == "ddl.pg.alter.drop_column.advisory" {
				t.Fatal("system deletion advice", f)
			}
			b, _ := json.Marshal(f.Metadata)
			if strings.Contains(string(b), "private_method_payload") || strings.Contains(string(b), "PGColumns") {
				t.Fatal("private facts leaked", string(b))
			}
			if tc.id == "" && strings.HasPrefix(f.ID, "ddl.pg.alter.column.") {
				t.Fatal("unexpected format finding", f)
			}
		}
		if found != (tc.id != "") || tc.verdict == "review" && (body.Coverage.Status != "unverified" || len(body.Statements[0].Gaps) != 1) {
			t.Fatal(rec.Body.String())
		}
	}
}
