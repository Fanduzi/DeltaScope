# DDL Coverage Catalog

The DDL coverage catalog answers a single question: for a given DDL form, does DeltaScope audit it, silently normalize it, explicitly mark it as an unsupported boundary, or fail to parse it?

A machine-readable JSON version is available at [`ddl-coverage-catalog.json`](ddl-coverage-catalog.json).

---

## What This Is

This is DeltaScope's **verified representative DDL coverage catalog**. It lists every DDL form that DeltaScope's census tests have classified, along with the classification and (where applicable) the rule IDs that fire for that form.

- Generated from DeltaScope's own census test data.
- Covers MySQL, TiDB, and PostgreSQL dialects.
- Each entry includes a classification, the dialect, the form label, and associated rule IDs.

## What This Is Not

This catalog has explicit boundaries. It is **not**:

- The official MySQL, TiDB, or PostgreSQL DDL grammar. Many vendor-grammar forms are not represented here.
- A claim of full DDL support. DeltaScope does not audit every DDL statement a database vendor accepts.
- A dialect parity declaration. Coverage counts differ across MySQL, TiDB, and PostgreSQL.
- A guarantee that all database vendor syntax is covered.
- A new parser, fallback parser, or new SQL audit rules. The catalog describes existing behavior only.

---

## Classifications

| Classification | Meaning |
|---|---|
| `finding_covered` | DeltaScope can parse and extract the form, and current rules may produce an audit finding. |
| `normalized_silent` | DeltaScope can parse and extract the form, but current default policy produces no finding. This is a known silent path, not an untracked gap. |
| `unsupported_boundary` | DeltaScope explicitly recognizes a product boundary and returns an unsupported diagnostic instead of pretending to audit. |
| `parser_error` | The selected dialect parser cannot parse the form. DeltaScope does not audit it. |
| `unclassified` | Reserved for catalog generation failures. The release gate requires this count to be zero. |

---

## Summary

| Dialect | Total | finding_covered | normalized_silent | unsupported_boundary | parser_error | unclassified |
|---------|------:|----------------:|------------------:|--------------------:|-------------:|-------------:|
| MySQL | 61 | 46 | 0 | 0 | 15 | 0 |
| TiDB | 54 | 45 | 0 | 0 | 9 | 0 |
| PostgreSQL | 285 | 274 | 6 | 0 | 5 | 0 |
| PG ALTER TABLE residual | 66 | 60 | 2 | 0 | 4 | 0 |

---

## User Examples

### MySQL `CREATE TABLE` — `finding_covered`

```bash
deltascope audit --dialect mysql --sql "CREATE TABLE t (id INT PRIMARY KEY)"
```

Classification: `finding_covered`. Rules that may fire include `ddl.table.primary_key.require`, `ddl.column.not_null.require`, and others depending on the table definition.

### MySQL `ALTER VIEW` — `parser_error` with guidance

```bash
deltascope audit --dialect mysql --sql "ALTER VIEW v AS SELECT 1"
```

Classification: `parser_error`. The MySQL dialect parser cannot parse `ALTER VIEW`. This entry carries `guidance_code=parser_upgrade_candidate` and an `evidence_ref` linking to the parser-upgrade candidate evidence.

### TiDB `ALTER TABLE TTL` — `parser_error`

```bash
deltascope audit --dialect tidb --sql "ALTER TABLE t TTL = INTERVAL 7 DAY"
```

Classification: `parser_error`. The TiDB dialect parser cannot parse the TTL clause.

### PostgreSQL `ALTER TABLE ... VALIDATE CONSTRAINT` — `finding_covered`

```bash
deltascope audit --dialect postgresql --sql "ALTER TABLE t VALIDATE CONSTRAINT ck"
```

Classification: `finding_covered`. Rule: `ddl.pg.alter.validate_constraint.advisory`.

### PostgreSQL `DROP SUBSCRIPTION ... WITH` — `parser_error` with guidance

```bash
deltascope audit --dialect postgresql --sql "DROP SUBSCRIPTION sub WITH (DROP SLOT)"
```

Classification: `parser_error`. The PostgreSQL dialect parser cannot parse this form. This entry carries `guidance_code=parser_upgrade_candidate` and an `evidence_ref`.

---

## Guidance Metadata

Some `parser_error` entries include additional fields that help explain why DeltaScope did not audit the statement.

### `guidance_code`

A label that classifies the parser-error case. The current value is `parser_upgrade_candidate`, which means the form would likely become parseable after a parser or library upgrade.

`parser_upgrade_candidate` is a documented classification. It is not current parser support, not a fallback parser, and not new SQL audit rules. DeltaScope does not infer findings from failed parse text.

### `evidence_ref`

A URL pointing to public documentation about the classification. For parser-upgrade candidates, this links to the v0.250.0 evidence section:

[`parser-upgrade-candidate-evidence-v02500`](cli.md#parser-upgrade-candidate-evidence-v02500)

---

## How to Verify Locally

Run the following commands to verify catalog integrity and census consistency:

```bash
# Verify the catalog JSON matches census baselines
make ddl-coverage-catalog-test

# View the DDL census report
make ddl-census-report

# View the SQL corpus report
make sql-corpus-report
```

To regenerate the catalog JSON when census data changes:

```bash
UPDATE_DDL_COVERAGE_CATALOG=1 go test ./internal/application/audit -tags postgresql -run TestDDLCoverageCatalog
```
