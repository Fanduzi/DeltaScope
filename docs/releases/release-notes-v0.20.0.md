# DeltaScope v0.20.0 Release Notes

Release date: 2026-04-10

## Overview

DeltaScope `v0.20.0` adds **PostgreSQL Trust & Misconfiguration Guardrails** — a set of additive behaviors that help you understand when your SQL might target a different dialect than the one DeltaScope is using, and make migration-safety findings more actionable. No new rules are added; no existing rule IDs, levels, or trigger conditions change.

## What's Changed

### PostgreSQL Syntax Heuristic Notice

When auditing SQL on the MySQL/TiDB path, DeltaScope now detects common PostgreSQL-specific syntax tokens and emits an advisory notice suggesting `--dialect postgresql`. This helps catch dialect mismatches early — especially in CI pipelines where a wrong dialect flag is easy to miss.

Detected tokens:

| Token | Example |
|-------|---------|
| `RETURNING` | `INSERT INTO users(…) VALUES (…) RETURNING id` |
| `ON CONFLICT` | `INSERT INTO users(…) VALUES (…) ON CONFLICT DO NOTHING` |
| `::` cast | `SELECT id::bigint FROM users` |
| `ALTER COLUMN TYPE USING` | `ALTER TABLE users ALTER COLUMN score TYPE bigint USING abs(score)` |
| `GENERATED AS IDENTITY` | `id bigint GENERATED ALWAYS AS IDENTITY` |

Key behaviors:
- DeltaScope **does not auto-switch dialect** — the notice is advisory only.
- The finding includes an explanation with a clear risk statement and actionable next step.
- Tokens inside string literals, quoted identifiers, and comments are excluded to avoid false positives.

```bash
deltascope audit --sql "insert into users(id) values (1) returning id;" --format markdown
```

Example output:

```text
## Audit Context
- Mode: `offline`
- Dialect: `mysql` (default)
- Trust Note: Dialect remains `mysql` (default). DeltaScope did not auto-switch dialect.

## Global Findings

- [notice] `dialect.postgresql.syntax.detected.notice`: SQL looks like PostgreSQL because it uses "RETURNING" syntax; if you are auditing PostgreSQL, pass --dialect postgresql
  Risk: DeltaScope does not auto-switch dialect. Auditing PostgreSQL SQL with the MySQL/TiDB parser can produce misleading parse errors or incomplete findings.
  Suggestion: If this SQL targets PostgreSQL, re-run with --dialect postgresql to get accurate findings. If not, you can safely ignore this notice.
```

### Explicit PostgreSQL Capability-Boundary Errors

When a PG-capable DeltaScope binary encounters PostgreSQL-specific functionality that is not yet supported (e.g., full PostgreSQL DDL parsing), it now returns typed `PostgreSQLCapabilityBoundaryError` values. This replaces previous heuristic string matching, making it straightforward for CI pipelines and tooling integrations to distinguish real parse failures from known capability limits.

### CLI Output Trust Signals

The CLI trust-oriented output formats now include trust context:

| Format | What's Added |
|--------|-------------|
| **Markdown** | `## Audit Context` section with mode, dialect source, and trust note when a PG syntax notice fires |
| **JSON** | Top-level `context` object with `mode`, `dialect`, `dialect_source` |
| **Quiet** | `[context]` line with mode/dialect/source, `[summary]` line with loaded/applicable/skipped counts |

Rule summary (loaded, applicable, skipped) is now visible in the CLI `json`, `markdown`, and `quiet` formats — making it easy to confirm which rules ran and which were skipped for the current dialect. The `github-actions` and `sarif` formats emit findings only and do not include rule-summary metadata.

### Suggestion Quality Pass for PG Migration-Safety Rules

The four PostgreSQL migration-safety rules introduced in v0.19.0 now provide step-by-step migration guidance:

| Rule | Improved Suggestion |
|------|-------------------|
| `ddl.pg.create_index.concurrently.require` | Mentions `CONCURRENTLY` cannot run inside a transaction |
| `ddl.pg.alter.add_column.non_null_default.rewrite.warn` | 4-step safe path: nullable → backfill → default → not null |
| `ddl.pg.alter.add_check.not_valid.require` | 2-step `NOT VALID` → `VALIDATE CONSTRAINT` with lock-level detail |
| `ddl.pg.alter.set_data_type.rewrite.warn` | Phased shadow-column migration strategy |

### False-Positive Fixes

The PostgreSQL syntax heuristic no longer fires for tokens that appear inside:
- String literals (`'returning'`)
- Double-quoted identifiers (`"returning"`)
- Backtick-quoted identifiers (`` `returning` ``)
- Line comments (`-- returning`)
- Block comments (`/* returning */`)

### Metadata Request Compatibility Fix

Mixed usage of top-level `Schema`/`MetadataProvider` fields with the legacy `Metadata` struct in the public `pkg/deltascope` API no longer drops schema or provider context during audit preparation.

## Install / Upgrade

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.20.0/install.sh | \
  DELTASCOPE_VERSION=v0.20.0 sh
```

macOS users can install with Homebrew:

```bash
brew tap Fanduzi/deltascope
brew install --cask deltascope
```

## Compatibility

No breaking changes. `v0.20.0` extends the existing audit contract with additive capabilities:

- All existing MySQL/TiDB/PostgreSQL offline and metadata-aware behavior is unchanged
- The PostgreSQL syntax notice is a new global finding type at `notice` level — it does not change exit codes unless `--fail-on notice` is set
- Rule IDs, severity levels, and trigger conditions are unchanged
- The typed capability-boundary error is a new error type; existing error-path consumers that check `errors.Is` or `errors.As` will continue to work
- `rule_summary` and `context` are additive fields in JSON output; existing consumers that ignore unknown fields are unaffected
