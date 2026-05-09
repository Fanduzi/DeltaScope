# Auditing MySQL ALTER TABLE Risks with a CLI (Real Output Included)

## Background

I'm the author of [DeltaScope](https://deltascope.pages.dev), an open-source offline SQL audit tool that supports MySQL, TiDB, and PostgreSQL. This post skips the marketing and shows real SQL inputs with real audit outputs — no fabrications.

## Scenario 1: An Innocent-Looking Migration File

Consider a migration file `migration.sql`:

```sql
-- Add columns and index to users table
ALTER TABLE users ADD COLUMN phone VARCHAR(20);
ALTER TABLE users ADD COLUMN age INT DEFAULT 0;
ALTER TABLE users ADD INDEX idx_phone (phone);

-- Clean up temp data
DELETE FROM temp_data;
```

Audit it:

```bash
deltascope audit --file migration.sql
```

Output:

```text
Verdict: reject

- Statements: 4
- Blockers: 1
- Warnings: 1
- Notices: 0

## Statement 1
- SQL: ALTER TABLE users ADD COLUMN phone VARCHAR(20)
No findings.

## Statement 2
- SQL: ALTER TABLE users ADD COLUMN age INT DEFAULT 0
No findings.

## Statement 3
- SQL: ALTER TABLE users ADD INDEX idx_phone (phone)
No findings.

## Statement 4
- SQL: DELETE FROM temp_data

### Findings
- [blocker] dml.where.require: UPDATE and DELETE statements must include a WHERE clause
  Suggestion: add a WHERE clause that narrows the affected rows

### Impact
- estimated_ratio: 1.0000
- risk_level: high
- confidence: high
- source: shape
- reason_code: missing_where

## Global Findings
- [warning] ddl.alter.merge.mysql.require: multiple ALTER TABLE statements
  target "users" under mysql mode
  alter_count: 3, dialect: mysql
  Suggestion: merge repeated alter statements on the same table into
  a single ALTER TABLE
```

Two issues:

1. `DELETE FROM temp_data` has no WHERE clause — offline mode estimates ratio = 1.0 (full table), flagged as high risk
2. Three ALTER statements target the same table `users` — MySQL recommends merging them

After fixing:

```sql
ALTER TABLE users
  ADD COLUMN phone VARCHAR(20),
  ADD COLUMN age INT DEFAULT 0,
  ADD INDEX idx_phone (phone);

DELETE FROM temp_data WHERE created_at < '2026-01-01';
```

Re-audit:

```text
Verdict: pass
- Statements: 2 | Blockers: 0 | Warnings: 0 | Notices: 0
```

## Scenario 2: Changing Column Type + NULL Constraint

```sql
ALTER TABLE orders MODIFY COLUMN amount VARCHAR(100) NOT NULL;
```

```bash
deltascope audit --sql "ALTER TABLE orders MODIFY COLUMN amount VARCHAR(100) NOT NULL"
```

```text
Verdict: reject

- [blocker] ddl.alter.modify_column.explicit_nullability_change.forbid:
  ALTER TABLE modify column explicitly changes nullability for "amount",
  which this policy forbids
  Suggestion: keep nullability unchanged for "amount" or relax the policy
  intentionally after review
```

This statement changes the column type from INT to VARCHAR and also changes the NULL constraint. Type changes may trigger a full table COPY in InnoDB (depending on the direction), and NULL constraint changes can silently break application logic that depends on the constraint.

The rule ID is `ddl.alter.modify_column.explicit_nullability_change.forbid`, default level blocker. Teams that genuinely need this change can adjust the level or add an approval flow in the config.

## Scenario 3: Dropping NOT NULL

```sql
ALTER TABLE users MODIFY COLUMN email VARCHAR(255) NULL;
```

```bash
deltascope audit --sql "ALTER TABLE users MODIFY COLUMN email VARCHAR(255) NULL"
```

```text
Verdict: reject

- [blocker] ddl.alter.modify_column.explicit_nullability_change.forbid:
  ALTER TABLE modify column explicitly changes nullability for "email",
  which this policy forbids
  Suggestion: keep nullability unchanged for "email" or relax the policy
  intentionally after review
```

Same rule as Scenario 2. After dropping NOT NULL, application code like `if user.Email != ""` silently breaks — NULL is not an empty string. This won't throw errors at deploy time, but behavior changes quietly, making it expensive to debug later.

## Scenario 4: Dropping Primary Key

```sql
ALTER TABLE users DROP PRIMARY KEY;
```

```bash
deltascope audit --sql "ALTER TABLE users DROP PRIMARY KEY"
```

```text
Verdict: reject

- [blocker] ddl.alter.drop_primary_key.forbid:
  ALTER TABLE drop primary key is forbidden for "primary"
  Suggestion: avoid drop primary key in this change or relax the policy intentionally
```

InnoDB uses the primary key as the clustered index. Dropping it forces a full table rebuild. Default policy rejects this outright. If you need to change the primary key scheme, the correct approach is to ADD the new primary key column first, then DROP the old one in a separate step.

## Scenario 5: Sloppy CREATE TABLE

```sql
CREATE TABLE t1 (
  id bigint unsigned NOT NULL AUTO_INCREMENT,
  name varchar(100),
  PRIMARY KEY(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

This SQL executes fine, but the audit finds 8 issues:

```bash
deltascope audit --sql "CREATE TABLE t1 (id bigint unsigned NOT NULL AUTO_INCREMENT, name varchar(100), PRIMARY KEY(id)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4"
```

```text
Verdict: review
- Statements: 1 | Blockers: 0 | Warnings: 8

- [warning] ddl.table.comment.require: table comment is required
- [warning] ddl.table.audit_columns.require: should include a created-time
  audit column with DEFAULT CURRENT_TIMESTAMP
- [warning] ddl.table.audit_columns.require: should include an updated-time
  audit column with DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
- [warning] ddl.column.comment.require: column "id" must include a comment
- [warning] ddl.column.comment.require: column "name" must include a comment
- [warning] ddl.column.default.require: column "id" should define a default value
- [warning] ddl.column.default.require: column "name" should define a default value
- [warning] ddl.column.not_null.require: column "name" should be declared NOT NULL
```

Missing table comment, missing audit columns (created_at / updated_at), missing column comments, missing default values, name allows NULL. A "working" CREATE TABLE has 8 compliance issues — every column added later compounds the debt.

Corrected version:

```sql
CREATE TABLE t1 (
  id bigint unsigned NOT NULL AUTO_INCREMENT COMMENT 'primary key',
  name varchar(100) NOT NULL DEFAULT '' COMMENT 'display name',
  created_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'created at',
  updated_at datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'updated at',
  PRIMARY KEY(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='demo table';
```

## Scenario 6: Three PostgreSQL Migration Pitfalls

PostgreSQL has a different DDL locking model than MySQL. Some risks are PG-specific. Take this migration file:

```sql
ALTER TABLE orders ALTER COLUMN amount TYPE TEXT;
ALTER TABLE orders ALTER COLUMN amount DROP NOT NULL;
CREATE INDEX idx_status ON orders (status);
```

```bash
deltascope audit --dialect postgresql --file pg_migration.sql
```

Output:

```text
Verdict: reject
- Statements: 3 | Blockers: 1 | Warnings: 4

## Statement 1: ALTER TABLE orders ALTER COLUMN amount TYPE TEXT
- [warning] ddl.alter.set_data_type.forbid: ALTER TABLE set data type is
  forbidden for "amount"
- [warning] ddl.pg.alter.set_data_type.rewrite.warn: ALTER COLUMN "amount"
  SET DATA TYPE carries table rewrite risk on PostgreSQL
  Suggestion: Assess table size and lock impact first. For large tables,
  use a phased migration: add a shadow column with the new type, backfill
  in batches, switch application reads, then drop the old column.

## Statement 2: ALTER TABLE orders ALTER COLUMN amount DROP NOT NULL
- [blocker] ddl.alter.drop_not_null.explicit_nullability_change.forbid:
  ALTER TABLE drop not null explicitly changes nullability for "amount"
- [warning] ddl.alter.drop_not_null.forbid: ALTER TABLE drop not null is
  forbidden for "amount"

## Statement 3: CREATE INDEX idx_status ON orders (status)
- [warning] ddl.pg.create_index.concurrently.require: CREATE INDEX "idx_status"
  without CONCURRENTLY can block writes on PostgreSQL
  Suggestion: Use CREATE INDEX CONCURRENTLY to build the index without
  blocking writes. Note that CONCURRENTLY cannot run inside a transaction;
  run it as a standalone migration step.
```

Three issues:

**Statement 1: Type change triggers table rewrite**

`SET DATA TYPE` in PostgreSQL acquires an ACCESS EXCLUSIVE lock (blocks all reads and writes) and rewrites the entire table. [DeltaScope](https://deltascope.pages.dev) suggests a phased migration: add a shadow column with the new type, backfill in batches, switch application reads, then drop the old column. This is a PG-only rule (`ddl.pg.alter.set_data_type.rewrite.warn`) — it won't fire under the MySQL dialect.

**Statement 2: Dropping NOT NULL**

Same risk as MySQL — application code depends on the NOT NULL constraint. But PG syntax uses `ALTER COLUMN ... DROP NOT NULL` instead of MySQL's `MODIFY COLUMN`. [DeltaScope](https://deltascope.pages.dev) recognizes PG syntax and fires the corresponding rule.

**Statement 3: CREATE INDEX without CONCURRENTLY**

This is a PG-specific pitfall. A regular `CREATE INDEX` holds a write-blocking lock for the entire index build. `CREATE INDEX CONCURRENTLY` builds the index without blocking writes, but it can't run inside a transaction. [DeltaScope](https://deltascope.pages.dev) checks for this and reminds you that CONCURRENTLY must be a standalone migration step.

One more PG-specific scenario — adding a CHECK constraint:

```sql
ALTER TABLE orders ADD CONSTRAINT chk_amount CHECK (amount > 0);
```

```bash
deltascope audit --dialect postgresql --sql "ALTER TABLE orders ADD CONSTRAINT chk_amount CHECK (amount > 0)"
```

```text
Verdict: review

- [warning] ddl.pg.alter.add_check.not_valid.require: CHECK constraint
  "chk_amount" without NOT VALID validates all existing rows immediately
  on PostgreSQL
  Suggestion: Use a two-step approach:
  1) ADD CONSTRAINT ... NOT VALID to register the constraint without
     scanning existing rows.
  2) VALIDATE CONSTRAINT in a separate step — it holds only a SHARE
     UPDATE EXCLUSIVE lock.
```

By default, PG scans the entire table to validate the constraint at `ADD CONSTRAINT` time, holding an ACCESS EXCLUSIVE lock on large tables. The correct approach: first register the constraint as `NOT VALID` (no scan), then run `VALIDATE CONSTRAINT` separately (only holds SHARE UPDATE EXCLUSIVE lock, doesn't block reads or writes).

These rules (`ddl.pg.alter.set_data_type.rewrite.warn`, `ddl.pg.create_index.concurrently.require`, `ddl.pg.alter.add_check.not_valid.require`) are exclusive to the PostgreSQL dialect. Switch with `--dialect postgresql` — they won't fire under MySQL or TiDB.

## Scenario 7: Dialect Differences — MySQL vs TiDB

```sql
ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL;
```

MySQL default audit:

```bash
$ deltascope audit --sql "ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL"
```

```text
Verdict: pass
```

TiDB dialect audit:

```bash
$ deltascope audit --dialect tidb --sql "ALTER TABLE users ADD COLUMN email VARCHAR(255) NOT NULL"
```

```text
Verdict: pass
```

Both pass for a single statement. But if a migration file has three ALTER statements on the same table, the MySQL dialect fires a warning (suggesting merge), while the TiDB dialect does not (because TiDB DDL is online — no table locking, no need to merge). This is what `--dialect` is for — different engines have different best practices, and audit rules should follow the engine.

Three dialects are currently supported: `mysql` (default), `tidb`, `postgresql`.

## CI Integration

All of the above checks belong in CI, not in manual review. GitHub Actions config:

```yaml
name: SQL Audit
on:
  pull_request:
    paths:
      - 'migrations/**'

jobs:
  audit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Install DeltaScope
        run: curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/main/install.sh | sh
      - name: Audit
        run: deltascope audit --file ./migrations/ --format github-actions --fail-on warning
```

Three CI output formats:

| Format | Flag | Use case |
|--------|------|----------|
| GitHub Actions | `--format github-actions` | PR annotations |
| GitLab Code Quality | `--format gitlab-codequality` | Code Quality artifact |
| SARIF | `--format sarif` | GitHub Code Scanning |

For more accurate audits using live table structure (e.g., checking for redundant indexes), use metadata-aware mode:

```bash
deltascope audit \
  --sql "ALTER TABLE orders ADD INDEX idx_status (status)" \
  --host 127.0.0.1 --port 3306 --user root --ask-password --schema app
```

This connects to the database to read table statistics but **never executes any DDL or DML** — read-only metadata access.

## JSON Output

For CI scripts that need machine-readable results:

```bash
deltascope audit --sql "ALTER TABLE orders MODIFY COLUMN amount VARCHAR(100) NOT NULL" --format json
```

```json
{
  "verdict": "reject",
  "summary": {
    "statements": 1,
    "blockers": 1,
    "warnings": 0,
    "notices": 0
  },
  "statements": [
    {
      "index": 0,
      "kind": "ddl",
      "raw_sql": "ALTER TABLE orders MODIFY COLUMN amount VARCHAR(100) NOT NULL",
      "findings": [
        {
          "rule_id": "ddl.alter.modify_column.explicit_nullability_change.forbid",
          "level": "blocker",
          "message": "ALTER TABLE modify column explicitly changes nullability for \"amount\"",
          "suggestion": "keep nullability unchanged for \"amount\" or relax the policy intentionally after review",
          "metadata": {
            "action": "modify_column",
            "change_kind": "explicit_nullability_change",
            "column_name": "amount",
            "table": "orders"
          }
        }
      ]
    }
  ],
  "context": {
    "mode": "offline",
    "dialect": "mysql",
    "dialect_source": "default"
  }
}
```

Script logic: `verdict == "reject"` → block, `"review"` → require human acknowledgment, `"pass"` → allow.

## Rule Configuration

151 built-in rules (run `deltascope rules` to list all), configurable via YAML:

```yaml
# deltascope.yaml
rules:
  # Require DBA sign-off for DROP COLUMN
  ddl.alter.drop_column.forbid:
    enabled: true
    level: blocker

  # Enforce idx_ prefix on secondary indexes
  ddl.alter.add_index.secondary.prefix.require:
    enabled: true
    level: warning
    params:
      prefix: idx_

  # Teams that don't need audit columns can disable
  ddl.table.audit_columns.require:
    enabled: false
```

Commit the config to the repo — CI loads it automatically.

---

**Links:**
- Website: https://deltascope.pages.dev
- GitHub: https://github.com/Fanduzi/DeltaScope
- Rule reference: https://github.com/Fanduzi/DeltaScope/blob/main/configs/deltascope.example.yaml
- CI integration docs: https://github.com/Fanduzi/DeltaScope/tree/main/docs/recipe

Install:

```bash
# macOS
brew tap Fanduzi/deltascope && brew install --cask deltascope

# Linux
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/main/install.sh | sh
```
