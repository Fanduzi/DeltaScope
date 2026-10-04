//go:build postgresql

// Package audit verifies that T05-A4 MODIFY state stays off the PostgreSQL path.
// input: PostgreSQL MODIFY requests in the no-provider, schema-only, and
// provider-backed forms
// output: the legacy per-statement contract, with no ordered-state gap added
// pos: dialect-scope guard for the T05-A4 MODIFY post-state
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestBatchStateA4PostgreSQLModifyKeepsLegacyContract(t *testing.T) {
	t.Parallel()
	policy := t05A4FourRulePolicy(t)
	sql := "ALTER TABLE t ALTER COLUMN c TYPE VARCHAR(20);"

	t.Run("no request metadata", func(t *testing.T) {
		t.Parallel()
		result, err := AuditSQL(context.Background(), Request{SQL: sql, Dialect: spec.DialectPostgreSQL, ConfigPath: policy})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		if len(result.Statements[0].EvidenceGaps) != 0 {
			t.Fatalf("providerless PostgreSQL MODIFY must not gain an ordered-state gap, got %+v", result.Statements[0].EvidenceGaps)
		}
	})
	t.Run("schema only", func(t *testing.T) {
		t.Parallel()
		result, err := AuditSQL(context.Background(), Request{SQL: sql, Dialect: spec.DialectPostgreSQL, Schema: "public", ConfigPath: policy})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		if len(result.Statements[0].EvidenceGaps) != 0 {
			t.Fatalf("schema-only PostgreSQL MODIFY must not gain an ordered-state gap, got %+v", result.Statements[0].EvidenceGaps)
		}
	})
	t.Run("provider snapshot stays on the legacy request cache", func(t *testing.T) {
		t.Parallel()
		const sql = "ALTER TABLE t ALTER COLUMN c TYPE VARCHAR(20); ALTER TABLE t ALTER COLUMN c TYPE VARCHAR(15);"
		snapshot := &spec.TableSnapshot{Exists: true, Table: &spec.Table{Schema: "public", Name: "t"}, Columns: []spec.Column{{Name: "c", Type: "varchar", Length: 10}}}
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
		}
		if len(t05FindingsByRule(result, 1, t05RuleModifyCompat)) != 0 {
			t.Fatalf("PostgreSQL must not consume a conditional MODIFY post-state, got %#v", result.Statements[1].Findings)
		}
		// Legacy enrichment reads each table once per request and reuses that
		// snapshot. A second read, or a derived length of 20, would mean the
		// ordered-state pass had taken the PostgreSQL path.
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
		lengthProvider := &dmlTableMetadataProvider{snapshots: map[string]*spec.TableSnapshot{"public.t": snapshot}}
		enriched, err := enrichStatementsWithMetadata(context.Background(), spec.DialectPostgreSQL, &MetadataRequest{Schema: "public", Provider: lengthProvider}, extracted, parsed.failures)
		if err != nil {
			t.Fatalf("enrich: %v", err)
		}
		if got := t05A4Length(t, enriched[1], "c"); got != 10 {
			t.Fatalf("statement 2 pre-state c.Length = %d, want the provider length 10", got)
		}
	})
}
