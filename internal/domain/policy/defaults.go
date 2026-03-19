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
