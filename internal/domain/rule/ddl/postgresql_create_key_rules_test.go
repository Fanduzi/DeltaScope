package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGCreateKeysAssessment(t *testing.T) {
	target := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "dst"}}
	for _, tt := range []struct {
		name          string
		declarations  []spec.PGCreateKeyDeclaration
		guard, exists bool
		reason        string
	}{
		{name: "duplicate include allowed", declarations: []spec.PGCreateKeyDeclaration{{Keys: []string{" x, "}, Include: []string{" x, ", " x, "}}}},
		{name: "key spelling exact", declarations: []spec.PGCreateKeyDeclaration{{Keys: []string{" X, "}}}, reason: "create_keys_column_absent"},
		{name: "guard skips bad key", declarations: []spec.PGCreateKeyDeclaration{{Keys: []string{" X, "}}}, guard: true, exists: true},
		{name: "duplicate key before include binding", declarations: []spec.PGCreateKeyDeclaration{{Keys: []string{" x, ", " x, "}, Include: []string{"absent"}}}, reason: "create_keys_column_duplicate"},
		{name: "second primary", declarations: []spec.PGCreateKeyDeclaration{{Primary: true, Keys: []string{" x, "}}, {Primary: true, Keys: []string{"id"}}}, reason: "create_keys_multiple_primary"},
		{name: "using existing index", declarations: []spec.PGCreateKeyDeclaration{{UsingIndex: true}}, reason: "create_keys_using_existing_index"},
		{name: "system column", declarations: []spec.PGCreateKeyDeclaration{{Keys: []string{"ctid"}}}, reason: "create_keys_system_column"},
		{name: "destination before system column", declarations: []spec.PGCreateKeyDeclaration{{Keys: []string{"ctid"}}}, exists: true, reason: "create_keys_destination_occupied"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL,
				DDL: &spec.DDL{Operation: spec.DDLOperationCreateTable, Columns: []spec.Column{{Name: " x, "}, {Name: "id"}},
					PGCreateKeys: &spec.PGCreateKeys{Target: target, IfNotExists: tt.guard, Declarations: tt.declarations}},
				Metadata: &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"},
					PGCreateDestination: &spec.PGCreateDestinationState{Target: target, SchemaExists: true, RelationExists: tt.exists}}}
			if reason, gap := assessPGCreateKeys(s); reason != tt.reason || gap != "" {
				t.Fatalf("assessment = (%q, %q), want (%q, no gap)", reason, gap, tt.reason)
			}
		})
	}
}
