//go:build postgresql

package audit

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestAuditSQLPostgreSQLCreateTriggerNotice(t *testing.T) {
	t.Parallel()
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "CREATE TRIGGER trg_audit AFTER INSERT ON users FOR EACH ROW EXECUTE FUNCTION log_change()",
		Dialect: spec.DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	found := false
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.create_trigger.notice" {
			found = true
			if f.Level != "notice" {
				t.Errorf("expected notice level, got %q", f.Level)
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected ddl.pg.create_trigger.notice finding, got %#v", result.Statements[0].Findings)
	}
}

func TestAuditSQLPostgreSQLCreateConstraintTriggerWarn(t *testing.T) {
	t.Parallel()
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "CREATE CONSTRAINT TRIGGER trg_fk AFTER INSERT ON orders FROM items DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_fk()",
		Dialect: spec.DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	found := false
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.create_constraint_trigger.warn" {
			found = true
			if f.Level != "warning" {
				t.Errorf("expected warning level, got %q", f.Level)
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected ddl.pg.create_constraint_trigger.warn finding, got %#v", result.Statements[0].Findings)
	}
}

func TestAuditSQLPostgreSQLDropTriggerAdvisory(t *testing.T) {
	t.Parallel()
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "DROP TRIGGER trg_audit ON users",
		Dialect: spec.DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	found := false
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.drop_trigger.advisory" {
			found = true
			if f.Level != "notice" {
				t.Errorf("expected notice level, got %q", f.Level)
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected ddl.pg.drop_trigger.advisory finding, got %#v", result.Statements[0].Findings)
	}
}

func TestAuditSQLPostgreSQLDropTriggerIfExistsAdvisory(t *testing.T) {
	t.Parallel()
	result, err := AuditSQL(context.Background(), Request{
		SQL:     "DROP TRIGGER IF EXISTS trg_audit ON users",
		Dialect: spec.DialectPostgreSQL,
	})
	if err != nil {
		t.Fatalf("audit sql: %v", err)
	}
	found := false
	for _, f := range result.Statements[0].Findings {
		if f.RuleID == "ddl.pg.drop_trigger.advisory" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected ddl.pg.drop_trigger.advisory finding for IF EXISTS, got %#v", result.Statements[0].Findings)
	}
}

func TestAuditSQLMySQLTriggerDialectIsolation(t *testing.T) {
	t.Parallel()
	_, err := AuditSQL(context.Background(), Request{
		SQL:     "CREATE TRIGGER trg_audit AFTER INSERT ON users FOR EACH ROW CALL log_change()",
		Dialect: spec.DialectMySQL,
	})
	if err == nil {
		t.Fatal("expected MySQL parser to reject trigger or treat as unsupported")
	}
}
