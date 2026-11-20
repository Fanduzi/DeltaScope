package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGSetWithoutOIDsTarget(t *testing.T) {
	for _, tc := range []struct {
		name, kind, reason, missing string
		guard, unqualified, stale   bool
	}{
		{name: "table", kind: "r"},
		{name: "partitioned root", kind: "p"},
		{name: "foreign table", kind: "f"},
		{name: "view", kind: "v", reason: "relation_wrong_kind"},
		{name: "guarded view", kind: "v", guard: true, reason: "relation_wrong_kind"},
		{name: "sequence", kind: "S", reason: "relation_wrong_kind"},
		{name: "absent", reason: "relation_absent"},
		{name: "guarded absence", guard: true},
		{name: "unqualified", kind: "r", unqualified: true, missing: "target_table.pg_relation"},
		{name: "stale", kind: "v", stale: true, missing: "target_table.pg_relation"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := pgGenerationTransitionStatement("set_without_oids", nil)
			s.DDL.PGTableIfExists = tc.guard
			s.Metadata.TargetTable.Exists = tc.kind != "" && tc.kind != "S"
			s.Metadata.TargetTable.PGRelation.Kind = tc.kind
			if tc.unqualified {
				s.DDL.Table.Schema = ""
			}
			if tc.stale {
				s.Metadata.TargetTable.PGRelation = spec.PGRelationState{}
			}
			r := tableExistenceRule{requireExist: true}
			findings, err := r.Evaluate(context.Background(), s)
			if err != nil || len(findings) > 1 || (len(findings) == 1) != (tc.reason != "") || len(findings) == 1 && findings[0].Metadata["reason_code"] != tc.reason {
				t.Fatalf("findings=%v err=%v", findings, err)
			}
			gaps := r.EvidenceGaps(s)
			if len(gaps) > 1 || (len(gaps) == 1) != (tc.missing != "") || len(gaps) == 1 && (len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != tc.missing) {
				t.Fatalf("gaps=%v", gaps)
			}
		})
	}
}
