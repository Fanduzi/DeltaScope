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

type pgColumnGenerationHTTPProvider struct{ catalog *deltascope.PGColumnCatalog }

func (p pgColumnGenerationHTTPProvider) LoadInstanceFacts(context.Context, deltascope.Dialect, string) (*deltascope.InstanceFacts, error) {
	return &deltascope.InstanceFacts{Version: "PostgreSQL 17.7"}, nil
}
func (p pgColumnGenerationHTTPProvider) LoadTableSnapshot(_ context.Context, _ deltascope.Dialect, schema, table string) (*deltascope.TableSnapshot, error) {
	return &deltascope.TableSnapshot{Schema: schema, Exists: true, Table: &deltascope.Table{Name: table}, PGColumns: p.catalog, PGRelation: deltascope.PGRelationState{Name: table, Kind: "r"}}, nil
}

func TestHandlerPGColumnGeneration(t *testing.T) {
	v := func(n int64) *int64 { return &n }
	cycle := false
	identity := deltascope.PGColumnState{Name: "id", Identity: "d", TypeWidth: 64, NotNull: true, IdentitySequence: &deltascope.PGIdentitySequenceState{TypeWidth: 16, Definition: deltascope.SequenceDefinition{Start: v(1), Increment: v(1), MinValue: v(1), MaxValue: v(10), CacheValue: v(1), Cycle: &cycle}}}
	for _, tc := range []struct {
		suffix, id, verdict string
		member              deltascope.PGColumnState
		unknown             bool
	}{
		{"SET DEFAULT COALESCE(id, 2)", "ddl.pg.alter.column.generation.transition.valid", "reject", deltascope.PGColumnState{Name: "id"}, false},
		{"SET DEFAULT 1", "ddl.pg.alter.column.generation.transition.valid", "reject", deltascope.PGColumnState{Name: "id", TypeWidth: -1, BuiltinBool: true}, false},
		{"SET DEFAULT COALESCE(2, $1)", "ddl.pg.alter.column.generation.transition.valid", "reject", deltascope.PGColumnState{Name: "id"}, false},
		{"DROP NOT NULL", "ddl.pg.alter.column.generation.transition.valid", "reject", deltascope.PGColumnState{Name: "id", TypeWidth: 32, NotNull: true, ReplicaIdentityKeyKnown: true, ReplicaIdentityKey: true}, false},
		{"ADD GENERATED ALWAYS AS IDENTITY", "ddl.pg.alter.column.generation.transition.valid", "reject", identity, false},
		{"ADD GENERATED ALWAYS AS IDENTITY(CACHE 0)", "ddl.pg.alter.identity.sequence_options.valid", "reject", deltascope.PGColumnState{Name: "id", TypeWidth: 64, NotNull: true}, false},
		{"ADD GENERATED ALWAYS AS IDENTITY(CYCLE)", "ddl.pg.alter.identity.cycle.warn", "review", deltascope.PGColumnState{Name: "id", TypeWidth: 64, NotNull: true}, false},
		{"RESTART WITH 5", "ddl.pg.alter.identity.restart.warn", "review", identity, false},
		{"ADD GENERATED ALWAYS AS IDENTITY", "", "review", deltascope.PGColumnState{}, true},
	} {
		catalog := &deltascope.PGColumnCatalog{Ordinary: true, Columns: []deltascope.PGColumnState{tc.member}}
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
		raw, _ := json.Marshal(map[string]string{"sql": "ALTER TABLE app.t ALTER COLUMN id " + tc.suffix, "dialect": "postgresql", "schema": "app"})
		req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatal(rec.Body.String())
		}
		var body struct {
			Verdict  string `json:"verdict"`
			Coverage struct {
				Status string `json:"status"`
			} `json:"coverage"`
			Statements []struct {
				Findings []struct {
					ID       string         `json:"rule_id"`
					Level    string         `json:"level"`
					Metadata map[string]any `json:"metadata"`
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
			t.Fatal(rec.Body.String())
		}
		found := false
		for _, f := range body.Statements[0].Findings {
			if f.ID == tc.id {
				found = true
				if tc.suffix == "SET DEFAULT COALESCE(2, $1)" && f.Metadata["reason_code"] != "default_parameter_inapplicable" {
					t.Fatal(rec.Body.String())
				}
				if tc.member.BuiltinBool && f.Metadata["reason_code"] != "default_literal_type_mismatch" {
					t.Fatal(rec.Body.String())
				}
			}
		}
		if found != (tc.id != "") || tc.unknown && (body.Coverage.Status != "unverified" || len(body.Statements[0].Gaps) == 0) {
			t.Fatal(rec.Body.String())
		}
	}
}

func TestHandlerPGCreateViewOptions(t *testing.T) {
	h, err := NewHandler("", "test", WithAuditFunc(func(ctx context.Context, request deltascope.Request) (deltascope.Result, error) {
		request.MetadataProvider = pgColumnGenerationHTTPProvider{}
		return deltascope.Audit(ctx, request)
	}))
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(`{"sql":"CREATE VIEW app.v WITH (security_invoker=' true ') AS SELECT 1","dialect":"postgresql"}`))
	req.Header.Set("Content-Type", "application/json")
	out := httptest.NewRecorder()
	h.ServeHTTP(out, req)
	if out.Code != http.StatusOK || !bytes.Contains(out.Body.Bytes(), []byte(`"ddl.pg.create_view.options.valid"`)) || !bytes.Contains(out.Body.Bytes(), []byte(`"view_option_invalid_value"`)) {
		t.Fatalf("status=%d body=%s", out.Code, out.Body)
	}
}
