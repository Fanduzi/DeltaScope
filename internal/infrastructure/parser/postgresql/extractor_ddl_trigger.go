//go:build postgresql

package postgresql

import (
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func extractCreateTrigStmt(statement spec.Statement, stmt *pg_query.CreateTrigStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "create_trigger", "postgresql create trigger statement payload is missing")
	}
	options := map[string]string{}
	if stmt.GetIsconstraint() {
		options["constraint"] = "true"
	}
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationCreateTrigger,
		ObjectName: stmt.GetTrigname(),
		ObjectType: "trigger",
		Table:      tableFromRangeVar(stmt.GetRelation()),
		Options:    options,
	}
	return statement
}

func dropTriggerName(stmt *pg_query.DropStmt) string {
	return objectNameFromObjectName(stmt.GetObjects())
}

func extractDropTriggerStmt(statement spec.Statement, stmt *pg_query.DropStmt) spec.Statement {
	options := map[string]string{}
	if stmt.GetMissingOk() {
		options["if_exists"] = "true"
	}
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationDropTrigger,
		ObjectName: dropTriggerName(stmt),
		ObjectType: "trigger",
		Options:    options,
	}
	return statement
}
