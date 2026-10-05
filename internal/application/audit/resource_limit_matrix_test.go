// Package audit verifies the T05-A7 ordered-state budget matrix.
// input: quota-bounded MySQL/TiDB audit runs against recording spies, synthetic rule consumers, and error/cancellation injectors
// output: gap/policy/reject, provider call-stop, rule-consumer, error/identity, and counting contract assertions for blocked statements
// pos: application-layer contract tests for the shared ordered-state admission seam (issue #84/T05-A7)
// note: if this file changes, update this header and module README.md.
package audit

import (
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

type t05A7Spy struct {
	events      []string
	snapshot    *spec.TableSnapshot
	instanceErr error
	snapshotErr error
	failTable   string
	onSnapshot  func()
	onPlan      func()
}

func (p *t05A7Spy) LoadInstanceFacts(context.Context, spec.Dialect, string) (*spec.InstanceFacts, error) {
	p.events = append(p.events, "instance")
	if p.instanceErr != nil {
		return nil, p.instanceErr
	}
	return &spec.InstanceFacts{}, nil
}

func (p *t05A7Spy) LoadTableSnapshot(_ context.Context, _ spec.Dialect, schema, table string) (*spec.TableSnapshot, error) {
	p.events = append(p.events, "snapshot:"+schema+"."+table)
	if p.onSnapshot != nil {
		p.onSnapshot()
	}
	if p.snapshotErr != nil && (p.failTable == "" || p.failTable == table) {
		return nil, p.snapshotErr
	}
	if p.snapshot != nil {
		return p.snapshot, nil
	}
	return absentTable(schema, table), nil
}

func (p *t05A7Spy) ResolveTableForIndex(_ context.Context, _ spec.Dialect, schema, index string) (string, error) {
	p.events = append(p.events, "owner:"+schema+"."+index)
	return "t", nil
}

func (p *t05A7Spy) ResolveObject(_ context.Context, _ spec.Dialect, request spec.ObjectLookupRequest) (*spec.ObjectSnapshot, error) {
	p.events = append(p.events, "object:"+request.Type+":"+request.Schema+"."+request.Name)
	return &spec.ObjectSnapshot{
		Schema: request.Schema,
		Type:   request.Type,
		Name:   request.Name,
		Status: spec.MetadataStatusConfirmed,
		Exists: true,
	}, nil
}

func (p *t05A7Spy) LoadPlanEstimate(_ context.Context, statement spec.Statement) (*spec.ImpactEstimate, error) {
	p.events = append(p.events, "plan:"+strings.TrimSpace(statement.RawSQL))
	if p.onPlan != nil {
		p.onPlan()
	}
	return &spec.ImpactEstimate{Source: spec.ImpactSourcePlan}, nil
}

type t05A7RuleSpy struct {
	id        string
	appliesTo func(spec.Statement) bool
	applied   []string
	evaluated []string
	gapsSeen  []string
}

func (r *t05A7RuleSpy) ID() string { return r.id }

func (r *t05A7RuleSpy) AppliesTo(statement spec.Statement) bool {
	r.applied = append(r.applied, strings.TrimSpace(statement.RawSQL))
	return r.appliesTo == nil || r.appliesTo(statement)
}

func (r *t05A7RuleSpy) Evaluate(_ context.Context, statement spec.Statement) ([]rule.Finding, error) {
	r.evaluated = append(r.evaluated, strings.TrimSpace(statement.RawSQL))
	return nil, nil
}

func (r *t05A7RuleSpy) EvidenceGaps(statement spec.Statement) []rule.EvidenceGap {
	r.gapsSeen = append(r.gapsSeen, strings.TrimSpace(statement.RawSQL))
	return nil
}

type t05A7GlobalSpy struct {
	id   string
	raws [][]string
}

func (g *t05A7GlobalSpy) ID() string { return g.id }

func (g *t05A7GlobalSpy) EvaluateAll(_ context.Context, statements []spec.Statement) ([]rule.Finding, error) {
	raws := make([]string, 0, len(statements))
	for _, statement := range statements {
		raws = append(raws, strings.TrimSpace(statement.RawSQL))
	}
	g.raws = append(g.raws, raws)
	return nil, nil
}

func t05A7Extract(t *testing.T, sql string, dialect spec.Dialect) []spec.Statement {
	t.Helper()
	parsed, err := Parse(context.Background(), sql, dialect)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	statements, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	return statements
}

func t05A7Audit(t *testing.T, sql string, dialect spec.Dialect, provider MetadataProvider, policy string, limit int) (report.Result, error) {
	t.Helper()
	return auditWithLimits(context.Background(), Request{
		SQL:              sql,
		Dialect:          dialect,
		ConfigPath:       policy,
		Schema:           "golden",
		MetadataProvider: provider,
	}, auditLimits{orderedStatements: limit})
}

func TestT05A7AllOffPolicyBlocksSuffix(t *testing.T) {
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			result, err := t05A7Audit(t, t05A7FirstPathSQL, dialect, nil, t05A7Policy(t, nil), 2)
			if !errors.Is(err, ErrUnsupportedStatement) {
				t.Fatalf("expected ErrUnsupportedStatement, got %v", err)
			}
			if result.Verdict != report.VerdictReview || result.Coverage.Status != report.CoverageIncomplete {
				t.Fatalf("aggregate = %s/%s, want review/incomplete", result.Verdict, result.Coverage.Status)
			}
			if len(result.Statements) != 3 {
				t.Fatalf("statements = %d, want 3 retained", len(result.Statements))
			}
			for i := 0; i < 2; i++ {
				statement := result.Statements[i]
				if statement.Coverage.Status != report.CoverageComplete {
					t.Fatalf("statement %d coverage = %s, want complete", i, statement.Coverage.Status)
				}
			}
			for i, statement := range result.Statements {
				if len(statement.Findings) != 0 || len(statement.EvidenceGaps) != 0 || statement.Impact != nil {
					t.Fatalf("statement %d findings=%+v gaps=%+v impact=%+v, want none", i, statement.Findings, statement.EvidenceGaps, statement.Impact)
				}
			}
			if len(result.GlobalFindings) != 0 {
				t.Fatalf("global findings = %+v, want none", result.GlobalFindings)
			}
			if result.RuleSummary != nil && (result.RuleSummary.Loaded != 0 || result.RuleSummary.Applicable != 0) {
				t.Fatalf("rule summary = %+v, want nil or zero loaded/applicable", result.RuleSummary)
			}
			blocked := result.Statements[2]
			if blocked.Coverage.Status != report.CoverageIncomplete {
				t.Fatalf("blocked coverage = %s, want incomplete", blocked.Coverage.Status)
			}
			if len(result.Unsupported) != 1 {
				t.Fatalf("unsupported entries = %d, want 1: %+v", len(result.Unsupported), result.Unsupported)
			}
			resources := t05A7ResourceEntries(result)
			if len(resources) != 1 {
				t.Fatalf("resource entries = %d, want 1", len(resources))
			}
			t05A7AssertResourceEntry(t, resources[0], blocked, 2, 2)
		})
	}
}

func TestT05A7RejectPrefixBlocksSuffix(t *testing.T) {
	const sql = "CREATE TABLE t (id INT PRIMARY KEY);\nALTER TABLE t ADD COLUMN id INT;\nCREATE INDEX idx_c ON t(c);"
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			policy := t05A7FourRulePolicy(t)
			provider := &t05AbsentProvider{}
			control, controlErr := t05A7Audit(t, sql, dialect, provider, policy, 3)
			if controlErr != nil {
				t.Fatalf("control audit: %v", controlErr)
			}

			result, err := t05A7Audit(t, sql, dialect, &t05AbsentProvider{}, policy, 2)
			if !errors.Is(err, ErrUnsupportedStatement) {
				t.Fatalf("expected ErrUnsupportedStatement, got %v", err)
			}
			if result.Verdict != report.VerdictReject || result.Coverage.Status != report.CoverageIncomplete {
				t.Fatalf("aggregate = %s/%s, want reject/incomplete", result.Verdict, result.Coverage.Status)
			}
			if !reflect.DeepEqual(result.Statements[:2], control.Statements[:2]) {
				t.Fatalf("admitted prefix diverged from the uncapped control:\n got %+v\nwant %+v", result.Statements[:2], control.Statements[:2])
			}
			if len(result.Statements[1].Findings) != 1 {
				t.Fatalf("statement 1 findings = %+v, want exactly one", result.Statements[1].Findings)
			}
			findings := t05FindingsByRule(result, 1, t05RuleAddColumnForbid)
			if len(findings) != 1 {
				t.Fatalf("statement 1 must carry exactly one add-column blocker, got %+v", result.Statements[1].Findings)
			}
			if findings[0].Level != rule.LevelBlocker {
				t.Fatalf("blocker level = %s, want blocker", findings[0].Level)
			}
			if findings[0].Metadata["table"] != "t" || findings[0].Metadata["name"] != "id" {
				t.Fatalf("blocker must bind table=t column=id, got %#v", findings[0].Metadata)
			}
			blocked := result.Statements[2]
			if len(blocked.Findings) != 0 || len(blocked.EvidenceGaps) != 0 {
				t.Fatalf("blocked statement must not produce findings or gaps, got findings=%+v gaps=%+v", blocked.Findings, blocked.EvidenceGaps)
			}
			if blocked.Coverage.Status != report.CoverageIncomplete {
				t.Fatalf("blocked coverage = %s, want incomplete", blocked.Coverage.Status)
			}
			resources := t05A7ResourceEntries(result)
			if len(resources) != 1 {
				t.Fatalf("resource entries = %d, want 1", len(resources))
			}
			t05A7AssertResourceEntry(t, resources[0], blocked, 2, 2)
		})
	}
}

func TestT05A7CallStopLedger(t *testing.T) {
	const sql = "UPDATE t SET id=2 WHERE id=1;\nDELETE FROM never_loaded WHERE id=1;\nCREATE DATABASE never_schema;\nCREATE TABLE never_created (id INT);"
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			shape := &spec.TableSnapshot{
				Exists:      true,
				Schema:      "golden",
				Table:       &spec.Table{Schema: "golden", Name: "t"},
				Columns:     []spec.Column{{Name: "id", Type: "int"}},
				PrimaryKey:  &spec.Index{Name: "primary", Kind: spec.IndexKindPrimary, Columns: []string{"id"}},
				Indexes:     []spec.Index{},
				Constraints: []spec.Constraint{},
				Options:     map[string]string{"table_rows": "100"},
			}
			shapeBefore := cloneTableSnapshot(shape)
			snapshotCalls := 0
			spy := &t05A7Spy{
				snapshot:   shape,
				onSnapshot: func() { snapshotCalls++ },
			}
			result, err := t05A7Audit(t, sql, dialect, spy, t05A7Policy(t, nil), 1)
			if !errors.Is(err, ErrUnsupportedStatement) {
				t.Fatalf("expected ErrUnsupportedStatement, got %v", err)
			}
			wantEvents := []string{"instance", "snapshot:golden.t", "plan:UPDATE t SET id=2 WHERE id=1;"}
			if !reflect.DeepEqual(spy.events, wantEvents) {
				t.Fatalf("provider events = %v, want %v", spy.events, wantEvents)
			}
			if snapshotCalls != 1 {
				t.Fatalf("onSnapshot calls = %d, want 1", snapshotCalls)
			}
			if result.Statements[0].Impact == nil || result.Statements[0].Impact.Source != spec.ImpactSourcePlan {
				t.Fatalf("admitted UPDATE impact = %+v, want source plan", result.Statements[0].Impact)
			}
			resources := t05A7ResourceEntries(result)
			if len(resources) != 3 {
				t.Fatalf("resource entries = %d, want 3", len(resources))
			}
			for i := 1; i < 4; i++ {
				blocked := result.Statements[i]
				if blocked.Coverage.Status != report.CoverageIncomplete || len(blocked.Findings) != 0 || len(blocked.EvidenceGaps) != 0 || blocked.Impact != nil {
					t.Fatalf("blocked statement %d = %+v, want incomplete with no findings/gaps/impact", i, blocked)
				}
				t05A7AssertResourceEntry(t, resources[i-1], blocked, i, 1)
			}
			if !reflect.DeepEqual(shape, shapeBefore) {
				t.Fatalf("provider-owned snapshot mutated:\n got %+v\nwant %+v", shape, shapeBefore)
			}

			controlSpy := &t05A7Spy{snapshot: cloneTableSnapshot(shapeBefore)}
			control, controlErr := t05A7Audit(t, "UPDATE t SET id=2 WHERE id=1;", dialect, controlSpy, t05A7Policy(t, nil), 1)
			if controlErr != nil {
				t.Fatalf("control audit: %v", controlErr)
			}
			if !reflect.DeepEqual(controlSpy.events, wantEvents) {
				t.Fatalf("control events = %v, want %v", controlSpy.events, wantEvents)
			}
			if !reflect.DeepEqual(control.Statements[0], result.Statements[0]) {
				t.Fatalf("admitted statement diverged from single-statement control:\n got %+v\nwant %+v", result.Statements[0], control.Statements[0])
			}
		})
	}
}

func TestT05A7TableSnapshotCutoff(t *testing.T) {
	const sql = "CREATE TABLE t (id INT PRIMARY KEY);\nCREATE TABLE never_loaded (id INT);"
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			spy := &t05A7Spy{}
			_, err := t05A7Audit(t, sql, dialect, spy, t05A7Policy(t, nil), 1)
			if !errors.Is(err, ErrUnsupportedStatement) {
				t.Fatalf("expected ErrUnsupportedStatement, got %v", err)
			}
			want := []string{"instance", "snapshot:golden.t"}
			if !reflect.DeepEqual(spy.events, want) {
				t.Fatalf("events = %v, want %v", spy.events, want)
			}
		})
	}
}

func TestT05A7ObjectCutoff(t *testing.T) {
	const sql = "CREATE DATABASE before_schema;\nCREATE DATABASE after_schema;"
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			spy := &t05A7Spy{}
			_, err := t05A7Audit(t, sql, dialect, spy, t05A7Policy(t, nil), 1)
			if !errors.Is(err, ErrUnsupportedStatement) {
				t.Fatalf("expected ErrUnsupportedStatement, got %v", err)
			}
			want := []string{"instance", "object:schema:golden.before_schema"}
			if !reflect.DeepEqual(spy.events, want) {
				t.Fatalf("events = %v, want %v", spy.events, want)
			}
		})
	}
}

func TestT05A7IndexOwnerCutoff(t *testing.T) {
	statements := []spec.Statement{
		{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectMySQL,
			RawSQL:  "DROP INDEX before_idx;",
			Line:    1,
			Column:  1,
			DDL:     &spec.DDL{Operation: spec.DDLOperationDropIndex, Alter: []spec.Alter{{Action: "drop_index", Name: "before_idx"}}},
		},
		{
			Kind:    spec.KindDDL,
			Dialect: spec.DialectMySQL,
			RawSQL:  "DROP INDEX after_idx;",
			Line:    2,
			Column:  1,
			DDL:     &spec.DDL{Operation: spec.DDLOperationDropIndex, Alter: []spec.Alter{{Action: "drop_index", Name: "after_idx"}}},
		},
	}
	request := &MetadataRequest{Schema: "golden"}

	spy := &t05A7Spy{}
	request.Provider = spy
	enriched, err := enrichStatementsWithMetadataLimits(context.Background(), spec.DialectMySQL, request, statements, nil, auditLimits{orderedStatements: 1})
	if err != nil {
		t.Fatalf("enrich: %v", err)
	}
	want := []string{"instance", "owner:golden.before_idx", "snapshot:golden.t"}
	if !reflect.DeepEqual(spy.events, want) {
		t.Fatalf("events = %v, want %v", spy.events, want)
	}
	if enriched[0].ResourceLimit != nil || enriched[1].ResourceLimit == nil {
		t.Fatalf("resource markers = %v/%v, want admitted/blocked", enriched[0].ResourceLimit, enriched[1].ResourceLimit)
	}

	fullSpy := &t05A7Spy{}
	fullRequest := &MetadataRequest{Schema: "golden", Provider: fullSpy}
	enriched, err = enrichStatementsWithMetadataLimits(context.Background(), spec.DialectMySQL, fullRequest, statements, nil, auditLimits{orderedStatements: 2})
	if err != nil {
		t.Fatalf("enrich: %v", err)
	}
	want = []string{"instance", "owner:golden.before_idx", "snapshot:golden.t", "owner:golden.after_idx"}
	if !reflect.DeepEqual(fullSpy.events, want) {
		t.Fatalf("events = %v, want %v", fullSpy.events, want)
	}
	if enriched[1].ResourceLimit != nil {
		t.Fatalf("quota 2 statement must not be blocked, got %+v", enriched[1].ResourceLimit)
	}
}

func TestT05A7RuleConsumersSeeOnlyAdmitted(t *testing.T) {
	const sql = "SELECT 1;\nDELETE FROM t;"

	enrich := func(t *testing.T, limit int) []spec.Statement {
		t.Helper()
		enriched, err := enrichStatementsWithMetadataLimits(context.Background(), spec.DialectMySQL, nil, t05A7Extract(t, sql, spec.DialectMySQL), nil, auditLimits{orderedStatements: limit})
		if err != nil {
			t.Fatalf("enrich: %v", err)
		}
		return enriched
	}

	evaluate := func(t *testing.T, statements []spec.Statement) (report.Result, *t05A7RuleSpy, *t05A7RuleSpy, *t05A7GlobalSpy) {
		t.Helper()
		first := &t05A7RuleSpy{id: "t05a7.spy.all"}
		blockedOnly := &t05A7RuleSpy{id: "t05a7.spy.dml", appliesTo: func(statement spec.Statement) bool { return statement.DML != nil }}
		global := &t05A7GlobalSpy{id: "t05a7.spy.global"}
		registry := rule.NewRegistry()
		for _, registered := range []rule.StatementRule{first, blockedOnly} {
			if err := registry.RegisterStatement(registered); err != nil {
				t.Fatalf("register %s: %v", registered.ID(), err)
			}
		}
		if err := registry.RegisterGlobal(global); err != nil {
			t.Fatalf("register global: %v", err)
		}
		result, err := EvaluateStatements(context.Background(), registry, statements)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		return result, first, blockedOnly, global
	}

	t.Run("limit1", func(t *testing.T) {
		result, first, blockedOnly, global := evaluate(t, enrich(t, 1))
		want := []string{"SELECT 1;"}
		if !reflect.DeepEqual(first.applied, want) || !reflect.DeepEqual(first.evaluated, want) || !reflect.DeepEqual(first.gapsSeen, want) {
			t.Fatalf("first spy saw applies=%v evaluate=%v gaps=%v, want only %v", first.applied, first.evaluated, first.gapsSeen, want)
		}
		if !reflect.DeepEqual(blockedOnly.applied, want) || len(blockedOnly.evaluated) != 0 || len(blockedOnly.gapsSeen) != 0 {
			t.Fatalf("blocked-only spy saw applies=%v evaluate=%v gaps=%v, want applies=%v only", blockedOnly.applied, blockedOnly.evaluated, blockedOnly.gapsSeen, want)
		}
		if len(global.raws) != 1 || !reflect.DeepEqual(global.raws[0], want) {
			t.Fatalf("global spy input = %v, want [%v]", global.raws, want)
		}
		if result.RuleSummary == nil || result.RuleSummary.Loaded != 2 || result.RuleSummary.Applicable != 1 {
			t.Fatalf("rule summary = %+v, want loaded=2 applicable=1", result.RuleSummary)
		}
		if len(t05A7ResourceEntries(result)) != 1 {
			t.Fatalf("resource entries = %d, want 1", len(t05A7ResourceEntries(result)))
		}
	})

	t.Run("limit0", func(t *testing.T) {
		result, first, blockedOnly, global := evaluate(t, enrich(t, 0))
		if len(first.applied) != 0 || len(first.evaluated) != 0 || len(first.gapsSeen) != 0 {
			t.Fatalf("first spy saw applies=%v evaluate=%v gaps=%v, want none", first.applied, first.evaluated, first.gapsSeen)
		}
		if len(blockedOnly.applied) != 0 || len(blockedOnly.evaluated) != 0 || len(blockedOnly.gapsSeen) != 0 {
			t.Fatalf("blocked-only spy saw applies=%v evaluate=%v gaps=%v, want none", blockedOnly.applied, blockedOnly.evaluated, blockedOnly.gapsSeen)
		}
		if len(global.raws) != 1 || len(global.raws[0]) != 0 {
			t.Fatalf("global spy input = %v, want one empty input", global.raws)
		}
		if result.RuleSummary == nil || result.RuleSummary.Loaded != 2 || result.RuleSummary.Applicable != 0 {
			t.Fatalf("rule summary = %+v, want loaded=2 applicable=0", result.RuleSummary)
		}
	})
}

func TestT05A7MySQLReturningBlockedNoGlobalFinding(t *testing.T) {
	const sql = "SELECT 1;\nDELETE FROM t RETURNING id;"

	blocked, err := auditWithLimits(context.Background(), Request{
		SQL:     sql,
		Dialect: spec.DialectMySQL,
	}, auditLimits{orderedStatements: 1})
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("expected ErrUnsupportedStatement, got %v", err)
	}
	for _, finding := range blocked.GlobalFindings {
		if finding.RuleID == "dialect.mysql.returning.unsupported.notice" {
			t.Fatalf("blocked RETURNING manufactured a global finding: %+v", finding)
		}
	}

	control, controlErr := auditWithLimits(context.Background(), Request{
		SQL:     sql,
		Dialect: spec.DialectMySQL,
	}, auditLimits{orderedStatements: 2})
	if controlErr != nil {
		t.Fatalf("control audit: %v", controlErr)
	}
	found := false
	for _, finding := range control.GlobalFindings {
		if finding.RuleID == "dialect.mysql.returning.unsupported.notice" {
			found = true
		}
	}
	if !found {
		t.Fatalf("admitted RETURNING must still produce the global finding, got %+v", control.GlobalFindings)
	}
}

type t05A7CancelOnCheck struct {
	context.Context
	cancel    context.CancelFunc
	remaining int
}

func (c *t05A7CancelOnCheck) Err() error {
	c.remaining--
	if c.remaining == 0 {
		c.cancel()
	}
	return c.Context.Err()
}

func t05A7CancelContext(remaining int) (*t05A7CancelOnCheck, context.CancelFunc) {
	base, cancel := context.WithCancel(context.Background())
	return &t05A7CancelOnCheck{Context: base, cancel: cancel, remaining: remaining}, cancel
}

func TestT05A7ParserFailureRetainsBudget(t *testing.T) {
	cases := []struct {
		name         string
		sql          string
		resourceLine int
		diagLine     int
		contaminated bool
	}{
		{name: "failure_first", sql: "THIS IS NOT SQL;\n" + t05A7FirstPathSQL, resourceLine: 4, diagLine: 1, contaminated: true},
		{name: "failure_last", sql: t05A7FirstPathSQL + "\nTHIS IS NOT SQL;", resourceLine: 3, diagLine: 4, contaminated: false},
	}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, tc := range cases {
			t.Run(string(dialect)+"/"+tc.name, func(t *testing.T) {
				policy := t05A7FourRulePolicy(t)
				control, controlErr := t05A7Audit(t, tc.sql, dialect, &t05AbsentProvider{}, policy, 3)
				if !errors.Is(controlErr, errParserUnsupported) {
					t.Fatalf("control must carry errParserUnsupported, got %v", controlErr)
				}

				result, err := t05A7Audit(t, tc.sql, dialect, &t05AbsentProvider{}, policy, 2)
				if !errors.Is(err, errParserUnsupported) {
					t.Fatalf("expected errParserUnsupported, got %v", err)
				}
				if errors.Is(err, ErrUnsupportedStatement) {
					t.Fatalf("parser failure must not masquerade as ErrUnsupportedStatement: %v", err)
				}
				if len(result.Statements) != 3 {
					t.Fatalf("statements = %d, want 3 normalized statements", len(result.Statements))
				}
				if result.Coverage.Status != report.CoverageIncomplete {
					t.Fatalf("aggregate coverage = %s, want incomplete", result.Coverage.Status)
				}
				if !reflect.DeepEqual(result.Statements[:2], control.Statements[:2]) {
					t.Fatalf("admitted prefix diverged from the uncapped control:\n got %+v\nwant %+v", result.Statements[:2], control.Statements[:2])
				}
				if tc.contaminated {
					if len(result.Statements[0].EvidenceGaps) == 0 {
						t.Fatalf("preceding parser failure must contaminate admitted state, got no gaps on statement 0")
					}
				}
				blocked := result.Statements[2]
				if len(blocked.Findings) != 0 || len(blocked.EvidenceGaps) != 0 {
					t.Fatalf("blocked statement findings=%+v gaps=%+v, want none", blocked.Findings, blocked.EvidenceGaps)
				}
				resources := t05A7ResourceEntries(result)
				if len(resources) != 1 {
					t.Fatalf("resource entries = %d, want 1", len(resources))
				}
				entry := resources[0]
				if entry.Index != 2 || entry.SQL != blocked.RawSQL {
					t.Fatalf("resource entry = %+v, want index 2 bound to the blocked statement", entry)
				}
				wantMeta := map[string]any{"phase": "ordered_state", "resource": "statements", "limit": 2, "consumed": 2, "line": tc.resourceLine, "column": 1}
				if !reflect.DeepEqual(entry.Metadata, wantMeta) {
					t.Fatalf("resource metadata = %+v, want %+v", entry.Metadata, wantMeta)
				}
				diagSeen := false
				for _, diagnostic := range result.Diagnostics {
					if diagnostic.Classification == spec.DiagnosticParserError && diagnostic.Line == tc.diagLine {
						diagSeen = true
					}
				}
				if !diagSeen {
					t.Fatalf("missing parser diagnostic at line %d, diagnostics=%+v", tc.diagLine, result.Diagnostics)
				}

				parsed, parseErr := Parse(context.Background(), tc.sql, dialect)
				if parseErr == nil {
					t.Fatalf("expected parse failure for %s", tc.name)
				}
				extracted, extractErr := Extract(context.Background(), parsed)
				if extractErr != nil {
					t.Fatalf("extract: %v", extractErr)
				}
				enriched, enrichErr := enrichStatementsWithMetadataLimits(context.Background(), dialect, &MetadataRequest{Schema: "golden", Provider: &t05AbsentProvider{}}, extracted, parsed.failures, auditLimits{orderedStatements: 2})
				if enrichErr != nil {
					t.Fatalf("enrich: %v", enrichErr)
				}
				if enriched[2].ResourceLimit == nil || enriched[2].DDL == nil {
					t.Fatalf("blocked statement must retain DDL identity with the marker, got %+v", enriched[2])
				}
				if enriched[2].Metadata != nil {
					t.Fatalf("blocked statement metadata = %+v, want nil", enriched[2].Metadata)
				}
			})
		}
	}
}

func TestT05A7VendorAndAspectBoundariesAtZeroQuota(t *testing.T) {
	t.Run("mysql_vendor", func(t *testing.T) {
		const sql = "CREATE SEQUENCE seq1 START WITH 1;"
		result, err := t05A7Audit(t, sql, spec.DialectMySQL, nil, t05A7Policy(t, nil), 0)
		if !errors.Is(err, ErrUnsupportedStatement) {
			t.Fatalf("expected ErrUnsupportedStatement, got %v", err)
		}
		if len(result.Unsupported) != 3 {
			t.Fatalf("unsupported entries = %d, want 3: %+v", len(result.Unsupported), result.Unsupported)
		}
		raw := result.Statements[0].RawSQL
		features := []string{"create_sequence", "create_sequence.options", "audit.resource_limit"}
		reasons := []string{spec.UnsupportedVendorBoundaryReason, spec.UnsupportedUnauditedReason, spec.AuditResourceLimitReason}
		for i, item := range result.Unsupported {
			if item.Feature != features[i] || item.Reason != reasons[i] || item.SQL != raw {
				t.Fatalf("unsupported[%d] = %+v, want feature=%s reason=%s sql=%q", i, item, features[i], reasons[i], raw)
			}
		}
		t05A7AssertResourceEntry(t, result.Unsupported[2], result.Statements[0], 0, 0)

		input := t05A7Extract(t, sql, spec.DialectMySQL)
		if input[0].Unsupported == nil {
			t.Fatalf("expected an extracted vendor boundary on the input, got %+v", input[0].Unsupported)
		}
		originalUnsupported := input[0].Unsupported
		originalValue := *originalUnsupported
		enriched, enrichErr := enrichStatementsWithMetadataLimits(context.Background(), spec.DialectMySQL, nil, input, nil, auditLimits{orderedStatements: 0})
		if enrichErr != nil {
			t.Fatalf("enrich: %v", enrichErr)
		}
		if enriched[0].Unsupported != originalUnsupported {
			t.Fatalf("enrichment must preserve the original Unsupported pointer, got %+v want %+v", enriched[0].Unsupported, originalUnsupported)
		}
		if !reflect.DeepEqual(*enriched[0].Unsupported, originalValue) {
			t.Fatalf("original Unsupported mutated:\n got %+v\nwant %+v", *enriched[0].Unsupported, originalValue)
		}
		if enriched[0].Unsupported.Feature != "create_sequence" || enriched[0].Unsupported.Reason != spec.UnsupportedVendorBoundaryReason {
			t.Fatalf("original Unsupported = %+v, want create_sequence vendor boundary", enriched[0].Unsupported)
		}
		if enriched[0].ResourceLimit == nil {
			t.Fatalf("expected ResourceLimit marker at quota 0")
		}
	})

	for _, tc := range []struct {
		dialect spec.Dialect
		sql     string
	}{
		{spec.DialectMySQL, "ALTER TABLE t ADD COLUMN c INT FIRST;"},
		{spec.DialectTiDB, "ALTER TABLE t ADD COLUMN c INT AFTER id;"},
	} {
		t.Run(string(tc.dialect)+"_aspect", func(t *testing.T) {
			result, err := t05A7Audit(t, tc.sql, tc.dialect, nil, t05A7Policy(t, nil), 0)
			if !errors.Is(err, ErrUnsupportedStatement) {
				t.Fatalf("expected ErrUnsupportedStatement, got %v", err)
			}
			if len(result.Unsupported) != 2 {
				t.Fatalf("unsupported entries = %d, want 2: %+v", len(result.Unsupported), result.Unsupported)
			}
			aspect := result.Unsupported[0]
			if aspect.Feature != "alter_table.add_columns.column_position" || aspect.Reason != spec.UnsupportedUnauditedReason {
				t.Fatalf("aspect entry = %+v, want unaudited column_position", aspect)
			}
			if aspect.SQL != result.Statements[0].RawSQL {
				t.Fatalf("aspect SQL = %q, want raw %q", aspect.SQL, result.Statements[0].RawSQL)
			}
			t05A7AssertResourceEntry(t, result.Unsupported[1], result.Statements[0], 0, 0)
		})
	}
}

func TestT05A7BlockedCreateAndStaleConclusions(t *testing.T) {
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			statements := t05A7Extract(t, "CREATE TABLE t (id INT PRIMARY KEY);\nDELETE FROM old_t;", dialect)
			oldMetadata := &spec.Metadata{
				Schema:      "stale",
				TargetTable: &spec.TableSnapshot{Exists: true, Table: &spec.Table{Name: "old_t"}, Options: map[string]string{"table_rows": "99"}},
				Objects:     []spec.ObjectSnapshot{{Name: "old_object", Type: "schema", Status: spec.MetadataStatusConfirmed, Exists: true}},
			}
			statements[1].Metadata = oldMetadata
			oldImpact := statements[1].DML.Impact
			state := newBatchState(dialect, "golden", nil)
			enriched, err := state.enrichStatements(context.Background(), nil, statements, nil, nil, auditLimits{orderedStatements: 0})
			if err != nil {
				t.Fatalf("enrich: %v", err)
			}
			if len(enriched) != 2 {
				t.Fatalf("enriched statements = %d, want 2", len(enriched))
			}
			if !state.contaminated {
				t.Fatalf("blocked statements must contaminate ongoing derivation")
			}
			if len(state.entries) != 0 {
				t.Fatalf("blocked CREATE must publish no derived state, got entries %+v", state.entries)
			}
			for i, statement := range enriched {
				if statement.ResourceLimit == nil || statement.ResourceLimit.Limit != 0 || statement.ResourceLimit.Consumed != 0 {
					t.Fatalf("statement %d ResourceLimit = %+v, want limit 0 consumed 0", i, statement.ResourceLimit)
				}
				if statement.Metadata != nil {
					t.Fatalf("blocked statement %d metadata = %+v, want nil (stale conclusions cleared)", i, statement.Metadata)
				}
			}
			if enriched[1].DML == nil || enriched[1].DML.Impact != nil {
				t.Fatalf("blocked DELETE impact = %+v, want cleared on the output copy", enriched[1].DML)
			}
			if statements[1].Metadata != oldMetadata {
				t.Fatalf("input metadata pointer replaced: %+v", statements[1].Metadata)
			}
			if len(statements[1].Metadata.Objects) != 1 || statements[1].Metadata.TargetTable == nil || statements[1].Metadata.TargetTable.Options["table_rows"] != "99" {
				t.Fatalf("input metadata mutated: %+v", statements[1].Metadata)
			}
			if statements[1].DML == nil || statements[1].DML.Impact == nil || statements[1].DML.Impact != oldImpact {
				t.Fatalf("input impact mutated: %+v want %+v", statements[1].DML, oldImpact)
			}

			spy := &t05A7Spy{}
			attached := attachImpactEstimatesWithPlanner(context.Background(), spy, enriched)
			if len(spy.events) != 0 {
				t.Fatalf("blocked statements must never reach the planner, got events %v", spy.events)
			}
			if attached[1].DML == nil || attached[1].DML.Impact != nil {
				t.Fatalf("attach impact on blocked statement = %+v, want unchanged", attached[1].DML)
			}
			if reportImpact(enriched[1]) != nil {
				t.Fatalf("reportImpact on a blocked statement = %+v, want nil", reportImpact(enriched[1]))
			}
		})
	}
}

func TestT05A7JSONProjectionOmitsMarker(t *testing.T) {
	const secret = "t05-a7-secret;payload"
	result, err := t05A7Audit(t, "SELECT '"+secret+"';", spec.DialectMySQL, nil, t05A7Policy(t, nil), 0)
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("expected ErrUnsupportedStatement, got %v", err)
	}
	if len(result.Unsupported) != 1 {
		t.Fatalf("unsupported entries = %d, want 1", len(result.Unsupported))
	}
	raw, marshalErr := json.Marshal(result.Unsupported)
	if marshalErr != nil {
		t.Fatalf("marshal unsupported: %v", marshalErr)
	}
	var decoded []map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil || len(decoded) != 1 {
		t.Fatalf("decode unsupported: %v entries=%d", err, len(decoded))
	}
	if _, present := decoded[0]["index"]; present {
		t.Fatalf("index 0 must be omitted (omitempty), got %#v", decoded[0])
	}
	var roundTrip []spec.UnsupportedDetail
	if err := json.Unmarshal(raw, &roundTrip); err != nil || len(roundTrip) != 1 {
		t.Fatalf("round-trip decode: %v entries=%d", err, len(roundTrip))
	}
	if roundTrip[0].Index != 0 {
		t.Fatalf("decoded index = %d, want zero default", roundTrip[0].Index)
	}
	if decoded[0]["sql"] != result.Statements[0].RawSQL {
		t.Fatalf("sql identity = %#v, want %q", decoded[0]["sql"], result.Statements[0].RawSQL)
	}
	reasonJSON, _ := json.Marshal(decoded[0]["reason"])
	metadataJSON, _ := json.Marshal(decoded[0]["metadata"])
	if strings.Contains(string(reasonJSON), secret) || strings.Contains(string(metadataJSON), secret) {
		t.Fatalf("secret sentinel leaked into reason/metadata: %s %s", reasonJSON, metadataJSON)
	}
	metadata, ok := decoded[0]["metadata"].(map[string]any)
	if !ok || len(metadata) != 6 {
		t.Fatalf("resource metadata must have exactly six keys, got %#v", decoded[0]["metadata"])
	}

	enriched, enrichErr := enrichStatementsWithMetadataLimits(context.Background(), spec.DialectMySQL, nil, t05A7Extract(t, "CREATE SEQUENCE seq1;", spec.DialectMySQL), nil, auditLimits{orderedStatements: 0})
	if enrichErr != nil {
		t.Fatalf("enrich: %v", enrichErr)
	}
	if enriched[0].ResourceLimit == nil || enriched[0].Unsupported == nil {
		t.Fatalf("expected marker plus vendor boundary, got %+v", enriched[0])
	}
	statementJSON, marshalErr := json.Marshal(enriched[0])
	if marshalErr != nil {
		t.Fatalf("marshal statement: %v", marshalErr)
	}
	var fields map[string]any
	if err := json.Unmarshal(statementJSON, &fields); err != nil {
		t.Fatalf("decode statement: %v", err)
	}
	for _, key := range []string{"ResourceLimit", "resource_limit", "resourceLimit"} {
		if _, present := fields[key]; present {
			t.Fatalf("nonserialized marker leaked as JSON key %q: %s", key, statementJSON)
		}
	}
	if _, present := fields["unsupported"]; !present {
		t.Fatalf("original Unsupported must remain serialized: %s", statementJSON)
	}
}

func TestT05A7SourceOffsets(t *testing.T) {
	const sql = "\n  SELECT 1;\n\n    CREATE TABLE later (id INT);"
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			result, err := t05A7Audit(t, sql, dialect, nil, t05A7Policy(t, nil), 1)
			if !errors.Is(err, ErrUnsupportedStatement) {
				t.Fatalf("expected ErrUnsupportedStatement, got %v", err)
			}
			if len(result.Statements) != 2 {
				t.Fatalf("statements = %d, want 2", len(result.Statements))
			}
			blocked := result.Statements[1]
			if strings.TrimSpace(blocked.RawSQL) != "CREATE TABLE later (id INT);" || blocked.NormalizedSQL == "" {
				t.Fatalf("blocked identity = %+v, want the CREATE TABLE", blocked)
			}
			resources := t05A7ResourceEntries(result)
			if len(resources) != 1 {
				t.Fatalf("resource entries = %d, want 1", len(resources))
			}
			entry := resources[0]
			wantMeta := map[string]any{"phase": "ordered_state", "resource": "statements", "limit": 1, "consumed": 1, "line": 4, "column": 5}
			if entry.Index != 1 || entry.SQL != blocked.RawSQL || !reflect.DeepEqual(entry.Metadata, wantMeta) {
				t.Fatalf("resource entry = %+v, want index 1 with metadata %+v", entry, wantMeta)
			}
		})
	}
}

func TestT05A7ProviderErrorPrecedence(t *testing.T) {
	sentinel := errors.New("t05-a7 provider sentinel")
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			spy := &t05A7Spy{snapshotErr: sentinel}
			result, err := t05A7Audit(t, "CREATE TABLE t (id INT PRIMARY KEY);", dialect, spy, t05A7Policy(t, nil), 1)
			if !errors.Is(err, sentinel) {
				t.Fatalf("admitted snapshot error must surface, got %v", err)
			}
			if len(result.Unsupported) != 0 || len(result.Statements) != 0 {
				t.Fatalf("provider errors must not produce a resource-shaped partial result, got %+v", result)
			}

			spy = &t05A7Spy{instanceErr: sentinel}
			_, err = t05A7Audit(t, t05A7FirstPathSQL, dialect, spy, t05A7Policy(t, nil), 0)
			if !errors.Is(err, sentinel) {
				t.Fatalf("instance-fact error must surface even at quota 0, got %v", err)
			}
			if !reflect.DeepEqual(spy.events, []string{"instance"}) {
				t.Fatalf("events = %v, want [instance]", spy.events)
			}

			spy = &t05A7Spy{snapshotErr: sentinel, failTable: "never_loaded"}
			result, err = t05A7Audit(t, "SELECT 1;\nALTER TABLE never_loaded ADD COLUMN c INT;", dialect, spy, t05A7Policy(t, nil), 1)
			if !errors.Is(err, ErrUnsupportedStatement) {
				t.Fatalf("expected ErrUnsupportedStatement for the blocked suffix, got %v", err)
			}
			if errors.Is(err, sentinel) {
				t.Fatalf("uncalled provider error must not surface: %v", err)
			}
			if !reflect.DeepEqual(spy.events, []string{"instance"}) {
				t.Fatalf("events = %v, want [instance] only", spy.events)
			}
			if len(t05A7ResourceEntries(result)) != 1 {
				t.Fatalf("resource entries = %d, want 1", len(t05A7ResourceEntries(result)))
			}
		})
	}
}

func TestT05A7ContextCancellation(t *testing.T) {
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			spy := &t05A7Spy{}
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			cancel()
			_, err := auditWithLimits(ctx, Request{
				SQL:              t05A7FirstPathSQL,
				Dialect:          dialect,
				Schema:           "golden",
				MetadataProvider: spy,
			}, auditLimits{orderedStatements: 2})
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("pre-canceled context must return context.Canceled, got %v", err)
			}
			if len(spy.events) != 0 {
				t.Fatalf("canceled audit must not call the provider, got %v", spy.events)
			}

			ctx, cancel = context.WithCancel(context.Background())
			t.Cleanup(cancel)
			spy = &t05A7Spy{onSnapshot: cancel}
			_, err = auditWithLimits(ctx, Request{
				SQL:              "CREATE TABLE t (id INT PRIMARY KEY);\nSELECT 1;",
				Dialect:          dialect,
				Schema:           "golden",
				MetadataProvider: spy,
			}, auditLimits{orderedStatements: 2})
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("snapshot-time cancellation must surface, got %v", err)
			}
			if !reflect.DeepEqual(spy.events, []string{"instance", "snapshot:golden.t"}) {
				t.Fatalf("events = %v, want [instance snapshot:golden.t]", spy.events)
			}

			ctx, cancel = context.WithCancel(context.Background())
			t.Cleanup(cancel)
			spy = &t05A7Spy{onPlan: cancel}
			_, err = auditWithLimits(ctx, Request{
				SQL:              "UPDATE t SET id=2 WHERE id=1;\nSELECT 1;",
				Dialect:          dialect,
				Schema:           "golden",
				MetadataProvider: spy,
			}, auditLimits{orderedStatements: 2})
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("planner-time cancellation must surface, got %v", err)
			}
		})
	}

	t.Run("enrich_skip_checkpoint", func(t *testing.T) {
		statements := []spec.Statement{{Kind: spec.KindUnknown, Dialect: spec.DialectMySQL, RawSQL: "SELECT 1;", Line: 1, Column: 1}}
		ctx, cancel := t05A7CancelContext(2)
		t.Cleanup(cancel)
		_, err := newBatchState(spec.DialectMySQL, "golden", nil).enrichStatements(ctx, nil, statements, nil, nil, auditLimits{orderedStatements: 0})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation at the blocked-skip checkpoint must surface, got %v", err)
		}
	})

	t.Run("enrich_final_checkpoint", func(t *testing.T) {
		statements := []spec.Statement{{Kind: spec.KindUnknown, Dialect: spec.DialectMySQL, RawSQL: "SELECT 1;", Line: 1, Column: 1}}
		ctx, cancel := t05A7CancelContext(3)
		t.Cleanup(cancel)
		_, err := newBatchState(spec.DialectMySQL, "golden", nil).enrichStatements(ctx, nil, statements, nil, nil, auditLimits{orderedStatements: 0})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation at the final enrich checkpoint must surface, got %v", err)
		}
	})

	t.Run("evaluate_skip_checkpoint", func(t *testing.T) {
		statements := []spec.Statement{{
			Kind:          spec.KindUnknown,
			Dialect:       spec.DialectMySQL,
			RawSQL:        "SELECT 1;",
			ResourceLimit: &spec.AuditResourceLimit{Limit: 0, Consumed: 0},
		}}
		ctx, cancel := t05A7CancelContext(2)
		t.Cleanup(cancel)
		_, err := EvaluateStatements(ctx, rule.NewRegistry(), statements)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation inside the blocked branch must surface, got %v", err)
		}
	})

	t.Run("evaluate_global_checkpoint", func(t *testing.T) {
		statements := []spec.Statement{{
			Kind:          spec.KindUnknown,
			Dialect:       spec.DialectMySQL,
			RawSQL:        "SELECT 1;",
			ResourceLimit: &spec.AuditResourceLimit{Limit: 0, Consumed: 0},
		}}
		ctx, cancel := t05A7CancelContext(3)
		t.Cleanup(cancel)
		_, err := EvaluateStatements(ctx, rule.NewRegistry(), statements)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation before global evaluation must surface, got %v", err)
		}
	})

	t.Run("evaluate_final_checkpoint", func(t *testing.T) {
		statements := []spec.Statement{{
			Kind:          spec.KindUnknown,
			Dialect:       spec.DialectMySQL,
			RawSQL:        "SELECT 1;",
			ResourceLimit: &spec.AuditResourceLimit{Limit: 0, Consumed: 0},
		}}
		ctx, cancel := t05A7CancelContext(4)
		t.Cleanup(cancel)
		_, err := EvaluateStatements(ctx, rule.NewRegistry(), statements)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation at the final evaluate checkpoint must surface, got %v", err)
		}
	})
}

func TestT05A7NormalizedCounting(t *testing.T) {
	sql := "SELECT 'semi;colon';\n/* only a comment; */;\nSET @x = 1;\nALTER TABLE t ADD COLUMN c INT, ADD COLUMN d INT;\nSELECT 1;"
	wantRaw := []string{
		"SELECT 'semi;colon';",
		"SET @x = 1;",
		"ALTER TABLE t ADD COLUMN c INT, ADD COLUMN d INT;",
		"SELECT 1;",
	}
	wantKind := []string{"unknown", "unknown", "ddl", "unknown"}
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			result, err := t05A7Audit(t, sql, dialect, nil, t05A7Policy(t, nil), 3)
			if !errors.Is(err, ErrUnsupportedStatement) {
				t.Fatalf("expected ErrUnsupportedStatement, got %v", err)
			}
			if len(result.Statements) != 4 {
				t.Fatalf("normalized statements = %d, want exactly 4 (comment-only block excluded)", len(result.Statements))
			}
			for i, statement := range result.Statements {
				if strings.TrimSpace(statement.RawSQL) != wantRaw[i] || statement.Kind != wantKind[i] {
					t.Fatalf("statement %d = kind %s raw %q, want kind %s raw %q", i, statement.Kind, statement.RawSQL, wantKind[i], wantRaw[i])
				}
			}
			for i := 0; i < 3; i++ {
				if result.Statements[i].Coverage.Status != report.CoverageComplete {
					t.Fatalf("admitted statement %d coverage = %s, want complete", i, result.Statements[i].Coverage.Status)
				}
			}
			resources := t05A7ResourceEntries(result)
			if len(resources) != 1 {
				t.Fatalf("resource entries = %d, want 1", len(resources))
			}
			entry := resources[0]
			blocked := result.Statements[3]
			wantMeta := map[string]any{"phase": "ordered_state", "resource": "statements", "limit": 3, "consumed": 3, "line": 5, "column": 1}
			if entry.Index != 3 || entry.SQL != blocked.RawSQL || !reflect.DeepEqual(entry.Metadata, wantMeta) {
				t.Fatalf("resource entry = %+v, want index 3 with metadata %+v", entry, wantMeta)
			}
		})
	}
}

func TestT05A7UnsupportedConsumesSlot(t *testing.T) {
	const sql = "CREATE SEQUENCE seq1;\nSELECT 1;\nSELECT 1;"
	for _, limit := range []int{1, 2} {
		t.Run(fmt.Sprintf("limit_%d", limit), func(t *testing.T) {
			result, err := t05A7Audit(t, sql, spec.DialectMySQL, nil, t05A7Policy(t, nil), limit)
			if !errors.Is(err, ErrUnsupportedStatement) {
				t.Fatalf("expected ErrUnsupportedStatement, got %v", err)
			}
			var vendor, resources []spec.UnsupportedDetail
			for _, item := range result.Unsupported {
				if item.Feature == spec.AuditResourceLimitFeature {
					resources = append(resources, item)
					continue
				}
				vendor = append(vendor, item)
			}
			if len(vendor) != 1 || vendor[0].Feature != "create_sequence" || vendor[0].Reason != spec.UnsupportedVendorBoundaryReason || vendor[0].Index != 0 {
				t.Fatalf("vendor boundary entry = %+v, want exactly one create_sequence at index 0", vendor)
			}
			if len(resources) != 3-limit {
				t.Fatalf("resource entries = %d, want %d", len(resources), 3-limit)
			}
			for i, item := range resources {
				index := limit + i
				if item.Index != index {
					t.Fatalf("resource entry %d index = %d, want %d", i, item.Index, index)
				}
				if got := item.Metadata["consumed"]; got != limit {
					t.Fatalf("resource entry %d consumed = %v, want %d — contamination must not reset the budget", i, got, limit)
				}
			}
		})
	}
}

func TestT05A7RequestIsolation(t *testing.T) {
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		t.Run(string(dialect), func(t *testing.T) {
			policy := t05A7FourRulePolicy(t)
			var control, quota2 report.Result
			for _, limit := range []int{2, 3, 0} {
				provider := &t05AbsentProvider{}
				result, err := t05A7Audit(t, t05A7FirstPathSQL, dialect, provider, policy, limit)
				if limit == 3 {
					if err != nil {
						t.Fatalf("control audit: %v", err)
					}
					control = result
				} else if !errors.Is(err, ErrUnsupportedStatement) {
					t.Fatalf("limit %d: expected ErrUnsupportedStatement, got %v", limit, err)
				}
				if limit == 2 {
					quota2 = result
				}
				wantCalls := []string(nil)
				if limit > 0 {
					wantCalls = []string{"golden.t"}
				}
				if !reflect.DeepEqual(provider.calls, wantCalls) {
					t.Fatalf("limit %d provider reads = %v, want %v", limit, provider.calls, wantCalls)
				}
				if want := 3 - limit; len(t05A7ResourceEntries(result)) != want {
					t.Fatalf("limit %d resource entries = %d, want %d", limit, len(t05A7ResourceEntries(result)), want)
				}
			}
			if !reflect.DeepEqual(quota2.Statements[:2], control.Statements[:2]) {
				t.Fatalf("quota 2 admitted prefix diverged from the independent quota 3 control:\n got %+v\nwant %+v", quota2.Statements[:2], control.Statements[:2])
			}
		})
	}
}

func TestT05A7ConcurrentFirstPath(t *testing.T) {
	for _, dialect := range []spec.Dialect{spec.DialectMySQL, spec.DialectTiDB} {
		for _, limit := range []int{0, 1, 2, 3} {
			dialect, limit := dialect, limit
			t.Run(fmt.Sprintf("%s/limit_%d", dialect, limit), func(t *testing.T) {
				t.Parallel()
				policy := t05A7FourRulePolicy(t)
				control, controlErr := t05A7Audit(t, t05A7FirstPathSQL, dialect, &t05AbsentProvider{}, policy, 3)
				if controlErr != nil {
					t.Fatalf("control audit: %v", controlErr)
				}
				provider := &t05AbsentProvider{}
				result, err := t05A7Audit(t, t05A7FirstPathSQL, dialect, provider, policy, limit)
				if limit == 3 {
					if err != nil {
						t.Fatalf("control audit: %v", err)
					}
				} else if !errors.Is(err, ErrUnsupportedStatement) {
					t.Fatalf("limit %d: expected ErrUnsupportedStatement, got %v", limit, err)
				}
				if want := 3 - limit; len(t05A7ResourceEntries(result)) != want {
					t.Fatalf("limit %d resource entries = %d, want %d", limit, len(t05A7ResourceEntries(result)), want)
				}
				if limit > 0 && !reflect.DeepEqual(result.Statements[:limit], control.Statements[:limit]) {
					t.Fatalf("limit %d prefix diverged from control", limit)
				}
				if limit > 0 && !reflect.DeepEqual(provider.calls, []string{"golden.t"}) {
					t.Fatalf("limit %d provider reads = %v, want [golden.t]", limit, provider.calls)
				}
				if limit == 0 && len(provider.calls) != 0 {
					t.Fatalf("limit 0 provider reads = %v, want none", provider.calls)
				}
			})
		}
	}
}
