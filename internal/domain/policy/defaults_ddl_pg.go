// Package policy defines audit policy configuration in domain terms.
// input: built-in rule identifiers and default severity/parameter choices
// output: baseline policy values used when no external config is supplied
// pos: domain default policy factory for v1 audit behavior
// note: if this file changes, update this header and module README.md.
package policy

import "github.com/Fanduzi/DeltaScope/internal/domain/rule"

func ddlPgRules() map[string]RulePolicy {
	return map[string]RulePolicy{
		// PG migration-safety rules (PostgreSQL-only).
		"ddl.pg.create_index.concurrently.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.alter.add_column.non_null_default.rewrite.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.alter.add_check.not_valid.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.alter.set_data_type.rewrite.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.table.foreign_key.cross_schema.advisory": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter.not_valid_constraint.validate.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"required": true,
			},
		},
		"ddl.pg.drop_index.advisory": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter.add_column.non_null_no_default.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.alter.add_unique_constraint.concurrent_index.advisory": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter.drop_constraint.advisory": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		// PG object lifecycle rules (PostgreSQL-only).
		"ddl.pg.alter.drop_column.advisory": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.alter.validate_constraint.advisory": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter.add_column.nullable.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter.set_schema.advisory": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter.owner.advisory": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter.enable_trigger.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter.disable_trigger.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.alter.attach_partition.advisory": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter.detach_partition.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.alter.set_logged.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter.set_unlogged.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter.replica_identity_full.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.alter.replica_identity_nothing.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.alter.replica_identity_using_index.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_schema.advisory": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.create_schema.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_schema.cascade.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.create_sequence.cycle.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_sequence.restart.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_sequence.cycle.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_sequence.advisory": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_sequence.cascade.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_materialized_view.advisory": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_materialized_view.cascade.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.refresh_materialized_view.concurrently.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.refresh_materialized_view.no_data.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		// PG type lifecycle rules (PostgreSQL-only).
		"ddl.pg.create_type.enum.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_type.add_value.advisory": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_type.add_value.position.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_type.advisory": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_type.cascade.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		// PG composite type lifecycle rules (PostgreSQL-only).
		"ddl.pg.create_type.composite.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_type.composite_rename.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_type.composite_set_schema.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		// PG composite type attribute lifecycle rules (PostgreSQL-only).
		"ddl.pg.alter_type.add_attribute.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_type.drop_attribute.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_type.alter_attribute_type.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_type.rename_attribute.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		// PG domain lifecycle rules (PostgreSQL-only).
		"ddl.pg.create_domain.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_domain.constraint.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_domain.default.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_domain.not_null.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_domain.rename.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_domain.advisory": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_domain.cascade.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		// PG table privilege rules (PostgreSQL-only).
		"ddl.pg.grant.table_privilege.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.grant.table_privilege.all.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.revoke.table_privilege.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.revoke.table_privilege.cascade.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		// PG extension lifecycle rules (PostgreSQL-only).
		"ddl.pg.create_extension.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.create_extension.cascade.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_extension.update.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_extension.set_schema.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_extension.advisory": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_extension.cascade.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_extension.add_member.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_extension.drop_member.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		// PG policy lifecycle rules (PostgreSQL-only).
		"ddl.pg.create_policy.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_policy.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_policy.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.alter.enable_rls.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter.disable_rls.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.alter.force_rls.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter.no_force_rls.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		// PG trigger lifecycle rules (PostgreSQL-only).
		"ddl.pg.create_trigger.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.create_constraint_trigger.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_trigger.advisory": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		// PG function/procedure lifecycle rules (PostgreSQL-only).
		"ddl.pg.create_function.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.create_function.security_definer.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.create_or_replace_function.advisory": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_function.advisory": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.create_procedure.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_procedure.advisory": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		// PG advanced view lifecycle rules (PostgreSQL-only).
		"ddl.pg.create_or_replace_view.advisory": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.create_temp_view.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.create_view.check_option.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_view.rename.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_view.set_schema.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_view.cascade.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_schema.rename.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_schema.owner.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_index.rename.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_index.set_tablespace.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_materialized_view.rename.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_materialized_view.set_schema.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},

		"ddl.pg.create_publication.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_publication.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_publication.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.create_subscription.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_subscription.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_subscription.disable.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_subscription.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},

		// PostgreSQL foreign object lifecycle rules (PG-only).
		"ddl.pg.create_foreign_table.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_foreign_table.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_foreign_table.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.create_foreign_server.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_foreign_server.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_foreign_server.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.create_user_mapping.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_user_mapping.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_user_mapping.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.create_foreign_data_wrapper.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_foreign_data_wrapper.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_foreign_data_wrapper.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		// PostgreSQL annotation lifecycle rules (PG-only).
		"ddl.pg.comment_on.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.comment_on.remove.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.security_label.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.security_label.remove.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		// PostgreSQL event trigger lifecycle rules (PG-only).
		"ddl.pg.create_event_trigger.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_event_trigger.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_event_trigger.disable.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_event_trigger.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		// PostgreSQL rewrite rule lifecycle rules (PG-only).
		"ddl.pg.create_rule.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_rule.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_rule.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		// PostgreSQL collation lifecycle rules (PG-only).
		"ddl.pg.create_collation.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_collation.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_collation.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		// PostgreSQL statistics lifecycle rules (PG-only).
		"ddl.pg.create_statistics.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_statistics.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_statistics.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.create_aggregate.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_aggregate.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_aggregate.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.create_operator.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_operator.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_operator.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.create_conversion.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_conversion.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_conversion.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.create_operator_family.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_operator_family.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_operator_family.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.create_operator_class.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_operator_class.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_operator_class.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.create_text_search_configuration.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_text_search_configuration.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_text_search_configuration.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.create_text_search_dictionary.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_text_search_dictionary.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_text_search_dictionary.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.create_text_search_parser.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_text_search_parser.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_text_search_parser.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.create_text_search_template.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_text_search_template.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_text_search_template.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_transform.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.drop_access_method.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.pg.alter_large_object.owner.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
	}
}
