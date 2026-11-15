package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule/ddl"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule/dml"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// routine_t27_test.go pins the T27 first golden path (issue #106): a
// CREATE PROCEDURE compound body is one top-level definition statement;
// the body's static SQL flows through the shared rule semantics with
// findings bound to the original body source positions. Semicolons
// inside BEGIN…END, strings, comments, and client DELIMITER wrappers
// are never outer migration boundaries.

func t27Registry(t *testing.T) *rule.Registry {
	t.Helper()
	registry := rule.NewRegistry()
	cfg := policy.Policy{Rules: map[string]policy.RulePolicy{
		"dml.where.require":          {Enabled: true},
		"ddl.create_procedure.notice": {Enabled: true},
	}}
	if err := dml.Register(registry, cfg); err != nil {
		t.Fatalf("register dml: %v", err)
	}
	if err := ddl.Register(registry, cfg); err != nil {
		t.Fatalf("register ddl: %v", err)
	}
	return registry
}

func t27Statements(t *testing.T, dialect spec.Dialect, sql string) []spec.Statement {
	t.Helper()
	parsed, err := Parse(context.Background(), sql, dialect)
	if err != nil {
		t.Fatalf("parse %q: %v", sql, err)
	}
	stmts, err := Extract(context.Background(), parsed)
	if err != nil {
		t.Fatalf("extract %q: %v", sql, err)
	}
	return stmts
}

func TestT27CompoundBodyDeleteFiresWhereRequire(t *testing.T) {
	stmts := t27Statements(t, spec.DialectMySQL, "CREATE PROCEDURE p() BEGIN DELETE FROM t; END;")
	if len(stmts) != 1 {
		t.Fatalf("expected one top-level statement, got %d", len(stmts))
	}
	top := stmts[0]
	if top.Kind != spec.KindDDL || top.DDL == nil {
		t.Fatalf("expected DDL statement, got kind=%s ddl=%v", top.Kind, top.DDL)
	}
	if top.DDL.Operation != spec.DDLOperationCreateProcedure {
		t.Fatalf("expected create_procedure, got %s", top.DDL.Operation)
	}
	if top.DDL.Routine == nil || top.DDL.Routine.Body == nil {
		t.Fatalf("expected routine declaration with body, got %+v", top.DDL)
	}
	if len(top.DDL.Routine.Body.Statements) != 1 {
		t.Fatalf("expected one body statement, got %d", len(top.DDL.Routine.Body.Statements))
	}
	inner := top.DDL.Routine.Body.Statements[0]
	if inner.Kind != spec.KindDML || inner.DML == nil || inner.DML.Operation != spec.DMLOperationDelete {
		t.Fatalf("expected inner DELETE, got kind=%s dml=%+v", inner.Kind, inner.DML)
	}

	res, err := EvaluateStatements(context.Background(), t27Registry(t), stmts)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if res.Verdict != report.VerdictReject {
		t.Fatalf("expected reject, got %q", res.Verdict)
	}
	if len(res.Statements) != 1 {
		t.Fatalf("expected one statement result, got %d", len(res.Statements))
	}
	var where int
	for _, f := range res.Statements[0].Findings {
		if f.RuleID == "dml.where.require" {
			where++
			if f.Location == nil {
				t.Fatalf("where.require finding missing location: %+v", f)
			}
			// DELETE begins at byte 27 of the input (line 1, col 28).
			if f.Location.Line != 1 || f.Location.Column != 28 {
				t.Fatalf("expected body position 1:28, got %d:%d", f.Location.Line, f.Location.Column)
			}
			if f.Level != rule.LevelBlocker {
				t.Fatalf("expected blocker, got %s", f.Level)
			}
			if f.StatementIndex != 0 {
				t.Fatalf("finding must bind the routine statement index, got %d", f.StatementIndex)
			}
		}
	}
	if where != 1 {
		t.Fatalf("expected exactly one dml.where.require finding, got %d: %+v", where, res.Statements[0].Findings)
	}
}

func TestT27CompoundBodyDeleteWithWhereIsClean(t *testing.T) {
	stmts := t27Statements(t, spec.DialectMySQL, "CREATE PROCEDURE p() BEGIN DELETE FROM t WHERE id=1; END;")
	res, err := EvaluateStatements(context.Background(), t27Registry(t), stmts)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	for _, f := range res.Statements[0].Findings {
		if f.RuleID == "dml.where.require" {
			t.Fatalf("guarded delete must not fire where.require: %+v", f)
		}
	}
}

func TestT27BodySemicolonShapesStayInside(t *testing.T) {
	cases := []string{
		// string and comment semicolons never split the body
		"CREATE PROCEDURE p() BEGIN SET @s='a;b'; /* c;d */ DELETE FROM t; END;",
		// two body statements keep one top-level definition
		"CREATE PROCEDURE p() BEGIN UPDATE t SET a=1 WHERE id=1; DELETE FROM t; END;",
	}
	for _, sql := range cases {
		stmts := t27Statements(t, spec.DialectMySQL, sql)
		if len(stmts) != 1 {
			t.Fatalf("%q: expected one top-level statement, got %d", sql, len(stmts))
		}
		if stmts[0].DDL == nil || stmts[0].DDL.Routine == nil || stmts[0].DDL.Routine.Body == nil {
			t.Fatalf("%q: expected routine body, got %+v", sql, stmts[0].DDL)
		}
	}
}

func TestT27DelimiterWrapperSplitsOnDirective(t *testing.T) {
	sql := "DELIMITER ;;\nCREATE PROCEDURE p() BEGIN DELETE FROM t; END;;\nDELIMITER ;\nDROP TABLE t;"
	stmts := t27Statements(t, spec.DialectMySQL, sql)
	if len(stmts) != 2 {
		t.Fatalf("expected 2 statements (routine + drop), got %d", len(stmts))
	}
	if stmts[0].DDL == nil || stmts[0].DDL.Operation != spec.DDLOperationCreateProcedure {
		t.Fatalf("expected create_procedure first, got %+v", stmts[0].DDL)
	}
	if len(stmts[0].DDL.Routine.Body.Statements) != 1 {
		t.Fatalf("expected one body statement, got %d", len(stmts[0].DDL.Routine.Body.Statements))
	}
	res, err := EvaluateStatements(context.Background(), t27Registry(t), stmts)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	var where int
	for _, f := range res.Statements[0].Findings {
		if f.RuleID == "dml.where.require" {
			where++
			if f.Location == nil || f.Location.Line != 2 {
				t.Fatalf("expected body position on line 2, got %+v", f.Location)
			}
		}
	}
	if where != 1 {
		t.Fatalf("expected one where.require, got %d", where)
	}
}

func TestT27DelimiterDollarAndSlashTerminators(t *testing.T) {
	// Regression: `END$$`/`END//` — the non-";" terminator is client
	// framing, never part of the statement handed to the parser; and '$'
	// is not a word byte, so `END$$` still scans the "end" keyword.
	cases := []struct {
		name string
		sql  string
	}{
		{"dollar", "DELIMITER $$\nCREATE PROCEDURE p() BEGIN DELETE FROM t; END$$\nDELIMITER ;\n"},
		{"slash", "DELIMITER //\nCREATE PROCEDURE p() BEGIN DELETE FROM t; END//\nDELIMITER ;\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stmts := t27Statements(t, spec.DialectMySQL, tc.sql)
			if len(stmts) != 1 {
				t.Fatalf("expected one top-level statement, got %d", len(stmts))
			}
			if stmts[0].DDL == nil || stmts[0].DDL.Operation != spec.DDLOperationCreateProcedure {
				t.Fatalf("expected create_procedure, got %+v", stmts[0].DDL)
			}
			res, err := EvaluateStatements(context.Background(), t27Registry(t), stmts)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			var where int
			for _, f := range res.Statements[0].Findings {
				if f.RuleID == "dml.where.require" {
					where++
				}
			}
			if where != 1 {
				t.Fatalf("expected one where.require finding, got %d", where)
			}
		})
	}
}

func TestT27RoutineDepthDoesNotLeakAcrossStatements(t *testing.T) {
	// Regression: a completed routine must not leave endSuppressed /
	// routineDepth armed — the next routine's IF/WHILE constructs still
	// nest inside its own BEGIN…END instead of splitting at body ";"s.
	sql := "CREATE PROCEDURE p1() BEGIN SELECT 1; END;\n" +
		"CREATE PROCEDURE p2() BEGIN IF 1=1 THEN DELETE FROM t; END IF; WHILE 0 DO DELETE FROM t2; END WHILE; END;\n"
	stmts := t27Statements(t, spec.DialectMySQL, sql)
	if len(stmts) != 2 {
		t.Fatalf("expected two routine statements, got %d", len(stmts))
	}
	p2 := stmts[1]
	if p2.DDL == nil || p2.DDL.Routine == nil || p2.DDL.Routine.Body == nil {
		t.Fatalf("expected p2 routine body, got %+v", p2.DDL)
	}
	if len(p2.DDL.Routine.Body.Statements) != 2 {
		t.Fatalf("expected two bounded compound members (IF, WHILE), got %d", len(p2.DDL.Routine.Body.Statements))
	}
	for _, m := range p2.DDL.Routine.Body.Statements {
		if m.Statement.Unsupported == nil || m.Statement.Unsupported.Feature != "routine.body.compound_member" {
			t.Fatalf("expected bounded compound_member member, got %+v", m.Statement.Unsupported)
		}
	}
}

func TestT27TiDBRoutineVendorBoundary(t *testing.T) {
	stmts := t27Statements(t, spec.DialectTiDB, "CREATE PROCEDURE p() BEGIN DELETE FROM t; END;")
	if len(stmts) != 1 {
		t.Fatalf("expected one statement, got %d", len(stmts))
	}
	if stmts[0].Unsupported == nil {
		t.Fatalf("expected vendor-boundary unsupported under TiDB, got %+v", stmts[0])
	}
}
