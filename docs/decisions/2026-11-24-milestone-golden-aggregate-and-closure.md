# Milestone golden aggregate run and closure evidence (T31)

Date: 2026-11-24
Issue: #110 (milestone #79)
Status: decided

## Decision

`make ddl-golden TASK=all` now runs every canonical `testdata/ddl-golden/T*.json`
manifest in one invocation. Discovery is the sorted `T<digits>.json` set —
platform-variant lanes such as `T21-arm64.json` are excluded because they are
not canonical milestone tasks.

Each task executes with its **own complete lifecycle**: compose up, anchor
health and identity verification, case execution, artifact persistence, and
compose down. Tasks never share a live stack. An earlier attempt ran a single
shared stack for all tasks; on resource-constrained hosts the concurrent
multi-service stack exhausted memory and TiKV exited 137, producing a
zero-case aggregate that no honest gate could accept. Per-task lifecycles keep
peak resource use at one task stack and make each task's evidence
identical to a standalone `TASK=<id>` run.

The canonical task set is locked twice: discovery must exactly equal
`required_all_tasks` in `anchors-baseline.json` (a deleted or renamed manifest
is a hard error, not a smaller denominator), and the aggregate validator
re-checks that every discovered task id actually executed.

The aggregate artifact records every task id, per-task case/assertion counts,
per-task failures, and the path to each persisted per-task artifact (per-case
detail lives in those artifacts). It fails closed on an empty task list, a
discovered-but-never-executed task, a task that executes zero cases, a skipped
required case, a per-task validation failure, or an aggregate counter that
does not equal the sum of executed cases. `validate` on an `all/` artifact
re-locks the denominator, fails on unresolvable per-task artifact paths, and
re-runs each task's own manifest oracle.

### Manifest expectations are executable evidence, not history

The first aggregate run surfaced pre-T27 expectations baked into T03/T04/T05
manifests: `mysql-create-procedure-body` still pinned the retired
`create_procedure.body` unsupported entry, and
`mysql-create-procedure-parser-error` still expected compound bodies to be
parser errors — semantics deliberately changed by T27 body analysis. Those
cases were re-pinned to current behavior and the misleading
`parser-error` case id was renamed to `mysql-create-procedure-compound-body`.
Any future semantic change must re-pin every cloned expectation in every
manifest, not only the manifest that introduced the behavior.

The same drift check caught three more pins during closure:

- **T20** pinned schema-level placement to the pre-boundary shape; commit
  `36e616f8` deliberately keeps `ALTER DATABASE … PLACEMENT POLICY` at the
  T17/T20 unaudited boundary, so the two `ALTER DATABASE` cases now expect
  the bounded `alter_schema.option.placement_policy` entry while table-level
  placement stays audited.
- **T26** expected SRS cases to carry a bare implementation-boundary
  metadata shape; the shipped SRS path is version-gated (`unaudited` under
  `srs.version_applicability` until the applicability premise is proven), so
  the seven cases re-pin to the honest bounded metadata.
- **T23S** anchor digest locks: `tidb85tikv`/`tidb85apk` were missing from
  `anchors-baseline.json` `auxiliary_anchors`, so their identity proof had
  no locked digest to compare against. Both anchors now lock the same
  `pingcap/tidb:v8.5.0` digest as `tidb85`.

### Live deployment evidence is captured, never committed

`--deployment-evidence-file` binds facts to the container's current
`@@server_uuid`, so a committed evidence file can only ever describe the
container that produced it — reuse across runs is impossible by contract.
The runner's `deployment_evidence` setup step therefore probes the live
anchor per run: `@@server_uuid`, `@@version`, `nproc`, and `/proc/1` CapEff
bit 23 (`CAP_SYS_NICE` → `priority_capable`), then writes the file at the
manifest-pinned path before the audit executes. Capture output is recorded
in the case's setup log; the validator refuses a step record missing target,
version, host_vcpus, or priority_capable facts, and a dead probe fails the
case closed. Committed T24 evidence files are provenance of shape, not
reusable credentials. T24's 24 cases pass on live-captured evidence.

### Reaggregation

`ddl_golden.py reaggregate` rebuilds the `all/` aggregate from persisted
per-task artifacts without re-running Docker, refusing a missing canonical
artifact and refusing to mix artifacts recorded at different git HEADs. It
exists for cheap denominator checks after manifest re-pins; it is not a
substitute for a fresh `TASK=all` run.

### Benchmark lanes measure audited paths

`service_benchmark_test.go` fixtures were authored before the
unsupported-boundary contract and hit `ErrUnsupportedStatement` through the
column-level `UNIQUE` sugar — a bounded, intentionally unsupported aspect. The
fixtures now use covered `UNIQUE KEY` forms so the lanes measure audit
throughput, not the error channel. `BenchmarkAuditRoutineBody` adds the
T27-T30 stored-body lane.

Observed on the closure host (Apple M4 Pro, 500 iterations): single CREATE
TABLE ~479µs/op, complex multi-feature ~722µs/op, ten-statement batch
~1.36ms/op (~136µs per statement amortized), routine compound body ~422µs/op,
TiDB dialect ~643µs/op. Member-level re-parse shares the outer parse cost, so
body analysis does not dominate; per-statement cost is bounded by the parser
call. No material regression is claimed — the numbers exist so a future
regression has a baseline.

## Rationale

The milestone claim "every listed statement family reaches real analysis" is
only as strong as the freshest execution of every manifest. A sequential
aggregate makes that check one command, keeps per-task evidence byte-identical
to standalone runs, and turns manifest drift into a hard failure instead of a
silent stale pin.

## Public contract

- `TASK=all` is a supported runner mode; per-task artifacts under
  `<ARTIFACT_DIR>/<TASK>/` plus an `all/` aggregate artifact are produced.
- Aggregate validation is fail-closed on empty task lists, zero-case tasks,
  skipped cases, validation failures, and counter mismatches.
- Manifest case ids remain stable addresses for evidence; a case id that no
  longer describes its assertion must be renamed in the manifest and in
  `required_case_ids`, never silently re-purposed.

## Verification

- `python3 scripts/test_ddl_golden.py` — 890 contract cases including
  canonical discovery, platform-variant exclusion, aggregate pass, empty-list,
  zero-case, propagation, skipped-case, dishonest-counter,
  discovered-but-unexecuted, baseline-denominator-lock, reaggregate
  missing-artifact/mixed-HEAD refusal, and live deployment-evidence
  capture/validator rejections.
- `make ddl-golden TASK=all ARTIFACT_DIR=/tmp/ddl-golden` — sequential run over
  31 canonical manifests (T32 joined the locked set under issue #111) with
  per-task lifecycles; final milestone run at `8816924f` passed 31 tasks,
  2565 cases, 50875 assertions, and `validate --task all` re-checked the
  aggregate artifact clean.
- `go test ./...` — 39 packages, no failures.
- `make sql-corpus-gates` — 790/790 supported rule-dialect targets covered.
- `make ddl-inventory-gate` — all inventory rows carry owner, status, and
  file-bound evidence.
- `make pg-unit-test-gates` — PostgreSQL-tagged packages all pass.
- `npm test --prefix packages/deltascope-mcp` — 20/20.
- `go test ./internal/application/audit -bench BenchmarkAudit` — five lanes
  pass on covered SQL.
