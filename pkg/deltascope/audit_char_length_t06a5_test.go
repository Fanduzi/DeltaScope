// Package deltascope verifies the T06-A5 declared-length policy contract at
// the public SDK seam — one under-limit and one over-limit representative;
// the full 16-cell matrix lives in the shared audit layer and the Golden run.
// input: CREATE TABLE statements with declared VARCHAR lengths through Audit
// output: pass for the under-limit control; reject with one pinned blocker for the over-limit case
// pos: public SDK contract test for issue #85 T06-A5
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule/catalog"
	"os"
)

// t06A5SDKPolicy writes an isolated single-rule policy (limit=8, blocker) so
// the representative binds the exact rule under test.
func t06A5SDKPolicy(t *testing.T, ruleID string) string {
	t.Helper()
	var text strings.Builder
	text.WriteString("rules:\n")
	for _, entry := range catalog.All() {
		if entry.RuleID == ruleID {
			fmt.Fprintf(&text, "  %q:\n    enabled: true\n    level: blocker\n    params:\n      limit: 8\n", entry.RuleID)
		} else {
			fmt.Fprintf(&text, "  %q:\n    enabled: false\n", entry.RuleID)
		}
	}
	path := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(path, []byte(text.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func t06A5Audit(t *testing.T, sql, ruleID string) Result {
	t.Helper()
	result, err := Audit(context.Background(), Request{
		SQL: sql, Dialect: DialectMySQL, ConfigPath: t06A5SDKPolicy(t, ruleID),
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	return result
}

// TestT06A5VarcharLengthSDK is the under-limit representative: VARCHAR(8)
// against limit=8 stays pass with zero findings.
func TestT06A5VarcharLengthSDK(t *testing.T) {
	result := t06A5Audit(t, "CREATE TABLE t (c VARCHAR(8));", "ddl.column.varchar.max_length")
	if result.Verdict != VerdictPass || result.Coverage.Status != "complete" {
		t.Fatalf("aggregate = %s/%s, want pass/complete", result.Verdict, result.Coverage.Status)
	}
	if len(result.Statements) != 1 || len(result.Statements[0].Findings) != 0 {
		t.Fatalf("statements = %+v, want one clean statement", result.Statements)
	}
}

// TestT06A5VarcharOverLimitSDK is the over-limit representative: VARCHAR(9)
// against limit=8 produces exactly the policy blocker — still a normal
// result value, not an SDK error.
func TestT06A5VarcharOverLimitSDK(t *testing.T) {
	result := t06A5Audit(t, "CREATE TABLE t (c VARCHAR(9));", "ddl.column.varchar.max_length")
	if result.Verdict != VerdictReject || result.Coverage.Status != "complete" {
		t.Fatalf("aggregate = %s/%s, want reject/complete", result.Verdict, result.Coverage.Status)
	}
	findings := result.Statements[0].Findings
	if len(findings) != 1 || findings[0].RuleID != "ddl.column.varchar.max_length" || findings[0].Level != "blocker" {
		t.Fatalf("findings = %#v, want one varchar.max_length blocker", findings)
	}
	if findings[0].Metadata["actual"] != 9 || findings[0].Metadata["limit"] != 8 {
		t.Fatalf("metadata = %#v, want actual=9 limit=8", findings[0].Metadata)
	}
}
