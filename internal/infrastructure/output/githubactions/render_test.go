// Package githubactions verifies GitHub Actions annotation output.
// input: synthetic report results with blocker/warning/notice findings and optional locations
// output: test coverage for GitHub Actions annotation severity mapping and location formatting
// pos: infrastructure output adapter test coverage for CI-native GitHub Actions format
// note: if this file changes, update this header and module README.md.
package githubactions

import (
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestRenderMapsBlockerToError(t *testing.T) {
	result := report.Result{
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Findings: []rule.Finding{{
				RuleID:  "dml.where.require",
				Level:   rule.LevelBlocker,
				Message: "where clause is required",
			}},
		}},
	}

	output, err := Render(result, Options{})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	rendered := string(output)
	if !strings.Contains(rendered, "::error") {
		t.Fatalf("expected ::error annotation for blocker, got %q", rendered)
	}
	if !strings.Contains(rendered, "where clause is required") {
		t.Fatalf("expected blocker message in output, got %q", rendered)
	}
}

func TestRenderMapsWarningToWarning(t *testing.T) {
	result := report.Result{
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "ddl",
			Findings: []rule.Finding{{
				RuleID:  "ddl.table.comment.require",
				Level:   rule.LevelWarning,
				Message: "table comment is required",
			}},
		}},
	}

	output, err := Render(result, Options{})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	rendered := string(output)
	if !strings.Contains(rendered, "::warning") {
		t.Fatalf("expected ::warning annotation for warning, got %q", rendered)
	}
	if !strings.Contains(rendered, "table comment is required") {
		t.Fatalf("expected warning message in output, got %q", rendered)
	}
}

func TestRenderMapsNoticeToNotice(t *testing.T) {
	result := report.Result{
		GlobalFindings: []rule.Finding{{
			RuleID:  "audit.batch.notice",
			Level:   rule.LevelNotice,
			Message: "batch processed",
		}},
	}

	output, err := Render(result, Options{})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	rendered := string(output)
	if !strings.Contains(rendered, "::notice") {
		t.Fatalf("expected ::notice annotation for notice, got %q", rendered)
	}
}

func TestRenderIncludesLocationWhenPresent(t *testing.T) {
	result := report.Result{
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Findings: []rule.Finding{{
				RuleID:  "dml.where.require",
				Level:   rule.LevelBlocker,
				Message: "where clause is required",
				Location: &rule.Location{
					Line:   3,
					Column: 1,
				},
			}},
		}},
	}

	output, err := Render(result, Options{})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	rendered := string(output)
	if !strings.Contains(rendered, "line=3") {
		t.Fatalf("expected line=3 in annotation, got %q", rendered)
	}
	if !strings.Contains(rendered, "col=1") {
		t.Fatalf("expected col=1 in annotation, got %q", rendered)
	}
}

func TestRenderIncludesFilePathWhenProvided(t *testing.T) {
	result := report.Result{
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Findings: []rule.Finding{{
				RuleID:  "dml.where.require",
				Level:   rule.LevelBlocker,
				Message: "where clause is required",
				Location: &rule.Location{
					Line:   9,
					Column: 1,
				},
			}},
		}},
	}

	output, err := Render(result, Options{Path: "migrations/001.sql"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	rendered := string(output)
	if !strings.Contains(rendered, "file=migrations/001.sql") {
		t.Fatalf("expected file=migrations/001.sql in annotation, got %q", rendered)
	}
	if !strings.Contains(rendered, "line=9") {
		t.Fatalf("expected line=9 in annotation, got %q", rendered)
	}
}

func TestRenderOmitsLocationWhenAbsent(t *testing.T) {
	result := report.Result{
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Findings: []rule.Finding{{
				RuleID:  "dml.where.require",
				Level:   rule.LevelBlocker,
				Message: "where clause is required",
			}},
		}},
	}

	output, err := Render(result, Options{})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	rendered := string(output)
	if strings.Contains(rendered, "line=") {
		t.Fatalf("expected no line= in annotation without location, got %q", rendered)
	}
}

func TestRenderUnsupportedAsNotice(t *testing.T) {
	result := report.Result{
		Unsupported: []spec.UnsupportedDetail{
			{Index: 0, Feature: "select", Reason: "not supported"},
		},
	}

	output, err := Render(result, Options{})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	rendered := string(output)
	if !strings.Contains(rendered, "::notice") {
		t.Fatalf("expected ::notice for unsupported statement, got %q", rendered)
	}
	if !strings.Contains(rendered, "not supported") {
		t.Fatalf("expected unsupported reason in output, got %q", rendered)
	}
}

func TestRenderEmptyResultProducesNoAnnotations(t *testing.T) {
	result := report.Result{Verdict: report.VerdictPass}

	output, err := Render(result, Options{})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if len(output) != 0 {
		t.Fatalf("expected empty output for clean result, got %q", string(output))
	}
}

func TestRenderEscapesSpecialCharacters(t *testing.T) {
	result := report.Result{
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Findings: []rule.Finding{{
				RuleID:  "dml.where.require",
				Level:   rule.LevelBlocker,
				Message: "progress 50%\nnext line",
			}},
		}},
	}

	output, err := Render(result, Options{})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	rendered := string(output)
	if strings.Contains(rendered, "50%\n") {
		t.Fatalf("expected %% to be escaped and \\n to be escaped, got %q", rendered)
	}
	if !strings.Contains(rendered, "50%25") {
		t.Fatalf("expected %% → %%25 encoding, got %q", rendered)
	}
	if !strings.Contains(rendered, "%0A") {
		t.Fatalf("expected \\n → %%0A encoding, got %q", rendered)
	}
}
