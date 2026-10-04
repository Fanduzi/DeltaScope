//go:build postgresql

// Package audit verifies that T05-A5 column identity stays off the PostgreSQL path.
// input: PostgreSQL ALTER COLUMN requests in the no-provider, schema-only, and
// provider-backed forms under the eleven-rule identity policy
// output: the legacy per-statement contract, with no ordered-state gap added
// pos: dialect-scope guard for the T05-A5 CHANGE and RENAME COLUMN post-state
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestBatchStateA5PostgreSQLIdentityKeepsLegacyContract(t *testing.T) {
	t.Parallel()
	policy := t05A5IdentityPolicy(t)
	sql := "ALTER TABLE t ALTER COLUMN c TYPE VARCHAR(20);"

	t.Run("no request metadata", func(t *testing.T) {
		t.Parallel()
		result, err := AuditSQL(context.Background(), Request{SQL: sql, Dialect: spec.DialectPostgreSQL, ConfigPath: policy})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		if len(result.Statements[0].EvidenceGaps) != 0 {
			t.Fatalf("providerless PostgreSQL ALTER must not gain an ordered-state gap, got %+v", result.Statements[0].EvidenceGaps)
		}
	})
	t.Run("schema only", func(t *testing.T) {
		t.Parallel()
		result, err := AuditSQL(context.Background(), Request{SQL: sql, Dialect: spec.DialectPostgreSQL, Schema: "public", ConfigPath: policy})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		if len(result.Statements[0].EvidenceGaps) != 0 {
			t.Fatalf("schema-only PostgreSQL ALTER must not gain an ordered-state gap, got %+v", result.Statements[0].EvidenceGaps)
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
			t.Fatalf("PostgreSQL must not consume a conditional column post-state, got %#v", result.Statements[1].Findings)
		}
		if len(provider.snapshotCalls) != 1 || provider.snapshotCalls[0] != "public.t" {
			t.Fatalf("PostgreSQL request cache ledger = %#v, want [public.t]", provider.snapshotCalls)
		}
	})
}
