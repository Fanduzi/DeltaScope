package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGCreateSchemaTargetAssessment(t *testing.T) {
	owner := spec.PGTableOwnerTarget{Kind: "named", Name: " Exact Owner "}
	for _, tt := range []struct {
		name, reason, fact string
		state              spec.PGCreateSchemaState
		guard              bool
	}{
		{name: "owner checked before duplicate guard", state: spec.PGCreateSchemaState{NameKnown: true, Exists: true}, guard: true, reason: "schema_owner_absent"},
		{name: "guard skips destination with present owner", state: spec.PGCreateSchemaState{OwnerExists: true, NameKnown: true, Exists: true}, guard: true},
		{name: "unguarded duplicate", state: spec.PGCreateSchemaState{OwnerExists: true, NameKnown: true, Exists: true}, reason: "schema_target_exists"},
		{name: "reserved before guard", state: spec.PGCreateSchemaState{OwnerExists: true, NameKnown: true, Exists: true, Reserved: true, SystemSchemaSettingKnown: true}, guard: true, reason: "schema_name_reserved"},
		{name: "unknown reserved setting", state: spec.PGCreateSchemaState{OwnerExists: true, NameKnown: true, Reserved: true}, fact: "schema.system_name_setting"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			target := spec.PGCreateSchemaTarget{Owner: owner}
			tt.state.Target = target
			s := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL,
				DDL:      &spec.DDL{Operation: spec.DDLOperationCreateSchema, PGCreateSchema: &target, Options: map[string]string{}},
				Metadata: &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, PGCreateSchema: &tt.state},
			}
			if tt.guard {
				s.DDL.Options["if_not_exists"] = "true"
			}
			if reason, fact := assessPGCreateSchemaTarget(s); reason != tt.reason || fact != tt.fact {
				t.Fatalf("got (%q,%q), want (%q,%q)", reason, fact, tt.reason, tt.fact)
			}
		})
	}
}
