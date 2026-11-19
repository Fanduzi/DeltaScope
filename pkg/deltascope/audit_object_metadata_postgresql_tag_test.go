//go:build postgresql

package deltascope

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type fakeMetadataProviderWithObjects struct {
	fakeMetadataProvider
	objectSnapshot *spec.ObjectSnapshot
	objectCalls    []spec.ObjectLookupRequest
}

func (f *fakeMetadataProviderWithObjects) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	f.objectCalls = append(f.objectCalls, req)
	return f.objectSnapshot, nil
}

func TestAuditProjectsConfirmedObjectMetadataThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		objectSnapshot: &spec.ObjectSnapshot{
			Schema: "public",
			Type:   "extension",
			Name:   "pg_trgm",
			Status: spec.MetadataStatusConfirmed,
			Exists: true,
			Attributes: map[string]string{
				"extension_version": "1.6",
				"type_kind":         "base",
				"password":          "should_be_filtered",
				"comment":           "also_filtered",
			},
		},
	}

	result, err := Audit(context.Background(), Request{
		SQL:              "DROP EXTENSION IF EXISTS pg_trgm",
		Dialect:          DialectPostgreSQL,
		Schema:           "public",
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}

	var finding *Finding
	for i := range result.Statements {
		for j := range result.Statements[i].Findings {
			if result.Statements[i].Findings[j].RuleID == "ddl.pg.drop_extension.advisory" {
				f := result.Statements[i].Findings[j]
				finding = &f
				break
			}
		}
	}
	if finding == nil {
		t.Fatalf("expected drop_extension finding, got %d statements", len(result.Statements))
	}

	meta := finding.Metadata
	if meta["metadata_status"] != "confirmed" {
		t.Fatalf("expected metadata_status=confirmed, got %#v", meta["metadata_status"])
	}
	if meta["metadata_object_type"] != "extension" {
		t.Fatalf("expected metadata_object_type=extension, got %#v", meta["metadata_object_type"])
	}
	if meta["metadata_object_name"] != "pg_trgm" {
		t.Fatalf("expected metadata_object_name=pg_trgm, got %#v", meta["metadata_object_name"])
	}
	if meta["metadata_exists"] != true {
		t.Fatalf("expected metadata_exists=true, got %#v", meta["metadata_exists"])
	}
	if meta["metadata_extension_version"] != "1.6" {
		t.Fatalf("expected metadata_extension_version=1.6, got %#v", meta["metadata_extension_version"])
	}

	sensitiveKeys := []string{"password", "secret", "conninfo", "connection", "host", "port", "options", "query", "body", "definition", "comment", "label"}
	for _, key := range sensitiveKeys {
		if _, ok := meta["metadata_"+key]; ok {
			t.Fatalf("sensitive key metadata_%q leaked into finding metadata", key)
		}
	}

	if len(provider.objectCalls) != 1 {
		t.Fatalf("expected 1 ResolveObject call, got %d", len(provider.objectCalls))
	}
	if provider.objectCalls[0].Type != "extension" || provider.objectCalls[0].Name != "pg_trgm" {
		t.Fatalf("expected lookup for extension/pg_trgm, got %#v", provider.objectCalls[0])
	}
}

func TestAuditReplicaIdentityAttachmentThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGReplicaIdentity: &spec.PGReplicaIdentityIndex{Schema: "app", Table: "t", Name: "Index Name", Kind: "i", IndexKnown: true, OnTable: true, Unique: true, Immediate: true, Partial: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER TABLE app.t REPLICA IDENTITY USING INDEX "Index Name"`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "Index Name" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter.replica_identity_using_index.valid" && f.Metadata["reason_code"] == "index_partial" {
			return
		}
	}
	t.Fatalf("missing attachment finding: %+v", result.Statements[0].Findings)
}

func TestAuditTableTablespaceThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGTableTablespace: &spec.PGTableTablespace{Schema: "app", Table: "t", Name: "Mixed Space"}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER TABLE app.t SET TABLESPACE "Mixed Space"`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "Mixed Space" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter.tablespace.target.valid" && f.Metadata["reason_code"] == "tablespace_absent" {
			return
		}
	}
	t.Fatalf("missing destination finding: %+v", result.Statements[0].Findings)
}

func TestAuditTableAccessMethodThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGTableAccessMethod: &spec.PGTableAccessMethod{Schema: "app", Table: "t", Name: "Mixed Space"}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER TABLE app.t SET ACCESS METHOD "Mixed Space"`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "Mixed Space" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter.access_method.target.valid" && f.Metadata["reason_code"] == "access_method_absent" {
			return
		}
	}
	t.Fatalf("missing destination finding: %+v", result.Statements[0].Findings)
}

func TestAuditTablePersistenceThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGTablePersistence: &spec.PGTablePersistence{Schema: "app", Table: "t", Current: "p", DirectPublication: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER TABLE app.t SET UNLOGGED`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "t" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter.persistence.transition.valid" && f.Metadata["reason_code"] == "persistence_direct_publication" {
			return
		}
	}
	t.Fatalf("missing persistence finding: %+v", result.Statements[0].Findings)
}

func TestAuditColumnTypeSourceThroughSDK(t *testing.T) {
	provider := &fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
		snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
			PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{{Name: "id"}}}}}
	result, err := Audit(context.Background(), Request{SQL: `ALTER TABLE app.t ALTER COLUMN "ID" TYPE bigint`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(result.Statements) != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter.column_type.source.valid" && f.Metadata["reason_code"] == "column_absent" && f.Metadata["column"] == "ID" {
			return
		}
	}
	t.Fatalf("missing exact source finding: %+v", result.Statements[0].Findings)
}

func TestAuditTableNamespaceThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGTableNamespace: &spec.PGTableNamespace{Schema: "app", Table: "t", Action: "rename_table", Destination: "Occupied Type", SchemaExists: true, SameSchema: true, RowTypeConflict: true, RowTypeConflictKnown: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER TABLE app.t RENAME TO "Occupied Type"`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "Occupied Type" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter.rename_table.target.valid" && f.Metadata["reason_code"] == "destination_row_type_conflict" {
			return
		}
	}
	t.Fatalf("missing destination finding: %+v", result.Statements[0].Findings)
}
