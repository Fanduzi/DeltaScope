// Package cli verifies the T05-A6 ordered DROP COLUMN contract at the CLI seam.
// input: offline six-rule audit invocations for the first path and the
// removed-column index across both ordered dialects and the fail thresholds
// output: exit-code and JSON assertions for the leading CREATE gap and the
// old-column blocker
// pos: CLI transport contract tests for ordinary single-column DROP COLUMN post-state
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

func writeT05A6IsolatedPolicy(t *testing.T) string {
	t.Helper()
	enabled := map[string]string{
		"ddl.table.exists.create.forbid":                "",
		"ddl.table.exists.alter.require":                "",
		"ddl.alter.drop_column.exists.require":          "",
		"ddl.alter.modify_column.exists.require":        "",
		"ddl.alter.modify_column.compatibility.require": "      required: true\n",
		"ddl.create_index.columns.exists.require":       "      required: true\n",
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
	path := filepath.Join(t.TempDir(), "t05-a6-isolated.yaml")
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatalf("write isolated policy: %v", err)
	}
	return path
}

const t05A6DropSQL = "CREATE TABLE t (id INT PRIMARY KEY, obsolete INT, keep_c VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL);\n" +
	"ALTER TABLE t DROP COLUMN obsolete;\n" +
	"ALTER TABLE t MODIFY COLUMN keep_c VARCHAR(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;\n" +
	"CREATE INDEX idx_keep ON t(keep_c);"

func runT05A6DropAudit(t *testing.T, configPath, dialect, sql string, extraArgs ...string) (map[string]any, int) {
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

func TestAuditCommandT05A6DropColumnFailOn(t *testing.T) {
	configPath := writeT05A6IsolatedPolicy(t)
	removedIndex := strings.Replace(t05A6DropSQL, "CREATE INDEX idx_keep ON t(keep_c);", "CREATE INDEX ix_removed ON t(obsolete);", 1)
	// The leading CREATE gap keeps #83 warning-equivalent weight. A blocker
	// finding on the negative path trips every threshold except none.
	positive := []struct {
		threshold   string
		wantExit    int
		wantTrigger bool
	}{
		{threshold: "blocker", wantExit: exitOK, wantTrigger: false},
		{threshold: "warning", wantExit: exitAudit, wantTrigger: true},
		{threshold: "notice", wantExit: exitAudit, wantTrigger: true},
		{threshold: "none", wantExit: exitOK, wantTrigger: false},
	}
	negative := []struct {
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
		for _, tc := range positive {
			t.Run(dialect+" positive fail-on-"+tc.threshold, func(t *testing.T) {
				decoded, code := runT05A6DropAudit(t, configPath, dialect, t05A6DropSQL, "--fail-on", tc.threshold)
				if code != tc.wantExit || decoded["fail_on_triggered"] != tc.wantTrigger || decoded["verdict"] != "review" {
					t.Fatalf("fail-on %s = exit %d triggered %#v verdict %#v", tc.threshold, code, decoded["fail_on_triggered"], decoded["verdict"])
				}
				coverage, _ := decoded["coverage"].(map[string]any)
				if coverage["status"] != "unverified" {
					t.Fatalf("coverage = %#v", decoded["coverage"])
				}
				summary, _ := decoded["rule_summary"].(map[string]any)
				if summary["loaded"] != float64(6) {
					t.Fatalf("loaded = %#v", summary)
				}
				stmts := decoded["statements"].([]any)
				first := stmts[0].(map[string]any)
				gaps, _ := first["evidence_gaps"].([]any)
				if len(gaps) != 1 {
					t.Fatalf("create gaps = %#v", first["evidence_gaps"])
				}
				gap := gaps[0].(map[string]any)
				if gap["rule_id"] != "ddl.table.exists.create.forbid" || gap["reason_code"] != "unknown_table_state" {
					t.Fatalf("create gap = %#v", gap)
				}
				for index := 1; index < 4; index++ {
					statement := stmts[index].(map[string]any)
					if statement["coverage"].(map[string]any)["status"] != "complete" {
						t.Fatalf("statement %d = %#v", index, statement["coverage"])
					}
					if findings, ok := statement["findings"].([]any); ok && len(findings) != 0 {
						t.Fatalf("statement %d findings = %#v", index, findings)
					}
				}
			})
		}
		for _, tc := range negative {
			t.Run(dialect+" negative fail-on-"+tc.threshold, func(t *testing.T) {
				decoded, code := runT05A6DropAudit(t, configPath, dialect, removedIndex, "--fail-on", tc.threshold)
				if code != tc.wantExit || decoded["fail_on_triggered"] != tc.wantTrigger || decoded["verdict"] != "reject" {
					t.Fatalf("exit %d triggered %#v verdict %#v", code, decoded["fail_on_triggered"], decoded["verdict"])
				}
				stmts := decoded["statements"].([]any)
				last := stmts[3].(map[string]any)
				findings, _ := last["findings"].([]any)
				if len(findings) != 1 {
					t.Fatalf("findings = %#v", last["findings"])
				}
				finding := findings[0].(map[string]any)
				metadata, _ := finding["metadata"].(map[string]any)
				if finding["rule_id"] != "ddl.create_index.columns.exists.require" || metadata["schema"] != "" || metadata["table"] != "t" || metadata["index"] != "ix_removed" || metadata["column"] != "obsolete" || metadata["exists"] != false {
					t.Fatalf("finding = %#v", finding)
				}
			})
		}
	}
}
