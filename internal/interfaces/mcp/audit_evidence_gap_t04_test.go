// Package mcpapi verifies the T04-A/#83 evidence-gap MCP contract.
// input: audit_sql tool calls under the isolated modify-column compatibility policy
// output: isError=false results carrying evidence_gaps and preserved isError=true unsupported semantics
// pos: MCP transport contract tests for metadata evidence gaps (issue #83 T04-A slice)
// note: if this file changes, update this header and module README.md.
package mcpapi

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
)

const t04TargetRuleID = "ddl.alter.modify_column.compatibility.require"

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

// A gap-only audit is a normal tool success: isError=false with the shared
// result carrying coverage=unverified, verdict=review, and evidence_gaps.
func TestAuditSQLT04EvidenceGapIsSuccess(t *testing.T) {
	t.Parallel()

	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test-version"}))
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql":         "ALTER TABLE t MODIFY COLUMN c VARCHAR(20);",
			"dialect":     "mysql",
			"config_path": writeT04IsolatedPolicy(t),
		},
	})
	if err != nil {
		t.Fatalf("expected tool success, got protocol error: %v", err)
	}
	if result.IsError {
		t.Fatal("expected isError=false for gap-only audit")
	}
	body := requireAuditStructuredMap(t, result)
	if body["verdict"] != "review" {
		t.Fatalf("structured verdict = %#v, want review", body["verdict"])
	}
	if c, _ := body["coverage"].(map[string]any); c["status"] != "unverified" {
		t.Fatalf("expected aggregate coverage unverified, got %#v", body["coverage"])
	}
	stmts, _ := body["statements"].([]any)
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %#v", body["statements"])
	}
	first, _ := stmts[0].(map[string]any)
	if c, _ := first["coverage"].(map[string]any); c["status"] != "unverified" {
		t.Fatalf("expected statement coverage unverified, got %#v", first["coverage"])
	}
	gaps, _ := first["evidence_gaps"].([]any)
	if len(gaps) != 1 {
		t.Fatalf("expected exactly one evidence gap, got %#v", gaps)
	}
	gap, _ := gaps[0].(map[string]any)
	if gap["rule_id"] != t04TargetRuleID || gap["reason_code"] != "missing_source_column" {
		t.Fatalf("gap = %#v, want %q/missing_source_column", gap, t04TargetRuleID)
	}
	if facts, _ := gap["required_facts"].([]any); len(facts) != 1 || facts[0] != "source_column.definition" {
		t.Fatalf("expected required_facts [source_column.definition], got %#v", gap["required_facts"])
	}
	// context.unproven stays a transport-context field; it is never copied into
	// rule evidence gaps.
	ctx, _ := body["context"].(map[string]any)
	if unproven, _ := ctx["unproven"].([]any); len(unproven) == 0 {
		t.Fatalf("expected context.unproven preserved, got %#v", ctx)
	}
}

// A gap plus a recognized-but-unsupported statement keeps isError=true with the
// partial result — evidence gaps never change the existing error boundary.
func TestAuditSQLT04GapWithUnsupportedStaysError(t *testing.T) {
	t.Parallel()

	session, err := connectClientSession(context.Background(), NewServer(Config{Version: "test-version"}))
	if err != nil {
		t.Fatalf("connect session: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "audit_sql",
		Arguments: map[string]any{
			"sql":         "ALTER TABLE t MODIFY COLUMN c VARCHAR(20); CREATE SEQUENCE golden_seq START WITH 1;",
			"dialect":     "mysql",
			"config_path": writeT04IsolatedPolicy(t),
		},
	})
	if err != nil {
		t.Fatalf("expected tool error result, got protocol error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected isError=true for unsupported mix")
	}
	body := requireAuditStructuredMap(t, result)
	if c, _ := body["coverage"].(map[string]any); c["status"] != "incomplete" {
		t.Fatalf("expected aggregate incomplete, got %#v", body["coverage"])
	}
	stmts, _ := body["statements"].([]any)
	if len(stmts) != 2 {
		t.Fatalf("expected 2 retained statements, got %#v", body["statements"])
	}
	first, _ := stmts[0].(map[string]any)
	if gaps, _ := first["evidence_gaps"].([]any); len(gaps) != 1 {
		t.Fatalf("expected evidence gap retained on first statement, got %#v", gaps)
	}
	unsupported, _ := body["unsupported"].([]any)
	if len(unsupported) != 1 {
		t.Fatalf("expected 1 unsupported detail, got %#v", body["unsupported"])
	}
}
