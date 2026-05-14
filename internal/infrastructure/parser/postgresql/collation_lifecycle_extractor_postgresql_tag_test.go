//go:build postgresql

package postgresql

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestExtractCreateCollation(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "CREATE COLLATION app_collation (provider = libc, locale = 'C')")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationCreateCollation {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationCreateCollation, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "app_collation" {
		t.Errorf("expected object_name app_collation, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "collation" {
		t.Errorf("expected object_type collation, got %q", s.DDL.ObjectType)
	}
}

func TestExtractAlterCollationRename(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER COLLATION app_collation RENAME TO app_collation_v2")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterCollation {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationAlterCollation, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "app_collation" {
		t.Errorf("expected object_name app_collation, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "collation" {
		t.Errorf("expected object_type collation, got %q", s.DDL.ObjectType)
	}
	if s.DDL.Options["action"] != "rename" {
		t.Errorf("expected action=rename, got %q", s.DDL.Options["action"])
	}
	if s.DDL.Options["new_name"] != "app_collation_v2" {
		t.Errorf("expected new_name=app_collation_v2, got %q", s.DDL.Options["new_name"])
	}
}

func TestExtractAlterCollationOwner(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER COLLATION app_collation OWNER TO app_owner")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterCollation {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationAlterCollation, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "app_collation" {
		t.Errorf("expected object_name app_collation, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "collation" {
		t.Errorf("expected object_type collation, got %q", s.DDL.ObjectType)
	}
	if s.DDL.Options["action"] != "set_owner" {
		t.Errorf("expected action=set_owner, got %q", s.DDL.Options["action"])
	}
}

func TestExtractAlterCollationSetSchema(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER COLLATION app_collation SET SCHEMA app")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterCollation {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationAlterCollation, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "app_collation" {
		t.Errorf("expected object_name app_collation, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "collation" {
		t.Errorf("expected object_type collation, got %q", s.DDL.ObjectType)
	}
	if s.DDL.Options["action"] != "set_schema" {
		t.Errorf("expected action=set_schema, got %q", s.DDL.Options["action"])
	}
	if s.DDL.Options["new_schema"] != "app" {
		t.Errorf("expected new_schema=app, got %q", s.DDL.Options["new_schema"])
	}
	// Must NOT be alter_table — this was the Task 1 normalized_silent bug
	if s.DDL.Operation == spec.DDLOperationAlterTable {
		t.Error("operation must not be alter_table for ALTER COLLATION SET SCHEMA")
	}
}

func TestExtractDropCollation(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "DROP COLLATION app_collation")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationDropCollation {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationDropCollation, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "app_collation" {
		t.Errorf("expected object_name app_collation, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "collation" {
		t.Errorf("expected object_type collation, got %q", s.DDL.ObjectType)
	}
	if s.DDL.Options["if_exists"] != "false" {
		t.Errorf("expected if_exists=false, got %q", s.DDL.Options["if_exists"])
	}
}

func TestExtractDropCollationIfExists(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "DROP COLLATION IF EXISTS app_collation CASCADE")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationDropCollation {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationDropCollation, s.DDL.Operation)
	}
	if s.DDL.Options["if_exists"] != "true" {
		t.Errorf("expected if_exists=true, got %q", s.DDL.Options["if_exists"])
	}
	if s.DDL.Options["cascade"] != "true" {
		t.Errorf("expected cascade=true, got %q", s.DDL.Options["cascade"])
	}
}
