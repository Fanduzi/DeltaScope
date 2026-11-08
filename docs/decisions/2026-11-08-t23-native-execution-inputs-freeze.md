# Decision: T23 native-execution input freeze — frozen manifest, per-case oracles, and the one-shot authorization card

Date: 2026-11-08
Status: Frozen, awaiting one-shot itemized authorization (no native execution performed)
Related milestone/version: mysql-tidb-ddl-completion (issue #79, task T23 under #102)
Related product commit: `f4fe758da61e196b69255034348dcd0cb182a66c` (accepted R9 fixed tree)
Related manifest: `testdata/ddl-golden/T23.json`
Related runner: `scripts/ddl_golden.py`, `scripts/ddl_golden_isolated.py`
Related docs: `docs/decisions/2026-11-13-tidb-auto-random-validity.md` (product semantics, R5–R9)

## Context

The accepted R9 tree proves the offline semantics for the T23 surface.
This record freezes the runnable inputs for the deferred native-validation
step: the manifest denominator, fixture seeds, per-case oracles, the
runner/lifecycle contract, attempt identity, and the resource
authorization card. Freezing inputs is not execution: nothing in this
record starts a daemon, connects to a database, or runs native SQL.

### Identity closed before authoring

- HEAD/branch: `f4fe758da61e196b69255034348dcd0cb182a66c` on
  `milestone/mysql-tidb-ddl-completion`; parent `09f412c5` (R8), R7-tail
  `182b1db`, R5 `47fff59`.
- Tracked/staged state at freeze start: clean; 13 historical untracked
  paths and all existing worktrees preserved.
- Anchor topology (frozen, `docker/ddl-golden-compose.yaml`): tidb85 is
  `pingcap/tidb:v8.5.0` running `--store=unistore` behind the MySQL
  protocol on `127.0.0.1:24000`; mysql80 is `mysql:8.0.46` on `23380`.
  There is no PD, no separate TiKV node, and no TiFlash store.

## Manifest defects repaired (execution-input fixes only)

The draft manifest predated the real runner contract:

1. `required_case_ids` were 25 bare ids; the runner emits fully prefixed
   `T23.db.*`/`T23.cli.*`/`T23.meta.*` case ids and `validate_artifact`
   requires exact equality. Frozen denominator is now **35**:
   8 db anchor cases + 17 CLI cases + 10 metadata cases.
2. `ddl_steps` was empty — no `t23fx` fixture seed existed; now mirrors
   the T21/T20 seed shape (`DROP DATABASE IF EXISTS` + schema + seed
   table + existence verify).
3. All 8 metadata cases were `EXPECT FILLED` stubs; they now carry
   complete setup/expect/post_verify/execute/structure/teardown bodies.
   Two new cases (`B-split-region`, `C-compact-noop`) were added because
   the authorization groups B (SPLIT) and C (COMPACT) had no native case.
4. `policy.enable` gained `ddl.tidb.table.cache.valid`,
   `ddl.tidb.table.attributes.valid`, `ddl.tidb.table.tiflash.valid`
   (13 enabled rules total), so the B/C cases evaluate their rules. The
   honest consequence: the cache guard now emits `table.cache_status`
   `missing_instance_fact` gaps on ALTER statements bound to unobserved
   tables — CLI-case expects were reconciled to the actual frozen product
   output (see below), not to older prose.
5. Older CLI-case expects were missing the strict checker's required
   keys (`diagnostics`, `unsupported`) and carried stale values: the two
   MySQL vendor-boundary cases now emit `diagnostics=1` + `exit=1`, the
   two undecidable ALTER cases emit 2 gaps each, and
   `tidb-alter-split-index-bounded` gained the cache-guard gap. All 17
   CLI cases were re-derived against the built CLI (`0` failures).
6. `INSTANCE_FACT_QUERIES` gained `tidb_enable_clustered_index`
   (session global), `tidb_alter_primary_key` and
   `tidb_split_region_max_num` (both `SHOW CONFIG` rows — the same read
   path the provider uses). A missing row returns `None`; nothing is
   default-guessed. Pinned by `t23_instance_fact_tests` in
   `scripts/test_ddl_golden.py`.

## Canonical denominator and A/B/C allocation

| Case id (prefix `T23.meta.`) | Group | Anchor | Proof scope |
|---|---|---|---|
| A-auto-random-first-path | A | tidb85 | CREATE+ALTER accepted path; instance facts int_only/false |
| A-auto-random-static-refusals | A | tidb85 | 8 static refusals; each preserves pre-state (table absent) |
| A-clustered-mode-comparison | A | tidb85 | `SET GLOBAL tidb_enable_clustered_index=off` → product reject + native refusal; on/int_only native accepts; restore to the recorded baseline ON |
| A-mysql-vendor-negative | A | mysql80 | real MySQL 8.0 refuses both forms; product bounded-unsupported |
| A-auto-random-base-controls | A | tidb85 | AUTO_RANDOM_BASE declaration vs allocator floor (`NEXT_GLOBAL_AUTO_ID >= 100`), never runtime progress |
| A-alter-layout-matrix | A | tidb85 | 12-statement matrix: add/modify refusals, increase allocator gap, AUTO_ID_CACHE 1↔non-1 boundary, SHARD clustered refusal + >15 clamp, AUTO_RANDOM_BASE, PRE_SPLIT refusal; per-arm dedicated fixtures |
| B-cache-attributes | B | tidb85 | NOCACHE/CACHE declaration + lock-row catalog evidence + partitioned refusal; ATTRIBUTES set/default/invalid/partition; effect stays open |
| B-split-region | B | tidb85 | 12 statements: valid split, plan-time refusals (step/bound/NULL/index/partition/regions), BY list; index+common-handle BETWEEN stay bounded-unproven product-side |
| C-compact-noop | C | tidb85 | COMPACT acceptance (warning no-op) + partition refusals; compaction effect stays open |
| C-tiflash-replica | C | tidb85 | `requires_service: tiflash`; declaration + replica_count=2 + AVAILABLE=0 negative arm only |

### Expected output contract (probe-derived, frozen)

Every `expect` block was derived by running the real parse → enrich →
`EvaluateStatements` path against provider-shaped snapshots on the fixed
tree — not by copying prose. Highlighted non-obvious frozen values:

- `A-alter-layout-matrix`: 7 blockers + 2 unverified gaps
  (`runtime.auto_random_allocator`, `runtime.row_data`), verdict reject.
  Each MODIFY variant owns a dedicated fixture table: an undecidable
  MODIFY makes the ordered-state post-shape unproven, so a shared table
  would make later statements read the bounded derived shape instead of
  the proven catalog.
- `B-cache-attributes`: `ALTER TABLE ... CACHE` stays `unverified` with
  the `runtime.cache_table_size` gap — a deliberately runtime-only fact;
  `CACHE` on a partitioned table is a proven refusal;
  `ATTRIBUTES='key'` (no `=value`) is a proven refusal.
- `B-split-region`: 7 blockers + 2 `unproven_statement_fact` gaps
  (`statement.split_bound_relation`) on index and common-handle BETWEEN;
  verdict reject. `(0,100) REGIONS 4` is a step-too-small refusal, not a
  success path; NULL bounds are `ErrBadNull` refusals on the table path.
- `C-tiflash-replica`: `unverified` gap `table.tiflash_replica` — the
  provider never publishes a replica-count fact, so the product can never
  claim a prior-replica answer; native replica_count=2 + AVAILABLE=0 is a
  proven negative only.
- `A-clustered-mode-comparison`: under `off` the product must reject the
  same statement A-auto-random-first-path passes under `int_only`.
  `alter-primary-key` is a SHOW CONFIG startup knob — it cannot be
  flipped by runtime SQL, so the apk=true axis is an authorization item
  (config mount), not a setup step.

## Runner and lifecycle contract

- Entry point: `scripts/ddl_golden_isolated.py` (`preflight`, `up`,
  `run`, `stop`, `rm`, `rmvolumes`). It requires an explicit `--daemon`;
  it rejects default Docker endpoints, clears `DOCKER_CONTEXT`, uses a
  run-scoped compose project and labels, and tears down only
  receipt-registered objects. No `down -v`, no global prune.
- The product audit path inside `_execute_metadata_case` is
  read-only: fixture SQL is applied by the test driver through
  `mysql_exec`; the product CLI only reads metadata. `post_verify`
  steps prove the audited objects were not mutated.
- Per-case `expect` asserts exit/verdict/statement/findings/diagnostics/
  unsupported/coverage/gaps plus optional `finding_entries`,
  `evidence_gap_entries` (with `required_facts`), and `instance_facts`
  (live `INSTANCE_FACT_QUERIES` reads).
- Attempt identity: `<artifact-root>/golden/T23.attempts/<head12>/attempt-N/`
  with `SHA256SUMS` and a `latest` index; failed attempts are never
  overwritten. Evidence must contain raw rc/stdout/stderr; a copied
  product result cannot substitute for native receipts.

## Async / service end-classifications

For every async or service-gated arm the run must end in exactly one of:

- `accepted_proven` — SQL rc=0 AND the declared post-condition verified
  (catalog row, count, marker). rc=0 alone never proves it.
- `refused_native` — SQL rc=1 with the recorded error.
- `mismatch` — native actual differs from the frozen expect; preserve
  rc/stdout/stderr and before/after state, then classify fixture error /
  oracle error / product counterevidence. Never rewrite expected to
  actual on the spot and never tune SQL to bypass a mismatch.
- `environment_gap` — a required store/capability is absent (TiFlash,
  non-unistore backend, a SHOW CONFIG row the image lacks). Record the
  probe evidence; never promote it to success.
- `timeout_undetermined` — a bounded poll expired; the result is
  undetermined, not failed-by-assumption. Polling caps live in the
  per-case notes; no unbounded waits and no service restarts.

Explicit non-proofs: a reachable port does not prove store capability;
`table_cache_meta` row presence proves the DDL job ran, not a cache
status; `information_schema.tiflash_replica` rows prove declaration,
not availability.

## One-shot itemized authorization card

Requested runtime actions, all inside the isolated lifecycle wrapper and
run-scoped compose project:

1. Start the golden stack (tidb85 unistore, mysql80, plus the baseline
   mysql57/mysql84 anchors the manifest's db cases require).
2. Execute all 35 `required_case_ids` via `ddl_golden.py run --task T23`.
3. `SET GLOBAL tidb_enable_clustered_index` flip to `off` and back to the
   recorded baseline (`ON` on this stack — verified, not assumed), inside
   `A-clustered-mode-comparison` only. Original values are recorded in
   the attempt artifact before mutation; teardown restores and verifies.
4. Fixture-only objects under the `t23fx` schema; no real user data, no
   secrets in argv/logs. MySQL connects use `password_env`/`password`
   the runner already models.
5. SPLIT/COMPACT/CACHE/SET-TIFLASH native executions on the unistore
   anchor within the per-case rc expectations; environment mismatches
   classify per the taxonomy above — they are never forced green.

Explicitly NOT requested / declared-open:

- TiFlash store availability/ready/recovery arms (no TiFlash service in
  the frozen stack); the case's positive arms stay `environment_gap`.
- `alter-primary-key=true` matrix arms — requires a tidb-server config
  mount change and restart; out of this card's scope.
- Any performance, region-count, cache-effectiveness, or
  compaction-benefit claim.
- Push, merge, PR, issue closure, release, or any non-fixture account.

## Verification Evidence

- `python3 scripts/test_ddl_golden.py`: 766 contract cases, 0 failures —
  including the 8 new T23 instance-fact pins.
- All 17 `cli_cases` re-executed through `make_policies` +
  `cli_case_expect_checks` against a locally built CLI: 0 failures.
- Manifest consistency: `baseline_manifest_failures` clean; 35 unique
  required ids; every case's policy profile resolves; all 13 enabled
  rule ids exist in the live `rules list` catalog (487 rules).
- Oracle derivation: read-only Go probe (verify worktree, uncommitted)
  exercising the real parse → enrich → evaluate path per case.

## Consequences

- The frozen denominator is 35 and is now runner-checkable; shrinking it
  silently is impossible because db anchor cases are independently
  required by the baseline contract.
- Product semantics were not touched: every `expect` change records the
  fixed tree's actual output. If a later review finds an oracle should
  differ, that is evidence against the product, not a manifest fix.
- `tidb_enable_clustered_index` restore targets the recorded
  pre-mutation value, never an assumed constant.

## R10 amendment — scoped authorization corrections (2026-11-08)

The reviewer's lightweight-stack authorization asked for four limited
input corrections before execution; this amendment records them.

### §二 corrections applied

- **A. actual vs oracle separation** — every metadata case now carries
  `oracle_basis`: the product `expect` is the frozen contract derived
  from the fixed tree `f4fe758d` through the real
  parse→enrich→EvaluateStatements path (the runner collects `actual`
  from the built binary, so `expect` never oracles itself), while each
  native `expect_rc` mirrors the documented upstream v8.5.0 arm and each
  `verify`/`post_verify`/`structure` assertion is a catalog observation,
  never an inference.
- **B. clustered mode baseline** — `A-clustered-mode-comparison` now
  records and asserts `@@global.tidb_enable_clustered_index=ON`
  in setup *before* the flip — the real v8.5.0 default is `ON`, which
  the first native run discovered (the earlier draft assumed
  `int_only`); teardown restores `ON` because that is the verified
  recorded baseline on this stack, not an assumed constant. The five
  AUTO_RANDOM-dependent cases additionally pin `int_only` for their own
  duration (AUTO_RANDOM requires a non-clustered integer PK) and restore
  `ON` in teardown. If the baseline ever drifts the case fails pre-mutation
  instead of restoring a wrong value. Scope mechanics documented: the
  variable is global+session scoped, `SET GLOBAL` affects new
  connections only, and both the runner client and the product provider
  open fresh connections, so `off` is observed through session
  `SHOW VARIABLES` exactly as `LoadInstanceFacts` reads it.
- **C. C-group scope separation** — `C-tiflash-replica` is marked
  negative-only; positive arms are out-of-scope pending real TiFlash,
  never silently absent.
- **D. canonical vs selected set** — the manifest gains
  `selected_case_ids` (= all 35 canonical ids for this run; selection is
  declared, not runner-enforced) so the canonical denominator and the
  authorized run scope are separate fields.

### §三 oracle audit

25→35 id mapping (the draft's 25 bare ids were never runner-valid):

- 17 `T23.cli.*` — 1:1 prefix fix of the 17 bare cli ids.
- 8 `T23.meta.*` — 1:1 prefix fix of the 8 bare meta ids (A-×6,
  `B-cache-attributes`, `C-tiflash-replica`), now with real bodies.
- +8 `T23.db.<anchor>.{ddl,syntax_negative}` — baseline-contract db
  anchor cases the draft omitted entirely.
- +2 `T23.meta.*` — `B-split-region`, `C-compact-noop`, added because
  groups B (SPLIT) and C (COMPACT) had no native case.

Instance-fact query audit (the three added `INSTANCE_FACT_QUERIES`):

- Executed by the runner through `mysql_exec` — a fresh fixture-client
  invocation per read; results are consumed only at the
  `expect.instance_facts` assertion layer as independent environment
  observations. They are not product inputs: the product reads
  `tidb_enable_clustered_index` itself via `SHOW VARIABLES` in
  `LoadInstanceFacts`, and `alter-primary-key`/`split-region-max-num`
  via its own `SHOW CONFIG` path, so nothing bypasses the product's
  own reads.
- Failed/empty reads return `None` and the assertion fails; zero rows,
  errors, and inconsistencies can never masquerade as observed values.
  Expected fact values are hypotheses checked at run time, not
  injected facts.

Probe-derived expects are the product contract (what the fixed product
should output), not independent oracles of correctness; the
independent axis in every case is the native `expect_rc`/catalog
assertion set, which mirrors upstream v8.5.0 semantics.

### §六 TiFlash classification

The claim "the provider never publishes a replica fact" is
classification **#3 — the catalog loader is not implemented**:
`information_schema.tiflash_replica` (REPLICA_COUNT, AVAILABLE per
table) is never queried by `internal/infrastructure/metadata/mysql`;
the only `tiflash_replica_count` producer is ordered-state same-batch
declaration tracking in `batch_state_layout_r6.go`. Interface scope for
a future loader: per-table replica count + availability from
`information_schema.tiflash_replica` (or `SHOW TABLE <t>
TIFLASH_REPLICA`), store capability from `information_schema
.tiflash_store`/`SHOW PD ...` equivalents. This is reported as an
unimplemented product read, not an environment gap and not
NATIVE-PENDING.

## R10 execution — lightweight-stack native run (2026-11-08)

Scoped authorization executed on the approved colima profile
(`colima-ds-t21-20261107t1158z-e514c8`, docker 29.5.2 arm64) via
`ddl_golden_isolated.py` run-id `t23r10-20261108t13311794144718z-a1`.
Attempt 6: **35/35 cases passed, 503 assertions, `[ddl-golden] PASS`**
on head `00ac3ada`. Evidence archived under
`docs/verification/t23-physical-layout/00ac3ada62c7/`
(artifact.json, attempt-6 case files, lifecycle receipts). Lifecycle:
5 containers + 1 network + 3 named volumes created, receipt-scoped
stop/rm/rmvolumes completed; 4 anonymous generation volumes also
verified-by-timestamp and removed; the 8 pre-existing protected
volumes untouched; VM returned to Stopped.

### Environment discoveries that corrected inputs (not product code)

- `tidb_enable_clustered_index` default on real v8.5.0 is `ON`, not
  `int_only`; live reads return uppercase `ON`/`OFF`/`INT_ONLY`. The
  five AUTO_RANDOM-dependent cases pin `int_only` for their duration
  and restore `ON`.
- `SET GLOBAL` only affects *subsequent* connections — a
  `SET GLOBAL x; CREATE` pair in one client exec still evaluates under
  the connect-time session value. Mode-flip steps were split so each
  CREATE runs on a fresh connection.
- Upstream refuses `SET TIFLASH REPLICA 2` at plan time when TiFlash
  store count is 0 (`ERROR 1105 ... replica count: 2 should be less
  than the total tiflash server count: 0`) — the provable negative is
  the plan-time refusal itself, stronger than `AVAILABLE=0`.
- Upstream refuses `DROP TABLE` on a cache-enabled table
  (`ERROR 8242`) — teardown must `NOCACHE` first, itself upstream
  guard evidence.
- `SPLIT TABLE <non-partitioned> PARTITION (p9)` is accepted on
  unistore — named members are ignored on non-partitioned targets on
  both the upstream executor and the product's member-resolution arm
  (consistent, not a bug). The missing-partition arm was re-pointed at
  the partitioned fixture where upstream does refuse.
- AUTO_RANDOM catalog marker is
  `information_schema.TABLES.TIDB_ROW_ID_SHARDING_INFO
  = 'PK_AUTO_RANDOM_BITS=5'`; `COLUMNS.EXTRA` is empty. Allocator
  floor is observable only via `SHOW TABLE <t> NEXT_ROW_ID`
  (observed `100` for declared `AUTO_RANDOM_BASE=100`).
- Fixture credential anchors were wrong (`T21_MYSQL_PASSWORD` on
  mysql57/84 vs `T23_MYSQL_PASSWORD` on mysql80, phantom
  `T23_TIDB_PASSWORD` on passwordless TiDB); unified on
  `T21_MYSQL_PASSWORD` and tidb85 exec client corrected to
  `--protocol=tcp -h tidb85` per the T21/T22 working shape.
- `tidb85` needed `needs_database_create: true` (golden schema is
  compose-created on MySQL only).

### Upstream receipts captured (attempt-6 native arms)

Refusals confirmed live: AUTO_RANDOM add/decrease/remove/type-change
(`ERROR 8216`), modify-PK-flag (`ERROR 8200`), AUTO_ID_CACHE 1↔non-1
boundary (`ERROR 1105`), SHARD_ROW_ID_BITS on clustered (`ERROR 8200`),
PRE_SPLIT_REGIONS alter (`ERROR 8200`), SPLIT step<1000 / upper<=lower
(`ERROR 8212`), NULL bound (`ERROR 1048`), missing index (`ERROR
1176`), REGIONS 0 / >1000 (`ERROR 1105`), CACHE on partitioned table
(`ERROR 8242`), invalid ATTRIBUTES (`ERROR 8237`). Acceptance arms:
AUTO_RANDOM create/alter under int_only, valid SPLIT (table/index/
common-handle/rowid/BY-list/partitioned), CACHE↔NOCACHE transitions,
ATTRIBUTES set/partition, COMPACT warning no-op, `SET GLOBAL` mode
flips with verified restore.

## R11 — isolated-entry ledger persistence repair (2026-11-08)

Post-run independent acceptance located a tool defect behind the
manual deletion of four anonymous volumes, reported honestly in the
dual-track verdict rather than signed as compliant.

### Root cause

`ddl_golden_isolated.py::load_run` reloaded `receipts.json` for every
subcommand **except `up`** (`args.command != "up"`). Each `up`
invocation therefore constructed a fresh receipts dict and the first
`record()` truncated the file, erasing every generation an earlier
`up`/`run` cycle had registered — including the anonymous
`/var/lib/mysql` volume of a recreated fixture client. `rmvolumes`
consumed only the last generation's registrations; earlier cycles'
anonymous volumes became unledgered orphans (compose `down` does not
remove anonymous volumes).

### Repair semantics (offline, fake-executor verified)

- Restore-once: every entry — `preflight`, `up`, `run`, `stop`, `rm`,
  `rmvolumes` — resumes the persisted ledger when it exists. A run
  initialises its ledger exactly once; a second `up` appends a new
  generation instead of reinitialising.
- Binding: a restored ledger must match run-id, project, daemon
  endpoint and owner; daemon server-id recorded at creation is
  verified live, never overwritten.
- Corruption/interruption: missing explicit `--receipts`, unreadable
  or non-object JSON, and teardown without any ledger all refuse —
  no silent empty-ledger init.
- Durability: ledger writes are tmp+fsync+`os.replace` against the
  loaded snapshot; the snapshot check is non-cooperating-modification
  detection, not an atomic compare-and-swap (see R12 amendment) — a
  serial writer holding a stale snapshot refuses instead of
  last-writer-wins; failed writes leave the last valid ledger intact;
  new ledger files are mode 0600.
- Generation records: re-observing a registered object appends a
  `reobserved` marker, never a second creation record; partial
  captures mark `capture_incomplete` instead of reporting a smaller
  world.
- Teardown: cleanup still consumes the cumulative ledger only;
  registered-but-mounted volumes refuse deletion and are reported
  (`volume.inuse.*`) rather than force-detached.
- Forbidden fallbacks unchanged: no label/time/label-prefix orphan
  sweeps, no `prune`, no `--remove-orphans`, no unregistered-ID
  re-derivation.

### Scope honesty

- The code fix does **not** retroactively legitimise the historical
  manual deletion of the four anonymous volumes; that deviation and
  its object-level evidence gap remain recorded as-is.
- The repaired entry is verified offline only (39 fake-daemon
  contract cases). A bounded real-lifecycle verification — repeated
  `up`, one real client recreation, cumulative-ledger reclamation —
  remains pending a separate authorization; this commit does not
  claim native re-validation.
- The accepted T23 lightweight-stack result (35/35, 503/503) stays
  bound to run tree `00ac3ada`; this tool repair does not rewrite
  `artifact.head_sha` or re-open product semantics.

### Authorization card — C-column supplement (real TiFlash request)

Deferred incremental request for the real success/recovery arms:

- Minimal components: 1× PD, 1× TiKV, 1× TiFlash plus the existing
  tidb-server, pinned to `v8.5.0` images (`pingcap/pd:v8.5.0`,
  `pingcap/tikv:v8.5.0`, `pingcap/tiflash:v8.5.0`) on the aarch64 Lima
  profile.
- Estimated increment over the current profile: ~4 vCPU / ~6-8 GiB RAM /
  ~10 GiB disk additional budget.
- Replica math: replica_count=2 requires ≥2 TiFlash stores for AVAILABLE
  to reach 1 — decide 1 vs 2 stores at request time and record which.
- Synthetic data: a single small `t23fx` table suffices; readiness is
  observed via `information_schema.tiflash_replica.AVAILABLE` polling
  with a bounded cap.
- End classifications stay per the taxonomy above; success requires
  AVAILABLE=1 observed, not just the declaration row.

## R12/R13 amendment — entry lock and incomplete-status propagation (2026-11-08)

The R12 independent audit replayed the repaired tool and found two
remaining blockers, fixed under the R13 order:

- B1 — cross-process lost update: the R11 snapshot check is a
  read/compare/replace sequence, not an interprocess CAS. Two entries
  loading the same ledger bytes could both pass the comparison and
  last-writer-wins. The tool now takes a stable sidecar lock
  (`<active receipts>.lock`, `fcntl.flock`, mode 0600) covering the
  whole entry lifecycle — resolved `--receipts` binding, ledger
  load/initialisation, identity validation, all daemon operations,
  every ledger persistence, release. The sidecar inode is never the
  receipts inode (atomic `os.replace` swaps the latter) and is never
  deleted on release; the descriptor is non-inheritable so children
  do not carry it. A finite monotonic wait (`DS_ISO_LOCK_WAIT_SEC`)
  refuses non-zero with zero daemon calls and no ledger write. The
  lock is per active receipts path — independent runs are not
  globally serialised — and does not claim protection against writers
  that bypass the protocol; the snapshot check still detects those.
- B2 — silent incomplete exits: `capture_incomplete`, `volume.inuse`,
  inspect/mount-query/removal failures and identity refusals were
  recorded in the ledger while the process still exited 0. Every such
  unclosed result now propagates non-zero through `main()`, the `run`
  wrapper, patched cleanup and `compose_down_rc`; the ledger retains
  the refused/unknown/failure records so a later retry — after the
  obstruction clears — can complete without erasing history.
  Explicit NotFound remains the idempotent success it always was.

Both repairs are verified offline only (51 fake-daemon contract
cases including real-`main()` subprocess lock races); no real
daemon/database/resource action was taken or is claimed.

R15 follow-up (2026-11-08): the R14 differential audit found one
remaining hole — a `load_run` failure after lock acquisition (corrupt
ledger, identity mismatch, unreadable file) leaked the held lock fd,
because `main()`'s finally only covers the dispatch phase. Standalone
CLI processes were unaffected (OS reclaims at exit), but the
in-process `iso.main()` convention used by the test driver would see
every later retry on that ledger refused by its own abandoned lock.
`load_run` now wraps the post-acquire read/initialise/validate block
in a `BaseException` boundary that releases the lock and re-raises
the original failure unchanged; a successful load still transfers
lock ownership to `main()`'s finally. Verified offline (55 cases:
corrupt-JSON, binding-mismatch and non-SystemExit load failures each
release for a corrected same-process retry, and a control proves the
lock stays held from successful load through dispatch).

## Links

- Manifest: `testdata/ddl-golden/T23.json`
- Runner: `scripts/ddl_golden.py` (`INSTANCE_FACT_QUERIES`,
  `_execute_metadata_case`, `cli_case_expect_checks`)
- Lifecycle: `scripts/ddl_golden_isolated.py`
- Product ADR: `docs/decisions/2026-11-13-tidb-auto-random-validity.md`
