package ddl

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGTableOwnerTargetAssessment(t *testing.T) {
	for _, tc := range []struct {
		name, kind, reason, gap string
		present, same           bool
		change                  func(*spec.Statement)
	}{
		{name: "literal exists", kind: "named", present: true},
		{name: "observed same owner", kind: "current_role", present: true, same: true},
		{name: "missing literal", kind: "named", reason: "owner_role_absent"},
		{name: "public", kind: "public", reason: "owner_public_inapplicable"},
		{name: "partitioned public", kind: "public", reason: "owner_public_inapplicable", change: func(s *spec.Statement) { s.Metadata.TargetTable.PGRelation.Kind = "p" }},
		{name: "partitioned missing role", kind: "named", reason: "owner_role_absent", change: func(s *spec.Statement) { s.Metadata.TargetTable.PGRelation.Kind = "p" }},
		{name: "partitioned known owner keeps gap", kind: "named", present: true, same: true, gap: "target_table.pg_owner", change: func(s *spec.Statement) { s.Metadata.TargetTable.PGRelation.Kind = "p" }},
		{name: "unobserved session", kind: "session_user", gap: "target_table.pg_owner"},
		{name: "offline", kind: "named", gap: "instance.version", change: func(s *spec.Statement) { s.Metadata = nil }},
		{name: "stale relation", kind: "named", gap: "target_table.pg_owner", change: func(s *spec.Statement) { s.Metadata.TargetTable.PGRelation = spec.PGRelationState{} }},
		{name: "mismatched role", kind: "named", gap: "target_table.pg_owner", change: func(s *spec.Statement) { s.Metadata.PGTableOwner.Target.Name = "private_owner" }},
		{name: "public unobserved table scope", kind: "public", gap: "target_table.pg_owner", change: func(s *spec.Statement) { s.Metadata.PGTableOwner = nil }},
		{name: "public on guarded absence", kind: "public", change: func(s *spec.Statement) {
			s.DDL.PGTableIfExists = true
			s.Metadata.TargetTable.Exists, s.Metadata.TargetTable.PGRelation.Kind = false, ""
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := spec.PGTableOwnerTarget{Kind: tc.kind}
			if tc.kind == "named" {
				target.Name = " Private Owner "
			}
			s := spec.Statement{Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL, DDL: &spec.DDL{
				Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Schema: "app", Name: "t"},
				Alter: []spec.Alter{{Action: "change_owner", PGTableOwner: target}},
			}, Metadata: &spec.Metadata{Schema: "app", Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"},
				PGTableOwner: &spec.PGTableOwnerState{Schema: "app", Table: "t", Target: target, Exists: tc.present, Same: tc.same},
				TargetTable: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
					PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
			if tc.change != nil {
				tc.change(&s)
			}
			r := pgTableOwnerRule{level: rule.LevelBlocker}
			findings, err := r.Evaluate(context.Background(), s)
			if err != nil || len(findings) > 1 || (len(findings) == 1) != (tc.reason != "") {
				t.Fatalf("findings=%+v err=%v", findings, err)
			}
			if len(findings) == 1 && (findings[0].Metadata["reason_code"] != tc.reason || findings[0].Level != rule.LevelBlocker) {
				t.Fatalf("finding=%+v", findings[0])
			}
			gaps := r.EvidenceGaps(s)
			if (len(gaps) == 1) != (tc.gap != "") || len(gaps) == 1 && (gaps[0].ReasonCode != "unproven_table_owner_target" || gaps[0].RequiredFacts[0] != tc.gap) {
				t.Fatalf("gaps=%+v", gaps)
			}
			s.DDL.Alter[0].Name = target.Name // Old normalized inputs must not leak a role either.
			advice, err := mustNewOwnerAdvisoryRule(t, policy.RulePolicy{}).Evaluate(context.Background(), s)
			wantSameAdvice := tc.same && tc.gap == ""
			if err != nil || len(advice) != 1 || advice[0].Level != rule.LevelNotice || strings.Contains(advice[0].Message, "observed owner") != wantSameAdvice {
				t.Fatalf("advice=%+v err=%v", advice, err)
			}
			b, err := json.Marshal(append(findings, advice...))
			if err != nil || strings.Contains(string(b), "Private Owner") {
				t.Fatalf("private owner leaked: %s err=%v", b, err)
			}
			s.Dialect = spec.DialectMySQL
			if r.AppliesTo(s) {
				t.Fatal("PostgreSQL rule applied to MySQL")
			}
		})
	}
}
