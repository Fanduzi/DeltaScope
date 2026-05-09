//go:build postgresql

package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestAuditCommandRendersPartialJSONForMixedUnsupportedPostgreSQL(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "alter table users add column email text; select 1", "--dialect", "postgresql", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitAudit {
		t.Fatalf("expected audit exit code %d, got %d", exitAudit, code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output when rendering partial result, got %q", stderr.String())
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
	}
	statements, ok := decoded["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one rendered supported statement, got %#v", decoded["statements"])
	}
	unsupported, ok := decoded["unsupported"].([]any)
	if !ok || len(unsupported) != 1 {
		t.Fatalf("expected one unsupported detail, got %#v", decoded["unsupported"])
	}
	item, ok := unsupported[0].(map[string]any)
	if !ok {
		t.Fatalf("expected unsupported object, got %#v", unsupported[0])
	}
	if item["feature"] != "select" || item["reason"] == "" {
		t.Fatalf("expected unsupported feature and reason, got %#v", item)
	}
}

func TestAuditCommandRendersPartialMarkdownForMixedUnsupportedPostgreSQL(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "alter table users add column email text; select 1", "--dialect", "postgresql"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitAudit {
		t.Fatalf("expected audit exit code %d, got %d", exitAudit, code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output when rendering partial markdown result, got %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "## Unsupported Statements") {
		t.Fatalf("expected markdown unsupported section, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Statement 2") || !strings.Contains(stdout.String(), "select") {
		t.Fatalf("expected markdown unsupported statement details, got %q", stdout.String())
	}
}

func TestAuditCommandPostgreSQLOnConflictDoesNotRenderMySQLSpecificMessage(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "insert into users(id, name) values (1, 'a') on conflict (id) do update set name = excluded.name;", "--dialect", "postgresql"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitOK {
		t.Fatalf("expected exit code %d, got %d\nstdout=%q\nstderr=%q", exitOK, code, stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), "ON DUPLICATE KEY") {
		t.Fatalf("expected stdout not to contain MySQL-specific duplicate-key text, got %q", stdout.String())
	}
	if strings.Contains(stdout.String(), "INSERT ... SELECT statements are forbidden") {
		t.Fatalf("expected stdout not to contain insert-select finding, got %q", stdout.String())
	}
	if strings.Contains(stderr.String(), "ON DUPLICATE KEY") {
		t.Fatalf("expected stderr not to contain MySQL-specific duplicate-key text, got %q", stderr.String())
	}
}

func TestAuditCommandPostgreSQLInsertSelectOnConflictKeepsInsertSelectRuleOnly(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}

	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "insert into users(id, name) select id, name from staging_users on conflict (id) do update set name = excluded.name;", "--dialect", "postgresql"},
		strings.NewReader(""),
		stdout,
		stderr,
	)

	if code != exitAudit {
		t.Fatalf("expected exit code %d, got %d\nstdout=%q\nstderr=%q", exitAudit, code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "INSERT ... SELECT statements are forbidden") {
		t.Fatalf("expected stdout to contain insert-select finding, got %q", stdout.String())
	}
	if strings.Contains(stdout.String(), "ON DUPLICATE KEY") {
		t.Fatalf("expected stdout not to contain MySQL-specific duplicate-key text, got %q", stdout.String())
	}
	if strings.Contains(stderr.String(), "ON DUPLICATE KEY") {
		t.Fatalf("expected stderr not to contain MySQL-specific duplicate-key text, got %q", stderr.String())
	}
}
