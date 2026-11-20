//go:build postgresql

package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	auditmeta "github.com/Fanduzi/DeltaScope/internal/application/auditmeta"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/Fanduzi/DeltaScope/internal/infrastructure/runtimeconfig"
)

func TestHTTPAuditProjectsAmbiguousObjectMetadata(t *testing.T) {
	testReg := newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) {
		c.Dialect = "postgresql"
		c.Port = 5432
		c.Database = "app"
	})
	handler, err := NewHandler("", "test-build", WithRegistry(testReg))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	previous := prepareHTTPMetadataAudit
	client := &metadataAuditTestClient{
		detectDialect: spec.DialectPostgreSQL,
		objectSnapshot: &spec.ObjectSnapshot{
			Schema:              "public",
			Type:                "publication",
			Name:                "my_pub",
			Status:              spec.MetadataStatusAmbiguous,
			Exists:              false,
			AmbiguousCandidates: []string{"app.my_pub", "public.my_pub"},
		},
	}
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{
			Client:        client,
			Dialect:       spec.DialectPostgreSQL,
			Schema:        "public",
			DialectSource: "request",
			SchemaSource:  "inferred",
		}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })

	body := `{"sql":"CREATE PUBLICATION my_pub FOR ALL TABLES","dialect":"postgresql","connection_id":"test-pg","schema":"public"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-value")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	statements, ok := payload["statements"].([]any)
	if !ok || len(statements) == 0 {
		t.Fatalf("expected statements, got %#v", payload["statements"])
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
		if strings.HasPrefix(f["rule_id"].(string), "ddl.pg.create_publication") {
			findingMeta, _ = f["metadata"].(map[string]any)
			break
		}
	}
	if findingMeta == nil {
		t.Fatalf("expected create_publication finding, got %#v", findings)
	}

	if findingMeta["metadata_status"] != "ambiguous" {
		t.Fatalf("expected metadata_status=ambiguous, got %#v", findingMeta["metadata_status"])
	}
	if findingMeta["metadata_object_type"] != "publication" {
		t.Fatalf("expected metadata_object_type=publication, got %#v", findingMeta["metadata_object_type"])
	}
	if findingMeta["metadata_object_name"] != "my_pub" {
		t.Fatalf("expected metadata_object_name=my_pub, got %#v", findingMeta["metadata_object_name"])
	}

	candidates, ok := findingMeta["metadata_ambiguous_candidates"].([]any)
	if !ok || len(candidates) != 2 {
		t.Fatalf("expected 2 ambiguous candidates, got %#v", findingMeta["metadata_ambiguous_candidates"])
	}

	sensitiveKeys := []string{"password", "secret", "conninfo", "connection", "host", "port", "options", "query", "body", "definition", "comment", "label"}
	for _, key := range sensitiveKeys {
		if _, ok := findingMeta["metadata_"+key]; ok {
			t.Fatalf("sensitive key metadata_%q leaked into finding metadata", key)
		}
	}
}

func TestHTTPReplicaIdentityAttachment(t *testing.T) {
	reg := newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) { c.Dialect, c.Database = "postgresql", "app" })
	h, err := NewHandler("", "test-build", WithRegistry(reg))
	if err != nil {
		t.Fatal(err)
	}
	client := &metadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
			PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}},
		objectSnapshot: &spec.ObjectSnapshot{PGReplicaIdentity: &spec.PGReplicaIdentityIndex{Schema: "app", Table: "t", Name: "missing"}}}
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", strings.NewReader(`{"sql":"ALTER TABLE app.t REPLICA IDENTITY USING INDEX missing","dialect":"postgresql","connection_id":"test-pg"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-value")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"ddl.pg.alter.replica_identity_using_index.valid"`) || !strings.Contains(body, `"index_absent"`) || strings.Contains(body, "PGReplicaIdentity") || len(client.objectCalls) != 1 {
		t.Fatalf("code=%d response=%s calls=%+v", rec.Code, body, client.objectCalls)
	}
}

func TestHTTPPGTableReloptions(t *testing.T) {
	reg := newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) { c.Dialect, c.Database = "postgresql", "app" })
	h, err := NewHandler("", "test-build", WithRegistry(reg))
	if err != nil {
		t.Fatal(err)
	}
	client := &metadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
			PGRelation: spec.PGRelationState{Name: "t", Kind: "r", Heap: true}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", strings.NewReader(`{"sql":"ALTER TABLE app.t SET (fillfactor=9)","dialect":"postgresql","connection_id":"test-pg"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-value")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"ddl.pg.alter.reloptions.valid"`) || !strings.Contains(body, `"table_reloption_out_of_range"`) || strings.Contains(body, "PGTableReloptions") {
		t.Fatalf("code=%d response=%s", rec.Code, body)
	}
}

func TestHTTPTableTablespace(t *testing.T) {
	reg := newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) { c.Dialect, c.Database = "postgresql", "app" })
	h, err := NewHandler("", "test-build", WithRegistry(reg))
	if err != nil {
		t.Fatal(err)
	}
	client := &metadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
			PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}},
		objectSnapshot: &spec.ObjectSnapshot{PGTableTablespace: &spec.PGTableTablespace{Schema: "app", Table: "t", Name: "missing"}}}
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", strings.NewReader(`{"sql":"ALTER TABLE app.t SET TABLESPACE missing","dialect":"postgresql","connection_id":"test-pg"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-value")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"ddl.pg.alter.tablespace.target.valid"`) || !strings.Contains(body, `"tablespace_absent"`) || strings.Contains(body, "PGTableTablespace") || len(client.objectCalls) != 1 {
		t.Fatalf("code=%d response=%s calls=%+v", rec.Code, body, client.objectCalls)
	}
}

func TestHTTPTableAccessMethod(t *testing.T) {
	reg := newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) { c.Dialect, c.Database = "postgresql", "app" })
	h, err := NewHandler("", "test-build", WithRegistry(reg))
	if err != nil {
		t.Fatal(err)
	}
	client := &metadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
			PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}},
		objectSnapshot: &spec.ObjectSnapshot{PGTableAccessMethod: &spec.PGTableAccessMethod{Schema: "app", Table: "t", Name: "missing"}}}
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", strings.NewReader(`{"sql":"ALTER TABLE app.t SET ACCESS METHOD missing","dialect":"postgresql","connection_id":"test-pg"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-value")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"ddl.pg.alter.access_method.target.valid"`) || !strings.Contains(body, `"access_method_absent"`) || strings.Contains(body, "PGTableAccessMethod") || len(client.objectCalls) != 1 {
		t.Fatalf("code=%d response=%s calls=%+v", rec.Code, body, client.objectCalls)
	}
}

func TestHTTPTablePersistence(t *testing.T) {
	reg := newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) { c.Dialect, c.Database = "postgresql", "app" })
	h, err := NewHandler("", "test-build", WithRegistry(reg))
	if err != nil {
		t.Fatal(err)
	}
	client := &metadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
			PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}},
		objectSnapshot: &spec.ObjectSnapshot{PGTablePersistence: &spec.PGTablePersistence{Schema: "app", Table: "t", Current: "p", DirectPublication: true}}}
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", strings.NewReader(`{"sql":"ALTER TABLE app.t SET UNLOGGED","dialect":"postgresql","connection_id":"test-pg"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-value")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"ddl.pg.alter.persistence.transition.valid"`) || !strings.Contains(body, `"persistence_direct_publication"`) || strings.Contains(body, "PGTablePersistence") || len(client.objectCalls) != 1 {
		t.Fatalf("code=%d response=%s calls=%+v", rec.Code, body, client.objectCalls)
	}
}

func TestHTTPColumnTypeSource(t *testing.T) {
	reg := newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) { c.Dialect, c.Database = "postgresql", "app" })
	h, err := NewHandler("", "test-build", WithRegistry(reg))
	if err != nil {
		t.Fatal(err)
	}
	client := &metadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
			PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{{Name: "id"}}, SystemColumns: []spec.PGColumnState{{Name: "ctid", System: true}}}}}
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", strings.NewReader(`{"sql":"ALTER TABLE app.t ALTER COLUMN ctid TYPE text","dialect":"postgresql","connection_id":"test-pg"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-value")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"ddl.pg.alter.column_type.source.valid"`) || !strings.Contains(body, `"system_column_inapplicable"`) || strings.Contains(body, "PGColumns") || len(client.objectCalls) != 0 {
		t.Fatalf("code=%d response=%s calls=%+v", rec.Code, body, client.objectCalls)
	}
}

func TestHTTPTableNamespace(t *testing.T) {
	reg := newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) { c.Dialect, c.Database = "postgresql", "app" })
	h, err := NewHandler("", "test-build", WithRegistry(reg))
	if err != nil {
		t.Fatal(err)
	}
	client := &metadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
			PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}},
		objectSnapshot: &spec.ObjectSnapshot{PGTableNamespace: &spec.PGTableNamespace{Schema: "app", Table: "t", Action: "set_schema", Destination: "missing"}}}
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", strings.NewReader(`{"sql":"ALTER TABLE app.t SET SCHEMA missing","dialect":"postgresql","connection_id":"test-pg"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-value")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"ddl.pg.alter.set_schema.target.valid"`) || !strings.Contains(body, `"destination_schema_absent"`) || strings.Contains(body, "PGTableNamespace") || len(client.objectCalls) != 1 {
		t.Fatalf("code=%d response=%s calls=%+v", rec.Code, body, client.objectCalls)
	}
}

func TestHTTPTableOwner(t *testing.T) {
	reg := newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) { c.Dialect, c.Database = "postgresql", "app" })
	h, err := NewHandler("", "test-build", WithRegistry(reg))
	if err != nil {
		t.Fatal(err)
	}
	client := &metadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
			PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}},
		objectSnapshot: &spec.ObjectSnapshot{PGTableOwner: &spec.PGTableOwnerState{Schema: "app", Table: "t", Target: spec.PGTableOwnerTarget{Kind: "named", Name: "missing"}}}}
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", strings.NewReader(`{"sql":"ALTER TABLE app.t OWNER TO missing","dialect":"postgresql","connection_id":"test-pg"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-value")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"ddl.pg.alter.owner.target.valid"`) || !strings.Contains(body, `"owner_role_absent"`) || strings.Contains(body, "PGTableOwner") || len(client.objectCalls) != 1 {
		t.Fatalf("code=%d response=%s calls=%+v", rec.Code, body, client.objectCalls)
	}
}

func TestHTTPBulkTablespace(t *testing.T) {
	reg := newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) { c.Dialect, c.Database = "postgresql", "app" })
	h, err := NewHandler("", "test-build", WithRegistry(reg))
	if err != nil {
		t.Fatal(err)
	}
	client := &metadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL,
		objectSnapshot: &spec.ObjectSnapshot{PGBulkTablespace: &spec.PGBulkTablespaceState{Target: spec.PGBulkTablespaceTarget{Source: "left_space", Destination: "missing"}, SourceExists: true}}}
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", strings.NewReader(`{"sql":"ALTER TABLE ALL IN TABLESPACE left_space SET TABLESPACE missing","dialect":"postgresql","connection_id":"test-pg"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-value")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"ddl.pg.alter.move_all_tablespace.target.valid"`) || !strings.Contains(body, `"destination_tablespace_absent"`) || strings.Contains(body, "PGBulkTablespace") || len(client.objectCalls) != 1 {
		t.Fatalf("code=%d response=%s calls=%+v", rec.Code, body, client.objectCalls)
	}
}

func TestHTTPColumnType(t *testing.T) {
	reg := newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) { c.Dialect, c.Database = "postgresql", "app" })
	h, err := NewHandler("", "test-build", WithRegistry(reg))
	if err != nil {
		t.Fatal(err)
	}
	client := &metadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
			PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{{Name: "id"}}}},
		objectSnapshot: &spec.ObjectSnapshot{PGColumnType: &spec.PGColumnTypeState{Target: spec.PGColumnTypeTarget{Schema: "pg_catalog", Name: "cstring", Array: true}, SchemaExists: true, BaseExists: true, Resolved: true, Defined: true, Pseudo: true}}}
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", strings.NewReader(`{"sql":"ALTER TABLE app.t ALTER COLUMN id TYPE pg_catalog.cstring[]","dialect":"postgresql","connection_id":"test-pg"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-value")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"ddl.pg.alter.column_type.destination.valid"`) || !strings.Contains(body, `"destination_pseudo_type_inapplicable"`) || strings.Contains(body, "PGColumnType") || len(client.objectCalls) != 1 {
		t.Fatalf("code=%d response=%s calls=%+v", rec.Code, body, client.objectCalls)
	}
}

func TestHTTPConstraintUsingIndex(t *testing.T) {
	reg := newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) { c.Dialect, c.Database = "postgresql", "app" })
	h, err := NewHandler("", "test-build", WithRegistry(reg))
	if err != nil {
		t.Fatal(err)
	}
	client := &metadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
			PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{{Name: "id"}}}},
		objectSnapshot: &spec.ObjectSnapshot{PGConstraintUsingIndex: &spec.PGConstraintUsingIndex{Schema: "app", Table: "t", Name: "exact_index", Kind: "i", IndexKnown: true, OnTable: true, Unique: true, BTree: true}}}
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", strings.NewReader(`{"sql":"ALTER TABLE app.t ADD CONSTRAINT pending UNIQUE USING INDEX exact_index","dialect":"postgresql","connection_id":"test-pg"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-value")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"ddl.pg.alter.constraint_using_index.valid"`) || !strings.Contains(body, `"constraint_index_invalid"`) || strings.Contains(body, "PGConstraintUsingIndex") || len(client.objectCalls) != 1 {
		t.Fatalf("code=%d response=%s calls=%+v", rec.Code, body, client.objectCalls)
	}
}
