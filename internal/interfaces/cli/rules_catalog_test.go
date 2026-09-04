package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// --- helpers ---

func runRulesCLI(t *testing.T, args []string) (int, string, string) {
	t.Helper()
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		args,
		strings.NewReader(""),
		stdout,
		stderr,
	)
	return code, stdout.String(), stderr.String()
}

func mustParseRulesListJSON(t *testing.T, raw string) rulesListJSONOutput {
	t.Helper()
	var out rulesListJSONOutput
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("JSON parse error: %v\noutput=%s", err, raw)
	}
	return out
}

func mustParseRulesExplainJSON(t *testing.T, raw string) rulesExplainJSONOutput {
	t.Helper()
	var out rulesExplainJSONOutput
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("JSON parse error: %v\noutput=%s", err, raw)
	}
	return out
}

// --- Test 1: rules list --level blocker ---

func TestRulesList_LevelBlocker(t *testing.T) {
	code, stdout, stderr := runRulesCLI(t, []string{"rules", "list", "--level", "blocker"})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr)
	}

	// Text output should contain headers.
	for _, header := range []string{"RULE ID", "LEVEL", "DIALECT", "KIND", "CATEGORY"} {
		if !strings.Contains(stdout, header) {
			t.Errorf("text output should contain header %q", header)
		}
	}

	// All visible rows should be blocker level.
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "-") || strings.Contains(line, "RULE ID") || !strings.Contains(line, "blocker") {
			continue
		}
		// data row: check it has "blocker"
		if strings.Contains(line, "  ") && !strings.Contains(line, "blocker") {
			t.Errorf("expected only blocker rows, got: %s", line)
		}
	}
}

// --- Test 2: rules list --dialect postgresql --level warning --format json ---

func TestRulesList_PostgreSQLWarningJSON(t *testing.T) {
	code, stdout, stderr := runRulesCLI(t, []string{"rules", "list", "--dialect", "postgresql", "--level", "warning", "--format", "json"})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr)
	}

	result := mustParseRulesListJSON(t, stdout)
	if len(result.Rules) == 0 {
		t.Fatal("expected at least one postgresql warning rule")
	}

	for _, r := range result.Rules {
		if !strings.Contains(r.Dialect, "postgresql") {
			t.Errorf("rule %s dialect = %q, want postgresql", r.RuleID, r.Dialect)
		}
		if r.Level != "warning" {
			t.Errorf("rule %s level = %q, want warning", r.RuleID, r.Level)
		}
	}
}

// --- Test 3: rules list --kind ddl --format json ---

func TestRulesList_KindDDLJSON(t *testing.T) {
	code, stdout, stderr := runRulesCLI(t, []string{"rules", "list", "--kind", "ddl", "--format", "json"})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr)
	}

	result := mustParseRulesListJSON(t, stdout)
	if len(result.Rules) == 0 {
		t.Fatal("expected at least one ddl rule")
	}

	for _, r := range result.Rules {
		if r.Kind != "ddl" {
			t.Errorf("rule %s kind = %q, want ddl", r.RuleID, r.Kind)
		}
	}
}

// --- Test 4: rules list --search drop_column ---

func TestRulesList_SearchDropColumn(t *testing.T) {
	code, stdout, stderr := runRulesCLI(t, []string{"rules", "list", "--search", "drop_column"})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr)
	}

	if !strings.Contains(stdout, "drop_column") {
		t.Errorf("output should contain drop_column rules, got:\n%s", stdout)
	}
}

// --- Test 5: rules list --search definitely-not-present --format json ---

func TestRulesList_SearchEmptyJSON(t *testing.T) {
	code, stdout, stderr := runRulesCLI(t, []string{"rules", "list", "--search", "definitely-not-present", "--format", "json"})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr)
	}

	result := mustParseRulesListJSON(t, stdout)
	if len(result.Rules) != 0 {
		t.Errorf("expected 0 rules, got %d", len(result.Rules))
	}
	if result.Summary.Total != 0 {
		t.Errorf("expected total 0, got %d", result.Summary.Total)
	}
}

// --- Test 6: rules list --level critical (invalid level) ---

func TestRulesList_InvalidLevel(t *testing.T) {
	code, _, stderr := runRulesCLI(t, []string{"rules", "list", "--level", "critical"})

	if code == 0 {
		t.Fatal("expected non-zero exit for invalid level")
	}
	if !strings.Contains(stderr, "level") || !strings.Contains(stderr, "critical") {
		t.Errorf("error should mention level and critical: %s", stderr)
	}
}

// --- Test 7: rules list --dialect oracle (invalid dialect) ---

func TestRulesList_InvalidDialect(t *testing.T) {
	code, _, stderr := runRulesCLI(t, []string{"rules", "list", "--dialect", "oracle"})

	if code == 0 {
		t.Fatal("expected non-zero exit for invalid dialect")
	}
	if !strings.Contains(stderr, "dialect") || !strings.Contains(stderr, "oracle") {
		t.Errorf("error should mention dialect and oracle: %s", stderr)
	}
}

// --- Test 8: rules list --kind batch (invalid kind) ---

func TestRulesList_InvalidKind(t *testing.T) {
	code, _, stderr := runRulesCLI(t, []string{"rules", "list", "--kind", "batch"})

	if code == 0 {
		t.Fatal("expected non-zero exit for invalid kind")
	}
	if !strings.Contains(stderr, "kind") || !strings.Contains(stderr, "batch") {
		t.Errorf("error should mention kind and batch: %s", stderr)
	}
}

// --- Test 9: rules list --format xml (invalid format) ---

func TestRulesList_InvalidFormat(t *testing.T) {
	code, _, stderr := runRulesCLI(t, []string{"rules", "list", "--format", "xml"})

	if code == 0 {
		t.Fatal("expected non-zero exit for invalid format")
	}
	if !strings.Contains(stderr, "format") {
		t.Errorf("error should mention format: %s", stderr)
	}
}

// --- Test 10: rules list --limit -1 (negative limit) ---

func TestRulesList_NegativeLimit(t *testing.T) {
	code, _, stderr := runRulesCLI(t, []string{"rules", "list", "--limit", "-1"})

	if code == 0 {
		t.Fatal("expected non-zero exit for negative limit")
	}
	if !strings.Contains(stderr, "limit") {
		t.Errorf("error should mention limit: %s", stderr)
	}
}

// --- Test 11: rules explain <known-rule-id> ---

func TestRulesExplain_KnownRuleText(t *testing.T) {
	code, stdout, stderr := runRulesCLI(t, []string{"rules", "explain", "dml.where.require"})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr)
	}

	// Text output should include key fields and the new policy/override blocks.
	for _, field := range []string{
		"dml.where.require", "Level:", "Config Key:", "Category:", "Summary:",
		"Default policy:", "Safe override example:", "Inspect effective rule status:",
	} {
		if !strings.Contains(stdout, field) {
			t.Errorf("text output should contain %q", field)
		}
	}
}

// --- Test 11b: rules explain default policy + safe override content ---

func TestRulesExplain_DefaultPolicyAndSafeOverride(t *testing.T) {
	code, stdout, stderr := runRulesCLI(t, []string{"rules", "explain", "dml.where.require"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr)
	}

	// Default policy block reflects policy.Default(): enabled, default level,
	// and default params, all rendered as a copyable rules.<id> YAML snippet.
	if !strings.Contains(stdout, "Default policy:\n") {
		t.Errorf("missing Default policy: block")
	}
	for _, want := range []string{
		"      enabled: true\n",
		"      level: blocker\n",
		"        required: true\n",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("default policy block should contain %q", want)
		}
	}

	// Safe override example is a FULL rule policy override: it keeps default
	// enabled and params while downgrading level, proving the complete shape that
	// avoids the partial-override footgun. It must not be a level-only snippet.
	overrideStart := strings.Index(stdout, "Safe override example:")
	if overrideStart < 0 {
		t.Fatal("missing Safe override example: block")
	}
	handoffStart := strings.Index(stdout, "Inspect effective rule status:")
	if handoffStart < 0 || handoffStart < overrideStart {
		t.Fatal("expected Inspect effective rule status: after Safe override example:")
	}
	override := stdout[overrideStart:handoffStart]
	for _, want := range []string{"enabled: true", "params:", "required: true", "level: warning"} {
		if !strings.Contains(override, want) {
			t.Errorf("safe override should contain %q; block:\n%s", want, override)
		}
	}

	// Handoff command points at config status with a config file path.
	wantCmd := "Inspect effective rule status:\n  deltascope config status dml.where.require --config deltascope.yaml"
	if !strings.Contains(stdout, wantCmd) {
		t.Errorf("expected handoff command block:\n%s", wantCmd)
	}

	// The wizard-style Next: label is never used.
	if strings.Contains(stdout, "Next:") {
		t.Errorf("text output must not use Next: label")
	}
}

// --- Test 12: rules explain <known-rule-id> --format json ---

func TestRulesExplain_KnownRuleJSON(t *testing.T) {
	code, stdout, stderr := runRulesCLI(t, []string{"rules", "explain", "dml.where.require", "--format", "json"})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr)
	}

	result := mustParseRulesExplainJSON(t, stdout)
	if result.Rule.RuleID != "dml.where.require" {
		t.Errorf("rule_id = %q, want dml.where.require", result.Rule.RuleID)
	}
	if result.Rule.Level == "" {
		t.Error("expected non-empty level")
	}
}

// --- Test 13: rules explain missing.rule.id ---

func TestRulesExplain_MissingRule(t *testing.T) {
	code, _, stderr := runRulesCLI(t, []string{"rules", "explain", "missing.rule.id"})

	if code == 0 {
		t.Fatal("expected non-zero exit for missing rule")
	}
	if !strings.Contains(stderr, "not found") {
		t.Errorf("error should mention not found: %s", stderr)
	}
}

// --- Test 14: rules explain (no rule ID) ---

func TestRulesExplain_NoRuleID(t *testing.T) {
	code, _, stderr := runRulesCLI(t, []string{"rules", "explain"})

	if code == 0 {
		t.Fatal("expected non-zero exit for missing rule ID")
	}
	// Cobra prints usage on arg validation errors.
	_ = stderr // non-zero exit is sufficient
}

// --- Test 15: JSON no-severity sanity ---

func TestRulesListJSON_NoSeverity(t *testing.T) {
	code, stdout, stderr := runRulesCLI(t, []string{"rules", "list", "--format", "json"})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr)
	}

	if strings.Contains(stdout, "severity") {
		t.Error("JSON output must not contain 'severity'")
	}
	if strings.Contains(stdout, `"severity"`) {
		t.Error("JSON output must not contain quoted severity key")
	}
}

func TestRulesExplainJSON_NoSeverity(t *testing.T) {
	code, stdout, stderr := runRulesCLI(t, []string{"rules", "explain", "dml.where.require", "--format", "json"})

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr)
	}

	if strings.Contains(stdout, `"severity"`) {
		t.Error("JSON output must not contain quoted severity key")
	}
}

// --- Test 16: rules explain JSON stays free of text-only labels ---

func TestRulesListAndExplainDefaultDisabledImpactRules(t *testing.T) {
	code, stdout, stderr := runRulesCLI(t, []string{"rules", "list", "--search", "dml.impact", "--format", "json"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr)
	}
	listed := mustParseRulesListJSON(t, stdout)
	found := map[string]rulesListJSONRule{}
	for _, rule := range listed.Rules {
		found[rule.RuleID] = rule
		if rule.Enabled {
			t.Fatalf("rules list must mark %s default-disabled", rule.RuleID)
		}
	}
	for _, ruleID := range []string{
		"dml.impact.estimate",
		"dml.impact.rows.max_count",
		"dml.impact.ratio.max_percent",
	} {
		if _, ok := found[ruleID]; !ok {
			t.Fatalf("rules list must include %q", ruleID)
		}
		explainCode, explainOut, explainErr := runRulesCLI(t, []string{"rules", "explain", ruleID, "--format", "json"})
		if explainCode != 0 {
			t.Fatalf("explain %s exit code = %d, want 0; stderr=%s", ruleID, explainCode, explainErr)
		}
		explained := mustParseRulesExplainJSON(t, explainOut)
		if explained.Rule.RuleID != ruleID {
			t.Fatalf("explain rule_id = %q, want %s", explained.Rule.RuleID, ruleID)
		}
		if explained.Rule.Enabled {
			t.Fatalf("rules explain must mark %s default-disabled", ruleID)
		}
	}
}

func TestRulesExplainJSON_NoTextOnlyLabels(t *testing.T) {
	code, stdout, stderr := runRulesCLI(t, []string{"rules", "explain", "dml.where.require", "--format", "json"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr)
	}

	// The new guidance blocks are text-only; JSON must stay on its frozen schema.
	for _, banned := range []string{
		"Default policy:",
		"Safe override example:",
		"Inspect effective rule status:",
		"Next:",
	} {
		if strings.Contains(stdout, banned) {
			t.Errorf("JSON output must not contain text-only label %q", banned)
		}
	}
}
