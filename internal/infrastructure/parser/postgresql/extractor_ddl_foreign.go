//go:build postgresql

package postgresql

import (
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func extractCreateForeignTableStmt(statement spec.Statement, stmt *pg_query.CreateForeignTableStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "create_foreign_table", "postgresql create foreign table statement payload is missing")
	}
	options := map[string]string{}
	if stmt.GetServername() != "" {
		options["server"] = stmt.GetServername()
	}
	if len(stmt.GetOptions()) > 0 {
		options["has_options"] = "true"
	}
	tableName := ""
	if base := stmt.GetBaseStmt(); base != nil && base.GetRelation() != nil {
		tableName = base.GetRelation().GetRelname()
	}
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationCreateForeignTable,
		ObjectName: tableName,
		ObjectType: "foreign_table",
		Options:    options,
	}
	return statement
}

func extractAlterForeignTableStmt(statement spec.Statement, stmt *pg_query.AlterTableStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "alter_foreign_table", "postgresql alter foreign table statement payload is missing")
	}
	options := map[string]string{}
	if stmt.GetRelation() != nil {
		options["if_exists"] = fmt.Sprintf("%t", stmt.GetMissingOk())
	}
	// Classify generic alter-table options for foreign tables.
	for _, cmd := range stmt.GetCmds() {
		atc := cmd.GetAlterTableCmd()
		if atc == nil {
			continue
		}
		switch atc.GetSubtype() {
		case pg_query.AlterTableType_AT_GenericOptions:
			options["action"] = "alter_options"
		default:
			options["action"] = "alter_table_cmd"
		}
		break
	}
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationAlterForeignTable,
		ObjectName: stmt.GetRelation().GetRelname(),
		ObjectType: "foreign_table",
		Options:    options,
	}
	return statement
}

func extractCreateForeignServerStmt(statement spec.Statement, stmt *pg_query.CreateForeignServerStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "create_foreign_server", "postgresql create foreign server statement payload is missing")
	}
	options := map[string]string{}
	if stmt.GetFdwname() != "" {
		options["foreign_data_wrapper"] = stmt.GetFdwname()
	}
	if len(stmt.GetOptions()) > 0 {
		options["has_options"] = "true"
	}
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationCreateForeignServer,
		ObjectName: stmt.GetServername(),
		ObjectType: "foreign_server",
		Options:    options,
	}
	return statement
}

func extractAlterForeignServerStmt(statement spec.Statement, stmt *pg_query.AlterForeignServerStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "alter_foreign_server", "postgresql alter foreign server statement payload is missing")
	}
	options := map[string]string{}
	if stmt.GetHasVersion() {
		options["has_version"] = "true"
	}
	if len(stmt.GetOptions()) > 0 {
		options["has_options"] = "true"
		options["action"] = "alter_options"
	}
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationAlterForeignServer,
		ObjectName: stmt.GetServername(),
		ObjectType: "foreign_server",
		Options:    options,
	}
	return statement
}

func extractCreateUserMappingStmt(statement spec.Statement, stmt *pg_query.CreateUserMappingStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "create_user_mapping", "postgresql create user mapping statement payload is missing")
	}
	options := map[string]string{}
	if stmt.GetServername() != "" {
		options["server"] = stmt.GetServername()
	}
	if u := stmt.GetUser(); u != nil && u.GetRolename() != "" {
		options["user"] = u.GetRolename()
	}
	if len(stmt.GetOptions()) > 0 {
		options["has_options"] = "true"
	}
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationCreateUserMapping,
		ObjectName: userMappingObjectName(stmt.GetUser(), stmt.GetServername()),
		ObjectType: "user_mapping",
		Options:    options,
	}
	return statement
}

func extractAlterUserMappingStmt(statement spec.Statement, stmt *pg_query.AlterUserMappingStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "alter_user_mapping", "postgresql alter user mapping statement payload is missing")
	}
	options := map[string]string{}
	if stmt.GetServername() != "" {
		options["server"] = stmt.GetServername()
	}
	if u := stmt.GetUser(); u != nil && u.GetRolename() != "" {
		options["user"] = u.GetRolename()
	}
	if len(stmt.GetOptions()) > 0 {
		options["has_options"] = "true"
		options["action"] = "alter_options"
	}
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationAlterUserMapping,
		ObjectName: userMappingObjectName(stmt.GetUser(), stmt.GetServername()),
		ObjectType: "user_mapping",
		Options:    options,
	}
	return statement
}

func extractDropUserMappingStmt(statement spec.Statement, stmt *pg_query.DropUserMappingStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "drop_user_mapping", "postgresql drop user mapping statement payload is missing")
	}
	options := map[string]string{
		"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk()),
	}
	if stmt.GetServername() != "" {
		options["server"] = stmt.GetServername()
	}
	if u := stmt.GetUser(); u != nil && u.GetRolename() != "" {
		options["user"] = u.GetRolename()
	}
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationDropUserMapping,
		ObjectName: userMappingObjectName(stmt.GetUser(), stmt.GetServername()),
		ObjectType: "user_mapping",
		Options:    options,
	}
	return statement
}

func extractCreateFdwStmt(statement spec.Statement, stmt *pg_query.CreateFdwStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "create_foreign_data_wrapper", "postgresql create foreign data wrapper statement payload is missing")
	}
	options := map[string]string{}
	if len(stmt.GetFuncOptions()) > 0 {
		options["has_handler"] = "true"
	}
	if len(stmt.GetOptions()) > 0 {
		options["has_options"] = "true"
	}
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationCreateForeignDataWrapper,
		ObjectName: stmt.GetFdwname(),
		ObjectType: "foreign_data_wrapper",
		Options:    options,
	}
	return statement
}

func extractAlterFdwStmt(statement spec.Statement, stmt *pg_query.AlterFdwStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "alter_foreign_data_wrapper", "postgresql alter foreign data wrapper statement payload is missing")
	}
	options := map[string]string{}
	if len(stmt.GetFuncOptions()) > 0 {
		options["has_handler"] = "true"
	}
	if len(stmt.GetOptions()) > 0 {
		options["has_options"] = "true"
		options["action"] = "alter_options"
	}
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationAlterForeignDataWrapper,
		ObjectName: stmt.GetFdwname(),
		ObjectType: "foreign_data_wrapper",
		Options:    options,
	}
	return statement
}

func userMappingObjectName(user *pg_query.RoleSpec, server string) string {
	userName := "public"
	if user != nil && user.GetRolename() != "" {
		userName = user.GetRolename()
	}
	if server != "" {
		return userName + "@" + server
	}
	return userName
}
