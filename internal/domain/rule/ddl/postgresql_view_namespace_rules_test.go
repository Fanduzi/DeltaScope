package ddl

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGViewNamespaceAssessment(t *testing.T) {
	for _, tc := range []struct {
		name, kind, reason, gap         string
		guard, unknown, misbound, index bool
	}{
		{name: "missing without guard", reason: "view_namespace_source_absent"},
		{name: "guarded missing source", guard: true},
		{name: "guarded wrong kind", kind: "v", guard: true, reason: "view_namespace_source_wrong_kind"},
		{name: "missing source evidence", guard: true, unknown: true, gap: "view.pg_namespace_destination"},
		{name: "mismatched destination evidence", kind: "m", misbound: true, gap: "view.pg_namespace_destination"},
		{name: "attached index conflicts", kind: "m", index: true, reason: "destination_index_conflict"},
		{name: "available destination", kind: "m"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := &spec.PGViewNamespaceSource{Schema: " Exact Schema ", Name: " Exact View ", IfExists: tc.guard}
			state := &spec.PGTableNamespace{Schema: source.Schema, Table: source.Name, Action: "set_schema", Destination: " Exact Destination ",
				SourceKind: tc.kind, SourceKindKnown: !tc.unknown, SchemaExists: true, RowTypeConflictKnown: true, ArrayTypeConflictKnown: true, IndexConflict: tc.index}
			if tc.misbound {
				state.Destination = " exact Destination "
			}
			s := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL,
				DDL:      &spec.DDL{Operation: spec.DDLOperationAlterMaterializedView, PGViewNamespaceSource: source, Options: map[string]string{"action": "set_schema", "new_schema": " Exact Destination "}},
				Metadata: &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, PGTableNamespace: state},
			}
			if reason, gap := assessPGViewNamespace(s); reason != tc.reason || gap != tc.gap {
				t.Fatalf("got %q/%q, want %q/%q", reason, gap, tc.reason, tc.gap)
			}
		})
	}
}
