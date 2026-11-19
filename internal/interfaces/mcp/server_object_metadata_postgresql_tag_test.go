//go:build postgresql

package mcpapi

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	auditmeta "github.com/Fanduzi/DeltaScope/internal/application/auditmeta"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPAuditProjectUnavailableObjectMetadata(t *testing.T) {
	server := NewServer(Config{Version: "test-version"})

	session, err := connectClientSession(context.Background(), server)
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	previous := prepareMetadataAudit
	client := &mcpMetadataAuditTestClient{
		detectDialect: spec.DialectPostgreSQL,
	}
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{
			Client:        client,
			Dialect:       spec.DialectPostgreSQL,
			Schema:        "public",
			DialectSource: "request",
			SchemaSource:  "inferred",
		}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql":     "DROP SCHEMA old_schema",
			"dialect": "postgresql",
			"connection": map[string]any{
				"host":   "127.0.0.1",
				"user":   "root",
				"schema": "public",
			},
		},
	})
	if err != nil {
		t.Fatalf("call audit_sql: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result, got tool error: %#v", result)
	}

	body, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("expected structured map result, got %T", result.StructuredContent)
	}

	statements, ok := body["statements"].([]any)
	if !ok || len(statements) == 0 {
		t.Fatalf("expected statements, got %#v", body["statements"])
	}
	stmt, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement map, got %T", statements[0])
	}

	findings, ok := stmt["findings"].([]any)
	if !ok {
		t.Fatalf("expected findings, got %#v", stmt["findings"])
	}

	var findingMeta map[string]any
	for _, item := range findings {
		f, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if f["rule_id"] == "ddl.pg.drop_schema.advisory" {
			findingMeta, _ = f["metadata"].(map[string]any)
			break
		}
	}
	if findingMeta == nil {
		t.Fatalf("expected drop_schema finding, got %#v", findings)
	}

	if findingMeta["metadata_status"] != "unavailable" {
		t.Fatalf("expected metadata_status=unavailable, got %#v", findingMeta["metadata_status"])
	}
	if findingMeta["metadata_object_type"] != "schema" {
		t.Fatalf("expected metadata_object_type=schema, got %#v", findingMeta["metadata_object_type"])
	}
	if findingMeta["metadata_object_name"] != "old_schema" {
		t.Fatalf("expected metadata_object_name=old_schema, got %#v", findingMeta["metadata_object_name"])
	}

	sensitiveKeys := []string{"password", "secret", "conninfo", "connection", "host", "port", "options", "query", "body", "definition", "comment", "label"}
	for _, key := range sensitiveKeys {
		if _, ok := findingMeta["metadata_"+key]; ok {
			t.Fatalf("sensitive key metadata_%q leaked into finding metadata", key)
		}
	}
}

type replicaIdentityAuditClient struct{ *mcpMetadataAuditTestClient }

func (replicaIdentityAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	return &spec.ObjectSnapshot{PGReplicaIdentity: &spec.PGReplicaIdentityIndex{Schema: req.Schema, Table: req.Qualifiers["table"], Name: req.Name, Kind: "i", IndexKnown: true}}, nil
}

func TestMCPReplicaIdentityAttachment(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := replicaIdentityAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER TABLE app.t REPLICA IDENTITY USING INDEX other_ix", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"index_not_on_table"`) || !strings.Contains(string(body), `"ddl.pg.alter.replica_identity_using_index.valid"`) || strings.Contains(string(body), "PGReplicaIdentity") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

func TestMCPPGTableReloptions(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := &mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r", Heap: true}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER TABLE app.t SET (autovacuum_enabled=1.0)", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"table_reloption_invalid_boolean"`) || !strings.Contains(string(body), `"ddl.pg.alter.reloptions.valid"`) || strings.Contains(string(body), "PGTableReloptions") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type tableTablespaceAuditClient struct{ *mcpMetadataAuditTestClient }

func (tableTablespaceAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	return &spec.ObjectSnapshot{PGTableTablespace: &spec.PGTableTablespace{Schema: req.Schema, Table: req.Qualifiers["table"], Name: req.Name, Exists: true, Global: true}}, nil
}

func TestMCPTableTablespace(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := tableTablespaceAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER TABLE app.t SET TABLESPACE pg_global", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"tablespace_global_inapplicable"`) || !strings.Contains(string(body), `"ddl.pg.alter.tablespace.target.valid"`) || strings.Contains(string(body), "PGTableTablespace") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}
