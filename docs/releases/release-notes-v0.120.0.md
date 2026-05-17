# DeltaScope v0.120.0 Release Notes

## Summary

v0.120.0 enriches PostgreSQL migration-safety findings with bounded semantic metadata. CREATE INDEX, ADD COLUMN, and ALTER COLUMN TYPE findings now carry structured metadata describing index shape, default value classification, and USING clause presence — without emitting raw SQL text.

## What Changed

v0.120.0 is a metadata enrichment release. Three existing PostgreSQL migration-safety rule families now project bounded semantic metadata into findings:

### CREATE INDEX — Bounded Index Shape Metadata

`ddl.pg.create_index.concurrently.require` findings now include:

| Metadata Key | Description |
|---|---|
| `index_kind` | Index classification (`secondary`, `unique`, `primary`) |
| `access_method` | PostgreSQL access method name (e.g., `btree`, `gin`) |
| `column_count` | Number of indexed columns |
| `included_column_count` | Number of INCLUDE covering columns |
| `has_predicate` | Whether the index has a WHERE clause |
| `has_expression_keys` | Whether any key column is an expression |
| `expression_count` | Number of expression key columns |

### ADD COLUMN — Default Semantics Metadata

`ddl.pg.alter.add_column.non_null_default.rewrite.warn` findings now include:

| Metadata Key | Description |
|---|---|
| `not_null` | Whether the column is NOT NULL |
| `has_default` | Whether the column has a DEFAULT |
| `default_kind` | Default value classification: `literal`, `null`, `function_call`, `expression`, `unknown` |

`ddl.pg.alter.add_column.non_null_no_default.warn` findings now include `not_null` and `has_default`.

### ALTER COLUMN TYPE — USING Clause Metadata

`ddl.pg.alter.set_data_type.rewrite.warn` findings now include:

| Metadata Key | Description |
|---|---|
| `has_using` | Whether the ALTER includes a USING clause |

### No-Leak Contract

Findings do not emit:
- Predicate SQL text
- Expression index SQL text
- Default expression SQL text
- Default function names
- USING expression SQL text

## Usage

```bash
# CREATE INDEX — check metadata in JSON output
deltascope audit --dialect postgresql \
  --sql "CREATE INDEX idx_users_email ON users USING gin (email) INCLUDE (status) WHERE active = true" \
  --format json

# ADD COLUMN — check default_kind in metadata
deltascope audit --dialect postgresql \
  --sql "ALTER TABLE users ADD COLUMN created_at timestamptz NOT NULL DEFAULT now()" \
  --format json

# ALTER COLUMN TYPE — check has_using in metadata
deltascope audit --dialect postgresql \
  --sql "ALTER TABLE users ALTER COLUMN score TYPE bigint USING score::bigint" \
  --format json
```

All three rules are offline — they do not require a database connection.

## Non-Goals

- No full PostgreSQL DDL support claim. This is bounded semantic metadata enrichment for selected migration-safety findings.
- No full PostgreSQL expression analysis. Expression and predicate presence is recorded as boolean/count metadata only; SQL text is not emitted.
- No function volatility/immutability analysis. `default_kind` classifies the AST node shape, not the runtime behavior.
- No live database/catalog validation.
- No DCL/permission expansion.
- No v1.0/stable API contract claim.
- DeltaScope does not execute migrations.

## Install

**macOS (recommended):**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**Generic installer:**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.120.0/install.sh | \
  DELTASCOPE_VERSION=v0.120.0 sh
```

## Upgrade

If you previously installed v0.110.0:

```bash
# Homebrew
brew upgrade --cask deltascope

# Generic installer (re-run with new version)
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.120.0/install.sh | \
  DELTASCOPE_VERSION=v0.120.0 sh
```

## Verification

```bash
deltascope --version
# Should output v0.120.0

# CREATE INDEX — verify metadata
deltascope audit --dialect postgresql --sql "CREATE INDEX idx_users_email ON users USING gin (email) INCLUDE (status) WHERE active = true" --format json
# Finding should include rule_id: "ddl.pg.create_index.concurrently.require" with metadata.access_method = "gin"

# ADD COLUMN — verify metadata
deltascope audit --dialect postgresql --sql "ALTER TABLE users ADD COLUMN created_at timestamptz NOT NULL DEFAULT now()" --format json
# Finding should include rule_id: "ddl.pg.alter.add_column.non_null_default.rewrite.warn" with metadata.default_kind = "function_call"
```
