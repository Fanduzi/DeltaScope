// Package deltascope verifies the T06-A10 AUTO_INCREMENT contract at the public SDK seam.
// input: MySQL CREATE TABLE audits under the isolated pk-auto_increment and
// init-value policies
// output: a passing declaration and a policy reject both return ordinary audit
// results — never transport-level errors
// pos: public SDK contract test for the frozen T06-A10 oracle (issue #85)
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

// t06A10SDKPolicy writes an isolated policy enabling exactly one rule.
func t06A10SDKPolicy(t *testing.T, ruleID string, params string) string {
	t.Helper()
	if _, ok := catalog.Lookup(ruleID); !ok {
		t.Fatalf("missing catalog rule %s", ruleID)
	}
	var text strings.Builder
	text.WriteString("rules:\n")
	for _, entry := range catalog.All() {
		fmt.Fprintf(&text, "  %q:\n    enabled: %t\n", entry.RuleID, entry.RuleID == ruleID)
		if entry.RuleID == ruleID {
			text.WriteString("    level: blocker\n")
			if params != "" {
				text.WriteString("    params:\n" + params)
			}
		}
	}
	path := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(path, []byte(text.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestT06A10PrimaryKeyAutoIncrementSDK runs the pk-auto declaration through
// the isolated pk policy — a clean pass result.
func TestT06A10PrimaryKeyAutoIncrementSDK(t *testing.T) {
	policy := t06A10SDKPolicy(t, "ddl.table.primary_key.auto_increment.require", "      required: true\n")
	result, err := Audit(context.Background(), Request{
		SQL:        "CREATE TABLE t (id BIGINT AUTO_INCREMENT PRIMARY KEY);",
		Dialect:    DialectMySQL,
		Schema:     "golden",
		ConfigPath: policy,
	})
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
	if result.Verdict != VerdictPass || result.Coverage.Status != CoverageComplete {
		t.Fatalf("aggregate = %s/%s, want pass/complete", result.Verdict, result.Coverage.Status)
	}
	if len(result.Statements) != 1 || len(result.Statements[0].Findings) != 0 {
		t.Fatalf("statements = %+v, want one statement with no findings", result.Statements)
	}
}

// TestT06A10InitValueMismatchSDK runs init-mismatch under the value=8 init
// policy — a policy reject stays an ordinary audit result carrying exactly the
// init-value blocker.
func TestT06A10InitValueMismatchSDK(t *testing.T) {
	policy := t06A10SDKPolicy(t, "ddl.table.auto_increment.init_value.require", "      value: 8\n")
	result, err := Audit(context.Background(), Request{
		SQL:        "CREATE TABLE t (id BIGINT AUTO_INCREMENT PRIMARY KEY) AUTO_INCREMENT=9;",
		Dialect:    DialectMySQL,
		Schema:     "golden",
		ConfigPath: policy,
	})
	if err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
	if result.Verdict != VerdictReject || result.Coverage.Status != CoverageComplete {
		t.Fatalf("aggregate = %s/%s, want reject/complete", result.Verdict, result.Coverage.Status)
	}
	statement := result.Statements[0]
	if len(statement.Findings) != 1 {
		t.Fatalf("findings = %+v, want exactly one", statement.Findings)
	}
	finding := statement.Findings[0]
	if finding.RuleID != "ddl.table.auto_increment.init_value.require" || finding.Level != LevelBlocker ||
		finding.Message != "table auto_increment init value must be 8" ||
		finding.Metadata["table"] != "t" || finding.Metadata["required_value"] != 8 || finding.Metadata["actual_value"] != 9 {
		t.Fatalf("finding = %+v, want the init-value blocker on t", finding)
	}
	if result.Summary.Blockers != 1 {
		t.Fatalf("blockers = %d, want 1", result.Summary.Blockers)
	}
}
