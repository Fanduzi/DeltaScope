# T20/#99 T20-R2 evidence — bound to `7bf6e257332b5fd0a632cd881af8487f5dbc9a0b`

Round scope: (A) MySQL `/*T![placement] ... */` feature comments were wrongly refused;
(B) legal whitespace-separated multi-option `ALTER DATABASE` post-state; (C) explicit
disposition of the real topology-validation gap. Prior T20-R1 fixes (ALTER POLICY
replace, mixed table-ALTER 8200 gate, schema-create inheritance + ref sync) are
preserved, not redone.

## A — MySQL feature comments (FIXED)

- `probes/00-prefix-red-evidence.txt` — public `audit.AuditSQL` on pre-fix `0632f66c`:
  policy-reference comment under MySQL → `unsupported statements` (vendor refusal);
  direct-options comment → parse failure; ALTER comment form → unsupported.
  All three are wrong: native MySQL ignores `/*T!` comments.
- `probes/00-prefix-red-probe.go.txt` — archived probe source (`.go.txt` per convention).
- `probes/05-mysql-comment-rc.txt` — mysql:8.0.46 receipts: comment-only ALTER lands
  rc=0 (empty `ALTER TABLE t` is legal), both CREATE comment forms land rc=0 with no
  binding, bare `PLACEMENT POLICY=` still 1064. Client comments preserved.
- Fix: `internal/application/audit/mysql_feature_comments.go` —
  `blankTiDBFeatureComments` is a lexer pass (not regex), applied only when
  `dialect == MySQL` in `parse.go`. Same-width blanking preserves byte offsets,
  newlines, `RawSQL`, statement indices, and positions. Strings, quoted
  identifiers, `/*!...*/`, `/*+...*/`, and ordinary comments are untouched.
- Permanent tests: `internal/application/audit/batch_state_t20r2_test.go`
  (TiDB control still binds via comment; bare MySQL form still vendor-refused;
  invalid remainder still reports real syntax error).
- Golden: `T20.cli.mysql-placement-comment-ignored` (new).

## B — ALTER DATABASE multi-option (FIXED)

- Prior comma-form probe (`probes/06`, negative control retained) returns 1064 —
  comma syntax is genuinely invalid; that receipt proved only the comma form.
- `probes/06-alter-database-partial.txt` — TiDB v8.5.0 unistore receipts for legal
  whitespace-separated `DatabaseOptionList`:
  - `CHARACTER SET latin1 PLACEMENT POLICY=missing` → 8239, but charset/collation
    already landed (partial apply, both option orders — executor applies
    `ModifySchemaCharsetAndCollate` before `ModifySchemaDefaultPlacement`).
  - Failed rebind keeps the prior valid binding.
  - `CREATE DATABASE ... PLACEMENT POLICY=missing` stays atomic (no schema).
- Fix: `batch_state_schema.go` — `schemaPostAttrs` publishes proven non-placement
  attrs while tri-stating only the placement dimension; refused bind keeps prior
  binding, unproven bind withholds placement only.
- Permanent tests: ALTER-DATABASE subtests in `batch_state_t20r2_test.go`
  (both option orders, prior-binding survival, unvouched catalog, CREATE atomic).
- Golden: `T20.meta.tidb85-alterdb-partial` (new). Manifest 23 → 25 cases.

## C — topology prerequisite (BLOCKED, disposition recorded)

- Read-only inventory: every TiDB fixture runs `--store=unistore`; no PD/TiKV
  exists anywhere in authorized fixtures. `SHOW PLACEMENT LABELS` →
  `1105 pd http client unavailable`. `PENDING` is the unistore resting state and
  is never deployment proof.
- Provider reads `information_schema.PLACEMENT_POLICIES` + `SHOW PLACEMENT`
  bound targets only; `SCHEDULING_STATE` is never projected into facts.
- Unknown topology surfaces as evidence gaps (not findings, not errors);
  provider query errors remain fail-loud (proven by
  `TestT20CatalogLoadErrorPropagates`).
- Minimal environment plan (deferred pending separate authorization):
  `pingcap/tidb:v8.5.0` tidb-server + `pingcap/pd:v8.5.0` PD + 3×
  `pingcap/tikv:v8.5.0` TiKV with distinct `--labels` (region/zone/host);
  dedicated compose network on an isolated subnet; health checks via
  `SHOW PLACEMENT LABELS` returning rows; teardown by `compose down -v`.
  Not executed this round — no authorized fixture exists.

## Gates (all bound to product SHA `7bf6e257`)

`gates/` — per-gate rc + stdout/stderr:
- 01 go test ./... rc=0 · 02 go vet rc=0 · 03 race (internal/...) rc=0
- 04 sql-corpus rc=0 · 05 ddl-inventory rc=0 · 06 coverage-catalog rc=0
- 07 parser-unsupported rc=0 · 08 unsupported-diagnostics rc=0
- 09 docs-examples rc=0 · 10 decision-record rc=0
- 11 L2 committed-range (`0632f66c..7bf6e257`) rc=0 · 12 golden-validator rc=0
- 13/14 lint scoped/full rc=1 (inherited baseline warnings, unchanged)
- 15 lint identity comparison rc=0 — same normalization as T19 baseline:
  scoped baseline=18 current=18 added=0 removed=0;
  full baseline=33 current=33 added=0 removed=0.
  (Historical 22/39 counts used a different normalization algorithm; not
  comparable as added/removed. No baseline was switched.)
- pg/pg-unit rc=0 · pg/pg-confidence rc=0 (docker_default on 10.210/16 subnet,
  same accommodation as T19 — recorded, not silent).

`green/T20/artifact.json` — real golden run, head_sha = cli.build.head_sha =
`7bf6e257332b5fd0a632cd881af8487f5dbc9a0b`, 25/25 cases, 255 assertions,
cleanup.compose_down_rc=0, residual_containers=[].

`attempts/` — empty this round: golden passed first attempt with
`T20_MYSQL_PASSWORD` set. (Receipt capture for `06` hit one transient docker
daemon disconnect mid-file; the file was regenerated and the corrected receipt
is what is archived.)

## Verdict

A and B are closed at the software layer with native receipts + product fix +
public regressions. P1/P2/P4 T20-R1 fixes unchanged. Topology satisfaction
remains BLOCKED on an authorized real PD/TiKV fixture — overall stays
SCOPED PARTIAL; this is not "complete T20" and does not advance to T21.
