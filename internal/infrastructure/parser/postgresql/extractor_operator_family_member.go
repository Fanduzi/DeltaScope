//go:build postgresql

package postgresql

import (
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func extractAlterOpFamilyStmt(s spec.Statement, stmt *pg_query.AlterOpFamilyStmt) spec.Statement {
	if len(stmt.GetItems()) != 1 {
		return unsupportedStatement(s, "alter_operator_family", "postgresql operator family member list is unsupported")
	}
	item := stmt.GetItems()[0].GetCreateOpClassItem()
	// OPCLASS_ITEM_OPERATOR has value 1 in the PostgreSQL parser.
	if item.GetItemtype() != 1 || len(item.GetOrderFamily()) != 0 {
		return unsupportedStatement(s, "alter_operator_family", "postgresql operator family member kind is unsupported")
	}
	member := &spec.PGOperatorFamilyMember{Number: item.GetNumber(), Drop: stmt.GetIsDrop()}
	action := "add_operator"
	operands := item.GetName().GetObjargs()
	if member.Drop {
		action = "drop_operator"
		operands = item.GetClassArgs()
	} else if name := item.GetName().GetObjname(); len(name) == 2 {
		member.Operator = spec.Table{Schema: name[0].GetString_().GetSval(), Name: name[1].GetString_().GetSval()}
	}
	if len(operands) == 2 || member.Drop && len(operands) == 1 {
		member.Left = pgOperatorMemberType(operands[0])
		member.Right = pgOperatorMemberType(operands[len(operands)-1])
	}
	parts := stmt.GetOpfamilyname()
	target := &spec.PGOperatorFamilyTarget{Name: lastStringFromNodes(parts), Method: stmt.GetAmname()}
	if len(parts) == 2 {
		target.Schema = parts[0].GetString_().GetSval()
	}
	s.DDL = &spec.DDL{Operation: spec.DDLOperationAlterOperatorFamily, ObjectName: target.Name, ObjectType: "operator_family",
		Options:          map[string]string{"action": action, "access_method": target.Method},
		PGOperatorFamily: target, PGOperatorFamilyMember: member,
	}
	return s
}

func pgOperatorMemberType(node *pg_query.Node) spec.PGColumnTypeTarget {
	typ := node.GetTypeName()
	names := typ.GetNames()
	if len(names) != 2 || len(typ.GetArrayBounds()) != 0 || typ.GetPctType() || typ.GetSetof() || len(typ.GetTypmods()) != 0 {
		return spec.PGColumnTypeTarget{}
	}
	return spec.PGColumnTypeTarget{Schema: names[0].GetString_().GetSval(), Name: names[1].GetString_().GetSval()}
}
