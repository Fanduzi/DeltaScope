package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGCreateStorageAssessment(t *testing.T) {
	target := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "dst"}}
	for _, tc := range []struct {
		name        string
		storage     spec.PGCreateStorage
		schema      bool
		space       bool
		global      bool
		kind        string
		reason, gap string
	}{
		{name: "explicit heap", storage: spec.PGCreateStorage{Method: "heap"}, schema: true, kind: "t"},
		{name: "wrong method", storage: spec.PGCreateStorage{Method: "btree"}, schema: true, kind: "i", reason: "create_storage_method_not_table"},
		{name: "tablespace before method", storage: spec.PGCreateStorage{Method: "btree", Tablespace: "absent"}, schema: true, kind: "i", reason: "create_storage_tablespace_absent"},
		{name: "global tablespace", storage: spec.PGCreateStorage{Tablespace: "pg_global"}, schema: true, space: true, global: true, reason: "create_storage_global_tablespace"},
		{name: "guard before missing method", storage: spec.PGCreateStorage{Method: "absent", IfNotExists: true}, schema: true},
		{name: "invalid fillfactor before method", storage: spec.PGCreateStorage{Method: "btree", Options: []spec.PGTableReloption{
			{Name: "fillfactor", HasValue: true, Value: spec.PGTableReloptionInteger, Integer: 1},
		}}, schema: true, kind: "i", reason: "table_reloption_out_of_range"},
		{name: "unknown option", storage: spec.PGCreateStorage{Options: []spec.PGTableReloption{{Name: "unknown"}}}, schema: true, gap: "table.reloptions"},
		{name: "session target", storage: spec.PGCreateStorage{SessionDependent: true}, gap: "create_table.storage_namespace"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.storage.Target = target
			s := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL,
				DDL: &spec.DDL{Operation: spec.DDLOperationCreateTable, PGCreateStorage: &tc.storage},
				Metadata: &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, PGCreateStorage: &spec.PGCreateStorageState{
					Target: target, SchemaExists: tc.schema, TargetExists: true, Method: tc.storage.Method, MethodKind: tc.kind,
					Tablespace: tc.storage.Tablespace, TablespaceExists: tc.space, Global: tc.global,
				}},
			}
			if reason, _, gap := assessPGCreateStorage(s); reason != tc.reason || gap != tc.gap {
				t.Fatalf("assessment = (%q, %q), want (%q, %q)", reason, gap, tc.reason, tc.gap)
			}
		})
	}
}
