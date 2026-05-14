//go:build postgresql

package postgresql

import (
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestExtractCreateStatistics(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "CREATE STATISTICS users_stats ON email, status FROM users")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationCreateStatistics {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationCreateStatistics, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "users_stats" {
		t.Errorf("expected object_name users_stats, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "statistics" {
		t.Errorf("expected object_type statistics, got %q", s.DDL.ObjectType)
	}
	if s.DDL.Options["target_table"] != "users" {
		t.Errorf("expected target_table=users, got %q", s.DDL.Options["target_table"])
	}
}

func TestExtractAlterStatisticsRename(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER STATISTICS users_stats RENAME TO users_stats_v2")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterStatistics {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationAlterStatistics, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "users_stats" {
		t.Errorf("expected object_name users_stats, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "statistics" {
		t.Errorf("expected object_type statistics, got %q", s.DDL.ObjectType)
	}
	if s.DDL.Options["action"] != "rename" {
		t.Errorf("expected action=rename, got %q", s.DDL.Options["action"])
	}
	if s.DDL.Options["new_name"] != "users_stats_v2" {
		t.Errorf("expected new_name=users_stats_v2, got %q", s.DDL.Options["new_name"])
	}
}

func TestExtractAlterStatisticsOwner(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER STATISTICS users_stats OWNER TO app_owner")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterStatistics {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationAlterStatistics, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "users_stats" {
		t.Errorf("expected object_name users_stats, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "statistics" {
		t.Errorf("expected object_type statistics, got %q", s.DDL.ObjectType)
	}
	if s.DDL.Options["action"] != "set_owner" {
		t.Errorf("expected action=set_owner, got %q", s.DDL.Options["action"])
	}
	if s.DDL.Options["owner"] != "app_owner" {
		t.Errorf("expected owner=app_owner, got %q", s.DDL.Options["owner"])
	}
}

func TestExtractAlterStatisticsSetSchema(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER STATISTICS users_stats SET SCHEMA app")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterStatistics {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationAlterStatistics, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "users_stats" {
		t.Errorf("expected object_name users_stats, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "statistics" {
		t.Errorf("expected object_type statistics, got %q", s.DDL.ObjectType)
	}
	if s.DDL.Options["action"] != "set_schema" {
		t.Errorf("expected action=set_schema, got %q", s.DDL.Options["action"])
	}
	if s.DDL.Options["new_schema"] != "app" {
		t.Errorf("expected new_schema=app, got %q", s.DDL.Options["new_schema"])
	}
	if s.DDL.Operation == spec.DDLOperationAlterTable {
		t.Error("operation must not be alter_table for ALTER STATISTICS SET SCHEMA")
	}
}

func TestExtractDropStatistics(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "DROP STATISTICS users_stats")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationDropStatistics {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationDropStatistics, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "users_stats" {
		t.Errorf("expected object_name users_stats, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "statistics" {
		t.Errorf("expected object_type statistics, got %q", s.DDL.ObjectType)
	}
	if s.DDL.Options["if_exists"] != "false" {
		t.Errorf("expected if_exists=false, got %q", s.DDL.Options["if_exists"])
	}
}

func TestExtractDropStatisticsIfExists(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "DROP STATISTICS IF EXISTS users_stats CASCADE")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationDropStatistics {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationDropStatistics, s.DDL.Operation)
	}
	if s.DDL.Options["if_exists"] != "true" {
		t.Errorf("expected if_exists=true, got %q", s.DDL.Options["if_exists"])
	}
	if s.DDL.Options["cascade"] != "true" {
		t.Errorf("expected cascade=true, got %q", s.DDL.Options["cascade"])
	}
}
