# T07 verification evidence — adf4652e

SHA-bound evidence for the T07/#86 advanced-column implementation
(generated columns, expression defaults, column visibility) and the
milestone's serial golden regression. Internal review evidence only — not
an external CI, reviewer, merge, or release claim.

## Head and binaries

- HEAD: `adf4652ebb083acd9548a97bc3c579b1d36760f0`
  (`milestone/mysql-tidb-ddl-completion`)
- Golden-built CLI SHA-256: `identity/binary-sha256.txt`
- `go version -m`: `identity/binary-version.txt` — revision `adf4652ebb08`
  with `+dirty` caused by the untracked local `.mcp.json` environment file;
  `git status` otherwise clean at run time.
- Go: `identity/go-version.txt`

## Golden results (serial, single head)

| Task | Cases | Passed | Assertions | Head |
|---|---|---|---|---|
| T02 | 10 | 10 | 57 | adf4652e |
| T03 | 118 | 118 | 1394 | adf4652e |
| T04 | 163 | 163 | 1898 | adf4652e |
| T05 | 247 | 247 | 5116 | adf4652e |
| T06 | 496 | 496 | 11772 | adf4652e |
| T07 | 207 | 207 | 6190 | adf4652e |

- `golden/T07/`: the T07 artifact, all 207 per-case evidence files, and the
  generated policy files.
- `golden-run.log`: the passing T07 runner output.
- `regressions/<TASK>/`: T02–T06 artifacts, case files, and policies from the
  same-head serial regression (`regressions/serial.log`).

## Red→green record

- `red/first-run-charset-failures.txt`: the first T07 run at `c5986c4c`
  (199/207) — eight mysql80/mysql84 native-step failures caused by the
  unpinned exec client negotiating latin1 and storing `_latin1` expression
  introducers while the frozen evidence recorded utf8mb4 sessions.
- `native-probe/session-charset-probe.txt`: the live 8.0.46 probe showing
  `_latin1\'a\'` (unpinned) vs `_utf8mb4\'a\'` (pinned) — the U09 root cause.
- Fix commit `adf4652e` pins `--default-character-set=utf8mb4` on every T07
  anchor `exec_client`; oracle, whitelists, and the 207-identity digest
  (`d723ac95…f1d25`) are unchanged.

## Offline gates at HEAD

- `gates/unit-and-validator.txt`: `make test` all packages ok;
  `make ddl-golden-validator-test` — 728 constructed contract checks + 86 T07
  constructed checks, zero failures.
- Three-level doc check (L3 headers + L2 module READMEs): OK.
- `scripts/t07_contract.py`: 207 required cases, 36 cli (2 baseline + 34
  offline), 163 metadata, frozen ID digest intact.
- `scripts/t06a17proof` and `scripts/t07proof` package tests green.
- Fixture credentials entered Docker only through `T07_MYSQL_PASSWORD` /
  `DS_T04_GOLDEN_PW` environment variables; no values in argv or artifacts.

## Boundaries

- Constructed validator checks are not product or native acceptance.
- This evidence does not close issues, merge, release, or claim external
  reviewer/CI acceptance; it supports human review of the milestone branch.
