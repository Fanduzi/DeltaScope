//go:build postgresql

// Package audit verifies the T05-A8 PostgreSQL procedure lifecycle control.
// input: a real PostgreSQL CREATE/DROP PROCEDURE batch through AuditSQL with the isolated pg lifecycle policy
// output: unchanged legacy per-statement notices and no ordered-state procedure boundary
// pos: tagged regression control proving the MySQL/TiDB procedure table-state branch never fires for PostgreSQL
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestT05A8PostgreSQLProcedureControl(t *testing.T) {
	for _, tc := range []struct {
		name, sql, ruleID string
	}{
		{"create", "CREATE PROCEDURE reset_counter() LANGUAGE plpgsql AS $$ BEGIN NULL; END $$;", "ddl.pg.create_procedure.notice"},
		{"drop", "DROP PROCEDURE reset_counter();", "ddl.pg.drop_procedure.advisory"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			policy := t05PolicyPathConfigured(t, map[string]t05RuleConfig{tc.ruleID: {level: "notice"}})
			result, err := AuditSQL(context.Background(), Request{
				SQL: tc.sql, Dialect: spec.DialectPostgreSQL, ConfigPath: policy,
			})
			if err != nil {
				t.Fatalf("audit: %v", err)
			}
			if len(result.Statements) != 1 {
				t.Fatalf("statements = %d, want 1", len(result.Statements))
			}
			statement := result.Statements[0]
			if statement.Kind != "ddl" || statement.Coverage.Status != report.CoverageComplete || len(statement.EvidenceGaps) != 0 {
				t.Fatalf("statement = %+v, want one complete ddl", statement)
			}
			if result.Verdict != report.VerdictPass || result.Coverage.Status != report.CoverageComplete {
				t.Fatalf("aggregate = %s/%s, want pass/complete", result.Verdict, result.Coverage.Status)
			}
			if len(result.Unsupported) != 0 {
				t.Fatalf("unsupported = %+v, want none", result.Unsupported)
			}
			if len(statement.Findings) != 1 || statement.Findings[0].RuleID != tc.ruleID || statement.Findings[0].Level != rule.LevelNotice {
				t.Fatalf("findings = %+v, want exactly one %s notice", statement.Findings, tc.ruleID)
			}
		})
	}
}
