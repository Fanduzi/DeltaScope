# Review DDL Before Migration

Gate migration SQL before rollout by running DeltaScope as a pre-merge or pre-apply check. This catches policy violations — missing comments, bad column defaults, risky drops — before the migration tool ever touches the database.

## Basic Usage

Suppose your migration file contains:

```sql
CREATE TABLE users (
  id   BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  name VARCHAR(255) NOT NULL COMMENT 'user name',
  PRIMARY KEY (id)
) COMMENT='user table';
```

```bash
deltascope audit --config ./deltascope.yaml --file ./migrations/20260322.sql
```

Expected output:

```text
# DeltaScope Audit Result

Verdict: `review`

- Statements: 1
- Blockers: 0
- Warnings: 1
- Notices: 0

## Result Explanation

Audit produced 1 finding(s) across 1 statement(s)
- column `id` must have a comment

## Statement 1

- Kind: `ddl`
- SQL: `CREATE TABLE users ( id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT, name VARCHAR(255) NOT NULL COMMENT 'user name', PRIMARY KEY (id) ) COMMENT='user table'`

### Explanation

Statement 1 has 1 finding(s)
- column `id` must have a comment

### Findings

- [warning] `ddl.column.comment.require`: column `id` must have a comment
  Why: The statement is missing a clause, option, or object that the shipped policy requires.
  Risk: Ignoring this rule can lead to schema changes that do not meet governance or review expectations.
  Suggestion: Add a COMMENT clause to column `id`
  Statement kind: `ddl`
```


## Multi-Statement Migration Files

Real migration files often contain several statements. DeltaScope audits all of them and reports findings per statement.

Example migration file (`./migrations/20260323.sql`):

```sql
CREATE TABLE products (
  id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  name       VARCHAR(255) NOT NULL,
  price      DECIMAL(10,2) NOT NULL DEFAULT 0.00,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'created time',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'updated time',
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='product catalog';

ALTER TABLE orders ADD COLUMN discount DECIMAL(10,2);

DELETE FROM audit_log WHERE created_at < '2020-01-01';
```

```bash
deltascope audit --file ./migrations/20260323.sql --format json --fail-on blocker
```

```json
{
  "verdict": "review",
  "summary": { "statements": 3, "blockers": 0, "warnings": 4, "notices": 0 },
  "explanation": {
    "summary": "Audit produced 4 finding(s) across 3 statement(s)",
    "reasons": [
      "column `id` must have a comment",
      "column `name` must have a comment",
      "column `price` must have a comment",
      "column `discount` must have a comment"
    ]
  },
  "statements": [
    {
      "index": 0,
      "kind": "ddl",
      "raw_sql": "CREATE TABLE products (...)",
      "explanation": {
        "summary": "Statement 1 has 3 finding(s)",
        "reasons": [
          "column `id` must have a comment",
          "column `name` must have a comment",
          "column `price` must have a comment"
        ]
      },
      "findings": [
        {
          "rule_id": "ddl.column.comment.require",
          "level": "warning",
          "message": "column `id` must have a comment",
          "suggestion": "Add a COMMENT clause to column `id`",
          "statement_kind": "ddl",
          "location": { "line": 2, "column": 3 }
        },
        {
          "rule_id": "ddl.column.comment.require",
          "level": "warning",
          "message": "column `name` must have a comment",
          "suggestion": "Add a COMMENT clause to column `name`",
          "statement_kind": "ddl",
          "location": { "line": 3, "column": 3 }
        },
        {
          "rule_id": "ddl.column.comment.require",
          "level": "warning",
          "message": "column `price` must have a comment",
          "suggestion": "Add a COMMENT clause to column `price`",
          "statement_kind": "ddl",
          "location": { "line": 4, "column": 3 }
        }
      ]
    },
    {
      "index": 1,
      "kind": "ddl",
      "raw_sql": "ALTER TABLE orders ADD COLUMN discount DECIMAL(10,2)",
      "explanation": {
        "summary": "Statement 2 has 1 finding(s)",
        "reasons": ["column `discount` must have a comment"]
      },
      "findings": [
        {
          "rule_id": "ddl.column.comment.require",
          "level": "warning",
          "message": "column `discount` must have a comment",
          "suggestion": "Add a COMMENT clause to column `discount`",
          "statement_kind": "ddl",
          "location": { "line": 1, "column": 38 }
        }
      ]
    },
    {
      "index": 2,
      "kind": "dml",
      "raw_sql": "DELETE FROM audit_log WHERE created_at < '2020-01-01'"
    }
  ]
}
```

Exit code `0` — in this example all findings are `warning`, so `--fail-on blocker` does not fail the audit. Use `--fail-on warning` to block on warnings too.

## Integration Patterns

### With golang-migrate

Audit the `.up.sql` file before running `migrate up`. If DeltaScope rejects the file, the migration is never applied:

```bash
MIGRATION_FILE="./migrations/000001_create_users.up.sql"

# Step 1: Audit
deltascope audit \
  --file "$MIGRATION_FILE" \
  --config ./deltascope.yaml \
  --fail-on blocker \
|| { echo "DeltaScope rejected $MIGRATION_FILE — migration aborted."; exit 1; }

# Step 2: Apply only if audit passed
migrate -database "$DATABASE_URL" -path ./migrations up
```

For CI, wrap this in a script so the pipeline fails clearly:

```bash
#!/usr/bin/env bash
set -euo pipefail

for f in ./migrations/*.up.sql; do
  echo "==> Auditing $f"
  deltascope audit --file "$f" --fail-on blocker
done

echo "All migrations passed audit. Running migrate up..."
migrate -database "$DATABASE_URL" -path ./migrations up
```

### With flyway

Audit all versioned migration scripts in order before letting Flyway apply them:

```bash
#!/usr/bin/env bash
set -euo pipefail

for f in ./sql/migrations/V*.sql; do
  echo "==> Auditing $f"
  deltascope audit --file "$f" --fail-on blocker || exit 1
done

echo "All migrations passed audit. Running flyway migrate..."
flyway migrate
```

In CI (GitHub Actions):

```yaml
- name: Audit Flyway migrations
  run: |
    for f in ./sql/migrations/V*.sql; do
      echo "==> Auditing $f"
      deltascope audit --file "$f" --format json --fail-on blocker || exit 1
    done

- name: Apply Flyway migrations
  run: flyway migrate
  env:
    FLYWAY_URL: ${{ secrets.FLYWAY_URL }}
    FLYWAY_USER: ${{ secrets.FLYWAY_USER }}
    FLYWAY_PASSWORD: ${{ secrets.FLYWAY_PASSWORD }}
```

## Metadata-Aware Variant

Add connection flags when migration safety depends on current schema state — for example, to detect columns that already exist or tables that do not yet exist.

```bash
deltascope audit \
  --config ./deltascope.yaml \
  --file ./migrations/20260322.sql \
  --host 127.0.0.1 \
  --port 3306 \
  --user deltascope \
  --ask-password \
  --schema app \
  --format json \
  --fail-on blocker
```

This is especially useful for:

- `ALTER TABLE` compatibility checks (column already exists, index already exists)
- Table existence checks before `DROP` or `TRUNCATE`
- Table-option comparisons against the current schema state

The JSON output includes a `context` field confirming metadata-aware mode:

```json
{
  "verdict": "pass",
  "summary": { "statements": 1, "blockers": 0, "warnings": 0, "notices": 0 },
  "context": {
    "mode": "metadata-aware",
    "dialect": "mysql",
    "dialect_source": "detected",
    "schema": "app",
    "schema_source": "flag"
  },
  "statements": [
    { "index": 0, "kind": "ddl", "raw_sql": "..." }
  ]
}
```

## JSON Output for CI

Parse the `verdict` field to drive CI logic without relying on exit codes alone. This is useful when you want to extract and display specific findings in the CI log:

```bash
RESULT=$(deltascope audit --file ./migrations/latest.sql --format json)
VERDICT=$(echo "$RESULT" | jq -r '.verdict')

if [ "$VERDICT" = "reject" ]; then
  echo "Migration blocked: verdict=$VERDICT"
  echo "Blocker findings:"
  echo "$RESULT" | jq -r '
    .statements[] | .findings[]?
    | select(.level == "blocker")
    | "  [\(.rule_id)] \(.message)"
  '
  exit 1
fi

if [ "$VERDICT" = "review" ]; then
  echo "Migration has warnings — review before deploying:"
  echo "$RESULT" | jq -r '
    .statements[] | .findings[]?
    | "  [\(.level)] [\(.rule_id)] \(.message)"
  '
fi

echo "Audit result: $VERDICT"
```

## Recommended CI Pattern

- [ ] Keep a checked-in policy file (`deltascope.yaml`) so every developer and CI run uses the same rules.
- [ ] Run the audit step **before** the migration step — never after.
- [ ] Use `--fail-on blocker` as the default gate. Tighten to `--fail-on warning` for stricter teams.
- [ ] Use `--format json` in CI so findings appear as structured data in logs.
- [ ] Keep at least one migration fixture in the repository so developers can reproduce the same audit locally.
- [ ] For multi-file migration directories, loop over files in migration order (alphabetical or version-sorted) so CI runs are deterministic. Cross-statement `merge-alter` findings are only detected when related statements are audited together in a single DeltaScope run.

## PostgreSQL Migration Safety

When auditing PostgreSQL migrations, DeltaScope applies an additional set of migration-safety rules that guard against common patterns causing table rewrites, long-held locks, or production incidents:

| Rule ID | What It Catches | Safe Pattern |
|---------|----------------|---------------|
| `ddl.pg.create_index.concurrently.require` | `CREATE INDEX` without `CONCURRENTLY` | Use `CREATE INDEX CONCURRENTLY` |
| `ddl.pg.alter.add_column.non_null_default.rewrite.warn` | `ADD COLUMN … NOT NULL DEFAULT` with volatile default | Add nullable first, backfill, then add NOT NULL |
| `ddl.pg.alter.add_check.not_valid.require` | `ADD CHECK (…)` without `NOT VALID` | Use `ADD CHECK (…) NOT VALID` |
| `ddl.pg.alter.set_data_type.rewrite.warn` | `ALTER COLUMN … TYPE …` may rewrite table | Use add-new-column + backfill + drop-old pattern |

Example: audit a PostgreSQL migration file with CI-native annotations:

```bash
deltascope audit --dialect postgresql --file ./migrations/20260409_add_index.sql --format github-actions
```

Generate SARIF output for GitHub Code Scanning:

```bash
deltascope audit --file ./migrations.sql --dialect postgresql --format sarif > deltascope.sarif
```

### Detecting Dialect Mismatches During Migration Review

When auditing PostgreSQL migrations without `--dialect postgresql`, DeltaScope runs in MySQL mode by default and may emit a `dialect.postgresql.syntax.detected.notice` advisory. This notice means the SQL contains PostgreSQL-specific syntax but DeltaScope did **not** auto-switch dialect — the audit ran with the MySQL/TiDB parser, which may produce misleading or incomplete findings.

How to recognize this in your migration review workflow:

```bash
# Check if the output includes a PostgreSQL syntax notice
deltascope audit --file ./migrations.sql --format json | \
  jq '.global_findings[] | select(.rule_id == "dialect.postgresql.syntax.detected.notice")'
```

If the notice fires during migration review, either:
- Re-run with `--dialect postgresql` to get accurate findings for PostgreSQL SQL, or
- Confirm the SQL is MySQL-compatible and ignore the notice.

The markdown output renders an `## Audit Context` section when this notice triggers, and JSON output always includes a top-level `context` object with `mode`, `dialect`, and `dialect_source`.

### Understanding Capability-Boundary Errors in Migration Review

When using `--dialect postgresql` with a PG-capable DeltaScope binary, encountering unsupported PostgreSQL surfaces (e.g., complex DDL that the parser cannot yet handle) returns a typed `PostgreSQLCapabilityBoundaryError`. In CI this appears as exit code `2`.

Distinguish it from a real parse failure by checking the error message — capability-boundary errors clearly state what surface was requested and what the current build supports. When this happens during migration review, the recommended action is to split the migration: audit the supported statements with DeltaScope and review the unsupported ones manually.

### Using Trust Context and Rule Summary to Assess Audit Confidence

All output formats report audit context and rule summary information that helps you judge how much of the audit is trustworthy:

- **Markdown**: `## Audit Context` section shows the dialect and trust note; `## Rule Summary` and `## Skipped Rules` sections show which rules ran.
- **JSON**: `context` object shows `mode`, `dialect`, and `dialect_source`; `rule_summary` object shows loaded, applicable, and skipped counts.
- **Quiet**: `[context]` line shows mode and dialect at the end of output.

When reviewing migrations, check the skipped-rules count — a high number of skipped rules (especially for your target dialect) may indicate the audit is running under the wrong dialect or that certain rule families are not applicable. This helps you decide whether the current audit result is sufficient or whether additional manual review is needed.

### PostgreSQL Coverage Milestones (v0.21.0 / v0.23.0 / v0.24.0)

`v0.21.0` expands DeltaScope's ability to audit more of the standard PostgreSQL phased migration sequence. `v0.23.0` expands common PostgreSQL `CREATE TABLE` coverage for richer constraint shapes. `v0.24.0` deepens the foreign-key semantics of those create-table shapes by preserving parser-owned `ReferencedTable` and `ReferencedColumns`. Together they let migration review cover more real-world PostgreSQL DDL with progressively richer semantics, without overstating support.

#### Phased Migration Follow-Up Actions (`v0.21.0`)

| Migration Phase | DDL Example | Status |
|----------------|-------------|--------|
| Set column default | `ALTER TABLE users ALTER COLUMN status SET DEFAULT 'active'` | Supported, auditable, shared alter rules apply |
| Drop column default | `ALTER TABLE users ALTER COLUMN status DROP DEFAULT` | Supported, auditable, shared alter rules apply |
| Enforce NOT NULL | `ALTER TABLE users ALTER COLUMN status SET NOT NULL` | Supported, auditable, shared alter rules apply |
| Relax NOT NULL | `ALTER TABLE users ALTER COLUMN status DROP NOT NULL` | Supported, auditable, shared alter rules apply |
| Validate constraint | `ALTER TABLE users VALIDATE CONSTRAINT chk_amount` | Supported, auditable; no dedicated rule, produces clean audit unless other findings apply |
| Drop constraint | `ALTER TABLE orders DROP CONSTRAINT chk_amount` | Supported, auditable; primary-key drops map to `ddl.alter.drop_primary_key` rules when metadata is available |

#### Constraint-Rich `CREATE TABLE` Coverage (`v0.23.0`)

| `CREATE TABLE` shape | Example | Status |
|----------------------|---------|--------|
| Table-level named `CHECK` | `CONSTRAINT chk_amount CHECK (amount >= 0)` | Supported, auditable; existing naming governance can apply when configured |
| Column-level inline `CHECK` | `amount numeric check (amount >= 0)` | Supported, auditable; no dedicated new rule family |
| Table-level named `UNIQUE` | `CONSTRAINT uniq_orders_user UNIQUE (user_id)` | Supported, auditable; existing naming governance can apply when configured |
| Column-level inline `UNIQUE` | `email text unique` | Supported, auditable; shared index rules can consume normalized index facts |
| Table-level named `FOREIGN KEY` | `CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES users(id)` | Supported, auditable; naming governance matters only when policy allows foreign keys. `v0.24.0` preserves `ReferencedTable`/`ReferencedColumns` |
| Column-level inline `REFERENCES` | `user_id bigint references users(id)` | Supported, auditable; parser-owned shared facts only, no invented metadata semantics. `v0.24.0` preserves `ReferencedTable`/`ReferencedColumns` |

Example: audit a constraint-rich PostgreSQL create-table statement:

```bash
deltascope audit \
  --dialect postgresql \
  --sql "create table orders (id bigint primary key, user_id bigint references users(id), amount numeric not null check (amount >= 0), constraint uniq_orders_user unique (user_id), constraint chk_orders_amount check (amount >= 0));"
```

Example: audit a phased migration follow-up step:

```bash
deltascope audit \
  --dialect postgresql \
  --sql "alter table users alter column status set default 'active';"
```

Example: audit a constraint lifecycle step:

```bash
deltascope audit \
  --dialect postgresql \
  --sql "alter table users validate constraint chk_amount;"
```

Important notes:
- `DROP CONSTRAINT` targeting a primary key (e.g., `DROP CONSTRAINT users_pkey`) triggers existing primary-key rules only in metadata-aware mode. In offline mode, it passes through as a normal alter action.
- `VALIDATE CONSTRAINT` does not have a dedicated rule. It is supported and auditable but produces a clean audit result unless other findings apply to the same statement.
- `v0.23.0` should be described as broader PostgreSQL `CREATE TABLE` coverage, not full PostgreSQL DDL support.
- `v0.24.0` deepens `v0.23.0` foreign-key semantics — `ReferencedTable` and `ReferencedColumns` are parser-owned structural facts, not metadata truth.
- Inline `REFERENCES` should be described narrowly as parser-owned shared facts, not as a new metadata-aware foreign-key contract.

#### Unsupported Boundary Tightening (`v0.26.0`)

Starting with `v0.26.0`, the following PostgreSQL `CREATE TABLE` forms are explicitly rejected as unsupported boundaries by the extractor. When encountered in migration review, DeltaScope returns an `unsupported` result rather than silently accepting or partially handling them:

| Feature | Extractor Tag |
|---------|---------------|
| Identity columns (`GENERATED ... AS IDENTITY`) | `generated_as_identity` |
| Generated stored columns (`GENERATED ALWAYS AS ... STORED`) | `generated_column` |
| Exclusion constraints (`EXCLUDE USING`) | `exclusion_constraint` |
| Partitioned tables (`PARTITION BY`) | `partitioning` |

When reviewing migrations that use these features, the recommended action remains the same: split the migration, audit the supported statements with DeltaScope, and review the unsupported ones manually. This is a boundary tightening, not a support expansion.
