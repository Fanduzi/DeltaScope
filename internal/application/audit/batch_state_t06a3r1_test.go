// Package audit verifies the T06-A3-R1 per-option DEFAULT contract through
// the shared audit path: a repeated DEFAULT ending in a string literal must
// keep the A6 conservative drop boundary, while one ending in SQL NULL stays
// provably column-free.
// input: three-statement CREATE+DROP+INDEX batches with repeated DEFAULT options through AuditSQL
// output: conservative unknown_table_state gap for string-ending duplicates; complete path for NULL-ending ones
// pos: application-layer contract tests for issue #85 T06-A3-R1
// note: if this file changes, update this header and module README.md.
package audit

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// t06a3r1Batch renders the frozen three-statement path with the given column
// default tail on keep_c.
func t06a3r1Batch(defaults string) string {
	return "CREATE TABLE t (id INT PRIMARY KEY, obsolete INT, keep_c VARCHAR(8) " + defaults + ");\n" +
		"ALTER TABLE t DROP COLUMN obsolete;\n" +
		"CREATE INDEX idx_keep ON t(keep_c);"
}

// TestT06A3R1DuplicateDefaultConservativePath pins the frozen contract for a
// repeated DEFAULT that ends in a string literal: the stale NULL flag must be
// gone, so the drop stays imprecise and the follower index keeps exactly the
// unknown_table_state gap — provider is read once and never re-asked.
func TestT06A3R1DuplicateDefaultConservativePath(t *testing.T) {
	for _, tail := range []string{
		"DEFAULT NULL DEFAULT 'NULL'",
		"DEFAULT NULL DEFAULT '<nil>'",
	} {
		for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
			t.Run(string(dialect)+"/"+tail, func(t *testing.T) {
				provider := &t05AbsentProvider{}
				result := t06A3Audit(t, dialect, t06a3r1Batch(tail), t06A3NullDropPolicy(t), provider)
				if result.Verdict != report.VerdictReview || result.Coverage.Status != report.CoverageUnverified {
					t.Fatalf("aggregate = %s/%s, want review/unverified", result.Verdict, result.Coverage.Status)
				}
				for i := 0; i < 2; i++ {
					statement := result.Statements[i]
					if statement.Coverage.Status != report.CoverageComplete || len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 {
						t.Fatalf("statement %d = %s findings %#v gaps %#v, want clean", i, statement.Coverage.Status, statement.Findings, statement.EvidenceGaps)
					}
				}
				third := result.Statements[2]
				if third.Coverage.Status != report.CoverageUnverified || len(third.Findings) != 0 {
					t.Fatalf("statement 2 = %s findings %#v, want unverified with no findings", third.Coverage.Status, third.Findings)
				}
				gaps := t05GapsByRule(result, 2, t06A3IndexColumns)
				if len(gaps) != 1 || gaps[0].ReasonCode != "unknown_table_state" {
					t.Fatalf("statement 2 gaps = %#v, want exactly one unknown_table_state", third.EvidenceGaps)
				}
				if len(provider.calls) != 1 || provider.calls[0] != "golden.t" {
					t.Fatalf("provider calls = %#v, want exactly one golden.t load", provider.calls)
				}
				enriched := enrichA3(t, t06a3r1Batch(tail), dialect, &t05AbsentProvider{})
				keep := enriched[1].Metadata.TargetTable.FindColumn("keep_c")
				if keep == nil || !keep.HasDefault || keep.DefaultValue != tail[len("DEFAULT NULL DEFAULT "):] || keep.DefaultIsNull {
					t.Fatalf("keep_c = %+v, want the final string literal with DefaultIsNull=false", keep)
				}
			})
		}
	}
}

// TestT06A3R1NullEndingDuplicateStaysComplete pins the opposite end: a
// repeated DEFAULT that ends in SQL NULL remains the precise drop path —
// all three statements complete with zero findings and gaps.
func TestT06A3R1NullEndingDuplicateStaysComplete(t *testing.T) {
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			provider := &t05AbsentProvider{}
			result := t06A3Audit(t, dialect, t06a3r1Batch("DEFAULT NULL DEFAULT 'NULL' DEFAULT NULL"), t06A3NullDropPolicy(t), provider)
			if result.Verdict != report.VerdictPass || result.Coverage.Status != report.CoverageComplete {
				t.Fatalf("aggregate = %s/%s, want pass/complete", result.Verdict, result.Coverage.Status)
			}
			for i, statement := range result.Statements {
				if len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 {
					t.Fatalf("statement %d findings %#v gaps %#v, want clean", i, statement.Findings, statement.EvidenceGaps)
				}
			}
			if len(provider.calls) != 1 {
				t.Fatalf("provider calls = %#v, want exactly one load", provider.calls)
			}
			enriched := enrichA3(t, t06a3r1Batch("DEFAULT NULL DEFAULT 'NULL' DEFAULT NULL"), dialect, &t05AbsentProvider{})
			keep := enriched[2].Metadata.TargetTable.FindColumn("keep_c")
			if keep == nil || !keep.DefaultIsNull || keep.DefaultValue != "NULL" {
				t.Fatalf("keep_c post-drop = %+v, want typed NULL default preserved through state", keep)
			}
		})
	}
}
