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
	ruleIDTableCommentRequired                = "ddl.table.comment.require"
	ruleIDTableNameMaxLength                  = "ddl.table.name.max_length"
	ruleIDPrimaryKeyRequired                  = "ddl.table.primary_key.require"
	ruleIDPrimaryKeyColumnsMaxCount           = "ddl.table.primary_key.columns.max_count"
	ruleIDTableColumnsMinCount                = "ddl.table.columns.min_count"
	ruleIDTableAuditColumnsRequire            = "ddl.table.audit_columns.require"
	ruleIDColumnCommentRequire                = "ddl.column.comment.require"
	ruleIDColumnNameMaxLength                 = "ddl.column.name.max_length"
	ruleIDColumnVarcharMaxLength              = "ddl.column.varchar.max_length"
	ruleIDColumnDefaultRequire                = "ddl.column.default.require"
	ruleIDColumnNotNullRequire                = "ddl.column.not_null.require"
	ruleIDColumnFloatDoubleForbid             = "ddl.column.float_double.forbid"
	ruleIDIndexTotalMaxCount                  = "ddl.index.total.max_count"
	ruleIDIndexColumnsMaxCount                = "ddl.index.columns.max_count"
	ruleIDIndexUniquePrefixRequire            = "ddl.index.unique.prefix.require"
	ruleIDIndexSecondaryPrefixRequire         = "ddl.index.secondary.prefix.require"
	ruleIDIndexFulltextPrefixRequire          = "ddl.index.fulltext.prefix.require"
	ruleIDIndexDuplicateForbid                = "ddl.index.duplicate.forbid"
	ruleIDAlterDropColumnForbid               = "ddl.alter.drop_column.forbid"
	ruleIDAlterDropPrimaryKeyForbid           = "ddl.alter.drop_primary_key.forbid"
	ruleIDAlterDropIndexForbid                = "ddl.alter.drop_index.forbid"
	ruleIDAlterRenameTableForbid              = "ddl.alter.rename_table.forbid"
	ruleIDAlterRenameColumnForbid             = "ddl.alter.rename_column.forbid"
	ruleIDAlterChangeColumnForbid             = "ddl.alter.change_column.forbid"
	ruleIDAlterModifyColumnForbid             = "ddl.alter.modify_column.forbid"
	ruleIDAlterRenameIndexForbid              = "ddl.alter.rename_index.forbid"
	ruleIDAlterModifyColumnCompatibleRequire  = "ddl.alter.modify_column.compatible.require"
	ruleIDAlterChangeColumnCompatibleRequire  = "ddl.alter.change_column.compatible.require"
	ruleIDAlterAddIndexUniquePrefixRequire    = "ddl.alter.add_index.unique.prefix.require"
	ruleIDAlterAddIndexSecondaryPrefixRequire = "ddl.alter.add_index.secondary.prefix.require"
	ruleIDAlterAddIndexFulltextPrefixRequire  = "ddl.alter.add_index.fulltext.prefix.require"
	ruleIDTableCommentMaxLength               = "ddl.table.comment.max_length"
	ruleIDTableEngineAllowlist                = "ddl.table.engine.allowlist"
	ruleIDTableCharsetAllowlist               = "ddl.table.charset.allowlist"
	ruleIDTableForeignKeyForbid               = "ddl.table.foreign_key.forbid"
	ruleIDTablePartitionForbid                = "ddl.table.partition.forbid"
	ruleIDTableCreateLikeForbid               = "ddl.table.create_like.forbid"
	ruleIDTableCreateAsForbid                 = "ddl.table.create_as.forbid"
	ruleIDPrimaryKeyBigintRequire             = "ddl.table.primary_key.bigint.require"
	ruleIDPrimaryKeyUnsignedRequire           = "ddl.table.primary_key.unsigned.require"
	ruleIDPrimaryKeyAutoIncrementRequire      = "ddl.table.primary_key.auto_increment.require"
	ruleIDPrimaryKeyNotNullRequire            = "ddl.table.primary_key.not_null.require"
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

func appliesToAlterTable(statement spec.Statement) bool {
	return statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Table != nil &&
		len(statement.DDL.Alter) > 0
}

func appliesToAlterActions(statement spec.Statement, actions ...string) bool {
	return appliesToAlterTable(statement) && len(matchingAlterActions(statement, actions...)) > 0
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

func containsFold(items []string, target string) bool {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}

func primaryKeyColumnSpecs(statement spec.Statement) []spec.Column {
	if statement.DDL == nil || statement.DDL.PrimaryKey == nil {
		return nil
	}
	columns := make([]spec.Column, 0, len(statement.DDL.PrimaryKey.Columns))
	for _, pkName := range statement.DDL.PrimaryKey.Columns {
		for _, column := range statement.DDL.Columns {
			if strings.EqualFold(column.Name, pkName) {
				columns = append(columns, column)
				break
			}
		}
	}
	return columns
}

func matchingAlterActions(statement spec.Statement, actions ...string) []spec.Alter {
	if !appliesToAlterTable(statement) || len(actions) == 0 {
		return nil
	}

	matched := make([]spec.Alter, 0)
	for _, alter := range statement.DDL.Alter {
		if containsFold(actions, alter.Action) {
			matched = append(matched, alter)
		}
	}
	return matched
}

func alterColumnDefinition(alter spec.Alter) (*spec.Column, bool) {
	if alter.Column == nil || alter.Column.Definition == nil {
		return nil, false
	}
	return alter.Column.Definition, true
}

func alterIndexDefinition(alter spec.Alter) (*spec.Index, bool) {
	if alter.Index == nil || alter.Index.Definition == nil {
		return nil, false
	}
	return alter.Index.Definition, true
}

func alterRenameNames(alter spec.Alter) (oldName, newName string, ok bool) {
	switch {
	case alter.Column != nil && alter.Column.OldName != "" && alter.Column.Definition != nil && alter.Column.Definition.Name != "":
		return alter.Column.OldName, alter.Column.Definition.Name, true
	case alter.Index != nil && alter.Index.OldName != "" && alter.Index.Definition != nil && alter.Index.Definition.Name != "":
		return alter.Index.OldName, alter.Index.Definition.Name, true
	default:
		return "", "", false
	}
}

func alterOptionValue(alter spec.Alter, key string) (string, bool) {
	if len(alter.Options) == 0 {
		return "", false
	}
	for optionKey, value := range alter.Options {
		if strings.EqualFold(optionKey, key) {
			return value, true
		}
	}
	return "", false
}

func columnTypeFamily(column spec.Column) string {
	switch baseType(column) {
	case "tinyint", "smallint", "mediumint", "int", "integer", "bigint":
		return "integer"
	case "decimal", "numeric":
		return "decimal"
	case "float", "double", "real":
		return "float"
	case "char", "varchar":
		return "string"
	case "binary", "varbinary":
		return "binary"
	case "blob", "tinyblob", "mediumblob", "longblob":
		return "blob"
	case "text", "tinytext", "mediumtext", "longtext", "json":
		return "text"
	case "datetime", "timestamp", "date", "time", "year":
		return "time"
	default:
		return "other"
	}
}
