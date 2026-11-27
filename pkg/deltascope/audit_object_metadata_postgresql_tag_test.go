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
	for _, kind := range []string{"r", "p"} {
		t.Run(kind, func(t *testing.T) { testAuditTableTablespaceThroughSDK(t, kind) })
	}
}

func testAuditTableTablespaceThroughSDK(t *testing.T, kind string) {
	t.Helper()
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: kind}, PGColumns: &PGColumnCatalog{Ordinary: kind == "r"}}},
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
	for _, kind := range []string{"r", "p"} {
		t.Run(kind, func(t *testing.T) {
			provider.snapshot.PGRelation.Kind = kind
			provider.snapshot.PGColumns.Ordinary = kind == "r"
			provider.objectSnapshot.PGTablePersistence.PartitionRoot = kind == "p"
			provider.objectCalls = nil
			result, err := Audit(context.Background(), Request{SQL: `ALTER TABLE app.t SET UNLOGGED`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
			if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "t" || (provider.objectCalls[0].Qualifiers["kind"] == "p") != (kind == "p") {
				t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
			}
			for _, f := range result.Statements[0].Findings {
				if f.RuleID == "ddl.pg.alter.persistence.transition.valid" && f.Metadata["reason_code"] == "persistence_direct_publication" {
					return
				}
			}
			t.Fatalf("missing persistence finding: %+v", result.Statements[0].Findings)
		})
	}
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
	for _, kind := range []string{"r", "p"} {
		t.Run(kind, func(t *testing.T) {
			provider := &fakeMetadataProviderWithObjects{
				fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
					snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
						PGRelation: PGRelationState{Name: "t", Kind: kind}, PGColumns: &PGColumnCatalog{Ordinary: kind == "r"}}},
				objectSnapshot: &spec.ObjectSnapshot{PGTableNamespace: &spec.PGTableNamespace{Schema: "app", Table: "t", Action: "rename_table", Destination: "Occupied Type", SchemaExists: true, SameSchema: true, RowTypeConflict: true, RowTypeConflictKnown: true, SourceKind: kind, SourceKindKnown: kind == "p"}},
			}
			result, err := Audit(context.Background(), Request{SQL: `ALTER TABLE app.t RENAME TO "Occupied Type"`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
			if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "Occupied Type" || (provider.objectCalls[0].Qualifiers["kind"] == "p") != (kind == "p") {
				t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
			}
			for _, f := range result.Statements[0].Findings {
				if f.RuleID == "ddl.pg.alter.rename_table.target.valid" && f.Metadata["reason_code"] == "destination_row_type_conflict" {
					return
				}
			}
			t.Fatalf("missing destination finding: %+v", result.Statements[0].Findings)
		})
	}
}

func TestAuditTableOwnerThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGTableOwner: &spec.PGTableOwnerState{Schema: "app", Table: "t", Target: spec.PGTableOwnerTarget{Kind: "named", Name: " Missing Owner "}}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER TABLE app.t OWNER TO " Missing Owner "`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != " Missing Owner " {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter.owner.target.valid" && f.Metadata["reason_code"] == "owner_role_absent" {
			return
		}
	}
	t.Fatalf("missing owner target finding: %+v", result.Statements[0].Findings)
}

func TestAuditBulkTablespaceThroughSDK(t *testing.T) {
	target := spec.PGBulkTablespaceTarget{Source: " Exact Source ", Destination: "missing", Nowait: true}
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"}},
		objectSnapshot:       &spec.ObjectSnapshot{PGBulkTablespace: &spec.PGBulkTablespaceState{Target: target, SourceExists: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER TABLE ALL IN TABLESPACE " Exact Source " SET TABLESPACE missing NOWAIT`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].PGBulkTablespace == nil || !provider.objectCalls[0].PGBulkTablespace.Matches(target) {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter.move_all_tablespace.target.valid" && f.Metadata["reason_code"] == "destination_tablespace_absent" {
			return
		}
	}
	t.Fatalf("missing bulk target finding: %+v", result.Statements[0].Findings)
}

func TestAuditColumnTypeThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{{Name: "id"}}}}},
		objectSnapshot: &spec.ObjectSnapshot{PGColumnType: &spec.PGColumnTypeState{Target: spec.PGColumnTypeTarget{Schema: "pg_catalog", Name: "cstring", Array: true}, SchemaExists: true, BaseExists: true, Resolved: true, Defined: true, Pseudo: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER TABLE app.t ALTER COLUMN id TYPE pg_catalog.cstring[]`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "cstring" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter.column_type.destination.valid" && f.Metadata["reason_code"] == "destination_pseudo_type_inapplicable" {
			return
		}
	}
	t.Fatalf("missing type destination finding: %+v", result.Statements[0].Findings)
}

func TestAuditConstraintUsingIndexThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{{Name: "id"}}}}},
		objectSnapshot: &spec.ObjectSnapshot{PGConstraintUsingIndex: &spec.PGConstraintUsingIndex{Schema: "app", Table: "t", Name: " Exact Index ", Kind: "i", IndexKnown: true, OnTable: true, Unique: true, BTree: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER TABLE app.t ADD CONSTRAINT pending UNIQUE USING INDEX " Exact Index "`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != " Exact Index " {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter.constraint_using_index.valid" && f.Metadata["reason_code"] == "constraint_index_invalid" {
			return
		}
	}
	t.Fatalf("missing index attachment finding: %+v", result.Statements[0].Findings)
}

func TestAuditForeignKeyReferenceKeysThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{{Name: "id"}}}}},
		objectSnapshot: &spec.ObjectSnapshot{PGForeignKeyKeys: &spec.PGForeignKeyKeys{Schema: "app", Table: "t"}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER TABLE app.t ADD FOREIGN KEY (id) REFERENCES app.t(id) NOT VALID`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "t" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter.add_foreign_key.targets.valid" && f.Metadata["reason_code"] == "foreign_key_reference_unique_key_absent" {
			return
		}
	}
	t.Fatalf("missing referenced unique key finding: %+v", result.Statements[0].Findings)
}

func TestAuditDropConstraintReferenceThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGConstraints: []PGConstraintState{{Name: "pk", Type: "p", Validated: true}}, PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{{Name: "id"}}}}},
		objectSnapshot: &spec.ObjectSnapshot{PGDropConstraintReference: &spec.PGDropConstraintReference{Schema: "app", Table: "t", Name: "pk", Kind: "p", Blocked: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER TABLE app.t DROP CONSTRAINT pk RESTRICT`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "pk" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter.drop_constraint.target.require" && f.Metadata["reason_code"] == "constraint_referenced_by_foreign_key" {
			return
		}
	}
	t.Fatalf("missing direct constraint dependency finding: %+v", result.Statements[0].Findings)
}

func TestAuditDropColumnDependencyThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{{Name: "id"}}}}},
		objectSnapshot: &spec.ObjectSnapshot{PGDropColumnDependency: &spec.PGDropColumnDependency{Schema: "app", Table: "t", Column: "id", Kind: "foreign_key"}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER TABLE app.t DROP COLUMN id RESTRICT`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "id" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.alter.drop_column.exists.require" && f.Metadata["reason_code"] == "column_referenced_by_foreign_key" {
			return
		}
	}
	t.Fatalf("missing direct column dependency finding: %+v", result.Statements[0].Findings)
}

func TestAuditTableNamespaceCompanionThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGTableNamespace: &spec.PGTableNamespace{Schema: "app", Table: "t", Action: "set_schema", Destination: "Destination", SchemaExists: true, RowTypeConflictKnown: true, ArrayTypeConflictKnown: true, ArrayTypeConflict: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER TABLE app.t SET SCHEMA "Destination"`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "Destination" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter.set_schema.target.valid" && f.Metadata["reason_code"] == "destination_array_type_conflict" {
			return
		}
	}
	t.Fatalf("missing destination finding: %+v", result.Statements[0].Findings)
}

func TestAuditSetWithoutOIDsThroughSDK(t *testing.T) {
	provider := &fakeMetadataProvider{
		instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
		snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
			PGRelation: PGRelationState{Name: "t", Kind: "v"}, PGColumns: &PGColumnCatalog{Ordinary: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER TABLE IF EXISTS app.t SET WITHOUT OIDS`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.table.exists.alter.require" && f.Metadata["reason_code"] == "relation_wrong_kind" {
			return
		}
	}
	t.Fatalf("missing destination finding: %+v", result.Statements[0].Findings)
}

func TestAuditColumnTypeAssignmentThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{{Name: "id"}}}}},
		objectSnapshot: &spec.ObjectSnapshot{PGColumnType: &spec.PGColumnTypeState{Target: spec.PGColumnTypeTarget{Schema: "pg_catalog", Name: "int4"}, SchemaExists: true, BaseExists: true, Resolved: true, Defined: true, AssignmentKnown: true, SourceSchema: "app", SourceTable: "t", SourceColumn: "id"}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER TABLE app.t ALTER COLUMN id TYPE pg_catalog.int4`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "int4" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter.column_type.destination.valid" && f.Metadata["reason_code"] == "destination_type_assignment_unavailable" {
			return
		}
	}
	t.Fatalf("missing type destination finding: %+v", result.Statements[0].Findings)
}

func TestAuditConstraintDefaultOpclassThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{{Name: "id"}}}}},
		objectSnapshot: &spec.ObjectSnapshot{PGConstraintUsingIndex: &spec.PGConstraintUsingIndex{Schema: "app", Table: "t", Name: " Exact Index ", Kind: "i", IndexKnown: true, OnTable: true, Unique: true, BTree: true, Valid: true, KeyDefaults: true, KeyDefaultsKnown: true, OpclassesDefaultKnown: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER TABLE app.t ADD CONSTRAINT pending UNIQUE USING INDEX " Exact Index "`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != " Exact Index " {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter.constraint_using_index.valid" && f.Metadata["reason_code"] == "constraint_index_nondefault_opclass" {
			return
		}
	}
	t.Fatalf("missing index attachment finding: %+v", result.Statements[0].Findings)
}

func TestAuditPartitionedForeignKeyNotValid(t *testing.T) {
	provider := &fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
		snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"}, PGRelation: PGRelationState{Name: "t", Kind: "p"}}}
	result, err := Audit(context.Background(), Request{SQL: `ALTER TABLE app.t ADD FOREIGN KEY (id) REFERENCES app.parent(id) NOT VALID`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(result.Statements) != 1 || len(result.Statements[0].EvidenceGaps) != 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter.add_foreign_key.targets.valid" && f.Metadata["reason_code"] == "partitioned_foreign_key_not_valid" {
			return
		}
	}
	t.Fatalf("missing partitioned FK finding: %+v", result.Statements[0].Findings)
}

func TestAuditForeignKeyDeleteSetColumnsThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{{Name: "id"}, {Name: "extra"}}}}},
		objectSnapshot: &spec.ObjectSnapshot{PGForeignKeyKeys: &spec.PGForeignKeyKeys{Schema: "app", Table: "t"}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER TABLE app.t ADD FOREIGN KEY (id) REFERENCES app.t(id) ON DELETE SET NULL (extra) NOT VALID`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "t" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter.add_foreign_key.targets.valid" && f.Metadata["reason_code"] == "foreign_key_delete_set_column_outside_key" {
			return
		}
	}
	t.Fatalf("missing referenced unique key finding: %+v", result.Statements[0].Findings)
}

func TestAuditInheritedColumnDropThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: false, Columns: []PGColumnState{{Name: "id"}}}}},
		objectSnapshot: &spec.ObjectSnapshot{PGColumnTraversal: &spec.PGColumnTraversal{Schema: "app", Table: "t", Column: "id", Inherited: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER TABLE app.t DROP COLUMN IF EXISTS id CASCADE`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "id" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.alter.drop_column.exists.require" && f.Metadata["reason_code"] == "column_inherited" {
			return
		}
	}
	t.Fatalf("missing direct column dependency finding: %+v", result.Statements[0].Findings)
}

func TestAuditColumnTypeDependencyThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{{Name: "id"}}}}},
		objectSnapshot: &spec.ObjectSnapshot{PGColumnType: &spec.PGColumnTypeState{Target: spec.PGColumnTypeTarget{Schema: "pg_catalog", Name: "int4"}, SchemaExists: true, BaseExists: true, Resolved: true, Defined: true, AssignmentKnown: true, Assignable: true, SourceSchema: "app", SourceTable: "t", SourceColumn: "id"}, PGDropColumnDependency: &spec.PGDropColumnDependency{Schema: "app", Table: "t", Column: "id", Kind: "view"}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER TABLE app.t ALTER COLUMN id TYPE pg_catalog.int4`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 2 || provider.objectCalls[0].Name != "int4" || provider.objectCalls[1].Type != "drop_column_dependency" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter.column_type.destination.valid" && f.Metadata["reason_code"] == "column_type_has_direct_dependency" {
			return
		}
	}
	t.Fatalf("missing type destination finding: %+v", result.Statements[0].Findings)
}

func TestAuditForeignKeyScalarEqualityThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{{Name: "id"}}}}},
		objectSnapshot: &spec.ObjectSnapshot{PGForeignKeyKeys: &spec.PGForeignKeyKeys{Schema: "app", Table: "t", Keys: []spec.PGForeignKeyKey{{Columns: []string{"id"}, Immediate: true}}}, PGForeignKeyEquality: &spec.PGForeignKeyEquality{Target: spec.PGForeignKeyEqualityTarget{Source: spec.Table{Schema: "app", Name: "t"}, Reference: spec.Table{Schema: "app", Name: "t"}, Columns: []string{"id"}, ReferencedColumns: []string{"id"}}, IncompatiblePosition: 1}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER TABLE app.t ADD FOREIGN KEY (id) REFERENCES app.t(id) NOT VALID`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 2 || provider.objectCalls[0].Name != "t" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter.add_foreign_key.targets.valid" && f.Metadata["reason_code"] == "foreign_key_column_types_incompatible" {
			return
		}
	}
	t.Fatalf("missing referenced unique key finding: %+v", result.Statements[0].Findings)
}

func TestAuditSetNotNullValidatedCheckThroughSDK(t *testing.T) {
	fact := &spec.PGColumnTraversal{Schema: "app", Table: "t", Column: "id", ColumnExists: true}
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true, Columns: []PGColumnState{{Name: "id", TypeWidth: 32}}}}},
		objectSnapshot: &spec.ObjectSnapshot{PGColumnTraversal: fact},
	}
	for _, proved := range []bool{false, true} {
		fact.ValidatedNotNullCheck = proved
		result, err := Audit(context.Background(), Request{SQL: `ALTER TABLE app.t ALTER COLUMN id SET NOT NULL`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
		if err != nil || result.Verdict != VerdictReject || len(result.Statements) != 1 || (len(result.Statements[0].EvidenceGaps) == 0) != proved {
			t.Fatalf("proved=%v result=%+v err=%v", proved, result, err)
		}
	}
}

func TestAuditPartitionIdentityThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{}}},
		objectSnapshot: &spec.ObjectSnapshot{PGColumnTraversal: &spec.PGColumnTraversal{Schema: "app", Table: "t", Column: "id", IdentityChangeBlocked: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER TABLE app.t ALTER COLUMN id RESTART WITH 5`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Qualifiers["action"] != "set_generated" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter.column.generation.transition.valid" && f.Metadata["reason_code"] == "identity_partition_target_inapplicable" {
			return
		}
	}
	t.Fatalf("missing identity partition rejection: %+v", result.Statements[0].Findings)
}

func TestAuditPartitionDropNotNullThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: false, Columns: []PGColumnState{{Name: "id"}}}}},
		objectSnapshot: &spec.ObjectSnapshot{PGColumnTraversal: &spec.PGColumnTraversal{Schema: "app", Table: "t", Column: "id", Partition: true, ParentNotNull: true, ColumnExists: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER TABLE app.t ALTER COLUMN id DROP NOT NULL`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "id" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter.column.generation.transition.valid" && f.Metadata["reason_code"] == "not_null_required_by_partition_parent" {
			return
		}
	}
	t.Fatalf("missing direct column dependency finding: %+v", result.Statements[0].Findings)
}

func TestAuditColumnTypeOnlyParentThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: false, Columns: []PGColumnState{{Name: "id"}}}}},
		objectSnapshot: &spec.ObjectSnapshot{PGColumnType: &spec.PGColumnTypeState{Target: spec.PGColumnTypeTarget{Schema: "pg_catalog", Name: "int4"}, SchemaExists: true, BaseExists: true, Resolved: true, Defined: true, AssignmentKnown: true, Assignable: true, SourceSchema: "app", SourceTable: "t", SourceColumn: "id"}, PGColumnTraversal: &spec.PGColumnTraversal{Schema: "app", Table: "t", Column: "id", ColumnExists: true, AttachedChildren: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER TABLE ONLY app.t ALTER COLUMN id TYPE pg_catalog.int4`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 2 || provider.objectCalls[0].Type != "column_traversal" || provider.objectCalls[1].Qualifiers["only_parent"] != "true" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter.column_type.destination.valid" && f.Metadata["reason_code"] == "column_type_requires_child_traversal" {
			return
		}
	}
	t.Fatalf("missing ONLY traversal finding: %+v", result.Statements[0].Findings)
}

func TestAuditColumnTypeRecursiveParentThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: false, Columns: []PGColumnState{{Name: "id"}}}}},
		objectSnapshot: &spec.ObjectSnapshot{PGColumnType: &spec.PGColumnTypeState{Target: spec.PGColumnTypeTarget{Schema: "pg_catalog", Name: "int4"}, SchemaExists: true, BaseExists: true, Resolved: true, Defined: true, AssignmentKnown: true, Assignable: true, SourceSchema: "app", SourceTable: "t", SourceColumn: "id"}, PGColumnTraversal: &spec.PGColumnTraversal{Schema: "app", Table: "t", Column: "id", ColumnExists: true, AttachedChildren: true, TypeDescendantConflict: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER TABLE app.t ALTER COLUMN id TYPE pg_catalog.int4`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 2 || provider.objectCalls[0].Type != "column_traversal" || provider.objectCalls[1].Qualifiers["recursive_parent"] != "true" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter.column_type.destination.valid" && f.Metadata["reason_code"] == "column_type_descendant_inheritance_conflict" {
			return
		}
	}
	t.Fatalf("missing recursive traversal finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGRefreshMaterializedViewThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGRefreshState: &spec.PGRefreshState{Target: spec.PGRefreshTarget{Schema: "app", Name: "mv"}, Kind: "m", Populated: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `REFRESH MATERIALIZED VIEW CONCURRENTLY app.mv`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 2 || provider.objectCalls[1].Name != "mv" || provider.objectCalls[1].Type != "refresh_materialized_view" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.refresh_materialized_view.target.valid" && f.Metadata["reason_code"] == "refresh_concurrent_unique_index_absent" {
			return
		}
	}
	t.Fatalf("missing refresh prerequisite finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGDropViewThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGDropViewState: &spec.PGDropViewState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "mv"}}, Kind: "v", DependentView: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `DROP VIEW app.mv RESTRICT`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "mv" || provider.objectCalls[0].Type != "drop_view_target" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.drop_view.target.valid" && f.Metadata["reason_code"] == "drop_view_has_direct_dependency" {
			return
		}
	}
	t.Fatalf("missing DROP VIEW dependency finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGViewNamespaceThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGTableNamespace: &spec.PGTableNamespace{Schema: "app", Table: "mv", Action: "rename_table", Destination: "destination", SourceKind: "v", SourceKindKnown: true, RelationConflict: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER VIEW app.mv RENAME TO destination`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "destination" || provider.objectCalls[0].Type != "view_namespace" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter_view.namespace.valid" && f.Metadata["reason_code"] == "destination_relation_conflict" {
			return
		}
	}
	t.Fatalf("missing view namespace collision: %+v", result.Statements[0].Findings)
}

func TestAuditPGViewOwnerThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGTableOwner: &spec.PGTableOwnerState{Schema: "app", Table: "mv", Target: spec.PGTableOwnerTarget{Kind: "named", Name: "missing_role"}, SourceKind: "v", SourceKindKnown: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER VIEW app.mv OWNER TO missing_role`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "missing_role" || provider.objectCalls[0].Type != "view_owner" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter_view.owner.valid" && f.Metadata["reason_code"] == "owner_role_absent" {
			return
		}
	}
	t.Fatalf("missing view owner finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGAlterViewOptionsThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGDropViewState: &spec.PGDropViewState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "mv"}}, Kind: "v"}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER VIEW app.mv RESET (security_invoker=true)`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "mv" || provider.objectCalls[0].Type != "alter_view_options" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter_view.options.valid" && f.Metadata["reason_code"] == "view_option_reset_has_value" {
			return
		}
	}
	t.Fatalf("missing ALTER VIEW option finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGViewDefaultThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGViewDefault: &spec.PGViewDefaultState{Schema: "app", View: "mv", Column: "id", Kind: "v", ColumnExists: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER VIEW app.mv ALTER COLUMN id SET DEFAULT (SELECT 42)`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "mv" || provider.objectCalls[0].Type != "view_column_default" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter_view.column_default.valid" && f.Metadata["reason_code"] == "view_default_reference_inapplicable" {
			return
		}
	}
	t.Fatalf("missing ALTER VIEW default finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGMaterializedViewOptionsThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGDropViewState: &spec.PGDropViewState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "mv"}}, Kind: "m", Heap: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER MATERIALIZED VIEW app.mv SET (fillfactor=9)`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 2 || provider.objectCalls[0].Type != "materialized_view" || provider.objectCalls[1].Name != "mv" || provider.objectCalls[1].Type != "materialized_view_options" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter_materialized_view.options.valid" && f.Metadata["reason_code"] == "materialized_view_option_out_of_range" {
			return
		}
	}
	t.Fatalf("missing ALTER MATERIALIZED VIEW storage option finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGMaterializedViewExtensionThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGMaterializedViewExtension: &spec.PGMaterializedViewExtensionState{Schema: "app", View: "mv", Extension: "missing_ext", Kind: "m"}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER MATERIALIZED VIEW app.mv NO DEPENDS ON EXTENSION missing_ext`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 2 || provider.objectCalls[0].Type != "materialized_view" || provider.objectCalls[1].Name != "mv" || provider.objectCalls[1].Type != "materialized_view_extension" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter_materialized_view.extension.valid" && f.Metadata["reason_code"] == "materialized_view_extension_absent" {
			return
		}
	}
	t.Fatalf("missing ALTER MATERIALIZED VIEW extension dependency finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGCreateViewOutputThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGCreateViewTarget: &spec.PGCreateViewTarget{Table: spec.Table{Schema: "app", Name: "mv"}, SchemaExists: true, Kind: "v"}},
	}
	result, err := Audit(context.Background(), Request{SQL: `CREATE VIEW app.mv AS SELECT 1 AS id`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "mv" || provider.objectCalls[0].Type != "create_view_output" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.create_view.output.valid" && f.Metadata["reason_code"] == "view_output_target_exists" {
			return
		}
	}
	t.Fatalf("missing CREATE VIEW collision finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGDropViewSetThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGDropViewSet: &spec.PGDropViewSetState{Targets: []spec.PGDestructiveTableTarget{{Table: spec.Table{Schema: "app", Name: "mv"}}, {Table: spec.Table{Schema: "other", Name: "v"}}}, Position: 2, Reason: "drop_view_has_row_type_dependency"}},
	}
	result, err := Audit(context.Background(), Request{SQL: `DROP VIEW app.mv,other.v RESTRICT`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || len(provider.objectCalls[0].PGDestructiveTargets) != 2 || provider.objectCalls[0].Type != "drop_view_set" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.drop_view.target.valid" && f.Metadata["reason_code"] == "drop_view_has_row_type_dependency" {
			return
		}
	}
	t.Fatalf("missing DROP VIEW row-type dependency finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGPartitionConstraintThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "p"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGPartitionConstraint: &spec.PGPartitionConstraintState{Target: spec.Table{Schema: "app", Name: "t"}, Columns: []spec.PGColumnState{{Name: "id"}, {Name: "payload"}}, PartitionKeys: []spec.PGPartitionConstraintKey{{Name: "id", CollationOK: true, EqualityRead: true, EqualityOK: true}}}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER TABLE app.t ADD UNIQUE (payload) INCLUDE (id)`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "t" || provider.objectCalls[0].Type != "pg_partition_constraint" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter.constraint_columns.valid" && f.Metadata["reason_code"] == "constraint_partition_column_missing" {
			return
		}
	}
	t.Fatalf("missing partition-key finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGCreateSchemaTargetThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGCreateSchema: &spec.PGCreateSchemaState{Target: spec.PGCreateSchemaTarget{Name: "app", Owner: spec.PGTableOwnerTarget{Kind: "named", Name: "absent_role"}}, NameKnown: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `CREATE SCHEMA app AUTHORIZATION absent_role`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) < 1 || provider.objectCalls[len(provider.objectCalls)-1].Name != "app" || provider.objectCalls[len(provider.objectCalls)-1].Type != "pg_create_schema" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.create_schema.target.valid" && f.Metadata["reason_code"] == "schema_owner_absent" {
			return
		}
	}
	t.Fatalf("missing CREATE SCHEMA owner finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGCollationRefreshThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGCollationRefresh: &spec.PGCollationRefreshState{Target: spec.PGColumnCollation{Schema: "pg_catalog", Name: "default"}, Known: true, Exists: true, Default: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER COLLATION pg_catalog."default" REFRESH VERSION`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "default" || provider.objectCalls[0].Type != "pg_collation_refresh" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter_collation.refresh.target.valid" && f.Metadata["reason_code"] == "collation_refresh_database_default" {
			return
		}
	}
	t.Fatalf("missing ALTER COLLATION default-target finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGCollationChangeThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGCollationChange: &spec.PGCollationChangeState{Target: spec.PGCollationChangeTarget{Source: spec.PGColumnCollation{Schema: "app", Name: "source"}, Action: "rename", Destination: "occupied"}, SourceKnown: true, SourceExists: true, DestinationOccupied: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER COLLATION app.source RENAME TO occupied`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "source" || provider.objectCalls[0].Type != "pg_collation_change" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter_collation.change.target.valid" && f.Metadata["reason_code"] == "collation_change_destination_occupied" {
			return
		}
	}
	t.Fatalf("missing ALTER COLLATION namespace collision finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGIndexAttachmentThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGIndexAttachment: &spec.PGIndexAttachmentState{Target: spec.PGIndexAttachment{Parent: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "mv"}}, Child: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "child"}}}, ParentKind: "I", ChildKind: "i", OtherParent: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER INDEX app.mv ATTACH PARTITION app.child`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "mv" || provider.objectCalls[0].Type != "pg_index_attachment" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter_index.attach_partition.valid" && f.Metadata["reason_code"] == "index_attachment_child_already_attached" {
			return
		}
	}
	t.Fatalf("missing ALTER INDEX partition attachment finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGIndexOptionsThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGIndexOptions: &spec.PGIndexOptionsState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "mv"}}, Kind: "i", Method: "btree"}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER INDEX app.mv SET (fillfactor=9)`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "mv" || provider.objectCalls[0].Type != "pg_index_options" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter_index.options.valid" && f.Metadata["reason_code"] == "index_option_out_of_range" {
			return
		}
	}
	t.Fatalf("missing ALTER INDEX storage option finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGIndexIncludeThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGIndexInclude: &spec.PGIndexIncludeState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "t"}}, Kind: "r", Method: "btree", Columns: []string{"id"}, System: []string{"ctid"}}},
	}
	result, err := Audit(context.Background(), Request{SQL: `CREATE INDEX idx ON app.t(id) INCLUDE (missing)`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "t" || provider.objectCalls[0].Type != "pg_index_include" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.create_index.include.valid" && f.Metadata["reason_code"] == "index_include_column_absent" {
			return
		}
	}
	t.Fatalf("missing CREATE INDEX INCLUDE finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGOperatorFamilyTargetThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGOperatorFamily: &spec.PGOperatorFamilyState{Target: spec.PGOperatorFamilyTarget{Schema: "app", Name: "family", Method: "btree"}, SchemaExists: true, MethodKind: "i", Exists: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `CREATE OPERATOR FAMILY app.family USING btree`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "family" || provider.objectCalls[0].Type != "pg_operator_family" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.create_operator_family.target.valid" && f.Metadata["reason_code"] == "operator_family_already_exists" {
			return
		}
	}
	t.Fatalf("missing CREATE OPERATOR FAMILY target finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGOperatorFamilyMemberThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGOperatorFamilyMember: &spec.PGOperatorFamilyMemberState{Target: spec.PGOperatorFamilyTarget{Schema: "app", Name: "family", Method: "btree"}, Member: spec.PGOperatorFamilyMember{Number: 1, Drop: true, Left: spec.PGColumnTypeTarget{Schema: "pg_catalog", Name: "int4"}, Right: spec.PGColumnTypeTarget{Schema: "pg_catalog", Name: "int4"}}, SchemaExists: true, MethodKind: "i", FamilyExists: true, BTree: true, OperandTypesRead: true, OperandTypesOK: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER OPERATOR FAMILY app.family USING btree DROP OPERATOR 1 (integer)`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "family" || provider.objectCalls[0].Type != "pg_operator_family_member" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter_operator_family.member.valid" && f.Metadata["reason_code"] == "operator_family_slot_absent" {
			return
		}
	}
	t.Fatalf("missing ALTER OPERATOR FAMILY member finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGIndexDestinationThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGIndexDestination: &spec.PGIndexDestinationState{Source: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "mv"}}, Action: "set_tablespace", Destination: "absent_sp", Kind: "i"}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER INDEX app.mv SET TABLESPACE absent_sp`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "mv" || provider.objectCalls[0].Type != "pg_index_destination" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter_index.destination.valid" && f.Metadata["reason_code"] == "index_tablespace_absent" {
			return
		}
	}
	t.Fatalf("missing ALTER INDEX tablespace finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGOperatorFamilyChangeThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGOperatorFamilyChange: &spec.PGOperatorFamilyChangeState{Target: spec.PGOperatorFamilyChange{Source: spec.PGOperatorFamilyTarget{Schema: "app", Name: "family", Method: "btree"}, Action: "rename", Destination: "occupied"}, SourceSchemaExists: true, MethodKind: "i", SourceExists: true, DestinationOccupied: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER OPERATOR FAMILY app.family USING btree RENAME TO occupied`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "family" || provider.objectCalls[0].Type != "pg_operator_family_change" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter_operator_family.change.target.valid" && f.Metadata["reason_code"] == "operator_family_destination_occupied" {
			return
		}
	}
	t.Fatalf("missing ALTER OPERATOR FAMILY rename finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGCreateLikeThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGCreateLike: &spec.PGCreateSources{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "mv"}}, SchemaExists: true, Sources: []spec.PGCreateSource{{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "absent"}}}}}},
	}
	result, err := Audit(context.Background(), Request{SQL: `CREATE TABLE app.mv (LIKE app.absent)`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "mv" || provider.objectCalls[0].Type != "pg_create_like" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.create_table.like.target.valid" && f.Metadata["reason_code"] == "create_like_source_absent" {
			return
		}
	}
	t.Fatalf("missing CREATE TABLE LIKE source finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGCreateCommitThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGCreateDestination: &spec.PGCreateDestinationState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "mv"}}, SchemaExists: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `CREATE TABLE app.mv (id integer) ON COMMIT DROP`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "mv" || provider.objectCalls[0].Type != "pg_create_destination" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.create_table.on_commit.valid" && f.Metadata["reason_code"] == "create_commit_temporary_required" {
			return
		}
	}
	t.Fatalf("missing CREATE TABLE ON COMMIT finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGCreateTypedThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGCreateTyped: &spec.PGCreateTypedState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "mv"}}, Type: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "private_type"}}, TargetSchema: true, TypeSchema: true, TypeExists: true, Defined: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `CREATE TABLE app.mv OF app.private_type`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "mv" || provider.objectCalls[0].Type != "pg_create_typed" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.create_table.of.valid" && f.Metadata["reason_code"] == "create_typed_type_not_standalone_composite" {
			return
		}
	}
	t.Fatalf("missing CREATE TABLE OF finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGCreateStorageThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGCreateStorage: &spec.PGCreateStorageState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "mv"}}, SchemaExists: true, Method: "btree", MethodKind: "i"}},
	}
	result, err := Audit(context.Background(), Request{SQL: `CREATE TABLE app.mv (id integer) USING btree`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "mv" || provider.objectCalls[0].Type != "pg_create_storage" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.create_table.storage.valid" && f.Metadata["reason_code"] == "create_storage_method_not_table" {
			return
		}
	}
	t.Fatalf("missing CREATE TABLE storage finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGCreateKeysThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGCreateDestination: &spec.PGCreateDestinationState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "mv"}}, SchemaExists: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `CREATE TABLE app.mv (id integer, UNIQUE(absent))`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "mv" || provider.objectCalls[0].Type != "pg_create_destination" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.create_table.keys.valid" && f.Metadata["reason_code"] == "create_keys_column_absent" {
			return
		}
	}
	t.Fatalf("missing CREATE TABLE key finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGCreateExpressionThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGCreateDestination: &spec.PGCreateDestinationState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "mv"}}, SchemaExists: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `CREATE TABLE app.mv (id integer DEFAULT (SELECT 1))`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "mv" || provider.objectCalls[0].Type != "pg_create_destination" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.create_table.expression.valid" && f.Metadata["reason_code"] == "create_default_disallowed_reference" {
			return
		}
	}
	t.Fatalf("missing CREATE TABLE expression finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGCreateForeignKeyThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: false, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGCreateDestination: &spec.PGCreateDestinationState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "mv"}}, SchemaExists: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `CREATE TABLE app.mv (id integer REFERENCES app.t(id))`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "mv" || provider.objectCalls[0].Type != "pg_create_destination" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.create_table.foreign_key.valid" && f.Metadata["reason_code"] == "foreign_key_reference_absent" {
			return
		}
	}
	t.Fatalf("missing CREATE TABLE foreign-key finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGCreateInheritanceThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGCreateInheritance: &spec.PGCreateSources{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "mv"}}, SchemaExists: true, Sources: []spec.PGCreateSource{{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "absent"}}}}}},
	}
	result, err := Audit(context.Background(), Request{SQL: `CREATE TABLE app.mv () INHERITS (app.absent)`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "mv" || provider.objectCalls[0].Type != "pg_create_inheritance" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.create_table.inherits.valid" && f.Metadata["reason_code"] == "create_inheritance_parent_absent" {
			return
		}
	}
	t.Fatalf("missing CREATE TABLE LIKE source finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGCreateColumnsThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGCreateDestination: &spec.PGCreateDestinationState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "mv"}}, SchemaExists: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `CREATE TABLE app.mv (ctid integer)`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) != 1 || provider.objectCalls[0].Name != "mv" || provider.objectCalls[0].Type != "pg_create_destination" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.create_table.columns.valid" && f.Metadata["reason_code"] == "create_column_system_name" {
			return
		}
	}
	t.Fatalf("missing CREATE TABLE column finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGAlterEnumThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGEnum: &spec.PGEnumState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "mood"}}, SchemaExists: true, TypeExists: true, Defined: true, TypeKind: "e", Encoding: "UTF8", LabelBytes: 63, Labels: []string{"private_catalog_label"}}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER TYPE app.mood RENAME VALUE 'missing' TO 'new'`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) < 1 || provider.objectCalls[len(provider.objectCalls)-1].Name != "mood" || provider.objectCalls[len(provider.objectCalls)-1].Type != "pg_alter_enum" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter_type.enum.valid" && f.Metadata["reason_code"] == "alter_enum_old_label_absent" {
			return
		}
	}
	t.Fatalf("missing ALTER TYPE label finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGCompositeThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGComposite: &spec.PGCompositeState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "pair"}}, SchemaExists: true, RelationKind: "c", Attributes: []string{"private_catalog_label"}}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER TYPE app.pair DROP ATTRIBUTE missing`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) < 1 || provider.objectCalls[len(provider.objectCalls)-1].Name != "pair" || provider.objectCalls[len(provider.objectCalls)-1].Type != "pg_composite_type" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.type.composite.valid" && f.Metadata["reason_code"] == "composite_attribute_absent" {
			return
		}
	}
	t.Fatalf("missing ALTER TYPE attribute finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGTypeChangeThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGTypeChange: &spec.PGTypeChangeState{Target: spec.PGTypeChange{Source: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "pair"}}, Action: "rename", Destination: "used"}, SourceExists: true, Defined: true, Composite: true, DestinationRelation: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER TYPE app.pair RENAME TO used`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) < 1 || provider.objectCalls[len(provider.objectCalls)-1].Name != "pair" || provider.objectCalls[len(provider.objectCalls)-1].Type != "pg_type_change" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter_type.change.target.valid" && f.Metadata["reason_code"] == "alter_type_relation_occupied" {
			return
		}
	}
	t.Fatalf("missing ALTER TYPE destination finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGOperatorChangeThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGOperatorChange: &spec.PGOperatorChangeState{Target: spec.PGOperatorChange{Source: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "==="}}, Action: "set_schema", Destination: "used", Signature: true, Left: spec.PGColumnTypeTarget{Schema: "pg_catalog", Name: "int4"}, Right: spec.PGColumnTypeTarget{Schema: "pg_catalog", Name: "int4"}}, OperandSchema: true, OperandTypes: true, SourceSchema: true, SourceExists: true, DestinationSchema: true, DestinationOccupied: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER OPERATOR app.=== (pg_catalog.int4, pg_catalog.int4) SET SCHEMA used`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) < 1 || provider.objectCalls[len(provider.objectCalls)-1].Name != "===" || provider.objectCalls[len(provider.objectCalls)-1].Type != "pg_operator_change" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter_operator.change.target.valid" && f.Metadata["reason_code"] == "alter_operator_destination_occupied" {
			return
		}
	}
	t.Fatalf("missing ALTER OPERATOR destination finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGCreateRangeThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGEnum: &spec.PGEnumState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "mood"}}, SchemaExists: true, TypeExists: true, Defined: true, TypeKind: "e", Encoding: "UTF8", LabelBytes: 63, Labels: []string{"private_catalog_label"}}},
	}
	result, err := Audit(context.Background(), Request{SQL: `CREATE TYPE app.mood AS RANGE (unknown_option=1)`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) < 1 || provider.objectCalls[len(provider.objectCalls)-1].Name != "mood" || provider.objectCalls[len(provider.objectCalls)-1].Type != "pg_create_range" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.create_type.range.valid" && f.Metadata["reason_code"] == "range_option_unknown" {
			return
		}
	}
	t.Fatalf("missing CREATE TYPE range finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGCreateBaseTypeThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGEnum: &spec.PGEnumState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "mood"}}, SchemaExists: true, TypeExists: true, Defined: false, Encoding: "UTF8", LabelBytes: 63, Labels: []string{"private_catalog_label"}}},
	}
	result, err := Audit(context.Background(), Request{SQL: `CREATE TYPE app.mood (OUTPUT=pg_catalog.int4out)`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) < 1 || provider.objectCalls[len(provider.objectCalls)-1].Name != "mood" || provider.objectCalls[len(provider.objectCalls)-1].Type != "pg_base_type" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.type.base.valid" && f.Metadata["reason_code"] == "base_type_io_required" {
			return
		}
	}
	t.Fatalf("missing CREATE TYPE base finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGPolicyChangeThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGPolicyChange: &spec.PGPolicyChangeState{Table: spec.Table{Schema: "app", Name: "t"}, Name: "read", RelationKind: "r", PolicyCommand: "r", Columns: []string{"private_policy_column"}}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER POLICY read ON app.t WITH CHECK (true)`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) < 1 || provider.objectCalls[len(provider.objectCalls)-1].Name != "t" || provider.objectCalls[len(provider.objectCalls)-1].Type != "pg_policy_change" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter_policy.target.valid" && f.Metadata["reason_code"] == "alter_policy_with_check_inapplicable" {
			return
		}
	}
	t.Fatalf("missing ALTER POLICY clause finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGSequenceChangeThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGSequenceChange: &spec.PGSequenceChangeState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "s"}}, SchemaExists: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `CREATE SEQUENCE app.s INCREMENT 0`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) < 1 || provider.objectCalls[len(provider.objectCalls)-1].Name != "s" || provider.objectCalls[len(provider.objectCalls)-1].Type != "pg_sequence_change" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.sequence.declaration.valid" && f.Metadata["reason_code"] == "sequence_increment_zero" {
			return
		}
	}
	t.Fatalf("missing sequence declaration finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGWrapperChangeThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGWrapperChange: &spec.PGWrapperChangeState{Name: "wrapper", Exists: true, Options: []string{"present", "private_wrapper_option"}}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER FOREIGN DATA WRAPPER wrapper OPTIONS (DROP missing)`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) < 1 || provider.objectCalls[len(provider.objectCalls)-1].Name != "wrapper" || provider.objectCalls[len(provider.objectCalls)-1].Type != "pg_wrapper_change" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter_foreign_data_wrapper.target.valid" && f.Metadata["reason_code"] == "foreign_option_absent" {
			return
		}
	}
	t.Fatalf("missing wrapper option finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGForeignTableChangeThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGForeignTableChange: &spec.PGForeignTableChangeState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "ft"}}, Action: "drop_column", Column: "missing", RelationKind: "f"}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER FOREIGN TABLE app.ft DROP COLUMN missing`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) < 1 || provider.objectCalls[len(provider.objectCalls)-1].Name != "ft" || provider.objectCalls[len(provider.objectCalls)-1].Type != "pg_foreign_table_change" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter_foreign_table.target.valid" && f.Metadata["reason_code"] == "foreign_table_column_absent" {
			return
		}
	}
	t.Fatalf("missing foreign table column finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGTextSearchChangeThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGTextSearchChange: &spec.PGTextSearchChangeState{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "dict"}}, Kind: "dictionary", Action: "set_dictionary_options", SourceSchema: true, SourceExists: true, SimpleTemplate: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER TEXT SEARCH DICTIONARY app.dict (unknown_option = 1)`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) < 1 || provider.objectCalls[len(provider.objectCalls)-1].Name != "dict" || provider.objectCalls[len(provider.objectCalls)-1].Type != "pg_text_search_change" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.text_search.change.target.valid" && f.Metadata["reason_code"] == "text_search_dictionary_option_unknown" {
			return
		}
	}
	t.Fatalf("missing dictionary option finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGSubscriptionChangeThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGSubscriptionChange: &spec.PGSubscriptionChangeState{Name: "app_sub", Exists: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `ALTER SUBSCRIPTION app_sub ENABLE`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) < 1 || provider.objectCalls[len(provider.objectCalls)-1].Name != "app_sub" || provider.objectCalls[len(provider.objectCalls)-1].Type != "pg_subscription_change" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.alter_subscription.target.valid" && f.Metadata["reason_code"] == "subscription_slot_required" {
			return
		}
	}
	t.Fatalf("missing subscription slot finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGLanguageThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGLanguage: &spec.PGLanguageState{Name: "fixture_lang", Database: "postgres", Routines: []spec.PGLanguageRoutineState{{Target: spec.PGLanguageRoutine{Kind: "handler", Schema: "app", Name: "missing_handler"}}}}},
	}
	result, err := Audit(context.Background(), Request{SQL: `CREATE LANGUAGE fixture_lang HANDLER app.missing_handler`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) < 1 || provider.objectCalls[len(provider.objectCalls)-1].Name != "fixture_lang" || provider.objectCalls[len(provider.objectCalls)-1].Type != "pg_language_declaration" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.create_language.handler.valid" && f.Metadata["reason_code"] == "language_callback_absent" {
			return
		}
	}
	t.Fatalf("missing missing language callback finding: %+v", result.Statements[0].Findings)
}

func TestAuditPGGrantDeclarationThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "PostgreSQL 17.7"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{PGGrant: &spec.PGGrantState{ObjectKind: "table", Targets: []spec.PGGrantTargetState{{Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "fixture"}}, Exists: true, Kind: "r", Columns: []string{"catalog_only_column"}}}, Grantees: []spec.PGTableOwnerTarget{{Kind: "public"}}, GranteeExists: []bool{true}}},
	}
	result, err := Audit(context.Background(), Request{SQL: `GRANT SELECT (missing_column) ON TABLE app.fixture TO PUBLIC`, Dialect: DialectPostgreSQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) < 1 || provider.objectCalls[len(provider.objectCalls)-1].Name != "table" || provider.objectCalls[len(provider.objectCalls)-1].Type != "pg_grant_declaration" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.grant.declaration.valid" && f.Metadata["reason_code"] == "grant_column_absent" {
			return
		}
	}
	t.Fatalf("missing missing column finding: %+v", result.Statements[0].Findings)
}

func TestAuditMySQLStoredObjectThroughSDK(t *testing.T) {
	provider := &fakeMetadataProviderWithObjects{
		fakeMetadataProvider: fakeMetadataProvider{instance: &InstanceFacts{Version: "8.0.46"},
			snapshot: &TableSnapshot{Schema: "app", Exists: true, Table: &Table{Name: "t"},
				PGRelation: PGRelationState{Name: "t", Kind: "r"}, PGColumns: &PGColumnCatalog{Ordinary: true}}},
		objectSnapshot: &spec.ObjectSnapshot{MySQLStoredObject: &spec.MySQLStoredObjectState{Target: spec.MySQLStoredObjectTarget{Kind: "event", Name: spec.Table{Schema: "app", Name: "fixture_event"}}, SourceSeen: true}},
	}
	result, err := Audit(context.Background(), Request{SQL: `CREATE EVENT app.fixture_event ON SCHEDULE EVERY 1 DAY DISABLE DO SET @audit_marker=1`, Dialect: DialectMySQL, MetadataProvider: provider})
	if err != nil || result.Verdict != VerdictReject || len(provider.objectCalls) < 1 || provider.objectCalls[len(provider.objectCalls)-1].Name != "fixture_event" || provider.objectCalls[len(provider.objectCalls)-1].Type != "mysql_stored_object" {
		t.Fatalf("result=%+v calls=%+v err=%v", result, provider.objectCalls, err)
	}
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.mysql.stored_object.target.valid" && f.Metadata["reason_code"] == "event_name_occupied" {
			return
		}
	}
	t.Fatalf("missing event conflict finding: %+v", result.Statements[0].Findings)
}
