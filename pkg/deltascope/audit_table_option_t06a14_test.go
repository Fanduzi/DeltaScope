// Package deltascope verifies the T06-A14 table ENGINE allowlist policy at
// the public SDK seam.
package deltascope

import (
	"context"
	"testing"
)

// TestT06A14EngineDeniedOptionalRejectSDK runs the explicit disallowed
// engine under the isolated optional profile — require_explicit=false
// exempts omission only, so the declared MyISAM still rejects with the
// complete finding identity.
func TestT06A14EngineDeniedOptionalRejectSDK(t *testing.T) {
	policy := t06A10SDKPolicy(t, "ddl.table.engine.allowlist",
		"      values: [\"InnoDB\"]\n      require_explicit: false\n")
	result, err := Audit(context.Background(), Request{
		SQL:        "CREATE TABLE t (c INT) ENGINE=MyISAM;",
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
	if finding.RuleID != "ddl.table.engine.allowlist" ||
		finding.Level != LevelBlocker ||
		finding.Message != "table engine must be one of [InnoDB]" ||
		finding.Metadata["option"] != "engine" ||
		finding.Metadata["actual"] != "MyISAM" {
		t.Fatalf("finding = %+v, want the engine allowlist blocker with actual=MyISAM", finding)
	}
}

// TestT06A14EngineOmittedOptionalPassSDK runs the option-less declaration
// under the same isolated optional profile — the omitted declaration stays
// applicable but produces no finding.
func TestT06A14EngineOmittedOptionalPassSDK(t *testing.T) {
	policy := t06A10SDKPolicy(t, "ddl.table.engine.allowlist",
		"      values: [\"InnoDB\"]\n      require_explicit: false\n")
	result, err := Audit(context.Background(), Request{
		SQL:        "CREATE TABLE t (c INT);",
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
