//go:build postgresql

package deltascope

import (
	"context"
	"strings"
	"testing"
)

func TestAuditDefaultPolicyDialectHygienePostgreSQLExcludesMySQLFamilyRules(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "CREATE TABLE pg_smoke (id bigint primary key);",
		Dialect: DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	mysqlOnly := []string{
		"ddl.table.engine.allowlist",
		"ddl.table.charset.allowlist",
		"ddl.table.row_format.allowlist",
		"ddl.table.auto_increment_init.value",
		"ddl.primary_key.unsigned.require",
		"ddl.primary_key.auto_increment.require",
		"ddl.primary_key.not_null.require",
		"ddl.database.create.notice",
		"ddl.database.drop.warn",
	}
	for _, stmt := range result.Statements {
		for _, finding := range stmt.Findings {
			for _, id := range mysqlOnly {
				if finding.RuleID == id {
					t.Errorf("PG default audit should not emit MySQL-only rule %q", id)
				}
			}
			combined := strings.ToUpper(finding.Message + " " + finding.Suggestion)
			for _, pattern := range []string{"UNSIGNED", "AUTO_INCREMENT", "ON UPDATE CURRENT_TIMESTAMP"} {
				if strings.Contains(combined, pattern) {
					t.Errorf("PG default audit should not contain MySQL-specific text %q", pattern)
				}
			}
		}
	}
	for _, finding := range result.GlobalFindings {
		for _, id := range mysqlOnly {
			if finding.RuleID == id {
				t.Errorf("PG default audit should not emit MySQL-only rule %q in global findings", id)
			}
		}
	}
}

func TestAuditPostgreSQLReturnsSourceLocationsForMultiStatementSQL(t *testing.T) {
	sql := `create table ok_users (
  id bigint primary key
);

delete from users;`

	result, err := Audit(context.Background(), Request{
		SQL:     sql,
		Dialect: DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) < 2 {
		t.Fatalf("expected at least 2 statements, got %d", len(result.Statements))
	}

	deleteStmt := result.Statements[1]
	var whereFinding *Finding
	for i := range deleteStmt.Findings {
		if deleteStmt.Findings[i].RuleID == "dml.where.require" {
			whereFinding = &deleteStmt.Findings[i]
			break
		}
	}
	if whereFinding == nil {
		t.Fatal("expected dml.where.require finding for 'delete from users'")
	}
	if whereFinding.Location == nil {
		t.Fatal("dml.where.require finding Location is nil, expected {Line:5,Column:1}")
	}
	if whereFinding.Location.Line != 5 {
		t.Errorf("finding Location.Line=%d, want 5", whereFinding.Location.Line)
	}
	if whereFinding.Location.Column != 1 {
		t.Errorf("finding Location.Column=%d, want 1", whereFinding.Location.Column)
	}
}

func TestAuditPostgreSQLAdvancedIndexFormsAreSupportedAndCovered(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "CREATE INDEX idx_users_active_email ON users (email) WHERE active = true",
		Dialect: DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Unsupported) != 0 {
		t.Fatalf("expected 0 unsupported, got %#v", result.Unsupported)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(result.Statements))
	}
	if result.Statements[0].Kind != "ddl" {
		t.Fatalf("expected ddl kind, got %q", result.Statements[0].Kind)
	}

	found := false
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.create_index.concurrently.require" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected concurrently.require finding, got %#v", result.Statements[0].Findings)
	}
}
