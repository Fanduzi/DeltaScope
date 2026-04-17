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

#### PostgreSQL Generated/Identity Rule Coverage (`v0.36.0`)

Starting with `v0.36.0`, the PostgreSQL generated/identity state-transition forms supported in v0.35.0 now produce explicit `rule_id` findings. When reviewing migrations containing these forms, DeltaScope returns standard audit results with explicit rule findings:

| Form | Rule ID |
|------|---------|
| `ALTER TABLE ... ALTER COLUMN ... DROP EXPRESSION` | `ddl.alter.drop_expression.forbid` |
| `ALTER TABLE ... ALTER COLUMN ... SET GENERATED ALWAYS` | `ddl.alter.set_generated.forbid` |
| `ALTER TABLE ... ALTER COLUMN ... SET GENERATED BY DEFAULT` | `ddl.alter.set_generated.forbid` |
| `ALTER TABLE ... ALTER COLUMN ... DROP IDENTITY` | `ddl.alter.drop_identity.forbid` |

This is rule coverage for already-supported forms — not parser support widening, not generated expression evaluation, and not complete PostgreSQL sequence semantics. The recommended migration review workflow is unchanged: split the migration, audit supported statements with DeltaScope, and review unsupported ones manually.

#### PostgreSQL Generated/Identity State-Transition Support (`v0.35.0`)

Starting with `v0.35.0`, PostgreSQL state-transition forms for generated and identity columns are now processed through the normal supported audit path. When reviewing migrations containing these forms, DeltaScope returns standard audit results with findings where applicable:

- `ALTER TABLE ... ALTER COLUMN ... DROP EXPRESSION` — audited as a normal DDL statement.
- `ALTER TABLE ... ALTER COLUMN ... SET GENERATED ALWAYS` — audited as a normal DDL statement.
- `ALTER TABLE ... ALTER COLUMN ... SET GENERATED BY DEFAULT` — audited as a normal DDL statement.
- `ALTER TABLE ... ALTER COLUMN ... DROP IDENTITY` — audited as a normal DDL statement.

The normalized contract is: `drop_expression`, `set_generated` with `generated_when` (`"a"` / `"d"`), `drop_identity`. No new rules were added.

#### PostgreSQL Generated/Identity Narrow Support (`v0.34.0`)

Starting with `v0.34.0`, narrow PostgreSQL generated/identity definition forms are processed through the normal supported audit path. Shared facts from `v0.33.0` (`generated_when`, `is_identity`, `identity_options`) continue flowing through the supported path. No new rules were added.

#### PostgreSQL Generated/Identity Fact Preservation + Unsupported Metadata (`v0.33.0`)

Starting with `v0.33.0`, unsupported generated/identity outcomes in migration review carry structured metadata. When DeltaScope encounters `GENERATED ALWAYS AS (...) STORED` or `GENERATED ... AS IDENTITY` in a `CREATE TABLE` or `ALTER TABLE ADD COLUMN`, the unsupported result now includes:

| Metadata Key | Type | Example |
|-------------|------|---------|
| `column` | string | `"id"` |
| `generated_when` | string | `"a"` or `"d"` |
| `is_identity` | bool | `true` for identity columns |
| `identity_options` | object | `{"start": 10, "increment": 5, "cycle": true}` |

This metadata helps migration reviewers identify which columns are generated/identity and what parameters they use — without DeltaScope needing to support or evaluate the expressions.

**Deferred**: `GeneratedExpression` (the computed expression for `GENERATED ALWAYS AS (...) STORED`) is not preserved. There is no stable expression renderer in the current `pg_query_go` dependency. Expression text remains unavailable until a future milestone provides a deparse path.

The recommended migration review workflow is unchanged: split the migration, audit supported statements with DeltaScope, and review unsupported ones manually. The metadata now provides additional context for the manual review step.

#### PostgreSQL Boundary Support-Readiness Gate (`v0.32.0`)

`v0.32.0` is the **PostgreSQL Boundary Support-Readiness Gate** — a decision milestone, not a feature release. No new migration review behavior was added. Characterization tests document stable AST facts about generated and identity columns; a readiness report recommends `v0.33.0` as a narrow fact-preservation pack. For migration reviewers, nothing changes: existing unsupported boundaries remain in place, and the recommended workflow (split migration, audit supported statements, review unsupported ones manually) is unchanged.

#### PostgreSQL ALTER TABLE GENERATED Follow-up Pack (`v0.31.0`)

Starting with `v0.31.0`, additional PostgreSQL generated/identity `ALTER TABLE` forms are explicitly surfaced as unsupported boundaries, closing the adjacent gap left by `v0.30.0`. When encountered in migration review, DeltaScope returns an `unsupported` result with the same stable feature tags used by the `v0.26.0` and `v0.30.0` boundary work.

| Feature | Extractor Tag |
|---------|---------------|
| Drop expression (`ALTER COLUMN ... DROP EXPRESSION`) | `generated_column` |
| Set generated (`ALTER COLUMN ... SET GENERATED ...`) | `generated_as_identity` |
| Drop identity (`ALTER COLUMN ... DROP IDENTITY`) | `generated_as_identity` |

When reviewing migrations that use these features, the recommended action remains the same: split the migration, audit the supported statements with DeltaScope, and review the unsupported ones manually.

Keep the scope narrow:
- This is boundary tightening, not generated-column support, identity-column support, or complete PostgreSQL `ALTER TABLE` support.
- Corpus, service, and CLI / HTTP / MCP / `pkg/deltascope` parity lock the same explicit unsupported contract.

#### PostgreSQL ALTER TABLE GENERATED Boundary Pack (`v0.30.0`)

Starting with `v0.30.0`, PostgreSQL `ALTER TABLE ... ADD COLUMN` forms that carry generated stored or identity semantics are explicitly surfaced as unsupported boundaries. When encountered in migration review, DeltaScope returns an `unsupported` result rather than silently accepting or partially handling them.

| Feature | Extractor Tag |
|---------|---------------|
| Generated stored add-column (`GENERATED ALWAYS AS (...) STORED`) | `generated_column` |
| Identity add-column (`GENERATED ALWAYS AS IDENTITY`) | `generated_as_identity` |

When reviewing migrations that use these features, the recommended action remains the same: split the migration, audit the supported statements with DeltaScope, and review the unsupported ones manually.

Keep the scope narrow:
- This is boundary tightening, not generated-column support, identity-column support, or broad PostgreSQL `ALTER TABLE` support.
- Corpus, service, and CLI / HTTP / MCP / `pkg/deltascope` parity lock the same explicit unsupported contract.
- Adjacent `DROP EXPRESSION`, `SET GENERATED`, and `DROP IDENTITY` forms now receive explicit unsupported mappings in `v0.31.0`.

#### Unsupported Boundary Tightening (`v0.26.0`)

Starting with `v0.26.0`, the following PostgreSQL `CREATE TABLE` forms are explicitly rejected as unsupported boundaries by the extractor. When encountered in migration review, DeltaScope returns an `unsupported` result rather than silently accepting or partially handling them:

| Feature | Extractor Tag |
|---------|---------------|
| Identity columns (`GENERATED ... AS IDENTITY`) | `generated_as_identity` |
| Generated stored columns (`GENERATED ALWAYS AS ... STORED`) | `generated_column` |
| Exclusion constraints (`EXCLUDE USING`) | `exclusion_constraint` |
| Partitioned tables (`PARTITION BY`) | `partitioning` |

When reviewing migrations that use these features, the recommended action remains the same: split the migration, audit the supported statements with DeltaScope, and review the unsupported ones manually. This is a boundary tightening, not a support expansion.

#### Schema-Qualified Reference Semantics (`v0.27.0`)

Starting with `v0.27.0`, the PostgreSQL extractor preserves schema-qualified referenced-object facts in the shared contract. For migrations using `REFERENCES public.users(id)`, the schema (`"public"`) is now preserved as `ReferencedSchema` alongside the existing `ReferencedTable` (`"users"`).

- This is additive semantic preservation — no new rules, no new CLI flags.
- No schema-aware rule decisions are made based on `ReferencedSchema` yet.
- Starting with `v0.28.0`, FK forbid finding metadata now exposes these referenced-object fields directly.

#### Referenced-Object Metadata Surface (`v0.28.0`)

Starting with `v0.28.0`, DeltaScope review outputs can surface PostgreSQL referenced-object facts directly in FK forbid findings. When auditing a migration containing a foreign key with schema-qualified references (e.g., `REFERENCES public.users(id)`), the FK forbid finding metadata now includes `referenced_schema`, `referenced_table`, and `referenced_columns`.

Example: audit a PostgreSQL migration with a named foreign key referencing another schema:

```bash
deltascope audit \
  --dialect postgresql \
  --sql "create table orders (user_id bigint, constraint fk_orders_user foreign key (user_id) references public.users(id));"
```

The FK forbid finding now shows which table and schema the constraint references, making it easier to assess cross-schema dependencies during migration review. This is an additive metadata widening — no new rules, no new CLI flags, and no schema-aware FK policy decisions.

#### Schema-Aware FK Policy Pack (`v0.29.0`)

Starting with `v0.29.0`, DeltaScope adds one narrow schema-aware FK policy step for PostgreSQL: explicit cross-schema foreign keys now emit the notice-level rule `ddl.pg.table.foreign_key.cross_schema.advisory`.

- `CREATE TABLE public.orders (...) REFERENCES auth.users(id)` now produces the existing `ddl.table.foreign_key.forbid` blocker plus the extra notice-level advisory.
- `CREATE TABLE public.orders (...) REFERENCES public.users(id)` stays on the existing FK forbid path only.
- `CREATE TABLE public.orders (...) REFERENCES users(id)` also stays on the existing FK forbid path only, because the referenced schema remains unknown.

This is intentionally narrow:

- DeltaScope does not infer `public`.
- DeltaScope does not model PostgreSQL `search_path`.
- This is not a cross-schema validation workflow and not a broad PostgreSQL FK implementation.
