//go:build postgresql

package mcpapi

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/application/auditmeta"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAuditSQLPGColumnGeneration(t *testing.T) {
	previous := prepareMetadataAudit
	t.Cleanup(func() { prepareMetadataAudit = previous })
	v := func(n int64) *int64 { return &n }
	cycle := false
	identity := spec.PGColumnState{Name: "id", Identity: "d", TypeWidth: 64, NotNull: true, IdentitySequence: &spec.PGIdentitySequenceState{TypeWidth: 16, Definition: spec.SequenceDefinition{Start: v(1), Increment: v(1), MinValue: v(1), MaxValue: v(10), CacheValue: v(1), Cycle: &cycle}}}
	for _, tc := range []struct {
		suffix, id, verdict string
		member              spec.PGColumnState
		unknown             bool
	}{
		{"SET DEFAULT (SELECT 2)", "ddl.pg.alter.column.generation.transition.valid", "reject", spec.PGColumnState{Name: "id"}, false},
		{"DROP NOT NULL", "ddl.pg.alter.column.generation.transition.valid", "reject", spec.PGColumnState{Name: "id", TypeWidth: 32, NotNull: true, ReplicaIdentityKeyKnown: true, ReplicaIdentityKey: true}, false},
		{"ADD GENERATED ALWAYS AS IDENTITY", "ddl.pg.alter.column.generation.transition.valid", "reject", identity, false},
		{"ADD GENERATED ALWAYS AS IDENTITY(CACHE 0)", "ddl.pg.alter.identity.sequence_options.valid", "reject", spec.PGColumnState{Name: "id", TypeWidth: 64, NotNull: true}, false},
		{"ADD GENERATED ALWAYS AS IDENTITY(CYCLE)", "ddl.pg.alter.identity.cycle.warn", "review", spec.PGColumnState{Name: "id", TypeWidth: 64, NotNull: true}, false},
		{"RESTART WITH 5", "ddl.pg.alter.identity.restart.warn", "review", identity, false},
		{"ADD GENERATED ALWAYS AS IDENTITY", "", "review", spec.PGColumnState{}, true},
	} {
		catalog := &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{tc.member}}
		if tc.unknown {
			catalog = nil
		}
		client := &mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL, snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGColumns: catalog, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}}}
		prepareMetadataAudit = func(context.Context, auditmeta.Request) (*auditmeta.PreparedAudit, error) {
			return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app", DialectSource: "request", SchemaSource: "request"}, nil
		}
		session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
		if err != nil {
			t.Fatal(err)
		}
		r, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{"sql": "ALTER TABLE app.t ALTER COLUMN id " + tc.suffix, "dialect": "postgresql", "connection": map[string]any{"host": "127.0.0.1", "user": "auditor", "database": "fixture", "schema": "app"}}})
		if e := session.Close(); e != nil {
			t.Fatal(e)
		}
		if err != nil || r.IsError {
			t.Fatalf("result=%+v err=%v", r, err)
		}
		body := requireAuditStructuredMap(t, r)
		if body["verdict"] != tc.verdict {
			t.Fatal(body)
		}
		statements := body["statements"].([]any)
		if len(statements) != 1 {
			t.Fatal(body)
		}
		s := statements[0].(map[string]any)
		findings, _ := s["findings"].([]any)
		found := false
		for _, f := range findings {
			if f.(map[string]any)["rule_id"] == tc.id {
				found = true
			}
		}
		if found != (tc.id != "") {
			t.Fatal("shared finding lost")
		}
		if tc.unknown {
			gaps, _ := s["evidence_gaps"].([]any)
			if body["coverage"].(map[string]any)["status"] != "unverified" || len(gaps) == 0 {
				t.Fatal(body)
			}
		}
	}
}

func TestAuditSQLPGCreateViewOptions(t *testing.T) {
	ctx := context.Background()
	previous := prepareMetadataAudit
	t.Cleanup(func() { prepareMetadataAudit = previous })
	prepareMetadataAudit = func(context.Context, auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		client := &mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL}
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	session, err := connectClientSession(ctx, NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	result, err := session.CallTool(ctx, &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": `CREATE VIEW app.v WITH (security_invoker=' true ') AS SELECT 1`, "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "auditor", "database": "fixture", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	out := requireAuditStructuredMap(t, result)
	row := out["statements"].([]any)[0].(map[string]any)
	for _, value := range row["findings"].([]any) {
		finding := value.(map[string]any)
		if finding["rule_id"] == "ddl.pg.create_view.options.valid" && finding["metadata"].(map[string]any)["reason_code"] == "view_option_invalid_value" {
			return
		}
	}
	t.Fatalf("missing view option finding: %+v", row)
}
