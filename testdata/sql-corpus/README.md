# SQL Corpus

This directory contains dialect-specific SQL examples paired with expected audit
outcomes. Each case is a pair of files:

- `<case>.sql`          — the SQL statement under test
- `<case>.expected.yaml` — the expected audit outcome

## Directory Layout

```
sql-corpus/
├── mysql/
│   ├── ddl/
│   │   ├── supported/     — statements DeltaScope fully supports
│   │   └── unsupported/   — statements parsed but not fully covered
│   └── dml/
│       ├── supported/
│       └── unsupported/
├── tidb/
│   └── ...
└── postgresql/
    └── ...
```

## Expected YAML Schema

```yaml
name: <string, required>          # descriptive case name
dialect: <mysql|tidb|postgresql>  # required
category: <ddl|dml>               # required
expect:
  parse_ok: <bool>                # whether parsing should succeed
  statement_kind: <string>        # ddl or dml
  operation: <string>             # e.g. create_table, alter_table
  unsupported:
    count: <int, >= 0>            # expected unsupported-feature count
  findings:
    include: [<string>]           # rule IDs that must appear
    exclude: [<string>]           # rule IDs that must NOT appear
```

All fields under `expect` are optional in this first version — the loader
validates structural shape only. Later tasks will enforce stricter schemas.
