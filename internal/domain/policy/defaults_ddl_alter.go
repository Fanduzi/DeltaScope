// Package policy defines audit policy configuration in domain terms.
// input: built-in rule identifiers and default severity/parameter choices
// output: baseline policy values used when no external config is supplied
// pos: domain default policy factory for v1 audit behavior
// note: if this file changes, update this header and module README.md.
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
		"ddl.alter.rename_column.exists.require": {
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
		"ddl.alter.rename_index.exists.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{},
		},
		"ddl.alter.drop_primary_key.exists.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{},
		},
	}
}
