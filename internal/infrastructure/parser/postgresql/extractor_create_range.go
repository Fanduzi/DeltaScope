//go:build postgresql

package postgresql

import (
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func extractCreateRangeStmt(statement spec.Statement, stmt *pg_query.CreateRangeStmt) spec.Statement {
	names := stmt.GetTypeName()
	target := spec.PGDestructiveTableTarget{}
	if len(names) == 2 {
		target.Table = spec.Table{Schema: stringNodeValue(names[0]), Name: stringNodeValue(names[1])}
	}
	decl := &spec.PGRangeDeclaration{Target: target}
	seenSubtype := false
	for _, node := range stmt.GetParams() {
		option := node.GetDefElem()
		name := option.GetDefname()
		decl.Options = append(decl.Options, name)
		if name == "subtype" && !seenSubtype {
			decl.Subtype = pgExactOperatorOperand(option.GetArg().GetTypeName())
			seenSubtype = true
		}
	}
	statement.DDL = &spec.DDL{Operation: spec.DDLOperationCreateType, ObjectType: "type",
		ObjectName: lastStringFromNodes(names), PGRange: decl}
	return statement
}
