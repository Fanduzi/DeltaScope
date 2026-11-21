//go:build postgresql

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

type pgConstraintTargetHTTPProvider struct {
	members []deltascope.PGConstraintState
}

func (p pgConstraintTargetHTTPProvider) LoadInstanceFacts(context.Context, deltascope.Dialect, string) (*deltascope.InstanceFacts, error) {
	return &deltascope.InstanceFacts{Version: "PostgreSQL 17.7"}, nil
}
func (p pgConstraintTargetHTTPProvider) LoadTableSnapshot(_ context.Context, _ deltascope.Dialect, schema, table string) (*deltascope.TableSnapshot, error) {
	kind := "r"
	if len(p.members) == 1 && p.members[0].PartitionCheckHasChildrenKnown {
		kind = "p"
	}
	return &deltascope.TableSnapshot{Schema: schema, Exists: true, Table: &deltascope.Table{Name: table}, PGConstraints: p.members, PGRelation: deltascope.PGRelationState{Name: table, Kind: kind}, PGColumns: &deltascope.PGColumnCatalog{Ordinary: true, Columns: []deltascope.PGColumnState{{Name: "id"}}}}, nil
}

func TestHandlerPGConstraintTarget(t *testing.T) {
	for _, tc := range []struct {
		sql, id, verdict string
		members          []deltascope.PGConstraintState
	}{
		{`ALTER TABLE app.t ADD CONSTRAINT " Exact Check " CHECK (1 > 0) NOT VALID`, "ddl.pg.alter.add_check.name.available", "reject", []deltascope.PGConstraintState{{Name: " Exact Check ", Type: "c", Validated: true}}},
		{`ALTER TABLE app.t ADD CHECK (id > (SELECT 0)) NOT VALID`, "ddl.pg.alter.add_check.name.available", "reject", nil},
		{`ALTER TABLE app.t ADD CHECK ('invalid') NOT VALID`, "ddl.pg.alter.add_check.name.available", "reject", nil},
		{`ALTER TABLE app.t ADD UNIQUE (id, id)`, "ddl.pg.alter.constraint_columns.valid", "reject", nil},
		{`ALTER TABLE app.t ADD FOREIGN KEY (id) REFERENCES app.t ("Missing") NOT VALID`, "ddl.pg.alter.add_foreign_key.targets.valid", "reject", []deltascope.PGConstraintState{}},
		{"ALTER TABLE ONLY app.t VALIDATE CONSTRAINT c", "ddl.pg.alter.validate_constraint.target.require", "reject", []deltascope.PGConstraintState{{Name: "c", Type: "c", CheckValidationChildrenKnown: true, CheckValidationHasChildren: true}}},
		{"ALTER TABLE app.t VALIDATE CONSTRAINT c", "ddl.pg.alter.validate_constraint.target.require", "reject", []deltascope.PGConstraintState{}},
		{"ALTER TABLE app.t ALTER CONSTRAINT c DEFERRABLE", "ddl.pg.alter.constraint_deferrability.target.require", "reject", []deltascope.PGConstraintState{}},
		{"ALTER TABLE app.t DROP CONSTRAINT IF EXISTS c CASCADE", "ddl.pg.alter.drop_constraint.target.require", "reject", []deltascope.PGConstraintState{{Name: "c", Type: "c", Validated: true, InheritanceKnown: true, Inherited: true}}},
		{"ALTER TABLE ONLY app.t DROP CONSTRAINT c CASCADE", "ddl.pg.alter.drop_constraint.target.require", "reject", []deltascope.PGConstraintState{{Name: "c", Type: "c", Validated: true, InheritanceKnown: true, PartitionCheckHasChildrenKnown: true, PartitionCheckHasChildren: true}}},
		{"ALTER TABLE app.t DROP CONSTRAINT c", "ddl.pg.alter.drop_constraint.target.require", "reject", []deltascope.PGConstraintState{}},
		{"ALTER TABLE app.t DROP CONSTRAINT IF EXISTS c", "", "pass", []deltascope.PGConstraintState{}},
		{"ALTER TABLE app.t VALIDATE CONSTRAINT c", "", "review", nil},
	} {
		provider := pgConstraintTargetHTTPProvider{members: tc.members}
		h, err := NewHandler("", "test", WithAuditFunc(func(ctx context.Context, r deltascope.Request) (deltascope.Result, error) {
			r.MetadataProvider = provider
			return deltascope.Audit(ctx, r)
		}))
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(map[string]string{"sql": tc.sql, "dialect": "postgresql", "schema": "app"})
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
			Verdict  string `json:"verdict"`
			Coverage struct {
				Status string `json:"status"`
			} `json:"coverage"`
			Statements []struct {
				Findings []struct {
					ID    string `json:"rule_id"`
					Level string `json:"level"`
				} `json:"findings"`
				Gaps []struct {
					ID string `json:"rule_id"`
				} `json:"evidence_gaps"`
			} `json:"statements"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Verdict != tc.verdict || len(body.Statements) != 1 {
			t.Fatalf("body=%s", rec.Body.String())
		}
		found := false
		for _, f := range body.Statements[0].Findings {
			if f.ID == tc.id {
				found = true
				if f.Level != "blocker" {
					t.Fatal(f.Level)
				}
			}
		}
		if found != (tc.id != "") {
			t.Fatal("lost shared target blocker")
		}
		if tc.verdict == "review" && (body.Coverage.Status != "unverified" || len(body.Statements[0].Gaps) != 1 || body.Statements[0].Gaps[0].ID != "ddl.pg.alter.validate_constraint.target.require") {
			t.Fatalf("lost shared gap: %s", rec.Body.String())
		}
	}
}
