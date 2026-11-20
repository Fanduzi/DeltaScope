package ddl

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGDestructiveTableOrderedTargets(t *testing.T) {
	for _, tc := range []struct {
		name, reason, missing string
		operation             spec.DDLOperation
		guard, only           bool
		kind                  string
		onlyPosition          int
		mutate                func(*spec.Statement)
	}{
		{name: "later absent", reason: "relation_absent"},
		{name: "later guarded absent", guard: true},
		{name: "guarded wrong kind", guard: true, kind: "v", reason: "relation_wrong_kind"},
		{name: "wrong exact name", missing: "target_table.pg_relation", mutate: func(s *spec.Statement) { s.Metadata.PGDestructiveTableRelations[1].Relation.Name = " Later " }},
		{name: "temporary namespace", missing: "target_table.pg_identity", mutate: func(s *spec.Statement) { s.DDL.PGDestructiveTable.Targets[1].Table.Schema = "pg_temp_44" }},
		{name: "catalog qualified", missing: "target_table.pg_identity", mutate: func(s *spec.Statement) { s.DDL.PGDestructiveTable.Targets[1].CatalogQualified = true }},
		{name: "unselected partitioned ONLY", operation: spec.DDLOperationTruncateTable, only: true, kind: "p", onlyPosition: 2, reason: "partition_root_only_inapplicable"},
		{name: "previously selected partitioned ONLY", operation: spec.DDLOperationTruncateTable, only: true, kind: "p"},
		{name: "ordinary partition traversal", operation: spec.DDLOperationTruncateTable, kind: "p"},
		{name: "foreign relation", operation: spec.DDLOperationTruncateTable, kind: "f", missing: "target_table.pg_fdw_truncate"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			operation := tc.operation
			if operation == "" {
				operation = spec.DDLOperationDropTable
			}
			first, last := spec.Table{Schema: "exact", Name: "first"}, spec.Table{Schema: " Exact Schema ", Name: " later "}
			s := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL, DDL: &spec.DDL{Operation: operation, Table: &first,
				PGDestructiveTable: &spec.PGDestructiveTable{IfExists: tc.guard, Targets: []spec.PGDestructiveTableTarget{{Table: first}, {Table: last, Only: tc.only}, {Table: last, Only: tc.only}}}},
				Metadata: &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, PGDestructiveTableRelations: []spec.PGDestructiveTableRelation{
					{Schema: first.Schema, Relation: spec.PGRelationState{Name: first.Name, Kind: "r"}},
					{Schema: last.Schema, Relation: spec.PGRelationState{Name: last.Name, Kind: tc.kind}},
					{Schema: last.Schema, Relation: spec.PGRelationState{Name: last.Name, Kind: tc.kind}},
				}}}
			s.Metadata.PGDestructiveReferences = &spec.PGDestructiveReferences{Targets: s.DDL.PGDestructiveTable.Targets, IneligibleOnlyPosition: tc.onlyPosition}
			if tc.mutate != nil {
				s.DDL.PGDestructiveTable.Targets = s.DDL.PGDestructiveTable.Targets[:2]
				tc.mutate(&s)
			}
			if reason, missing := pgDestructiveTableTargetResult(s, 1); reason != tc.reason || missing != tc.missing {
				t.Fatalf("reason=%s missing=%s", reason, missing)
			}
			r := tableOperationExistenceRule{label: "table operation"}
			findings, err := r.evaluatePGDestructiveTable(context.Background(), s)
			if err != nil || (len(findings) == 1) != (tc.reason != "") || len(findings) > 1 {
				t.Fatalf("findings=%v error=%v", findings, err)
			}
			if len(findings) == 1 && (findings[0].Metadata["table"] != last.Name || findings[0].Metadata["schema"] != last.Schema) {
				t.Fatal(findings[0].Metadata)
			}
			if gaps := pgDestructiveTableEvidenceGaps(s); (len(gaps) == 1) != (tc.missing != "") {
				t.Fatal(gaps)
			}
			payload, err := json.Marshal(s)
			if err != nil || strings.Contains(string(payload), "PGDestructiveTable") {
				t.Fatalf("private binding appeared in JSON: %s", payload)
			}
		})
	}
}
