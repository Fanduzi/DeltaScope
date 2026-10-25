# T12 (#91) verification — CHECK constraint & enforcement-state analysis

Product SHA: `1f8e5a0b4e89d8cc4e6f8dd34708476808e2ba91`
(`feat(t12): CHECK constraint lifecycle & enforcement-state semantics (issue #91)`)

Parent: `186710db69088d59eae82726bbc6ad4789f8a353` (T11-R2-tail evidence).

## What shipped

- Extractor: table-level CHECK, `ALTER TABLE ... ADD CONSTRAINT ... CHECK`,
  and column-level `CHECK` options project into `spec.Constraint` members
  (normalized name, `Enforced`, `InColumn`, `ExpressionFacts`; raw expression
  text never retained). `ALTER CHECK`/`ALTER CONSTRAINT` carry
  `Options["check_enforced"]`; `ADD COLUMN` list-forms project through
  `extractAlterSpecs`.
- Spec: `ResolveCheckArm` (MySQL ≥8.0.16 enforced, <8.0.16 ignored, unknown
  when unvalidated; TiDB gated on `tidb_enable_check_constraint`, unknown
  when unobserved), `CheckAlterSyntaxSupported` (MySQL 8.0.19 syntax gate),
  `TiDBEnableCheckConstraint[Known]` instance facts.
- Provider: capability-probed CHECK loading — MySQL via
  `TABLE_CONSTRAINTS` (+`ENFORCED` when present) joined to
  `CHECK_CONSTRAINTS` on catalog/schema/name (MySQL's check table has no
  `table_name`); TiDB via `TIDB_CHECK_CONSTRAINTS` (enforcement stays
  unvouched — only `SHOW CREATE` exposes it). Query failures propagate;
  `tidb_enable_check_constraint` joins loaded instance facts.
- Rules (5 new IDs): `ddl.constraint.check.not_enforced.notice`,
  `ddl.constraint.check.ignored.notice`,
  `ddl.alter.alter_check.exists.require`,
  `ddl.alter.drop_check.exists.require`,
  `ddl.alter.alter_check.version.require`. Rule evaluation and
  EvidenceGaps share one arm decision.
- Ordered state: CHECK members tracked across ADD→ALTER/DROP sequences,
  `ALTER CONSTRAINT` aliases `ALTER CHECK`, 3959 refusal paths publish
  unchanged state, TiDB divergences modeled (CHANGE silently cascades the
  member; RENAME refuses; flag OFF = parse-and-ignore arm; ENFORCED
  post-state unvouches — data-conditional).
- Golden: `testdata/ddl-golden/T12.json` — 25 cases / 280 assertions.
- Corpus: `check_lifecycle` + `check_ignored_{57,off}` fixtures for
  mysql/tidb.

## Gates (18 groups, all quads under `gates/`)

17 × rc=0. `lint` rc=1 carries the accepted 15-issue baseline — byte-
identical stdout to the T11-R2-tail bundle (zero findings on any T12
file; one new `S1021` found during this gate was fixed and folded into
the product SHA via amend). `gofmt -l` reports only the two accepted T07
baseline files. `three-level-doc-diff` replicates the script rule
against `186710db..1f8e5a0b` (script reads the worktree only): every
changed source module's README updated → rc=0.

## Goldens (serial, shared anchors, bound to the product SHA)

| Task | Cases | Assertions | head_sha | CLI sha256 |
|------|-------|------------|----------|------------|
| T09  | 125/125 | 3442 | `1f8e5a0b4e89…` | `71c6ca99…` |
| T10  | 126/126 | 3175 | `1f8e5a0b4e89…` | `71c6ca99…` |
| T11  | 40/40   | 506  | `1f8e5a0b4e89…` | `71c6ca99…` |
| T12  | 25/25   | 280  | `1f8e5a0b4e89…` | `71c6ca99…` |

Artifacts: `golden/T09-artifact.json`, `golden/T10-artifact.json`,
`golden/T11-artifact.json`, `golden/T12-artifact.json` —
`cli/build/head_sha` equals the product SHA in all four; identical CLI
sha256 across all four (one build tree per run, same source).

**T11 denominator note:** the frozen T11 baseline was 40 cases / 505
assertions. The authorized oracle update for
`t11-mysql80-dropfk-check-bearing` (CONTRACT §3a) swaps 3 gap assertions
for 4 finding assertions → 506. All other T11 expectations are frozen.

## Red → green

- `red/baseline_red.log` — first-run behavioral red on the pre-T12
  worktree (spec fields + registered rule skeletons, no implementation),
  via the public `Audit` path: 8 `--- FAIL` markers. The Golden Path,
  reverse-ENFORCED, ordinary-column, non-CHECK-name, and ADD-declare
  cases all red on tombstoned post-state / `coverage=incomplete` / the
  missing notice — the positive controls (missing-name blocker,
  5.7 syntax-refusal, 5.7/TiDB-OFF ignored reveal, unobserved-flag gap)
  were already green on skeleton because they assert bounded absence.
- `red/t12_golden_run{1,2,3}.log` — development-run failures, each root-
  caused and fixed (not oracle edits): run1 = compose anchor ownership
  conflict; run2 = MySQL `CHECK_CONSTRAINTS` has no `table_name`
  (join → catalog/schema/name); run3 = TiDB stores members in
  `TIDB_CHECK_CONSTRAINTS` only + `@@global.` returns `1`/`0` (manifest
  verify + provider TiDB arm added).
- `green/t12-focused.stdout.txt` — 36 `--- PASS` markers / 0 FAIL for
  `go test -run TestT12` on the product SHA.
- `green/pkg.stdout.txt` — provider + rule + spec + parser + public API
  packages green on the product SHA.
- `green/t12_full_test.log` — earlier full-suite run during development.

## Prior-oracle change (authorized, CONTRACT §3a)

`T11.meta.t11-mysql80-dropfk-check-bearing`: in T11 the check-bearing
constraint collection was unvouched → `unknown_table_state` gap, exit 0,
verdict review. T12 loads CHECK members, so `ghost_fk` is a proven-
missing FK → `ddl.alter.drop_foreign_key.exists.require` blocker,
exit 1, verdict reject, coverage complete. Verified against the real
anchor — live DB state unchanged, finding metadata
`{action=drop_foreign_key, exists=false, name=ghost_fk, table=t11ck}`.

## Anchors (ddl-golden compose, healthy during gates + goldens)

- mysql57 = mysql:5.7.44, mysql80 = mysql:8.0.46, mysql84 = mysql:8.4.10,
  tidb85 = pingcap/tidb:v8.5.0 (`tidb_enable_check_constraint` toggled
  ON/OFF via manifest `setup`/`teardown` SQL — verified `1`/`0`).
- Credentials via per-task `<TASK>_MYSQL_PASSWORD` env (never archived).

## Boundaries (see CONTRACT §4)

- `ALTER CHECK ... ENFORCED` post-state unvouched (data-conditional on
  violating rows — native 3819-class refusal).
- `RENAME COLUMN`/`CHANGE COLUMN` on a check-referenced column: probed
  3959 refusal on MySQL 8.0.46/8.4.10 and TiDB RENAME; TiDB flag-ON
  CHANGE silently drops the member (modeled, not refused). Flag-OFF /
  flag-unobserved arms never publish a claimed outcome.
- TiDB enforcement is provider-unvouched (catalog has no surface).
- MySQL <8.0.46 sub-versions: the 8.0.16/8.0.19 documentation boundary
  only — anchors prove ≥8.0.46 behavior, not every patch release.
- CHECK expression solving / `MATCH` clauses stay out of scope.
