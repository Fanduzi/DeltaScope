# T06 base CREATE TABLE traceability (issue #85, slices A1+A2+A3)

Scope: the `mysql.create-table` and `tidb.create-table` inventory rows owned by
T06. This file maps each #85 acceptance dimension to the normalized spec
fields, consuming rules, and the evidence that actually proves it today. It is
a bounded traceability table, not a 380-rule census: dimensions owned by later
tasks are listed as deferred, not silently absorbed.

Status vocabulary:

- **golden-proven (this slice)** — pinned by `testdata/ddl-golden/T06.json`
  (72 cases: 10 baseline + 26 offline controls + 36 anchored structure proofs)
  and/or the `T06A1`/`T06A2`/`T06A3` Go regression tests.
- **earlier evidence** — covered by pre-T06 artifacts (T02–T05 golden runs,
  sql-corpus fixtures, catalog examples); not re-proven by this slice.
- **not proven** — the field/rule exists but no dedicated evidence pins the
  behavior for this dimension yet; later T06 slices must supply it.
- **deferred** — explicitly owned by another milestone task.

## Row ownership

| Inventory row | Status in inventory | T06-A1/A2/A3 addition |
|---|---|---|
| `mysql.create-table` | `semantically_checked` | `file:testdata/ddl-golden/T06.json` + this file appended to `acceptance.refs` |
| `tidb.create-table` | `semantically_checked` | same |

Neighboring rows share the CREATE TABLE grammar but stay with their owners:
`*.create-table-generated-column` (T07), `*.create-table-constraints` (T11),
`*.create-table-partitioning` (T14), `*.create-table-options` (T15),
`*.create-table-like` / `*.create-table-select` / temporary tables (T16),
`tidb.create-table-ttl` / `-placement-hint` / `-auto-random` / `-cache`
(T19/T20/T23/T23), CHECK constraints (T12), index families (T09/T10).

## Dimension table

| #85 dimension | Normalized facts (spec fields) | Consuming rules | Evidence | Status |
|---|---|---|---|---|
| Explicit PRIMARY KEY presence (inline + table-level) | `DDL.PrimaryKey` (`*spec.Index`, name `primary`, kind `IndexKindPrimary`, `Columns`) | `ddl.table.primary_key.require` | `T06.json` 10 cli + 12 meta cases; `TestT06A1PrimaryKeyPresenceControls/Normalization` | **golden-proven (this slice)** |
| UNIQUE must not substitute for PK | `DDL.Indexes` (`IndexKindUnique`) vs `DDL.PrimaryKey` | `ddl.table.primary_key.require` | `TestT06A1UniqueKeyNotPrimaryKey` | **golden-proven (this slice)** |
| PK fact consumed by follower statements | `applyCreateTable` → `TableSnapshot.PrimaryKey`/`HasPrimaryKey` | state layer (T05), not a rule | `TestT06A1PrimaryKeyPreStateConsumption`; T05 golden manifests | **golden-proven (this slice, consumption seam)** |
| Policy on/off is independent of legality | policy `enabled`/`params.required` | `ddl.table.primary_key.require` | `T06.json` `t06-*-rule-off` / `t06-*-required-false` cli cases; anchored no-PK cases show product reject + driver success | **golden-proven (this slice)** |
| Type family presence (INT etc.) | `Column.Type` | `ddl.column.blob_text.forbid`, `ddl.column.json.forbid`, `ddl.column.bit.forbid`, `ddl.column.float_double.forbid`, `ddl.column.timestamp.forbid`, `ddl.table.primary_key.bigint.require`, `ddl.table.primary_key.unsigned.require` | corpus `findings`/`metadata` fixtures; catalog examples; `T06.json` structure oracle pins `DATA_TYPE=int` | **earlier evidence** — per-type-family version matrix not yet pinned |
| Length / precision | `Column.Length` (and type args) | `ddl.column.char.max_length`, `ddl.column.varchar.max_length`, `ddl.table.row_size.max_bytes.require` | corpus fixtures; catalog examples | **earlier evidence** |
| PK member implied NOT NULL (inline + table-level + composite) | `Column.NotNull` normalized in `extractCreateTable`; `DDL.PrimaryKey.Columns` binding | `ddl.table.primary_key.not_null.require`, `ddl.column.not_null.require` | `T06.json` `t06-a2-*` 8 cli + 16 meta cases (`IS_NULLABLE=NO`, ordered `b:1,a:2` members, ERROR 1171 negatives); `TestT06A2*` Go tests | **golden-proven (A2)** |
| Explicit NULL on PK member keeps the declaration conflict | `ColumnOptionNull` → member `NotNull=false` | `ddl.table.primary_key.not_null.require` | `t06-a2-*-explicit-null-{table,inline}` meta cases (product reject + driver ERROR 1171 + absence); `primary_key_explicit_null` corpus fixtures | **golden-proven (A2)** |
| DEFAULT clause presence (absent vs explicit `DEFAULT NULL`) | `HasDefault`, `DefaultValue`, `DefaultIsNull` | `ddl.column.default.require` | `T06.json` `t06-a2-*-no-default`/`default-null` + `t06-a3-{mysql\|tidb}-{no-default,sql-null,text-null,text-nil}` cli cases; `TestT06A3*` | **golden-proven (A3)** — SQL `NULL` records `DefaultValue="NULL"` + `DefaultIsNull=true` from the typed AST datum; `'NULL'`/`'<nil>'` keep quoted text with `DefaultIsNull=false` |
| Typed NULL default reaches drop-column state seam | `DefaultIsNull` consumed by `dropOtherColumnMayReference`; post-state column clone | state layer (T05), not a rule | `T06.json` `t06-a3-{anchor}-null-drop-state` meta cases (3-statement complete/pass + post-DROP structure oracle); `TestT06A3NullDropStatePath` | **golden-proven (A3)** |
| Server-side `COLUMN_DEFAULT` NULL vs literal bytes | `information_schema.COLUMNS` `COLUMN_DEFAULT`/`HEX` observation | — (oracle, not a product fact) | `T06.json` `t06-a3-{anchor}-default-representation` structure verifies (`a:1:-,b:1:-,c:0:4E554C4C,d:0:3C6E696C3E`) | **golden-proven (A3)** — declaration-vs-stored-source gap remains deferred below |
| NULL / literal defaults beyond PK members | `Column.NotNull`, `HasDefault`, `DefaultValue`, `DefaultKind`, `DefaultIsNull` | `ddl.column.not_null.require`, `ddl.column.default.require` | corpus fixtures; `T06.json` `IS_NULLABLE` structure assertions | **earlier evidence** — expression defaults are T07 |
| Temporal columns | `DefaultIsCurrentTimestamp`, `OnUpdateCurrentTimestamp` | `ddl.column.timestamp.forbid` | corpus fixtures | **earlier evidence** — temporal default/version semantics not yet pinned per anchor |
| AUTO_INCREMENT | `Column.AutoIncrement` | `ddl.table.auto_increment.init_value.require`, `ddl.table.primary_key.auto_increment.require` | corpus fixtures | **earlier evidence** |
| Charset / collation | `Column.Charset`, `Column.Collation`, `DDL.Options` entries | `ddl.column.charset.allowlist`, `ddl.column.collation.allowlist`, `ddl.column.charset_collation.match.require`, `ddl.table.charset.allowlist` | corpus fixtures | **earlier evidence** |
| Comments | `Column.Comment`, `DDL.Options["comment"]` | `ddl.table.comment.require`, `ddl.table.comment.max_length`, `ddl.column.comment.require`, `ddl.table.audit_columns.require` | corpus fixtures | **earlier evidence** |
| Common table options | `DDL.Options`, `DDL.UnextractedOptions` | `ddl.table.engine.allowlist`, `ddl.table.row_format.allowlist`, `ddl.table.partition.forbid`, `ddl.table.create_as.forbid`, `ddl.table.create_like.forbid` | corpus fixtures; `T06.json` ENGINE structure assertion on MySQL anchors | **earlier evidence** — option enumeration completeness belongs to T15 |
| GIPK absence on MySQL 8.0/8.4 | `instance_facts` reads | — (fixture facts, not findings) | `T06.json` `expect.instance_facts` on `mysql80`/`mysql84` cases (`sql_require_primary_key`/`sql_generate_invisible_primary_key` OFF, `show_gipk_in_create_table_and_information_schema` ON) | **golden-proven (this slice)** |

## Honest gaps recorded for later T06 slices

- Version-conditional column forms (e.g. CHECK-as-parsed-but-ignored on 5.7 vs
  enforced on 8.0) are T12/T04 territory; this slice does not pin them.
- `CREATE TABLE ... LIKE`/`SELECT`/`IF NOT EXISTS` lifecycle semantics are T16.
- Full table-option coverage (ROW_FORMAT/KEY_BLOCK_SIZE/STATS_*/comment sizes)
  is T15; `DDL.UnextractedOptions` rows are extracted-but-unaudited boundary
  entries (unsupported evidence), not #83-style missing-fact evidence gaps
  and not findings.
- TiDB-only CREATE TABLE options (AUTO_RANDOM, SHARD_ROW_ID_BITS,
  PRE_SPLIT_REGIONS, TTL, placement) keep their existing vendor-boundary or
  dedicated-row status under T19/T20/T23.
- Metadata default-value semantics/source stay pending: the provider reads
  `COLUMN_DEFAULT` where SQL NULL conflates "no clause" with an explicit
  `DEFAULT NULL` and a non-NULL `"NULL"` text cannot be told from a real SQL
  NULL by text alone (`internal/infrastructure/metadata/mysql/provider.go`);
  the four-anchor `t06-a3-*-default-representation` oracles record the stored
  reality but no declaration-source equivalence is claimed.
- Expression defaults, `DEFAULT` execution semantics, and generated-column
  defaults stay outside T06 (T07/T27–T30).
- `ColumnOptionNull` stays a no-op marker on non-PK columns, so "declared
  NULL" vs "omitted nullability" is only observable on primary-key members.
- No stored-body or generated/expression semantics enter this slice
  (T07/T27–T30).

## What this slice does NOT claim

`semantically_checked` on the two inventory rows predates T06-A1 and stays
unchanged; the new refs record that the primary-key-presence path now has
four-anchor product/database split evidence. No status was widened, no gap was
relabeled to notice/unsupported to shrink the denominator, and #85 remains
open — this slice covers one dimension, not the row.
