// Package deltascope verifies the T06-A13 FLOAT/DOUBLE forbid policy at the
// public SDK seam.
// input: MySQL CREATE TABLE audits under the isolated float-double policy
// output: a two-finding policy reject bound to each declared column and a
// clean pass on the safe input — both ordinary audit results, never
// transport-level errors
// pos: public SDK contract test for the frozen T06-A13 oracle (issue #85)
// note: if this file changes, update this header and module README.md.
package deltascope

import (
	"context"
	"testing"
)

// TestT06A13FloatDoubleRejectSDK runs the mixed declaration under the
// isolated float-double blocker policy — a policy reject stays an ordinary
// audit result carrying exactly the two column-bound findings.
func TestT06A13FloatDoubleRejectSDK(t *testing.T) {
	policy := t06A10SDKPolicy(t, "ddl.column.float_double.forbid",
		"      forbid: true\n")
	result, err := Audit(context.Background(), Request{
		SQL:        "CREATE TABLE t (c_blob BLOB, c_text TEXT, c_json JSON, c_bit BIT(1), c_float FLOAT, c_double DOUBLE, c_timestamp TIMESTAMP NULL, c_datetime DATETIME NULL, c_int INT);",
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
	findings := result.Statements[0].Findings
	if len(result.Statements) != 1 || len(findings) != 2 {
		t.Fatalf("statements = %+v, want one statement with two findings", result.Statements)
	}
	wants := []struct{ column, typ string }{{"c_float", "float"}, {"c_double", "double"}}
	for i, want := range wants {
		finding := findings[i]
		if finding.RuleID != "ddl.column.float_double.forbid" ||
			finding.Level != LevelBlocker ||
			finding.Message != `column "`+want.column+`" should not use `+want.typ ||
			finding.Metadata["column"] != want.column || finding.Metadata["type"] != want.typ {
			t.Fatalf("finding[%d] = %+v, want %s %s blocker", i, finding, want.column, want.typ)
		}
	}
}

// TestT06A13FloatDoubleSafeSDK runs the non-matching declaration under the
// same isolated policy — a clean pass result.
func TestT06A13FloatDoubleSafeSDK(t *testing.T) {
	policy := t06A10SDKPolicy(t, "ddl.column.float_double.forbid",
		"      forbid: true\n")
	result, err := Audit(context.Background(), Request{
		SQL:        "CREATE TABLE t (c_int INT, c_string VARCHAR(8), c_datetime DATETIME NULL);",
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
