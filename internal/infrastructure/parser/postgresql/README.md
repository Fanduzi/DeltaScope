# PostgreSQL Parser Module

Build-tagged PostgreSQL parser adapter for parser wiring and parser-neutral extractor handoff.

## Files

| File | Responsibility |
|------|---------------|
| parser.go | Parses PostgreSQL SQL text and classifies statements when built with the `postgresql` tag |
| extractor.go | Defines the PostgreSQL extracted-statement wrapper and minimal Phase 3 extractor implementation for normalized raw SQL handoff |
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
- This Phase 3 adapter only establishes the parser seam and normalized raw-SQL handoff.
- It does not yet expose PostgreSQL in pure-Go capability summaries.
- Rich PostgreSQL statement extraction stays for later phases.

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
