// Package audit verifies the TiDB cached-table guard's ALTER RENAME arm:
// only a genuinely unobserved cache-status premise may unvouch the pair.
package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const t05RenamePairSQL = "ALTER TABLE src.t RENAME TO dst.t2;\n" +
	"ALTER TABLE dst.t2 ADD COLUMN c INT;\n" +
	"ALTER TABLE src.t ADD COLUMN z INT;"

func t05RenamePairAudit(t *testing.T, sql string, provider MetadataProvider) report.Result {
	t.Helper()
	result, err := AuditSQL(context.Background(), Request{
		SQL:              sql,
		Dialect:          spec.DialectTiDB,
		Schema:           "golden",
		ConfigPath:       t05FourRulePolicy(t),
		MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	return result
}

// A batch-created source starts at the proven upstream Disable default, so
// the cached-table guard cannot unvouch the rename: the destination
// publishes the migrated shape and the source name settles absent.
func TestBatchStateRenamePairProvenCacheStatusMigrates(t *testing.T) {
	t.Parallel()
	result := t05RenamePairAudit(t,
		"CREATE TABLE src.t (id INT PRIMARY KEY);\n"+t05RenamePairSQL,
		&t05AbsentProvider{})
	for i := 1; i <= 3; i++ {
		if len(result.Statements[i].EvidenceGaps) != 0 {
			t.Fatalf("statement %d gaps = %+v, want none under a proven Disable source", i, result.Statements[i].EvidenceGaps)
		}
	}
	if result.Statements[3].Coverage.Status != report.CoverageComplete {
		t.Fatalf("source-name follow-up coverage = %s, want complete on the settled absent name", result.Statements[3].Coverage.Status)
	}
}

// A provider-loaded source whose cache status was never observed cannot
// prove the guard either way — the rename's hidden outcome unvouches the
// source name and a destination that was not proven live.
func TestBatchStateRenamePairUnobservedCacheStatusWithholds(t *testing.T) {
	t.Parallel()
	source := presentTable("src", "t", "id")
	delete(source.Options, "cache_status")
	provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
		"src.t":    source,
		"dst.t2":   absentTable("dst", "t2"),
		"golden.t": absentTable("golden", "t"),
	}}
	result := t05RenamePairAudit(t, t05RenamePairSQL, provider)
	for i, facts := range [][]string{
		{"target_table.existence"},
		{"target_table.existence"},
	} {
		gaps := t05GapsByRule(result, i+1, t05RuleAlterRequire)
		if len(gaps) != 1 || gaps[0].ReasonCode != "unknown_table_state" {
			t.Fatalf("statement %d alter.require gaps = %+v, want one unknown_table_state gap", i+1, result.Statements[i+1].EvidenceGaps)
		}
		if len(gaps[0].RequiredFacts) != len(facts) {
			t.Fatalf("statement %d facts = %+v, want %v", i+1, gaps[0].RequiredFacts, facts)
		}
	}
}

// A proven cache-enabled source makes the guard's refusal the statement's
// answer: the pre-state stands — the source keeps its shape and the
// destination stays proven absent with the ordinary missing-table finding.
func TestBatchStateRenamePairProvenCacheEnabledKeepsPreState(t *testing.T) {
	t.Parallel()
	source := presentTable("src", "t", "id")
	source.Options["cache_status"] = "enable"
	provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{
		"src.t":  source,
		"dst.t2": absentTable("dst", "t2"),
	}}
	result := t05RenamePairAudit(t, t05RenamePairSQL, provider)
	if findings := t05FindingsByRule(result, 1, t05RuleAlterRequire); len(findings) == 0 {
		t.Fatalf("destination follow-up findings = %+v, want the proven-absent missing-table blocker", result.Statements[1].Findings)
	}
	if len(result.Statements[1].EvidenceGaps) != 0 {
		t.Fatalf("destination follow-up gaps = %+v, want none on proven absent", result.Statements[1].EvidenceGaps)
	}
	if findings := t05FindingsByRule(result, 2, t05RuleAlterRequire); len(findings) != 0 {
		t.Fatalf("source follow-up findings = %+v, want none — the refused rename keeps the source live", result.Statements[2].Findings)
	}
}
