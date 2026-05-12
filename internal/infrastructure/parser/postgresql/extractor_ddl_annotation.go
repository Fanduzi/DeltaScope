//go:build postgresql

package postgresql

import (
	"fmt"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func extractCommentStmt(statement spec.Statement, stmt *pg_query.CommentStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "comment_on", "postgresql comment on statement payload is missing")
	}

	targetType := pgObjectTypeToTargetType(stmt.GetObjtype())
	if targetType == "" {
		return unsupportedStatement(statement, "comment_on",
			fmt.Sprintf("postgresql comment on %s is deferred", stmt.GetObjtype()))
	}

	targetName := objectNameFromNode(stmt.GetObject())
	options := map[string]string{
		"target_type": targetType,
		"target_name": targetName,
	}
	isNull := stmt.GetComment() == ""
	if isNull {
		options["is_null"] = "true"
	} else {
		options["is_null"] = "false"
	}

	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationCommentOn,
		ObjectName: targetName,
		ObjectType: "comment",
		Options:    options,
	}
	return statement
}

func extractSecLabelStmt(statement spec.Statement, stmt *pg_query.SecLabelStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "security_label", "postgresql security label statement payload is missing")
	}

	targetType := pgObjectTypeToTargetType(stmt.GetObjtype())
	if targetType == "" {
		return unsupportedStatement(statement, "security_label",
			fmt.Sprintf("postgresql security label on %s is deferred", stmt.GetObjtype()))
	}

	targetName := objectNameFromNode(stmt.GetObject())
	options := map[string]string{
		"target_type": targetType,
		"target_name": targetName,
	}
	if provider := stmt.GetProvider(); provider != "" {
		options["provider"] = provider
	}
	isNull := stmt.GetLabel() == ""
	if isNull {
		options["is_null"] = "true"
	} else {
		options["is_null"] = "false"
	}

	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationSecurityLabel,
		ObjectName: targetName,
		ObjectType: "security_label",
		Options:    options,
	}
	return statement
}

func pgObjectTypeToTargetType(objType pg_query.ObjectType) string {
	switch objType {
	case pg_query.ObjectType_OBJECT_TABLE:
		return "table"
	case pg_query.ObjectType_OBJECT_VIEW:
		return "view"
	case pg_query.ObjectType_OBJECT_MATVIEW:
		return "materialized_view"
	case pg_query.ObjectType_OBJECT_INDEX:
		return "index"
	case pg_query.ObjectType_OBJECT_SEQUENCE:
		return "sequence"
	case pg_query.ObjectType_OBJECT_SCHEMA:
		return "schema"
	case pg_query.ObjectType_OBJECT_COLUMN:
		return "column"
	case pg_query.ObjectType_OBJECT_FUNCTION:
		return "function"
	case pg_query.ObjectType_OBJECT_PROCEDURE:
		return "procedure"
	case pg_query.ObjectType_OBJECT_TYPE:
		return "type"
	case pg_query.ObjectType_OBJECT_DOMAIN:
		return "domain"
	case pg_query.ObjectType_OBJECT_EXTENSION:
		return "extension"
	default:
		return ""
	}
}

func objectNameFromNode(obj *pg_query.Node) string {
	if obj == nil {
		return ""
	}
	if list := obj.GetList(); list != nil {
		return firstStringFromNodes(list.GetItems())
	}
	return firstStringFromNodes([]*pg_query.Node{obj})
}
