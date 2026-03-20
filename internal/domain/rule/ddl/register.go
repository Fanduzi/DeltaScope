// Package ddl defines Tier-1 DDL rules.
// input: domain policy values and a shared rule registry
// output: deterministic registration of the first DDL rule batch
// pos: DDL rule assembly entrypoint for application wiring
// note: if this file changes, update this header and module README.md.
package ddl

import (
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
		{ruleID: ruleIDColumnVarcharMaxLength, construct: newColumnVarcharMaxLengthRule},
		{ruleID: ruleIDColumnDefaultRequire, construct: newColumnDefaultRequiredRule},
		{ruleID: ruleIDColumnNotNullRequire, construct: newColumnNotNullRequiredRule},
		{ruleID: ruleIDColumnFloatDoubleForbid, construct: newColumnFloatDoubleForbiddenRule},
		{ruleID: ruleIDIndexTotalMaxCount, construct: newIndexTotalMaxCountRule},
		{ruleID: ruleIDIndexColumnsMaxCount, construct: newIndexColumnsMaxCountRule},
		{ruleID: ruleIDIndexUniquePrefixRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newIndexPrefixRequiredRule(ruleIDIndexUniquePrefixRequire, spec.IndexKindUnique, "uniq_", rule.LevelWarning, cfg)
		}},
		{ruleID: ruleIDIndexSecondaryPrefixRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newIndexPrefixRequiredRule(ruleIDIndexSecondaryPrefixRequire, spec.IndexKindSecondary, "idx_", rule.LevelWarning, cfg)
		}},
		{ruleID: ruleIDIndexFulltextPrefixRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newIndexPrefixRequiredRule(ruleIDIndexFulltextPrefixRequire, spec.IndexKindFulltext, "full_", rule.LevelWarning, cfg)
		}},
		{ruleID: ruleIDIndexDuplicateForbid, construct: newDuplicateIndexForbiddenRule},
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
		{ruleID: ruleIDAlterChangeColumnForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newForbiddenAlterActionRule(ruleIDAlterChangeColumnForbid, "change_column", "change column", rule.LevelBlocker, cfg)
		}},
		{ruleID: ruleIDAlterModifyColumnForbid, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newForbiddenAlterActionRule(ruleIDAlterModifyColumnForbid, "modify_column", "modify column", rule.LevelWarning, cfg)
		}},
		{ruleID: ruleIDAlterModifyColumnCompatibleRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newAlterTargetTypeFamilyRule(ruleIDAlterModifyColumnCompatibleRequire, "modify_column", "modify column", rule.LevelBlocker, defaultConservativeAlterTypeFamilies, cfg)
		}},
		{ruleID: ruleIDAlterChangeColumnCompatibleRequire, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newAlterTargetTypeFamilyRule(ruleIDAlterChangeColumnCompatibleRequire, "change_column", "change column", rule.LevelBlocker, defaultConservativeAlterTypeFamilies, cfg)
		}},
		{ruleID: ruleIDTableCommentMaxLength, construct: newTableCommentMaxLengthRule},
		{ruleID: ruleIDTableEngineAllowlist, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newTableOptionAllowlistRule(ruleIDTableEngineAllowlist, "engine", "engine", []string{"InnoDB"}, rule.LevelBlocker, cfg)
		}},
		{ruleID: ruleIDTableCharsetAllowlist, construct: func(cfg policy.RulePolicy) (rule.StatementRule, error) {
			return newTableOptionAllowlistRule(ruleIDTableCharsetAllowlist, "charset", "charset", []string{"utf8", "utf8mb4"}, rule.LevelBlocker, cfg)
		}},
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
	} {
		ruleCfg, ok := cfg.Rules[factory.ruleID]
		if !ok || !ruleCfg.Enabled {
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

	return nil
}
