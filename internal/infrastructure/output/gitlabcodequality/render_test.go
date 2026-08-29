// Package gitlabcodequality_test verifies GitLab Code Quality output generation.
// input: synthetic report findings and parser diagnostics with source locations
// output: stable GitLab issue shape, severity, fingerprint, and location coverage
// pos: black-box infrastructure output adapter regression coverage
// note: if this file changes, update this header and module README.md.
package gitlabcodequality_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	"github.com/Fanduzi/DeltaScope/internal/infrastructure/output/gitlabcodequality"
)

func TestRenderEmptyResultReturnsEmptyJSONArray(t *testing.T) {
	t.Parallel()
	data, err := gitlabcodequality.Render(report.Result{}, gitlabcodequality.Options{})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if string(data) != "[]" {
		t.Errorf("expected [], got %s", data)
	}
}

func TestRenderStatementFindingHasRequiredKeys(t *testing.T) {
	t.Parallel()
	result := report.Result{
		Statements: []report.StatementResult{
			{
				Index: 0,
				Findings: []rule.Finding{
					{
						RuleID:   "ddl.table.comment.require",
						Level:    rule.LevelBlocker,
						Message:  "table comment is required",
						Location: &rule.Location{Line: 5},
					},
				},
			},
		},
	}
	data, err := gitlabcodequality.Render(result, gitlabcodequality.Options{Path: "migrations.sql"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	var issues []map[string]any
	if err := json.Unmarshal(data, &issues); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	issue := issues[0]

	assertStringField(t, issue, "check_name", "ddl.table.comment.require")
	assertStringField(t, issue, "description", "table comment is required")
	assertStringField(t, issue, "severity", "major")
	assertNonEmptyString(t, issue, "fingerprint")

	loc, ok := issue["location"].(map[string]any)
	if !ok {
		t.Fatal("location is not an object")
	}
	if loc["path"] != "migrations.sql" {
		t.Errorf("location.path = %v, want migrations.sql", loc["path"])
	}
	lines, ok := loc["lines"].(map[string]any)
	if !ok {
		t.Fatal("location.lines is not an object")
	}
	if lines["begin"] != float64(5) {
		t.Errorf("location.lines.begin = %v, want 5", lines["begin"])
	}
}

func TestRenderParserDiagnosticHasRequiredKeys(t *testing.T) {
	t.Parallel()
	data, err := gitlabcodequality.Render(report.Result{Diagnostics: []spec.Diagnostic{{
		Classification: "parser_error",
		Reason:         "statement was not audited",
		ActionHint:     "verify the selected dialect",
		Line:           2,
		Column:         3,
	}}}, gitlabcodequality.Options{Path: "migrations.sql"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	var issues []map[string]any
	if err := json.Unmarshal(data, &issues); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected one parser diagnostic issue, got %s", data)
	}
	assertStringField(t, issues[0], "check_name", "parser_error")
	assertStringField(t, issues[0], "severity", "major")
	lines := issues[0]["location"].(map[string]any)["lines"].(map[string]any)
	if lines["begin"] != float64(2) {
		t.Fatalf("expected line 2, got %#v", lines)
	}
}

func TestRenderGlobalFindingHasRequiredKeys(t *testing.T) {
	t.Parallel()
	result := report.Result{
		GlobalFindings: []rule.Finding{
			{RuleID: "global.policy.check", Level: rule.LevelNotice, Message: "global notice"},
		},
	}
	data, err := gitlabcodequality.Render(result, gitlabcodequality.Options{Path: "schema.sql"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	var issues []map[string]any
	if err := json.Unmarshal(data, &issues); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}

	assertStringField(t, issues[0], "check_name", "global.policy.check")
	assertStringField(t, issues[0], "description", "global notice")
	assertStringField(t, issues[0], "severity", "info")
	loc := issues[0]["location"].(map[string]any)
	if loc["path"] != "schema.sql" {
		t.Errorf("path = %v, want schema.sql", loc["path"])
	}
}

func TestSeverityMapping(t *testing.T) {
	t.Parallel()
	cases := []struct {
		level    rule.Level
		expected string
	}{
		{rule.LevelBlocker, "major"},
		{rule.LevelWarning, "minor"},
		{rule.LevelNotice, "info"},
	}
	for _, tc := range cases {
		result := report.Result{
			GlobalFindings: []rule.Finding{
				{RuleID: "test", Level: tc.level, Message: "msg"},
			},
		}
		data, _ := gitlabcodequality.Render(result, gitlabcodequality.Options{})
		var issues []map[string]any
		json.Unmarshal(data, &issues)
		assertStringField(t, issues[0], "severity", tc.expected)
	}
}

func TestSeverityMappingUnknown(t *testing.T) {
	t.Parallel()
	result := report.Result{
		GlobalFindings: []rule.Finding{
			{RuleID: "test", Level: "unknown", Message: "msg"},
		},
	}
	data, _ := gitlabcodequality.Render(result, gitlabcodequality.Options{})
	var issues []map[string]any
	json.Unmarshal(data, &issues)
	assertStringField(t, issues[0], "severity", "minor")
}

func TestPathCleanupRemovesDotSlash(t *testing.T) {
	t.Parallel()
	result := report.Result{
		GlobalFindings: []rule.Finding{
			{RuleID: "test", Level: rule.LevelWarning, Message: "msg"},
		},
	}
	data, _ := gitlabcodequality.Render(result, gitlabcodequality.Options{Path: "./migrations.sql"})
	var issues []map[string]any
	json.Unmarshal(data, &issues)
	loc := issues[0]["location"].(map[string]any)
	if loc["path"] != "migrations.sql" {
		t.Errorf("path = %v, want migrations.sql (stripped ./)", loc["path"])
	}
}

func TestEmptyPathUsesDefault(t *testing.T) {
	t.Parallel()
	result := report.Result{
		GlobalFindings: []rule.Finding{
			{RuleID: "test", Level: rule.LevelWarning, Message: "msg"},
		},
	}
	data, _ := gitlabcodequality.Render(result, gitlabcodequality.Options{})
	var issues []map[string]any
	json.Unmarshal(data, &issues)
	loc := issues[0]["location"].(map[string]any)
	if loc["path"] != "deltascope.sql" {
		t.Errorf("path = %v, want deltascope.sql", loc["path"])
	}
}

func TestLineFallbackUsesStatementIndex(t *testing.T) {
	t.Parallel()
	result := report.Result{
		Statements: []report.StatementResult{
			{
				Index: 3,
				Findings: []rule.Finding{
					{RuleID: "test", Level: rule.LevelWarning, Message: "msg", StatementIndex: 3},
				},
			},
		},
	}
	data, _ := gitlabcodequality.Render(result, gitlabcodequality.Options{})
	var issues []map[string]any
	json.Unmarshal(data, &issues)
	loc := issues[0]["location"].(map[string]any)
	lines := loc["lines"].(map[string]any)
	if lines["begin"] != float64(4) {
		t.Errorf("line = %v, want 4 (statementIndex+1)", lines["begin"])
	}
}

func TestLineFallbackTo1(t *testing.T) {
	t.Parallel()
	result := report.Result{
		GlobalFindings: []rule.Finding{
			{RuleID: "test", Level: rule.LevelWarning, Message: "msg"},
		},
	}
	data, _ := gitlabcodequality.Render(result, gitlabcodequality.Options{})
	var issues []map[string]any
	json.Unmarshal(data, &issues)
	loc := issues[0]["location"].(map[string]any)
	lines := loc["lines"].(map[string]any)
	if lines["begin"] != float64(1) {
		t.Errorf("line = %v, want 1", lines["begin"])
	}
}

func TestSuggestionAppendedToDescription(t *testing.T) {
	t.Parallel()
	result := report.Result{
		GlobalFindings: []rule.Finding{
			{
				RuleID:  "test",
				Level:   rule.LevelWarning,
				Message: "missing comment",
				Explanation: &rule.FindingExplanation{
					Suggestion: "add COMMENT = 'description'",
				},
			},
		},
	}
	data, _ := gitlabcodequality.Render(result, gitlabcodequality.Options{})
	var issues []map[string]any
	json.Unmarshal(data, &issues)
	desc := issues[0]["description"].(string)
	if !strings.Contains(desc, "missing comment") {
		t.Error("description missing original message")
	}
	if !strings.Contains(desc, "Suggestion: add COMMENT = 'description'") {
		t.Errorf("description missing suggestion, got: %s", desc)
	}
	if strings.Contains(desc, "\n") {
		t.Error("description should not contain newlines")
	}
}

func TestUnsupportedStatementsOmitted(t *testing.T) {
	t.Parallel()
	result := report.Result{
		Statements: []report.StatementResult{
			{
				Index: 0,
				Findings: []rule.Finding{
					{RuleID: "a", Level: rule.LevelWarning, Message: "finding A"},
				},
			},
		},
		Unsupported: []spec.UnsupportedDetail{
			{Index: 1, Feature: "CREATE TRIGGER", Reason: "unsupported"},
		},
	}
	data, _ := gitlabcodequality.Render(result, gitlabcodequality.Options{})
	var issues []map[string]any
	json.Unmarshal(data, &issues)
	if len(issues) != 1 {
		t.Errorf("expected 1 issue (unsupported excluded), got %d", len(issues))
	}
	assertStringField(t, issues[0], "check_name", "a")
}

func TestFingerprintDeterministic(t *testing.T) {
	t.Parallel()
	result := report.Result{
		GlobalFindings: []rule.Finding{
			{RuleID: "test", Level: rule.LevelWarning, Message: "msg"},
		},
	}
	d1, _ := gitlabcodequality.Render(result, gitlabcodequality.Options{Path: "f.sql"})
	d2, _ := gitlabcodequality.Render(result, gitlabcodequality.Options{Path: "f.sql"})
	var i1, i2 []map[string]any
	json.Unmarshal(d1, &i1)
	json.Unmarshal(d2, &i2)
	if i1[0]["fingerprint"] != i2[0]["fingerprint"] {
		t.Error("fingerprint not deterministic")
	}
}

func TestFingerprintDiffersOnDifferentInputs(t *testing.T) {
	t.Parallel()
	base := report.Result{
		GlobalFindings: []rule.Finding{
			{RuleID: "rule.a", Level: rule.LevelWarning, Message: "msg"},
		},
	}
	d1, _ := gitlabcodequality.Render(base, gitlabcodequality.Options{Path: "a.sql"})

	changed := report.Result{
		GlobalFindings: []rule.Finding{
			{RuleID: "rule.b", Level: rule.LevelWarning, Message: "msg"},
		},
	}
	d2, _ := gitlabcodequality.Render(changed, gitlabcodequality.Options{Path: "a.sql"})

	var i1, i2 []map[string]any
	json.Unmarshal(d1, &i1)
	json.Unmarshal(d2, &i2)
	if i1[0]["fingerprint"] == i2[0]["fingerprint"] {
		t.Error("different rule IDs should produce different fingerprints")
	}
}

func TestFingerprintLength(t *testing.T) {
	t.Parallel()
	result := report.Result{
		GlobalFindings: []rule.Finding{
			{RuleID: "test", Level: rule.LevelWarning, Message: "msg"},
		},
	}
	data, _ := gitlabcodequality.Render(result, gitlabcodequality.Options{})
	var issues []map[string]any
	json.Unmarshal(data, &issues)
	fp := issues[0]["fingerprint"].(string)
	if len(fp) != 64 {
		t.Errorf("fingerprint length = %d, want 64 hex chars", len(fp))
	}
}

func TestOutputIsValidJSONArrayNoBOM(t *testing.T) {
	t.Parallel()
	result := report.Result{
		GlobalFindings: []rule.Finding{
			{RuleID: "test", Level: rule.LevelWarning, Message: "msg"},
		},
	}
	data, _ := gitlabcodequality.Render(result, gitlabcodequality.Options{})
	if len(data) > 0 && data[0] == 0xEF {
		t.Error("output starts with BOM")
	}
	if data[0] != '[' {
		t.Errorf("output does not start with [, got %c", data[0])
	}
	if data[len(data)-1] != ']' {
		t.Errorf("output does not end with ], got %c", data[len(data)-1])
	}
}

func TestMultipleFindings(t *testing.T) {
	t.Parallel()
	result := report.Result{
		Statements: []report.StatementResult{
			{
				Index: 0,
				Findings: []rule.Finding{
					{RuleID: "a", Level: rule.LevelBlocker, Message: "finding a", Location: &rule.Location{Line: 1}},
					{RuleID: "b", Level: rule.LevelWarning, Message: "finding b", Location: &rule.Location{Line: 5}},
				},
			},
		},
		GlobalFindings: []rule.Finding{
			{RuleID: "c", Level: rule.LevelNotice, Message: "global c"},
		},
	}
	data, _ := gitlabcodequality.Render(result, gitlabcodequality.Options{Path: "f.sql"})
	var issues []map[string]any
	json.Unmarshal(data, &issues)
	if len(issues) != 3 {
		t.Fatalf("expected 3 issues, got %d", len(issues))
	}

	fps := make(map[string]bool)
	for _, issue := range issues {
		fp := issue["fingerprint"].(string)
		if fps[fp] {
			t.Errorf("duplicate fingerprint: %s", fp)
		}
		fps[fp] = true
	}
}

func TestNoLocationUsesStatementIndexFallback(t *testing.T) {
	t.Parallel()
	result := report.Result{
		Statements: []report.StatementResult{
			{
				Index: 5,
				Findings: []rule.Finding{
					{RuleID: "test", Level: rule.LevelWarning, Message: "msg", StatementIndex: 5},
				},
			},
		},
	}
	data, _ := gitlabcodequality.Render(result, gitlabcodequality.Options{})
	var issues []map[string]any
	json.Unmarshal(data, &issues)
	loc := issues[0]["location"].(map[string]any)
	lines := loc["lines"].(map[string]any)
	if lines["begin"] != float64(6) {
		t.Errorf("line = %v, want 6", lines["begin"])
	}
}

func TestFindingWithLocationOverridesStatementIndex(t *testing.T) {
	t.Parallel()
	result := report.Result{
		Statements: []report.StatementResult{
			{
				Index: 5,
				Findings: []rule.Finding{
					{
						RuleID:         "test",
						Level:          rule.LevelWarning,
						Message:        "msg",
						StatementIndex: 5,
						Location:       &rule.Location{Line: 42},
					},
				},
			},
		},
	}
	data, _ := gitlabcodequality.Render(result, gitlabcodequality.Options{})
	var issues []map[string]any
	json.Unmarshal(data, &issues)
	loc := issues[0]["location"].(map[string]any)
	lines := loc["lines"].(map[string]any)
	if lines["begin"] != float64(42) {
		t.Errorf("line = %v, want 42 (from Location, not statementIndex)", lines["begin"])
	}
}

func assertStringField(t *testing.T, m map[string]any, key, expected string) {
	t.Helper()
	got, ok := m[key].(string)
	if !ok {
		t.Errorf("%s is not a string", key)
		return
	}
	if got != expected {
		t.Errorf("%s = %q, want %q", key, got, expected)
	}
}

func assertNonEmptyString(t *testing.T, m map[string]any, key string) {
	t.Helper()
	got, ok := m[key].(string)
	if !ok || got == "" {
		t.Errorf("%s is empty or not a string", key)
	}
}
