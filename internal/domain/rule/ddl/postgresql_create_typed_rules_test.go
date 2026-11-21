package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGCreateTypedAssessment(t *testing.T) {
	target := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "dst"}}
	source := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "Types", Name: "C"}}
	for _, tc := range []struct {
		name        string
		adjust      func(*spec.Statement, *spec.PGCreateTypedState)
		reason, gap string
	}{
		{name: "qualified exact column"},
		{name: "guard before type resolution", adjust: func(s *spec.Statement, state *spec.PGCreateTypedState) {
			s.DDL.PGCreateTyped.IfNotExists, state.TargetExists, state.TypeExists = true, true, false
		}},
		{name: "table row type", adjust: func(_ *spec.Statement, state *spec.PGCreateTypedState) {
			state.Composite = false
		}, reason: "create_typed_type_not_standalone_composite"},
		{name: "wrong case", adjust: func(s *spec.Statement, _ *spec.PGCreateTypedState) {
			s.DDL.Columns[0].Name = " x "
		}, reason: "create_typed_option_column_absent"},
		{name: "duplicate option", adjust: func(s *spec.Statement, _ *spec.PGCreateTypedState) {
			s.DDL.Columns = append(s.DDL.Columns, s.DDL.Columns[0])
		}, reason: "create_typed_option_column_duplicate"},
		{name: "proved empty", adjust: func(_ *spec.Statement, state *spec.PGCreateTypedState) {
			state.Columns = []string{}
		}, reason: "create_typed_option_column_absent"},
		{name: "unknown columns", adjust: func(_ *spec.Statement, state *spec.PGCreateTypedState) {
			state.Columns = nil
		}, gap: "create_table.typed_columns"},
		{name: "unqualified", adjust: func(s *spec.Statement, _ *spec.PGCreateTypedState) {
			s.DDL.PGCreateTyped.Type.Table.Schema = ""
		}, gap: "create_table.typed_namespace"},
		{name: "mismatched catalog", adjust: func(_ *spec.Statement, state *spec.PGCreateTypedState) {
			state.Type.Table.Name = "Different"
		}, gap: "create_table.typed_catalog"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := &spec.PGCreateTypedState{Target: target, Type: source, TargetSchema: true, TypeSchema: true,
				TypeExists: true, Defined: true, Composite: true, Columns: []string{" X "}}
			s := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL,
				DDL: &spec.DDL{Operation: spec.DDLOperationCreateTable, Columns: []spec.Column{{Name: " X "}},
					PGCreateTyped: &spec.PGCreateTyped{Target: target, Type: source}},
				Metadata: &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, PGCreateTyped: state}}
			if tc.adjust != nil {
				tc.adjust(&s, state)
			}
			if reason, gap := assessPGCreateTyped(s); reason != tc.reason || gap != tc.gap {
				t.Fatalf("assessment = (%q, %q), want (%q, %q)", reason, gap, tc.reason, tc.gap)
			}
		})
	}
}
