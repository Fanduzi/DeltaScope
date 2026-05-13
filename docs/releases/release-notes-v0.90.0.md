# DeltaScope v0.90.0 Release Notes

## Summary

v0.90.0 adds PostgreSQL metadata-aware object validation. When auditing PostgreSQL DDL with a live metadata connection, DeltaScope now resolves metadata for selected non-table objects (types, domains, extensions, sequences, materialized views, schemas, foreign servers, user mappings, publications, comments) and enriches lifecycle rule findings with object existence and safe attribute information. This milestone does not add new rules or change rule behavior.

## Metadata-Aware Object Validation

When a PostgreSQL metadata connection is configured, DeltaScope resolves object metadata through `pg_catalog` queries and projects safe attributes into lifecycle rule findings:

```bash
deltascope audit \
  --dialect postgresql \
  --sql "DROP DOMAIN app.email_address" \
  --host 127.0.0.1 --port 5432 --user root --ask-password --schema app
```

Finding output now includes object metadata fields when available:

```json
{
  "rule_id": "ddl.pg.drop_domain.advisory",
  "metadata": {
    "metadata_status": "confirmed",
    "metadata_object_type": "domain",
    "metadata_object_name": "email_address",
    "metadata_exists": true,
    "metadata_has_check": "true"
  }
}
```

When the object does not exist:

```json
{
  "rule_id": "ddl.pg.drop_schema.advisory",
  "metadata": {
    "metadata_status": "not_found",
    "metadata_object_type": "schema",
    "metadata_object_name": "old_schema",
    "metadata_exists": false
  }
}
```

### Supported Object Types

| Object Type | Example SQL | Projected Attributes |
|-------------|-------------|---------------------|
| `schema` | `DROP SCHEMA old_schema` | (none beyond status/name/exists) |
| `type` | `DROP TYPE app.color` | `type_kind` |
| `domain` | `DROP DOMAIN app.email_address` | `has_check` |
| `extension` | `DROP EXTENSION pgcrypto` | `extension_version`, `enabled` |
| `sequence` | `DROP SEQUENCE ticket_seq` | (none beyond status/name/exists) |
| `materialized_view` | `DROP MATERIALIZED VIEW user_summary` | (none beyond status/name/exists) |
| `publication` | `DROP PUBLICATION pub_users` | (none beyond status/name/exists) |
| `foreign_server` | `DROP SERVER fs_test` | `foreign_data_wrapper`, `has_options` |
| `user_mapping` | `DROP USER MAPPING FOR current_user SERVER fs_test` | `server` |
| `comment` | `COMMENT ON TABLE users IS '...'` | `target_type` |

### Safe Attribute Projection

Only 8 safe attribute keys are projected into findings. All sensitive values are filtered by a dual blacklist/whitelist:

**Projectable keys** (whitelist): `type_kind`, `extension_version`, `enabled`, `server`, `foreign_data_wrapper`, `target_type`, `has_options`, `table`.

**Blocked keys** (blacklist): password, secret, token, api_key, connection, dsn, connstr, body, definition, comment, label, query, action_sql, options.

## Fixes

- Schema-qualified name parsing: `DROP DOMAIN app.email_address` and `COMMENT ON TABLE app.users IS '...'` now correctly extract the object name (`email_address`, `users`) instead of the schema prefix (`app`).

## Non-Goals

- No new rule IDs. v0.90.0 enriches existing rule findings with metadata.
- No full PostgreSQL DDL support claim.
- No live privilege/role validation.
- No DCL execution or runtime database firewall behavior.
- DeltaScope does not execute migrations.
- MySQL/TiDB object metadata resolution returns `unavailable` — no behavior change.

## Install

**macOS (recommended):**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**Generic installer:**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.90.0/install.sh | \
  DELTASCOPE_VERSION=v0.90.0 sh
```

## Upgrade

If you previously installed v0.80.0:

```bash
# Homebrew
brew upgrade --cask deltascope

# Generic installer (re-run with new version)
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.90.0/install.sh | \
  DELTASCOPE_VERSION=v0.90.0 sh
```

## Verification

```bash
deltascope --version
# Should output v0.90.0

# Offline audit (no metadata — metadata_status absent from findings)
deltascope audit --dialect postgresql --sql "DROP SCHEMA old_schema"

# Metadata-aware audit (requires PostgreSQL connection)
deltascope audit --dialect postgresql --sql "DROP SCHEMA old_schema" \
  --host 127.0.0.1 --port 5432 --user root --ask-password --schema public \
  --format json
# Finding should include metadata_status: "not_found" or "confirmed"
```
