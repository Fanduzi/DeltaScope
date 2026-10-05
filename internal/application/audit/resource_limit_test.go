// Package audit verifies the T05-A7 ordered-state statement budget.
// input: small MySQL/TiDB first-path batches driven through the real auditWithLimits core under explicit quotas, plus a direct batchState.enrichStatements call for state-level proof
// output: quota-exhaustion assertions on retained blocked statements, ordered-state resource-limit evidence, control-prefix equality, provider-call bounds, post-state non-publication, and input/provider immutability
// pos: application-layer regression for the T05-A7 shared ordered-state resource boundary (issue #84)
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule/catalog"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const t05A7FirstPathSQL = "CREATE TABLE t (id INT PRIMARY KEY);\nALTER TABLE t ADD COLUMN c INT;\nCREATE INDEX idx_c ON t(c);"

var t05A7FirstPathLines = []string{
	"CREATE TABLE t (id INT PRIMARY KEY);",
	"ALTER TABLE t ADD COLUMN c INT;",
	"CREATE INDEX idx_c ON t(c);",
}

func t05A7Policy(t *testing.T, enabled map[string]string) string {
	t.Helper()
	for id := range enabled {
		if _, ok := catalog.Lookup(id); !ok {
			t.Fatalf("missing catalog rule %s", id)
		}
	}
	var text strings.Builder
	text.WriteString("rules:\n")
	for _, entry := range catalog.All() {
		params, on := enabled[entry.RuleID]
		fmt.Fprintf(&text, "  %q:\n    enabled: %t\n", entry.RuleID, on)
		if on {
			text.WriteString("    level: blocker\n")
			if params != "" {
				text.WriteString("    params:\n" + params)
			}
		}
	}
	path := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(path, []byte(text.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func t05A7FourRulePolicy(t *testing.T) string {
	return t05A7Policy(t, map[string]string{
		t05RuleCreateForbid: "", t05RuleAlterRequire: "", t05RuleAddColumnForbid: "",
		t05RuleCreateIndexColumns: "      required: true\n",
	})
}

func t05A7ResourceEntries(result report.Result) []spec.UnsupportedDetail {
	var resources []spec.UnsupportedDetail
	for _, item := range result.Unsupported {
		if item.Feature == spec.AuditResourceLimitFeature {
			resources = append(resources, item)
		}
	}
	return resources
}

func t05A7AssertResourceEntry(t *testing.T, item spec.UnsupportedDetail, statement report.StatementResult, index, limit int) {
	t.Helper()
	if item.Index != index || item.Reason != spec.AuditResourceLimitReason || item.SQL != statement.RawSQL {
		t.Fatalf("resource entry = %+v, want index %d bound to statement %d", item, index, index)
	}
	want := map[string]any{"phase": "ordered_state", "resource": "statements", "limit": limit, "consumed": limit, "line": index + 1, "column": 1}
	if !reflect.DeepEqual(item.Metadata, want) {
		t.Fatalf("resource metadata = %+v, want %+v", item.Metadata, want)
	}
}

func t05A7AssertBlockedStatement(t *testing.T, statement report.StatementResult, index int, line string) {
	t.Helper()
	if statement.Index != index || statement.Kind != "ddl" || strings.TrimSpace(statement.RawSQL) != line || statement.NormalizedSQL == "" {
		t.Fatalf("blocked statement %d identity = %+v", index, statement)
	}
	if statement.Coverage.Status != report.CoverageIncomplete || len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 || statement.Impact != nil {
		t.Fatalf("blocked statement %d = %s findings=%+v gaps=%+v impact=%+v, want incomplete with none", index, statement.Coverage.Status, statement.Findings, statement.EvidenceGaps, statement.Impact)
	}
}

func TestT05A7FirstPath(t *testing.T) {
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			for _, online := range []bool{true, false} {
				name := "offline"
				if online {
					name = "known_absent"
				}
				t.Run(name, func(t *testing.T) {
					policy := t05A7FourRulePolicy(t)
					run := func(limit int) (report.Result, error, *t05AbsentProvider) {
						var provider *t05AbsentProvider
						request := Request{SQL: t05A7FirstPathSQL, Dialect: dialect, Schema: "golden", ConfigPath: policy}
						if online {
							provider = &t05AbsentProvider{}
							request.MetadataProvider = provider
						}
						result, err := auditWithLimits(context.Background(), request, auditLimits{orderedStatements: limit})
						return result, err, provider
					}

					control, controlErr, controlProvider := run(len(t05A7FirstPathLines))
					if controlErr != nil {
						t.Fatalf("control audit: %v", controlErr)
					}
					if len(control.Statements) != len(t05A7FirstPathLines) {
						t.Fatalf("control retained %d statements, want %d", len(control.Statements), len(t05A7FirstPathLines))
					}
					for i, want := range t05A7FirstPathLines {
						statement := control.Statements[i]
						if statement.Index != i || statement.Kind != "ddl" || strings.TrimSpace(statement.RawSQL) != want || statement.NormalizedSQL == "" {
							t.Fatalf("control statement %d identity = %+v", i, statement)
						}
						if len(statement.Findings) != 0 {
							t.Fatalf("control statement %d findings = %+v, want none", i, statement.Findings)
						}
					}
					if len(control.GlobalFindings) != 0 {
						t.Fatalf("control global findings = %+v, want none", control.GlobalFindings)
					}
					if len(control.Unsupported) != 0 {
						t.Fatalf("control unsupported = %+v, want none", control.Unsupported)
					}
					if control.RuleSummary == nil || control.RuleSummary.Loaded != 4 {
						t.Fatalf("control loaded rules = %+v, want 4", control.RuleSummary)
					}
					if online {
						if control.Verdict != report.VerdictPass || control.Coverage.Status != report.CoverageComplete {
							t.Fatalf("control aggregate = %s/%s, want pass/complete", control.Verdict, control.Coverage.Status)
						}
						for i := range control.Statements {
							if control.Statements[i].Coverage.Status != report.CoverageComplete || len(control.Statements[i].EvidenceGaps) != 0 {
								t.Fatalf("control statement %d = %s gaps=%+v, want complete with none", i, control.Statements[i].Coverage.Status, control.Statements[i].EvidenceGaps)
							}
						}
						if controlProvider == nil || len(controlProvider.calls) != 1 || controlProvider.calls[0] != "golden.t" {
							t.Fatalf("control provider calls = %#v, want exactly golden.t once", controlProvider.calls)
						}
					} else {
						if control.Verdict != report.VerdictReview || control.Coverage.Status != report.CoverageUnverified {
							t.Fatalf("offline control aggregate = %s/%s, want review/unverified", control.Verdict, control.Coverage.Status)
						}
						if len(control.Statements[0].EvidenceGaps) != 1 {
							t.Fatalf("offline statement 0 gaps = %+v, want exactly one", control.Statements[0].EvidenceGaps)
						}
						gaps := t05GapsByRule(control, 0, t05RuleCreateForbid)
						if len(gaps) != 1 || gaps[0].ReasonCode != "unknown_table_state" {
							t.Fatalf("offline statement 0 create-forbid gaps = %+v, want one unknown_table_state", control.Statements[0].EvidenceGaps)
						}
						if control.Statements[0].Coverage.Status != report.CoverageUnverified {
							t.Fatalf("offline statement 0 coverage = %s, want unverified", control.Statements[0].Coverage.Status)
						}
						for i := 1; i < len(control.Statements); i++ {
							if control.Statements[i].Coverage.Status != report.CoverageComplete || len(control.Statements[i].EvidenceGaps) != 0 {
								t.Fatalf("offline statement %d = %s gaps=%+v, want complete with none", i, control.Statements[i].Coverage.Status, control.Statements[i].EvidenceGaps)
							}
						}
					}

					for _, limit := range []int{0, 1, 2, 3} {
						t.Run(fmt.Sprintf("limit_%d", limit), func(t *testing.T) {
							result, err, provider := run(limit)
							wantBlocked := len(t05A7FirstPathLines) - limit
							if wantBlocked < 0 {
								wantBlocked = 0
							}
							if wantBlocked == 0 {
								if err != nil {
									t.Fatalf("audit at exact budget = %v, want nil error", err)
								}
							} else {
								if !errors.Is(err, ErrUnsupportedStatement) {
									t.Fatalf("audit error = %v, want ErrUnsupportedStatement", err)
								}
								if result.Coverage.Status != report.CoverageIncomplete || result.Verdict != report.VerdictReview {
									t.Fatalf("aggregate = %s/%s, want incomplete/review", result.Coverage.Status, result.Verdict)
								}
							}
							if len(result.Statements) != len(t05A7FirstPathLines) {
								t.Fatalf("retained statements = %d, want %d", len(result.Statements), len(t05A7FirstPathLines))
							}
							admitted := limit
							if admitted > len(t05A7FirstPathLines) {
								admitted = len(t05A7FirstPathLines)
							}
							for i := 0; i < admitted; i++ {
								if !reflect.DeepEqual(result.Statements[i], control.Statements[i]) {
									t.Fatalf("admitted statement %d diverged from control:\n got %+v\nwant %+v", i, result.Statements[i], control.Statements[i])
								}
							}
							for i := admitted; i < len(t05A7FirstPathLines); i++ {
								t05A7AssertBlockedStatement(t, result.Statements[i], i, t05A7FirstPathLines[i])
							}
							resources := t05A7ResourceEntries(result)
							if len(resources) != wantBlocked {
								t.Fatalf("resource entries = %d, want %d (unsupported=%+v)", len(resources), wantBlocked, result.Unsupported)
							}
							if len(result.Unsupported) != wantBlocked {
								t.Fatalf("unsupported entries = %d, want %d (all resource): %+v", len(result.Unsupported), wantBlocked, result.Unsupported)
							}
							for k, item := range resources {
								t05A7AssertResourceEntry(t, item, result.Statements[admitted+k], admitted+k, limit)
							}
							if online {
								if limit == 0 {
									if len(provider.calls) != 0 {
										t.Fatalf("provider calls = %#v, want none at quota zero", provider.calls)
									}
								} else if len(provider.calls) != 1 || provider.calls[0] != "golden.t" {
									t.Fatalf("provider calls = %#v, want exactly golden.t once", provider.calls)
								}
							}
						})
					}
				})
			}
		})
	}
}

func TestT05A7RetainedSuffixAndState(t *testing.T) {
	suffixLines := append(append([]string{}, t05A7FirstPathLines...), "ALTER TABLE t ADD COLUMN d INT;")
	suffixSQL := strings.Join(suffixLines, "\n")
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			policy := t05A7FourRulePolicy(t)

			control, err := auditWithLimits(context.Background(), Request{
				SQL:              t05A7FirstPathSQL,
				Dialect:          dialect,
				Schema:           "golden",
				ConfigPath:       policy,
				MetadataProvider: &t05AbsentProvider{},
			}, auditLimits{orderedStatements: len(t05A7FirstPathLines)})
			if err != nil {
				t.Fatalf("control audit: %v", err)
			}
			if len(control.Statements) != len(t05A7FirstPathLines) {
				t.Fatalf("control retained %d statements, want %d", len(control.Statements), len(t05A7FirstPathLines))
			}

			result, err := auditWithLimits(context.Background(), Request{
				SQL:              suffixSQL,
				Dialect:          dialect,
				Schema:           "golden",
				ConfigPath:       policy,
				MetadataProvider: &t05AbsentProvider{},
			}, auditLimits{orderedStatements: 2})
			if !errors.Is(err, ErrUnsupportedStatement) {
				t.Fatalf("audit error = %v, want ErrUnsupportedStatement", err)
			}
			if len(result.Statements) != len(suffixLines) {
				t.Fatalf("retained statements = %d, want %d", len(result.Statements), len(suffixLines))
			}
			for i := 0; i < 2; i++ {
				if !reflect.DeepEqual(result.Statements[i], control.Statements[i]) {
					t.Fatalf("admitted statement %d diverged from control:\n got %+v\nwant %+v", i, result.Statements[i], control.Statements[i])
				}
			}
			for i := 2; i < len(suffixLines); i++ {
				t05A7AssertBlockedStatement(t, result.Statements[i], i, suffixLines[i])
			}
			resources := t05A7ResourceEntries(result)
			if len(resources) != 2 {
				t.Fatalf("resource entries = %d, want 2 (unsupported=%+v)", len(resources), result.Unsupported)
			}
			for k, item := range resources {
				t05A7AssertResourceEntry(t, item, result.Statements[2+k], 2+k, 2)
			}
			if result.Coverage.Status != report.CoverageIncomplete || result.Verdict != report.VerdictReview {
				t.Fatalf("aggregate = %s/%s, want incomplete/review", result.Coverage.Status, result.Verdict)
			}

			snapshot := absentTable("golden", "t")
			snapshotBefore := cloneTableSnapshot(snapshot)
			provider := &t05PresentProvider{snapshots: map[string]*spec.TableSnapshot{"golden.t": snapshot}}
			parsed, err := Parse(context.Background(), suffixSQL, dialect)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			extracted, err := Extract(context.Background(), parsed)
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			state := newBatchState(dialect, "golden", provider)
			instanceFacts, err := provider.LoadInstanceFacts(context.Background(), dialect, "golden")
			if err != nil {
				t.Fatalf("instance facts: %v", err)
			}
			enriched, err := state.enrichStatements(context.Background(), &MetadataRequest{Schema: "golden", Provider: provider}, extracted, parsed.failures, instanceFacts, auditLimits{orderedStatements: 2})
			if err != nil {
				t.Fatalf("enrich: %v", err)
			}
			if !state.contaminated {
				t.Fatalf("state must be contaminated after budget exhaustion")
			}
			if len(enriched) != len(suffixLines) {
				t.Fatalf("enriched %d statements, want %d", len(enriched), len(suffixLines))
			}
			if len(provider.calls) != 1 || provider.calls[0] != "golden.t" {
				t.Fatalf("provider calls = %#v, want exactly golden.t once", provider.calls)
			}
			if enriched[0].Metadata == nil || enriched[0].Metadata.TargetTable == nil || enriched[0].Metadata.TargetTable.Exists {
				t.Fatalf("first pre-state = %+v, want confirmed-absent golden.t", enriched[0].Metadata)
			}
			secondPreState := enriched[1].Metadata.TargetTable
			if secondPreState == nil {
				t.Fatalf("second pre-state missing, want derived post-CREATE shape")
			}
			if got := t05A6ColumnNames(secondPreState); !reflect.DeepEqual(got, []string{"id"}) {
				t.Fatalf("second pre-state columns = %v, want [id]", got)
			}
			for i := 2; i < len(suffixLines); i++ {
				boundary := enriched[i].ResourceLimit
				if boundary == nil || boundary.Limit != 2 || boundary.Consumed != 2 {
					t.Fatalf("enriched[%d] boundary = %+v, want limit=2 consumed=2", i, boundary)
				}
				if m := enriched[i].Metadata; m != nil && (m.TargetTable != nil || len(m.Objects) > 0) {
					t.Fatalf("blocked metadata = %+v, want no target/objects", m)
				}
			}
			entry := state.entries[state.keyFor("golden", spec.Table{Name: "t"})]
			if entry == nil || entry.shape == nil {
				t.Fatalf("state entry for golden.t = %+v, want a published shape", entry)
			}
			if got := t05A6ColumnNames(entry.shape); !reflect.DeepEqual(got, []string{"id", "c"}) {
				t.Fatalf("state columns = %v, want [id c]", got)
			}
			for _, index := range entry.shape.Indexes {
				if index.Name == "idx_c" {
					t.Fatalf("blocked CREATE INDEX published idx_c into the state")
				}
			}
			if entry.shape.PrimaryKey == nil || len(entry.shape.PrimaryKey.Columns) != 1 || entry.shape.PrimaryKey.Columns[0] != "id" {
				t.Fatalf("state primary key = %+v, want PRIMARY(id)", entry.shape.PrimaryKey)
			}
			if got := t05A6ColumnNames(secondPreState); !reflect.DeepEqual(got, []string{"id"}) {
				t.Fatalf("retained second pre-state mutated to %v, want [id]", got)
			}
			if !reflect.DeepEqual(snapshot, snapshotBefore) {
				t.Fatalf("provider-owned snapshot mutated:\n got %+v\nwant %+v", snapshot, snapshotBefore)
			}

			parsedDML, err := Parse(context.Background(), "CREATE TABLE t (id INT PRIMARY KEY);\nDELETE FROM t;", dialect)
			if err != nil {
				t.Fatalf("parse dml: %v", err)
			}
			extractedDML, err := Extract(context.Background(), parsedDML)
			if err != nil {
				t.Fatalf("extract dml: %v", err)
			}
			if len(extractedDML) != 2 || extractedDML[1].DML == nil || extractedDML[1].DML.Impact == nil {
				t.Fatalf("expected extracted nonnil DML impact on the DELETE input, got %+v", extractedDML)
			}
			dmlState := newBatchState(dialect, "golden", nil)
			enrichedDML, err := dmlState.enrichStatements(context.Background(), nil, extractedDML, parsedDML.failures, nil, auditLimits{orderedStatements: 1})
			if err != nil {
				t.Fatalf("enrich dml: %v", err)
			}
			if enrichedDML[1].ResourceLimit == nil {
				t.Fatalf("blocked DELETE must carry the resource boundary, got %+v", enrichedDML[1])
			}
			if enrichedDML[1].DML == nil || enrichedDML[1].DML.Impact != nil {
				t.Fatalf("blocked DML clone must clear impact, got %+v", enrichedDML[1].DML)
			}
			if enrichedDML[1].DML == extractedDML[1].DML {
				t.Fatalf("blocked DML must be a clone, not the extracted pointer")
			}
			if extractedDML[1].DML.Impact == nil {
				t.Fatalf("extracted input mutated: impact cleared")
			}
		})
	}
}
