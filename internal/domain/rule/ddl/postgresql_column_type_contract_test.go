package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGColumnTypeCatalogAndModifiers(t *testing.T) {
	name := spec.PGColumnCollation{Schema: " Exact Schema ", Name: " Exact Collation "}
	for _, tc := range []struct {
		label, typeName, reason, gap                                               string
		mods                                                                       *spec.PGColumnTypeModifiers
		coll                                                                       *spec.PGColumnCollation
		collKnown, collExists, collatable, assignment, assignable, using, misbound bool
	}{
		{label: "no assignment cast", typeName: "int4", assignment: true, reason: "destination_type_assignment_unavailable"},
		{label: "assignment read missing", typeName: "int4", gap: "column.type_assignment"},
		{label: "assignment identity differs", typeName: "int4", assignment: true, misbound: true, gap: "column.type_assignment"},
		{label: "using expression", typeName: "int4", using: true},
		{label: "varchar length too large", typeName: "varchar", mods: &spec.PGColumnTypeModifiers{Known: true, Count: 1, Values: [2]int32{10485761}}, reason: "destination_character_type_modifier_invalid"},
		{label: "varchar max length", typeName: "varchar", assignment: true, assignable: true, mods: &spec.PGColumnTypeModifiers{Known: true, Count: 1, Values: [2]int32{10485760}}},
		{label: "numeric scale too negative", typeName: "numeric", mods: &spec.PGColumnTypeModifiers{Known: true, Count: 2, Values: [2]int32{7, -1001}}, reason: "destination_numeric_type_modifier_invalid"},
		{label: "numeric negative scale", typeName: "numeric", assignment: true, assignable: true, mods: &spec.PGColumnTypeModifiers{Known: true, Count: 2, Values: [2]int32{7, -1000}}},
		{label: "unmodeled modifier", typeName: "varchar", mods: &spec.PGColumnTypeModifiers{Count: 1}, gap: "column.type_modifiers"},
		{label: "missing collation precedes target kind", typeName: "int4", coll: &name, collKnown: true, reason: "destination_collation_absent"},
		{label: "exact collation uses actual type collatability", typeName: "int4", coll: &name, collKnown: true, collExists: true, reason: "destination_type_not_collatable"},
		{label: "case exact collation", typeName: "text", coll: &name, collKnown: true, collExists: true, collatable: true, assignment: true, assignable: true},
		{label: "collation catalog observation unavailable", typeName: "text", coll: &name, gap: "column.pg_collation"},
		{label: "collation unqualified", typeName: "text", coll: &spec.PGColumnCollation{}, gap: "column.collation_identity"},
	} {
		t.Run(tc.label, func(t *testing.T) {
			s := pgGenerationTransitionStatement("set_data_type", &spec.PGColumnState{})
			a := &s.DDL.Alter[0]
			a.PGColumnType = spec.PGColumnTypeTarget{Schema: "pg_catalog", Name: tc.typeName}
			a.PGColumnTypeModifiers, a.PGColumnCollation = tc.mods, tc.coll
			if tc.using {
				a.Options["has_using"] = "true"
			}
			facts := &spec.PGColumnTypeState{Target: a.PGColumnType, SchemaExists: true, BaseExists: true, Resolved: true, Defined: true,
				Collation: name, CollationKnown: tc.collKnown, CollationExists: tc.collExists, Collatable: tc.collatable,
				AssignmentKnown: tc.assignment, Assignable: tc.assignable, SourceSchema: "app", SourceTable: "t", SourceColumn: a.Name}
			if tc.misbound {
				facts.SourceColumn = "ID"
			}
			s.Metadata.PGDropColumnDependency = &spec.PGDropColumnDependency{Schema: "app", Table: "t", Column: a.Name}
			s.Metadata.PGColumnType = facts
			reason, gap := assessPGColumnTypeDestination(s)
			if reason != tc.reason || gap != tc.gap {
				t.Fatalf("reason=%q gap=%q", reason, gap)
			}
		})
	}
}
