//go:build postgresql

package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditCommandPostgreSQLGitLabCodeQualityRendersGlobalFinding(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "ALTER TABLE orders ADD CONSTRAINT chk_orders_amount CHECK (amount >= 0) NOT VALID;", "--dialect", "postgresql", "--format", "gitlab-codequality", "--fail-on", "none"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != 0 {
		t.Fatalf("expected exit code 0 with --fail-on none, got %d, stderr=%s", code, stderr.String())
	}

	var issues []map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &issues); err != nil {
		t.Fatalf("unmarshal: %v\noutput=%s", err, stdout.String())
	}

	hasNotValid := false
	for _, issue := range issues {
		cn, _ := issue["check_name"].(string)
		if cn == "ddl.pg.alter.not_valid_constraint.validate.require" {
			hasNotValid = true
			loc, _ := issue["location"].(map[string]any)
			if loc == nil {
				t.Fatal("global finding missing location")
			}
			if loc["path"] != "deltascope.sql" {
				t.Errorf("location.path = %v, want deltascope.sql", loc["path"])
			}
			sev, _ := issue["severity"].(string)
			if sev == "" {
				t.Error("severity is empty")
			}
			fp, _ := issue["fingerprint"].(string)
			if len(fp) != 64 {
				t.Errorf("fingerprint length = %d, want 64", len(fp))
			}
			break
		}
	}
	if !hasNotValid {
		t.Errorf("expected ddl.pg.alter.not_valid_constraint.validate.require in issues, got check_names: %v", func() []string {
			var names []string
			for _, issue := range issues {
				names = append(names, issue["check_name"].(string))
			}
			return names
		}())
	}
}

const locationFidelityPGMultiStmtSQL = `create table ok_users (
  id bigint primary key
);

delete from users;`

// TestLocationFidelityPostgreSQLGitHubActionsFileAndLine verifies that --format
// github-actions with --file and --dialect postgresql outputs the file path
// and real statement-start line number.
func TestLocationFidelityPostgreSQLGitHubActionsFileAndLine(t *testing.T) {
	dir := t.TempDir()
	sqlPath := filepath.Join(dir, "migrations.sql")
	if err := os.WriteFile(sqlPath, []byte(locationFidelityPGMultiStmtSQL), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	stdout := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--dialect", "postgresql", "--file", sqlPath, "--format", "github-actions", "--fail-on", "none"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)
	if code != 0 {
		t.Fatalf("expected exit code 0 with --fail-on none, got %d", code)
	}

	output := stdout.String()
	if !strings.Contains(output, "dml.where.require") {
		t.Fatalf("expected dml.where.require in output, got: %s", output)
	}

	if !strings.Contains(output, "file="+filepath.ToSlash(sqlPath)) {
		t.Errorf("expected file=%s in annotation, got: %s", filepath.ToSlash(sqlPath), output)
	}
	// "delete from users;" starts on line 5 in locationFidelityPGMultiStmtSQL.
	if !strings.Contains(output, "line=5") {
		t.Errorf("expected line=5 (delete statement start) in annotation, got: %s", output)
	}
}

// TestLocationFidelityPostgreSQLSARIFArtifactURIAndLine verifies that --format sarif
// with --file and --dialect postgresql outputs artifactLocation.uri and real line numbers.
func TestLocationFidelityPostgreSQLSARIFArtifactURIAndLine(t *testing.T) {
	dir := t.TempDir()
	sqlPath := filepath.Join(dir, "migrations.sql")
	if err := os.WriteFile(sqlPath, []byte(locationFidelityPGMultiStmtSQL), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	stdout := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--dialect", "postgresql", "--file", sqlPath, "--format", "sarif", "--fail-on", "none"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)
	if code != 0 {
		t.Fatalf("expected exit code 0 with --fail-on none, got %d", code)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal sarif: %v\noutput=%s", err, stdout.String())
	}

	runs, ok := decoded["runs"].([]any)
	if !ok || len(runs) == 0 {
		t.Fatal("expected runs array in SARIF output")
	}
	run, _ := runs[0].(map[string]any)
	results, _ := run["results"].([]any)

	var whereResult map[string]any
	for _, r := range results {
		result, _ := r.(map[string]any)
		if result["ruleId"] == "dml.where.require" {
			whereResult = result
			break
		}
	}
	if whereResult == nil {
		t.Fatal("expected dml.where.require result in SARIF")
	}

	locations, _ := whereResult["locations"].([]any)
	if len(locations) == 0 {
		t.Fatal("expected locations array in dml.where.require result")
	}

	loc, _ := locations[0].(map[string]any)
	phys, _ := loc["physicalLocation"].(map[string]any)
	if phys == nil {
		t.Fatal("expected physicalLocation in SARIF location")
	}

	artifact, _ := phys["artifactLocation"].(map[string]any)
	if artifact == nil {
		t.Fatal("expected artifactLocation in SARIF physicalLocation")
	}
	uri, _ := artifact["uri"].(string)
	if uri == "" {
		t.Error("expected artifactLocation.uri to be populated")
	}

	region, _ := phys["region"].(map[string]any)
	startLine, _ := region["startLine"].(float64)
	// "delete from users;" starts on line 5 in locationFidelityPGMultiStmtSQL.
	if startLine != 5 {
		t.Errorf("expected startLine=5 (delete statement start), got %v", startLine)
	}
}

// TestLocationFidelityPostgreSQLGitLabCodeQualityLineReal verifies that --format
// gitlab-codequality with --file and --dialect postgresql preserves location.path
// and uses real statement-start line numbers.
func TestLocationFidelityPostgreSQLGitLabCodeQualityLineReal(t *testing.T) {
	dir := t.TempDir()
	sqlPath := filepath.Join(dir, "migrations.sql")
	if err := os.WriteFile(sqlPath, []byte(locationFidelityPGMultiStmtSQL), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	stdout := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--dialect", "postgresql", "--file", sqlPath, "--format", "gitlab-codequality", "--fail-on", "none"},
		strings.NewReader(""),
		stdout,
		&strings.Builder{},
	)
	if code != 0 {
		t.Fatalf("expected exit code 0 with --fail-on none, got %d", code)
	}

	var issues []map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &issues); err != nil {
		t.Fatalf("unmarshal gitlab-codequality: %v\noutput=%s", err, stdout.String())
	}

	var whereIssue map[string]any
	for _, issue := range issues {
		if issue["check_name"] == "dml.where.require" {
			whereIssue = issue
			break
		}
	}
	if whereIssue == nil {
		t.Fatal("expected dml.where.require issue")
	}

	loc, _ := whereIssue["location"].(map[string]any)
	path, _ := loc["path"].(string)
	lines, _ := loc["lines"].(map[string]any)
	begin := lines["begin"]

	if path == "" {
		t.Fatal("expected location.path to be populated from --file")
	}

	beginFloat, _ := begin.(float64)
	// "delete from users;" starts on line 5 in locationFidelityPGMultiStmtSQL.
	if beginFloat != 5 {
		t.Errorf("expected lines.begin=5 (delete statement start line), got %v", begin)
	}
}
