//go:build postgresql

// Package audit verifies that the T05-A3 drop-existence evidence gap stays
// scoped to the MySQL/TiDB ordered-state machine.
// input: PostgreSQL DROP TABLE requests across the no-provider, schema-only,
// and provider-absent request shapes
// output: assertions that PostgreSQL keeps its legacy lifecycle contract —
// provider facts still drive the strict blocker and no new evidence gap is
// emitted under any request shape
// pos: application-layer dialect-scope guard for the T05-A3 drop existence gap
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestBatchStateA3PostgreSQLDropKeepsLegacyContract(t *testing.T) {
	t.Parallel()
	policy := t05PolicyPath(t, map[string]string{
		t05RuleDropExistsRequire: "",
	})
	snapshots := map[string]*spec.TableSnapshot{
		"public.gone": {
			Exists:  false,
			Table:   &spec.Table{Schema: "public", Name: "gone"},
			Columns: []spec.Column{},
		},
	}

	t.Run("no request metadata emits no new gaps", func(t *testing.T) {
		t.Parallel()
		result, err := AuditSQL(context.Background(), Request{
			SQL:        "DROP TABLE t;",
			Dialect:    spec.DialectPostgreSQL,
			ConfigPath: policy,
		})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		if len(result.Statements[0].Findings) != 0 || len(result.Statements[0].EvidenceGaps) != 0 {
			t.Fatalf("providerless PostgreSQL drop must stay silent, got findings=%+v gaps=%+v", result.Statements[0].Findings, result.Statements[0].EvidenceGaps)
		}
	})

	t.Run("schema-only request emits no new gaps", func(t *testing.T) {
		t.Parallel()
		result, err := AuditSQL(context.Background(), Request{
			SQL:        "DROP TABLE IF EXISTS t;",
			Dialect:    spec.DialectPostgreSQL,
			Schema:     "public",
			ConfigPath: policy,
		})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		if len(result.Statements[0].Findings) != 0 || len(result.Statements[0].EvidenceGaps) != 0 {
			t.Fatalf("schema-only PostgreSQL drop must stay silent, got findings=%+v gaps=%+v", result.Statements[0].Findings, result.Statements[0].EvidenceGaps)
		}
	})

	for _, dropSQL := range []string{"DROP TABLE gone;", "DROP TABLE IF EXISTS gone;"} {
		dropSQL := dropSQL
		t.Run("provider-absent strict blocker "+dropSQL, func(t *testing.T) {
			t.Parallel()
			provider := &dmlTableMetadataProvider{snapshots: snapshots}
			result, err := AuditSQL(context.Background(), Request{
				SQL:              dropSQL,
				Dialect:          spec.DialectPostgreSQL,
				Schema:           "public",
				ConfigPath:       policy,
				MetadataProvider: provider,
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			findings := t05FindingsByRule(result, 0, t05RuleDropExistsRequire)
			if len(findings) != 1 || findings[0].Level != rule.LevelBlocker {
				t.Fatalf("provider-confirmed absence must keep the strict blocker, got %+v", result.Statements[0].Findings)
			}
			if len(result.Statements[0].EvidenceGaps) != 0 {
				t.Fatalf("PostgreSQL must not emit the new drop gap, got %+v", result.Statements[0].EvidenceGaps)
			}
			if result.Verdict != report.VerdictReject {
				t.Fatalf("verdict = %s, want reject", result.Verdict)
			}
		})
	}
}
