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

func TestAuditSQLPGColumnFormat(t *testing.T) {
	previous := prepareMetadataAudit
	t.Cleanup(func() { prepareMetadataAudit = previous })
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
		catalog := &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{{Name: "id", TypeStorage: tc.storage}}, SystemColumns: []spec.PGColumnState{{Name: "ctid", System: true, TypeStorage: "p"}}}
		client := &mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL, snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGColumns: catalog}}
		prepareMetadataAudit = func(context.Context, auditmeta.Request) (*auditmeta.PreparedAudit, error) {
			return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app", DialectSource: "request", SchemaSource: "request"}, nil
		}
		session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
		if err != nil {
			t.Fatal(err)
		}
		r, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{"sql": "ALTER TABLE app.t " + tc.clause, "dialect": "postgresql", "connection": map[string]any{"host": "127.0.0.1", "user": "auditor", "database": "fixture", "schema": "app"}}})
		if closeErr := session.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
		if err != nil || r.IsError {
			t.Fatalf("result=%+v err=%v", r, err)
		}
		body := requireAuditStructuredMap(t, r)
		statements := body["statements"].([]any)
		if body["verdict"] != tc.verdict || len(statements) != 1 {
			t.Fatal(body)
		}
		s := statements[0].(map[string]any)
		findings, _ := s["findings"].([]any)
		found := false
		for _, item := range findings {
			f := item.(map[string]any)
			id := f["rule_id"].(string)
			if id == tc.id {
				found = true
			}
			if strings.Contains(tc.clause, "ctid") && id == "ddl.pg.alter.drop_column.advisory" {
				t.Fatal("system deletion advice", f)
			}
			b, _ := json.Marshal(f["metadata"])
			if strings.Contains(string(b), "private_method_payload") || strings.Contains(string(b), "PGColumns") {
				t.Fatal("private facts leaked", string(b))
			}
			if tc.id == "" && strings.HasPrefix(id, "ddl.pg.alter.column.") {
				t.Fatal("unexpected format finding", f)
			}
		}
		if found != (tc.id != "") {
			t.Fatal("shared finding lost", body)
		}
		if tc.verdict == "review" {
			gaps, _ := s["evidence_gaps"].([]any)
			if body["coverage"].(map[string]any)["status"] != "unverified" || len(gaps) != 1 {
				t.Fatal(body)
			}
		}
	}
}
