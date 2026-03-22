# Audit SQL Offline

Use this path when you want repeatable SQL review with no database dependency.

## Inline SQL

```bash
deltascope audit --sql "delete from users"
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
