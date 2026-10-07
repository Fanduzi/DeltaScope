// Package audit verifies the T06-A5 declared-length policy contract through
// the shared parse→enrich→rule path: the 7/8/9/disabled threshold matrix on
// both dialects, per-statement/per-column finding attribution, real-parser
// type controls, and the pinned consumption of the same Length fact by the
// CREATE-derived pre-state and the MODIFY compatibility shrink check.
// input: CREATE TABLE batches with declared CHAR/VARCHAR lengths through AuditSQL plus the enrichment seam
// output: exact blocker findings at the boundary, silent non-matches, and preserved Length facts in derived state
// pos: application-layer contract tests for the T06-A5 length-policy oracle (issue #85)
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"reflect"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const (
	t06A5CharRule    = "ddl.column.char.max_length"
	t06A5VarcharRule = "ddl.column.varchar.max_length"
	t06A5ModifyRule  = "ddl.alter.modify_column.compatibility.require"
)

// t06A5LengthPolicy enables exactly one length rule at blocker/limit=8 so a
// finding can only come from the rule under test.
func t06A5LengthPolicy(t *testing.T, ruleID string) string {
	t.Helper()
	return t05PolicyPath(t, map[string]string{ruleID: "      limit: 8\n"})
}

func t06A5Audit(t *testing.T, dialect spec.Dialect, sql, policy string) report.Result {
	t.Helper()
	result, err := AuditSQL(context.Background(), Request{
		SQL: sql, Dialect: dialect, Schema: "golden", ConfigPath: policy,
	})
	if err != nil {
		t.Fatalf("%s: unexpected error %v", dialect, err)
	}
	return result
}

// t06A5WantLengthFinding pins the exact finding contract of the frozen A5
// oracle: one blocker bound to the owning statement, table/column/limit/
// actual metadata, and the type-spelled message.
func t06A5WantLengthFinding(t *testing.T, result report.Result, ruleID, typeName string) {
	t.Helper()
	if result.Verdict != report.VerdictReject || result.Coverage.Status != report.CoverageComplete {
		t.Fatalf("aggregate = %s/%s, want reject/complete", result.Verdict, result.Coverage.Status)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("statements = %d, want 1", len(result.Statements))
	}
	statement := result.Statements[0]
	if statement.Index != 0 || statement.Kind != "ddl" {
		t.Fatalf("statement identity = index %d kind %s, want 0/ddl", statement.Index, statement.Kind)
	}
	if statement.Coverage.Status != report.CoverageComplete || len(statement.EvidenceGaps) != 0 {
		t.Fatalf("statement = %s gaps %#v, want complete/no gaps", statement.Coverage.Status, statement.EvidenceGaps)
	}
	if len(statement.Findings) != 1 {
		t.Fatalf("findings = %#v, want exactly 1", statement.Findings)
	}
	finding := statement.Findings[0]
	if finding.RuleID != ruleID || finding.Level != "blocker" {
		t.Fatalf("finding = %s/%s, want %s blocker", finding.RuleID, finding.Level, ruleID)
	}
	wantMessage := typeName + ` column "c" must not exceed 8 characters`
	if finding.Message != wantMessage {
		t.Fatalf("message = %q, want %q", finding.Message, wantMessage)
	}
	wantMeta := map[string]any{"table": "t", "column": "c", "limit": 8, "actual": 9}
	if !reflect.DeepEqual(finding.Metadata, wantMeta) {
		t.Fatalf("metadata = %#v, want %#v", finding.Metadata, wantMeta)
	}
	if finding.Location == nil || finding.Location.Line != 1 || finding.Location.Column != 1 {
		t.Fatalf("location = %+v, want 1:1", finding.Location)
	}
}

// TestT06A5LengthThresholdMatrix is group 1: CHAR/VARCHAR × mysql/tidb ×
// {7,8,9 under the isolated rule, 9 with every rule off}. Findings may only
// come from the rule under test — the other length rule stays disabled.
func TestT06A5LengthThresholdMatrix(t *testing.T) {
	type shape struct {
		name    string
		ruleID  string
		keyword string
	}
	shapes := []shape{
		{"char", t06A5CharRule, "CHAR"},
		{"varchar", t06A5VarcharRule, "VARCHAR"},
	}
	for _, sh := range shapes {
		policy := t06A5LengthPolicy(t, sh.ruleID)
		for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
			for _, n := range []int{7, 8, 9} {
				sql := "CREATE TABLE t (c " + sh.keyword + "(" + string(rune('0'+n)) + "));"
				t.Run(sh.name+"/"+string(dialect)+"/n"+string(rune('0'+n)), func(t *testing.T) {
					result := t06A5Audit(t, dialect, sql, policy)
					if n <= 8 {
						if result.Verdict != report.VerdictPass || result.Coverage.Status != report.CoverageComplete {
							t.Fatalf("aggregate = %s/%s, want pass/complete", result.Verdict, result.Coverage.Status)
						}
						statement := result.Statements[0]
						if len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 {
							t.Fatalf("statement = findings %#v gaps %#v, want none", statement.Findings, statement.EvidenceGaps)
						}
						return
					}
					t06A5WantLengthFinding(t, result, sh.ruleID, sh.name)
				})
			}
			t.Run(sh.name+"/"+string(dialect)+"/off", func(t *testing.T) {
				result := t06A5Audit(t, dialect,
					"CREATE TABLE t (c "+sh.keyword+"(9));",
					t05PolicyPath(t, map[string]string{}))
				if result.Verdict != report.VerdictPass || result.Coverage.Status != report.CoverageComplete {
					t.Fatalf("off aggregate = %s/%s, want pass/complete", result.Verdict, result.Coverage.Status)
				}
				statement := result.Statements[0]
				if len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 {
					t.Fatalf("off statement = findings %#v gaps %#v, want none", statement.Findings, statement.EvidenceGaps)
				}
			})
		}
	}
}

// TestT06A5TypeAndStatementAttribution is group 2: a wrong-type or
// other-statement declaration must never borrow the target rule's finding —
// attribution stays on the exact column and statement that violates.
func TestT06A5TypeAndStatementAttribution(t *testing.T) {
	charPolicy := t06A5LengthPolicy(t, t06A5CharRule)
	varcharPolicy := t06A5LengthPolicy(t, t06A5VarcharRule)
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect)+"/cross_type", func(t *testing.T) {
			// The char rule must ignore VARCHAR(9); the varchar rule CHAR(9).
			result := t06A5Audit(t, dialect, "CREATE TABLE t (c VARCHAR(9));", charPolicy)
			if len(result.Statements[0].Findings) != 0 {
				t.Fatalf("char rule fired on VARCHAR: %#v", result.Statements[0].Findings)
			}
			result = t06A5Audit(t, dialect, "CREATE TABLE t (c CHAR(9));", varcharPolicy)
			if len(result.Statements[0].Findings) != 0 {
				t.Fatalf("varchar rule fired on CHAR: %#v", result.Statements[0].Findings)
			}
		})
		t.Run(string(dialect)+"/mixed_columns", func(t *testing.T) {
			// One statement mixing a violating char, an at-limit varchar, and a
			// non-string column: exactly one finding, bound to column "a".
			result := t06A5Audit(t, dialect,
				"CREATE TABLE t (a CHAR(9), b VARCHAR(8), d INT, e BINARY(64));", charPolicy)
			statement := result.Statements[0]
			if len(statement.Findings) != 1 || statement.Findings[0].Metadata["column"] != "a" {
				t.Fatalf("mixed findings = %#v, want one finding on a", statement.Findings)
			}
		})
		t.Run(string(dialect)+"/statement_owner", func(t *testing.T) {
			// Two statements: the violation belongs to statement 0; statement 1
			// stays clean so the aggregate finding count cannot hide misattribution.
			result := t06A5Audit(t, dialect,
				"CREATE TABLE t (c CHAR(9));\nCREATE TABLE u (c CHAR(8));", charPolicy)
			if len(result.Statements) != 2 {
				t.Fatalf("statements = %d, want 2", len(result.Statements))
			}
			if len(result.Statements[0].Findings) != 1 || result.Statements[0].Findings[0].Metadata["table"] != "t" {
				t.Fatalf("statement 0 findings = %#v, want one t-bound finding", result.Statements[0].Findings)
			}
			if len(result.Statements[1].Findings) != 0 || len(result.Statements[1].EvidenceGaps) != 0 {
				t.Fatalf("statement 1 = findings %#v gaps %#v, want clean", result.Statements[1].Findings, result.Statements[1].EvidenceGaps)
			}
		})
	}
}

// TestT06A5NonStringTypesSilent pins the real-parser control for the
// non-string types in scope: INT/TEXT/BINARY/VARBINARY declarations never
// reach either length rule's gate even at lengths beyond the limit.
func TestT06A5NonStringTypesSilent(t *testing.T) {
	sql := "CREATE TABLE t (a INT, b TEXT, c BINARY(64), d VARBINARY(64));"
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, ruleID := range []string{t06A5CharRule, t06A5VarcharRule} {
			t.Run(string(dialect)+"/"+ruleID, func(t *testing.T) {
				result := t06A5Audit(t, dialect, sql, t06A5LengthPolicy(t, ruleID))
				if result.Verdict != report.VerdictPass || len(result.Statements[0].Findings) != 0 {
					t.Fatalf("%s on non-strings: %s findings %#v, want silent pass",
						ruleID, result.Verdict, result.Statements[0].Findings)
				}
			})
		}
	}
}

// TestT06A5DerivedStatePreservesLength is group 5a: the CREATE-derived
// prospective state must carry the declared N so downstream consumers see the
// same fact the threshold rules just evaluated — no re-parse, no provider.
func TestT06A5DerivedStatePreservesLength(t *testing.T) {
	sql := "CREATE TABLE t (id INT PRIMARY KEY, c VARCHAR(9), keep INT);\n" +
		"ALTER TABLE t DROP COLUMN c;\n" +
		"CREATE INDEX idx_k ON t(keep);"
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			enriched := enrichA3(t, sql, dialect, &t05AbsentProvider{})
			preDrop := enriched[1].Metadata.TargetTable
			if preDrop == nil {
				t.Fatalf("pre-drop state missing")
			}
			c := preDrop.FindColumn("c")
			if c == nil || c.Length != 9 || c.Type != "varchar(9)" {
				t.Fatalf("derived c = %+v, want varchar(9) Length=9", c)
			}
		})
	}
}

// TestT06A5ModifyShrinkConsumesSameFact is group 5b: with only the existing
// MODIFY compatibility rule enabled (required:true), the same CREATE-derived
// Length drives the original shrink finding — source_length=9, target_length=7
// — while every other column attribute stays intact. The trailing paren in
// the task sketch is not part of the SQL: the audited statement is exactly
// ALTER TABLE t MODIFY COLUMN c VARCHAR(7);
func TestT06A5ModifyShrinkConsumesSameFact(t *testing.T) {
	sql := "CREATE TABLE t (c VARCHAR(9));\nALTER TABLE t MODIFY COLUMN c VARCHAR(7);"
	policy := t05PolicyPath(t, map[string]string{
		t06A5ModifyRule: "      required: true\n",
	})
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			result := t06A5Audit(t, dialect, sql, policy)
			if len(result.Statements) != 2 {
				t.Fatalf("statements = %d, want 2", len(result.Statements))
			}
			alter := result.Statements[1]
			if len(alter.Findings) != 1 {
				t.Fatalf("alter findings = %#v, want exactly the shrink finding", alter.Findings)
			}
			finding := alter.Findings[0]
			if finding.RuleID != t06A5ModifyRule {
				t.Fatalf("finding rule = %s, want %s", finding.RuleID, t06A5ModifyRule)
			}
			if finding.Metadata["source_length"] != 9 || finding.Metadata["target_length"] != 7 {
				t.Fatalf("shrink metadata = %#v, want source_length=9 target_length=7", finding.Metadata)
			}
			if len(alter.EvidenceGaps) != 0 {
				t.Fatalf("alter gaps = %#v, want none (derived state supplies the facts)", alter.EvidenceGaps)
			}
		})
	}
}
