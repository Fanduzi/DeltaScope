# Decision: T23-S remaining-native-specials input package

Date: 2026-11-09
Status: Proposed (inputs prepared offline; execution awaits a single
resource-increment authorization)
Related milestone/version: mysql-tidb-ddl-completion, issue #102 (T23),
parent #79, T08/#87 dependency
Related commits: prepared on top of `5f28a808`
Related tests: `scripts/test_ddl_golden.py` (`t23s_manifest_tests`,
`t23n_verify_primitive_tests`, validator `contains`/`timeout` pins),
`scripts/test_ddl_golden_isolated.py` (`data_volumes` + multi-service
capture tests)
Related docs: `docs/decisions/2026-11-08-t23-native-execution-inputs-freeze.md`,
`docs/work-plans/t23-native-specials/RESEARCH-tiflash-topology.md`,
`docs/verification/t23-physical-layout/INDEX.md`

## Context

The accepted lightweight run (`00ac3ada` tree, 35/35 cases, 503/503
assertions) proved everything provable on `--store=unistore`. Two cells
remain unprovable there and are explicitly still open:

1. **Real TiFlash arms.** `C-tiflash-replica` proved only the zero-store
   refusal; declaration success, `AVAILABLE=1`, an actual TiFlash-served
   read, clearing, and rebuilding need a TiKV-backed topology. The
   with-replica COMPACT arm can reuse the same fixture for a request-level
   check only (no compaction-benefit claim).
2. **`alter-primary-key=true` matrix.** The knob is startup config
   (`SHOW CONFIG`, not runtime SQL). The differentiating cell is
   `INT_ONLY × apk=true`: a bare single-int PK becomes NONCLUSTERED, so
   AUTO_RANDOM is refused where the accepted `apk=false` arm passes.

This package prepares — but does not execute — both: stack, manifest,
oracles, offline tests, and one concrete authorization card.

## Decision

### Residual mapping (what is NOT re-run)

- The 35 accepted cases keep their identity and evidence; nothing is
  re-run. COMPACT no-replica warning/no-op stays accepted as a no-op arm,
  never promoted to a success-path claim. Performance, HA, allocator
  recovery, and fault injection stay out of scope. No CACHE/SPLIT/MySQL
  negative case is duplicated merely because a real cluster exists.
- New cells only: `C-tiflash-real` (declare→poll→forced read→clear→
  rebuild→compact-request), `apk-false-control-plain-pk`, and the five
  `apk-*` cells on the apk=true aux server.

### Stack (all `pingcap/*:v8.5.0`, matching the tidb85 anchor)

`t23s-pd` (single PD, `max-replicas=1` via `t23s-pd.toml`, placement
rules kept on for learner rules) → `t23s-tikv` (single store) →
`t23s-tiflash` (single store, embedded raft proxy via `t23s-tiflash.toml`)
→ `t23s-tidb` (`--store=tikv --path=t23s-pd:2379`, loopback `24002`) +
`t23s-tidb-client` (mysql:8.4.10). Replica count is **1** matching the
**single TiFlash store**: `SET TIFLASH REPLICA 2` on this stack is a
second refused arm (2 > 1 store) while `REPLICA 1` is the positive arm —
this resolves the earlier one-vs-two-store discrepancy in favour of the
smaller footprint, and the choice is recorded here because the card's
budget depends on it.

`tidb85-apk` is a **separate unistore** TiDB mounting `t23s-apk.toml`
(`alter-primary-key = true`), port `24003`. It deliberately does not join
the TiKV cluster: the provider requires every `Type='tidb'` `SHOW CONFIG`
row to agree, so apk=true and apk=false servers may never share a cluster.

### Runner increments (offline-tested)

- `infra_only` anchors skip db-case emission/validation but stay inside
  ledger capture, health gating, and image pinning.
- `data_volumes` anchor declarations create run-labelled named volumes
  for non-MySQL services (suffix/mount validated; client-side mounts
  refused).
- `verify` entries gained `contains` (substring comparator shared by
  runner and validator) and `poll` (`attempts`/`interval_sec` bounded
  retries). An exhausted poll records the last observation plus
  `timeout: true`; the validator rejects any `timeout` record — including
  a forged timeout-with-matching-output — because undetermined evidence
  is never a pass. Single-shot records keep their historical shape.
- Baseline anchors remain mandatory in `T23S.json`; the manifest is a
  superset, never a replacement.

### apk=true matrix (all on `tidb85apk`, `INT_ONLY` set via `SET GLOBAL`)

| cell | statement | product | native |
|---|---|---|---|
| `apk-intonly-plain-pk` | `BIGINT UNSIGNED PK AUTO_INCREMENT` | pass/complete | NONCLUSTERED (`SHOW CREATE` marker) |
| `apk-intonly-auto-random` | `BIGINT UNSIGNED PK AUTO_RANDOM(5,64)` | reject `auto_random.valid` | refused |
| `apk-explicit-clustered` | `... PK CLUSTERED AUTO_RANDOM(5,64)` | pass | accepted, CLUSTERED marker |
| `apk-explicit-nonclustered` | `... PK NONCLUSTERED AUTO_RANDOM(5,64)` | reject `auto_random.valid` | refused |
| `apk-on-auto-random` | AUTO_RANDOM under `ON` | pass | accepted |
| `apk-false-control-plain-pk` (on `tidb85tikv`) | same as cell 1 | pass/complete | CLUSTERED marker; also pins the cluster-path `SHOW CONFIG` read of `alter-primary-key='false'` |

### Authorization card (the single remaining decision)

**Requested increment over the historical `ds-t21` colima profile:**
create one new run (`ds-t23s-<timestamp>`) bringing up at most **12
containers**: the 4 baseline anchors, 3 existing-style clients, PD/TiKV/
TiFlash, the TiKV-backed TiDB, and the apk aux TiDB.

| resource | bound | basis |
|---|---|---|
| vCPU | ≤ 4 total for the VM (increment ≈ +2) | dev/test machine minimums do not apply; fixture is idle between statements (UNVERIFIED headroom) |
| RAM | ≤ 12 GiB total (increment ≈ +6–8 GiB over the lightweight run) | tikv ~1.5–4 GiB (block cache 45% RAM), tiflash ~2–4 GiB (C++ engine + proxy + 1 GiB mark cache), mysql×3 + tidb×3 + pd + clients ≈ 3.5 GiB (UNVERIFIED — no container minimums are documented) |
| disk | ≤ 12 GiB incremental | image pulls ≈ 1.85 GiB compressed arm64 (~4 GiB uncompressed) + small fixture data + receipts |
| services | ≤ 12 containers, ≤ 1 network, ≤ 8 declared volumes (4 mysql-named + pd-data + tikv-data + tiflash-data + tiflash-logs) + incidental anonymous client volumes | manifest anchors + `data_volumes` declarations |
| loopback ports | 24002, 24003, 24379 only | tidb85tikv/apk product connects + PD API debug |

Images (all `linux/arm64` manifests verified on Docker Hub, v8.5.0
manifest-list digests): `pingcap/pd@sha256:5cf66a73…`, `pingcap/tikv@sha256:
0524a207…`, `pingcap/tiflash@sha256:bb0b83a8…`, `pingcap/tidb@sha256:
490861bd…`, plus existing `mysql:5.7.44/8.0.46/8.4.10`. Full digests are in
`docs/work-plans/t23-native-specials/RESEARCH-tiflash-topology.md` §1.

Config mounts (sha256): `t23s-pd.toml`
`ff76880181fedddcb787969214ab200f253edf51c660302a977fd0ed59936140`,
`t23s-tiflash.toml`
`c105674a549899f2dbfdaf22fc46496fa9bdff4cd8960e2c1b71e8ab4eb4a03d`,
`t23s-apk.toml`
`7b58a0c2ea6cffe5f221c21989a124316d95a3a9eeee4060c98c21c22f15cf0c`.
Fixture account: `root`, no
password on the fixture TiDBs (same convention as tidb85); MySQL anchors
keep `T21_MYSQL_PASSWORD` env semantics — never logged.

Scope granted if approved: image pull for the listed tags only; one
profile budget adjustment; one new run under a new RUN_ID; the DDL/DML/
read-only verifies the manifest declares; normal `stop`/`rm`; and
ledger-scoped S/R/V cleanup through `ddl_golden_isolated.py`.
Not granted: fault injection, mounting old volumes, wider account
permissions, VM deletion, or "adjust until it works". If resources are
insufficient the run stops and reports — no auto-expansion, no external
cloud, no deletion of the eight protected volumes.

**User choice required at authorization:** restore the VM budget to its
pre-run profile afterwards, or leave the new budget — record the choice
in the run receipts.

## Rationale

- One TiFlash store + `REPLICA 1` is the smallest topology that makes
  `AVAILABLE=1` reachable; keeping `REPLICA 2` as a refused arm preserves
  a second negative cell for free instead of doubling the TiFlash
  footprint.
- The apk aux is unistore because isolation — not storage — is what the
  knob's semantics require; the TiKV cluster is reserved for the single
  TiDB whose `SHOW CONFIG` cluster-path read is itself new evidence.
- Bounded polls make every async oracle finite: `AVAILABLE` is a
  persisted flag that never unflips, so the forced-engine read +
  `EXPLAIN` task column are the actual-serve proof while the flag only
  gates the planner.

## Public Contract

Nothing in this package changes product behaviour, the frozen manifest,
or the accepted artifact. `T23S.json` is a new manifest; `infra_only`,
`data_volumes`, `contains`, and `poll` are additive runner vocabulary
with validator pins. Undetermined (`timeout`) evidence can never
validate as success — now enforced at six validator sites.

## Deferred / Out Of Scope

Execution itself, performance claims, HA/fault injection, allocator
recovery, COMPACT benefit/space-reclamation measurement, `PROGRESS`
ratio pinning, and `tidb_split_region_max_num` on the real cluster (its
TiKV-cluster value was not source-verified, so no fact is pinned).

## Verification Evidence

Offline only: 796 contract cases (runner primitives + validator pins +
`t23s_manifest_tests`) and 61 isolated-lifecycle cases (incl.
`data_volumes` + six-service capture). Topology, flags, ports, and the
`alter-primary-key`/`tiflash_replica` semantics are source-verified in
`RESEARCH-tiflash-topology.md`; product expects were re-derived with the
locally rebuilt CLI under the generated `t23-layout-id` policy. Zero
daemon, zero database, zero resource operations were performed for this
record.

## Consequences

A future authorized run needs only: profile budget set → pull listed
images → `ddl_golden_isolated.py` `up`/`run`/`stop`/`rm`/`rmvolumes`
under a new RUN_ID → archive + validate. Any resource shortfall stops
the run; the ledger-scoped cleanup contract still applies.

## Links

- Commits: (this record + manifest/config/test increments — see commit
  list on `milestone/mysql-tidb-ddl-completion`)
- Research: `docs/work-plans/t23-native-specials/RESEARCH-tiflash-topology.md`
