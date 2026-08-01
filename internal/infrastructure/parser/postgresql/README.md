# PostgreSQL Parser Module

Build-tagged PostgreSQL parser adapter for parser wiring and parser-neutral extractor handoff.

## Files

| File | Responsibility |
|------|---------------|
| parser.go | Parses PostgreSQL SQL text and classifies statements when built with the `postgresql` tag |
| extractor.go | Defines the PostgreSQL extracted-statement wrapper and extractor that populates normalized spec fields (column `NotNull`, `Default`, constraints) for ALTER TABLE ADD COLUMN statements |
| query_access.go | Extracts query access facts (read classification, relations, column references, output lineage, unproven effect reasons) from PostgreSQL AST |
| query_access_effect_candidates.go | Defines internal EffectCandidate facts and helpers collected during the complete effect traversal |
| query_access_effect_candidates_postgresql_tag_test.go | Verifies exact COUNT(1) statement envelopes and literal rejection |
| query_access_stub.go | Returns ErrPostgreSQLNotAvailable for query access extraction when built without the `postgresql` tag |
| parser_stub.go | Returns the PG-capable build guidance error when PostgreSQL support is not compiled in |

## Exports

- `Parser`
- `Result`
- `ExtractedStatement`
- `New()`
- `Parser.Parse(sql)`
- `QueryAccessExtractor`
- `QueryAccessExtractor.ExtractQueryAccess(ctx, sql, dialect, defaultSchema)`
- `QueryAccessFacts`
- `EffectCandidate` (internal-only; untrusted; not a public Result field)
- `EffectCandidateKind`
- `OperandKindHint`
- `RelationFacts`
- `ColumnRefFacts`
- `OutputFacts`
- `UnresolvedFacts`

## Dependencies
- Upstream: `internal/application/audit`, `internal/application/queryaccess`
- Downstream: `github.com/pganalyze/pg_query_go/v6`, `internal/domain/spec`, `internal/domain/queryaccess`

## Notes
- Query Access `EffectCandidate` values are **internal-only and untrusted**: they are resolver inputs (kind, ordinal, name path, arity, operand kind hints, aggregate/window/filter/cast flags), not a trust root. They must not be copied into `domain.Result`, reason codes, or SDK/CLI/HTTP JSON. Public outputs continue to use only bounded `unproven_*` reason codes. Structural `AND/OR/NOT` is not a catalog candidate.
- `QueryAccessFacts.ExactCountIntegerOneStatement` is a parser-only envelope fact; it is true only for one unqualified `COUNT(1)` target over one schema-qualified relation with no query modifiers, joins, subqueries, set operations, or relationless form.
- The extractor populates `CONSTR_NOTNULL` and `CONSTR_DEFAULT` constraints on `ALTER TABLE ADD COLUMN` into the normalized spec column fields, enabling rules that depend on these facts.
- The extractor normalizes advanced PostgreSQL `CREATE INDEX` forms (partial, expression, INCLUDE, non-btree access methods) into coarse `spec.Index` facts. DeltaScope does not render or semantically analyze predicate SQL or expression SQL.
- The extractor normalizes `ALTER TABLE SET SCHEMA` (dispatched via `AlterObjectSchemaStmt`), `OWNER TO`, named trigger enable/disable, trigger ALL/USER enable/disable, `REPLICA IDENTITY` (DEFAULT/FULL/NOTHING/USING INDEX), and partition attach/detach into `spec.Alter` actions. Trigger ALL/USER variants carry `Options["trigger_scope"]` instead of a trigger name. `REPLICA IDENTITY` variants carry `Options["identity"]` and optionally `Options["index"]`. DeltaScope does not perform live validation of trigger state or replica identity index validity.
- The extractor normalizes schema, sequence, and materialized view create/drop lifecycle forms into `spec.DDL` with `ObjectName` / `ObjectType`. `CREATE SCHEMA name` / `CREATE SCHEMA IF NOT EXISTS name` normalize to `create_schema` with `ObjectType="schema"`. `CREATE SCHEMA AUTHORIZATION` returns unsupported with feature `create_schema_authorization`. `CREATE SCHEMA` with nested schema elements returns unsupported with feature `create_schema_nested_objects`. Sequence numeric values and materialized view query semantics are not interpreted. `REFRESH MATERIALIZED VIEW` is normalized with `concurrently` and `with_no_data` coarse facts. DeltaScope does not verify unique-index requirements for concurrent refresh or analyze materialized view query SQL.
- The extractor normalizes PostgreSQL enum type lifecycle DDL (`CREATE TYPE ... AS ENUM`, `ALTER TYPE ... ADD VALUE`, `DROP TYPE`) into `spec.DDL` with `ObjectName` / `ObjectType` and coarse options (`type_kind`, `labels`, `action`, `value`, `if_not_exists`, `placement`, `neighbor`, `if_exists`, `cascade`). Composite type creation (`CREATE TYPE ... AS (...)`) normalizes as `create_type` with `type_kind=composite`, `attributes`, and `attribute_names`. Composite `ALTER TYPE ... RENAME TO` normalizes as `alter_type` with `action=rename`. Composite `ALTER TYPE ... SET SCHEMA` normalizes as `alter_type` with `action=set_schema`. Composite attribute actions (`ADD/DROP/ALTER ATTRIBUTE`, `RENAME ATTRIBUTE`) return explicit unsupported details with stable feature names (`alter_type_add_attribute`, `alter_type_drop_attribute`, `alter_type_alter_attribute_type`, `alter_type_rename_attribute`). DeltaScope does not inspect live type dependencies or enum usage.
- The extractor normalizes PostgreSQL domain lifecycle DDL (`CREATE DOMAIN`, `DROP DOMAIN`, `ALTER DOMAIN`) into `spec.DDL` with operations `create_domain`, `drop_domain`, and `alter_domain`. Create domain options include `type_kind=domain`, `base_type`, `not_null`, `has_default`, `has_check`, and `constraint`. Alter domain options include `action` (set_default, drop_default, set_not_null, drop_not_null, add_constraint, drop_constraint, validate_constraint, rename), `constraint`, `new_name`, `not_null`, `has_default`, and `has_check`. Drop domain options include `if_exists` and `cascade`. DeltaScope does not render DEFAULT or CHECK expression text or perform live dependency validation.
- The extractor normalizes PostgreSQL extension lifecycle DDL (`CREATE EXTENSION`, `DROP EXTENSION`, `ALTER EXTENSION UPDATE`, `ALTER EXTENSION SET SCHEMA`) into `spec.DDL` with operations `create_extension`, `drop_extension`, and `alter_extension`. Create extension options include `if_not_exists`, `schema`, `version`, and `cascade`. Drop extension options include `if_exists` and `cascade`. Alter extension update options include `action=update` and `version`. Alter extension set schema options include `action=set_schema` and `new_schema`. Member mutation (`ALTER EXTENSION ... ADD/DROP member`) returns explicit unsupported details with stable feature names (`alter_extension_add_member`, `alter_extension_drop_member`). DeltaScope does not inspect live extension dependencies or catalog membership.
- The extractor normalizes PostgreSQL table privilege DCL (`GRANT ... ON TABLE`, `REVOKE ... ON TABLE`) for ordinary single-table forms into `spec.DDL` with operations `grant_table` and `revoke_table`. Options include `privileges` (CSV), `all_privileges` (when `ALL PRIVILEGES` is used), `grantees` (CSV), `schema`, and `cascade` (revoke with `CASCADE`). Deferred forms return explicit unsupported details: `grant_all_tables_in_schema` for `ALL TABLES IN SCHEMA`, `grant_table` for non-table object types (e.g. sequences), `grant_role` for role membership (`GRANT role TO role`), and `alter_default_privileges` for `ALTER DEFAULT PRIVILEGES`. DeltaScope does not inspect live grant state or resolve wildcard privilege expansion.
- The extractor normalizes PostgreSQL function and procedure lifecycle DDL (`CREATE FUNCTION`, `CREATE OR REPLACE FUNCTION`, `DROP FUNCTION`, `CREATE PROCEDURE`, `DROP PROCEDURE`) into `spec.DDL` with operations `create_function`, `drop_function`, `create_procedure`, and `drop_procedure`. Create function options include `replace` (when `OR REPLACE` is used), `security_definer` (`"true"` or `"false"` from the `SECURITY` defelem), and `schema` for schema-qualified names. Drop function/procedure options include `if_exists`. Function body, language, and argument types are not interpreted. DeltaScope does not inspect live function state or dependency graphs.
- The extractor normalizes PostgreSQL advanced view lifecycle DDL (`CREATE OR REPLACE VIEW`, `CREATE TEMP VIEW`, `CREATE VIEW ... WITH CHECK OPTION`, `ALTER VIEW ... RENAME TO`, `ALTER VIEW ... SET SCHEMA`, `DROP VIEW ... CASCADE`) into `spec.DDL` with operations `create_view`, `alter_view`, and `drop_view`. Create view options include `replace`, `temporary`, `check_option`, and `check_option_scope` (local/cascaded). Alter view options include `action` (rename_view, set_schema), `new_name`, and `new_schema`. Drop view options include `cascade` and `if_exists`. DeltaScope does not inspect live view definitions or dependency graphs.
- The extractor normalizes PostgreSQL annotation DDL (`COMMENT ON`, `SECURITY LABEL`) into `spec.DDL` with operations `comment_on` and `security_label`. Options include `target_type` (table, view, etc.), `target_name`, `is_null` (true/false), and `provider` (for security labels). Comment text and security label text are never stored in normalized specs. DeltaScope does not inspect live comment or label state.
- The extractor normalizes PostgreSQL event trigger DDL (`CREATE/ALTER/DROP EVENT TRIGGER`) into `spec.DDL` with operations `create_event_trigger`, `alter_event_trigger`, and `drop_event_trigger`. Options include `event` (ddl_command_end, etc.), `function` (name only, no body), `action` (enable/disable/rename/enable_replica/enable_always), `new_name`, and `if_exists`. Function bodies are never stored. DeltaScope does not inspect live event trigger state.
- The extractor normalizes PostgreSQL rewrite rule DDL (`CREATE/ALTER/DROP RULE`) into `spec.DDL` with operations `create_rule`, `alter_rule`, and `drop_rule`. Options include `table` (target relation), `event` (insert/update/delete/select), `action` (rename), `new_name`, and `if_exists`. Rule action/query bodies are never stored. DeltaScope does not inspect live rule state.
- This adapter only establishes the parser seam and normalized statement extraction.
- Rich PostgreSQL statement extraction continues to expand across phases.

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
