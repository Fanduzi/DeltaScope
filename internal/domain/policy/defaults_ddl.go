// Package policy defines audit policy configuration in domain terms.
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
				// TiDB AUTO_RANDOM is the native auto-allocating
				// surrogate key; the shipped default satisfies the
				// allocation intent while a strict literal policy can
				// set this param false.
				"accept_tidb_auto_random": true,
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
		"ddl.constraint.check.not_enforced.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
		},
		"ddl.constraint.check.ignored.notice": {
			Enabled: true,
			Level:   rule.LevelNotice,
			Params:  map[string]any{},
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
		"ddl.table.collation.allowlist": {
			Enabled: false,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"values":           []string{"utf8mb4_bin"},
				"require_explicit": true,
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
		"ddl.view.target.valid":          {Enabled: true, Level: rule.LevelBlocker},
		"ddl.view.source.valid":          {Enabled: true, Level: rule.LevelBlocker},
		"ddl.view.security.definer.warn": {Enabled: true, Level: rule.LevelWarning},
		"ddl.view.algorithm.warn":        {Enabled: true, Level: rule.LevelWarning},
		"ddl.view.check_option.valid":    {Enabled: true, Level: rule.LevelBlocker},
		"ddl.view.options.ignored.warn":  {Enabled: true, Level: rule.LevelWarning},
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
		"ddl.table.split_region.exists.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{},
		},
		"ddl.table.compact.exists.require": {
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
		"ddl.create_index.columns.exists.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"required": true,
			},
		},
		// T16 lifecycle rules: multi-target DROP members, RENAME endpoints,
		// LIKE/CTAS sources, truncate inbound-FK, and TiDB local-temporary
		// refusals. All consume ordered-state bound metadata.
		"ddl.table.like.source.exists.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{},
		},
		"ddl.table.create_as.source.exists.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{},
		},
		"ddl.table.rename.source.exists.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{},
		},
		"ddl.table.rename.dest.exists.forbid": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{},
		},
		"ddl.table.truncate.inbound_fk": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{},
		},
		"ddl.tidb.local_temporary.alter": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{},
		},
		"ddl.tidb.local_temporary.rename": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{},
		},
		"ddl.tidb.local_temporary.like": {
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
		// T17 schema-lifecycle rules: ordered schema-existence premises,
		// business-schema denylist, and uniform system-schema protection.
		"ddl.database.create.exists.forbid": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{},
		},
		"ddl.database.alter.exists.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{},
		},
		"ddl.database.drop.exists.require": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{},
		},
		"ddl.database.denylist.forbid": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params: map[string]any{
				"schemas": []string{},
			},
		},
		"ddl.database.system.modify.forbid": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{},
		},
		"ddl.database.option.encryption.applicability": {
			Enabled: true,
			Level:   rule.LevelBlocker,
			Params:  map[string]any{},
		},
		// MySQL/TiDB lifecycle notice rules.
		"ddl.rename_table.notice":                     {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.create_index.notice":                     {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.drop_index.notice":                       {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.alter_database.notice":                   {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.create_procedure.notice":                 {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.alter_procedure.notice":                  {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.drop_procedure.notice":                   {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.create_function.notice":                  {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.alter_function.notice":                   {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.drop_function.notice":                    {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.create_trigger.notice":                   {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.create_trigger.ordering.version.require": {Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{}},
		"ddl.drop_trigger.notice":                     {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.create_event.notice":                     {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.alter_event.notice":                      {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.drop_event.notice":                       {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.event.enabled_execution.warn":            {Enabled: true, Level: rule.LevelWarning, Params: map[string]any{}},
		"ddl.pg.column.generation.declaration.valid":  {Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{}},
		"ddl.pg.column.identity.cycle.warn":           {Enabled: true, Level: rule.LevelWarning, Params: map[string]any{}},
		"ddl.routine.target.valid":                    {Enabled: true, Level: rule.LevelBlocker},
		"ddl.routine.security.definer.warn":           {Enabled: true, Level: rule.LevelWarning, Params: map[string]any{}},
		"ddl.routine.variable_shadowing.warn":         {Enabled: true, Level: rule.LevelWarning, Params: map[string]any{}},
		"ddl.create_user.notice":                      {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.alter_user.notice":                       {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.drop_user.notice":                        {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.create_role.notice":                      {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.drop_role.notice":                        {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.grant.notice":                            {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.revoke.notice":                           {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.create_resource_group.notice":            {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.alter_resource_group.notice":             {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.drop_resource_group.notice":              {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.tidb.resource_group.valid":               {Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{}},
		"ddl.tidb.resource_group.burstable.warn":      {Enabled: true, Level: rule.LevelWarning, Params: map[string]any{}},
		"ddl.tidb.resource_group.runaway.warn":        {Enabled: true, Level: rule.LevelWarning, Params: map[string]any{}},
		"ddl.mysql.resource_group.valid":              {Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{}},
		"ddl.mysql.resource_group.force.warn":         {Enabled: true, Level: rule.LevelWarning, Params: map[string]any{}},
		"ddl.mysql.resource_group.affinity.warn":      {Enabled: true, Level: rule.LevelWarning, Params: map[string]any{}},
		"ddl.mysql.resource_group.priority.warn":      {Enabled: true, Level: rule.LevelWarning, Params: map[string]any{}},
		// T25: the MySQL tablespace family — the drop forbids the
		// storage-object destruction outright; create/alter warn on the
		// mutation; the applicability gate binds engine, version,
		// existence, and occupancy premises.
		"ddl.mysql.tablespace.create.warn":           {Enabled: true, Level: rule.LevelWarning, Params: map[string]any{}},
		"ddl.mysql.tablespace.alter.warn":            {Enabled: true, Level: rule.LevelWarning, Params: map[string]any{}},
		"ddl.mysql.tablespace.drop.forbid":           {Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{"forbid": true}},
		"ddl.mysql.tablespace.applicability":         {Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{}},
		"ddl.mysql.server.drop.notice":               {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.mysql.server.create.notice":             {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.mysql.server.alter.notice":              {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.mysql.srs.drop.srid0":                   {Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{}},
		"ddl.mysql.srs.drop.dependency":              {Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{}},
		"ddl.mysql.srs.drop.reserved":                {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.mysql.srs.drop.privilege":               {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.mysql.srs.drop.absent":                  {Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{}},
		"ddl.mysql.srs.drop.absent_noop":             {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.mysql.srs.create.srid0":                 {Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{}},
		"ddl.mysql.srs.create.duplicate_attribute":   {Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{}},
		"ddl.mysql.srs.create.missing_attribute":     {Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{}},
		"ddl.mysql.srs.create.existence":             {Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{}},
		"ddl.mysql.srs.create.exists_noop":           {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.mysql.srs.create.name_conflict":         {Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{}},
		"ddl.mysql.srs.create.organization_conflict": {Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{}},
		"ddl.mysql.srs.create.privilege":             {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.mysql.alter_instance.reload_tls":        {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.mysql.alter_instance.rotate_master_key": {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.mysql.alter_instance.privilege":         {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.tidb.table.auto_random.valid":           {Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{}},
		"ddl.tidb.table.options.valid":               {Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{}},
		"ddl.tidb.alter.auto_random.valid":           {Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{}},
		"ddl.tidb.alter.table_options.valid":         {Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{}},
		"ddl.tidb.table.cache.valid":                 {Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{}},
		"ddl.tidb.table.attributes.valid":            {Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{}},
		"ddl.tidb.table.tiflash.valid":               {Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{}},
		"ddl.tidb.table.split_region.valid":          {Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{}},
		"ddl.tidb.table.compact.valid":               {Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{}},
		"ddl.create_placement_policy.notice":         {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.alter_placement_policy.notice":          {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.drop_placement_policy.notice":           {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.create_sequence.notice":                 {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.alter_sequence.notice":                  {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.drop_sequence.notice":                   {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.tidb.sequence.valid":                    {Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{}},
		"ddl.tidb.sequence.restart.warn":             {Enabled: true, Level: rule.LevelWarning, Params: map[string]any{}},
		"ddl.tidb.sequence.cycle.warn":               {Enabled: true, Level: rule.LevelWarning, Params: map[string]any{}},
		"ddl.tidb.sequence.references.warn":          {Enabled: true, Level: rule.LevelWarning, Params: map[string]any{}},
		// MySQL/TiDB alter action notice rules.
		"ddl.alter.add_column.notice":                  {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.alter.add_constraint.notice":              {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.alter.drop_column.notice":                 {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.alter.modify_column.notice":               {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.alter.drop_index.notice":                  {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.alter.drop_foreign_key.notice":            {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.tidb.alter_table.placement_policy.notice": {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
		"ddl.tidb.placement.warn":                      {Enabled: true, Level: rule.LevelWarning, Params: map[string]any{}},
		"ddl.tidb.placement.exists.require":            {Enabled: true, Level: rule.LevelBlocker, Params: map[string]any{}},
		"ddl.tidb.alter.compat_options.notice":         {Enabled: true, Level: rule.LevelNotice, Params: map[string]any{}},
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
