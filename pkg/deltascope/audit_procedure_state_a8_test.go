// Package deltascope verifies the T05-A8 procedure lifecycle at the public SDK seam.
// input: four-statement TiDB CREATE PROCEDURE and MySQL DROP PROCEDURE migration batches through the public Audit entrypoint
// output: ErrUnsupportedStatement partial results versus nil-error gap-only results, with complete outer followers
// pos: public SDK contract test for procedure table-state isolation (T05-A8)
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule/catalog"
)

var t05A8ProcedureLines = []string{
	"CREATE TABLE t (id INT PRIMARY KEY);",
	"",
	"ALTER TABLE t ADD COLUMN c INT;",
	"CREATE INDEX idx_c ON t(c);",
}

func t05A8SDKPolicy(t *testing.T) string {
	t.Helper()
	enabled := map[string]string{
		"ddl.table.exists.create.forbid":          "",
		"ddl.table.exists.alter.require":          "",
		"ddl.alter.add_column.exists.forbid":      "",
		"ddl.create_index.columns.exists.require": "      required: true\n",
	}
	for id := range enabled {
		if _, ok := catalog.Lookup(id); !ok {
			t.Fatalf("missing catalog rule %s", id)
		}
	}
	var text strings.Builder
	text.WriteString("rules:\n")
	for _, entry := range catalog.All() {
		params, on := enabled[entry.RuleID]
		fmt.Fprintf(&text, "  %q:\n    enabled: %t\n", entry.RuleID, on)
		if on {
			text.WriteString("    level: blocker\n")
			if params != "" {
				text.WriteString("    params:\n" + params)
			}
		}
	}
	path := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(path, []byte(text.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestT05A8ProcedureOuterState(t *testing.T) {
	policy := t05A8SDKPolicy(t)
	cases := []struct {
		name      string
		dialect   Dialect
		procedure string
		wantErr   bool
		verdict   Verdict
		coverage  CoverageStatus
		boundary  bool
	}{
		{"tidb_create", DialectTiDB, "CREATE PROCEDURE p() SELECT 1;", true, VerdictReview, CoverageIncomplete, true},
		{"mysql_drop", DialectMySQL, "DROP PROCEDURE p;", false, VerdictReview, CoverageUnverified, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lines := append([]string(nil), t05A8ProcedureLines...)
			lines[1] = tc.procedure
			sql := strings.Join(lines, "\n")
			result, err := Audit(context.Background(), Request{
				SQL: sql, Dialect: tc.dialect, Schema: "golden", ConfigPath: policy,
			})
			if tc.wantErr {
				if !errors.Is(err, ErrUnsupportedStatement) {
					t.Fatalf("error = %v, want ErrUnsupportedStatement", err)
				}
			} else if err != nil {
				t.Fatalf("error = %v, want nil", err)
			}
			if result.Verdict != tc.verdict || result.Coverage.Status != tc.coverage {
				t.Fatalf("aggregate = %s/%s, want %s/%s", result.Verdict, result.Coverage.Status, tc.verdict, tc.coverage)
			}
			if len(result.Statements) != 4 {
				t.Fatalf("statements = %d, want 4", len(result.Statements))
			}
			for i, statement := range result.Statements {
				if statement.Index != i || statement.Kind != "ddl" || statement.RawSQL != lines[i] ||
					statement.NormalizedSQL != strings.TrimSuffix(lines[i], ";") {
					t.Fatalf("statement %d identity = %+v", i, statement)
				}
				if statement.Impact != nil {
					t.Fatalf("statement %d impact = %+v, want nil", i, statement.Impact)
				}
			}
			first := result.Statements[0]
			if first.Coverage.Status != CoverageUnverified || len(first.Findings) != 0 {
				t.Fatalf("first statement = %+v, want unverified with no findings", first)
			}
			if len(first.EvidenceGaps) != 1 {
				t.Fatalf("first gaps = %+v, want exactly the create-existence gap", first.EvidenceGaps)
			}
			gap := first.EvidenceGaps[0]
			if gap.RuleID != "ddl.table.exists.create.forbid" || gap.ReasonCode != "unknown_table_state" ||
				!reflect.DeepEqual(gap.RequiredFacts, []string{"target_table.existence"}) {
				t.Fatalf("first gap = %+v, want unknown_table_state create-existence", gap)
			}
			procedure := result.Statements[1]
			wantProcedureCoverage := CoverageComplete
			if tc.boundary {
				wantProcedureCoverage = CoverageIncomplete
			}
			if procedure.Coverage.Status != wantProcedureCoverage || len(procedure.Findings) != 0 || len(procedure.EvidenceGaps) != 0 {
				t.Fatalf("procedure statement = %+v, want %s with no findings/gaps", procedure, wantProcedureCoverage)
			}
			for i := 2; i < 4; i++ {
				statement := result.Statements[i]
				if statement.Coverage.Status != CoverageComplete || len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 {
					t.Fatalf("follower %d = %+v, want complete with no findings/gaps", i, statement)
				}
			}
			wantUnsupported := 0
			if tc.boundary {
				wantUnsupported = 1
			}
			if len(result.Unsupported) != wantUnsupported {
				t.Fatalf("unsupported = %+v, want %d", result.Unsupported, wantUnsupported)
			}
			if tc.boundary {
				item := result.Unsupported[0]
				if item.Index != 1 || item.Feature != "create_procedure" ||
					item.Reason != "parsed by the shared parser but outside the supported statement surface for this dialect" ||
					item.SQL != procedure.RawSQL ||
					!reflect.DeepEqual(item.Metadata, map[string]any{"boundary": "vendor"}) {
					t.Fatalf("boundary = %+v, want create_procedure vendor at index 1", item)
				}
			}
			if len(result.GlobalFindings) != 0 {
				t.Fatalf("global findings = %+v, want none", result.GlobalFindings)
			}
			if result.Summary.Blockers != 0 || result.Summary.Warnings != 0 || result.Summary.Notices != 0 {
				t.Fatalf("finding counters = %+v, want zero", result.Summary)
			}
		})
	}
}
