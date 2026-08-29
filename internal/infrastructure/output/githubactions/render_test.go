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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

func TestRenderIncludesLocatedParserDiagnostic(t *testing.T) {
	t.Parallel()
	result := report.Result{Diagnostics: []spec.Diagnostic{{
		Classification: "parser_error",
		Reason:         "statement was not audited",
		ActionHint:     "verify the selected dialect",
		Audited:        false,
		Dialect:        "mysql",
		Line:           2,
		Column:         3,
	}}}

	output, err := Render(result, Options{Path: "migrations/001.sql"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	rendered := string(output)
	for _, want := range []string{"::error", "file=migrations/001.sql", "line=2", "col=3", "title=Parser Error", "statement was not audited"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("expected %q in parser annotation, got %q", want, rendered)
		}
	}
}

func TestRenderIncludesFilePathWhenProvided(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

func TestRenderLocationWithoutPathOmitsFile(t *testing.T) {
	t.Parallel()
	result := report.Result{
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Findings: []rule.Finding{{
				RuleID:  "dml.where.require",
				Level:   rule.LevelBlocker,
				Message: "where clause is required",
				Location: &rule.Location{
					Line:   5,
					Column: 2,
				},
			}},
		}},
	}

	output, err := Render(result, Options{})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	rendered := string(output)
	if strings.Contains(rendered, "file=") {
		t.Fatalf("expected no file= when path is empty, got %q", rendered)
	}
	if !strings.Contains(rendered, "line=5") {
		t.Fatalf("expected line=5 in annotation, got %q", rendered)
	}
	if !strings.Contains(rendered, "col=2") {
		t.Fatalf("expected col=2 in annotation, got %q", rendered)
	}
}

func TestRenderUnsupportedAsNotice(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

// TestRenderAnnotationTitleIncludesLevelAndRuleID verifies that a finding
// annotation title carries the level and rule id so a reviewer can triage
// without opening raw logs.
func TestRenderAnnotationTitleIncludesLevelAndRuleID(t *testing.T) {
	t.Parallel()
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
	if !strings.Contains(rendered, "title=[blocker] dml.where.require") {
		t.Fatalf("expected title=[blocker] dml.where.require, got %q", rendered)
	}
	if !strings.Contains(rendered, "dml.where.require") {
		t.Fatalf("expected rule id still present in annotation, got %q", rendered)
	}
}

// TestRenderAnnotationMessageIncludesExplainCommand verifies the message
// carries the follow-up explain command so a reviewer can resolve a finding
// without searching docs. Workflow commands escape newlines as %0A.
func TestRenderAnnotationMessageIncludesExplainCommand(t *testing.T) {
	t.Parallel()
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
	if !strings.Contains(rendered, "%0AExplain: deltascope rules explain dml.where.require") {
		t.Fatalf("expected escaped explain command in message, got %q", rendered)
	}
}

// TestRenderAnnotationSuggestionPrecedesExplain verifies that when a finding
// carries a suggestion it is emitted before the explain command, preserving
// remediation-before-followup ordering.
func TestRenderAnnotationSuggestionPrecedesExplain(t *testing.T) {
	t.Parallel()
	result := report.Result{
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Findings: []rule.Finding{{
				RuleID:      "dml.where.require",
				Level:       rule.LevelBlocker,
				Message:     "where clause is required",
				Explanation: &rule.FindingExplanation{Suggestion: "Add a WHERE clause."},
			}},
		}},
	}

	output, err := Render(result, Options{})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	rendered := string(output)
	suggestion := strings.Index(rendered, "Suggestion: Add a WHERE clause.")
	explain := strings.Index(rendered, "Explain: deltascope rules explain dml.where.require")
	if suggestion < 0 {
		t.Fatalf("expected suggestion text in annotation, got %q", rendered)
	}
	if explain < 0 {
		t.Fatalf("expected explain command in annotation, got %q", rendered)
	}
	if suggestion >= explain {
		t.Fatalf("expected suggestion before explain; suggestion at %d, explain at %d, got %q", suggestion, explain, rendered)
	}
}

// TestRenderUnsupportedNoticeDoesNotAddExplainCommand verifies that
// unsupported-statement notices keep their existing shape and do not gain a
// rules explain command, since unsupported statements have no rule id.
func TestRenderUnsupportedNoticeDoesNotAddExplainCommand(t *testing.T) {
	t.Parallel()
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
	if strings.Contains(rendered, "rules explain") {
		t.Fatalf("expected unsupported notice to omit rules explain command, got %q", rendered)
	}
}

// TestRenderEscapesExplainMessageNewline verifies the newline inserted before
// the Explain line is escaped as %0A, consistent with workflow-command rules.
func TestRenderEscapesExplainMessageNewline(t *testing.T) {
	t.Parallel()
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
	if !strings.Contains(rendered, "%0AExplain:") {
		t.Fatalf("expected %%0A-escaped newline before Explain:, got %q", rendered)
	}
	if strings.Contains(rendered, "\nExplain:") {
		t.Fatalf("expected no raw newline before Explain:, got %q", rendered)
	}
}
