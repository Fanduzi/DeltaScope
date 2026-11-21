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
	decl := pgForeignTableChange(stmt.GetRelation(), "", "", stmt.GetMissingOk())
	if cmds := stmt.GetCmds(); len(cmds) == 1 {
		cmd := cmds[0].GetAlterTableCmd()
		decl.ColumnGuard = cmd.GetMissingOk()
		switch cmd.GetSubtype() {
		case pg_query.AlterTableType_AT_GenericOptions:
			decl.Action = "alter_options"
			decl.Options = pgForeignOptions(cmd.GetDef().GetList().GetItems())
		case pg_query.AlterTableType_AT_AddColumn:
			decl.Action, decl.Column = "add_column", cmd.GetDef().GetColumnDef().GetColname()
		case pg_query.AlterTableType_AT_DropColumn:
			decl.Action, decl.Column = "drop_column", cmd.GetName()
		case pg_query.AlterTableType_AT_ChangeOwner:
			decl.Action, decl.Owner = "set_owner", pgRoleSelection(cmd.GetNewowner())
		}
	}
	statement.DDL = &spec.DDL{
		PGForeignTableChange: decl,
		Operation:            spec.DDLOperationAlterForeignTable,
		ObjectName:           stmt.GetRelation().GetRelname(),
		ObjectType:           "foreign_table",
		Options:              options,
	}
	return statement
}

func pgForeignTableChange(relation *pg_query.RangeVar, action, destination string, guard bool) *spec.PGForeignTableChange {
	return &spec.PGForeignTableChange{
		Target: spec.PGDestructiveTableTarget{Table: spec.Table{Schema: relation.GetSchemaname(), Name: relation.GetRelname()}, CatalogQualified: relation.GetCatalogname() != ""},
		Action: action, Destination: destination, IfExists: guard,
	}
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
	decl := &spec.PGWrapperChange{Name: stmt.GetFdwname(), Options: pgForeignOptions(stmt.GetOptions())}
	for _, node := range stmt.GetFuncOptions() {
		d := node.GetDefElem()
		f := spec.PGWrapperFunction{Kind: d.GetDefname(), Clear: d.GetArg() == nil}
		names := d.GetArg().GetList().GetItems()
		if len(names) == 2 {
			f.Target = spec.Table{Schema: stringNodeValue(names[0]), Name: stringNodeValue(names[1])}
		}
		decl.Functions = append(decl.Functions, f)
	}
	statement.DDL = &spec.DDL{
		PGWrapperChange: decl,
		Operation:       spec.DDLOperationAlterForeignDataWrapper,
		ObjectName:      stmt.GetFdwname(),
		ObjectType:      "foreign_data_wrapper",
		Options:         options,
	}
	return statement
}

func pgForeignOptions(items []*pg_query.Node) []spec.PGForeignOption {
	options := make([]spec.PGForeignOption, 0, len(items))
	for _, item := range items {
		d := item.GetDefElem()
		action := ""
		switch d.GetDefaction() {
		case pg_query.DefElemAction_DEFELEM_ADD, pg_query.DefElemAction_DEFELEM_UNSPEC:
			action = "add"
		case pg_query.DefElemAction_DEFELEM_SET:
			action = "set"
		case pg_query.DefElemAction_DEFELEM_DROP:
			action = "drop"
		}
		options = append(options, spec.PGForeignOption{Name: d.GetDefname(), Action: action})
	}
	return options
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
