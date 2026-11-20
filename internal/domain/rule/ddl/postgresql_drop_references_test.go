package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGDropReferenceSet(t *testing.T) {
	for _, tc := range []struct {
		name, firstKind, secondKind, reason string
		guard, cascade, observed, mismatch  bool
		position                            int
		gap                                 bool
	}{
		{name: "blocked after guarded absence", secondKind: "r", guard: true, observed: true, position: 2, reason: "drop_table_has_direct_dependency"},
		{name: "guarded entire set absent", guard: true},
		{name: "missing before dependency", secondKind: "r", observed: true, position: 2, reason: "relation_absent"},
		{name: "wrong kind before dependency", firstKind: "v", secondKind: "r", guard: true, observed: true, position: 2, reason: "relation_wrong_kind"},
		{name: "seen empty", firstKind: "r", secondKind: "r", observed: true},
		{name: "unobserved", firstKind: "r", secondKind: "r", gap: true},
		{name: "misbound", firstKind: "r", secondKind: "r", observed: true, mismatch: true, gap: true},
		{name: "CASCADE has separate semantics", firstKind: "r", secondKind: "r", cascade: true, observed: true, position: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			left, right := spec.Table{Schema: "app", Name: "left"}, spec.Table{Schema: " Exact ", Name: " Right "}
			targets := []spec.PGDestructiveTableTarget{{Table: left}, {Table: right}}
			s := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL,
				DDL: &spec.DDL{Operation: spec.DDLOperationDropTable, Table: &left, PGDestructiveTable: &spec.PGDestructiveTable{IfExists: tc.guard, Cascade: tc.cascade, Targets: targets}},
				Metadata: &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, PGDestructiveTableRelations: []spec.PGDestructiveTableRelation{
					{Schema: left.Schema, Relation: spec.PGRelationState{Name: left.Name, Kind: tc.firstKind}},
					{Schema: right.Schema, Relation: spec.PGRelationState{Name: right.Name, Kind: tc.secondKind}},
				}}}
			if tc.observed {
				s.Metadata.PGDestructiveReferences = &spec.PGDestructiveReferences{Targets: append([]spec.PGDestructiveTableTarget(nil), targets...), BlockedPosition: tc.position}
				if tc.mismatch {
					s.Metadata.PGDestructiveReferences.Targets[1].Table.Name = "right"
				}
			}
			r := tableOperationExistenceRule{}
			findings, err := r.evaluatePGDestructiveTable(context.Background(), s)
			if err != nil || len(findings) > 1 || (len(findings) == 1) != (tc.reason != "") {
				t.Fatalf("findings=%+v err=%v", findings, err)
			}
			if len(findings) == 1 && findings[0].Metadata["reason_code"] != tc.reason {
				t.Fatal(findings)
			}
			if tc.reason == "drop_table_has_direct_dependency" && (findings[0].Metadata["table"] != right.Name || findings[0].Metadata["schema"] != right.Schema) {
				t.Fatal(findings)
			}
			gaps := pgDestructiveTableEvidenceGaps(s)
			if len(gaps) > 1 || (len(gaps) == 1) != tc.gap || len(gaps) == 1 && (len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != "target_table.pg_drop_references") {
				t.Fatal(gaps)
			}
		})
	}
}
