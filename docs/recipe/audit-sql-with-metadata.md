# Audit SQL With Metadata

Use metadata-aware mode when current schema state or instance configuration matters. Metadata-aware mode activates automatically when any connection flag is provided (`--host`, `--port`, `--user`, `--password-env`, `--password-file`, `--ask-password`, `--schema`, or `--socket`).

In this mode DeltaScope connects to the target database before evaluating rules, attaches a `TableSnapshot` (current column list, indexes, row estimates) and `InstanceFacts` (version, key configuration variables) to each statement, and then runs the full rule set — including rules that require live schema context.

## Minimum Permissions

DeltaScope only reads metadata. Grant at least:

```sql
-- Minimum: read information_schema and performance_schema
GRANT SELECT ON information_schema.* TO 'deltascope'@'%';
GRANT SELECT ON performance_schema.global_variables TO 'deltascope'@'%';

-- If you want per-database metadata (table existence, column list, index list):
-- The information_schema grants above are sufficient; no additional per-table grants are needed.
```

> **Note:** DeltaScope never writes to your database. No DDL/DML permissions are required.

TiDB uses the same `information_schema` structure. The `performance_schema.global_variables` grant is optional for TiDB — DeltaScope falls back gracefully when it is unavailable.

For PostgreSQL, the user needs `USAGE` on the target schema and default access to `pg_catalog` (available to all roles by default):

```sql
-- PostgreSQL: grant USAGE on the target schema
GRANT USAGE ON SCHEMA app TO deltascope;
-- pg_catalog access is granted to PUBLIC by default; no additional grant is needed.
```

## Connecting via TCP

```bash
deltascope audit \
  --sql "ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'email address'" \
  --host 127.0.0.1 \
  --port 3306 \
  --user deltascope \
  --ask-password \
  --schema app
```

`--ask-password` prompts for the password interactively so it never appears in shell history or process listings. For scripted environments, prefer `--password-env` or `--password-file` so the secret does not appear in process arguments:

```bash
deltascope audit \
  --sql "ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'email address'" \
  --host 127.0.0.1 \
  --port 3306 \
  --user deltascope \
  --password-env DELTASCOPE_PASSWORD \
  --schema app
```

Or load the password from a file:

```bash
deltascope audit \
  --sql "ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'email address'" \
  --host 127.0.0.1 \
  --port 3306 \
  --user deltascope \
  --password-file ~/.config/deltascope/mysql-password.txt \
  --schema app
```

### PostgreSQL Connection

Use `--dialect postgresql` and the PostgreSQL port (default `5432`) when connecting to a PostgreSQL instance. `--schema` maps to the PostgreSQL schema (default `public`):

```bash
deltascope audit \
  --sql "ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL DEFAULT ''" \
  --dialect postgresql \
  --host 127.0.0.1 \
  --port 5432 \
  --user deltascope \
  --ask-password \
  --schema public
```

For scripted use, prefer `--password-env` or `--password-file`:

```bash
deltascope audit \
  --sql "ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL DEFAULT ''" \
  --dialect postgresql \
  --host 127.0.0.1 \
  --port 5432 \
  --user deltascope \
  --password-env DELTASCOPE_PASSWORD \
  --schema public
```

## Connecting via TLS

When the database requires an encrypted connection, use `--tls-mode enabled`. TLS requires `--host` and `--user` and rejects `--socket`:

```bash
deltascope audit \
  --sql "ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'email address'" \
  --host db.example.com \
  --port 3306 \
  --user deltascope \
  --ask-password \
  --tls-mode enabled \
  --schema app
```

To provide a custom CA certificate for TLS verification:

```bash
deltascope audit \
  --sql "ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL DEFAULT ''" \
  --dialect postgresql \
  --host pg.example.com \
  --port 5432 \
  --user deltascope \
  --ask-password \
  --tls-mode enabled --tls-ca-file /etc/ssl/certs/pg-ca.pem \
  --schema public
```

## Connecting via Unix Socket

On a machine where MySQL or TiDB runs locally, a Unix socket connection avoids TCP overhead and firewall rules:

```bash
deltascope audit \
  --sql "ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'email address'" \
  --socket /var/run/mysqld/mysqld.sock \
  --user deltascope \
  --ask-password \
  --schema app
```

Use TCP when the database is on a different host. Use Unix socket for local development or CI containers where the socket path is known.

## Complete JSON Output

The JSON output includes a `context` field that records how dialect and schema were resolved:

```bash
deltascope audit \
  --sql "ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'email address'" \
  --host 127.0.0.1 \
  --port 3306 \
  --user deltascope \
  --ask-password \
  --schema app \
  --format json
```

```json
{
  "verdict": "pass",
  "summary": {
    "statements": 1,
    "blockers": 0,
    "warnings": 0,
    "notices": 0
  },
  "context": {
    "mode": "metadata-aware",
    "dialect": "mysql",
    "dialect_source": "detected",
    "schema": "app",
    "schema_source": "flag"
  },
  "statements": [
    {
      "index": 0,
      "kind": "ddl",
      "raw_sql": "ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'email address'",
    }
  ]
}
```

`dialect_source: "detected"` means DeltaScope queried `tidb_version()` (not found → MySQL confirmed). `schema_source: "flag"` means the schema was taken from `--schema`.

## Schema Resolution

DeltaScope resolves the target schema for each statement using the following priority order:

| Case | How it works | Example |
|------|-------------|---------|
| **SQL-qualified** | The SQL names the schema explicitly. DeltaScope uses it directly. | `ALTER TABLE app.users ADD COLUMN ...` |
| **`--schema` flag** | No schema in SQL; `--schema` is set. DeltaScope uses the flag value. | `--schema app` with `ALTER TABLE users ...` |
| **Auto-inferred** | No schema in SQL; no `--schema`. The table name exists in exactly one schema visible to the connected user. DeltaScope uses it. | Single `users` table across all visible schemas |
| **Ambiguous error** | No schema in SQL; no `--schema`. The table name exists in more than one visible schema. DeltaScope exits with code `2` and asks you to disambiguate. | `users` in both `app` and `legacy` |

### Ambiguous schema example

```bash
# users exists in both `app` and `legacy` schemas — DeltaScope refuses to guess
deltascope audit \
  --sql "ALTER TABLE users ADD COLUMN email VARCHAR(255)" \
  --host 127.0.0.1 \
  --port 3306 \
  --user deltascope \
  --ask-password
```

```text
Error: schema inference is ambiguous; table `users` found in [app legacy]; pass --schema to disambiguate
exit code: 2
```

Fix by adding `--schema app` or by qualifying the table name in SQL: `ALTER TABLE app.users ADD COLUMN email VARCHAR(255)`.

## TiDB Notes

- **Auto-detection**: DeltaScope queries `tidb_version()` at connection time. If the function exists, the dialect is set to `tidb`; otherwise MySQL is assumed. `dialect_source` in the JSON output will be `"detected"`.
- **Omit `--dialect`** when connecting to TiDB — let auto-detection handle it. Passing `--dialect mysql` when the target is TiDB (or vice versa) causes exit code `2`.
- **`innodb_adaptive_hash_index`**: Always treated as inactive for TiDB targets. Rules that depend on this variable behave accordingly.
- **Merge-alter rules**: `ddl.alter.merge.mysql.require` is enabled in the shipped policy; `ddl.alter.merge.tidb.require` is disabled by default (`required: false`). Adjust in your config if needed.
- **`performance_schema.global_variables`**: DeltaScope falls back gracefully when this view is unavailable on TiDB. Instance facts that depend on it may be absent, but the audit proceeds.

## PostgreSQL Notes

- **`--dialect postgresql` is required**: Unlike MySQL/TiDB, DeltaScope does not auto-detect PostgreSQL at connection time. You must pass `--dialect postgresql` explicitly.
- **Schema defaults to `public`**: When `--schema` is omitted, DeltaScope uses `public` as the default PostgreSQL schema. Pass `--schema` explicitly if your tables live in a different schema.
- **MySQL-specific rules are not applicable**: Rules that reference InnoDB features (engine allowlists, row format, adaptive hash index, `innodb_large_prefix`) are skipped for PostgreSQL targets.
- **DML impact estimation uses `EXPLAIN`**: For `UPDATE` and `DELETE` statements, DeltaScope issues a read-only `EXPLAIN` against the PostgreSQL planner to refine row estimates. It does not execute `EXPLAIN ANALYZE` and never runs the actual DML. `INSERT` statements do not trigger planner estimation.

## Metadata-Aware vs Offline Output

The same SQL can produce different findings depending on whether metadata is available. For example, an `ALTER TABLE` that adds a column which already exists is only detectable in metadata-aware mode:

**Offline** (no connection):

```bash
deltascope audit --sql "ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'email'" --format json
```

```json
{
  "verdict": "pass",
  "summary": { "statements": 1, "blockers": 0, "warnings": 0, "notices": 0 },
  "statements": [
    { "index": 0, "kind": "ddl", "raw_sql": "..." }
  ]
}
```

**Metadata-aware** (column already exists in `app.users`):

```bash
deltascope audit \
  --sql "ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'email'" \
  --host 127.0.0.1 --port 3306 --user deltascope --ask-password --schema app \
  --format json
```

```json
{
  "verdict": "reject",
  "summary": { "statements": 1, "blockers": 1, "warnings": 0, "notices": 0 },
  "context": {
    "mode": "metadata-aware",
    "dialect": "mysql",
    "dialect_source": "detected",
    "schema": "app",
    "schema_source": "flag"
  },
  "statements": [
    {
      "index": 0,
      "kind": "ddl",
      "raw_sql": "ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'email'",
      "findings": [
        {
          "rule_id": "ddl.alter.add_column.exists.forbid",
          "level": "blocker",
          "message": "column `email` already exists in table `users`",
          "suggestion": "Remove the ADD COLUMN clause or check the column name",
          "location": { "line": 1, "column": 1 }
        }
      ]
    }
  ]
}
```

This is why metadata-aware mode is valuable for `ALTER TABLE` pre-flight checks.
