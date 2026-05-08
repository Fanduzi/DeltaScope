//go:build postgresql

package postgresql

import (
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func extractInsert(stmt *pg_query.InsertStmt) *spec.DML {
	return &spec.DML{
		Operation:      spec.DMLOperationInsert,
		Tables:         singleTableSlice(tableFromRangeVar(stmt.GetRelation())),
		IsInsertSelect: isInsertSelect(stmt),
		HasOnDuplicate: false,
	}
}

func isInsertSelect(stmt *pg_query.InsertStmt) bool {
	if stmt == nil || stmt.GetSelectStmt() == nil {
		return false
	}
	selectStmt := stmt.GetSelectStmt().GetSelectStmt()
	if selectStmt == nil {
		return true
	}
	return len(selectStmt.GetValuesLists()) == 0
}

func extractUpdate(stmt *pg_query.UpdateStmt) *spec.DML {
	hasJoin := len(stmt.GetFromClause()) > 0
	return &spec.DML{
		Operation:     spec.DMLOperationUpdate,
		Tables:        singleTableSlice(tableFromRangeVar(stmt.GetRelation())),
		HasWhere:      stmt.GetWhereClause() != nil,
		HasJoin:       hasJoin,
		IsSingleTable: !hasJoin,
	}
}

func extractDelete(stmt *pg_query.DeleteStmt) *spec.DML {
	hasJoin := len(stmt.GetUsingClause()) > 0
	return &spec.DML{
		Operation:     spec.DMLOperationDelete,
		Tables:        singleTableSlice(tableFromRangeVar(stmt.GetRelation())),
		HasWhere:      stmt.GetWhereClause() != nil,
		HasJoin:       hasJoin,
		IsSingleTable: !hasJoin,
	}
}
