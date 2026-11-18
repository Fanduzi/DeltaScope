//go:build postgresql

package mcpapi

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/application/auditmeta"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAuditSQLPGColumnTuning(t *testing.T) {
	previous := prepareMetadataAudit
	t.Cleanup(func() { prepareMetadataAudit = previous })
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
		catalog := &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{{Name: "id"}}}
		if tc.unknown {
			catalog = nil
		}
		client := &mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL, snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGColumns: catalog}}
		prepareMetadataAudit = func(context.Context, auditmeta.Request) (*auditmeta.PreparedAudit, error) {
			return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app", DialectSource: "request", SchemaSource: "request"}, nil
		}
		session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
		if err != nil {
			t.Fatal(err)
		}
		r, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{"sql": "ALTER TABLE app.t " + tc.sql, "dialect": "postgresql", "connection": map[string]any{"host": "127.0.0.1", "user": "auditor", "database": "fixture", "schema": "app"}}})
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
		for _, item := range findings {
			f := item.(map[string]any)
			if f["rule_id"] == tc.id {
				found = true
			}
			b, _ := json.Marshal(f["metadata"])
			if strings.Contains(string(b), "10001") || strings.Contains(string(b), "-1.1") || strings.Contains(string(b), "PGColumns") {
				t.Fatal("private facts leaked", string(b))
			}
			id := f["rule_id"].(string)
			if tc.id == "" && (strings.HasPrefix(id, "ddl.pg.alter.column.") || id == "ddl.pg.alter.drop_column.advisory") {
				t.Fatal("no-op finding", f)
			}
		}
		if found != (tc.id != "") {
			t.Fatal("shared finding lost", body)
		}
		if tc.unknown {
			gaps, _ := s["evidence_gaps"].([]any)
			if body["coverage"].(map[string]any)["status"] != "unverified" || len(gaps) != 1 {
				t.Fatal(body)
			}
		}
	}
}
