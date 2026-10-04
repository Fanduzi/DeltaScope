// Package deltascope verifies the public ordered MODIFY contract (T05-A4).
// input: public audit requests under the isolated four-rule policy for the
// VARCHAR(10) then VARCHAR(20) then VARCHAR(15|30) sequence
// output: nil-error reject/pass results that pin source_length and
// target_length, plus the offline gap shapes and a wrapped provider error
// pos: public SDK contract tests for ordinary single-column MODIFY post-state
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
)

func writeT05A4FourRulePolicy(t *testing.T) string {
	t.Helper()
	enabled := map[string]string{
		"ddl.table.exists.create.forbid":                "",
		"ddl.table.exists.alter.require":                "",
		"ddl.alter.modify_column.exists.require":        "",
		"ddl.alter.modify_column.compatibility.require": "      required: true\n",
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
	path := filepath.Join(t.TempDir(), "t05-a4-isolated.yaml")
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatalf("write isolated policy: %v", err)
	}
	return path
}

const t05A4NarrowSQL = "CREATE TABLE t (id INT PRIMARY KEY, c VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL);\n" +
	"ALTER TABLE t MODIFY COLUMN c VARCHAR(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;\n" +
	"ALTER TABLE t MODIFY COLUMN c VARCHAR(15) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;"

func t05A4WideSQL() string {
	return strings.Replace(t05A4NarrowSQL, "VARCHAR(15)", "VARCHAR(30)", 1)
}

func t05A4AbsentProvider() *fakeMetadataProvider {
	return &fakeMetadataProvider{snapshot: &TableSnapshot{Exists: false, Schema: "golden"}}
}

func TestAuditT05A4ModifyNarrowReject(t *testing.T) {
	t.Parallel()
	for _, dialect := range []Dialect{DialectMySQL, DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			provider := t05A4AbsentProvider()
			result, err := Audit(context.Background(), Request{
				SQL:              t05A4NarrowSQL,
				Dialect:          dialect,
				Schema:           "golden",
				ConfigPath:       writeT05A4FourRulePolicy(t),
				MetadataProvider: provider,
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			if len(result.Statements) != 3 {
				t.Fatalf("expected 3 statements, got %#v", result.Statements)
			}
			for _, index := range []int{0, 1} {
				statement := result.Statements[index]
				if statement.Coverage.Status != CoverageComplete || len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 {
					t.Fatalf("statement %d = %+v, want complete with none", index, statement)
				}
			}
			finding := result.Statements[2].Findings
			if result.Statements[2].Coverage.Status != CoverageComplete || len(result.Statements[2].EvidenceGaps) != 0 || len(finding) != 1 {
				t.Fatalf("statement 2 = %+v, want one complete blocker", result.Statements[2])
			}
			if finding[0].RuleID != "ddl.alter.modify_column.compatibility.require" || finding[0].Level != LevelBlocker || finding[0].StatementIndex != 2 {
				t.Fatalf("finding = %#v, want compatibility blocker on statement 2", finding[0])
			}
			if finding[0].Metadata["source_length"] != 20 || finding[0].Metadata["target_length"] != 15 {
				t.Fatalf("lengths = %#v, want 20 to 15", finding[0].Metadata)
			}
			if finding[0].Metadata["action"] != "modify_column" || finding[0].Metadata["table"] != "t" || finding[0].Metadata["name"] != "c" || finding[0].Metadata["column_name"] != "c" {
				t.Fatalf("identity = %#v, want modify_column t.c", finding[0].Metadata)
			}
			if result.Verdict != VerdictReject || result.Coverage.Status != CoverageComplete {
				t.Fatalf("aggregate = %s/%s, want reject/complete", result.Verdict, result.Coverage.Status)
			}
			if len(provider.tableCalls) != 1 || provider.tableCalls[0] != "t" {
				t.Fatalf("provider table calls = %#v, want exactly [t]", provider.tableCalls)
			}
		})
	}
}

func TestAuditT05A4ModifyWidePass(t *testing.T) {
	t.Parallel()
	for _, dialect := range []Dialect{DialectMySQL, DialectTiDB} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			provider := t05A4AbsentProvider()
			result, err := Audit(context.Background(), Request{
				SQL:              t05A4WideSQL(),
				Dialect:          dialect,
				Schema:           "golden",
				ConfigPath:       writeT05A4FourRulePolicy(t),
				MetadataProvider: provider,
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			if len(result.Statements) != 3 {
				t.Fatalf("expected 3 statements, got %d", len(result.Statements))
			}
			for i, statement := range result.Statements {
				if statement.Coverage.Status != CoverageComplete || len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 {
					t.Fatalf("statement %d = %+v, want complete with none", i, statement)
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

func TestAuditT05A4ModifyNoProviderKeepsCreateGap(t *testing.T) {
	t.Parallel()
	narrow, err := Audit(context.Background(), Request{
		SQL:        t05A4NarrowSQL,
		Dialect:    DialectMySQL,
		Schema:     "golden",
		ConfigPath: writeT05A4FourRulePolicy(t),
	})
	if err != nil {
		t.Fatalf("narrow audit: %v", err)
	}
	if narrow.Verdict != VerdictReject || narrow.Coverage.Status != CoverageUnverified {
		t.Fatalf("narrow aggregate = %s/%s, want reject/unverified", narrow.Verdict, narrow.Coverage.Status)
	}
	if len(narrow.Statements[0].Findings) != 0 || len(narrow.Statements[0].EvidenceGaps) != 1 {
		t.Fatalf("create gap must stay on statement 0, got %+v", narrow.Statements[0])
	}
	gap := narrow.Statements[0].EvidenceGaps[0]
	if gap.RuleID != "ddl.table.exists.create.forbid" || gap.ReasonCode != "unknown_table_state" {
		t.Fatalf("create gap = %#v", gap)
	}
	finding := narrow.Statements[2].Findings
	if len(finding) != 1 || finding[0].Metadata["source_length"] != 20 || finding[0].Metadata["target_length"] != 15 {
		t.Fatalf("offline shrink = %#v, want 20 to 15", finding)
	}

	wide, err := Audit(context.Background(), Request{
		SQL:        t05A4WideSQL(),
		Dialect:    DialectTiDB,
		Schema:     "golden",
		ConfigPath: writeT05A4FourRulePolicy(t),
	})
	if err != nil {
		t.Fatalf("wide audit: %v", err)
	}
	if wide.Verdict != VerdictReview || wide.Coverage.Status != CoverageUnverified {
		t.Fatalf("wide aggregate = %s/%s, want review/unverified", wide.Verdict, wide.Coverage.Status)
	}
	if len(wide.Statements[0].EvidenceGaps) != 1 || wide.Statements[0].EvidenceGaps[0].RuleID != "ddl.table.exists.create.forbid" {
		t.Fatalf("wide create gap = %#v", wide.Statements[0].EvidenceGaps)
	}
	for _, index := range []int{1, 2} {
		statement := wide.Statements[index]
		if statement.Coverage.Status != CoverageComplete || len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 {
			t.Fatalf("statement %d = %+v, want complete derived state", index, statement)
		}
	}
}

func TestAuditT05A4ProviderError(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("t05-a4 provider down")
	_, err := Audit(context.Background(), Request{
		SQL:              t05A4NarrowSQL,
		Dialect:          DialectMySQL,
		Schema:           "golden",
		ConfigPath:       writeT05A4FourRulePolicy(t),
		MetadataProvider: &fakeMetadataProvider{err: sentinel},
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("provider error = %v, want the sentinel", err)
	}
}
