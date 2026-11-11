# T26-SERVER-DROP-01 — acceptance record (PASS, minimal audited slice)

Status: **PASS** by independent reviewer (ruling "T26-SERVER-DROP-01 /
R2：PASS，解除最后一项 HOLD", issued after the R2 supplementary
receipt). This document archives the ruling; nothing here was
re-executed for archival. T26/#105 overall remains **OPEN** — this
slice passing does not complete the SERVER family or the rest of T26.
T27 not advanced. Worker = Devin (implemented and ran everything
locally); Reviewer = independent auditor of delivered material, no
independent code execution.

## 1. Accepted scope

The minimal audited slice for **MySQL identifier-form `DROP SERVER`**:
coverage-boundary narrowing into the audited path, existence-fact
consumption, one notice plus bounded evidence gaps, the
unknown/barrier ordered channel (unaudited write records, channel
taint, relation protection), table-state isolation, and the public
output shape (verbatim name redaction, `if_exists` metadata,
`unverified` coverage, `review` verdict).

## 2. Final product/test identity

| item | identity |
| --- | --- |
| commit | `2ee906d55cdc3916e5169f090eaf701d17a3bb9d` |
| tree | `7c6ea98c999f54bb60cda72495860d8ee26d2cb0` |
| parent | `5de2da5e9fb458355b090159698f4c54cfe816e9` |
| semantics contract | `docs/verification/t26-semantics-01` approved at `b8a67e7b5f7e62a825adace06113a233240d28df` |

## 3. Identity chain (unchanged, no history rewrite)

`23fa186f` (t26-semantics-01 acceptance registration) → `c0d09649`
(archive repair: `mcp_exit.go` → `mcp_exit.go.txt` rename + 2 reference
syncs, 3 archive paths) → `e9d1b5ae` (original product candidate, 24
product paths) → `5de2da5e` (R1 fix, re-modified 7 of those paths) →
`2ee906d5` (R2 test increment, +3 lines on one of the same paths).

- Product segment `c0d09649..e9d1b5ae`: **24 paths**.
- Archive-repair segment `23fa186f..c0d09649`: **3 paths**.
- Union: **27 paths**.
- `e9d1b5ae..2ee906d5` cumulative: **7 distinct paths** (R1 touched 7;
  R2 re-touched `batch_state_server_drop_test.go`, already inside the
  7). An earlier receipt's "8 paths" claim was wrong; corrected here.

## 4. R1 issue → closure basis

| R1 item | closure |
| --- | --- |
| Public message claimed deletion | Both notice arms rewritten — present arm "proven present at bind time; execution remains unverified", other arm "existence and execution remain unverified"; no dropped/removed/connection-target wording |
| Unconditional dependent-failure claim | `Risk` narrowed — FEDERATED tables survive the definition drop; effects conditional on "if the removal lands"; `Why` keeps `mysql.servers` row + SUPER per documented semantics |
| Rule message assertions | Present/unknown arms assert corrected semantics (proven-present + unverified; unverified; banned-completion tokens absent); new explanation-boundedness test |
| TABLE pre-state isolation | `TestT26DropServerTableSiblingsUnaffected` now uses real fixtures — batch-created `t` keeps `Exists`+`id` and provider-vouched `u` keeps `Exists`+`k` after the SERVER statement |
| Barrier public shape | `TestT26DropServerBarrierPublicShape` proves second (case-alias) statement consumes the barrier: notice + `if_exists:true`, `batch_effects`+`drop_privilege` without `existence`, `unverified` coverage |
| Top-level verdict | New `result.Verdict == report.VerdictReview` assertion; targeted `-v` run PASS on `2ee906d5` (`verdict-test-run.log`) |
| Three CLI units | `manifest-cli-results.json`: `mysql-drop-server` rc=0 review/unverified + notice + existence/privilege gaps; `if-exists` same + `if_exists:true`; `mysql-server-mixed-batch` rc=1 with the CREATE SERVER unaudited boundary intact — all matching manifest expectations under the real `make_policies`/`execute_cli_case` path |

## 5. Run-record attribution (bound to actual run identity)

| record | bound identity |
| --- | --- |
| three-package `go test -count=1`, gofmt, go vet | `5de2da5e` (`three-pkg-test.log`) |
| `mysql-drop-server`, `mysql-drop-server-if-exists`, `mysql-server-mixed-batch` CLI units | binary built on the pre-commit working tree containing all 7 R1 file changes, whose committed content is `5de2da5e` (`manifest-cli-results.json`, generated policies) |
| `TestT26DropServerBarrierPublicShape` top-level verdict | `2ee906d5` (`verdict-test-run.log`) |
| product code delta `5de2da5e..2ee906d5` | test-only (+3 assertion lines); no product change |

## 6. Still-open / not-implemented (unchanged registration)

- `server.drop_privilege` evidence remains unwired — always owed.
- No `OrderedAbsent`, no successful absent publication.
- `CREATE/ALTER SERVER` lifecycle semantics not implemented (named boundaries).
- SRS semantics unchanged; literal `DROP SERVER 'name'` remains a named boundary; TiDB vendor boundary unchanged.
- No permission/grant collection, no dependency reads, no connection test.
- No native SQL, Docker, live-DB verification — all evidence is offline.
- No push, merge, close, tag, release.

## 7. Raw-evidence index

| evidence | archived file | sha256 |
| --- | --- | --- |
| three-package test log | `three-pkg-test.log` | see SHA256SUMS.txt |
| manifest CLI results (single copy; original tmp paths `/tmp/t26-r1/results.json` and `/tmp/t26-r1-receipt/manifest-results.json` were byte-identical) | `manifest-cli-results.json` | see SHA256SUMS.txt |
| verdict targeted run | `verdict-test-run.log` | see SHA256SUMS.txt |
| server-drop isolated profile | `golden-policy-server-drop.yaml` | see SHA256SUMS.txt |
| all-rules-disabled profile | `golden-policy-all-off.yaml` | see SHA256SUMS.txt |
| R1 increment diff | `e9d1b5ae-to-5de2da5e.diff` | see SHA256SUMS.txt |
| R2 test-increment diff | `5de2da5e-to-2ee906d5.diff` | see SHA256SUMS.txt |

Runner assertion labels are preserved verbatim; `zero findings` /
`no diagnostics` label text does not match the non-zero expectation
shape — verification compared `actual` vs manifest `expected` and
actual values matched. The tested binary (`/tmp/t26-r1/deltascope`,
37 MB) is not archived; its build command and path are registered in
`build-invocation-record.md`.
