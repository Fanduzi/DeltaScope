// Package deltascope verifies the T06-A9 audit-column role contract at the
// public SDK seam — a complete pair passes and a table missing both roles
// returns an ordinary two-blocker reject result.
// input: CREATE TABLE statements with explicit DATETIME audit columns through Audit
// output: pass for the complete pair; reject with the created+updated blocker pair otherwise
// pos: public SDK contract test for issue #85 T06-A9
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"testing"
)

// TestT06A9AuditColumnPairSDK is the pass representative: explicit DATETIME
// columns carrying the declared facts satisfy both roles.
func TestT06A9AuditColumnPairSDK(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "CREATE TABLE t (id INT PRIMARY KEY, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP);",
		Dialect: DialectMySQL,
		ConfigPath: t06A8SDKPolicy(t, map[string]string{
			"ddl.table.audit_columns.require": "      required: true\n",
		}),
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if result.Verdict != VerdictPass || result.Coverage.Status != "complete" {
		t.Fatalf("aggregate = %s/%s, want pass/complete", result.Verdict, result.Coverage.Status)
	}
	if len(result.Statements[0].Findings) != 0 {
		t.Fatalf("findings = %#v, want none", result.Statements[0].Findings)
	}
}

// TestT06A9AuditColumnMissingBothSDK is the double-finding representative:
// the missing-both shape emits exactly one created blocker and one updated
// blocker — a policy reject stays an ordinary audit result, not an error.
func TestT06A9AuditColumnMissingBothSDK(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "CREATE TABLE t (id INT PRIMARY KEY);",
		Dialect: DialectMySQL,
		ConfigPath: t06A8SDKPolicy(t, map[string]string{
			"ddl.table.audit_columns.require": "      required: true\n",
		}),
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if result.Verdict != VerdictReject || result.Coverage.Status != "complete" {
		t.Fatalf("aggregate = %s/%s, want reject/complete", result.Verdict, result.Coverage.Status)
	}
	findings := result.Statements[0].Findings
	if len(findings) != 2 {
		t.Fatalf("findings = %#v, want exactly 2 (created+updated)", findings)
	}
	kinds := map[string]string{}
	for _, f := range findings {
		if f.RuleID != "ddl.table.audit_columns.require" || f.Level != LevelBlocker {
			t.Fatalf("finding = %#v, want audit-columns blocker", f)
		}
		kind, _ := f.Metadata["kind"].(string)
		if _, dup := kinds[kind]; dup {
			t.Fatalf("duplicate role finding %q in %#v", kind, findings)
		}
		kinds[kind] = f.Message
	}
	if kinds["created"] != "table should include a created-time audit column with DEFAULT CURRENT_TIMESTAMP" ||
		kinds["updated"] != "table should include an updated-time audit column with DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP" {
		t.Fatalf("role findings = %#v, want created+updated pair", findings)
	}
}
