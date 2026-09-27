// Package cli verifies the T04-A/#83 evidence-gap CLI contract.
// input: isolated-policy audit invocations with missing metadata and mixed unsupported input
// output: exit-code and JSON evidence_gaps/coverage assertions for the warning-equivalent fail threshold
// pos: CLI transport contract tests for metadata evidence gaps (issue #83 T04-A slice)
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

const t04GoldenSQL = "ALTER TABLE t MODIFY COLUMN c VARCHAR(20);"
const t04TargetRuleID = "ddl.alter.modify_column.compatibility.require"

// writeT04IsolatedPolicy disables every cataloged rule and enables only the
// target rule with requires_metadata — the same policy shape the golden runner
// builds for T04.
func writeT04IsolatedPolicy(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "t04-isolated.yaml")
	ids := make([]string, 0, len(policy.Default().Rules))
	for id := range policy.Default().Rules {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var builder strings.Builder
	builder.WriteString("rules:\n")
	for _, id := range ids {
		if id == t04TargetRuleID {
			builder.WriteString("  " + strconv.Quote(id) + ":\n    enabled: true\n    level: blocker\n    params:\n      required: true\n      requires_metadata: true\n")
			continue
		}
		fmt.Fprintf(&builder, "  %s:\n    enabled: false\n", strconv.Quote(id))
	}
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatalf("write isolated policy: %v", err)
	}
	return path
}

func runT04Audit(t *testing.T, configPath string, extraArgs ...string) (map[string]any, int) {
	t.Helper()
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	args := []string{
		"audit", "--sql", t04GoldenSQL, "--dialect", "mysql",
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

func t04EvidenceGaps(t *testing.T, decoded map[string]any) []any {
	t.Helper()
	stmts, ok := decoded["statements"].([]any)
	if !ok || len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %#v", decoded["statements"])
	}
	first, _ := stmts[0].(map[string]any)
	gaps, _ := first["evidence_gaps"].([]any)
	return gaps
}

// Evidence gaps carry warning-equivalent threshold weight: warning/notice trip,
// blocker and none do not, and the gap never appears as a finding.
func TestAuditCommandT04EvidenceGapFailOnMatrix(t *testing.T) {
	configPath := writeT04IsolatedPolicy(t)

	cases := []struct {
		threshold   string
		wantExit    int
		wantTrigger bool
	}{
		{threshold: "blocker", wantExit: exitOK, wantTrigger: false},
		{threshold: "warning", wantExit: exitAudit, wantTrigger: true},
		{threshold: "notice", wantExit: exitAudit, wantTrigger: true},
		{threshold: "none", wantExit: exitOK, wantTrigger: false},
	}

	for _, tc := range cases {
		t.Run("fail-on-"+tc.threshold, func(t *testing.T) {
			decoded, code := runT04Audit(t, configPath, "--fail-on", tc.threshold)
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
			gaps := t04EvidenceGaps(t, decoded)
			if len(gaps) != 1 {
				t.Fatalf("expected exactly one evidence gap, got %#v", gaps)
			}
			gap, _ := gaps[0].(map[string]any)
			if gap["rule_id"] != t04TargetRuleID || gap["reason_code"] != "missing_source_column" {
				t.Fatalf("gap = %#v, want %q/missing_source_column", gap, t04TargetRuleID)
			}
			facts, _ := gap["required_facts"].([]any)
			if len(facts) != 1 || facts[0] != "source_column.definition" {
				t.Fatalf("expected required_facts [source_column.definition], got %#v", gap["required_facts"])
			}
		})
	}
}

// A missing-metadata MODIFY plus a recognized-but-unsupported statement keeps
// both evidence kinds: the unsupported entry still forces exit 1 under
// --fail-on none and the statement list preserves per-statement coverage.
func TestAuditCommandT04MixedGapAndUnsupported(t *testing.T) {
	configPath := writeT04IsolatedPolicy(t)

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{
			"audit", "--dialect", "mysql", "--format", "json", "--fail-on", "none",
			"--config", configPath,
			"--sql", t04GoldenSQL + " CREATE SEQUENCE golden_seq START WITH 1;",
		},
		strings.NewReader(""),
		stdout,
		stderr,
	)
	if code != exitAudit {
		t.Fatalf("expected exit %d for unsupported mix, got %d, stderr=%q", exitAudit, code, stderr.String())
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal: %v\noutput=%s", err, stdout.String())
	}
	coverage, _ := decoded["coverage"].(map[string]any)
	if coverage["status"] != "incomplete" {
		t.Fatalf("expected aggregate incomplete, got %#v", decoded["coverage"])
	}
	unsupported, _ := decoded["unsupported"].([]any)
	if len(unsupported) != 1 {
		t.Fatalf("expected 1 unsupported entry, got %#v", decoded["unsupported"])
	}
	stmts, _ := decoded["statements"].([]any)
	first, _ := stmts[0].(map[string]any)
	if firstCov, _ := first["coverage"].(map[string]any); firstCov["status"] != "unverified" {
		t.Fatalf("expected first statement unverified, got %#v", first["coverage"])
	}
	gaps, _ := first["evidence_gaps"].([]any)
	if len(gaps) != 1 {
		t.Fatalf("expected evidence gap preserved on first statement, got %#v", gaps)
	}
}

// A valid MODIFY followed by a parser failure preserves the partial-result
// contract: parser error wins over gap output and exits 2.
func TestAuditCommandT04ParserFailurePriority(t *testing.T) {
	configPath := writeT04IsolatedPolicy(t)

	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{
			"audit", "--dialect", "mysql", "--format", "json",
			"--config", configPath,
			"--sql", t04GoldenSQL + " THIS IS NOT VALID SQL AT ALL;",
		},
		strings.NewReader(""),
		stdout,
		stderr,
	)
	if code != exitUser {
		t.Fatalf("expected exit %d for parser failure, got %d, stderr=%q", exitUser, code, stderr.String())
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal: %v\noutput=%s", err, stdout.String())
	}
	diagnostics, _ := decoded["diagnostics"].([]any)
	if len(diagnostics) == 0 {
		t.Fatalf("expected parser diagnostics, got %#v", decoded["diagnostics"])
	}
	coverage, _ := decoded["coverage"].(map[string]any)
	if coverage["status"] != "incomplete" {
		t.Fatalf("expected aggregate incomplete from parser failure, got %#v", decoded["coverage"])
	}
	stmts, _ := decoded["statements"].([]any)
	if len(stmts) != 1 {
		t.Fatalf("expected retained statement in partial result, got %#v", decoded["statements"])
	}
	first, _ := stmts[0].(map[string]any)
	gaps, _ := first["evidence_gaps"].([]any)
	if len(gaps) != 1 {
		t.Fatalf("expected evidence gap retained on the parsed statement, got %#v", gaps)
	}
}
