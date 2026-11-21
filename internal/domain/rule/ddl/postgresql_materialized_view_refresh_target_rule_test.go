package ddl

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGRefreshMaterializedViewTargetPremises(t *testing.T) {
	r := pgRefreshMaterializedViewTargetRule{}
	for _, tc := range []struct {
		name, kind, reason, gap                string
		populated, index, withNoData, ordinary bool
		mutate                                 func(*spec.Statement)
	}{
		{name: "missing", reason: "refresh_materialized_view_absent"},
		{name: "wrong kind", kind: "r", reason: "refresh_target_wrong_kind"},
		{name: "unpopulated comes first", kind: "m", index: true, withNoData: true, reason: "refresh_concurrent_unpopulated"},
		{name: "conflicting flags precede index", kind: "m", populated: true, withNoData: true, reason: "refresh_concurrent_no_data"},
		{name: "missing usable index", kind: "m", populated: true, reason: "refresh_concurrent_unique_index_absent"},
		{name: "eligible concurrent", kind: "m", populated: true, index: true},
		{name: "nonconcurrent skips concurrent prerequisites", kind: "m", ordinary: true, withNoData: true},
		{name: "unqualified", gap: "materialized_view.target_identity", mutate: func(s *spec.Statement) { s.DDL.PGRefreshTarget.Schema = "" }},
		{name: "misbound", kind: "m", gap: "materialized_view.pg_refresh_state", mutate: func(s *spec.Statement) { s.Metadata.PGRefreshState.Target.Name = "another" }},
		{name: "missing state", gap: "materialized_view.pg_refresh_state", mutate: func(s *spec.Statement) { s.Metadata.PGRefreshState = nil }},
		{name: "version unknown", gap: "instance.version", mutate: func(s *spec.Statement) { s.Metadata.Instance = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := spec.PGRefreshTarget{Schema: " Exact Schema ", Name: " Exact MV "}
			s := spec.Statement{Dialect: spec.DialectPostgreSQL, Kind: spec.KindDDL, DDL: &spec.DDL{
				Operation: spec.DDLOperationRefreshMaterializedView, ObjectName: target.Name, PGRefreshTarget: target,
				Options: map[string]string{"concurrently": "true", "with_no_data": "false"}},
				Metadata: &spec.Metadata{Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, PGRefreshState: &spec.PGRefreshState{Target: target, Kind: tc.kind, Populated: tc.populated, UniqueIndex: tc.index}}}
			if tc.ordinary {
				s.DDL.Options["concurrently"] = "false"
			}
			if tc.withNoData {
				s.DDL.Options["with_no_data"] = "true"
			}
			if tc.mutate != nil {
				tc.mutate(&s)
			}
			if reason, fact := assessPGRefreshMaterializedViewTarget(s); reason != tc.reason || fact != tc.gap {
				t.Fatalf("reason=%s fact=%s", reason, fact)
			}
			findings, err := r.Evaluate(context.Background(), s)
			if err != nil || (len(findings) == 1) != (tc.reason != "") {
				t.Fatalf("findings=%v error=%v", findings, err)
			}
			if gaps := r.EvidenceGaps(s); (len(gaps) == 1) != (tc.gap != "") {
				t.Fatal(gaps)
			}
			body, err := json.Marshal(s)
			if err != nil || strings.Contains(string(body), "PGRefreshState") || strings.Contains(string(body), "Exact Schema") {
				t.Fatalf("private metadata output: %s", body)
			}
			advice, err := newRefreshMaterializedViewNoDataNoticeRule(policy.RulePolicy{})
			if err != nil {
				t.Fatal(err)
			}
			notices, err := advice.Evaluate(context.Background(), s)
			if err != nil || (len(notices) == 1) != (tc.ordinary && tc.withNoData) {
				t.Fatalf("notices=%v error=%v", notices, err)
			}
		})
	}
}
