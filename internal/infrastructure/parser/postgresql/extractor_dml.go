//go:build postgresql

// Package postgresql extracts PostgreSQL DML into normalized statement facts.
// input: PostgreSQL DML AST nodes from pg_query_go
// output: parser-neutral DML operations, target tables, and bounded predicate facts
// pos: PostgreSQL parser adapter DML extraction beneath the application audit flow
// note: if this file changes, update this header and module README.md.
package postgresql

import (
	"strings"

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
	return extractMutationDML(spec.DMLOperationUpdate, stmt.GetRelation(), stmt.GetWhereClause(), hasJoin)
}

func extractDelete(stmt *pg_query.DeleteStmt) *spec.DML {
	hasJoin := len(stmt.GetUsingClause()) > 0
	return extractMutationDML(spec.DMLOperationDelete, stmt.GetRelation(), stmt.GetWhereClause(), hasJoin)
}

func extractMutationDML(operation spec.DMLOperation, relation *pg_query.RangeVar, where *pg_query.Node, hasJoin bool) *spec.DML {
	tables := singleTableSlice(tableFromRangeVar(relation))
	isSingleTable := len(tables) == 1 && !hasJoin
	shape, lookupColumns, matchedKeyName, matchedKeyKind := extractMutationPredicateShape(where, hasJoin, isSingleTable)
	return &spec.DML{
		Operation:      operation,
		Tables:         tables,
		HasWhere:       where != nil,
		HasJoin:        hasJoin,
		PredicateShape: shape,
		LookupColumns:  lookupColumns,
		MatchedKeyName: matchedKeyName,
		MatchedKeyKind: matchedKeyKind,
		IsSingleTable:  isSingleTable,
	}
}

func extractMutationPredicateShape(where *pg_query.Node, hasJoin, isSingleTable bool) (spec.PredicateShape, []string, string, spec.IndexKind) {
	switch {
	case hasJoin:
		return spec.PredicateShapeJoin, nil, "", spec.IndexKindUnknown
	case where == nil:
		return spec.PredicateShapeMissingWhere, nil, "", spec.IndexKindUnknown
	case predicateIsIDLiteralEquality(where, isSingleTable):
		return spec.PredicateShapeUniqueEquality, []string{"id"}, "PRIMARY", spec.IndexKindPrimary
	default:
		return spec.PredicateShapeUnknown, nil, "", spec.IndexKindUnknown
	}
}

func predicateIsIDLiteralEquality(where *pg_query.Node, isSingleTable bool) bool {
	if where == nil || !isSingleTable {
		return false
	}
	predicate := where.GetAExpr()
	if predicate == nil || predicate.GetKind() != pg_query.A_Expr_Kind_AEXPR_OP || len(predicate.GetName()) != 1 || stringNodeValue(predicate.GetName()[0]) != "=" {
		return false
	}
	left := predicate.GetLexpr()
	right := predicate.GetRexpr()
	return (exprIsIDColumnRef(left) && exprIsLiteralValue(right)) || (exprIsIDColumnRef(right) && exprIsLiteralValue(left))
}

func exprIsIDColumnRef(node *pg_query.Node) bool {
	if node == nil || node.GetColumnRef() == nil {
		return false
	}
	return strings.EqualFold(columnRefName(node.GetColumnRef()), "id")
}

func exprIsLiteralValue(node *pg_query.Node) bool {
	if node == nil {
		return false
	}
	if node.GetAConst() != nil || node.GetParamRef() != nil {
		return true
	}

	operator := node.GetAExpr()
	if operator == nil || operator.GetKind() != pg_query.A_Expr_Kind_AEXPR_OP || operator.GetLexpr() != nil || len(operator.GetName()) != 1 {
		return false
	}
	operatorName := stringNodeValue(operator.GetName()[0])
	return (operatorName == "-" || operatorName == "+") && exprIsLiteralValue(operator.GetRexpr())
}
