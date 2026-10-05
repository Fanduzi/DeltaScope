// Package deltascope verifies the T06-A1 primary-key-presence contract at the public SDK seam.
// input: MySQL and TiDB CREATE TABLE audits with the isolated ddl.table.primary_key.require policy
// output: a reject verdict carrying exactly one blocker finding, or a clean pass when the rule is off
// pos: public SDK contract test for the frozen T06-A1 oracle (issue #85)
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule/catalog"
)

func t06A1SDKPolicy(t *testing.T, required bool) string {
	t.Helper()
	if _, ok := catalog.Lookup("ddl.table.primary_key.require"); !ok {
		t.Fatal("missing catalog rule ddl.table.primary_key.require")
	}
	var text strings.Builder
	text.WriteString("rules:\n")
	for _, entry := range catalog.All() {
		fmt.Fprintf(&text, "  %q:\n    enabled: %t\n", entry.RuleID,
			entry.RuleID == "ddl.table.primary_key.require")
		if entry.RuleID == "ddl.table.primary_key.require" {
			text.WriteString("    level: blocker\n")
			fmt.Fprintf(&text, "    params:\n      required: %t\n", required)
		}
	}
	path := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(path, []byte(text.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestT06A1PrimaryKeyPresenceSDK(t *testing.T) {
	cases := []struct {
		name    string
		dialect Dialect
		sql     string
		verdict Verdict
	}{
		{"mysql_no_pk", DialectMySQL, "CREATE TABLE t (id INT);", VerdictReject},
		{"tidb_no_pk", DialectTiDB, "CREATE TABLE t (id INT);", VerdictReject},
		{"mysql_table_pk", DialectMySQL, "CREATE TABLE t (id INT, PRIMARY KEY (id));", VerdictPass},
	}
	policy := t06A1SDKPolicy(t, true)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Audit(context.Background(), Request{
				SQL: tc.sql, Dialect: tc.dialect, Schema: "golden", ConfigPath: policy,
			})
			if err != nil {
				t.Fatalf("error = %v, want nil", err)
			}
			if result.Verdict != tc.verdict || result.Coverage.Status != CoverageComplete {
				t.Fatalf("aggregate = %s/%s, want %s/complete", result.Verdict, result.Coverage.Status, tc.verdict)
			}
			if len(result.Statements) != 1 {
				t.Fatalf("statements = %d, want 1", len(result.Statements))
			}
			statement := result.Statements[0]
			if statement.Index != 0 || statement.Kind != "ddl" || statement.RawSQL != tc.sql ||
				statement.NormalizedSQL != strings.TrimSuffix(tc.sql, ";") || statement.Impact != nil {
				t.Fatalf("statement identity = %+v", statement)
			}
			if statement.Coverage.Status != CoverageComplete || len(statement.EvidenceGaps) != 0 {
				t.Fatalf("statement coverage/gaps = %+v", statement)
			}
			if tc.verdict == VerdictReject {
				if len(statement.Findings) != 1 {
					t.Fatalf("findings = %+v, want exactly one", statement.Findings)
				}
				finding := statement.Findings[0]
				if finding.RuleID != "ddl.table.primary_key.require" || finding.Level != LevelBlocker ||
					finding.Message != "primary key is required" ||
					finding.Metadata["table"] != "t" {
					t.Fatalf("finding = %+v, want the primary-key blocker on t", finding)
				}
				if result.Summary.Blockers != 1 || result.Summary.Warnings != 0 || result.Summary.Notices != 0 {
					t.Fatalf("summary = %+v, want exactly one blocker", result.Summary)
				}
			} else if len(statement.Findings) != 0 {
				t.Fatalf("findings = %+v, want none", statement.Findings)
			}
			if len(result.Unsupported) != 0 || len(result.GlobalFindings) != 0 || len(result.Diagnostics) != 0 {
				t.Fatalf("unexpected channels: unsupported=%+v global=%+v diagnostics=%+v",
					result.Unsupported, result.GlobalFindings, result.Diagnostics)
			}
		})
	}
}
