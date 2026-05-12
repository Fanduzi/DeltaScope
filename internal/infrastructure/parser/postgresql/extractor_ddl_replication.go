//go:build postgresql

package postgresql

import (
	"fmt"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func extractCreatePublicationStmt(statement spec.Statement, stmt *pg_query.CreatePublicationStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "create_publication", "postgresql create publication statement payload is missing")
	}
	options := map[string]string{}
	if stmt.GetForAllTables() {
		options["all_tables"] = "true"
	}
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationCreatePublication,
		ObjectName: stmt.GetPubname(),
		ObjectType: "publication",
		Options:    options,
	}
	return statement
}

func extractAlterPublicationStmt(statement spec.Statement, stmt *pg_query.AlterPublicationStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "alter_publication", "postgresql alter publication statement payload is missing")
	}
	options := map[string]string{}
	switch stmt.GetAction() {
	case pg_query.AlterPublicationAction_AP_AddObjects:
		options["action"] = "add_table"
	case pg_query.AlterPublicationAction_AP_DropObjects:
		options["action"] = "drop_table"
	case pg_query.AlterPublicationAction_AP_SetObjects:
		options["action"] = "set_table"
	default:
		options["action"] = fmt.Sprintf("action_%d", stmt.GetAction())
	}
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationAlterPublication,
		ObjectName: stmt.GetPubname(),
		ObjectType: "publication",
		Options:    options,
	}
	return statement
}

func extractCreateSubscriptionStmt(statement spec.Statement, stmt *pg_query.CreateSubscriptionStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "create_subscription", "postgresql create subscription statement payload is missing")
	}
	options := map[string]string{}
	// Do NOT store connection string value. Record only presence.
	if stmt.GetConninfo() != "" {
		options["has_connection"] = "true"
	}
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationCreateSubscription,
		ObjectName: stmt.GetSubname(),
		ObjectType: "subscription",
		Options:    options,
	}
	return statement
}

func extractAlterSubscriptionStmt(statement spec.Statement, stmt *pg_query.AlterSubscriptionStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "alter_subscription", "postgresql alter subscription statement payload is missing")
	}
	options := map[string]string{}
	kind := stmt.GetKind()
	// pg_query maps both ENABLE and DISABLE to ALTER_SUBSCRIPTION_ENABLED (7).
	// Detect DISABLE via raw SQL since the AST does not distinguish them.
	if kind == pg_query.AlterSubscriptionType_ALTER_SUBSCRIPTION_ENABLED {
		upper := strings.ToUpper(statement.NormalizedSQL)
		if strings.Contains(upper, " DISABLE") {
			options["action"] = "disable"
		} else {
			options["action"] = "enable"
		}
	} else {
		switch kind {
		case pg_query.AlterSubscriptionType_ALTER_SUBSCRIPTION_CONNECTION:
			options["action"] = "connection"
			// Do NOT store connection string value.
			if stmt.GetConninfo() != "" {
				options["has_connection"] = "true"
			}
		case pg_query.AlterSubscriptionType_ALTER_SUBSCRIPTION_SET_PUBLICATION:
			options["action"] = "set_publication"
		case pg_query.AlterSubscriptionType_ALTER_SUBSCRIPTION_ADD_PUBLICATION:
			options["action"] = "add_publication"
		case pg_query.AlterSubscriptionType_ALTER_SUBSCRIPTION_DROP_PUBLICATION:
			options["action"] = "drop_publication"
		case pg_query.AlterSubscriptionType_ALTER_SUBSCRIPTION_REFRESH:
			options["action"] = "refresh"
		case pg_query.AlterSubscriptionType_ALTER_SUBSCRIPTION_OPTIONS:
			options["action"] = "set_options"
		case pg_query.AlterSubscriptionType_ALTER_SUBSCRIPTION_SKIP:
			options["action"] = "skip"
		default:
			options["action"] = fmt.Sprintf("kind_%d", kind)
		}
	}
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationAlterSubscription,
		ObjectName: stmt.GetSubname(),
		ObjectType: "subscription",
		Options:    options,
	}
	return statement
}

func extractDropSubscriptionStmt(statement spec.Statement, stmt *pg_query.DropSubscriptionStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "drop_subscription", "postgresql drop subscription statement payload is missing")
	}
	options := map[string]string{"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk())}
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationDropSubscription,
		ObjectName: stmt.GetSubname(),
		ObjectType: "subscription",
		Options:    options,
	}
	return statement
}
