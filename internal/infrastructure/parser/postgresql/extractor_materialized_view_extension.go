//go:build postgresql

package postgresql

import (
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func extractAlterMaterializedViewExtension(statement spec.Statement, stmt *pg_query.AlterObjectDependsStmt) spec.Statement {
	table := tableFromRangeVar(stmt.GetRelation())
	extension := stmt.GetExtname().GetSval()
	if table == nil || extension == "" {
		return unsupportedStatement(statement, "alter_materialized_view", "postgresql materialized view or extension identity is missing")
	}
	action := "depends_on_extension"
	if stmt.GetRemove() {
		action = "no_depends_on_extension"
	}
	statement.DDL = &spec.DDL{
		Operation: spec.DDLOperationAlterMaterializedView, ObjectType: "materialized_view", ObjectName: table.Name, Table: table,
		PGTableCatalogQualified: stmt.GetRelation().GetCatalogname() != "", PGMaterializedViewExtension: extension,
		Alter: []spec.Alter{{Action: action}},
	}
	return statement
}
