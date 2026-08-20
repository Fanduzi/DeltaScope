// Package cli verifies offline existence caveats on default audit surfaces.
// input: offline ALTER DROP COLUMN / ALTER missing-table CLI invocations
// output: pass verdict plus markdown/quiet/JSON copy that existence was not checked
// pos: CLI contract coverage for issue #28
// note: if this file changes, update this header and module README.md.
package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/report"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
)

const offlineExistenceSQLDropColumn = "alter table users drop column not_a_col"
const offlineExistenceSQLMissingTable = "alter table missing_table add column x int"

func TestOfflineDropColumnMissingStaysPassAndStatesExistenceNotChecked(t *testing.T) {
	assertOfflineExistenceCaveat(t, offlineExistenceSQLDropColumn, []string{
		"ddl.alter.drop_column.notice",
		"would drop column",
		"not_a_col",
		"if it exists",
	}, []string{
		"removes an existing column",
		"existing column",
	})
}

func TestOfflineAlterMissingTableStaysPassAndStatesExistenceNotChecked(t *testing.T) {
	assertOfflineExistenceCaveat(t, offlineExistenceSQLMissingTable, []string{
		"ddl.alter.add_column.notice",
	}, []string{
		"existing column",
		"table missing_table already exists",
	})
}

func TestQuietJSONOfflineDropColumnKeepsStatementFindingsAndContextCaveat(t *testing.T) {
	// --quiet leaves JSON unchanged: findings stay on statements[].findings.
	stdout := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", offlineExistenceSQLDropColumn, "--quiet", "--format", "json"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)
	if code != 0 {
		t.Fatalf("expected exit 0 for offline notice-only ALTER, got %d\n%s", code, stdout.String())
	}

	decoded := decodeAuditJSON(t, stdout.String())
	if decoded["verdict"] != "pass" {
		t.Fatalf("expected verdict pass, got %#v", decoded["verdict"])
	}
	if _, ok := decoded["findings"]; ok {
		t.Fatalf("JSON contract has no top-level findings array, got %#v", decoded["findings"])
	}
	assertJSONContextExistenceCaveat(t, decoded)
	if !jsonHasRuleID(decoded, "ddl.alter.drop_column.notice") {
		t.Fatalf("expected drop-column notice on statements[].findings, got %s", stdout.String())
	}
	if strings.Contains(stdout.String(), "existing column") {
		t.Fatalf("JSON notice must not claim the column exists, got %s", stdout.String())
	}
}

func TestRenderSurfacesCarryOfflineExistenceNote(t *testing.T) {
	t.Parallel()
	result := report.Result{
		Verdict: report.VerdictPass,
		Summary: report.Summary{Statements: 1, Notices: 1},
		Statements: []report.StatementResult{{
			Index: 0,
			Kind:  "ddl",
			Findings: []rule.Finding{{
				RuleID:  "ddl.alter.drop_column.notice",
				Level:   rule.LevelNotice,
				Message: `ALTER TABLE users DROP COLUMN would drop column "not_a_col" if it exists`,
			}},
		}},
	}
	runContext := &auditRunContext{
		Mode:          "offline",
		Dialect:       "mysql",
		DialectSource: "default",
		Note:          existenceNotCheckedNote,
		Unproven:      offlineExistenceUnproven(),
	}

	markdown, err := renderMarkdownResult(result, runContext)
	if err != nil {
		t.Fatalf("render markdown: %v", err)
	}
	rendered := string(markdown)
	if !strings.Contains(rendered, "## Action Summary") {
		t.Fatalf("expected Action Summary, got %s", rendered)
	}
	action := actionSummarySection(rendered)
	if !strings.Contains(action, existenceNotCheckedNote) {
		t.Fatalf("Action Summary must state existence was not checked, got %s", action)
	}
	if !strings.Contains(rendered, "- "+existenceNotCheckedNote) {
		t.Fatalf("Audit Context must state existence was not checked, got %s", rendered)
	}

	quiet := string(renderQuietResult(result, runContext))
	if !strings.Contains(quiet, "[context] mode=offline dialect=mysql dialect_source=default "+existenceNotCheckedNote) {
		t.Fatalf("quiet context line must carry the caveat, got %q", quiet)
	}

	jsonBytes, err := renderJSONResult(result, runContext)
	if err != nil {
		t.Fatalf("render json: %v", err)
	}
	assertJSONContextExistenceCaveat(t, decodeAuditJSON(t, string(jsonBytes)))
}

func TestMetadataAwareContextOmitsOfflineExistenceNote(t *testing.T) {
	t.Parallel()
	output, err := renderJSONResult(report.Result{Verdict: report.VerdictPass}, &auditRunContext{
		Mode:          "metadata-aware",
		Dialect:       "mysql",
		DialectSource: "detected",
		Schema:        "app",
		SchemaSource:  "flag",
	})
	if err != nil {
		t.Fatalf("render json: %v", err)
	}
	decoded := decodeAuditJSON(t, string(output))
	contextValue, ok := decoded["context"].(map[string]any)
	if !ok {
		t.Fatalf("expected context object, got %#v", decoded["context"])
	}
	if contextValue["note"] != nil {
		t.Fatalf("metadata-aware context must omit offline existence note, got %#v", contextValue)
	}
	if contextValue["unproven"] != nil {
		t.Fatalf("metadata-aware context must omit unproven, got %#v", contextValue)
	}
}

func assertOfflineExistenceCaveat(t *testing.T, sql string, wantContain, wantOmit []string) {
	t.Helper()

	markdownOut, markdownCode := runAudit(t, []string{"audit", "--sql", sql})
	if markdownCode != 0 {
		t.Fatalf("markdown: expected exit 0, got %d\n%s", markdownCode, markdownOut)
	}
	if !strings.Contains(markdownOut, "Verdict: `pass`") {
		t.Fatalf("markdown: expected pass verdict, got %s", markdownOut)
	}
	action := actionSummarySection(markdownOut)
	if !strings.Contains(action, existenceNotCheckedNote) {
		t.Fatalf("markdown Action Summary must state existence was not checked, got %s", action)
	}
	if !strings.Contains(markdownOut, "- "+existenceNotCheckedNote) {
		t.Fatalf("markdown Audit Context must state existence was not checked, got %s", markdownOut)
	}
	assertContainsAll(t, "markdown", markdownOut, wantContain)
	assertOmitsAll(t, "markdown", markdownOut, wantOmit)

	quietOut, quietCode := runAudit(t, []string{"audit", "--sql", sql, "--quiet"})
	if quietCode != 0 {
		t.Fatalf("quiet: expected exit 0, got %d\n%s", quietCode, quietOut)
	}
	if !strings.Contains(quietOut, "[context] mode=offline dialect=mysql dialect_source=default "+existenceNotCheckedNote) {
		t.Fatalf("quiet context line must carry the caveat, got %q", quietOut)
	}
	assertContainsAll(t, "quiet", quietOut, wantContain)
	assertOmitsAll(t, "quiet", quietOut, wantOmit)

	jsonOut, jsonCode := runAudit(t, []string{"audit", "--sql", sql, "--format", "json"})
	if jsonCode != 0 {
		t.Fatalf("json: expected exit 0, got %d\n%s", jsonCode, jsonOut)
	}
	decoded := decodeAuditJSON(t, jsonOut)
	if decoded["verdict"] != "pass" {
		t.Fatalf("json: expected verdict pass, got %#v", decoded["verdict"])
	}
	assertJSONContextExistenceCaveat(t, decoded)
	assertContainsAll(t, "json", jsonOut, wantContain)
	assertOmitsAll(t, "json", jsonOut, wantOmit)
}

func runAudit(t *testing.T, args []string) (string, int) {
	t.Helper()
	stdout := &strings.Builder{}
	code := Execute(context.Background(), args, strings.NewReader(""), stdout, &strings.Builder{})
	return stdout.String(), code
}

func decodeAuditJSON(t *testing.T, raw string) map[string]any {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("unmarshal json: %v\noutput=%s", err, raw)
	}
	return decoded
}

func assertJSONContextExistenceCaveat(t *testing.T, decoded map[string]any) {
	t.Helper()
	contextValue, ok := decoded["context"].(map[string]any)
	if !ok {
		t.Fatalf("expected context object, got %#v", decoded["context"])
	}
	if contextValue["mode"] != "offline" {
		t.Fatalf("expected offline mode, got %#v", contextValue)
	}
	if contextValue["note"] != existenceNotCheckedNote {
		t.Fatalf("expected context.note=%q, got %#v", existenceNotCheckedNote, contextValue["note"])
	}
	unproven, ok := contextValue["unproven"].([]any)
	if !ok {
		t.Fatalf("expected context.unproven array, got %#v", contextValue["unproven"])
	}
	got := make([]string, 0, len(unproven))
	for _, item := range unproven {
		text, _ := item.(string)
		got = append(got, text)
	}
	if strings.Join(got, ",") != "column_exists,table_exists" {
		t.Fatalf("expected unproven [column_exists table_exists], got %#v", unproven)
	}
}

func jsonHasRuleID(decoded map[string]any, ruleID string) bool {
	statements, _ := decoded["statements"].([]any)
	for _, raw := range statements {
		statement, _ := raw.(map[string]any)
		findings, _ := statement["findings"].([]any)
		for _, findingRaw := range findings {
			finding, _ := findingRaw.(map[string]any)
			if finding["rule_id"] == ruleID {
				return true
			}
		}
	}
	return false
}

func actionSummarySection(output string) string {
	const heading = "## Action Summary"
	start := strings.Index(output, heading)
	if start < 0 {
		return ""
	}
	rest := output[start+len(heading):]
	if end := strings.Index(rest, "\n## "); end >= 0 {
		return heading + rest[:end]
	}
	return heading + rest
}

func assertContainsAll(t *testing.T, surface, output string, needles []string) {
	t.Helper()
	for _, needle := range needles {
		if !strings.Contains(output, needle) {
			t.Fatalf("%s output must contain %q, got %s", surface, needle, output)
		}
	}
}

func assertOmitsAll(t *testing.T, surface, output string, needles []string) {
	t.Helper()
	for _, needle := range needles {
		if strings.Contains(output, needle) {
			t.Fatalf("%s output must not contain %q, got %s", surface, needle, output)
		}
	}
}
