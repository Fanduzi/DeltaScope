//go:build postgresql

package postgresql

import (
	"fmt"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func extractDropStmt(statement spec.Statement, stmt *pg_query.DropStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "drop", "postgresql drop statement payload is missing")
	}
	switch stmt.GetRemoveType() {
	case pg_query.ObjectType_OBJECT_TABLE:
		statement.DDL = &spec.DDL{Operation: spec.DDLOperationDropTable, Table: tableFromObjectName(stmt.GetObjects())}
	case pg_query.ObjectType_OBJECT_VIEW:
		if len(stmt.GetObjects()) != 1 {
			return unsupportedStatement(statement, "drop", "postgresql multi-target drop view is unsupported in v1")
		}
		statement.DDL = &spec.DDL{
			Operation: spec.DDLOperationDropView,
			Table:     tableFromObjectName(stmt.GetObjects()),
			Options:   map[string]string{"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk())},
		}
	case pg_query.ObjectType_OBJECT_INDEX:
		options := map[string]string{}
		indexTable := tableFromObjectName(stmt.GetObjects())
		if indexTable != nil && strings.TrimSpace(indexTable.Schema) != "" {
			options["schema"] = strings.TrimSpace(indexTable.Schema)
		}
		statement.DDL = &spec.DDL{
			Operation: spec.DDLOperationDropIndex,
			Alter:     []spec.Alter{{Action: "drop_index", Name: objectNameFromObjectName(stmt.GetObjects()), Options: options}},
		}
	case pg_query.ObjectType_OBJECT_SCHEMA:
		options := map[string]string{"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk())}
		if stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE {
			options["cascade"] = "true"
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationDropSchema,
			ObjectName: dropTargetName(stmt),
			ObjectType: "schema",
			Options:    options,
		}
	case pg_query.ObjectType_OBJECT_SEQUENCE:
		options := map[string]string{"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk())}
		if stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE {
			options["cascade"] = "true"
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationDropSequence,
			ObjectName: dropTargetName(stmt),
			ObjectType: "sequence",
			Options:    options,
		}
	case pg_query.ObjectType_OBJECT_MATVIEW:
		options := map[string]string{"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk())}
		if stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE {
			options["cascade"] = "true"
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationDropMaterializedView,
			ObjectName: dropTargetName(stmt),
			ObjectType: "materialized_view",
			Options:    options,
		}
	case pg_query.ObjectType_OBJECT_TYPE:
		options := map[string]string{"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk())}
		if stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE {
			options["cascade"] = "true"
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationDropType,
			ObjectName: dropTypeNameFromObjects(stmt.GetObjects()),
			ObjectType: "type",
			Options:    options,
		}
	case pg_query.ObjectType_OBJECT_DOMAIN:
		options := map[string]string{"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk())}
		if stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE {
			options["cascade"] = "true"
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationDropDomain,
			ObjectName: dropTypeNameFromObjects(stmt.GetObjects()),
			ObjectType: "domain",
			Options:    options,
		}
	case pg_query.ObjectType_OBJECT_EXTENSION:
		options := map[string]string{"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk())}
		if stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE {
			options["cascade"] = "true"
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationDropExtension,
			ObjectName: dropTargetName(stmt),
			ObjectType: "extension",
			Options:    options,
		}
	case pg_query.ObjectType_OBJECT_POLICY:
		options := map[string]string{"if_exists": fmt.Sprintf("%t", stmt.GetMissingOk())}
		if stmt.GetBehavior() == pg_query.DropBehavior_DROP_CASCADE {
			options["cascade"] = "true"
		}
		statement.DDL = &spec.DDL{
			Operation:  spec.DDLOperationDropPolicy,
			ObjectName: dropPolicyName(stmt),
			ObjectType: "policy",
			Options:    options,
		}
	default:
		return unsupportedStatement(statement, "drop", "postgresql drop target is not in the approved v1 subset")
	}
	return statement
}

func extractTruncateStmt(statement spec.Statement, stmt *pg_query.TruncateStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "truncate", "postgresql truncate statement payload is missing")
	}
	statement.DDL = &spec.DDL{Operation: spec.DDLOperationTruncateTable, Table: tableFromRelationNodeList(stmt.GetRelations())}
	return statement
}

func dropTargetName(stmt *pg_query.DropStmt) string {
	if name := objectNameFromObjectName(stmt.GetObjects()); name != "" {
		return name
	}
	for _, obj := range stmt.GetObjects() {
		if s := obj.GetString_(); s != nil && s.GetSval() != "" {
			return s.GetSval()
		}
	}
	return ""
}

func dropTypeNameFromObjects(objects []*pg_query.Node) string {
	for _, obj := range objects {
		tn := obj.GetTypeName()
		if tn == nil {
			continue
		}
		for _, name := range tn.GetNames() {
			if s := name.GetString_(); s != nil && s.GetSval() != "" {
				return s.GetSval()
			}
		}
	}
	return ""
}
