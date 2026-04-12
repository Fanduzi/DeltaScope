# DeltaScope v0.26.0 Release Notes

Release date: 2026-04-12

## Overview

DeltaScope `v0.26.0` is the **PostgreSQL CREATE TABLE Unsupported Boundary Pack**. It tightens the extractor-level boundary contract for PostgreSQL `CREATE TABLE` forms that are explicitly outside the supported surface, backing each boundary with corpus cases and surface parity tests. This release does not add new rules, new CLI flags, or new public API contracts, and it does not represent full PostgreSQL `CREATE TABLE` support.

## What Changed

The PostgreSQL extractor now marks four `CREATE TABLE` boundaries as explicit unsupported at the extractor level:

| Feature | Extractor Tag | Example |
|---------|---------------|---------|
| Identity columns | `generated_as_identity` | `id bigint GENERATED ALWAYS AS IDENTITY` |
| Generated stored columns | `generated_column` | `full_name text GENERATED ALWAYS AS (...) STORED` |
| Exclusion constraints | `exclusion_constraint` | `EXCLUDE USING gist (...)` |
| Partitioned tables | `partitioning` | `PARTITION BY RANGE (created_at)` |

Previously, some of these forms were silently accepted or partially handled. They are now explicitly rejected with a clear `unsupported` reason, making the boundary contract stable and testable.

## Boundary Contracts

Each boundary is locked by three layers:

1. **Extractor-level**: the PostgreSQL extractor returns an `UnsupportedDetail` with the feature tag and reason string.
2. **Corpus-level**: `testdata/sql-corpus/postgresql/` includes dedicated cases with `.expected.yaml` assertions on `unsupported.count` and `unsupported.include`.
3. **Surface-level**: CLI, HTTP, MCP, and `pkg/deltascope` parity tests verify the unsupported contract on every transport.

## Surface Contract

Unsupported statements are exposed differently depending on the transport:

- **CLI** and **`pkg/deltascope`**: return a partial result with an `unsupported` array carrying `feature` and `reason` fields, plus the `ErrUnsupportedStatement` sentinel error. The CLI renders the result (including the unsupported section) and exits with the audit exit code.
- **HTTP** and **MCP**: expose unsupported statements as transport-level errors (HTTP error response, MCP tool error with `IsError: true` and structured error content). The underlying `deltascope.Audit` returns an error for unsupported boundaries, and the transport adapters propagate it.

## What Did Not Change

- No new rule IDs were added. These boundaries are extractor-level unsupported contracts, not rule findings.
- No new CLI flags, HTTP payload contracts, MCP tool contracts, or public Go API contracts.
- No changes to the supported PostgreSQL `CREATE TABLE` semantics from `v0.23.0` and `v0.24.0` (named CHECK, UNIQUE, FOREIGN KEY, inline REFERENCES, inline UNIQUE).
- No changes to the MySQL or TiDB audit surfaces.

## Follow-up

- **Schema-qualified foreign-key references** remain a decision point. If preserving schema requires a shared contract expansion, `ReferencedSchema` will be addressed in a later milestone rather than mixed into boundary tightening.
- **`ALTER TABLE ... GENERATED`** boundary coverage is a potential follow-up but is not committed.

## Install / Upgrade

```bash
# macOS (recommended)
brew tap Fanduzi/deltascope
brew install --cask deltascope

# Generic installer
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.26.0/install.sh | \
  DELTASCOPE_VERSION=v0.26.0 sh
```
