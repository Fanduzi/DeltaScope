package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGEnumLabelAssessment(t *testing.T) {
	target := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "mood"}}
	s := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL,
		DDL: &spec.DDL{Operation: spec.DDLOperationCreateType, PGCreateEnum: &spec.PGCreateEnum{Target: target, Labels: []string{"", ""}}},
		Metadata: &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, PGEnum: &spec.PGEnumState{
			Target: target, SchemaExists: true, TypeExists: true, Defined: true, TypeKind: "e", Encoding: "UTF8", LabelBytes: 63, Labels: []string{"", " present "},
		}}}
	if reason, gap := assessPGCreateEnum(s); reason != "create_enum_label_duplicate" || gap != "" {
		t.Fatalf("CREATE ENUM refusal = %q, gap = %q", reason, gap)
	}
	s.DDL.Operation = spec.DDLOperationAlterType
	s.DDL.PGCreateEnum = nil
	for _, tc := range []struct {
		name, value, reason string
		old, neighbor       *string
		guard               bool
	}{
		{name: "empty neighbor", value: "new", neighbor: new("")},
		{name: "missing old before occupied new", value: " present ", old: new("missing"), reason: "alter_enum_old_label_absent"},
		{name: "exact whitespace", value: "present", old: new(" present ")},
		{name: "duplicate guard precedes missing neighbor", value: "", neighbor: new("missing"), guard: true},
		{name: "rename old to itself", value: "", old: new(""), reason: "alter_enum_label_occupied"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s.DDL.PGAlterEnum = &spec.PGAlterEnum{Target: target, Known: true, Value: tc.value, Old: tc.old,
				Neighbor: tc.neighbor, IfNotExists: tc.guard}
			if reason, gap := assessPGAlterEnum(s); reason != tc.reason || gap != "" {
				t.Fatalf("ALTER ENUM refusal = %q, gap = %q, want %q", reason, gap, tc.reason)
			}
		})
	}
}
