// Package cli verifies the T05-A3 ordered DROP contract at the CLI seam.
// input: isolated five-rule audit invocations for plain and IF EXISTS drops
// across both ordered dialects and every fail threshold
// output: exit-code and JSON evidence_gaps/coverage assertions for the
// drop-existence unknown_table_state gap
// pos: CLI transport contract tests for the ordered-state drop transition
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

func writeT05A3IsolatedPolicy(t *testing.T) string {
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

func runT05A3DropAudit(t *testing.T, configPath, dialect, sql string, extraArgs ...string) (map[string]any, int) {
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

// The offline drop-existence gap carries warning-equivalent threshold weight:
// warning/notice trip the fail gate, blocker and none do not, and the gap
// never appears as a finding on either ordered dialect or drop form.
func TestAuditCommandT05A3DropGapFailOnMatrix(t *testing.T) {
	configPath := writeT05A3IsolatedPolicy(t)

	cases := []struct {
		threshold   string
		wantExit    int
		wantTrigger bool
	}{
		{threshold: "warning", wantExit: exitAudit, wantTrigger: true},
		{threshold: "notice", wantExit: exitAudit, wantTrigger: true},
		{threshold: "blocker", wantExit: exitOK, wantTrigger: false},
		{threshold: "none", wantExit: exitOK, wantTrigger: false},
	}

	for _, dialect := range []string{"mysql", "tidb"} {
		for _, sql := range []string{"DROP TABLE t;", "DROP TABLE IF EXISTS t;"} {
			for _, tc := range cases {
				name := dialect + " " + sql + " fail-on-" + tc.threshold
				t.Run(name, func(t *testing.T) {
					decoded, code := runT05A3DropAudit(t, configPath, dialect, sql, "--fail-on", tc.threshold)
					if code != tc.wantExit {
						t.Fatalf("expected exit %d, got %d", tc.wantExit, code)
					}
					if decoded["fail_on_triggered"] != tc.wantTrigger {
						t.Fatalf("expected fail_on_triggered=%v, got %#v", tc.wantTrigger, decoded["fail_on_triggered"])
					}
					if decoded["verdict"] != "review" {
						t.Fatalf("expected verdict review, got %#v", decoded["verdict"])
					}
					coverage, _ := decoded["coverage"].(map[string]any)
					if coverage["status"] != "unverified" {
						t.Fatalf("expected aggregate coverage unverified, got %#v", decoded["coverage"])
					}
					summary, _ := decoded["summary"].(map[string]any)
					if summary["blockers"] != float64(0) || summary["warnings"] != float64(0) || summary["notices"] != float64(0) {
						t.Fatalf("gaps must not raise finding counters, got %#v", summary)
					}
					stmts, ok := decoded["statements"].([]any)
					if !ok || len(stmts) != 1 {
						t.Fatalf("expected 1 statement, got %#v", decoded["statements"])
					}
					first, _ := stmts[0].(map[string]any)
					if findings, ok := first["findings"].([]any); ok && len(findings) != 0 {
						t.Fatalf("isolated drop must have no findings, got %#v", findings)
					}
					gaps, _ := first["evidence_gaps"].([]any)
					if len(gaps) != 1 {
						t.Fatalf("expected exactly one evidence gap, got %#v", first["evidence_gaps"])
					}
					gap, _ := gaps[0].(map[string]any)
					if gap["rule_id"] != "ddl.table.drop.exists.require" || gap["reason_code"] != "unknown_table_state" {
						t.Fatalf("gap = %#v, want drop.exists.require/unknown_table_state", gap)
					}
					facts, _ := gap["required_facts"].([]any)
					if len(facts) != 1 || facts[0] != "target_table.existence" {
						t.Fatalf("expected required_facts [target_table.existence], got %#v", gap["required_facts"])
					}
				})
			}
		}
	}
}
