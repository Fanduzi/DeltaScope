// Package policy defines audit policy configuration in domain terms.
// input: built-in rule identifiers and default severity/parameter choices
// output: baseline policy values used when no external config is supplied
// pos: domain default policy factory for v1 audit behavior
// note: if this file changes, update this header and module README.md.
package policy

import "github.com/Fanduzi/DeltaScope/internal/domain/rule"

// ddlCoreRules returns the core (MySQL/TiDB shared) DDL rule defaults
// excluding PG-specific and ALTER-specific rules.
func ddlCoreRules() map[string]RulePolicy {
	return map[string]RulePolicy{
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
		"ddl.table.name.prefix.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"prefix": "",
			},
		},
		"ddl.table.name.suffix.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"suffix": "",
			},
		},
		"ddl.table.name.contains.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"contains": []string{},
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
		"ddl.column.name.prefix.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"prefix": "",
			},
		},
		"ddl.column.name.suffix.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"suffix": "",
			},
		},
		"ddl.column.name.contains.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"contains": []string{},
			},
		},
		"ddl.table.name.pattern.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"required": true,
				"pattern":  "^[A-Za-z0-9_]+$",
			},
		},
		"ddl.column.name.pattern.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"required": true,
				"pattern":  "^[A-Za-z0-9_]+$",
			},
		},
		"ddl.index.name.pattern.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"required": true,
				"pattern":  "^[A-Za-z0-9_]+$",
			},
		},
		"ddl.table.name.keyword.forbid": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"forbid": true,
			},
		},
		"ddl.column.name.keyword.forbid": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"forbid": true,
			},
		},
		"ddl.index.name.keyword.forbid": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"forbid": true,
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
		"ddl.column.blob_text.forbid": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"forbid": false,
			},
		},
		"ddl.column.json.forbid": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"forbid": false,
			},
		},
		"ddl.column.bit.forbid": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"forbid": false,
			},
		},
		"ddl.column.timestamp.forbid": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"forbid": true,
			},
		},
		"ddl.column.char.max_length": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"limit": 64,
			},
		},
		"ddl.column.charset.allowlist": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"values": []string{"utf8", "utf8mb4"},
			},
		},
		"ddl.column.collation.allowlist": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"values": []string{"utf8_general_ci", "utf8mb4_general_ci", "utf8mb4_bin"},
			},
		},
		"ddl.column.charset_collation.match.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"required": true,
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
		"ddl.index.unique.suffix.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"suffix": "",
			},
		},
		"ddl.index.unique.contains.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"contains": []string{},
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
		"ddl.index.secondary.suffix.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"suffix": "",
			},
		},
		"ddl.index.secondary.contains.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"contains": []string{},
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
		"ddl.index.fulltext.suffix.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"suffix": "",
			},
		},
		"ddl.index.fulltext.contains.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"contains": []string{},
			},
		},
		"ddl.constraint.primary_key.name.prefix.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"prefix": "",
			},
		},
		"ddl.constraint.primary_key.name.suffix.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"suffix": "",
			},
		},
		"ddl.constraint.primary_key.name.contains.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"contains": []string{},
			},
		},
		"ddl.constraint.unique_key.name.prefix.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"prefix": "",
			},
		},
		"ddl.constraint.unique_key.name.suffix.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"suffix": "",
			},
		},
		"ddl.constraint.unique_key.name.contains.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"contains": []string{},
			},
		},
		"ddl.constraint.foreign_key.name.prefix.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"prefix": "",
			},
		},
		"ddl.constraint.foreign_key.name.suffix.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"suffix": "",
			},
		},
		"ddl.constraint.foreign_key.name.contains.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"contains": []string{},
			},
		},
		"ddl.constraint.check.name.prefix.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"prefix": "",
			},
		},
		"ddl.constraint.check.name.suffix.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"suffix": "",
			},
		},
		"ddl.constraint.check.name.contains.require": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"contains": []string{},
			},
		},
		"ddl.index.duplicate.forbid": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"forbid": true,
			},
		},
		"ddl.index.redundant_left_prefix.forbid": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"forbid": true,
			},
		},
		"ddl.index.redundant_unique_overlap.forbid": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"forbid": true,
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
		"ddl.table.row_size.max_bytes.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"required": true,
			},
		},
		"ddl.table.charset.allowlist": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"values": []string{"utf8", "utf8mb4"},
			},
		},
		"ddl.table.row_format.allowlist": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"values":           []string{"DYNAMIC"},
				"require_explicit": false,
			},
		},
		"ddl.index.key_length.max_bytes.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"required": true,
			},
		},
		"ddl.table.auto_increment.init_value.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"value": 1,
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
		"ddl.view.create.forbid": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"forbid": true,
			},
		},
		"ddl.view.drop.forbid": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"forbid": true,
			},
		},
		"ddl.table.drop.forbid": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"forbid": true,
			},
		},
		"ddl.table.drop.exists.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{},
		},
		"ddl.table.drop.adaptive_hash.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.table.drop.rows.max_count": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"limit": 100,
			},
		},
		"ddl.table.truncate.forbid": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"forbid": true,
			},
		},
		"ddl.table.truncate.exists.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{},
		},
		"ddl.table.truncate.adaptive_hash.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
		"ddl.table.truncate.rows.max_count": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params: map[string]any{
				"limit": 100,
			},
		},
		"ddl.table.denylist.forbid": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"schemas":          []string{},
				"tables":           []string{},
				"qualified_tables": []string{},
			},
		},
		"ddl.table.exists.create.forbid": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{},
		},
		"ddl.table.exists.alter.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{},
		},
		// MySQL/TiDB database lifecycle rules.
		"ddl.database.create.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.database.drop.warn": {
			Enabled: true,
			Level:   rule.LevelWarning,
			Params:  map[string]any{},
		},
	}
}

func ddlRules() map[string]RulePolicy {
	rules := make(map[string]RulePolicy, len(ddlCoreRules())+len(ddlPgRules())+len(ddlAlterRules()))
	for k, v := range ddlCoreRules() {
		rules[k] = v
	}
	for k, v := range ddlPgRules() {
		rules[k] = v
	}
	for k, v := range ddlAlterRules() {
		rules[k] = v
	}
	return rules
}
