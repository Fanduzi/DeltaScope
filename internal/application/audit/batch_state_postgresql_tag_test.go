//go:build postgresql

// Package audit verifies that the T05 ordered schema-state machine is scoped
// to MySQL/TiDB only.
// input: PostgreSQL audit requests across every request/provider shape
// output: assertions that no request shape enters the ordered derivation
// path or emits this slice's new evidence gaps — PostgreSQL keeps its
// pre-T05 enrichment and result contract unchanged
// pos: application-layer dialect-scope guard for issue #84/T05-A1-R1/R2
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// C4: PostgreSQL with a request schema and no provider must not enter the
// ordered-state pass — the pre-T05 loop attaches schema/version metadata but
// never derives table snapshots.
func TestBatchStatePostgreSQLSchemaWithoutProviderStaysOnLegacyPath(t *testing.T) {
	t.Parallel()
	parsed, parseErr := parseSQL(context.Background(),
		"CREATE TABLE t (id INT PRIMARY KEY); ALTER TABLE t ADD COLUMN c INT;", spec.DialectPostgreSQL)
	if parseErr != nil || len(parsed.Statements) != 2 {
		t.Fatalf("parse: err=%v statements=%d", parseErr, len(parsed.Statements))
	}
	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	enriched, err := enrichStatementsWithMetadata(context.Background(), spec.DialectPostgreSQL,
		&MetadataRequest{Schema: "public"}, statements, parsed.failures)
	if err != nil {
		t.Fatalf("enrich: %v", err)
	}
	for i, statement := range enriched {
		if statement.Metadata != nil && statement.Metadata.TargetTable != nil {
			t.Fatalf("PostgreSQL statement %d must not carry a derived target snapshot, got %+v", i, statement.Metadata.TargetTable)
		}
	}
	if enriched[1].Metadata == nil || enriched[1].Metadata.Schema != "public" {
		t.Fatalf("PostgreSQL metadata schema must stay attached, got %+v", enriched[1].Metadata)
	}
}

// C4: PostgreSQL with provider also keeps its per-statement enrichment —
// the ordered pass is never entered regardless of request shape.
func TestBatchStatePostgreSQLProviderStaysOnLegacyPath(t *testing.T) {
	t.Parallel()
	parsed, parseErr := parseSQL(context.Background(),
		"CREATE TABLE t (id INT PRIMARY KEY); ALTER TABLE t ADD COLUMN c INT;", spec.DialectPostgreSQL)
	if parseErr != nil || len(parsed.Statements) != 2 {
		t.Fatalf("parse: err=%v statements=%d", parseErr, len(parsed.Statements))
	}
	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	provider := &dmlTableMetadataProvider{snapshots: map[string]*spec.TableSnapshot{
		"public.t": {
			Exists:  true,
			Table:   &spec.Table{Schema: "public", Name: "t"},
			Columns: []spec.Column{{Name: "id", Type: "int"}},
		},
	}}
	enriched, err := enrichStatementsWithMetadata(context.Background(), spec.DialectPostgreSQL,
		&MetadataRequest{Schema: "public", Provider: provider}, statements, parsed.failures)
	if err != nil {
		t.Fatalf("enrich: %v", err)
	}
	// The legacy path attaches the provider snapshot to each statement
	// independently — no in-batch derivation, no cross-statement state.
	if enriched[1].Metadata == nil || enriched[1].Metadata.TargetTable == nil {
		t.Fatalf("PostgreSQL provider path still resolves table metadata, got %+v", enriched[1].Metadata)
	}
	if got := len(enriched[1].Metadata.TargetTable.Columns); got != 1 {
		t.Fatalf("PostgreSQL must read the provider snapshot verbatim (no derived c column), got %d columns", got)
	}
}

// C4/R2: PostgreSQL keeps its full pre-T05 result contract — the new
// unknown_table_state gap projection is MySQL/TiDB-only. Every request
// shape runs the real AuditSQL path so findings, gaps, coverage, and the
// verdict are pinned together, not just the enrichment seam.
func TestBatchStatePostgreSQLResultContractKeepsLegacyShape(t *testing.T) {
	t.Parallel()

	existencePolicy := t05PolicyPath(t, map[string]string{
		"ddl.table.exists.create.forbid": "",
		"ddl.table.exists.alter.require": "",
	})
	provider := &dmlTableMetadataProvider{snapshots: map[string]*spec.TableSnapshot{
		"public.t": {
			Exists:  true,
			Table:   &spec.Table{Schema: "public", Name: "t"},
			Columns: []spec.Column{{Name: "id", Type: "int"}},
		},
		"public.gone": {
			Exists:  false,
			Table:   &spec.Table{Schema: "public", Name: "gone"},
			Columns: []spec.Column{},
		},
	}}

	t.Run("no request metadata emits no new gaps", func(t *testing.T) {
		t.Parallel()
		result, err := AuditSQL(context.Background(), Request{
			SQL:        "CREATE TABLE t (id INT PRIMARY KEY); ALTER TABLE t ADD COLUMN c INT;",
			Dialect:    spec.DialectPostgreSQL,
			ConfigPath: existencePolicy,
		})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		for i := range result.Statements {
			if len(result.Statements[i].EvidenceGaps) != 0 {
				t.Fatalf("PostgreSQL statement %d must not report ordered-state gaps, got %+v", i, result.Statements[i].EvidenceGaps)
			}
			if len(result.Statements[i].Findings) != 0 {
				t.Fatalf("PostgreSQL statement %d must stay clean without a provider, got %+v", i, result.Statements[i].Findings)
			}
		}
		if result.Verdict != report.VerdictPass {
			t.Fatalf("verdict must stay pass, got %s", result.Verdict)
		}
	})

	t.Run("schema-only request emits no new gaps", func(t *testing.T) {
		t.Parallel()
		result, err := AuditSQL(context.Background(), Request{
			SQL:        "CREATE TABLE t (id INT PRIMARY KEY); ALTER TABLE t ADD COLUMN c INT;",
			Dialect:    spec.DialectPostgreSQL,
			Schema:     "public",
			ConfigPath: existencePolicy,
		})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		for i := range result.Statements {
			if len(result.Statements[i].EvidenceGaps) != 0 {
				t.Fatalf("PostgreSQL schema-only statement %d must not report gaps, got %+v", i, result.Statements[i].EvidenceGaps)
			}
		}
		if result.Verdict != report.VerdictPass {
			t.Fatalf("verdict must stay pass, got %s", result.Verdict)
		}
	})

	t.Run("provider-backed findings stay intact", func(t *testing.T) {
		t.Parallel()
		result, err := AuditSQL(context.Background(), Request{
			SQL:              "CREATE TABLE t (id INT PRIMARY KEY); ALTER TABLE gone ADD COLUMN c INT;",
			Dialect:          spec.DialectPostgreSQL,
			Schema:           "public",
			ConfigPath:       existencePolicy,
			MetadataProvider: provider,
		})
		if err != nil {
			t.Fatalf("audit: %v", err)
		}
		// The legacy real findings still fire against provider facts.
		create := t05FindingsByRule(result, 0, "ddl.table.exists.create.forbid")
		if len(create) != 1 || create[0].Level != rule.LevelBlocker {
			t.Fatalf("existing-table create must keep its PG blocker, got %+v", result.Statements[0].Findings)
		}
		alter := t05FindingsByRule(result, 1, "ddl.table.exists.alter.require")
		if len(alter) != 1 || alter[0].Level != rule.LevelBlocker {
			t.Fatalf("absent-table alter must keep its PG blocker, got %+v", result.Statements[1].Findings)
		}
		for i := range result.Statements {
			if len(result.Statements[i].EvidenceGaps) != 0 {
				t.Fatalf("provider-backed PostgreSQL statement %d must not report gaps, got %+v", i, result.Statements[i].EvidenceGaps)
			}
		}
	})
}
