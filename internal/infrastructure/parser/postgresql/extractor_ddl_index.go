//go:build postgresql

package postgresql

import (
	"strconv"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func extractIndexStmt(statement spec.Statement, stmt *pg_query.IndexStmt) spec.Statement {
	if stmt == nil {
		return unsupportedStatement(statement, "create_index", "postgresql create index statement payload is missing")
	}
	if stmt.GetNullsNotDistinct() {
		return unsupportedStatement(statement, "create_index", "postgresql create index nulls not distinct is unsupported in this milestone")
	}

	columns := indexColumnsFromIndexParams(stmt.GetIndexParams())
	kind := spec.IndexKindSecondary
	if stmt.GetUnique() {
		kind = spec.IndexKindUnique
	}

	accessMethod := stmt.GetAccessMethod()
	if accessMethod == "" {
		accessMethod = "btree"
	}

	exprCount := expressionIndexElemCount(stmt.GetIndexParams())
	includedColumns := indexElemNames(stmt.GetIndexIncludingParams())

	statement.DDL = &spec.DDL{
		Operation: spec.DDLOperationCreateIndex,
		Table:     tableFromRangeVar(stmt.GetRelation()),
		Indexes: []spec.Index{
			{
				Name:              stmt.GetIdxname(),
				Kind:              kind,
				Columns:           columns,
				AccessMethod:      accessMethod,
				IncludedColumns:   includedColumns,
				HasPredicate:      stmt.GetWhereClause() != nil,
				HasExpressionKeys: exprCount > 0,
				ExpressionCount:   exprCount,
			},
		},
		Options: map[string]string{
			"concurrently": strconv.FormatBool(stmt.GetConcurrent()),
		},
	}
	return statement
}

func extractRefreshMatViewStmt(statement spec.Statement, stmt *pg_query.RefreshMatViewStmt) spec.Statement {
	if stmt == nil || stmt.GetRelation() == nil {
		return unsupportedStatement(statement, "refresh_materialized_view", "postgresql refresh materialized view target is missing")
	}
	statement.DDL = &spec.DDL{
		Operation:  spec.DDLOperationRefreshMaterializedView,
		ObjectName: rangeVarName(stmt.GetRelation()),
		ObjectType: "materialized_view",
		Options: map[string]string{
			"concurrently": strconv.FormatBool(stmt.GetConcurrent()),
			"with_no_data": strconv.FormatBool(stmt.GetSkipData()),
		},
	}
	return statement
}

func expressionIndexElemCount(params []*pg_query.Node) int {
	count := 0
	for _, n := range params {
		elem := n.GetIndexElem()
		if elem == nil || elem.GetExpr() != nil {
			count++
		}
	}
	return count
}

func indexElemNames(nodes []*pg_query.Node) []string {
	var names []string
	for _, n := range nodes {
		elem := n.GetIndexElem()
		if elem != nil && elem.GetName() != "" {
			names = append(names, elem.GetName())
		}
	}
	return names
}

func indexColumnsFromIndexParams(params []*pg_query.Node) []string {
	return indexElemNames(params)
}
