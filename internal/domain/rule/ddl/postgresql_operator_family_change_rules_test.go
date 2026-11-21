package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGOperatorFamilyChangeAssessment(t *testing.T) {
	for _, tc := range []struct {
		name, action, reason string
		state                spec.PGOperatorFamilyChangeState
	}{
		{name: "owner checked before access method", action: "set_owner", reason: "operator_family_change_owner_absent"},
		{name: "method must be an index method", action: "rename", state: spec.PGOperatorFamilyChangeState{MethodKind: "t"}, reason: "operator_family_method_wrong_kind"},
		{name: "source family checked before destination", action: "set_schema", state: spec.PGOperatorFamilyChangeState{MethodKind: "i", SourceSchemaExists: true}, reason: "operator_family_absent"},
		{name: "rename collides in the same method", action: "rename", state: spec.PGOperatorFamilyChangeState{MethodKind: "i", SourceSchemaExists: true, SourceExists: true, DestinationOccupied: true}, reason: "operator_family_destination_occupied"},
		{name: "same schema move bypasses collision", action: "set_schema", state: spec.PGOperatorFamilyChangeState{MethodKind: "i", SourceSchemaExists: true, SourceExists: true, SameNamespace: true, DestinationExists: true, DestinationOccupied: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := spec.PGOperatorFamilyChange{Source: spec.PGOperatorFamilyTarget{Schema: "app", Name: "Exact Family", Method: "btree"}, Action: tc.action}
			if tc.action == "set_owner" {
				target.Owner = spec.PGTableOwnerTarget{Kind: "named", Name: "absent"}
			} else {
				target.Destination = "destination"
			}
			tc.state.Target = target
			s := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL,
				DDL:      &spec.DDL{Operation: spec.DDLOperationAlterOperatorFamily, PGOperatorFamilyChange: &target},
				Metadata: &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, PGOperatorFamilyChange: &tc.state},
			}
			if reason, gap := assessPGOperatorFamilyChange(s); reason != tc.reason || gap != "" {
				t.Fatalf("assessment=(%q,%q), want %q", reason, gap, tc.reason)
			}
		})
	}
}
