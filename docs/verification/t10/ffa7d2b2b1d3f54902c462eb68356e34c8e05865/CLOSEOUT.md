# T10-R3 closeout — ffa7d2b2

## Verdict

T10-R3 independent-acceptance candidate delivered. All three
independent-review defect classes fixed with red→green tests, native
re-verification performed for every previously unsubstantiated claim,
and the directly affected golden suites (T09, T10) re-run green at the
bound SHA.

## Fixes vs. review findings

| Finding | Disposition |
|---------|-------------|
| CREATE TABLE declared-pair with non-plain member completed silently | Fixed — `declaredComparisonGap` emits `unknown_table_state` + `target_table.indexes.key_parts` for applicable duplicate/left-prefix/unique-overlap pairs; no self-pairing; non-applicable controls unchanged |
| `PrimaryKeyHandleUnknown` missing from exists/compat evidence-gap premise | Fixed — participates alongside `PrimaryKeyUnknown`/`ConstraintsUnknown`; exists-only runs now emit `target_table.primary_key` gap (unverified coverage) |
| Unnamed-member successor resolved against empty stored name | Fixed — `spec.StoredIndexNames` derives product names (first column + ordered `_N` suffix); unprovable name space → `IndexNamesUnknown` gating name-keyed consumers only; member-shape consumers keep authored facts |
| TiDB `int_only` conflated with `on` | Fixed — defers to `alter-primary-key` via the verified all-row `SHOW CONFIG` loader; unobserved config → unknown |
| Expression member direction dropped by provider | Fixed — `STATISTICS.COLLATION=D` → `Descending`; `A` ascending; NULL/unrecognized unmodeled, expression identity preserved |
| Generic index-collection gate over-blocked `ALTER INDEX PRIMARY` | Fixed — exists evaluation skips the member-collection gate for `PRIMARY` targets when resolution proceeds from primary-key facts |
| `applyCreateIndex` dropped authored members when names unprovable | Fixed — early return narrowed to `IndexesUnknown`; members merge, `IndexNamesUnknown` stays set |
| `applyAddIndex` same member-loss under `IndexNamesUnknown` | Fixed symmetrically — member-premise vs name-space provability split |
| `DROP PRIMARY KEY` ignored handle facts | Fixed — proven handle keeps state (native 8200, pinned by T09 manifest + M7 probe); unproven handle invalidates; real object drops |
| `applyAddIndex` PK branch handle-awareness | No code change — natively proven: ALTER ADD PK is always a NONCLUSTERED real object under tested modes (M7, `alter-primary-key=false`), so the existing path is correct; documented in ADR/comment |
| R2 archive overstated native evidence | Corrected — R2 CLOSEOUT/README amended; R1-blob pin log no longer credited with R2 matrices; missing matrices re-verified here with preserved script + transcript |

## Evidence ledger

- Red→green: focused T10-R3/T09-R1 unit suites
  (`gates/t10r3-focused-tests.log`, 600 checks run) — each fix landed
  against a failing test asserting rule attribution, gap facts,
  coverage, verdict, and post-state.
- Golden at bound SHA: T09 125/125 (3442 assertions), T10 126/126
  (3175 assertions) — `golden/T09`, `golden/T10`,
  `golden/regression-t09-t10.log`.
- Validator suite: 938 checks, 0 failures
  (`gates/ddl-golden-validator-test.log` = 758 contract cases + 86 T07 +
  25 T08 + 36 T09 + 33 T10 constructed controls). 938 is the merged
  validator count; `test_ddl_golden.py` alone is 758.
- Offline gates: `make test`, `go vet`, race (4 touched packages),
  `pg-unit-test-gates`, `pg-confidence-gates`, sql/query-access
  corpora, inventory, catalog, parser-error, unsupported-diagnostics,
  docs-examples, dialect-hygiene, decision-record, three-level-doc —
  all rc=0 (`gates/*.log`).
- `release-gofmt-gate` nonzero on the same five baseline files as R2
  (3 t06 archive probes + 2 t07 tests) — pre-existing, unchanged by
  this diff.
- Native re-verification: `native-probe/r3-bounded-probes.sh` +
  `.log` — script preserved this time; every command and rc captured
  against mysql:8.0.46 and pingcap/tidb:v8.5.0.

## Boundaries held

- No T10 surface expansion: no expression equivalence/evaluation, no
  optimizer claims, no per-function TiDB expression legality, no MV
  query semantics, no vendor-boundary changes (MySQL GLOBAL, TiDB
  FULLTEXT/SPATIAL untouched).
- T03–T08 golden not re-run: R3 touched no oracle/manifest/behavior
  those suites pin; covered by `go test ./...` + validator 938.
- `.mcp.json` and unrelated untracked tool dirs intentionally
  excluded from commits.
- Stops at T10-R3 candidate — no T11 work started.
