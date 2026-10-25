# T11-R2-tail verification bundle — product `27edfea2`

Tail closure under issue #90 (parent #79): two reviewer-located defects
plus the bounded evidence corrections, in one delivery. Product diff is
`efc004c7..27edfea2` (6 files, +114/−7); this bundle's parent bundle is
`docs/verification/t11/988560c2abff75e52d31cf1a249712ae53a95eb8/`, which
also carries the R2 evidence-precision corrections landed in the same
delivery (native-probe README, red README, the E focused replay log, and
`r2_tail_probe.{sh,log}`).

## Red → green

| group | defect | red (pre-fix) | fix | green |
|-------|--------|---------------|-----|-------|
| F-tail | `applyDropCheck` bound any proven-name member that was not `IndexKindUnique` — an `IndexKindUnknown`/empty-kind member may still be the unique member `DROP CONSTRAINT` removes, so it stayed `FindProvenIndex`-visible and the successor `DROP INDEX` answered present | `tail_f_a_red_988560c2.log` (in the R2 bundle's `red/`): 3 markers over 2 new tests, rc=1 on `988560c2` | only a proven `IndexKindSecondary` hit keeps the bound arm; all other kinds take `unvouchIndexMember` | `green/t11r2-focused.stdout.txt`: successor owes the attributed `target_table.indexes` gap, `CoverageUnverified`, `VerdictReview`, zero findings; successor `Metadata.TargetTable` unvouched (`IndexesUnknown`, member not bindable, `ix_u` intact) |
| A-tail | `orderedColumnFact`'s MySQL rename pass-through returned the pre-state column object — finding attributed `column=a` for `RENAME COLUMN a TO id, ADD PRIMARY KEY(id)` | same log: `TestT11R2MySQLRenameCarriesPreStateFact` red on `988560c2` (`column=a`) | the resolved copy is rebound to the new member name; provider object never mutated | finding `column=id`, `type=int` (inherited from `a`) |

Focused suite on `27edfea2`: **56 `--- PASS` markers = 48 passing leaves**
(33 parents, 8 with subtests) — the R2 52/45 plus the tail additions.

## Gates (17 groups, all quads under `gates/`)

16 × rc=0. `lint` rc=1 carries the accepted 15-issue baseline — zero
findings on `batch_state_fk.go`, `common.go`, or the T11R2 test files.
`gofmt -l` reports only the two accepted T07 baseline files
(`batch_state_t07_matrix_test.go`, `batch_state_t07_test.go`).
`three-level-doc-diff` replicates the script rule against the committed
`efc004c7..27edfea2` file set (script reads the worktree only): every
changed source module's README updated → rc=0.

## Goldens (serial, shared anchors, frozen denominators)

| task | cases | assertions | head_sha | cli sha256 |
|------|-------|------------|----------|------------|
| T09 | 125/125 | 3442 | `27edfea2f41e…` | `89073e28…` |
| T10 | 126/126 | 3175 | `27edfea2f41e…` | `89073e28…` |
| T11 | 40/40 | 505 | `27edfea2f41e…` | `89073e28…` |

Artifacts: `golden/T09-artifact.json`, `golden/T10-artifact.json`,
`golden/T11-artifact.json` — `cli/build/head_sha` equals the product SHA
in all three.

## Evidence corrections (same delivery, archived in the R2 bundle)

- `native-probe/r2_tail_probe.{sh,log}` — cells run live but never
  archived: MySQL 8.0.46/8.4.10 `ADD COLUMN`+`ADD PRIMARY KEY` both
  orders + `MODIFY`+`ADD PRIMARY KEY` (all rc=0); TiDB v8.5.0 1072/8200;
  TiDB v8.5.0 partial UNIQUE/PK/secondary acceptance (rc=0, KCU lands);
  plus the corrected F cells — `DROP CHECK`/`DROP CONSTRAINT` on
  still-present objects (3821/3940, objects survive; real CHECK removed
  rc=0).
- `red/e_binding_replay_d0c7cf33.log` — marked REPLAY (not first-run):
  final fixture with leading child-load statement on untouched
  `d0c7cf33` product, rc=1, fabricated `exists:false` on statement 2.
  The historical `baseline_red.log` E-binding entry is re-labelled as
  the accepted lazy-boundary artifact, preserved byte-for-byte.
- Wording corrections: original red counts stated as 25 markers /
  17 parents / 21 failing leaves, green as 52 markers / 45 leaves;
  wrong-order native error per arm (OFF 1822 / ON 6125); TiDB
  self-reference PK cell = clustered / 8200 (not NONCLUSTERED / 1553);
  `r2_probe2.log` post-removal `DROP CHECK` cells caveated.

## Unproven boundaries kept honest

- Other index kinds' `DROP CHECK`/`DROP CONSTRAINT` refusal beyond
  secondary is unprobed — they take the conservative unvouch arm.
- Lazy dependency scan boundary unchanged (only loaded entries).
- Real CHECK constraint semantics still T12.
