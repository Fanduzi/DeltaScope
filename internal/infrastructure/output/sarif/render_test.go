// Package sarif verifies SARIF 2.1.0 output generation.
// input: synthetic report results with findings at various severity levels
// output: test coverage for SARIF version, rule metadata, result entries, and severity mapping
// pos: infrastructure output adapter test coverage for CI-native SARIF format
// note: if this file changes, update this header and module README.md.
package sarif

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
)

func TestRenderIncludesVersion(t *testing.T) {
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

	var doc map[string]any
	if err := json.Unmarshal(output, &doc); err != nil {
		t.Fatalf("unmarshal sarif: %v\noutput=%s", err, string(output))
	}
	if doc["version"] != "2.1.0" {
		t.Fatalf("expected version 2.1.0, got %v", doc["version"])
	}
	if doc["$schema"] == "" {
		t.Fatal("expected $schema to be present")
	}
}

func TestRenderIncludesRuleMetadata(t *testing.T) {
	t.Parallel()
	result := report.Result{
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Findings: []rule.Finding{{
				RuleID:  "dml.where.require",
				Level:   rule.LevelBlocker,
				Message: "where clause is required",
			}, {
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

	var doc map[string]any
	if err := json.Unmarshal(output, &doc); err != nil {
		t.Fatalf("unmarshal sarif: %v", err)
	}

	runs, ok := doc["runs"].([]any)
	if !ok || len(runs) == 0 {
		t.Fatalf("expected runs array, got %v", doc["runs"])
	}
	driverRaw := nested(driverPath, doc)
	if driverRaw == nil {
		t.Fatal("expected tool.driver object")
	}
	driver, ok := driverRaw.(map[string]any)
	if !ok {
		t.Fatalf("expected driver to be a map, got %T", driverRaw)
	}
	rules, ok := driver["rules"].([]any)
	if !ok || len(rules) < 2 {
		t.Fatalf("expected at least 2 rules in driver, got %v", rules)
	}
}

func TestRenderIncludesResults(t *testing.T) {
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

	var doc map[string]any
	if err := json.Unmarshal(output, &doc); err != nil {
		t.Fatalf("unmarshal sarif: %v", err)
	}

	results := nested(resultsPath, doc)
	if results == nil {
		t.Fatal("expected results array in SARIF")
	}
	resultsArr, ok := results.([]any)
	if !ok || len(resultsArr) != 1 {
		t.Fatalf("expected 1 result, got %v", results)
	}
	entry, _ := resultsArr[0].(map[string]any)
	if entry["ruleId"] != "dml.where.require" {
		t.Fatalf("expected ruleId dml.where.require, got %v", entry["ruleId"])
	}
}

func TestRenderSeverityMapping(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		level        rule.Level
		wantSeverity string
	}{
		{"blocker maps to error", rule.LevelBlocker, "error"},
		{"warning maps to warning", rule.LevelWarning, "warning"},
		{"notice maps to note", rule.LevelNotice, "note"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := report.Result{
				Statements: []report.StatementResult{{
					Index: 0,
					Kind:  "dml",
					Findings: []rule.Finding{{
						RuleID:  "test.rule",
						Level:   tc.level,
						Message: "test message",
					}},
				}},
			}

			output, err := Render(result, Options{})
			if err != nil {
				t.Fatalf("render: %v", err)
			}

			var doc map[string]any
			if err := json.Unmarshal(output, &doc); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			results := nested(resultsPath, doc).([]any)
			entry, _ := results[0].(map[string]any)
			if entry["level"] != tc.wantSeverity {
				t.Fatalf("expected level %s, got %v", tc.wantSeverity, entry["level"])
			}
		})
	}
}

func TestRenderIncludesLocationWhenPresent(t *testing.T) {
	t.Parallel()
	result := report.Result{
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Findings: []rule.Finding{{
				RuleID:   "dml.where.require",
				Level:    rule.LevelBlocker,
				Message:  "where clause is required",
				Location: &rule.Location{Line: 5, Column: 10},
			}},
		}},
	}

	output, err := Render(result, Options{})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	rendered := string(output)
	if !strings.Contains(rendered, `"startLine":5`) {
		t.Fatalf("expected startLine 5 in SARIF location, got %s", rendered)
	}
	if !strings.Contains(rendered, `"startColumn":10`) {
		t.Fatalf("expected startColumn 10 in SARIF location, got %s", rendered)
	}
}

func TestRenderIncludesArtifactLocationWhenPathProvided(t *testing.T) {
	t.Parallel()
	result := report.Result{
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Findings: []rule.Finding{{
				RuleID:   "dml.where.require",
				Level:    rule.LevelBlocker,
				Message:  "where clause is required",
				Location: &rule.Location{Line: 9, Column: 1},
			}},
		}},
	}

	output, err := Render(result, Options{Path: "migrations/001.sql"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	rendered := string(output)
	if !strings.Contains(rendered, `"uri":"migrations/001.sql"`) {
		t.Fatalf("expected artifactLocation.uri in SARIF, got %s", rendered)
	}
	if !strings.Contains(rendered, `"startLine":9`) {
		t.Fatalf("expected startLine 9 in SARIF location, got %s", rendered)
	}
}

func TestRenderOmitsArtifactLocationWhenPathEmpty(t *testing.T) {
	t.Parallel()
	result := report.Result{
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Findings: []rule.Finding{{
				RuleID:   "dml.where.require",
				Level:    rule.LevelBlocker,
				Message:  "where clause is required",
				Location: &rule.Location{Line: 3, Column: 1},
			}},
		}},
	}

	output, err := Render(result, Options{})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	rendered := string(output)
	if strings.Contains(rendered, `"artifactLocation"`) {
		t.Fatalf("expected no artifactLocation when path is empty, got %s", rendered)
	}
	if !strings.Contains(rendered, `"startLine":3`) {
		t.Fatalf("expected startLine 3 in SARIF location, got %s", rendered)
	}
}

func TestRenderEmptyResultProducesValidSARIF(t *testing.T) {
	t.Parallel()
	result := report.Result{Verdict: report.VerdictPass}

	output, err := Render(result, Options{})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(output, &doc); err != nil {
		t.Fatalf("unmarshal empty sarif: %v", err)
	}
	if doc["version"] != "2.1.0" {
		t.Fatalf("expected version 2.1.0, got %v", doc["version"])
	}
	results := nested(resultsPath, doc)
	arr, ok := results.([]any)
	if !ok || len(arr) != 0 {
		t.Fatalf("expected empty results for clean audit, got %v", results)
	}
}

func TestRenderIncludesHelpFromExplanation(t *testing.T) {
	t.Parallel()
	result := report.Result{
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "dml",
			Findings: []rule.Finding{{
				RuleID:  "dml.where.require",
				Level:   rule.LevelBlocker,
				Message: "where clause is required",
				Explanation: &rule.FindingExplanation{
					Why:        "Policy requires a predicate.",
					Risk:       "Full table scan.",
					Suggestion: "Add a WHERE clause.",
				},
			}},
		}},
	}

	output, err := Render(result, Options{})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(output, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// help must live on the rule in tool.driver.rules[], not on the result
	driverRaw := nested(driverPath, doc)
	driver, ok := driverRaw.(map[string]any)
	if !ok {
		t.Fatalf("expected driver map, got %T", driverRaw)
	}
	rules, ok := driver["rules"].([]any)
	if !ok || len(rules) == 0 {
		t.Fatalf("expected at least 1 rule, got %v", rules)
	}
	firstRule, _ := rules[0].(map[string]any)
	help, _ := firstRule["help"].(map[string]any)
	if help == nil {
		t.Fatal("expected rule.help object with suggestion text")
	}
	if help["text"] != "Add a WHERE clause." {
		t.Fatalf("expected help.text 'Add a WHERE clause.', got %v", help["text"])
	}

	// result.message should still include the full explanation text
	results := nested(resultsPath, doc).([]any)
	entry, _ := results[0].(map[string]any)
	msg, _ := entry["message"].(map[string]any)
	if !strings.Contains(msg["text"].(string), "Add a WHERE clause.") {
		t.Fatalf("expected suggestion in result message text, got %v", msg["text"])
	}
}

// nested extracts a value from nested map keys using dot-separated path.
// Numeric indices traverse arrays.
var (
	driverPath  = "runs.0.tool.driver"
	resultsPath = "runs.0.results"
)

func nested(path string, doc map[string]any) any {
	parts := strings.Split(path, ".")
	var current any = doc
	for _, p := range parts {
		switch c := current.(type) {
		case map[string]any:
			current = c[p]
		case []any:
			idx := 0
			for _, ch := range p {
				if ch < '0' || ch > '9' {
					return nil
				}
				idx = idx*10 + int(ch-'0')
			}
			if idx >= len(c) {
				return nil
			}
			current = c[idx]
		default:
			return nil
		}
	}
	return current
}
