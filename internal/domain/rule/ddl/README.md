# Domain DDL Rule Module

Expanded DDL rule catalog for create-table governance, table options/object shape, and action-level alter restrictions.

## Files

| File | Responsibility |
|------|---------------|
| common.go | Shared DDL rule IDs plus parser-neutral alter matching (including add-index actions), standalone DDL action matching, explicit-change, rename, standalone rename-name extraction, option, target-type-family, alter-index projection helpers, and `projectObjectMetadata` for projecting `spec.Metadata.Objects` validation state into finding metadata maps, including pinned Milestone 4 create-table superset IDs and `hasColumnConstraint` helper for column-constraint checks |
| common_test.go | Verifies richer alter helper boundaries and future alter rule IDs remain stable |
| config.go | Parses policy params for DDL rule constructors, including normalized string-list, structured naming requirements, and bounded integer helpers for upcoming alter semantics |
| table_rules.go | Implements table comment and table name rules |
| primary_key_rules.go | Implements primary-key presence and column-count rules |
| primary_key_semantic_rules.go | Implements bigint/unsigned/auto-increment/not-null primary-key semantic rules |
| column_rules.go | Implements table-column count and column-level governance rules |
| audit_column_rules.go | Implements audit timestamp column rules |
| identifier_rules.go | Implements create-table identifier-pattern, reserved-keyword, and reusable naming-governance primitives for prefix/suffix/contains checks |
| index_rules.go | Implements create-table index count, prefix, suffix, contains, and duplicate-index rules |
| type_family_rules.go | Implements create-table type-family, char-length, and charset/collation rules |
| alter_rules.go | Implements action-level ALTER TABLE restriction rules plus approved standalone DDL action reuse for PostgreSQL DROP INDEX |
| database_lifecycle_rules.go | Implements MySQL/TiDB database lifecycle rules: create-database notice, drop-database warning |
| database_lifecycle_rules_test.go | Verifies MySQL/TiDB database lifecycle rules with positive, negative, cross-dialect, registration, and defaults coverage |
| mysql_tidb_lifecycle_rules.go | Implements MySQL/TiDB standalone lifecycle notices plus create-index notice reuse for normalized ALTER add-index actions, with normalized object-identifier messages |
| postgresql_migration_rules.go | Implements PostgreSQL-only migration-safety rules: concurrent index, NOT NULL without default, concurrent unique constraint, drop constraint advisory, NOT VALID validation, set-data-type rewrite, add-column with volatile default, and add-check without NOT VALID |
| postgresql_object_lifecycle_rules.go | Implements PostgreSQL-only object lifecycle rules: create-schema notice, drop-schema advisory/cascade, create-sequence cycle, alter-sequence restart/cycle, drop-sequence advisory/cascade, drop-materialized-view advisory/cascade |
| postgresql_materialized_view_refresh_rules.go | Implements PostgreSQL-only materialized view refresh rules: non-concurrent refresh warning, WITH NO DATA notice |
| postgresql_alter_table_rules.go | Implements PostgreSQL-only alter table gap rules: drop-column advisory, validate-constraint advisory, add-column nullable notice, set-schema advisory, owner advisory, enable-trigger notice, disable-trigger warn, attach-partition advisory, detach-partition warn |
| postgresql_type_lifecycle_rules.go | Implements PostgreSQL-only type lifecycle rules: create-type enum/composite notice, alter-type add-value advisory, alter-type add-value position notice, alter-type composite rename/set-schema notice, alter-type composite add/drop/alter/rename attribute notice/warn, drop-type advisory, drop-type cascade warn |
| postgresql_domain_lifecycle_rules.go | Implements PostgreSQL-only domain lifecycle rules: create-domain notice, alter-domain constraint/default/not-null/rename notices, drop-domain advisory, drop-domain cascade warn |
| postgresql_extension_lifecycle_rules.go | Implements PostgreSQL-only extension lifecycle rules: create-extension notice, create-extension cascade warn, alter-extension update/set-schema/add-member/drop-member notices/warnings, drop-extension advisory, drop-extension cascade warn |
| postgresql_replication_lifecycle_rules.go | Implements PostgreSQL-only publication/subscription lifecycle rules: create/alter/drop publication notices/warnings, create/alter/drop subscription notices/warnings, alter-subscription disable warning |
| postgresql_privilege_rules.go | Implements PostgreSQL-only table privilege rules: grant-table notice, grant-table all-privileges warn, revoke-table notice, revoke-table cascade warn |
| postgresql_privilege_rules_test.go | Verifies PG table privilege rules with positive, negative, cross-dialect, deferred-form, registration, and defaults coverage |
| metadata_rules.go | Implements metadata-backed table, column, index, and primary-key existence rules |
| object_lifecycle_rules.go | Implements create-view, drop-table, truncate-table, metadata-backed lifecycle existence, and adaptive-hash caution rules |
| merge_alter_rules.go | Implements global merge-alter governance across statement batches |
| denylist_rules.go | Implements DDL table denylist checks against protected schemas or tables |
| size_rules.go | Implements metadata-backed rough row-size and index-key-length checks for create-table statements |
| alter_compatibility_rules.go | Implements source-aware compatibility checks for metadata-backed change/modify column operations |
| alter_semantic_rules.go | Implements rename-index forbids, explicit alter-column change forbids, normalized add-index naming/lifecycle rules, and conservative alter target-type-family rules |
| table_option_rules.go | Implements create-table option, foreign-key, and object-shape rules |
| register.go | Registers enabled DDL rules into the shared registry, including shipped alter-added index lifecycle rules |
| table_rules_test.go | Verifies table comment and name-length rule behavior |
| primary_key_rules_test.go | Verifies primary-key requirement and shape rules |
| primary_key_semantic_rules_test.go | Verifies primary-key semantic rules for bigint/unsigned/auto-increment/not-null requirements |
| column_rules_test.go | Verifies column-count, comment, naming, default, nullability, and type rules |
| audit_column_rules_test.go | Verifies audit timestamp column rules |
| identifier_rules_test.go | Verifies create-table identifier-pattern, reserved-keyword, and reusable naming primitive behavior |
| index_rules_test.go | Verifies create-table index governance rules |
| type_family_rules_test.go | Verifies create-table type-family, char-length, and charset/collation rules |
| alter_rules_test.go | Verifies action-level ALTER TABLE restriction rules |
| mysql_tidb_alter_action_rules.go | Implements MySQL/TiDB ALTER TABLE action notices, including hypothetical DROP COLUMN wording that does not claim the column exists |
| mysql_tidb_alter_action_rules_test.go | Verifies DROP COLUMN notice copy stays hypothetical and does not assert live-schema existence |
| metadata_rules_test.go | Verifies metadata-backed table, column, index, and primary-key existence rules |
| object_lifecycle_rules_test.go | Verifies create-view, drop-table, truncate-table, metadata-backed lifecycle existence, and adaptive-hash caution rules |
| merge_alter_rules_test.go | Verifies global merge-alter governance rules |
| alter_compatibility_rules_test.go | Verifies source-aware compatibility checks for change/modify column operations |
| size_rules_test.go | Verifies metadata-backed row-size and index-key-length checks |
| alter_semantic_rules_test.go | Verifies semantic alter rename-index, explicit alter-column change, alter-added index lifecycle, and conservative target-type-family rules plus registration order |
| table_option_rules_test.go | Verifies create-table option and object-shape rules |
| register_test.go | Verifies policy-backed DDL rule registration and deterministic ordering |
| postgresql_object_lifecycle_rules_test.go | Verifies PG object lifecycle rules with positive, negative, cross-dialect, and registration coverage |
| postgresql_materialized_view_refresh_rules_test.go | Verifies PG refresh materialized view rules with positive, negative, cross-dialect, registration, and defaults coverage |
| postgresql_alter_table_rules_test.go | Verifies PG alter table gap rules with positive, negative, cross-dialect, registration, and defaults coverage |
| postgresql_type_lifecycle_rules_test.go | Verifies PG type lifecycle rules with positive, negative, cross-dialect, registration, and defaults coverage |
| postgresql_domain_lifecycle_rules_test.go | Verifies PG domain lifecycle rules with positive, negative, cross-dialect, registration, and defaults coverage |
| postgresql_extension_lifecycle_rules_test.go | Verifies PG extension lifecycle rules with positive, negative, cross-dialect, registration, and defaults coverage |
| postgresql_function_lifecycle_rules.go | Implements PostgreSQL-only function/procedure lifecycle rules: create-function notice, create-function security-definer warn, create-or-replace-function advisory, drop-function advisory, create-procedure notice, drop-procedure advisory |
| postgresql_advanced_view_lifecycle_rules.go | Implements PostgreSQL-only advanced view lifecycle rules: create-or-replace-view advisory, create-temp-view notice, create-view check-option notice, alter-view rename notice, alter-view set-schema notice, drop-view cascade warn |
| postgresql_foreign_object_lifecycle_rules.go | Implements PostgreSQL-only foreign object lifecycle rules: create/alter/drop foreign table, foreign server, user mapping, and foreign data wrapper notices/warnings |
| postgresql_annotation_lifecycle_rules.go | Implements PostgreSQL-only annotation lifecycle rules: comment-on notice, comment-on remove notice, security-label notice, security-label remove notice |
| postgresql_annotation_lifecycle_rules_test.go | Verifies PG annotation lifecycle rules with positive, negative, cross-dialect, null/non-null skip, registration, and defaults coverage |
| postgresql_event_rule_lifecycle_rules.go | Implements PostgreSQL-only event trigger and rewrite rule lifecycle rules: create/alter/drop event trigger notices/warnings, create/alter/drop rule notices/warnings |
| postgresql_event_rule_lifecycle_rules_test.go | Verifies PG event trigger and rule lifecycle rules with positive, negative, cross-dialect, action-specific disable-vs-enable, registration, and defaults coverage |
| postgresql_collation_lifecycle_rules.go | Implements PostgreSQL-only collation lifecycle rules: create/alter collation notices, drop collation warning |
| postgresql_collation_lifecycle_rules_test.go | Verifies PG collation lifecycle rules with positive, cross-dialect, wrong-operation, and registry coverage |
| postgresql_statistics_lifecycle_rules.go | Implements PostgreSQL-only statistics lifecycle rules: create/alter statistics notices, drop statistics warning |
| postgresql_statistics_lifecycle_rules_test.go | Verifies PG statistics lifecycle rules with positive, cross-dialect, wrong-operation, and registry coverage |
| postgresql_semantic_object_lifecycle_rules.go | Implements PostgreSQL-only aggregate, operator, and conversion lifecycle rules: create/alter notices, drop warnings |
| postgresql_semantic_object_lifecycle_rules_test.go | Verifies PG aggregate/operator/conversion lifecycle rules with positive, cross-dialect, wrong-operation, and registry coverage |
| postgresql_operator_family_lifecycle_rules.go | Implements PostgreSQL-only operator class/family lifecycle rules: create/alter operator family/class notices, drop operator family/class warnings |
| postgresql_operator_family_lifecycle_rules_test.go | Verifies PG operator family/class lifecycle rules with positive, cross-dialect, wrong-operation, and registry coverage |
| postgresql_text_search_lifecycle_rules.go | Implements PostgreSQL-only text search lifecycle rules: create/alter text search configuration/dictionary/parser/template notices, drop text search configuration/dictionary/parser/template warnings |
| postgresql_text_search_lifecycle_rules_test.go | Verifies PG text search lifecycle rules with positive, cross-dialect, wrong-operation, and registry coverage |
| object_metadata_projection_test.go | Verifies `projectObjectMetadata` helper and object metadata projection across all PG lifecycle rule families covering confirmed, not_found, ambiguous, unavailable statuses, sensitive attribute filtering, and finding count preservation |

## Exports

- `Register(registry *rule.Registry, cfg policy.Policy) error`

## Dependencies
- Upstream: future application rule assembly and higher-level audit services
- Downstream: `internal/domain/policy`, `internal/domain/rule`, `internal/domain/spec`

## Rule IDs

- `ddl.table.comment.require`
- `ddl.table.name.max_length`
- `ddl.table.name.prefix.require`
- `ddl.table.name.suffix.require`
- `ddl.table.name.contains.require`
- `ddl.table.primary_key.require`
- `ddl.table.primary_key.columns.max_count`
- `ddl.table.primary_key.bigint.require`
- `ddl.table.primary_key.unsigned.require`
- `ddl.table.primary_key.auto_increment.require`
- `ddl.table.primary_key.not_null.require`
- `ddl.table.columns.min_count`
- `ddl.table.audit_columns.require`
- `ddl.column.comment.require`
- `ddl.column.name.max_length`
- `ddl.column.name.prefix.require`
- `ddl.column.name.suffix.require`
- `ddl.column.name.contains.require`
- `ddl.table.name.pattern.require`
- `ddl.column.name.pattern.require`
- `ddl.index.name.pattern.require`
- `ddl.table.name.keyword.forbid`
- `ddl.column.name.keyword.forbid`
- `ddl.index.name.keyword.forbid`
- `ddl.constraint.primary_key.name.prefix.require`
- `ddl.constraint.primary_key.name.suffix.require`
- `ddl.constraint.primary_key.name.contains.require`
- `ddl.constraint.unique_key.name.prefix.require`
- `ddl.constraint.unique_key.name.suffix.require`
- `ddl.constraint.unique_key.name.contains.require`
- `ddl.constraint.foreign_key.name.prefix.require`
- `ddl.constraint.foreign_key.name.suffix.require`
- `ddl.constraint.foreign_key.name.contains.require`
- `ddl.constraint.check.name.prefix.require`
- `ddl.constraint.check.name.suffix.require`
- `ddl.constraint.check.name.contains.require`
- `ddl.column.varchar.max_length`
- `ddl.column.blob_text.forbid`
- `ddl.column.json.forbid`
- `ddl.column.bit.forbid`
- `ddl.column.timestamp.forbid`
- `ddl.column.char.max_length`
- `ddl.column.charset.allowlist`
- `ddl.column.collation.allowlist`
- `ddl.column.charset_collation.match.require`
- `ddl.column.default.require`
- `ddl.column.not_null.require`
- `ddl.column.float_double.forbid`
- `ddl.index.total.max_count`
- `ddl.index.columns.max_count`
- `ddl.index.unique.prefix.require`
- `ddl.index.unique.suffix.require`
- `ddl.index.unique.contains.require`
- `ddl.index.secondary.prefix.require`
- `ddl.index.secondary.suffix.require`
- `ddl.index.secondary.contains.require`
- `ddl.index.fulltext.prefix.require`
- `ddl.index.fulltext.suffix.require`
- `ddl.index.fulltext.contains.require`
- `ddl.index.key_length.max_bytes.require`
- `ddl.index.duplicate.forbid`
- `ddl.index.redundant_left_prefix.forbid`
- `ddl.index.redundant_unique_overlap.forbid`
- `ddl.alter.drop_column.forbid`
- `ddl.alter.drop_primary_key.forbid`
- `ddl.alter.drop_index.forbid`
- `ddl.alter.rename_table.forbid`
- `ddl.alter.rename_column.forbid`
- `ddl.alter.change_column.forbid`
- `ddl.alter.modify_column.forbid`
- `ddl.alter.rename_index.forbid`
- `ddl.alter.modify_column.target_type_family.allowlist`
- `ddl.alter.change_column.target_type_family.allowlist`
- `ddl.alter.modify_column.compatibility.require`
- `ddl.alter.change_column.compatibility.require`
- `ddl.alter.table_option.compatibility.require`
- `ddl.alter.modify_column.explicit_nullability_change.forbid`
- `ddl.alter.change_column.explicit_nullability_change.forbid`
- `ddl.alter.modify_column.explicit_default_change.forbid`
- `ddl.alter.change_column.explicit_default_change.forbid`
- `ddl.alter.modify_column.explicit_auto_increment_change.forbid`
- `ddl.alter.change_column.explicit_auto_increment_change.forbid`
- `ddl.alter.add_index.columns.max_count`
- `ddl.alter.add_index.duplicate.forbid`
- `ddl.alter.add_index.redundant_left_prefix.forbid`
- `ddl.alter.add_index.redundant_unique_overlap.forbid`
- `ddl.alter.add_index.unique.prefix.require`
- `ddl.alter.add_index.unique.suffix.require`
- `ddl.alter.add_index.unique.contains.require`
- `ddl.alter.add_index.secondary.prefix.require`
- `ddl.alter.add_index.secondary.suffix.require`
- `ddl.alter.add_index.secondary.contains.require`
- `ddl.alter.add_index.fulltext.prefix.require`
- `ddl.alter.add_index.fulltext.suffix.require`
- `ddl.alter.add_index.fulltext.contains.require`
- `ddl.table.comment.max_length`
- `ddl.table.engine.allowlist`
- `ddl.table.row_size.max_bytes.require`
- `ddl.table.charset.allowlist`
- `ddl.table.row_format.allowlist`
- `ddl.table.auto_increment.init_value.require`
- `ddl.table.foreign_key.forbid`
- `ddl.table.partition.forbid`
- `ddl.table.create_like.forbid`
- `ddl.table.create_as.forbid`
- `ddl.view.create.forbid`
- `ddl.table.drop.forbid`
- `ddl.table.drop.exists.require`
- `ddl.table.drop.adaptive_hash.warn`
- `ddl.table.drop.rows.max_count`
- `ddl.table.truncate.forbid`
- `ddl.table.truncate.exists.require`
- `ddl.table.truncate.adaptive_hash.warn`
- `ddl.table.truncate.rows.max_count`
- `ddl.alter.merge.mysql.require`
- `ddl.alter.merge.tidb.require`
- `ddl.table.denylist.forbid`
- `ddl.table.exists.create.forbid`
- `ddl.table.exists.alter.require`
- `ddl.alter.add_column.exists.forbid`
- `ddl.alter.drop_column.exists.require`
- `ddl.alter.modify_column.exists.require`
- `ddl.alter.change_column.exists.require`
- `ddl.alter.rename_column.exists.require`
- `ddl.alter.add_index.exists.forbid`
- `ddl.alter.drop_index.exists.require`
- `ddl.alter.rename_index.exists.require`
- `ddl.alter.drop_primary_key.exists.require`
- `ddl.database.create.notice`
- `ddl.database.drop.warn`
- `ddl.pg.create_schema.notice`
- `ddl.pg.drop_index.advisory`
- `ddl.pg.alter.add_column.non_null_no_default.warn`
- `ddl.pg.alter.add_unique_constraint.concurrent_index.advisory`
- `ddl.pg.alter.drop_constraint.advisory`
- `ddl.pg.drop_schema.advisory`
- `ddl.pg.drop_schema.cascade.warn`
- `ddl.pg.create_sequence.cycle.warn`
- `ddl.pg.alter_sequence.restart.warn`
- `ddl.pg.alter_sequence.cycle.warn`
- `ddl.pg.drop_sequence.advisory`
- `ddl.pg.drop_sequence.cascade.warn`
- `ddl.pg.drop_materialized_view.advisory`
- `ddl.pg.drop_materialized_view.cascade.warn`
- `ddl.pg.refresh_materialized_view.concurrently.warn`
- `ddl.pg.refresh_materialized_view.no_data.notice`
- `ddl.pg.alter.drop_column.advisory`
- `ddl.pg.alter.validate_constraint.advisory`
- `ddl.pg.alter.add_column.nullable.notice`
- `ddl.pg.alter.set_schema.advisory`
- `ddl.pg.alter.owner.advisory`
- `ddl.pg.alter.enable_trigger.notice`
- `ddl.pg.alter.disable_trigger.warn`
- `ddl.pg.alter.attach_partition.advisory`
- `ddl.pg.alter.detach_partition.warn`
- `ddl.pg.alter.set_logged.notice`
- `ddl.pg.alter.set_unlogged.notice`
- `ddl.pg.alter.replica_identity_full.warn`
- `ddl.pg.alter.replica_identity_nothing.warn`
- `ddl.pg.alter.replica_identity_using_index.notice`
- `ddl.pg.create_type.enum.notice`
- `ddl.pg.alter_type.add_value.advisory`
- `ddl.pg.alter_type.add_value.position.notice`
- `ddl.pg.drop_type.advisory`
- `ddl.pg.drop_type.cascade.warn`
- `ddl.pg.create_type.composite.notice`
- `ddl.pg.alter_type.composite_rename.notice`
- `ddl.pg.alter_type.composite_set_schema.notice`
- `ddl.pg.alter_type.add_attribute.notice`
- `ddl.pg.alter_type.drop_attribute.warn`
- `ddl.pg.alter_type.alter_attribute_type.warn`
- `ddl.pg.alter_type.rename_attribute.notice`
- `ddl.pg.create_domain.notice`
- `ddl.pg.alter_domain.constraint.notice`
- `ddl.pg.alter_domain.default.notice`
- `ddl.pg.alter_domain.not_null.notice`
- `ddl.pg.alter_domain.rename.notice`
- `ddl.pg.drop_domain.advisory`
- `ddl.pg.drop_domain.cascade.warn`
- `ddl.pg.grant.table_privilege.notice`
- `ddl.pg.grant.table_privilege.all.warn`
- `ddl.pg.revoke.table_privilege.notice`
- `ddl.pg.revoke.table_privilege.cascade.warn`
- `ddl.pg.create_extension.notice`
- `ddl.pg.create_extension.cascade.warn`
- `ddl.pg.alter_extension.update.notice`
- `ddl.pg.alter_extension.set_schema.notice`
- `ddl.pg.drop_extension.advisory`
- `ddl.pg.drop_extension.cascade.warn`
- `ddl.pg.alter_extension.add_member.notice`
- `ddl.pg.alter_extension.drop_member.warn`
- `ddl.pg.create_publication.notice`
- `ddl.pg.alter_publication.notice`
- `ddl.pg.drop_publication.warn`
- `ddl.pg.create_subscription.notice`
- `ddl.pg.alter_subscription.notice`
- `ddl.pg.alter_subscription.disable.warn`
- `ddl.pg.drop_subscription.warn`
- `ddl.pg.create_function.notice`
- `ddl.pg.create_function.security_definer.warn`
- `ddl.pg.create_or_replace_function.advisory`
- `ddl.pg.drop_function.advisory`
- `ddl.pg.create_procedure.notice`
- `ddl.pg.drop_procedure.advisory`

- `ddl.pg.create_statistics.notice`
- `ddl.pg.alter_statistics.notice`
- `ddl.pg.drop_statistics.warn`

## Milestone 4 Planned Create-Table Surface

Milestone 4 is the remaining create-table breadth push. The following rule IDs are pinned now so later tasks can implement them without churn. These IDs are planned surface, not shipped behavior yet.

Milestone 4 is intentionally still create-table scoped:

- no live metadata dependencies
- no new parser-owned domain leakage
- no claims of source-to-target alter compatibility

Within that create-table scope, Milestone 4 is the line where DeltaScope aims to exceed `gAudit` specifically on `CREATE TABLE` breadth by closing the remaining offline-safe gaps around:

- identifier syntax and reserved-keyword governance
- blob/json/bit/timestamp and char-length type-family policy
- column charset/collation allowlists plus charset-collation coherence
- deeper redundant-index detection beyond exact duplicates
- row-format and auto-increment-init table-option checks

## Alter Helper Surface

Shared alter helpers in `common.go` now cover the richer parser-neutral alter stream:

- applicability checks for selected alter actions
- locating matching alter records by action
- matching closed-switch standalone DDL action payloads such as PostgreSQL `DROP INDEX` and standalone `ALTER INDEX ... RENAME TO ...`
- extracting target column and index definitions
- extracting explicit statement-local column change facts
- checking whether nullability/default/auto-increment changes are explicitly requested
- extracting old/new names for rename-style alters
- detecting column renames from parser-neutral names only
- reading normalized table-option values
- classifying target column types into coarse families without claiming source-to-target compatibility
- projecting alter-added indexes into parser-neutral index lists for shared add-index governance
- projecting all alter-added indexes into parser-neutral index lists for shared lifecycle governance

## Semantic Alter Surface

The first semantic alter batch currently covers:

- `ddl.alter.rename_index.forbid`
- `ddl.alter.modify_column.explicit_nullability_change.forbid`
- `ddl.alter.change_column.explicit_nullability_change.forbid`
- `ddl.alter.modify_column.explicit_default_change.forbid`
- `ddl.alter.change_column.explicit_default_change.forbid`
- `ddl.alter.modify_column.explicit_auto_increment_change.forbid`
- `ddl.alter.change_column.explicit_auto_increment_change.forbid`
- `ddl.alter.add_index.unique.prefix.require`
- `ddl.alter.add_index.unique.suffix.require`
- `ddl.alter.add_index.unique.contains.require`
- `ddl.alter.add_index.secondary.prefix.require`
- `ddl.alter.add_index.secondary.suffix.require`
- `ddl.alter.add_index.secondary.contains.require`
- `ddl.alter.add_index.fulltext.prefix.require`
- `ddl.alter.add_index.fulltext.suffix.require`
- `ddl.alter.add_index.fulltext.contains.require`
- `ddl.alter.add_index.columns.max_count`
- `ddl.alter.add_index.duplicate.forbid`
- `ddl.alter.add_index.redundant_left_prefix.forbid`
- `ddl.alter.add_index.redundant_unique_overlap.forbid`
- `ddl.alter.modify_column.target_type_family.allowlist`
- `ddl.alter.change_column.target_type_family.allowlist`

## Object-Scope Denylist Surface

The DDL denylist rule is intentionally simple and policy-driven:

- `ddl.table.denylist.forbid`
- matches by `schemas`, `tables`, or `qualified_tables`
- consumes request schema context from `statement.Metadata.Schema`
- stays inert by default because the shipped policy keeps those lists empty

The `target_type_family.allowlist` rules are intentionally conservative in offline mode:

- they inspect only the extracted target column definition
- they use coarse target type-family allowlists
- they do not claim to prove end-to-end source-to-target compatibility without live schema context
- under the default policy, `ddl.alter.change_column.forbid` stays stricter, so the change-column allowlist acts as a follow-on guard only after that coarse forbid is intentionally relaxed

The explicit `*_change.forbid` rules are narrower:

- they only fire when the statement-local extractor marked a nullability/default/auto_increment change explicitly
- they do not claim to prove wider type or unsigned transitions
- the `change_column` variants also sit behind the stricter default `ddl.alter.change_column.forbid` gate unless a team intentionally relaxes it

The alter-added index lifecycle rules reuse the existing create-table index rule bodies by projecting `ADD CONSTRAINT` index payloads into temporary parser-neutral index lists before evaluation.

Within this batch:

- prefix checks reuse the existing create-table prefix rule body
- add-index width checks reuse the existing create-table index-column-count rule body
- add-index duplicate checks reuse the existing create-table duplicate-index rule body
- add-index redundant-index checks now also reuse the existing create-table left-prefix and unique-overlap rule bodies by projecting current snapshot indexes plus newly added indexes into one lifecycle view

Those alter-added index lifecycle rules are part of the normal `Register(...)` path when their policies are enabled.

The next source-aware alter batch is intentionally prepared with honest names:

- `explicit_*_change.forbid` rule IDs only claim statement-local explicit change detection
- `target_type_family.allowlist` IDs still describe target-side allowlists, not source-to-target compatibility
- alter-add-index helper seams project parser-neutral index payloads without inventing live schema state

## Source-Aware Compatibility Surface

The first source-aware alter compatibility batch now covers:

- same-family change/modify operations that shrink string length
- integer-width narrowing
- signedness changes
- nullable to not-null tightening
- auto-increment removal
- family changes between source and target column definitions
- `ALTER TABLE ...` option changes for `engine`, `charset`, `collation`, `row_format`, and `auto_increment` against the current snapshot

These compatibility rules are intentionally limited:

- they require a live `TargetTable` snapshot
- they compare only current source shape and requested target shape
- they do not claim row-level data validation or full runtime safety

## Metadata-Backed Existence Surface

The first metadata-backed DDL rule batch covers:

- create-table target table already exists
- alter-table target table must exist
- add-column target already exists
- drop/modify/change/rename column target must exist
- add-index target already exists
- drop/rename index target must exist
- drop primary key target must exist

These rules are intentionally metadata-gated:

- the shipped default policy keeps them enabled, but offline audits still skip them when no live table snapshot is attached
- offline audits skip them when no live table snapshot is attached
- `policy.Enabled` still controls whether the rule is registered at all
- in metadata-aware mode they consume the normalized `TargetTable` snapshot carried on `spec.Statement.Metadata`

## Metadata-Backed Sizing Surface

The current sizing rules are intentionally rough but honest:

- `ddl.table.row_size.max_bytes.require`
- `ddl.index.key_length.max_bytes.require`
- they require instance facts for default charset, default row format, and large-prefix behavior
- they estimate maximum row and key length from parser-neutral column definitions
- they are designed as conservative preflight guards, not as execution-plan or storage-engine simulators

## Object Lifecycle Surface

The lifecycle rule batch now also covers:

- `ddl.view.create.forbid`
- `ddl.table.drop.forbid`
- `ddl.table.drop.exists.require`
- `ddl.table.drop.adaptive_hash.warn`
- `ddl.table.truncate.forbid`
- `ddl.table.truncate.exists.require`
- `ddl.table.truncate.adaptive_hash.warn`

Within this batch:

- create-view, drop-table, and truncate-table statements are distinguished through `spec.DDL.Operation`
- drop/truncate existence checks remain metadata-gated and only fire when a live `TargetTable` snapshot is attached
- adaptive-hash cautions remain metadata-gated and only fire when instance facts report `innodb_adaptive_hash_index=ON`
- row-count cautions remain metadata-gated and only fire when the snapshot carries `table_rows`

## Merge Alter Surface

The global merge-alter rule batch now covers:

- `ddl.alter.merge.mysql.require`
- `ddl.alter.merge.tidb.require`

These rules evaluate full statement batches instead of individual statements:

- repeated `ALTER TABLE` statements on the same table are grouped per dialect
- MySQL merge enforcement is enabled by default
- TiDB merge enforcement is shipped but relaxed by default

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
