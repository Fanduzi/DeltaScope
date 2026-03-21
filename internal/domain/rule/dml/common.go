// Package dml defines Tier-1 DML rules.
// input: normalized DML Statement specs and policy-backed rule IDs
// output: reusable DML predicates and rule identifier constants
// pos: DML rule common helpers shared across concrete rules
// note: if this file changes, update this header and module README.md.
package dml

import "github.com/Fanduzi/DeltaScope/internal/domain/spec"

const (
	ruleIDWhereRequire       = "dml.where.require"
	ruleIDLimitForbid        = "dml.limit.forbid"
	ruleIDOrderByForbid      = "dml.order_by.forbid"
	ruleIDSubqueryForbid     = "dml.subquery.forbid"
	ruleIDJoinOnRequire      = "dml.join.on.require"
	ruleIDInsertRowsMaxCount = "dml.insert.rows.max_count"
	ruleIDReplaceForbid      = "dml.replace.forbid"
	ruleIDInsertSelectForbid = "dml.insert.select.forbid"
	ruleIDOnDuplicateForbid  = "dml.insert.on_duplicate.forbid"
	ruleIDTableDenylistForbid = "dml.table.denylist.forbid"
)

func appliesToMutation(statement spec.Statement) bool {
	return statement.Kind == spec.KindDML &&
		statement.DML != nil &&
		(statement.DML.Operation == spec.DMLOperationUpdate || statement.DML.Operation == spec.DMLOperationDelete)
}

func appliesToInsert(statement spec.Statement) bool {
	return statement.Kind == spec.KindDML &&
		statement.DML != nil &&
		statement.DML.Operation == spec.DMLOperationInsert
}
