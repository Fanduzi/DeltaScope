//go:build postgresql

package mcpapi

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/application/auditmeta"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAuditSQLPGConstraintTarget(t *testing.T) {
	previous := prepareMetadataAudit
	t.Cleanup(func() { prepareMetadataAudit = previous })
	for _, tc := range []struct {
		sql, id, verdict string
		members          []spec.PGConstraintState
	}{
		{`ALTER TABLE app.t ADD CONSTRAINT " Exact Check " CHECK (1 > 0) NOT VALID`, "ddl.pg.alter.add_check.name.available", "reject", []spec.PGConstraintState{{Name: " Exact Check ", Type: "c", Validated: true}}},
		{`ALTER TABLE app.t ADD FOREIGN KEY (id) REFERENCES app.t ("Missing") NOT VALID`, "ddl.pg.alter.add_foreign_key.targets.valid", "reject", []spec.PGConstraintState{}},
		{"ALTER TABLE ONLY app.t VALIDATE CONSTRAINT c", "ddl.pg.alter.validate_constraint.target.require", "reject", []spec.PGConstraintState{{Name: "c", Type: "c", CheckValidationChildrenKnown: true, CheckValidationHasChildren: true}}},
		{"ALTER TABLE app.t VALIDATE CONSTRAINT c", "ddl.pg.alter.validate_constraint.target.require", "reject", []spec.PGConstraintState{}},
		{"ALTER TABLE app.t ALTER CONSTRAINT c DEFERRABLE", "ddl.pg.alter.constraint_deferrability.target.require", "reject", []spec.PGConstraintState{}},
		{"ALTER TABLE app.t DROP CONSTRAINT IF EXISTS c CASCADE", "ddl.pg.alter.drop_constraint.target.require", "reject", []spec.PGConstraintState{{Name: "c", Type: "c", Validated: true, InheritanceKnown: true, Inherited: true}}},
		{"ALTER TABLE ONLY app.t DROP CONSTRAINT c CASCADE", "ddl.pg.alter.drop_constraint.target.require", "reject", []spec.PGConstraintState{{Name: "c", Type: "c", Validated: true, InheritanceKnown: true, PartitionCheckHasChildrenKnown: true, PartitionCheckHasChildren: true}}},
		{"ALTER TABLE app.t DROP CONSTRAINT c", "ddl.pg.alter.drop_constraint.target.require", "reject", []spec.PGConstraintState{}},
		{"ALTER TABLE app.t DROP CONSTRAINT IF EXISTS c", "", "pass", []spec.PGConstraintState{}},
		{"ALTER TABLE app.t VALIDATE CONSTRAINT c", "", "review", nil},
	} {
		kind := "r"
		if len(tc.members) == 1 && tc.members[0].PartitionCheckHasChildrenKnown {
			kind = "p"
		}
		client := &mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL, snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGConstraints: tc.members, PGRelation: spec.PGRelationState{Name: "t", Kind: kind}, PGColumns: &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{{Name: "id"}}}}}
		prepareMetadataAudit = func(context.Context, auditmeta.Request) (*auditmeta.PreparedAudit, error) {
			return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app", DialectSource: "request", SchemaSource: "request"}, nil
		}
		session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
		if err != nil {
			t.Fatal(err)
		}
		r, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{"sql": tc.sql, "dialect": "postgresql", "connection": map[string]any{"host": "127.0.0.1", "user": "auditor", "database": "fixture", "schema": "app"}}})
		if closeErr := session.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
		if err != nil || r.IsError {
			t.Fatalf("result=%+v err=%v", r, err)
		}
		body := requireAuditStructuredMap(t, r)
		if body["verdict"] != tc.verdict {
			t.Fatalf("body=%v", body)
		}
		statements := body["statements"].([]any)
		if len(statements) != 1 {
			t.Fatal(statements)
		}
		s := statements[0].(map[string]any)
		found := false
		findings, _ := s["findings"].([]any)
		for _, v := range findings {
			f := v.(map[string]any)
			if f["rule_id"] == tc.id {
				found = true
				if f["level"] != "blocker" {
					t.Fatal(f)
				}
			}
		}
		if found != (tc.id != "") {
			t.Fatal("lost shared target blocker")
		}
		if tc.verdict == "review" {
			gaps, _ := s["evidence_gaps"].([]any)
			if body["coverage"].(map[string]any)["status"] != "unverified" || len(gaps) != 1 || gaps[0].(map[string]any)["rule_id"] != "ddl.pg.alter.validate_constraint.target.require" {
				t.Fatalf("lost shared gap: %v", body)
			}
		}
	}
}
