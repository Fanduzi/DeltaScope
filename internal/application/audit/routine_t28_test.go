package audit

import (
	"context"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule/ddl"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule/dml"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// routine_t28_test.go pins the T28 first golden path (issue #107):
// stored-body control flow, variable binding, and dynamic SQL get
// bounded analysis — statically known members still audit, dynamic
// sources stay explicitly incomplete, resource bounds never pass, and
// nothing executes or guesses from strings.

func t28Registry(t *testing.T) *rule.Registry {
	t.Helper()
	registry := rule.NewRegistry()
	cfg := policy.Policy{Rules: map[string]policy.RulePolicy{
		"dml.where.require":                   {Enabled: true},
		"ddl.create_procedure.notice":         {Enabled: true},
		"ddl.routine.variable_shadowing.warn": {Enabled: true},
	}}
	if err := dml.Register(registry, cfg); err != nil {
		t.Fatalf("register dml: %v", err)
	}
	if err := ddl.Register(registry, cfg); err != nil {
		t.Fatalf("register ddl: %v", err)
	}
	return registry
}

func t28Evaluate(t *testing.T, sql string) report.Result {
	t.Helper()
	parsed, err := Parse(context.Background(), sql, spec.DialectMySQL)
	if err != nil {
		t.Fatalf("parse %q: %v", sql, err)
	}
	stmts, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract %q: %v", sql, err)
	}
	res, err := EvaluateStatements(context.Background(), t28Registry(t), stmts)
	if err != nil {
		t.Fatalf("evaluate %q: %v", sql, err)
	}
	return res
}

func t28Unsupported(res report.Result) []spec.UnsupportedDetail {
	return res.Unsupported
}

func TestT28GoldenPathDynamicPrepareBounded(t *testing.T) {
	res := t28Evaluate(t, "CREATE PROCEDURE p(IN s TEXT) BEGIN SET @q=s; PREPARE stmt FROM @q; EXECUTE stmt; END;")
	if len(res.Statements) != 1 {
		t.Fatalf("expected one top-level definition, got %d", len(res.Statements))
	}
	if res.Coverage.Status != report.CoverageIncomplete {
		t.Fatalf("expected incomplete coverage (dynamic boundary), got %q", res.Coverage.Status)
	}
	feats := map[string]bool{}
	for _, u := range t28Unsupported(res) {
		feats[u.Feature] = true
	}
	if !feats["routine.body.prepare"] {
		t.Fatalf("expected bounded routine.body.prepare, got %+v", t28Unsupported(res))
	}
	if !feats["execute_prepared"] {
		t.Fatalf("expected execute_prepared boundary, got %+v", t28Unsupported(res))
	}
	for _, f := range res.Statements[0].Findings {
		if f.RuleID == "dml.where.require" {
			t.Fatalf("dynamic path must not fabricate where.require: %+v", f)
		}
	}
}

func TestT28CompoundMemberInnerSQLAudited(t *testing.T) {
	res := t28Evaluate(t, "CREATE PROCEDURE p() BEGIN IF 1=1 THEN DELETE FROM t; END IF; END;")
	var where int
	for _, f := range res.Statements[0].Findings {
		if f.RuleID == "dml.where.require" {
			where++
			if f.Location == nil || f.Location.Line != 1 {
				t.Fatalf("inner finding missing translated position: %+v", f)
			}
		}
	}
	if where != 1 {
		t.Fatalf("inner DELETE must fire where.require, got %d: %+v", where, res.Statements[0].Findings)
	}
}

func TestT28LoopBodyInnerSQLAudited(t *testing.T) {
	for _, body := range []string{
		"WHILE 0 DO DELETE FROM t; END WHILE",
		"lbl: LOOP DELETE FROM t; END LOOP lbl",
		"REPEAT DELETE FROM t; UNTIL 1 END REPEAT",
		"CASE WHEN 1 THEN DELETE FROM t; ELSE SELECT 1; END CASE",
	} {
		res := t28Evaluate(t, "CREATE PROCEDURE p() BEGIN "+body+"; END;")
		var where int
		for _, f := range res.Statements[0].Findings {
			if f.RuleID == "dml.where.require" {
				where++
			}
		}
		if where != 1 {
			t.Fatalf("%q: expected inner where.require, got findings %+v unsupported %+v", body, res.Statements[0].Findings, t28Unsupported(res))
		}
	}
}

func TestT28PrepareLiteralAudited(t *testing.T) {
	res := t28Evaluate(t, "CREATE PROCEDURE p() BEGIN PREPARE s FROM 'DELETE FROM t'; END;")
	var where int
	for _, f := range res.Statements[0].Findings {
		if f.RuleID == "dml.where.require" {
			where++
		}
	}
	if where != 1 {
		t.Fatalf("literal prepared SQL must be audited, got %+v", res.Statements[0].Findings)
	}
}

func TestT28PrepareDynamicBoundIncomplete(t *testing.T) {
	for _, src := range []string{"@q", "@q2", "CONCAT('DELETE',' FROM t')", "v"} {
		res := t28Evaluate(t, "CREATE PROCEDURE p(IN v TEXT) BEGIN PREPARE s FROM "+src+"; END;")
		var prep bool
		for _, u := range t28Unsupported(res) {
			if u.Feature == "routine.body.prepare" {
				prep = true
			}
		}
		if !prep {
			t.Fatalf("%q: expected routine.body.prepare boundary, got %+v", src, t28Unsupported(res))
		}
	}
}

func TestT28SetBoundLiteralPropagatesToPrepare(t *testing.T) {
	for _, op := range []string{"=", ":="} {
		res := t28Evaluate(t, "CREATE PROCEDURE p() BEGIN SET @q"+op+"'DELETE FROM t'; PREPARE s FROM @q; END;")
		var where int
		for _, f := range res.Statements[0].Findings {
			if f.RuleID == "dml.where.require" {
				where++
			}
		}
		if where != 1 {
			t.Fatalf("SET %s-bound literal must resolve into audited prepare, got %+v", op, res.Statements[0].Findings)
		}
	}
}

func TestT28BranchMergeDoesNotFabricateBinding(t *testing.T) {
	res := t28Evaluate(t, "CREATE PROCEDURE p() BEGIN IF 1=1 THEN SET @q='DELETE FROM t'; END IF; PREPARE s FROM @q; END;")
	var prep bool
	for _, u := range t28Unsupported(res) {
		if u.Feature == "routine.body.prepare" {
			prep = true
		}
	}
	if !prep {
		t.Fatalf("branch-local binding must not leak into merged facts: %+v", t28Unsupported(res))
	}
	for _, f := range res.Statements[0].Findings {
		if f.RuleID == "dml.where.require" {
			t.Fatalf("merged-unknown prepare must not fabricate where.require: %+v", f)
		}
	}
}

func TestT28DeclareVariableModeled(t *testing.T) {
	res := t28Evaluate(t, "CREATE PROCEDURE p() BEGIN DECLARE x INT DEFAULT 0; DELETE FROM t WHERE id=x; END;")
	for _, u := range t28Unsupported(res) {
		if u.Feature == "routine.body.declaration" {
			t.Fatalf("plain variable DECLARE must be modeled, got %+v", u)
		}
	}
}

func TestT28VariableShadowingWarns(t *testing.T) {
	res := t28Evaluate(t, "CREATE PROCEDURE p(IN s TEXT) BEGIN DECLARE s INT; DELETE FROM t WHERE id=1; END;")
	var shadow bool
	for _, f := range res.Statements[0].Findings {
		if f.RuleID == "ddl.routine.variable_shadowing.warn" {
			shadow = true
			if f.Level != rule.LevelWarning {
				t.Fatalf("expected warning level, got %s", f.Level)
			}
		}
	}
	if !shadow {
		t.Fatalf("expected variable_shadowing finding, got %+v", res.Statements[0].Findings)
	}
}

func TestT28HandlerBodyAudited(t *testing.T) {
	res := t28Evaluate(t, "CREATE PROCEDURE p() BEGIN DECLARE h HANDLER FOR SQLEXCEPTION DELETE FROM t; END;")
	var where int
	for _, f := range res.Statements[0].Findings {
		if f.RuleID == "dml.where.require" {
			where++
		}
	}
	if where != 1 {
		t.Fatalf("handler body must audit through rules, got %+v", res.Statements[0].Findings)
	}
}

func TestT28DepthBoundExplicitIncomplete(t *testing.T) {
	inner := "DELETE FROM t WHERE id=1;"
	for i := 0; i < 12; i++ {
		inner = "IF 1=1 THEN " + inner + " END IF;"
	}
	res := t28Evaluate(t, "CREATE PROCEDURE p() BEGIN "+inner+" END;")
	var depth bool
	for _, u := range t28Unsupported(res) {
		if u.Feature == "routine.body.depth" {
			depth = true
		}
	}
	if !depth {
		t.Fatalf("depth overflow must be bounded incomplete, got %+v", t28Unsupported(res))
	}
}

func TestT28MemberParseFailureBoundedNotSilent(t *testing.T) {
	res := t28Evaluate(t, "CREATE PROCEDURE p() BEGIN GARBAGE SYNTAX HERE; END;")
	var bounded bool
	for _, u := range t28Unsupported(res) {
		if strings.HasPrefix(u.Feature, "routine.body.") {
			bounded = true
		}
	}
	if !bounded {
		t.Fatalf("unparseable member must be bounded incomplete, got %+v", t28Unsupported(res))
	}
}

// Spec-review regressions: mid-member keywords are never constructs
// (DROP TABLE IF EXISTS, IF() calls), and bounded members invalidate
// proven bindings rather than auditing stale literals.

func TestT28IfExistsInsideMemberNotConstruct(t *testing.T) {
	res := t28Evaluate(t, "CREATE PROCEDURE p() BEGIN DROP TABLE IF EXISTS t; DELETE FROM t; END;")
	var where int
	for _, f := range res.Statements[0].Findings {
		if f.RuleID == "dml.where.require" {
			where++
		}
	}
	if where != 1 {
		t.Fatalf("DROP TABLE IF EXISTS must not corrupt member splitting, got %+v", res.Statements[0].Findings)
	}
}

func TestT28CaseExpressionEndStaysInsideMember(t *testing.T) {
	res := t28Evaluate(t, "CREATE PROCEDURE p() BEGIN SELECT CASE 1 WHEN 1 THEN 1 END; SELECT 1; END;")
	for _, u := range t28Unsupported(res) {
		if strings.HasPrefix(u.Feature, "routine.body.") {
			t.Fatalf("case-expression END must stay inside its member, got %+v", t28Unsupported(res))
		}
	}
}

func TestT28SetGlobalNotVariableBinding(t *testing.T) {
	res := t28Evaluate(t, "CREATE PROCEDURE p() BEGIN SET GLOBAL gtid_mode=ON; END;")
	var bounded bool
	for _, u := range t28Unsupported(res) {
		if u.Feature == "set_global" {
			bounded = true
		}
	}
	if !bounded {
		t.Fatalf("SET GLOBAL must surface as a bounded statement, not a silent member, got %+v", t28Unsupported(res))
	}
}

func TestT28SetMixedTargetsKeepBoundedEvidence(t *testing.T) {
	res := t28Evaluate(t, "CREATE PROCEDURE p() BEGIN SET @q='x', @@global.y=1; END;")
	var bounded bool
	for _, u := range t28Unsupported(res) {
		if u.Feature == "routine.body.set" {
			bounded = true
		}
	}
	if !bounded {
		t.Fatalf("unmodeled SET targets must stay visible, got %+v", t28Unsupported(res))
	}
}

func TestT28BoundedMemberInvalidatesBinding(t *testing.T) {
	// The opaque member rebinds @q — a stale 'SELECT 1' must never be
	// audited as the prepared text.
	res := t28Evaluate(t, "CREATE PROCEDURE p() BEGIN SET @q='SELECT 1'; SELECT 'DELETE FROM t' INTO @q; PREPARE s FROM @q; END;")
	for _, f := range res.Statements[0].Findings {
		if f.RuleID == "dml.where.require" {
			t.Fatalf("stale literal must not fabricate a finding, got %+v", res.Statements[0].Findings)
		}
	}
	var prep bool
	for _, u := range t28Unsupported(res) {
		if u.Feature == "routine.body.prepare" {
			prep = true
		}
	}
	if !prep {
		t.Fatalf("PREPARE after an opaque member must be bounded, got %+v", t28Unsupported(res))
	}
}

func TestT28FlowMembersStayBounded(t *testing.T) {
	for _, verb := range []string{"OPEN c", "FETCH c INTO v", "LEAVE lbl", "SIGNAL SQLSTATE '45000'"} {
		sql := "CREATE PROCEDURE p() BEGIN lbl: BEGIN " + verb + "; END lbl; END;"
		res := t28Evaluate(t, sql)
		var flow bool
		for _, u := range t28Unsupported(res) {
			if u.Feature == "routine.body.flow" {
				flow = true
			}
		}
		if !flow {
			t.Fatalf("%q must be bounded flow evidence, got %+v", verb, t28Unsupported(res))
		}
	}
}

func TestT28NestedThenIfInnerSQLAudited(t *testing.T) {
	res := t28Evaluate(t, "CREATE PROCEDURE p() BEGIN IF 1=1 THEN IF 1=1 THEN DELETE FROM t; END IF; END IF; END;")
	var where int
	for _, f := range res.Statements[0].Findings {
		if f.RuleID == "dml.where.require" {
			where++
		}
	}
	if where != 1 {
		t.Fatalf("IF nested directly under THEN must audit, got %+v", res.Statements[0].Findings)
	}
}
