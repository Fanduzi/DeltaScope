// Package deltascope verifies the T06-A11 single-member primary-key
// BIGINT/UNSIGNED contract at the public SDK seam.
package deltascope

import (
	"context"
	"testing"
)

// TestT06A11UnsignedPassSDK runs the unsigned declaration through the
// isolated unsigned policy — a clean pass result.
func TestT06A11UnsignedPassSDK(t *testing.T) {
	policy := t06A10SDKPolicy(t, "ddl.table.primary_key.unsigned.require", "      required: true\n")
	result, err := Audit(context.Background(), Request{
		SQL:        "CREATE TABLE t (id BIGINT UNSIGNED PRIMARY KEY);",
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

// TestT06A11BigintRejectSDK runs the signed-INT declaration through the
// isolated bigint policy — a policy reject stays an ordinary audit result
// carrying exactly the bigint blocker.
func TestT06A11BigintRejectSDK(t *testing.T) {
	policy := t06A10SDKPolicy(t, "ddl.table.primary_key.bigint.require", "      required: true\n")
	result, err := Audit(context.Background(), Request{
		SQL:        "CREATE TABLE t (id INT PRIMARY KEY);",
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
	if finding.RuleID != "ddl.table.primary_key.bigint.require" ||
		finding.Level != LevelBlocker ||
		finding.Message != `primary key column "id" must use bigint` {
		t.Fatalf("finding = %+v, want the bigint blocker", finding)
	}
	if finding.Metadata["table"] != "t" || finding.Metadata["column"] != "id" ||
		finding.Metadata["type"] != "int(11)" {
		t.Fatalf("metadata = %+v, want table=t column=id type=int(11)", finding.Metadata)
	}
}
