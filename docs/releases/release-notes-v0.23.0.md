# DeltaScope v0.23.0 Release Notes

Release date: 2026-04-11

## Overview

DeltaScope `v0.23.0` is the **PostgreSQL CREATE TABLE Coverage Pack**. It expands the PostgreSQL `CREATE TABLE` shapes that flow through the shared audit pipeline so common constraint-rich create-table statements now produce normal audit results instead of falling outside the supported surface.

This release does not claim full PostgreSQL DDL support. It also does not add new PostgreSQL rule IDs, new severity levels, new CLI flags, new HTTP payload contracts, new MCP tool contracts, or new public Go API contracts.

## What's Changed

### Broader PostgreSQL `CREATE TABLE` Coverage

DeltaScope now supports and audits more common PostgreSQL `CREATE TABLE` constraint forms:

- table-level named `CHECK`
- column-level inline `CHECK`
- table-level named `UNIQUE`
- column-level inline `UNIQUE`
- table-level named `FOREIGN KEY`
- column-level inline `REFERENCES`

### Shared Rule Reuse

This is a coverage expansion, not a new rule pack.

- Existing structured naming governance can apply to named PostgreSQL `CHECK`, `UNIQUE`, and `FOREIGN KEY` constraints when policy enables those rule families.
- Existing shared index rules can consume normalized index facts emitted by inline `UNIQUE`.
- Inline `REFERENCES` is exposed as parser-owned shared structure only. It should not be read as adding new metadata-aware foreign-key semantics.

### Surface Parity

The expanded PostgreSQL `CREATE TABLE` coverage is confirmed across:

- `deltascope` CLI
- HTTP `POST /v1/audit`
- MCP `audit_sql`
- public Go API `pkg/deltascope`

## Compatibility

No breaking changes.

- Existing MySQL, TiDB, and PostgreSQL audit behavior remains compatible.
- No new rule IDs, severity levels, or trigger conditions are introduced.
- CLI, HTTP, MCP, and `pkg/deltascope` public contracts remain unchanged.
- Release-surface entrypoints from `v0.22.0` remain the canonical package/docs verification path.

## Install / Upgrade

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.23.0/install.sh | \
  DELTASCOPE_VERSION=v0.23.0 sh
```

macOS users can install with Homebrew:

```bash
brew tap Fanduzi/deltascope
brew install --cask deltascope
```
