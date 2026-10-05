//go:build postgresql

// Package audit verifies PostgreSQL never enters the T05-A7 ordered-state statement budget.
// input: 1025-statement PostgreSQL CREATE/ALTER batches under the bare, schema-only, provider, and legacy Metadata request shapes
// output: identical results between the production default budget and a zero quota, complete coverage, and untouched provider snapshots
// pos: application-layer PostgreSQL bypass contract for the ordered-state admission seam (issue #84/T05-A7)
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

var t05A7PostgreSQLBatch = "CREATE TABLE t (id INT PRIMARY KEY);\n" + strings.Repeat("ALTER TABLE t ADD COLUMN c INT;\n", 1024)

func t05A7PostgreSQLPolicy(t *testing.T) string {
	return t05A7Policy(t, map[string]string{
		t05RuleCreateForbid: "",
		t05RuleAlterRequire: "",
	})
}

func t05A7AssertPostgreSQLResult(t *testing.T, result report.Result, wantVerdict report.Verdict, wantCreateBlocker bool) {
	t.Helper()
	if len(result.Statements) != 1025 {
		t.Fatalf("statements = %d, want 1025 retained", len(result.Statements))
	}
	if result.Verdict != wantVerdict {
		t.Fatalf("verdict = %s, want %s", result.Verdict, wantVerdict)
	}
	if result.Coverage.Status != report.CoverageComplete {
		t.Fatalf("aggregate coverage = %s, want complete", result.Coverage.Status)
	}
	for i, statement := range result.Statements {
		if statement.Coverage.Status != report.CoverageComplete {
			t.Fatalf("statement %d coverage = %s, want complete", i, statement.Coverage.Status)
		}
		if len(statement.EvidenceGaps) != 0 {
			t.Fatalf("statement %d evidence gaps = %+v, want none", i, statement.EvidenceGaps)
		}
	}
	for _, item := range result.Unsupported {
		if item.Feature == "audit.resource_limit" {
			t.Fatalf("PostgreSQL must never project a resource-limit entry: %+v", item)
		}
	}
	if wantCreateBlocker {
		findings := t05FindingsByRule(result, 0, t05RuleCreateForbid)
		if len(findings) != 1 || findings[0].Level != rule.LevelBlocker {
			t.Fatalf("statement 0 create-forbid findings = %+v, want one blocker", findings)
		}
		for i := 1; i < len(result.Statements); i++ {
			if len(result.Statements[i].Findings) != 0 {
				t.Fatalf("statement %d findings = %+v, want none", i, result.Statements[i].Findings)
			}
		}
		return
	}
	for i := range result.Statements {
		if len(result.Statements[i].Findings) != 0 {
			t.Fatalf("statement %d findings = %+v, want none", i, result.Statements[i].Findings)
		}
	}
}

func TestT05A7PostgreSQLNeverEntersBudget(t *testing.T) {
	policy := t05A7PostgreSQLPolicy(t)

	t.Run("no_metadata", func(t *testing.T) {
		request := Request{SQL: t05A7PostgreSQLBatch, Dialect: spec.DialectPostgreSQL, ConfigPath: policy}
		result, err := AuditSQL(context.Background(), request)
		if err != nil {
			t.Fatalf("default-budget audit: %v", err)
		}
		t05A7AssertPostgreSQLResult(t, result, report.VerdictPass, false)

		zero, err := auditWithLimits(context.Background(), request, auditLimits{orderedStatements: 0})
		if err != nil {
			t.Fatalf("quota-0 audit: %v", err)
		}
		t05A7AssertPostgreSQLResult(t, zero, report.VerdictPass, false)
		if !reflect.DeepEqual(result, zero) {
			t.Fatal("quota 0 must produce byte-identical behavior to the default budget for PostgreSQL")
		}
	})

	t.Run("schema_only", func(t *testing.T) {
		request := Request{SQL: t05A7PostgreSQLBatch, Dialect: spec.DialectPostgreSQL, ConfigPath: policy, Schema: "public"}
		result, err := AuditSQL(context.Background(), request)
		if err != nil {
			t.Fatalf("default-budget audit: %v", err)
		}
		t05A7AssertPostgreSQLResult(t, result, report.VerdictPass, false)

		zero, err := auditWithLimits(context.Background(), request, auditLimits{orderedStatements: 0})
		if err != nil {
			t.Fatalf("quota-0 audit: %v", err)
		}
		t05A7AssertPostgreSQLResult(t, zero, report.VerdictPass, false)
		if !reflect.DeepEqual(result, zero) {
			t.Fatal("quota 0 must produce identical results for the schema-only PostgreSQL request")
		}
	})

	t.Run("provider", func(t *testing.T) {
		snapshot := presentTable("public", "t", "id")
		snapshotBefore := cloneTableSnapshot(snapshot)
		provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"public.t": snapshot}}
		request := Request{
			SQL:              t05A7PostgreSQLBatch,
			Dialect:          spec.DialectPostgreSQL,
			ConfigPath:       policy,
			Schema:           "public",
			MetadataProvider: provider,
		}
		result, err := AuditSQL(context.Background(), request)
		if err != nil {
			t.Fatalf("default-budget audit: %v", err)
		}
		t05A7AssertPostgreSQLResult(t, result, report.VerdictReject, true)

		zero, err := auditWithLimits(context.Background(), request, auditLimits{orderedStatements: 0})
		if err != nil {
			t.Fatalf("quota-0 audit: %v", err)
		}
		t05A7AssertPostgreSQLResult(t, zero, report.VerdictReject, true)
		if !reflect.DeepEqual(result, zero) {
			t.Fatal("quota 0 must produce identical results for the provider PostgreSQL request")
		}
		if !reflect.DeepEqual(snapshot, snapshotBefore) {
			t.Fatalf("provider snapshot mutated (derived column c must not leak):\n got %+v\nwant %+v", snapshot, snapshotBefore)
		}
	})

	t.Run("legacy_metadata_field", func(t *testing.T) {
		snapshot := presentTable("public", "t", "id")
		snapshotBefore := cloneTableSnapshot(snapshot)
		request := Request{
			SQL:        t05A7PostgreSQLBatch,
			Dialect:    spec.DialectPostgreSQL,
			ConfigPath: policy,
			Metadata: &MetadataRequest{
				Schema:   "public",
				Provider: &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"public.t": snapshot}},
			},
		}
		result, err := AuditSQL(context.Background(), request)
		if err != nil {
			t.Fatalf("default-budget audit: %v", err)
		}
		t05A7AssertPostgreSQLResult(t, result, report.VerdictReject, true)

		zero, err := auditWithLimits(context.Background(), request, auditLimits{orderedStatements: 0})
		if err != nil {
			t.Fatalf("quota-0 audit: %v", err)
		}
		if !reflect.DeepEqual(result, zero) {
			t.Fatal("quota 0 must produce identical results for the legacy Metadata request shape")
		}
		if !reflect.DeepEqual(snapshot, snapshotBefore) {
			t.Fatalf("provider snapshot mutated:\n got %+v\nwant %+v", snapshot, snapshotBefore)
		}
	})
}

func TestT05A7PostgreSQLEnrichmentIgnoresQuota(t *testing.T) {
	extracted := t05A7Extract(t, "CREATE TABLE t (id INT PRIMARY KEY);\nALTER TABLE t ADD COLUMN c INT;", spec.DialectPostgreSQL)

	requests := map[string]*MetadataRequest{
		"nil":      nil,
		"schema":   {Schema: "public"},
		"provider": {Schema: "public", Provider: &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"public.t": presentTable("public", "t", "id")}}},
	}
	for name, request := range requests {
		t.Run(name, func(t *testing.T) {
			enriched, err := enrichStatementsWithMetadataLimits(context.Background(), spec.DialectPostgreSQL, request, extracted, nil, auditLimits{orderedStatements: 0})
			if err != nil {
				t.Fatalf("enrich: %v", err)
			}
			for i, statement := range enriched {
				if statement.ResourceLimit != nil {
					t.Fatalf("statement %d must never carry the ordered-state marker under PostgreSQL", i)
				}
			}
			if request == nil {
				if enriched[1].Metadata != nil {
					t.Fatalf("bare request must not attach metadata, got %+v", enriched[1].Metadata)
				}
				return
			}
			if enriched[1].Metadata == nil || enriched[1].Metadata.Schema != "public" {
				t.Fatalf("statement 1 metadata schema = %+v, want public", enriched[1].Metadata)
			}
			if name == "provider" {
				target := enriched[1].Metadata.TargetTable
				if target == nil || !target.Exists {
					t.Fatalf("provider shape must attach the real table snapshot, got %+v", target)
				}
				names := make([]string, 0, len(target.Columns))
				for _, column := range target.Columns {
					names = append(names, column.Name)
				}
				if !reflect.DeepEqual(names, []string{"id"}) {
					t.Fatalf("PostgreSQL enrichment must not derive column c, got %v", names)
				}
			}
		})
	}
}
