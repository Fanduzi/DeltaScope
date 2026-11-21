package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGIndexDestinationAssessment(t *testing.T) {
	source := spec.PGDestructiveTableTarget{Table: spec.Table{Schema: "app", Name: "idx"}}
	for _, tc := range []struct {
		name, action, reason, gap string
		guard                     bool
		state                     spec.PGIndexDestinationState
	}{
		{name: "guarded missing source", action: "set_tablespace", guard: true},
		{name: "unguarded missing source", action: "rename", reason: "index_destination_source_absent"},
		{name: "composite rename", action: "rename", state: spec.PGIndexDestinationState{Kind: "c", DestinationExists: true}, reason: "index_destination_source_wrong_kind"},
		{name: "rename collides with a table", action: "rename", state: spec.PGIndexDestinationState{Kind: "i", DestinationExists: true}, reason: "index_rename_name_occupied"},
		{name: "table rename needs row type", action: "rename", state: spec.PGIndexDestinationState{Kind: "r", HasRowType: true}, gap: "index.rename_row_type"},
		{name: "constraint index needs companion name", action: "rename", state: spec.PGIndexDestinationState{Kind: "i", ConstraintBacked: true}, gap: "index.rename_constraint"},
		{name: "tablespace target must be an index", action: "set_tablespace", guard: true, state: spec.PGIndexDestinationState{Kind: "r"}, reason: "index_destination_source_wrong_kind"},
		{name: "partitioned index destination absent", action: "set_tablespace", state: spec.PGIndexDestinationState{Kind: "I"}, reason: "index_tablespace_absent"},
		{name: "same tablespace short circuits", action: "set_tablespace", state: spec.PGIndexDestinationState{Kind: "i", DestinationExists: true, SameTablespace: true, Mapped: true, Global: true}},
		{name: "system relation before global tablespace", action: "set_tablespace", state: spec.PGIndexDestinationState{Kind: "i", DestinationExists: true, Mapped: true, Global: true}, reason: "index_tablespace_mapped_relation"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := spec.PGIndexDestinationTarget{Source: source, Action: tc.action, Destination: "destination", IfExists: tc.guard}
			tc.state.Source, tc.state.Action, tc.state.Destination = target.Source, target.Action, target.Destination
			s := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL,
				DDL:      &spec.DDL{Operation: spec.DDLOperationAlterIndex, PGIndexDestination: &target},
				Metadata: &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, PGIndexDestination: &tc.state},
			}
			if reason, gap := assessPGIndexDestination(s); reason != tc.reason || gap != tc.gap {
				t.Fatalf("assessment = (%q,%q), want (%q,%q)", reason, gap, tc.reason, tc.gap)
			}
		})
	}
}
