//go:build postgresql

package postgresql

import (
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func extractCreateEventTrigStmt(statement spec.Statement, stmt *pg_query.CreateEventTrigStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "create_event_trigger", "postgresql create event trigger statement payload is missing")
	}

	options := map[string]string{}
	if eventName := stmt.GetEventname(); eventName != "" {
		options["event"] = eventName
	}
	if funcName := firstStringFromNodes(stmt.GetFuncname()); funcName != "" {
		options["function"] = funcName
	}

	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationCreateEventTrigger,
		ObjectName: stmt.GetTrigname(),
		ObjectType: "event_trigger",
		Options:    options,
	}
	return statement
}

func extractAlterEventTrigStmt(statement spec.Statement, stmt *pg_query.AlterEventTrigStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "alter_event_trigger", "postgresql alter event trigger statement payload is missing")
	}

	options := map[string]string{}
	switch stmt.GetTgenabled() {
	case "D":
		options["action"] = "disable"
	case "O", "":
		options["action"] = "enable"
	case "R":
		options["action"] = "enable_replica"
	case "A":
		options["action"] = "enable_always"
	default:
		options["action"] = "enable"
	}

	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationAlterEventTrigger,
		ObjectName: stmt.GetTrigname(),
		ObjectType: "event_trigger",
		Options:    options,
	}
	return statement
}

func extractRuleStmt(statement spec.Statement, stmt *pg_query.RuleStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "create_rule", "postgresql create rule statement payload is missing")
	}

	options := map[string]string{}
	if rel := stmt.GetRelation(); rel != nil {
		if name := rel.GetRelname(); name != "" {
			options["table"] = name
		}
	}
	options["event"] = cmdTypeToRuleEvent(stmt.GetEvent())
	if stmt.GetInstead() {
		options["instead"] = "true"
	}
	if stmt.GetReplace() {
		options["replace"] = "true"
	}

	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationCreateRule,
		ObjectName: stmt.GetRulename(),
		ObjectType: "rule",
		Options:    options,
	}
	return statement
}

func cmdTypeToRuleEvent(cmd pg_query.CmdType) string {
	switch cmd {
	case pg_query.CmdType_CMD_SELECT:
		return "select"
	case pg_query.CmdType_CMD_INSERT:
		return "insert"
	case pg_query.CmdType_CMD_UPDATE:
		return "update"
	case pg_query.CmdType_CMD_DELETE:
		return "delete"
	default:
		return fmt.Sprintf("unknown_%d", int(cmd))
	}
}

func extractRenameEventTrigger(statement spec.Statement, stmt *pg_query.RenameStmt) spec.Statement {
	objectName := ""
	if obj := stmt.GetObject(); obj != nil {
		objectName = stringNodeValue(obj)
	}
	if objectName == "" {
		objectName = stmt.GetSubname()
	}

	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationAlterEventTrigger,
		ObjectName: objectName,
		ObjectType: "event_trigger",
		Options: map[string]string{
			"action":   "rename",
			"new_name": stmt.GetNewname(),
		},
	}
	return statement
}

func extractRenameRule(statement spec.Statement, stmt *pg_query.RenameStmt) spec.Statement {
	options := map[string]string{
		"action":   "rename",
		"new_name": stmt.GetNewname(),
	}
	if rel := stmt.GetRelation(); rel != nil {
		if name := rel.GetRelname(); name != "" {
			options["table"] = name
		}
	}

	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationAlterRule,
		ObjectName: stmt.GetSubname(),
		ObjectType: "rule",
		Options:    options,
	}
	return statement
}

func extractDropEventTrigger(statement spec.Statement, stmt *pg_query.DropStmt) spec.Statement {
	options := map[string]string{
		"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk()),
	}
	if stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE {
		options["cascade"] = "true"
	}

	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationDropEventTrigger,
		ObjectName: dropTargetName(stmt),
		ObjectType: "event_trigger",
		Options:    options,
	}
	return statement
}

func extractDropRule(statement spec.Statement, stmt *pg_query.DropStmt) spec.Statement {
	options := map[string]string{
		"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk()),
	}
	if stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE {
		options["cascade"] = "true"
	}

	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationDropRule,
		ObjectName: dropTargetName(stmt),
		ObjectType: "rule",
		Options:    options,
	}
	return statement
}
