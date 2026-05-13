// Package ddl defines Tier-1 DDL rules.
// input: normalized Statement specs emitted by application extraction, including alter-table and standalone rename payloads
// output: reusable DDL rule predicates and rule identifier constants plus shared rename matching helpers
// pos: DDL rule common helpers shared across concrete rules and parser-neutral rename semantics
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
	ruleIDAlterSetNotNullExplicitNullabilityChangeForbid     = "ddl.alter.set_not_null.explicit_nullability_change.forbid"
	ruleIDAlterDropNotNullExplicitNullabilityChangeForbid    = "ddl.alter.drop_not_null.explicit_nullability_change.forbid"
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
	ruleIDPGCreateTypeEnumNotice                             = "ddl.pg.create_type.enum.notice"
	ruleIDPGAlterTypeAddValueAdvisory                        = "ddl.pg.alter_type.add_value.advisory"
	ruleIDPGAlterTypeAddValuePositionNotice                  = "ddl.pg.alter_type.add_value.position.notice"
	ruleIDPGDropTypeAdvisory                                 = "ddl.pg.drop_type.advisory"
	ruleIDPGDropTypeCascadeWarn                              = "ddl.pg.drop_type.cascade.warn"
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

func appliesToDDLWithIndexes(statement spec.Statement) bool {
	if statement.Kind != spec.KindDDL || statement.DDL == nil || statement.DDL.Table == nil || len(statement.DDL.Indexes) == 0 {
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
	// ALTER TABLE ADD CONSTRAINT PRIMARY KEY path: name-only specs (no type info)
	for _, alter := range statement.DDL.Alter {
		if alter.Action == "add_constraint" && alter.Options["constraint_type"] == "primary_key" {
			if cols := splitAlterConstraintColumns(alter.Options["columns"]); len(cols) > 0 {
				columns := make([]spec.Column, 0, len(cols))
				for _, name := range cols {
					columns = append(columns, spec.Column{Name: name})
				}
				return columns
			}
		}
	}
	return nil
}

func appliesToCreateTableOrAlterFKConstraint(statement spec.Statement) bool {
	if appliesToCreateTable(statement) {
		return true
	}
	if !appliesToAlterTable(statement) {
		return false
	}
	for _, alter := range statement.DDL.Alter {
		if alter.Action == "add_constraint" && alter.Options["constraint_type"] == "foreign_key" {
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
		if alter.Action == "add_constraint" && alter.Options["constraint_type"] == "primary_key" {
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

func matchingStandaloneDDLActions(statement spec.Statement, actions ...string) []spec.Alter {
	if statement.Kind != spec.KindDDL || statement.DDL == nil || statement.DDL.Table != nil || len(statement.DDL.Alter) == 0 || len(actions) == 0 {
		return nil
	}
	switch statement.DDL.Operation {
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
	if !appliesToAlterActions(statement, "add_constraint") {
		return nil
	}

	indexes := make([]spec.Index, 0)
	for _, alter := range matchingAlterActions(statement, "add_constraint") {
		index, ok := alterConstraintIndex(alter)
		if !ok || index.Kind != kind {
			continue
		}
		indexes = append(indexes, index)
	}
	return indexes
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
