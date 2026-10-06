// Package deltascope verifies the T06-A2 primary-key member nullability contract at the public SDK seam.
// input: MySQL and TiDB CREATE TABLE audits with the isolated ddl.table.primary_key.not_null.require policy
// output: legal PK members pass, while explicit-NULL member declarations keep one policy blocker
// pos: public SDK contract test for the frozen T06-A2 oracle (issue #85)
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

func t06A2SDKPolicy(t *testing.T) string {
	t.Helper()
	const ruleID = "ddl.table.primary_key.not_null.require"
	if _, ok := catalog.Lookup(ruleID); !ok {
		t.Fatalf("missing catalog rule %s", ruleID)
	}
	var text strings.Builder
	text.WriteString("rules:\n")
	for _, entry := range catalog.All() {
		fmt.Fprintf(&text, "  %q:\n    enabled: %t\n", entry.RuleID, entry.RuleID == ruleID)
		if entry.RuleID == ruleID {
			text.WriteString("    level: blocker\n")
			text.WriteString("    params:\n      required: true\n")
		}
	}
	path := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(path, []byte(text.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestT06A2PrimaryKeyNullabilitySDK(t *testing.T) {
	cases := []struct {
		name    string
		dialect Dialect
		sql     string
		verdict Verdict
	}{
		{"mysql_table_pk", DialectMySQL, "CREATE TABLE t (id INT, PRIMARY KEY (id));", VerdictPass},
		{"tidb_table_pk", DialectTiDB, "CREATE TABLE t (id INT, PRIMARY KEY (id));", VerdictPass},
		{"mysql_explicit_null_table", DialectMySQL, "CREATE TABLE t (id INT NULL, PRIMARY KEY (id));", VerdictReject},
		{"tidb_explicit_null_inline", DialectTiDB, "CREATE TABLE t (id INT NULL PRIMARY KEY);", VerdictReject},
	}
	policy := t06A2SDKPolicy(t)
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
			if statement.Index != 0 || statement.Kind != "ddl" || statement.RawSQL != tc.sql {
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
				if finding.RuleID != "ddl.table.primary_key.not_null.require" || finding.Level != LevelBlocker ||
					finding.Message != `primary key column "id" must be NOT NULL` ||
					finding.Metadata["table"] != "t" || finding.Metadata["column"] != "id" {
					t.Fatalf("finding = %+v, want the primary-key not-null blocker on t.id", finding)
				}
				if result.Summary.Blockers != 1 {
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
