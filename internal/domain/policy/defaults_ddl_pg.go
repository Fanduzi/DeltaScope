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
	}
}
