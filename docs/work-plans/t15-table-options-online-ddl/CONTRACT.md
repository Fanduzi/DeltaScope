# T15 — Table Options, Charset Conversion & Online DDL Conditions

Issue: [#94](https://github.com/Fanduzi/DeltaScope/issues/94). Parent: [#79](https://github.com/Fanduzi/DeltaScope/issues/79).
Baseline: `1813f062d2205716536f1fda503c537e3f26141f` (T14-R2-tail2 closeout).
Frozen 2026-11-08. Native evidence: `probes/p15_online_matrix.log` (anchors: MySQL 5.7.44 / 8.0.46 / 8.4.10, TiDB v8.5.0) plus `probes/probe15_online.sh`.

## 1. Scope — four inventory rows expanded

T15 owns the remaining ALTER TABLE modifier/option semantics plus the
per-option CREATE-side declaration space:

| Inventory row | Dimensions |
|---|---|
| `mysql.create-table-options` | ENGINE, ROW_FORMAT, KEY_BLOCK_SIZE, CHARSET/COLLATE, COMPRESSION, ENCRYPTION, COMMENT, AUTO_INCREMENT, STATS_*, TABLESPACE declarations |
| `mysql.alter-table.online-ddl` | ALGORITHM=INSTANT/INPLACE/COPY/DEFAULT × LOCK=NONE/SHARED/EXCLUSIVE/DEFAULT vs operation/engine/product/version |
| `mysql.alter-table.options` | ORDER BY, CONVERT TO CHARACTER SET, DEFAULT CHARACTER SET, ROW_FORMAT, AUTO_INCREMENT, FORCE, ENABLE/DISABLE KEYS, COMMENT, ENGINE, KEY_BLOCK_SIZE, COMPRESSION, ENCRYPTION, STATS_*, TABLESPACE, MyISAM-family options |
| `tidb.alter-table.online-options` | ALGORITHM/LOCK capability assertions on TiDB, multi-action integrity, vendor-independent semantics |

Explicitly out of scope:

- Full tablespace lifecycle (CREATE/ALTER/DROP TABLESPACE statements, `IMPORT/DISCARD TABLESPACE` actions — owned by T25). T15 captures the `TABLESPACE=` *declaration* only.
- Lock-duration promises, execution timing, or requiring all DDL to be online. The audit answers applicability and risk, never "this statement will take lock L for T seconds".
- Executing user SQL, adding a new execution engine.
- T14 leftovers: INTERVAL partitioning, subpartition member modeling, arbitrary row routing.
- Option names outside the extracted set below stay `UnextractedOptions` with the existing bounded `<op>.option.<name>` gaps: `password`, `union`, `nodegroup`, `secondary_engine*`, `engine_attribute`, `autoextend_size`, `page_*`, `transactional`, `ietf_quotes`, `sequence`, `affinity`, `connection`, `data_directory`, `index_directory`, `insert_method` (extracted but see §6.4 for its bounded consumption), and all TiDB-only options (`auto_random_base`, `auto_id_cache`, `shard_row_id_bits`, `pre_split_regions`, `ttl*`, `stats_buckets`, `stats_top_n`, `stats_cols_choice`, `stats_col_list`, `stats_sample_rate`).

## 2. Probed native matrix (all observations in `p15_online_matrix.log` + probe3 supplements)

### 2.1 ALGORITHM × version

| Declaration | MySQL 5.7.44 | MySQL 8.0.46 | MySQL 8.4.10 | TiDB v8.5.0 |
|---|---|---|---|---|
| `ALGORITHM=INSTANT` + ADD COLUMN | **ERROR 1800** "Unknown ALGORITHM 'INSTANT'" | rc=0 | rc=0 | rc=0 (ignored) |
| `ALGORITHM=INSTANT` + ADD INDEX | 1800 | **ERROR 1845** "not supported for this operation" | 1845 | **ERROR 1846** "ALGORITHM=INSTANT is not supported … Try ALGORITHM=INPLACE" |
| `ALGORITHM=INSTANT` + MODIFY COLUMN | 1800 | **1845** | 1845 | rc=0 (ignored) |
| `ALGORITHM=INSTANT` + RENAME COLUMN | 1800 | rc=0 | rc=0 | rc=0 |
| `ALGORITHM=INSTANT` + DROP COLUMN | 1800 | rc=0 | rc=0 | rc=0 |
| `ALGORITHM=INSTANT` + explicit LOCK | 1800 (algorithm gate first) | **ERROR 1221** "Incorrect usage of ALGORITHM=INSTANT and LOCK=NONE/SHARED/EXCLUSIVE" | 1221 | rc=0 |
| `ALGORITHM=INPLACE` + ADD COLUMN | rc=0 | rc=0 | rc=0 | rc=0 |
| `ALGORITHM=INPLACE, LOCK=NONE` + ADD COLUMN / ADD INDEX | rc=0 | rc=0 | rc=0 | rc=0 |
| `ALGORITHM=COPY` + ADD COLUMN | rc=0 | rc=0 | rc=0 | rc=0 (column added; algorithm ignored) |
| `ALGORITHM=COPY, LOCK=NONE` | **ERROR 1846** "COPY algorithm requires a lock. Try LOCK=SHARED" | 1846 | 1846 | rc=0 |
| `ALGORITHM=COPY, LOCK=EXCLUSIVE` (seed) | rc=0 | rc=0 | rc=0 | rc=0 |
| `ALGORITHM=DEFAULT`, `LOCK=DEFAULT` | rc=0 | rc=0 | rc=0 | rc=0 |
| `LOCK=NONE` + ADD COLUMN (no ALGORITHM) | rc=0 | rc=0 | rc=0 | rc=0 |
| `LOCK=SHARED/EXCLUSIVE` + ADD COLUMN | rc=0 | rc=0 | rc=0 | rc=0 |
| `ALGORITHM=COPY` alone (no action) | rc=0 | rc=0 | rc=0 | rc=0 |
| `LOCK=EXCLUSIVE` alone | rc=0 | rc=0 | rc=0 | rc=0 |
| duplicate `ALGORITHM=COPY, ALGORITHM=INSTANT` | 1800 (INSTANT unknown) | rc=0 (last wins) | rc=0 | rc=0 |
| duplicate `LOCK=NONE, LOCK=EXCLUSIVE` | rc=0 (last wins) | rc=0 | rc=0 | rc=0 |

TiDB accepts and ignores every ALGORITHM/LOCK declaration on every probed
combination **except `ADD INDEX … ALGORITHM=INSTANT`, which refuses 1846**
(the same error class MySQL raises for an unsatisfiable lock) — the
parser-level comment suggesting COPY errors is stale; v8.5.0 executes the
action under its own online-DDL machinery (verified: column physically
added after `ALGORITHM=COPY`). INSTANT on column actions is accepted;
index INSTANT is refused, and unprobed INSTANT cells stay unproven.

### 2.2 ALTER table options (per-option × dialect)

MySQL 5.7/8.0/8.4 (InnoDB fixtures unless noted):

| Option | Outcome | Catalog effect |
|---|---|---|
| `ENGINE=MyISAM` ↔ `ENGINE=InnoDB` | rc=0 | `information_schema.tables.engine` flips |
| `ENGINE=nosuchengine` | **ERROR 1286** | none |
| `ROW_FORMAT=DYNAMIC/COMPACT` | rc=0 | `row_format` + `create_options` |
| `AUTO_INCREMENT=500` | rc=0 | seed becomes 500 |
| `AUTO_INCREMENT=1` (below current) | rc=0 | **clamped — seed unchanged** (stays 500) |
| `COMMENT='x'` | rc=0 | `table_comment` |
| `DEFAULT CHARACTER SET utf8mb4` | rc=0 | default changes; collation follows charset default when no COLLATE |
| `DEFAULT CHARACTER SET x COLLATE y` | rc=0 | `table_collation`=y |
| `CONVERT TO CHARACTER SET utf8mb4` | rc=0 | table + **every string column** converted |
| `CONVERT TO latin1` | rc=0 | converts (data-loss risk, not refused) |
| `CONVERT TO utf8` (mb3) | rc=0 | converts |
| `KEY_BLOCK_SIZE=8` on non-compressed InnoDB | **ERROR 1478** "does not support the create option" | none |
| `KEY_BLOCK_SIZE=8` on MyISAM | rc=0 | `create_options` |
| `COMPRESSION='zlib'` | rc=0 | `create_options` |
| `ENCRYPTION='Y'` | **ERROR 3185** "Can't find master key from keyring" (env-dependent — a keyring-equipped server accepts) | none |
| `STATS_PERSISTENT=1`, `STATS_AUTO_RECALC=0`, `STATS_SAMPLE_PAGES=25` | rc=0 | `create_options` |
| `TABLESPACE=innodb_file_per_table` | rc=0 | none visible |
| `TABLESPACE=nonexistent_ts` | **ERROR 1812** (5.7) / **3510** (8.0/8.4) | none |
| `DELAY_KEY_WRITE=1`, `PACK_KEYS=1`, `AVG_ROW_LENGTH`, `CHECKSUM`, `MAX_ROWS`, `MIN_ROWS`, `INSERT_METHOD=NO`, `DATA DIRECTORY`, `CONNECTION` on InnoDB | rc=0 (accepted; MyISAM-scoped options are inert on InnoDB) | MyISAM-family ones land in `create_options` on MyISAM |
| `ORDER BY id` / `ORDER BY id DESC, s` | rc=0 | physical row reorder (invisible to catalog); `ORDER BY nonexistent_col` also **rc=0** — server does not validate the name |
| `FORCE` | rc=0 | rebuild |
| `DISABLE KEYS` / `ENABLE KEYS` | rc=0 both engines (real effect on MyISAM, inert on InnoDB) | none |

Multi-spec behavior:

- `ADD COLUMN, ENGINE=InnoDB` / `DISABLE KEYS, ADD COLUMN` / `FORCE, ADD COLUMN` / `ADD COLUMN, TABLESPACE=...` / `CONVERT TO ..., AUTO_INCREMENT=n` → **execute** on MySQL (probed 8.0/8.4).
- `ORDER BY id, ADD COLUMN` / `ORDER BY id, FORCE` / `ORDER BY id, ORDER BY s` → **1064 grammar refusal** on MySQL AND TiDB: `order_by_columns` cannot share a spec list.

TiDB v8.5.0:

| Spec | Outcome |
|---|---|
| `ENGINE=`(any name incl. nonsense) | nonsense → **1286**; `ENGINE=MyISAM` → rc=0 (catalog still reports InnoDB — compat display) |
| `charset`/`DEFAULT CHARSET`, `COLLATE`, `ROW_FORMAT`, `COMMENT`, `AUTO_INCREMENT` | rc=0 — accepted; `AUTO_INCREMENT=500` leaves seed 0 (**ignored**, unlike MySQL's clamp) |
| `KEY_BLOCK_SIZE`, `COMPRESSION`, `ENCRYPTION`, `STATS_*`, `TABLESPACE`, `PACK_KEYS`, `DELAY_KEY_WRITE`, `AVG_ROW_LENGTH`, `CHECKSUM`, `MAX_ROWS`, `MIN_ROWS`, `DATA DIRECTORY`, `CONNECTION` | **8200** "This type of ALTER TABLE is currently unsupported" |
| `INSERT_METHOD` | **8233** "CREATE/ALTER table with insert method option is not supported" |
| `CONVERT TO utf8mb4` | rc=0; `CONVERT TO latin1`/`utf8` → **8200** "Unsupported modify charset" |
| `ORDER BY`, `DISABLE KEYS`, `ENABLE KEYS`, `ALGORITHM`, `LOCK` | rc=0 |
| `FORCE` | **8200** "Unsupported/unknown ALTER TABLE specification" |
| `ADD COLUMN, ALGORITHM=COPY` | rc=0 — column added |
| `DISABLE KEYS, ADD COLUMN` | rc=0 — mixed multi-spec executes |

CREATE-side (TiDB): every table-option form probed accepted rc=0 and ignored
(`create_options` stays empty) — including `key_block_size`,
`compression='zlib'`, `encryption='Y'`, `stats_*`, `tablespace=`, MyISAM
`engine`. MySQL CREATE mirrors the ALTER column: `key_block_size=8` on
InnoDB **succeeds** (implies `row_format=COMPRESSED`), `encryption='Y'`
refuses 3185 (keyring), `compression`, `stats_*`, `tablespace` accepted,
MyISAM options accepted on MyISAM.

### 2.3 Version gates recorded

- `ALGORITHM=INSTANT`: exists only MySQL ≥ 8.0.12 (5.7 → 1800). Bounded T15
  claims: supported on {add_columns, drop_column, rename_column}
  (probed), refused on {add_index/add_constraint, modify_column} (1845
  probed); every other action stays **unproven**.
- `ALGORITHM=INSTANT` + any explicit LOCK → 1221 on ≥ 8.0.
- `ALGORITHM=COPY` + `LOCK=NONE` → 1846 on all MySQL.
- TiDB: no version gate inside v8.5.0 — every modifier cell accepted
  **except `add_index` + `ALGORITHM=INSTANT` → 1846 (probed)**; other
  INSTANT cells not in the probed-capable set stay unproven.

## 3. Normalized facts (spec additions)

1. **`spec.Alter.Options` new extracted keys** (declaration facts; the
   map keeps authored lowercase forms for enum-like values and authored
   text for names/numbers):
   - `key_block_size`, `stats_sample_pages`, `stats_persistent`,
     `stats_auto_recalc`, `pack_keys`, `delay_key_write`,
     `avg_row_length`, `checksum`, `table_checksum`, `max_rows`,
     `min_rows`, `data_directory`, `index_directory`, `connection`,
     `password`, `insert_method` → decimal text (`STATS_*=DEFAULT` and
     `PACK_KEYS=DEFAULT` record `"default"`).
   - `compression`, `encryption`, `tablespace`, `storage_media`,
     `secondary_engine`, `union` → authored string.
   - `convert_to_charset` / `convert_to_collate` — set **instead of**
     `charset`/`collate` when the charset option carries the
     `TableOptionCharsetWithConvertTo` flag (the AST marks CONVERT TO).
     `DEFAULT CHARACTER SET` keeps `charset`/`collate`.
   - `algorithm` / `lock` already extracted (unchanged).
2. **`spec.Alter.OrderBy []spec.OrderByItem`** — new typed field on the
   `order_by_columns` action: `{column string, desc bool}` in authored
   order (structure captured even though the server does not validate the
   names — probed `ORDER BY nonexistent_col` rc=0).
3. **`spec.TableSnapshot.Options`** — no new keys this round: catalog
   projection stays `engine/row_format/auto_increment/table_rows/
   collation/charset`; `Table.Comment` is already projected. `create_options`
   string members (KBS/compression/STATS_*) are **not** catalog facts the
   provider models — declared-only.

## 4. Shared judgment — `spec.OnlineDDLOutcome`

A three-way classifier in `internal/domain/spec` (same home as
`CheckAlterSyntaxOutcome` / `RenameColumnVersionSupportFor`), consumed by
the two MySQL rules, coverage, and ordered state — never by TiDB:

```
OnlineDDLOutcome(version *VersionIdentity, engine string,
                 actions []string, algorithms []string, locks []string)
  → Refused(reason) | Combinable | Unproven(reason)
```

MySQL matrix (each cell is §2 evidence):

- `algorithms`/`locks` collect the authored values in spec order; **last
  wins** for duplicates (probed). Unknown/other values → Unproven.
- `algorithm=instant`: version nil/unvalidated → Unproven(gap facts);
  5.x → Refused(1800); 8.x → per-action at the **documented floors**
  {add_columns ≥8.0.12, drop_column ≥8.0.29, rename_column ≥8.0.3} →
  continue; refused set {add_index, add_constraint, modify_column,
  change_column} → Refused(1845); a capable action below its documented
  floor → Refused (1845-class); anything else → Unproven.
- `algorithm=instant` + any explicit lock (none/shared/exclusive) →
  Refused(1221). `lock=default` is not "explicit lock" for this cell.
- `algorithm=copy` + `lock=none` → Refused(1846).
- `lock=none` (no algorithm): combinable only on probed cells — actions
  ⊆ {add_columns, add_index/add_constraint} with engine ∈ {innodb}; any
  other action or engine → Unproven; engine unknown → Unproven(engine).
- `algorithm=inplace` + `lock=none`: same cell coverage as lock=none
  above (probed on InnoDB add_column/add_index).
- `algorithm=inplace|copy` or `lock=shared|exclusive` alone →
  Combinable (accepted cells). `inplace` is NOT claimed online — see rule.
- `algorithm=default` / `lock=default` / absent → Combinable.
- Combinations not reachable by the above cells → Unproven.

TiDB: `algorithm`/`lock` specs are ignored declarations → Combinable
for every accepted cell (the one probed refusal is `add_index` +
`ALGORITHM=INSTANT` → 1846, adjudicated refused); the statement's real
actions decide state.

## 5. Ordered-state publication

- `table_option` single action: proven refused cells (TiDB vendor
  options, MySQL KBS-on-non-compressed-InnoDB, `ENGINE=` unknown-name is
  *not* statically provable → unproven) keep the exact pre-state;
  proven-executed catalog-mapped declarations publish precise updates
  (`engine`, `row_format`, `collation`+derived `charset`, `comment`,
  `auto_increment` when declared > snapshot and an auto-increment column
  exists); `DEFAULT CHARACTER SET x` without `COLLATE` publishes
  `charset=x` and **unvouches** `collation` (charset default collation is
  version-dependent, unmapped); `CONVERT TO` publishes table charset/
  collation and every string column's charset/collation where provable —
  charset declared + collate absent unvouches column collation rather
  than guessing; `AUTO_INCREMENT` clamp semantics (lower-than-seed =
  no-op publish unchanged); runtime-dependent cells (encryption,
  tablespace name) → Unproven → bounded invalidation.
- `order_by_columns`/`force`/`enable_keys`/`disable_keys`: MySQL executes
  → publish unchanged shape (row order and key-maintenance are not
  modeled facts; a rebuild leaves members intact). TiDB `force` → vendor
  refusal → pre-state. TiDB order_by/keys accepted → unchanged shape.
- `algorithm`/`lock` specs: no own transition; they gate the statement
  outcome only. Modifier-only alters (`ALGORITHM=COPY` alone) publish the
  unchanged entry when combinable.
- Multi-action: `prepareMultiAction` skips modifier/option specs in the
  primary-object ledger; before applying, the statement-level outcome is
  decided — online-DDL refused → keep exact pre-state; option-level
  refusal (TiDB 8200 vendor option, probed 1478 cell) → keep pre-state;
  `order_by_columns` with any sibling → refused (probed 1064); `force`+
  structural siblings executes on MySQL (probed) → publish actions then
  unchanged option effects; TiDB `force` anywhere → refused (8200).
  Unproven → `invalidate(primary)` exactly as today.
- `table_option` inside multi-action: publishes its catalog-mapped
  updates onto `working` after the structural actions apply.

## 6. Rule inventory (frozen IDs & defaults)

| Rule ID | Level | Owns |
|---|---|---|
| `ddl.alter.online_ddl.applicability` | blocker | MySQL refused ALGORITHM/LOCK combinations (§4 Refused cells). EvidenceGaps: `target.version` / `target_table.engine` when a required fact is missing. |
| `ddl.alter.online_ddl.warn` | warning | Combinable-but-risky declarations: `algorithm=copy` (full copy + writes blocked), `lock=exclusive` (no concurrent access), `lock=shared` (writes blocked), `algorithm=inplace` ("not an online guarantee"). No finding on instant/default/none or TiDB. |
| `ddl.tidb.alter.compat_options.notice` | notice | TiDB ALTER carrying ignored compat declarations — `algorithm`/`lock` modifiers, `order_by_columns`, `enable_keys`/`disable_keys`: parsed for compatibility; TiDB's own DDL engine decides execution (probed accepted; the refused `add_index`+INSTANT cell reports through `ddl.alter.online_ddl.applicability`, not this notice). |
| `ddl.alter.table_option.compatibility.require` (existing) | warning | extended: adds `comment` compare (`Table.Comment`) + `default_charset` semantics; never touches `convert_to_*` keys. |
| `ddl.alter.charset_conversion.warn` | warning | MySQL/TiDB-accepted `convert_to_charset` declarations: rewrites every string column (count + names from snapshot when present). |
| `ddl.alter.charset_conversion.applicability` | blocker | TiDB refused conversions: target outside utf8mb4-family is refused-probed ({latin1, utf8}); utf8mb4 accepted; other pairs → unproven gap. |
| `ddl.alter.rebuild.warn` | warning | `order_by_columns`, `force` on MySQL (rebuild + row-order impact). TiDB order_by → silent-compat (accepted); TiDB force → vendor aspect (not this rule). |
| `ddl.alter.keys_toggle.notice` | notice | `disable_keys`/`enable_keys`: MyISAM → real index-maintenance effect; InnoDB/other → accepted-but-inert; unknown engine → gap `target_table.engine`. |
| `ddl.alter.table_option.applicability` | blocker | MySQL proven option refusals: `key_block_size` on engine=InnoDB with row_format ∉ {compressed} → refused (probed 1478). Needs `target_table.engine`, `target_table.row_format` facts → gaps. |
| `ddl.alter.table_option.warn` | warning | Accepted-but-risky declarations: `compression` (storage format), `encryption` (runtime keyring precondition — observed 3185), `tablespace` (runtime existence precondition — observed 1812/3510), `key_block_size` accepted cells, `insert_method` (MyISAM-only; inert elsewhere). |

Rules not added: no TiDB CREATE-side notice (consistent with the existing
`ENGINE=MyISAM` silent-compat precedent — the TiDB-ignored CREATE options
are captured declarations with zero effect, documented here); no
`order_by`/`keys` TiDB findings (accepted-compat, same precedent).

### 6.4 Option-consumption matrix (per-dialect, per-op)

`alterTableOptionOutcome(dialect, optionName)` in coverage:

- **MySQL alter**: catalog keys {engine, charset, collate, row_format,
  auto_increment, comment} consumed by the compat rule/state;
  `convert_to_*` by the conversion rules/state; `key_block_size` by
  applicability+warn; `compression/encryption/tablespace/insert_method`
  by warn; the long tail per the §6.5 bounded cut (extracted + gap).
- **MySQL create**: the six catalog keys stay consumed (T06 rules); the
  newly extracted options {key_block_size, compression, encryption,
  insert_method, tablespace + §6.5 tail} keep bounded
  `create_table.option.<name>` gaps — declarations now captured with
  values, per-option CREATE semantics remain unaudited.
- **TiDB create**: extracted options are parsed-ignored compat
  declarations → same bounded `create_table.option.<name>` gaps
  (not vendor — accepted, not refused).
- **TiDB alter**: accepted set {engine, charset, collate, row_format,
  comment, auto_increment, convert_to_*} consumed; refused set
  {key_block_size, compression, encryption, stats_*, tablespace,
  pack_keys, delay_key_write, avg_row_length, checksum, table_checksum,
  max_rows, min_rows, data_directory, index_directory, connection,
  password, insert_method(8233), storage_media, secondary_engine, union}
  → **vendor-boundary aspects** `alter_table.option.<name>`
  (`vendor=true`), and the statement keeps its pre-state (T14
  vendor-refusal convention extended to option level).

### 6.5 Bounded cut for the long tail

`stats_*`, `pack_keys`, `delay_key_write`, `avg_row_length`, `checksum`,
`table_checksum`, `max_rows`, `min_rows`, `data_directory`,
`index_directory`, `connection`, `password`, `storage_media`,
`secondary_engine`, `union` declarations are **extracted** into
`Alter.Options`/`DDL.Options` with provenance but carry no audited
semantics: MySQL → bounded `alter_table.option.<name>` /
`create_table.option.<name>` gaps (unaudited, same feature names the
`UnextractedOptions` path used — extraction upgrades evidence quality,
not the verdict); TiDB alter → vendor aspect (refused-probed); TiDB
create → bounded gap (parsed-ignored). They do NOT get per-option
catalog modeling or fake silent-accept findings (deferred boundary,
documented).

Parser-faithfulness notes for the bounded cut:

- `stats_persistent` and `pack_keys`: the TiDB-parser grammar
  (`StatsPersistentVal`) discards the authored `0|1|DEFAULT` value —
  extraction records the presence marker `"declared"`, never a
  fabricated numeric.
- `secondary_engine` additionally normalizes `SECONDARY_ENGINE=NULL`
  to the value `"null"`.
- `insert_method` and `storage_media` values are keyword-typed and are
  lower-cased like `algorithm`/`lock`; identifier-typed values
  (`engine`, `tablespace`, `secondary_engine`, `union` member names,
  `connection`, `password`, directories) keep authored spelling.

## 7. Coverage

- `algorithm`, `lock` join `auditedAlterActions*` — consumed by the
  online-DDL machinery on both dialects. On statements with a partition
  action the T14 `partitionCombinationProven` path keeps ownership
  (unchanged).
- `order_by_columns`, `force`, `enable_keys`, `disable_keys` join the
  MySQL audited set. On TiDB: `force` joins `tidbVendorRefusedPartitionActions`
  — renamed concept `tidbVendorRefusedAlterAspects` (8200 vendor
  boundary); `order_by_columns`, `enable_keys`, `disable_keys` join the
  TiDB audited set (accepted-compat).
- Unproven online-DDL combinations keep a bounded aspect
  `alter_table.online_ddl` (aspect=action) — the capability boundary is
  never laundered into pass.
- TiDB-refused `table_option` options → `alter_table.option.<name>`
  vendor aspects (new per-option vendor classification).
- `convert_to_*` extracted keys are consumed (never gaps).

## 8. First Golden Path (frozen oracle — #94 card)

Input: `ALTER TABLE t ADD COLUMN c INT, ALGORITHM=COPY, LOCK=EXCLUSIVE;`
Anchor: MySQL 8.0.46, real InnoDB fixture `t(id INT PK, s VARCHAR(20))`,
`c` absent, observed product/version/engine.

Oracle:

- Normalized parts: `add_columns(c)` + `algorithm=copy` + `lock=exclusive`.
- Findings: exactly 1 — `ddl.alter.online_ddl.warn` level `warning`,
  message names COPY + EXCLUSIVE risk; metadata carries `algorithm=copy`,
  `lock=exclusive`, `action=add_columns`-scoped payload; location = the
  original statement line (not reordered normalized SQL).
- Coverage: `complete`; zero unsupported aspects; zero evidence gaps
  (version observed 8.0.46, engine InnoDB observed).
- Verdict: `warning`-level verdict (not pass — the warn finding exists;
  not reject — the combination executes).
- Successor: `c` projected present (combinable outcome → action applies);
  `id`, `s`, PK intact; `table_rows` invalidated per rebuild convention.
- CLI: exit 0 with the warning finding in output (warning ≠ reject).

## 9. Golden manifest sketch (~20 cases)

Seed + refusal arm (1846, 1221, 1800-on-5.7, 1845-on-index), risk arm
(copy alone, exclusive alone, inplace-not-guarantee), combinable
(instant add on 8.0, lock=none InnoDB), unproven-boundary (instant on
unprobed op → aspect), missing-prerequisite (no version → gap), TiDB
arms (copy-ignored notice, all-modifiers-accepted), table-option arms
(engine change warning+state, convert warn+state, TiDB convert refuse
blocker, KBS-on-compact-InnoDB 1478 blocker, TiDB-8200 vendor aspect,
AUTO_INCREMENT clamp state, ORDER BY warn, FORCE warn, keys engine-gated
notice, multi-spec option+action integrity, stats_* declaration), rule-off
representatives, successor-state arms (sibling reads updated facts).
Final denominator comes from the frozen manifest — not pre-filled.

## 10. Deferred boundaries (explicit)

- `create_options`-member catalog modeling (KBS/compression/STATS_* post
  state) — declaration-only this round.
- Charset→default-collation resolution table — `DEFAULT CHARSET` w/o
  COLLATE unvouches collation rather than resolving it.
- ENCRYPTION keyring / general-TABLESPACE existence — runtime
  preconditions outside metadata scope; warn finding + unproven state.
- LOCK granularity/duration guarantees — never claimed.
- TiDB `order_by`/`keys` real engine effects — accepted-compat, unmodeled.
- `INSERT_METHOD` on MyISAM real effect — declaration-level only.
- Instant-capable actions beyond {add_columns, drop_column,
  rename_column} — unprobed cells stay Unproven, never inferred.
