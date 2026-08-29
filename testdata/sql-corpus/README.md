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

## PostgreSQL Representative Pack

The PostgreSQL corpus keeps production-shaped coverage bounded and separate:

- standalone `CREATE INDEX CONCURRENTLY`, `DROP INDEX CONCURRENTLY`, and `REFRESH MATERIALIZED VIEW CONCURRENTLY`
- one-fixture `CREATE TABLE` cases for `UNLOGGED`, identity, `JSONB`, arrays, generated stored columns, and `LIKE ... INCLUDING`
- a mixed migration that retains valid `ALTER TABLE`, concurrent-index, and DML results around an intentionally malformed statement under the issue #43 partial-result contract

Pack manifests assert the retained statement count, first statement kind, unsupported count, and meaningful finding or parser-diagnostic expectations. These are representative PostgreSQL production shapes, not official grammar-completeness coverage.

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
  statements:
    count: <int, >= 0>            # retained audited statements, including partial-error cases
  diagnostics:
    parser_error_count: <int, >= 0>
    lines: [<int>]                 # ordered 1-based parser-diagnostic lines
    columns: [<int>]               # ordered 1-based parser-diagnostic columns
  impact:                         # optional statement-level DML impact contract
    estimated_rows: <int>         # expected conservative row estimate
    estimated_ratio: <number>     # expected conservative ratio
    risk_level: <low|medium|high|unknown>
    confidence: <low|medium|high>
    source: <shape|metadata|plan>
    reason_codes: [<string>]
    notes: [<string>]
facts:                            # optional semantic assertions
  constraints:
    - type: <string, required>    # e.g. foreign_key, check
      name: <string>              # constraint name (optional)
      columns: [<string>]         # constraint columns (optional)
      referenced_table: <string>  # FK target table (optional)
      referenced_columns: [<string>]  # FK target columns (optional)
```

Metadata-aware cases may add `metadata.schema` and `metadata.tables` snapshots.
An explicit `exists: false` snapshot represents a provider-confirmed missing
relation; omitting the snapshot keeps the audit offline or makes no existence
claim.

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
