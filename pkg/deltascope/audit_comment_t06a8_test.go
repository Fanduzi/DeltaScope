// Package deltascope verifies the T06-A8 comment fidelity upgrade at the
// public SDK seam — an explicitly empty column COMMENT is a policy finding,
// and a multibyte table comment is measured in code points.
// input: CREATE TABLE statements with declared comments through Audit
// output: reject with one pinned blocker for empty column comment, pass for the 3-code-point comment under limit 8
// pos: public SDK contract test for issue #85 T06-A8
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

// t06A8SDKPolicy writes a policy enabling the named rules (or none) so each
// representative binds its exact profile.
func t06A8SDKPolicy(t *testing.T, enabled map[string]string) string {
	t.Helper()
	var text strings.Builder
	text.WriteString("rules:\n")
	for _, entry := range catalog.All() {
		if params, keep := enabled[entry.RuleID]; keep {
			fmt.Fprintf(&text, "  %q:\n    enabled: true\n    level: blocker\n", entry.RuleID)
			if params != "" {
				text.WriteString("    params:\n" + params)
			}
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

// TestT06A8ColumnEmptyCommentSDK is the field-fidelity representative: an
// explicitly empty COMMENT literal is a missing comment — quote wrappers must
// not fake presence.
func TestT06A8ColumnEmptyCommentSDK(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "CREATE TABLE t (c INT COMMENT '');",
		Dialect: DialectMySQL,
		ConfigPath: t06A8SDKPolicy(t, map[string]string{
			"ddl.column.comment.require": "      required: true\n",
		}),
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if result.Verdict != VerdictReject || result.Coverage.Status != "complete" {
		t.Fatalf("aggregate = %s/%s, want reject/complete", result.Verdict, result.Coverage.Status)
	}
	statements := result.Statements
	if len(statements) != 1 || len(statements[0].Findings) != 1 {
		t.Fatalf("statements/findings = %#v, want exactly one finding", statements)
	}
	finding := statements[0].Findings[0]
	if finding.RuleID != "ddl.column.comment.require" || finding.Message != `column "c" must include a comment` {
		t.Fatalf("finding = %#v, want column comment require", finding)
	}
}

// TestT06A8TableCommentRuneLengthSDK is the unit representative: '中文注' is
// three code points — under limit=8 it passes where a byte count would fail.
func TestT06A8TableCommentRuneLengthSDK(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "CREATE TABLE t (c INT) COMMENT='中文注';",
		Dialect: DialectMySQL,
		ConfigPath: t06A8SDKPolicy(t, map[string]string{
			"ddl.table.comment.max_length": "      limit: 8\n",
		}),
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if result.Verdict != VerdictPass || result.Coverage.Status != "complete" {
		t.Fatalf("aggregate = %s/%s, want pass/complete", result.Verdict, result.Coverage.Status)
	}
	if len(result.Statements[0].Findings) != 0 {
		t.Fatalf("findings = %#v, want none (3 code points <= 8)", result.Statements[0].Findings)
	}
}
