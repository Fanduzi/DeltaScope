//go:build postgresql

package postgresql

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestParserAlterConstraintDeferrable(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER TABLE orders ALTER CONSTRAINT orders_user_id_fkey DEFERRABLE")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterTable {
		t.Errorf("expected operation alter_table, got %s", s.DDL.Operation)
	}
	if s.DDL.Table == nil || s.DDL.Table.Name != "orders" {
		t.Errorf("expected table orders, got %v", s.DDL.Table)
	}
	if len(s.DDL.Alter) != 1 {
		t.Fatalf("expected 1 alter, got %d", len(s.DDL.Alter))
	}
	alter := s.DDL.Alter[0]
	if alter.Action != "alter_constraint_deferrable" {
		t.Errorf("expected action alter_constraint_deferrable, got %s", alter.Action)
	}
	if alter.Name != "orders_user_id_fkey" {
		t.Errorf("expected name orders_user_id_fkey, got %s", alter.Name)
	}
	assertAlterOption(t, alter, "constraint_type", "foreign_key")
	assertAlterOption(t, alter, "deferrable", "true")
	assertAlterOption(t, alter, "initially_deferred", "false")
	assertAlterForbiddenOptions(t, alter)
}

func TestParserAlterConstraintInitiallyDeferred(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER TABLE orders ALTER CONSTRAINT orders_user_id_fkey INITIALLY DEFERRED")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterTable {
		t.Errorf("expected operation alter_table, got %s", s.DDL.Operation)
	}
	if s.DDL.Table == nil || s.DDL.Table.Name != "orders" {
		t.Errorf("expected table orders, got %v", s.DDL.Table)
	}
	if len(s.DDL.Alter) != 1 {
		t.Fatalf("expected 1 alter, got %d", len(s.DDL.Alter))
	}
	alter := s.DDL.Alter[0]
	if alter.Action != "alter_constraint_initially_deferred" {
		t.Errorf("expected action alter_constraint_initially_deferred, got %s", alter.Action)
	}
	if alter.Name != "orders_user_id_fkey" {
		t.Errorf("expected name orders_user_id_fkey, got %s", alter.Name)
	}
	assertAlterOption(t, alter, "constraint_type", "foreign_key")
	assertAlterOption(t, alter, "deferrable", "true")
	assertAlterOption(t, alter, "initially_deferred", "true")
	assertAlterForbiddenOptions(t, alter)
}

func TestParserSetWithoutOids(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER TABLE users SET WITHOUT OIDS")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterTable {
		t.Errorf("expected operation alter_table, got %s", s.DDL.Operation)
	}
	if s.DDL.Table == nil || s.DDL.Table.Name != "users" {
		t.Errorf("expected table users, got %v", s.DDL.Table)
	}
	if len(s.DDL.Alter) != 1 {
		t.Fatalf("expected 1 alter, got %d", len(s.DDL.Alter))
	}
	alter := s.DDL.Alter[0]
	if alter.Action != "set_without_oids" {
		t.Errorf("expected action set_without_oids, got %s", alter.Action)
	}
}

func assertAlterOption(t *testing.T, alter spec.Alter, key, expected string) {
	t.Helper()
	if alter.Options == nil {
		t.Errorf("expected option %s=%s, got nil options", key, expected)
		return
	}
	got, ok := alter.Options[key]
	if !ok {
		t.Errorf("expected option %s=%s, key not found", key, expected)
		return
	}
	if got != expected {
		t.Errorf("expected option %s=%s, got %s", key, expected, got)
	}
}

func assertAlterForbiddenOptions(t *testing.T, alter spec.Alter) {
	t.Helper()
	forbidden := []string{
		"raw_sql",
		"expression",
		"predicate",
		"operator_class",
		"exclusions",
		"sequence_options",
		"catalog_state",
		"validation_result",
	}
	for _, key := range forbidden {
		if _, ok := alter.Options[key]; ok {
			t.Errorf("forbidden option %s present in alter options", key)
		}
	}
}
