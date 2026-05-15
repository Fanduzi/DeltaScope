//go:build postgresql

package postgresql

import (
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

func TestExtractCreateOperatorFamily(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "CREATE OPERATOR FAMILY int4_ops_family USING btree")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationCreateOperatorFamily {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationCreateOperatorFamily, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "int4_ops_family" {
		t.Errorf("expected object_name int4_ops_family, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "operator_family" {
		t.Errorf("expected object_type operator_family, got %q", s.DDL.ObjectType)
	}
	if s.DDL.Options["access_method"] != "btree" {
		t.Errorf("expected access_method=btree, got %q", s.DDL.Options["access_method"])
	}
	assertNoLeakedOpClassDefinition(t, s)
}

func TestExtractAlterOperatorFamilyRename(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER OPERATOR FAMILY int4_ops_family USING btree RENAME TO int4_ops_family_v2")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterOperatorFamily {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationAlterOperatorFamily, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "int4_ops_family" {
		t.Errorf("expected object_name int4_ops_family, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "operator_family" {
		t.Errorf("expected object_type operator_family, got %q", s.DDL.ObjectType)
	}
	if s.DDL.Options["action"] != "rename" {
		t.Errorf("expected action=rename, got %q", s.DDL.Options["action"])
	}
	if s.DDL.Options["new_name"] != "int4_ops_family_v2" {
		t.Errorf("expected new_name=int4_ops_family_v2, got %q", s.DDL.Options["new_name"])
	}
	if s.DDL.Options["access_method"] != "btree" {
		t.Errorf("expected access_method=btree, got %q", s.DDL.Options["access_method"])
	}
}

func TestExtractAlterOperatorFamilyOwner(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER OPERATOR FAMILY int4_ops_family USING btree OWNER TO app_owner")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterOperatorFamily {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationAlterOperatorFamily, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "int4_ops_family" {
		t.Errorf("expected object_name int4_ops_family, got %q", s.DDL.ObjectName)
	}
	if s.DDL.Options["action"] != "set_owner" {
		t.Errorf("expected action=set_owner, got %q", s.DDL.Options["action"])
	}
	if s.DDL.Options["owner"] != "app_owner" {
		t.Errorf("expected owner=app_owner, got %q", s.DDL.Options["owner"])
	}
	if s.DDL.Options["access_method"] != "btree" {
		t.Errorf("expected access_method=btree, got %q", s.DDL.Options["access_method"])
	}
}

func TestExtractAlterOperatorFamilySetSchema(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER OPERATOR FAMILY int4_ops_family USING btree SET SCHEMA app")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterOperatorFamily {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationAlterOperatorFamily, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "int4_ops_family" {
		t.Errorf("expected object_name int4_ops_family, got %q", s.DDL.ObjectName)
	}
	if s.DDL.Options["action"] != "set_schema" {
		t.Errorf("expected action=set_schema, got %q", s.DDL.Options["action"])
	}
	if s.DDL.Options["new_schema"] != "app" {
		t.Errorf("expected new_schema=app, got %q", s.DDL.Options["new_schema"])
	}
	if s.DDL.Options["access_method"] != "btree" {
		t.Errorf("expected access_method=btree, got %q", s.DDL.Options["access_method"])
	}
}

func TestExtractDropOperatorFamily(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "DROP OPERATOR FAMILY int4_ops_family USING btree")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationDropOperatorFamily {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationDropOperatorFamily, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "int4_ops_family" {
		t.Errorf("expected object_name int4_ops_family, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "operator_family" {
		t.Errorf("expected object_type operator_family, got %q", s.DDL.ObjectType)
	}
	if s.DDL.Options["access_method"] != "btree" {
		t.Errorf("expected access_method=btree, got %q", s.DDL.Options["access_method"])
	}
}

func TestExtractDropOperatorFamilyIfExists(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "DROP OPERATOR FAMILY IF EXISTS int4_ops_family USING btree CASCADE")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationDropOperatorFamily {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationDropOperatorFamily, s.DDL.Operation)
	}
	if s.DDL.Options["if_exists"] != "true" {
		t.Errorf("expected if_exists=true, got %q", s.DDL.Options["if_exists"])
	}
	if s.DDL.Options["cascade"] != "true" {
		t.Errorf("expected cascade=true, got %q", s.DDL.Options["cascade"])
	}
}

func TestExtractCreateOperatorClass(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "CREATE OPERATOR CLASS int4_ops_class DEFAULT FOR TYPE int4 USING btree FAMILY int4_ops_family AS OPERATOR 1 < (int4, int4)")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationCreateOperatorClass {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationCreateOperatorClass, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "int4_ops_class" {
		t.Errorf("expected object_name int4_ops_class, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "operator_class" {
		t.Errorf("expected object_type operator_class, got %q", s.DDL.ObjectType)
	}
	if s.DDL.Options["access_method"] != "btree" {
		t.Errorf("expected access_method=btree, got %q", s.DDL.Options["access_method"])
	}
	if s.DDL.Options["is_default"] != "true" {
		t.Errorf("expected is_default=true, got %q", s.DDL.Options["is_default"])
	}
	assertNoLeakedOpClassDefinition(t, s)
}

func TestExtractAlterOperatorClassRename(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER OPERATOR CLASS int4_ops_class USING btree RENAME TO int4_ops_class_v2")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterOperatorClass {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationAlterOperatorClass, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "int4_ops_class" {
		t.Errorf("expected object_name int4_ops_class, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "operator_class" {
		t.Errorf("expected object_type operator_class, got %q", s.DDL.ObjectType)
	}
	if s.DDL.Options["action"] != "rename" {
		t.Errorf("expected action=rename, got %q", s.DDL.Options["action"])
	}
	if s.DDL.Options["new_name"] != "int4_ops_class_v2" {
		t.Errorf("expected new_name=int4_ops_class_v2, got %q", s.DDL.Options["new_name"])
	}
	if s.DDL.Options["access_method"] != "btree" {
		t.Errorf("expected access_method=btree, got %q", s.DDL.Options["access_method"])
	}
}

func TestExtractAlterOperatorClassOwner(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER OPERATOR CLASS int4_ops_class USING btree OWNER TO app_owner")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterOperatorClass {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationAlterOperatorClass, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "int4_ops_class" {
		t.Errorf("expected object_name int4_ops_class, got %q", s.DDL.ObjectName)
	}
	if s.DDL.Options["action"] != "set_owner" {
		t.Errorf("expected action=set_owner, got %q", s.DDL.Options["action"])
	}
	if s.DDL.Options["owner"] != "app_owner" {
		t.Errorf("expected owner=app_owner, got %q", s.DDL.Options["owner"])
	}
	if s.DDL.Options["access_method"] != "btree" {
		t.Errorf("expected access_method=btree, got %q", s.DDL.Options["access_method"])
	}
}

func TestExtractAlterOperatorClassSetSchema(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER OPERATOR CLASS int4_ops_class USING btree SET SCHEMA app")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterOperatorClass {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationAlterOperatorClass, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "int4_ops_class" {
		t.Errorf("expected object_name int4_ops_class, got %q", s.DDL.ObjectName)
	}
	if s.DDL.Options["action"] != "set_schema" {
		t.Errorf("expected action=set_schema, got %q", s.DDL.Options["action"])
	}
	if s.DDL.Options["new_schema"] != "app" {
		t.Errorf("expected new_schema=app, got %q", s.DDL.Options["new_schema"])
	}
	if s.DDL.Options["access_method"] != "btree" {
		t.Errorf("expected access_method=btree, got %q", s.DDL.Options["access_method"])
	}
}

func TestExtractDropOperatorClass(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "DROP OPERATOR CLASS int4_ops_class USING btree")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationDropOperatorClass {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationDropOperatorClass, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "int4_ops_class" {
		t.Errorf("expected object_name int4_ops_class, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "operator_class" {
		t.Errorf("expected object_type operator_class, got %q", s.DDL.ObjectType)
	}
	if s.DDL.Options["access_method"] != "btree" {
		t.Errorf("expected access_method=btree, got %q", s.DDL.Options["access_method"])
	}
}

func TestExtractDropOperatorClassIfExists(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "DROP OPERATOR CLASS IF EXISTS int4_ops_class USING btree CASCADE")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationDropOperatorClass {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationDropOperatorClass, s.DDL.Operation)
	}
	if s.DDL.Options["if_exists"] != "true" {
		t.Errorf("expected if_exists=true, got %q", s.DDL.Options["if_exists"])
	}
	if s.DDL.Options["cascade"] != "true" {
		t.Errorf("expected cascade=true, got %q", s.DDL.Options["cascade"])
	}
}

// assertNoLeakedOpClassDefinition checks that no support procedure/operator strategy metadata leaks into normalized output.
func assertNoLeakedOpClassDefinition(t *testing.T, s spec.Statement) {
	t.Helper()
	if s.DDL == nil {
		return
	}
	forbidden := []string{"procedure", "function", "support", "strategy", "definition", "body", "query", "options", "password", "secret", "token", "family"}
	for k, v := range s.DDL.Options {
		kl := strings.ToLower(k)
		for _, f := range forbidden {
			if strings.Contains(kl, f) {
				t.Errorf("leaked implementation detail in options: %s=%s", k, v)
			}
		}
	}
}
