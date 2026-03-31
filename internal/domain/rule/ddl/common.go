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
	ruleIDTableCommentRequired                               = "ddl.table.comment.require"
	ruleIDTableNameMaxLength                                 = "ddl.table.name.max_length"
	ruleIDTableNamePrefixRequire                             = "ddl.table.name.prefix.require"
	ruleIDTableNameSuffixRequire                             = "ddl.table.name.suffix.require"
	ruleIDTableNameContainsRequire                           = "ddl.table.name.contains.require"
	ruleIDPrimaryKeyRequired                                 = "ddl.table.primary_key.require"
	ruleIDPrimaryKeyColumnsMaxCount                          = "ddl.table.primary_key.columns.max_count"
	ruleIDTableColumnsMinCount                               = "ddl.table.columns.min_count"
	ruleIDTableAuditColumnsRequire                           = "ddl.table.audit_columns.require"
	ruleIDColumnCommentRequire                               = "ddl.column.comment.require"
	ruleIDColumnNameMaxLength                                = "ddl.column.name.max_length"
	ruleIDColumnNamePrefixRequire                            = "ddl.column.name.prefix.require"
	ruleIDColumnNameSuffixRequire                            = "ddl.column.name.suffix.require"
	ruleIDColumnNameContainsRequire                          = "ddl.column.name.contains.require"
	ruleIDColumnVarcharMaxLength                             = "ddl.column.varchar.max_length"
	ruleIDColumnDefaultRequire                               = "ddl.column.default.require"
	ruleIDColumnNotNullRequire                               = "ddl.column.not_null.require"
	ruleIDColumnFloatDoubleForbid                            = "ddl.column.float_double.forbid"
	ruleIDTableNamePatternRequire                            = "ddl.table.name.pattern.require"
	ruleIDColumnNamePatternRequire                           = "ddl.column.name.pattern.require"
	ruleIDIndexNamePatternRequire                            = "ddl.index.name.pattern.require"
	ruleIDTableNameKeywordForbid                             = "ddl.table.name.keyword.forbid"
	ruleIDColumnNameKeywordForbid                            = "ddl.column.name.keyword.forbid"
	ruleIDIndexNameKeywordForbid                             = "ddl.index.name.keyword.forbid"
	ruleIDColumnBlobTextForbid                               = "ddl.column.blob_text.forbid"
	ruleIDColumnJSONForbid                                   = "ddl.column.json.forbid"
	ruleIDColumnBitForbid                                    = "ddl.column.bit.forbid"
	ruleIDColumnTimestampForbid                              = "ddl.column.timestamp.forbid"
	ruleIDColumnCharMaxLength                                = "ddl.column.char.max_length"
	ruleIDColumnCharsetAllowlist                             = "ddl.column.charset.allowlist"
	ruleIDColumnCollationAllowlist                           = "ddl.column.collation.allowlist"
	ruleIDColumnCharsetCollationMatchRequire                 = "ddl.column.charset_collation.match.require"
	ruleIDIndexTotalMaxCount                                 = "ddl.index.total.max_count"
	ruleIDIndexColumnsMaxCount                               = "ddl.index.columns.max_count"
	ruleIDIndexUniquePrefixRequire                           = "ddl.index.unique.prefix.require"
	ruleIDIndexSecondaryPrefixRequire                        = "ddl.index.secondary.prefix.require"
	ruleIDIndexFulltextPrefixRequire                         = "ddl.index.fulltext.prefix.require"
	ruleIDIndexDuplicateForbid                               = "ddl.index.duplicate.forbid"
	ruleIDIndexRedundantLeftPrefixForbid                     = "ddl.index.redundant_left_prefix.forbid"
	ruleIDIndexRedundantUniqueOverlapForbid                  = "ddl.index.redundant_unique_overlap.forbid"
	ruleIDAlterDropColumnForbid                              = "ddl.alter.drop_column.forbid"
	ruleIDAlterDropPrimaryKeyForbid                          = "ddl.alter.drop_primary_key.forbid"
	ruleIDAlterDropIndexForbid                               = "ddl.alter.drop_index.forbid"
	ruleIDAlterRenameTableForbid                             = "ddl.alter.rename_table.forbid"
	ruleIDAlterRenameColumnForbid                            = "ddl.alter.rename_column.forbid"
	ruleIDAlterChangeColumnForbid                            = "ddl.alter.change_column.forbid"
	ruleIDAlterModifyColumnForbid                            = "ddl.alter.modify_column.forbid"
	ruleIDAlterRenameIndexForbid                             = "ddl.alter.rename_index.forbid"
	ruleIDAlterModifyColumnTargetTypeFamilyAllowlist         = "ddl.alter.modify_column.target_type_family.allowlist"
	ruleIDAlterChangeColumnTargetTypeFamilyAllowlist         = "ddl.alter.change_column.target_type_family.allowlist"
	ruleIDAlterModifyColumnExplicitNullabilityChangeForbid   = "ddl.alter.modify_column.explicit_nullability_change.forbid"
	ruleIDAlterChangeColumnExplicitNullabilityChangeForbid   = "ddl.alter.change_column.explicit_nullability_change.forbid"
	ruleIDAlterModifyColumnExplicitDefaultChangeForbid       = "ddl.alter.modify_column.explicit_default_change.forbid"
	ruleIDAlterChangeColumnExplicitDefaultChangeForbid       = "ddl.alter.change_column.explicit_default_change.forbid"
	ruleIDAlterModifyColumnExplicitAutoIncrementChangeForbid = "ddl.alter.modify_column.explicit_auto_increment_change.forbid"
	ruleIDAlterChangeColumnExplicitAutoIncrementChangeForbid = "ddl.alter.change_column.explicit_auto_increment_change.forbid"
	ruleIDAlterAddIndexColumnsMaxCount                       = "ddl.alter.add_index.columns.max_count"
	ruleIDAlterAddIndexDuplicateForbid                       = "ddl.alter.add_index.duplicate.forbid"
	ruleIDAlterAddIndexRedundantLeftPrefixForbid             = "ddl.alter.add_index.redundant_left_prefix.forbid"
	ruleIDAlterAddIndexRedundantUniqueOverlapForbid          = "ddl.alter.add_index.redundant_unique_overlap.forbid"
	ruleIDAlterAddIndexUniquePrefixRequire                   = "ddl.alter.add_index.unique.prefix.require"
	ruleIDAlterAddIndexSecondaryPrefixRequire                = "ddl.alter.add_index.secondary.prefix.require"
	ruleIDAlterAddIndexFulltextPrefixRequire                 = "ddl.alter.add_index.fulltext.prefix.require"
	ruleIDTableCommentMaxLength                              = "ddl.table.comment.max_length"
	ruleIDTableEngineAllowlist                               = "ddl.table.engine.allowlist"
	ruleIDTableCharsetAllowlist                              = "ddl.table.charset.allowlist"
	ruleIDTableRowFormatAllowlist                            = "ddl.table.row_format.allowlist"
	ruleIDTableAutoIncrementInitValueRequire                 = "ddl.table.auto_increment.init_value.require"
	ruleIDTableRowSizeMaxBytesRequire                        = "ddl.table.row_size.max_bytes.require"
	ruleIDIndexKeyLengthMaxBytesRequire                      = "ddl.index.key_length.max_bytes.require"
	ruleIDTableForeignKeyForbid                              = "ddl.table.foreign_key.forbid"
	ruleIDTablePartitionForbid                               = "ddl.table.partition.forbid"
	ruleIDTableCreateLikeForbid                              = "ddl.table.create_like.forbid"
	ruleIDTableCreateAsForbid                                = "ddl.table.create_as.forbid"
	ruleIDPrimaryKeyBigintRequire                            = "ddl.table.primary_key.bigint.require"
	ruleIDPrimaryKeyUnsignedRequire                          = "ddl.table.primary_key.unsigned.require"
	ruleIDPrimaryKeyAutoIncrementRequire                     = "ddl.table.primary_key.auto_increment.require"
	ruleIDPrimaryKeyNotNullRequire                           = "ddl.table.primary_key.not_null.require"
	ruleIDTableExistsCreateForbid                            = "ddl.table.exists.create.forbid"
	ruleIDTableExistsAlterRequire                            = "ddl.table.exists.alter.require"
	ruleIDAlterAddColumnExistsForbid                         = "ddl.alter.add_column.exists.forbid"
	ruleIDAlterDropColumnExistsRequire                       = "ddl.alter.drop_column.exists.require"
	ruleIDAlterModifyColumnExistsRequire                     = "ddl.alter.modify_column.exists.require"
	ruleIDAlterChangeColumnExistsRequire                     = "ddl.alter.change_column.exists.require"
	ruleIDAlterRenameColumnExistsRequire                     = "ddl.alter.rename_column.exists.require"
	ruleIDAlterAddIndexExistsForbid                          = "ddl.alter.add_index.exists.forbid"
	ruleIDAlterDropIndexExistsRequire                        = "ddl.alter.drop_index.exists.require"
	ruleIDAlterRenameIndexExistsRequire                      = "ddl.alter.rename_index.exists.require"
	ruleIDAlterDropPrimaryKeyExistsRequire                   = "ddl.alter.drop_primary_key.exists.require"
	ruleIDAlterModifyColumnCompatibilityRequire              = "ddl.alter.modify_column.compatibility.require"
	ruleIDAlterChangeColumnCompatibilityRequire              = "ddl.alter.change_column.compatibility.require"
	ruleIDAlterTableOptionCompatibilityRequire              = "ddl.alter.table_option.compatibility.require"
	ruleIDViewCreateForbid                                   = "ddl.view.create.forbid"
	ruleIDTableDropForbid                                    = "ddl.table.drop.forbid"
	ruleIDTableDropExistsRequire                             = "ddl.table.drop.exists.require"
	ruleIDTableDropAdaptiveHashWarn                          = "ddl.table.drop.adaptive_hash.warn"
	ruleIDTableDropRowsMaxCount                              = "ddl.table.drop.rows.max_count"
	ruleIDTableTruncateForbid                                = "ddl.table.truncate.forbid"
	ruleIDTableTruncateExistsRequire                         = "ddl.table.truncate.exists.require"
	ruleIDTableTruncateAdaptiveHashWarn                      = "ddl.table.truncate.adaptive_hash.warn"
	ruleIDTableTruncateRowsMaxCount                          = "ddl.table.truncate.rows.max_count"
	ruleIDAlterMergeMySQLRequire                             = "ddl.alter.merge.mysql.require"
	ruleIDAlterMergeTiDBRequire                              = "ddl.alter.merge.tidb.require"
	ruleIDTableDenylistForbid                                = "ddl.table.denylist.forbid"
)

func appliesToCreateTable(statement spec.Statement) bool {
	if statement.Kind != spec.KindDDL || statement.DDL == nil || statement.DDL.Table == nil {
		return false
	}
	switch statement.DDL.Operation {
	case "", spec.DDLOperationUnknown:
		return len(statement.DDL.Alter) == 0
	case spec.DDLOperationCreateTable:
		return true
	default:
		return false
	}
}

func appliesToCreateTableColumns(statement spec.Statement) bool {
	return appliesToCreateTable(statement) && statement.DDL != nil
}

func appliesToCreateTableIndexes(statement spec.Statement) bool {
	return appliesToCreateTable(statement) && statement.DDL != nil
}

func appliesToAlterTable(statement spec.Statement) bool {
	if statement.Kind != spec.KindDDL || statement.DDL == nil || statement.DDL.Table == nil || len(statement.DDL.Alter) == 0 {
		return false
	}
	switch statement.DDL.Operation {
	case "", spec.DDLOperationUnknown:
		return true
	case spec.DDLOperationAlterTable:
		return true
	default:
		return false
	}
}

func appliesToCreateView(statement spec.Statement) bool {
	return statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Table != nil &&
		statement.DDL.Operation == spec.DDLOperationCreateView
}

func appliesToDropTable(statement spec.Statement) bool {
	return statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Table != nil &&
		statement.DDL.Operation == spec.DDLOperationDropTable
}

func appliesToTruncateTable(statement spec.Statement) bool {
	return statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Table != nil &&
		statement.DDL.Operation == spec.DDLOperationTruncateTable
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

func alterColumnChange(alter spec.Alter) (*spec.AlterColumnChange, bool) {
	if alter.Column == nil || alter.Column.Change == nil {
		return nil, false
	}
	return alter.Column.Change, true
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

func targetTableSnapshot(statement spec.Statement) (*spec.TableSnapshot, bool) {
	if statement.Metadata == nil || statement.Metadata.TargetTable == nil {
		return nil, false
	}
	return statement.Metadata.TargetTable, true
}

func alterTouchesExplicitNullability(alter spec.Alter) bool {
	change, ok := alterColumnChange(alter)
	return ok && change.TouchesNullability
}

func alterTouchesExplicitDefault(alter spec.Alter) bool {
	change, ok := alterColumnChange(alter)
	return ok && change.TouchesDefault
}

func alterTouchesExplicitAutoIncrement(alter spec.Alter) bool {
	change, ok := alterColumnChange(alter)
	return ok && change.TouchesAutoIncrement
}

func alterRenamesColumn(alter spec.Alter) bool {
	if alter.Column == nil || alter.Column.OldName == "" || alter.Column.Definition == nil || alter.Column.Definition.Name == "" {
		return false
	}
	return !strings.EqualFold(alter.Column.OldName, alter.Column.Definition.Name)
}

func alterTargetColumnTypeFamily(alter spec.Alter) (string, bool) {
	column, ok := alterColumnDefinition(alter)
	if !ok || strings.TrimSpace(column.Type) == "" {
		return "", false
	}
	return columnTypeFamily(*column), true
}

func alterAddedIndexesByKind(statement spec.Statement, kind spec.IndexKind) []spec.Index {
	if !appliesToAlterActions(statement, "add_constraint") {
		return nil
	}

	indexes := make([]spec.Index, 0)
	for _, alter := range matchingAlterActions(statement, "add_constraint") {
		index, ok := alterIndexDefinition(alter)
		if !ok || index.Kind != kind {
			continue
		}
		indexes = append(indexes, *index)
	}
	return indexes
}

func projectedAlterIndexesStatement(statement spec.Statement, indexes []spec.Index) spec.Statement {
	projected := statement
	projected.DDL = &spec.DDL{
		Table:   statement.DDL.Table,
		Indexes: indexes,
	}
	return projected
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

func integerTypeRank(column spec.Column) int {
	switch baseType(column) {
	case "tinyint":
		return 1
	case "smallint":
		return 2
	case "mediumint":
		return 3
	case "int", "integer":
		return 4
	case "bigint":
		return 5
	default:
		return 0
	}
}
