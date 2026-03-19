// Package ddl defines Tier-1 DDL rules.
// input: normalized Statement specs emitted by application extraction
// output: reusable DDL rule predicates and rule identifier constants
// pos: DDL rule common helpers shared across concrete rules
// note: if this file changes, update this header and module README.md.
package ddl

import "github.com/Fanduzi/DeltaScope/internal/domain/spec"

const (
	ruleIDTableCommentRequired      = "ddl.table.comment.require"
	ruleIDTableNameMaxLength        = "ddl.table.name.max_length"
	ruleIDPrimaryKeyRequired        = "ddl.table.primary_key.require"
	ruleIDPrimaryKeyColumnsMaxCount = "ddl.table.primary_key.columns.max_count"
)

func appliesToCreateTable(statement spec.Statement) bool {
	return statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Table != nil &&
		len(statement.DDL.Alter) == 0
}
