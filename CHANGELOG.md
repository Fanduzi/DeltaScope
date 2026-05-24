# Changelog

All notable changes to DeltaScope will be documented in this file.

The format follows Keep a Changelog and the project uses semantic versioning for release tags.

## [Unreleased]

## [v0.170.0] - 2026-05-24

### Added

- 4 new PostgreSQL-only ALTER TABLE notice rules covering final parseable boundaries:
  - Final parseable (4): `ddl.pg.alter.set_expression.notice` (notice), `ddl.pg.alter.add_identity.notice` (notice), `ddl.pg.alter.add_exclusion_constraint.notice` (notice), `ddl.pg.alter.move_all_tablespace.notice` (notice).
- PostgreSQL ALTER TABLE residual census: `finding_covered` 60 → 64, `unsupported_boundary` 0 (unchanged), `parser_error` 4 (unchanged).
- SQL corpus: 535/535 supported rule-dialect targets covered (100%), 243 expected YAML files.
- No-leak contract: findings do not emit expression body, sequence options, exclusion operators/predicates, or raw SQL.

### Non-Goals

- Not full PostgreSQL ALTER TABLE support.
- No PostgreSQL 18 parser support (4 `parser_error` forms remain blocked on `pg_query_go v7`).
- No runtime/live validation.
- No rewrite duration estimate.
- No v1.0/stable API contract claim.

## [v0.160.0] - 2026-05-23

### Added

- 2 new PostgreSQL-only ALTER TABLE constraint deferrability rules covering FK constraint deferrability changes:
  - Constraint deferrability (2): `ddl.pg.alter.constraint_deferrable.notice` (notice), `ddl.pg.alter.constraint_initially_deferred.notice` (notice).
- Silent normalization for `ALTER TABLE ... SET WITHOUT OIDS` (action: `set_without_oids`). No finding emitted — this is a table-only action obsolete since PG12.
- PostgreSQL ALTER TABLE residual census: `finding_covered` 54 → 56, `normalized_silent` 1 → 2, `unsupported_boundary` 7 → 4.
- SQL corpus: 531/531 supported rule-dialect targets covered (100%), 239 expected YAML files.
- No-leak contract: constraint deferrability findings do not emit raw SQL, expression text, predicate text, operator class names, exclusion operators, sequence options, catalog state, validation results, or dependency graphs. Metadata is limited to operation, action, table, constraint name, constraint type, and deferrable/initially_deferred boolean flags.

### Non-Goals

- Not full PostgreSQL ALTER TABLE support.
- No PostgreSQL 18 parser support (4 `parser_error` forms remain blocked on `pg_query_go v7`).
- No live catalog validation.
- No runtime FK behavior validation.
- No lock/rewrite duration estimate.
- No support yet for `SET EXPRESSION`, `ADD GENERATED ... AS IDENTITY`, `EXCLUDE USING`, `ALTER TABLE ALL IN TABLESPACE ... SET TABLESPACE ...`.
- Remaining `unsupported_boundary` forms (4) deferred to later milestones.
- No v1.0/stable API contract claim.

## [v0.150.0] - 2026-05-22

### Added

- 4 new PostgreSQL-only ALTER TABLE table relationship rules covering INHERIT and OF type operations:
  - Table relationship (4): `ddl.pg.alter.add_inherit.notice` (notice), `ddl.pg.alter.drop_inherit.notice` (notice), `ddl.pg.alter.add_of_type.notice` (notice), `ddl.pg.alter.drop_of_type.notice` (notice).
- PostgreSQL ALTER TABLE residual census: `finding_covered` 50 → 54, `unsupported_boundary` 11 → 7.
- SQL corpus: 529/529 supported rule-dialect targets covered (100%), 237 expected YAML files.
- No-leak contract: table relationship findings do not emit parent table names, typed table type names, or live schema validation claims.

### Non-Goals

- Not full PostgreSQL ALTER TABLE support.
- No live catalog validation.
- No rewrite duration estimate.
- No runtime behavior validation.
- No DCL expansion.
- Remaining `unsupported_boundary` forms (7) deferred to later milestones.
- No v1.0/stable API contract claim.

## [v0.140.0] - 2026-05-21

### Added

- 8 new PostgreSQL-only ALTER TABLE rules covering column attribute mutations and cluster/detach-finalize operations:
  - Column attributes (5): `ddl.pg.alter.set_column_statistics.notice` (notice), `ddl.pg.alter.set_column_options.notice` (notice), `ddl.pg.alter.reset_column_options.notice` (notice), `ddl.pg.alter.set_column_storage.notice` (notice), `ddl.pg.alter.set_column_compression.notice` (notice).
  - Cluster / detach-finalize (3): `ddl.pg.alter.cluster_on.notice` (notice), `ddl.pg.alter.set_without_cluster.notice` (notice), `ddl.pg.alter.detach_partition_finalize.notice` (notice).
- PostgreSQL ALTER TABLE residual census: `finding_covered` 40 → 50, `unsupported_boundary` 21 → 11.
- SQL corpus: 334 policy rule IDs, 525/525 supported rule-dialect targets covered (100%), 233 expected YAML files.
- No-leak contract: column attribute findings do not emit raw SQL, option names or values, storage parameter names, compression method names, or compression_kind; cluster/detach-finalize findings do not emit partition bounds, catalog index names, or live validation claims.

### Non-Goals

- Not full PostgreSQL ALTER TABLE support.
- No live catalog validation.
- No rewrite duration estimate.
- No runtime behavior validation.
- No DCL expansion.
- Remaining `unsupported_boundary` forms (11) deferred to later milestones.
- No v1.0/stable API contract claim.

## [v0.130.0] - 2026-05-19

### Added

- 10 new PostgreSQL ALTER TABLE residual rules across three semantic families:
  - Storage/layout (2): `ddl.pg.alter.set_tablespace.notice` (notice), `ddl.pg.alter.set_access_method.warn` (warning).
  - Trigger/rule residual (6): `ddl.pg.alter.enable_replica_trigger.notice` (notice), `ddl.pg.alter.enable_always_trigger.notice` (notice), `ddl.pg.alter.enable_rule.notice` (notice), `ddl.pg.alter.disable_rule.warn` (warning), `ddl.pg.alter.enable_replica_rule.notice` (notice), `ddl.pg.alter.enable_always_rule.notice` (notice).
  - Reloptions (2): `ddl.pg.alter.set_reloptions.warn` (warning), `ddl.pg.alter.reset_reloptions.notice` (notice).
- PostgreSQL ALTER TABLE residual census: `finding_covered` 29 → 40, `unsupported_boundary` 32 → 21.
- SQL corpus: 326 policy rule IDs, 517/517 supported rule-dialect targets covered (100%), 225 expected YAML files.
- No-leak contract: storage/layout findings do not emit raw tablespace or access method names; trigger/rule residual findings do not emit trigger function names, trigger body, rule body, or rule query text; reloptions findings do not emit option names or values.

### Non-Goals

- Not full PostgreSQL ALTER TABLE support.
- No live catalog validation.
- No rewrite duration estimate.
- No runtime behavior validation.
- No DCL expansion.
- Remaining `unsupported_boundary` forms deferred to later milestones.
- No v1.0/stable API contract claim.

## [v0.120.0] - 2026-05-17

### Added

- PostgreSQL migration-safety findings now include bounded semantic metadata for CREATE INDEX, ADD COLUMN, and ALTER COLUMN TYPE operations.
- `CREATE INDEX` findings include bounded index shape metadata:
  - `index_kind` — index classification (e.g., `secondary`, `unique`).
  - `access_method` — PostgreSQL access method name (e.g., `btree`, `gin`).
  - `column_count` — number of indexed columns.
  - `included_column_count` — number of INCLUDE covering columns.
  - `has_predicate` — whether the index has a WHERE clause.
  - `has_expression_keys` — whether any key column is an expression.
  - `expression_count` — number of expression key columns.
- `ADD COLUMN` rewrite and no-default findings include:
  - `not_null` — whether the column is NOT NULL.
  - `has_default` — whether the column has a DEFAULT.
  - `default_kind` — default value classification (`literal`, `null`, `function_call`, `expression`, `unknown`).
- `ALTER COLUMN TYPE` rewrite findings include:
  - `has_using` — whether the ALTER includes a USING clause.
- SDK/CLI/HTTP/MCP parity tests cover the new metadata across all four public surfaces.
- No-leak contract: findings do not emit predicate SQL text, expression index SQL text, default expression SQL text, default function names, or USING expression SQL text.

### Non-Goals

- No full PostgreSQL DDL support claim. This is bounded semantic metadata enrichment for selected migration-safety findings.
- No full PostgreSQL expression analysis. Expression and predicate presence is recorded as boolean/count metadata only; SQL text is not emitted.
- No function volatility/immutability analysis. `default_kind` classifies the AST node shape, not the runtime behavior.
- No live database/catalog validation.
- No DCL/permission expansion.
- No v1.0/stable API contract claim.

## [v0.110.0] - 2026-05-17

### Added

- Promoted `CREATE TRANSFORM` from deferred/unsupported to supported PostgreSQL DDL finding with bounded identity (`type@language`, e.g., `jsonb@plpython3u`). No `FROM SQL`/`TO SQL` function names are emitted.
- Promoted `CREATE ACCESS METHOD` from deferred/unsupported to supported PostgreSQL DDL finding with bounded identity (`amname`, e.g., `heap2`). No handler function name is emitted.
- PostgreSQL DDL long-tail census: 57 of 57 forms `finding_covered`, 0 `unsupported_boundary`.
- SQL corpus: 316 policy rule IDs, 507/507 supported rule-dialect targets covered (100%), 215 expected YAML fixtures (postgresql: 191, mysql: 13, tidb: 11).
- Payload safety: no handler, function, body, query, definition, or options payloads emitted. Object identities use bounded tokens only.

### Non-Goals

- No new object families beyond CREATE TRANSFORM and CREATE ACCESS METHOD.
- No full PostgreSQL DDL support claim. Selected long-tail object lifecycle families are covered; many forms remain deferred.
- No DCL/permission expansion beyond existing table-level grant/revoke support.
- No live DDL execution or migration outcome validation.
- No v1.0/stable API contract claim.

## [v0.100.0] - 2026-05-16

### Added

- 36 new PostgreSQL-only DDL lifecycle and boundary rules across 6 object families:
  - Collation lifecycle (3): `create_collation`, `alter_collation`, `drop_collation`.
  - Extended statistics lifecycle (3): `create_statistics`, `alter_statistics`, `drop_statistics`.
  - Aggregate/operator/conversion lifecycle (9): `create/alter/drop` for aggregate, operator, and conversion.
  - Operator family/class lifecycle (6): `create/alter/drop` for operator family and operator class.
  - Text search object lifecycle (12): `create/alter/drop` for text search configuration, dictionary, parser, and template.
  - Boundary closure (3): `drop_transform`, `drop_access_method`, `alter_large_object` (owner change).
- PostgreSQL DDL long-tail census: 55 of 57 forms `finding_covered`, 2 `unsupported_boundary` (intentionally deferred: `CREATE TRANSFORM` and `CREATE ACCESS METHOD` — handler/function names are the object identity, making safe normalization incompatible with payload safety constraints).
- SQL corpus: 314 policy rule IDs, 505/505 supported rule-dialect targets covered (100%), 213 expected YAML fixtures (postgresql: 189, mysql: 13, tidb: 11).
- Payload safety: normalized findings avoid handler, function, body, query, definition, and options payloads. Object identities use bounded tokens only (e.g., `jsonb@plpython3u` for transforms, OID integers for large objects, names for access methods).

### Non-Goals

- No full PostgreSQL DDL support claim. Selected long-tail object lifecycle families are covered; many forms remain deferred.
- No DCL/permission expansion beyond existing table-level grant/revoke support.
- No live DDL execution or migration outcome validation.
- No v1.0/stable API contract claim.

## [v0.90.0] - 2026-05-13

### Added

- PostgreSQL metadata-aware object validation: DeltaScope now resolves metadata for selected non-table PostgreSQL objects (types, domains, extensions, sequences, materialized views, schemas, foreign servers, user mappings, publications, comments) during metadata-aware audit, enriching lifecycle rule findings with object existence and attribute information.
- Object metadata resolution pipeline: `ObjectSnapshot` with `Status` (`confirmed`, `not_found`, `unavailable`), `Exists`, `Schema`, `Type`, `Name`, and safe `Attributes` projected into findings.
- `ResolveObject` interface on metadata clients: PostgreSQL metadata provider now resolves object metadata through `pg_catalog` queries. MySQL/TiDB metadata provider returns `unavailable` for object resolution (no behavior change).
- CLI, HTTP, and MCP adapters forward `ResolveObject` through `cliMetadataProvider` / adapter wrappers so all three transports surface object metadata in finding output.
- Metadata-aware finding enrichment: supported PostgreSQL lifecycle rules (`drop_schema`, `drop_type`, `drop_domain`, `drop_extension`, `drop_sequence`, `drop_materialized_view`, `drop_publication`, `drop_foreign_server`, `drop_user_mapping`, `comment_on`) now include `metadata_status`, `metadata_object_type`, `metadata_object_name`, `metadata_exists`, and safe projectable attributes when metadata is available.
- Safe metadata attribute projection: dual protection via `sensitiveAttributeKeys` blacklist (blocks password, secret, connection string, body, definition, comment text, label, query, action_sql, options values) and `projectableAttributeKeys` whitelist (8 safe keys: `type_kind`, `extension_version`, `enabled`, `server`, `foreign_data_wrapper`, `target_type`, `has_options`, `table`).
- Docker-backed E2E test suite for PostgreSQL object metadata: 12 cases covering confirmed/not_found statuses across 10 object types with privacy assertions.
- Schema-qualified name parsing fix: parser functions (`dropTypeNameFromObjects`, `objectNameFromNode`) now correctly return the last name part for schema-qualified names like `app.email_address` instead of the first part (`app`).

### Changed

- Parser name resolution: `dropTypeNameFromObjects` and `objectNameFromNode` iterate backwards through AST name arrays to extract the correct object name from qualified identifiers.
- CLI adapter: `cliMetadataProvider` wrapper forwards `ResolveObject` via interface assertion, matching HTTP/MCP adapter patterns.

### Non-Goals

- No new rule IDs. v0.90.0 enriches existing rule findings with metadata; it does not add rules.
- No full PostgreSQL DDL support claim. Selected non-permission DDL families are covered; many remain deferred.
- No live privilege/role validation. GRANT/REVOKE rules emit informational notices only.
- No DCL execution or runtime database firewall behavior.
- DeltaScope does not execute migrations.
- MySQL/TiDB object metadata resolution remains `unavailable` — no behavior change.
- Only 8 safe attribute keys are projected into findings. All other attributes are filtered by the dual blacklist/whitelist.

## [v0.80.0] - 2026-05-13

### Added

- Selected PostgreSQL non-permission DDL deep coverage: 36 new PostgreSQL-only DDL lifecycle rules across 6 families (composite type attributes, extension members, publication/subscription, foreign objects, annotation lifecycle, event trigger/rewrite rule).
- Composite type attribute lifecycle rules:
  - `ddl.pg.alter_type.add_attribute.notice` (notice) — fires on `ALTER TYPE ... ADD ATTRIBUTE`
  - `ddl.pg.alter_type.drop_attribute.warn` (warning) — fires on `ALTER TYPE ... DROP ATTRIBUTE`
  - `ddl.pg.alter_type.alter_attribute_type.warn` (warning) — fires on `ALTER TYPE ... ALTER ATTRIBUTE ... TYPE`
  - `ddl.pg.alter_type.rename_attribute.notice` (notice) — fires on `ALTER TYPE ... RENAME ATTRIBUTE`
- Extension member lifecycle rules:
  - `ddl.pg.alter_extension.add_member.notice` (notice) — fires on `ALTER EXTENSION ... ADD TABLE`
  - `ddl.pg.alter_extension.drop_member.warn` (warning) — fires on `ALTER EXTENSION ... DROP TABLE`
- Publication/subscription lifecycle rules:
  - `ddl.pg.create_publication.notice` (notice) — fires on `CREATE PUBLICATION`
  - `ddl.pg.alter_publication.notice` (notice) — fires on `ALTER PUBLICATION`
  - `ddl.pg.drop_publication.warn` (warning) — fires on `DROP PUBLICATION`
  - `ddl.pg.create_subscription.notice` (notice) — fires on `CREATE SUBSCRIPTION`
  - `ddl.pg.alter_subscription.notice` (notice) — fires on `ALTER SUBSCRIPTION`
  - `ddl.pg.alter_subscription.disable.warn` (warning) — fires on `ALTER SUBSCRIPTION ... DISABLE`
  - `ddl.pg.drop_subscription.warn` (warning) — fires on `DROP SUBSCRIPTION`
- Foreign object lifecycle rules:
  - `ddl.pg.create_foreign_table.notice` (notice) — fires on `CREATE FOREIGN TABLE`
  - `ddl.pg.alter_foreign_table.notice` (notice) — fires on `ALTER FOREIGN TABLE`
  - `ddl.pg.drop_foreign_table.warn` (warning) — fires on `DROP FOREIGN TABLE`
  - `ddl.pg.create_foreign_server.notice` (notice) — fires on `CREATE SERVER`
  - `ddl.pg.alter_foreign_server.notice` (notice) — fires on `ALTER SERVER`
  - `ddl.pg.drop_foreign_server.warn` (warning) — fires on `DROP SERVER`
  - `ddl.pg.create_user_mapping.notice` (notice) — fires on `CREATE USER MAPPING`
  - `ddl.pg.alter_user_mapping.notice` (notice) — fires on `ALTER USER MAPPING`
  - `ddl.pg.drop_user_mapping.warn` (warning) — fires on `DROP USER MAPPING`
  - `ddl.pg.create_foreign_data_wrapper.notice` (notice) — fires on `CREATE FOREIGN DATA WRAPPER`
  - `ddl.pg.alter_foreign_data_wrapper.notice` (notice) — fires on `ALTER FOREIGN DATA WRAPPER`
  - `ddl.pg.drop_foreign_data_wrapper.warn` (warning) — fires on `DROP FOREIGN DATA WRAPPER`
- Annotation lifecycle rules:
  - `ddl.pg.comment_on.notice` (notice) — fires on `COMMENT ON`
  - `ddl.pg.comment_on.remove.notice` (notice) — fires on `COMMENT ON ... IS NULL`
  - `ddl.pg.security_label.notice` (notice) — fires on `SECURITY LABEL`
  - `ddl.pg.security_label.remove.notice` (notice) — fires on `SECURITY LABEL ... IS NULL`
- Event trigger/rewrite rule lifecycle rules:
  - `ddl.pg.create_event_trigger.notice` (notice) — fires on `CREATE EVENT TRIGGER`
  - `ddl.pg.alter_event_trigger.notice` (notice) — fires on `ALTER EVENT TRIGGER`
  - `ddl.pg.alter_event_trigger.disable.warn` (warning) — fires on `ALTER EVENT TRIGGER ... DISABLE`
  - `ddl.pg.drop_event_trigger.warn` (warning) — fires on `DROP EVENT TRIGGER`
  - `ddl.pg.create_rule.notice` (notice) — fires on `CREATE RULE`
  - `ddl.pg.alter_rule.notice` (notice) — fires on `ALTER RULE`
  - `ddl.pg.drop_rule.warn` (warning) — fires on `DROP RULE`
- PostgreSQL DDL deep coverage census: 38 selected non-permission DDL forms are now `finding_covered` across composite type attributes, extension members, publication/subscription, foreign objects, annotation, event trigger, and rewrite rule lifecycle.
- SQL corpus: 278 policy rules, 469/469 supported targets, 100% coverage.
- Public surface coverage verified across SDK, CLI, HTTP, and MCP for all 36 new rules.
- Privacy boundary: expected corpus outputs do not contain subscription connection strings, foreign object option values, comment text, security label text, event trigger function bodies, or rewrite rule bodies.

### Changed

- SQL corpus expanded from 433 to 469 supported targets (36 new PostgreSQL fixtures).

### Non-Goals

- No full PostgreSQL DDL support claim. Selected non-permission DDL families are covered; many remain deferred (see below).
- No live privilege/role validation. GRANT/REVOKE rules emit informational notices only.
- No DCL execution or runtime database firewall behavior.
- DeltaScope does not execute migrations.
- Deferred: `DROP SUBSCRIPTION ... WITH (drop_slot = true)` remains a genuine parser error.
- Deferred PG DDL families: broader PostgreSQL grammar, aggregate/operator/conversion/statistics lifecycle, and other out-of-scope families remain intentionally not covered.
- Permissions/DCL remain out of scope for runtime validation: GRANT/REVOKE, roles/users, default privileges (except pre-existing v0.60.0 table-level DCL support).

## [v0.70.0] - 2026-05-12

### Added

- Selected PostgreSQL non-permission DDL lifecycle coverage for v0.70.0 scope: RLS/Policy, Trigger, Function/Procedure, Advanced View, and selected ALTER object lifecycle rules.
- RLS/Policy lifecycle rules:
  - `ddl.pg.create_policy.notice` (notice) — fires on `CREATE POLICY`
  - `ddl.pg.alter_policy.notice` (notice) — fires on `ALTER POLICY`
  - `ddl.pg.drop_policy.warn` (warning) — fires on `DROP POLICY`
  - `ddl.pg.alter.enable_rls.notice` (notice) — fires on `ALTER TABLE ... ENABLE ROW LEVEL SECURITY`
  - `ddl.pg.alter.disable_rls.warn` (warning) — fires on `ALTER TABLE ... DISABLE ROW LEVEL SECURITY`
  - `ddl.pg.alter.force_rls.notice` (notice) — fires on `ALTER TABLE ... FORCE ROW LEVEL SECURITY`
  - `ddl.pg.alter.no_force_rls.notice` (notice) — fires on `ALTER TABLE ... NO FORCE ROW LEVEL SECURITY`
- Trigger lifecycle rules:
  - `ddl.pg.create_trigger.notice` (notice) — fires on `CREATE TRIGGER`
  - `ddl.pg.create_constraint_trigger.warn` (warning) — fires on `CREATE CONSTRAINT TRIGGER`
  - `ddl.pg.drop_trigger.advisory` (notice) — fires on `DROP TRIGGER`
- Function/Procedure lifecycle rules:
  - `ddl.pg.create_function.notice` (notice) — fires on `CREATE FUNCTION`
  - `ddl.pg.create_function.security_definer.warn` (warning) — fires on `CREATE FUNCTION ... SECURITY DEFINER`
  - `ddl.pg.create_or_replace_function.advisory` (notice) — fires on `CREATE OR REPLACE FUNCTION`
  - `ddl.pg.drop_function.advisory` (notice) — fires on `DROP FUNCTION`
  - `ddl.pg.create_procedure.notice` (notice) — fires on `CREATE PROCEDURE`
  - `ddl.pg.drop_procedure.advisory` (notice) — fires on `DROP PROCEDURE`
- Advanced View lifecycle rules:
  - `ddl.pg.create_or_replace_view.advisory` (notice) — fires on `CREATE OR REPLACE VIEW`
  - `ddl.pg.create_temp_view.notice` (notice) — fires on `CREATE TEMP VIEW` / `CREATE TEMPORARY VIEW`
  - `ddl.pg.create_view.check_option.notice` (notice) — fires on `CREATE VIEW ... WITH CHECK OPTION`
  - `ddl.pg.drop_view.cascade.warn` (warning) — fires on `DROP VIEW ... CASCADE`
  - `ddl.pg.alter_view.rename.notice` (notice) — fires on `ALTER VIEW ... RENAME TO`
  - `ddl.pg.alter_view.set_schema.notice` (notice) — fires on `ALTER VIEW ... SET SCHEMA`
- Selected ALTER object lifecycle rules:
  - `ddl.pg.alter_schema.rename.notice` (notice) — fires on `ALTER SCHEMA ... RENAME TO`
  - `ddl.pg.alter_schema.owner.notice` (notice) — fires on `ALTER SCHEMA ... OWNER TO`
  - `ddl.pg.alter_index.rename.notice` (notice) — fires on `ALTER INDEX ... RENAME TO`
  - `ddl.pg.alter_index.set_tablespace.notice` (notice) — fires on `ALTER INDEX ... SET TABLESPACE`
  - `ddl.pg.alter_materialized_view.rename.notice` (notice) — fires on `ALTER MATERIALIZED VIEW ... RENAME TO`
  - `ddl.pg.alter_materialized_view.set_schema.notice` (notice) — fires on `ALTER MATERIALIZED VIEW ... SET SCHEMA`
- PostgreSQL DDL completion census: 31 selected non-permission DDL forms are now `finding_covered` (RLS/Policy, Trigger, Function/Procedure, View, and selected ALTER object lifecycle).
- SQL corpus: 242 policy rules, 433/433 supported targets, 100% coverage.
- Public surface coverage verified across SDK, CLI, HTTP, and MCP for all new rules.
- Standalone `ALTER INDEX` metadata resolution path: `metadataTargetTableName` now resolves standalone index operations through `IndexOwnerResolver` for PostgreSQL metadata-aware audit.
- `pgObjectLifecycleRule` pattern with `newPGObjectLifecycleRuleWithAction` supporting operation + action + objectType matching.
- PostgreSQL extractor coverage for `ALTER SCHEMA`, `ALTER INDEX`, and `ALTER MATERIALIZED VIEW` lifecycle forms via `extractRenameStmt`, `extractAlterOwnerStmt`, `extractAlterObjectSchemaStmt`, and `extractAlterTableStmt`.

### Changed

- PostgreSQL `ALTER INDEX ... RENAME TO` now produces `ddl.pg.alter_index.rename.notice` instead of passing silently.
- PostgreSQL `ALTER MATERIALIZED VIEW` rename and set-schema now produce explicit findings instead of passing silently.
- PostgreSQL `ALTER SCHEMA` rename and owner now produce explicit findings instead of passing silently.
- SQL corpus expanded from 405 to 433 supported targets (28 new PostgreSQL fixtures).

### Non-Goals

- No full PostgreSQL DDL support claim. Selected non-permission DDL families are covered; many remain deferred (see below).
- No live privilege/role validation. GRANT/REVOKE rules emit informational notices only.
- No DCL execution or runtime database firewall behavior.
- DeltaScope does not execute migrations.
- Deferred PG DDL families: `ALTER TYPE` deep mutation, `ALTER DOMAIN` beyond rename, `ALTER EXTENSION` expanded lifecycle, `ALTER FOREIGN TABLE`/`SERVER`/`USER MAPPING`, `ALTER AGGREGATE`/`OPERATOR`/`CONVERSION`, `ALTER PUBLICATION`/`SUBSCRIPTION`, `ALTER STATISTICS`/`ALTER RULE`, `COMMENT ON`, `SECURITY LABEL`.
- Permissions/DCL remain out of scope for runtime validation: GRANT/REVOKE, roles/users, default privileges.

## [v0.64.0] - 2026-05-11

### Added

- Cross-dialect DDL coverage census characterizing representative DDL forms across MySQL, TiDB, and PostgreSQL (census report is not committed; classification tests are committed).
- MySQL/TiDB database/schema lifecycle normalization: `CREATE DATABASE`, `CREATE DATABASE IF NOT EXISTS`, `CREATE SCHEMA`, `DROP DATABASE`, `DROP DATABASE IF EXISTS`, `DROP SCHEMA` now pass through the audit pipeline instead of passing silently.
- PostgreSQL `CREATE SCHEMA` normalization: `CREATE SCHEMA` and `CREATE SCHEMA IF NOT EXISTS` now produce an explicit notice instead of passing silently.
- Three new audit rules:
  - `ddl.database.create.notice` (MySQL/TiDB, notice) — fires on `CREATE DATABASE` / `CREATE SCHEMA`
  - `ddl.database.drop.warn` (MySQL/TiDB, warning) — fires on `DROP DATABASE` / `DROP SCHEMA`
  - `ddl.pg.create_schema.notice` (PostgreSQL, notice) — fires on `CREATE SCHEMA`
- Default policy entries for all three new rules.
- SQL corpus fixtures covering all three new rules across MySQL, TiDB, and PostgreSQL.
- Public surface tests across CLI, HTTP, MCP, and SDK for all new rules.
- AST characterization tests documenting stable parser facts for database/schema lifecycle forms.

### Changed

- MySQL/TiDB `CREATE DATABASE`/`DROP DATABASE` and synonyms (`CREATE SCHEMA`/`DROP SCHEMA`) were previously normalized silent; they now produce findings.
- PostgreSQL `CREATE SCHEMA` was previously normalized silent; it now produces a notice finding.
- MySQL/TiDB `CREATE DATABASE ... CHARACTER SET`/`COLLATE` options are preserved as parser facts but no policy rule governs them.
- SQL corpus: 214 policy rules, 405/405 supported targets, 100% coverage.

### Non-Goals

- No full DDL support claim. Trigger, routine, event, and database privilege lifecycle remain deferred.
- No live database/schema existence validation.
- No charset/collation/tablespace/owner validation.
- `CREATE SCHEMA AUTHORIZATION` and nested `CREATE SCHEMA ... CREATE TABLE ...` on PostgreSQL remain unsupported/deferred.
- Existing PostgreSQL `DROP SCHEMA` behavior (`ddl.pg.drop_schema.advisory`, `ddl.pg.drop_schema.cascade.warn`) is unchanged.
- DeltaScope does not execute migrations.

## [v0.63.0] - 2026-05-10

### Added

- Runtime config for server and MCP operations (`-runtime-config` flag): load a YAML file with logging and metadata defaults separate from policy config
- Runtime config logging: `level`, `format`, `output`, `file`, and `rotation` (max size, max age, max backups, compress)
- Runtime config metadata: `metadata.connect_timeout` default for server/MCP metadata connections
- MCP stdout logging remains forbidden to protect the stdio protocol; runtime config can set `output: file` or `output: stderr`, but not `stdout`
- CLI metadata connect timeout: `--metadata-connect-timeout` flag for explicit per-request timeout
- HTTP metadata connect timeout: `connection.connect_timeout` in the JSON request body
- MCP metadata connect timeout: `connect_timeout` on direct and named connection inputs
- Timeout precedence: request-level `connect_timeout` > runtime `metadata.connect_timeout` > opener default; empty or `0s` means unset/default
- Runtime config and metadata timeout examples in adoption docs

### Changed

- Public surface and E2E coverage for all new inputs across CLI, HTTP, and MCP surfaces
- Full E2E matrix coverage: CLI/HTTP/MCP x MySQL/TiDB/PostgreSQL
- SQL corpus 400/400 targets and fixture execution verified

### Documentation

- Runtime config example for server and MCP
- CLI, HTTP, and MCP metadata connect timeout examples
- SQL audit MCP server and TiDB schema change audit adoption pages

### Non-Goals

- No new SQL rules in v0.63.0
- No SQL rule severity or default policy changes
- Parser cache remains deferred
- Runtime config is separate from policy config (`--config`)
- Runtime config currently applies to `deltascope-server` and `deltascope-mcp`, not ordinary CLI logging
- SDK `deltascope.Request` does NOT have `MetadataConnectTimeout`; SDK callers that pass their own `MetadataProvider` manage connection behavior themselves
- DeltaScope does not execute migrations and is not a database proxy or runtime query firewall
- No live privilege/role validation expansion

## [v0.62.0] - 2026-05-10

### Added

- Structured logging foundation: server (`-log-output`, `-log-level`, `-log-file`) and MCP (`-log-output`, `-log-level`, `-log-file`) logging flags
- Log file rotation support with configurable max size, max age, max backups, and compress options
- Internal 5s default connect timeout for metadata-aware MySQL/TiDB and PostgreSQL openers; `OpenDBContext` propagates caller cancellation
- Parser benchmark coverage for hot paths

### Changed

- Code maintainability: `defaults.go` split into 5 files by rule category, `extractor.go` split into 7 files by statement type
- Context propagation improved in boundary error wrapping and impact estimation

### Fixed

- Log file and directory permissions restricted to owner-only (`0750` for directories, `0600` for files)

### Documentation

- SQL audit articles for CSDN/Zhihu/Juejin (Chinese and English versions)
- DeltaScope name links updated to official site across articles

## [v0.61.0] - 2026-05-08

### Added

- Static analysis integration: golangci-lint v2 with 15 active linters and 903 code-quality issues auto-fixed
- Context propagation support: audit pipeline now respects `context.Context` timeout and cancellation across all layers
- Parallel test execution: 1522 tests run concurrently with `t.Parallel()`, reducing CI time
- Performance benchmarks for hot paths (rule evaluation, markdown rendering, string concatenation)
- MCP server panic recovery with three-layer protection: tool handler, server handler, and process-level recovery

### Changed

- Performance optimization in hot paths: slice preallocation, `strings.Builder` for string concatenation, builder pool for markdown renderer reuse
- Database connection pool configuration hardened: connection leaks fixed, proper lifecycle management with `sql.DB.SetMaxOpenConns`, `SetMaxIdleConns`, and `SetConnMaxLifetime`
- golangci-lint configuration updated to v2 format

### Fixed

- Database connection pool leaks under concurrent metadata-aware audit workloads
- MCP server crashes from unexpected panics in tool handlers now recover gracefully instead of terminating the process
- 903 lint issues across the codebase (unused variables, error handling, naming conventions, and more)

## [v0.60.0] - 2026-05-06

### Added

- PostgreSQL table-level privilege DCL narrow support: `GRANT ... ON TABLE` and `REVOKE ... ON TABLE` now pass through the audit pipeline instead of returning unsupported.
- Four new PostgreSQL-only rules: `ddl.pg.grant.table_privilege.notice` (notice), `ddl.pg.grant.table_privilege.all.warn` (warning), `ddl.pg.revoke.table_privilege.notice` (notice), `ddl.pg.revoke.table_privilege.cascade.warn` (warning).
- `GRANT ALL PRIVILEGES ON TABLE` triggers both `ddl.pg.grant.table_privilege.notice` and `ddl.pg.grant.table_privilege.all.warn` — duplicate findings are intentional.
- `REVOKE ... ON TABLE ... CASCADE` triggers both `ddl.pg.revoke.table_privilege.notice` and `ddl.pg.revoke.table_privilege.cascade.warn` — duplicate findings are intentional.
- Supports single-table GRANT/REVOKE with multiple privileges (e.g., `SELECT, INSERT`), multiple grantees, and schema-qualified table names (e.g., `public.users`).
- Corpus fixtures covering all four new rules' trigger forms.
- Service-level tests through `AuditSQL` for representative table privilege DCL variants.
- Public surface tests across all four surfaces: `pkg/deltascope` Audit, CLI Execute, HTTP handler, and MCP audit_sql tool.
- AST census tests documenting stable parser facts for all table privilege DCL forms.

### Non-Goals

- DeltaScope does not perform live validation of any kind for table privileges: no grantee/role existence checks, no table/object existence checks, no grantor permission checks, no effective privilege computation, no role inheritance resolution, no ownership verification, no RLS/policy evaluation.
- `ALL TABLES IN SCHEMA` GRANT/REVOKE is not supported.
- Sequence privileges are not supported.
- Role membership GRANT/REVOKE is not supported.
- `ALTER DEFAULT PRIVILEGES` is not supported.
- This is narrow table-level privilege DCL support — not broad governance or admin DCL support.
- No MySQL/TiDB behavior changes.
- No default policy changes beyond the four new PostgreSQL-only rule entries.

## [v0.59.0] - 2026-05-06

### Added

- PostgreSQL extension lifecycle narrow support: `CREATE EXTENSION`, `ALTER EXTENSION` (`UPDATE`, `UPDATE TO`, `SET SCHEMA`), and `DROP EXTENSION` now pass through the audit pipeline instead of returning unsupported.
- Six new PostgreSQL-only rules: `ddl.pg.create_extension.notice` (notice), `ddl.pg.create_extension.cascade.warn` (warning), `ddl.pg.alter_extension.update.notice` (notice), `ddl.pg.alter_extension.set_schema.notice` (notice), `ddl.pg.drop_extension.advisory` (warning), `ddl.pg.drop_extension.cascade.warn` (warning).
- `CREATE EXTENSION ... CASCADE` triggers both the base `notice` and `cascade.warn` rules; `DROP EXTENSION ... CASCADE` triggers both the base `advisory` and `cascade.warn` rules — duplicate findings are intentional.
- Corpus fixtures covering all six new rules' trigger forms.
- Service-level tests through `AuditSQL` for representative extension lifecycle variants.
- Public surface tests across all four surfaces: `pkg/deltascope` Audit, CLI Execute, HTTP handler, and MCP audit_sql tool.
- AST census tests documenting stable parser facts for all extension lifecycle forms.

### Non-Goals

- DeltaScope does not perform live dependency validation on extensions.
- DeltaScope does not model full PostgreSQL extension system semantics.
- This is narrow extension lifecycle support — not broad governance or admin DDL support.
- Extension member mutation (`ALTER EXTENSION ... ADD/DROP TABLE`) remains explicitly deferred.
- No MySQL/TiDB behavior changes.
- No default policy changes beyond the six new PostgreSQL-only rule entries.

## [v0.58.0] - 2026-05-06

### Added

- PostgreSQL composite type lifecycle narrow support: `CREATE TYPE ... AS (...)`, `ALTER TYPE ... RENAME TO`, and `ALTER TYPE ... SET SCHEMA` now pass through the audit pipeline instead of returning unsupported.
- Three new PostgreSQL-only rules: `ddl.pg.create_type.composite.notice` (notice), `ddl.pg.alter_type.composite_rename.notice` (notice), `ddl.pg.alter_type.composite_set_schema.notice` (notice).
- `DROP TYPE` for composite types reuses existing v0.55.0 rules (`ddl.pg.drop_type.advisory`, `ddl.pg.drop_type.cascade.warn`) — no new composite-specific DROP TYPE rule.
- Collation annotations in composite type attribute definitions are recognized structurally but not interpreted or validated.
- Corpus fixtures covering all three new rules' trigger forms.
- Service-level tests through `AuditSQL` for representative composite lifecycle variants.
- Public surface tests across all four surfaces: `pkg/deltascope` Audit, CLI Execute, HTTP handler, and MCP audit_sql tool.
- AST census tests documenting stable parser facts for all composite type lifecycle forms.

### Non-Goals

- DeltaScope does not perform live dependency validation on composite types.
- DeltaScope does not model full PostgreSQL type system semantics.
- This is narrow composite type lifecycle support — not complete PostgreSQL type system support.
- Attribute-level operations (`ADD ATTRIBUTE`, `DROP ATTRIBUTE`, `ALTER ATTRIBUTE ... TYPE`, `RENAME ATTRIBUTE`) remain explicitly deferred.
- No MySQL/TiDB behavior changes.
- No default policy changes beyond the three new PostgreSQL-only rule entries.

## [v0.57.0] - 2026-05-05

### Added

- PostgreSQL domain lifecycle DDL normalization: `CREATE DOMAIN`, `ALTER DOMAIN` (SET/DROP DEFAULT, SET/DROP NOT NULL, ADD/DROP/VALIDATE CONSTRAINT, RENAME TO), and `DROP DOMAIN` (including `IF EXISTS ... CASCADE`) now pass through the audit pipeline instead of returning unsupported.
- Seven new PostgreSQL-only rules: `ddl.pg.create_domain.notice` (notice), `ddl.pg.alter_domain.constraint.notice` (notice), `ddl.pg.alter_domain.default.notice` (notice), `ddl.pg.alter_domain.not_null.notice` (notice), `ddl.pg.alter_domain.rename.notice` (notice), `ddl.pg.drop_domain.advisory` (warning), `ddl.pg.drop_domain.cascade.warn` (warning).
- Corpus fixtures covering all seven new rules' trigger forms.
- Service-level tests through `AuditSQL` for 12 domain lifecycle variants.
- Public surface tests across all four surfaces: `pkg/deltascope` Audit, CLI Execute, HTTP handler, and MCP audit_sql tool.
- AST census tests documenting stable parser facts for all 15 domain lifecycle forms.

### Non-Goals

- DeltaScope does not render `CHECK` or `DEFAULT` expression text. Rules emit boolean facts (`has_check`, `has_default`, `not_null`) and constraint names, but never the expression body.
- DeltaScope does not perform live dependency validation on domains.
- `CREATE TYPE ... AS (...)` composite types remain explicitly unsupported as `create_type_composite`.
- No MySQL/TiDB behavior changes.
- No default policy changes beyond the seven new PostgreSQL-only rule entries.

## [v0.56.0] - 2026-05-04

### Added

- PostgreSQL ALTER TABLE logged-state normalization: `ALTER TABLE ... SET LOGGED` and `ALTER TABLE ... SET UNLOGGED` now pass through the audit pipeline instead of returning unsupported.
- Two new PostgreSQL-only rules: `ddl.pg.alter.set_logged.notice` (notice) and `ddl.pg.alter.set_unlogged.notice` (notice).
- ALTER COLUMN TYPE USING metadata: the USING expression is now captured in normalized alter metadata for visibility during review.
- Corpus fixtures covering both new rules' trigger forms.
- Service-level tests through `AuditSQL` for logged-state variants.
- Public surface tests across all four surfaces: `pkg/deltascope` Audit, CLI Execute, HTTP handler, and MCP audit_sql tool.
- AST census tests documenting stable parser facts for logged-state forms.

### Non-Goals

- DeltaScope does not verify whether the target table is currently logged or unlogged.
- DeltaScope does not evaluate WAL or replication implications of logged-state transitions.
- This is not full PostgreSQL ALTER TABLE grammar support.
- No MySQL/TiDB behavior changes.
- No default policy changes beyond the two new PostgreSQL-only rule entries.

## [v0.55.0] - 2026-05-02

### Added

- PostgreSQL type lifecycle DDL normalization: `CREATE TYPE ... AS ENUM`, `ALTER TYPE ... ADD VALUE` (including `IF NOT EXISTS`, `BEFORE`, `AFTER` variants), and `DROP TYPE` (including `IF EXISTS ... CASCADE`) now pass through the audit pipeline instead of returning unsupported.
- Five new PostgreSQL-only rules: `ddl.pg.create_type.enum.notice` (notice), `ddl.pg.alter_type.add_value.advisory` (warning), `ddl.pg.alter_type.add_value.position.notice` (notice), `ddl.pg.drop_type.advisory` (warning), `ddl.pg.drop_type.cascade.warn` (warning).
- Corpus fixtures covering all five new rules' trigger forms.
- Service-level tests through `AuditSQL` for type lifecycle variants.
- Public surface tests across all four surfaces: `pkg/deltascope` Audit, CLI Execute, HTTP handler, and MCP audit_sql tool.
- AST census tests documenting stable parser facts for all seven supported type lifecycle forms and two deferred forms.

### Non-Goals

- DeltaScope does not inspect live dependent objects.
- DeltaScope does not validate whether enum values are already used by data or application code.
- DeltaScope does not model full PostgreSQL type system semantics.
- This is not full PostgreSQL type lifecycle support.
- `CREATE TYPE ... AS (...)` composite types remain explicit unsupported as `create_type_composite`.
- `CREATE DOMAIN ...` remains explicit unsupported as `create_domain`.
- No MySQL/TiDB behavior changes.
- No default policy changes beyond the five new PostgreSQL-only rule entries.

## [v0.54.0] - 2026-05-02

### Added

- PostgreSQL ALTER TABLE trigger-scope normalization: `ENABLE/DISABLE TRIGGER ALL` and `ENABLE/DISABLE TRIGGER USER` now pass through the audit pipeline, reusing existing `ddl.pg.alter.enable_trigger.notice` and `ddl.pg.alter.disable_trigger.warn` rules.
- PostgreSQL ALTER TABLE replica identity normalization: all four `REPLICA IDENTITY` variants (`DEFAULT`, `FULL`, `NOTHING`, `USING INDEX ...`) now pass through the audit pipeline instead of returning unsupported.
- Three new PostgreSQL-only rules: `ddl.pg.alter.replica_identity_full.warn` (warning), `ddl.pg.alter.replica_identity_nothing.warn` (warning), `ddl.pg.alter.replica_identity_using_index.notice` (notice).
- `REPLICA IDENTITY DEFAULT` normalizes as a clean silent pass — no rule fires.
- Corpus fixtures covering all three new rules' trigger forms.
- Service-level tests through `AuditSQL` for replica identity and trigger-scope variants.
- Public surface tests across all four surfaces: `pkg/deltascope` Audit, CLI Execute, HTTP handler, and MCP audit_sql tool.
- AST census tests documenting stable parser facts for all eight residual forms.

### Non-Goals

- DeltaScope does not inspect live trigger state or validate trigger definitions or functions.
- DeltaScope does not verify whether `REPLICA IDENTITY USING INDEX` names a valid, unique, or non-partial index.
- This is not full PostgreSQL ALTER TABLE grammar support.
- No MySQL/TiDB behavior changes.
- No default policy changes beyond the three new PostgreSQL-only rule entries.

## [v0.53.0] - 2026-05-01

### Added

- PostgreSQL `REFRESH MATERIALIZED VIEW` normalization: all four variants (basic, `CONCURRENTLY`, `WITH DATA`, `WITH NO DATA`) now pass through the audit pipeline instead of returning unsupported.
- Two new PostgreSQL-only rules: `ddl.pg.refresh_materialized_view.concurrently.warn` (warning) and `ddl.pg.refresh_materialized_view.no_data.notice` (notice).
- Corpus fixtures covering both rules' trigger forms.
- Service-level tests through `AuditSQL` for all four refresh variants.
- Public surface tests across all four surfaces: `pkg/deltascope` Audit, CLI Execute, HTTP handler, and MCP audit_sql tool.
- AST census tests documenting stable parser facts for all four refresh variants.

### Non-Goals

- Not live unique-index validation for `CONCURRENTLY`. DeltaScope does not verify whether a unique index exists on the materialized view.
- No query, cost, or dependency analysis is performed on the underlying view query.
- No MySQL/TiDB behavior changes.
- No default policy changes beyond the two new PostgreSQL-only rule entries.

## [v0.52.0] - 2026-05-01

### Added

- Six new PostgreSQL-only ALTER TABLE unsupported-action rules: `ddl.pg.alter.set_schema.advisory` (notice), `ddl.pg.alter.owner.advisory` (notice), `ddl.pg.alter.enable_trigger.notice` (notice), `ddl.pg.alter.disable_trigger.warn` (warning), `ddl.pg.alter.attach_partition.advisory` (notice), `ddl.pg.alter.detach_partition.warn` (warning).
- Parser/extractor normalization for all six ALTER TABLE action types (SET SCHEMA, OWNER TO, ENABLE/DISABLE TRIGGER name, ATTACH/DETACH PARTITION).
- Corpus fixtures covering each rule's trigger forms.
- Service-level tests through `AuditSQL` for all six rules.
- Public surface tests across `pkg/deltascope`, CLI, HTTP handler, and MCP tool.
- AST census tests documenting stable parser facts for each action type.

### Non-Goals

- Not full PostgreSQL ALTER TABLE grammar support. Remaining sub-commands (e.g., `ALTER COLUMN TYPE`, `ENABLE/DISABLE TRIGGER ALL/USER`, `REPLICA IDENTITY`) are explicit boundaries.
- Partition bound semantic analysis is not performed.
- No MySQL/TiDB behavior changes.
- No default policy changes beyond the six new PostgreSQL-only rule entries.

## [v0.51.0] - 2026-04-30

### Added

- Three new PostgreSQL-only ALTER TABLE gap-fill rules: `ddl.pg.alter.drop_column.advisory` (warning), `ddl.pg.alter.validate_constraint.advisory` (notice), `ddl.pg.alter.add_column.nullable.notice` (notice).
- Corpus fixtures for each rule's trigger forms.
- Service-level tests through `AuditSQL` for all three rules.
- Public surface tests across `pkg/deltascope`, CLI, HTTP handler, and MCP tool.

### Non-Goals

- Not full PostgreSQL ALTER TABLE coverage. Remaining ALTER TABLE sub-commands (e.g., `ALTER COLUMN TYPE`, `ADD CONSTRAINT ... NOT VALID`, `DISABLE TRIGGER`) remain explicit boundaries.
- No MySQL/TiDB behavior changes.
- No default policy changes beyond the three new PostgreSQL-only rule entries.

## [v0.50.0] - 2026-04-30

### Added

- PostgreSQL object lifecycle DDL normalization: schemas, sequences, and materialized views now pass through the audit pipeline instead of returning unsupported.
- Newly normalized operations: `CREATE SCHEMA`, `DROP SCHEMA`, `CREATE SEQUENCE`, `ALTER SEQUENCE`, `DROP SEQUENCE`, `CREATE MATERIALIZED VIEW`, `DROP MATERIALIZED VIEW`.
- Nine new PostgreSQL-only rules: `ddl.pg.drop_schema.advisory`, `ddl.pg.drop_schema.cascade.warn`, `ddl.pg.create_sequence.cycle.warn`, `ddl.pg.alter_sequence.restart.warn`, `ddl.pg.alter_sequence.cycle.warn`, `ddl.pg.drop_sequence.advisory`, `ddl.pg.drop_sequence.cascade.warn`, `ddl.pg.drop_materialized_view.advisory`, `ddl.pg.drop_materialized_view.cascade.warn`.
- Service-level, corpus, SDK, CLI, HTTP, and MCP test coverage for all lifecycle operations.

### Non-Goals

- `REFRESH MATERIALIZED VIEW` remains unsupported/deferred.
- Not full PostgreSQL object lifecycle coverage.
- No MySQL/TiDB behavior changes.

## [v0.49.0] - 2026-04-28

### Added

- PostgreSQL advanced `CREATE INDEX` normalization: partial indexes, expression indexes, INCLUDE covering indexes, and non-btree access methods now normalize into coarse index facts instead of returning unsupported.
- `spec.Index` extended with `AccessMethod`, `IncludedColumns`, `HasPredicate`, `HasExpressionKeys`, and `ExpressionCount` fields.
- Service-level tests for all five advanced index forms through `AuditSQL`.
- Corpus fixtures for partial, expression, INCLUDE, and GIN index forms.
- Public surface tests across `pkg/deltascope`, CLI, HTTP handler, and MCP tool.

### Changed

- PostgreSQL extractor widened: removed unsupported guards for partial, expression, INCLUDE, and non-btree `CREATE INDEX` variants.
- Census movement: finding-covered 31→35, unsupported-explicit 22→18, normalized 34→38, corpus-covered 19/56→23/56.

### Non-Goals

- No new rule IDs. Existing `ddl.pg.create_index.concurrently.require` now covers the newly normalized forms.
- No default policy changes.
- No MySQL/TiDB behavior changes.
- No predicate or expression SQL semantic analysis.
- Public response types do not expose full `spec.Index` advanced fields yet.

## [v0.48.0] - 2026-04-28

### Added

- PostgreSQL DDL coverage census: 56 representative PostgreSQL DDL forms characterized through the full audit pipeline (parse → extract → rule evaluation), establishing a reproducible baseline for gap analysis.
- Four new PostgreSQL-only advisory and warning rules covering previously silent-pass gap forms:
  - `ddl.pg.drop_index.advisory` — advisory notice when `DROP INDEX` is executed without `CONCURRENTLY` on PostgreSQL.
  - `ddl.pg.alter.add_column.non_null_no_default.warn` — warns when `ALTER TABLE ADD COLUMN` adds a `NOT NULL` column without a `DEFAULT` value, which can cause table rewrites on large tables.
  - `ddl.pg.alter.add_unique_constraint.concurrent_index.advisory` — advisory notice when `ADD UNIQUE CONSTRAINT` is added without using a concurrent index build strategy.
  - `ddl.pg.alter.drop_constraint.advisory` — advisory notice when `DROP CONSTRAINT` removes a constraint, flagging potential data-integrity implications.
- PostgreSQL extractor now recognizes `CONSTR_NOTNULL` and `CONSTR_DEFAULT` column constraints from pg_query for `ALTER TABLE ADD COLUMN` statements, correctly populating `Column.NotNull` and `Column.HasDefault` in the normalized statement model.
- Census report locked at: total 56, parseable 56, classified 34, normalized 34, finding-covered 31, normalized-silent-pass 3, unsupported-explicit 22, parser-error 0, corpus-covered 19/56.
- Corpus improvements for PostgreSQL DDL coverage validation.

### Changed

- Release-facing docs now position `v0.48.0` as the PostgreSQL DDL Coverage Census & Gap Closure Pack.

### Non-Goals

- No MySQL/TiDB rule behavior change.
- No SQL parser grammar change.
- No public API request type change.
- No release asset naming or npm launcher behavior change.
- Remaining unsupported PG DDL forms (22) remain explicit boundaries, not silently claimed as covered.

## [v0.47.0] - 2026-04-28

### Changed

- Source location fidelity: the audit pipeline now populates `Line` and `Column` on each parsed statement using a progressive source mapper that scans the original SQL buffer forward, matching each `RawSQL` text and counting newlines — replacing the previous statement-index fallback.
- `Finding.Location` is now populated from statement location in the evaluation layer (only when the rule does not already provide a custom location), so all CI renderers automatically pick up source coordinates.
- GitHub Actions output (`--format github-actions`) now emits `file=<path>,line=N,col=N` with the correct statement-start line; when no `--file` path is provided, `file=` is omitted entirely.
- SARIF output (`--format sarif`) now includes `artifactLocation.uri` and `startLine`/`startColumn` for each result; when no `--file` path is provided, `artifactLocation` is omitted.
- GitLab Code Quality output (`--format gitlab-codequality`) now carries the correct statement-start line number in `location.lines.begin`.
- Added `make release-source-location-smoke` gate for cross-renderer source location verification; included in `release-contract-gates`.

### Non-Goals

- No new rule IDs, parser features, or policy changes.
- No domain logic changes beyond statement location propagation.
- No MySQL/TiDB/PostgreSQL audit behavior changes.
- No HTTP/MCP transport protocol changes beyond auto-serialized `location`.
- No release asset naming changes.

## [v0.46.0] - 2026-04-27

### Changed

- Cleaned Homebrew cask install verification output in the release workflow. The `verify-homebrew-cask-install` job now uses conditional cleanup probes instead of tolerated failure fallbacks (`|| true`) so successful runs no longer show misleading Homebrew tap/cask unavailable error annotations.
- Added `release-workflow-hygiene-gates` static gate that enforces conditional cleanup probes, lowercase tap names, and rejects tolerated failure patterns in the release workflow. Included in `release-contract-gates`.
- Documented Homebrew verification hygiene contract for release maintainers.

### Non-Goals

- No SQL audit behavior changes.
- No parser, rule, or policy changes.
- No formatter changes.
- No release asset naming changes.
- No npm launcher behavior changes.

## [v0.45.0] - 2026-04-26

### Added

- `--format gitlab-codequality` CLI flag emits a JSON array matching the GitLab Code Quality report contract so merge-request pipelines can surface SQL audit findings as inline code-quality annotations.
- Contract characterization tests lock the required JSON shape and semantic field guarantees for the GitLab Code Quality renderer.
- Unit tests cover the GitLab Code Quality renderer with zero findings, single finding, and multiple finding cases.
- `make release-gitlab-codequality-smoke` release gate validates the built CLI binary against the GitLab Code Quality output contract.
- `make release-contract-gates` now includes the GitLab Code Quality smoke.
- Recipe: [Using DeltaScope in GitLab CI](docs/recipe/use-deltascope-in-gitlab-ci.md) with step-by-step `.gitlab-ci.yml` setup.
- CLI reference updated with `--format` flag documentation.
- Audit capability matrix updated to list GitLab Code Quality as a supported output format.

### Changed

- Release-facing docs now position `v0.45.0` as the GitLab CI Integration Pack. This does not add new rule IDs, parser features, public API contracts, domain logic changes, MySQL/TiDB or PostgreSQL audit behavior changes, HTTP/MCP/pkg production code changes, or new dependencies.

## [v0.44.0] - 2026-04-25

### Added

- Centralized version surface verification script (`scripts/verify_release_version_surfaces.sh`) checks all release-facing version references in one pass: source constants, npm package, README install pins, release notes H1, release index links, landing DOM hero/release-version/footer, and landing JS i18n strings.
- Binary version smoke target (`make release-local-version-smoke`) builds all three binaries with version ldflags and asserts `deltascope --version`, `deltascope-server --version`, and `deltascope-mcp -version` report the expected tag.
- npm launcher archive and checksum naming contract tests in `packages/deltascope-mcp/test/platform.test.js` verify `resolveArchiveName` and `resolveChecksumsName` follow the `deltascope_<version>_<os>_<arch>` contract.
- Archive verifier (`scripts/verify_release_archive.sh`) now runs dialect hygiene against the extracted binary after the PG audit smoke, catching cross-dialect rule leaks inside packaged release artifacts.
- Release dialect hygiene smoke script (`scripts/verify_release_dialect_hygiene.sh`) verifies default policy dialect isolation: PostgreSQL audits must not emit MySQL/TiDB-only rules or remediation text, and MySQL/TiDB audits must not emit PostgreSQL-only rules.
- Unified release contract gate target (`make release-contract-gates VERSION=vX.Y.Z`) composes version surface gates, local binary version smoke, dialect hygiene gates, npm launcher tests, and goreleaser config check into a single pre-release entry point.
- Release workflow now runs `make release-contract-gates` before GoReleaser publishes, blocking tag pushes that have stale runtime versions, missing release notes, or dialect isolation regressions.

### Changed

- Release-facing docs now position `v0.44.0` as the Release Contract Hardening Pack. This does not add new rule IDs, new parser features, new public API contracts, live schema validation, domain logic changes, MySQL/TiDB or PostgreSQL audit behavior changes, or release artifact structure changes.

## [v0.43.0] - 2026-04-24

### Added

- Default policy now isolates rules by `--dialect`. PostgreSQL audits no longer emit MySQL/TiDB-only rule IDs or remediation text; MySQL/TiDB audits no longer emit PostgreSQL-only rule IDs. This is enforced at the rule `AppliesTo` gate level, not by post-filtering reports.
- PostgreSQL audits now skip MySQL-family rules: `ddl.table.engine.allowlist`, `ddl.table.charset.allowlist`, `ddl.table.row_format.allowlist`, `ddl.table.auto_increment.init_value.require`, `ddl.table.primary_key.unsigned.require`, `ddl.table.primary_key.auto_increment.require`, `ddl.table.primary_key.not_null.require`, `ddl.table.partition.forbid`, `ddl.table.create_as.forbid`, `ddl.table.create_like.forbid`, `ddl.column.charset.allowlist`, `ddl.column.collation.allowlist`, `ddl.column.charset_collation.match.require`, `ddl.alter.change_column.forbid`, `ddl.alter.modify_column.forbid`.
- PostgreSQL `CREATE TABLE` audits no longer suggest MySQL-only `ON UPDATE CURRENT_TIMESTAMP` for the updated-time audit column check.
- MySQL/TiDB audits now exclude all `ddl.pg.*` rules and PostgreSQL-only unprefixed dialect-gated rules.
- Service-level tests assert cross-dialect rule isolation: `TestPostgreSQLDefaultAuditExcludesMySQLFamilyRules`, `TestPostgreSQLDefaultAuditExcludesMySQLRemediationText`, `TestMySQLDefaultAuditExcludesPostgreSQLRules`.
- SQL corpus PostgreSQL probe now includes a negative `exclude:` block listing MySQL-family rules that must never appear.

### Changed

- Release-facing docs now position `v0.43.0` as the Default Policy Dialect Hygiene Pack. This does not add new rule IDs, new parser features, new public API contracts, live schema validation, cross-database tracking, or MySQL/TiDB behavior changes beyond the dialect isolation fixes.

## [v0.42.0] - 2026-04-22

### Added

- v0.42.0 adds PostgreSQL NOT VALID constraint validation pairing. DeltaScope now warns when a named `ALTER TABLE ... ADD CONSTRAINT ... NOT VALID` CHECK or FOREIGN KEY constraint is not followed by a later matching `ALTER TABLE ... VALIDATE CONSTRAINT ...` statement in the same audited SQL batch.
- New PostgreSQL-only GlobalRule: `ddl.pg.alter.not_valid_constraint.validate.require`.
- The rule defaults to `warning`, applies to named CHECK / FOREIGN KEY `NOT VALID` constraints, and suppresses when a later matching validation uses the same schema + table + constraint name.
- CLI, HTTP, MCP, and `pkg/deltascope` surfaces expose the result as a global finding.
- SQL corpus coverage and Docker-backed PostgreSQL e2e now lock the release-facing contract for this pairing rule.

### Changed

- Release-facing docs now position `v0.42.0` as the PostgreSQL NOT VALID Constraint Validation Pairing Pack. This does not add first-time `VALIDATE CONSTRAINT` support, live database validation-state lookup, cross-file or cross-deployment tracking, unnamed-constraint matching, CHECK expression validation, FK referenced-table correctness validation, MySQL/TiDB behavior changes, or a new public API contract.

## [v0.41.0] - 2026-04-22

### Added

- PostgreSQL `ALTER TABLE ... ADD CONSTRAINT CHECK` statement-local check constraint facts: named CHECK forms now preserve constraint name and check expression through the PostgreSQL extractor. The `DDL.Constraints` projection allows existing check naming rules and the PostgreSQL `NOT VALID` advisory to trigger on ALTER TABLE CHECK additions.
- Existing check naming rules now apply to PostgreSQL `ALTER TABLE ... ADD CONSTRAINT ... CHECK` statements:
  - `ddl.constraint.check.name.prefix.require` — flags explicitly named check constraints that do not start with the required prefix (when configured).
  - `ddl.constraint.check.name.suffix.require` — flags explicitly named check constraints that do not end with the required suffix (when configured).
  - `ddl.constraint.check.name.contains.require` — flags explicitly named check constraints that do not contain any configured token (when configured).
- `ddl.pg.alter.add_check.not_valid.require` now fires on `ALTER TABLE ... ADD CONSTRAINT ... CHECK` statements by default when `--dialect postgresql` is set.
- CLI, HTTP, MCP, and `pkg/deltascope` surfaces produce explicit `rule_id` findings for ALTER TABLE CHECK additions.
- Corpus expected outcomes and service-level tests lock PostgreSQL ALTER TABLE CHECK fact extraction and rule coverage.
- Docker-backed PostgreSQL CLI e2e covers `ddl.pg.alter.add_check.not_valid.require` and `ddl.constraint.check.name.prefix.require` for statement-local `ALTER TABLE ... ADD CONSTRAINT ... CHECK` audit.

### Changed

- Release-facing docs now position `v0.41.0` as the PostgreSQL ALTER TABLE CHECK Fact Support Pack. It does not add live schema CHECK existence validation, new rule IDs, `NOT VALID` validation enforcement, deferred constraint support, or MySQL/TiDB behavior changes.

## [v0.40.0] - 2026-04-21

### Added

- PostgreSQL `ALTER TABLE ... ADD CONSTRAINT FOREIGN KEY` statement-local FK facts: named and unnamed FK forms now preserve local columns, referenced table, referenced columns, and referenced schema (for schema-qualified references) through the PostgreSQL extractor. The `DDL.Constraints` projection allows existing FK rules to trigger on ALTER TABLE FK additions.
- Existing FK rules now apply to PostgreSQL `ALTER TABLE ... ADD CONSTRAINT ... FOREIGN KEY` statements:
  - `ddl.table.foreign_key.forbid` — flags foreign key constraints as forbidden under the default policy.
  - `ddl.pg.table.foreign_key.cross_schema.advisory` — emits a notice when the owning table schema and referenced schema are both explicit and different.
- CLI, HTTP, MCP, and `pkg/deltascope` surfaces produce explicit `rule_id` findings for ALTER TABLE FK additions.
- Corpus expected outcomes and service-level tests lock PostgreSQL ALTER TABLE FK fact extraction and rule coverage.
- Docker-backed PostgreSQL CLI e2e covers `ddl.table.foreign_key.forbid` for statement-local `ALTER TABLE ... ADD CONSTRAINT ... FOREIGN KEY` audit.

### Changed

- Release-facing docs now position `v0.40.0` as the PostgreSQL ALTER TABLE Foreign Key Fact Support Pack. It does not add live schema FK existence validation, new FK rule IDs, deferrable constraint support, MATCH FULL policy expansion, full constraint/index parity, or MySQL/TiDB behavior changes.

## [v0.39.0] - 2026-04-19

### Added

- PostgreSQL `ALTER TABLE ... ADD CONSTRAINT` primary-key and unique constraint facts: inline (`ADD PRIMARY KEY`), named (`ADD CONSTRAINT users_pkey PRIMARY KEY`), and unnamed (`ADD UNIQUE`) forms now preserve statement-local constraint metadata through the PostgreSQL extractor and rule projection helpers.
- Existing primary-key rules now apply to PostgreSQL `ALTER TABLE ... ADD CONSTRAINT ... PRIMARY KEY` statements:
  - `ddl.table.primary_key.bigint.require` — flags PostgreSQL primary-key columns that are not BIGINT.
  - `ddl.table.primary_key.columns.max_count` — flags PostgreSQL composite primary keys that exceed the configured column limit.
- Existing unique/index prefix rule now applies to PostgreSQL `ALTER TABLE ... ADD CONSTRAINT ... UNIQUE` statements:
  - `ddl.alter.add_index.unique.prefix.require` — flags unique constraint names that do not start with the required prefix.
- CLI, HTTP, MCP, and `pkg/deltascope` surfaces produce explicit `rule_id` findings.
- Corpus expected outcomes and service-level tests lock PostgreSQL constraint rule coverage.
- Docker-backed PostgreSQL CLI e2e covers `ddl.alter.add_index.unique.prefix.require` for statement-local `ALTER TABLE ... ADD CONSTRAINT ... UNIQUE` audit.

### Changed

- Release-facing docs now position `v0.39.0` as the PostgreSQL ALTER TABLE Constraint Fact Support Pack. It does not add full `ALTER TABLE ADD CONSTRAINT` support, foreign key/check constraint rule support, deferrable constraint support, constraint validation lifecycle support, partial/expression index support, operator class support, live schema reconstruction, new rule IDs, or MySQL/TiDB behavior changes.

## [v0.38.0] - 2026-04-18

### Added

- PostgreSQL standalone `CREATE INDEX` and `CREATE UNIQUE INDEX` statements now trigger existing generic index rules for approved btree forms. The following rules now produce findings for PostgreSQL `CREATE INDEX` statements:
  - `ddl.index.secondary.prefix.require` — flags secondary index names that do not start with the required prefix.
  - `ddl.index.unique.prefix.require` — flags unique index names that do not start with the required prefix.
  - `ddl.index.columns.max_count` — flags indexes that exceed the configured column limit.
- Docker-backed PostgreSQL CLI e2e covers `ddl.index.unique.prefix.require` for statement-local `CREATE UNIQUE INDEX` audit.

### Changed

- Release-facing docs now position `v0.38.0` as extending PostgreSQL unique/index audit coverage for statement-local unique constraints and simple btree `CREATE INDEX` forms. It does not add full PostgreSQL index support, partial index support, expression index support, INCLUDE support, operator class support, non-btree access method support, NULLS NOT DISTINCT support, live schema index introspection, new index rule IDs, or MySQL/TiDB behavior changes.

## [v0.37.0] - 2026-04-18

### Added

- PostgreSQL `CREATE TABLE` primary-key facts: inline (`id bigint PRIMARY KEY`), table-level (`PRIMARY KEY (id)`), named (`CONSTRAINT users_pkey PRIMARY KEY (id)`), and composite primary-key declarations now populate DeltaScope's normalized `DDL.PrimaryKey` contract through the PostgreSQL extractor.
- Primary-key columns are treated as effectively `NOT NULL` for PostgreSQL `CREATE TABLE` statements, consistent with PostgreSQL's primary-key semantics.
- Existing primary-key rules now apply to PostgreSQL `CREATE TABLE` statements:
  - `ddl.table.primary_key.bigint.require` — flags PostgreSQL primary-key columns that are not BIGINT.
  - `ddl.table.primary_key.columns.max_count` — flags PostgreSQL composite primary keys that exceed the configured column limit.
- CLI, HTTP, MCP, and `pkg/deltascope` surfaces produce explicit `rule_id` findings for PostgreSQL primary-key rule violations.
- Corpus expected outcomes and service-level tests lock PostgreSQL primary-key fact extraction and rule coverage with precise assertions.

### Changed

- Release-facing docs now position `v0.37.0` as the **PostgreSQL Primary Key Fact Support Pack**. It does not add full PostgreSQL index support, `ALTER TABLE ADD PRIMARY KEY` support, live schema primary-key introspection, new primary-key rule IDs, or full PostgreSQL constraint/index parity.

## [v0.36.1] - 2026-04-18

### Added

- SQL corpus supported-rule coverage now has a reusable inventory report via `make sql-corpus-report`, showing rule counts, supported `rule_id × dialect` targets, covered targets, fixture counts by dialect, and deferred surfaces.
- SQL corpus fixtures were expanded across MySQL, TiDB, and PostgreSQL to cover every currently supported rule/dialect surface under the repository coverage contract.

### Changed

- Release test gates now run `make sql-corpus-gates`, so supported-rule corpus coverage drift blocks release validation.
- Testing docs now distinguish SQL corpus supported-rule coverage from Go line coverage and from theoretical “all policy keys on all dialects” coverage.

## [v0.36.0] - 2026-04-17

### Added

- Three new PostgreSQL-only forbid rules cover the generated/identity state-transition forms that became supported in v0.35.0:
  - `ddl.alter.drop_expression.forbid` — flags `ALTER TABLE ... ALTER COLUMN ... DROP EXPRESSION` on PostgreSQL.
  - `ddl.alter.set_generated.forbid` — flags `ALTER TABLE ... ALTER COLUMN ... SET GENERATED ...` on PostgreSQL.
  - `ddl.alter.drop_identity.forbid` — flags `ALTER TABLE ... ALTER COLUMN ... DROP IDENTITY` on PostgreSQL.
- CLI, HTTP, MCP, and `pkg/deltascope` surfaces now produce explicit `rule_id` findings for these state-transition forms instead of passing silently.

### Changed

- Release-facing docs now position `v0.36.0` as the **PostgreSQL Generated/Identity Rule Coverage Pack**. It does not add parser support widening, spec contract widening, generated expression evaluation, complete PostgreSQL sequence semantics, or MySQL/TiDB changes.

## [v0.35.0] - 2026-04-16

### Added

- PostgreSQL generated/identity state-transition forms are now processed through the normal supported audit path instead of returning `ErrUnsupportedStatement`. The following forms are supported:
  - `ALTER TABLE ... ALTER COLUMN ... DROP EXPRESSION`
  - `ALTER TABLE ... ALTER COLUMN ... SET GENERATED ALWAYS`
  - `ALTER TABLE ... ALTER COLUMN ... SET GENERATED BY DEFAULT`
  - `ALTER TABLE ... ALTER COLUMN ... DROP IDENTITY`

### Changed

- Supported state-transition forms now normalize to explicit existing alter actions: `drop_expression`, `set_generated` (`generated_when = "a"` for ALWAYS, `"d"` for BY DEFAULT), and `drop_identity`.
- CLI, HTTP, MCP, and `pkg/deltascope` surfaces now use the normal supported result path for these forms instead of unsupported outcomes.
- Release-facing docs now position `v0.35.0` as the **PostgreSQL Generated/Identity State-Transition Pack**. It does not add full generated/identity lifecycle support, generated expression evaluation, complete PostgreSQL sequence semantics, new rule IDs, or MySQL/TiDB changes.

## [v0.34.0] - 2026-04-15

### Added

- PostgreSQL narrow generated/identity definition forms are now processed through the normal supported audit path instead of returning `ErrUnsupportedStatement`. The following forms are supported:
  - `CREATE TABLE ... GENERATED ALWAYS AS (...) STORED` — generated stored column definitions.
  - `CREATE TABLE ... GENERATED {ALWAYS|BY DEFAULT} AS IDENTITY` — identity column definitions, including identity with sequence options (`START WITH`, `INCREMENT BY`, `CACHE`, `CYCLE`).
  - `ALTER TABLE ... ADD COLUMN ... GENERATED ALWAYS AS (...) STORED` — generated stored add-column definitions.
  - `ALTER TABLE ... ADD COLUMN ... GENERATED {ALWAYS|BY DEFAULT} AS IDENTITY` — identity add-column definitions.
- CLI, HTTP, MCP, and `pkg/deltascope` surface tests switched from unsupported contract assertions to supported result contract assertions for the narrow generated/identity forms.

### Changed

- PostgreSQL extractor no longer rejects narrow generated/identity definition forms at the unsupported boundary. These forms enter the normal normalization and rule evaluation path.
- Corpus expected outcomes and service-level tests updated to assert supported results (normal statement output, no unsupported detail) for narrow generated/identity forms.
- Shared facts preserved from `v0.33.0` continue flowing through the supported path: `generated_when`, `is_identity`, `identity_options`.
- Release-facing docs now position `v0.34.0` as the **PostgreSQL Generated/Identity Narrow Support Pack** — a narrow support widening release. It does not add full generated-column support, full identity-column support, generated expression evaluation, complete PostgreSQL sequence semantics, state-transition support, or new rules. Unsupported feature names for state-transition forms remain: `generated_column` (`DROP EXPRESSION`), `generated_as_identity` (`SET GENERATED`, `DROP IDENTITY`).

## [v0.33.0] - 2026-04-15

### Added

- `GeneratedWhen` (string, `omitempty`) and `IsIdentity` (bool, `omitempty`) fields on `spec.Column` preserve narrow generated/identity column facts for PostgreSQL `CREATE TABLE` and `ALTER TABLE ADD COLUMN` paths. `GeneratedWhen` encodes `"a"` (ALWAYS) or `"d"` (BY DEFAULT); `IsIdentity` is `true` for identity columns. `GeneratedExpression` is deferred — no expression text is preserved.
- `IdentityOptions map[string]any` on `spec.Column` carries finite structured identity sequence option facts (`start`, `increment`, `minvalue`, `maxvalue`, `cache`, `cycle`). This is not complete PostgreSQL sequence semantics — only options present in the SQL text are preserved.
- `Metadata map[string]any` on `spec.UnsupportedDetail` surfaces structured metadata for unsupported generated/identity outcomes. Keys: `column` (string), `generated_when` (string), `is_identity` (bool for identity cases), `identity_options` (object for options cases).
- PostgreSQL extractor populates the new shared contract fields and unsupported metadata for both `CREATE TABLE` and `ALTER TABLE ADD COLUMN` generated/identity paths.
- Corpus cases updated to assert metadata on unsupported generated/identity boundary outcomes.
- Service-level tests lock the new metadata contract with precise assertions.
- Surface parity tests across CLI, HTTP, MCP, and `pkg/deltascope` verify unsupported metadata flows through each transport. MCP surface limitation documented: metadata is not directly surfaced in MCP tool error responses.

### Changed

- Release-facing docs now position `v0.33.0` as the **PostgreSQL Generated/Identity Fact Preservation + Unsupported Metadata Surfacing Pack** — a fact preservation and metadata widening release. It does not add generated-column support, identity-column support, expression evaluation, rule behavior changes, or ALTER TABLE state-transition support. Unsupported feature names remain unchanged: `generated_column`, `generated_as_identity`.

## [v0.32.0] - 2026-04-14

### Added

- Characterization tests in `internal/infrastructure/parser/postgresql/parser_test.go` documenting stable AST facts about PostgreSQL generated and identity columns: `GeneratedWhen` encoding (`"a"` / `"d"`), `CONSTR_IDENTITY` / `CONSTR_GENERATED` constraint types, identity sequence option shape (`DefElem` nodes with `defname` and `Integer` arg), and AST shape consistency between `CREATE TABLE` and `ALTER TABLE ADD COLUMN`.
- Decision report at `docs/plans/reports/2026-04-14-v0.32.0-pg-boundary-support-readiness-report.md` documenting the complete unsupported boundary inventory, AST fact coverage, shared contract decision, and v0.33.0 recommendation.

### Changed

- Release-facing docs now position `v0.32.0` as the **PostgreSQL Boundary Support-Readiness Gate** — a decision milestone, not a feature release. No new PostgreSQL support behavior, rule IDs, CLI flags, or public API fields were added.

## [v0.31.0] - 2026-04-14

### Changed

- PostgreSQL `ALTER TABLE ... ALTER COLUMN ... DROP EXPRESSION` now returns the explicit unsupported feature `generated_column` instead of a generic AST-subtype unsupported boundary.
- PostgreSQL `ALTER TABLE ... ALTER COLUMN ... SET GENERATED ...` now returns the explicit unsupported feature `generated_as_identity` instead of a generic AST-subtype unsupported boundary.
- PostgreSQL `ALTER TABLE ... ALTER COLUMN ... DROP IDENTITY` now returns the explicit unsupported feature `generated_as_identity` instead of a generic AST-subtype unsupported boundary.
- These mappings align the adjacent PostgreSQL generated/identity alteration forms with the same stable unsupported feature names used by `v0.26.0` (`CREATE TABLE`) and `v0.30.0` (`ADD COLUMN`).
- Corpus, service, and CLI / HTTP / MCP / `pkg/deltascope` parity tests lock these boundary outcomes with precise assertions.
- Release-facing docs now position `v0.31.0` as the **PostgreSQL ALTER TABLE GENERATED Follow-up Pack** — boundary tightening only, not generated-column support, identity-column support, or complete PostgreSQL `ALTER TABLE` support.

## [v0.30.0] - 2026-04-14

### Added

- PostgreSQL `ALTER TABLE ... ADD COLUMN` generated/identity boundary coverage is now locked as an explicit unsupported contract across corpus, service checks, and surface parity for CLI, HTTP, MCP, and `pkg/deltascope`.

### Changed

- PostgreSQL `ALTER TABLE ... ADD COLUMN ... GENERATED ALWAYS AS (...) STORED` now returns the explicit unsupported feature `generated_column` instead of looking like an ordinary supported add-column path.
- PostgreSQL `ALTER TABLE ... ADD COLUMN ... GENERATED ALWAYS AS IDENTITY` now returns the explicit unsupported feature `generated_as_identity` instead of looking like an ordinary supported add-column path.
- Adjacent PostgreSQL `ALTER TABLE` generated/identity alteration forms such as `DROP EXPRESSION`, `SET GENERATED`, and `DROP IDENTITY` remain generic unsupported boundaries.
- Release-facing docs now position `v0.30.0` as the **PostgreSQL ALTER TABLE GENERATED Boundary Pack** — a boundary-tightening release, not generated-column support, identity-column support, or broad PostgreSQL `ALTER TABLE` support.

## [v0.29.0] - 2026-04-14

### Added

- PostgreSQL now ships the notice-level advisory rule `ddl.pg.table.foreign_key.cross_schema.advisory` for explicit cross-schema foreign keys when the owning table schema and referenced schema are both explicit and different.
- Cross-schema advisory findings can expose `table_schema`, `referenced_schema`, `referenced_table`, and `referenced_columns` in outward finding metadata. `referenced_table` remains normalized as `"users"`, never `"auth.users"`.

### Changed

- The additive FK finding metadata surface introduced in `v0.28.0` now participates in a narrow PostgreSQL policy decision. Same-schema foreign keys and bare references such as `REFERENCES users(id)` remain unchanged.
- Bare references remain schema unknown. DeltaScope does not infer `public` and does not model PostgreSQL `search_path` semantics.
- Release-facing docs now position `v0.29.0` as the **Schema-Aware FK Policy Pack** — the first schema-aware FK policy step, not full PostgreSQL foreign key support and not a cross-schema validation engine.

## [v0.28.0] - 2026-04-13

### Added

- FK forbid finding metadata now exposes referenced-object fields (`referenced_schema`, `referenced_table`, `referenced_columns`) for PostgreSQL foreign key constraints. These fields were already present in the shared semantic contract (`spec.Constraint`) from `v0.27.0`; `v0.28.0` widens the outward finding metadata contract so CLI, HTTP, MCP, and `pkg/deltascope` users can see them directly.
- `referenced_table` is never concatenated with `referenced_schema` (e.g., never `"public.users"`). The two fields are always separate and normalized.

### Changed

- Release-facing docs now position `v0.28.0` as the **Referenced-Object Metadata Surface Pack** — an additive metadata widening of the FK forbid finding contract. It does not add new rules, new CLI flags, new public API contracts, or schema-aware FK policy support. Parser/extractor semantics are unchanged from `v0.27.0`.

## [v0.27.0] - 2026-04-13

### Added

- Additive `ReferencedSchema` field on `spec.Constraint`: PostgreSQL schema-qualified `REFERENCES public.users(id)` now preserves the referenced-object schema (`"public"`) as a parser-owned shared contract fact alongside the existing `ReferencedTable` (`"users"`). The normalized representation is always `ReferencedSchema` + `ReferencedTable` — `ReferencedTable` is never concatenated into `"public.users"`.
- PostgreSQL extractor preserves schema-qualified reference facts for both named `FOREIGN KEY ... REFERENCES schema.table` and inline `REFERENCES schema.table` forms.
- Corpus cases updated to lock schema-qualified reference semantics with precise `.expected.yaml` assertions (`ReferencedSchema = "public"`, `ReferencedTable = "users"`).
- Service-level semantic tests assert schema-qualified reference facts are preserved through the audit pipeline.

### Changed

- Release-facing docs now position `v0.27.0` as the **Schema-Qualified Reference Semantics Pack** — an additive semantic preservation of PostgreSQL schema-qualified referenced-object facts in the shared contract. It does not add new rules, new CLI flags, or new public API contracts. Current public finding metadata (CLI, HTTP, MCP, `pkg/deltascope`) remains unchanged; the shared semantic contract is richer underneath.

## [v0.26.0] - 2026-04-12

### Added

- PostgreSQL `CREATE TABLE` unsupported boundary tightening: identity columns (`GENERATED ... AS IDENTITY`), generated stored columns (`GENERATED ALWAYS AS ... STORED`), exclusion constraints (`EXCLUDE USING`), and partitioned tables (`PARTITION BY`) are now explicitly marked as unsupported at the extractor level instead of being silently accepted or partially handled.
- PostgreSQL corpus cases updated to lock the four unsupported boundaries (`generated_as_identity`, `generated_column`, `exclusion_constraint`, `partitioning`) with precise expected-outcome assertions.
- Surface parity tests across CLI, HTTP, MCP, and public Go API (`pkg/deltascope`) verify that each boundary is exposed through the correct unsupported contract on every transport.

### Changed

- Release-facing docs now position `v0.26.0` as the **PostgreSQL CREATE TABLE Unsupported Boundary Pack** — an extractor-level boundary tightening backed by corpus and surface tests. It does not add new rules, new CLI flags, or new public API contracts. It does not represent full PostgreSQL `CREATE TABLE` support.

## [v0.25.0] - 2026-04-12

### Added

- Dialect-wide SQL corpus harness (`testdata/sql-corpus/`) with MySQL, TiDB, and PostgreSQL baseline cases covering supported, unsupported, finding-producing, clean, and boundary categories.
- Two-layer corpus assertions: report-level audit checks (unsupported count, statement kind, findings) plus semantic parse/extract assertions (operation name, constraint facts) driven by a single `.expected.yaml` file per case.
- MySQL baseline corpus: DDL supported (primary key), DDL findings (foreign key forbid), DML findings (UPDATE/DELETE without WHERE), DML clean (UPDATE/DELETE with WHERE).
- TiDB baseline corpus: DDL supported (primary key), DML findings (UPDATE/DELETE without WHERE), DML clean (UPDATE with WHERE).
- PostgreSQL baseline corpus: DDL supported (named CHECK, UNIQUE, FOREIGN KEY, inline REFERENCES), DDL findings (inline REFERENCES forbid), DDL unsupported (CREATE OR REPLACE VIEW), DDL boundary (GENERATED ... AS IDENTITY, PARTITION BY).
- `GENERATED ... AS IDENTITY` is recorded as a current boundary finding in the corpus — it is not fixed in this release. Follow-up: `PostgreSQL CREATE TABLE Unsupported Boundary Pack`.

### Changed

- Release-facing docs now position `v0.25.0` as the **SQL Corpus & Boundary Confidence Pack** — a durable corpus and table-driven audit harness that answers which representative SQL statements have been run through DeltaScope and what outcomes are expected. It does not add new rules, new CLI flags, or new public API contracts.

## [v0.24.0] - 2026-04-11

### Added

- PostgreSQL `CREATE TABLE` foreign-key semantics now preserve parser-owned referenced table and referenced columns in the shared `spec.Constraint` model. Named `FOREIGN KEY` and inline `REFERENCES` forms both carry `ReferencedTable` and `ReferencedColumns` as shared contract facts. These are parser-owned structural facts, not metadata-dependent truth claims.
- Service-level and surface parity tests tightened across CLI, HTTP, MCP, and public Go API (`pkg/deltascope`) for richer PostgreSQL foreign-key shapes. Inline `REFERENCES` now asserts `ddl.table.foreign_key.forbid` fires under the default policy.
- Unsupported boundary tests added for `CREATE OR REPLACE VIEW` and `PARTITION BY` on the service layer and public Go API, confirming that adjacent unsupported forms remain explicitly outside the supported surface.

### Changed

- Release-facing docs now position `v0.24.0` as a PostgreSQL `CREATE TABLE` semantics pack — a semantic deepening of `v0.23.0` — not a new rule pack and not full PostgreSQL DDL support.

## [v0.23.0] - 2026-04-11

### Added

- PostgreSQL `CREATE TABLE` coverage expanded for common constraint shapes: table-level named `CHECK`, column-level inline `CHECK`, table-level named `UNIQUE`, column-level inline `UNIQUE`, table-level named `FOREIGN KEY`, and column-level inline `REFERENCES`.
- Shared rule reuse for the newly normalized PostgreSQL create-table structures. Named `CHECK`, `UNIQUE`, and `FOREIGN KEY` constraints can flow into existing structured naming governance where the policy makes those rule families applicable; inline `UNIQUE` contributes index facts; inline `REFERENCES` is exposed as parser-owned shared facts without adding metadata-only semantics.
- CLI, HTTP, MCP, and public Go API (`pkg/deltascope`) parity confirmed for the expanded PostgreSQL `CREATE TABLE` coverage.

### Changed

- Release-facing docs now position `v0.23.0` as a PostgreSQL `CREATE TABLE` coverage pack, not full PostgreSQL DDL support and not a new-rule release.
- Reference docs and recipes now distinguish supported, auditable, rule-mapped, and metadata-dependent behavior for the richer PostgreSQL create-table shapes.

## [v0.22.0] - 2026-04-11

### Added

- Canonical PostgreSQL confidence entrypoints for local and CI verification: `pg-unit-test-gates`, `pg-e2e-gates`, and `pg-confidence-gates`.
- Reusable release confidence gates: `release-surface-gates` for package/release contract checks and `release-version-surface-gates` for versioned docs/install surface checks.
- Bilingual release notes and release-facing docs aligned around the `v0.22.0` **E2E & Release Confidence Pack** milestone.

### Changed

- DeltaScope now documents confidence closure around the existing PostgreSQL product surfaces instead of introducing new PostgreSQL SQL semantics in this release.
- README, landing page, CLI/reference docs, CI recipes, and scripts guide now point to the `v0.22.0` release line and the canonical confidence targets used to verify transport and release-surface alignment.

## [v0.21.0] - 2026-04-11

### Added

- PostgreSQL DDL coverage expanded for common migration follow-up statements. The following `ALTER TABLE` forms are now normalized into the shared audit pipeline instead of returning capability-boundary errors:
  - `ALTER COLUMN ... SET DEFAULT` — column default assignment during phased rollout
  - `ALTER COLUMN ... DROP DEFAULT` — column default removal
  - `ALTER COLUMN ... SET NOT NULL` — nullability enforcement after backfill
  - `ALTER COLUMN ... DROP NOT NULL` — nullability relaxation
  - `VALIDATE CONSTRAINT` — constraint validation step in the recommended `NOT VALID` → `VALIDATE CONSTRAINT` pattern
  - `DROP CONSTRAINT` — constraint removal, including primary-key mapping via metadata-aware rules

- Shared rule and metadata semantics now apply to the newly normalized PostgreSQL DDL actions. `DROP CONSTRAINT` on a primary key reuses existing `ddl.alter.drop_primary_key` rules when metadata is available.

- CLI, HTTP, MCP, and public API (`pkg/deltascope`) parity confirmed for all newly supported PostgreSQL DDL forms.

### Changed

- PostgreSQL migration review workflows that previously hit capability-boundary errors for `SET DEFAULT`, `DROP DEFAULT`, `SET NOT NULL`, `DROP NOT NULL`, `VALIDATE CONSTRAINT`, or `DROP CONSTRAINT` now return normal audit results.

## [v0.20.0] - 2026-04-10

### Added

- PostgreSQL syntax heuristic notice (`dialect.postgresql.syntax.detected.notice`): when auditing on the MySQL/TiDB path, DeltaScope now detects common PostgreSQL-specific syntax tokens (`RETURNING`, `ON CONFLICT`, `::` casts, `ALTER COLUMN TYPE USING`, `GENERATED AS IDENTITY`) and emits a notice-level global finding suggesting `--dialect postgresql`. DeltaScope does not auto-switch dialect — the notice is advisory only.
- Explicit PostgreSQL capability-boundary errors: unsupported-build surfaces now return typed `PostgreSQLCapabilityBoundaryError` values instead of heuristic string matching, making it easier for tooling and CI to distinguish real parse failures from capability limits.
- CLI output trust signals: markdown output now includes a `## Audit Context` section with mode, dialect source, and an explicit trust note when a PostgreSQL syntax notice is present. JSON output includes a top-level `context` object. Quiet output appends a `[context]` line.
- Rule summary and skipped-rules visibility in CLI output formats: `rule_summary` (loaded, applicable, skipped counts) appears in JSON; `## Rule Summary` and `## Skipped Rules` sections appear in markdown; `[summary]` line appears in quiet output. GitHub Actions and SARIF output continue to emit findings only.

### Changed

- PostgreSQL migration-safety rule suggestions now provide step-by-step migration guidance instead of generic tips:
  - `ddl.pg.create_index.concurrently.require`: mentions that `CONCURRENTLY` cannot run inside a transaction.
  - `ddl.pg.alter.add_column.non_null_default.rewrite.warn`: recommends a 4-step safe path (nullable → backfill → default → not null).
  - `ddl.pg.alter.add_check.not_valid.require`: describes the 2-step `NOT VALID` → `VALIDATE CONSTRAINT` approach with lock-level detail.
  - `ddl.pg.alter.set_data_type.rewrite.warn`: recommends phased migration with shadow column strategy for large tables.

### Fixed

- PostgreSQL syntax heuristic no longer fires for tokens inside string literals, quoted identifiers, backtick identifiers, line comments, or block comments.
- Metadata request merge: mixed top-level `Schema`/`MetadataProvider` fields with legacy `Metadata` struct fields no longer drop schema or provider context.

## [v0.19.0] - 2026-04-09

### Added

- PostgreSQL migration-safety rule pack (4 rules, default level `warning`):
  - `ddl.pg.create_index.concurrently.require` — flags `CREATE INDEX` without `CONCURRENTLY` on PostgreSQL
  - `ddl.pg.alter.add_column.non_null_default.rewrite.warn` — warns when `ALTER TABLE … ADD COLUMN … NOT NULL DEFAULT …` may trigger a full table rewrite
  - `ddl.pg.alter.add_check.not_valid.require` — flags `ALTER TABLE … ADD CHECK (…)` without `NOT VALID` on large tables
  - `ddl.pg.alter.set_data_type.rewrite.warn` — warns when `ALTER TABLE … ALTER COLUMN … TYPE …` may require a full table rewrite
- `--format github-actions` output for inline CI annotations (`::error`, `::warning`, `::notice`) with proper workflow-command escaping
- `--format sarif` output producing valid SARIF 2.1.0 JSON for GitHub Code Scanning and other SARIF consumers
- `rule_summary` field in JSON output and `## Rule Summary` / `## Skipped Rules` sections in Markdown output showing loaded, applicable, and skipped rule counts

### Changed

- Documentation updated across all reference, recipe, and landing pages to reflect the v0.19.0 PostgreSQL migration-safety pack and CI output formats

## [v0.18.0] - 2026-04-09

### Added

- PostgreSQL metadata-aware audit: DeltaScope can now connect to a live PostgreSQL 12+ instance to retrieve schema metadata, run `EXPLAIN` for DML impact estimation, and evaluate rules against real database state
- Transport parity: CLI, HTTP (`POST /v1/audit`), and MCP (`audit_sql`) all support PostgreSQL metadata-aware audit with `--dialect postgresql` or `"dialect": "postgresql"`
- PostgreSQL schema resolution: qualified table names in SQL are parsed automatically; unqualified names resolve via `--schema` flag or unique-match inference across accessible schemas
- PostgreSQL DML impact estimation via `EXPLAIN` (read-only, conservative, never executes the DML)
- PostgreSQL metadata-aware rules: `ddl.alter.drop_primary_key.forbid` detects `DROP CONSTRAINT` on primary keys via `pg_constraint` mapping, `ddl.alter.rename_column.exists.require` verifies column existence, `ddl.alter.rename_index.forbid` flags index renames, `ddl.alter.drop_column.exists.require` verifies column existence, `ddl.table.exists.create.forbid` checks table existence
- PostgreSQL metadata provider (`internal/infrastructure/metadata/postgresql`) with `pgx/v5` driver for schema introspection and planner queries
- Full E2E test suites for all three transports against PostgreSQL 17: CLI (9 shell-based cases), HTTP (9 Go subtests), MCP (9 Go subtests)

### Changed

- Documentation updated across all reference, concept, recipe, and landing pages to reflect PostgreSQL metadata-aware support
- `pgx/v5` promoted from indirect to direct dependency in `go.mod`

## [v0.13.1] - 2026-04-02

### Fixed

- Landing page inline i18n script no longer embeds unescaped SQL single quotes in the DDL / CI examples, which previously caused a browser-side `Unexpected string` syntax error and prevented the page JavaScript from loading

### Changed

- Release-facing docs, examples, landing content, and source-build defaults now align with `v0.13.1`

## [v0.13.0] - 2026-04-02

### Added

- HTTP `POST /v1/audit` now supports metadata-aware execution through direct `connection` inputs, including additive `context` fields that report resolved dialect, schema, and metadata source
- Shared `internal/interfaces/metadata` helpers now normalize direct connection validation and password resolution across HTTP and MCP adapters
- Docker-backed HTTP metadata e2e coverage now exercises real `deltascope-server` binaries against MySQL and TiDB fixtures

### Changed

- Release-facing docs, examples, landing content, and source-build defaults now align with `v0.13.0`
- HTTP metadata-aware requests snapshot the policy config per request so preparation and final audit read the same policy bytes
- HTTP API docs now describe direct credential lookup failures (`password_env`, `password_file`) under the stable `connection_invalid` error contract

## [v0.12.0] - 2026-04-02

### Added

- Structured naming governance for `CREATE TABLE` table, column, index, and explicitly named constraint objects, with configurable `prefix`, `suffix`, and `contains` requirements
- Reusable naming configuration helpers and rule primitives used across table, column, index, and constraint governance checks
- Application-layer and CLI end-to-end coverage for config-driven naming governance findings

### Changed

- Release-facing docs, examples, and landing content now present naming governance as the latest shipped milestone and align versioned install snippets to `v0.12.0`
- Foreign key naming governance stays policy-aware: it only applies when foreign keys are allowed by policy and remains suppressed by the shipped default `ddl.table.foreign_key.forbid` baseline

## [v0.11.1] - 2026-03-31

### Changed

- macOS install guidance now leads with Homebrew across README and landing page surfaces, while the portable installer remains the fallback for Linux and other environments
- `install.sh` now defaults to installing only `deltascope`, prompts interactive users to choose binaries and install directory, prints an install summary, and warns before requiring `sudo`
- `deltascope audit` now prints an interactive stdin hint before waiting for pasted SQL from a terminal session
- `deltascope rules list` and `deltascope rules search` now render shipped rules as an aligned ASCII table for easier scanning in terminals and screenshots
- CLI and reference docs now match the current install and rule-list output contracts in English and Chinese

## [v0.11.0] - 2026-03-30

### Added

- GitHub Actions composite action (`action.yml`) for one-step SQL audit in CI — supports `fail-on` severity threshold, optional PR comment, and auto-downloads the correct release binary
- `docs/examples/github-actions.yml` — caller workflow example for GitHub Actions
- `docs/examples/gitlab-ci.yml` — standalone GitLab CI job example
- `/readyz` endpoint alongside existing `/healthz`; both bypass auth and rate-limit
- Structured JSON access log lines from `accessLogMiddleware` (replaces plain-text format)
- SIGTERM/SIGINT graceful shutdown with 15-second drain timeout in `deltascope-server`

### Changed

- Auth and rate-limit allow-paths defaults now include `/readyz` in addition to `/healthz` and `/metrics`

## [v0.10.0] - 2026-03-29

### Added

- Gin-based HTTP adapter with middleware guardrails (request ID, panic recovery, timeout context, structured access logs)
- Optional `X-API-Key` authentication for HTTP audit endpoints with `401 auth_required` and `403 auth_invalid`
- Optional rate limiting with `429 rate_limited` support (`api-key` and `ip` strategies)
- Prometheus `/metrics` endpoint with HTTP request count and latency metrics
- `-trusted-proxies` flag to explicitly configure trusted proxy CIDRs for client IP extraction

### Fixed

- Removed Gin global mode side effect from library-level handler construction
- Added stale-entry cleanup for in-memory rate-limit key buckets

## [v0.9.2] - 2026-03-28

### Changed

- Documentation now aligns top-level install, MCP, skill, and architecture guidance with the current multi-surface product story and release metadata
- Release notes index now includes the published `v0.9.0`, `v0.9.1`, and `v0.9.2` entries for stable navigation from the root README

## [v0.9.1] - 2026-03-28

### Fixed

- CI release pipeline: removed redundant `npm publish --dry-run` call that caused false failures when publishing a new tag (npm rejects even dry-run publishes for already-published versions)

## [v0.9.0] - 2026-03-28

### Added

- Homebrew Cask distribution via `brew tap Fanduzi/deltascope && brew install --cask deltascope`
- Claude Code Skill `deltascope-review` for inline SQL review in Claude Code, Codex, Cursor and 40+ AI agents
- `skills/` directory with public Skill file and install documentation
- Install via `npx skills add Fanduzi/DeltaScope --skill deltascope-review`

## [v0.8.1] - 2026-03-28

### Fixed

- npm launcher package metadata now declares the canonical GitHub repository URL so npm provenance validation can accept CI publishes
- source-build default version and release-facing install links now point to `v0.8.1`

## [v0.8.0] - 2026-03-28

### Added

- npm launcher package `@fanduzi/deltascope-mcp` for copy-and-use MCP onboarding through `npx`
- dedicated DeltaScope MCP onboarding guides in English and Chinese with Claude Code, Codex, generic stdio, direct connection, and `connection_ref` examples
- launcher bootstrap diagnostics on `stderr`, GitHub release checksum verification, cache metadata, and override support for release mirrors

### Changed

- release workflow now validates and publishes the MCP launcher package alongside Go release assets
- README and recipe entrypoints now present MCP quick-start guidance and explicit Node 24+ / platform requirements
- launcher cache handling now uses lock timeout and stale-lock recovery to avoid wedged first-run installs

## [v0.7.0] - 2026-03-27

### Added

- Official `deltascope-mcp` stdio server with `audit_sql`, `describe_rule`, `list_rules`, and `get_capabilities`
- Structured MCP tool errors with stable machine-readable codes
- Metadata-aware MCP support for direct `connection` inputs and named `connection_ref` configs
- Shared `internal/application/auditmeta` preparation flow reused by both CLI and MCP adapters
- Explicit MCP output schemas and client-facing capability summaries
- Docker-backed MCP metadata e2e coverage for MySQL and TiDB, including direct and `connection_ref` flows

### Changed

- Release archives and the default installer now ship `deltascope-mcp` alongside `deltascope` and `deltascope-server`
- Source-build default version now points to `v0.7.0`
- English and Chinese README, recipes, release notes, and module docs now describe the official MCP contract and release surface

## [v0.6.2] - 2026-03-25

### Added

- Aggregate `explanation` blocks on audit results and per-statement results
- Structured per-finding explanation fields including `summary`, `why`, `risk`, and `suggestion`
- Metadata-availability notes on explanation metadata for metadata-aware findings
- Public API and HTTP coverage for explainable audit result shapes
- English and Chinese release notes for `v0.6.2`

### Changed

- Markdown CLI output now renders richer explanation details for findings and aggregate audit summaries
- Rule catalog entries now carry explanation-oriented metadata, examples, and remediation hints for discovery commands
- English and Chinese README, recipe, and reference docs now align with runtime output contracts and localized links
- Default source-build version target now points to `v0.6.2`
- Release-facing install examples now target `v0.6.2`

## [v0.5.0] - 2026-03-21

### Added

- Stable `pkg/deltascope` public API
- `deltascope` CLI with `audit`, `config init`, and `version`
- ASCII-logo version command and explicit `--version` behavior
- `deltascope-server` HTTP service with `healthz`, `version`, and `v1/audit`
- Offline-first DDL and DML rule catalog for MySQL and TiDB
- Optional metadata-aware instance facts and table snapshots
