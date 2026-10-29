# Decision: Table options, charset conversion, and online-DDL modifier governance (MySQL/TiDB ALTER)

Date: 2026-11-08 — milestone decision-record convention: decision dates
are assigned per task slot and run ahead of commit dates (this record
committed 2026-10-29); neighboring records follow the same convention
(e.g. 2026-11-07 committed 2026-10-28).
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (issue #79, task T15 under #94)
Related commits: this task's commits on the workfree line after `1813f062`
Related tests: `internal/application/audit` corpus + T15 seam tests
(`batch_state_t15_test.go`),
`internal/infrastructure/parser/tidb/extractor_t15_test.go`,
`make ddl-golden TASK=T15 ARTIFACT_DIR=...`, `testdata/ddl-golden/T15.json`
Related docs: `docs/work-plans/t15-table-options-online-ddl/CONTRACT.md`,
native probe logs `docs/work-plans/t15-table-options-online-ddl/probes/`,
`docs/decisions/2026-08-30-mysql-tidb-alter-index-action-normalization.md`
(whose ALGORITHM/LOCK deferral this closes), module README updates in the
same change

## Context

Before T15, `ALGORITHM=`/`LOCK=` alter modifiers — explicitly deferred by
the 2026-08-30 index-action normalization decision ("no ALGORITHM/LOCK
safety policy or finding") — surfaced as unsupported aspects, and the
`mysql.create-table-options`, `mysql.alter-table.online-ddl`,
`mysql.alter-table.options`, and `tidb.alter-table.online-options`
inventory rows carried per-option evidence gaps for declarations the
parser already recognized. The p15 probes froze the matrix on MySQL
5.7.44 / 8.0.46 / 8.4.10 and TiDB v8.5.0:

- MySQL online DDL: `ALGORITHM=INSTANT` is refused 1800 on 5.7 and
  version-floored per action on 8.x (ADD ≥8.0.12, DROP ≥8.0.29, RENAME
  ≥8.0.3); `INSTANT` + explicit `LOCK=` refuses 1221; `COPY` +
  `LOCK=NONE` refuses 1846; unknown ALGORITHM values refuse 1800;
  `INPLACE` is accepted but never an online guarantee.
- TiDB accepts `COPY`/`INPLACE`/`DEFAULT` and every `LOCK=` level as
  ignored compat declarations — including with structural siblings —
  with one probed refusal: `ADD INDEX … ALGORITHM=INSTANT` raises 1846
  (the same error class MySQL raises for an unsatisfiable lock). Other
  unprobed INSTANT cells stay unproven.
- `ORDER BY`, `ENABLE|DISABLE KEYS`, `FORCE`, `CONVERT TO`, and the
  `DEFAULT CHARSET`/`COLLATE` vs `CONVERT TO` distinction are separate
  declarations: CONVERT rewrites every string column; a bare charset
  option changes only the table default (collation resets to the
  charset's default).
- TiDB refuses `CONVERT TO` latin1/utf8 (8200), `FORCE` (8200),
  `KEY_BLOCK_SIZE`/`COMPRESSION`/`ENCRYPTION`/`STATS_*`/`TABLESPACE` and
  the MyISAM long tail (8200; `INSERT_METHOD` 8233) — vendor-boundary
  refusals, not unaudited gaps.
- MySQL `KEY_BLOCK_SIZE` on a non-COMPRESSED InnoDB table refuses 1478;
  `compression`/`encryption`/`tablespace` are accepted declarations with
  runtime preconditions (probed 3185 keyring, 1812/3510 tablespace).
- Duplicate `ALGORITHM=`/`LOCK=` declarations resolve last-wins on both
  engines; multi-action ALTER is atomic — no prefix publishes.

## Decision

1. **One shared adjudication layer in `spec`** (`online_ddl.go`):
   `OnlineDDLOutcomeFor` returns the three-way verdict
   (combinable/refused/unproven + bounded `Refusal` class +
   `GapFacts`) for the statement's folded modifier pair
   (`OnlineDDLModifierValues`, last-wins) against the probed matrix;
   `TiDBCharsetConversionOutcome` shares the TiDB conversion verdict;
   `AlterTableOptionOutcome`/`TableOptionOutcome` classifies every
   extracted option name per dialect as consumed, vendor-refused, or
   unaudited. Rules, coverage, and ordered state consume the same
   answer — they can never disagree about whether a declaration was
   audited.
2. **Nine new rules** plus an extension of
   `ddl.alter.table_option.compatibility.require` (declared `collate`
   compares against the snapshot `collation`; `Table.Comment`
   compares): `ddl.alter.online_ddl.applicability` (blocker, proven
   modifier refusals), `ddl.alter.online_ddl.warn` (warning,
   combinable-but-risky cells), `ddl.tidb.alter.compat_options.notice`
   (notice, TiDB silently-accepted compat declarations — one finding
   per statement), `ddl.alter.rebuild.warn` (warning, ORDER BY/FORCE
   rebuild declarations), `ddl.alter.keys_toggle.notice` (notice,
   MyISAM-armed key toggles), `ddl.alter.charset_conversion.warn` /
   `.applicability` (warning on accepted rewrites / blocker on TiDB
   refused targets), `ddl.alter.table_option.applicability` (blocker,
   probed 1478 KBS refusal) and `ddl.alter.table_option.warn`
   (warning, accepted-but-risky option declarations).
3. **Extraction widened** to the bounded long-tail vocabulary so a
   recognized option never silently drops into `UnextractedOptions`:
   `key_block_size`/`compression`/`encryption`/`tablespace`/
   `insert_method`/`stats_*`/`pack_keys`/`delay_key_write`/
   `avg_row_length`/`checksum`/`table_checksum`/`max_rows`/`min_rows`/
   directory/connection/password/`storage_media`/`secondary_engine`
   (incl. `NULL`)/`union` — plus `Alter.OrderBy` members in clause
   order and the `convert_to_charset`/`convert_to_collate` keys that
   keep CONVERT distinct from default-setting options. The grammar
   discards `STATS_PERSISTENT`/`PACK_KEYS` authored values, so those
   record the presence marker `declared`, never a fabricated number.
4. **Coverage.** `algorithm`/`lock`/`order_by_columns`/`force`/
   `enable_keys`/`disable_keys` join the MySQL audited action set;
   unproven modifier cells keep the bounded `alter_table.online_ddl`
   aspect; TiDB `force` joins the vendor-refused aspect set while
   `order_by`/key toggles stay accepted-compat; TiDB-refused option
   keys become `alter_table.option.<name>` vendor aspects; unaudited
   keys keep bounded `alter_table.option.<name>`/
   `create_table.option.<name>` gaps.
5. **Ordered state.** A statement-level outcome gate runs before any
   structural mutation: proven refusal preserves the exact pre-state,
   unproven withholds the affected table, combinable publishes the
   structural actions then applies table-option effects last (so a
   CONVERT rewrites columns the same statement added). Option
   publication is fact-faithful: engine/charset/collation/row_format/
   comment publish verbatim, `AUTO_INCREMENT` only publishes an
   increased seed (lower clamps to a no-op), a bare charset
   declaration does not fabricate a surviving collation, CONVERT
   rewrites every modeled string column, and `IsPartitionAlterAction`
   gives the partition-action vocabulary one canonical home in spec.

## Rationale

Option declarations, online-DDL modifiers, and conversion outcomes are
the same class of fact: an authored declaration whose native verdict
depends on product × version × engine × action. Centralizing the verdict
in spec keeps the three consumers (findings, coverage aspects, state
transitions) from re-deriving — and possibly contradicting — each
other's judgment, which is exactly the failure the T13/T14 shared
adjudicators (`LessThanPairOutcome`, `partitionCombinationProven`)
already proved out. Extracting the long tail instead of leaving it in
`UnextractedOptions` upgrades evidence quality (a named option with a
value beats a feature string) without pretending per-option semantics
exist.

## Public Contract

- `ddl.alter.online_ddl.{applicability,warn}`,
  `ddl.tidb.alter.compat_options.notice`, `ddl.alter.rebuild.warn`,
  `ddl.alter.keys_toggle.notice`,
  `ddl.alter.charset_conversion.{warn,applicability}`, and
  `ddl.alter.table_option.{applicability,warn}` are shipped, enabled,
  and appear in `configs/deltascope.example.yaml` at the levels above.
- Findings carry bounded metadata (`action`, `option`, `value`,
  `engine`, `row_format`, `algorithm`, `lock`, `refusal`) and locate on
  the original SQL — never normalized-reordered text.
- `UnsupportedDetail` feature IDs are stable:
  `alter_table.online_ddl` for unproven modifier cells and
  `alter_table.option.<name>` / `create_table.option.<name>` for
  extracted-but-unaudited or vendor-refused options.
- Successor statements read the published post-state (added columns,
  converted charsets, clamped seeds) through the same ordered-state seam
  every transport shares; SDK/CLI/HTTP/MCP see identical verdicts,
  coverage, and gaps.

## Deferred / Out Of Scope

- TABLESPACE lifecycle, IMPORT/DISCARD TABLESPACE, and engine runtime
  semantics stay T25; only declaration capture + bounded gaps here.
- No lock-duration/timing claims; `INPLACE` is never reported as an
  online guarantee.
- No charset ordering/collation-arithmetic or arbitrary data conversion
  solving; CONVERT models columns/defaults, not row bytes.
- TiDB CREATE-side option notices stay unadded (silent-compat
  precedent); the long tail carries no per-option catalog modeling.
- T14's INTERVAL/subpartition/row-routing deferrals are unchanged.

## Verification

- Golden: `make ddl-golden TASK=T15` — 42/42 cases, 470 assertions
  across mysql 5.7.44/8.0.46/8.4.10 + tidb v8.5.0, including the frozen
  seed (`ADD COLUMN c INT, ALGORITHM=COPY, LOCK=EXCLUSIVE`), refusal
  matrix, TiDB vendor boundaries, conversion, option publication, and
  policy-off controls.
- Unit: `internal/application/audit` T15 seam tests (19 tests),
  extractor T15 tests, corpus updated for promoted aspects.
- Gates: `go test ./...`, `make ddl-inventory-gate`,
  `make ddl-coverage-catalog-test`, `config lint` on the example config.

## Consequences

- Future ALTER modifiers or option semantics extend the spec
  adjudication tables, not parallel rule/state logic.
- Any newly extracted option must pick a `TableOptionOutcome` cell —
  silently extracting without classification would reintroduce the
  drop-into-unextracted failure this decision removed.
