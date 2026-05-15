//go:build postgresql

package postgresql

import (
	"strings"
	"testing"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// ===== Aggregate lifecycle =====

func TestExtractCreateAggregate(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "CREATE AGGREGATE sum2(integer) (SFUNC = int4pl, STYPE = integer)")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationCreateAggregate {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationCreateAggregate, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "sum2" {
		t.Errorf("expected object_name sum2, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "aggregate" {
		t.Errorf("expected object_type aggregate, got %q", s.DDL.ObjectType)
	}
	assertNoLeakedDefinition(t, s)
}

func TestExtractAlterAggregateRename(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER AGGREGATE sum2(integer) RENAME TO sum2_v2")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterAggregate {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationAlterAggregate, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "sum2" {
		t.Errorf("expected object_name sum2, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "aggregate" {
		t.Errorf("expected object_type aggregate, got %q", s.DDL.ObjectType)
	}
	if s.DDL.Options["action"] != "rename" {
		t.Errorf("expected action=rename, got %q", s.DDL.Options["action"])
	}
	if s.DDL.Options["new_name"] != "sum2_v2" {
		t.Errorf("expected new_name=sum2_v2, got %q", s.DDL.Options["new_name"])
	}
}

func TestExtractAlterAggregateOwner(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER AGGREGATE sum2(integer) OWNER TO app_owner")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterAggregate {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationAlterAggregate, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "sum2" {
		t.Errorf("expected object_name sum2, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "aggregate" {
		t.Errorf("expected object_type aggregate, got %q", s.DDL.ObjectType)
	}
	if s.DDL.Options["action"] != "set_owner" {
		t.Errorf("expected action=set_owner, got %q", s.DDL.Options["action"])
	}
	if s.DDL.Options["owner"] != "app_owner" {
		t.Errorf("expected owner=app_owner, got %q", s.DDL.Options["owner"])
	}
}

func TestExtractAlterAggregateSetSchema(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER AGGREGATE sum2(integer) SET SCHEMA app")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterAggregate {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationAlterAggregate, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "sum2" {
		t.Errorf("expected object_name sum2, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "aggregate" {
		t.Errorf("expected object_type aggregate, got %q", s.DDL.ObjectType)
	}
	if s.DDL.Options["action"] != "set_schema" {
		t.Errorf("expected action=set_schema, got %q", s.DDL.Options["action"])
	}
	if s.DDL.Options["new_schema"] != "app" {
		t.Errorf("expected new_schema=app, got %q", s.DDL.Options["new_schema"])
	}
	if s.DDL.Operation == spec.DDLOperationAlterTable {
		t.Error("operation must not be alter_table for ALTER AGGREGATE SET SCHEMA")
	}
}

func TestExtractDropAggregate(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "DROP AGGREGATE sum2(integer)")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationDropAggregate {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationDropAggregate, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "sum2" {
		t.Errorf("expected object_name sum2, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "aggregate" {
		t.Errorf("expected object_type aggregate, got %q", s.DDL.ObjectType)
	}
	if s.DDL.Options["if_exists"] != "false" {
		t.Errorf("expected if_exists=false, got %q", s.DDL.Options["if_exists"])
	}
}

func TestExtractDropAggregateIfExists(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "DROP AGGREGATE IF EXISTS sum2(integer) CASCADE")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationDropAggregate {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationDropAggregate, s.DDL.Operation)
	}
	if s.DDL.Options["if_exists"] != "true" {
		t.Errorf("expected if_exists=true, got %q", s.DDL.Options["if_exists"])
	}
	if s.DDL.Options["cascade"] != "true" {
		t.Errorf("expected cascade=true, got %q", s.DDL.Options["cascade"])
	}
}

// ===== Operator lifecycle =====

func TestExtractCreateOperator(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "CREATE OPERATOR === (LEFTARG = integer, RIGHTARG = integer, PROCEDURE = int4eq)")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationCreateOperator {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationCreateOperator, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "===" {
		t.Errorf("expected object_name ===, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "operator" {
		t.Errorf("expected object_type operator, got %q", s.DDL.ObjectType)
	}
	assertNoLeakedDefinition(t, s)
}

func TestExtractAlterOperatorOwner(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER OPERATOR === (integer, integer) OWNER TO app_owner")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterOperator {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationAlterOperator, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "===" {
		t.Errorf("expected object_name ===, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "operator" {
		t.Errorf("expected object_type operator, got %q", s.DDL.ObjectType)
	}
	if s.DDL.Options["action"] != "set_owner" {
		t.Errorf("expected action=set_owner, got %q", s.DDL.Options["action"])
	}
	if s.DDL.Options["owner"] != "app_owner" {
		t.Errorf("expected owner=app_owner, got %q", s.DDL.Options["owner"])
	}
}

func TestExtractAlterOperatorSetSchema(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER OPERATOR === (integer, integer) SET SCHEMA app")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterOperator {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationAlterOperator, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "===" {
		t.Errorf("expected object_name ===, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "operator" {
		t.Errorf("expected object_type operator, got %q", s.DDL.ObjectType)
	}
	if s.DDL.Options["action"] != "set_schema" {
		t.Errorf("expected action=set_schema, got %q", s.DDL.Options["action"])
	}
	if s.DDL.Options["new_schema"] != "app" {
		t.Errorf("expected new_schema=app, got %q", s.DDL.Options["new_schema"])
	}
	if s.DDL.Operation == spec.DDLOperationAlterTable {
		t.Error("operation must not be alter_table for ALTER OPERATOR SET SCHEMA")
	}
}

func TestExtractDropOperator(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "DROP OPERATOR === (integer, integer)")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationDropOperator {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationDropOperator, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "===" {
		t.Errorf("expected object_name ===, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "operator" {
		t.Errorf("expected object_type operator, got %q", s.DDL.ObjectType)
	}
	if s.DDL.Options["if_exists"] != "false" {
		t.Errorf("expected if_exists=false, got %q", s.DDL.Options["if_exists"])
	}
}

func TestExtractDropOperatorIfExists(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "DROP OPERATOR IF EXISTS === (integer, integer) CASCADE")
	assertSupportedDDL(t, s)
	if s.DDL.Options["if_exists"] != "true" {
		t.Errorf("expected if_exists=true, got %q", s.DDL.Options["if_exists"])
	}
	if s.DDL.Options["cascade"] != "true" {
		t.Errorf("expected cascade=true, got %q", s.DDL.Options["cascade"])
	}
}

// ===== Conversion lifecycle =====

func TestExtractCreateConversion(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "CREATE CONVERSION conv FOR 'UTF8' TO 'LATIN1' FROM utf8_to_latin1")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationCreateConversion {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationCreateConversion, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "conv" {
		t.Errorf("expected object_name conv, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "conversion" {
		t.Errorf("expected object_type conversion, got %q", s.DDL.ObjectType)
	}
	assertNoLeakedDefinition(t, s)
}

func TestExtractAlterConversionRename(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER CONVERSION conv RENAME TO conv_v2")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterConversion {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationAlterConversion, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "conv" {
		t.Errorf("expected object_name conv, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "conversion" {
		t.Errorf("expected object_type conversion, got %q", s.DDL.ObjectType)
	}
	if s.DDL.Options["action"] != "rename" {
		t.Errorf("expected action=rename, got %q", s.DDL.Options["action"])
	}
	if s.DDL.Options["new_name"] != "conv_v2" {
		t.Errorf("expected new_name=conv_v2, got %q", s.DDL.Options["new_name"])
	}
}

func TestExtractAlterConversionOwner(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER CONVERSION conv OWNER TO app_owner")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterConversion {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationAlterConversion, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "conv" {
		t.Errorf("expected object_name conv, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "conversion" {
		t.Errorf("expected object_type conversion, got %q", s.DDL.ObjectType)
	}
	if s.DDL.Options["action"] != "set_owner" {
		t.Errorf("expected action=set_owner, got %q", s.DDL.Options["action"])
	}
	if s.DDL.Options["owner"] != "app_owner" {
		t.Errorf("expected owner=app_owner, got %q", s.DDL.Options["owner"])
	}
}

func TestExtractAlterConversionSetSchema(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "ALTER CONVERSION conv SET SCHEMA app")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationAlterConversion {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationAlterConversion, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "conv" {
		t.Errorf("expected object_name conv, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "conversion" {
		t.Errorf("expected object_type conversion, got %q", s.DDL.ObjectType)
	}
	if s.DDL.Options["action"] != "set_schema" {
		t.Errorf("expected action=set_schema, got %q", s.DDL.Options["action"])
	}
	if s.DDL.Options["new_schema"] != "app" {
		t.Errorf("expected new_schema=app, got %q", s.DDL.Options["new_schema"])
	}
	if s.DDL.Operation == spec.DDLOperationAlterTable {
		t.Error("operation must not be alter_table for ALTER CONVERSION SET SCHEMA")
	}
}

func TestExtractDropConversion(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "DROP CONVERSION conv")
	assertSupportedDDL(t, s)
	if s.DDL.Operation != spec.DDLOperationDropConversion {
		t.Errorf("expected operation %q, got %q", spec.DDLOperationDropConversion, s.DDL.Operation)
	}
	if s.DDL.ObjectName != "conv" {
		t.Errorf("expected object_name conv, got %q", s.DDL.ObjectName)
	}
	if s.DDL.ObjectType != "conversion" {
		t.Errorf("expected object_type conversion, got %q", s.DDL.ObjectType)
	}
	if s.DDL.Options["if_exists"] != "false" {
		t.Errorf("expected if_exists=false, got %q", s.DDL.Options["if_exists"])
	}
}

func TestExtractDropConversionIfExists(t *testing.T) {
	t.Parallel()
	s := extractStmt(t, "DROP CONVERSION IF EXISTS conv CASCADE")
	assertSupportedDDL(t, s)
	if s.DDL.Options["if_exists"] != "true" {
		t.Errorf("expected if_exists=true, got %q", s.DDL.Options["if_exists"])
	}
	if s.DDL.Options["cascade"] != "true" {
		t.Errorf("expected cascade=true, got %q", s.DDL.Options["cascade"])
	}
}

// assertNoLeakedDefinition checks that no function/procedure/body metadata leaks into findings.
func assertNoLeakedDefinition(t *testing.T, s spec.Statement) {
	t.Helper()
	if s.DDL == nil {
		return
	}
	forbidden := []string{"sfunc", "stype", "procedure", "function", "definition", "body", "query", "options", "password", "secret", "token"}
	for k, v := range s.DDL.Options {
		kl := strings.ToLower(k)
		for _, f := range forbidden {
			if strings.Contains(kl, f) {
				t.Errorf("leaked implementation detail in options: %s=%s", k, v)
			}
		}
	}
}
