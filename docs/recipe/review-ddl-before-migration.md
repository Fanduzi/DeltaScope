# Review DDL Before Migration

Gate migration SQL before rollout by running DeltaScope as a pre-merge or pre-apply check.

## Example

Suppose your migration file contains:

```sql
create table users (
  id bigint unsigned not null auto_increment,
  name varchar(255) not null comment 'user name',
  primary key (id)
) comment='user table';
```

```bash
deltascope audit --config ./deltascope.yaml --file ./migrations/20260322.sql
```

Expected shape:

```text
Verdict: review
Statements: 1
Warnings: 1

Statement 1: CREATE TABLE
- [warning] ddl.column.comment.require: column `id` must have a comment
```

## Metadata-Aware Variant

Use this when migration safety depends on current schema state:

```bash
deltascope audit \
  --config ./deltascope.yaml \
  --file ./migrations/20260322.sql \
  --host 127.0.0.1 \
  --port 3306 \
  --user root \
  --ask-password \
  --schema app
```

This is especially useful for:

- `ALTER TABLE` compatibility checks
- existence checks for columns or indexes
- table-option comparisons against the current schema

## Recommended Pattern

- keep a checked-in policy file
- run the audit in CI before migration execution
- fail on `blocker` or `warning` depending on team tolerance
- keep at least one migration fixture in the repository so developers can reproduce the same audit locally
