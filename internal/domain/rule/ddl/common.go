// Package ddl defines Tier-1 DDL rules.
// input: normalized Statement specs emitted by application extraction, including alter-table and standalone rename payloads
// output: reusable DDL rule predicates and rule identifier constants, including the CHANGE and RENAME destination-name, RENAME COLUMN version, and T07 advanced-column identifiers, plus shared alter-action and rename matching helpers, the indexPlainComparable plain-key premise shared by the pair-comparison rules, and the memberComparisonGap/declaredComparisonGap attribution for applicable pair comparisons a non-plain member leaves unverifiable (declared-vs-existing on create_index, declaration-vs-declaration inside create_table — R6: a kind-withheld maybe-PK candidate also unverifies the pairing and owes target_table.indexes), and the T10 alter_index/invisibility/advanced-index-compatibility rule identifiers — left-prefix pairing treats any member the key-part list counts beyond the named columns (expression-bearing or unmodeled alike) as a truncated projection, so a declared prefix of the named columns is pairable even at equal length — plus the T11 dual-path constraint applicability helpers (alter.Constraint on MySQL/TiDB beside the PostgreSQL options vocabulary, the produced index definition for primary keys, the ordered-state column-fact resolution that binds an ALTER-side PK declaration's member names to pre-state column facts, and the drop_foreign_key existence rule identifier)
// pos: DDL rule common helpers shared across concrete rules and parser-neutral rename semantics
// note: if this file changes, update this header and module README.md.
package ddl

import (
	"slices"
	"strings"

	"github.com/Fanduzi/DeltaScope/internal/domain/rule"
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
	ruleIDConstraintPrimaryKeyNamePrefixRequire              = "ddl.constraint.primary_key.name.prefix.require"
	ruleIDConstraintPrimaryKeyNameSuffixRequire              = "ddl.constraint.primary_key.name.suffix.require"
	ruleIDConstraintPrimaryKeyNameContainsRequire            = "ddl.constraint.primary_key.name.contains.require"
	ruleIDConstraintUniqueKeyNamePrefixRequire               = "ddl.constraint.unique_key.name.prefix.require"
	ruleIDConstraintUniqueKeyNameSuffixRequire               = "ddl.constraint.unique_key.name.suffix.require"
	ruleIDConstraintUniqueKeyNameContainsRequire             = "ddl.constraint.unique_key.name.contains.require"
	ruleIDConstraintForeignKeyNamePrefixRequire              = "ddl.constraint.foreign_key.name.prefix.require"
	ruleIDConstraintForeignKeyNameSuffixRequire              = "ddl.constraint.foreign_key.name.suffix.require"
	ruleIDConstraintForeignKeyNameContainsRequire            = "ddl.constraint.foreign_key.name.contains.require"
	ruleIDConstraintCheckNamePrefixRequire                   = "ddl.constraint.check.name.prefix.require"
	ruleIDConstraintCheckNameSuffixRequire                   = "ddl.constraint.check.name.suffix.require"
	ruleIDConstraintCheckNameContainsRequire                 = "ddl.constraint.check.name.contains.require"
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
	ruleIDIndexUniqueSuffixRequire                           = "ddl.index.unique.suffix.require"
	ruleIDIndexUniqueContainsRequire                         = "ddl.index.unique.contains.require"
	ruleIDIndexSecondaryPrefixRequire                        = "ddl.index.secondary.prefix.require"
	ruleIDIndexSecondarySuffixRequire                        = "ddl.index.secondary.suffix.require"
	ruleIDIndexSecondaryContainsRequire                      = "ddl.index.secondary.contains.require"
	ruleIDIndexFulltextPrefixRequire                         = "ddl.index.fulltext.prefix.require"
	ruleIDIndexFulltextSuffixRequire                         = "ddl.index.fulltext.suffix.require"
	ruleIDIndexFulltextContainsRequire                       = "ddl.index.fulltext.contains.require"
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
	ruleIDAlterAddIndexUniqueSuffixRequire                   = "ddl.alter.add_index.unique.suffix.require"
	ruleIDAlterAddIndexUniqueContainsRequire                 = "ddl.alter.add_index.unique.contains.require"
	ruleIDAlterAddIndexSecondaryPrefixRequire                = "ddl.alter.add_index.secondary.prefix.require"
	ruleIDAlterAddIndexSecondarySuffixRequire                = "ddl.alter.add_index.secondary.suffix.require"
	ruleIDAlterAddIndexSecondaryContainsRequire              = "ddl.alter.add_index.secondary.contains.require"
	ruleIDAlterAddIndexFulltextPrefixRequire                 = "ddl.alter.add_index.fulltext.prefix.require"
	ruleIDAlterAddIndexFulltextSuffixRequire                 = "ddl.alter.add_index.fulltext.suffix.require"
	ruleIDAlterAddIndexFulltextContainsRequire               = "ddl.alter.add_index.fulltext.contains.require"
	ruleIDAlterSetDataTypeForbid                             = "ddl.alter.set_data_type.forbid"
	ruleIDAlterSetDefaultForbid                              = "ddl.alter.set_default.forbid"
	ruleIDAlterDropDefaultForbid                             = "ddl.alter.drop_default.forbid"
	ruleIDAlterSetNotNullForbid                              = "ddl.alter.set_not_null.forbid"
	ruleIDAlterDropNotNullForbid                             = "ddl.alter.drop_not_null.forbid"
	ruleIDAlterDropExpressionForbid                          = "ddl.alter.drop_expression.forbid"
	ruleIDAlterSetGeneratedForbid                            = "ddl.alter.set_generated.forbid"
	ruleIDAlterDropIdentityForbid                            = "ddl.alter.drop_identity.forbid"
	ruleIDAlterSetDefaultExplicitDefaultChangeForbid         = "ddl.alter.set_default.explicit_default_change.forbid"
	ruleIDAlterDropDefaultExplicitDefaultChangeForbid        = "ddl.alter.drop_default.explicit_default_change.forbid"
	ruleIDAlterSetDefaultExistsRequire                       = "ddl.alter.set_default.exists.require"
	ruleIDAlterDropDefaultExistsRequire                      = "ddl.alter.drop_default.exists.require"
	ruleIDAlterColumnPositionExistsRequire                   = "ddl.alter.column_position.exists.require"
	ruleIDAlterSetNotNullExplicitNullabilityChangeForbid     = "ddl.alter.set_not_null.explicit_nullability_change.forbid"
	ruleIDAlterDropNotNullExplicitNullabilityChangeForbid    = "ddl.alter.drop_not_null.explicit_nullability_change.forbid"
	ruleIDTableCommentMaxLength                              = "ddl.table.comment.max_length"
	ruleIDTableEngineAllowlist                               = "ddl.table.engine.allowlist"
	ruleIDTableCharsetAllowlist                              = "ddl.table.charset.allowlist"
	ruleIDTableCollationAllowlist                            = "ddl.table.collation.allowlist"
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
	ruleIDCreateIndexColumnsExistRequire                     = "ddl.create_index.columns.exists.require"
	ruleIDAlterAddColumnExistsForbid                         = "ddl.alter.add_column.exists.forbid"
	ruleIDAlterDropColumnExistsRequire                       = "ddl.alter.drop_column.exists.require"
	ruleIDAlterModifyColumnExistsRequire                     = "ddl.alter.modify_column.exists.require"
	ruleIDAlterChangeColumnExistsRequire                     = "ddl.alter.change_column.exists.require"
	ruleIDAlterChangeColumnTargetExistsForbid                = "ddl.alter.change_column.target.exists.forbid"
	ruleIDAlterRenameColumnExistsRequire                     = "ddl.alter.rename_column.exists.require"
	ruleIDAlterRenameColumnTargetExistsForbid                = "ddl.alter.rename_column.target.exists.forbid"
	ruleIDAlterRenameColumnVersionRequire                    = "ddl.alter.rename_column.version.require"
	ruleIDAlterColumnDependencyRequire                       = "ddl.alter.column.dependency.require"
	ruleIDColumnAdvancedCompatibilityRequire                 = "ddl.column.advanced.compatibility.require"
	ruleIDAlterAddIndexExistsForbid                          = "ddl.alter.add_index.exists.forbid"
	ruleIDAlterDropIndexExistsRequire                        = "ddl.alter.drop_index.exists.require"
	ruleIDAlterRenameIndexExistsRequire                      = "ddl.alter.rename_index.exists.require"
	ruleIDAlterIndexExistsRequire                            = "ddl.alter.alter_index.exists.require"
	ruleIDIndexInvisibleForbid                               = "ddl.index.invisible.forbid"
	ruleIDIndexAdvancedCompatibilityRequire                  = "ddl.index.advanced.compatibility.require"
	ruleIDAlterDropPrimaryKeyExistsRequire                   = "ddl.alter.drop_primary_key.exists.require"
	ruleIDAlterDropForeignKeyExistsRequire                   = "ddl.alter.drop_foreign_key.exists.require"
	ruleIDAlterModifyColumnCompatibilityRequire              = "ddl.alter.modify_column.compatibility.require"
	ruleIDAlterChangeColumnCompatibilityRequire              = "ddl.alter.change_column.compatibility.require"
	ruleIDAlterTableOptionCompatibilityRequire               = "ddl.alter.table_option.compatibility.require"
	ruleIDViewCreateForbid                                   = "ddl.view.create.forbid"
	ruleIDViewDropForbid                                     = "ddl.view.drop.forbid"
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
	ruleIDPGCreateIndexConcurrentlyRequire                   = "ddl.pg.create_index.concurrently.require"
	ruleIDPGAlterAddColumnNonNullDefaultRewriteWarn          = "ddl.pg.alter.add_column.non_null_default.rewrite.warn"
	ruleIDPGAlterAddCheckNotValidRequire                     = "ddl.pg.alter.add_check.not_valid.require"
	ruleIDPGAlterSetDataTypeRewriteWarn                      = "ddl.pg.alter.set_data_type.rewrite.warn"
	ruleIDPGTableForeignKeyCrossSchemaAdvisory               = "ddl.pg.table.foreign_key.cross_schema.advisory"
	ruleIDPGAlterNotValidConstraintValidateRequire           = "ddl.pg.alter.not_valid_constraint.validate.require"
	ruleIDPGDropIndexAdvisory                                = "ddl.pg.drop_index.advisory"
	ruleIDPGAlterAddColumnNonNullNoDefaultWarn               = "ddl.pg.alter.add_column.non_null_no_default.warn"
	ruleIDPGAlterAddUniqueConstraintConcurrentIndexAdvisory  = "ddl.pg.alter.add_unique_constraint.concurrent_index.advisory"
	ruleIDPGAlterDropConstraintAdvisory                      = "ddl.pg.alter.drop_constraint.advisory"
	ruleIDPGDropSchemaAdvisory                               = "ddl.pg.drop_schema.advisory"
	ruleIDPGDropSchemaCascadeWarn                            = "ddl.pg.drop_schema.cascade.warn"
	ruleIDPGCreateSequenceCycleWarn                          = "ddl.pg.create_sequence.cycle.warn"
	ruleIDPGAlterSequenceRestartWarn                         = "ddl.pg.alter_sequence.restart.warn"
	ruleIDPGAlterSequenceCycleWarn                           = "ddl.pg.alter_sequence.cycle.warn"
	ruleIDPGDropSequenceAdvisory                             = "ddl.pg.drop_sequence.advisory"
	ruleIDPGDropSequenceCascadeWarn                          = "ddl.pg.drop_sequence.cascade.warn"
	ruleIDPGDropMaterializedViewAdvisory                     = "ddl.pg.drop_materialized_view.advisory"
	ruleIDPGDropMaterializedViewCascadeWarn                  = "ddl.pg.drop_materialized_view.cascade.warn"
	ruleIDPGRefreshMaterializedViewConcurrentlyWarn          = "ddl.pg.refresh_materialized_view.concurrently.warn"
	ruleIDPGRefreshMaterializedViewNoDataNotice              = "ddl.pg.refresh_materialized_view.no_data.notice"
	ruleIDPGAlterDropColumnAdvisory                          = "ddl.pg.alter.drop_column.advisory"
	ruleIDPGAlterValidateConstraintAdvisory                  = "ddl.pg.alter.validate_constraint.advisory"
	ruleIDPGAlterAddColumnNullableNotice                     = "ddl.pg.alter.add_column.nullable.notice"
	ruleIDPGAlterSetSchemaAdvisory                           = "ddl.pg.alter.set_schema.advisory"
	ruleIDPGAlterOwnerAdvisory                               = "ddl.pg.alter.owner.advisory"
	ruleIDPGAlterEnableTriggerNotice                         = "ddl.pg.alter.enable_trigger.notice"
	ruleIDPGAlterDisableTriggerWarn                          = "ddl.pg.alter.disable_trigger.warn"
	ruleIDPGAlterAttachPartitionAdvisory                     = "ddl.pg.alter.attach_partition.advisory"
	ruleIDPGAlterDetachPartitionWarn                         = "ddl.pg.alter.detach_partition.warn"
	ruleIDPGAlterReplicaIdentityFullWarn                     = "ddl.pg.alter.replica_identity_full.warn"
	ruleIDPGAlterReplicaIdentityNothingWarn                  = "ddl.pg.alter.replica_identity_nothing.warn"
	ruleIDPGAlterReplicaIdentityUsingIndexNotice             = "ddl.pg.alter.replica_identity_using_index.notice"
	ruleIDPGAlterLoggedNotice                                = "ddl.pg.alter.set_logged.notice"
	ruleIDPGAlterUnloggedNotice                              = "ddl.pg.alter.set_unlogged.notice"
	ruleIDPGAlterSetTablespaceNotice                         = "ddl.pg.alter.set_tablespace.notice"
	ruleIDPGAlterSetAccessMethodWarn                         = "ddl.pg.alter.set_access_method.warn"
	ruleIDPGAlterEnableReplicaTriggerNotice                  = "ddl.pg.alter.enable_replica_trigger.notice"
	ruleIDPGAlterEnableAlwaysTriggerNotice                   = "ddl.pg.alter.enable_always_trigger.notice"
	ruleIDPGAlterEnableRuleNotice                            = "ddl.pg.alter.enable_rule.notice"
	ruleIDPGAlterDisableRuleWarn                             = "ddl.pg.alter.disable_rule.warn"
	ruleIDPGAlterEnableReplicaRuleNotice                     = "ddl.pg.alter.enable_replica_rule.notice"
	ruleIDPGAlterEnableAlwaysRuleNotice                      = "ddl.pg.alter.enable_always_rule.notice"
	ruleIDPGAlterSetReloptionsWarn                           = "ddl.pg.alter.set_reloptions.warn"
	ruleIDPGAlterResetReloptionsNotice                       = "ddl.pg.alter.reset_reloptions.notice"
	// PostgreSQL column attribute rules (PG-only).
	ruleIDPGAlterSetColumnStatisticsNotice  = "ddl.pg.alter.set_column_statistics.notice"
	ruleIDPGAlterSetColumnOptionsNotice     = "ddl.pg.alter.set_column_options.notice"
	ruleIDPGAlterResetColumnOptionsNotice   = "ddl.pg.alter.reset_column_options.notice"
	ruleIDPGAlterSetColumnStorageNotice     = "ddl.pg.alter.set_column_storage.notice"
	ruleIDPGAlterSetColumnCompressionNotice = "ddl.pg.alter.set_column_compression.notice"
	// PostgreSQL cluster and detach-finalize rules (PG-only).
	ruleIDPGAlterClusterOnNotice               = "ddl.pg.alter.cluster_on.notice"
	ruleIDPGAlterSetWithoutClusterNotice       = "ddl.pg.alter.set_without_cluster.notice"
	ruleIDPGAlterDetachPartitionFinalizeNotice = "ddl.pg.alter.detach_partition_finalize.notice"
	ruleIDPGCreateTypeEnumNotice               = "ddl.pg.create_type.enum.notice"
	ruleIDPGAlterTypeAddValueAdvisory          = "ddl.pg.alter_type.add_value.advisory"
	ruleIDPGAlterTypeAddValuePositionNotice    = "ddl.pg.alter_type.add_value.position.notice"
	ruleIDPGDropTypeAdvisory                   = "ddl.pg.drop_type.advisory"
	ruleIDPGDropTypeCascadeWarn                = "ddl.pg.drop_type.cascade.warn"
	// PostgreSQL domain lifecycle rules (PG-only).
	ruleIDPGCreateDomainNotice          = "ddl.pg.create_domain.notice"
	ruleIDPGAlterDomainConstraintNotice = "ddl.pg.alter_domain.constraint.notice"
	ruleIDPGAlterDomainDefaultNotice    = "ddl.pg.alter_domain.default.notice"
	ruleIDPGAlterDomainNotNullNotice    = "ddl.pg.alter_domain.not_null.notice"
	ruleIDPGAlterDomainRenameNotice     = "ddl.pg.alter_domain.rename.notice"
	ruleIDPGDropDomainAdvisory          = "ddl.pg.drop_domain.advisory"
	ruleIDPGDropDomainCascadeWarn       = "ddl.pg.drop_domain.cascade.warn"
	// PostgreSQL composite type lifecycle rules (PG-only).
	ruleIDPGCreateTypeCompositeNotice         = "ddl.pg.create_type.composite.notice"
	ruleIDPGAlterTypeCompositeRenameNotice    = "ddl.pg.alter_type.composite_rename.notice"
	ruleIDPGAlterTypeCompositeSetSchemaNotice = "ddl.pg.alter_type.composite_set_schema.notice"
	// PostgreSQL composite type attribute lifecycle rules (PG-only).
	ruleIDPGAlterTypeAddAttributeNotice     = "ddl.pg.alter_type.add_attribute.notice"
	ruleIDPGAlterTypeDropAttributeWarn      = "ddl.pg.alter_type.drop_attribute.warn"
	ruleIDPGAlterTypeAlterAttributeTypeWarn = "ddl.pg.alter_type.alter_attribute_type.warn"
	ruleIDPGAlterTypeRenameAttributeNotice  = "ddl.pg.alter_type.rename_attribute.notice"
	// PostgreSQL table privilege rules (PG-only).
	ruleIDPGGrantTablePrivilegeNotice       = "ddl.pg.grant.table_privilege.notice"
	ruleIDPGGrantTablePrivilegeAllWarn      = "ddl.pg.grant.table_privilege.all.warn"
	ruleIDPGRevokeTablePrivilegeNotice      = "ddl.pg.revoke.table_privilege.notice"
	ruleIDPGRevokeTablePrivilegeCascadeWarn = "ddl.pg.revoke.table_privilege.cascade.warn"
	// PostgreSQL extension lifecycle rules (PG-only).
	ruleIDPGCreateExtensionNotice         = "ddl.pg.create_extension.notice"
	ruleIDPGCreateExtensionCascadeWarn    = "ddl.pg.create_extension.cascade.warn"
	ruleIDPGAlterExtensionUpdateNotice    = "ddl.pg.alter_extension.update.notice"
	ruleIDPGAlterExtensionSetSchemaNotice = "ddl.pg.alter_extension.set_schema.notice"
	ruleIDPGDropExtensionAdvisory         = "ddl.pg.drop_extension.advisory"
	ruleIDPGDropExtensionCascadeWarn      = "ddl.pg.drop_extension.cascade.warn"
	ruleIDPGAlterExtensionAddMemberNotice = "ddl.pg.alter_extension.add_member.notice"
	ruleIDPGAlterExtensionDropMemberWarn  = "ddl.pg.alter_extension.drop_member.warn"
	// MySQL/TiDB database lifecycle rules.
	ruleIDDatabaseCreateNotice = "ddl.database.create.notice"
	ruleIDDatabaseDropWarn     = "ddl.database.drop.warn"
	// PostgreSQL create schema rule.
	ruleIDPGCreateSchemaNotice = "ddl.pg.create_schema.notice"
	// PostgreSQL policy lifecycle rules (PG-only).
	ruleIDPGCreatePolicyNotice    = "ddl.pg.create_policy.notice"
	ruleIDPGAlterPolicyNotice     = "ddl.pg.alter_policy.notice"
	ruleIDPGDropPolicyWarn        = "ddl.pg.drop_policy.warn"
	ruleIDPGAlterEnableRLSNotice  = "ddl.pg.alter.enable_rls.notice"
	ruleIDPGAlterDisableRLSWarn   = "ddl.pg.alter.disable_rls.warn"
	ruleIDPGAlterForceRLSNotice   = "ddl.pg.alter.force_rls.notice"
	ruleIDPGAlterNoForceRLSNotice = "ddl.pg.alter.no_force_rls.notice"
	// PostgreSQL trigger lifecycle rules (PG-only).
	ruleIDPGCreateTriggerNotice         = "ddl.pg.create_trigger.notice"
	ruleIDPGCreateConstraintTriggerWarn = "ddl.pg.create_constraint_trigger.warn"
	ruleIDPGDropTriggerAdvisory         = "ddl.pg.drop_trigger.advisory"
	// PostgreSQL function/procedure lifecycle rules (PG-only).
	ruleIDPGCreateFunctionNotice              = "ddl.pg.create_function.notice"
	ruleIDPGCreateFunctionSecurityDefinerWarn = "ddl.pg.create_function.security_definer.warn"
	ruleIDPGCreateOrReplaceFunctionAdvisory   = "ddl.pg.create_or_replace_function.advisory"
	ruleIDPGDropFunctionAdvisory              = "ddl.pg.drop_function.advisory"
	ruleIDPGCreateProcedureNotice             = "ddl.pg.create_procedure.notice"
	ruleIDPGDropProcedureAdvisory             = "ddl.pg.drop_procedure.advisory"

	// PG advanced view lifecycle rules (PostgreSQL-only).
	ruleIDPGCreateOrReplaceViewAdvisory = "ddl.pg.create_or_replace_view.advisory"
	ruleIDPGCreateTempViewNotice        = "ddl.pg.create_temp_view.notice"
	ruleIDPGCreateViewCheckOptionNotice = "ddl.pg.create_view.check_option.notice"
	ruleIDPGAlterViewRenameNotice       = "ddl.pg.alter_view.rename.notice"
	ruleIDPGAlterViewSetSchemaNotice    = "ddl.pg.alter_view.set_schema.notice"
	ruleIDPGDropViewCascadeWarn         = "ddl.pg.drop_view.cascade.warn"

	// PG alter object lifecycle rules (PostgreSQL-only).
	ruleIDPGAlterSchemaRenameNotice              = "ddl.pg.alter_schema.rename.notice"
	ruleIDPGAlterSchemaOwnerNotice               = "ddl.pg.alter_schema.owner.notice"
	ruleIDPGAlterIndexRenameNotice               = "ddl.pg.alter_index.rename.notice"
	ruleIDPGAlterIndexSetTablespaceNotice        = "ddl.pg.alter_index.set_tablespace.notice"
	ruleIDPGAlterMaterializedViewRenameNotice    = "ddl.pg.alter_materialized_view.rename.notice"
	ruleIDPGAlterMaterializedViewSetSchemaNotice = "ddl.pg.alter_materialized_view.set_schema.notice"

	ruleIDPGCreatePublicationNotice      = "ddl.pg.create_publication.notice"
	ruleIDPGAlterPublicationNotice       = "ddl.pg.alter_publication.notice"
	ruleIDPGDropPublicationWarn          = "ddl.pg.drop_publication.warn"
	ruleIDPGCreateSubscriptionNotice     = "ddl.pg.create_subscription.notice"
	ruleIDPGAlterSubscriptionNotice      = "ddl.pg.alter_subscription.notice"
	ruleIDPGAlterSubscriptionDisableWarn = "ddl.pg.alter_subscription.disable.warn"
	ruleIDPGDropSubscriptionWarn         = "ddl.pg.drop_subscription.warn"
	// PostgreSQL foreign object lifecycle rules (PG-only).
	ruleIDPGCreateForeignTableNotice       = "ddl.pg.create_foreign_table.notice"
	ruleIDPGAlterForeignTableNotice        = "ddl.pg.alter_foreign_table.notice"
	ruleIDPGDropForeignTableWarn           = "ddl.pg.drop_foreign_table.warn"
	ruleIDPGCreateForeignServerNotice      = "ddl.pg.create_foreign_server.notice"
	ruleIDPGAlterForeignServerNotice       = "ddl.pg.alter_foreign_server.notice"
	ruleIDPGDropForeignServerWarn          = "ddl.pg.drop_foreign_server.warn"
	ruleIDPGCreateUserMappingNotice        = "ddl.pg.create_user_mapping.notice"
	ruleIDPGAlterUserMappingNotice         = "ddl.pg.alter_user_mapping.notice"
	ruleIDPGDropUserMappingWarn            = "ddl.pg.drop_user_mapping.warn"
	ruleIDPGCreateForeignDataWrapperNotice = "ddl.pg.create_foreign_data_wrapper.notice"
	ruleIDPGAlterForeignDataWrapperNotice  = "ddl.pg.alter_foreign_data_wrapper.notice"
	ruleIDPGDropForeignDataWrapperWarn     = "ddl.pg.drop_foreign_data_wrapper.warn"
	// PostgreSQL annotation lifecycle rules (PG-only).
	ruleIDPGCommentOnNotice           = "ddl.pg.comment_on.notice"
	ruleIDPGCommentOnRemoveNotice     = "ddl.pg.comment_on.remove.notice"
	ruleIDPGSecurityLabelNotice       = "ddl.pg.security_label.notice"
	ruleIDPGSecurityLabelRemoveNotice = "ddl.pg.security_label.remove.notice"
	// PostgreSQL event trigger lifecycle rules (PG-only).
	ruleIDPGCreateEventTriggerNotice     = "ddl.pg.create_event_trigger.notice"
	ruleIDPGAlterEventTriggerNotice      = "ddl.pg.alter_event_trigger.notice"
	ruleIDPGAlterEventTriggerDisableWarn = "ddl.pg.alter_event_trigger.disable.warn"
	ruleIDPGDropEventTriggerWarn         = "ddl.pg.drop_event_trigger.warn"
	// PostgreSQL rewrite rule lifecycle rules (PG-only).
	ruleIDPGCreateRuleNotice = "ddl.pg.create_rule.notice"
	ruleIDPGAlterRuleNotice  = "ddl.pg.alter_rule.notice"
	ruleIDPGDropRuleWarn     = "ddl.pg.drop_rule.warn"

	ruleIDPGCreateCollationNotice = "ddl.pg.create_collation.notice"
	ruleIDPGAlterCollationNotice  = "ddl.pg.alter_collation.notice"
	ruleIDPGDropCollationWarn     = "ddl.pg.drop_collation.warn"

	ruleIDPGCreateStatisticsNotice = "ddl.pg.create_statistics.notice"
	ruleIDPGAlterStatisticsNotice  = "ddl.pg.alter_statistics.notice"
	ruleIDPGDropStatisticsWarn     = "ddl.pg.drop_statistics.warn"
	// PostgreSQL semantic object lifecycle rules (PG-only).
	ruleIDPGCreateAggregateNotice  = "ddl.pg.create_aggregate.notice"
	ruleIDPGAlterAggregateNotice   = "ddl.pg.alter_aggregate.notice"
	ruleIDPGDropAggregateWarn      = "ddl.pg.drop_aggregate.warn"
	ruleIDPGCreateOperatorNotice   = "ddl.pg.create_operator.notice"
	ruleIDPGAlterOperatorNotice    = "ddl.pg.alter_operator.notice"
	ruleIDPGDropOperatorWarn       = "ddl.pg.drop_operator.warn"
	ruleIDPGCreateConversionNotice = "ddl.pg.create_conversion.notice"
	ruleIDPGAlterConversionNotice  = "ddl.pg.alter_conversion.notice"
	ruleIDPGDropConversionWarn     = "ddl.pg.drop_conversion.warn"

	ruleIDPGCreateOperatorFamilyNotice          = "ddl.pg.create_operator_family.notice"
	ruleIDPGAlterOperatorFamilyNotice           = "ddl.pg.alter_operator_family.notice"
	ruleIDPGDropOperatorFamilyWarn              = "ddl.pg.drop_operator_family.warn"
	ruleIDPGCreateOperatorClassNotice           = "ddl.pg.create_operator_class.notice"
	ruleIDPGAlterOperatorClassNotice            = "ddl.pg.alter_operator_class.notice"
	ruleIDPGDropOperatorClassWarn               = "ddl.pg.drop_operator_class.warn"
	ruleIDPGCreateTextSearchConfigurationNotice = "ddl.pg.create_text_search_configuration.notice"
	ruleIDPGAlterTextSearchConfigurationNotice  = "ddl.pg.alter_text_search_configuration.notice"
	ruleIDPGDropTextSearchConfigurationWarn     = "ddl.pg.drop_text_search_configuration.warn"
	ruleIDPGCreateTextSearchDictionaryNotice    = "ddl.pg.create_text_search_dictionary.notice"
	ruleIDPGAlterTextSearchDictionaryNotice     = "ddl.pg.alter_text_search_dictionary.notice"
	ruleIDPGDropTextSearchDictionaryWarn        = "ddl.pg.drop_text_search_dictionary.warn"
	ruleIDPGCreateTextSearchParserNotice        = "ddl.pg.create_text_search_parser.notice"
	ruleIDPGAlterTextSearchParserNotice         = "ddl.pg.alter_text_search_parser.notice"
	ruleIDPGDropTextSearchParserWarn            = "ddl.pg.drop_text_search_parser.warn"
	ruleIDPGCreateTextSearchTemplateNotice      = "ddl.pg.create_text_search_template.notice"
	ruleIDPGAlterTextSearchTemplateNotice       = "ddl.pg.alter_text_search_template.notice"
	ruleIDPGDropTextSearchTemplateWarn          = "ddl.pg.drop_text_search_template.warn"

	ruleIDPGCreateTransformNotice       = "ddl.pg.create_transform.notice"
	ruleIDPGCreateAccessMethodNotice    = "ddl.pg.create_access_method.notice"
	ruleIDPGDropTransformWarn           = "ddl.pg.drop_transform.warn"
	ruleIDPGDropAccessMethodWarn        = "ddl.pg.drop_access_method.warn"
	ruleIDPGAlterLargeObjectOwnerNotice = "ddl.pg.alter_large_object.owner.notice"
	// PostgreSQL table relationship rules (PG-only).
	ruleIDPGAlterAddInheritNotice  = "ddl.pg.alter.add_inherit.notice"
	ruleIDPGAlterDropInheritNotice = "ddl.pg.alter.drop_inherit.notice"
	ruleIDPGAlterAddOfTypeNotice   = "ddl.pg.alter.add_of_type.notice"
	ruleIDPGAlterDropOfTypeNotice  = "ddl.pg.alter.drop_of_type.notice"
	// PostgreSQL constraint deferrability rules (PG-only).
	ruleIDPGAlterConstraintDeferrableNotice        = "ddl.pg.alter.constraint_deferrable.notice"
	ruleIDPGAlterConstraintInitiallyDeferredNotice = "ddl.pg.alter.constraint_initially_deferred.notice"
	// PostgreSQL alter table final boundary rules (PG-only).
	ruleIDPGAlterSetExpressionNotice          = "ddl.pg.alter.set_expression.notice"
	ruleIDPGAlterAddIdentityNotice            = "ddl.pg.alter.add_identity.notice"
	ruleIDPGAlterAddExclusionConstraintNotice = "ddl.pg.alter.add_exclusion_constraint.notice"
	ruleIDPGAlterMoveAllTablespaceNotice      = "ddl.pg.alter.move_all_tablespace.notice"
	// MySQL/TiDB lifecycle notice rules.
	ruleIDRenameTableNotice           = "ddl.rename_table.notice"
	ruleIDCreateIndexNotice           = "ddl.create_index.notice"
	ruleIDDropIndexNotice             = "ddl.drop_index.notice"
	ruleIDAlterDatabaseNotice         = "ddl.alter_database.notice"
	ruleIDCreateProcedureNotice       = "ddl.create_procedure.notice"
	ruleIDDropProcedureNotice         = "ddl.drop_procedure.notice"
	ruleIDCreateUserNotice            = "ddl.create_user.notice"
	ruleIDAlterUserNotice             = "ddl.alter_user.notice"
	ruleIDDropUserNotice              = "ddl.drop_user.notice"
	ruleIDCreateRoleNotice            = "ddl.create_role.notice"
	ruleIDDropRoleNotice              = "ddl.drop_role.notice"
	ruleIDGrantNotice                 = "ddl.grant.notice"
	ruleIDRevokeNotice                = "ddl.revoke.notice"
	ruleIDDropResourceGroupNotice     = "ddl.drop_resource_group.notice"
	ruleIDCreatePlacementPolicyNotice = "ddl.create_placement_policy.notice"
	ruleIDAlterPlacementPolicyNotice  = "ddl.alter_placement_policy.notice"
	ruleIDDropPlacementPolicyNotice   = "ddl.drop_placement_policy.notice"
	ruleIDCreateSequenceNotice        = "ddl.create_sequence.notice"
	ruleIDAlterSequenceNotice         = "ddl.alter_sequence.notice"
	ruleIDDropSequenceNotice          = "ddl.drop_sequence.notice"
	// MySQL/TiDB alter action notice rules.
	ruleIDAlterAddColumnNotice                = "ddl.alter.add_column.notice"
	ruleIDAlterAddConstraintNotice            = "ddl.alter.add_constraint.notice"
	ruleIDAlterDropColumnNotice               = "ddl.alter.drop_column.notice"
	ruleIDAlterModifyColumnNotice             = "ddl.alter.modify_column.notice"
	ruleIDAlterDropIndexNotice                = "ddl.alter.drop_index.notice"
	ruleIDAlterDropForeignKeyNotice           = "ddl.alter.drop_foreign_key.notice"
	ruleIDTiDBAlterTablePlacementPolicyNotice = "ddl.tidb.alter_table.placement_policy.notice"
)

const ruleIDAlterModifyColumnExplicitNullabilityUnknownPriorStateAdvisory = "ddl.alter.modify_column.explicit_nullability_change.unknown_prior_state.advisory"

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

func appliesToDDLWithIndexes(statement spec.Statement) bool {
	if statement.Kind != spec.KindDDL || statement.DDL == nil || statement.DDL.Table == nil || len(declaredIndexes(statement)) == 0 {
		return false
	}
	switch statement.DDL.Operation {
	case spec.DDLOperationCreateTable, spec.DDLOperationCreateIndex:
		return true
	default:
		return false
	}
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

func appliesToStandaloneDDLAction(statement spec.Statement, action string) bool {
	return len(matchingStandaloneDDLActions(statement, action)) > 0
}

//nolint:unused
func appliesToCreateView(statement spec.Statement) bool {
	return statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Table != nil &&
		statement.DDL.Operation == spec.DDLOperationCreateView
}

//nolint:unused
func appliesToDropTable(statement spec.Statement) bool {
	return statement.Kind == spec.KindDDL &&
		statement.DDL != nil &&
		statement.DDL.Table != nil &&
		statement.DDL.Operation == spec.DDLOperationDropTable
}

//nolint:unused
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

// indexPlainComparable reports whether an index carries only plain ascending
// whole-column key parts. The ordinary-key comparisons (duplicate, left
// prefix, unique overlap) may pair two indexes only when both sides pass —
// a member fact of any kind (prefix length, DESC, expression, predicate,
// included columns, GLOBAL, unmodeled option, or an unclassifiable provider
// row) means the index is not sufficiently known for a plain-column verdict.
// Non-plain pairs are suppressed, never silently equated and never convicted.
func indexPlainComparable(idx spec.Index) bool {
	return idx.PrefixParts == 0 &&
		idx.DescParts == 0 &&
		idx.ExpressionCount == 0 &&
		!idx.HasExpressionKeys &&
		idx.UnmodeledKeyParts == 0 &&
		idx.DirectionUnknownParts == 0 &&
		!idx.HasPredicate &&
		len(idx.IncludedColumns) == 0 &&
		!idx.Global &&
		len(idx.UnmodeledOptions) == 0
}

// indexKeyPartsGapFact is the required_facts name for member-level key-part
// detail: the index *set* may be vouched while an individual member's parts
// are not plain-comparable. Set knownness (`target_table.indexes`) and
// member comparability stay distinct facts.
const indexKeyPartsGapFact = "target_table.indexes.key_parts"

// indexPairVerdict classifies one declared/existing pair's comparison
// reach — the pairing can apply, cannot apply, or is blocked only by the
// existing member's withheld kind (a demoted PK-attribution candidate that
// may still satisfy the kind side).
type indexPairVerdict int

const (
	indexPairNone indexPairVerdict = iota
	indexPairApplies
	indexPairKindWithheld
)

// indexPairing encodes a rule's plain-key comparison reach: whether the
// declared index could have been compared against the existing member.
// A non-plain member on either side leaves the comparison unanswerable —
// the gap functions below report it as attributable key-parts evidence
// rather than reading the declared subject as standalone unaudited — and
// an existing member whose kind is withheld leaves the pair itself
// unanswerable when its column shape otherwise matches.
type indexPairing func(declared, existing spec.Index) indexPairVerdict

func duplicatePairing(declared, existing spec.Index) indexPairVerdict {
	if !slices.Equal(existing.Columns, declared.Columns) {
		return indexPairNone
	}
	if existing.Kind == spec.IndexKindUnknown {
		return indexPairKindWithheld
	}
	if existing.Kind == declared.Kind {
		return indexPairApplies
	}
	return indexPairNone
}

func leftPrefixPairing(declared, existing spec.Index) indexPairVerdict {
	if declared.Kind != spec.IndexKindSecondary || len(declared.Columns) == 0 {
		return indexPairNone
	}
	pairable := false
	if existing.ExpressionCount > 0 || existing.HasExpressionKeys ||
		len(existing.KeyParts) > len(existing.Columns) {
		// Expression parts never enter Columns, so the recorded columns are
		// a truncated projection of the real key sequence and expression
		// positions are unrecorded; the same truncation holds for any
		// member the key-part list counts beyond the named columns
		// (unmodeled rows hide members too). A declared index naming every
		// recorded column in order can still be a strict left prefix of
		// the real sequence once hidden parts follow — only a non-strict
		// prefix of the named projection is provably pairable; anything
		// else (a declared column absent from the named parts, or a
		// declared prefix reaching past them) is unrelated under every
		// interleaving.
		pairable = hasKeyPartPrefix(declared.Columns, existing.Columns)
	} else {
		pairable = isLeftPrefix(declared.Columns, existing.Columns)
	}
	if !pairable {
		return indexPairNone
	}
	if existing.Kind == spec.IndexKindUnknown {
		return indexPairKindWithheld
	}
	if existing.Kind == spec.IndexKindSecondary {
		return indexPairApplies
	}
	return indexPairNone
}

// hasKeyPartPrefix reports whether prefix matches the head of the named
// key-part projection without requiring strict shortness — the caller uses
// it only when hidden key parts guarantee the real sequence is longer.
func hasKeyPartPrefix(prefix, sequence []string) bool {
	if len(prefix) == 0 || len(prefix) > len(sequence) {
		return false
	}
	for idx := range prefix {
		if !strings.EqualFold(prefix[idx], sequence[idx]) {
			return false
		}
	}
	return true
}

func uniqueOverlapPairing(declared, existing spec.Index) indexPairVerdict {
	if declared.Kind != spec.IndexKindSecondary ||
		!slices.Equal(existing.Columns, declared.Columns) {
		return indexPairNone
	}
	if existing.Kind == spec.IndexKindUnknown {
		return indexPairKindWithheld
	}
	if existing.Kind == spec.IndexKindUnique {
		return indexPairApplies
	}
	return indexPairNone
}

// memberComparisonGap reports one attributable gap when a declared index and
// a vouched existing member could pair under the rule's column shape but
// either side carries a non-plain member: the pairing itself is only a
// column-name shape, so a prefix/DESC/expression member on either side
// leaves the plain-key comparison unanswered — "no redundancy found" is
// unverifiable rather than proven. A declared non-plain member counts the
// same way: product normalization can even collapse a declared form into a
// plainer stored shape (TiDB catalogs DESC ascending), so member-shape
// distinctness is never asserted on the declared side either. The gap stays
// scoped to actually-affected pairings — a non-plain member no declared
// index can pair with emits nothing.
func memberComparisonGap(declared, existing []spec.Index, pairing indexPairing) []rule.EvidenceGap {
	kindWithheld := false
	keyPartsWithheld := false
	for i := range declared {
		for j := range existing {
			switch verdict := pairing(declared[i], existing[j]); verdict {
			case indexPairKindWithheld:
				kindWithheld = true
				continue
			case indexPairApplies:
				// compare member shapes below
			default:
				continue
			}
			if indexPlainComparable(declared[i]) && indexPlainComparable(existing[j]) {
				continue
			}
			keyPartsWithheld = true
		}
	}
	var facts []string
	if kindWithheld {
		// A pair whose column shape matches but whose existing-side kind is
		// withheld could apply under either answer — "no comparison hit"
		// is unverifiable, so the member collection fact is what's owed.
		facts = append(facts, "target_table.indexes")
	}
	if keyPartsWithheld {
		facts = append(facts, indexKeyPartsGapFact)
	}
	if len(facts) > 0 {
		return []rule.EvidenceGap{{
			ReasonCode:    gapReasonUnknownTableState,
			RequiredFacts: facts,
		}}
	}
	return nil
}

// declaredComparisonGap is the create-table analogue of memberComparisonGap:
// every comparison pairs two different declarations inside the statement —
// never a member with itself — and direction stays inside the pairing
// predicate (a left-prefix subject is the shorter side, a unique-overlap
// subject the secondary). An applicable pair carrying a non-plain member on
// either side leaves the plain-key comparison unanswered, so the skipped
// comparison reports the same attributable key-parts gap the existing-set
// path emits.
func declaredComparisonGap(declared []spec.Index, pairing indexPairing) []rule.EvidenceGap {
	for i := range declared {
		for j := range declared {
			if i == j {
				continue
			}
			if pairing(declared[i], declared[j]) != indexPairApplies {
				continue
			}
			if indexPlainComparable(declared[i]) && indexPlainComparable(declared[j]) {
				continue
			}
			return []rule.EvidenceGap{{
				ReasonCode:    gapReasonUnknownTableState,
				RequiredFacts: []string{indexKeyPartsGapFact},
			}}
		}
	}
	return nil
}

func primaryKeyColumnSpecs(statement spec.Statement) []spec.Column {
	if statement.DDL == nil {
		return nil
	}
	// CREATE TABLE path: match PK columns against declared column types
	if statement.DDL.PrimaryKey != nil {
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
	return nil
}

// alterDeclaredPrimaryKeyMembers returns the member column names the ALTER
// add-primary-key action declares. MySQL/TiDB carry the structured index
// definition (alter.Index), PostgreSQL the options vocabulary, and any
// structured constraint payload (alter.Constraint) is honored the same —
// names only, never column facts; callers resolve facts through
// orderedColumnFact.
func alterDeclaredPrimaryKeyMembers(statement spec.Statement) []string {
	if !appliesToAlterTable(statement) {
		return nil
	}
	for _, alter := range statement.DDL.Alter {
		if alter.Action != "add_constraint" {
			continue
		}
		if index, ok := alterIndexDefinition(alter); ok && index.Kind == spec.IndexKindPrimary && len(index.Columns) > 0 {
			return index.Columns
		}
		if alter.Constraint != nil && alter.Constraint.Type == "primary_key" && len(alter.Constraint.Columns) > 0 {
			return alter.Constraint.Columns
		}
		if alter.Options["constraint_type"] == "primary_key" {
			if members := splitAlterConstraintColumns(alter.Options["columns"]); len(members) > 0 {
				return members
			}
		}
	}
	return nil
}

// orderedColumnFact resolves one declared member name through the
// statement's ordered pre-state column facts. Only a proven member
// answers: an absent snapshot, a settled-absent table, a withheld column
// set, or a member outside the proven set all return false so consumers
// never predicate on name-only stubs.
func orderedColumnFact(statement spec.Statement, name string) (spec.Column, bool) {
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || snapshot == nil || !snapshot.Exists || snapshot.Columns == nil {
		return spec.Column{}, false
	}
	if column := snapshot.FindColumn(name); column != nil {
		return *column, true
	}
	return spec.Column{}, false
}

func appliesToCreateTableOrAlterFKConstraint(statement spec.Statement) bool {
	if appliesToCreateTable(statement) {
		return true
	}
	if !appliesToAlterTable(statement) {
		return false
	}
	for _, alter := range statement.DDL.Alter {
		if alter.Action != "add_constraint" {
			continue
		}
		// MySQL/TiDB carry the constraint payload on alter.Constraint;
		// PostgreSQL carries it in the options vocabulary.
		if alter.Constraint != nil && alter.Constraint.Type == "foreign_key" {
			return true
		}
		if alter.Options["constraint_type"] == "foreign_key" {
			return true
		}
	}
	return false
}

func appliesToCreateTableOrAlterCheckConstraint(statement spec.Statement) bool {
	if appliesToCreateTable(statement) {
		return true
	}
	if !appliesToAlterTable(statement) {
		return false
	}
	for _, alter := range statement.DDL.Alter {
		if alter.Action == "add_constraint" && alter.Options["constraint_type"] == "check" {
			return true
		}
	}
	return false
}

func appliesToAlterAddConstraintPrimaryKey(statement spec.Statement) bool {
	if !appliesToAlterTable(statement) {
		return false
	}
	for _, alter := range statement.DDL.Alter {
		if alter.Action != "add_constraint" {
			continue
		}
		if alter.Options["constraint_type"] == "primary_key" {
			return true
		}
		if index, ok := alterIndexDefinition(alter); ok && index.Kind == spec.IndexKindPrimary {
			return true
		}
		if alter.Constraint != nil && alter.Constraint.Type == "primary_key" {
			return true
		}
	}
	return false
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

// matchingStandaloneDDLActions matches actions carried under the standalone
// DDL operations. The extractor sets DDL.Table to the target table on every
// one of them, so applicability keys off the operation and action name only.
// Callers chain it after matchingAlterActions, so the alter-table rename
// fallback never double-counts.
func matchingStandaloneDDLActions(statement spec.Statement, actions ...string) []spec.Alter {
	if statement.Kind != spec.KindDDL || statement.DDL == nil || len(statement.DDL.Alter) == 0 || len(actions) == 0 {
		return nil
	}
	switch statement.DDL.Operation {
	case spec.DDLOperationCreateIndex:
		if !containsFold(actions, "create_index") {
			return nil
		}
	case spec.DDLOperationDropIndex:
		if !containsFold(actions, "drop_index") {
			return nil
		}
	case spec.DDLOperationAlterTable:
		if !containsFold(actions, "rename_index") {
			return nil
		}
	default:
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

// alterIndexSubjectName resolves the index identity an add/create action
// declares: alter.Name on ALTER index actions, Index.Definition.Name on
// standalone create_index actions whose alter.Name the extractor leaves empty.
func alterIndexSubjectName(alter spec.Alter) string {
	if name := strings.TrimSpace(alter.Name); name != "" {
		return name
	}
	if definition, ok := alterIndexDefinition(alter); ok {
		return strings.TrimSpace(definition.Name)
	}
	return ""
}

// alterIndexTargetName resolves the existing index a drop/rename action names:
// alter.Name on ALTER actions, Index.OldName on standalone drop_index actions.
func alterIndexTargetName(alter spec.Alter) string {
	if name := strings.TrimSpace(alter.Name); name != "" {
		return name
	}
	if alter.Index != nil {
		return strings.TrimSpace(alter.Index.OldName)
	}
	return ""
}

// declaredIndexes returns the indexes the statement itself declares: a create
// table's embedded index list plus standalone CREATE INDEX definitions carried
// under the create_index alter action. The two sources never overlap — a
// create-table statement has no create_index alter actions and a standalone
// CREATE INDEX leaves DDL.Indexes empty.
func declaredIndexes(statement spec.Statement) []spec.Index {
	if statement.DDL == nil {
		return nil
	}
	indexes := make([]spec.Index, 0, len(statement.DDL.Indexes)+1)
	indexes = append(indexes, statement.DDL.Indexes...)
	if statement.DDL.Operation != spec.DDLOperationCreateIndex {
		return indexes
	}
	for _, alter := range statement.DDL.Alter {
		if !strings.EqualFold(alter.Action, "create_index") {
			continue
		}
		if definition, ok := alterIndexDefinition(alter); ok {
			indexes = append(indexes, *definition)
		}
	}
	return indexes
}

// declaredIndexNames returns the lowercased name set of declaredIndexes.
func declaredIndexNames(statement spec.Statement) map[string]bool {
	names := make(map[string]bool)
	for _, index := range declaredIndexes(statement) {
		if name := strings.ToLower(strings.TrimSpace(index.Name)); name != "" {
			names[name] = true
		}
	}
	return names
}

// existingIndexSet returns the target table's known existing index set. The
// second return value is false when the set is unknowable — no snapshot, an
// absent-table projection is knowable-empty instead, or the projection marks
// IndexesUnknown. Rules must never read an unknowable set as empty.
func existingIndexSet(statement spec.Statement) ([]spec.Index, bool) {
	snapshot, ok := targetTableSnapshot(statement)
	if !ok || snapshot == nil {
		return nil, false
	}
	if !snapshot.Exists {
		return nil, true
	}
	if snapshot.IndexesUnknown {
		return nil, false
	}
	return snapshot.Indexes, true
}

// lifecycleIndexes returns the union of the statement's declared indexes and
// the target's known existing set — the candidate set for duplicate and
// redundancy checks. Only the standalone create_index entry adds the snapshot;
// ALTER paths reach the same union through their own projection wrappers.
func lifecycleIndexes(statement spec.Statement) ([]spec.Index, bool) {
	declared := declaredIndexes(statement)
	if statement.DDL == nil || statement.DDL.Operation != spec.DDLOperationCreateIndex {
		return declared, true
	}
	existing, known := existingIndexSet(statement)
	if !known {
		return declared, false
	}
	merged := make([]spec.Index, 0, len(existing)+len(declared))
	merged = append(merged, existing...)
	merged = append(merged, declared...)
	return merged, true
}

func matchingRenameActions(statement spec.Statement, action string) []spec.Alter {
	matched := matchingAlterActions(statement, action)
	if len(matched) > 0 {
		return matched
	}
	return matchingStandaloneDDLActions(statement, action)
}

func alterRenameNames(alter spec.Alter) (oldName, newName string, ok bool) {
	switch {
	case alter.Column != nil && alter.Column.OldName != "" && alter.Column.Definition != nil && alter.Column.Definition.Name != "":
		return alter.Column.OldName, alter.Column.Definition.Name, true
	case alter.Index != nil && alter.Index.OldName != "" && alter.Index.Definition != nil && alter.Index.Definition.Name != "":
		return alter.Index.OldName, alter.Index.Definition.Name, true
	case strings.TrimSpace(alter.Name) != "":
		newName, ok := alterOptionValue(alter, "new_name")
		if !ok || strings.TrimSpace(newName) == "" {
			return "", "", false
		}
		return alter.Name, newName, true
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

// projectableAttributeKeys is the explicit whitelist of ObjectSnapshot
// attribute keys that may be projected into finding metadata. Only these
// keys are forwarded; any key not listed here is silently dropped, even if
// the provider returns it and it is not in the sensitive-key blacklist.
var projectableAttributeKeys = map[string]bool{
	"type_kind":            true,
	"extension_version":    true,
	"enabled":              true,
	"server":               true,
	"foreign_data_wrapper": true,
	"target_type":          true,
	"has_options":          true,
	"table":                true,
}

// projectObjectMetadata projects the matching ObjectSnapshot from statement
// metadata into a flat map suitable for merging into a finding's Metadata.
// Returns nil if no matching object is available.
func projectObjectMetadata(statement spec.Statement) map[string]any {
	if statement.DDL == nil || statement.Metadata == nil {
		return nil
	}
	objectType := strings.TrimSpace(statement.DDL.ObjectType)
	objectName := strings.TrimSpace(statement.DDL.ObjectName)
	if objectType == "" || objectName == "" {
		return nil
	}
	snap := statement.Metadata.FindObject(objectType, objectName)
	if snap == nil {
		return nil
	}
	result := map[string]any{
		"metadata_status":      string(snap.Status),
		"metadata_exists":      snap.Exists,
		"metadata_object_type": snap.Type,
		"metadata_object_name": snap.Name,
	}
	if snap.Schema != "" {
		result["metadata_schema"] = snap.Schema
	}
	if len(snap.AmbiguousCandidates) > 0 {
		result["metadata_ambiguous_candidates"] = snap.AmbiguousCandidates
	}
	for k, v := range snap.SafeAttributes() {
		if projectableAttributeKeys[k] {
			result["metadata_"+k] = v
		}
	}
	return result
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
	if !appliesToAlterActions(statement, "add_constraint", "add_index") {
		return nil
	}

	indexes := make([]spec.Index, 0)
	for _, alter := range matchingAlterActions(statement, "add_constraint", "add_index") {
		index, ok := alterConstraintIndex(alter)
		if !ok || index.Kind != kind {
			continue
		}
		indexes = append(indexes, index)
	}
	return indexes
}

// alterAddedIndexSyntaxByKind returns added indexes declared with index
// syntax — the add_index action covers ADD [UNIQUE|FULLTEXT] INDEX|KEY in
// both single and list form. Explicit CONSTRAINT syntax (add_constraint)
// names a constraint and is governed by the ddl.constraint.* naming family;
// its produced index still participates in the physical checks through
// allAlterAddedIndexes and the lifecycle candidate set. PostgreSQL keeps its
// pinned contract: ALTER TABLE has no index-add syntax there, so
// add_constraint names stay with the add_index naming family on that dialect.
func alterAddedIndexSyntaxByKind(statement spec.Statement, kind spec.IndexKind) []spec.Index {
	actions := []string{"add_index"}
	if statement.Dialect == spec.DialectPostgreSQL {
		actions = append(actions, "add_constraint")
	}
	if !appliesToAlterActions(statement, actions...) {
		return nil
	}

	indexes := make([]spec.Index, 0)
	for _, alter := range matchingAlterActions(statement, actions...) {
		index, ok := alterConstraintIndex(alter)
		if !ok || index.Kind != kind {
			continue
		}
		indexes = append(indexes, index)
	}
	return indexes
}

// alterConstraintSubjectName resolves the constraint name an add_constraint
// action declares when the constraint produces an index of the given kind.
// Constraint.Name is authoritative; alter.Name and the produced index name
// are the extractor's fallbacks for the same identity.
func alterConstraintSubjectName(alter spec.Alter, kind spec.IndexKind) string {
	index, ok := alterConstraintIndex(alter)
	if !ok || index.Kind != kind {
		return ""
	}
	if alter.Constraint != nil && strings.TrimSpace(alter.Constraint.Name) != "" {
		return strings.TrimSpace(alter.Constraint.Name)
	}
	if name := strings.TrimSpace(alter.Name); name != "" {
		return name
	}
	return strings.TrimSpace(index.Name)
}

func splitAlterConstraintColumns(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	columns := make([]string, 0, len(parts))
	for _, part := range parts {
		column := strings.TrimSpace(part)
		if column != "" {
			columns = append(columns, column)
		}
	}
	return columns
}

func alterConstraintIndex(alter spec.Alter) (spec.Index, bool) {
	if index, ok := alterIndexDefinition(alter); ok {
		return *index, true
	}
	ct := alter.Options["constraint_type"]
	if ct != "unique" && ct != "primary_key" {
		return spec.Index{}, false
	}
	cols := splitAlterConstraintColumns(alter.Options["columns"])
	if len(cols) == 0 {
		return spec.Index{}, false
	}
	kind := spec.IndexKindSecondary
	switch ct {
	case "unique":
		kind = spec.IndexKindUnique
	case "primary_key":
		kind = spec.IndexKindPrimary
	}
	return spec.Index{Name: alter.Name, Kind: kind, Columns: cols}, true
}

func projectedAlterIndexesStatement(statement spec.Statement, indexes []spec.Index) spec.Statement {
	projected := statement
	projected.DDL = &spec.DDL{
		Operation: spec.DDLOperationCreateTable,
		Table:     statement.DDL.Table,
		Indexes:   indexes,
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
