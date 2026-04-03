# Troubleshoot Metadata-Aware Audit

Use this guide when `deltascope audit` works offline but metadata-aware mode fails or behaves unexpectedly. Metadata-aware mode activates automatically when any connection flag (`--host`, `--port`, `--user`, `--password`, `--password-env`, `--password-file`, `--ask-password`, `--schema`, `--socket`) is supplied.

## Required MySQL Permissions

DeltaScope only reads metadata — it never writes to your database.

| Permission | Why it is needed |
|------------|-----------------|
| `SELECT ON information_schema.*` | Table existence, column list, index list, table options |
| `SELECT ON performance_schema.global_variables` | Instance facts: version, key configuration variables (e.g. `innodb_adaptive_hash_index`) |

Grant these with:

```sql
GRANT SELECT ON information_schema.* TO 'deltascope'@'%';
GRANT SELECT ON performance_schema.global_variables TO 'deltascope'@'%';
FLUSH PRIVILEGES;
```

**TiDB:** The same `information_schema` grants apply. `performance_schema.global_variables` is optional — DeltaScope falls back gracefully when it is unavailable. No additional per-database or per-table grants are required.

## Common Symptoms and Fixes

### "schema inference is ambiguous; pass --schema"

Full message shape:

```text
Error: schema inference is ambiguous; table `users` found in [app legacy]; pass --schema to disambiguate
exit code: 2
```

**Why it happens:** The target table name exists in more than one schema visible to the connected user. DeltaScope refuses to guess which schema you meant.

**Resolution steps (in priority order):**

1. **Qualify the table name in SQL** — most explicit, always wins:
   ```sql
   ALTER TABLE app.users ADD COLUMN email VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'email address';
   ```
2. **Pass `--schema`** — applies to all unqualified table references in the batch:
   ```bash
   deltascope audit \
     --sql "ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'email address'" \
     --host 127.0.0.1 --port 3306 --user deltascope --ask-password \
     --schema app
   ```
3. **Reduce schema visibility** — connect with a user that can only see the target schema.
4. **Check your `--schema` spelling** — a typo causes DeltaScope to fall back to ambiguous resolution.

### "table does not exist in schema"

Full message shape:

```text
Error: table `orders` does not exist in schema `app`
exit code: 2
```

**Why it happens:** The table was not found in the resolved schema. Common causes:

- You are connected to the wrong database instance (staging vs. production).
- The schema name is wrong — check `--schema` or the qualifier in your SQL.
- The table has not been created yet (the migration that creates it is in the same batch, but it runs before the snapshot is taken).
- The user lacks `SELECT ON information_schema.*` — the table appears to not exist because DeltaScope cannot read its metadata.

**Fix:** Verify the schema name and table name against the live instance:

```bash
mysql -h 127.0.0.1 -u deltascope -p -e "SHOW TABLES IN app LIKE 'orders';"
```

### Cannot Connect

Checklist:

- `--host` and `--port` point to the correct instance.
- `--user` is spelled correctly (case-sensitive on some systems).
- Password method: use `--ask-password` for interactive use; use `--password-env VAR_NAME` or `--password-file /path/to/file` for scripted use. Avoid plaintext `--password` in production and never hardcode passwords in shell commands.
- The MySQL/TiDB port (default `3306`) is open and reachable from the machine running DeltaScope.
- The user account is not restricted to a specific host (check `mysql.user.Host`).

#### Socket vs TCP

| Method | When to use | Example |
|--------|------------|---------|
| **Unix socket** | Database is on the same machine; socket path is known | `--socket /var/run/mysqld/mysqld.sock` |
| **TCP** | Database is on a different host, or in a container/VM | `--host 127.0.0.1 --port 3306` |

Unix socket example:

```bash
deltascope audit \
  --sql "ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'email address'" \
  --socket /var/run/mysqld/mysqld.sock \
  --user deltascope \
  --ask-password \
  --schema app
```

TCP example:

```bash
deltascope audit \
  --sql "ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'email address'" \
  --host 127.0.0.1 \
  --port 3306 \
  --user deltascope \
  --ask-password \
  --schema app
```

If `--socket` and `--host` are both supplied, `--socket` takes precedence.

### Dialect Conflict

```text
Error: dialect conflict: --dialect mysql specified but connected instance is TiDB
exit code: 2
```

**Fix:** Omit `--dialect` and let auto-detection handle it. DeltaScope queries `tidb_version()` at connection time — if the function returns a result, the dialect is set to `tidb`; otherwise MySQL is assumed. The resolved dialect is recorded in `context.dialect_source: "detected"` in JSON output.

Only pass `--dialect` in offline mode when you want to override the default without a connection.

### Output Looks Different From Offline

This is expected. Metadata-aware mode activates additional rules that require live schema context. For example, a column-existence check only fires when DeltaScope can read the current column list.

Confirm metadata-aware mode is active by checking the `context` field in JSON output:

```json
{
  "context": {
    "mode": "metadata-aware",
    "dialect": "mysql",
    "dialect_source": "detected",
    "schema": "app",
    "schema_source": "inferred"
  }
}
```

If `context.mode` is not `"metadata-aware"`, connection flags were not supplied or the connection failed silently. Check `--host`/`--socket` and verify credentials.

### Metadata Rules Not Firing

Metadata-backed rules (those with `Metadata: true` in `deltascope rules show` output) are no-ops in offline mode. If you expect a metadata rule to fire but it does not:

1. Confirm the audit is running in metadata-aware mode (see above — look for `"mode": "metadata-aware"` in JSON output).
2. Confirm the rule is enabled in your policy (`deltascope config show-default` or your `deltascope.yaml`).
3. Confirm the connected user has `SELECT ON information_schema.*`.
4. Confirm the target table exists in the resolved schema — if DeltaScope cannot find the table, metadata rules cannot evaluate against it.

## TiDB-Specific Notes

- **`innodb_adaptive_hash_index`**: Always treated as inactive for TiDB targets. Rules that depend on this variable behave as if it is off.
- **`ddl.alter.merge.tidb.require`**: Disabled by default in the shipped policy (`required: false`). Enable it in your config if your TiDB version supports online DDL merge.
- **`tidb_version()` detection**: Runs automatically at connection time. The result is recorded in `context.dialect_source: "detected"`. You do not need to pass `--dialect tidb`.
- **`performance_schema.global_variables`**: DeltaScope falls back gracefully when this view is unavailable on TiDB. Instance facts that depend on it may be absent, but the audit proceeds normally.

## Verifying Metadata Mode Is Active

Check the JSON output for the `context` field:

```bash
deltascope audit \
  --sql "SELECT 1" \
  --host 127.0.0.1 --port 3306 --user deltascope --ask-password --schema app \
  --format json --quiet \
  | jq '.context'
```

Expected output when metadata mode is active:

```json
{
  "mode": "metadata-aware",
  "dialect": "mysql",
  "dialect_source": "detected",
  "schema": "app",
  "schema_source": "flag"
}
```

If the `context` field is absent, or `mode` is not `"metadata-aware"`, no connection flags were supplied (or the connection failed and DeltaScope fell back to offline mode). Review your flags and credentials.

For the conceptual model of metadata-aware mode, see [../concept/metadata-aware-mode.md](../concept/metadata-aware-mode.md).
