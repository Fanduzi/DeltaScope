//go:build postgresql

package postgresql

import (
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func extractCreatePolicyStmt(statement spec.Statement, stmt *pg_query.CreatePolicyStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "create_policy", "postgresql create policy statement payload is missing")
	}
	options := map[string]string{}
	if !stmt.GetPermissive() {
		options["permissive"] = "false"
	}
	if stmt.GetCmdName() != "" {
		options["cmd"] = stmt.GetCmdName()
	}
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationCreatePolicy,
		ObjectName: stmt.GetPolicyName(),
		ObjectType: "policy",
		Table:      tableFromRangeVar(stmt.GetTable()),
		Options:    options,
	}
	return statement
}

func extractAlterPolicyStmt(statement spec.Statement, stmt *pg_query.AlterPolicyStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "alter_policy", "postgresql alter policy statement payload is missing")
	}
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationAlterPolicy,
		ObjectName: stmt.GetPolicyName(),
		ObjectType: "policy",
		Table:      tableFromRangeVar(stmt.GetTable()),
	}
	return statement
}

func dropPolicyName(stmt *pg_query.DropStmt) string {
	return objectNameFromObjectName(stmt.GetObjects())
}
