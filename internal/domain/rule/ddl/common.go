// Package ddl defines Tier-1 DDL rules.
// input: normalized Statement specs emitted by application extraction
// output: reusable DDL rule predicates and rule identifier constants
// pos: DDL rule common helpers shared across concrete rules
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

const (
	ruleIDTableCommentRequired        = "ddl.table.comment.require"
	ruleIDTableNameMaxLength          = "ddl.table.name.max_length"
	ruleIDPrimaryKeyRequired          = "ddl.table.primary_key.require"
	ruleIDPrimaryKeyColumnsMaxCount   = "ddl.table.primary_key.columns.max_count"
	ruleIDTableColumnsMinCount        = "ddl.table.columns.min_count"
	ruleIDTableAuditColumnsRequire    = "ddl.table.audit_columns.require"
	ruleIDColumnCommentRequire        = "ddl.column.comment.require"
	ruleIDColumnNameMaxLength         = "ddl.column.name.max_length"
	ruleIDColumnVarcharMaxLength      = "ddl.column.varchar.max_length"
	ruleIDColumnDefaultRequire        = "ddl.column.default.require"
	ruleIDColumnNotNullRequire        = "ddl.column.not_null.require"
	ruleIDColumnFloatDoubleForbid     = "ddl.column.float_double.forbid"
	ruleIDIndexTotalMaxCount          = "ddl.index.total.max_count"
	ruleIDIndexColumnsMaxCount        = "ddl.index.columns.max_count"
	ruleIDIndexUniquePrefixRequire    = "ddl.index.unique.prefix.require"
	ruleIDIndexSecondaryPrefixRequire = "ddl.index.secondary.prefix.require"
	ruleIDIndexFulltextPrefixRequire  = "ddl.index.fulltext.prefix.require"
	ruleIDIndexDuplicateForbid        = "ddl.index.duplicate.forbid"
)

func appliesToCreateTable(statement spec.Statement) bool {
	return statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Table != nil &&
		len(statement.DDL.Alter) == 0
}

func appliesToCreateTableColumns(statement spec.Statement) bool {
	return appliesToCreateTable(statement) && statement.DDL != nil
}

func appliesToCreateTableIndexes(statement spec.Statement) bool {
	return appliesToCreateTable(statement) && statement.DDL != nil
}

func baseType(column spec.Column) string {
	tp := strings.ToLower(strings.TrimSpace(column.Type))
	if idx := strings.Index(tp, "("); idx >= 0 {
		tp = tp[:idx]
	}
	if idx := strings.Index(tp, " "); idx >= 0 {
		tp = tp[:idx]
	}
	return tp
}

func isBlobTextLike(column spec.Column) bool {
	switch baseType(column) {
	case "blob", "tinyblob", "mediumblob", "longblob", "text", "tinytext", "mediumtext", "longtext", "json":
		return true
	default:
		return false
	}
}

func isTimeLike(column spec.Column) bool {
	switch baseType(column) {
	case "datetime", "timestamp", "date", "time", "year":
		return true
	default:
		return false
	}
}

func indexKindLabel(kind spec.IndexKind) string {
	switch kind {
	case spec.IndexKindUnique:
		return "unique"
	case spec.IndexKindFulltext:
		return "fulltext"
	case spec.IndexKindSecondary:
		return "secondary"
	case spec.IndexKindPrimary:
		return "primary"
	default:
		return "index"
	}
}
