//go:build postgresql

package postgresql

import (
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func pgOperatorChangeDeclaration(object *pg_query.ObjectWithArgs, action, destination string, owner spec.PGTableOwnerTarget) *spec.PGOperatorChange {
	decl := &spec.PGOperatorChange{Action: action, Destination: destination, Owner: owner}
	if names := object.GetObjname(); len(names) == 2 {
		decl.Source.Table = spec.Table{Schema: stringNodeValue(names[0]), Name: stringNodeValue(names[1])}
	}
	args := object.GetObjargs()
	if len(args) != 2 || object.GetArgsUnspecified() {
		return decl
	}
	decl.Unary = args[0].GetNode() == nil
	decl.Left = pgExactOperatorOperand(args[0].GetTypeName())
	decl.Right = pgExactOperatorOperand(args[1].GetTypeName())
	decl.Signature = (decl.Unary || decl.Left.Qualified()) && decl.Right.Qualified()
	return decl
}

func pgExactOperatorOperand(node *pg_query.TypeName) spec.PGColumnTypeTarget {
	names := node.GetNames()
	if len(names) != 2 || node.GetPctType() || node.GetSetof() || len(node.GetTypmods()) != 0 {
		return spec.PGColumnTypeTarget{}
	}
	schema := stringNodeValue(names[0])
	if schema == "pg_toast_temp" {
		return spec.PGColumnTypeTarget{}
	}
	return spec.PGColumnTypeTarget{Schema: schema, Name: stringNodeValue(names[1]), Array: len(node.GetArrayBounds()) != 0}
}

func extractOperatorEstimators(statement spec.Statement, stmt *pg_query.AlterOperatorStmt) spec.Statement {
	object := stmt.GetOpername()
	decl := pgOperatorChangeDeclaration(object, "set_estimators", "", spec.PGTableOwnerTarget{})
	options := make([]spec.PGOperatorEstimatorOption, 0, len(stmt.GetOptions()))
	for _, node := range stmt.GetOptions() {
		option := node.GetDefElem()
		kind := option.GetDefname()
		if kind != "restrict" && kind != "join" {
			return unsupportedStatement(statement, "unknown", "postgresql statement type is not in the approved v1 subset")
		}
		var function spec.Table
		if arg := option.GetArg(); arg != nil {
			names := arg.GetTypeName().GetNames()
			if len(names) == 0 {
				return unsupportedStatement(statement, "unknown", "postgresql statement type is not in the approved v1 subset")
			}
			function.Name = lastStringFromNodes(names)
			if len(names) == 2 {
				function.Schema = stringNodeValue(names[0])
			}
		}
		options = append(options, spec.PGOperatorEstimatorOption{Kind: kind, Function: function})
		if kind == "restrict" {
			decl.Restrict = function
		} else {
			decl.Join = function
		}
	}
	statement.DDL = &spec.DDL{
		Operation: spec.DDLOperationAlterOperator, ObjectName: lastStringFromNodes(object.GetObjname()), ObjectType: "operator",
		Options: map[string]string{"action": "set_estimators"}, PGOperatorChange: decl, PGOperatorEstimatorOptions: options,
	}
	return statement
}
