//go:build postgresql

package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestAuditCommandPostgreSQLCreateSchemaRendersNotice(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "CREATE SCHEMA app;", "--dialect", "postgresql", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)
	_ = code

	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
	}
	statements, ok := decoded["statements"].([]any)
	if !ok || len(statements) != 1 {
		t.Fatalf("expected one statement, got %#v", decoded["statements"])
	}
	statement, ok := statements[0].(map[string]any)
	if !ok {
		t.Fatalf("expected statement object, got %#v", statements[0])
	}
	findings, ok := statement["findings"].([]any)
	if !ok || len(findings) < 1 {
		t.Fatalf("expected at least one finding, got %#v", statement["findings"])
	}

	found := false
	for _, item := range findings {
		finding, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if finding["rule_id"] == "ddl.pg.create_schema.notice" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected rule_id ddl.pg.create_schema.notice, got %#v", findings)
	}

	mysqlDatabaseRules := []string{"ddl.database.create.notice", "ddl.database.drop.warn"}
	for _, item := range findings {
		finding, ok := item.(map[string]any)
		if !ok {
			continue
		}
		ruleID, _ := finding["rule_id"].(string)
		for _, forbidden := range mysqlDatabaseRules {
			if ruleID == forbidden {
				t.Fatalf("PG CLI audit must not emit MySQL-family database rule %q", forbidden)
			}
		}
	}
}
