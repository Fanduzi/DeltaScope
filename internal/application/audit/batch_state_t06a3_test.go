// Package audit verifies the T06-A3 typed DEFAULT NULL contract through the
// shared audit path and the ordered-state DROP COLUMN seam.
// input: three-statement CREATE+DROP+INDEX batches through AuditSQL plus the enrichment seam
// output: typed DEFAULT NULL facts flowing into prospective state, string-literal conservative controls, and unchanged default-rule behavior
// pos: application-layer contract tests for the frozen T06-A3 oracle (issue #85)
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"reflect"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const (
	t06A3CreateForbid  = "ddl.table.exists.create.forbid"
	t06A3AlterRequire  = "ddl.table.exists.alter.require"
	t06A3DropExists    = "ddl.alter.drop_column.exists.require"
	t06A3IndexColumns  = "ddl.create_index.columns.exists.require"
	t06A3DefaultRule   = "ddl.column.default.require"
	t06A3PKNotNullRule = "ddl.table.primary_key.not_null.require"
)

// t06A3NullDropSQL is the frozen three-statement path: create, drop the
// unrelated column, then index the DEFAULT NULL sibling.
const t06A3NullDropSQL = "CREATE TABLE t (id INT PRIMARY KEY, obsolete INT, keep_c VARCHAR(8) DEFAULT NULL);\n" +
	"ALTER TABLE t DROP COLUMN obsolete;\n" +
	"CREATE INDEX idx_keep ON t(keep_c);"

// t06A3NullDropPolicy is the four-rule isolated profile pinned by the frozen
// contract (the three existence rules keep their shipped parameter shape).
func t06A3NullDropPolicy(t *testing.T) string {
	t.Helper()
	return t05PolicyPath(t, map[string]string{
		t06A3CreateForbid: "",
		t06A3AlterRequire: "",
		t06A3DropExists:   "",
		t06A3IndexColumns: "      required: true\n",
	})
}

func t06A3Audit(t *testing.T, dialect spec.Dialect, sql, policy string, provider MetadataProvider) report.Result {
	t.Helper()
	result, err := AuditSQL(context.Background(), Request{
		SQL: sql, Dialect: dialect, Schema: "golden",
		ConfigPath: policy, MetadataProvider: provider,
	})
	if err != nil {
		t.Fatalf("%s: unexpected error %v", dialect, err)
	}
	if len(result.Statements) != 3 {
		t.Fatalf("%s statements = %d, want 3", dialect, len(result.Statements))
	}
	return result
}

// TestT06A3NullDropStatePath pins the frozen green contract: a DEFAULT NULL
// sibling is provably column-free, so the drop stays precise and the index
// sees the real post-drop state.
func TestT06A3NullDropStatePath(t *testing.T) {
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			provider := &t05AbsentProvider{}
			result := t06A3Audit(t, dialect, t06A3NullDropSQL, t06A3NullDropPolicy(t), provider)
			if result.Verdict != report.VerdictPass || result.Coverage.Status != report.CoverageComplete {
				t.Fatalf("aggregate = %s/%s, want pass/complete", result.Verdict, result.Coverage.Status)
			}
			for i, statement := range result.Statements {
				if statement.Coverage.Status != report.CoverageComplete || len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 {
					t.Fatalf("statement %d = coverage %s findings %#v gaps %#v, want clean",
						i, statement.Coverage.Status, statement.Findings, statement.EvidenceGaps)
				}
			}
			if len(provider.calls) != 1 || provider.calls[0] != "golden.t" {
				t.Fatalf("provider calls = %#v, want exactly one golden.t load", provider.calls)
			}
			enriched := enrichA3(t, t06A3NullDropSQL, dialect, &t05AbsentProvider{})
			preDrop := enriched[1].Metadata.TargetTable
			if preDrop == nil || len(preDrop.Columns) != 3 || preDrop.Columns[0].Name != "id" || preDrop.Columns[1].Name != "obsolete" || preDrop.Columns[2].Name != "keep_c" {
				t.Fatalf("pre-drop state = %+v, want [id obsolete keep_c]", preDrop)
			}
			keep := preDrop.FindColumn("keep_c")
			if keep == nil || !keep.HasDefault || keep.DefaultValue != "NULL" || !keep.DefaultIsNull || keep.Length != 8 {
				t.Fatalf("keep_c pre-drop = %+v, want typed DEFAULT NULL with length 8", keep)
			}
			preIndex := enriched[2].Metadata.TargetTable
			if preIndex == nil || len(preIndex.Columns) != 2 || preIndex.Columns[0].Name != "id" || preIndex.Columns[1].Name != "keep_c" {
				t.Fatalf("pre-index state = %+v, want [id keep_c]", preIndex)
			}
			keep = preIndex.FindColumn("keep_c")
			if keep == nil || !keep.DefaultIsNull || keep.DefaultValue != "NULL" || !keep.HasDefault || keep.Length != 8 {
				t.Fatalf("keep_c post-drop = %+v, want typed NULL default preserved through state", keep)
			}
			if preIndex.PrimaryKey == nil || len(preIndex.PrimaryKey.Columns) != 1 || preIndex.PrimaryKey.Columns[0] != "id" {
				t.Fatalf("primary key post-drop = %+v, want PRIMARY(id)", preIndex.PrimaryKey)
			}
		})
	}
}

// TestT06A3StringDefaultsKeepConservativeDrop pins the A6 boundary: a string
// default ('NULL'/'<nil>') cannot be proved column-free, so the imprecise
// drop and the follower unknown_table_state gap stay — the fix must not
// relax real literal defaults.
func TestT06A3StringDefaultsKeepConservativeDrop(t *testing.T) {
	for _, literal := range []string{"'NULL'", "'<nil>'"} {
		sql := "CREATE TABLE t (id INT PRIMARY KEY, obsolete INT, keep_c VARCHAR(8) DEFAULT " + literal + ");\n" +
			"ALTER TABLE t DROP COLUMN obsolete;\n" +
			"CREATE INDEX idx_keep ON t(keep_c);"
		for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
			t.Run(string(dialect)+"/"+literal, func(t *testing.T) {
				result := t06A3Audit(t, dialect, sql, t06A3NullDropPolicy(t), &t05AbsentProvider{})
				if len(result.Statements[2].Findings) != 0 {
					t.Fatalf("index statement findings = %#v, want none", result.Statements[2].Findings)
				}
				gaps := t05GapsByRule(result, 2, t06A3IndexColumns)
				if len(gaps) != 1 || gaps[0].ReasonCode != "unknown_table_state" ||
					!reflect.DeepEqual(gaps[0].RequiredFacts, []string{"target_table.columns", "target_table.existence"}) {
					t.Fatalf("index statement gaps = %#v, want the preserved unknown_table_state gap", result.Statements[2].EvidenceGaps)
				}
				enriched := enrichA3(t, sql, dialect, &t05AbsentProvider{})
				preDrop := enriched[1].Metadata.TargetTable
				keep := preDrop.FindColumn("keep_c")
				if keep == nil || !keep.HasDefault || keep.DefaultValue != literal || keep.DefaultIsNull {
					t.Fatalf("keep_c = %+v, want string literal %s with DefaultIsNull=false", keep, literal)
				}
				t05A4R1WantUnknown(t, enriched[2])
			})
		}
	}
}

// TestT06A3OfflineProviderlessPreservesExistenceGap pins the offline control:
// without a provider the first CREATE keeps its existence evidence gap while
// the derived in-batch state still feeds the followers.
func TestT06A3OfflineProviderlessPreservesExistenceGap(t *testing.T) {
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			result := t06A3Audit(t, dialect, t06A3NullDropSQL, t06A3NullDropPolicy(t), nil)
			if result.Verdict != report.VerdictReview {
				t.Fatalf("aggregate verdict = %s, want review", result.Verdict)
			}
			if result.Coverage.Status == report.CoverageComplete {
				t.Fatalf("aggregate coverage = %s, want unverified/incomplete", result.Coverage.Status)
			}
			gaps := t05GapsByRule(result, 0, t06A3CreateForbid)
			if len(gaps) != 1 || gaps[0].ReasonCode != "unknown_table_state" {
				t.Fatalf("create gaps = %#v, want the original existence gap", result.Statements[0].EvidenceGaps)
			}
			for i := 1; i < 3; i++ {
				statement := result.Statements[i]
				if statement.Coverage.Status != report.CoverageComplete || len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 {
					t.Fatalf("statement %d = %s findings %#v gaps %#v, want clean", i, statement.Coverage.Status, statement.Findings, statement.EvidenceGaps)
				}
			}
		})
	}
}

// TestT06A3DefaultRequireSemanticsUnchanged pins the default-presence rule:
// absent clause still produces exactly one blocker, every explicit DEFAULT
// form satisfies it, and disabled/required:false stay silent.
func TestT06A3DefaultRequireSemanticsUnchanged(t *testing.T) {
	policy := t05PolicyPath(t, map[string]string{t06A3DefaultRule: "      required: true\n"})
	offPolicy := t05PolicyPath(t, map[string]string{t06A3DefaultRule: "      required: false\n"})
	cases := []struct {
		name    string
		sql     string
		wantOne bool
	}{
		{"no_default", "CREATE TABLE t (c VARCHAR(8));", true},
		{"sql_null", "CREATE TABLE t (c VARCHAR(8) DEFAULT NULL);", false},
		{"text_null", "CREATE TABLE t (c VARCHAR(8) DEFAULT 'NULL');", false},
		{"text_nil", "CREATE TABLE t (c VARCHAR(8) DEFAULT '<nil>');", false},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, tc := range cases {
			t.Run(string(dialect)+"/"+tc.name, func(t *testing.T) {
				result := t06A2Audit(t, dialect, tc.sql, policy)
				statement := result.Statements[0]
				if tc.wantOne {
					if result.Verdict != report.VerdictReject || len(statement.Findings) != 1 ||
						statement.Findings[0].RuleID != t06A3DefaultRule || statement.Findings[0].Level != "blocker" {
						t.Fatalf("want one default-require blocker, got %s findings %#v", result.Verdict, statement.Findings)
					}
				} else {
					if result.Verdict != report.VerdictPass || len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 {
						t.Fatalf("want pass, got %s findings %#v gaps %#v", result.Verdict, statement.Findings, statement.EvidenceGaps)
					}
				}
			})
		}
	}
	result := t06A2Audit(t, spec.DialectMySQL, "CREATE TABLE t (c VARCHAR(8));", offPolicy)
	if len(result.Statements[0].Findings) != 0 || len(result.Statements[0].EvidenceGaps) != 0 {
		t.Fatalf("required:false findings = %#v gaps %#v, want none", result.Statements[0].Findings, result.Statements[0].EvidenceGaps)
	}
}
