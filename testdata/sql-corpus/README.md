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
│   │   ├── unsupported/   — statements parsed but not fully covered
│   │   ├── findings/      — statements expected to produce specific findings
│   │   └── boundary/      — edge cases recording current behaviour
│   └── dml/
│       ├── clean/         — statements producing no findings
│       ├── findings/
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
  statement_kind: <ddl|dml|unknown>  # reported statement kind
  operation: <string>             # e.g. create_table, alter_table, update
  unsupported:
    count: <int, >= 0>            # expected unsupported-feature count
  findings:
    include: [<string>]           # rule IDs that must appear
    exclude: [<string>]           # rule IDs that must NOT appear
facts:                            # optional semantic assertions
  constraints:
    - type: <string, required>    # e.g. foreign_key, check
      name: <string>              # constraint name (optional)
      columns: [<string>]         # constraint columns (optional)
      referenced_table: <string>  # FK target table (optional)
      referenced_columns: [<string>]  # FK target columns (optional)
```

## Two-Layer Test Architecture

Corpus tests assert behaviour at two layers:

1. **Report-level audit assertions** — run the full `AuditSQL` pipeline and check
   `unsupported.count`, `statement_kind`, `findings.include`, and `findings.exclude`
   against the rendered `report.Result`.

2. **Semantic parse/extract assertions** — use the internal `Parse` + `Extract`
   path to access `spec.Statement` fields that the report does not expose. This
   layer asserts `operation` (DDL or DML operation name) and `facts.constraints`
   (constraint type, name, columns, referenced table/columns).

Both layers are driven by the same `.expected.yaml` file. The semantic layer only
runs when the expected file includes `operation` or `facts` fields.
