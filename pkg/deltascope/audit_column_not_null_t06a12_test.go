// Package deltascope verifies the T06-A12 ordinary CREATE TABLE non-PK-column
// NOT NULL contract at the public SDK seam.
// input: MySQL CREATE TABLE audits under the isolated time-allow policy
// output: a passing time-exempt declaration and a policy reject bound to the
// owning column both return ordinary audit results — never transport-level
// errors
// pos: public SDK contract test for the frozen T06-A12 oracle (issue #85)
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"testing"
)

// TestT06A12TimeExemptSDK runs the DATETIME NULL declaration under the
// isolated time-allow policy — a clean pass result.
func TestT06A12TimeExemptSDK(t *testing.T) {
	policy := t06A10SDKPolicy(t, "ddl.column.not_null.require",
		"      required: true\n      allow_time_null: true\n")
	result, err := Audit(context.Background(), Request{
		SQL:        "CREATE TABLE t (c DATETIME NULL);",
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

// TestT06A12MixedRejectSDK runs the three-column declaration under the
// isolated time-allow policy — a policy reject stays an ordinary audit
// result carrying exactly the blocker bound to the ordinary column.
func TestT06A12MixedRejectSDK(t *testing.T) {
	policy := t06A10SDKPolicy(t, "ddl.column.not_null.require",
		"      required: true\n      allow_time_null: true\n")
	result, err := Audit(context.Background(), Request{
		SQL:        "CREATE TABLE t (ordinary INT, time_c DATETIME NULL, payload TEXT);",
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
	if len(result.Statements) != 1 || len(result.Statements[0].Findings) != 1 {
		t.Fatalf("statements = %+v, want one statement with one finding", result.Statements)
	}
	finding := result.Statements[0].Findings[0]
	if finding.RuleID != "ddl.column.not_null.require" ||
		finding.Level != LevelBlocker ||
		finding.Message != `column "ordinary" should be declared NOT NULL` ||
		finding.Metadata["column"] != "ordinary" {
		t.Fatalf("finding = %+v, want the not_null blocker bound to ordinary", finding)
	}
}
