//go:build postgresql

package postgresql

import (
	"strconv"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func pgCreateViewOutput(stmt *pg_query.ViewStmt) *spec.PGCreateViewOutput {
	persistence := stmt.GetView().GetRelpersistence()
	result := &spec.PGCreateViewOutput{AliasCount: len(stmt.GetAliases()), Temporary: persistence == "t", Unlogged: persistence == "u"}
	query := stmt.GetQuery().GetSelectStmt()
	if query == nil || query.GetOp() != pg_query.SetOperation_SETOP_NONE || query.GetIntoClause() != nil || query.GetWithClause() != nil ||
		len(query.GetFromClause()) != 0 || len(query.GetValuesLists()) != 0 || query.GetWhereClause() != nil ||
		len(query.GetDistinctClause()) != 0 || len(query.GetGroupClause()) != 0 || query.GetHavingClause() != nil ||
		len(query.GetWindowClause()) != 0 || len(query.GetSortClause()) != 0 || len(query.GetLockingClause()) != 0 ||
		query.GetLimitOffset() != nil || query.GetLimitCount() != nil {
		return result
	}
	columns := make([]spec.PGViewOutputColumn, 0, len(query.GetTargetList()))
	for i, item := range query.GetTargetList() {
		target := item.GetResTarget()
		if target == nil || len(target.GetIndirection()) != 0 {
			return result
		}
		column, known := pgLiteralViewColumn(target.GetVal())
		if !known {
			return result
		}
		column.Name = target.GetName()
		if i < result.AliasCount {
			column.Name = stmt.GetAliases()[i].GetString_().GetSval()
		}
		if column.Name == "" {
			column.Name = "?column?"
		}
		columns = append(columns, column)
	}
	result.Columns = columns
	return result
}

func pgLiteralViewColumn(value *pg_query.Node) (spec.PGViewOutputColumn, bool) {
	column := spec.PGViewOutputColumn{Typmod: -1}
	if collate := value.GetCollateClause(); collate != nil {
		name := collate.GetCollname()
		if len(name) != 2 || name[0].GetString_().GetSval() != "pg_catalog" || collate.GetArg().GetAConst().GetSval() == nil {
			return column, false
		}
		column.TypeOID = 25 // text
		switch name[1].GetString_().GetSval() {
		case "C":
			column.Collation = 950
		case "POSIX":
			column.Collation = 951
		default:
			return column, false
		}
		return column, true
	}
	constant := value.GetAConst()
	if constant == nil {
		return column, false
	}
	switch {
	case constant.GetIval() != nil:
		column.TypeOID = 23 // int4
	case constant.GetFval() != nil:
		number, err := strconv.ParseInt(constant.GetFval().GetFval(), 10, 64)
		if err != nil {
			return column, false
		}
		column.TypeOID = 20 // int8
		if int64(int32(number)) == number {
			column.TypeOID = 23
		}
	case constant.GetBoolval() != nil:
		column.TypeOID = 16 // bool
	case constant.GetSval() != nil || constant.GetIsnull():
		column.TypeOID, column.Collation = 25, 100 // text, default collation
	default:
		return column, false
	}
	return column, true
}
