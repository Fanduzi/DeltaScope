// Package mcpapi verifies the T05-A3 ordered DROP contract at the MCP seam.
// input: offline five-rule audit_sql tool calls covering the drop-recreate
// path and isolated DROP forms
// output: isError=false structured results pinning the first-statement gap,
// complete derived followers, and the bounded unknown_table_state drop gap
// pos: MCP transport contract tests for the ordered-state drop transition
// note: if this file changes, update this header and module README.md.
package mcpapi

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
)

func t05MCPFiveRulePolicy(t *testing.T) string {
	t.Helper()
	var builder strings.Builder
	builder.WriteString("rules:\n")
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
	path := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatalf("write policy: %v", err)
	}
	return path
}

// The offline drop-recreate first path returns isError=false with only the
// leading CREATE's unknown-existence gap; the DROP and every rebuilt follower
// stay complete.
func TestAuditSQLT05A3DropRecreateFirstPath(t *testing.T) {
	t.Parallel()
	result := callAuditSQL(t, map[string]any{
		"sql":         "CREATE TABLE t (old_c INT PRIMARY KEY); DROP TABLE t; CREATE TABLE t (id INT PRIMARY KEY); ALTER TABLE t ADD COLUMN c INT; CREATE INDEX idx_c ON t(c);",
		"dialect":     "mysql",
		"config_path": t05MCPFiveRulePolicy(t),
	})
	if result.IsError {
		t.Fatalf("expected isError=false, got %+v", result)
	}
	body := requireAuditStructuredMap(t, result)
	if body["verdict"] != "review" {
		t.Fatalf("expected review verdict, got %#v", body["verdict"])
	}
	statements, ok := body["statements"].([]any)
	if !ok || len(statements) != 5 {
		t.Fatalf("expected 5 statements, got %#v", body["statements"])
	}
	stmt0 := statements[0].(map[string]any)
	if c, _ := stmt0["coverage"].(map[string]any); c["status"] != "unverified" {
		t.Fatalf("statement 0 coverage must be unverified, got %#v", stmt0["coverage"])
	}
	gaps, _ := stmt0["evidence_gaps"].([]any)
	if len(gaps) != 1 {
		t.Fatalf("statement 0 must carry exactly one gap, got %#v", stmt0["evidence_gaps"])
	}
	gap := gaps[0].(map[string]any)
	if gap["rule_id"] != "ddl.table.exists.create.forbid" || gap["reason_code"] != "unknown_table_state" {
		t.Fatalf("statement 0 gap = %#v, want create.forbid/unknown_table_state", gap)
	}
	for i := 1; i <= 4; i++ {
		stmt := statements[i].(map[string]any)
		if c, _ := stmt["coverage"].(map[string]any); c["status"] != "complete" {
			t.Fatalf("statement %d coverage must be complete via derived state, got %#v", i, stmt["coverage"])
		}
		if gaps, ok := stmt["evidence_gaps"].([]any); ok && len(gaps) != 0 {
			t.Fatalf("statement %d must have no gaps, got %#v", i, gaps)
		}
		if findings, ok := stmt["findings"].([]any); ok && len(findings) != 0 {
			t.Fatalf("statement %d must have no findings, got %#v", i, findings)
		}
	}
}

// An isolated DROP without a provider keeps its bounded existence gap instead
// of a missing-table finding.
func TestAuditSQLT05A3IsolatedDropGap(t *testing.T) {
	t.Parallel()
	for _, sql := range []string{"DROP TABLE t;", "DROP TABLE IF EXISTS t;"} {
		sql := sql
		t.Run(sql, func(t *testing.T) {
			t.Parallel()
			result := callAuditSQL(t, map[string]any{
				"sql":         sql,
				"dialect":     "mysql",
				"config_path": t05MCPFiveRulePolicy(t),
			})
			if result.IsError {
				t.Fatalf("expected isError=false, got %+v", result)
			}
			body := requireAuditStructuredMap(t, result)
			if body["verdict"] != "review" {
				t.Fatalf("expected review verdict, got %#v", body["verdict"])
			}
			statements, ok := body["statements"].([]any)
			if !ok || len(statements) != 1 {
				t.Fatalf("expected 1 statement, got %#v", body["statements"])
			}
			stmt := statements[0].(map[string]any)
			if c, _ := stmt["coverage"].(map[string]any); c["status"] != "unverified" {
				t.Fatalf("statement coverage must be unverified, got %#v", stmt["coverage"])
			}
			if findings, ok := stmt["findings"].([]any); ok && len(findings) != 0 {
				t.Fatalf("isolated drop must have no findings, got %#v", findings)
			}
			gaps, _ := stmt["evidence_gaps"].([]any)
			if len(gaps) != 1 {
				t.Fatalf("expected exactly one evidence gap, got %#v", stmt["evidence_gaps"])
			}
			gap := gaps[0].(map[string]any)
			if gap["rule_id"] != "ddl.table.drop.exists.require" || gap["reason_code"] != "unknown_table_state" {
				t.Fatalf("gap = %#v, want drop.exists.require/unknown_table_state", gap)
			}
			if facts, _ := gap["required_facts"].([]any); len(facts) != 1 || facts[0] != "target_table.existence" {
				t.Fatalf("expected required_facts [target_table.existence], got %#v", gap["required_facts"])
			}
		})
	}
}
