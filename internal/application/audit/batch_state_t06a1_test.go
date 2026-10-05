// Package audit verifies the T06-A1 CREATE TABLE primary-key-presence baseline.
// input: single-statement and multi-table CREATE TABLE inputs through AuditSQL plus the enrichment seam
// output: isolated ddl.table.primary_key.require identity, normalization equivalence, and pre-state propagation
// pos: application-layer contract tests for the frozen T06-A1 oracle (issue #85)
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
	t06A1PKRule   = "ddl.table.primary_key.require"
	t06A1NoPK     = "CREATE TABLE t (id INT);"
	t06A1InlinePK = "CREATE TABLE t (id INT PRIMARY KEY);"
	t06A1TablePK  = "CREATE TABLE t (id INT, PRIMARY KEY (id));"
)

func t06A1PKPolicy(t *testing.T, required bool) string {
	t.Helper()
	params := "      required: true\n"
	if !required {
		params = "      required: false\n"
	}
	return t05PolicyPath(t, map[string]string{t06A1PKRule: params})
}

func t06A1AllOffPolicy(t *testing.T) string {
	t.Helper()
	return t05PolicyPath(t, map[string]string{})
}

func t06A1Audit(t *testing.T, dialect spec.Dialect, sql, policy string) report.Result {
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

func t06A1CheckStatementIdentity(t *testing.T, statement report.StatementResult, index int, raw string) {
	t.Helper()
	if statement.Index != index || statement.Kind != "ddl" ||
		statement.RawSQL != raw || statement.NormalizedSQL != strings.TrimSuffix(raw, ";") {
		t.Fatalf("statement %d identity = %+v, want ddl %q", index, statement, raw)
	}
	if statement.Impact != nil {
		t.Fatalf("statement %d impact = %+v, want none", index, statement.Impact)
	}
	if statement.Coverage.Status != report.CoverageComplete || len(statement.EvidenceGaps) != 0 {
		t.Fatalf("statement %d = %s gaps=%+v, want complete with no gaps", index, statement.Coverage.Status, statement.EvidenceGaps)
	}
}

func t06A1CheckPKFinding(t *testing.T, statement report.StatementResult, table string) {
	t.Helper()
	if len(statement.Findings) != 1 {
		t.Fatalf("findings = %+v, want exactly the primary-key blocker", statement.Findings)
	}
	finding := statement.Findings[0]
	locationOK := finding.Location != nil && finding.Location.Line == 1 && finding.Location.Column == 1
	if finding.RuleID != t06A1PKRule || finding.Level != "blocker" ||
		finding.Message != "primary key is required" ||
		finding.StatementIndex != statement.Index || finding.StatementKind != "ddl" ||
		!locationOK || !reflect.DeepEqual(finding.Metadata, map[string]any{"table": table}) {
		t.Fatalf("finding = %+v, want ddl.table.primary_key.require blocker on %s at 1:1", finding, table)
	}
}

// TestT06A1PrimaryKeyPresenceControls pins the five frozen controls for both
// dialects on the shared AuditSQL path: required:true reject, both PK forms
// pass, rule disabled passes, and required:false passes as a distinct profile.
func TestT06A1PrimaryKeyPresenceControls(t *testing.T) {
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			required := t06A1PKPolicy(t, true)
			optional := t06A1PKPolicy(t, false)
			disabled := t06A1AllOffPolicy(t)

			result := t06A1Audit(t, dialect, t06A1NoPK, required)
			if result.Verdict != report.VerdictReject || result.Coverage.Status != report.CoverageComplete {
				t.Fatalf("no-pk aggregate = %s/%s, want reject/complete", result.Verdict, result.Coverage.Status)
			}
			statement := result.Statements[0]
			t06A1CheckStatementIdentity(t, statement, 0, t06A1NoPK)
			t06A1CheckPKFinding(t, statement, "t")
			if len(result.Unsupported) != 0 || len(result.GlobalFindings) != 0 || len(result.Diagnostics) != 0 {
				t.Fatalf("no-pk unexpected channels: unsupported=%+v global=%+v diagnostics=%+v",
					result.Unsupported, result.GlobalFindings, result.Diagnostics)
			}
			if result.Summary.Blockers != 1 || result.Summary.Warnings != 0 || result.Summary.Notices != 0 {
				t.Fatalf("no-pk summary = %+v, want exactly one blocker", result.Summary)
			}
			if result.RuleSummary == nil || result.RuleSummary.Loaded != 1 {
				t.Fatalf("no-pk rule summary = %+v, want loaded 1", result.RuleSummary)
			}

			for name, sql := range map[string]string{"inline": t06A1InlinePK, "table": t06A1TablePK} {
				result := t06A1Audit(t, dialect, sql, required)
				if result.Verdict != report.VerdictPass || result.Coverage.Status != report.CoverageComplete {
					t.Fatalf("%s aggregate = %s/%s, want pass/complete", name, result.Verdict, result.Coverage.Status)
				}
				statement := result.Statements[0]
				t06A1CheckStatementIdentity(t, statement, 0, sql)
				if len(statement.Findings) != 0 {
					t.Fatalf("%s findings = %+v, want none", name, statement.Findings)
				}
				if result.RuleSummary == nil || result.RuleSummary.Loaded != 1 {
					t.Fatalf("%s rule summary = %+v, want loaded 1", name, result.RuleSummary)
				}
			}

			for name, policy := range map[string]string{"rule-off": disabled, "required-false": optional} {
				result := t06A1Audit(t, dialect, t06A1NoPK, policy)
				if result.Verdict != report.VerdictPass || result.Coverage.Status != report.CoverageComplete {
					t.Fatalf("%s aggregate = %s/%s, want pass/complete", name, result.Verdict, result.Coverage.Status)
				}
				statement := result.Statements[0]
				t06A1CheckStatementIdentity(t, statement, 0, t06A1NoPK)
				if len(statement.Findings) != 0 {
					t.Fatalf("%s findings = %+v, want none (disabling must not fabricate gaps)", name, statement.Findings)
				}
				if name == "rule-off" && result.RuleSummary != nil {
					t.Fatalf("rule-off rule summary = %+v, want none loaded", result.RuleSummary)
				}
				if name == "required-false" && (result.RuleSummary == nil || result.RuleSummary.Loaded != 1) {
					t.Fatalf("required-false rule summary = %+v, want the optional rule still loaded", result.RuleSummary)
				}
			}
		})
	}
}

// TestT06A1BatchAttribution pins a multi-statement batch over different
// tables: the blocker stays bound to the offending statement and never leaks
// onto the PK-bearing creates.
func TestT06A1BatchAttribution(t *testing.T) {
	lines := []string{
		"CREATE TABLE no_pk_t (id INT);",
		"CREATE TABLE inline_t (id INT PRIMARY KEY);",
		"CREATE TABLE table_t (id INT, PRIMARY KEY (id));",
	}
	sql := strings.Join(lines, "\n")
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			result := t06A1Audit(t, dialect, sql, t06A1PKPolicy(t, true))
			if result.Verdict != report.VerdictReject || result.Coverage.Status != report.CoverageComplete {
				t.Fatalf("aggregate = %s/%s, want reject/complete", result.Verdict, result.Coverage.Status)
			}
			if len(result.Statements) != 3 {
				t.Fatalf("statements = %d, want 3", len(result.Statements))
			}
			for i, statement := range result.Statements {
				t06A1CheckStatementIdentity(t, statement, i, lines[i])
			}
			t06A1CheckPKFinding(t, result.Statements[0], "no_pk_t")
			for i := 1; i < 3; i++ {
				if len(result.Statements[i].Findings) != 0 {
					t.Fatalf("statement %d findings = %+v, want none — the blocker must not leak", i, result.Statements[i].Findings)
				}
			}
		})
	}
}

// TestT06A1UniqueKeyNotPrimaryKey pins that a table-level UNIQUE index is
// normalized yet never satisfies the explicit-primary-key requirement.
func TestT06A1UniqueKeyNotPrimaryKey(t *testing.T) {
	sql := "CREATE TABLE t (id INT, UNIQUE KEY uq_id (id));"
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			result := t06A1Audit(t, dialect, sql, t06A1PKPolicy(t, true))
			if result.Verdict != report.VerdictReject || result.Coverage.Status != report.CoverageComplete {
				t.Fatalf("aggregate = %s/%s, want reject/complete", result.Verdict, result.Coverage.Status)
			}
			statement := result.Statements[0]
			t06A1CheckStatementIdentity(t, statement, 0, sql)
			t06A1CheckPKFinding(t, statement, "t")
			if len(result.Unsupported) != 0 {
				t.Fatalf("table-level UNIQUE must stay audited, unsupported = %+v", result.Unsupported)
			}

			parsed, err := Parse(context.Background(), sql, dialect)
			if err != nil {
				t.Fatal(err)
			}
			statements, err := Extract(context.Background(), parsed)
			if err != nil {
				t.Fatal(err)
			}
			ddl := statements[0].DDL
			if ddl == nil || ddl.PrimaryKey != nil {
				t.Fatalf("UNIQUE KEY leaked into DDL.PrimaryKey = %+v", ddl)
			}
			found := false
			for _, index := range ddl.Indexes {
				if index.Name == "uq_id" && index.Kind == spec.IndexKindUnique &&
					reflect.DeepEqual(index.Columns, []string{"id"}) {
					found = true
				}
			}
			if !found {
				t.Fatalf("UNIQUE KEY not normalized into Indexes: %+v", ddl.Indexes)
			}
		})
	}
}

// TestT06A1PrimaryKeyNormalization pins the normalized shape shared by the
// inline and table-level forms: same name, kind, and columns.
func TestT06A1PrimaryKeyNormalization(t *testing.T) {
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			extract := func(sql string) *spec.Index {
				parsed, err := Parse(context.Background(), sql, dialect)
				if err != nil {
					t.Fatal(err)
				}
				statements, err := Extract(context.Background(), parsed)
				if err != nil {
					t.Fatal(err)
				}
				if len(statements) != 1 || statements[0].DDL == nil {
					t.Fatalf("extract %q = %+v, want one ddl statement", sql, statements)
				}
				return statements[0].DDL.PrimaryKey
			}
			if pk := extract(t06A1NoPK); pk != nil {
				t.Fatalf("no-pk PrimaryKey = %+v, want nil", pk)
			}
			inline := extract(t06A1InlinePK)
			table := extract(t06A1TablePK)
			for name, pk := range map[string]*spec.Index{"inline": inline, "table": table} {
				if pk == nil || pk.Name != "primary" || pk.Kind != spec.IndexKindPrimary ||
					!reflect.DeepEqual(pk.Columns, []string{"id"}) {
					t.Fatalf("%s PrimaryKey = %+v, want primary/[id]", name, pk)
				}
			}
			if !reflect.DeepEqual(inline, table) {
				t.Fatalf("inline %+v and table-level %+v primary keys must normalize identically", inline, table)
			}
		})
	}
}

// TestT06A1PrimaryKeyPreStateConsumption pins that the PK fact published by
// applyCreateTable is readable from the next statement's pre-state snapshot,
// for both normalized PK forms, and that the snapshot is a clone (the
// extracted statement is never mutated).
func TestT06A1PrimaryKeyPreStateConsumption(t *testing.T) {
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for name, create := range map[string]string{
			"inline": t06A1InlinePK,
			"table":  t06A1TablePK,
			"no-pk":  t06A1NoPK,
		} {
			t.Run(string(dialect)+"/"+name, func(t *testing.T) {
				ctx := context.Background()
				sql := create + "\nALTER TABLE t ADD COLUMN c INT;"
				parsed, err := Parse(ctx, sql, dialect)
				if err != nil {
					t.Fatal(err)
				}
				statements, err := Extract(ctx, parsed)
				if err != nil {
					t.Fatal(err)
				}
				if len(statements) != 2 {
					t.Fatalf("extracted = %d, want 2", len(statements))
				}
				provider := &t05AbsentProvider{}
				state := newBatchState(dialect, "golden", provider)
				facts, _ := provider.LoadInstanceFacts(ctx, dialect, "golden")
				enriched, err := state.enrichStatements(ctx, &MetadataRequest{Schema: "golden", Provider: provider},
					statements, parsed.failures, facts, auditLimits{orderedStatements: 2})
				if err != nil {
					t.Fatal(err)
				}
				var snapshot *spec.TableSnapshot
				if enriched[1].Metadata != nil {
					snapshot = enriched[1].Metadata.TargetTable
				}
				if snapshot == nil || !snapshot.Exists {
					t.Fatalf("follower pre-state = %+v, want an existing t", snapshot)
				}
				if name == "no-pk" {
					if snapshot.PrimaryKey != nil || snapshot.HasPrimaryKey() {
						t.Fatalf("no-pk pre-state PrimaryKey = %+v, want absent", snapshot.PrimaryKey)
					}
					return
				}
				pk := snapshot.PrimaryKey
				if pk == nil || !snapshot.HasPrimaryKey() || pk.Name != "primary" ||
					pk.Kind != spec.IndexKindPrimary || !reflect.DeepEqual(pk.Columns, []string{"id"}) {
					t.Fatalf("pre-state PrimaryKey = %+v, want primary/[id]", pk)
				}
				if pk == statements[0].DDL.PrimaryKey {
					t.Fatal("pre-state shares the statement's PrimaryKey pointer — it must be a clone")
				}
			})
		}
	}
}
