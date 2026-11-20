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
