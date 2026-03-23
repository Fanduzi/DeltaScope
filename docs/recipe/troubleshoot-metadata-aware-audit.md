# Troubleshoot Metadata-Aware Audit

Use this guide when `deltascope audit` works offline but metadata-aware mode fails or behaves differently than expected.

## Symptom: DeltaScope asks for `--schema`

Typical message shape:

```text
target table exists in multiple schemas; pass --schema to disambiguate
```

Why it happens:

- the target table name exists in more than one schema
- DeltaScope refuses to guess

What to do:

```bash
deltascope audit \
  --sql "alter table users add column email varchar(255)" \
  --host 127.0.0.1 \
  --port 3306 \
  --user root \
  --ask-password \
  --schema app
```

## Symptom: SQL already names a schema, but the target still fails

Use fully qualified SQL:

```sql
alter table app.users add column email varchar(255);
```

DeltaScope should use the SQL-qualified schema directly. If it still fails, verify:

- the schema name is spelled correctly
- the connected instance actually contains that schema
- you are not connecting to the wrong MySQL/TiDB target

## Symptom: Metadata-Aware Mode Cannot Connect

Checklist:

- `--host`, `--port`, and `--user` are correct
- use `--ask-password` if you do not want secrets in shell history
- use `--socket` for local Unix socket workflows
- verify the target schema exists and the user can read metadata

## Symptom: Output Looks Different From Offline Mode

That is expected when metadata-aware mode is active. JSON output should include:

```json
{
  "context": {
    "mode": "metadata-aware",
    "dialect": "mysql",
    "schema": "app"
  }
}
```

If `context.mode` is not `metadata-aware`, check whether you actually supplied connection flags.

## Symptom: DDL Checks Still Seem Incomplete

Remember what metadata-aware mode does and does not do:

- it adds instance facts and current schema state
- it improves existence and compatibility checks
- it does not replace the core rule engine with a second online-only engine

For the conceptual model, see [../concept/metadata-aware-mode.md](../concept/metadata-aware-mode.md).
