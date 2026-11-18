package ddl

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestPGReplicaIdentityIndexAssessment(t *testing.T) {
	for _, tc := range []struct {
		name, reason, gap string
		change            func(*spec.Statement, *spec.PGReplicaIdentityIndex)
	}{
		{name: "eligible"},
		{name: "index missing", reason: "index_absent", change: func(_ *spec.Statement, f *spec.PGReplicaIdentityIndex) { f.Kind = "" }},
		{name: "other relation", reason: "relation_not_index", change: func(_ *spec.Statement, f *spec.PGReplicaIdentityIndex) { f.Kind = "v" }},
		{name: "other table", reason: "index_not_on_table", change: func(_ *spec.Statement, f *spec.PGReplicaIdentityIndex) { f.OnTable = false }},
		{name: "nonunique", reason: "index_not_unique", change: func(_ *spec.Statement, f *spec.PGReplicaIdentityIndex) { f.Unique = false }},
		{name: "method cannot enforce uniqueness", reason: "index_not_unique", change: func(_ *spec.Statement, f *spec.PGReplicaIdentityIndex) { f.CanUnique = false }},
		{name: "deferred", reason: "index_not_immediate", change: func(_ *spec.Statement, f *spec.PGReplicaIdentityIndex) { f.Immediate = false }},
		{name: "expression", reason: "index_expression", change: func(_ *spec.Statement, f *spec.PGReplicaIdentityIndex) { f.Expression = true }},
		{name: "partial", reason: "index_partial", change: func(_ *spec.Statement, f *spec.PGReplicaIdentityIndex) { f.Partial = true }},
		{name: "nullable key", reason: "index_key_inapplicable", change: func(_ *spec.Statement, f *spec.PGReplicaIdentityIndex) { f.KeyAttributesValid = false }},
		{name: "unknown keys", gap: "target_table.pg_replica_identity_index", change: func(_ *spec.Statement, f *spec.PGReplicaIdentityIndex) { f.KeyAttributesKnown = false }},
		{name: "unknown AM", gap: "target_table.pg_replica_identity_index", change: func(_ *spec.Statement, f *spec.PGReplicaIdentityIndex) { f.CanUniqueKnown = false }},
		{name: "unknown index", gap: "target_table.pg_replica_identity_index", change: func(_ *spec.Statement, f *spec.PGReplicaIdentityIndex) { f.IndexKnown = false }},
		{name: "misbound", gap: "target_table.pg_replica_identity_index", change: func(_ *spec.Statement, f *spec.PGReplicaIdentityIndex) { f.Name = "EXACT" }},
		{name: "stale", gap: "target_table.pg_replica_identity_index", change: func(s *spec.Statement, _ *spec.PGReplicaIdentityIndex) { s.Metadata.TargetTable.PGColumns = nil }},
		{name: "unqualified", gap: "target_table.pg_replica_identity_index", change: func(s *spec.Statement, _ *spec.PGReplicaIdentityIndex) { s.DDL.Table.Schema = "" }},
		{name: "offline", gap: "instance.version", change: func(s *spec.Statement, _ *spec.PGReplicaIdentityIndex) { s.Metadata = nil }},
		{name: "known absent guarded table", change: func(s *spec.Statement, _ *spec.PGReplicaIdentityIndex) {
			s.DDL.PGTableIfExists = true
			s.Metadata.TargetTable.Exists = false
			s.Metadata.TargetTable.PGRelation.Kind = ""
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			facts := &spec.PGReplicaIdentityIndex{Schema: "app", Table: "t", Name: "exact", Kind: "i", IndexKnown: true, OnTable: true,
				Unique: true, Immediate: true, CanUnique: true, CanUniqueKnown: true, KeyAttributesValid: true, KeyAttributesKnown: true}
			s := spec.Statement{Kind: spec.KindDDL, Dialect: spec.DialectPostgreSQL, DDL: &spec.DDL{
				Operation: spec.DDLOperationAlterTable, Table: &spec.Table{Schema: "app", Name: "t"},
				Alter: []spec.Alter{{Action: "replica_identity", Name: "exact", Options: map[string]string{"identity": "using_index"}}},
			}, Metadata: &spec.Metadata{Schema: "app", Instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, PGReplicaIdentity: facts,
				TargetTable: &spec.TableSnapshot{Schema: "app", Exists: true, Table: &spec.Table{Name: "t"},
					PGRelation: spec.PGRelationState{Name: "t", Kind: "r"}, PGColumns: &spec.PGColumnCatalog{Ordinary: true}}}}
			if tc.change != nil {
				tc.change(&s, facts)
			}
			r := pgReplicaIdentityIndexRule{level: rule.LevelBlocker}
			findings, err := r.Evaluate(context.Background(), s)
			if err != nil || (len(findings) > 0) != (tc.reason != "") {
				t.Fatalf("findings=%+v err=%v", findings, err)
			}
			if len(findings) == 1 && (findings[0].Metadata["reason_code"] != tc.reason || findings[0].Metadata["index"] != "exact" || findings[0].Level != rule.LevelBlocker) {
				t.Fatalf("finding=%+v", findings[0])
			}
			gaps := r.EvidenceGaps(s)
			if (len(gaps) > 0) != (tc.gap != "") || len(gaps) == 1 && (gaps[0].ReasonCode != "unproven_replica_identity_index" || gaps[0].RequiredFacts[0] != tc.gap) {
				t.Fatalf("gaps=%+v", gaps)
			}
			for _, mode := range []string{"default", "full", "nothing"} {
				s.DDL.Alter[0].Options["identity"] = mode
				if r.AppliesTo(s) {
					t.Fatalf("unexpected application to %s", mode)
				}
			}
		})
	}
}
