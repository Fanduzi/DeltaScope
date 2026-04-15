# DeltaScope v0.32.0 Release Notes

Release date: 2026-04-14

## Summary

DeltaScope `v0.32.0` is the **PostgreSQL Boundary Support-Readiness Gate**. It is a decision milestone — not a feature release. No new PostgreSQL support behavior, rule IDs, CLI flags, or public API fields were added. Characterization tests were added to document stable AST facts about generated and identity columns, and a shared contract decision was made to recommend `v0.33.0` as a narrow fact-preservation milestone.

## What Changed

### Characterization tests

Seven characterization tests were added to `internal/infrastructure/parser/postgresql/parser_test.go` to document stable AST facts produced by `pg_query_go/v6`:

- `GeneratedWhen` encoding: `"a"` (ALWAYS) and `"d"` (BY DEFAULT) — stable single-character strings
- `CONSTR_IDENTITY` and `CONSTR_GENERATED` constraint types are distinct and deterministic
- Identity sequence options are `[]*Node` of `DefElem` nodes with `defname` and `Integer` arg
- `CREATE TABLE` and `ALTER TABLE ADD COLUMN` produce identical AST shapes for generated/identity columns
- `ALTER TABLE ... SET GENERATED` produces `DefElem` nodes (not `Constraint` nodes)

These tests assert AST structure only — they do not change any production code path.

### Decision report

A readiness report at `docs/plans/reports/2026-04-14-v0.32.0-pg-boundary-support-readiness-report.md` documents:

- The complete unsupported boundary inventory (generic and explicit)
- AST fact coverage table for generated/identity columns
- Shared contract decision: DeltaScope is ready for narrow fact preservation, not full semantic support
- Recommended `v0.33.0` fields: `GeneratedWhen` (string) and `IsIdentity` (bool) on `spec.Column`
- Deferred areas: generated expression rendering, identity sequence option normalization, rule behavior changes, ALTER TABLE state transitions

## Surface Contract

No surface contract changed. This release does not modify:

- CLI flags or output format
- HTTP API request/response shape
- MCP tool signatures
- `pkg/deltascope` public Go API
- Corpus YAML schema
- Rule configuration or behavior

## What Did Not Change

- DeltaScope does not model generated expressions or identity semantics as supported features.
- No new rule IDs, CLI flags, or public API types were added.
- No parser dependency or package version was changed.
- MySQL and TiDB audit behavior is unchanged.
- All existing PostgreSQL unsupported boundaries (v0.26.0, v0.30.0, v0.31.0) remain in place.
- Production extractor, spec, rule, and policy code is unchanged.

## Follow-up / Next Milestone

Recommended next milestone: **v0.33.0 PostgreSQL Generated/Identity Fact Preservation Pack**.

Scope: Add `GeneratedWhen` (string) and `IsIdentity` (bool) as `omitempty` fields to `spec.Column`. This is narrow fact preservation only — no generated expression rendering, no identity sequence option normalization, no rule behavior changes.

See the readiness report for the full v0.33.0 recommendation, including explicit non-goals.

## Install / Upgrade

```bash
# macOS (recommended)
brew tap Fanduzi/deltascope
brew install --cask deltascope

# Generic installer
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.32.0/install.sh | \
  DELTASCOPE_VERSION=v0.32.0 sh
```
