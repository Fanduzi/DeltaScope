# Metadata-Aware Mode

## Overview

Metadata-aware mode adds live instance and schema facts to the same audit flow used in offline mode. It does **not** replace offline evaluation — it enriches it. All offline-eligible rules continue to run normally. Rules that require live facts activate automatically when the relevant facts are present and no-op gracefully when they are not.

---

## Activating Metadata-Aware Mode

Metadata-aware mode is activated by supplying any database connection flag to `deltascope audit`. You do not need to pass a dedicated mode flag — the presence of any connection parameter is sufficient.

Connection flags that activate the mode:

| Flag | Purpose |
|---|---|
| `--host` | MySQL/TiDB host address |
| `--port` | Port number (default: 3306) |
| `--user` | Database user |
| `--password` | Password (passed on command line) |
| `--ask-password` | Prompt for password interactively |
| `--socket` | Unix socket path |
| `--schema` | Default schema for unqualified table names |

Any single connection flag is sufficient to activate the mode. For example:

```bash
# Minimal: host + user (password prompted interactively)
deltascope audit --host db.example.com --user readonly --ask-password migration.sql

# With explicit schema and socket
deltascope audit --socket /var/run/mysqld/mysqld.sock --user auditor --schema app migration.sql
```

---

## Dialect Auto-Detection

When connected to a live instance, DeltaScope automatically detects the dialect by querying the `tidb_version()` system variable:

- **Success** (variable exists and returns a value): dialect is detected as **TiDB**
- **Failure** (variable not found or returns an error): dialect is detected as **MySQL**

If you pass `--dialect` explicitly and it conflicts with the auto-detected dialect, the audit fails with exit code 2. To avoid conflicts, omit `--dialect` when connecting to a live instance and let auto-detection handle it.

---

## Schema Inference

When a SQL statement references a table without a schema qualifier (e.g., `ALTER TABLE users ...`), DeltaScope resolves the target schema using the following four-step logic. The first step that matches wins:

1. **SQL-qualified name**: if the SQL already qualifies the table name (e.g., `ALTER TABLE app.users ...`), that schema is used directly.
2. **`--schema` flag**: if `--schema` is provided on the command line, that value is used.
3. **Unique match**: if the table exists in exactly one schema visible to the connected user, that schema is inferred automatically.
4. **Ambiguous match**: if the table exists in more than one schema, the audit fails with:
   ```
   schema inference for table "users" is ambiguous; pass --schema to specify
   ```

If the target table is not found in any schema (and the rule requires an existing table), the audit fails with a similar message asking you to pass `--schema` or confirm the table name.

---

## What Is Loaded

### Instance Facts

Instance facts describe the MySQL or TiDB instance configuration and are loaded once per audit session. They are attached to every statement in the batch.

| Fact | Description |
|---|---|
| Version string | MySQL or TiDB version reported by the instance |
| `character_set_database` | Default character set for the connected database |
| `innodb_large_prefix` | Whether large index prefixes are enabled (`ON`/`OFF`) |
| `innodb_default_row_format` | Default InnoDB row format (`DYNAMIC`, `COMPACT`, etc.) |
| `innodb_adaptive_hash_index` | Whether adaptive hash index is enabled (`ON`/`OFF`) |

### Table Snapshot

A table snapshot is the current definition of the target table, loaded from `information_schema`. It is attached to statements that reference a specific table.

A snapshot includes:

- **Column definitions**: name, data type, nullability, default value, comment
- **Index definitions**: index name, type (BTREE/HASH/FULLTEXT), uniqueness, indexed columns
- **Primary key state**: whether a primary key exists and which columns it covers
- **Table options**: storage engine, character set, row format, table comment, and other CREATE TABLE options

---

## What Metadata-Aware Mode Enables

The following checks become active only when the relevant facts are loaded:

| Check | Requires |
|---|---|
| Column/index/table existence checks (e.g., the column being added does not already exist) | Table snapshot |
| ALTER TABLE type compatibility (new type must be compatible with existing column type) | Table snapshot |
| Row-size estimation (projected row size must not exceed InnoDB row size limits) | Table snapshot + instance facts |
| Index key-length estimation (index key must fit within instance-defined limits) | Table snapshot + instance facts |
| Drop/truncate row-count caution (warns when `table_rows` in `information_schema` is large) | Table snapshot |
| Adaptive-hash index warning | `innodb_adaptive_hash_index` instance fact |
| Table option compatibility (e.g., charset change against current schema charset) | Table snapshot + instance facts |

---

## What It Does Not Do

- **Does not replace offline rules**: all offline-eligible rules continue to run when metadata is present. Metadata-aware mode is strictly additive.
- **Does not guess schema when ambiguous**: if a table name exists in multiple schemas, the audit fails with a clear error. DeltaScope never silently picks one schema over another.
- **Does not silently skip metadata requirements**: if metadata cannot be loaded for a specific table, the metadata-dependent checks for that table are skipped gracefully. The offline checks still run. No spurious errors are raised due to missing metadata.

---

## Required MySQL Permissions

The database user supplied via `--user` needs the following minimum permissions:

```sql
-- Read schema metadata
GRANT SELECT ON information_schema.* TO 'auditor'@'%';

-- Read InnoDB instance configuration variables
GRANT SELECT ON performance_schema.global_variables TO 'auditor'@'%';
```

No write permissions are required. DeltaScope never modifies the target database.

---

## TiDB Notes

- **Auto-detection**: TiDB is identified via the `tidb_version()` system variable. No manual flag is needed.
- **Adaptive hash index**: `innodb_adaptive_hash_index` is always treated as inactive on TiDB; the corresponding warning rule is suppressed.
- **Merge-alter guidance**: the `ddl.alter.merge.mysql.require` rule targets MySQL DDL conventions. On TiDB, this rule is disabled by default in the shipped policy because TiDB handles concurrent DDL differently.
- **Permissions**: the same `information_schema` grants apply to TiDB. TiDB does not expose `performance_schema.global_variables` in the same way; DeltaScope falls back gracefully when these variables are unavailable.

---

## Output Context

When metadata-aware mode is active, DeltaScope annotates the output with context describing how the connection and schema were resolved.

**JSON output** includes a `context` object at the top level:

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

Field meanings:

| Field | Values | Description |
|---|---|---|
| `mode` | `offline`, `metadata-aware` | Whether metadata enrichment was active |
| `dialect` | `mysql`, `tidb` | Dialect used for evaluation |
| `dialect_source` | `detected`, `explicit` | Whether dialect came from auto-detection or `--dialect` flag |
| `schema` | schema name | The resolved default schema |
| `schema_source` | `flag`, `inferred`, `qualified` | How the schema was determined |

**Markdown output** prepends an `## Audit Context` section with the same information in human-readable form before the findings table.
