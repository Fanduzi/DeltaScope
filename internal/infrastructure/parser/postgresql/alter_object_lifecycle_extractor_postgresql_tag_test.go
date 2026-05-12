//go:build postgresql

package postgresql

import (
	"context"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func extractStmt(t *testing.T, sql string) spec.Statement {
	t.Helper()
	parser := New()
	result, err := parser.Parse(context.Background(), sql)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(result.Statements) == 0 {
		t.Fatal("no statements returned")
	}
	s, extractErr := result.Statements[0].Extractor.Extract(spec.DialectPostgreSQL, result.Statements[0].RawSQL)
	if extractErr != nil {
		t.Fatalf("extract: %v", extractErr)
	}
	return s
}

func assertSupportedDDL(t *testing.T, s spec.Statement) {
	t.Helper()
	if s.Unsupported != nil {
		t.Fatalf("expected supported, got unsupported: %s: %s", s.Unsupported.Feature, s.Unsupported.Reason)
	}
	if s.DDL == nil {
		t.Fatal("expected DDL, got nil")
	}
}

func TestExtractAlterSchemaRename(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER SCHEMA staging RENAME TO production")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterSchema {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationAlterSchema, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "staging" {
		t.Errorf("expected object_name staging, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "schema" {
		t.Errorf("expected object_type schema, got %q", s.DDL.ObjectType)
	}
	if s.DDL.Options["action"] != "rename" {
		t.Errorf("expected action=rename, got %q", s.DDL.Options["action"])
	}
	if s.DDL.Options["new_name"] != "production" {
		t.Errorf("expected new_name=production, got %q", s.DDL.Options["new_name"])
	}
}

func TestExtractAlterSchemaOwner(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER SCHEMA staging OWNER TO admin_role")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterSchema {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationAlterSchema, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "staging" {
		t.Errorf("expected object_name staging, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "schema" {
		t.Errorf("expected object_type schema, got %q", s.DDL.ObjectType)
	}
	if s.DDL.Options["action"] != "set_owner" {
		t.Errorf("expected action=set_owner, got %q", s.DDL.Options["action"])
	}
	if s.DDL.Options["owner"] != "admin_role" {
		t.Errorf("expected owner=admin_role, got %q", s.DDL.Options["owner"])
	}
}

func TestExtractAlterIndexRename(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER INDEX idx_users_name RENAME TO idx_accounts_name")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterIndex {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationAlterIndex, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "idx_users_name" {
		t.Errorf("expected object_name idx_users_name, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "index" {
		t.Errorf("expected object_type index, got %q", s.DDL.ObjectType)
	}
	if s.DDL.Options["action"] != "rename" {
		t.Errorf("expected action=rename, got %q", s.DDL.Options["action"])
	}
	if s.DDL.Options["new_name"] != "idx_accounts_name" {
		t.Errorf("expected new_name=idx_accounts_name, got %q", s.DDL.Options["new_name"])
	}
}

func TestExtractAlterIndexSetTablespace(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER INDEX idx_users_name SET TABLESPACE pg_default")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterIndex {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationAlterIndex, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "idx_users_name" {
		t.Errorf("expected object_name idx_users_name, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "index" {
		t.Errorf("expected object_type index, got %q", s.DDL.ObjectType)
	}
	if s.DDL.Options["action"] != "set_tablespace" {
		t.Errorf("expected action=set_tablespace, got %q", s.DDL.Options["action"])
	}
	if s.DDL.Options["tablespace"] != "pg_default" {
		t.Errorf("expected tablespace=pg_default, got %q", s.DDL.Options["tablespace"])
	}
}

func TestExtractAlterMaterializedViewRename(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER MATERIALIZED VIEW mv_stats RENAME TO mv_dashboard")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterMaterializedView {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationAlterMaterializedView, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "mv_stats" {
		t.Errorf("expected object_name mv_stats, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "materialized_view" {
		t.Errorf("expected object_type materialized_view, got %q", s.DDL.ObjectType)
	}
	if s.DDL.Options["action"] != "rename" {
		t.Errorf("expected action=rename, got %q", s.DDL.Options["action"])
	}
	if s.DDL.Options["new_name"] != "mv_dashboard" {
		t.Errorf("expected new_name=mv_dashboard, got %q", s.DDL.Options["new_name"])
	}
}

func TestExtractAlterMaterializedViewSetSchema(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER MATERIALIZED VIEW mv_stats SET SCHEMA archive")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterMaterializedView {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationAlterMaterializedView, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "mv_stats" {
		t.Errorf("expected object_name mv_stats, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "materialized_view" {
		t.Errorf("expected object_type materialized_view, got %q", s.DDL.ObjectType)
	}
	if s.DDL.Options["action"] != "set_schema" {
		t.Errorf("expected action=set_schema, got %q", s.DDL.Options["action"])
	}
	if s.DDL.Options["new_schema"] != "archive" {
		t.Errorf("expected new_schema=archive, got %q", s.DDL.Options["new_schema"])
	}
}
