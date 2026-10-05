// Package audit verifies the T05-A8 procedure outer-state first path.
// input: real MySQL/TiDB CREATE or DROP PROCEDURE migration batches with known-absent table metadata
// output: retained procedure boundaries, complete followers, exact table pre-states, and once-only table lookup assertions
// pos: public application and production enrichment regression for procedure lifecycle isolation
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func t05A8Input(procedure string) string {
	return "CREATE TABLE t (id INT PRIMARY KEY);\n" + procedure + "\nALTER TABLE t ADD COLUMN c INT;\nCREATE INDEX idx_c ON t(c);"
}

func TestT05A8ProcedureFirstPath(t *testing.T) {
	for _, dialect := range []spec.Dialect{spec.DialectTiDB, spec.DialectMySQL} {
		for _, tc := range []struct {
			name, sql string
			create    bool
		}{
			{"create", "CREATE PROCEDURE p() SELECT 1;", true},
			{"drop", "DROP PROCEDURE p;", false},
		} {
			t.Run(string(dialect)+"/"+tc.name, func(t *testing.T) {
				ctx := context.Background()
				sql := t05A8Input(tc.sql)
				provider := &t05AbsentProvider{}
				result, err := AuditSQL(ctx, Request{SQL: sql, Dialect: dialect, Schema: "golden", ConfigPath: t05A7FourRulePolicy(t), MetadataProvider: provider})
				boundary := dialect == spec.DialectTiDB || tc.create
				if boundary {
					if !errors.Is(err, ErrUnsupportedStatement) {
						t.Fatalf("error = %v, want ErrUnsupportedStatement", err)
					}
					if result.Verdict != report.VerdictReview || result.Coverage.Status != report.CoverageIncomplete {
						t.Errorf("aggregate = %s/%s, want review/incomplete", result.Verdict, result.Coverage.Status)
					}
				} else if err != nil || result.Verdict != report.VerdictPass || result.Coverage.Status != report.CoverageComplete {
					t.Errorf("MySQL DROP = %s/%s error %v, want pass/complete nil", result.Verdict, result.Coverage.Status, err)
				}
				if len(result.Statements) != 4 {
					t.Fatalf("statements = %d, want 4", len(result.Statements))
				}
				for i, statement := range result.Statements {
					if statement.Index != i || statement.Kind != "ddl" || statement.RawSQL != strings.Split(sql, "\n")[i] || statement.NormalizedSQL != strings.TrimSuffix(statement.RawSQL, ";") {
						t.Errorf("statement %d identity = %+v", i, statement)
					}
					wantCoverage := report.CoverageComplete
					if i == 1 && boundary {
						wantCoverage = report.CoverageIncomplete
					}
					if statement.Coverage.Status != wantCoverage || len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 || statement.Impact != nil {
						t.Errorf("statement %d = %s findings=%+v gaps=%+v impact=%+v, want %s with no findings/gaps/impact", i, statement.Coverage.Status, statement.Findings, statement.EvidenceGaps, statement.Impact, wantCoverage)
					}
				}
				wantUnsupported := 0
				if boundary {
					wantUnsupported = 1
				}
				if len(result.Unsupported) != wantUnsupported {
					t.Fatalf("unsupported = %+v, want %d entries", result.Unsupported, wantUnsupported)
				}
				if boundary {
					feature, reason := "create_procedure.body", spec.UnsupportedUnauditedReason
					var metadata map[string]any = map[string]any{"aspect": "option"}
					if dialect == spec.DialectTiDB {
						feature, reason, metadata = "create_procedure", spec.UnsupportedVendorBoundaryReason, map[string]any{"boundary": "vendor"}
						if !tc.create {
							feature = "drop_procedure"
						}
					}
					item := result.Unsupported[0]
					if item.Index != 1 || item.Feature != feature || item.Reason != reason || item.SQL != result.Statements[1].RawSQL || !reflect.DeepEqual(item.Metadata, metadata) {
						t.Errorf("procedure boundary = %+v", item)
					}
				}
				if !reflect.DeepEqual(provider.calls, []string{"golden.t"}) {
					t.Errorf("provider reads = %v, want only golden.t once", provider.calls)
				}
				if len(result.GlobalFindings) != 0 || result.RuleSummary == nil || result.RuleSummary.Loaded != 4 {
					t.Errorf("global=%+v rules=%+v", result.GlobalFindings, result.RuleSummary)
				}

				parsed, parseErr := Parse(ctx, sql, dialect)
				if parseErr != nil {
					t.Fatal(parseErr)
				}
				statements, extractErr := Extract(ctx, parsed)
				if extractErr != nil {
					t.Fatal(extractErr)
				}
				if len(statements) != 4 {
					t.Fatalf("extracted = %d, want 4", len(statements))
				}
				for i, statement := range statements {
					if statement.Line != i+1 || statement.Column != 1 {
						t.Errorf("statement %d location = %d:%d", i, statement.Line, statement.Column)
					}
				}
				innerProvider := &t05AbsentProvider{}
				state := newBatchState(dialect, "golden", innerProvider)
				facts, _ := innerProvider.LoadInstanceFacts(ctx, dialect, "golden")
				enriched, enrichErr := state.enrichStatements(ctx, &MetadataRequest{Schema: "golden", Provider: innerProvider}, statements, parsed.failures, facts, auditLimits{orderedStatements: 4})
				if enrichErr != nil {
					t.Fatal(enrichErr)
				}
				for i, want := range map[int][]string{2: {"id"}, 3: {"id", "c"}} {
					var snapshot *spec.TableSnapshot
					if enriched[i].Metadata != nil {
						snapshot = enriched[i].Metadata.TargetTable
					}
					if got := t05A6ColumnNames(snapshot); !reflect.DeepEqual(got, want) {
						t.Errorf("statement %d pre-state columns = %v, want %v", i, got, want)
					}
				}
				if state.contaminated {
					t.Error("procedure lifecycle contaminated outer state")
				}
				entry := state.entries[state.keyFor("golden", spec.Table{Name: "t"})]
				if entry == nil || entry.shape == nil {
					t.Fatal("missing outer table state")
				}
				if got := t05A6ColumnNames(entry.shape); !reflect.DeepEqual(got, []string{"id", "c"}) {
					t.Errorf("final columns = %v, want [id c]", got)
				}
				found := false
				for _, index := range entry.shape.Indexes {
					if index.Name == "idx_c" && reflect.DeepEqual(index.Columns, []string{"c"}) {
						found = true
					}
				}
				if !found {
					t.Error("missing derived idx_c(c)")
				}
				if len(state.entries) != 1 || !reflect.DeepEqual(innerProvider.calls, []string{"golden.t"}) {
					t.Errorf("entries=%+v reads=%v, want only golden.t", state.entries, innerProvider.calls)
				}
			})
		}
	}
}
