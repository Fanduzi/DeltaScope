# Application Audit Metadata Preparation Module

Shared preparation helpers for metadata-aware audit requests before they enter the core DeltaScope audit service.

## Files

| File | Responsibility |
|------|---------------|
| `errors.go` | Defines typed metadata-preparation errors, including MySQL/TiDB alias conflicts and PostgreSQL schema/database validation, for adapter-level classification |
| `prepare.go` | Opens metadata clients, detects dialect, binds MySQL/TiDB catalog aliases through connresolve, validates PostgreSQL database/schema selection, resolves schema, and returns prepared audit context |
| `targets.go` | Infers the session schema from every valid statement's targets — all DDL `TableTargets()` for drop/truncate/create, the alter subject only (rename destinations are new names, not existing-object evidence), and the first DML MutationTarget — even when another bounded statement has a parser error; fails only when no statement can be parsed |
| `client.go` | Bridges MySQL-compatible infrastructure providers into the shared preparation client contract |
| `prepare_test.go` | Verifies shared metadata-aware preparation behavior, including schema inference from valid statements around one parser error |
| `audit_t06a4_test.go` | Verifies the T06-A4 AuditSQL contract with the real `mysqlmeta.Provider` behind a controlled database/sql driver whose `information_schema.columns` fixture now carries the nine-column row shape including `generation_expression`: a stored `'NULL'` sibling keeps DROP COLUMN + CREATE INDEX conservative (`review`/`unverified`, one `unknown_table_state` gap, exactly one provider read), a stored `'<nil>'` sibling stays equally conservative, a stored SQL NULL sibling stays `pass`/`complete`, a parsed-source `DEFAULT 'NULL'` agrees with the provider outcome, provider errors propagate via `errors.Is` rather than degrading into gaps, `ddl.column.default.require` reads only submitted DDL, and all-disabled rules fabricate nothing; the recording driver also stubs the T11 foreign-key join fragment with a default empty result |
| `audit_t06a17_test.go` | Verifies the T06-A17 provider-source carry contract: a real `mysqlmeta.Provider` reads constructed nine-column catalog rows (`decimal(12,2)`, `enum('Ready','Closed')`, `set('Write','Read')`, including `generation_expression`) and AuditSQL's ordered pre-state lands the verbatim `COLUMN_TYPE` text in the modify-compatibility finding's `source_type` echo — never a truncated base keyword; the recording driver also stubs the T11 foreign-key join fragment with a default empty result |
| `audit_t08r1_default_comparison_test.go` | Verifies the T08-R1/R2 default-comparison knowledge boundary through the real parser, real `mysqlmeta.Provider`, and real `AuditSQL` over a controlled `database/sql` driver: same-value literals across source representations emit zero findings (S1–S7 strings plus numeric controls on MODIFY and CHANGE, both dialects), different values keep one transition finding, unprovable source/target identities emit bounded `source_column.default`/`target_column.default` gaps instead of findings, expression fingerprints compare only when known and non-empty, numeric representation differences outside the canonical-integer domain (catalog `1.00` vs declared `1`, derived `1.00` vs `1`) stay unproven on both sources while scale-0 decimal catalog text stays canonical-comparable, year-column different-text numeric pairs stay unproven because the type normalizes the literal on storage (catalog `2001` vs declared `1`, derived `1` vs `2001`, T08-R3), and an identical default identity under a type-family change stays silent beside the independent type finding, CHANGE reads the old-named source column, provider-sourced and CREATE-derived pre-states judge identically, SET/DROP/CHANGE ordered chains publish and clear the comparable identity, the explicit-default-change forbid stays independent of the comparison, the TiDB fixture stubs `table_constraints` with a count=1 `PRIMARY KEY` answer so the catalog `PRIMARY` fold binds the constraint identity under `resolveTiDBPrimaryName`, and the provider snapshot is never mutated (issue #87 T08-R1/R2); the recording driver also stubs the T11 foreign-key join fragment with a default empty result |

## Exports

- `Client`
- `ConnectionConfig` — includes `ConnectTimeout` for metadata connection timeout
- `Request`
- `PreparedAudit`
- `Prepare(ctx, request)`
- `Error` / `ErrorKind` — typed preparation failures, including `ErrorMySQLDatabaseSchemaConflict` and `ErrorPostgreSQLDatabaseRequired`

## Dependencies

- Upstream: `internal/interfaces/cli`, `internal/interfaces/http`, `internal/interfaces/mcp`
- Downstream: `internal/application/audit`, `internal/application/connresolve`, `internal/domain/spec`, `internal/infrastructure/metadata/mysql`

## Update Rule

- If members/interfaces/dependencies change, update this file in same change.
