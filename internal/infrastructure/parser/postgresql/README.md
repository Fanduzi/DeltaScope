# PostgreSQL Parser Module

Build-tagged PostgreSQL parser adapter for parser wiring and parser-neutral extractor handoff.

## Files

| File | Responsibility |
|------|---------------|
| parser.go | Parses PostgreSQL SQL text and classifies statements when built with the `postgresql` tag |
| extractor.go | Defines the PostgreSQL extracted-statement wrapper and extractor that populates normalized spec fields (column `NotNull`, `Default`, constraints) for ALTER TABLE ADD COLUMN statements |
| parser_stub.go | Returns the PG-capable build guidance error when PostgreSQL support is not compiled in |

## Exports

- `Parser`
- `Result`
- `ExtractedStatement`
- `New()`
- `Parser.Parse(sql)`

## Dependencies
- Upstream: `internal/application/audit`
- Downstream: `github.com/pganalyze/pg_query_go/v6`, `internal/domain/spec`

## Notes
- The extractor populates `CONSTR_NOTNULL` and `CONSTR_DEFAULT` constraints on `ALTER TABLE ADD COLUMN` into the normalized spec column fields, enabling rules that depend on these facts.
- The extractor normalizes advanced PostgreSQL `CREATE INDEX` forms (partial, expression, INCLUDE, non-btree access methods) into coarse `spec.Index` facts. DeltaScope does not render or semantically analyze predicate SQL or expression SQL.
- The extractor normalizes `ALTER TABLE SET SCHEMA` (dispatched via `AlterObjectSchemaStmt`), `OWNER TO`, named trigger enable/disable, trigger ALL/USER enable/disable, `REPLICA IDENTITY` (DEFAULT/FULL/NOTHING/USING INDEX), and partition attach/detach into `spec.Alter` actions. Trigger ALL/USER variants carry `Options["trigger_scope"]` instead of a trigger name. `REPLICA IDENTITY` variants carry `Options["identity"]` and optionally `Options["index"]`. DeltaScope does not perform live validation of trigger state or replica identity index validity.
- The extractor normalizes schema, sequence, and materialized view create/drop lifecycle forms into `spec.DDL` with `ObjectName` / `ObjectType`. Sequence numeric values and materialized view query semantics are not interpreted. `REFRESH MATERIALIZED VIEW` is normalized with `concurrently` and `with_no_data` coarse facts. DeltaScope does not verify unique-index requirements for concurrent refresh or analyze materialized view query SQL.
- The extractor normalizes PostgreSQL enum type lifecycle DDL (`CREATE TYPE ... AS ENUM`, `ALTER TYPE ... ADD VALUE`, `DROP TYPE`) into `spec.DDL` with `ObjectName` / `ObjectType` and coarse options (`type_kind`, `labels`, `action`, `value`, `if_not_exists`, `placement`, `neighbor`, `if_exists`, `cascade`). Composite type creation (`CREATE TYPE ... AS (...)`) normalizes as `create_type` with `type_kind=composite`, `attributes`, and `attribute_names`. Composite `ALTER TYPE ... RENAME TO` normalizes as `alter_type` with `action=rename`. Composite `ALTER TYPE ... SET SCHEMA` normalizes as `alter_type` with `action=set_schema`. Composite attribute actions (`ADD/DROP/ALTER ATTRIBUTE`, `RENAME ATTRIBUTE`) return explicit unsupported details with stable feature names (`alter_type_add_attribute`, `alter_type_drop_attribute`, `alter_type_alter_attribute_type`, `alter_type_rename_attribute`). DeltaScope does not inspect live type dependencies or enum usage.
- The extractor normalizes PostgreSQL domain lifecycle DDL (`CREATE DOMAIN`, `DROP DOMAIN`, `ALTER DOMAIN`) into `spec.DDL` with operations `create_domain`, `drop_domain`, and `alter_domain`. Create domain options include `type_kind=domain`, `base_type`, `not_null`, `has_default`, `has_check`, and `constraint`. Alter domain options include `action` (set_default, drop_default, set_not_null, drop_not_null, add_constraint, drop_constraint, validate_constraint, rename), `constraint`, `new_name`, `not_null`, `has_default`, and `has_check`. Drop domain options include `if_exists` and `cascade`. DeltaScope does not render DEFAULT or CHECK expression text or perform live dependency validation.
- This adapter only establishes the parser seam and normalized statement extraction.
- Rich PostgreSQL statement extraction continues to expand across phases.

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
