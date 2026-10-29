# Domain Policy Module

Policy model for rule configuration and future audit settings.

## Files

| File | Responsibility |
|------|---------------|
| policy.go | Defines policy and per-rule configuration |
| defaults.go | Defines the built-in rule policy, including create-table identifier governance, expanded column/type-family breadth rules, primary-key semantics, indexes, alter restrictions, shipped semantic alter rules including the MODIFY unknown-prior-state nullability advisory, metadata-backed existence rules, object-lifecycle rules for view/drop/truncate, table options/object shape, database lifecycle rules for MySQL/TiDB, the T10 advanced-index trio (`ddl.alter.alter_index.exists.require` and `ddl.index.advanced.compatibility.require` at blocker, `ddl.index.invisible.forbid` at warning — all enabled; the compat rule's `features` default covering visibility/expression_keys/descending_keys/multi_valued), the T11 `ddl.alter.drop_foreign_key.exists.require` blocker, and the T12 CHECK lifecycle rules (`ddl.constraint.check.not_enforced.notice`/`ddl.constraint.check.ignored.notice` at notice; `ddl.alter.alter_check.exists.require`/`ddl.alter.drop_check.exists.require`/`ddl.alter.alter_check.version.require` at blocker with `required: true`), and the T13 destructive-partition rules (`ddl.alter.drop_partition.forbid`/`ddl.alter.truncate_partition.forbid` at blocker with `forbid: true`; `ddl.alter.exchange_partition.warn` at warning) — all enabled — plus the T13-R1 applicability family (`ddl.alter.{drop,truncate,exchange}_partition.applicability`, `ddl.alter.{drop,truncate,exchange}_partition.target`, `ddl.alter.exchange_partition.counterpart`) — all default blockers binding observed partition-catalog facts with per-rule evidence gaps — and the T14 topology/maintenance family (issue #93): governance `ddl.alter.{reorganize,add,coalesce}_partition.warn`/`partition_by.warn`/`remove_partitioning.warn`/`{rebuild,optimize,repair}_partition.warn`/`import_partition_tablespace.warn` at warning, `check_partition.warn` at notice (read-only status action), `discard_partition_tablespace.forbid` at blocker; applicability at blocker for the hard-refusal actions (`reorganize`/`add`/`coalesce`/`partition_by`/`remove_partitioning`/`rebuild`/`import`/`discard`) and at warning for the natively soft-failing actions (`optimize`/`repair`/`check`); member-target rules at blocker for `reorganize`/`add`/`rebuild`/`import`/`discard` and at warning for `optimize`/`repair`/`check`, and the T15 online-DDL/table-option family (issue #94): `ddl.alter.online_ddl.applicability`/`ddl.alter.charset_conversion.applicability`/`ddl.alter.table_option.applicability` at blocker, `ddl.alter.online_ddl.warn`/`ddl.alter.rebuild.warn`/`ddl.alter.charset_conversion.warn`/`ddl.alter.table_option.warn` at warning, `ddl.alter.keys_toggle.notice`/`ddl.tidb.alter.compat_options.notice` at notice — all enabled — and the Tier-1 DML rule set |
| suppression.go | Names FK-forbid suppression (`fk_forbid`) of Default Policy foreign-key naming rules that stay enabled but are not Loaded |
| policy_test.go | Verifies flexible per-rule parameter modeling |
| suppression_test.go | Verifies Default Policy keeps FK naming enabled and suppressed, not missing |

## Exports

- `RulePolicy`
- `Policy`
- `Default()`
- `ForeignKeyForbidRuleID`
- `ForeignKeyNamingSuppressionReason`
- `ForeignKeyNamingRuleIDs`
- `SuppressesForeignKeyNaming(cfg Policy, ruleID string) bool`

## Notes

- The default alter policy keeps `ddl.alter.change_column.forbid` enabled as the stricter coarse guard.
- `ddl.alter.change_column.target_type_family.allowlist` remains enabled as a follow-on semantic guard for teams that intentionally relax the coarse forbid later.
- The default metadata-aware alter policy now also enables source-aware compatibility checks for `modify column` and `change column`; those rules only fire when the statement carries a live table snapshot.
- The default alter policy also enables explicit nullability/default/auto_increment change forbids; the `change_column` variants act as follow-on guards when the coarse `ddl.alter.change_column.forbid` gate is intentionally relaxed.
- The default policy also enables shipped alter-added index prefix checks for unique, secondary, and fulltext indexes.
- The default create-table policy now enables identifier pattern and reserved-keyword checks for table, column, and secondary-index names.
- The default policy also ships structured naming governance for table, column, index, and constraint names via `prefix`, `suffix`, and `contains`; empty shipped values keep the optional checks inert until a team fills them in.
- Structured naming governance complements the existing `pattern` and `keyword` rules instead of replacing those harder lexical guards.
- The default create-table type-family policy keeps `timestamp` forbidden, keeps `char` length capped, and enforces charset/collation allowlists plus pair-coherence checks.
- Blob/text, json, and bit forbids are shipped in the default template but remain relaxed by default via `forbid: false` until a team intentionally tightens them.
- The default create-table index policy now also enables left-prefix and unique-overlap redundant-index findings on top of exact duplicate detection.
- The default create-table option policy now requires `ROW_FORMAT=DYNAMIC` when row format is specified and requires explicit `AUTO_INCREMENT` seeds to stay at `1`.
- The default metadata-aware create-table sizing policy now also enables rough `row_size` and `index key length` guards; they only run when instance facts are attached and intentionally stay conservative.
- The default metadata-backed DDL policy enables existence checks for create/alter table plus add/drop/rename column/index, drop-primary-key, standalone `create_index` column operations, the T08 setter targets (`ddl.alter.set_default.exists.require`, `ddl.alter.drop_default.exists.require`), and `AFTER`-relative column positions (`ddl.alter.column_position.exists.require`).
- The default policy also enables `ddl.alter.change_column.target.exists.forbid`, `ddl.alter.rename_column.target.exists.forbid`, and `ddl.alter.rename_column.version.require` (`required: true`). Those three rules do not change the level or parameters of the existing source-existence rules.
- For MySQL/TiDB the existence rules (`ddl.table.exists.create.forbid`, `ddl.table.exists.alter.require`, every column/index member-existence rule, `ddl.alter.drop_primary_key.exists.require`, `ddl.create_index.columns.exists.require`) evaluate the request-local ordered schema state offline — deriving facts from earlier statements in the same batch — and emit `unknown_table_state` evidence gaps instead of silently passing when that state cannot prove the premise; the gap projection never applies to PostgreSQL. The remaining existence rules still no-op without a usable snapshot.
- The default lifecycle policy now also blocks `create view`, `drop table`, and `truncate table`, and ships adaptive-hash cautions plus metadata-backed existence checks for drop/truncate operations.
- The default lifecycle policy also ships row-count cautions for `drop table` and `truncate table`, using metadata-backed `table_rows` snapshots with a default threshold of `100`.
- The default global DDL policy enables MySQL merge-alter guidance and ships the TiDB variant in a relaxed state until a team chooses to require merged alters there as well.
- The default policy now also exposes DDL and DML table denylist hooks via `schemas`, `tables`, and `qualified_tables`; the shipped defaults keep those lists empty so the rules stay inert until a team populates them.
- The default policy also enables `dml.table.exists.require`; it blocks only definitively absent MySQL/TiDB DML target tables when live table metadata is attached and stays inert offline and for PostgreSQL.
- The default policy also enables `ddl.alter.column.dependency.require` (`required: true`), which blocks alter operations that would break stored generated-column or expression-default dependents proven by the ordered schema state.
- The default policy also enables `ddl.column.advanced.compatibility.require` (`required: true`, `features: [generated, expression_default, visibility]`), which gates generated-column, expression-default, and column-visibility declarations against the observed product/version compatibility table.
- Default Policy does not include `dml.impact.estimate`, `dml.impact.rows.max_count`, or `dml.impact.ratio.max_percent`. Those IDs stay in the Rule Catalog as default-disabled opt-in rules.
- Default Policy still enables `ddl.constraint.foreign_key.name.prefix.require`, `ddl.constraint.foreign_key.name.suffix.require`, and `ddl.constraint.foreign_key.name.contains.require`. While `ddl.table.foreign_key.forbid` is enabled they are not Loaded (`fk_forbid`). They are suppressed, not missing.

## Dependencies
- Upstream: application policy loading and future config adapters
- Downstream: `internal/domain/rule`

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
