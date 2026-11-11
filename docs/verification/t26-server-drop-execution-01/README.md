# T26-SERVER-DROP-EXECUTION-01 — Acceptance Archive

Independent reviewer ruling: **R2 PASS** — this slice
(`T26-SERVER-DROP-EXECUTION-01`, MySQL-only conditional privilege
premise + shared adjudication + ordered `OrderedAbsent` post-state for
audited identifier-form `DROP SERVER`) is accepted. The PASS covers
this slice only; it does not extend to other SERVER forms, TiDB
semantics, or any later milestone.

## Reviewed identities

| role | commit | tree | parent |
|---|---|---|---|
| Accepted product candidate (R2) | `9dad37b873a8938ae79e4212d1ce8e6c1ba80b73` | `eb544128238d8605d6d45df7cda6c3f7ff35f17a` | `d2fb5dbacd4483024310460b6394bbb2ab967697` |
| Retained R1 evidence commit | `d2fb5dbacd4483024310460b6394bbb2ab967697` | `f4bd6f3da60e28c1a6d12d8ab37a20f86a13d964` | `8b52fd55c8452731bfbbc64f3fa0ba440004b04b` |
| Base implementation candidate | `8b52fd55c8452731bfbbc64f3fa0ba440004b04b` | `be1d1236...` | `3a02f73f...` |

R1 (`d2fb5dba`) carried the product-source revisions: `set_default_role`
scope terminator, effective-mode merge-then-validate, provider read
distinctions (query error / zero rows / scan / iteration), bounded
public wording, and the inherited conditional-source projection
(`publishedArm`, `OrderedAbsentConditional`, `ConditionalSource`).
R2 (`9dad37b8`) is test-and-documentation only — consumer assertions,
comments, README/ADR corrections; no product-source change.

## Evidence attribution

### Test logs (delivered as R2 receipt segments, gz+base64)

| receipt segments | file | bytes | sha256 | attribution |
|---|---|---|---|---|
| 回执 21–47 (five-pkg 1/27–27/27) | `/tmp/t26r1-five-pkg.log` | 2,261,266 | `7ea6f65e10652f21b2c72f06fb5b46d2b1cc5dfcb71d20bb8532cc01148a92e8` | `d2fb5dba` — `go test -count=1 -v` on `./internal/domain/spec ./internal/application/audit ./internal/infrastructure/metadata/mysql ./internal/domain/rule/ddl ./pkg/deltascope` |
| 回执 48–68 (four-pkg 1/21–21/21) | `/tmp/t26r2-four-pkg.log` | 1,790,679 | `88f2d0f95249119dc017b149bc8a69abdcc55a0769820e4c0e5e96ac2f9a9b4a` | `9dad37b8` — `go test -count=1 -v` on `./internal/domain/spec ./internal/application/audit ./internal/infrastructure/metadata/mysql ./pkg/deltascope` |

Counts registered by the reviewer:

- five-pkg (`d2fb5dba`): RUN 7288; PASS 7288 incl. subtests;
  top-level PASS 2429; FAIL 0; SKIP 0.
- four-pkg (`9dad37b8`): RUN 5620; PASS 5620 incl. subtests;
  top-level PASS 1774; FAIL 0; SKIP 0.

### CLI unit evidence (R1 round, `d2fb5dba`-era originals)

| file | bytes | sha256 |
|---|---|---|
| `/tmp/t26exec-r1-evidence/run_cli_cases.py` | 856 | `047f4eb2bea4e68e51fbbfa31a7d0ffb311f4a540674049c2f31d706aa09b2c9` |
| `/tmp/t26exec-r1-evidence/cli-results.json` | 25,458 | `eb5ecba828aa67c426e79b288d68f92a7c7c8d543224fad94b56ea97606938d5` |
| `/tmp/t26exec-r1-evidence/work/golden-policy-server-drop.yaml` | 29,575 | `582bbb1078911b74eb801d205befa2965e21dba2b4d418e60898c84c7a0bf9d6` |
| `/tmp/t26exec-r1-evidence/work/golden-policy-all-off.yaml` | 29,558 | `176f813d40e771a4cf08d47376bbe8701e35b41e165f036e0f6fd9774412b3ad` |
| `/tmp/t26exec-r1-evidence/deltascope` (binary built from the `d2fb5dba` clean tree) | 37,070,578 | `fadc30a601c9c94632dba30abe583c12c67aff94d49adc0c977b47d10e6dad8c` |

Three CLI cases (`mysql-drop-server`, `mysql-drop-server-if-exists`,
`mysql-server-mixed-batch` over `testdata/ddl-golden/T26.json` via the
`ddl_golden.py` runner path): all `status: pass` with every assertion
`ok`; complete argv/stdout/stderr/exit/expected/assertions were
delivered in the R2 receipt originals. Template-label ambiguities in
any summary text defer to the actual/expected/detail records.

### Source closures delivered (R2-E3)

- `resolveServerBarrier` — `internal/application/audit/batch_state_server.go:134-157`
- `serverEntryFor` — `internal/application/audit/batch_state_server.go:113-124`
- `Metadata.ServerDrop` attach → `state.apply` — `internal/application/audit/metadata.go:602-635`
- `OrderedAbsent` consumption with preceding barrier/taint context — `internal/application/audit/batch_state_server_srs.go:56-98`

## Unverified / not re-registered

- `/tmp/t26exec-evidence/deltascope` (the earlier evidence directory
  binary): whether it was a byte copy or a separate build is
  **unverified** — registered as 【未核对】. Byte-level equality with
  the `d2fb5dba`-built binary is not claimed.
- Historical FAIL evidence (earlier native runs, R1 22/24, R17
  13/24 harness failure, NATIVE-01 48/51) remains preserved
  unmodified; nothing here rewrites an earlier FAIL into PASS.
- The reviewer is read-only: it consumed delivered materials and did
  not execute project commands, the wrapper, or the binary.

## Boundaries kept

- No `go test ./...` or other full-repo run was used for R2; only the
  four actually-modified test packages were exercised.
- No push, merge, issue close, tag, release, or T27 work is implied
  by this archive.
