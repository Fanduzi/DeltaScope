package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGCreateCommitAssessment(t *testing.T) {
	for _, tc := range []struct {
		name, schema, persistence, reason, gap string
		schemaExists, relationExists, guard    bool
	}{
		{name: "explicit temporary", persistence: "t"},
		{name: "literal persistent namespace rejects TEMP before guard", schema: "app", persistence: "t", schemaExists: true, relationExists: true, guard: true, reason: "create_commit_temporary_namespace_required"},
		{name: "missing schema before ON COMMIT", schema: "app", persistence: "p", reason: "create_commit_schema_absent"},
		{name: "guard skips ON COMMIT", schema: "app", persistence: "p", schemaExists: true, relationExists: true, guard: true},
		{name: "unguarded permanent table", schema: "app", persistence: "p", schemaExists: true, reason: "create_commit_temporary_required"},
		{name: "unlogged table", schema: "app", persistence: "u", schemaExists: true, reason: "create_commit_temporary_required"},
		{name: "session alias", schema: "pg_temp", persistence: "p", gap: "create_table.commit_namespace"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: tc.schema, Name: "t"}}
			s := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL,
				DDL: &spec.DDL{Operation: spec.DDLOperationCreateTable, PGCreateCommit: &spec.PGCreateCommit{
					Target: target, Persistence: tc.persistence, Action: "drop", IfNotExists: tc.guard,
				}},
				Metadata: &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, PGCreateDestination: &spec.PGCreateDestinationState{
					Target: target, SchemaExists: tc.schemaExists, RelationExists: tc.relationExists,
				}},
			}
			if reason, gap := assessPGCreateCommit(s); reason != tc.reason || gap != tc.gap {
				t.Fatalf("assessment=(%q,%q), want (%q,%q)", reason, gap, tc.reason, tc.gap)
			}
		})
	}
}
