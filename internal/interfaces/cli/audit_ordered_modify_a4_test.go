// Package cli verifies the T05-A4 ordered MODIFY contract at the CLI seam.
// input: offline four-rule audit invocations for the VARCHAR narrow and wide
// sequences across both ordered dialects and the fail thresholds
// output: exit-code and JSON assertions for the shrink blocker and the
// leading CREATE gap that stays when no provider is attached
// pos: CLI transport contract tests for ordinary single-column MODIFY post-state
// note: if this file changes, update this header and module README.md.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
)

func writeT05A4IsolatedPolicy(t *testing.T) string {
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

func runT05A4ModifyAudit(t *testing.T, configPath, dialect, sql string, extraArgs ...string) (map[string]any, int) {
	t.Helper()
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	args := []string{
		"audit", "--sql", sql, "--dialect", dialect,
		"--format", "json", "--config", configPath,
	}
	args = append(args, extraArgs...)
	code := Execute(context.Background(), args, strings.NewReader(""), stdout, stderr)
	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal: %v\noutput=%s\nstderr=%s", err, stdout.String(), stderr.String())
	}
	return decoded, code
}

func t05A4ShrinkFinding(t *testing.T, decoded map[string]any, source, target float64) {
	t.Helper()
	stmts, _ := decoded["statements"].([]any)
	if len(stmts) != 3 {
		t.Fatalf("expected 3 statements, got %#v", decoded["statements"])
	}
	third, _ := stmts[2].(map[string]any)
	findings, _ := third["findings"].([]any)
	if len(findings) != 1 {
		t.Fatalf("expected one finding, got %#v", third["findings"])
	}
	finding, _ := findings[0].(map[string]any)
	if finding["rule_id"] != "ddl.alter.modify_column.compatibility.require" || finding["level"] != "blocker" {
		t.Fatalf("finding = %#v, want compatibility blocker", finding)
	}
	metadata, _ := finding["metadata"].(map[string]any)
	if metadata["source_length"] != source || metadata["target_length"] != target {
		t.Fatalf("lengths = %#v, want %v to %v", metadata, source, target)
	}
	if metadata["table"] != "t" || metadata["name"] != "c" || metadata["column_name"] != "c" || metadata["action"] != "modify_column" {
		t.Fatalf("identity = %#v, want modify_column t.c", metadata)
	}
}

// Offline narrow carries both the leading CREATE gap and the 20-to-15 blocker.
// The blocker trips every threshold except none. Wide has only the CREATE gap,
// so --fail-on blocker stays exit 0.
func TestAuditCommandT05A4ModifyFailOn(t *testing.T) {
	configPath := writeT05A4IsolatedPolicy(t)
	wide := strings.Replace(t05A4NarrowSQL, "VARCHAR(15)", "VARCHAR(30)", 1)

	cases := []struct {
		threshold   string
		wantExit    int
		wantTrigger bool
	}{
		{threshold: "blocker", wantExit: exitAudit, wantTrigger: true},
		{threshold: "warning", wantExit: exitAudit, wantTrigger: true},
		{threshold: "notice", wantExit: exitAudit, wantTrigger: true},
		{threshold: "none", wantExit: exitOK, wantTrigger: false},
	}
	for _, dialect := range []string{"mysql", "tidb"} {
		for _, tc := range cases {
			name := dialect + " narrow fail-on-" + tc.threshold
			t.Run(name, func(t *testing.T) {
				decoded, code := runT05A4ModifyAudit(t, configPath, dialect, t05A4NarrowSQL, "--fail-on", tc.threshold)
				if code != tc.wantExit {
					t.Fatalf("expected exit %d, got %d", tc.wantExit, code)
				}
				if decoded["fail_on_triggered"] != tc.wantTrigger {
					t.Fatalf("expected fail_on_triggered=%v, got %#v", tc.wantTrigger, decoded["fail_on_triggered"])
				}
				if decoded["verdict"] != "reject" {
					t.Fatalf("expected verdict reject, got %#v", decoded["verdict"])
				}
				coverage, _ := decoded["coverage"].(map[string]any)
				if coverage["status"] != "unverified" {
					t.Fatalf("expected aggregate coverage unverified, got %#v", decoded["coverage"])
				}
				summary, _ := decoded["summary"].(map[string]any)
				if summary["blockers"] != float64(1) {
					t.Fatalf("expected one blocker, got %#v", summary)
				}
				t05A4ShrinkFinding(t, decoded, 20, 15)
				stmts := decoded["statements"].([]any)
				first, _ := stmts[0].(map[string]any)
				gaps, _ := first["evidence_gaps"].([]any)
				if len(gaps) != 1 {
					t.Fatalf("expected the create gap to stay, got %#v", first["evidence_gaps"])
				}
				gap, _ := gaps[0].(map[string]any)
				if gap["rule_id"] != "ddl.table.exists.create.forbid" || gap["reason_code"] != "unknown_table_state" {
					t.Fatalf("create gap = %#v", gap)
				}
			})
		}
		t.Run(dialect+" wide fail-on-blocker", func(t *testing.T) {
			decoded, code := runT05A4ModifyAudit(t, configPath, dialect, wide, "--fail-on", "blocker")
			if code != exitOK {
				t.Fatalf("expected exit %d, got %d", exitOK, code)
			}
			if decoded["fail_on_triggered"] != false || decoded["verdict"] != "review" {
				t.Fatalf("wide = triggered %#v verdict %#v, want false/review", decoded["fail_on_triggered"], decoded["verdict"])
			}
			coverage, _ := decoded["coverage"].(map[string]any)
			if coverage["status"] != "unverified" {
				t.Fatalf("expected unverified, got %#v", decoded["coverage"])
			}
			summary, _ := decoded["summary"].(map[string]any)
			if summary["blockers"] != float64(0) {
				t.Fatalf("wide must have zero blockers, got %#v", summary)
			}
			stmts := decoded["statements"].([]any)
			for _, index := range []int{1, 2} {
				statement, _ := stmts[index].(map[string]any)
				if findings, ok := statement["findings"].([]any); ok && len(findings) != 0 {
					t.Fatalf("statement %d findings = %#v", index, findings)
				}
				if gaps, ok := statement["evidence_gaps"].([]any); ok && len(gaps) != 0 {
					t.Fatalf("statement %d gaps = %#v", index, gaps)
				}
			}
		})
	}
}
