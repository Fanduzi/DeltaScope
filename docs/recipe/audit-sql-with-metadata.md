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

## Notes

- DeltaScope auto-detects MySQL vs TiDB from the live target.
- `--schema` avoids ambiguity when the same table name exists in multiple schemas.
- JSON output includes a `context` object showing the resolved dialect and schema.
