# Audit SQL With Metadata

Use metadata-aware mode when current schema state or instance facts matter.

## Example

```bash
deltascope audit \
  --sql "alter table users add column email varchar(255)" \
  --host 127.0.0.1 \
  --port 3306 \
  --user root \
  --ask-password \
  --schema app
```

Expected JSON shape:

```json
{
  "verdict": "review",
  "context": {
    "mode": "metadata-aware",
    "dialect": "mysql",
    "schema": "app"
  }
}
```

## Ambiguous Schema Example

If `users` exists in more than one schema and you omit `--schema`, DeltaScope should fail honestly instead of guessing:

```bash
deltascope audit \
  --sql "alter table users add column email varchar(255)" \
  --host 127.0.0.1 \
  --port 3306 \
  --user root \
  --ask-password
```

Expected behavior:

- exit code `2`
- clear message telling you to pass `--schema`

## Notes

- DeltaScope auto-detects MySQL vs TiDB from the live target.
- `--schema` avoids ambiguity when the same table name exists in multiple schemas.
- SQL that already names a schema, such as `app.users`, should use that schema directly.
- JSON output includes a `context` object showing the resolved dialect and schema.
