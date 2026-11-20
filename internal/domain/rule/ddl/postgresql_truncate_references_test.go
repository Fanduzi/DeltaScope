package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGTruncateReferenceSet(t *testing.T) {
	for _, tc := range []struct {
		name, kind                       string
		position, ineligibleOnly         int
		missing, mismatch, cascade, only bool
		wantPosition                     int
		wantGap                          bool
	}{
		{name: "observed complete set", kind: "r"},
		{name: "outside reference", kind: "r", position: 2, wantPosition: 2},
		{name: "unavailable", kind: "r", missing: true, wantGap: true},
		{name: "misbound set", kind: "r", mismatch: true, wantGap: true},
		{name: "invalid position", kind: "r", position: 3, wantGap: true},
		{name: "known absent target", kind: "", position: 1},
		{name: "observed partition closure", kind: "p", position: 2, wantPosition: 2},
		{name: "partition closure unavailable", kind: "p", missing: true, wantGap: true},
		{name: "first partitioned ONLY is ineligible", kind: "p", only: true, position: 2, ineligibleOnly: 2},
		{name: "partitioned ONLY already covered", kind: "p", only: true, position: 2, wantPosition: 2},
		{name: "partitioned ONLY membership unavailable", kind: "p", only: true, missing: true},
		{name: "explicit cascade", kind: "r", position: 1, cascade: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			targets := []spec.PGDestructiveTableTarget{{Table: spec.Table{Schema: "app", Name: "left"}}, {Table: spec.Table{Schema: " Exact ", Name: " Right "}, Only: tc.only}}
			observation := &spec.PGDestructiveReferences{Targets: append([]spec.PGDestructiveTableTarget(nil), targets...), BlockedPosition: tc.position, IneligibleOnlyPosition: tc.ineligibleOnly}
			if tc.missing {
				observation = nil
			}
			if tc.mismatch {
				observation.Targets[1].Table.Name = "right"
			}
			s := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL,
				DDL: &spec.DDL{Operation: spec.DDLOperationTruncateTable, Table: &targets[0].Table, PGDestructiveTable: &spec.PGDestructiveTable{Targets: targets, Cascade: tc.cascade}},
				Metadata: &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, PGDestructiveReferences: observation,
					PGDestructiveTableRelations: []spec.PGDestructiveTableRelation{{Schema: "app", Relation: spec.PGRelationState{Name: "left", Kind: "r"}}, {Schema: " Exact ", Relation: spec.PGRelationState{Name: " Right ", Kind: tc.kind}}}}}
			position, missing := pgTruncateReferenceResult(s)
			if position != tc.wantPosition || (missing != "") != tc.wantGap {
				t.Fatalf("position=%d missing=%q", position, missing)
			}
			r := truncateInboundFKRule{}
			findings, err := r.Evaluate(context.Background(), s)
			if err != nil || len(findings) > 1 || (len(findings) == 1) != (tc.wantPosition > 0) {
				t.Fatalf("findings=%+v err=%v", findings, err)
			}
			if len(findings) > 0 && (findings[0].Metadata["table"] != " Right " || findings[0].Metadata["reason_code"] != "truncate_referenced_by_external_foreign_key") {
				t.Fatal(findings)
			}
			gaps := r.EvidenceGaps(s)
			if len(gaps) > 1 || (len(gaps) == 1) != tc.wantGap || len(gaps) > 0 && gaps[0].ReasonCode != "unproven_pg_truncate_references" {
				t.Fatal(gaps)
			}
		})
	}
}
