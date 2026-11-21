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

func TestHTTPForeignKeyReferenceKeys(t *testing.T) {
	reg := newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) { c.Dialect, c.Database = "postgresql", "app" })
	h, err := NewHandler("", "test-build", WithRegistry(reg))
	if err != nil {
		t.Fatal(err)
	}
	client := &metadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
			PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{{Name: "id"}}}},
		objectSnapshot: &spec.ObjectSnapshot{PGForeignKeyKeys: &spec.PGForeignKeyKeys{Schema: "app", Table: "t"}}}
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", strings.NewReader(`{"sql":"ALTER TABLE app.t ADD FOREIGN KEY (id) REFERENCES app.t(id) NOT VALID","dialect":"postgresql","connection_id":"test-pg"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-value")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"ddl.pg.alter.add_foreign_key.targets.valid"`) || !strings.Contains(body, `"foreign_key_reference_unique_key_absent"`) || strings.Contains(body, "PGForeignKeyKeys") || len(client.objectCalls) != 1 {
		t.Fatalf("code=%d response=%s calls=%+v", rec.Code, body, client.objectCalls)
	}
}

func TestHTTPDropConstraintReference(t *testing.T) {
	reg := newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) { c.Dialect, c.Database = "postgresql", "app" })
	h, err := NewHandler("", "test-build", WithRegistry(reg))
	if err != nil {
		t.Fatal(err)
	}
	client := &metadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
			PGConstraints: []spec.PGConstraintState{{Name: "pk", Type: "p", Validated: true}}, PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{{Name: "id"}}}},
		objectSnapshot: &spec.ObjectSnapshot{PGDropConstraintReference: &spec.PGDropConstraintReference{Schema: "app", Table: "t", Name: "pk", Kind: "p", Blocked: true}}}
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", strings.NewReader(`{"sql":"ALTER TABLE app.t DROP CONSTRAINT pk RESTRICT","dialect":"postgresql","connection_id":"test-pg"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-value")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"ddl.pg.alter.drop_constraint.target.require"`) || !strings.Contains(body, `"constraint_referenced_by_foreign_key"`) || strings.Contains(body, "PGDropConstraintReference") || len(client.objectCalls) != 1 {
		t.Fatalf("code=%d response=%s calls=%+v", rec.Code, body, client.objectCalls)
	}
}

func TestHTTPDropColumnDependency(t *testing.T) {
	reg := newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) { c.Dialect, c.Database = "postgresql", "app" })
	h, err := NewHandler("", "test-build", WithRegistry(reg))
	if err != nil {
		t.Fatal(err)
	}
	client := &metadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
			PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{{Name: "id"}}}},
		objectSnapshot: &spec.ObjectSnapshot{PGDropColumnDependency: &spec.PGDropColumnDependency{Schema: "app", Table: "t", Column: "id", Kind: "foreign_key"}}}
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", strings.NewReader(`{"sql":"ALTER TABLE app.t DROP COLUMN id RESTRICT","dialect":"postgresql","connection_id":"test-pg"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-value")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"ddl.alter.drop_column.exists.require"`) || !strings.Contains(body, `"column_referenced_by_foreign_key"`) || strings.Contains(body, "PGDropColumnDependency") || len(client.objectCalls) != 1 {
		t.Fatalf("code=%d response=%s calls=%+v", rec.Code, body, client.objectCalls)
	}
}

func TestHTTPTableNamespaceCompanion(t *testing.T) {
	reg := newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) { c.Dialect, c.Database = "postgresql", "app" })
	h, err := NewHandler("", "test-build", WithRegistry(reg))
	if err != nil {
		t.Fatal(err)
	}
	client := &metadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
			PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}},
		objectSnapshot: &spec.ObjectSnapshot{PGTableNamespace: &spec.PGTableNamespace{Schema: "app", Table: "t", Action: "set_schema", Destination: "dst", SchemaExists: true, RowTypeConflictKnown: true, ArrayTypeConflictKnown: true, IndexConflict: true}}}
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", strings.NewReader(`{"sql":"ALTER TABLE app.t SET SCHEMA dst","dialect":"postgresql","connection_id":"test-pg"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-value")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"ddl.pg.alter.set_schema.target.valid"`) || !strings.Contains(body, `"destination_index_conflict"`) || strings.Contains(body, "PGTableNamespace") || len(client.objectCalls) != 1 {
		t.Fatalf("code=%d response=%s calls=%+v", rec.Code, body, client.objectCalls)
	}
}

func TestHTTPSetWithoutOIDs(t *testing.T) {
	reg := newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) { c.Dialect, c.Database = "postgresql", "app" })
	h, err := NewHandler("", "test-build", WithRegistry(reg))
	if err != nil {
		t.Fatal(err)
	}
	client := &metadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
			PGRelation: spec.PGRelationState{Name: "t", Kind: "v"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", strings.NewReader(`{"sql":"ALTER TABLE IF EXISTS app.t SET WITHOUT OIDS","dialect":"postgresql","connection_id":"test-pg"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-value")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"ddl.table.exists.alter.require"`) || !strings.Contains(body, `"relation_wrong_kind"`) || strings.Contains(body, "PGTableNamespace") || len(client.objectCalls) != 0 {
		t.Fatalf("code=%d response=%s calls=%+v", rec.Code, body, client.objectCalls)
	}
}

func TestHTTPColumnTypeAssignment(t *testing.T) {
	reg := newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) { c.Dialect, c.Database = "postgresql", "app" })
	h, err := NewHandler("", "test-build", WithRegistry(reg))
	if err != nil {
		t.Fatal(err)
	}
	client := &metadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
			PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{{Name: "id"}}}},
		objectSnapshot: &spec.ObjectSnapshot{PGColumnType: &spec.PGColumnTypeState{Target: spec.PGColumnTypeTarget{Schema: "pg_catalog", Name: "int4"}, SchemaExists: true, BaseExists: true, Resolved: true, Defined: true, AssignmentKnown: true, SourceSchema: "app", SourceTable: "t", SourceColumn: "id"}}}
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", strings.NewReader(`{"sql":"ALTER TABLE app.t ALTER COLUMN id TYPE pg_catalog.int4","dialect":"postgresql","connection_id":"test-pg"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-value")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"ddl.pg.alter.column_type.destination.valid"`) || !strings.Contains(body, `"destination_type_assignment_unavailable"`) || strings.Contains(body, "PGColumnType") || len(client.objectCalls) != 1 {
		t.Fatalf("code=%d response=%s calls=%+v", rec.Code, body, client.objectCalls)
	}
}

func TestHTTPConstraintDefaultOpclass(t *testing.T) {
	reg := newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) { c.Dialect, c.Database = "postgresql", "app" })
	h, err := NewHandler("", "test-build", WithRegistry(reg))
	if err != nil {
		t.Fatal(err)
	}
	client := &metadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
			PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{{Name: "id"}}}},
		objectSnapshot: &spec.ObjectSnapshot{PGConstraintUsingIndex: &spec.PGConstraintUsingIndex{Schema: "app", Table: "t", Name: "exact_index", Kind: "i", IndexKnown: true, OnTable: true, Unique: true, BTree: true, Valid: true, KeyDefaults: true, KeyDefaultsKnown: true, OpclassesDefaultKnown: true}}}
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
	if rec.Code != http.StatusOK || !strings.Contains(body, `"ddl.pg.alter.constraint_using_index.valid"`) || !strings.Contains(body, `"constraint_index_nondefault_opclass"`) || strings.Contains(body, "PGConstraintUsingIndex") || len(client.objectCalls) != 1 {
		t.Fatalf("code=%d response=%s calls=%+v", rec.Code, body, client.objectCalls)
	}
}

func TestHTTPForeignKeyDeleteSetColumns(t *testing.T) {
	reg := newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) { c.Dialect, c.Database = "postgresql", "app" })
	h, err := NewHandler("", "test-build", WithRegistry(reg))
	if err != nil {
		t.Fatal(err)
	}
	client := &metadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
			PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{{Name: "id"}, {Name: "extra"}}}},
		objectSnapshot: &spec.ObjectSnapshot{PGForeignKeyKeys: &spec.PGForeignKeyKeys{Schema: "app", Table: "t"}}}
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", strings.NewReader(`{"sql":"ALTER TABLE app.t ADD FOREIGN KEY (id) REFERENCES app.t(id) ON DELETE SET NULL (extra) NOT VALID","dialect":"postgresql","connection_id":"test-pg"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-value")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"ddl.pg.alter.add_foreign_key.targets.valid"`) || !strings.Contains(body, `"foreign_key_delete_set_column_outside_key"`) || strings.Contains(body, "DeleteSetColumns") || len(client.objectCalls) != 1 {
		t.Fatalf("code=%d response=%s calls=%+v", rec.Code, body, client.objectCalls)
	}
}

func TestHTTPInheritedColumnDrop(t *testing.T) {
	reg := newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) { c.Dialect, c.Database = "postgresql", "app" })
	h, err := NewHandler("", "test-build", WithRegistry(reg))
	if err != nil {
		t.Fatal(err)
	}
	client := &metadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
			PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: false, Columns: []spec.PGColumnState{{Name: "id"}}}},
		objectSnapshot: &spec.ObjectSnapshot{PGColumnTraversal: &spec.PGColumnTraversal{Schema: "app", Table: "t", Column: "id", Inherited: true}}}
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", strings.NewReader(`{"sql":"ALTER TABLE app.t DROP COLUMN IF EXISTS id CASCADE","dialect":"postgresql","connection_id":"test-pg"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-value")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"ddl.alter.drop_column.exists.require"`) || !strings.Contains(body, `"column_inherited"`) || strings.Contains(body, "PGColumnTraversal") || len(client.objectCalls) != 1 {
		t.Fatalf("code=%d response=%s calls=%+v", rec.Code, body, client.objectCalls)
	}
}

func TestHTTPInheritedDropExpression(t *testing.T) {
	reg := newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) { c.Dialect, c.Database = "postgresql", "app" })
	h, err := NewHandler("", "test-build", WithRegistry(reg))
	if err != nil {
		t.Fatal(err)
	}
	client := &metadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
			PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: false, Columns: []spec.PGColumnState{{Name: "id"}}}},
		objectSnapshot: &spec.ObjectSnapshot{PGColumnTraversal: &spec.PGColumnTraversal{Schema: "app", Table: "t", Column: "id", Inherited: true}}}
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", strings.NewReader(`{"sql":"ALTER TABLE app.t ALTER COLUMN id DROP EXPRESSION IF EXISTS","dialect":"postgresql","connection_id":"test-pg"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-value")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"ddl.pg.alter.column.generation.transition.valid"`) || !strings.Contains(body, `"drop_expression_inherited_column"`) || strings.Contains(body, "PGColumnTraversal") || len(client.objectCalls) != 1 {
		t.Fatalf("code=%d response=%s calls=%+v", rec.Code, body, client.objectCalls)
	}
}

func TestHTTPForeignKeyScalarEquality(t *testing.T) {
	reg := newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) { c.Dialect, c.Database = "postgresql", "app" })
	h, err := NewHandler("", "test-build", WithRegistry(reg))
	if err != nil {
		t.Fatal(err)
	}
	client := &metadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
			PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true, Columns: []spec.PGColumnState{{Name: "id"}}}},
		objectSnapshot: &spec.ObjectSnapshot{PGForeignKeyKeys: &spec.PGForeignKeyKeys{Schema: "app", Table: "t", Keys: []spec.PGForeignKeyKey{{Columns: []string{"id"}, Immediate: true}}}, PGForeignKeyEquality: &spec.PGForeignKeyEquality{Target: spec.PGForeignKeyEqualityTarget{Source: spec.Table{Schema: "app", Name: "t"}, Reference: spec.Table{Schema: "app", Name: "t"}, Columns: []string{"id"}, ReferencedColumns: []string{"id"}}, IncompatiblePosition: 1}}}
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", strings.NewReader(`{"sql":"ALTER TABLE app.t ADD FOREIGN KEY (id) REFERENCES app.t(id) NOT VALID","dialect":"postgresql","connection_id":"test-pg"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-value")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"ddl.pg.alter.add_foreign_key.targets.valid"`) || !strings.Contains(body, `"foreign_key_column_types_incompatible"`) || strings.Contains(body, "PGForeignKeyEquality") || len(client.objectCalls) != 2 {
		t.Fatalf("code=%d response=%s calls=%+v", rec.Code, body, client.objectCalls)
	}
}

func TestHTTPPartitionDropNotNull(t *testing.T) {
	reg := newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) { c.Dialect, c.Database = "postgresql", "app" })
	h, err := NewHandler("", "test-build", WithRegistry(reg))
	if err != nil {
		t.Fatal(err)
	}
	client := &metadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
			PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: false, Columns: []spec.PGColumnState{{Name: "id"}}}},
		objectSnapshot: &spec.ObjectSnapshot{PGColumnTraversal: &spec.PGColumnTraversal{Schema: "app", Table: "t", Column: "id", Partition: true, ParentNotNull: true, ColumnExists: true}}}
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", strings.NewReader(`{"sql":"ALTER TABLE app.t ALTER COLUMN id DROP NOT NULL","dialect":"postgresql","connection_id":"test-pg"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-value")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"ddl.pg.alter.column.generation.transition.valid"`) || !strings.Contains(body, `"not_null_required_by_partition_parent"`) || strings.Contains(body, "PGColumnTraversal") || len(client.objectCalls) != 1 {
		t.Fatalf("code=%d response=%s calls=%+v", rec.Code, body, client.objectCalls)
	}
}

func TestHTTPColumnTypeOnlyParent(t *testing.T) {
	reg := newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) { c.Dialect, c.Database = "postgresql", "app" })
	h, err := NewHandler("", "test-build", WithRegistry(reg))
	if err != nil {
		t.Fatal(err)
	}
	client := &metadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
			PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: false, Columns: []spec.PGColumnState{{Name: "id"}}}},
		objectSnapshot: &spec.ObjectSnapshot{PGColumnType: &spec.PGColumnTypeState{Target: spec.PGColumnTypeTarget{Schema: "pg_catalog", Name: "int4"}, SchemaExists: true, BaseExists: true, Resolved: true, Defined: true, AssignmentKnown: true, Assignable: true, SourceSchema: "app", SourceTable: "t", SourceColumn: "id"}, PGColumnTraversal: &spec.PGColumnTraversal{Schema: "app", Table: "t", Column: "id", ColumnExists: true, AttachedChildren: true}}}
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", strings.NewReader(`{"sql":"ALTER TABLE ONLY app.t ALTER COLUMN id TYPE pg_catalog.int4","dialect":"postgresql","connection_id":"test-pg"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-value")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"ddl.pg.alter.column_type.destination.valid"`) || !strings.Contains(body, `"column_type_requires_child_traversal"`) || strings.Contains(body, "PGColumnType") || len(client.objectCalls) != 2 || client.objectCalls[1].Qualifiers["only_parent"] != "true" {
		t.Fatalf("code=%d response=%s calls=%+v", rec.Code, body, client.objectCalls)
	}
}

func TestHTTPColumnTypeRecursiveParent(t *testing.T) {
	reg := newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) { c.Dialect, c.Database = "postgresql", "app" })
	h, err := NewHandler("", "test-build", WithRegistry(reg))
	if err != nil {
		t.Fatal(err)
	}
	client := &metadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
			PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: false, Columns: []spec.PGColumnState{{Name: "id"}}}},
		objectSnapshot: &spec.ObjectSnapshot{PGColumnType: &spec.PGColumnTypeState{Target: spec.PGColumnTypeTarget{Schema: "pg_catalog", Name: "int4"}, SchemaExists: true, BaseExists: true, Resolved: true, Defined: true, AssignmentKnown: true, Assignable: true, SourceSchema: "app", SourceTable: "t", SourceColumn: "id"}, PGColumnTraversal: &spec.PGColumnTraversal{Schema: "app", Table: "t", Column: "id", ColumnExists: true, AttachedChildren: true, TypeDescendantConflict: true}}}
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", strings.NewReader(`{"sql":"ALTER TABLE app.t ALTER COLUMN id TYPE pg_catalog.int4","dialect":"postgresql","connection_id":"test-pg"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-value")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"ddl.pg.alter.column_type.destination.valid"`) || !strings.Contains(body, `"column_type_descendant_inheritance_conflict"`) || strings.Contains(body, "PGColumnType") || len(client.objectCalls) != 2 || client.objectCalls[1].Qualifiers["recursive_parent"] != "true" {
		t.Fatalf("code=%d response=%s calls=%+v", rec.Code, body, client.objectCalls)
	}
}

func TestHTTPPGRefreshMaterializedViewTarget(t *testing.T) {
	reg := newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) { c.Dialect, c.Database = "postgresql", "app" })
	h, err := NewHandler("", "test-build", WithRegistry(reg))
	if err != nil {
		t.Fatal(err)
	}
	client := &metadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
			PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}},
		objectSnapshot: &spec.ObjectSnapshot{PGRefreshState: &spec.PGRefreshState{Target: spec.PGRefreshTarget{Schema: "app", Name: "mv"}, Kind: "m", Populated: true}}}
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", strings.NewReader(`{"sql":"REFRESH MATERIALIZED VIEW CONCURRENTLY app.mv","dialect":"postgresql","connection_id":"test-pg"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-value")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"ddl.pg.refresh_materialized_view.target.valid"`) || !strings.Contains(body, `"refresh_concurrent_unique_index_absent"`) || strings.Contains(body, "PGRefreshState") || len(client.objectCalls) != 2 || client.objectCalls[1].Name != "mv" || client.objectCalls[1].Type != "refresh_materialized_view" {
		t.Fatalf("code=%d response=%s calls=%+v", rec.Code, body, client.objectCalls)
	}
}

func TestHTTPPGDropViewTarget(t *testing.T) {
	reg := newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) { c.Dialect, c.Database = "postgresql", "app" })
	h, err := NewHandler("", "test-build", WithRegistry(reg))
	if err != nil {
		t.Fatal(err)
	}
	client := &metadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
			PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}},
		objectSnapshot: &spec.ObjectSnapshot{PGDropViewState: &spec.PGDropViewState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "mv"}}, Kind: "v", DependentView: true}}}
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", strings.NewReader(`{"sql":"DROP VIEW app.mv RESTRICT","dialect":"postgresql","connection_id":"test-pg"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-value")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"ddl.pg.drop_view.target.valid"`) || !strings.Contains(body, `"drop_view_has_direct_dependency"`) || strings.Contains(body, "PGDropViewState") || len(client.objectCalls) != 1 || client.objectCalls[0].Name != "mv" || client.objectCalls[0].Type != "drop_view_target" {
		t.Fatalf("code=%d response=%s calls=%+v", rec.Code, body, client.objectCalls)
	}
}

func TestHTTPPGViewNamespaceTarget(t *testing.T) {
	reg := newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) { c.Dialect, c.Database = "postgresql", "app" })
	h, err := NewHandler("", "test-build", WithRegistry(reg))
	if err != nil {
		t.Fatal(err)
	}
	client := &metadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
			PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}},
		objectSnapshot: &spec.ObjectSnapshot{PGTableNamespace: &spec.PGTableNamespace{Schema: "app", Table: "mv", Action: "rename_table", Destination: "destination", SourceKind: "v", SourceKindKnown: true, RelationConflict: true}}}
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", strings.NewReader(`{"sql":"ALTER VIEW app.mv RENAME TO destination","dialect":"postgresql","connection_id":"test-pg"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-value")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"ddl.pg.alter_view.namespace.valid"`) || !strings.Contains(body, `"destination_relation_conflict"`) || strings.Contains(body, "PGTableNamespace") || len(client.objectCalls) != 1 || client.objectCalls[0].Name != "destination" || client.objectCalls[0].Type != "view_namespace" {
		t.Fatalf("code=%d response=%s calls=%+v", rec.Code, body, client.objectCalls)
	}
}

func TestHTTPPGViewOwnerTarget(t *testing.T) {
	reg := newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) { c.Dialect, c.Database = "postgresql", "app" })
	h, err := NewHandler("", "test-build", WithRegistry(reg))
	if err != nil {
		t.Fatal(err)
	}
	client := &metadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
			PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}},
		objectSnapshot: &spec.ObjectSnapshot{PGTableOwner: &spec.PGTableOwnerState{Schema: "app", Table: "mv", Target: spec.PGTableOwnerTarget{Kind: "named", Name: "missing_role"}, SourceKind: "v", SourceKindKnown: true}}}
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", strings.NewReader(`{"sql":"ALTER VIEW app.mv OWNER TO missing_role","dialect":"postgresql","connection_id":"test-pg"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-value")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"ddl.pg.alter_view.owner.valid"`) || !strings.Contains(body, `"owner_role_absent"`) || strings.Contains(body, "PGTableOwner") || len(client.objectCalls) != 1 || client.objectCalls[0].Name != "missing_role" || client.objectCalls[0].Type != "view_owner" {
		t.Fatalf("code=%d response=%s calls=%+v", rec.Code, body, client.objectCalls)
	}
}

func TestHTTPPGAlterViewOptionsTarget(t *testing.T) {
	reg := newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) { c.Dialect, c.Database = "postgresql", "app" })
	h, err := NewHandler("", "test-build", WithRegistry(reg))
	if err != nil {
		t.Fatal(err)
	}
	client := &metadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
			PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}},
		objectSnapshot: &spec.ObjectSnapshot{PGDropViewState: &spec.PGDropViewState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "mv"}}, Kind: "v"}}}
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", strings.NewReader(`{"sql":"ALTER VIEW app.mv RESET (security_invoker=true)","dialect":"postgresql","connection_id":"test-pg"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-value")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"ddl.pg.alter_view.options.valid"`) || !strings.Contains(body, `"view_option_reset_has_value"`) || strings.Contains(body, "PGDropViewState") || len(client.objectCalls) != 1 || client.objectCalls[0].Name != "mv" || client.objectCalls[0].Type != "alter_view_options" {
		t.Fatalf("code=%d response=%s calls=%+v", rec.Code, body, client.objectCalls)
	}
}

func TestHTTPPGViewDefaultTarget(t *testing.T) {
	reg := newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) { c.Dialect, c.Database = "postgresql", "app" })
	h, err := NewHandler("", "test-build", WithRegistry(reg))
	if err != nil {
		t.Fatal(err)
	}
	client := &metadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
			PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}},
		objectSnapshot: &spec.ObjectSnapshot{PGViewDefault: &spec.PGViewDefaultState{Schema: "app", View: "mv", Column: "id", Kind: "v", ColumnExists: true}}}
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", strings.NewReader(`{"sql":"ALTER VIEW app.mv ALTER COLUMN id SET DEFAULT (SELECT 42)","dialect":"postgresql","connection_id":"test-pg"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-value")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"ddl.pg.alter_view.column_default.valid"`) || !strings.Contains(body, `"view_default_reference_inapplicable"`) || strings.Contains(body, "PGViewDefault") || len(client.objectCalls) != 1 || client.objectCalls[0].Name != "mv" || client.objectCalls[0].Type != "view_column_default" {
		t.Fatalf("code=%d response=%s calls=%+v", rec.Code, body, client.objectCalls)
	}
}

func TestHTTPPGMaterializedViewOptionsTarget(t *testing.T) {
	reg := newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) { c.Dialect, c.Database = "postgresql", "app" })
	h, err := NewHandler("", "test-build", WithRegistry(reg))
	if err != nil {
		t.Fatal(err)
	}
	client := &metadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
			PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}},
		objectSnapshot: &spec.ObjectSnapshot{PGDropViewState: &spec.PGDropViewState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "mv"}}, Kind: "m", Heap: true}}}
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", strings.NewReader(`{"sql":"ALTER MATERIALIZED VIEW app.mv SET (fillfactor=9)","dialect":"postgresql","connection_id":"test-pg"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-value")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"ddl.pg.alter_materialized_view.options.valid"`) || !strings.Contains(body, `"materialized_view_option_out_of_range"`) || strings.Contains(body, "PGDropViewState") || len(client.objectCalls) != 2 || client.objectCalls[0].Type != "materialized_view" || client.objectCalls[1].Name != "mv" || client.objectCalls[1].Type != "materialized_view_options" {
		t.Fatalf("code=%d response=%s calls=%+v", rec.Code, body, client.objectCalls)
	}
}

func TestHTTPPGMaterializedViewExtensionTarget(t *testing.T) {
	reg := newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) { c.Dialect, c.Database = "postgresql", "app" })
	h, err := NewHandler("", "test-build", WithRegistry(reg))
	if err != nil {
		t.Fatal(err)
	}
	client := &metadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
			PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}},
		objectSnapshot: &spec.ObjectSnapshot{PGMaterializedViewExtension: &spec.PGMaterializedViewExtensionState{Schema: "app", View: "mv", Extension: "missing_ext", Kind: "m"}}}
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", strings.NewReader(`{"sql":"ALTER MATERIALIZED VIEW app.mv NO DEPENDS ON EXTENSION missing_ext","dialect":"postgresql","connection_id":"test-pg"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-value")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"ddl.pg.alter_materialized_view.extension.valid"`) || !strings.Contains(body, `"materialized_view_extension_absent"`) || strings.Contains(body, "PGMaterializedViewExtension") || len(client.objectCalls) != 2 || client.objectCalls[0].Type != "materialized_view" || client.objectCalls[1].Name != "mv" || client.objectCalls[1].Type != "materialized_view_extension" {
		t.Fatalf("code=%d response=%s calls=%+v", rec.Code, body, client.objectCalls)
	}
}

func TestHTTPPGCreateViewOutputTarget(t *testing.T) {
	reg := newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) { c.Dialect, c.Database = "postgresql", "app" })
	h, err := NewHandler("", "test-build", WithRegistry(reg))
	if err != nil {
		t.Fatal(err)
	}
	client := &metadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
			PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}},
		objectSnapshot: &spec.ObjectSnapshot{PGCreateViewTarget: &spec.PGCreateViewTarget{Table: spec.Table{Schema: "app", Name: "mv"}, SchemaExists: true, Kind: "v"}}}
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", strings.NewReader(`{"sql":"CREATE VIEW app.mv AS SELECT 1 AS id","dialect":"postgresql","connection_id":"test-pg"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-value")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"ddl.pg.create_view.output.valid"`) || !strings.Contains(body, `"view_output_target_exists"`) || strings.Contains(body, "PGCreateViewTarget") || len(client.objectCalls) != 1 || client.objectCalls[0].Name != "mv" || client.objectCalls[0].Type != "create_view_output" {
		t.Fatalf("code=%d response=%s calls=%+v", rec.Code, body, client.objectCalls)
	}
}

func TestHTTPPGDropViewSet(t *testing.T) {
	reg := newTestRegistry(t, "test-pg", func(c *runtimeconfig.ConnectionConfig) { c.Dialect, c.Database = "postgresql", "app" })
	h, err := NewHandler("", "test-build", WithRegistry(reg))
	if err != nil {
		t.Fatal(err)
	}
	client := &metadataAuditTestClient{version: "PostgreSQL 17.7", detectDialect: spec.DialectPostgreSQL,
		snapshot: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
			PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}},
		objectSnapshot: &spec.ObjectSnapshot{PGDropViewSet: &spec.PGDropViewSetState{Targets: []spec.PGDestructiveTableTarget{{Table: spec.Table{Schema: "app", Name: "mv"}}, {Table: spec.Table{Schema: "other", Name: "v"}}}, Position: 2, Reason: "drop_view_has_row_type_dependency"}}}
	previous := prepareHTTPMetadataAudit
	prepareHTTPMetadataAudit = func(_ context.Context, _ auditmeta.Request) (*auditmeta.PreparedAudit, error) {
		return &auditmeta.PreparedAudit{Client: client, Dialect: spec.DialectPostgreSQL, Schema: "app"}, nil
	}
	t.Cleanup(func() { prepareHTTPMetadataAudit = previous })
	req := httptest.NewRequest(http.MethodPost, "/v1/audit", strings.NewReader(`{"sql":"DROP VIEW app.mv,other.v RESTRICT","dialect":"postgresql","connection_id":"test-pg"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "test-key-value")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"ddl.pg.drop_view.target.valid"`) || !strings.Contains(body, `"drop_view_has_row_type_dependency"`) || strings.Contains(body, "PGDropViewSet") || len(client.objectCalls) != 1 || len(client.objectCalls[0].PGDestructiveTargets) != 2 || client.objectCalls[0].Type != "drop_view_set" {
		t.Fatalf("code=%d response=%s calls=%+v", rec.Code, body, client.objectCalls)
	}
}
