package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGCreateInheritanceAssessment(t *testing.T) {
	target := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "dst"}}
	parent := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "Other", Name: " Parent "}}
	for _, tc := range []struct {
		name, kind, reason, gap string
		partition               *bool
		guard, repeated         bool
	}{
		{name: "ordinary", kind: "r", partition: new(false), gap: "create_table.inheritance_column_merge"},
		{name: "unknown partition status", kind: "r", gap: "create_table.inheritance_parent_partition"},
		{name: "parent absent", reason: "create_inheritance_parent_absent"},
		{name: "guard before parent absence", guard: true},
		{name: "partition child", kind: "r", partition: new(true), reason: "create_inheritance_parent_partition"},
		{name: "partition root", kind: "p", reason: "create_inheritance_parent_wrong_kind"},
		{name: "duplicate before kind", kind: "v", repeated: true, reason: "create_inheritance_parent_duplicate"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decl := &spec.PGCreateInheritance{Target: target, Parents: []spec.PGDestructiveTableTarget{parent}, IfNotExists: tc.guard}
			state := &spec.PGCreateSources{Target: target, SchemaExists: true, Exists: tc.guard,
				Sources: []spec.PGCreateSource{{Target: parent, Kind: tc.kind, Partition: tc.partition}}}
			if tc.repeated {
				decl.Parents = append(decl.Parents, parent)
				state.Sources = append(state.Sources, state.Sources[0])
			}
			s := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL,
				DDL:      &spec.DDL{Operation: spec.DDLOperationCreateTable, Table: &target.Table, PGCreateInheritance: decl},
				Metadata: &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, PGCreateInheritance: state}}
			if reason, gap := assessPGCreateInheritance(s); reason != tc.reason || gap != tc.gap {
				t.Fatalf("assessment = (%q, %q), want (%q, %q)", reason, gap, tc.reason, tc.gap)
			}
		})
	}
}
