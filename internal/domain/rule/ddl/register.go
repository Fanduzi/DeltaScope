// Package ddl defines Tier-1 DDL rules.
// input: domain policy values and a shared rule registry
// output: deterministic registration of shipped create-table and alter-table DDL rule batches
// pos: DDL rule assembly entrypoint for application wiring
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/policy"
	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
	"github.com/Fanduzi/DeltaScope/internal/domain/spec"
)

// Register installs the first Tier-1 DDL rule batch into the shared registry.
func Register(registry *rule.Registry, cfg policy.Policy) error {
	for _, factory := range []struct {
		ruleID    string
		construct func(policy.RulePolicy) (rule.StatementRule, error)
	}{
		{ruleID: ruleIDTableCommentRequired, construct: newTableCommentRequiredRule},
		{ruleID: ruleIDTableNameMaxLength, construct: newTableNameMaxLengthRule},
		{ruleID: ruleIDTableNamePrefixRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newNamingPrefixRule(ruleIDTableNamePrefixRequire, "table", rule.LevelWarning, cfg, selectTableName)
		}},
		{ruleID: ruleIDTableNameSuffixRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newNamingSuffixRule(ruleIDTableNameSuffixRequire, "table", rule.LevelWarning, cfg, selectTableName)
		}},
		{ruleID: ruleIDTableNameContainsRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newNamingContainsRule(ruleIDTableNameContainsRequire, "table", rule.LevelWarning, cfg, selectTableName)
		}},
		{ruleID: ruleIDPrimaryKeyRequired, construct: newPrimaryKeyRequiredRule},
		{ruleID: ruleIDPrimaryKeyColumnsMaxCount, construct: newPrimaryKeyColumnCountRule},
		{ruleID: ruleIDPrimaryKeyBigintRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newSinglePrimaryKeyColumnRule(ruleIDPrimaryKeyBigintRequire, rule.LevelBlocker, "must use bigint", "change the primary key column type to bigint", func(column spec.Column) bool {
				return baseType(column) == "bigint"
			}, cfg)
		}},
		{ruleID: ruleIDPrimaryKeyUnsignedRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newSinglePrimaryKeyColumnRule(ruleIDPrimaryKeyUnsignedRequire, rule.LevelBlocker, "must be unsigned", "mark the primary key column as UNSIGNED", func(column spec.Column) bool {
				return column.Unsigned
			}, cfg)
		}},
		{ruleID: ruleIDPrimaryKeyAutoIncrementRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newSinglePrimaryKeyColumnRule(ruleIDPrimaryKeyAutoIncrementRequire, rule.LevelBlocker, "must use auto_increment", "add AUTO_INCREMENT to the primary key column", func(column spec.Column) bool {
				return column.AutoIncrement
			}, cfg)
		}},
		{ruleID: ruleIDPrimaryKeyNotNullRequire, construct: newPrimaryKeyNotNullRule},
		{ruleID: ruleIDTableColumnsMinCount, construct: newTableColumnsMinCountRule},
		{ruleID: ruleIDTableAuditColumnsRequire, construct: newTableAuditColumnsRequiredRule},
		{ruleID: ruleIDColumnCommentRequire, construct: newColumnCommentRequiredRule},
		{ruleID: ruleIDColumnNameMaxLength, construct: newColumnNameMaxLengthRule},
		{ruleID: ruleIDColumnNamePrefixRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newNamingPrefixRule(ruleIDColumnNamePrefixRequire, "column", rule.LevelWarning, cfg, selectColumnNames)
		}},
		{ruleID: ruleIDColumnNameSuffixRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newNamingSuffixRule(ruleIDColumnNameSuffixRequire, "column", rule.LevelWarning, cfg, selectColumnNames)
		}},
		{ruleID: ruleIDColumnNameContainsRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newNamingContainsRule(ruleIDColumnNameContainsRequire, "column", rule.LevelWarning, cfg, selectColumnNames)
		}},
		{ruleID: ruleIDTableNamePatternRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newIdentifierPatternRule(ruleIDTableNamePatternRequire, "table", rule.LevelBlocker, cfg, selectTableName)
		}},
		{ruleID: ruleIDColumnNamePatternRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newIdentifierPatternRule(ruleIDColumnNamePatternRequire, "column", rule.LevelBlocker, cfg, selectColumnNames)
		}},
		{ruleID: ruleIDIndexNamePatternRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newIdentifierPatternRule(ruleIDIndexNamePatternRequire, "index", rule.LevelBlocker, cfg, selectIndexNames)
		}},
		{ruleID: ruleIDTableNameKeywordForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newIdentifierKeywordRule(ruleIDTableNameKeywordForbid, "table", rule.LevelBlocker, cfg, selectTableName)
		}},
		{ruleID: ruleIDColumnNameKeywordForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newIdentifierKeywordRule(ruleIDColumnNameKeywordForbid, "column", rule.LevelBlocker, cfg, selectColumnNames)
		}},
		{ruleID: ruleIDIndexNameKeywordForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newIdentifierKeywordRule(ruleIDIndexNameKeywordForbid, "index", rule.LevelBlocker, cfg, selectIndexNames)
		}},
		{ruleID: ruleIDConstraintPrimaryKeyNamePrefixRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newNamingPrefixRule(ruleIDConstraintPrimaryKeyNamePrefixRequire, "primary key constraint", rule.LevelWarning, cfg, selectPrimaryKeyConstraintNames)
		}},
		{ruleID: ruleIDConstraintPrimaryKeyNameSuffixRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newNamingSuffixRule(ruleIDConstraintPrimaryKeyNameSuffixRequire, "primary key constraint", rule.LevelWarning, cfg, selectPrimaryKeyConstraintNames)
		}},
		{ruleID: ruleIDConstraintPrimaryKeyNameContainsRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newNamingContainsRule(ruleIDConstraintPrimaryKeyNameContainsRequire, "primary key constraint", rule.LevelWarning, cfg, selectPrimaryKeyConstraintNames)
		}},
		{ruleID: ruleIDConstraintUniqueKeyNamePrefixRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newNamingPrefixRule(ruleIDConstraintUniqueKeyNamePrefixRequire, "unique key constraint", rule.LevelWarning, cfg, selectUniqueConstraintNames)
		}},
		{ruleID: ruleIDConstraintUniqueKeyNameSuffixRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newNamingSuffixRule(ruleIDConstraintUniqueKeyNameSuffixRequire, "unique key constraint", rule.LevelWarning, cfg, selectUniqueConstraintNames)
		}},
		{ruleID: ruleIDConstraintUniqueKeyNameContainsRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newNamingContainsRule(ruleIDConstraintUniqueKeyNameContainsRequire, "unique key constraint", rule.LevelWarning, cfg, selectUniqueConstraintNames)
		}},
		{ruleID: ruleIDConstraintForeignKeyNamePrefixRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newNamingPrefixRule(ruleIDConstraintForeignKeyNamePrefixRequire, "foreign key constraint", rule.LevelWarning, cfg, selectForeignKeyConstraintNames)
		}},
		{ruleID: ruleIDConstraintForeignKeyNameSuffixRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newNamingSuffixRule(ruleIDConstraintForeignKeyNameSuffixRequire, "foreign key constraint", rule.LevelWarning, cfg, selectForeignKeyConstraintNames)
		}},
		{ruleID: ruleIDConstraintForeignKeyNameContainsRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newNamingContainsRule(ruleIDConstraintForeignKeyNameContainsRequire, "foreign key constraint", rule.LevelWarning, cfg, selectForeignKeyConstraintNames)
		}},
		{ruleID: ruleIDConstraintCheckNamePrefixRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newNamingPrefixRule(ruleIDConstraintCheckNamePrefixRequire, "check constraint", rule.LevelWarning, cfg, selectCheckConstraintNames, appliesToCreateTableOrAlterCheckConstraint)
		}},
		{ruleID: ruleIDConstraintCheckNameSuffixRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newNamingSuffixRule(ruleIDConstraintCheckNameSuffixRequire, "check constraint", rule.LevelWarning, cfg, selectCheckConstraintNames, appliesToCreateTableOrAlterCheckConstraint)
		}},
		{ruleID: ruleIDConstraintCheckNameContainsRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newNamingContainsRule(ruleIDConstraintCheckNameContainsRequire, "check constraint", rule.LevelWarning, cfg, selectCheckConstraintNames, appliesToCreateTableOrAlterCheckConstraint)
		}},
		{ruleID: ruleIDColumnVarcharMaxLength, construct: newColumnVarcharMaxLengthRule},
		{ruleID: ruleIDColumnDefaultRequire, construct: newColumnDefaultRequiredRule},
		{ruleID: ruleIDColumnNotNullRequire, construct: newColumnNotNullRequiredRule},
		{ruleID: ruleIDColumnFloatDoubleForbid, construct: newColumnFloatDoubleForbiddenRule},
		{ruleID: ruleIDColumnBlobTextForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newColumnTypeForbiddenRule(ruleIDColumnBlobTextForbid, "blob/text", rule.LevelWarning, "switch to varchar or relax the blob/text policy intentionally", func(column spec.Column) bool {
				return isBlobTextLike(column) && !strings.EqualFold(baseType(column), "json")
			}, cfg)
		}},
		{ruleID: ruleIDColumnJSONForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newColumnTypeForbiddenRule(ruleIDColumnJSONForbid, "json", rule.LevelWarning, "store structured data in relational columns or relax the json policy intentionally", func(column spec.Column) bool {
				return strings.EqualFold(baseType(column), "json")
			}, cfg)
		}},
		{ruleID: ruleIDColumnBitForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newColumnTypeForbiddenRule(ruleIDColumnBitForbid, "bit", rule.LevelWarning, "use integer or boolean-friendly types instead of bit", func(column spec.Column) bool {
				return strings.EqualFold(baseType(column), "bit")
			}, cfg)
		}},
		{ruleID: ruleIDColumnTimestampForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newColumnTypeForbiddenRule(ruleIDColumnTimestampForbid, "timestamp", rule.LevelWarning, "prefer datetime unless the team intentionally allows timestamp columns", func(column spec.Column) bool {
				return strings.EqualFold(baseType(column), "timestamp")
			}, cfg)
		}},
		{ruleID: ruleIDColumnCharMaxLength, construct: newColumnCharMaxLengthRule},
		{ruleID: ruleIDColumnCharsetAllowlist, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newColumnValueAllowlistRule(ruleIDColumnCharsetAllowlist, "charset", []string{"utf8", "utf8mb4"}, rule.LevelBlocker, cfg)
		}},
		{ruleID: ruleIDColumnCollationAllowlist, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newColumnValueAllowlistRule(ruleIDColumnCollationAllowlist, "collation", []string{"utf8_general_ci", "utf8mb4_general_ci", "utf8mb4_bin"}, rule.LevelBlocker, cfg)
		}},
		{ruleID: ruleIDColumnCharsetCollationMatchRequire, construct: newColumnCharsetCollationMatchRule},
		{ruleID: ruleIDIndexTotalMaxCount, construct: newIndexTotalMaxCountRule},
		{ruleID: ruleIDIndexColumnsMaxCount, construct: newIndexColumnsMaxCountRule},
		{ruleID: ruleIDIndexUniquePrefixRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newIndexPrefixRequiredRule(ruleIDIndexUniquePrefixRequire, spec.IndexKindUnique, "uniq_", rule.LevelWarning, cfg)
		}},
		{ruleID: ruleIDIndexUniqueSuffixRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newIndexSuffixRequiredRule(ruleIDIndexUniqueSuffixRequire, spec.IndexKindUnique, rule.LevelWarning, cfg)
		}},
		{ruleID: ruleIDIndexUniqueContainsRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newIndexContainsRequiredRule(ruleIDIndexUniqueContainsRequire, spec.IndexKindUnique, rule.LevelWarning, cfg)
		}},
		{ruleID: ruleIDIndexSecondaryPrefixRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newIndexPrefixRequiredRule(ruleIDIndexSecondaryPrefixRequire, spec.IndexKindSecondary, "idx_", rule.LevelWarning, cfg)
		}},
		{ruleID: ruleIDIndexSecondarySuffixRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newIndexSuffixRequiredRule(ruleIDIndexSecondarySuffixRequire, spec.IndexKindSecondary, rule.LevelWarning, cfg)
		}},
		{ruleID: ruleIDIndexSecondaryContainsRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newIndexContainsRequiredRule(ruleIDIndexSecondaryContainsRequire, spec.IndexKindSecondary, rule.LevelWarning, cfg)
		}},
		{ruleID: ruleIDIndexFulltextPrefixRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newIndexPrefixRequiredRule(ruleIDIndexFulltextPrefixRequire, spec.IndexKindFulltext, "full_", rule.LevelWarning, cfg)
		}},
		{ruleID: ruleIDIndexFulltextSuffixRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newIndexSuffixRequiredRule(ruleIDIndexFulltextSuffixRequire, spec.IndexKindFulltext, rule.LevelWarning, cfg)
		}},
		{ruleID: ruleIDIndexFulltextContainsRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newIndexContainsRequiredRule(ruleIDIndexFulltextContainsRequire, spec.IndexKindFulltext, rule.LevelWarning, cfg)
		}},
		{ruleID: ruleIDIndexDuplicateForbid, construct: newDuplicateIndexForbiddenRule},
		{ruleID: ruleIDIndexRedundantLeftPrefixForbid, construct: newRedundantLeftPrefixIndexRule},
		{ruleID: ruleIDIndexRedundantUniqueOverlapForbid, construct: newRedundantUniqueOverlapIndexRule},
		{ruleID: ruleIDAlterDropColumnForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newForbiddenAlterActionRule(ruleIDAlterDropColumnForbid, "drop_column", "drop column", rule.LevelWarning, cfg)
		}},
		{ruleID: ruleIDAlterDropPrimaryKeyForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newForbiddenAlterActionRule(ruleIDAlterDropPrimaryKeyForbid, "drop_primary_key", "drop primary key", rule.LevelBlocker, cfg)
		}},
		{ruleID: ruleIDAlterDropIndexForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newForbiddenAlterActionRule(ruleIDAlterDropIndexForbid, "drop_index", "drop index", rule.LevelWarning, cfg)
		}},
		{ruleID: ruleIDAlterRenameTableForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newForbiddenAlterActionRule(ruleIDAlterRenameTableForbid, "rename_table", "rename table", rule.LevelBlocker, cfg)
		}},
		{ruleID: ruleIDAlterRenameColumnForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newForbiddenAlterActionRule(ruleIDAlterRenameColumnForbid, "rename_column", "rename column", rule.LevelBlocker, cfg)
		}},
		{ruleID: ruleIDAlterRenameIndexForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newForbiddenAlterRenameRule(ruleIDAlterRenameIndexForbid, "rename_index", "rename index", rule.LevelBlocker, cfg)
		}},
		{ruleID: ruleIDAlterAddIndexColumnsMaxCount, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newAlterAddedIndexColumnsMaxCountRule(8, rule.LevelWarning, cfg)
		}},
		{ruleID: ruleIDAlterAddIndexDuplicateForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newAlterAddedDuplicateIndexForbiddenRule(rule.LevelWarning, cfg)
		}},
		{ruleID: ruleIDAlterAddIndexRedundantLeftPrefixForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newAlterAddedRedundantLeftPrefixRule(rule.LevelWarning, cfg)
		}},
		{ruleID: ruleIDAlterAddIndexRedundantUniqueOverlapForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newAlterAddedRedundantUniqueOverlapRule(rule.LevelWarning, cfg)
		}},
		{ruleID: ruleIDAlterAddIndexUniquePrefixRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newAlterAddedIndexPrefixRule(ruleIDAlterAddIndexUniquePrefixRequire, spec.IndexKindUnique, "uniq_", rule.LevelWarning, cfg)
		}},
		{ruleID: ruleIDAlterAddIndexUniqueSuffixRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newAlterAddedIndexSuffixRule(ruleIDAlterAddIndexUniqueSuffixRequire, spec.IndexKindUnique, rule.LevelWarning, cfg)
		}},
		{ruleID: ruleIDAlterAddIndexUniqueContainsRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newAlterAddedIndexContainsRule(ruleIDAlterAddIndexUniqueContainsRequire, spec.IndexKindUnique, rule.LevelWarning, cfg)
		}},
		{ruleID: ruleIDAlterAddIndexSecondaryPrefixRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newAlterAddedIndexPrefixRule(ruleIDAlterAddIndexSecondaryPrefixRequire, spec.IndexKindSecondary, "idx_", rule.LevelWarning, cfg)
		}},
		{ruleID: ruleIDAlterAddIndexSecondarySuffixRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newAlterAddedIndexSuffixRule(ruleIDAlterAddIndexSecondarySuffixRequire, spec.IndexKindSecondary, rule.LevelWarning, cfg)
		}},
		{ruleID: ruleIDAlterAddIndexSecondaryContainsRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newAlterAddedIndexContainsRule(ruleIDAlterAddIndexSecondaryContainsRequire, spec.IndexKindSecondary, rule.LevelWarning, cfg)
		}},
		{ruleID: ruleIDAlterAddIndexFulltextPrefixRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newAlterAddedIndexPrefixRule(ruleIDAlterAddIndexFulltextPrefixRequire, spec.IndexKindFulltext, "full_", rule.LevelWarning, cfg)
		}},
		{ruleID: ruleIDAlterAddIndexFulltextSuffixRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newAlterAddedIndexSuffixRule(ruleIDAlterAddIndexFulltextSuffixRequire, spec.IndexKindFulltext, rule.LevelWarning, cfg)
		}},
		{ruleID: ruleIDAlterAddIndexFulltextContainsRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newAlterAddedIndexContainsRule(ruleIDAlterAddIndexFulltextContainsRequire, spec.IndexKindFulltext, rule.LevelWarning, cfg)
		}},
		{ruleID: ruleIDAlterChangeColumnForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newForbiddenAlterActionRule(ruleIDAlterChangeColumnForbid, "change_column", "change column", rule.LevelBlocker, cfg)
		}},
		{ruleID: ruleIDAlterModifyColumnForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newForbiddenAlterActionRule(ruleIDAlterModifyColumnForbid, "modify_column", "modify column", rule.LevelWarning, cfg)
		}},
		{ruleID: ruleIDAlterSetDataTypeForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newForbiddenAlterActionRule(ruleIDAlterSetDataTypeForbid, "set_data_type", "set data type", rule.LevelWarning, cfg, withDialectAllowlist(spec.DialectPostgreSQL))
		}},
		{ruleID: ruleIDAlterSetDefaultForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newForbiddenAlterActionRule(ruleIDAlterSetDefaultForbid, "set_default", "set default", rule.LevelWarning, cfg, withDialectAllowlist(spec.DialectPostgreSQL))
		}},
		{ruleID: ruleIDAlterDropDefaultForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newForbiddenAlterActionRule(ruleIDAlterDropDefaultForbid, "drop_default", "drop default", rule.LevelWarning, cfg, withDialectAllowlist(spec.DialectPostgreSQL))
		}},
		{ruleID: ruleIDAlterSetNotNullForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newForbiddenAlterActionRule(ruleIDAlterSetNotNullForbid, "set_not_null", "set not null", rule.LevelWarning, cfg, withDialectAllowlist(spec.DialectPostgreSQL))
		}},
		{ruleID: ruleIDAlterDropNotNullForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newForbiddenAlterActionRule(ruleIDAlterDropNotNullForbid, "drop_not_null", "drop not null", rule.LevelWarning, cfg, withDialectAllowlist(spec.DialectPostgreSQL))
		}},
		{ruleID: ruleIDAlterDropExpressionForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newForbiddenAlterActionRule(ruleIDAlterDropExpressionForbid, "drop_expression", "drop expression", rule.LevelWarning, cfg, withDialectAllowlist(spec.DialectPostgreSQL))
		}},
		{ruleID: ruleIDAlterSetGeneratedForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newForbiddenAlterActionRule(ruleIDAlterSetGeneratedForbid, "set_generated", "set generated", rule.LevelWarning, cfg, withDialectAllowlist(spec.DialectPostgreSQL))
		}},
		{ruleID: ruleIDAlterDropIdentityForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newForbiddenAlterActionRule(ruleIDAlterDropIdentityForbid, "drop_identity", "drop identity", rule.LevelWarning, cfg, withDialectAllowlist(spec.DialectPostgreSQL))
		}},
		{ruleID: ruleIDAlterModifyColumnTargetTypeFamilyAllowlist, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newAlterTargetTypeFamilyRule(ruleIDAlterModifyColumnTargetTypeFamilyAllowlist, "modify_column", "modify column", rule.LevelBlocker, defaultConservativeAlterTypeFamilies, cfg)
		}},
		{ruleID: ruleIDAlterChangeColumnTargetTypeFamilyAllowlist, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newAlterTargetTypeFamilyRule(ruleIDAlterChangeColumnTargetTypeFamilyAllowlist, "change_column", "change column", rule.LevelBlocker, defaultConservativeAlterTypeFamilies, cfg)
		}},
		{ruleID: ruleIDAlterModifyColumnCompatibilityRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newAlterColumnCompatibilityRule(ruleIDAlterModifyColumnCompatibilityRequire, "modify_column", "modify column", rule.LevelBlocker, cfg)
		}},
		{ruleID: ruleIDAlterChangeColumnCompatibilityRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newAlterColumnCompatibilityRule(ruleIDAlterChangeColumnCompatibilityRequire, "change_column", "change column", rule.LevelBlocker, cfg)
		}},
		{ruleID: ruleIDAlterTableOptionCompatibilityRequire, construct: newAlterTableOptionCompatibilityRule},
		{ruleID: ruleIDAlterModifyColumnExplicitNullabilityChangeForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newForbiddenExplicitAlterColumnChangeRule(ruleIDAlterModifyColumnExplicitNullabilityChangeForbid, "modify_column", "modify column", "explicit_nullability_change", rule.LevelBlocker, alterTouchesExplicitNullability, cfg)
		}},
		{ruleID: ruleIDAlterChangeColumnExplicitNullabilityChangeForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newForbiddenExplicitAlterColumnChangeRule(ruleIDAlterChangeColumnExplicitNullabilityChangeForbid, "change_column", "change column", "explicit_nullability_change", rule.LevelBlocker, alterTouchesExplicitNullability, cfg)
		}},
		{ruleID: ruleIDAlterModifyColumnExplicitDefaultChangeForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newForbiddenExplicitAlterColumnChangeRule(ruleIDAlterModifyColumnExplicitDefaultChangeForbid, "modify_column", "modify column", "explicit_default_change", rule.LevelBlocker, alterTouchesExplicitDefault, cfg)
		}},
		{ruleID: ruleIDAlterChangeColumnExplicitDefaultChangeForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newForbiddenExplicitAlterColumnChangeRule(ruleIDAlterChangeColumnExplicitDefaultChangeForbid, "change_column", "change column", "explicit_default_change", rule.LevelBlocker, alterTouchesExplicitDefault, cfg)
		}},

		// PG-native explicit default change semantic rules (action-space isolated).
		{ruleID: ruleIDAlterSetDefaultExplicitDefaultChangeForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newForbiddenExplicitAlterColumnChangeRule(ruleIDAlterSetDefaultExplicitDefaultChangeForbid, "set_default", "set default", "explicit_default_change", rule.LevelBlocker, alterTouchesExplicitDefault, cfg)
		}},
		{ruleID: ruleIDAlterDropDefaultExplicitDefaultChangeForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newForbiddenExplicitAlterColumnChangeRule(ruleIDAlterDropDefaultExplicitDefaultChangeForbid, "drop_default", "drop default", "explicit_default_change", rule.LevelBlocker, alterTouchesExplicitDefault, cfg)
		}},

		// PG-native explicit nullability change semantic rules (action-space isolated).
		{ruleID: ruleIDAlterSetNotNullExplicitNullabilityChangeForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newForbiddenExplicitAlterColumnChangeRule(ruleIDAlterSetNotNullExplicitNullabilityChangeForbid, "set_not_null", "set not null", "explicit_nullability_change", rule.LevelBlocker, alterTouchesExplicitNullability, cfg)
		}},
		{ruleID: ruleIDAlterDropNotNullExplicitNullabilityChangeForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newForbiddenExplicitAlterColumnChangeRule(ruleIDAlterDropNotNullExplicitNullabilityChangeForbid, "drop_not_null", "drop not null", "explicit_nullability_change", rule.LevelBlocker, alterTouchesExplicitNullability, cfg)
		}},
		{ruleID: ruleIDAlterModifyColumnExplicitAutoIncrementChangeForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newForbiddenExplicitAlterColumnChangeRule(ruleIDAlterModifyColumnExplicitAutoIncrementChangeForbid, "modify_column", "modify column", "explicit_auto_increment_change", rule.LevelBlocker, alterTouchesExplicitAutoIncrement, cfg)
		}},
		{ruleID: ruleIDAlterChangeColumnExplicitAutoIncrementChangeForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newForbiddenExplicitAlterColumnChangeRule(ruleIDAlterChangeColumnExplicitAutoIncrementChangeForbid, "change_column", "change column", "explicit_auto_increment_change", rule.LevelBlocker, alterTouchesExplicitAutoIncrement, cfg)
		}},
		{ruleID: ruleIDTableCommentMaxLength, construct: newTableCommentMaxLengthRule},
		{ruleID: ruleIDTableEngineAllowlist, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newTableOptionAllowlistRule(ruleIDTableEngineAllowlist, "engine", "engine", []string{"InnoDB"}, rule.LevelBlocker, cfg)
		}},
		{ruleID: ruleIDTableRowSizeMaxBytesRequire, construct: newTableRowSizeRule},
		{ruleID: ruleIDTableCharsetAllowlist, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newTableOptionAllowlistRule(ruleIDTableCharsetAllowlist, "charset", "charset", []string{"utf8", "utf8mb4"}, rule.LevelBlocker, cfg)
		}},
		{ruleID: ruleIDTableRowFormatAllowlist, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newTableOptionAllowlistRule(ruleIDTableRowFormatAllowlist, "row_format", "row format", []string{"DYNAMIC"}, rule.LevelBlocker, cfg)
		}},
		{ruleID: ruleIDIndexKeyLengthMaxBytesRequire, construct: newIndexKeyLengthRule},
		{ruleID: ruleIDTableAutoIncrementInitValueRequire, construct: newTableAutoIncrementInitValueRule},
		{ruleID: ruleIDTableForeignKeyForbid, construct: newTableForeignKeyForbidRule},
		{ruleID: ruleIDTablePartitionForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newTableBooleanShapeRule(ruleIDTablePartitionForbid, "partitioning", rule.LevelBlocker, func(ddl *spec.DDL) bool { return ddl.HasPartition }, cfg)
		}},
		{ruleID: ruleIDTableCreateLikeForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newTableBooleanShapeRule(ruleIDTableCreateLikeForbid, "like", rule.LevelBlocker, func(ddl *spec.DDL) bool { return ddl.HasReferTable }, cfg)
		}},
		{ruleID: ruleIDTableCreateAsForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newTableBooleanShapeRule(ruleIDTableCreateAsForbid, "as select", rule.LevelBlocker, func(ddl *spec.DDL) bool { return ddl.HasSelect }, cfg)
		}},
		{ruleID: ruleIDViewCreateForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newForbiddenDDLOperationRule(ruleIDViewCreateForbid, spec.DDLOperationCreateView, "create view", rule.LevelBlocker, cfg)
		}},
		{ruleID: ruleIDViewDropForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newForbiddenDDLOperationRule(ruleIDViewDropForbid, spec.DDLOperationDropView, "drop view", rule.LevelBlocker, cfg)
		}},
		{ruleID: ruleIDTableDropForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newForbiddenDDLOperationRule(ruleIDTableDropForbid, spec.DDLOperationDropTable, "drop table", rule.LevelBlocker, cfg)
		}},
		{ruleID: ruleIDTableDropExistsRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newTableOperationExistenceRule(ruleIDTableDropExistsRequire, spec.DDLOperationDropTable, "drop table", rule.LevelBlocker, cfg)
		}},
		{ruleID: ruleIDTableDropAdaptiveHashWarn, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newAdaptiveHashLifecycleRule(ruleIDTableDropAdaptiveHashWarn, spec.DDLOperationDropTable, "drop table", rule.LevelWarning, cfg)
		}},
		{ruleID: ruleIDTableDropRowsMaxCount, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newTableRowCountRiskRule(ruleIDTableDropRowsMaxCount, spec.DDLOperationDropTable, "drop table", rule.LevelWarning, cfg)
		}},
		{ruleID: ruleIDTableTruncateForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newForbiddenDDLOperationRule(ruleIDTableTruncateForbid, spec.DDLOperationTruncateTable, "truncate table", rule.LevelBlocker, cfg)
		}},
		{ruleID: ruleIDTableTruncateExistsRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newTableOperationExistenceRule(ruleIDTableTruncateExistsRequire, spec.DDLOperationTruncateTable, "truncate table", rule.LevelBlocker, cfg)
		}},
		{ruleID: ruleIDTableTruncateAdaptiveHashWarn, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newAdaptiveHashLifecycleRule(ruleIDTableTruncateAdaptiveHashWarn, spec.DDLOperationTruncateTable, "truncate table", rule.LevelWarning, cfg)
		}},
		{ruleID: ruleIDTableTruncateRowsMaxCount, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newTableRowCountRiskRule(ruleIDTableTruncateRowsMaxCount, spec.DDLOperationTruncateTable, "truncate table", rule.LevelWarning, cfg)
		}},
		{ruleID: ruleIDTableDenylistForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newTableDenylistRule(ruleIDTableDenylistForbid, rule.LevelBlocker, cfg)
		}},
		{ruleID: ruleIDTableExistsCreateForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newTableExistenceRule(ruleIDTableExistsCreateForbid, false, rule.LevelBlocker, cfg)
		}},
		{ruleID: ruleIDTableExistsAlterRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newTableExistenceRule(ruleIDTableExistsAlterRequire, true, rule.LevelBlocker, cfg)
		}},
		{ruleID: ruleIDAlterAddColumnExistsForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newAlterObjectExistenceRule(ruleIDAlterAddColumnExistsForbid, []string{"add_columns"}, "column", true, rule.LevelBlocker, cfg, alterObjectName, snapshotHasColumn)
		}},
		{ruleID: ruleIDAlterDropColumnExistsRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newAlterObjectExistenceRule(ruleIDAlterDropColumnExistsRequire, []string{"drop_column"}, "column", false, rule.LevelBlocker, cfg, alterObjectName, snapshotHasColumn)
		}},
		{ruleID: ruleIDAlterModifyColumnExistsRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newAlterObjectExistenceRule(ruleIDAlterModifyColumnExistsRequire, []string{"modify_column"}, "column", false, rule.LevelBlocker, cfg, alterObjectName, snapshotHasColumn)
		}},
		{ruleID: ruleIDAlterChangeColumnExistsRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newAlterObjectExistenceRule(ruleIDAlterChangeColumnExistsRequire, []string{"change_column"}, "column", false, rule.LevelBlocker, cfg, alterObjectName, snapshotHasColumn)
		}},
		{ruleID: ruleIDAlterRenameColumnExistsRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newAlterObjectExistenceRule(ruleIDAlterRenameColumnExistsRequire, []string{"rename_column"}, "column", false, rule.LevelBlocker, cfg, alterObjectName, snapshotHasColumn)
		}},
		{ruleID: ruleIDAlterAddIndexExistsForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newAlterObjectExistenceRule(ruleIDAlterAddIndexExistsForbid, []string{"add_constraint"}, "index", true, rule.LevelBlocker, cfg, alterObjectName, snapshotHasIndex)
		}},
		{ruleID: ruleIDAlterDropIndexExistsRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newAlterObjectExistenceRule(ruleIDAlterDropIndexExistsRequire, []string{"drop_index"}, "index", false, rule.LevelBlocker, cfg, alterObjectName, snapshotHasIndex)
		}},
		{ruleID: ruleIDAlterRenameIndexExistsRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newAlterObjectExistenceRule(ruleIDAlterRenameIndexExistsRequire, []string{"rename_index"}, "index", false, rule.LevelBlocker, cfg, alterObjectName, snapshotHasIndex)
		}},
		{ruleID: ruleIDAlterDropPrimaryKeyExistsRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newAlterPrimaryKeyExistenceRule(ruleIDAlterDropPrimaryKeyExistsRequire, rule.LevelBlocker, cfg)
		}},
		// PostgreSQL migration-safety rules (PG-only).
		{ruleID: ruleIDPGCreateIndexConcurrentlyRequire, construct: newCreateIndexConcurrentlyRequiredRule},
		{ruleID: ruleIDPGAlterAddColumnNonNullDefaultRewriteWarn, construct: newAddColumnNonNullDefaultRewriteWarnRule},
		{ruleID: ruleIDPGAlterAddCheckNotValidRequire, construct: newAddCheckNotValidRequiredRule},
		{ruleID: ruleIDPGAlterSetDataTypeRewriteWarn, construct: newSetDataTypeRewriteWarnRule},
		{ruleID: ruleIDPGTableForeignKeyCrossSchemaAdvisory, construct: newTableForeignKeyCrossSchemaAdvisoryRule},
		{ruleID: ruleIDPGDropIndexAdvisory, construct: newDropIndexAdvisoryRule},
		{ruleID: ruleIDPGAlterAddColumnNonNullNoDefaultWarn, construct: newAddColumnNonNullNoDefaultWarnRule},
		{ruleID: ruleIDPGAlterAddUniqueConstraintConcurrentIndexAdvisory, construct: newAddUniqueConstraintAdvisoryRule},
		{ruleID: ruleIDPGAlterDropConstraintAdvisory, construct: newDropConstraintAdvisoryRule},
		// PostgreSQL object lifecycle rules (PG-only).
		{ruleID: ruleIDPGAlterDropColumnAdvisory, construct: newDropColumnAdvisoryRule},
		{ruleID: ruleIDPGAlterValidateConstraintAdvisory, construct: newValidateConstraintAdvisoryRule},
		{ruleID: ruleIDPGAlterAddColumnNullableNotice, construct: newAddColumnNullableNoticeRule},
		{ruleID: ruleIDPGAlterSetSchemaAdvisory, construct: newSetSchemaAdvisoryRule},
		{ruleID: ruleIDPGAlterOwnerAdvisory, construct: newOwnerAdvisoryRule},
		{ruleID: ruleIDPGAlterEnableTriggerNotice, construct: newEnableTriggerNoticeRule},
		{ruleID: ruleIDPGAlterDisableTriggerWarn, construct: newDisableTriggerWarnRule},
		{ruleID: ruleIDPGAlterAttachPartitionAdvisory, construct: newAttachPartitionAdvisoryRule},
		{ruleID: ruleIDPGAlterDetachPartitionWarn, construct: newDetachPartitionWarnRule},
		{ruleID: ruleIDPGAlterLoggedNotice, construct: newSetLoggedNoticeRule},
		{ruleID: ruleIDPGAlterUnloggedNotice, construct: newSetUnloggedNoticeRule},
		{ruleID: ruleIDPGAlterSetTablespaceNotice, construct: newSetTablespaceNoticeRule},
		{ruleID: ruleIDPGAlterSetAccessMethodWarn, construct: newSetAccessMethodWarnRule},
		{ruleID: ruleIDPGAlterEnableReplicaTriggerNotice, construct: newEnableReplicaTriggerNoticeRule},
		{ruleID: ruleIDPGAlterEnableAlwaysTriggerNotice, construct: newEnableAlwaysTriggerNoticeRule},
		{ruleID: ruleIDPGAlterEnableRuleNotice, construct: newEnableRuleNoticeRule},
		{ruleID: ruleIDPGAlterDisableRuleWarn, construct: newDisableRuleWarnRule},
		{ruleID: ruleIDPGAlterEnableReplicaRuleNotice, construct: newEnableReplicaRuleNoticeRule},
		{ruleID: ruleIDPGAlterEnableAlwaysRuleNotice, construct: newEnableAlwaysRuleNoticeRule},
		{ruleID: ruleIDPGAlterSetReloptionsWarn, construct: newSetReloptionsWarnRule},
		{ruleID: ruleIDPGAlterResetReloptionsNotice, construct: newResetReloptionsNoticeRule},
		// PostgreSQL column attribute rules (PG-only).
		{ruleID: ruleIDPGAlterSetColumnStatisticsNotice, construct: newSetColumnStatisticsNoticeRule},
		{ruleID: ruleIDPGAlterSetColumnOptionsNotice, construct: newSetColumnOptionsNoticeRule},
		{ruleID: ruleIDPGAlterResetColumnOptionsNotice, construct: newResetColumnOptionsNoticeRule},
		{ruleID: ruleIDPGAlterSetColumnStorageNotice, construct: newSetColumnStorageNoticeRule},
		{ruleID: ruleIDPGAlterSetColumnCompressionNotice, construct: newSetColumnCompressionNoticeRule},
		{ruleID: ruleIDPGAlterReplicaIdentityFullWarn, construct: newReplicaIdentityFullWarnRule},
		{ruleID: ruleIDPGAlterReplicaIdentityNothingWarn, construct: newReplicaIdentityNothingWarnRule},
		{ruleID: ruleIDPGAlterReplicaIdentityUsingIndexNotice, construct: newReplicaIdentityUsingIndexNoticeRule},
		{ruleID: ruleIDPGDropSchemaAdvisory, construct: newDropSchemaAdvisoryRule},
		{ruleID: ruleIDPGDropSchemaCascadeWarn, construct: newDropSchemaCascadeWarnRule},
		{ruleID: ruleIDPGCreateSequenceCycleWarn, construct: newCreateSequenceCycleWarnRule},
		{ruleID: ruleIDPGAlterSequenceRestartWarn, construct: newAlterSequenceRestartWarnRule},
		{ruleID: ruleIDPGAlterSequenceCycleWarn, construct: newAlterSequenceCycleWarnRule},
		{ruleID: ruleIDPGDropSequenceAdvisory, construct: newDropSequenceAdvisoryRule},
		{ruleID: ruleIDPGDropSequenceCascadeWarn, construct: newDropSequenceCascadeWarnRule},
		{ruleID: ruleIDPGDropMaterializedViewAdvisory, construct: newDropMaterializedViewAdvisoryRule},
		{ruleID: ruleIDPGDropMaterializedViewCascadeWarn, construct: newDropMaterializedViewCascadeWarnRule},
		{ruleID: ruleIDPGRefreshMaterializedViewConcurrentlyWarn, construct: newRefreshMaterializedViewConcurrentlyWarnRule},
		{ruleID: ruleIDPGRefreshMaterializedViewNoDataNotice, construct: newRefreshMaterializedViewNoDataNoticeRule},
		// PostgreSQL type lifecycle rules (PG-only).
		{ruleID: ruleIDPGCreateTypeEnumNotice, construct: newCreateTypeEnumNoticeRule},
		{ruleID: ruleIDPGAlterTypeAddValueAdvisory, construct: newAlterTypeAddValueAdvisoryRule},
		{ruleID: ruleIDPGAlterTypeAddValuePositionNotice, construct: newAlterTypeAddValuePositionNoticeRule},
		{ruleID: ruleIDPGDropTypeAdvisory, construct: newDropTypeAdvisoryRule},
		{ruleID: ruleIDPGDropTypeCascadeWarn, construct: newDropTypeCascadeWarnRule},
		// PostgreSQL composite type lifecycle rules (PG-only).
		{ruleID: ruleIDPGCreateTypeCompositeNotice, construct: newCreateTypeCompositeNoticeRule},
		{ruleID: ruleIDPGAlterTypeCompositeRenameNotice, construct: newAlterTypeCompositeRenameNoticeRule},
		{ruleID: ruleIDPGAlterTypeCompositeSetSchemaNotice, construct: newAlterTypeCompositeSetSchemaNoticeRule},
		// PostgreSQL composite type attribute lifecycle rules (PG-only).
		{ruleID: ruleIDPGAlterTypeAddAttributeNotice, construct: newAlterTypeAddAttributeNoticeRule},
		{ruleID: ruleIDPGAlterTypeDropAttributeWarn, construct: newAlterTypeDropAttributeWarnRule},
		{ruleID: ruleIDPGAlterTypeAlterAttributeTypeWarn, construct: newAlterTypeAlterAttributeTypeWarnRule},
		{ruleID: ruleIDPGAlterTypeRenameAttributeNotice, construct: newAlterTypeRenameAttributeNoticeRule},
		// PostgreSQL domain lifecycle rules (PG-only).
		{ruleID: ruleIDPGCreateDomainNotice, construct: newCreateDomainNoticeRule},
		{ruleID: ruleIDPGAlterDomainConstraintNotice, construct: newAlterDomainConstraintNoticeRule},
		{ruleID: ruleIDPGAlterDomainDefaultNotice, construct: newAlterDomainDefaultNoticeRule},
		{ruleID: ruleIDPGAlterDomainNotNullNotice, construct: newAlterDomainNotNullNoticeRule},
		{ruleID: ruleIDPGAlterDomainRenameNotice, construct: newAlterDomainRenameNoticeRule},
		{ruleID: ruleIDPGDropDomainAdvisory, construct: newDropDomainAdvisoryRule},
		{ruleID: ruleIDPGDropDomainCascadeWarn, construct: newDropDomainCascadeWarnRule},
		// PostgreSQL table privilege rules (PG-only).
		{ruleID: ruleIDPGGrantTablePrivilegeNotice, construct: newGrantTablePrivilegeNoticeRule},
		{ruleID: ruleIDPGGrantTablePrivilegeAllWarn, construct: newGrantTablePrivilegeAllWarnRule},
		{ruleID: ruleIDPGRevokeTablePrivilegeNotice, construct: newRevokeTablePrivilegeNoticeRule},
		{ruleID: ruleIDPGRevokeTablePrivilegeCascadeWarn, construct: newRevokeTablePrivilegeCascadeWarnRule},
		// PostgreSQL extension lifecycle rules (PG-only).
		{ruleID: ruleIDPGCreateExtensionNotice, construct: newCreateExtensionNoticeRule},
		{ruleID: ruleIDPGCreateExtensionCascadeWarn, construct: newCreateExtensionCascadeWarnRule},
		{ruleID: ruleIDPGAlterExtensionUpdateNotice, construct: newAlterExtensionUpdateNoticeRule},
		{ruleID: ruleIDPGAlterExtensionSetSchemaNotice, construct: newAlterExtensionSetSchemaNoticeRule},
		{ruleID: ruleIDPGDropExtensionAdvisory, construct: newDropExtensionAdvisoryRule},
		{ruleID: ruleIDPGDropExtensionCascadeWarn, construct: newDropExtensionCascadeWarnRule},
		{ruleID: ruleIDPGAlterExtensionAddMemberNotice, construct: newAlterExtensionAddMemberNoticeRule},
		{ruleID: ruleIDPGAlterExtensionDropMemberWarn, construct: newAlterExtensionDropMemberWarnRule},
		// MySQL/TiDB database lifecycle rules.
		{ruleID: ruleIDDatabaseCreateNotice, construct: newDatabaseCreateNoticeRule},
		{ruleID: ruleIDDatabaseDropWarn, construct: newDatabaseDropWarnRule},
		// PostgreSQL create schema rule.
		{ruleID: ruleIDPGCreateSchemaNotice, construct: newCreateSchemaNoticeRule},
		// PostgreSQL policy lifecycle rules (PG-only).
		{ruleID: ruleIDPGCreatePolicyNotice, construct: newCreatePolicyNoticeRule},
		{ruleID: ruleIDPGAlterPolicyNotice, construct: newAlterPolicyNoticeRule},
		{ruleID: ruleIDPGDropPolicyWarn, construct: newDropPolicyWarnRule},
		{ruleID: ruleIDPGAlterEnableRLSNotice, construct: newEnableRLSNoticeRule},
		{ruleID: ruleIDPGAlterDisableRLSWarn, construct: newDisableRLSWarnRule},
		{ruleID: ruleIDPGAlterForceRLSNotice, construct: newForceRLSNoticeRule},
		{ruleID: ruleIDPGAlterNoForceRLSNotice, construct: newNoForceRLSNoticeRule},
		// PostgreSQL trigger lifecycle rules (PG-only).
		{ruleID: ruleIDPGCreateTriggerNotice, construct: newCreateTriggerNoticeRule},
		{ruleID: ruleIDPGCreateConstraintTriggerWarn, construct: newCreateConstraintTriggerWarnRule},
		{ruleID: ruleIDPGDropTriggerAdvisory, construct: newDropTriggerAdvisoryRule},
		// PostgreSQL function/procedure lifecycle rules (PG-only).
		{ruleID: ruleIDPGCreateFunctionNotice, construct: newCreateFunctionNoticeRule},
		{ruleID: ruleIDPGCreateFunctionSecurityDefinerWarn, construct: newCreateFunctionSecurityDefinerWarnRule},
		{ruleID: ruleIDPGCreateOrReplaceFunctionAdvisory, construct: newCreateOrReplaceFunctionAdvisoryRule},
		{ruleID: ruleIDPGDropFunctionAdvisory, construct: newDropFunctionAdvisoryRule},
		{ruleID: ruleIDPGCreateProcedureNotice, construct: newCreateProcedureNoticeRule},
		{ruleID: ruleIDPGDropProcedureAdvisory, construct: newDropProcedureAdvisoryRule},
		// PostgreSQL advanced view lifecycle rules (PG-only).
		{ruleID: ruleIDPGCreateOrReplaceViewAdvisory, construct: newCreateOrReplaceViewAdvisoryRule},
		{ruleID: ruleIDPGCreateTempViewNotice, construct: newCreateTempViewNoticeRule},
		{ruleID: ruleIDPGCreateViewCheckOptionNotice, construct: newCreateViewCheckOptionNoticeRule},
		{ruleID: ruleIDPGAlterViewRenameNotice, construct: newAlterViewRenameNoticeRule},
		{ruleID: ruleIDPGAlterViewSetSchemaNotice, construct: newAlterViewSetSchemaNoticeRule},
		{ruleID: ruleIDPGDropViewCascadeWarn, construct: newDropViewCascadeWarnRule},
		// PostgreSQL alter object lifecycle rules (PG-only).
		{ruleID: ruleIDPGAlterSchemaRenameNotice, construct: newAlterSchemaRenameNoticeRule},
		{ruleID: ruleIDPGAlterSchemaOwnerNotice, construct: newAlterSchemaOwnerNoticeRule},
		{ruleID: ruleIDPGAlterIndexRenameNotice, construct: newAlterIndexRenameNoticeRule},
		{ruleID: ruleIDPGAlterIndexSetTablespaceNotice, construct: newAlterIndexSetTablespaceNoticeRule},
		{ruleID: ruleIDPGAlterMaterializedViewRenameNotice, construct: newAlterMaterializedViewRenameNoticeRule},
		{ruleID: ruleIDPGAlterMaterializedViewSetSchemaNotice, construct: newAlterMaterializedViewSetSchemaNoticeRule},

		{ruleID: ruleIDPGCreatePublicationNotice, construct: newCreatePublicationNoticeRule},
		{ruleID: ruleIDPGAlterPublicationNotice, construct: newAlterPublicationNoticeRule},
		{ruleID: ruleIDPGDropPublicationWarn, construct: newDropPublicationWarnRule},
		{ruleID: ruleIDPGCreateSubscriptionNotice, construct: newCreateSubscriptionNoticeRule},
		{ruleID: ruleIDPGAlterSubscriptionNotice, construct: newAlterSubscriptionNoticeRule},
		{ruleID: ruleIDPGAlterSubscriptionDisableWarn, construct: newAlterSubscriptionDisableWarnRule},
		{ruleID: ruleIDPGDropSubscriptionWarn, construct: newDropSubscriptionWarnRule},
		// PostgreSQL foreign object lifecycle rules (PG-only).
		{ruleID: ruleIDPGCreateForeignTableNotice, construct: newCreateForeignTableNoticeRule},
		{ruleID: ruleIDPGAlterForeignTableNotice, construct: newAlterForeignTableNoticeRule},
		{ruleID: ruleIDPGDropForeignTableWarn, construct: newDropForeignTableWarnRule},
		{ruleID: ruleIDPGCreateForeignServerNotice, construct: newCreateForeignServerNoticeRule},
		{ruleID: ruleIDPGAlterForeignServerNotice, construct: newAlterForeignServerNoticeRule},
		{ruleID: ruleIDPGDropForeignServerWarn, construct: newDropForeignServerWarnRule},
		{ruleID: ruleIDPGCreateUserMappingNotice, construct: newCreateUserMappingNoticeRule},
		{ruleID: ruleIDPGAlterUserMappingNotice, construct: newAlterUserMappingNoticeRule},
		{ruleID: ruleIDPGDropUserMappingWarn, construct: newDropUserMappingWarnRule},
		{ruleID: ruleIDPGCreateForeignDataWrapperNotice, construct: newCreateForeignDataWrapperNoticeRule},
		{ruleID: ruleIDPGAlterForeignDataWrapperNotice, construct: newAlterForeignDataWrapperNoticeRule},
		{ruleID: ruleIDPGDropForeignDataWrapperWarn, construct: newDropForeignDataWrapperWarnRule},
		// PostgreSQL annotation lifecycle rules (PG-only).
		{ruleID: ruleIDPGCommentOnNotice, construct: newCommentOnNoticeRule},
		{ruleID: ruleIDPGCommentOnRemoveNotice, construct: newCommentOnRemoveNoticeRule},
		{ruleID: ruleIDPGSecurityLabelNotice, construct: newSecurityLabelNoticeRule},
		{ruleID: ruleIDPGSecurityLabelRemoveNotice, construct: newSecurityLabelRemoveNoticeRule},
		// PostgreSQL event trigger lifecycle rules (PG-only).
		{ruleID: ruleIDPGCreateEventTriggerNotice, construct: newCreateEventTriggerNoticeRule},
		{ruleID: ruleIDPGAlterEventTriggerNotice, construct: newAlterEventTriggerNoticeRule},
		{ruleID: ruleIDPGAlterEventTriggerDisableWarn, construct: newAlterEventTriggerDisableWarnRule},
		{ruleID: ruleIDPGDropEventTriggerWarn, construct: newDropEventTriggerWarnRule},
		// PostgreSQL rewrite rule lifecycle rules (PG-only).
		{ruleID: ruleIDPGCreateRuleNotice, construct: newCreateRuleNoticeRule},
		{ruleID: ruleIDPGAlterRuleNotice, construct: newAlterRuleNoticeRule},
		{ruleID: ruleIDPGDropRuleWarn, construct: newDropRuleWarnRule},
		// PostgreSQL collation lifecycle rules (PG-only).
		{ruleID: ruleIDPGCreateCollationNotice, construct: newCreateCollationNoticeRule},
		{ruleID: ruleIDPGAlterCollationNotice, construct: newAlterCollationNoticeRule},
		{ruleID: ruleIDPGDropCollationWarn, construct: newDropCollationWarnRule},
		// PostgreSQL statistics lifecycle rules (PG-only).
		{ruleID: ruleIDPGCreateStatisticsNotice, construct: newCreateStatisticsNoticeRule},
		{ruleID: ruleIDPGAlterStatisticsNotice, construct: newAlterStatisticsNoticeRule},
		{ruleID: ruleIDPGDropStatisticsWarn, construct: newDropStatisticsWarnRule},
		// PostgreSQL semantic object lifecycle rules (PG-only).
		{ruleID: ruleIDPGCreateAggregateNotice, construct: newCreateAggregateNoticeRule},
		{ruleID: ruleIDPGAlterAggregateNotice, construct: newAlterAggregateNoticeRule},
		{ruleID: ruleIDPGDropAggregateWarn, construct: newDropAggregateWarnRule},
		{ruleID: ruleIDPGCreateOperatorNotice, construct: newCreateOperatorNoticeRule},
		{ruleID: ruleIDPGAlterOperatorNotice, construct: newAlterOperatorNoticeRule},
		{ruleID: ruleIDPGDropOperatorWarn, construct: newDropOperatorWarnRule},
		{ruleID: ruleIDPGCreateConversionNotice, construct: newCreateConversionNoticeRule},
		{ruleID: ruleIDPGAlterConversionNotice, construct: newAlterConversionNoticeRule},
		{ruleID: ruleIDPGDropConversionWarn, construct: newDropConversionWarnRule},
		{ruleID: ruleIDPGCreateOperatorFamilyNotice, construct: newCreateOperatorFamilyNoticeRule},
		{ruleID: ruleIDPGAlterOperatorFamilyNotice, construct: newAlterOperatorFamilyNoticeRule},
		{ruleID: ruleIDPGDropOperatorFamilyWarn, construct: newDropOperatorFamilyWarnRule},
		{ruleID: ruleIDPGCreateOperatorClassNotice, construct: newCreateOperatorClassNoticeRule},
		{ruleID: ruleIDPGAlterOperatorClassNotice, construct: newAlterOperatorClassNoticeRule},
		{ruleID: ruleIDPGDropOperatorClassWarn, construct: newDropOperatorClassWarnRule},
		{ruleID: ruleIDPGCreateTextSearchConfigurationNotice, construct: newCreateTextSearchConfigurationNoticeRule},
		{ruleID: ruleIDPGAlterTextSearchConfigurationNotice, construct: newAlterTextSearchConfigurationNoticeRule},
		{ruleID: ruleIDPGDropTextSearchConfigurationWarn, construct: newDropTextSearchConfigurationWarnRule},
		{ruleID: ruleIDPGCreateTextSearchDictionaryNotice, construct: newCreateTextSearchDictionaryNoticeRule},
		{ruleID: ruleIDPGAlterTextSearchDictionaryNotice, construct: newAlterTextSearchDictionaryNoticeRule},
		{ruleID: ruleIDPGDropTextSearchDictionaryWarn, construct: newDropTextSearchDictionaryWarnRule},
		{ruleID: ruleIDPGCreateTextSearchParserNotice, construct: newCreateTextSearchParserNoticeRule},
		{ruleID: ruleIDPGAlterTextSearchParserNotice, construct: newAlterTextSearchParserNoticeRule},
		{ruleID: ruleIDPGDropTextSearchParserWarn, construct: newDropTextSearchParserWarnRule},
		{ruleID: ruleIDPGCreateTextSearchTemplateNotice, construct: newCreateTextSearchTemplateNoticeRule},
		{ruleID: ruleIDPGAlterTextSearchTemplateNotice, construct: newAlterTextSearchTemplateNoticeRule},
		{ruleID: ruleIDPGDropTextSearchTemplateWarn, construct: newDropTextSearchTemplateWarnRule},
		{ruleID: ruleIDPGCreateTransformNotice, construct: newCreateTransformNoticeRule},
		{ruleID: ruleIDPGCreateAccessMethodNotice, construct: newCreateAccessMethodNoticeRule},
		{ruleID: ruleIDPGDropTransformWarn, construct: newDropTransformWarnRule},
		{ruleID: ruleIDPGDropAccessMethodWarn, construct: newDropAccessMethodWarnRule},
		{ruleID: ruleIDPGAlterLargeObjectOwnerNotice, construct: newAlterLargeObjectOwnerNoticeRule},
	} {
		ruleCfg, ok := cfg.Rules[factory.ruleID]
		if !ok || !ruleCfg.Enabled {
			continue
		}
		if suppressesForeignKeyConstraintNaming(cfg, factory.ruleID) {
			continue
		}

		statementRule, err := factory.construct(ruleCfg)
		if err != nil {
			return err
		}
		if err := registry.RegisterStatement(statementRule); err != nil {
			return err
		}
	}

	for _, factory := range []struct {
		ruleID    string
		construct func(policy.RulePolicy) (rule.GlobalRule, error)
	}{
		{ruleID: ruleIDAlterMergeMySQLRequire, construct: func(cfg policy.RulePolicy) (rule.GlobalRule, error) {
			return newMergeAlterRule(ruleIDAlterMergeMySQLRequire, spec.DialectMySQL, rule.LevelWarning, cfg)
		}},
		{ruleID: ruleIDAlterMergeTiDBRequire, construct: func(cfg policy.RulePolicy) (rule.GlobalRule, error) {
			return newMergeAlterRule(ruleIDAlterMergeTiDBRequire, spec.DialectTiDB, rule.LevelWarning, cfg)
		}},
		{ruleID: ruleIDPGAlterNotValidConstraintValidateRequire, construct: newNotValidConstraintValidateRequiredRule},
	} {
		ruleCfg, ok := cfg.Rules[factory.ruleID]
		if !ok || !ruleCfg.Enabled {
			continue
		}
		globalRule, err := factory.construct(ruleCfg)
		if err != nil {
			return err
		}
		if err := registry.RegisterGlobal(globalRule); err != nil {
			return err
		}
	}

	return nil
}

func suppressesForeignKeyConstraintNaming(cfg policy.Policy, ruleID string) bool {
	switch ruleID {
	case ruleIDConstraintForeignKeyNamePrefixRequire, ruleIDConstraintForeignKeyNameSuffixRequire, ruleIDConstraintForeignKeyNameContainsRequire:
	default:
		return false
	}

	forbidCfg, ok := cfg.Rules[ruleIDTableForeignKeyForbid]
	if !ok || !forbidCfg.Enabled {
		return false
	}
	forbid, err := boolParam(ruleIDTableForeignKeyForbid, forbidCfg, "forbid", true)
	if err != nil {
		return false
	}
	return forbid
}
