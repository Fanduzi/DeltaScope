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

type destructiveTargetClient struct{ *mcpMetadataAuditTestClient }

func (c *destructiveTargetClient) LoadTableSnapshot(_ context.Context, _ spec.Dialect, schema, table string) (*spec.TableSnapshot, error) {
	c.tableCalls = append(c.tableCalls, table)
	switch table {
	case "absent":
		return &spec.TableSnapshot{Schema: schema, PGRelation: spec.PGRelationState{Name: table}}, nil
	case "partition_root":
		return &spec.TableSnapshot{Schema: schema, Exists: true, Table: &spec.Table{Name: table}, PGRelation: spec.PGRelationState{Name: table, Kind: "p"}}, nil
	default:
		return &spec.TableSnapshot{Schema: schema, Exists: true, Table: &spec.Table{Name: table}, PGRelation: spec.PGRelationState{Name: table, Kind: "v"}}, nil
	}
}

func (c *destructiveTargetClient) ResolveObject(_ context.Context, _ spec.Dialect, request spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if request.Type == "truncate_references" && len(request.PGDestructiveTargets) > 0 {
		return &spec.ObjectSnapshot{PGDestructiveReferences: &spec.PGDestructiveReferences{Targets: request.PGDestructiveTargets, BlockedPosition: 1}}, nil
	}
	return nil, nil
}

func TestMCPPGDestructiveTargets(t *testing.T) {
	ctx := context.Background()
	session, err := connectClientSession(ctx, NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := &destructiveTargetClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL}}
	old := prepareMetadataAudit
	t.Cleanup(func() { prepareMetadataAudit = old })
	prepareMetadataAudit = func(context.Context, auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	out, err := session.CallTool(ctx, &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "TRUNCATE ONLY app.partition_root", "dialect": "postgresql", "connection": map[string]any{"host": "127.0.0.1", "user": "auditor", "schema": "app"},
	}})
	if err != nil || out.IsError {
		t.Fatalf("result=%+v err=%v", out, err)
	}
	body, err := json.Marshal(out.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"ddl.table.truncate.exists.require"`) || !strings.Contains(string(body), `"partition_root_only_inapplicable"`) || strings.Contains(string(body), "PGDestructiveTableRelations") {
		t.Fatalf("result=%s err=%v", body, err)
	}

	out, err = session.CallTool(ctx, &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "TRUNCATE app.partition_root", "dialect": "postgresql", "connection": map[string]any{"host": "127.0.0.1", "user": "auditor", "schema": "app"},
	}})
	if err != nil || out.IsError {
		t.Fatalf("result=%+v err=%v", out, err)
	}
	body, err = json.Marshal(out.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"truncate_referenced_by_external_foreign_key"`) || strings.Contains(string(body), "IneligibleOnlyPosition") {
		t.Fatalf("result=%s err=%v", body, err)
	}
}
