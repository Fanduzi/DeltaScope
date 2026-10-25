# T11-R2-tail closeout — product `27edfea2`

Parent chain: `efc004c7` (R2 evidence) → `27edfea2` (tail product) →
tail evidence commit. Scope is exactly the two reviewer-located defects
and the bounded evidence corrections — no design reopening, no lazy-
scan expansion, no T12 work.

## What shipped

### F-tail — unknown/empty index kind cannot vouch survival

`applyDropCheck`'s `FindProvenIndex` arm bound on name+existence and
unvouched only `IndexKindUnique`. A proven name carrying
`IndexKindUnknown` (or an empty kind) may still BE the unique member
`DROP CONSTRAINT` removes, so it stayed bindable — the successor
`DROP INDEX` answered present and skipped the owed gap.

Fix (batch_state_fk.go, one condition): only a proven
`IndexKindSecondary` member keeps the bound arm — it provably refuses
under both keywords (3821/3940 on 8.0.46/8.4.10, probed on
still-present objects). Every other kind — unique, unknown, empty, and
the unprobed specialized kinds — takes `unvouchIndexMember`: the member
projection drops and the index set unvouches, so name-keyed readers
owe the attributed `target_table.indexes` gap instead of answering
either way. Unaffected members, the PK/constraint arms, and snapshot
isolation are untouched; no global tombstone was needed because the
existing per-member unvouch already expresses the precise loss.

Successor shape (asserted on the public Audit path and on the enriched
`Metadata.TargetTable`): zero findings, the attributed index gap,
`CoverageUnverified`, `VerdictReview`; the pre-state carries
`IndexesUnknown` with `uk_v` unbindable and `ix_u` intact.

### A-tail — RENAME finding attribution

`orderedColumnFact`'s MySQL rename pass-through returned the pre-state
`spec.Column` under the old name, so `RENAME COLUMN a TO id, ADD
PRIMARY KEY(id)` attributed the finding to `a`. The resolved value is
now a copy rebound to the new member name — type/attributes still
proven from `a`, the provider object never mutated, no broader ALTER
semantics added.

## Verified vs not verified

- Verified: both red cells above are first-run reds on `988560c2`
  (`tail_f_a_red_988560c2.log`, rc=1, 4 markers); green bound to
  `27edfea2` (56 PASS / 48 leaves focused, package suites, race).
- Verified: 17 gate groups — 16×rc=0, lint rc=1 = the accepted
  15-issue baseline with zero hits on the touched files; gofmt reports
  only the two accepted T07 baseline files.
- Verified: three goldens serial on shared anchors — T09 125/125
  (3442 assertions), T10 126/126 (3175), T11 40/40 (505), all
  head_sha/cli-build-sha = `27edfea2`, identical cli sha256
  `89073e28…`.
- Verified: `r2_tail_probe.log` fills the missing native cells
  (MySQL 8.0.46/8.4.10 same-statement A cells in both action orders;
  TiDB 1072/8200; TiDB partial UNIQUE/PK/secondary acceptance) plus
  the corrected still-present F refusal cells.
- Not verified (kept honest): `DROP CHECK`/`DROP CONSTRAINT` refusal
  for kinds beyond secondary is unprobed → conservative unvouch arm;
  lazy dependency scanning unchanged; real CHECK semantics remain T12.

## Evidence corrections in this delivery

- `red/e_binding_replay_d0c7cf33.log` — marked REPLAY on untouched
  `d0c7cf33` product + verbatim final test files; rc=1 fabricated
  `exists:false`. The `baseline_red.log` E-binding entry is re-labelled
  (old fixture exercised the accepted lazy boundary), preserved
  byte-for-byte.
- Counts restated: original red = 25 FAIL markers / 17 parents /
  21 failing leaves; concentrated green = 52 PASS / 45 leaves on
  `988560c2` (tail suite = 56 PASS / 48 leaves on `27edfea2`).
- Wrong-order native error recorded per arm: OFF 1822 / ON 6125.
- TiDB self-reference PK cell = clustered / 8200 (earlier
  NONCLUSTERED/1553 wording corrected).
- `r2_probe2.log` post-removal `DROP CHECK` cells caveated; the
  still-present refusals live in `r2_tail_probe.log`.

## Remote / lifecycle status

Pushed fast-forward to `origin/workfree` after this commit; live remote
SHA, parent chain, and evidence readability verified (see final
report). `origin/milestone/mysql-tidb-ddl-completion` untouched.
Compose anchors used for probes/goldens are disposable fixtures.
