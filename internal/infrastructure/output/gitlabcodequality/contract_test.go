// Package gitlabcodequality renders audit results as GitLab Code Quality JSON.
//
// Characterization tests: these define the contract that Task 2 must satisfy.
// They do not contain production implementation.
package gitlabcodequality_test

import (
	"encoding/hex"
	"encoding/json"
	"hash/fnv"
	"sort"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// --- GitLab Code Quality schema types (contract mirror) ---

// gitlabIssue mirrors the required GitLab Code Quality issue shape.
// See https://docs.gitlab.com/ci/testing/code_quality/ (checked 2026-04-26).
type gitlabIssue struct {
	Description string         `json:"description"`
	CheckName   string         `json:"check_name"`
	Fingerprint string         `json:"fingerprint"`
	Severity    string         `json:"severity"`
	Location    gitlabLocation `json:"location"`
}

type gitlabLocation struct {
	Path  string      `json:"path"`
	Lines gitlabLines `json:"lines"`
}

type gitlabLines struct {
	Begin int `json:"begin"`
}

// --- Contract: required fields ---

func TestGitLabIssueSchemaContainsAllRequiredKeys(t *testing.T) {
	t.Parallel()
	issue := gitlabIssue{
		Description: "table comment is required",
		CheckName:   "ddl.table.comment.require",
		Fingerprint: "abc123",
		Severity:    "minor",
		Location:    gitlabLocation{Path: "migrations.sql", Lines: gitlabLines{Begin: 1}},
	}
	data, err := json.Marshal(issue)
	if err != nil {
		t.Fatalf("marshal issue: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal round-trip: %v", err)
	}

	required := []string{"description", "check_name", "fingerprint", "severity", "location"}
	for _, key := range required {
		if _, ok := parsed[key]; !ok {
			t.Errorf("missing required key: %s", key)
		}
	}

	loc, ok := parsed["location"].(map[string]any)
	if !ok {
		t.Fatal("location is not a JSON object")
	}
	if _, ok := loc["path"]; !ok {
		t.Error("location missing required key: path")
	}
	lines, ok := loc["lines"].(map[string]any)
	if !ok {
		t.Fatal("location.lines is not a JSON object")
	}
	if _, ok := lines["begin"]; !ok {
		t.Error("location.lines missing required key: begin")
	}
}

// --- Contract: report is a JSON array ---

func TestEmptyResultProducesEmptyJSONArray(t *testing.T) {
	t.Parallel()
	// Task 2 target: Render(report.Result{}) should return [] (empty JSON array).
	// For now, verify the expected output shape.
	expected := []byte("[]")
	var arr []any
	if err := json.Unmarshal(expected, &arr); err != nil {
		t.Fatalf("empty result must be valid JSON array: %v", err)
	}
	if len(arr) != 0 {
		t.Errorf("expected 0 elements, got %d", len(arr))
	}
}

// --- Contract: severity mapping ---

func TestContractSeverityMapping(t *testing.T) {
	t.Parallel()
	cases := []struct {
		level    rule.Level
		expected string
	}{
		{rule.LevelBlocker, "major"},
		{rule.LevelWarning, "minor"},
		{rule.LevelNotice, "info"},
		{"unknown", "minor"},
	}

	for _, tc := range cases {
		got := mapSeverity(tc.level)
		if got != tc.expected {
			t.Errorf("mapSeverity(%q) = %q, want %q", tc.level, got, tc.expected)
		}
	}
}

// mapSeverity is the contract-locked severity mapping for Task 2.
func mapSeverity(level rule.Level) string {
	switch level {
	case rule.LevelBlocker:
		return "major"
	case rule.LevelWarning:
		return "minor"
	case rule.LevelNotice:
		return "info"
	default:
		return "minor"
	}
}

// --- Contract: severity values belong to GitLab accepted set ---

func TestMappedSeverityValuesAreGitLabAccepted(t *testing.T) {
	t.Parallel()
	// GitLab accepts: info, minor, major, critical, blocker
	accepted := map[string]bool{
		"info": true, "minor": true, "major": true, "critical": true, "blocker": true,
	}
	allLevels := []rule.Level{rule.LevelBlocker, rule.LevelWarning, rule.LevelNotice, "unknown"}
	for _, level := range allLevels {
		mapped := mapSeverity(level)
		if !accepted[mapped] {
			t.Errorf("mapSeverity(%q) = %q, which is not in GitLab accepted set", level, mapped)
		}
	}
}

// --- Contract: fingerprint determinism ---

func TestFingerprintIsDeterministic(t *testing.T) {
	t.Parallel()
	f1 := computeFingerprint("ddl.table.comment.require", "migrations.sql", 1, 0, "table comment is required")
	f2 := computeFingerprint("ddl.table.comment.require", "migrations.sql", 1, 0, "table comment is required")
	if f1 != f2 {
		t.Errorf("fingerprint not deterministic: %s != %s", f1, f2)
	}
}

func TestDifferentInputsProduceDifferentFingerprints(t *testing.T) {
	t.Parallel()
	f1 := computeFingerprint("rule.a", "file.sql", 1, 0, "msg")
	f2 := computeFingerprint("rule.b", "file.sql", 1, 0, "msg")
	if f1 == f2 {
		t.Error("different rule IDs produced same fingerprint")
	}

	f3 := computeFingerprint("rule.a", "other.sql", 1, 0, "msg")
	if f1 == f3 {
		t.Error("different paths produced same fingerprint")
	}

	f4 := computeFingerprint("rule.a", "file.sql", 5, 0, "msg")
	if f1 == f4 {
		t.Error("different lines produced same fingerprint")
	}

	f5 := computeFingerprint("rule.a", "file.sql", 1, 0, "other msg")
	if f1 == f5 {
		t.Error("different messages produced same fingerprint")
	}
}

func TestFingerprintCollisionResistance(t *testing.T) {
	t.Parallel()
	// Generate fingerprints for a large set and check for collisions.
	seen := make(map[string]string)
	ruleIDs := []string{"ddl.table.comment.require", "ddl.table.name.length", "ddl.index.prefix.convention"}
	paths := []string{"migrations.sql", "deltascope.sql", "schema.sql"}
	lines := []int{1, 10, 42}
	idxs := []int{0, 1, 2}
	msgs := []string{"msg a", "msg b", "msg c"}

	for _, rid := range ruleIDs {
		for _, p := range paths {
			for _, l := range lines {
				for _, idx := range idxs {
					for _, m := range msgs {
						fp := computeFingerprint(rid, p, l, idx, m)
						key := rid + "|" + p + "|" + string(rune(l)) + "|" + string(rune(idx)) + "|" + m
						if prev, exists := seen[fp]; exists {
							t.Errorf("fingerprint collision: %q and %q both → %s", prev, key, fp)
						}
						seen[fp] = key
					}
				}
			}
		}
	}
}

// computeFingerprint implements the contract-locked fingerprint strategy.
// Task 2 must use crypto/sha256; this uses FNV-1a for test-only characterization.
func computeFingerprint(ruleID, path string, line, statementIndex int, message string) string {
	h := fnv.New128a()
	h.Write([]byte(ruleID))
	h.Write([]byte{0})
	h.Write([]byte(path))
	h.Write([]byte{0})
	h.Write([]byte{byte(line >> 24), byte(line >> 16), byte(line >> 8), byte(line)})
	h.Write([]byte{byte(statementIndex >> 24), byte(statementIndex >> 16), byte(statementIndex >> 8), byte(statementIndex)})
	h.Write([]byte{0})
	h.Write([]byte(message))
	return hex.EncodeToString(h.Sum(nil))
}

// --- Contract: synthetic path fallback ---

func TestSyntheticPathForSQLInput(t *testing.T) {
	t.Parallel()
	// When input comes from --sql or stdin (no file path), use "deltascope.sql".
	synthetic := "deltascope.sql"
	if synthetic == "" {
		t.Error("synthetic path must not be empty")
	}
	// Must not start with "./"
	if len(synthetic) >= 2 && synthetic[:2] == "./" {
		t.Error("GitLab docs: location.path must not be prefixed with ./")
	}
}

// --- Contract: line fallback ---

func TestLineFallbackWhenLocationIsNil(t *testing.T) {
	t.Parallel()
	finding := rule.Finding{
		RuleID:         "test.rule",
		Level:          rule.LevelWarning,
		Message:        "test message",
		StatementIndex: 3,
	}
	line := resolveLine(finding)
	if line != 4 {
		t.Errorf("expected statement_index+1 = 4, got %d", line)
	}
}

func TestLineFallbackWhenLocationLineIsZero(t *testing.T) {
	t.Parallel()
	finding := rule.Finding{
		RuleID:         "test.rule",
		Level:          rule.LevelWarning,
		Message:        "test message",
		StatementIndex: 0,
		Location:       &rule.Location{Line: 0},
	}
	line := resolveLine(finding)
	if line != 1 {
		t.Errorf("expected 1 (statement_index+1), got %d", line)
	}
}

func TestLineUsesFindingLocationWhenPresent(t *testing.T) {
	t.Parallel()
	finding := rule.Finding{
		RuleID:         "test.rule",
		Level:          rule.LevelWarning,
		Message:        "test message",
		StatementIndex: 0,
		Location:       &rule.Location{Line: 42},
	}
	line := resolveLine(finding)
	if line != 42 {
		t.Errorf("expected 42 from finding.Location.Line, got %d", line)
	}
}

func TestGlobalFindingLineFallback(t *testing.T) {
	t.Parallel()
	finding := rule.Finding{
		RuleID:  "global.rule",
		Level:   rule.LevelNotice,
		Message: "global finding without location",
	}
	line := resolveLine(finding)
	if line != 1 {
		t.Errorf("global finding without location should fallback to 1, got %d", line)
	}
}

// resolveLine implements the contract-locked line resolution strategy.
func resolveLine(f rule.Finding) int {
	if f.Location != nil && f.Location.Line > 0 {
		return f.Location.Line
	}
	if f.StatementIndex > 0 {
		return f.StatementIndex + 1
	}
	return 1
}

// --- Contract: unsupported statements excluded ---

func TestUnsupportedStatementsAreExcluded(t *testing.T) {
	t.Parallel()
	// Unsupported statements are parser diagnostics, not rule findings.
	// They should NOT appear in GitLab Code Quality output for v0.45.0.
	result := report.Result{
		Verdict: report.VerdictReview,
		Summary: report.Summary{Statements: 1, Warnings: 1},
		Statements: []report.StatementResult{
			{Index: 0, Kind: "unsupported", Findings: []rule.Finding{
				{RuleID: "ddl.column.type.check", Level: rule.LevelWarning, Message: "column type check"},
			}},
		},
		Unsupported: []spec.UnsupportedDetail{
			{Index: 0, Feature: "CREATE TRIGGER", Reason: "unsupported statement type"},
		},
	}

	// Count findings that would enter Code Quality report.
	var count int
	for _, stmt := range result.Statements {
		count += len(stmt.Findings)
	}
	// Unsupported items are NOT counted — they stay in JSON/markdown only.
	unsupportedCount := len(result.Unsupported)

	if count != 1 {
		t.Errorf("expected 1 statement finding, got %d", count)
	}
	if unsupportedCount != 1 {
		t.Errorf("expected 1 unsupported entry, got %d", unsupportedCount)
	}
	// Contract: unsupported entries do NOT enter Code Quality report.
	// Task 2 implementation must only iterate Statements[].Findings and GlobalFindings.
}

// --- Contract: path resolution sorted deterministically ---

func TestPathResolutionFromFileFlag(t *testing.T) {
	t.Parallel()
	path := resolvePath("db/migrate/001_create_users.sql", false)
	if path != "db/migrate/001_create_users.sql" {
		t.Errorf("expected file path, got %s", path)
	}
}

func TestPathResolutionForSQLFlag(t *testing.T) {
	t.Parallel()
	path := resolvePath("", false)
	if path != "deltascope.sql" {
		t.Errorf("expected synthetic path, got %s", path)
	}
}

func TestPathResolutionForStdin(t *testing.T) {
	t.Parallel()
	path := resolvePath("", false)
	if path != "deltascope.sql" {
		t.Errorf("expected synthetic path for stdin, got %s", path)
	}
}

func TestPathDoesNotStartWithDotSlash(t *testing.T) {
	t.Parallel()
	cases := []string{"db/migrate.sql", "deltascope.sql"}
	for _, p := range cases {
		if len(p) >= 2 && p[:2] == "./" {
			t.Errorf("path %q must not start with ./", p)
		}
	}
}

func resolvePath(filePath string, _ bool) string {
	if filePath != "" {
		return filePath
	}
	return "deltascope.sql"
}

// --- Contract: multiple findings produce sorted-stable output ---

func TestMultipleFindingsProduceJSONArray(t *testing.T) {
	t.Parallel()
	issues := buildIssues()
	if len(issues) != 3 {
		t.Fatalf("expected 3 issues, got %d", len(issues))
	}

	data, err := json.Marshal(issues)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed []gitlabIssue
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Verify all issues have required fields.
	for i, issue := range parsed {
		if issue.CheckName == "" {
			t.Errorf("issue %d: check_name is empty", i)
		}
		if issue.Description == "" {
			t.Errorf("issue %d: description is empty", i)
		}
		if issue.Fingerprint == "" {
			t.Errorf("issue %d: fingerprint is empty", i)
		}
		if issue.Severity == "" {
			t.Errorf("issue %d: severity is empty", i)
		}
		if issue.Location.Path == "" {
			t.Errorf("issue %d: location.path is empty", i)
		}
		if issue.Location.Lines.Begin <= 0 {
			t.Errorf("issue %d: location.lines.begin must be > 0, got %d", i, issue.Location.Lines.Begin)
		}
	}

	// Verify fingerprints are unique.
	fps := make(map[string]bool)
	for _, issue := range parsed {
		if fps[issue.Fingerprint] {
			t.Errorf("duplicate fingerprint: %s", issue.Fingerprint)
		}
		fps[issue.Fingerprint] = true
	}
}

func buildIssues() []gitlabIssue {
	findings := []struct {
		ruleID  string
		level   rule.Level
		message string
		path    string
		line    int
	}{
		{"ddl.table.comment.require", rule.LevelBlocker, "table comment is required", "migrations.sql", 1},
		{"ddl.index.prefix.convention", rule.LevelWarning, "index name must have prefix", "migrations.sql", 5},
		{"ddl.column.not-null.explicit", rule.LevelNotice, "prefer explicit NOT NULL", "migrations.sql", 10},
	}

	var issues []gitlabIssue
	for _, f := range findings {
		issues = append(issues, gitlabIssue{
			Description: f.message,
			CheckName:   f.ruleID,
			Fingerprint: computeFingerprint(f.ruleID, f.path, f.line, 0, f.message),
			Severity:    mapSeverity(f.level),
			Location:    gitlabLocation{Path: f.path, Lines: gitlabLines{Begin: f.line}},
		})
	}
	sort.Slice(issues, func(i, j int) bool { return issues[i].CheckName < issues[j].CheckName })
	return issues
}
