// Package deltascope verifies the public ordered DROP contract (T05-A3).
// input: public audit requests under the isolated five-rule policy with a
// provider-confirmed-absent or unknown target
// output: pass results for the drop-recreate first path and the bounded
// unknown_table_state drop gap for an isolated unknown DROP
// pos: public SDK contract tests for the ordered-state drop transition
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
)

func writeT05A3FiveRulePolicy(t *testing.T) string {
	t.Helper()
	enabled := map[string]string{
		"ddl.table.exists.create.forbid":          "",
		"ddl.table.drop.exists.require":           "",
		"ddl.table.exists.alter.require":          "",
		"ddl.alter.add_column.exists.forbid":      "",
		"ddl.create_index.columns.exists.require": "      required: true\n",
	}
	ruleIDs := make([]string, 0, len(policy.Default().Rules))
	for id := range policy.Default().Rules {
		ruleIDs = append(ruleIDs, id)
	}
	sort.Strings(ruleIDs)
	var builder strings.Builder
	builder.WriteString("rules:\n")
	for _, id := range ruleIDs {
		if _, keep := enabled[id]; keep {
			continue
		}
		fmt.Fprintf(&builder, "  %s:\n    enabled: false\n", strconv.Quote(id))
	}
	for id, params := range enabled {
		fmt.Fprintf(&builder, "  %s:\n    enabled: true\n    level: blocker\n", strconv.Quote(id))
		if params != "" {
			builder.WriteString("    params:\n")
			builder.WriteString(params)
		}
	}
	path := filepath.Join(t.TempDir(), "t05-a3-isolated.yaml")
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatalf("write isolated policy: %v", err)
	}
	return path
}

const t05A3FirstPathSQL = "CREATE TABLE t (old_c INT PRIMARY KEY); DROP TABLE t; CREATE TABLE t (id INT PRIMARY KEY); ALTER TABLE t ADD COLUMN c INT; CREATE INDEX idx_c ON t(c);"

func TestAuditT05A3DropRecreateFirstPath(t *testing.T) {
	t.Parallel()
	for _, dialect := range []Dialect{DialectMySQL, DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			provider := &fakeMetadataProvider{snapshot: &TableSnapshot{Exists: false, Schema: "golden"}}
			result, err := Audit(context.Background(), Request{
				SQL:              t05A3FirstPathSQL,
				Dialect:          dialect,
				Schema:           "golden",
				ConfigPath:       writeT05A3FiveRulePolicy(t),
				MetadataProvider: provider,
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			if len(result.Statements) != 5 {
				t.Fatalf("expected 5 statements, got %#v", result.Statements)
			}
			for i, statement := range result.Statements {
				if statement.Coverage.Status != CoverageComplete || len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 {
					t.Fatalf("statement %d must be complete with zero findings and gaps, got %+v", i, statement)
				}
			}
			if result.Verdict != VerdictPass || result.Coverage.Status != CoverageComplete {
				t.Fatalf("aggregate = %s/%s, want pass/complete", result.Verdict, result.Coverage.Status)
			}
			if len(provider.tableCalls) != 1 || provider.tableCalls[0] != "t" {
				t.Fatalf("provider table calls = %#v, want exactly [t]", provider.tableCalls)
			}
		})
	}
}

func TestAuditT05A3IsolatedUnknownDropGap(t *testing.T) {
	t.Parallel()
	for _, dialect := range []Dialect{DialectMySQL, DialectTiDB} {
		dialect := dialect
		for _, sql := range []string{"DROP TABLE t;", "DROP TABLE IF EXISTS t;"} {
			sql := sql
			t.Run(string(dialect)+" "+sql, func(t *testing.T) {
				t.Parallel()
				provider := &fakeMetadataProvider{}
				result, err := Audit(context.Background(), Request{
					SQL:              sql,
					Dialect:          dialect,
					Schema:           "golden",
					ConfigPath:       writeT05A3FiveRulePolicy(t),
					MetadataProvider: provider,
				})
				if err != nil {
					t.Fatalf("gap-only audit must not error, got %v", err)
				}
				if result.Verdict != VerdictReview || result.Coverage.Status != CoverageUnverified {
					t.Fatalf("aggregate = %s/%s, want review/unverified", result.Verdict, result.Coverage.Status)
				}
				statement := result.Statements[0]
				if len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 1 {
					t.Fatalf("expected exactly one gap and zero findings, got %+v", statement)
				}
				gap := statement.EvidenceGaps[0]
				if gap.RuleID != "ddl.table.drop.exists.require" || gap.ReasonCode != "unknown_table_state" {
					t.Fatalf("gap = %#v, want ddl.table.drop.exists.require/unknown_table_state", gap)
				}
				if len(gap.RequiredFacts) != 1 || gap.RequiredFacts[0] != "target_table.existence" {
					t.Fatalf("expected required_facts [target_table.existence], got %#v", gap.RequiredFacts)
				}
				if len(provider.tableCalls) != 1 || provider.tableCalls[0] != "t" {
					t.Fatalf("provider table calls = %#v, want exactly [t]", provider.tableCalls)
				}
			})
		}
	}
}
