package audit

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestRoutineSecurityDefaultPolicy(t *testing.T) {
	for _, tc := range []struct {
		name, sql, dialect, source string
		warn, incomplete           bool
	}{
		{"procedure explicit", "CREATE PROCEDURE p() SQL SECURITY DEFINER SET @x=1", "mysql", "authored", true, false},
		{"procedure default", "CREATE PROCEDURE p() SET @x=1", "mysql", "create_default", true, false},
		{"procedure invoker", "CREATE PROCEDURE p() SQL SECURITY INVOKER SET @x=1", "mysql", "", false, false},
		{"function return", "CREATE FUNCTION f() RETURNS INT SQL SECURITY DEFINER RETURN 1", "mysql", "authored", true, true},
		{"function default return", "CREATE FUNCTION f() RETURNS INT RETURN 1", "mysql", "create_default", true, true},
		{"function invoker return", "CREATE FUNCTION f() RETURNS INT SQL SECURITY INVOKER RETURN 1", "mysql", "", false, true},
		{"alter procedure", "ALTER PROCEDURE p SQL SECURITY DEFINER", "mysql", "authored", true, false},
		{"alter function", "ALTER FUNCTION f SQL SECURITY DEFINER", "mysql", "authored", true, false},
		{"alter omission", "ALTER PROCEDURE p COMMENT 'sql security definer'", "mysql", "", false, false},
		{"alter function omission", "ALTER FUNCTION f NO SQL", "mysql", "", false, false},
		{"alter invoker", "ALTER FUNCTION f SQL SECURITY INVOKER", "mysql", "", false, false},
		{"account only with invoker", "CREATE DEFINER='account_sentinel'@'host_sentinel' PROCEDURE p() SQL SECURITY INVOKER SET @x=1", "mysql", "", false, false},
		{"account with omitted mode", "CREATE DEFINER='account_sentinel'@'host_sentinel' PROCEDURE p() SET @x=1", "mysql", "create_default", true, false},
		{"body boundary retained", "CREATE PROCEDURE p() SQL SECURITY DEFINER BEGIN EXECUTE stmt; END", "mysql", "authored", true, true},
		{"soname", "CREATE FUNCTION f RETURNS INTEGER SONAME 'lib_sentinel.so'", "mysql", "", false, false},
		{"tidb vendor", "CREATE PROCEDURE p() SQL SECURITY DEFINER SET @x=1", "tidb", "", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := AuditSQL(context.Background(), Request{SQL: tc.sql, Dialect: spec.Dialect(tc.dialect)})
			if tc.incomplete != errors.Is(err, ErrUnsupportedStatement) || (err != nil && !errors.Is(err, ErrUnsupportedStatement)) {
				t.Fatalf("incomplete=%t, err=%v", tc.incomplete, err)
			}
			count := 0
			for _, f := range res.Statements[0].Findings {
				if f.RuleID == "ddl.routine.security.definer.warn" {
					count++
					if f.Metadata["security_source"] != tc.source || f.Location == nil || f.Location.Line != 1 || f.Location.Column != 1 {
						t.Fatalf("source/location lost: %+v", f)
					}
				}
			}
			if (count == 1) != tc.warn || count > 1 {
				t.Fatalf("warning count=%d, want warn=%t", count, tc.warn)
			}
			if tc.warn && res.Verdict != report.VerdictReview {
				t.Fatalf("default-on warning must review: %s", res.Verdict)
			}
			if tc.incomplete && len(res.Unsupported) == 0 {
				t.Fatal("lost independent unsupported boundary")
			}
		})
	}
}

func TestRoutineSecurityUnownedParserModesStayUnverified(t *testing.T) {
	for _, mode := range []string{"UNKNOWN", "INVOKER SQL SECURITY DEFINER", "DEFINER SQL SECURITY INVOKER", "INVOKER SQL SECURITY INVOKER"} {
		res, err := AuditSQL(context.Background(), Request{SQL: "ALTER PROCEDURE p SQL SECURITY " + mode, Dialect: spec.DialectMySQL})
		if err != nil || res.Verdict != report.VerdictReview || len(res.Statements) != 1 || len(res.Statements[0].EvidenceGaps) != 1 {
			t.Fatalf("unowned %q must not pass: result=%+v err=%v", mode, res, err)
		}
		if res.Statements[0].EvidenceGaps[0].ReasonCode != "unproven_routine_security" {
			t.Fatalf("unexpected gap: %+v", res.Statements[0].EvidenceGaps)
		}
	}
}

func TestRoutineSecurityCharacteristicSeam(t *testing.T) {
	for _, kind := range []string{"PROCEDURE", "FUNCTION"} {
		for _, op := range []string{"CREATE", "ALTER"} {
			for _, mode := range []string{"definer", "invoker", ""} {
				t.Run(op+"/"+kind+"/"+mode, func(t *testing.T) {
					name, returns, body := "`app`.`Routine`", "", ""
					if op == "CREATE" {
						name += "()"
						body = " SET @probe=1"
						if kind == "FUNCTION" {
							returns, body = " RETURNS INT", " RETURN 1"
						}
					}
					characteristic := " COMMENT 'seam'"
					if mode != "" {
						characteristic = " SQL SECURITY " + mode
					}
					sql := fmt.Sprintf("%s %s %s%s%s%s", op, kind, name, returns, characteristic, body)
					stmts := t27Statements(t, spec.DialectMySQL, sql)
					if len(stmts) != 1 || stmts[0].DDL == nil || stmts[0].DDL.Routine == nil {
						t.Fatalf("missing routine declaration: %+v", stmts)
					}
					decl := stmts[0].DDL.Routine
					if decl.Name.Schema != "app" || decl.Name.Name != "routine" {
						t.Fatalf("qualified identity lost: %+v", decl.Name)
					}
					got := ""
					for _, c := range decl.Characteristics {
						if c.Key == "sql_security" {
							got = c.Value
							if c.Offset < 0 || sql[c.Offset:c.Offset+12] != "SQL SECURITY" {
								t.Fatalf("security offset lost: %+v", c)
							}
						}
					}
					if got != mode {
						t.Fatalf("security=%q, want %q", got, mode)
					}
				})
			}
		}
	}
}
