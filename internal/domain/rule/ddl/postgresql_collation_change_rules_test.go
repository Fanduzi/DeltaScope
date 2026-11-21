package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGCollationChangeAssessment(t *testing.T) {
	for _, tc := range []struct {
		name, action, reason string
		state                spec.PGCollationChangeState
	}{
		{name: "source absence precedes destination", action: "set_schema", reason: "collation_change_source_absent"},
		{name: "same namespace is a no-op", action: "set_schema", state: spec.PGCollationChangeState{SourceExists: true, DestinationExists: true, SameNamespace: true, DestinationOccupied: true}},
		{name: "rename collision", action: "rename", state: spec.PGCollationChangeState{SourceExists: true, DestinationOccupied: true}, reason: "collation_change_destination_occupied"},
		{name: "name exists in unsupported destination", action: "set_schema", state: spec.PGCollationChangeState{SourceExists: true, DestinationExists: true, NamespaceUnsupported: true, DestinationOccupied: true}, reason: "collation_change_schema_inapplicable"},
		{name: "missing literal owner", action: "set_owner", state: spec.PGCollationChangeState{SourceExists: true}, reason: "collation_change_owner_absent"},
		{name: "missing literal owner precedes source absence", action: "set_owner", reason: "collation_change_owner_absent"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := spec.PGCollationChangeTarget{Source: spec.PGColumnCollation{Schema: "app", Name: "Exact C"}, Action: tc.action}
			if tc.action == "set_owner" {
				target.Owner = spec.PGTableOwnerTarget{Kind: "named", Name: "owner"}
			}
			tc.state.Target, tc.state.SourceKnown = target, true
			stmt := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL,
				DDL:      &spec.DDL{Operation: spec.DDLOperationAlterCollation, PGCollationChange: &target},
				Metadata: &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, PGCollationChange: &tc.state},
			}
			if reason, gap := assessPGCollationChange(stmt); reason != tc.reason || gap != "" {
				t.Fatalf("got (%q,%q), want %q", reason, gap, tc.reason)
			}
		})
	}
}
