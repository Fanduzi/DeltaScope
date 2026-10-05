// Package audit verifies the T05-A8 procedure outer-state regression groups.
// input: real MySQL/TiDB CREATE/DROP PROCEDURE batches against known-absent, seeded, failing, and recording providers
// output: metadata premises, definition identity/preservation, whitelist scope, contamination/budget, and error/consumer assertions
// pos: application-level regression matrix for procedure lifecycle table-state isolation
// note: if this file changes, update this header and module README.md.
package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

type t05A8Provider struct {
	snapshots   map[string]*spec.TableSnapshot
	events      []string
	instanceErr error
	snapshotErr error
	failTable   string
	onInstance  func()
}

func (p *t05A8Provider) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	p.events = append(p.events, "instance")
	if p.onInstance != nil {
		p.onInstance()
	}
	if p.instanceErr != nil {
		return nil, p.instanceErr
	}
	return &spec.InstanceFacts{}, nil
}

func (p *t05A8Provider) LoadTableSnapshot(_ context.Context, _ spec.Dialect, schema, table string) (*spec.TableSnapshot, error) {
	key := schema + "." + table
	p.events = append(p.events, "snapshot:"+key)
	if p.snapshotErr != nil && (p.failTable == "" || p.failTable == key) {
		return nil, p.snapshotErr
	}
	if snapshot, ok := p.snapshots[key]; ok {
		return snapshot, nil
	}
	return absentTable(schema, table), nil
}

func (p *t05A8Provider) ResolveObject(_ context.Context, _ spec.Dialect, req spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	p.events = append(p.events, "object:"+req.Type+":"+req.Schema+"."+req.Name)
	return &spec.ObjectSnapshot{Schema: req.Schema, Type: req.Type, Name: req.Name, Status: spec.MetadataStatusConfirmed, Exists: true}, nil
}

func (p *t05A8Provider) LoadPlanEstimate(_ context.Context, statement spec.Statement) (*spec.ImpactEstimate, error) {
	p.events = append(p.events, "plan:"+strings.TrimSpace(statement.RawSQL))
	return &spec.ImpactEstimate{Source: spec.ImpactSourcePlan}, nil
}

var t05A8GapCreate = rule.EvidenceGap{
	RuleID:        "ddl.table.exists.create.forbid",
	ReasonCode:    "unknown_table_state",
	RequiredFacts: []string{"target_table.existence"},
}

var t05A8GapFollowers = map[int][]rule.EvidenceGap{
	2: {
		{RuleID: "ddl.table.exists.alter.require", ReasonCode: "unknown_table_state", RequiredFacts: []string{"target_table.existence"}},
		{RuleID: "ddl.alter.add_column.exists.forbid", ReasonCode: "unknown_table_state", RequiredFacts: []string{"target_table.columns", "target_table.existence"}},
	},
	3: {
		{RuleID: "ddl.create_index.columns.exists.require", ReasonCode: "unknown_table_state", RequiredFacts: []string{"target_table.columns", "target_table.existence"}},
	},
}

func t05A8Boundary(dialect spec.Dialect, create bool) bool {
	return dialect == spec.DialectTiDB || create
}

func t05A8Unsupported(dialect spec.Dialect, create bool) (feature, reason string, metadata map[string]any) {
	feature, reason = "create_procedure.body", spec.UnsupportedUnauditedReason
	metadata = map[string]any{"aspect": "option"}
	if dialect == spec.DialectTiDB {
		feature, reason, metadata = "create_procedure", spec.UnsupportedVendorBoundaryReason, map[string]any{"boundary": "vendor"}
		if !create {
			feature = "drop_procedure"
		}
	}
	return feature, reason, metadata
}

func t05A8AssertIdentities(t *testing.T, result report.Result, sql string) {
	t.Helper()
	lines := strings.Split(sql, "\n")
	if len(result.Statements) != len(lines) {
		t.Fatalf("statements = %d, want %d", len(result.Statements), len(lines))
	}
	for i, statement := range result.Statements {
		if statement.Index != i || statement.Kind != "ddl" || statement.RawSQL != lines[i] ||
			statement.NormalizedSQL != strings.TrimSuffix(lines[i], ";") {
			t.Fatalf("statement %d identity = %+v", i, statement)
		}
	}
}

func t05A8AssertBoundaryItem(t *testing.T, result report.Result, dialect spec.Dialect, create bool, index, statementIndex int) {
	t.Helper()
	feature, reason, metadata := t05A8Unsupported(dialect, create)
	item := result.Unsupported[index]
	if item.Index != statementIndex || item.Feature != feature || item.Reason != reason ||
		item.SQL != result.Statements[statementIndex].RawSQL || !reflect.DeepEqual(item.Metadata, metadata) {
		t.Fatalf("procedure boundary = %+v, want feature=%s reason=%s metadata=%v index=%d", item, feature, reason, metadata, statementIndex)
	}
}

func TestT05A8MetadataPremises(t *testing.T) {
	procedures := []struct {
		name, sql string
		create    bool
	}{
		{"create_select", "CREATE PROCEDURE p() SELECT 1;", true},
		{"drop", "DROP PROCEDURE p;", false},
		{"create_delete", "CREATE PROCEDURE p() DELETE FROM t;", true},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, tc := range procedures {
			for _, online := range []bool{true, false} {
				name := "offline"
				if online {
					name = "provider"
				}
				t.Run(string(dialect)+"/"+tc.name+"/"+name, func(t *testing.T) {
					ctx := context.Background()
					sql := t05A8Input(tc.sql)
					provider := &t05A8Provider{}
					request := Request{SQL: sql, Dialect: dialect, Schema: "golden", ConfigPath: t05A7FourRulePolicy(t)}
					if online {
						request.MetadataProvider = provider
					}
					result, err := AuditSQL(ctx, request)
					boundary := t05A8Boundary(dialect, tc.create)
					if boundary {
						if !errors.Is(err, ErrUnsupportedStatement) {
							t.Fatalf("error = %v, want ErrUnsupportedStatement", err)
						}
						if result.Verdict != report.VerdictReview || result.Coverage.Status != report.CoverageIncomplete {
							t.Errorf("aggregate = %s/%s, want review/incomplete", result.Verdict, result.Coverage.Status)
						}
					} else {
						if err != nil {
							t.Fatalf("MySQL DROP error = %v, want nil", err)
						}
						wantVerdict, wantCoverage := report.VerdictPass, report.CoverageComplete
						if !online {
							wantVerdict, wantCoverage = report.VerdictReview, report.CoverageUnverified
						}
						if result.Verdict != wantVerdict || result.Coverage.Status != wantCoverage {
							t.Errorf("aggregate = %s/%s, want %s/%s", result.Verdict, result.Coverage.Status, wantVerdict, wantCoverage)
						}
					}
					t05A8AssertIdentities(t, result, sql)
					for i, statement := range result.Statements {
						wantCoverage := report.CoverageComplete
						if i == 0 && !online {
							wantCoverage = report.CoverageUnverified
						}
						if i == 1 && boundary {
							wantCoverage = report.CoverageIncomplete
						}
						var wantGaps []rule.EvidenceGap
						if i == 0 && !online {
							wantGaps = []rule.EvidenceGap{t05A8GapCreate}
						}
						if statement.Coverage.Status != wantCoverage || len(statement.Findings) != 0 ||
							!reflect.DeepEqual(statement.EvidenceGaps, wantGaps) || statement.Impact != nil {
							t.Errorf("statement %d = %s findings=%+v gaps=%+v impact=%+v, want %s gaps=%v no findings/impact",
								i, statement.Coverage.Status, statement.Findings, statement.EvidenceGaps, statement.Impact, wantCoverage, wantGaps)
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
						t05A8AssertBoundaryItem(t, result, dialect, tc.create, 0, 1)
					}
					if online && !reflect.DeepEqual(provider.events, []string{"instance", "snapshot:golden.t"}) {
						t.Errorf("provider events = %v, want [instance snapshot:golden.t]", provider.events)
					}
					if len(result.GlobalFindings) != 0 || result.RuleSummary == nil ||
						result.RuleSummary.Loaded != 4 || result.RuleSummary.Applicable != 4 {
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
				})
			}
		}
	}
}

func t05A8SeedSnapshots() map[string]*spec.TableSnapshot {
	cardinality := int64(37)
	return map[string]*spec.TableSnapshot{
		"golden.t": {
			Schema:     "golden",
			Exists:     true,
			Table:      &spec.Table{Schema: "golden", Name: "t"},
			Columns:    []spec.Column{{Name: "id", Type: "int"}, {Name: "c", Type: "int"}},
			PrimaryKey: &spec.Index{Name: "PRIMARY", Columns: []string{"id"}, Kind: spec.IndexKindPrimary},
			Indexes: []spec.Index{
				{Name: "ix_seed", Columns: []string{"c"}, Kind: spec.IndexKindSecondary, Cardinality: &cardinality},
			},
			Options:            map[string]string{"table_rows": "123", "auto_increment": "456"},
			ConstraintsUnknown: true,
		},
		"other.u": {
			Schema:            "other",
			Exists:            true,
			Table:             &spec.Table{Schema: "other", Name: "u"},
			Columns:           []spec.Column{{Name: "id", Type: "int"}},
			Options:           map[string]string{"table_rows": "789"},
			PrimaryKeyUnknown: true,
			IndexesUnknown:    true,
			Constraints:       []spec.Constraint{},
		},
	}
}

func t05A8SnapshotEntries(state *batchState) map[batchTableKey]batchTableEntry {
	out := make(map[batchTableKey]batchTableEntry, len(state.entries))
	for key, entry := range state.entries {
		copied := *entry
		if entry.shape != nil {
			copied.shape = cloneTableSnapshot(entry.shape)
		}
		out[key] = copied
	}
	return out
}

func t05A8AssertEntriesEqual(t *testing.T, before map[batchTableKey]batchTableEntry, state *batchState) {
	t.Helper()
	if len(state.entries) != len(before) {
		t.Fatalf("entries count = %d, want %d: %+v", len(state.entries), len(before), state.entries)
	}
	for key, want := range before {
		got, ok := state.entries[key]
		if !ok {
			t.Fatalf("entry %v removed", key)
		}
		if got.state != want.state || got.displaySchema != want.displaySchema || got.displayTable != want.displayTable ||
			!reflect.DeepEqual(got.shape, want.shape) {
			t.Fatalf("entry %v = %+v, want %+v", key, *got, want)
		}
	}
}

func TestT05A8DefinitionsAndIdentity(t *testing.T) {
	variants := []struct{ name, procedure string }{
		{"create_select", "CREATE PROCEDURE p() SELECT 1;"},
		{"create_delete_t", "CREATE PROCEDURE p() DELETE FROM t;"},
		{"create_delete_other_u", "CREATE PROCEDURE p() DELETE FROM other.u;"},
		{"create_named_t", "CREATE PROCEDURE t() SELECT 1;"},
		{"drop_p", "DROP PROCEDURE p;"},
		{"drop_named_t", "DROP PROCEDURE t;"},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, tc := range variants {
			t.Run(string(dialect)+"/"+tc.name, func(t *testing.T) {
				ctx := context.Background()
				expectedSnapshots := t05A8SeedSnapshots()
				provider := &t05A8Provider{snapshots: t05A8SeedSnapshots()}
				state := newBatchState(dialect, "golden", provider)

				earlierT, err := state.preState(ctx, "golden", spec.Table{Name: "t"})
				if err != nil {
					t.Fatal(err)
				}
				earlierU, err := state.preState(ctx, "other", spec.Table{Name: "u"})
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(provider.events, []string{"snapshot:golden.t", "snapshot:other.u"}) {
					t.Fatalf("seed events = %v, want two snapshot reads", provider.events)
				}
				if earlierT.Options["table_rows"] != "123" || earlierT.Options["auto_increment"] != "456" ||
					len(earlierT.Indexes) != 1 || earlierT.Indexes[0].Cardinality == nil || *earlierT.Indexes[0].Cardinality != 37 ||
					!earlierT.ConstraintsUnknown || earlierT.Constraints != nil {
					t.Fatalf("golden.t projection = %+v, want literal seeded members", earlierT)
				}
				if !earlierU.PrimaryKeyUnknown || earlierU.PrimaryKey != nil ||
					!earlierU.IndexesUnknown || earlierU.Indexes != nil ||
					earlierU.ConstraintsUnknown || len(earlierU.Constraints) != 0 ||
					earlierU.Options["table_rows"] != "789" {
					t.Fatalf("other.u projection = %+v, want literal seeded members", earlierU)
				}
				if !reflect.DeepEqual(earlierT, provider.snapshots["golden.t"]) ||
					!reflect.DeepEqual(earlierU, provider.snapshots["other.u"]) {
					t.Fatal("projections diverge from provider originals")
				}
				state.invalidatedSchemas["retired"] = struct{}{}
				beforeEntries := t05A8SnapshotEntries(state)
				beforeSchemas := map[string]struct{}{}
				for scope := range state.invalidatedSchemas {
					beforeSchemas[scope] = struct{}{}
				}
				if len(beforeSchemas) != 1 {
					t.Fatalf("seeded schemas = %v, want only retired", beforeSchemas)
				}
				if state.contaminated {
					t.Fatal("precondition: state already contaminated")
				}

				parsed, parseErr := Parse(ctx, t05A8Input(tc.procedure), dialect)
				if parseErr != nil {
					t.Fatal(parseErr)
				}
				extracted, extractErr := Extract(ctx, parsed)
				if extractErr != nil {
					t.Fatal(extractErr)
				}
				procedure := extracted[1]
				procedureBytes, marshalErr := json.Marshal(procedure)
				if marshalErr != nil {
					t.Fatal(marshalErr)
				}
				var expectedStatement spec.Statement
				if unmarshalErr := json.Unmarshal(procedureBytes, &expectedStatement); unmarshalErr != nil {
					t.Fatal(unmarshalErr)
				}
				var unsupportedPtr *spec.UnsupportedDetail
				var unsupportedCopy spec.UnsupportedDetail
				if procedure.Unsupported != nil {
					unsupportedPtr = procedure.Unsupported
					unsupportedCopy = *procedure.Unsupported
				}
				enriched, enrichErr := state.enrichStatements(ctx, &MetadataRequest{Schema: "golden", Provider: provider},
					extracted[1:2], parsed.failures, &spec.InstanceFacts{}, auditLimits{orderedStatements: 1})
				if enrichErr != nil {
					t.Fatal(enrichErr)
				}
				if len(enriched) != 1 {
					t.Fatalf("enriched = %d, want 1", len(enriched))
				}

				if state.contaminated {
					t.Error("procedure lifecycle contaminated outer state")
				}
				t05A8AssertEntriesEqual(t, beforeEntries, state)
				if !reflect.DeepEqual(state.invalidatedSchemas, beforeSchemas) {
					t.Errorf("invalidatedSchemas = %v, want unchanged %v", state.invalidatedSchemas, beforeSchemas)
				}
				if key := state.keyFor("golden", spec.Table{Name: "p"}); state.entries[key] != nil {
					t.Errorf("procedure name created table entry %v", key)
				}
				if key := state.keyFor("golden", spec.Table{Name: "t"}); state.entries[key] == nil {
					t.Error("golden.t entry removed")
				}
				if key := state.keyFor("other", spec.Table{Name: "u"}); state.entries[key] == nil {
					t.Error("other.u entry removed")
				}
				if !reflect.DeepEqual(provider.events, []string{"snapshot:golden.t", "snapshot:other.u"}) {
					t.Errorf("provider events after procedure = %v, want unchanged seed reads", provider.events)
				}
				if !reflect.DeepEqual(provider.snapshots, expectedSnapshots) {
					t.Error("provider-owned snapshots mutated")
				}
				if !reflect.DeepEqual(earlierT, expectedSnapshots["golden.t"]) || !reflect.DeepEqual(earlierU, expectedSnapshots["other.u"]) {
					t.Error("earlier pre-state projections diverged from expected literals")
				}
				if earlierT.Options["table_rows"] != "123" || earlierT.Options["auto_increment"] != "456" ||
					*earlierT.Indexes[0].Cardinality != 37 ||
					!earlierT.ConstraintsUnknown || earlierT.PrimaryKeyUnknown || earlierT.IndexesUnknown ||
					!earlierU.PrimaryKeyUnknown || !earlierU.IndexesUnknown || earlierU.ConstraintsUnknown ||
					earlierU.Options["table_rows"] != "789" {
					t.Errorf("post-procedure literals changed: t=%+v u=%+v", earlierT, earlierU)
				}
				if afterBytes, marshalErr := json.Marshal(extracted[1]); marshalErr != nil || !bytes.Equal(afterBytes, procedureBytes) {
					t.Errorf("extracted procedure mutated by enrichment: %v", marshalErr)
				}
				if extracted[1].ResourceLimit != nil {
					t.Error("procedure gained a resource-limit marker")
				}
				if unsupportedPtr != nil && (enriched[0].Unsupported != unsupportedPtr || !reflect.DeepEqual(*enriched[0].Unsupported, unsupportedCopy)) {
					t.Errorf("procedure unsupported identity = %+v, want original pointer/value", enriched[0].Unsupported)
				}
				got := enriched[0]
				if got.Kind != expectedStatement.Kind || got.Dialect != expectedStatement.Dialect ||
					got.RawSQL != expectedStatement.RawSQL || got.NormalizedSQL != expectedStatement.NormalizedSQL ||
					got.Line != expectedStatement.Line || got.Column != expectedStatement.Column ||
					!reflect.DeepEqual(got.Warnings, expectedStatement.Warnings) ||
					!reflect.DeepEqual(got.DDL, expectedStatement.DDL) ||
					!reflect.DeepEqual(got.Unsupported, expectedStatement.Unsupported) {
					t.Errorf("enriched procedure = %+v, want original fields %+v", got, expectedStatement)
				}

				attached := attachImpactEstimatesWithPlanner(ctx, provider, enriched)
				if !reflect.DeepEqual(provider.events, []string{"snapshot:golden.t", "snapshot:other.u"}) {
					t.Errorf("planner touched provider: %v", provider.events)
				}
				if attached[0].DML != nil || reportImpact(attached[0]) != nil {
					t.Errorf("procedure gained dml/impact: %+v %+v", attached[0].DML, reportImpact(attached[0]))
				}
			})
		}
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, tc := range []struct {
			name, procedure string
			create          bool
		}{
			{"create_named_t", "CREATE PROCEDURE t() SELECT 1;", true},
			{"drop_named_t", "DROP PROCEDURE t;", false},
		} {
			t.Run(string(dialect)+"/audit/"+tc.name, func(t *testing.T) {
				provider := &t05A8Provider{}
				result, err := AuditSQL(context.Background(), Request{
					SQL: t05A8Input(tc.procedure), Dialect: dialect, Schema: "golden",
					ConfigPath: t05A7FourRulePolicy(t), MetadataProvider: provider,
				})
				boundary := t05A8Boundary(dialect, tc.create)
				if boundary {
					if !errors.Is(err, ErrUnsupportedStatement) {
						t.Fatalf("error = %v, want ErrUnsupportedStatement", err)
					}
					if result.Verdict != report.VerdictReview || result.Coverage.Status != report.CoverageIncomplete {
						t.Errorf("aggregate = %s/%s, want review/incomplete", result.Verdict, result.Coverage.Status)
					}
					if len(result.Unsupported) != 1 {
						t.Fatalf("unsupported = %+v, want 1", result.Unsupported)
					}
					t05A8AssertBoundaryItem(t, result, dialect, tc.create, 0, 1)
				} else if err != nil || result.Verdict != report.VerdictPass || result.Coverage.Status != report.CoverageComplete || len(result.Unsupported) != 0 {
					t.Errorf("MySQL DROP = %s/%s unsupported=%v err=%v, want pass/complete none", result.Verdict, result.Coverage.Status, result.Unsupported, err)
				}
				for i, statement := range result.Statements {
					if i == 1 {
						continue
					}
					if statement.Coverage.Status != report.CoverageComplete || len(statement.EvidenceGaps) != 0 || len(statement.Findings) != 0 {
						t.Errorf("follower %d = %s findings=%+v gaps=%+v, want complete and empty", i, statement.Coverage.Status, statement.Findings, statement.EvidenceGaps)
					}
				}
				if !reflect.DeepEqual(provider.events, []string{"instance", "snapshot:golden.t"}) {
					t.Errorf("provider events = %v, want one golden.t read and no procedure names", provider.events)
				}
			})
		}
	}
}

func t05A8ValidProcedure() spec.Statement {
	return spec.Statement{
		Kind:    spec.KindDDL,
		Dialect: spec.DialectMySQL,
		DDL:     &spec.DDL{Operation: spec.DDLOperationCreateProcedure},
	}
}

func TestT05A8WhitelistAndExecution(t *testing.T) {
	executables := []struct{ name, sql, feature string }{
		{"execute", "EXECUTE stmt;", "execute_prepared"},
		{"do", "DO 1;", "do"},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, tc := range executables {
			t.Run(string(dialect)+"/"+tc.name, func(t *testing.T) {
				sql := t05A8Input(tc.sql)
				provider := &t05A8Provider{}
				result, err := AuditSQL(context.Background(), Request{
					SQL: sql, Dialect: dialect, Schema: "golden",
					ConfigPath: t05A7FourRulePolicy(t), MetadataProvider: provider,
				})
				if !errors.Is(err, ErrUnsupportedStatement) {
					t.Fatalf("error = %v, want ErrUnsupportedStatement", err)
				}
				lines := strings.Split(sql, "\n")
				if len(result.Statements) != len(lines) {
					t.Fatalf("statements = %d, want %d", len(result.Statements), len(lines))
				}
				for i, statement := range result.Statements {
					wantKind := "ddl"
					if i == 1 {
						wantKind = "unknown"
					}
					if statement.Index != i || statement.Kind != wantKind || statement.RawSQL != lines[i] ||
						statement.NormalizedSQL != strings.TrimSuffix(lines[i], ";") {
						t.Fatalf("statement %d identity = %+v", i, statement)
					}
					if len(statement.Findings) != 0 {
						t.Errorf("statement %d findings = %+v, want none", i, statement.Findings)
					}
				}
				first := result.Statements[0]
				if first.Coverage.Status != report.CoverageComplete || len(first.EvidenceGaps) != 0 {
					t.Errorf("first statement = %s gaps=%+v, want complete with none", first.Coverage.Status, first.EvidenceGaps)
				}
				blocked := result.Statements[1]
				if blocked.Kind != "unknown" || blocked.Coverage.Status != report.CoverageIncomplete {
					t.Errorf("executable = kind %s coverage %s, want unknown/incomplete", blocked.Kind, blocked.Coverage.Status)
				}
				for index, want := range t05A8GapFollowers {
					got := result.Statements[index]
					if got.Coverage.Status != report.CoverageUnverified || !reflect.DeepEqual(got.EvidenceGaps, want) {
						t.Errorf("statement %d = %s gaps=%+v, want unverified %+v", index, got.Coverage.Status, got.EvidenceGaps, want)
					}
				}
				if len(result.Unsupported) != 1 {
					t.Fatalf("unsupported = %+v, want 1", result.Unsupported)
				}
				item := result.Unsupported[0]
				if item.Index != 1 || item.Feature != tc.feature || item.Reason != spec.UnsupportedUnauditedReason || item.SQL != blocked.RawSQL {
					t.Errorf("executable boundary = %+v, want feature=%s unaudited at index 1", item, tc.feature)
				}
				if !reflect.DeepEqual(provider.events, []string{"instance", "snapshot:golden.t"}) {
					t.Errorf("provider events = %v, want [instance snapshot:golden.t]", provider.events)
				}
			})
		}
	}

	t.Run("predicate_positive", func(t *testing.T) {
		for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
			for _, operation := range []spec.DDLOperation{spec.DDLOperationCreateProcedure, spec.DDLOperationDropProcedure} {
				statement := t05A8ValidProcedure()
				statement.Dialect = dialect
				statement.DDL.Operation = operation
				if !procedureLifecyclePreservesTableState(statement) {
					t.Errorf("%s/%s: valid procedure not whitelisted", dialect, operation)
				}
			}
		}
		statement := t05A8ValidProcedure()
		statement.RawSQL = "EXECUTE stmt;"
		statement.NormalizedSQL = "arbitrary"
		statement.Warnings = []string{"w"}
		statement.Unsupported = &spec.UnsupportedDetail{Feature: "execute_prepared", Reason: spec.UnsupportedVendorBoundaryReason}
		statement.DDL.ObjectName = "arbitrary_name"
		statement.DDL.Options = map[string]string{"has_body": "true", "params": "true"}
		if !procedureLifecyclePreservesTableState(statement) {
			t.Error("identity must come only from the typed operation, not names/features/body aspects")
		}
		statement.DDL.Targets = []spec.Table{}
		if !procedureLifecyclePreservesTableState(statement) {
			t.Error("empty-but-nonnil Targets must stay whitelisted")
		}
	})

	negatives := []struct {
		name   string
		mutate func(*spec.Statement)
	}{
		{"dialect_postgresql", func(s *spec.Statement) { s.Dialect = spec.DialectPostgreSQL }},
		{"dialect_unknown", func(s *spec.Statement) { s.Dialect = spec.DialectUnknown }},
		{"kind_unknown", func(s *spec.Statement) { s.Kind = spec.KindUnknown }},
		{"kind_dml", func(s *spec.Statement) { s.Kind = spec.KindDML }},
		{"ddl_nil", func(s *spec.Statement) { s.DDL = nil }},
		{"operation_unknown", func(s *spec.Statement) { s.DDL.Operation = spec.DDLOperationUnknown }},
		{"create_function", func(s *spec.Statement) { s.DDL.Operation = spec.DDLOperationCreateFunction }},
		{"drop_function", func(s *spec.Statement) { s.DDL.Operation = spec.DDLOperationDropFunction }},
		{"create_trigger", func(s *spec.Statement) { s.DDL.Operation = spec.DDLOperationCreateTrigger }},
		{"drop_trigger", func(s *spec.Statement) { s.DDL.Operation = spec.DDLOperationDropTrigger }},
		{"create_event", func(s *spec.Statement) { s.DDL.Operation = spec.DDLOperation("create_event") }},
		{"call", func(s *spec.Statement) { s.DDL.Operation = spec.DDLOperation("call") }},
		{"alter_procedure", func(s *spec.Statement) { s.DDL.Operation = spec.DDLOperation("alter_procedure") }},
		{"dml_nonnil", func(s *spec.Statement) { s.DML = &spec.DML{} }},
		{"table_named", func(s *spec.Statement) { s.DDL.Table = &spec.Table{Name: "t"} }},
		{"table_empty", func(s *spec.Statement) { s.DDL.Table = &spec.Table{} }},
		{"targets_nonempty", func(s *spec.Statement) { s.DDL.Targets = []spec.Table{{Name: "t"}} }},
		{"resource_limit", func(s *spec.Statement) { s.ResourceLimit = &spec.AuditResourceLimit{} }},
		{"lookalike", func(s *spec.Statement) {
			s.DDL.Operation = spec.DDLOperationCreateTable
			s.RawSQL = "CREATE PROCEDURE p() SELECT 1;"
			s.Unsupported = &spec.UnsupportedDetail{Feature: "create_procedure"}
		}},
	}
	for _, tc := range negatives {
		t.Run("predicate_negative/"+tc.name, func(t *testing.T) {
			statement := t05A8ValidProcedure()
			tc.mutate(&statement)
			if procedureLifecyclePreservesTableState(statement) {
				t.Errorf("mutation %s must not match the procedure whitelist", tc.name)
			}
		})
	}

	t.Run("bound_target_not_preserved", func(t *testing.T) {
		ctx := context.Background()
		provider := &t05A8Provider{snapshots: t05A8SeedSnapshots()}
		state := newBatchState(spec.DialectMySQL, "golden", provider)
		if _, err := state.preState(ctx, "golden", spec.Table{Name: "t"}); err != nil {
			t.Fatal(err)
		}
		if _, err := state.preState(ctx, "other", spec.Table{Name: "u"}); err != nil {
			t.Fatal(err)
		}
		before := t05A8SnapshotEntries(state)
		statement := spec.Statement{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectMySQL,
			DDL:     &spec.DDL{Operation: spec.DDLOperationCreateProcedure, Table: &spec.Table{Name: "t"}},
			Unsupported: &spec.UnsupportedDetail{
				Feature: "create_procedure", Reason: spec.UnsupportedVendorBoundaryReason,
			},
		}
		if procedureLifecyclePreservesTableState(statement) {
			t.Fatal("table-bound procedure must not match the whitelist")
		}
		if err := state.apply(ctx, statement); err != nil {
			t.Fatal(err)
		}
		tKey, uKey := state.keyFor("golden", spec.Table{Name: "t"}), state.keyFor("other", spec.Table{Name: "u"})
		if got := state.entries[tKey]; got == nil || got.state != tableUnknown {
			t.Errorf("bound target t = %+v, want tableUnknown after generic unsupported invalidation", got)
		}
		if got := state.entries[uKey]; got == nil || got.state != tablePresent || !reflect.DeepEqual(got.shape, before[uKey].shape) {
			t.Errorf("unrelated u = %+v, want untouched present entry", got)
		}
		if state.contaminated {
			t.Error("bound-target invalidation must not contaminate")
		}
		if !reflect.DeepEqual(provider.events, []string{"snapshot:golden.t", "snapshot:other.u"}) {
			t.Errorf("provider events = %v, want unchanged seed reads", provider.events)
		}
	})
}

func TestT05A8ContaminationAndBudget(t *testing.T) {
	pollutedSQL := "CREATE TABLE t (id INT PRIMARY KEY);\nEXECUTE stmt;\nCREATE PROCEDURE p() SELECT 1;\nALTER TABLE t ADD COLUMN c INT;\nCREATE INDEX idx_c ON t(c);"
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect)+"/prior_pollution", func(t *testing.T) {
			ctx := context.Background()
			provider := &t05A8Provider{}
			result, err := AuditSQL(ctx, Request{
				SQL: pollutedSQL, Dialect: dialect, Schema: "golden",
				ConfigPath: t05A7FourRulePolicy(t), MetadataProvider: provider,
			})
			if !errors.Is(err, ErrUnsupportedStatement) {
				t.Fatalf("error = %v, want ErrUnsupportedStatement", err)
			}
			if len(result.Statements) != 5 {
				t.Fatalf("statements = %d, want 5", len(result.Statements))
			}
			for index, want := range t05A8GapFollowers {
				got := result.Statements[index+1]
				if got.Coverage.Status != report.CoverageUnverified || !reflect.DeepEqual(got.EvidenceGaps, want) {
					t.Errorf("statement %d = %s gaps=%+v, want unverified %+v", index+1, got.Coverage.Status, got.EvidenceGaps, want)
				}
			}
			feature, reason, metadata := t05A8Unsupported(dialect, true)
			foundProcedure := false
			for _, item := range result.Unsupported {
				if item.Feature == feature && item.Index == 2 {
					if item.Reason != reason || !reflect.DeepEqual(item.Metadata, metadata) || item.SQL != result.Statements[2].RawSQL {
						t.Errorf("procedure boundary = %+v, want %s/%s", item, feature, reason)
					}
					foundProcedure = true
				}
			}
			if !foundProcedure {
				t.Errorf("procedure boundary missing from %+v", result.Unsupported)
			}
			if !reflect.DeepEqual(provider.events, []string{"instance", "snapshot:golden.t"}) {
				t.Errorf("provider events = %v, want [instance snapshot:golden.t]", provider.events)
			}

			parsed, parseErr := Parse(ctx, pollutedSQL, dialect)
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			extracted, extractErr := Extract(ctx, parsed)
			if extractErr != nil {
				t.Fatal(extractErr)
			}
			inner := &t05A8Provider{}
			state := newBatchState(dialect, "golden", inner)
			facts, _ := inner.LoadInstanceFacts(ctx, dialect, "golden")
			if _, enrichErr := state.enrichStatements(ctx, &MetadataRequest{Schema: "golden", Provider: inner},
				extracted[:2], parsed.failures, facts, auditLimits{orderedStatements: 2}); enrichErr != nil {
				t.Fatal(enrichErr)
			}
			if !state.contaminated {
				t.Fatal("precondition: EXECUTE must contaminate")
			}
			beforeEntries := t05A8SnapshotEntries(state)
			beforeSchemas := map[string]struct{}{}
			for scope := range state.invalidatedSchemas {
				beforeSchemas[scope] = struct{}{}
			}
			beforeEvents := append([]string(nil), inner.events...)
			if _, enrichErr := state.enrichStatements(ctx, &MetadataRequest{Schema: "golden", Provider: inner},
				extracted[2:3], parsed.failures, facts, auditLimits{orderedStatements: 1}); enrichErr != nil {
				t.Fatal(enrichErr)
			}
			if !state.contaminated {
				t.Error("procedure must not clear contamination")
			}
			t05A8AssertEntriesEqual(t, beforeEntries, state)
			if !reflect.DeepEqual(state.invalidatedSchemas, beforeSchemas) || !reflect.DeepEqual(inner.events, beforeEvents) {
				t.Error("procedure changed state or provider ledger under prior pollution")
			}
		})

		t.Run(string(dialect)+"/schema_invalidation", func(t *testing.T) {
			ctx := context.Background()
			sql := "CREATE TABLE t (id INT PRIMARY KEY);\nDROP DATABASE golden;\nCREATE PROCEDURE p() SELECT 1;\nALTER TABLE t ADD COLUMN c INT;\nCREATE INDEX idx_c ON t(c);"
			parsed, parseErr := Parse(ctx, sql, dialect)
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			extracted, extractErr := Extract(ctx, parsed)
			if extractErr != nil {
				t.Fatal(extractErr)
			}
			provider := &t05A8Provider{}
			state := newBatchState(dialect, "golden", provider)
			facts, _ := provider.LoadInstanceFacts(ctx, dialect, "golden")
			if _, enrichErr := state.enrichStatements(ctx, &MetadataRequest{Schema: "golden", Provider: provider},
				extracted[:2], parsed.failures, facts, auditLimits{orderedStatements: 2}); enrichErr != nil {
				t.Fatal(enrichErr)
			}
			if state.contaminated {
				t.Fatal("schema invalidation must not contaminate")
			}
			if _, ok := state.invalidatedSchemas["golden"]; !ok {
				t.Fatal("DROP DATABASE must record invalidated schema golden")
			}
			beforeEntries := t05A8SnapshotEntries(state)
			beforeSchemas := map[string]struct{}{}
			for scope := range state.invalidatedSchemas {
				beforeSchemas[scope] = struct{}{}
			}
			beforeEvents := append([]string(nil), provider.events...)
			if _, enrichErr := state.enrichStatements(ctx, &MetadataRequest{Schema: "golden", Provider: provider},
				extracted[2:3], parsed.failures, facts, auditLimits{orderedStatements: 1}); enrichErr != nil {
				t.Fatal(enrichErr)
			}
			t05A8AssertEntriesEqual(t, beforeEntries, state)
			if !reflect.DeepEqual(state.invalidatedSchemas, beforeSchemas) {
				t.Errorf("invalidatedSchemas = %v, want unchanged %v", state.invalidatedSchemas, beforeSchemas)
			}
			if !reflect.DeepEqual(provider.events, beforeEvents) {
				t.Errorf("provider events = %v, want unchanged %v", provider.events, beforeEvents)
			}
			if snapshot, err := state.preState(ctx, "golden", spec.Table{Name: "t"}); err != nil || snapshot != nil {
				t.Errorf("tombstoned t pre-state = %+v err %v, want nil", snapshot, err)
			}
			if snapshot, err := state.preState(ctx, "golden", spec.Table{Name: "u"}); err != nil || snapshot != nil {
				t.Errorf("unrelated u under dropped schema = %+v err %v, want nil", snapshot, err)
			}
			if !reflect.DeepEqual(provider.events, beforeEvents) {
				t.Errorf("provider events after reads = %v, want no reload %v", provider.events, beforeEvents)
			}
		})

		t.Run(string(dialect)+"/tombstone_preserved", func(t *testing.T) {
			ctx := context.Background()
			provider := &t05A8Provider{snapshots: t05A8SeedSnapshots()}
			state := newBatchState(dialect, "golden", provider)
			if _, err := state.preState(ctx, "golden", spec.Table{Name: "t"}); err != nil {
				t.Fatal(err)
			}
			if _, err := state.preState(ctx, "other", spec.Table{Name: "u"}); err != nil {
				t.Fatal(err)
			}
			tKey := state.keyFor("golden", spec.Table{Name: "t"})
			uKey := state.keyFor("other", spec.Table{Name: "u"})
			if entry := state.entries[tKey]; entry == nil || entry.state != tablePresent {
				t.Fatalf("seeded t entry = %+v, want present", entry)
			}
			state.invalidateKey(tKey)
			if entry := state.entries[tKey]; entry == nil || entry.state != tableUnknown {
				t.Fatalf("tombstoned t entry = %+v, want unknown", entry)
			}
			if state.contaminated || len(state.invalidatedSchemas) != 0 {
				t.Fatal("precondition: tombstone must not contaminate or invalidate schemas")
			}
			beforeEntries := t05A8SnapshotEntries(state)
			beforeSchemas := map[string]struct{}{}
			for scope := range state.invalidatedSchemas {
				beforeSchemas[scope] = struct{}{}
			}
			beforeEvents := append([]string(nil), provider.events...)
			for _, procSQL := range []string{"CREATE PROCEDURE p() SELECT 1;", "DROP PROCEDURE p;"} {
				parsed, parseErr := Parse(ctx, procSQL, dialect)
				if parseErr != nil {
					t.Fatal(parseErr)
				}
				extracted, extractErr := Extract(ctx, parsed)
				if extractErr != nil {
					t.Fatal(extractErr)
				}
				if _, enrichErr := state.enrichStatements(ctx, &MetadataRequest{Schema: "golden", Provider: provider},
					extracted, parsed.failures, &spec.InstanceFacts{}, auditLimits{orderedStatements: 1}); enrichErr != nil {
					t.Fatal(enrichErr)
				}
			}
			if state.contaminated {
				t.Error("procedure lifecycle must not contaminate")
			}
			t05A8AssertEntriesEqual(t, beforeEntries, state)
			if !reflect.DeepEqual(state.invalidatedSchemas, beforeSchemas) {
				t.Errorf("invalidatedSchemas = %v, want unchanged %v", state.invalidatedSchemas, beforeSchemas)
			}
			if entry := state.entries[tKey]; entry == nil || entry.state != tableUnknown {
				t.Errorf("t entry = %+v, want still unknown", entry)
			}
			if entry := state.entries[uKey]; entry == nil || entry.state != tablePresent ||
				!reflect.DeepEqual(entry.shape, beforeEntries[uKey].shape) {
				t.Errorf("u entry = %+v, want untouched present", entry)
			}
			if snapshot, err := state.preState(ctx, "golden", spec.Table{Name: "t"}); err != nil || snapshot != nil {
				t.Errorf("tombstoned t pre-state = %+v err %v, want nil", snapshot, err)
			}
			if !reflect.DeepEqual(provider.events, beforeEvents) {
				t.Errorf("provider events = %v, want unchanged %v", provider.events, beforeEvents)
			}
		})
	}

	procedures := []struct {
		name, sql string
		create    bool
	}{
		{"create", "CREATE PROCEDURE p() SELECT 1;", true},
		{"drop", "DROP PROCEDURE p;", false},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, tc := range procedures {
			t.Run(string(dialect)+"/budget/"+tc.name, func(t *testing.T) {
				ctx := context.Background()
				sql := t05A8Input(tc.sql)
				lines := strings.Split(sql, "\n")
				policy := t05A7FourRulePolicy(t)
				control, controlErr := t05A7Audit(t, sql, dialect, &t05AbsentProvider{}, policy, 4)
				boundary := t05A8Boundary(dialect, tc.create)
				if boundary {
					if !errors.Is(controlErr, ErrUnsupportedStatement) {
						t.Fatalf("limit4 control error = %v, want ErrUnsupportedStatement", controlErr)
					}
					if control.Verdict != report.VerdictReview || control.Coverage.Status != report.CoverageIncomplete {
						t.Fatalf("limit4 control = %s/%s, want review/incomplete", control.Verdict, control.Coverage.Status)
					}
				} else if controlErr != nil || control.Verdict != report.VerdictPass || control.Coverage.Status != report.CoverageComplete {
					t.Fatalf("limit4 control = %s/%s err %v, want pass/complete nil", control.Verdict, control.Coverage.Status, controlErr)
				}
				wantLimit4Items := 0
				if boundary {
					wantLimit4Items = 1
				}
				if len(control.Unsupported) != wantLimit4Items {
					t.Fatalf("limit4 unsupported = %+v, want %d", control.Unsupported, wantLimit4Items)
				}

				for _, limit := range []int{1, 2, 3, 4} {
					t.Run(fmt.Sprintf("limit%d", limit), func(t *testing.T) {
						provider := &t05AbsentProvider{}
						result, err := t05A7Audit(t, sql, dialect, provider, policy, limit)
						if len(result.Statements) != 4 {
							t.Fatalf("statements = %d, want 4", len(result.Statements))
						}
						resources := t05A7ResourceEntries(result)
						if len(resources) != 4-limit {
							t.Fatalf("resource entries = %d, want %d: %+v", len(resources), 4-limit, resources)
						}
						for offset, item := range resources {
							expectedIndex := limit + offset
							t05A7AssertResourceEntry(t, item, result.Statements[expectedIndex], expectedIndex, limit)
						}
						for i := limit; i < 4; i++ {
							t05A7AssertBlockedStatement(t, result.Statements[i], i, lines[i])
						}
						if limit < 4 {
							if !errors.Is(err, ErrUnsupportedStatement) {
								t.Fatalf("error = %v, want ErrUnsupportedStatement", err)
							}
							if result.Verdict != report.VerdictReview || result.Coverage.Status != report.CoverageIncomplete {
								t.Errorf("limit%d aggregate = %s/%s, want review/incomplete", limit, result.Verdict, result.Coverage.Status)
							}
						} else if boundary && !errors.Is(err, ErrUnsupportedStatement) {
							t.Fatalf("error = %v, want ErrUnsupportedStatement", err)
						} else if !boundary && err != nil {
							t.Fatalf("error = %v, want nil", err)
						}
						if !reflect.DeepEqual(result.Statements[:limit], control.Statements[:limit]) ||
							!reflect.DeepEqual(result.GlobalFindings, control.GlobalFindings) {
							t.Error("admitted prefix differs from independent limit4 control")
						}
						if !reflect.DeepEqual(provider.calls, []string{"golden.t"}) {
							t.Errorf("outer provider reads = %v, want only golden.t", provider.calls)
						}

						wantFeatures := map[int][]string{1: {spec.AuditResourceLimitFeature}}
						if tc.create {
							wantFeatures[1] = []string{"create_procedure.body", spec.AuditResourceLimitFeature}
							if dialect == spec.DialectTiDB {
								wantFeatures[1] = []string{"create_procedure", "create_procedure.body", spec.AuditResourceLimitFeature}
							}
						} else if dialect == spec.DialectTiDB {
							wantFeatures[1] = []string{"drop_procedure", spec.AuditResourceLimitFeature}
						}
						if limit > 1 {
							feature, _, _ := t05A8Unsupported(dialect, tc.create)
							if boundary {
								wantFeatures[1] = []string{feature}
							} else {
								wantFeatures[1] = nil
							}
						}
						var gotFeatures []string
						for _, item := range result.Unsupported {
							if item.Index == 1 {
								gotFeatures = append(gotFeatures, item.Feature)
							}
						}
						if !reflect.DeepEqual(gotFeatures, wantFeatures[1]) {
							t.Errorf("index1 unsupported features = %v, want %v", gotFeatures, wantFeatures[1])
						}

						parsed, parseErr := Parse(ctx, sql, dialect)
						if parseErr != nil {
							t.Fatal(parseErr)
						}
						extracted, extractErr := Extract(ctx, parsed)
						if extractErr != nil {
							t.Fatal(extractErr)
						}
						inner := &t05A8Provider{}
						state := newBatchState(dialect, "golden", inner)
						facts, _ := inner.LoadInstanceFacts(ctx, dialect, "golden")
						enriched, enrichErr := state.enrichStatements(ctx, &MetadataRequest{Schema: "golden", Provider: inner},
							extracted, parsed.failures, facts, auditLimits{orderedStatements: limit})
						if enrichErr != nil {
							t.Fatal(enrichErr)
						}
						for i := limit; i < 4; i++ {
							marker := enriched[i].ResourceLimit
							if marker == nil || marker.Limit != limit || marker.Consumed != limit {
								t.Errorf("enriched[%d].ResourceLimit = %+v, want limit=consumed=%d", i, marker, limit)
							}
						}
						if state.contaminated != (limit < 4) {
							t.Errorf("contaminated = %v, want %v", state.contaminated, limit < 4)
						}
						entry := state.entries[state.keyFor("golden", spec.Table{Name: "t"})]
						if entry == nil || entry.shape == nil {
							t.Fatal("missing outer table state")
						}
						wantColumns := []string{"id"}
						if limit >= 3 {
							wantColumns = []string{"id", "c"}
						}
						if got := t05A6ColumnNames(entry.shape); !reflect.DeepEqual(got, wantColumns) {
							t.Errorf("final columns = %v, want %v", got, wantColumns)
						}
						wantIndex := limit >= 4
						foundIndex := false
						for _, index := range entry.shape.Indexes {
							if index.Name == "idx_c" && reflect.DeepEqual(index.Columns, []string{"c"}) {
								foundIndex = true
							}
						}
						if foundIndex != wantIndex {
							t.Errorf("idx_c(c) present = %v, want %v", foundIndex, wantIndex)
						}
						if !reflect.DeepEqual(inner.events, []string{"instance", "snapshot:golden.t"}) {
							t.Errorf("provider events = %v, want [instance snapshot:golden.t]", inner.events)
						}
					})
				}
			})
		}
	}
}

func TestT05A8ErrorsAndConsumers(t *testing.T) {
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, tc := range []struct {
			name        string
			sql         string
			invalidLine int
			retained    []int
		}{
			{"failure_before", "CREATE TABLE t (id INT PRIMARY KEY);\nTHIS IS NOT SQL;\nCREATE PROCEDURE p() SELECT 1;\nALTER TABLE t ADD COLUMN c INT;\nCREATE INDEX idx_c ON t(c);", 2, []int{1, 3, 4, 5}},
			{"failure_after", t05A8Input("CREATE PROCEDURE p() SELECT 1;") + "\nTHIS IS NOT SQL;", 5, []int{1, 2, 3, 4}},
		} {
			t.Run(string(dialect)+"/"+tc.name, func(t *testing.T) {
				ctx := context.Background()
				provider := &t05A8Provider{}
				result, err := AuditSQL(ctx, Request{
					SQL: tc.sql, Dialect: dialect, Schema: "golden",
					ConfigPath: t05A7FourRulePolicy(t), MetadataProvider: provider,
				})
				if !errors.Is(err, errParserUnsupported) {
					t.Fatalf("error = %v, want errParserUnsupported", err)
				}
				if errors.Is(err, ErrUnsupportedStatement) {
					t.Fatal("parser failure must not surface as ErrUnsupportedStatement")
				}
				if len(result.Statements) != 4 {
					t.Fatalf("statements = %d, want 4 retained", len(result.Statements))
				}
				var parserDiagnostics, unsupportedDiagnostics int
				for _, diagnostic := range result.Diagnostics {
					switch diagnostic.Classification {
					case spec.DiagnosticParserError:
						parserDiagnostics++
						if diagnostic.Line != tc.invalidLine || diagnostic.Column != 1 {
							t.Errorf("parser diagnostic = %d:%d, want %d:1", diagnostic.Line, diagnostic.Column, tc.invalidLine)
						}
					case DiagnosticUnsupportedStatement:
						unsupportedDiagnostics++
					}
				}
				if parserDiagnostics != 1 || unsupportedDiagnostics != 1 {
					t.Errorf("diagnostics = %d parser + %d unsupported, want 1+1: %+v", parserDiagnostics, unsupportedDiagnostics, result.Diagnostics)
				}
				if len(result.Unsupported) != 1 {
					t.Fatalf("unsupported = %+v, want exactly one procedure boundary", result.Unsupported)
				}
				t05A8AssertBoundaryItem(t, result, dialect, true, 0, 1)
				for _, item := range result.Unsupported {
					if item.Feature == spec.AuditResourceLimitFeature {
						t.Error("parser failure must not fabricate a resource entry")
					}
				}

				parsed, parseErr := Parse(ctx, tc.sql, dialect)
				if parseErr == nil {
					t.Fatal("expected a partial-parse error for the invalid line")
				}
				extracted, extractErr := Extract(ctx, parsed)
				if extractErr != nil {
					t.Fatal(extractErr)
				}
				var gotLines []int
				for _, statement := range extracted {
					gotLines = append(gotLines, statement.Line)
					if statement.Column != 1 {
						t.Errorf("statement column = %d, want 1", statement.Column)
					}
				}
				if !reflect.DeepEqual(gotLines, tc.retained) {
					t.Errorf("retained lines = %v, want %v", gotLines, tc.retained)
				}

				if tc.name == "failure_before" {
					for index, want := range t05A8GapFollowers {
						got := result.Statements[index]
						if got.Coverage.Status != report.CoverageUnverified || !reflect.DeepEqual(got.EvidenceGaps, want) {
							t.Errorf("statement %d = %s gaps=%+v, want unverified %+v", index, got.Coverage.Status, got.EvidenceGaps, want)
						}
					}
				} else {
					control, controlErr := AuditSQL(ctx, Request{
						SQL: t05A8Input("CREATE PROCEDURE p() SELECT 1;"), Dialect: dialect, Schema: "golden",
						ConfigPath: t05A7FourRulePolicy(t), MetadataProvider: &t05A8Provider{},
					})
					if !errors.Is(controlErr, ErrUnsupportedStatement) {
						t.Fatalf("clean control error = %v, want ErrUnsupportedStatement", controlErr)
					}
					for i := range result.Statements {
						if !reflect.DeepEqual(result.Statements[i], control.Statements[i]) {
							t.Errorf("statement %d changed by trailing parser failure: %+v vs %+v", i, result.Statements[i], control.Statements[i])
						}
					}
				}
			})
		}
	}

	t.Run("compound_parser_control_mysql", func(t *testing.T) {
		result, err := AuditSQL(context.Background(), Request{
			SQL: t05A8Input("CREATE PROCEDURE p() BEGIN SELECT 1; END;"), Dialect: spec.DialectMySQL, Schema: "golden",
			ConfigPath: t05A7FourRulePolicy(t), MetadataProvider: &t05A8Provider{},
		})
		if !errors.Is(err, errParserUnsupported) {
			t.Fatalf("error = %v, want errParserUnsupported", err)
		}
		for _, item := range result.Unsupported {
			if strings.Contains(item.Feature, "procedure") {
				t.Errorf("failed procedure text must not yield a procedure boundary: %+v", item)
			}
		}
	})

	t.Run("cancellation", func(t *testing.T) {
		ctx := context.Background()
		parsed, err := Parse(ctx, t05A8Input("CREATE PROCEDURE p() SELECT 1;"), spec.DialectMySQL)
		if err != nil {
			t.Fatal(err)
		}
		extracted, err := Extract(ctx, parsed)
		if err != nil {
			t.Fatal(err)
		}
		procedure := extracted[1]

		for _, tc := range []struct {
			name string
			ctx  func() (context.Context, context.CancelFunc)
		}{
			{"already_canceled", func() (context.Context, context.CancelFunc) {
				base, cancel := context.WithCancel(context.Background())
				cancel()
				return base, cancel
			}},
			{"cancel_on_first_check", func() (context.Context, context.CancelFunc) {
				checking, cancel := t05A7CancelContext(1)
				return checking, cancel
			}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				provider := &t05A8Provider{snapshots: t05A8SeedSnapshots()}
				state := newBatchState(spec.DialectMySQL, "golden", provider)
				if _, err := state.preState(context.Background(), "golden", spec.Table{Name: "t"}); err != nil {
					t.Fatal(err)
				}
				before := t05A8SnapshotEntries(state)
				beforeEvents := append([]string(nil), provider.events...)
				cancelCtx, cancel := tc.ctx()
				t.Cleanup(cancel)
				procedureBefore := procedure
				if err := state.apply(cancelCtx, procedure); !errors.Is(err, context.Canceled) {
					t.Fatalf("apply error = %v, want context.Canceled", err)
				}
				t05A8AssertEntriesEqual(t, before, state)
				if state.contaminated {
					t.Error("canceled apply must not contaminate")
				}
				if !reflect.DeepEqual(provider.events, beforeEvents) || !reflect.DeepEqual(procedure, procedureBefore) {
					t.Error("canceled apply touched provider or statement")
				}
			})
		}

		t.Run("full_core_precanceled", func(t *testing.T) {
			cancelCtx, cancel := context.WithCancel(context.Background())
			cancel()
			t.Cleanup(cancel)
			provider := &t05A8Provider{}
			_, err := auditWithLimits(cancelCtx, Request{
				SQL: t05A8Input("CREATE PROCEDURE p() SELECT 1;"), Dialect: spec.DialectMySQL, Schema: "golden",
				ConfigPath: t05A7FourRulePolicy(t), MetadataProvider: provider,
			}, auditLimits{orderedStatements: 4})
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("error = %v, want context.Canceled", err)
			}
			if len(provider.events) != 0 {
				t.Errorf("provider events = %v, want none", provider.events)
			}
		})

		t.Run("full_core_instance_cancel", func(t *testing.T) {
			cancelCtx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			provider := &t05A8Provider{onInstance: cancel}
			_, err := auditWithLimits(cancelCtx, Request{
				SQL: t05A8Input("CREATE PROCEDURE p() SELECT 1;"), Dialect: spec.DialectMySQL, Schema: "golden",
				ConfigPath: t05A7FourRulePolicy(t), MetadataProvider: provider,
			}, auditLimits{orderedStatements: 4})
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("error = %v, want context.Canceled", err)
			}
			if !reflect.DeepEqual(provider.events, []string{"instance"}) {
				t.Errorf("provider events = %v, want [instance] only", provider.events)
			}
		})
	})

	t.Run("provider_errors", func(t *testing.T) {
		sentinel := errors.New("t05a8 provider sentinel")
		_, err := t05A7Audit(t, t05A8Input("CREATE PROCEDURE p() SELECT 1;"), spec.DialectMySQL,
			&t05A8Provider{instanceErr: sentinel}, t05A7FourRulePolicy(t), 4)
		if !errors.Is(err, sentinel) {
			t.Fatalf("instance error = %v, want sentinel", err)
		}
		_, err = t05A7Audit(t, t05A8Input("CREATE PROCEDURE p() SELECT 1;"), spec.DialectMySQL,
			&t05A8Provider{snapshotErr: sentinel, failTable: "golden.t"}, t05A7FourRulePolicy(t), 4)
		if !errors.Is(err, sentinel) {
			t.Fatalf("snapshot error = %v, want sentinel", err)
		}
		provider := &t05A8Provider{snapshotErr: sentinel, failTable: "golden.never_loaded"}
		result, err := t05A7Audit(t,
			"CREATE TABLE t (id INT PRIMARY KEY);\nCREATE PROCEDURE p() SELECT 1;\nALTER TABLE never_loaded ADD COLUMN c INT;\nCREATE INDEX idx_c ON t(c);",
			spec.DialectMySQL, provider, t05A7FourRulePolicy(t), 1)
		if !errors.Is(err, ErrUnsupportedStatement) || errors.Is(err, sentinel) {
			t.Fatalf("blocked-suffix error = %v, want unsupported not the unread provider sentinel", err)
		}
		for _, event := range provider.events {
			if event == "snapshot:golden.never_loaded" {
				t.Errorf("blocked table was read: %v", provider.events)
			}
		}
		if len(t05A7ResourceEntries(result)) != 3 {
			t.Errorf("resource entries = %+v, want 3", result.Unsupported)
		}
	})

	t.Run("reject_preserved", func(t *testing.T) {
		for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
			t.Run(string(dialect), func(t *testing.T) {
				sql := "CREATE TABLE t (id INT PRIMARY KEY);\n" +
					"ALTER TABLE t ADD COLUMN id INT;\n" +
					"CREATE PROCEDURE p() SELECT 1;\n" +
					"ALTER TABLE t ADD COLUMN c INT;\n" +
					"CREATE INDEX idx_c ON t(c);"
				provider := &t05AbsentProvider{}
				result, err := AuditSQL(context.Background(), Request{
					SQL: sql, Dialect: dialect, Schema: "golden",
					ConfigPath: t05A7FourRulePolicy(t), MetadataProvider: provider,
				})
				if !errors.Is(err, ErrUnsupportedStatement) {
					t.Fatalf("error = %v, want ErrUnsupportedStatement", err)
				}
				if result.Verdict != report.VerdictReject || result.Verdict == report.VerdictReview {
					t.Fatalf("verdict = %s, want reject (never review)", result.Verdict)
				}
				if result.Coverage.Status != report.CoverageIncomplete {
					t.Fatalf("coverage = %s, want incomplete", result.Coverage.Status)
				}
				if len(result.Statements) != 5 {
					t.Fatalf("statements = %d, want 5", len(result.Statements))
				}
				duplicate := result.Statements[1]
				if len(duplicate.Findings) != 1 {
					t.Fatalf("duplicate-ADD findings = %+v, want exactly one blocker", duplicate.Findings)
				}
				finding := duplicate.Findings[0]
				if finding.RuleID != "ddl.alter.add_column.exists.forbid" || finding.Level != rule.LevelBlocker ||
					finding.Metadata["table"] != "t" || finding.Metadata["name"] != "id" {
					t.Fatalf("duplicate-ADD finding = %+v, want exists.forbid blocker bound to t/id", finding)
				}
				if duplicate.Coverage.Status != report.CoverageComplete {
					t.Errorf("duplicate-ADD coverage = %s, want complete", duplicate.Coverage.Status)
				}
				if len(result.Unsupported) != 1 {
					t.Fatalf("unsupported = %+v, want one procedure boundary", result.Unsupported)
				}
				t05A8AssertBoundaryItem(t, result, dialect, true, 0, 2)
				procedure := result.Statements[2]
				if procedure.Coverage.Status != report.CoverageIncomplete || len(procedure.Findings) != 0 || len(procedure.EvidenceGaps) != 0 {
					t.Fatalf("procedure = %s findings=%+v gaps=%+v, want incomplete with none", procedure.Coverage.Status, procedure.Findings, procedure.EvidenceGaps)
				}
				indexGap := []rule.EvidenceGap{{RuleID: "ddl.create_index.columns.exists.require", ReasonCode: "unknown_table_state", RequiredFacts: []string{"target_table.columns"}}}
				for i, want := range map[int][]rule.EvidenceGap{3: t05A8GapFollowers[2], 4: indexGap} {
					got := result.Statements[i]
					if got.Coverage.Status != report.CoverageUnverified || !reflect.DeepEqual(got.EvidenceGaps, want) {
						t.Errorf("statement %d = %s gaps=%+v, want unverified %+v", i, got.Coverage.Status, got.EvidenceGaps, want)
					}
				}
				if !reflect.DeepEqual(provider.calls, []string{"golden.t"}) {
					t.Errorf("provider reads = %v, want only golden.t once", provider.calls)
				}
			})
		}
	})

	t.Run("object_resolver_cache", func(t *testing.T) {
		ctx := context.Background()
		sql := "CREATE DATABASE before_schema;\nCREATE PROCEDURE p() SELECT 1;\nCREATE DATABASE before_schema;"
		for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
			t.Run(string(dialect), func(t *testing.T) {
				parsed, parseErr := Parse(ctx, sql, dialect)
				if parseErr != nil {
					t.Fatal(parseErr)
				}
				extracted, extractErr := Extract(ctx, parsed)
				if extractErr != nil {
					t.Fatal(extractErr)
				}
				if len(extracted) != 3 {
					t.Fatalf("extracted = %d, want 3", len(extracted))
				}
				provider := &t05A8Provider{}
				enriched, err := enrichStatementsWithMetadata(ctx, dialect,
					&MetadataRequest{Schema: "golden", Provider: provider}, extracted, parsed.failures)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(provider.events, []string{"instance", "object:schema:golden.before_schema"}) {
					t.Fatalf("provider events = %v, want one cached schema lookup", provider.events)
				}
				want := []spec.ObjectSnapshot{{Schema: "golden", Type: "schema", Name: "before_schema", Status: spec.MetadataStatusConfirmed, Exists: true}}
				if !reflect.DeepEqual(enriched[0].Metadata.Objects, want) ||
					!reflect.DeepEqual(enriched[2].Metadata.Objects, want) {
					t.Errorf("object projections = %+v / %+v, want %+v", enriched[0].Metadata.Objects, enriched[2].Metadata.Objects, want)
				}
				if len(enriched[1].Metadata.Objects) != 0 {
					t.Errorf("procedure objects = %+v, want none", enriched[1].Metadata.Objects)
				}
			})
		}
	})

	t.Run("params_fixture", func(t *testing.T) {
		for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
			t.Run(string(dialect), func(t *testing.T) {
				provider := &t05A8Provider{}
				result, err := AuditSQL(context.Background(), Request{
					SQL: t05A8Input("CREATE PROCEDURE p(IN x INT) SELECT 1;"), Dialect: dialect, Schema: "golden",
					ConfigPath: t05A7FourRulePolicy(t), MetadataProvider: provider,
				})
				if !errors.Is(err, ErrUnsupportedStatement) {
					t.Fatalf("error = %v, want ErrUnsupportedStatement", err)
				}
				for i, statement := range result.Statements {
					if i != 1 && (statement.Coverage.Status != report.CoverageComplete || len(statement.EvidenceGaps) != 0) {
						t.Errorf("follower %d = %s gaps=%+v, want complete", i, statement.Coverage.Status, statement.EvidenceGaps)
					}
				}
				if dialect == spec.DialectMySQL {
					if len(result.Unsupported) != 2 {
						t.Fatalf("unsupported = %+v, want exactly two entries", result.Unsupported)
					}
					features := map[string]spec.UnsupportedDetail{}
					for _, item := range result.Unsupported {
						features[item.Feature] = item
					}
					if len(features) != 2 {
						t.Fatalf("unsupported = %+v, want exactly params+body", result.Unsupported)
					}
					for _, feature := range []string{"create_procedure.option.params", "create_procedure.body"} {
						item, ok := features[feature]
						if !ok || item.Reason != spec.UnsupportedUnauditedReason || item.Index != 1 ||
							item.SQL != result.Statements[1].RawSQL || !reflect.DeepEqual(item.Metadata, map[string]any{"aspect": "option"}) {
							t.Fatalf("aspect %s = %+v, want unaudited at index 1", feature, item)
						}
					}
				} else {
					if len(result.Unsupported) != 1 {
						t.Fatalf("unsupported = %+v, want the single vendor projection", result.Unsupported)
					}
					t05A8AssertBoundaryItem(t, result, dialect, true, 0, 1)
				}
			})
		}
	})

	t.Run("lifecycle_notice", func(t *testing.T) {
		for _, tc := range []struct {
			name, sql, ruleID string
			create            bool
		}{
			{"create", "CREATE PROCEDURE p() SELECT 1;", "ddl.create_procedure.notice", true},
			{"drop", "DROP PROCEDURE p;", "ddl.drop_procedure.notice", false},
		} {
			for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
				t.Run(string(dialect)+"/"+tc.name, func(t *testing.T) {
					policy := t05PolicyPathConfigured(t, map[string]t05RuleConfig{tc.ruleID: {level: "notice"}})
					result, err := AuditSQL(context.Background(), Request{
						SQL: tc.sql, Dialect: dialect, ConfigPath: policy, MetadataProvider: &t05A8Provider{},
					})
					if dialect == spec.DialectMySQL && !tc.create {
						if err != nil {
							t.Fatalf("mysql drop error = %v, want nil", err)
						}
						if len(result.Unsupported) != 0 || result.Coverage.Status != report.CoverageComplete || result.Verdict != report.VerdictPass {
							t.Fatalf("mysql drop = %s/%s unsupported=%+v, want pass/complete none", result.Verdict, result.Coverage.Status, result.Unsupported)
						}
					} else {
						if !errors.Is(err, ErrUnsupportedStatement) {
							t.Fatalf("error = %v, want ErrUnsupportedStatement", err)
						}
						if result.Coverage.Status != report.CoverageIncomplete {
							t.Fatalf("coverage = %s, want incomplete; vendor/aspect must not promote to complete", result.Coverage.Status)
						}
					}
					if dialect == spec.DialectTiDB {
						if len(result.Statements) != 1 || len(result.Statements[0].Findings) != 0 {
							t.Fatalf("tidb findings = %+v, want none (vendor skip)", result.Statements)
						}
						if result.RuleSummary == nil || result.RuleSummary.Loaded != 1 || result.RuleSummary.Applicable != 0 {
							t.Fatalf("tidb rule summary = %+v, want loaded=1 applicable=0", result.RuleSummary)
						}
						if len(result.Unsupported) != 1 {
							t.Fatalf("tidb unsupported = %+v, want vendor entry", result.Unsupported)
						}
						t05A8AssertBoundaryItem(t, result, dialect, tc.create, 0, 0)
						return
					}
					if len(result.Statements) != 1 {
						t.Fatalf("statements = %d, want 1", len(result.Statements))
					}
					if len(result.Statements[0].Findings) != 1 || result.Statements[0].Findings[0].RuleID != tc.ruleID ||
						result.Statements[0].Findings[0].Level != rule.LevelNotice {
						t.Fatalf("findings = %+v, want exactly one %s notice", result.Statements[0].Findings, tc.ruleID)
					}
				})
			}
		}
	})

	t.Run("registry_consumers", func(t *testing.T) {
		for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
			for _, tc := range []struct {
				name, sql string
				create    bool
			}{
				{"create", "CREATE PROCEDURE p() SELECT 1;", true},
				{"drop", "DROP PROCEDURE p;", false},
			} {
				t.Run(string(dialect)+"/"+tc.name, func(t *testing.T) {
					ctx := context.Background()
					parsed, err := Parse(ctx, t05A8Input(tc.sql), dialect)
					if err != nil {
						t.Fatal(err)
					}
					extracted, err := Extract(ctx, parsed)
					if err != nil {
						t.Fatal(err)
					}
					enriched, err := enrichStatementsWithMetadata(ctx, dialect,
						&MetadataRequest{Schema: "golden", Provider: &t05A8Provider{}}, extracted[1:2], parsed.failures)
					if err != nil {
						t.Fatal(err)
					}
					statementSpy := &t05A7RuleSpy{id: "t05a8.spy.statement"}
					globalSpy := &t05A7GlobalSpy{id: "t05a8.spy.global"}
					registry := rule.NewRegistry()
					if err := registry.RegisterStatement(statementSpy); err != nil {
						t.Fatal(err)
					}
					if err := registry.RegisterGlobal(globalSpy); err != nil {
						t.Fatal(err)
					}
					if _, err := EvaluateStatements(ctx, registry, enriched); err != nil {
						t.Fatal(err)
					}
					raw := strings.TrimSpace(tc.sql)
					if dialect == spec.DialectMySQL {
						if !reflect.DeepEqual(statementSpy.applied, []string{raw}) ||
							!reflect.DeepEqual(statementSpy.evaluated, []string{raw}) ||
							!reflect.DeepEqual(statementSpy.gapsSeen, []string{raw}) {
							t.Errorf("mysql procedure must reach statement rule consumers: applies=%v eval=%v gaps=%v",
								statementSpy.applied, statementSpy.evaluated, statementSpy.gapsSeen)
						}
						if len(globalSpy.raws) != 1 || !reflect.DeepEqual(globalSpy.raws[0], []string{raw}) {
							t.Errorf("mysql procedure must reach global rules: %v", globalSpy.raws)
						}
					} else {
						if len(statementSpy.applied) != 0 || len(statementSpy.evaluated) != 0 || len(statementSpy.gapsSeen) != 0 {
							t.Errorf("tidb vendor statement must stay skipped: applies=%v eval=%v gaps=%v",
								statementSpy.applied, statementSpy.evaluated, statementSpy.gapsSeen)
						}
						for _, batch := range globalSpy.raws {
							for _, seen := range batch {
								if seen == raw {
									t.Errorf("tidb vendor statement reached global input: %v", globalSpy.raws)
								}
							}
						}
					}
				})
			}
		}
	})
}
