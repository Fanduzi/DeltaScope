//go:build postgresql

package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestAuditCommandDefaultPolicyDialectHygienePostgreSQLExcludesMySQLFamilyRules(t *testing.T) {
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	code := Execute(
		context.Background(),
		[]string{"audit", "--sql", "CREATE TABLE pg_smoke (id bigint primary key);", "--dialect", "postgresql", "--format", "json"},
		strings.NewReader(""),
		stdout,
		stderr,
	)
	_ = code
	var decoded map[string]any
	if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
		t.Fatalf("unmarshal json output: %v\noutput=%s", err, stdout.String())
	}
	mysqlOnly := []string{
		"ddl.table.engine.allowlist",
		"ddl.table.charset.allowlist",
		"ddl.table.row_format.allowlist",
		"ddl.table.auto_increment_init.value",
		"ddl.primary_key.unsigned.require",
		"ddl.primary_key.auto_increment.require",
		"ddl.primary_key.not_null.require",
	}
	statements, ok := decoded["statements"].([]any)
	if !ok {
		t.Fatalf("expected statements array, got %#v", decoded["statements"])
	}
	for _, rawStmt := range statements {
		stmt, ok := rawStmt.(map[string]any)
		if !ok {
			continue
		}
		findings, ok := stmt["findings"].([]any)
		if !ok {
			continue
		}
		for _, rawFinding := range findings {
			finding, ok := rawFinding.(map[string]any)
			if !ok {
				continue
			}
			ruleID, _ := finding["rule_id"].(string)
			for _, forbidden := range mysqlOnly {
				if ruleID == forbidden {
					t.Errorf("PG default CLI audit should not emit MySQL-only rule %q", forbidden)
				}
			}
			message, _ := finding["message"].(string)
			suggestion, _ := finding["suggestion"].(string)
			combined := strings.ToUpper(message + " " + suggestion)
			for _, pattern := range []string{"UNSIGNED", "AUTO_INCREMENT", "ON UPDATE CURRENT_TIMESTAMP"} {
				if strings.Contains(combined, pattern) {
					t.Errorf("PG default CLI audit should not contain MySQL-specific text %q", pattern)
				}
			}
		}
	}
	globalFindings, ok := decoded["global_findings"].([]any)
	if !ok {
		return
	}
	for _, rawFinding := range globalFindings {
		finding, ok := rawFinding.(map[string]any)
		if !ok {
			continue
		}
		ruleID, _ := finding["rule_id"].(string)
		for _, forbidden := range mysqlOnly {
			if ruleID == forbidden {
				t.Errorf("PG default CLI audit should not emit MySQL-only rule %q in global findings", forbidden)
			}
		}
	}
}
