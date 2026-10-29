# T15 evidence bundle — f9febe77

Independent-acceptance evidence for T15 (#94): table options, charset
conversion, and online-DDL modifier governance across the four
inventory rows (`mysql.create-table-options`,
`mysql.alter-table.online-ddl`, `mysql.alter-table.options`,
`tidb.alter-table.online-options`). Product commits `c761fa38` +
`f9febe77` (parent `1813f062`, the T14-R2-tail2 evidence commit —
acceptance pending). Contract:
`docs/work-plans/t15-table-options-online-ddl/CONTRACT.md`; ADR:
`docs/decisions/2026-11-08-ddl-table-options-online-ddl.md`.

## Scope delivered

- **Shared adjudication in `spec` (`online_ddl.go`).**
  `OnlineDDLOutcomeFor` returns combinable/refused/unproven plus a
  bounded refusal class and per-fact gap keys for the folded
  ALGORITHM/LOCK pair (last-wins on duplicates) per dialect;
  `TiDBCharsetConversionOutcome` and `AlterTableOptionOutcome`
  adjudicate conversion targets and every extracted option name. Rules,
  coverage, and ordered state read the same verdict — they cannot
  disagree about whether a declaration was audited.
- **Nine new rules + one extension.** `online_ddl.applicability`
  (blocker, proven refusals on both dialects incl. TiDB
  `add_index`+INSTANT 1846), `online_ddl.warn` (warning, risky
  combinable cells), `tidb.alter.compat_options.notice` (TiDB
  silently-accepted compat declarations, one finding per statement),
  `rebuild.warn` (ORDER BY/FORCE), `keys_toggle.notice`,
  `charset_conversion.warn` / `.applicability` (blocker on proven TiDB
  refusals), `table_option.applicability` (blocker, probed KBS-1478) /
  `.warn`; `table_option.compatibility.require` gained the declared
  `collate` compare and `comment` compare while never touching
  `convert_to_*` keys.
- **Extraction widened.** `Alter.OrderBy` ordered members, the
  `convert_to_charset`/`convert_to_collate` keys distinct from
  default-setting `charset`/`collate`, and the bounded long-tail
  vocabulary (key_block_size, compression, encryption, tablespace,
  insert_method, stats_*, pack_keys, delay_key_write, avg_row_length,
  checksum, table_checksum, max_rows, min_rows, directories,
  connection, password, storage_media, secondary_engine incl. NULL,
  union). Grammar-dropped values record `declared` presence markers,
  never fabricated numbers.
- **Coverage.** Audited actions and consumed options leave
  `UnextractedOptions`; unproven modifier cells keep the bounded
  `alter_table.online_ddl` aspect; TiDB `force` and the refused option
  set become vendor-boundary aspects; unaudited keys keep
  `alter_table.option.<name>`/`create_table.option.<name>` gaps.
- **Ordered state.** A statement-level outcome gate precedes all
  structural mutation: proven refusal preserves the exact pre-state,
  unproven withholds the affected table, combinable publishes the
  structural actions then `applyTableOptionEffects` last — a same-
  statement CONVERT rewrites the columns the statement added. Option
  effects are fact-faithful (AUTO_INCREMENT publishes only an
  increased seed; a bare charset declaration unvouches collation
  instead of inventing one; CONVERT rewrites every modeled string
  column); the multi-action ledger stays atomic and skips non-primary
  modifier/option specs.
- **Corpus.** 100% supported-rule coverage (694/694 targets) with the
  nine rules classified to their real dialects — mysql-only
  (online_ddl.warn, rebuild.warn, keys_toggle.notice,
  table_option.{applicability,warn}), mysql+tidb
  (online_ddl.applicability, charset_conversion.warn), tidb-only
  (compat_options.notice, charset_conversion.applicability).

## Red baseline

`red/` — captured on the unremediated baseline `1813f062` in the main
worktree before implementation: `go test -run 'TestT15'` recorded
**22 RUN / 18 FAIL** (rules unregistered → zero findings;
algorithm/lock unadjudicated → coverage incomplete; order_by and
conversion keys unextracted). `head_sha.txt` pins the baseline;
`go_test.stdout.txt`/`go_test.stderr.txt` archive the real output.

## Green run

`green/` — the same verbose matrix at the final product SHA: the T15
ordered-state tests (19 tests) and the extractor T15 tests both rc=0,
command + argv archived.

## Gates

`gates/` — 19 required gates + 2 committed-diff/final-tree L2 checks
+ 1 full-repo lint diagnostic + 1 codegraph impact analysis, all on
`f9febe77` (or the identical pre-commit tree), each with real argv,
stdout, stderr, and rc. See `gates/INDEX.md`: 18 required green,
`lint-scoped` at exactly the 13-issue accepted baseline (stdout
byte-identical to the T14 record), `gofmt` listing only the two
accepted T07 files, `lint-fullrepo` recorded verbatim as a 37-issue
diagnostic outside the accepted scope (one line-number shift only).

## Golden

`golden/` — seven serial runs on `f9febe77`, one CLI sha256
(`dbe0f1d1813ddb8b…`):

| Task | cases | assertions | result |
|------|-------|------------|--------|
| T09 | 125/125 | 3442 | PASS |
| T10 | 126/126 | 3175 | PASS |
| T11 | 40/40 | 506 | PASS |
| T12 | 25/25 | 282 | PASS |
| T13 | 47/47 | 697 | PASS |
| T14 | 69/69 | 1087 | PASS |
| T15 | 42/42 | 470 | PASS |

All previously accepted denominators frozen — no historical golden
cell was added, removed, or weakened; T15's 42/470 are new. All seven
artifacts: `head_sha` = `cli.build.head_sha` = `f9febe7771c4`,
required == executed, no duplicate IDs, `compose_down_rc=0`,
`residual_containers=[]`. The T15 run covers the frozen seed
(`ADD COLUMN c INT, ALGORITHM=COPY, LOCK=EXCLUSIVE` on real
InnoDB/MySQL 8.0.46), the MySQL refusal matrix, TiDB vendor
boundaries, conversion, option publication, and policy-off controls.

## Verified / unverified boundaries

Verified on this SHA: the full MySQL modifier matrix incl. seed and
all refusal cells on 5.7/8.0/8.4; TiDB accepted-compat cells plus the
one probed refusal (`add_index`+INSTANT 1846); CONVERT vs
default-setting charset on both dialects; option publication and
clamping; multi-action atomicity; exact pre-state on refusal and
withheld state on unproven premises; policy-off controls. Natively
observed on all four anchors; additionally unit-covered only: per-gap
attribution internals, snapshot-clone equality, and cells the contract
marks unproven without native arms (unprobed INSTANT actions,
non-utf8mb4-family collations, runtime-dependent
encryption/tablespace). Deferred unchanged: TABLESPACE lifecycle and
IMPORT/DISCARD (T25), lock-duration claims, `INPLACE` as an online
guarantee, charset ordering/arithmetic and row-byte conversion,
`stats_persistent`/`pack_keys` authored values (grammar-discarded —
presence recorded as `declared`), TiDB CREATE-side option notices,
T14's INTERVAL/subpartition/row-routing deferrals.
