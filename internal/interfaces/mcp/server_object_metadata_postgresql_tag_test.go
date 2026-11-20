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
