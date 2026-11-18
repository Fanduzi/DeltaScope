//go:build postgresql

// Package audit verifies PG keeps its legacy cache, not MySQL ordered state,
// while the approved D6 existence check explicitly owes exact PG proof.
package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestBatchStateA6PostgreSQLDropColumnKeepsCacheAndOwesPGProof(t *testing.T) {
	t.Parallel()
	policy := t05A6SixRulePolicy(t)
	sql := "ALTER TABLE t DROP COLUMN obsolete;"

	t.Run("no request metadata", func(t *testing.T) {
		t.Parallel()
		result, err := AuditSQL(context.Background(), Request{SQL: sql, Dialect: spec.DialectPostgreSQL, ConfigPath: policy})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		gaps := result.Statements[0].EvidenceGaps
		if len(gaps) != 1 || gaps[0].RuleID != t05RuleDropColumnExists || gaps[0].ReasonCode != "unproven_column_target" || len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != "instance.version" {
			t.Fatalf("providerless PG DROP owes observed PG proof, not guessed ordered state: %+v", gaps)
		}
	})
	t.Run("schema only", func(t *testing.T) {
		t.Parallel()
		result, err := AuditSQL(context.Background(), Request{SQL: sql, Dialect: spec.DialectPostgreSQL, Schema: "public", ConfigPath: policy})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		gaps := result.Statements[0].EvidenceGaps
		if len(gaps) != 1 || gaps[0].RuleID != t05RuleDropColumnExists || gaps[0].ReasonCode != "unproven_column_target" || len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != "instance.version" {
			t.Fatalf("schema-only PG DROP owes observed PG proof: %+v", gaps)
		}
	})
	t.Run("provider snapshot stays on the legacy request cache", func(t *testing.T) {
		t.Parallel()
		const sql = "ALTER TABLE t DROP COLUMN obsolete; ALTER TABLE t DROP COLUMN obsolete;"
		snapshot := &spec.TableSnapshot{Options: map[string]string{"cache_status": "disable"}, Exists: true,
			Table: &spec.Table{Schema: "public", Name: "t"},
			Columns: []spec.Column{
				{Name: "id"},
				{Name: "obsolete"},
				{Name: "keep_c", Type: "varchar", Length: 10},
			},
		}
		provider := &dmlTableMetadataProvider{snapshots: map[string]*spec.TableSnapshot{"public.t": snapshot}}
		result, err := AuditSQL(context.Background(), Request{
			SQL:              sql,
			Dialect:          spec.DialectPostgreSQL,
			Schema:           "public",
			ConfigPath:       policy,
			MetadataProvider: provider,
		})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		if len(result.Statements) != 2 {
			t.Fatalf("expected 2 statements, got %d", len(result.Statements))
		}
		for index, statement := range result.Statements {
			gaps := statement.EvidenceGaps
			if len(gaps) != 1 || gaps[0].RuleID != t05RuleDropColumnExists || gaps[0].ReasonCode != "unproven_column_target" || len(gaps[0].RequiredFacts) != 1 || gaps[0].RequiredFacts[0] != "instance.version" {
				t.Fatalf("statement %d cannot promote legacy cache to exact PG proof: %+v", index, gaps)
			}
			if len(t05FindingsByRule(result, index, t05RuleDropColumnExists)) != 0 {
				t.Fatalf("statement %d reported obsolete absent, got %#v", index, statement.Findings)
			}
		}
		if len(provider.snapshotCalls) != 1 || provider.snapshotCalls[0] != "public.t" {
			t.Fatalf("PostgreSQL request cache ledger = %#v, want [public.t]", provider.snapshotCalls)
		}
		parsed, parseErr := parseSQL(context.Background(), sql, spec.DialectPostgreSQL)
		if parseErr != nil || len(parsed.Statements) != 2 {
			t.Fatalf("parse: err=%v statements=%d", parseErr, len(parsed.Statements))
		}
		extracted, err := Extract(context.Background(), parsed)
		if err != nil {
			t.Fatalf("extract: %v", err)
		}
		legacy := &dmlTableMetadataProvider{snapshots: map[string]*spec.TableSnapshot{"public.t": snapshot}}
		enriched, err := enrichStatementsWithMetadata(context.Background(), spec.DialectPostgreSQL, &MetadataRequest{Schema: "public", Provider: legacy}, extracted, parsed.failures)
		if err != nil {
			t.Fatalf("enrich: %v", err)
		}
		for index, statement := range enriched {
			table := statement.Metadata
			if table == nil || table.TargetTable == nil || table.TargetTable.FindColumn("obsolete") == nil {
				t.Fatalf("statement %d lost obsolete from the legacy snapshot: %+v", index, table)
			}
		}
		if len(legacy.snapshotCalls) != 1 || legacy.snapshotCalls[0] != "public.t" {
			t.Fatalf("legacy enrichment ledger = %#v, want one public.t read", legacy.snapshotCalls)
		}
	})
}
