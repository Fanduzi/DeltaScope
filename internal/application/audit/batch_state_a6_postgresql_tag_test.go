//go:build postgresql

// Package audit verifies that T05-A6 DROP COLUMN state stays off the PostgreSQL path.
// input: PostgreSQL DROP COLUMN requests in the no-provider, schema-only, and
// provider-backed forms under the six-rule profile
// output: the legacy per-statement snapshot, with obsolete still present on the
// second statement and no ordered-state gap
// pos: dialect-scope guard for the T05-A6 DROP COLUMN post-state
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestBatchStateA6PostgreSQLDropColumnKeepsLegacyContract(t *testing.T) {
	t.Parallel()
	policy := t05A6SixRulePolicy(t)
	sql := "ALTER TABLE t DROP COLUMN obsolete;"

	t.Run("no request metadata", func(t *testing.T) {
		t.Parallel()
		result, err := AuditSQL(context.Background(), Request{SQL: sql, Dialect: spec.DialectPostgreSQL, ConfigPath: policy})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		if len(result.Statements[0].EvidenceGaps) != 0 {
			t.Fatalf("providerless PostgreSQL DROP COLUMN must not gain an ordered-state gap, got %+v", result.Statements[0].EvidenceGaps)
		}
	})
	t.Run("schema only", func(t *testing.T) {
		t.Parallel()
		result, err := AuditSQL(context.Background(), Request{SQL: sql, Dialect: spec.DialectPostgreSQL, Schema: "public", ConfigPath: policy})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		if len(result.Statements[0].EvidenceGaps) != 0 {
			t.Fatalf("schema-only PostgreSQL DROP COLUMN must not gain an ordered-state gap, got %+v", result.Statements[0].EvidenceGaps)
		}
	})
	t.Run("provider snapshot stays on the legacy request cache", func(t *testing.T) {
		t.Parallel()
		const sql = "ALTER TABLE t DROP COLUMN obsolete; ALTER TABLE t DROP COLUMN obsolete;"
		snapshot := &spec.TableSnapshot{
			Exists: true,
			Table:  &spec.Table{Schema: "public", Name: "t"},
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
			if len(statement.EvidenceGaps) != 0 {
				t.Fatalf("statement %d gained an ordered-state gap, got %+v", index, statement.EvidenceGaps)
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
