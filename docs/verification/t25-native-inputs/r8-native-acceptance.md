# T25 native verification — acceptance record (R8-NATIVE-01 FAIL + R8-NATIVE-02 PASS)

Status: **APPROVED** by independent reviewer (work order T25-R8-CLOSEOUT-01).
This document archives the two native rounds that closed T25 / #104 native
verification. The tested snapshot is frozen; this file is archival only and
does not describe a re-execution.

## 1. Tested identity (NATIVE-02, accepted candidate)

- HEAD: `bdc83bdda2c2930980b75bd2d7d8ede9da67d74f`
- tree: `6449046e9e4c4ce2f1ea6d4e9be0c6feeb65357b`
- parent: `cdcd62b944eab89266a8898138e1b3f8f5e5dd19`
- native binary SHA256:
  `6f670ddf473db9eb8a902ed8f848151f8ecd624dd87deaea79fadae0947b280d`
- build: `CGO_ENABLED=0 go build -o <artifact>/deltascope ./cmd/deltascope`,
  go1.26.1, built_at 2026-11-10T08:44:12Z (runner-built inside the artifact)

The accepted candidate sits on top of the R8 product baseline
`cdcd62b9...` (tree `58ea2a26...`) with exactly one manifest-calibration
commit (`bdc83bdd`, diff: `testdata/ddl-golden/T25.json` 5+/3-, three
fields). The tested manifest still carries its original `pending_native`
fields; the run that corresponds to this snapshot is recorded here — the
fields were not rewritten to claim the result.

## 2. Two rounds, recorded separately (never merged or rewritten)

| Round | Candidate | Result | run rc | validate rc | assertions |
|---|---|---|---|---|---|
| T25-R8-NATIVE-01 | cdcd62b9 | **FAIL** 48/51 | 1 | 1 | 745 |
| T25-R8-NATIVE-02 | bdc83bdd | **PASS** 51/51 | 0 | 0 | 748 |

NATIVE-01's FAIL is permanent record; NATIVE-02 is an independent complete
run on the new candidate, not a rerun of the three failing cases.

## 3. Artifact index

| Item | Path |
|---|---|
| NATIVE-01 artifact | `artifacts/T25-R8-native-cdcd62b9-20261110-162346-04e71083/T25/artifact.json` |
| NATIVE-01 per-case records | `artifacts/T25-R8-native-cdcd62b9-20261110-162346-04e71083/T25/cases/*.json` (51) |
| NATIVE-01 binary | same dir `bin/deltascope`, sha256 `82bc5377a49f553409288a374499ecbc4f17bfd8efefd74804c83380a130ab66` |
| NATIVE-01 run/validate logs | `native01-run.log`, `native01-validate.log` (this dir) |
| NATIVE-02 artifact | `artifacts/T25-R8-native-bdc83bdd-20261110-164410-00c27700/T25/artifact.json` |
| NATIVE-02 per-case records | `artifacts/T25-R8-native-bdc83bdd-20261110-164410-00c27700/T25/cases/*.json` (51) |
| NATIVE-02 binary | same dir `bin/deltascope`, sha256 `6f670ddf...` (above) |
| NATIVE-02 run/validate logs | `native02-run.log`, `native02-validate.log` (this dir) |
| TiDB offline probe raw output | `tidb85-vendor-boundary-offline-probe.stdout.json`, `tidb85-vendor-boundary-offline-probe.stderr.txt` (this dir) |

All artifacts, per-case JSON, embedded expected/status, and binaries are
preserved verbatim; nothing was repacked as a retroactive PASS.

## 4. NATIVE-01 failure causes → NATIVE-02 bounded fix

Three manifest-side defects (no product adjudication defect found):

1. `mysql80-absent-alter-refused` — `stderr_contains` declared
   `"does not exist"`; MySQL 8.0.46 actually emitted
   `ERROR 3510 (HY000) ... Tablespace t25fx_ghost doesn't exist.`
   Calibrated to `["ERROR 3510 (HY000)", "Tablespace t25fx_ghost doesn't exist."]` (AND semantics, both markers verified against the recorded stderr before the run).
2. `mysql80-rename-collision-refused` — declared `"already exists"`;
   actual `ERROR 1813 (HY000) ... Tablespace 't25fx_cb' exists.`
   Calibrated to `["ERROR 1813 (HY000)", "Tablespace 't25fx_cb' exists."]`.
3. `tidb85-vendor-boundary` — declared `dialect: "mysql"` while connecting
   to the tidb85 anchor; the CLI dialect-identity guard refused pre-audit
   (`detected dialect "tidb" does not match requested dialect "mysql"`,
   exit=2). Calibrated to `dialect: "tidb"` with every other field kept;
   pre-run offline probe (first-round binary + first-round policy,
   `audit --dialect tidb`, no connection args) satisfied all 11 declared
   audit-expect comparisons — raw probe output archived in this dir.

## 5. NATIVE-02 result grouping (748 assertions)

- 8 DB cases — 29 assertions
- 28 CLI cases — 390 assertions
- 15 metadata cases — 329 assertions

All 51 required case IDs present; zero failures.

Notable verified paths:
- ghost ALTER: audit stayed `review`/`unverified` with
  `unknown_tablespace_state` gap owing `tablespace.existence` +
  `tablespace.rename_target`; native 3510 refused; post-count 0.
- rename collision: audit `reject`/`complete` with
  `tablespace_rename_target_present` metadata pin; native 1813 refused;
  both identities survived (count=2); teardown DROPs + absence verify ok.
- TiDB vendor boundary: online audit reached
  `unsupported.feature=create_tablespace`, `metadata.boundary=vendor`,
  `audited=false`, `coverage=incomplete`, `fail_on_triggered=false`,
  `dialect_source="detected"`; native execute returned ERROR 1064 as
  declared. This is the first round where the online audit path for this
  case actually produced a result.
- owned undo lifecycle (`mysql80-undo-owned-lifecycle`): bounded
  deactivate → empty-poll → drop chain executed and verified.
- memberless InnoDB CREATE, DISCARD forbid, IMPORT warn, MyISAM boundary,
  NDB vendor boundary: all passed per their declared contracts.

## 6. Anchors (as recorded in the NATIVE-02 artifact)

| anchor | version observed | image | digest |
|---|---|---|---|
| mysql57 | 5.7.44 | mysql:5.7.44 | `mysql@sha256:4bc6bc963e6d8443453676cae56536f4b8156d78bae03c0145cbe47c2aad73bb` |
| mysql80 | 8.0.46 | mysql:8.0.46 | `mysql@sha256:7dcddc01f13bab2f15cde676d44d01f61fc9f99fe7785e86196dfc07d358ae2b` |
| mysql84 | 8.4.10 | mysql:8.4.10 | `mysql@sha256:8dbcf531a03aade657e181b9cf2f1d1803ce621a1d55610cb44cb531ab7d7db6` |
| tidb85 | 8.0.11-TiDB-v8.5.0 | pingcap/tidb:v8.5.0 | `pingcap/tidb@sha256:490861bd4a2fa643dbe76b77ffecafcad66e6a42774e580b82f4ec2429a80e57` |

Compose project `deltascope-ddl-golden`, file
`docker/ddl-golden-compose.yaml`; all 5 containers reported `healthy`
before the run.

## 7. Teardown and residual record (both rounds)

- `compose_down_rc=0`, `compose down -v --remove-orphans` removed all 5
  containers and the project network; `residual_containers=[]`.
- Post-run read-only check: no `deltascope-ddl-golden` prefixed
  containers, volumes, or networks remained.
- `external_blocker` was null in both artifacts.

## 8. Proof boundaries (registered, still open)

- **ghost / zero-row provider**: the run observed the concrete path where
  the driver counts 0 for both names while the product emits
  `unknown_tablespace_state` owing `tablespace.existence` +
  `tablespace.rename_target`. This is *not* registered as the product
  proving absence, nor generalized to all zero-row scenarios.
- **default undo**: the frozen `tablespace.drop_eligibility` gap output
  passed comparison in `mysql80-undo-default-drop-refused`; default
  `innodb_undo_*` objects were never natively ALTERed/DROPped — runtime
  deletability and native refusal on default objects remain unproven.
- **ENCRYPTION**: this round was a refusal probe; positive `ENCRYPTION='Y'`
  success coverage is unproven. A single `keyring_file` plugin row does
  not represent all keyring implementations.
- **IMPORT**: registered at its declared limited refusal-probe scope.
- **TiDB**: the online product vendor boundary plus independent native
  1064 refusal are now verified for this case; object-level post-verify
  is registered only as actually recorded; the zero-residual claim lives
  in the environment teardown evidence, not in a catalog query.
- **schema-reset DB cases / offline stubs / native execute**: each is
  registered at its own scope; none substitutes for another's proof, and
  known coverage boundaries are not restated as new failures.

## 9. Environment notes

- Single run each round; no second run, no per-case rerun, no expectation
  edits during runs, no `pending_native` changes.
- Fixture credentials: `T21_MYSQL_PASSWORD` supplied at run time;
  `DS_T25_GOLDEN_PW` injected per-case by the runner (`password=root`).
- No keyring installation, no ad-hoc SQL, no broad-prefix cleanup, no
  mutation of default undo or foreign objects.

Native verification for T25 / #104 is complete at this scope. Push,
merge, release, and issue closure were not part of any work order.
