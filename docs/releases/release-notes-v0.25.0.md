# DeltaScope v0.25.0 Release Notes

Release date: 2026-04-12

## Overview

DeltaScope `v0.25.0` is the **SQL Corpus & Boundary Confidence Pack**. It introduces a durable, table-driven SQL corpus harness that covers representative MySQL, TiDB, and PostgreSQL baseline cases with two-layer assertions. This release does not add new rules, new CLI flags, new HTTP payload contracts, new MCP tool contracts, or new public Go API contracts.

The corpus answers a practical release-confidence question: which representative SQL statements have actually been run through DeltaScope, and what outcomes are expected?

## What Changed

### SQL Corpus Harness

A new `testdata/sql-corpus/` directory contains dialect-specific SQL examples paired with expected audit outcomes:

- Each case is a `.sql` file + `.expected.yaml` pair.
- The harness is driven through the existing `AuditSQL` application layer — no new runtime code paths.
- Corpus tests run as part of `go test ./internal/application/audit`.

### Two-Layer Assertions

Each corpus case is verified at two layers:

1. **Report-level assertions** — run the full audit pipeline and check `unsupported.count`, `statement_kind`, `findings.include`, and `findings.exclude` against the rendered report.
2. **Semantic parse/extract assertions** — use the internal `Parse` + `Extract` path to access `spec.Statement` fields the report does not expose. This layer asserts `operation` (DDL/DML operation name) and `facts.constraints` (constraint type, name, columns, referenced table/columns).

Both layers are driven by the same `.expected.yaml` file. The semantic layer only runs when the expected file includes `operation` or `facts` fields.

### Corpus Coverage

| Dialect | DDL | DML | Categories |
|---------|-----|-----|------------|
| MySQL | `CREATE TABLE` (primary key, foreign key) | `UPDATE`, `DELETE` | supported, findings, clean |
| TiDB | `CREATE TABLE` (primary key) | `UPDATE`, `DELETE` | supported, findings, clean |
| PostgreSQL | `CREATE TABLE` (CHECK, UNIQUE, FOREIGN KEY, REFERENCES), `CREATE OR REPLACE VIEW` | — | supported, unsupported, findings, boundary |

### Boundary Findings

The PostgreSQL corpus records two current boundary cases:

- `GENERATED ... AS IDENTITY` — recorded as a boundary finding; not fixed in this release.
- `PARTITION BY` — recorded as a boundary finding.

These are intentionally not fixed in `v0.25.0`. The corpus captures current behavior so that future fixes can be verified against a stable baseline.

### What Did Not Change

- No new rules or rule configuration items.
- No new CLI flags or output formats.
- No changes to HTTP, MCP, or public Go API contracts.
- No changes to parser behavior or rule evaluation logic.
- `GENERATED ... AS IDENTITY` remains an unsupported boundary.

## Follow-Up

The following is tracked as a separate follow-up milestone:

- **PostgreSQL CREATE TABLE Unsupported Boundary Pack** — owns the actual boundary fixes for `GENERATED ... AS IDENTITY`, generated stored columns, partitioned `CREATE TABLE`, exclusion constraints, and schema-qualified foreign-key references. This is intentionally separate from the corpus confidence work.

## Compatibility

No breaking changes.

- Existing MySQL, TiDB, and PostgreSQL audit behavior is unchanged.
- Corpus tests are developer/release-confidence assets and do not affect end-user audit behavior.

## Install / Upgrade

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.25.0/install.sh | \
  DELTASCOPE_VERSION=v0.25.0 sh
```

macOS users can install with Homebrew:

```bash
brew tap Fanduzi/deltascope
brew install --cask deltascope
```
