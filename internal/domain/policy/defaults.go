// Package policy defines audit policy configuration in domain terms.
// input: built-in rule identifiers and default severity/parameter choices
// output: baseline policy values used when no external config is supplied
// pos: domain default policy factory for v1 audit behavior
// note: if this file changes, update this header and module README.md.
package policy

import "github.com/Fanduzi/DeltaScope/internal/domain/rule"

// Default returns the built-in v1 policy baseline.
func Default() Policy {
	return Policy{
		Rules: map[string]RulePolicy{
			"ddl.table.comment.require": {
				Enabled: true,
				Level:   rule.LevelWarning,
				Params: map[string]any{
					"required": true,
				},
			},
			"ddl.table.name.max_length": {
				Enabled: true,
				Level:   rule.LevelBlocker,
				Params: map[string]any{
					"limit": 64,
				},
			},
			"ddl.table.primary_key.require": {
				Enabled: true,
				Level:   rule.LevelBlocker,
				Params: map[string]any{
					"required": true,
				},
			},
			"ddl.table.primary_key.columns.max_count": {
				Enabled: true,
				Level:   rule.LevelWarning,
				Params: map[string]any{
					"limit": 1,
				},
			},
			"ddl.table.primary_key.bigint.require": {
				Enabled: true,
				Level:   rule.LevelBlocker,
				Params: map[string]any{
					"required": true,
				},
			},
			"ddl.table.primary_key.unsigned.require": {
				Enabled: true,
				Level:   rule.LevelBlocker,
				Params: map[string]any{
					"required": true,
				},
			},
			"ddl.table.primary_key.auto_increment.require": {
				Enabled: true,
				Level:   rule.LevelBlocker,
				Params: map[string]any{
					"required": true,
				},
			},
			"ddl.table.primary_key.not_null.require": {
				Enabled: true,
				Level:   rule.LevelBlocker,
				Params: map[string]any{
					"required": true,
				},
			},
			"ddl.table.columns.min_count": {
				Enabled: true,
				Level:   rule.LevelBlocker,
				Params: map[string]any{
					"limit": 1,
				},
			},
			"ddl.table.audit_columns.require": {
				Enabled: true,
				Level:   rule.LevelWarning,
				Params: map[string]any{
					"required": true,
				},
			},
			"ddl.column.comment.require": {
				Enabled: true,
				Level:   rule.LevelWarning,
				Params: map[string]any{
					"required": true,
				},
			},
			"ddl.column.name.max_length": {
				Enabled: true,
				Level:   rule.LevelBlocker,
				Params: map[string]any{
					"limit": 64,
				},
			},
			"ddl.column.varchar.max_length": {
				Enabled: true,
				Level:   rule.LevelBlocker,
				Params: map[string]any{
					"limit": 16383,
				},
			},
			"ddl.column.default.require": {
				Enabled: true,
				Level:   rule.LevelWarning,
				Params: map[string]any{
					"required": true,
				},
			},
			"ddl.column.not_null.require": {
				Enabled: true,
				Level:   rule.LevelWarning,
				Params: map[string]any{
					"required":        true,
					"allow_time_null": true,
				},
			},
			"ddl.column.float_double.forbid": {
				Enabled: true,
				Level:   rule.LevelWarning,
				Params: map[string]any{
					"forbid": true,
				},
			},
			"ddl.index.total.max_count": {
				Enabled: true,
				Level:   rule.LevelWarning,
				Params: map[string]any{
					"limit": 12,
				},
			},
			"ddl.index.columns.max_count": {
				Enabled: true,
				Level:   rule.LevelWarning,
				Params: map[string]any{
					"limit": 8,
				},
			},
			"ddl.index.unique.prefix.require": {
				Enabled: true,
				Level:   rule.LevelWarning,
				Params: map[string]any{
					"required": true,
					"prefix":   "uniq_",
				},
			},
			"ddl.index.secondary.prefix.require": {
				Enabled: true,
				Level:   rule.LevelWarning,
				Params: map[string]any{
					"required": true,
					"prefix":   "idx_",
				},
			},
			"ddl.index.fulltext.prefix.require": {
				Enabled: true,
				Level:   rule.LevelWarning,
				Params: map[string]any{
					"required": true,
					"prefix":   "full_",
				},
			},
			"ddl.index.duplicate.forbid": {
				Enabled: true,
				Level:   rule.LevelWarning,
				Params: map[string]any{
					"forbid": true,
				},
			},
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
			"ddl.alter.add_index.unique.prefix.require": {
				Enabled: true,
				Level:   rule.LevelWarning,
				Params: map[string]any{
					"required": true,
					"prefix":   "uniq_",
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
			"ddl.alter.add_index.fulltext.prefix.require": {
				Enabled: true,
				Level:   rule.LevelWarning,
				Params: map[string]any{
					"required": true,
					"prefix":   "full_",
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
			"ddl.table.comment.max_length": {
				Enabled: true,
				Level:   rule.LevelWarning,
				Params: map[string]any{
					"limit": 128,
				},
			},
			"ddl.table.engine.allowlist": {
				Enabled: true,
				Level:   rule.LevelBlocker,
				Params: map[string]any{
					"values": []string{"InnoDB"},
				},
			},
			"ddl.table.charset.allowlist": {
				Enabled: true,
				Level:   rule.LevelBlocker,
				Params: map[string]any{
					"values": []string{"utf8", "utf8mb4"},
				},
			},
			"ddl.table.foreign_key.forbid": {
				Enabled: true,
				Level:   rule.LevelBlocker,
				Params: map[string]any{
					"forbid": true,
				},
			},
			"ddl.table.partition.forbid": {
				Enabled: true,
				Level:   rule.LevelBlocker,
				Params: map[string]any{
					"forbid": true,
				},
			},
			"ddl.table.create_like.forbid": {
				Enabled: true,
				Level:   rule.LevelBlocker,
				Params: map[string]any{
					"forbid": true,
				},
			},
			"ddl.table.create_as.forbid": {
				Enabled: true,
				Level:   rule.LevelBlocker,
				Params: map[string]any{
					"forbid": true,
				},
			},
			"dml.where.require": {
				Enabled: true,
				Level:   rule.LevelBlocker,
				Params: map[string]any{
					"required": true,
				},
			},
			"dml.limit.forbid": {
				Enabled: true,
				Level:   rule.LevelWarning,
				Params: map[string]any{
					"forbid": true,
				},
			},
			"dml.order_by.forbid": {
				Enabled: true,
				Level:   rule.LevelWarning,
				Params: map[string]any{
					"forbid": true,
				},
			},
			"dml.subquery.forbid": {
				Enabled: true,
				Level:   rule.LevelBlocker,
				Params: map[string]any{
					"forbid": true,
				},
			},
			"dml.join.on.require": {
				Enabled: true,
				Level:   rule.LevelBlocker,
				Params: map[string]any{
					"required": true,
				},
			},
			"dml.insert.rows.max_count": {
				Enabled: true,
				Level:   rule.LevelWarning,
				Params: map[string]any{
					"limit": 100,
				},
			},
			"dml.replace.forbid": {
				Enabled: true,
				Level:   rule.LevelBlocker,
				Params: map[string]any{
					"forbid": true,
				},
			},
			"dml.insert.select.forbid": {
				Enabled: true,
				Level:   rule.LevelBlocker,
				Params: map[string]any{
					"forbid": true,
				},
			},
			"dml.insert.on_duplicate.forbid": {
				Enabled: true,
				Level:   rule.LevelBlocker,
				Params: map[string]any{
					"forbid": true,
				},
			},
		},
	}
}
