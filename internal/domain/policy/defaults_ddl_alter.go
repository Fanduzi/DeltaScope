// Package policy defines audit policy configuration in domain terms.
package policy

import "github.com/Fanduzi/DeltaScope/internal/domain/rule"

func ddlAlterRules() map[string]RulePolicy {
	return map[string]RulePolicy{
		"ddl.alter.drop_column.forbid": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"forbid": false,
			},
		},
		"ddl.alter.drop_primary_key.forbid": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"forbid": true,
			},
		},
		"ddl.alter.drop_index.forbid": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"forbid": false,
			},
		},
		"ddl.alter.drop_partition.forbid": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"forbid": true,
			},
		},
		"ddl.alter.truncate_partition.forbid": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"forbid": true,
			},
		},
		"ddl.alter.exchange_partition.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
		},
		// T13-R1 partition applicability family: proven-inapplicable cells
		// block; unobserved premises downgrade to bounded evidence gaps.
		"ddl.alter.drop_partition.applicability": {
			Enabled: true,
			Level:   rule.LevelBlocker,
		},
		"ddl.alter.truncate_partition.applicability": {
			Enabled: true,
			Level:   rule.LevelBlocker,
		},
		"ddl.alter.exchange_partition.applicability": {
			Enabled: true,
			Level:   rule.LevelBlocker,
		},
		"ddl.alter.drop_partition.target": {
			Enabled: true,
			Level:   rule.LevelBlocker,
		},
		"ddl.alter.truncate_partition.target": {
			Enabled: true,
			Level:   rule.LevelBlocker,
		},
		"ddl.alter.exchange_partition.target": {
			Enabled: true,
			Level:   rule.LevelBlocker,
		},
		"ddl.alter.exchange_partition.counterpart": {
			Enabled: true,
			Level:   rule.LevelBlocker,
		},
		// T14 partition topology + maintenance (frozen per CONTRACT §6):
		// policy findings warn on redistribution/rewrite risk; DISCARD
		// forbids directly as data destruction; CHECK is read-only notice.
		"ddl.alter.reorganize_partition.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
		},
		"ddl.alter.add_partition.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
		},
		"ddl.alter.coalesce_partition.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
		},
		"ddl.alter.partition_by.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
		},
		"ddl.alter.remove_partitioning.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
		},
		"ddl.alter.rebuild_partition.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
		},
		"ddl.alter.optimize_partition.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
		},
		"ddl.alter.repair_partition.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
		},
		"ddl.alter.check_partition.warn": {
			Enabled: true,
			Level:   rule.LevelNotice,
		},
		"ddl.alter.import_partition_tablespace.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
		},
		"ddl.alter.discard_partition_tablespace.forbid": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"forbid": true,
			},
		},
		// T14 applicability + member-target semantic checks: proven refusal
		// cells block; soft-fail cells (check/optimize/repair targets and
		// non-partitioned soft ops) report at warning level.
		"ddl.alter.reorganize_partition.applicability": {
			Enabled: true,
			Level:   rule.LevelBlocker,
		},
		"ddl.alter.add_partition.applicability": {
			Enabled: true,
			Level:   rule.LevelBlocker,
		},
		"ddl.alter.coalesce_partition.applicability": {
			Enabled: true,
			Level:   rule.LevelBlocker,
		},
		"ddl.alter.partition_by.applicability": {
			Enabled: true,
			Level:   rule.LevelBlocker,
		},
		"ddl.alter.remove_partitioning.applicability": {
			Enabled: true,
			Level:   rule.LevelBlocker,
		},
		"ddl.alter.rebuild_partition.applicability": {
			Enabled: true,
			Level:   rule.LevelBlocker,
		},
		"ddl.alter.optimize_partition.applicability": {
			Enabled: true,
			Level:   rule.LevelWarning,
		},
		"ddl.alter.repair_partition.applicability": {
			Enabled: true,
			Level:   rule.LevelWarning,
		},
		"ddl.alter.check_partition.applicability": {
			Enabled: true,
			Level:   rule.LevelWarning,
		},
		"ddl.alter.import_partition_tablespace.applicability": {
			Enabled: true,
			Level:   rule.LevelBlocker,
		},
		"ddl.alter.discard_partition_tablespace.applicability": {
			Enabled: true,
			Level:   rule.LevelBlocker,
		},
		"ddl.alter.reorganize_partition.target": {
			Enabled: true,
			Level:   rule.LevelBlocker,
		},
		"ddl.alter.add_partition.target": {
			Enabled: true,
			Level:   rule.LevelBlocker,
		},
		"ddl.alter.rebuild_partition.target": {
			Enabled: true,
			Level:   rule.LevelBlocker,
		},
		"ddl.alter.optimize_partition.target": {
			Enabled: true,
			Level:   rule.LevelWarning,
		},
		"ddl.alter.repair_partition.target": {
			Enabled: true,
			Level:   rule.LevelWarning,
		},
		"ddl.alter.check_partition.target": {
			Enabled: true,
			Level:   rule.LevelWarning,
		},
		"ddl.alter.import_partition_tablespace.target": {
			Enabled: true,
			Level:   rule.LevelBlocker,
		},
		"ddl.alter.discard_partition_tablespace.target": {
			Enabled: true,
			Level:   rule.LevelBlocker,
		},
		"ddl.alter.rename_table.forbid": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"forbid": true,
			},
		},
		"ddl.alter.rename_column.forbid": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"forbid": true,
			},
		},
		"ddl.alter.rename_index.forbid": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"forbid": true,
			},
		},
		"ddl.alter.add_index.columns.max_count": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"limit": 8,
			},
		},
		"ddl.alter.add_index.duplicate.forbid": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"forbid": true,
			},
		},
		"ddl.alter.add_index.redundant_left_prefix.forbid": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"forbid": true,
			},
		},
		"ddl.alter.add_index.redundant_unique_overlap.forbid": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"forbid": true,
			},
		},
		"ddl.alter.add_index.unique.prefix.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"required": true,
				"prefix":   "uniq_",
			},
		},
		"ddl.alter.add_index.unique.suffix.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"suffix": "",
			},
		},
		"ddl.alter.add_index.unique.contains.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"contains": []string{},
			},
		},
		"ddl.alter.add_index.secondary.prefix.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"required": true,
				"prefix":   "idx_",
			},
		},
		"ddl.alter.add_index.secondary.suffix.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"suffix": "",
			},
		},
		"ddl.alter.add_index.secondary.contains.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"contains": []string{},
			},
		},
		"ddl.alter.add_index.fulltext.prefix.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"required": true,
				"prefix":   "full_",
			},
		},
		"ddl.alter.add_index.fulltext.suffix.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"suffix": "",
			},
		},
		"ddl.alter.add_index.fulltext.contains.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"contains": []string{},
			},
		},
		"ddl.alter.change_column.forbid": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"forbid": true,
			},
		},
		"ddl.alter.modify_column.forbid": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"forbid": false,
			},
		},
		// PG-native alter action forbid rules (PostgreSQL-only).
		"ddl.alter.set_data_type.forbid": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"forbid": true,
			},
		},
		"ddl.alter.set_default.forbid": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"forbid": true,
			},
		},
		"ddl.alter.drop_default.forbid": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"forbid": true,
			},
		},
		"ddl.alter.set_not_null.forbid": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"forbid": true,
			},
		},
		"ddl.alter.drop_not_null.forbid": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"forbid": true,
			},
		},
		"ddl.alter.drop_expression.forbid": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"forbid": true,
			},
		},
		"ddl.alter.set_generated.forbid": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"forbid": true,
			},
		},
		"ddl.alter.drop_identity.forbid": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"forbid": true,
			},
		},
		"ddl.alter.modify_column.target_type_family.allowlist": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"required":              true,
				"allowed_type_families": []string{"integer", "decimal", "string", "binary", "time"},
			},
		},
		"ddl.alter.change_column.target_type_family.allowlist": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"required":              true,
				"allowed_type_families": []string{"integer", "decimal", "string", "binary", "time"},
			},
		},
		"ddl.alter.modify_column.compatibility.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"required": true,
			},
		},
		"ddl.alter.change_column.compatibility.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"required": true,
			},
		},
		"ddl.alter.table_option.compatibility.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"required": true,
			},
		},
		"ddl.alter.modify_column.explicit_nullability_change.forbid": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"forbid": true,
			},
		},
		"ddl.alter.modify_column.explicit_nullability_change.unknown_prior_state.advisory": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.alter.change_column.explicit_nullability_change.forbid": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"forbid": true,
			},
		},
		"ddl.alter.modify_column.explicit_default_change.forbid": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"forbid": true,
			},
		},
		"ddl.alter.change_column.explicit_default_change.forbid": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"forbid": true,
			},
		},
		"ddl.alter.modify_column.explicit_auto_increment_change.forbid": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"forbid": true,
			},
		},
		"ddl.alter.change_column.explicit_auto_increment_change.forbid": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"forbid": true,
			},
		},

		"ddl.alter.set_default.explicit_default_change.forbid": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"forbid": true,
			},
		},
		"ddl.alter.drop_default.explicit_default_change.forbid": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"forbid": true,
			},
		},
		"ddl.alter.set_not_null.explicit_nullability_change.forbid": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"forbid": true,
			},
		},
		"ddl.alter.drop_not_null.explicit_nullability_change.forbid": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"forbid": true,
			},
		},
		"ddl.alter.merge.mysql.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"required": true,
			},
		},
		"ddl.alter.merge.tidb.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"required": false,
			},
		},
		"ddl.alter.add_column.exists.forbid": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{},
		},
		"ddl.alter.drop_column.exists.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{},
		},
		"ddl.alter.modify_column.exists.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{},
		},
		"ddl.alter.change_column.exists.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{},
		},
		"ddl.alter.change_column.target.exists.forbid": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{},
		},
		"ddl.alter.rename_column.exists.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{},
		},
		"ddl.alter.rename_column.target.exists.forbid": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{},
		},
		"ddl.alter.rename_column.version.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"required": true,
			},
		},
		"ddl.alter.column.dependency.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"required": true,
			},
		},
		"ddl.column.advanced.compatibility.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"required": true,
				"features": []string{
					"generated",
					"expression_default",
					"visibility",
				},
			},
		},
		"ddl.alter.set_default.exists.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{},
		},
		"ddl.alter.drop_default.exists.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{},
		},
		"ddl.alter.column_position.exists.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{},
		},
		"ddl.alter.add_index.exists.forbid": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{},
		},
		"ddl.alter.drop_index.exists.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{},
		},
		"ddl.alter.drop_foreign_key.exists.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{},
		},
		"ddl.alter.alter_check.exists.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{},
		},
		"ddl.alter.drop_check.exists.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{},
		},
		"ddl.alter.alter_check.version.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"required": true,
			},
		},
		"ddl.alter.rename_index.exists.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{},
		},
		"ddl.alter.alter_index.exists.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{},
		},
		"ddl.index.invisible.forbid": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.index.advanced.compatibility.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"required": true,
				"features": []string{
					"visibility",
					"expression_keys",
					"descending_keys",
					"multi_valued",
				},
			},
		},
		"ddl.alter.drop_primary_key.exists.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{},
		},
	}
}
