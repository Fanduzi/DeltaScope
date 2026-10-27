# T13-R2 evidence bundle — 0cfa42a5

Acceptance-candidate evidence for T13-R2 (#92 remediation): partition
catalog post-state/completeness/bounds, EXCHANGE counterpart comparison
validity, metadata mounting, and modifier coverage boundary. Product
commit `0cfa42a5a4990afdf51ca40013aa8eab5e261ae8` (parent `c8f8948c`,
the T13-R1 evidence-bundle commit). Contract amendment:
`docs/work-plans/t13-partition-risk/CONTRACT.md` §10 (frozen before
implementation); ADR:
`docs/decisions/2026-11-05-ddl-partition-r2-remediation.md`.

## Scope delivered

- **A — catalog post-state, completeness, bounds:**
  `applyPartitionOperation` no longer republishes the pre-state member
  directory after DROP. `dropPartitionMembers` returns
  `([]string, provable)` — every authored name must be proven present
  and at least one member must survive, otherwise the exact pre-state is
  preserved (native atomic refusal: missing name → 1507, last member →
  refusal). A proven drop removes exactly the named members
  case-insensitively; TRUNCATE/EXCHANGE keep the directory and only
  clear data-derived statistics. Completeness: `partitionMemberNames`
  returns nil for subpartitioned declarations, `RANGE ... INTERVAL`,
  `PARTITIONS 0`, counts above the bounded max 8192, and definitions
  containing nested subpartitions; `derivedCreateShape` preserves a nil
  `PartitionNames` instead of collapsing it to known-empty. Provider
  `loadPartitionFacts` now selects `subpartition_name` too — any present
  subpartition marks the top-level list incomplete (`Partitioned`/
  method stay known, `PartitionNames` nil).
- **B — comparison validity:** `exchangeComparisonFacts` is the single
  shared walk for `Evaluate` and `EvidenceGaps`. Effective-type
  normalization: integer display widths (`INT(10)` vs `INT(11)`) and
  defaulted `CHAR`/`DECIMAL`/`BIT` parameters no longer fake a 1736.
  PK comparison honors explicit/proven absence — unrelated
  `ConstraintsUnknown` no longer reopens a proven PK-absent target;
  genuinely unproven PK blocks only the PK dimension, while an
  independently proven column-count mismatch still emits 1736. Each
  enabled rule owns only the facts it consumes (target rules now report
  `target_table.partitioned`/`partitions`/`existence` themselves
  instead of relying on the applicability rule). Engine applicability is
  dialect-scoped: MySQL compares `Options["engine"]` when both sides are
  observed (mismatch → `counterpart_engine_mismatch`, native 1497;
  unobserved side → `exchange_table.engine`/`target_table.engine` gap);
  TiDB does not consume the engine label because the catalog normalizes
  it (probed: `ENGINE=MyISAM` stores `InnoDB`, EXCHANGE executes).
- **C — metadata mount + coverage boundary:** `enrichStatements` mounts
  `Metadata` when `ExchangeTable` alone is present (no schema/instance/
  target/version needed); declaration-side counterpart facts reach the
  rule instead of degrading to `exchange_table.existence` unknown.
  `coverage.go` only lifts the unsupported `algorithm`/`lock` aspect
  when the extracted `partitionMixedOutcome`/`partitionModifierOutcome`
  classifier reaches a proven result for the enriched statement version;
  `ALGORITHM=COPY`, non-DEFAULT locks, multi-modifier mixes, and
  version-gated mixes without a version fact keep the unsupported aspect
  on the current statement (incomplete/unsupported, not silently
  complete).

## Red baseline

`red/` — replayed while the R2 test files sat on the pre-R2 product
(`c8f8948c` line); all failures are behavioral, none compile-driven:

- `extract.log` — records exactly one failure:
  `TestT13R2UnboundedPartitionsNeverExpands` panics on the base with
  `makeslice: cap out of range` (the A3 bound). The run was cut short
  by the panic, so no other extractor-level semantic red is recorded in
  this log.
- `provider.log` — both recorded failures are the same mock-column
  shape error: `TestT13R2SubpartitionedCatalogKeepsNamesUnobserved` and
  `TestT13R2PlainCatalogStillComplete` each fail at Scan with
  `expected 3 destination arguments in Scan, not 2` (the fixture feeds
  the new 3-column query while the base reads 2). Neither test reached
  its directory-semantics assertion on the base — recorded as
  fixture-shape reds, not semantic assertions.
- `rules.log` — B-class reds: PK absence reopened by
  `ConstraintsUnknown`, column-count mismatch swallowed by the PK
  knownness gate, engine comparison absent, target rules not owning
  their premise gaps.
- `public.log` — A1 successor TRUNCATE answered with a stale directory;
  C1 counterpart-only metadata not mounted; C2 `ALGORITHM=COPY` mix
  reported current-statement complete.

## Gates

`gates/` — 18 command tuples (`.command.txt`/`.stdout.txt`/
`.stderr.txt`/`.rc.txt`): 17 × rc=0; `lint` rc=1 carries the same
13-issue baseline as the R1 bundle (unchecked assertion in
`metadata_rules.go`, gocritic/staticcheck in T07 tests and older domain
files — zero new issues in this diff). `gofmt` lists only the two
accepted T07 files. `ddl-golden-validator-test` and
`ddl-inventory-gate` were re-run against the final product SHA after
the manifest correction (the other 16 tuples were produced on the
identical code; only `testdata/ddl-golden/T13.json` expectation values
changed between `e86d2a18` and `0cfa42a5`).

## Golden

`golden/<TASK>/` — five serial runs sharing anchors, each bound to
`0cfa42a5` (`head_sha` and `cli.build.head_sha` in every artifact.json;
one CLI sha256 `98e8b71f6ccd98d16e4d6b35ab5413508b266a9a14fbc70cfd1a2a4b89de063a`
across all five). `cli.path` records the runtime build path; the binary
itself is not committed (`bin/` is gitignored) — rebuild from
`0cfa42a5` with `CGO_ENABLED=0 go build -o bin/deltascope ./cmd/deltascope`
and compare against the recorded sha256.

| Task | Cases | Assertions |
|------|-------|------------|
| T09  | 125   | 3442 |
| T10  | 126   | 3175 |
| T11  | 40    | 506  |
| T12  | 25    | 282  |
| T13  | 45    | 662  |

All PASS; `executed_count` equals the manifest `required_case_ids` set
for every task (verified set-equal, no missing, no extras);
`residual_containers=[]`, `compose_down_rc=0`. T09–T12 denominators are
byte-stable against the R1 bundle. T13 keeps the R1 38-cell set and adds
7 T13-R2 cells:

- `t13r2-cli-inplace-drop-noversion-unsupported` — offline CLI cell:
  `ALGORITHM=INPLACE, DROP` without a version fact keeps the
  unsupported `alter_table.algorithm` aspect and the
  `target_table.partitioned` gap on the same statement.
- `t13r2-mysql80-exchange-engine-mismatch` — MySQL 8.0: MyISAM
  counterpart → 1497 `counterpart_engine_mismatch`.
- `t13r2-tidb85-exchange-engine-normalized` — TiDB: declared-`MyISAM`
  counterpart stored `InnoDB` → EXCHANGE executes.
- `t13r2-mysql80-drop-then-truncate-absent` — the successor TRUNCATE
  answers 1735 against the preserved directory after the proven drop
  removed `p0` (the manifest expectation was corrected from 1507 to
  1735 after native probing; the product emitted 1735 all along).
- `t13r2-mysql80-drop-then-truncate-survives` — the proven drop shrinks
  the directory; the successor truncates survivor `p1` and the member
  row count is verified empty.
- `t13r2-mysql80-exchange-subpartition-leaf-gap` — subpartitioned
  target → `unknown_table_state` (`target_table.partitions`),
  review/unverified, never a fabricated 1735.
- `t13r2-tidb85-interval-generated-member` — online TiDB materializes
  the interval-generated member (`P_LT_10`, catalog uppercase) into
  `information_schema.PARTITIONS`, so the named TRUNCATE is
  complete/pass/0-gap and executes — distinct from the declared-side
  `RANGE ... INTERVAL` clause, which stays nil-directory.

## Native archives

`native/` — this round's bounded probe logs (anchors mysql
5.7.44/8.0.46/8.4.10 + tidb v8.5.0 as named per log):

- `p9_engine_exchange.log` — EXCHANGE with `ENGINE=MyISAM`
  counterpart: MySQL 5.7/8.0/8.4 → 1497; TiDB stores `InnoDB` for the
  declared label and executes — the dialect-scoped engine premise.
- `p10_tidb_validation_arms.log` — the TiDB three-arm rerun this round
  owes: absent clause → 1737, `WITH VALIDATION` → 1737,
  `WITHOUT VALIDATION` executes and the violating row 500 lands in
  `t13v`; each arm rebuilds the fixture independently with pre/post
  row-content and catalog dumps. Replaces the R1-era logs'
  missing-database (1049) TiDB section, which is retained in
  `probes/` with an annotation.
- `p11_subpartition_interval_bounds_inplace.log` — subpartitioned
  catalog shape (top-level + leaf names are distinct rows), TiDB
  `RANGE INTERVAL` materializing the `P_LT_10` member name into the
  catalog (the log did not query `PARTITION_EXPRESSION`/`INTERVAL`
  columns), `PARTITIONS 18446744073709551615` and `PARTITIONS 8193`
  both refused with 1499 on 8.0.46 with a post-refusal member count of
  0 — the 8192 boundary itself was never created — and
  `ALGORITHM=INPLACE` + TRUNCATE/EXCHANGE executing on all four
  anchors (the D2 success arms the classifier's combinable verdict
  relies on).
- Source scripts: `probe9_t13r2.sh`, `probe10_tidb_validation.sh`,
  `probe11_t13r2.sh` committed under
  `docs/work-plans/t13-partition-risk/probes/` alongside the logs.

## Verified / unverified boundaries

Verified this round: MySQL engine mismatch → 1497 (three anchors);
TiDB engine label normalization → EXCHANGE executes; TiDB validation
absent/WITH → 1737, WITHOUT lands violating rows; subpartition/interval
catalogs keep `PartitionNames` unobserved on both entry paths (the
online TiDB interval catalog does materialize member names — the
unobserved boundary holds on the declared-side clause and the
subpartitioned catalog); `INT(10)` vs `INT(11)` compatible;
`ALGORITHM=INPLACE` + TRUNCATE/EXCHANGE execute on all four anchors;
missing TRUNCATE member → 1735 (MySQL); `PARTITIONS` above the
catalog bound refused with 1499 (8193 and max uint64 observed refused
with an empty directory — the 8192 boundary itself was never
materialized).
Unverified and honestly surfaced: subpartition/interval member
operations themselves (gap, not finding); row-content compatibility
solving (clean compare still never claims the exchange validated);
`ALGORITHM=COPY`, non-DEFAULT `LOCK`, multi-modifier mixes,
`placement_policy` (kept on the unsupported/unproven path — T15
scope); TiDB versions outside the v8.5 anchor; INTERVAL expression
evaluation and T14 topology actions.
