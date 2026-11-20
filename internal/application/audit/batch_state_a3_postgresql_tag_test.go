//go:build postgresql

package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestBatchStateA3PostgreSQLDropUsesCatalogEvidence(t *testing.T) {
	t.Parallel()
	policy := t05PolicyPath(t, map[string]string{t05RuleDropExistsRequire: ""})
	for _, tc := range []struct {
		name, sql, schema, missingFact string
		provider                       bool
		want                           report.Verdict
	}{
		{name: "offline", sql: "DROP TABLE t", missingFact: "instance.version", want: report.VerdictReview},
		{name: "schema only", sql: "DROP TABLE IF EXISTS t", schema: "public", missingFact: "instance.version", want: report.VerdictReview},
		{name: "provider and unqualified", sql: "DROP TABLE gone", schema: "public", provider: true, missingFact: "target_table.pg_identity", want: report.VerdictReview},
		{name: "qualified absence", sql: "DROP TABLE public.gone", provider: true, want: report.VerdictReject},
		{name: "guarded qualified absence", sql: "DROP TABLE IF EXISTS public.gone", provider: true, want: report.VerdictPass},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := Request{SQL: tc.sql, Dialect: spec.DialectPostgreSQL, Schema: tc.schema, ConfigPath: policy}
			if tc.provider {
				r.MetadataProvider = &fakeMetadataProvider{instance: &spec.InstanceFacts{Version: "PostgreSQL 17.7"}, snapshot: &spec.TableSnapshot{
					Schema: "public", PGRelation: spec.PGRelationState{Name: "gone"},
				}}
			}
			result, err := AuditSQL(context.Background(), r)
			if err != nil || result.Verdict != tc.want || len(result.Statements) != 1 {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			findings := t05FindingsByRule(result, 0, t05RuleDropExistsRequire)
			if tc.want == report.VerdictReject {
				if len(findings) != 1 || findings[0].Level != rule.LevelBlocker || findings[0].Metadata["reason_code"] != "relation_absent" {
					t.Fatalf("missing exact absence blocker: %+v", findings)
				}
			} else if len(findings) != 0 {
				t.Fatalf("unexpected findings: %+v", findings)
			}
			gaps := result.Statements[0].EvidenceGaps
			if tc.missingFact == "" {
				if len(gaps) != 0 {
					t.Fatal(gaps)
				}
			} else if len(gaps) != 1 || gaps[0].RuleID != t05RuleDropExistsRequire || gaps[0].ReasonCode != "unproven_pg_destructive_table_target" ||
				len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != tc.missingFact {
				t.Fatalf("unexpected evidence gaps: %+v", gaps)
			}
		})
	}
}
