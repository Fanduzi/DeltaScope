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
- This adapter only establishes the parser seam and normalized statement extraction.
- Rich PostgreSQL statement extraction continues to expand across phases.

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
