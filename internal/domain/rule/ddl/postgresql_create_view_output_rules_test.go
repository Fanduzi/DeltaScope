package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGCreateViewOutputAssessment(t *testing.T) {
	table := spec.Table{Schema: "app", Name: "report"}
	column := spec.PGViewOutputColumn{Name: "id", TypeOID: 23, Typmod: -1}
	for _, tc := range []struct {
		name, reason, fact string
		output             *spec.PGCreateViewOutput
		target             *spec.PGCreateViewTarget
	}{
		{name: "excess aliases precede namespace", output: &spec.PGCreateViewOutput{AliasCount: 2, Columns: []spec.PGViewOutputColumn{column}}, reason: "view_output_excess_aliases"},
		{name: "unproven output", output: &spec.PGCreateViewOutput{}, fact: "view.query_output"},
		{name: "observed empty output", output: &spec.PGCreateViewOutput{Columns: []spec.PGViewOutputColumn{}}, target: &spec.PGCreateViewTarget{Table: table, SchemaExists: true}},
		{name: "unproven existing output", output: &spec.PGCreateViewOutput{Columns: []spec.PGViewOutputColumn{column}},
			target: &spec.PGCreateViewTarget{Table: table, SchemaExists: true, Kind: "v"}, fact: "view.existing_output"},
		{name: "column type modifier changed", output: &spec.PGCreateViewOutput{Columns: []spec.PGViewOutputColumn{column}},
			target: &spec.PGCreateViewTarget{Table: table, SchemaExists: true, Kind: "v", Columns: []spec.PGViewOutputColumn{{Name: "id", TypeOID: 23, Typmod: 7}}}, reason: "view_output_column_type_changed"},
		{name: "addition after exact prefix", output: &spec.PGCreateViewOutput{Columns: []spec.PGViewOutputColumn{column, {Name: "label", TypeOID: 25, Typmod: -1, Collation: 100}}},
			target: &spec.PGCreateViewTarget{Table: table, SchemaExists: true, Kind: "v", Columns: []spec.PGViewOutputColumn{column}}},
		{name: "temporary in a persistent schema", output: &spec.PGCreateViewOutput{Temporary: true, Columns: []spec.PGViewOutputColumn{column}},
			target: &spec.PGCreateViewTarget{Table: table, SchemaExists: true}, reason: "view_output_temporary_in_persistent_schema"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := spec.Statement{Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL,
				DDL:      &spec.DDL{Operation: spec.DDLOperationCreateView, Table: &table, Options: map[string]string{"replace": "true"}, PGCreateViewOutput: tc.output},
				Metadata: &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, PGCreateViewTarget: tc.target},
			}
			if reason, fact := assessPGCreateViewOutput(s); reason != tc.reason || fact != tc.fact {
				t.Fatalf("reason=%q fact=%q, want %q and %q", reason, fact, tc.reason, tc.fact)
			}
		})
	}
}
