# PostgreSQL DML table-existence parity (T60 / #145)

## Decision

`dml.table.exists.require` now applies to PostgreSQL DML statements under
metadata-connected audits, matching the MySQL/TiDB contract. A qualified
DML target (`schema.table`) resolves its schema qualifier for provider
reads on every dialect — PostgreSQL no longer keeps the request schema for
DML provider lookups.

## Rationale

- The T54 acceptance pass recorded honestly that the postgres
  metadata-connected path emitted no `dml.table.exists.require` finding on
  missing tables. The absence was a rule-side dialect gate plus a
  legacy request-schema contract — not a provider limitation.
  `postgresql.LoadTableSnapshot` has always answered `Exists=false`
  correctly, and the PG DML extractor has always populated
  `MutationTargets`.
- Keeping the gate meant PostgreSQL mutation audits could green-light
  statements against tables that do not exist — a finding MySQL/TiDB
  already produce. Parity removes a real cross-dialect asymmetry.
- Unqualified PG targets still resolve under the request schema. PG
  `search_path` resolution is a connection property, not an audit
  property; the request schema stays the audit-side context, matching the
  DDL path's existing convention.

## Public contract

- PostgreSQL `INSERT`/`UPDATE`/`DELETE` on a proven-missing table now emit
  a `dml.table.exists.require` blocker under metadata-connected audits
  (single mutation target only, same fail-closed scope as MySQL/TiDB).
- `metadataTargetSchema` now honors the DML target's schema qualifier on
  every dialect — previously the qualifier won only for MySQL/TiDB.
- Offline PostgreSQL audits are unchanged: no provider means no
  existence fact, and the rule stays silent rather than fabricating a
  finding.

## Verification

- `TestTableExistenceRuleUsesOnlyDefinitiveMySQLTiDBSnapshots` —
  postgresql case now expects the finding.
- `TestEnrichMetadataKeepsRequestSchemaForPostgreSQLAndDDL` — documents
  the qualifier-wins contract for DML on all dialects; DDL keeps the
  legacy request-schema read.
- `testdata/sql-corpus/postgresql/dml/metadata/` — new stub-provider
  corpus cases pin `missing_table_delete` (exists.require fires) and
  `existing_table_update` (rule stays silent on a proven-present table).
- Golden: T54 meta case re-pinned — `t54.t54m` (fixture present via
  case-owned setup) emits no existence finding while
  `t54.t54_missing` emits the blocker; `post_verify` proves the audited
  fixture was not mutated.
- Inventory rows `postgresql.dml.insert|update|delete` status_evidence
  updated with the metadata-connected feature finding.

## Links

- Issue: #145 (child of #144)
- Prior deferred scope: `docs/decisions/2026-11-15-dml-acceptance-closure.md`
