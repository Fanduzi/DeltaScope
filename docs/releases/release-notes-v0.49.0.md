# DeltaScope v0.49.0 Release Notes

## Summary

v0.49.0 ships the PostgreSQL Advanced CREATE INDEX Normalization Pack. PostgreSQL partial indexes, expression indexes, INCLUDE covering indexes, and non-btree indexes now normalize instead of returning unsupported. These forms flow through the normal audit pipeline and trigger the existing `ddl.pg.create_index.concurrently.require` rule when `CONCURRENTLY` is missing.

## Added

- Advanced PostgreSQL `CREATE INDEX` variants now normalize into coarse index facts:
  - Partial indexes (`WHERE` clause) — `HasPredicate` flag
  - Expression indexes (`LOWER(col)`, etc.) — `HasExpressionKeys` flag and `ExpressionCount`
  - INCLUDE covering indexes — `IncludedColumns` list
  - Non-btree access methods (`USING gin`, `USING hash`, etc.) — `AccessMethod` field
- `spec.Index` extended with five new fields: `AccessMethod`, `IncludedColumns`, `HasPredicate`, `HasExpressionKeys`, `ExpressionCount`.
- Service-level tests for all five advanced index forms through `AuditSQL`.
- Corpus fixtures for partial, expression, INCLUDE, and GIN index forms.
- Public surface tests across all four surfaces: `pkg/deltascope` Audit, CLI Execute, HTTP handler, and MCP audit_sql tool.

## Changed

- PostgreSQL extractor no longer returns unsupported for partial, expression, INCLUDE, or non-btree `CREATE INDEX` variants.
- Census movement from v0.48 to v0.49:

  | Metric | v0.48 | v0.49 |
  |--------|-------|-------|
  | finding-covered | 31 | 35 |
  | unsupported-explicit | 22 | 18 |
  | classified DDL | 34 | 38 |
  | normalized | 34 | 38 |
  | corpus-covered | 19/56 | 23/56 |
  | parseable | 56 | 56 |
  | parser-error | 0 | 0 |

## Non-Goals

- No new rule IDs. The existing `ddl.pg.create_index.concurrently.require` rule now applies to the newly normalized forms.
- No default policy changes.
- No MySQL/TiDB behavior changes.
- No predicate SQL or expression SQL semantic analysis. DeltaScope preserves coarse presence/count flags only.
- Public response types (`StatementResult`, CLI JSON, HTTP JSON, MCP structured content) do not expose full internal `spec.Index` advanced fields yet. This is a future surface extension.
- Remaining unsupported PG DDL forms (18) remain explicit boundaries.

## Install

**macOS (recommended):**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**Generic installer:**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.49.0/install.sh | \
  DELTASCOPE_VERSION=v0.49.0 sh
```
