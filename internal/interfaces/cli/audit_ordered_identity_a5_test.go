// Package cli verifies the T05-A5 column-identity contract at the CLI seam.
// input: offline eleven-rule audit invocations for the CHANGE publication and
// the MySQL 5.7 RENAME blocker across the fail thresholds
// output: exit-code and JSON assertions for the version blocker and the
// leading CREATE gap that stays when no provider is attached
// pos: CLI transport contract tests for CHANGE and RENAME COLUMN post-state
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

func writeT05A5IsolatedPolicy(t *testing.T) string {
	t.Helper()
	enabled := map[string]string{
		"ddl.table.exists.create.forbid":                "",
		"ddl.table.exists.alter.require":                "",
		"ddl.alter.change_column.exists.require":        "",
		"ddl.alter.change_column.compatibility.require": "      required: true\n",
		"ddl.alter.rename_column.exists.require":        "",
		"ddl.alter.modify_column.exists.require":        "",
		"ddl.alter.modify_column.compatibility.require": "      required: true\n",
		"ddl.create_index.columns.exists.require":       "      required: true\n",
		"ddl.alter.change_column.target.exists.forbid":  "",
		"ddl.alter.rename_column.target.exists.forbid":  "",
		"ddl.alter.rename_column.version.require":       "      required: true\n",
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
	path := filepath.Join(t.TempDir(), "t05-a5-isolated.yaml")
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatalf("write isolated policy: %v", err)
	}
	return path
}

const t05A5ChangeSQL = "CREATE TABLE t (id INT PRIMARY KEY, c VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL);\n" +
	"ALTER TABLE t CHANGE COLUMN c c2 VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;\n" +
	"ALTER TABLE t MODIFY COLUMN c2 VARCHAR(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL;\n" +
	"CREATE INDEX idx_c2 ON t(c2);"

func runT05A5Audit(t *testing.T, configPath, dialect, sql string, extraArgs ...string) (map[string]any, int) {
	t.Helper()
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	args := []string{"audit", "--sql", sql, "--dialect", dialect, "--format", "json", "--config", configPath}
	args = append(args, extraArgs...)
	code := Execute(context.Background(), args, strings.NewReader(""), stdout, stderr)
	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal: %v\noutput=%s\nstderr=%s", err, stdout.String(), stderr.String())
	}
	return decoded, code
}

func TestAuditCommandT05A5FailOn(t *testing.T) {
	configPath := writeT05A5IsolatedPolicy(t)
	rename := strings.Replace(t05A5ChangeSQL, "CHANGE COLUMN c c2 VARCHAR(10) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL", "RENAME COLUMN c TO c2", 1)
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
	for _, tc := range cases {
		tc := tc
		t.Run("mysql rename 5.7 fail-on-"+tc.threshold, func(t *testing.T) {
			decoded, code := runT05A5Audit(t, configPath, "mysql", rename, "--target-version", "5.7.44", "--fail-on", tc.threshold)
			if code != tc.wantExit {
				t.Fatalf("expected exit %d, got %d", tc.wantExit, code)
			}
			if decoded["fail_on_triggered"] != tc.wantTrigger || decoded["verdict"] != "reject" {
				t.Fatalf("triggered %#v verdict %#v", decoded["fail_on_triggered"], decoded["verdict"])
			}
			summary, _ := decoded["summary"].(map[string]any)
			if summary["blockers"] != float64(1) {
				t.Fatalf("summary = %#v", summary)
			}
			statements, _ := decoded["statements"].([]any)
			renameStmt, _ := statements[1].(map[string]any)
			findings, _ := renameStmt["findings"].([]any)
			finding, _ := findings[0].(map[string]any)
			metadata, _ := finding["metadata"].(map[string]any)
			if finding["rule_id"] != "ddl.alter.rename_column.version.require" || metadata["target_version"] != "5.7.44" {
				t.Fatalf("finding = %#v", finding)
			}
			next, _ := statements[2].(map[string]any)
			if gaps, _ := next["evidence_gaps"].([]any); len(gaps) == 0 {
				t.Fatal("5.7 RENAME published c2")
			}
		})
	}
	for _, dialect := range []string{"mysql", "tidb"} {
		t.Run(dialect+" change fail-on-blocker", func(t *testing.T) {
			decoded, code := runT05A5Audit(t, configPath, dialect, t05A5ChangeSQL, "--fail-on", "blocker")
			if code != exitOK {
				t.Fatalf("expected exit %d, got %d", exitOK, code)
			}
			if decoded["fail_on_triggered"] != false || decoded["verdict"] != "review" {
				t.Fatalf("change = triggered %#v verdict %#v", decoded["fail_on_triggered"], decoded["verdict"])
			}
			statements, _ := decoded["statements"].([]any)
			for _, index := range []int{1, 2, 3} {
				statement, _ := statements[index].(map[string]any)
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
