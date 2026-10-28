# T13-R3 evidence bundle — 01525861

Acceptance-candidate evidence for T13-R3 (#92 remediation): DROP
directory publication consumes the action's own applicability,
EXCHANGE compares effective DECIMAL/NUMERIC parameters, and PK
presence is proven independently of member sequence. Product commit
`01525861559b5486e6f084265ed561451cc8f6d6` (parent `1f10ea99`, the
T13-R2 evidence-bundle commit). Contract amendment:
`docs/work-plans/t13-partition-risk/CONTRACT.md` §11 (frozen before
implementation); ADR:
`docs/decisions/2026-11-06-ddl-partition-r3-remediation.md`.

## Scope delivered

- **A — drop-applicability directory gating:**
  `applyPartitionOperation` consulted only the member facts (name
  present, member survives) before publishing a reduced directory; a
  proven `hash|key` method — a native 1512 refusal archived in p6 —
  still published `p0` as removed, so a successor TRUNCATE fabricated
  `partition_absent`/1735. The publication now consumes
  `partitionDropApplicability` (same `range|list` combinable /
  `hash|key` refused vocabulary the rule side carries): proven refusal
  keeps the exact pre-state, an unobserved or unmodeled method
  unvouches the directory alone (`PartitionNames = nil`) instead of
  publishing either an unproven removal or an unvouched retention, and
  only a proven combinable method publishes the member-minus-named
  set. The gate applies on the single-action route and both modifier
  routes (`ALGORITHM=DEFAULT` combinable, version-permitted `INPLACE`)
  — ordered-state semantics never depend on which rules are enabled.
- **B — effective DECIMAL/NUMERIC parameters:** the faithful-base
  normalizer folded only the bare `decimal` spelling, so `DECIMAL(12)`
  vs `DECIMAL(12,0)` fabricated 1736. `normalizeFaithfulColumnType`
  now folds `decimal`/`numeric` spellings to `base(M,D)` against
  native defaults (bare ≡ `(10,0)`, one parameter ≡ `(M,0)`) after
  consuming the trailing `unsigned` attribute word; a faithful form
  the bounded parser cannot project returns unproven and owes the
  column fact rather than choosing a verdict. Declared and catalog
  spelling channels share the same walk — declared `DECIMAL(12)`
  equals catalog `decimal(12,0)`. p12 archives the pairs on
  mysql80 + mysql57: every equivalence arm executes, every
  scale/precision/signedness control refuses 1736. The T06
  faithful-spelling contract is unchanged — normalization stays
  inside the rule.
- **C — PK presence vs member-sequence proof:**
  `primaryKeyIdentityFacts` read `PrimaryKey != nil` before checking
  `PrimaryKeyUnknown`, re-proving a residual pointer that
  `markPrimaryUnknown` had legitimately unvouched (the
  `DROP INDEX <unbound-name>` → `markPrimaryUnknown` chain). A
  proven-present placeholder (`Kind=Primary`, `NameUnknown`, nil
  `Columns`) was then compared member-for-member against a real member
  list, fabricating 1736 a second way. Presence and members now
  resolve independently: `PrimaryKeyUnknown` outranks the residual
  pointer on either side; a proven-present placeholder owes that
  side's `*_table.primary_key` fact (nil members are unproven, never
  an empty key); members compare only when both sequences are
  vouched; a proven presence-vs-absence pair still emits 1736
  independently; proven absence is still never reopened by
  `ConstraintsUnknown`.

## Red baseline

`red/` — captured on the unfixed product (this round's tests against
`1f10ea99` source); all failures are behavioral, none compile-driven.
The logs were not run with `-v`, so only the FAIL lines are archived
verbatim — the controls named below stayed green on the base and are
listed from the test source, not from per-leaf PASS lines:

- `audit.log` — `TestT13R3HashDropKeepsProvenDirectory`
  (hash + key): successor TRUNCATE fabricated `partition_absent`/1735
  over the wrongly-published removal. `…ModifierRoutesKeepDirectory`
  (ALGORITHM=DEFAULT and versioned INPLACE): same fabricated 1735 on
  both modifier routes. `…UnknownMethodDropUnvouchesDirectory`: the
  undecidable drop published a removal instead of a gap.
  `…ResidualPKPointerStaysUnproven`: fabricated
  `counterpart_definition_mismatch`/1736 from the unvouched pointer.
  `…ExchangeDecimalEffectiveTypePublic`: fabricated 1736 on the
  `DECIMAL(12)`/`DECIMAL(12,0)` pair. Control
  `…UnknownMethodDropAbsentNameKeepsDirectory` stayed green on the
  base.
- `rules.log` — `…CounterpartDecimalEffectiveParams`: three
  equivalence arms fabricated 1736 (declared M vs M,0;
  declared-vs-catalog spelling; unsigned default params). The
  `bare DECIMAL` vs `DECIMAL(10,0)` and `bare` vs `bare` arms are
  pre-existing passing controls — the R2 normalizer already folded
  the bare spelling — and every real-difference control (scale,
  precision, non-default bare, signedness) also stayed green.
  `…CounterpartUnnormalizableTypeOwesGap`: a `zerofill` faithful
  spelling chose a refusal instead of unproven.
  `…PKUnknownOutranksResidualPointer`: target-side unvouch fabricated
  1736; counterpart-side unvouch owed no `exchange_table.primary_key`.
  `…PKPresenceSeparateFromMemberProof`: placeholder-vs-proven
  members fabricated 1736 in both directions; both-placeholders owed
  no member facts. The presence-vs-absence control stayed green.

## Gates

`gates/` — see `gates/INDEX.md` for the authoritative index. The
18-name required set (mapped from the R2 bundle) is green or on its
accepted baseline:

- `lint-scoped` (the accepted four-package scope) reports the same
  13 issues as the R2 baseline — byte-identical, `lint-scoped-vs-r2.diff`
  empty; `ddl.go:476 typeDefFirst` unchanged. `lint` (the full-repo
  `./...` diagnostic) reports 37 issues and is recorded separately —
  it is not the gate scope and is not the baseline.
- `ddl-coverage-catalog-test` was first recorded under a
  `-tags postgresql`-less miscall (`[no tests to run]`); the make entry
  rerun and a `-v` record show `--- PASS: TestDDLCoverageCatalog`.
  The same test already ran inside `pg-unit-test-gates` — record-scope
  correction, not a new defect.
- `go_race` (CGO_ENABLED=1, full `./internal/...`) passes; the earlier
  two-package run is kept as a named partial-scope diagnostic.
- `three-level-doc` (working-tree L2 check — clean tree) and
  `three-level-doc-diff` (every `internal/**.go` changed in
  `1f10ea99..01525861` has its package README.md changed — all 4
  SYNCED) both pass.
- Extra diagnostics beyond the required set (`go-build`,
  `pg-unit-test-gates`, partial-scope race, full-repo lint) are
  listed separately and never counted toward the required gates.
- `gofmt` lists only the two accepted T07 files.

## Golden

`golden/<TASK>/` — five serial runs sharing anchors, each bound to
`01525861` (`head_sha` and `cli.build.head_sha` in every
artifact.json; one CLI sha256 across all five — see each artifact's
`cli.sha256`). `cli.path` records the runtime build path; the binary
itself is not committed (`bin/` is gitignored) — rebuild from
`01525861` with `CGO_ENABLED=0 go build -o bin/deltascope
./cmd/deltascope` and compare against the recorded sha256.

| Task | Cases | Assertions |
|------|-------|------------|
| T09  | 125   | 3442 |
| T10  | 126   | 3175 |
| T11  | 40    | 506  |
| T12  | 25    | 282  |
| T13  | 47    | 697 |

All PASS; `executed_count` equals the manifest `required_case_ids` set
for every task (verified set-equal, no missing, no extras);
`residual_containers=[]`, `compose_down_rc=0`. T09–T12 denominators are
byte-stable against the R2 bundle. T13 keeps the 45-cell R2 set and
adds 2 T13-R3 cells:

- `t13r3-mysql80-hash-drop-keeps-directory` — `DROP PARTITION p0` on a
  `PARTITION BY HASH … PARTITIONS 2` table (native 1512, verified in
  the execute arm) keeps the directory proven: the audited pair
  reports pass/complete on both statements, the successor TRUNCATE
  finds `p0` present, executes natively, and removes exactly p0's
  rows (post-verify count = 1, p1's member row survives).
- `t13r3-mysql80-exchange-decimal-effective` — partitioned
  `DECIMAL(12)` vs counterpart `DECIMAL(12,0)`: the audit reports
  pass/complete with zero findings and the native EXCHANGE executes
  (the same effective type, per p12's archived pairs). Both column
  spellings in this cell come from the live provider catalog
  (provider/provider); the declared-vs-provider counter-channel
  (batch-declared `DECIMAL(12)` vs catalog `decimal(12,0)`) is covered
  by the public `AuditSQL` regression
  `TestT13R3ExchangeDecimalEffectiveTypePublic`.

## Native archives

`native/` — this round's bounded probe:

- `p12_exchange_effective_types.log` (`probe12_t13r3.sh`, committed
  under `docs/work-plans/t13-partition-risk/probes/`) — EXCHANGE
  effective-type pairs on mysql80 (8.0.46) + mysql57 (5.7.44), one
  rebuilt fixture pair per arm. Executes: `DECIMAL(12)`↔
  `DECIMAL(12,0)`, `DECIMAL`↔`DECIMAL(10,0)`,
  `DECIMAL(12) UNSIGNED`↔`DECIMAL(12,0) UNSIGNED`, `INT(10)`↔
  `INT(11)`. Refuses 1736: `DECIMAL(12,1)`↔`DECIMAL(12,0)`,
  `DECIMAL(11)`↔`DECIMAL(12)`, `DECIMAL(12)`↔`DECIMAL(12) UNSIGNED`.
  Each arm records the DDL rc plus a catalog `COLUMN_TYPE` read per
  side — no post-state row claims. Also backfills the `INT(10)`↔
  `INT(11)` pair the R2 ADR cited from an unarchived 5.7 check.

## Verified / unverified boundaries

Verified this round: `hash|key` DROP keeps the exact directory
single-action and on both modifier routes (native 1512 re-confirmed on
8.0.46); an unobserved method unvouches the directory without
inventing either outcome; DECIMAL/NUMERIC equivalence pairs execute
and their controls refuse on 5.7/8.0; `PrimaryKeyUnknown` outranks
the residual pointer on both sides; presence-proven/member-unproven
placeholders owe the member fact instead of a fabricated 1736.
Unverified and honestly surfaced: member-level subpartition/interval
operations (gap, not finding); row-content compatibility solving;
unnormalizable faithful spellings beyond the probed set (zerofill
owes a gap, never a verdict); TiDB index-object attribution modeling
(the placeholder shape is consumed, not produced); T14/T15 scope.
