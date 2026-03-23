# Audit SQL Offline

Use this path when you want repeatable SQL review with no database dependency.

## Risky DML Example

```bash
deltascope audit --sql "delete from users"
```

Expected shape:

```text
Verdict: reject
Statements: 1
Blockers: 1

Statement 1: DELETE
- [blocker] dml.where.require: DELETE or UPDATE must include a WHERE clause
```

## DDL Example

```bash
deltascope audit --sql "create table users (id bigint unsigned not null auto_increment, primary key (id), name varchar(255) not null comment 'user name') comment='user table'"
```

Expected shape:

```text
Verdict: review
Statements: 1
Warnings: 1

Statement 1: CREATE TABLE
- [warning] ddl.column.comment.require: column `id` must have a comment
```

## File Input

```bash
deltascope audit --file ./change.sql
```

## Stdin Input

```bash
cat ./change.sql | deltascope audit
```

## JSON Output For Automation

```bash
deltascope audit --sql "alter table users drop column age" --format json
```

Expected keys:

- `verdict`
- `summary`
- `statements`
- `findings`
