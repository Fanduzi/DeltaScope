# Domain DDL Rule Module

Expanded DDL rule catalog for create-table governance, table options/object shape, and action-level alter restrictions.

## Files

| File | Responsibility |
|------|---------------|
| common.go | Shared DDL rule IDs plus parser-neutral alter matching, explicit-change, rename, option, target-type-family, and alter-index projection helpers, including pinned Milestone 4 create-table superset IDs |
| common_test.go | Verifies richer alter helper boundaries and future alter rule IDs remain stable |
| config.go | Parses policy params for DDL rule constructors, including normalized string-list and bounded integer helpers for upcoming alter semantics |
| table_rules.go | Implements table comment and table name rules |
| primary_key_rules.go | Implements primary-key presence and column-count rules |
| primary_key_semantic_rules.go | Implements bigint/unsigned/auto-increment/not-null primary-key semantic rules |
| column_rules.go | Implements table-column count and column-level governance rules |
| audit_column_rules.go | Implements audit timestamp column rules |
| identifier_rules.go | Implements create-table identifier-pattern and reserved-keyword rules |
| index_rules.go | Implements create-table index count, prefix, and duplicate-index rules |
| alter_rules.go | Implements action-level ALTER TABLE restriction rules |
| alter_semantic_rules.go | Implements rename-index forbids, explicit alter-column change forbids, alter-added index lifecycle rules, and conservative alter target-type-family rules |
| table_option_rules.go | Implements create-table option, foreign-key, and object-shape rules |
| register.go | Registers enabled DDL rules into the shared registry, including shipped alter-added index lifecycle rules |
| table_rules_test.go | Verifies table comment and name-length rule behavior |
| primary_key_rules_test.go | Verifies primary-key requirement and shape rules |
| primary_key_semantic_rules_test.go | Verifies primary-key semantic rules for bigint/unsigned/auto-increment/not-null requirements |
| column_rules_test.go | Verifies column-count, comment, naming, default, nullability, and type rules |
| audit_column_rules_test.go | Verifies audit timestamp column rules |
| identifier_rules_test.go | Verifies create-table identifier-pattern and reserved-keyword rules |
| index_rules_test.go | Verifies create-table index governance rules |
| alter_rules_test.go | Verifies action-level ALTER TABLE restriction rules |
| alter_semantic_rules_test.go | Verifies semantic alter rename-index, explicit alter-column change, alter-added index lifecycle, and conservative target-type-family rules plus registration order |
| table_option_rules_test.go | Verifies create-table option and object-shape rules |
| register_test.go | Verifies policy-backed DDL rule registration and deterministic ordering |

## Exports

- `Register(registry *rule.Registry, cfg policy.Policy) error`

## Dependencies
- Upstream: future application rule assembly and higher-level audit services
- Downstream: `internal/domain/policy`, `internal/domain/rule`, `internal/domain/spec`

## Rule IDs

- `ddl.table.comment.require`
- `ddl.table.name.max_length`
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
- `ddl.table.name.pattern.require`
- `ddl.column.name.pattern.require`
- `ddl.index.name.pattern.require`
- `ddl.table.name.keyword.forbid`
- `ddl.column.name.keyword.forbid`
- `ddl.index.name.keyword.forbid`
- `ddl.column.varchar.max_length`
- `ddl.column.default.require`
- `ddl.column.not_null.require`
- `ddl.column.float_double.forbid`
- `ddl.index.total.max_count`
- `ddl.index.columns.max_count`
- `ddl.index.unique.prefix.require`
- `ddl.index.secondary.prefix.require`
- `ddl.index.fulltext.prefix.require`
- `ddl.index.duplicate.forbid`
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
- `ddl.alter.modify_column.explicit_nullability_change.forbid`
- `ddl.alter.change_column.explicit_nullability_change.forbid`
- `ddl.alter.modify_column.explicit_default_change.forbid`
- `ddl.alter.change_column.explicit_default_change.forbid`
- `ddl.alter.modify_column.explicit_auto_increment_change.forbid`
- `ddl.alter.change_column.explicit_auto_increment_change.forbid`
- `ddl.alter.add_index.columns.max_count`
- `ddl.alter.add_index.duplicate.forbid`
- `ddl.alter.add_index.unique.prefix.require`
- `ddl.alter.add_index.secondary.prefix.require`
- `ddl.alter.add_index.fulltext.prefix.require`
- `ddl.table.comment.max_length`
- `ddl.table.engine.allowlist`
- `ddl.table.charset.allowlist`
- `ddl.table.foreign_key.forbid`
- `ddl.table.partition.forbid`
- `ddl.table.create_like.forbid`
- `ddl.table.create_as.forbid`

## Milestone 4 Planned Create-Table Surface

Milestone 4 is the remaining create-table breadth push. The following rule IDs are pinned now so later tasks can implement them without churn. These IDs are planned surface, not shipped behavior yet.

### Wider Type-Family and Charset/Collation Governance

- `ddl.column.blob_text.forbid`
- `ddl.column.json.forbid`
- `ddl.column.bit.forbid`
- `ddl.column.timestamp.forbid`
- `ddl.column.char.max_length`
- `ddl.column.charset.allowlist`
- `ddl.column.collation.allowlist`
- `ddl.column.charset_collation.match.require`

### Deeper Redundant-Index Governance

- `ddl.index.redundant_left_prefix.forbid`
- `ddl.index.redundant_unique_overlap.forbid`

### Remaining Create-Table Object Shape

- `ddl.table.row_format.allowlist`
- `ddl.table.auto_increment.init_value.require`

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
- `ddl.alter.add_index.secondary.prefix.require`
- `ddl.alter.add_index.fulltext.prefix.require`
- `ddl.alter.add_index.columns.max_count`
- `ddl.alter.add_index.duplicate.forbid`
- `ddl.alter.modify_column.target_type_family.allowlist`
- `ddl.alter.change_column.target_type_family.allowlist`

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

Those alter-added index lifecycle rules are part of the normal `Register(...)` path when their policies are enabled.

The next source-aware alter batch is intentionally prepared with honest names:

- `explicit_*_change.forbid` rule IDs only claim statement-local explicit change detection
- `target_type_family.allowlist` IDs still describe target-side allowlists, not source-to-target compatibility
- alter-add-index helper seams project parser-neutral index payloads without inventing live schema state

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
