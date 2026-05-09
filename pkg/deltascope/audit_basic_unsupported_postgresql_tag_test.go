//go:build postgresql

package deltascope

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestAuditReturnsUnsupportedSentinelAndPartialResultForPostgreSQL(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "alter table users rename column old_name to new_name; select 1;",
		Dialect: DialectPostgreSQL,
	})
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("expected unsupported statement sentinel, got %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 supported statement result, got %#v", result.Statements)
	}
	if result.Statements[0].Kind != "ddl" {
		t.Fatalf("expected supported statement kind ddl, got %#v", result.Statements[0])
	}
	if len(result.Unsupported) != 1 {
		t.Fatalf("expected 1 unsupported detail, got %#v", result.Unsupported)
	}
	if result.Unsupported[0].Feature != "select" {
		t.Fatalf("expected unsupported feature select, got %#v", result.Unsupported[0])
	}
	if result.Unsupported[0].Reason == "" {
		t.Fatalf("expected unsupported reason, got %#v", result.Unsupported[0])
	}
	if result.Unsupported[0].Index != 1 {
		t.Fatalf("expected unsupported statement index 1, got %#v", result.Unsupported[0])
	}
}

func TestAuditPostgreSQLOnConflictDoesNotReturnMySQLSpecificFinding(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "insert into users(id, name) values (1, 'a') on conflict (id) do update set name = excluded.name;",
		Dialect: DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "dml.insert.on_duplicate.forbid" {
			t.Fatalf("expected no on-duplicate finding for postgresql on conflict, got %#v", finding)
		}
		if finding.RuleID == "dml.insert.select.forbid" {
			t.Fatalf("expected no insert-select finding for values-based postgresql insert, got %#v", finding)
		}
		if strings.Contains(finding.Message, "ON DUPLICATE KEY") {
			t.Fatalf("expected no MySQL-specific finding message, got %#v", finding)
		}
	}
}

func TestAuditPostgreSQLInsertSelectOnConflictKeepsInsertSelectFindingOnly(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "insert into users(id, name) select id, name from staging_users on conflict (id) do update set name = excluded.name;",
		Dialect: DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit: %v", err)
	}
	if len(result.Statements) != 1 {
		t.Fatalf("expected 1 statement result, got %#v", result.Statements)
	}

	foundInsertSelect := false
	for _, finding := range result.Statements[0].Findings {
		if finding.RuleID == "dml.insert.on_duplicate.forbid" {
			t.Fatalf("expected no on-duplicate finding for postgresql on conflict, got %#v", finding)
		}
		if finding.RuleID == "dml.insert.select.forbid" {
			foundInsertSelect = true
		}
		if strings.Contains(finding.Message, "ON DUPLICATE KEY") {
			t.Fatalf("expected no MySQL-specific finding message, got %#v", finding)
		}
	}
	if !foundInsertSelect {
		t.Fatal("expected insert-select finding for postgresql insert-select on conflict")
	}
}

func TestAuditPostgreSQLCreateOrReplaceViewReturnsUnsupported(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "create or replace view active_users as select id from users;",
		Dialect: DialectPostgreSQL,
	})
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("expected unsupported statement sentinel, got %v", err)
	}
	if len(result.Unsupported) != 1 {
		t.Fatalf("expected 1 unsupported detail, got %#v", result.Unsupported)
	}
	if result.Unsupported[0].Feature != "create_view" {
		t.Fatalf("expected unsupported feature create_view, got %#v", result.Unsupported[0])
	}
	if result.Unsupported[0].Reason == "" {
		t.Fatalf("expected unsupported reason, got %#v", result.Unsupported[0])
	}
}

func TestAuditPostgreSQLCreateTablePartitioningReturnsUnsupported(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "create table orders (id bigint, created_at date) partition by range (created_at);",
		Dialect: DialectPostgreSQL,
	})
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("expected unsupported statement sentinel, got %v", err)
	}
	if len(result.Unsupported) != 1 {
		t.Fatalf("expected 1 unsupported detail, got %#v", result.Unsupported)
	}
	if result.Unsupported[0].Feature != "partitioning" {
		t.Fatalf("expected unsupported feature partitioning, got %#v", result.Unsupported[0])
	}
	if result.Unsupported[0].Reason == "" {
		t.Fatalf("expected unsupported reason, got %#v", result.Unsupported[0])
	}
}

func TestAuditPostgreSQLCreateTableExclusionReturnsUnsupported(t *testing.T) {
	result, err := Audit(context.Background(), Request{
		SQL:     "CREATE TABLE bookings (room_id int, during tsrange, EXCLUDE USING gist (room_id WITH =, during WITH &&));",
		Dialect: DialectPostgreSQL,
	})
	if !errors.Is(err, ErrUnsupportedStatement) {
		t.Fatalf("expected unsupported statement sentinel, got %v", err)
	}
	if len(result.Unsupported) != 1 {
		t.Fatalf("expected 1 unsupported detail, got %#v", result.Unsupported)
	}
	if result.Unsupported[0].Feature != "exclusion_constraint" {
		t.Fatalf("expected unsupported feature exclusion_constraint, got %#v", result.Unsupported[0])
	}
	if result.Unsupported[0].Reason == "" {
		t.Fatalf("expected unsupported reason, got %#v", result.Unsupported[0])
	}
}
