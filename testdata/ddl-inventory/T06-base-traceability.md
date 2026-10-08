# T06 base CREATE TABLE traceability (issue #85, slices A1+A2+A3+A4+A5+A7+A8+A9)

Scope: the `mysql.create-table` and `tidb.create-table` inventory rows owned by
T06. This file maps each #85 acceptance dimension to the normalized spec
fields, consuming rules, and the evidence that actually proves it today. It is
a bounded traceability table, not a 380-rule census: dimensions owned by later
tasks are listed as deferred, not silently absorbed.

Status vocabulary:

- **golden-proven (this slice)** — pinned by `testdata/ddl-golden/T06.json`
  (256 cases: 8 baseline database + 112 offline CLI + 136 anchored metadata
  structure proofs)
  and/or the `T06A1`/`T06A2`/`T06A3`/`T06A4`/`T06A5`/`T06A7`/`T06A8`/`T06A9` Go regression tests.
- **earlier evidence** — covered by pre-T06 artifacts (T02–T05 golden runs,
  sql-corpus fixtures, catalog examples); not re-proven by this slice.
- **not proven** — the field/rule exists but no dedicated evidence pins the
  behavior for this dimension yet; later T06 slices must supply it.
- **deferred** — explicitly owned by another milestone task.

## Row ownership

| Inventory row | Status in inventory | T06-A1/A2/A3/A4/A5/A7/A8 addition |
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
| Length / precision | `Column.Length` (and type args) | `ddl.column.char.max_length`, `ddl.column.varchar.max_length`, `ddl.table.row_size.max_bytes.require` | `T06.json` `t06-a5-*` 16 cli + 24 meta cases pin declared CHAR/VARCHAR lengths against `limit=8` (7/8/9 + all-off control, exact `limit`/`actual` metadata, post-audit absence + native replay on both, `CHARACTER_MAXIMUM_LENGTH=N`/`CHARACTER_OCTET_LENGTH=4*N`/`utf8mb4_bin` oracle); `TestT06A5*` Go tests; corpus fixtures; catalog examples | **partially golden-proven (A5)** — declared CHAR/VARCHAR character length on CREATE only; DECIMAL precision/scale, other type args, `row_size`/`key_length` byte estimation, and ALTER-side length policy (#87/T08) stay earlier evidence |
| PK member implied NOT NULL (inline + table-level + composite) | `Column.NotNull` normalized in `extractCreateTable`; `DDL.PrimaryKey.Columns` binding | `ddl.table.primary_key.not_null.require`, `ddl.column.not_null.require` | `T06.json` `t06-a2-*` 8 cli + 16 meta cases (`IS_NULLABLE=NO`, ordered `b:1,a:2` members, ERROR 1171 negatives); `TestT06A2*` Go tests | **golden-proven (A2)** |
| Explicit NULL on PK member keeps the declaration conflict | `ColumnOptionNull` → member `NotNull=false` | `ddl.table.primary_key.not_null.require` | `t06-a2-*-explicit-null-{table,inline}` meta cases (product reject + driver ERROR 1171 + absence); `primary_key_explicit_null` corpus fixtures | **golden-proven (A2)** |
| DEFAULT clause presence (absent vs explicit `DEFAULT NULL`) | `HasDefault`, `DefaultValue`, `DefaultIsNull` | `ddl.column.default.require` | `T06.json` `t06-a2-*-no-default`/`default-null` + `t06-a3-{mysql\|tidb}-{no-default,sql-null,text-null,text-nil}` cli cases; `TestT06A3*` | **golden-proven (A3)** — SQL `NULL` records `DefaultValue="NULL"` + `DefaultIsNull=true` from the typed AST datum; `'NULL'`/`'<nil>'` keep quoted text with `DefaultIsNull=false` |
| Typed NULL default reaches drop-column state seam | `DefaultIsNull` consumed by `dropOtherColumnMayReference`; post-state column clone | state layer (T05), not a rule | `T06.json` `t06-a3-{anchor}-null-drop-state` meta cases (3-statement complete/pass + post-DROP structure oracle); `TestT06A3NullDropStatePath` | **golden-proven (A3)** |
| Server-side `COLUMN_DEFAULT` NULL vs literal bytes | `information_schema.COLUMNS` `COLUMN_DEFAULT`/`HEX` observation | — (oracle, not a product fact) | `T06.json` `t06-a3-{anchor}-default-representation` structure verifies (`a:1:-,b:1:-,c:0:4E554C4C,d:0:3C6E696C3E`); `t06-a4-{anchor}-*` setup/post_verify oracles | **golden-proven (A3/A4)** — declaration provenance stays unrecoverable below |
| Provider snapshot default identity consumed by drop-column state | `Column.HasDefault` (non-NULL stored default), `DefaultValue` (raw stored text), `DefaultIsNull` (never set by metadata path) | state layer via `dropOtherColumnMayReference`, not a rule | `T06.json` `t06-a4-{anchor}-{drop-d-text-null,drop-c-text-nil,drop-d-null-control}` meta cases (setup-CREATE + product `review`/`pass` split + mid-flight oracle); `TestT06A4*` Go tests | **golden-proven (A4)** — stored `'NULL'` can no longer read as SQL NULL |
| NULL / literal defaults beyond PK members | `Column.NotNull`, `HasDefault`, `DefaultValue`, `DefaultKind`, `DefaultIsNull` | `ddl.column.not_null.require`, `ddl.column.default.require` | corpus fixtures; `T06.json` `IS_NULLABLE` structure assertions | **earlier evidence** — expression defaults are T07 |
| Temporal columns (explicit DATETIME audit roles) | `DefaultIsCurrentTimestamp`, `OnUpdateCurrentTimestamp`, `HasDefault`, `Column.Type` | `ddl.table.audit_columns.require` (created = time type + current-timestamp default; updated = same + `ON UPDATE`), `ddl.column.timestamp.forbid` | `T06.json` `t06-a9-*` 12 cli + 16 meta cases under the `t06-a9-audit-columns-isolated` blocker profile: complete-pair / missing-created / missing-updated / missing-both (exactly two per-role findings) / `NOW()`-spelling / all-off roles across both dialects; anchored roles pin stored `COLUMN_DEFAULT` whitelisted to `current_timestamp`/`current_timestamp()`/`current_timestamp(0)`, normalized `EXTRA` (`created_at` empty-or-DEFAULT_GENERATED, `updated_at` = `on update current_timestamp`), PK membership, and driver CREATE success alongside product reject; `TestT06A9*` Go tests pin parser normalization of `CURRENT_TIMESTAMP`/`CURRENT_TIMESTAMP()`/`NOW()`/`LOCALTIME`/`LOCALTIMESTAMP` (fsp `NOW(3)` arg preserved), role attribution from facts not names, `ON UPDATE`-without-default counting as neither role, `required=false`/disabled silence, and pre-state flag carry-over | **partially golden-proven (A9)** — explicit `DATETIME ... DEFAULT CURRENT_TIMESTAMP [ON UPDATE CURRENT_TIMESTAMP]` roles only; TIMESTAMP implicit defaults/`explicit_defaults_for_timestamp`, fsp>0 policy, time-zone/runtime-value semantics, other temporal types, and ALTER-side governance stay earlier evidence or deferred |
| AUTO_INCREMENT | `Column.AutoIncrement` | `ddl.table.auto_increment.init_value.require`, `ddl.table.primary_key.auto_increment.require` | corpus fixtures | **earlier evidence** |
| Charset / collation | `Column.Charset`, `Column.Collation`, `DDL.Options["collate"]` | `ddl.column.charset.allowlist`, `ddl.column.collation.allowlist`, `ddl.column.charset_collation.match.require`, `ddl.table.charset.allowlist`, `ddl.table.collation.allowlist` (new, default-disabled) | `T06.json` `t06-a7-*` 32 cli + 32 meta cases: each column rule isolated (allow/deny/off), match pair/single/empty/mismatch/`required=false`, table collate allowed/denied/missing under both `require_explicit` values; anchored roles pin `CHARACTER_SET_NAME`/`COLLATION_NAME`/`TABLE_COLLATION` as resolved facts distinct from declared fields, `latin1` octets (16) vs `utf8mb4` (64), and the utf8mb4+latin1_bin driver ERROR 1253 negative; `TestT06A7*` Go tests; corpus fixtures | **golden-proven (A7)** — declared-declaration facts only; native collation catalog completeness, ordering behavior, and ALTER-side governance are not claimed |
| Comments | `Table.Comment`, `Column.Comment` (parser-decoded content, no SQL quote wrap), `DDL.Options["comment"]`, derived-shape `Table.Comment` | `ddl.table.comment.require`, `ddl.table.comment.max_length` (Unicode code points), `ddl.column.comment.require`, `ddl.table.audit_columns.require` | `T06.json` `t06-a8-*` 24 cli + 16 meta cases: per-field require isolation (missing/empty/blank), code-point length boundaries (ASCII 8/9, `中文注`=3, 8/9-rune at/above), all-off control; anchored roles pin raw comment + `CHAR_LENGTH`/`OCTET_LENGTH`/`HEX` under recorded `utf8mb4` session (`it''s 中文`→`it's 中文`, `表注`, `中文注中文注ab[c]`) including product-reject+driver-success cases; `TestT06A8*` Go tests | **golden-proven (A8)** — column-comment length policy and comment provenance (`HasComment`) are not claimed |
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
- Metadata default declaration provenance stays unrecoverable (permanent
  catalog bound, not a pending task): `COLUMN_DEFAULT IS NULL` cannot
  distinguish an omitted `DEFAULT` clause from an explicit `DEFAULT NULL`,
  and `SHOW CREATE` only emits normalized output. T06-A4 resolved the
  provider fact half — a non-NULL `COLUMN_DEFAULT` is always the stored
  representation and never sets `DefaultIsNull` — so parse-derived and
  provider-derived `HasDefault` remain two evidence layers that are not
  required to agree field-for-field; no source framework is introduced
  because no consumer needs provenance
  (`docs/decisions/2026-10-07-ddl-provider-default-null-fidelity.md`).
- Expression defaults, `DEFAULT` execution semantics, and generated-column
  defaults stay outside T06 (T07/T27–T30).
- `ColumnOptionNull` stays a no-op marker on non-PK columns, so "declared
  NULL" vs "omitted nullability" is only observable on primary-key members.
- No stored-body or generated/expression semantics enter this slice
  (T07/T27–T30).
- The two length rules are CREATE-only: `ALTER TABLE ... ADD/MODIFY COLUMN`
  length policy consistency is #87/T08 scope, asserted at the rule-method
  layer by `TestT06A5LengthRulesIgnoreAlterActions`, not widened here.
- Bare `CHAR` records the parser sentinel `UnspecifiedLength` (-1) and
  `CHAR(0)` records 0 — two distinct facts sharing only the under-limit
  outcome; no length-known flag was added. Bare `VARCHAR` and `CHAR BYTE`
  are recorded as real parser refusals, and the `CHARACTER`/`CHAR VARYING`
  aliases carry no A5 evidence.
- A7 keeps the declaration layer and the native resolution layer separate:
  an empty `Column.Charset`/`Column.Collation` is "not declared", never an
  observed server default — the anchored oracles prove the same statements
  resolve to `utf8mb4`/`utf8mb4_bin` via table-level inheritance. No
  inheritance inference was added to the extractor or the T05 state model.
- `CREATE DATABASE`/`ALTER DATABASE` collate stays unaudited boundary
  evidence (`create_schema.option.collate`/`alter_schema.option.collate`);
  `ALTER TABLE` collation governance is T15/#94 scope; collation catalog
  completeness and ordering semantics are not claimed anywhere in this
  slice.

## What this slice does NOT claim

`semantically_checked` on the two inventory rows predates T06-A1 and stays
unchanged; the new refs record that the primary-key-presence path now has
four-anchor product/database split evidence. No status was widened, no gap was
relabeled to notice/unsupported to shrink the denominator, and #85 remains
open — this slice covers one dimension, not the row. A5 extends the same
discipline to the declared-length dimension: a policy `blocker` on
`CHAR(9)`/`VARCHAR(9)` under `limit=8` is a team threshold verdict, never a
claim that the four anchors reject the DDL — every anchored case records the
product rejection and the driver-side native CREATE success as two
independent facts (`docs/decisions/2026-10-07-ddl-character-length-policy-proof.md`).
