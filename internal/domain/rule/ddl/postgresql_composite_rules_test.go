package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGCompositeAssessment(t *testing.T) {
	target := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "pair"}}
	for _, tc := range []struct {
		action, name, newName, relation, reason, gap string
		catalog, authored                            []string
		guard, occupied                              bool
	}{
		{action: "create", authored: []string{"ctid", " X ", " x "}},
		{action: "create", authored: []string{" X ", " X "}, reason: "composite_attribute_duplicate"},
		{action: "create", authored: []string{"a", "a"}, occupied: true, reason: "composite_type_name_occupied"},
		{action: "add_attribute", name: " X ", relation: "c", catalog: []string{" x "}},
		{action: "drop_attribute", name: "missing", relation: "c", catalog: []string{}, guard: true},
		{action: "drop_attribute", name: "missing", relation: "c", gap: "composite_type.attributes"},
		{action: "rename_attribute", name: "a", newName: "a", relation: "c", catalog: []string{"a"}, reason: "composite_attribute_duplicate"},
		{action: "rename_attribute", name: "missing", newName: "a", relation: "c", catalog: []string{"a"}, reason: "composite_attribute_absent"},
		{action: "add_attribute", name: "a", relation: "r", reason: "composite_type_wrong_relation_kind"},
		{gap: "composite_type.action_order"},
	} {
		op := spec.DDLOperationAlterType
		if tc.action == "create" {
			op = spec.DDLOperationCreateType
		}
		s := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL,
			DDL: &spec.DDL{Operation: op, PGComposite: &spec.PGCompositeDeclaration{Target: target,
				Action: tc.action, Attributes: tc.authored, Name: tc.name, NewName: tc.newName, IfExists: tc.guard}},
			Metadata: &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"},
				PGComposite: &spec.PGCompositeState{Target: target, SchemaExists: true,
					TypeOccupied: tc.occupied, RelationKind: tc.relation, Attributes: tc.catalog}}}
		if reason, gap := assessPGComposite(s); reason != tc.reason || gap != tc.gap {
			t.Fatalf("%+v: assessment = (%q, %q), want (%q, %q)", tc, reason, gap, tc.reason, tc.gap)
		}
	}
}
