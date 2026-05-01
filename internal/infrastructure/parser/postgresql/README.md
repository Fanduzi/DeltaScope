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
- This adapter only establishes the parser seam and normalized statement extraction.
- Rich PostgreSQL statement extraction continues to expand across phases.

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
