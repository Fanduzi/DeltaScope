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

type tableAccessMethodAuditClient struct{ *mcpMetadataAuditTestClient }

func (tableAccessMethodAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	return &spec.ObjectSnapshot{PGTableAccessMethod: &spec.PGTableAccessMethod{Schema: req.Schema, Table: req.Qualifiers["table"], Name: req.Name, Kind: "i"}}, nil
}

func TestMCPTableAccessMethod(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := tableAccessMethodAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER TABLE app.t SET ACCESS METHOD btree", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"access_method_wrong_kind"`) || !strings.Contains(string(body), `"ddl.pg.alter.access_method.target.valid"`) || strings.Contains(string(body), "PGTableAccessMethod") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type tablePersistenceAuditClient struct{ *mcpMetadataAuditTestClient }

func (tablePersistenceAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	return &spec.ObjectSnapshot{PGTablePersistence: &spec.PGTablePersistence{Schema: req.Schema, Table: req.Name, Current: "p", ForeignKeyConflict: true}}, nil
}

func TestMCPTablePersistence(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := tablePersistenceAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER TABLE app.t SET UNLOGGED", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"persistence_inbound_foreign_key"`) || !strings.Contains(string(body), `"ddl.pg.alter.persistence.transition.valid"`) || strings.Contains(string(body), "PGTablePersistence") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

func TestMCPColumnTypeSource(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := &mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{{Name: "stored", Generated: "s"}}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER TABLE app.t ALTER COLUMN stored TYPE integer USING stored", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"generated_column_using_inapplicable"`) || !strings.Contains(string(body), `"ddl.pg.alter.column_type.source.valid"`) || strings.Contains(string(body), "PGColumns") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type tableNamespaceAuditClient struct{ *mcpMetadataAuditTestClient }

func (tableNamespaceAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	return &spec.ObjectSnapshot{PGTableNamespace: &spec.PGTableNamespace{Schema: req.Schema, Table: req.Qualifiers["table"], Action: "set_schema", Destination: req.Name}}, nil
}

func TestMCPTableNamespace(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := tableNamespaceAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER TABLE app.t SET SCHEMA missing", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"destination_schema_absent"`) || !strings.Contains(string(body), `"ddl.pg.alter.set_schema.target.valid"`) || strings.Contains(string(body), "PGTableNamespace") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type tableOwnerAuditClient struct{ *mcpMetadataAuditTestClient }

func (tableOwnerAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	return &spec.ObjectSnapshot{PGTableOwner: &spec.PGTableOwnerState{Schema: req.Schema, Table: req.Qualifiers["table"], Target: spec.PGTableOwnerTarget{Kind: req.Qualifiers["kind"], Name: req.Name}}}, nil
}

func TestMCPTableOwner(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := tableOwnerAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER TABLE app.t OWNER TO missing", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"owner_role_absent"`) || !strings.Contains(string(body), `"ddl.pg.alter.owner.target.valid"`) || strings.Contains(string(body), "PGTableOwner") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type bulkTablespaceAuditClient struct{ *mcpMetadataAuditTestClient }

func (bulkTablespaceAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "bulk_tablespace" || req.PGBulkTablespace == nil {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGBulkTablespace: &spec.PGBulkTablespaceState{Target: *req.PGBulkTablespace, SourceExists: true}}, nil
}

func TestMCPBulkTablespace(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := bulkTablespaceAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7"}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER TABLE ALL IN TABLESPACE left_space SET TABLESPACE missing", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"destination_tablespace_absent"`) || !strings.Contains(string(body), `"ddl.pg.alter.move_all_tablespace.target.valid"`) || strings.Contains(string(body), "PGBulkTablespace") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type columnTypeAuditClient struct{ *mcpMetadataAuditTestClient }

func (columnTypeAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	return &spec.ObjectSnapshot{PGColumnType: &spec.PGColumnTypeState{Target: spec.PGColumnTypeTarget{Schema: req.Schema, Name: req.Name, Array: req.Qualifiers["array"] == "true"}, SchemaExists: true, BaseExists: true, Resolved: true, Defined: true, Pseudo: true}}, nil
}

func TestMCPColumnType(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := columnTypeAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{{Name: "id"}}}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER TABLE app.t ALTER COLUMN id TYPE pg_catalog.cstring[]", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"destination_pseudo_type_inapplicable"`) || !strings.Contains(string(body), `"ddl.pg.alter.column_type.destination.valid"`) || strings.Contains(string(body), "PGColumnType") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type constraintUsingIndexAuditClient struct{ *mcpMetadataAuditTestClient }

func (constraintUsingIndexAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	return &spec.ObjectSnapshot{PGConstraintUsingIndex: &spec.PGConstraintUsingIndex{Schema: req.Schema, Table: req.Qualifiers["table"], Name: req.Name, Kind: "i", IndexKnown: true, OnTable: true, Unique: true, BTree: true}}, nil
}

func TestMCPConstraintUsingIndex(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := constraintUsingIndexAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{{Name: "id"}}}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": `ALTER TABLE app.t ADD CONSTRAINT pending UNIQUE USING INDEX " Exact Index "`, "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"constraint_index_invalid"`) || !strings.Contains(string(body), `"ddl.pg.alter.constraint_using_index.valid"`) || strings.Contains(string(body), "PGConstraintUsingIndex") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type foreignKeyReferenceKeysAuditClient struct{ *mcpMetadataAuditTestClient }

func (foreignKeyReferenceKeysAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, _ spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	return &spec.ObjectSnapshot{PGForeignKeyKeys: &spec.PGForeignKeyKeys{Schema: "app", Table: "t"}}, nil
}

func TestMCPForeignKeyReferenceKeys(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := foreignKeyReferenceKeysAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{{Name: "id"}}}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": `ALTER TABLE app.t ADD FOREIGN KEY (id) REFERENCES app.t(id) NOT VALID`, "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"foreign_key_reference_unique_key_absent"`) || !strings.Contains(string(body), `"ddl.pg.alter.add_foreign_key.targets.valid"`) || strings.Contains(string(body), "PGForeignKeyKeys") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type dropConstraintReferenceAuditClient struct{ *mcpMetadataAuditTestClient }

func (dropConstraintReferenceAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, _ spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	return &spec.ObjectSnapshot{PGDropConstraintReference: &spec.PGDropConstraintReference{Schema: "app", Table: "t", Name: "pk", Kind: "p", Blocked: true}}, nil
}

func TestMCPDropConstraintReference(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := dropConstraintReferenceAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGConstraints: []spec.PGConstraintState{{Name: "pk", Type: "p", Validated: true}}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{{Name: "id"}}}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": `ALTER TABLE app.t DROP CONSTRAINT pk RESTRICT`, "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"constraint_referenced_by_foreign_key"`) || !strings.Contains(string(body), `"ddl.pg.alter.drop_constraint.target.require"`) || strings.Contains(string(body), "PGDropConstraintReference") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type dropColumnDependencyAuditClient struct{ *mcpMetadataAuditTestClient }

func (dropColumnDependencyAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, _ spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	return &spec.ObjectSnapshot{PGDropColumnDependency: &spec.PGDropColumnDependency{Schema: "app", Table: "t", Column: "id", Kind: "foreign_key"}}, nil
}

func TestMCPDropColumnDependency(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := dropColumnDependencyAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{{Name: "id"}}}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": `ALTER TABLE app.t DROP COLUMN id RESTRICT`, "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"column_referenced_by_foreign_key"`) || !strings.Contains(string(body), `"ddl.alter.drop_column.exists.require"`) || strings.Contains(string(body), "PGDropColumnDependency") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type tableNamespaceCompanionAuditClient struct{ *mcpMetadataAuditTestClient }

func (tableNamespaceCompanionAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	return &spec.ObjectSnapshot{PGTableNamespace: &spec.PGTableNamespace{Schema: req.Schema, Table: req.Qualifiers["table"], Action: "set_schema", Destination: req.Name, SchemaExists: true, RowTypeConflictKnown: true, ArrayTypeConflictKnown: true, OwnedSequenceConflict: true}}, nil
}

func TestMCPTableNamespaceCompanion(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := tableNamespaceCompanionAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER TABLE app.t SET SCHEMA dst", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"destination_owned_sequence_conflict"`) || !strings.Contains(string(body), `"ddl.pg.alter.set_schema.target.valid"`) || strings.Contains(string(body), "PGTableNamespace") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

func TestMCPSetWithoutOIDs(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := &mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "v"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER TABLE IF EXISTS app.t SET WITHOUT OIDS", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"relation_wrong_kind"`) || !strings.Contains(string(body), `"ddl.table.exists.alter.require"`) || strings.Contains(string(body), "PGTableNamespace") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type columnTypeAssignmentAuditClient struct{ *mcpMetadataAuditTestClient }

func (columnTypeAssignmentAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	return &spec.ObjectSnapshot{PGColumnType: &spec.PGColumnTypeState{Target: spec.PGColumnTypeTarget{Schema: req.Schema, Name: req.Name, Array: req.Qualifiers["array"] == "true"}, SchemaExists: true, BaseExists: true, Resolved: true, Defined: true, AssignmentKnown: true, SourceSchema: req.Qualifiers["source_schema"], SourceTable: req.Qualifiers["source_table"], SourceColumn: req.Qualifiers["source_column"]}}, nil
}

func TestMCPColumnTypeAssignment(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := columnTypeAssignmentAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{{Name: "id"}}}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER TABLE app.t ALTER COLUMN id TYPE pg_catalog.int4", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"destination_type_assignment_unavailable"`) || !strings.Contains(string(body), `"ddl.pg.alter.column_type.destination.valid"`) || strings.Contains(string(body), "PGColumnType") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type constraintDefaultOpclassAuditClient struct{ *mcpMetadataAuditTestClient }

func (constraintDefaultOpclassAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	return &spec.ObjectSnapshot{PGConstraintUsingIndex: &spec.PGConstraintUsingIndex{Schema: req.Schema, Table: req.Qualifiers["table"], Name: req.Name, Kind: "i", IndexKnown: true, OnTable: true, Unique: true, BTree: true, Valid: true, KeyDefaults: true, KeyDefaultsKnown: true, OpclassesDefaultKnown: true}}, nil
}

func TestMCPConstraintDefaultOpclass(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := constraintDefaultOpclassAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{{Name: "id"}}}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": `ALTER TABLE app.t ADD CONSTRAINT pending UNIQUE USING INDEX " Exact Index "`, "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"constraint_index_nondefault_opclass"`) || !strings.Contains(string(body), `"ddl.pg.alter.constraint_using_index.valid"`) || strings.Contains(string(body), "PGConstraintUsingIndex") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

func TestMCPForeignKeyDeleteSetColumns(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := foreignKeyReferenceKeysAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{{Name: "id"}, {Name: "extra"}}}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": `ALTER TABLE app.t ADD FOREIGN KEY (id) REFERENCES app.t(id) ON DELETE SET NULL (extra) NOT VALID`, "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"foreign_key_delete_set_column_outside_key"`) || !strings.Contains(string(body), `"ddl.pg.alter.add_foreign_key.targets.valid"`) || strings.Contains(string(body), "DeleteSetColumns") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type inheritedColumnDropClient struct{ *mcpMetadataAuditTestClient }

func (inheritedColumnDropClient) ResolveObject(_ context.Context, _ spec.Dialect, _ spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	return &spec.ObjectSnapshot{PGColumnTraversal: &spec.PGColumnTraversal{Schema: "app", Table: "t", Column: "id", Inherited: true}}, nil
}

func TestMCPInheritedColumnDrop(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := inheritedColumnDropClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: false, Columns: []spec.PGColumnState{{Name: "id"}}}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": `ALTER TABLE app.t DROP COLUMN IF EXISTS id CASCADE`, "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"column_inherited"`) || !strings.Contains(string(body), `"ddl.alter.drop_column.exists.require"`) || strings.Contains(string(body), "PGColumnTraversal") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type partitionAddColumnClient struct{ *mcpMetadataAuditTestClient }

func (partitionAddColumnClient) ResolveObject(_ context.Context, _ spec.Dialect, _ spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	return &spec.ObjectSnapshot{PGColumnTraversal: &spec.PGColumnTraversal{Schema: "app", Table: "t", Column: "id", Partition: true, ColumnExists: true}}, nil
}

func TestMCPPartitionAddColumn(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := partitionAddColumnClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: false, Columns: []spec.PGColumnState{{Name: "id"}}}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": `ALTER TABLE app.t ADD COLUMN IF NOT EXISTS id int`, "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"add_column_partition_inapplicable"`) || !strings.Contains(string(body), `"ddl.alter.add_column.exists.forbid"`) || strings.Contains(string(body), "PGColumnTraversal") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type foreignKeyScalarEqualityClient struct{ *mcpMetadataAuditTestClient }

func (foreignKeyScalarEqualityClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.PGForeignKeyEqualityTarget != nil {
		return &spec.ObjectSnapshot{PGForeignKeyEquality: &spec.PGForeignKeyEquality{Target: *req.PGForeignKeyEqualityTarget, IncompatiblePosition: 1}}, nil
	}
	return &spec.ObjectSnapshot{PGForeignKeyKeys: &spec.PGForeignKeyKeys{Schema: "app", Table: "t", Keys: []spec.PGForeignKeyKey{{Columns: []string{"id"}, Immediate: true}}}}, nil
}

func TestMCPForeignKeyScalarEquality(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := foreignKeyScalarEqualityClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{{Name: "id"}}}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": `ALTER TABLE app.t ADD FOREIGN KEY (id) REFERENCES app.t(id) NOT VALID`, "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"foreign_key_column_types_incompatible"`) || !strings.Contains(string(body), `"ddl.pg.alter.add_foreign_key.targets.valid"`) || strings.Contains(string(body), "PGForeignKeyEquality") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type partitionDropNotNullClient struct{ *mcpMetadataAuditTestClient }

func (partitionDropNotNullClient) ResolveObject(_ context.Context, _ spec.Dialect, _ spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	return &spec.ObjectSnapshot{PGColumnTraversal: &spec.PGColumnTraversal{Schema: "app", Table: "t", Column: "id", Partition: true, ColumnExists: true, ParentNotNull: true}}, nil
}

func TestMCPPartitionDropNotNull(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := partitionDropNotNullClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: false, Columns: []spec.PGColumnState{{Name: "id"}}}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": `ALTER TABLE app.t ALTER COLUMN id DROP NOT NULL`, "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"not_null_required_by_partition_parent"`) || !strings.Contains(string(body), `"ddl.pg.alter.column.generation.transition.valid"`) || strings.Contains(string(body), "PGColumnTraversal") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type columnTypeOnlyParentAuditClient struct{ *mcpMetadataAuditTestClient }

func (columnTypeOnlyParentAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type == "column_traversal" {
		return &spec.ObjectSnapshot{PGColumnTraversal: &spec.PGColumnTraversal{Schema: req.Schema, Table: req.Qualifiers["table"], Column: req.Name, ColumnExists: true, AttachedChildren: true}}, nil
	}
	return &spec.ObjectSnapshot{PGColumnType: &spec.PGColumnTypeState{Target: spec.PGColumnTypeTarget{Schema: req.Schema, Name: req.Name, Array: req.Qualifiers["array"] == "true"}, SchemaExists: true, BaseExists: true, Resolved: true, Defined: true, AssignmentKnown: true, Assignable: true, SourceSchema: req.Qualifiers["source_schema"], SourceTable: req.Qualifiers["source_table"], SourceColumn: req.Qualifiers["source_column"]}}, nil
}

func TestMCPColumnTypeOnlyParent(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := columnTypeOnlyParentAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: false, Columns: []spec.PGColumnState{{Name: "id"}}}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER TABLE ONLY app.t ALTER COLUMN id TYPE pg_catalog.int4", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"column_type_requires_child_traversal"`) || !strings.Contains(string(body), `"ddl.pg.alter.column_type.destination.valid"`) || strings.Contains(string(body), "PGColumnType") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type columnTypeRecursiveParentAuditClient struct{ *mcpMetadataAuditTestClient }

func (columnTypeRecursiveParentAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type == "column_traversal" {
		return &spec.ObjectSnapshot{PGColumnTraversal: &spec.PGColumnTraversal{Schema: req.Schema, Table: req.Qualifiers["table"], Column: req.Name, ColumnExists: true, AttachedChildren: true, TypeDescendantConflict: true}}, nil
	}
	return &spec.ObjectSnapshot{PGColumnType: &spec.PGColumnTypeState{Target: spec.PGColumnTypeTarget{Schema: req.Schema, Name: req.Name, Array: req.Qualifiers["array"] == "true"}, SchemaExists: true, BaseExists: true, Resolved: true, Defined: true, AssignmentKnown: true, Assignable: true, SourceSchema: req.Qualifiers["source_schema"], SourceTable: req.Qualifiers["source_table"], SourceColumn: req.Qualifiers["source_column"]}}, nil
}

func TestMCPColumnTypeRecursiveParent(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := columnTypeRecursiveParentAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: false, Columns: []spec.PGColumnState{{Name: "id"}}}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER TABLE app.t ALTER COLUMN id TYPE pg_catalog.int4", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"column_type_descendant_inheritance_conflict"`) || !strings.Contains(string(body), `"ddl.pg.alter.column_type.destination.valid"`) || strings.Contains(string(body), "PGColumnType") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type refreshMaterializedViewAuditClient struct{ *mcpMetadataAuditTestClient }

func (refreshMaterializedViewAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "refresh_materialized_view" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGRefreshState: &spec.PGRefreshState{Target: spec.PGRefreshTarget{Schema: req.Schema, Name: req.Name}, Kind: "m", Populated: true}}, nil
}

func TestMCPPGRefreshMaterializedViewTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := refreshMaterializedViewAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "REFRESH MATERIALIZED VIEW CONCURRENTLY app.mv", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"refresh_concurrent_unique_index_absent"`) || !strings.Contains(string(body), `"ddl.pg.refresh_materialized_view.target.valid"`) || strings.Contains(string(body), "PGRefreshState") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type dropViewTargetAuditClient struct{ *mcpMetadataAuditTestClient }

func (dropViewTargetAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "drop_view_target" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGDropViewState: &spec.PGDropViewState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: req.Schema, Name: req.Name}}, Kind: "v", DependentView: true}}, nil
}

func TestMCPPGDropViewTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := dropViewTargetAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "DROP VIEW app.mv RESTRICT", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"drop_view_has_direct_dependency"`) || !strings.Contains(string(body), `"ddl.pg.drop_view.target.valid"`) || strings.Contains(string(body), "PGDropViewState") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type viewNamespaceAuditClient struct{ *mcpMetadataAuditTestClient }

func (viewNamespaceAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "view_namespace" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGTableNamespace: &spec.PGTableNamespace{Schema: req.Schema, Table: req.Qualifiers["table"], Action: req.Qualifiers["action"], Destination: req.Name, SourceKind: "v", SourceKindKnown: true, RelationConflict: true}}, nil
}

func TestMCPPGViewNamespaceTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := viewNamespaceAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER VIEW app.mv RENAME TO destination", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"destination_relation_conflict"`) || !strings.Contains(string(body), `"ddl.pg.alter_view.namespace.valid"`) || strings.Contains(string(body), "PGTableNamespace") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type viewOwnerAuditClient struct{ *mcpMetadataAuditTestClient }

func (viewOwnerAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "view_owner" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGTableOwner: &spec.PGTableOwnerState{Schema: req.Schema, Table: req.Qualifiers["table"], Target: spec.PGTableOwnerTarget{Kind: req.Qualifiers["kind"], Name: req.Name}, SourceKind: "v", SourceKindKnown: true}}, nil
}

func TestMCPPGViewOwnerTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := viewOwnerAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER VIEW app.mv OWNER TO missing_role", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"owner_role_absent"`) || !strings.Contains(string(body), `"ddl.pg.alter_view.owner.valid"`) || strings.Contains(string(body), "PGTableOwner") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type alterViewOptionsAuditClient struct{ *mcpMetadataAuditTestClient }

func (alterViewOptionsAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "alter_view_options" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGDropViewState: &spec.PGDropViewState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: req.Schema, Name: req.Name}}, Kind: "v"}}, nil
}

func TestMCPPGAlterViewOptionsTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := alterViewOptionsAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER VIEW app.mv RESET (security_invoker=true)", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"view_option_reset_has_value"`) || !strings.Contains(string(body), `"ddl.pg.alter_view.options.valid"`) || strings.Contains(string(body), "PGDropViewState") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type viewDefaultAuditClient struct{ *mcpMetadataAuditTestClient }

func (viewDefaultAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "view_column_default" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGViewDefault: &spec.PGViewDefaultState{Schema: req.Schema, View: req.Name, Column: req.Qualifiers["column"], Kind: "v", ColumnExists: true}}, nil
}

func TestMCPPGViewDefaultTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := viewDefaultAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER VIEW app.mv ALTER COLUMN id SET DEFAULT (SELECT 42)", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"view_default_reference_inapplicable"`) || !strings.Contains(string(body), `"ddl.pg.alter_view.column_default.valid"`) || strings.Contains(string(body), "PGViewDefault") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type materializedViewOptionsAuditClient struct{ *mcpMetadataAuditTestClient }

func (materializedViewOptionsAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "materialized_view_options" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGDropViewState: &spec.PGDropViewState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: req.Schema, Name: req.Name}}, Kind: "m", Heap: true}}, nil
}

func TestMCPPGMaterializedViewOptionsTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := materializedViewOptionsAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER MATERIALIZED VIEW app.mv SET (fillfactor=9)", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"materialized_view_option_out_of_range"`) || !strings.Contains(string(body), `"ddl.pg.alter_materialized_view.options.valid"`) || strings.Contains(string(body), "PGDropViewState") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type materializedViewExtensionAuditClient struct{ *mcpMetadataAuditTestClient }

func (materializedViewExtensionAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "materialized_view_extension" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGMaterializedViewExtension: &spec.PGMaterializedViewExtensionState{Schema: req.Schema, View: req.Name, Extension: req.Qualifiers["extension"], Kind: "m"}}, nil
}

func TestMCPPGMaterializedViewExtensionTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := materializedViewExtensionAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER MATERIALIZED VIEW app.mv NO DEPENDS ON EXTENSION missing_ext", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"materialized_view_extension_absent"`) || !strings.Contains(string(body), `"ddl.pg.alter_materialized_view.extension.valid"`) || strings.Contains(string(body), "PGMaterializedViewExtension") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type createViewOutputAuditClient struct{ *mcpMetadataAuditTestClient }

func (createViewOutputAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "create_view_output" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGCreateViewTarget: &spec.PGCreateViewTarget{Table: spec.Table{Schema: req.Schema, Name: req.Name}, SchemaExists: true, Kind: "v"}}, nil
}

func TestMCPPGCreateViewOutputTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := createViewOutputAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "CREATE VIEW app.mv AS SELECT 1 AS id", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"view_output_target_exists"`) || !strings.Contains(string(body), `"ddl.pg.create_view.output.valid"`) || strings.Contains(string(body), "PGCreateViewTarget") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type dropViewSetAuditClient struct{ *mcpMetadataAuditTestClient }

func (dropViewSetAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "drop_view_set" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGDropViewSet: &spec.PGDropViewSetState{Targets: req.PGDestructiveTargets, Position: 2, Reason: "drop_view_has_row_type_dependency"}}, nil
}

func TestMCPPGDropViewSet(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := dropViewSetAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "DROP VIEW app.mv,other.v RESTRICT", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"drop_view_has_row_type_dependency"`) || !strings.Contains(string(body), `"ddl.pg.drop_view.target.valid"`) || strings.Contains(string(body), "PGDropViewSet") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type partitionConstraintAuditClient struct{ *mcpMetadataAuditTestClient }

func (partitionConstraintAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "pg_partition_constraint" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGPartitionConstraint: &spec.PGPartitionConstraintState{Target: spec.Table{Schema: req.Schema, Name: req.Name}, Columns: []spec.PGColumnState{{Name: "id"}, {Name: "payload"}}, PartitionKeys: []spec.PGPartitionConstraintKey{{Name: "id", CollationOK: true, EqualityRead: true, EqualityOK: true}}}}, nil
}

func TestMCPPGPartitionConstraintTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := partitionConstraintAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "p"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER TABLE app.t ADD UNIQUE (payload) INCLUDE (id)", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"constraint_partition_column_missing"`) || !strings.Contains(string(body), `"ddl.pg.alter.constraint_columns.valid"`) || strings.Contains(string(body), "PGPartitionConstraint") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type createSchemaTargetAuditClient struct{ *mcpMetadataAuditTestClient }

func (createSchemaTargetAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "pg_create_schema" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGCreateSchema: &spec.PGCreateSchemaState{Target: spec.PGCreateSchemaTarget{Name: "app", Owner: spec.PGTableOwnerTarget{Kind: "named", Name: "absent_role"}}, NameKnown: true}}, nil
}

func TestMCPPGCreateSchemaTargetTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := createSchemaTargetAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "CREATE SCHEMA app AUTHORIZATION absent_role", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"schema_owner_absent"`) || !strings.Contains(string(body), `"ddl.pg.create_schema.target.valid"`) || strings.Contains(string(body), "PGCreateSchema") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type collationRefreshAuditClient struct{ *mcpMetadataAuditTestClient }

func (collationRefreshAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "pg_collation_refresh" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGCollationRefresh: &spec.PGCollationRefreshState{Target: spec.PGColumnCollation{Schema: req.Schema, Name: req.Name}, Known: true, Exists: true, Default: true}}, nil
}

func TestMCPPGCollationRefreshTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := collationRefreshAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER COLLATION pg_catalog.\"default\" REFRESH VERSION", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"collation_refresh_database_default"`) || !strings.Contains(string(body), `"ddl.pg.alter_collation.refresh.target.valid"`) || strings.Contains(string(body), "PGCollationRefresh") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type collationChangeAuditClient struct{ *mcpMetadataAuditTestClient }

func (collationChangeAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "pg_collation_change" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGCollationChange: &spec.PGCollationChangeState{Target: spec.PGCollationChangeTarget{Source: spec.PGColumnCollation{Schema: "app", Name: "source"}, Action: "rename", Destination: "occupied"}, SourceKnown: true, SourceExists: true, DestinationOccupied: true}}, nil
}

func TestMCPPGCollationChangeTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := collationChangeAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER COLLATION app.source RENAME TO occupied", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"collation_change_destination_occupied"`) || !strings.Contains(string(body), `"ddl.pg.alter_collation.change.target.valid"`) || strings.Contains(string(body), "PGCollationChange") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type indexAttachmentAuditClient struct{ *mcpMetadataAuditTestClient }

func (indexAttachmentAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "pg_index_attachment" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGIndexAttachment: &spec.PGIndexAttachmentState{Target: spec.PGIndexAttachment{Parent: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: req.Schema, Name: req.Name}}, Child: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: req.Qualifiers["child_schema"], Name: req.Qualifiers["child"]}}}, ParentKind: "I", ChildKind: "i", OtherParent: true}}, nil
}

func TestMCPPGIndexAttachmentTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := indexAttachmentAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER INDEX app.mv ATTACH PARTITION app.child", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"index_attachment_child_already_attached"`) || !strings.Contains(string(body), `"ddl.pg.alter_index.attach_partition.valid"`) || strings.Contains(string(body), "PGIndexAttachment") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type indexOptionsAuditClient struct{ *mcpMetadataAuditTestClient }

func (indexOptionsAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "pg_index_options" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGIndexOptions: &spec.PGIndexOptionsState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: req.Schema, Name: req.Name}}, Kind: "i", Method: "btree"}}, nil
}

func TestMCPPGIndexOptionsTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := indexOptionsAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER INDEX app.mv SET (fillfactor=9)", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"index_option_out_of_range"`) || !strings.Contains(string(body), `"ddl.pg.alter_index.options.valid"`) || strings.Contains(string(body), "PGIndexOptions") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type indexIncludeAuditClient struct{ *mcpMetadataAuditTestClient }

func (indexIncludeAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "pg_index_include" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGIndexInclude: &spec.PGIndexIncludeState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: req.Schema, Name: req.Name}}, Kind: "r", Method: "btree", Columns: []string{"id"}, System: []string{"ctid"}}}, nil
}

func TestMCPPGIndexIncludeTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := indexIncludeAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "CREATE INDEX idx ON app.t(id) INCLUDE (missing)", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"index_include_column_absent"`) || !strings.Contains(string(body), `"ddl.pg.create_index.include.valid"`) || strings.Contains(string(body), "PGIndexInclude") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type operatorFamilyTargetAuditClient struct{ *mcpMetadataAuditTestClient }

func (operatorFamilyTargetAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "pg_operator_family" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGOperatorFamily: &spec.PGOperatorFamilyState{Target: spec.PGOperatorFamilyTarget{Schema: req.Schema, Name: req.Name, Method: req.Qualifiers["method"]}, SchemaExists: true, MethodKind: "i", Exists: true}}, nil
}

func TestMCPPGOperatorFamilyTargetTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := operatorFamilyTargetAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "CREATE OPERATOR FAMILY app.family USING btree", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"operator_family_already_exists"`) || !strings.Contains(string(body), `"ddl.pg.create_operator_family.target.valid"`) || strings.Contains(string(body), "PGOperatorFamily") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type operatorFamilyMemberAuditClient struct{ *mcpMetadataAuditTestClient }

func (operatorFamilyMemberAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "pg_operator_family_member" || req.PGOperatorFamilyMember == nil {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGOperatorFamilyMember: &spec.PGOperatorFamilyMemberState{Target: spec.PGOperatorFamilyTarget{Schema: req.Schema, Name: req.Name, Method: req.Qualifiers["method"]}, Member: *req.PGOperatorFamilyMember, SchemaExists: true, MethodKind: "i", FamilyExists: true, BTree: true, OperandTypesRead: true, OperandTypesOK: true}}, nil
}

func TestMCPPGOperatorFamilyMemberTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := operatorFamilyMemberAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER OPERATOR FAMILY app.family USING btree DROP OPERATOR 1 (integer)", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"operator_family_slot_absent"`) || !strings.Contains(string(body), `"ddl.pg.alter_operator_family.member.valid"`) || strings.Contains(string(body), "PGOperatorFamilyMember") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type indexDestinationAuditClient struct{ *mcpMetadataAuditTestClient }

func (indexDestinationAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "pg_index_destination" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGIndexDestination: &spec.PGIndexDestinationState{Source: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: req.Schema, Name: req.Name}}, Action: "set_tablespace", Destination: "absent_sp", Kind: "i"}}, nil
}

func TestMCPPGIndexDestinationTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := indexDestinationAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER INDEX app.mv SET TABLESPACE absent_sp", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"index_tablespace_absent"`) || !strings.Contains(string(body), `"ddl.pg.alter_index.destination.valid"`) || strings.Contains(string(body), "PGIndexDestination") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type operatorFamilyChangeAuditClient struct{ *mcpMetadataAuditTestClient }

func (operatorFamilyChangeAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "pg_operator_family_change" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGOperatorFamilyChange: &spec.PGOperatorFamilyChangeState{Target: spec.PGOperatorFamilyChange{Source: spec.PGOperatorFamilyTarget{Schema: req.Schema, Name: req.Name, Method: req.Qualifiers["method"]}, Action: "rename", Destination: "occupied"}, SourceSchemaExists: true, MethodKind: "i", SourceExists: true, DestinationOccupied: true}}, nil
}

func TestMCPPGOperatorFamilyChangeTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := operatorFamilyChangeAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER OPERATOR FAMILY app.family USING btree RENAME TO occupied", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"operator_family_destination_occupied"`) || !strings.Contains(string(body), `"ddl.pg.alter_operator_family.change.target.valid"`) || strings.Contains(string(body), "PGOperatorFamilyChange") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type createLikeAuditClient struct{ *mcpMetadataAuditTestClient }

func (createLikeAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "pg_create_like" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGCreateLike: &spec.PGCreateSources{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: req.Schema, Name: req.Name}}, SchemaExists: true, Sources: []spec.PGCreateSource{{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "absent"}}}}}}, nil
}

func TestMCPPGCreateLikeTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := createLikeAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "CREATE TABLE app.mv (LIKE app.absent)", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"create_like_source_absent"`) || !strings.Contains(string(body), `"ddl.pg.create_table.like.target.valid"`) || strings.Contains(string(body), "PGCreateLike") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type createCommitAuditClient struct{ *mcpMetadataAuditTestClient }

func (createCommitAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "pg_create_destination" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGCreateDestination: &spec.PGCreateDestinationState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: req.Schema, Name: req.Name}}, SchemaExists: true}}, nil
}

func TestMCPPGCreateCommitTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := createCommitAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "CREATE TABLE app.mv (id integer) ON COMMIT DROP", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"create_commit_temporary_required"`) || !strings.Contains(string(body), `"ddl.pg.create_table.on_commit.valid"`) || strings.Contains(string(body), "PGCreateCommit") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type createTypedAuditClient struct{ *mcpMetadataAuditTestClient }

func (createTypedAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "pg_create_typed" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGCreateTyped: &spec.PGCreateTypedState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "mv"}}, Type: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "private_type"}}, TargetSchema: true, TypeSchema: true, TypeExists: true, Defined: true}}, nil
}

func TestMCPPGCreateTypedTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := createTypedAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "CREATE TABLE app.mv OF app.private_type", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"create_typed_type_not_standalone_composite"`) || !strings.Contains(string(body), `"ddl.pg.create_table.of.valid"`) || strings.Contains(string(body), "PGCreateTyped") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type createStorageAuditClient struct{ *mcpMetadataAuditTestClient }

func (createStorageAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "pg_create_storage" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGCreateStorage: &spec.PGCreateStorageState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: req.Name}}, SchemaExists: true, Method: req.Qualifiers["method"], MethodKind: "i"}}, nil
}

func TestMCPPGCreateStorageTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := createStorageAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "CREATE TABLE app.mv (id integer) USING btree", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"create_storage_method_not_table"`) || !strings.Contains(string(body), `"ddl.pg.create_table.storage.valid"`) || strings.Contains(string(body), "PGCreateStorage") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

func TestMCPPGCreateKeysTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := createCommitAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "CREATE TABLE app.mv (id integer, UNIQUE(absent))", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"create_keys_column_absent"`) || !strings.Contains(string(body), `"ddl.pg.create_table.keys.valid"`) || strings.Contains(string(body), "PGCreateKeys") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

func TestMCPPGCreateExpressionTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := createCommitAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "CREATE TABLE app.mv (id integer DEFAULT (SELECT 1))", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"create_default_disallowed_reference"`) || !strings.Contains(string(body), `"ddl.pg.create_table.expression.valid"`) || strings.Contains(string(body), "PGCreateExpression") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

func TestMCPPGCreateForeignKeyTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := createCommitAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: false, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "CREATE TABLE app.mv (id integer REFERENCES app.t(id))", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"foreign_key_reference_absent"`) || !strings.Contains(string(body), `"ddl.pg.create_table.foreign_key.valid"`) || strings.Contains(string(body), "PGCreateForeignKey") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type createInheritanceAuditClient struct{ *mcpMetadataAuditTestClient }

func (createInheritanceAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "pg_create_inheritance" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGCreateInheritance: &spec.PGCreateSources{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: req.Schema, Name: req.Name}}, SchemaExists: true, Sources: []spec.PGCreateSource{{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "absent"}}}}}}, nil
}

func TestMCPPGCreateInheritanceTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := createInheritanceAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "CREATE TABLE app.mv () INHERITS (app.absent)", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"create_inheritance_parent_absent"`) || !strings.Contains(string(body), `"ddl.pg.create_table.inherits.valid"`) || strings.Contains(string(body), "PGCreateInheritance") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

func TestMCPPGCreateColumnsTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := createCommitAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "CREATE TABLE app.mv (ctid integer)", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"create_column_system_name"`) || !strings.Contains(string(body), `"ddl.pg.create_table.columns.valid"`) || strings.Contains(string(body), "PGCreateColumns") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type alterEnumAuditClient struct{ *mcpMetadataAuditTestClient }

func (alterEnumAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "pg_alter_enum" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGEnum: &spec.PGEnumState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "mood"}}, SchemaExists: true, TypeExists: true, Defined: true, TypeKind: "e", Encoding: "UTF8", LabelBytes: 63, Labels: []string{"private_catalog_label"}}}, nil
}

func TestMCPPGAlterEnumTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := alterEnumAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER TYPE app.mood RENAME VALUE 'missing' TO 'new'", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"alter_enum_old_label_absent"`) || !strings.Contains(string(body), `"ddl.pg.alter_type.enum.valid"`) || strings.Contains(string(body), "private_catalog_label") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type compositeAuditClient struct{ *mcpMetadataAuditTestClient }

func (compositeAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "pg_composite_type" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGComposite: &spec.PGCompositeState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "pair"}}, SchemaExists: true, RelationKind: "c", Attributes: []string{"private_catalog_label"}}}, nil
}

func TestMCPPGCompositeTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := compositeAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER TYPE app.pair DROP ATTRIBUTE missing", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"composite_attribute_absent"`) || !strings.Contains(string(body), `"ddl.pg.type.composite.valid"`) || strings.Contains(string(body), "private_catalog_label") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type typeChangeAuditClient struct{ *mcpMetadataAuditTestClient }

func (typeChangeAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "pg_type_change" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGTypeChange: &spec.PGTypeChangeState{Target: spec.PGTypeChange{Source: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "pair"}}, Action: "rename", Destination: "used"}, SourceExists: true, Defined: true, Composite: true, DestinationRelation: true}}, nil
}

func TestMCPPGTypeChangeTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := typeChangeAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER TYPE app.pair RENAME TO used", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"alter_type_relation_occupied"`) || !strings.Contains(string(body), `"ddl.pg.alter_type.change.target.valid"`) || strings.Contains(string(body), "DestinationRelation") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type operatorChangeAuditClient struct{ *mcpMetadataAuditTestClient }

func (operatorChangeAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "pg_operator_change" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGOperatorChange: &spec.PGOperatorChangeState{Target: spec.PGOperatorChange{Source: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "==="}}, Action: "set_schema", Destination: "used", Signature: true, Left: spec.PGColumnTypeTarget{Schema: "pg_catalog", Name: "int4"}, Right: spec.PGColumnTypeTarget{Schema: "pg_catalog", Name: "int4"}}, OperandSchema: true, OperandTypes: true, SourceSchema: true, SourceExists: true, DestinationSchema: true, DestinationOccupied: true}}, nil
}

func TestMCPPGOperatorChangeTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := operatorChangeAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER OPERATOR app.=== (pg_catalog.int4, pg_catalog.int4) SET SCHEMA used", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"alter_operator_destination_occupied"`) || !strings.Contains(string(body), `"ddl.pg.alter_operator.change.target.valid"`) || strings.Contains(string(body), "DestinationOccupied") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type createRangeAuditClient struct{ *mcpMetadataAuditTestClient }

func (createRangeAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "pg_create_range" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGEnum: &spec.PGEnumState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "mood"}}, SchemaExists: true, TypeExists: true, Defined: true, TypeKind: "e", Encoding: "UTF8", LabelBytes: 63, Labels: []string{"private_catalog_label"}}}, nil
}

func TestMCPPGCreateRangeTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := createRangeAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "CREATE TYPE app.mood AS RANGE (unknown_option=1)", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"range_option_unknown"`) || !strings.Contains(string(body), `"ddl.pg.create_type.range.valid"`) || strings.Contains(string(body), "private_catalog_label") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type createBaseTypeAuditClient struct{ *mcpMetadataAuditTestClient }

func (createBaseTypeAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "pg_base_type" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGEnum: &spec.PGEnumState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "mood"}}, SchemaExists: true, TypeExists: true, Defined: false, Encoding: "UTF8", LabelBytes: 63, Labels: []string{"private_catalog_label"}}}, nil
}

func TestMCPPGCreateBaseTypeTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := createBaseTypeAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "CREATE TYPE app.mood (OUTPUT=pg_catalog.int4out)", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"base_type_io_required"`) || !strings.Contains(string(body), `"ddl.pg.type.base.valid"`) || strings.Contains(string(body), "private_catalog_label") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type policyChangeAuditClient struct{ *mcpMetadataAuditTestClient }

func (policyChangeAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "pg_policy_change" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGPolicyChange: &spec.PGPolicyChangeState{Table: spec.Table{Schema: "app", Name: "t"}, Name: "read", RelationKind: "r", PolicyCommand: "r", Columns: []string{"private_policy_column"}}}, nil
}

func TestMCPPGPolicyChangeTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := policyChangeAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER POLICY read ON app.t WITH CHECK (true)", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"alter_policy_with_check_inapplicable"`) || !strings.Contains(string(body), `"ddl.pg.alter_policy.target.valid"`) || strings.Contains(string(body), "private_policy_column") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type sequenceChangeAuditClient struct{ *mcpMetadataAuditTestClient }

func (sequenceChangeAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "pg_sequence_change" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGSequenceChange: &spec.PGSequenceChangeState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "s"}}, SchemaExists: true}}, nil
}

func TestMCPPGSequenceChangeTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := sequenceChangeAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "CREATE SEQUENCE app.s INCREMENT 0", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"sequence_increment_zero"`) || !strings.Contains(string(body), `"ddl.pg.sequence.declaration.valid"`) || strings.Contains(string(body), "PGSequenceChangeState") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type wrapperChangeAuditClient struct{ *mcpMetadataAuditTestClient }

func (wrapperChangeAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "pg_wrapper_change" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGWrapperChange: &spec.PGWrapperChangeState{Name: "wrapper", Exists: true, Options: []string{"present", "private_wrapper_option"}}}, nil
}

func TestMCPPGWrapperChangeTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := wrapperChangeAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER FOREIGN DATA WRAPPER wrapper OPTIONS (DROP missing)", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"foreign_option_absent"`) || !strings.Contains(string(body), `"ddl.pg.alter_foreign_data_wrapper.target.valid"`) || strings.Contains(string(body), "private_wrapper_option") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type foreignTableChangeAuditClient struct{ *mcpMetadataAuditTestClient }

func (foreignTableChangeAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "pg_foreign_table_change" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGForeignTableChange: &spec.PGForeignTableChangeState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "ft"}}, Action: "drop_column", Column: "missing", RelationKind: "f"}}, nil
}

func TestMCPPGForeignTableChangeTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := foreignTableChangeAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER FOREIGN TABLE app.ft DROP COLUMN missing", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"foreign_table_column_absent"`) || !strings.Contains(string(body), `"ddl.pg.alter_foreign_table.target.valid"`) || strings.Contains(string(body), "PGForeignTableChangeState") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type textSearchChangeAuditClient struct{ *mcpMetadataAuditTestClient }

func (textSearchChangeAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "pg_text_search_change" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGTextSearchChange: &spec.PGTextSearchChangeState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "dict"}}, Kind: "dictionary", Action: "set_dictionary_options", SourceSchema: true, SourceExists: true, SimpleTemplate: true}}, nil
}

func TestMCPPGTextSearchChangeTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := textSearchChangeAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER TEXT SEARCH DICTIONARY app.dict (unknown_option = 1)", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"text_search_dictionary_option_unknown"`) || !strings.Contains(string(body), `"ddl.pg.text_search.change.target.valid"`) || strings.Contains(string(body), "PGTextSearchChangeState") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type subscriptionChangeAuditClient struct{ *mcpMetadataAuditTestClient }

func (subscriptionChangeAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "pg_subscription_change" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGSubscriptionChange: &spec.PGSubscriptionChangeState{Name: "app_sub", Exists: true}}, nil
}

func TestMCPPGSubscriptionChangeTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := subscriptionChangeAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "ALTER SUBSCRIPTION app_sub ENABLE", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"subscription_slot_required"`) || !strings.Contains(string(body), `"ddl.pg.alter_subscription.target.valid"`) || strings.Contains(string(body), "PGSubscriptionChangeState") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type languageDeclarationAuditClient struct{ *mcpMetadataAuditTestClient }

func (languageDeclarationAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "pg_language_declaration" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGLanguage: &spec.PGLanguageState{Name: "fixture_lang", Database: "postgres", Routines: []spec.PGLanguageRoutineState{{Target: spec.PGLanguageRoutine{Kind: "handler", Schema: "app", Name: "missing_handler"}}}}}, nil
}

func TestMCPPGLanguageTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := languageDeclarationAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "CREATE LANGUAGE fixture_lang HANDLER app.missing_handler", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"language_callback_absent"`) || !strings.Contains(string(body), `"ddl.pg.create_language.handler.valid"`) || strings.Contains(string(body), "PGLanguageState") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}

type grantDeclarationAuditClient struct{ *mcpMetadataAuditTestClient }

func (grantDeclarationAuditClient) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	if req.Type != "pg_grant_declaration" {
		return nil, nil
	}
	return &spec.ObjectSnapshot{PGGrant: &spec.PGGrantState{ObjectKind: "table", Targets: []spec.PGGrantTargetState{{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "fixture"}}, Exists: true, Kind: "r", Columns: []string{"catalog_only_column"}}}, Grantees: []spec.PGTableOwnerTarget{{Kind: "public"}}, GranteeExists: []bool{true}}}, nil
}

func TestMCPPGGrantDeclarationTarget(t *testing.T) {
	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	client := grantDeclarationAuditClient{&mcpMetadataAuditTestClient{version: "PostgreSQL 17.7", snapshot: &spec.TableSnapshot{
		Schema: "app", Exists: true, Table: &spec.Table{Name: "t"}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
	previous := prepareMetadataAudit
	prepareMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareMetadataAudit = previous })
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "audit_sql", Arguments: map[string]any{
		"sql": "GRANT SELECT (missing_column) ON TABLE app.fixture TO PUBLIC", "dialect": "postgresql",
		"connection": map[string]any{"host": "127.0.0.1", "user": "root", "schema": "app"},
	}})
	if err != nil || result.IsError {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	body, err := json.Marshal(result.StructuredContent)
	if err != nil || !strings.Contains(string(body), `"grant_column_absent"`) || !strings.Contains(string(body), `"ddl.pg.grant.declaration.valid"`) || strings.Contains(string(body), "catalog_only_column") {
		t.Fatalf("unexpected response: %s %v", body, err)
	}
}
