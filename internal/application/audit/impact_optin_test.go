// Package audit verifies default-disabled DML impact-rule behavior.
// input: default and opt-in policy audits of UPDATE/DELETE statements
// output: no default impact-rule findings, preserved statement impact objects, and config-enabled findings
// pos: application audit proof that dml.impact.* rules stay cataloged but Default Policy disabled
// note: if this file changes, update this header and module README.md.
package audit

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestDefaultAuditDoesNotEmitImpactRuleFindings(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		sql     string
		verdict report.Verdict
	}{
		{name: "delete missing where", sql: "DELETE FROM users;", verdict: report.VerdictReject},
		{name: "update pk equality", sql: "UPDATE users SET status = 'disabled' WHERE id = 1;", verdict: report.VerdictPass},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := AuditSQL(context.Background(), Request{
				SQL:     tc.sql,
				Dialect: spec.DialectMySQL,
			})
			if err != nil {
				t.Fatalf("audit sql: %v", err)
			}
			if result.Verdict != tc.verdict {
				t.Fatalf("verdict = %q, want %q", result.Verdict, tc.verdict)
			}
			if len(result.Statements) != 1 || result.Statements[0].Impact == nil {
				t.Fatalf("expected statement impact object, got %#v", result.Statements)
			}
			assertNoImpactRuleFindings(t, result)
		})
	}
}

func TestEnabledImpactConfigEmitsFindingsFromImpactObject(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "deltascope.yaml")
	content := []byte(`
rules:
  dml.impact.estimate:
    enabled: true
    level: notice
  dml.impact.rows.max_count:
    enabled: true
    level: warning
    params:
      value: 1000
  dml.impact.ratio.max_percent:
    enabled: true
    level: warning
    params:
      value: 10
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	result, err := AuditSQL(context.Background(), Request{
		SQL:        "DELETE FROM users;",
		Dialect:    spec.DialectMySQL,
		ConfigPath: path,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	if result.Verdict != report.VerdictReject {
		t.Fatalf("expected reject from WHERE require, got %q", result.Verdict)
	}
	if len(result.Statements) != 1 || result.Statements[0].Impact == nil {
		t.Fatalf("expected statement impact object, got %#v", result.Statements)
	}

	foundRatio := false
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "dml.impact.ratio.max_percent" {
			foundRatio = true
		}
	}
	if !foundRatio {
		t.Fatalf("expected enabled ratio rule finding, got %#v", result.Statements[0].Findings)
	}
}

func assertNoImpactRuleFindings(t *testing.T, result report.Result) {
	t.Helper()
	for _, statement := range result.Statements {
		for _, finding := range statement.Findings {
			switch finding.RuleID {
			case "dml.impact.estimate", "dml.impact.rows.max_count", "dml.impact.ratio.max_percent":
				t.Fatalf("default audit must not emit %s, got %#v", finding.RuleID, finding)
			}
		}
	}
	for _, finding := range result.GlobalFindings {
		switch finding.RuleID {
		case "dml.impact.estimate", "dml.impact.rows.max_count", "dml.impact.ratio.max_percent":
			t.Fatalf("default audit must not emit global %s, got %#v", finding.RuleID, finding)
		}
	}
}
