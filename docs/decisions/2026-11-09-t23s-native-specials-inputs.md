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

**Requested increment over the historical `ds-t21` colima profile**
(current profile: 4 vCPU, 8 GiB RAM): create one new run
(`ds-t23s-<timestamp>`) bringing up at most **12 concurrent containers**
— an upper bound on simultaneous identities, not a cumulative count —
the 4 baseline anchors, 3 existing-style clients, PD/TiKV/TiFlash, the
TiKV-backed TiDB, and the apk aux TiDB.

| resource | bound | basis |
|---|---|---|
| vCPU | ≤ 4 total for the VM (increment +0 over the 4 vCPU baseline) | dev/test machine minimums do not apply; fixture is idle between statements (UNVERIFIED headroom) |
| RAM | ≤ 12 GiB total (increment +4 GiB over the 8 GiB baseline) | tikv ~1.5–4 GiB (block cache 45% RAM), tiflash ~2–4 GiB (C++ engine + proxy + 1 GiB mark cache), mysql×3 + tidb×3 + pd + clients ≈ 3.5 GiB (UNVERIFIED — no container minimums are documented) |
| disk | accounted within existing capacity, not a new fixed budget | existing root/data volume already carries prior run images and protected volumes; this run adds ≈ 1.85 GiB compressed arm64 pulls (~4 GiB unpacked) + small fixture data + receipts, bounded by the free space recorded in preflight — the preflight `docker system df`/`df` reading is the gate, and any capacity expansion is a separate approval item |
| services | ≤ 12 concurrent containers, ≤ 1 network, ≤ 8 declared volumes — mysql57-data, mysql80-data, mysql84-data, tidb85-data, t23spd-data, t23stikv-data, t23stiflash-data, t23stiflash-logs — plus ≈ 3 anonymous client volumes per creation generation (one per MySQL client service; actual ownership from `Mounts` capture + ledger registration) | manifest anchors + `data_volumes` declarations |
| recreation generations | ≤ 2 creation generations for the whole run (initial `up` + at most one recreate) | anonymous client volumes grow per generation; unbounded retries are not sanctioned |
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

First step before any service start — a configuration-visibility probe
with a synthetic minimal file (not a real TOML): write a throwaway
`probe-mount.conf` under `docker/tidb/` whose only content is a random
nonce; start one short-lived `mysql:8.4.10` probe container (already
pulled) bind-mounting that single file read-only; `sha256sum` the
container-side path and compare the fingerprint to the host-side hash.
If the bind mount is unavailable or resolves to the wrong path the run
stops immediately and reports — it does **not** silently enable home
sharing, widen file sharing, mount old sockets, make the repository
broadly writable, or modify VM/profile settings to make mounts work.
The probe container is part of the authorised resource plan: it is
registered in the lifecycle ledger and removed by the same S/R/V
cleanup — no manual `docker run`/`rm` bypass. If the probe image's own
`VOLUME` declaration produces an implicit anonymous volume, that volume
must be captured through `Mounts` at creation, registered, counted in
this run's concurrent/per-generation resource budget, and recovered in
the sanctioned V phase — a read-only bind or temporary probe identity
does not exempt it, and unregistered objects are never claimed or
deleted after the fact by label, age, or mount state.

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

## R19 remediation (reviewer-driven)

The R18 audit found three runner blockers and four input/card gaps on
the `2ddaa52a` tree; the fixes below close them without touching any
accepted artifact.

- A1 — timeout enforcement was nested inside the output-mismatch branch
  at `setup`/`execute`/`teardown`, so a forged `timeout:true` record
  with matching output passed. All six verify sites now share
  `_verify_marker_failures` as a sibling of the comparator: `timeout`,
  `error`, invalid declarations and malformed poll histories fail even
  when the recorded output matches exactly.
- A2 — `contains` now has a static shape contract (`verify_shape_error`):
  non-empty string only, mutually exclusive with `expect`, no `str()`
  coercion. The runner rejects an invalid declaration before any mysql
  call (`error_kind: declaration`); the comparator can never wildcard.
- A3 — `run_verify_check` preserves every attempt in `observations`
  (`n`/`rc`/`output`/`stderr`/markers), fails fast on nonzero rc,
  execution exceptions and per-query timeouts (`query_timeout`), and
  continues only on rc=0 unmatched output. The validator independently
  re-checks: cap vs declared attempts, no post-error continuation, and
  summary ≡ last observation — a matching summary cannot launder a
  missing or forged history. Single-shot records keep their historical
  shape.
- B1 — the EXPLAIN step already pinned `stdout_contains: ["tiflash"]`
  (plan-task marker); kept and pinned by contract test.
- B2 — both forced reads drop the weak `contains("1")` for
  `expect_stdout: "tiflash_rows\n1"` (aliased `COUNT(*)` under `-e`
  tabular mode), a new exact-stdout oracle the runner asserts and the
  validator independently re-checks.
- B3 — the stale `pending_native` flag is removed from all seven
  metadata cases; execution status lives only in this record.
- B4/C — the authorization card now fixes a synthetic bind-mount
  visibility probe as the first step (nonce fingerprint, stop on
  failure, probe container inside the ledger), declares ≤12 as a
  *concurrent* container bound, names all eight declared volumes,
  quantifies ≈3 anonymous client volumes per creation generation with
  a two-generation cap, and states the resource increment against the
  real profile baseline: total ≤4 vCPU (+0) and ≤12 GiB RAM (+4 GiB
  over 8 GiB), with disk accounted inside existing capacity rather
  than as a new fixed budget.

A latent harness defect was also found and fixed in this round: a
mid-file `unittest.main()` in `test_ddl_golden_isolated.py` ran before
the R15 `TestLoadFailureLockRelease` class was defined, so its four
tests silently never collected **via the direct-script entry** (the
`python -m unittest` module entry already collected all 61; after the
fix both entries collect 61).

## R21 amendment

The R20 differential audit accepted everything except three poll-history
invariants. `_verify_marker_failures` now also enforces: (a) observation
`n` values are 1..n contiguous in recorded order — duplicates, gaps, or
mistyped values are bounded validation failures, never renumbered or
re-sorted; (b) no observation before the last may already satisfy the
declaration's real comparator (`expect` or `contains`) — the runner
stops at the first match, so a continued history is impossible evidence.
The three reproduced counterexamples (post-match continuation, duplicate
sequence, broken sequence) are pinned as permanent validator negatives;
the mount-visibility probe paragraph gains the anonymous-volume
responsibility clause (`Mounts` capture, budget accounting, V-phase
recovery).

## Attempt-1 revision (config delivery + controlled probe)

Attempt-0 stopped at the card's own gate: `mounts: null` +
`vmType: vz` on `ds-t21-…` means no host-home mount exists in the VM,
so every `./tidb/*.toml` bind source resolved to a dockerd-created
directory inside the VM and the synthetic probe reported
`/probe.conf: Is a directory`. Two corrections are recorded honestly:

- The attempt-0 probe was created and removed manually (`docker
  create`/`rm`/`volume rm` against the pinned endpoint), NOT through
  the isolated entry's lock + capture + ledger path the card required.
  This is an execution-method deviation, recorded as such; it does not
  change the mount finding and is not retroactively presented as
  entry-driven. The objects were removed by exact captured identity
  and the run produced zero residual objects.
- The frozen compose's relative bind sources cannot work on this
  profile under any config change — the daemon resolves bind sources
  inside the VM filesystem.

Revision (single focused input commit, no runner-semantics or oracle
change):

1. The three config binds in `ddl-golden-compose.yaml` switch to long
   syntax (`type: bind`, explicit `source`/`target`, `read_only: true`,
   `bind.create_host_path: false`) with the absolute guest-visible path
   `/Users/fan/GolangProjects/DeltaScope/docker/tidb/<file>` — the path
   at which the dedicated read-only profile share is mounted in-VM.
   A missing source is now a hard create-time failure, never a
   silently created directory.
2. `ddl_golden_isolated.py` gains a `probe` subcommand: it creates one
   run-labelled container through the same entry lock and Mounts
   capture as `up` (the implicit anonymous volume registers in the
   generation ledger identically), refuses a held name, verifies the
   captured bind is `RW:false`, then `start -a`s it and compares the
   emitted sha256 to `--probe-expect-sha256`. Any failure exits
   non-zero with the creation records already persisted, consumable by
   `rm`/`rmvolumes`.
3. The profile mount is a single explicit entry — the run's
   `t23s-config` staging directory (three frozen TOML copies + one
   nonce probe file, mode 0444) mapped read-only to the guest path
   above. After the run completes or stops, the mount increment is
   reverted (`mounts:` back to `null`) while the authorized 12 GiB RAM
   setting is preserved; the reversion is a next-boot configuration
   state, not a runtime unmount claim.

The 21 required case IDs, oracles, runner validation semantics, and
the Go product are unchanged; the artifacts of this run bind the
revision's HEAD, not `a7f5d08d`.

## Deferred / Out Of Scope

Execution itself, performance claims, HA/fault injection, allocator
recovery, COMPACT benefit/space-reclamation measurement, `PROGRESS`
ratio pinning, and `tidb_split_region_max_num` on the real cluster (its
TiKV-cluster value was not source-verified, so no fact is pinned).

## Verification Evidence

Offline only: 822 contract cases (runner primitives + validator pins +
`t23s_manifest_tests`, including a six-site forged-marker matrix and
poll-history forgery matrix) and 61 isolated-lifecycle cases (incl.
`data_volumes` + six-service capture + the four load-failure lock tests
now actually collected). Topology, flags, ports, and the
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
