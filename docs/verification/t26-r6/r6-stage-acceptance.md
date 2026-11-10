# T26 offline stage — R6 acceptance record (SCOPED OFFLINE PHASE)

Status: **APPROVED — SCOPED OFFLINE PHASE** by independent reviewer
(work order T26-R6, "T26 注释闭合修正与离线证据收口").
This document archives the reviewer's final ruling on the T26 / #105
recognition + named-boundary + output-redaction phase. The tested
snapshot is frozen; this file is archival only and does not describe a
re-execution.

## 1. Tested identity (accepted candidate)

- commit: `16eba8a08fa797549bc8dae428bf75536b0fa385`
- tree: `a537f033270ed0437380c3ebcc1648ccd2cc50f9`
- parent: `716501edf852836bdfb59fae48f9f441a13c24d3` (R5 baseline)
- CLI binary SHA-256:
  `307ffeb0d0ff14b1c4b20a8635506618fddd7c96eaea90742d53ee529923a18b`
- build: `CGO_ENABLED=1 go build -tags postgresql -o bin/deltascope
  ./cmd/deltascope`, go1.26.1 darwin/arm64
- manifest `testdata/ddl-golden/T26.json` SHA-256:
  `67e7044d1df13b837782a0efcd2c16cb6c0f2fd095974634927e67f16fd4d2e7`
- stage policy SHA-256:
  `ea767529d6f0199ffd605522fdf2da7bf473cf0f53a308dba042819fb330198a`
- stage harness `run_cli_cases.py` SHA-256:
  `464312433608729ce45beb5bfc8b6c3398b2054378702d1e982e9cdd2a6a3755`

## 2. Recorded results (as ruled)

| Check | Result |
|---|---|
| Targeted parser tests (R6 recognizer layer) | 15/15 PASS |
| Audit diagnostic entry test (R6) | PASS |
| Full repository `go test ./... -count=1` | 39 packages PASS, 5 no-test, 0 FAIL |
| Independent offline CLI stage | 23/23 PASS |
| Product artifact marker scan | 25 clean, 0 HIT, 0 ERROR |
| Positive controls (4 markers) | 4/4 detected |

R6 red-state evidence is split: `r6_red_recapture` is an explicitly
labeled recapture run on the old compilable combination (15 subtests:
6 FAIL / 9 PASS — `server_mid_unclosed` stayed a correct-reject case);
it does not rewrite the original R6 red log. The R5 historical test
combination is recorded only as "a compilable reconstruction consistent
with the log", never as a uniquely recovered run.

## 3. Accepted scope (what this ruling covers)

- Unterminated block comments are no longer removable-then-claimed:
  `scanBlockComment` keeps the `*/` result, `tokenizeSQLState` carries
  the state, and the SERVER/SRS/ROTATE whole-input recognizers refuse
  to claim before shape checks. `tokenizeSQL` keeps its signature.
- MySQL SERVER family (`create_server`, `alter_server`, `drop_server`):
  named `unaudited` boundary; public SQL is a fixed redacted skeleton.
- MySQL SRS (`create_spatial_reference_system`,
  `drop_spatial_reference_system`): named `implementation` boundary;
  attribute payloads never enter serialized output.
- TiDB SERVER/SRS/ROTATE forms: `vendor` boundary.
- `ALTER INSTANCE RELOAD TLS` keeps precise
  `alter_instance.reload_tls` feature with `NO ROLLBACK ON ERROR` as
  bounded member metadata; `ROTATE INNODB MASTER KEY` keeps
  `alter_instance.rotate_innodb_master_key`.
- Splitter → recognizer → `parser_error` diagnostic path verified at
  the real CLI entry (exit 2, 0 retained statements, no marker echo).
- All four serialized SQL assembly sites use `PublicSQL()` /
  `PublicNormalizedSQL()`; pre-populated `item.SQL` cannot override
  redaction for sensitive families.
- T26 batch-state early returns sit inside `statement.Unsupported !=
  nil` before generic invalidation — limited to the tracked
  schema/table model; no lifecycle or post-state claim.

## 4. Deferred scope (not accepted, not authorized)

- Manifest `db_*` required cases: 8 cells — unexecuted, unverified.
- Native configuration, resource operations, and any execution
  authorization.
- SRS WKT validation, provider fact loaders, SERVER/SRS lifecycle and
  ordered post-state, and all oracle-pending semantic units (including
  decision-table row 13 `srs-version-unknown`).
- SQL-mode/version-comment semantics beyond this phase; no other SQL
  family consumes the new lexical state.

## 5. Evidence archive

Independent R6 archive (offline only, zero native execution, zero
remote writes). Raw records copied into this directory:

| Item | File |
|---|---|
| Build record | `BUILD-IDENTITY.txt` |
| Stage summary | `stage-summary.json` |
| Stage log | `stage.log` |
| Per-file product scan record | `scan_record.txt` |
| Archive identity manifest | `SHA256SUMS.txt` + `INPUT-HASHES.txt` |
| R6 red recapture / green / CLI-pair / suite logs | `r6_red_recapture.log`, `r6_green_full.log`, `r6_cli_pair.log`, `r6_suite.log` |

The 23 per-case JSON outputs, full `T26.json`, and `policy.yaml` were
delivered to the reviewer verbatim; the originals remain under the
independent stage archive (`/tmp/t26stage/r6run`, ephemeral).

## 6. Verbatim final ruling

```
T26-R6 FINAL REVIEW

Decision: APPROVE — SCOPED OFFLINE PHASE
HOLD: CLEARED
Outstanding evidence requests for this round: 0

Candidate:
  commit 16eba8a08fa797549bc8dae428bf75536b0fa385
  tree   a537f033270ed0437380c3ebcc1648ccd2cc50f9
  parent 716501edf852836bdfb59fae48f9f441a13c24d3

Accepted scope:
  R6 未闭合块注释认领修复；
  T26 SERVER / SRS / 指定 ALTER INSTANCE 的识别、具名边界及输出脱敏；
  所交 manifest、policy、23 个 CLI case 与相关回归、诊断、扫描证据的阶段对账。

Recorded results:
  Targeted parser tests: 15/15 PASS
  Audit diagnostic test: PASS
  Repository test log: 39 packages PASS, 5 packages with no test files, 0 FAIL
  Independent offline CLI stage: 23/23 PASS
  Product scan: 25 clean, 0 HIT, 0 ERROR
  Positive controls: 4/4 detected

Evidence provenance:
  r6_red_recapture 为明确标注的补捕获运行，不替代原始红态记录。
  R5 历史测试组合仅记为与日志相容的可编译复原方案。
  历史原始记录保持不变。

Deferred scope:
  8 个 DB required cases 未执行、未验证。
  原生配置、资源操作和执行授权未获本次批准。
  WKT / provider / SERVER-SRS 生命周期 / post-state / 待裁决语义单元未验收。

Disposition:
  本轮材料审查结束，无新增补件、复跑或 R7 要求。
  可将本裁决追加归档至 T26 验证记录。
  T26/#105 保持未关闭；本裁决不包含 push、merge、close、tag 或 release 授权。
```

## 7. Disposition

- T26/#105 remains open; this ruling completes the recognition +
  named-boundary + redaction phase only.
- No push, merge, issue close, tag, release, or native execution is
  authorized by this archive.
