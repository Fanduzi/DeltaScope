// Package deltascope verifies the T06-A17 type-fidelity change stays invisible
// at the public SDK seam: case-bearing ENUM members and explicit DECIMAL
// parameters audit as one ordinary DDL statement with the usual result and
// error channels. The public contract never exposed Column.Type, so this test
// proves transport identity only — field fidelity is pinned at the parser,
// provider, and state seams.
package deltascope

import (
	"context"
	"testing"
)

// TestT06A17TypeFactsSDKTransport runs the fidelity fixture through the public
// Audit seam — the internal Type upgrade must not change statement identity,
// verdict, coverage, or the error channel.
func TestT06A17TypeFactsSDKTransport(t *testing.T) {
	// One loaded-but-non-firing rule keeps the request on the ordinary audit
	// path without turning the pass result into a policy signal.
	policy := t06A10SDKPolicy(t, "ddl.column.float_double.forbid", "      forbid: false\n")
	result, err := Audit(context.Background(), Request{
		SQL: "CREATE TABLE t (\n" +
			"  d DECIMAL(12,2),\n" +
			"  e ENUM('Ready','Closed'),\n" +
			"  s SET('Write','Read'),\n" +
			"  f FLOAT(10,0)\n" +
			");",
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
	if len(result.Statements) != 1 || result.Statements[0].Index != 0 || result.Statements[0].Kind != "ddl" {
		t.Fatalf("statements = %+v, want one ddl statement at index 0", result.Statements)
	}
	if len(result.Statements[0].Findings) != 0 {
		t.Fatalf("findings = %+v, want none under all-off policy", result.Statements[0].Findings)
	}
}
