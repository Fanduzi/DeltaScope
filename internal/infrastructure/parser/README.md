# Infrastructure Parser Module

Parser adapter namespace for external SQL parser integrations.

## Files

| File | Responsibility |
|------|---------------|
| README.md | Documents parser adapter modules |

## Exports

- No direct Go exports; concrete adapters live in submodules

## Dependencies
- Upstream: `internal/application/audit`
- Downstream: parser-specific adapter modules

## Notes
- `tidb` remains the pure-Go parser adapter for MySQL and TiDB.
- `postgresql` is the Phase 3 build-tagged adapter namespace for PostgreSQL parser wiring.

## Update Rule
- If members/interfaces/dependencies change, update this file in same change.
