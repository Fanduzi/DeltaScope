# T24-R17/R18 evidence appendix — call-environment defect, corrected fixed-tree canonical pass, and layered result accounting

Status: appendix beside `e8320ce76827` artifacts and `r15-evidence-appendix.md`. It adds records only; it does not modify any attempt artifact, the manifests, or earlier receipts. Historical results stand unchanged: the `e8320ce76827` run stays **24 executed, 22 pass, 2 fail**; T24-R17 stays **24 executed, 13 pass, 11 fail**.

## 1. R17 — executed lifecycle, harness defect, preserved evidence

- RUN_ID `t24-r17-20261110t0037z-509a1c`; EXEC_SHA `78054402c822ca56268fd9be8308aa692736ec08` (product `585c590c` + authorized two-path-only input commit); CLI `38f117acfbf52c733733da8843ac4a46d1439505895d9324901780c0f1156c05` (go1.26.1, CGO=0, `vcs.modified=false`).
- Cap-off hard admission passed on both anchors (mysqld CapEff `0x0`, `priority_capable=false`); fresh UUID-bound evidence files sealed `0600`.
- Fixture + 41 companion records executed through the real CLI → provider → auditmeta → public `AuditSQL` chain; occupancy, privilege (4 arms), read-only (4 arms + restore readback), bound-vs-foreign evidence discrimination, and existing-group ALTER all matched the frozen contract.
- Single canonical `run --task T24`: 24 attempted, **13 pass / 11 fail**. All 11 MySQL cells failed at `SELECT VERSION() rc=1 version=''` — `mysql_exec` early-returns when `anchor.exec_password_env` (`T21_MYSQL_PASSWORD`) is unset, before any `docker exec`. Harness invocation defect (same class as R11 attempt-1), not a product defect; tidb85 (no password contract) and all 10 CLI cells passed.
- One real workload generation; all run resources removed; 8 protected volumes list-identical; profile stopped normally.

## 2. R18 — authorized single canonical rerun after env correction

- RUN_ID `t24-r18-20261110t0058z-d40748`; RUN_ROOT `/tmp/t24-r18-run`; EXEC_SHA `a88341f3fd1d04a57cc11afdc0ad92c881808138` — diff vs product `585c590c` verified as exactly two `--deployment-evidence-file` argument values pointing at the R18 RUN_ROOT; vs R17 input only `r17`→`r18` in the same two values.
- Credential contract recorded (names/sources/status only): `T21_MYSQL_PASSWORD` → anchor exec surface (`mysql_exec` → `docker exec --env MYSQL_PWD`), `DS_T24_GOLDEN_PW` → product connect surface (`--password-env`), `DOCKER_HOST` → pinned daemon. TiDB anchor has no password field and none was injected. Two fake-negative controls (each required var removed) aborted before any resource command.
- Per-anchor preflight through the real imported `ddl_golden.mysql_exec`: `5.7.44`, `8.0.46`, `8.4.10`, `8.0.11-TiDB-v8.5.0` — all rc=0, frozen versions matched. Frozen `ddl_steps` executed on every anchor including TiDB's `needs_database_create` path. Product connect preflight (official CLI + frozen connect/password_env) reached adjudication on all six meta connects; mysql57 returned the version-refusal `reject`.
- Fresh cap-off admission: mysql80 `00c18fc3-dca2-11f1-9a4d-f221638644f4`, mysql84 `00c6e7f2-dca2-11f1-bf51-4eabe2afe732` (new UUIDs, not R17's); CapEff `0x0`, Uid 999, Cpus `0-3`; evidence sealed `0600`.
- Single canonical `run --task T24`: **24/24 pass, 278 assertions**, runner rc=0. Attempt archive `T24.attempts/a88341f3fd1d/attempt-1`, 27 files, SHA256SUMS verified.
- Canonical CLI `c24d2890e8a9609591bc0885ccef2e4683c3d53004576ea7cab412162fe791b4` — go1.26.3, CGO=0, `vcs.revision=a88341f3`, `vcs.modified=false`. The manual preflight binary `1ffb8fc6…` differs by toolchain only (go1.26.1 via interactive-shell PATH vs go1.26.3 via driver PATH); same revision and tree — recorded separately, never claimed equivalent.
- One real creation generation (runner's internal `compose up` reobserved identical IDs); anonymous volume registered at creation via Mounts/`mounted_by`; cleanup S/R/V all rc=0; 8 protected volumes list-identical; profile stopped normally. One ledger-recorded failure preserved: an early `compose up` rejected the uppercase-`T` project name before creating anything (renamed lowercase, rerun once — no second canonical).

## 3. Key closed-evidence cells

- `T24.meta.mysql80-rg-lifecycle` (D4 original three-statement path): stmt0 `affinity.warn`+`priority.warn` + `unknown_resource_group_state`; stmt1 state gap only; stmt2 state gap only — **zero occupancy gap**. The proven `occupied=false` census fact now survives the ALTER early-return into DROP premises — exact frozen cap-off oracle.
- `T24.meta.mysql84-rg-catalog-definition`: 2 warnings + 1 state gap — cap-off shape.
- `T24.meta.mysql57-version-refused`: product-level blocker "requires MySQL 8.0.3 or later and the observed version predates it" — genuine version refusal, D2 native evidence.
- `T24.meta.tidb85-mysql-cpu-vendor-boundary` + 10 CLI cells: vendor boundary preserved; MySQL CPU claims never leak into TiDB.

## 4. Acceptance record (R19 read-only terminal verification)

Independent recompute confirmed: EXEC_SHA chain, 24=required=selected=executed, per-case pass, assertion sum 278, archive hashes, credential early-return source path, single generation, cleanup and protected-set identity, profile stop receipt. R17 companion items map onto 13 contract clauses under their own EXEC_SHA/CLI/UUID identity. Final verdict: **#103 contract coverage signable** within the approved scope; cap-on branch, actual scheduling, and performance guarantees remain out of scope per the frozen contract. No push/merge/issue-close/release/T25 was performed or authorized.
