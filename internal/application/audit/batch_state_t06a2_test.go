// Package audit verifies the T06-A2 CREATE TABLE primary-key member
// nullability contract through the shared audit path.
// input: single-statement CREATE variants and CREATE+ALTER batches through AuditSQL plus the enrichment seam
// output: PK member NotNull facts, preserved explicit-NULL conflicts, and unchanged default/compat behavior
// pos: application-layer contract tests for the frozen T06-A2 oracle (issue #85)
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const (
	t06A2PKNotNullRule = "ddl.table.primary_key.not_null.require"
	t06A2ColumnNNRule  = "ddl.column.not_null.require"
	t06A2DefaultRule   = "ddl.column.default.require"
	t06A2ModifyRule    = "ddl.alter.modify_column.compatibility.require"

	t06A2InlinePK     = "CREATE TABLE t (id INT PRIMARY KEY);"
	t06A2TablePK      = "CREATE TABLE t (id INT, PRIMARY KEY (id));"
	t06A2ExplicitNNPK = "CREATE TABLE t (id INT NOT NULL, PRIMARY KEY (id));"
	t06A2CompositePK  = "CREATE TABLE t (a INT, spare INT, b INT, PRIMARY KEY (b,a));"
	t06A2NullTablePK  = "CREATE TABLE t (id INT NULL, PRIMARY KEY (id));"
	t06A2NullInlinePK = "CREATE TABLE t (id INT NULL PRIMARY KEY);"
	t06A2UniqueKey    = "CREATE TABLE t (id INT, UNIQUE KEY u_id (id));"
	t06A2SecondaryKey = "CREATE TABLE t (id INT, KEY k_id (id));"
	t06A2NoPK         = "CREATE TABLE t (id INT);"
	t06A2NoDefault    = "CREATE TABLE t (c INT);"
	t06A2DefaultNull  = "CREATE TABLE t (c INT DEFAULT NULL);"
)

// t06A2Policy builds an isolated blocker policy holding only the listed rules.
func t06A2Policy(t *testing.T, required bool, ruleIDs ...string) string {
	t.Helper()
	enabled := make(map[string]string, len(ruleIDs))
	for _, id := range ruleIDs {
		enabled[id] = "      required: " + map[bool]string{true: "true", false: "false"}[required] + "\n"
	}
	return t05PolicyPath(t, enabled)
}

func t06A2Audit(t *testing.T, dialect spec.Dialect, sql, policy string) report.Result {
	t.Helper()
	result, err := AuditSQL(context.Background(), Request{
		SQL: sql, Dialect: dialect, Schema: "golden",
		ConfigPath: policy, MetadataProvider: &t05AbsentProvider{},
	})
	if err != nil {
		t.Fatalf("%s %q: unexpected error %v", dialect, sql, err)
	}
	if len(result.Statements) == 0 {
		t.Fatalf("%s %q: no statements projected", dialect, sql)
	}
	return result
}

// t06A2CheckPKNotNullFinding pins the preserved conflict finding identity for
// explicit-NULL primary-key members.
func t06A2CheckPKNotNullFinding(t *testing.T, statement report.StatementResult, table, column string) {
	t.Helper()
	if len(statement.Findings) != 1 {
		t.Fatalf("findings = %+v, want exactly the primary-key not-null blocker", statement.Findings)
	}
	finding := statement.Findings[0]
	locationOK := finding.Location != nil && finding.Location.Line == 1 && finding.Location.Column == 1
	wantMessage := "primary key column \"" + column + "\" must be NOT NULL"
	if finding.RuleID != t06A2PKNotNullRule || finding.Level != "blocker" ||
		finding.Message != wantMessage ||
		finding.StatementIndex != statement.Index || finding.StatementKind != "ddl" ||
		!locationOK || !reflect.DeepEqual(finding.Metadata, map[string]any{"table": table, "column": column}) {
		t.Fatalf("finding = %+v, want %s blocker on %s.%s at 1:1", finding, t06A2PKNotNullRule, table, column)
	}
}

// TestT06A2PKMemberNullabilityAudit pins the frozen audit oracle: omitted and
// explicit NOT NULL members pass, while explicit NULL members keep exactly one
// policy finding through the shared AuditSQL path.
func TestT06A2PKMemberNullabilityAudit(t *testing.T) {
	passCases := map[string]string{
		"inline":      t06A2InlinePK,
		"table":       t06A2TablePK,
		"explicit_nn": t06A2ExplicitNNPK,
		"composite":   t06A2CompositePK,
		"no_pk":       t06A2NoPK,
		"unique_key":  t06A2UniqueKey,
		"key":         t06A2SecondaryKey,
	}
	conflictCases := map[string]string{
		"explicit_null_table":  t06A2NullTablePK,
		"explicit_null_inline": t06A2NullInlinePK,
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			policy := t06A2Policy(t, true, t06A2PKNotNullRule)
			for name, sql := range passCases {
				result := t06A2Audit(t, dialect, sql, policy)
				if result.Verdict != report.VerdictPass || result.Coverage.Status != report.CoverageComplete {
					t.Fatalf("%s aggregate = %s/%s, want pass/complete", name, result.Verdict, result.Coverage.Status)
				}
				statement := result.Statements[0]
				t06A1CheckStatementIdentity(t, statement, 0, sql)
				if len(statement.Findings) != 0 {
					t.Fatalf("%s findings = %+v, want none", name, statement.Findings)
				}
			}
			for name, sql := range conflictCases {
				result := t06A2Audit(t, dialect, sql, policy)
				if result.Verdict != report.VerdictReject || result.Coverage.Status != report.CoverageComplete {
					t.Fatalf("%s aggregate = %s/%s, want reject/complete", name, result.Verdict, result.Coverage.Status)
				}
				statement := result.Statements[0]
				t06A1CheckStatementIdentity(t, statement, 0, sql)
				t06A2CheckPKNotNullFinding(t, statement, "t", "id")
				if len(result.Unsupported) != 0 || len(result.GlobalFindings) != 0 {
					t.Fatalf("%s unexpected channels: unsupported=%+v global=%+v",
						name, result.Unsupported, result.GlobalFindings)
				}
				if result.Summary.Blockers != 1 {
					t.Fatalf("%s summary = %+v, want exactly one blocker", name, result.Summary)
				}
			}

			optional := t06A2Policy(t, false, t06A2PKNotNullRule)
			disabled := t05PolicyPath(t, map[string]string{})
			for name, offPolicy := range map[string]string{"required-false": optional, "rule-off": disabled} {
				result := t06A2Audit(t, dialect, t06A2NullTablePK, offPolicy)
				statement := result.Statements[0]
				if result.Verdict != report.VerdictPass || len(statement.Findings) != 0 ||
					len(statement.EvidenceGaps) != 0 {
					t.Fatalf("%s = %s findings=%+v gaps=%+v, want a clean opt-out",
						name, result.Verdict, statement.Findings, statement.EvidenceGaps)
				}
			}
		})
	}
}

// TestT06A2ColumnNotNullConsumer pins that the shared NOT NULL consumer reads
// the normalized member facts: composite members stay silent and the real
// nullable spare keeps its single finding.
func TestT06A2ColumnNotNullConsumer(t *testing.T) {
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			policy := t06A2Policy(t, true, t06A2ColumnNNRule)

			result := t06A2Audit(t, dialect, t06A2CompositePK, policy)
			statement := result.Statements[0]
			if result.Verdict != report.VerdictReject || len(statement.Findings) != 1 {
				t.Fatalf("composite = %s findings=%+v, want exactly the spare finding",
					result.Verdict, statement.Findings)
			}
			finding := statement.Findings[0]
			if finding.RuleID != t06A2ColumnNNRule || finding.Metadata["column"] != "spare" {
				t.Fatalf("finding = %+v, want ddl.column.not_null.require on spare", finding)
			}

			result = t06A2Audit(t, dialect, t06A2TablePK, policy)
			if result.Verdict != report.VerdictPass || len(result.Statements[0].Findings) != 0 {
				t.Fatalf("table-pk = %s findings=%+v, want none — members carry implied NOT NULL",
					result.Verdict, result.Statements[0].Findings)
			}

			off := t06A2Audit(t, dialect, t06A2CompositePK, t05PolicyPath(t, map[string]string{}))
			if len(off.Statements[0].EvidenceGaps) != 0 || len(off.Statements[0].Findings) != 0 {
				t.Fatalf("disabled consumer = findings %+v gaps %+v, want neither",
					off.Statements[0].Findings, off.Statements[0].EvidenceGaps)
			}
		})
	}
}

// TestT06A2ModifyColumnPreStateConsumption pins that the normalized member
// fact reaches the next statement's pre-state: MODIFY ... NOT NULL on a PK
// member no longer fabricates a false nullable→not-null tightening, while a
// truly nullable column still reports it.
func TestT06A2ModifyColumnPreStateConsumption(t *testing.T) {
	pkBatch := t06A2TablePK + "\nALTER TABLE t MODIFY COLUMN id INT NOT NULL;"
	nullableBatch := "CREATE TABLE t (c INT);\nALTER TABLE t MODIFY COLUMN c INT NOT NULL;"
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			policy := t06A2Policy(t, true, t06A2ModifyRule)

			result := t06A2Audit(t, dialect, pkBatch, policy)
			if len(result.Statements) != 2 {
				t.Fatalf("pk batch statements = %d, want 2", len(result.Statements))
			}
			if len(result.Statements[1].Findings) != 0 {
				t.Fatalf("pk-member modify findings = %+v, want none — source is already NOT NULL",
					result.Statements[1].Findings)
			}

			result = t06A2Audit(t, dialect, nullableBatch, policy)
			findings := result.Statements[1].Findings
			if len(findings) != 1 || findings[0].RuleID != t06A2ModifyRule ||
				findings[0].Message != `column "c" tightens nullability from nullable to not null` {
				t.Fatalf("nullable modify findings = %+v, want the real tightening blocker", findings)
			}

			ctx := context.Background()
			parsed, err := Parse(ctx, pkBatch, dialect)
			if err != nil {
				t.Fatal(err)
			}
			statements, err := Extract(ctx, parsed)
			if err != nil {
				t.Fatal(err)
			}
			provider := &t05AbsentProvider{}
			state := newBatchState(dialect, "golden", provider)
			facts, _ := provider.LoadInstanceFacts(ctx, dialect, "golden")
			enriched, err := state.enrichStatements(ctx, &MetadataRequest{Schema: "golden", Provider: provider},
				statements, parsed.failures, facts, auditLimits{orderedStatements: 2})
			if err != nil {
				t.Fatal(err)
			}
			snapshot := enriched[1].Metadata.TargetTable
			if snapshot == nil || !snapshot.Exists {
				t.Fatalf("follower pre-state = %+v, want an existing t", snapshot)
			}
			source := snapshot.FindColumn("id")
			if source == nil || !source.NotNull {
				t.Fatalf("pre-state id = %+v, want NotNull=true carried from CREATE", source)
			}
		})
	}
}

// TestT06A2DefaultRequirePreserved pins that the default-presence rule keeps
// its exact contract: absent clause reports, explicit DEFAULT NULL satisfies.
func TestT06A2DefaultRequirePreserved(t *testing.T) {
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			policy := t06A2Policy(t, true, t06A2DefaultRule)

			result := t06A2Audit(t, dialect, t06A2NoDefault, policy)
			statement := result.Statements[0]
			if result.Verdict != report.VerdictReject || result.Coverage.Status != report.CoverageComplete {
				t.Fatalf("no-default aggregate = %s/%s, want reject/complete", result.Verdict, result.Coverage.Status)
			}
			if len(statement.Findings) != 1 || statement.Findings[0].RuleID != t06A2DefaultRule ||
				statement.Findings[0].Message != `column "c" should define a default value` {
				t.Fatalf("no-default findings = %+v, want exactly the default-require blocker", statement.Findings)
			}

			result = t06A2Audit(t, dialect, t06A2DefaultNull, policy)
			statement = result.Statements[0]
			if result.Verdict != report.VerdictPass || len(statement.Findings) != 0 ||
				len(statement.EvidenceGaps) != 0 {
				t.Fatalf("default-null = %s findings=%+v gaps=%+v, want a clean pass",
					result.Verdict, statement.Findings, statement.EvidenceGaps)
			}
		})
	}
}

// TestT06A2StatementIdentityPreserved pins per-statement identity on a mixed
// batch so member normalization never leaks onto neighboring statements.
func TestT06A2StatementIdentityPreserved(t *testing.T) {
	lines := []string{t06A2NullTablePK, t06A2TablePK, t06A2NoPK}
	sql := strings.Join(lines, "\n")
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			result := t06A2Audit(t, dialect, sql, t06A2Policy(t, true, t06A2PKNotNullRule))
			if len(result.Statements) != 3 {
				t.Fatalf("statements = %d, want 3", len(result.Statements))
			}
			for i, statement := range result.Statements {
				t06A1CheckStatementIdentity(t, statement, i, lines[i])
			}
			t06A2CheckPKNotNullFinding(t, result.Statements[0], "t", "id")
			for i := 1; i < 3; i++ {
				if len(result.Statements[i].Findings) != 0 {
					t.Fatalf("statement %d findings = %+v — the conflict blocker must not leak",
						i, result.Statements[i].Findings)
				}
			}
		})
	}
}
