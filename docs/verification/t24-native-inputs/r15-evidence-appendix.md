# T24-R15 evidence appendix — historical run identity, grading, and the frozen minimum revalidation plan

Status: appendix beside the `e8320ce76827` artifacts. It adds records;
it does not modify any attempt artifact, manifest history, or the
receipts ledger. The historical run permanently stays **24 executed,
22 pass, 2 fail**; nothing here rewrites that result.

## 1. Per-attempt identity table (from each attempt's own artifact.json)

| attempt | cli.sha256 (built binary) | input generation | score | distinguishing input |
|---|---|---|---|---|
| 1 | `0da04de1…` (binary A) | pre-D2 code, post-password manifest | 13/24 | connect `password_env` present; DROP-style setup |
| 2 | `0da04de1…` (binary A) | pre-D2 code | 20/24 | same binary; run env now carried `T21_MYSQL_PASSWORD` |
| 3 | `0767b376…` (binary B) | D2 repair + manifest setup fix | 21/24 | mysql57 cell turned green |
| 4 | `0767b376…` (binary B) | absent-assertion setup | 22/24 | final artifacts under `run/golden/T24/` |

`cli.build.head_sha` reads `e8320ce7` on every attempt — it records the
Git HEAD, not the dirty worktree. Binary identity divides cleanly at
attempt 2→3, matching the D2 repair timeline: attempt 1→2 changed only
the manifest (same hash), 2→3 changed product code (hash changed),
3→4 changed only the manifest (same hash).

Binary-level attribution of binary B: `vcs.revision=e8320ce7` +
`vcs.modified=true` is embedded in the retained artifact binary itself
(`run/golden/T24/bin/deltascope`, sha256 `0767b376…`), and `go tool nm
-size` against a HEAD (`69807f93` = `8765f0b3` product code) rebuild at
the same path shows an identical 26,831-entry symbol set with identical
sizes and an identical 36,777,714-byte file — structural equivalence,
not byte equivalence. Byte-identical reproduction requires building
inside the same repository path with the same vcs stamp, which the
read-only constraint prohibits.

## 2. Evidence grading

- **A — retained artifact binaries + receipts ledger** (proves what ran
  and what it reported): `run/golden/T24/bin/`, per-attempt `cases/`,
  `artifact.json`, `SHA256SUMS`, `receipts.json`.
- **B — offline re-adjudication** (proves what the repaired chain emits
  on the same premise shape): `TestT24R15FrozenOracleShapes` and the
  `TestT24R15*` provider/ordered-state matrices. These are new
  executions on fixed source, not new native receipts.
- **C — structural equivalence** (bounds binary attribution): nm
  symbol-set/size parity + embedded vcs stamps, as above. Cannot prove
  machine-code equivalence; fixed-source attribution is closed only to
  this level and is disclosed as such.
- **Not evidence**: attempt-3/4 artifacts carry `head_sha=e8320ce7` while
  the worktree contained the D2 patch — they cannot be re-presented as
  executions of the final tree. T21 materials are unrelated lineage and
  prove nothing about T24.

## 3. Corrections and deviations ledger (all previously disclosed)

- Container generations: **4**, not 5 — attempt-1 reobserved the `up`
  generation (20 unique container IDs total; all stop/rm rc=0).
- Anonymous volumes: 4 removed; creation was implicit through Compose
  (no independent create records).
- Protected volumes: integrity is volume-list consistency only; the
  persisted complete name list was never snapshotted — byte-level
  verification is not claimed.
- Attempt-1 manifest was not snapshotted; its input lineage is
  reconstructed from attempt-1 case records, not a frozen copy.
- Procedural deviations during execution (mid-run code edits, >1
  container generation vs the `generations:1` budget, post-verdict test
  commit `69807f93`) are recorded in the R13/R14 handoffs and stand.
- This round (R15): zero daemons, zero databases, zero resource
  operations; all proof is offline on the fixed tree.

## 4. Frozen minimum native revalidation set (NOT authorized)

A future authorized run re-proves exactly what the repairs touched:

| arm | cells | what it must show |
|---|---|---|
| real fact preload | `mysql57-version-refused`, `mysql80-alter-absent-refused`, `mysql80-drop-reserved-refused`, `mysql80-rg-lifecycle`, `mysql84-rg-catalog-definition`, `tidb85-mysql-cpu-vendor-boundary` (6 metadata cells) | version-gated probes on 5.7 (no doomed query), propagated vs degraded errors on 8.x, evidence binding through the real CLI chain |
| companion product arms | occupancy census + `evidence-uuid-binding-witness` | `BoundComplete` under `lost=0`; bound vs foreign-uuid discrimination through `cliMetadataProvider`→`auditmeta`→provider |
| offline rerun | 10 CLI cells | unaffected by the provider path; can be re-run without infrastructure |
| historical anchors | 8 db-native cells | may remain historical observations; must not be re-presented as new-tree execution |

Run protocol requirements carried forward: new `RUN_ID`; new receipts
and newly collected uuid-bound evidence (old `DeploymentEvidence` files
are dead — their targets pointed at destroyed instances); record
tracked/staged status before build, build-related untracked inputs, the
actual CLI sha256 + embedded vcs stamp + toolchain, separate hashes for
manifest / policy / evidence; no product or oracle edits during
execution; stop and preserve evidence if a prerequisite fails.

**Selection capability, recorded honestly**: neither
`ddl_golden_isolated.py` nor `ddl_golden.py` supports case-level
selection — `--task T24` executes the whole 24-cell manifest. The
minimum set above is therefore the *impact* set, not a runnable subset:
a revalidation either runs all 24 cells (selected = full manifest,
unselected = none) or stays unauthorized. No cases were deleted,
renamed, or disabled to manufacture green.
