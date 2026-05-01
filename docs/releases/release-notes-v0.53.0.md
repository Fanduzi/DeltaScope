# DeltaScope v0.53.0 Release Notes

## Summary

v0.53.0 adds PostgreSQL `REFRESH MATERIALIZED VIEW` offline audit coverage. DeltaScope now normalizes all four refresh variants through the audit pipeline and surfaces two new PostgreSQL-only rules: one warning on non-concurrent refreshes and one notice on `WITH NO DATA` refreshes that may surprise downstream readers.

## Normalized Forms

| SQL | Normalized Operation |
|-----|---------------------|
| `REFRESH MATERIALIZED VIEW mv` | `refresh_materialized_view` |
| `REFRESH MATERIALIZED VIEW CONCURRENTLY mv` | `refresh_materialized_view` |
| `REFRESH MATERIALIZED VIEW mv WITH DATA` | `refresh_materialized_view` |
| `REFRESH MATERIALIZED VIEW mv WITH NO DATA` | `refresh_materialized_view` |

## New PostgreSQL-Only Rules

| Rule ID | Trigger | Default Level |
|---------|---------|---------------|
| `ddl.pg.refresh_materialized_view.concurrently.warn` | Non-concurrent refresh (default or explicit `WITH DATA`) | warning |
| `ddl.pg.refresh_materialized_view.no_data.notice` | `REFRESH MATERIALIZED VIEW ... WITH NO DATA` | notice |

- `CONCURRENTLY` refresh passes both rules without findings.
- `WITH NO DATA` triggers both rules because it is also non-concurrent.

## Test Coverage

- AST census tests documenting stable parser facts for all four refresh variants.
- Parser/extractor normalization tests.
- Corpus fixtures for both rules.
- Service-level tests through `AuditSQL` for all four variants.
- Public surface tests across all four surfaces: `pkg/deltascope` Audit, CLI Execute, HTTP handler, and MCP audit_sql tool.

## Non-Goals

- This is not live unique-index validation for `CONCURRENTLY`. DeltaScope does not verify whether a unique index exists on the materialized view.
- No query, cost, or dependency analysis is performed on the underlying view query.
- No MySQL/TiDB behavior changes.
- No default policy changes beyond the two new PostgreSQL-only rule entries.

## Install

**macOS (recommended):**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**Generic installer:**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.53.0/install.sh | \
  DELTASCOPE_VERSION=v0.53.0 sh
```
