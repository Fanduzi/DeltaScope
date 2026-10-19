# T07-R1 verification evidence — aba44783

SHA-bound evidence for the T07-R1 semantic-closure amendment (issue #86,
advanced-column facts): declaration-time generated-reference premise
validation and whole-shape TiDB `SET DEFAULT` adjudication. Internal review
evidence only — not an external CI, reviewer, merge, or release claim.

## Head and binaries

- HEAD: `aba4478328875790e39ed6aeaae0ad67c6d5a996`
  (`milestone/mysql-tidb-ddl-completion`), parent `994bd8eb3e84`.
- Golden-built CLI SHA-256: `identity/binary-sha256.txt`
- `go version -m`: `identity/binary-version.txt` — revision
  `aba447832887` with `+dirty` caused by the tracked-but-uncommitted local
  `.mcp.json` environment file; `git status` otherwise clean at run time.
- Go: `identity/go-version.txt`

## Golden results (serial, single head)

| Task | Cases | Passed | Assertions | Head |
|---|---|---|---|---|
| T02 | 10 | 10 | 57 | aba44783 |
| T03 | 118 | 118 | 1394 | aba44783 |
| T04 | 163 | 163 | 1898 | aba44783 |
| T05 | 247 | 247 | 5116 | aba44783 |
| T06 | 496 | 496 | 11772 | aba44783 |
| T07 | 207 | 207 | 6190 | aba44783 |

- `golden/T07/`: the T07 artifact, all 207 per-case evidence files, and the
  generated policy files.
- `golden-run.log`: the serial runner output.
- `regressions/<TASK>/`: T02–T06 artifacts, case files, and policies from
  the same-head serial regression.
- The frozen T07 oracle, whitelists, and the 207-identity digest
  (`d723ac95…f1d25`) are unchanged; R1 regressions live outside the frozen
  denominator.

## Red→green record (real entry captures)

Fixed inputs through the built CLI, `t07-all` policy, JSON output:

- `entry/prefix/`: run at `994bd8eb` (pre-R1 base). Root-cause 1 proof:
  `G_SELF` (`b INT AS (b + 1)`) and `G_MISSING` (reference to an absent
  name) produced zero findings and zero gaps — rejected declarations
  audited clean. Root-cause 2 proof: `D_CASE`/`D_PARENS` audited clean
  because `CASE … uuid()` and `((uuid()))` inherited the bare-uuid
  verbatim positive — fold cells published as expression defaults.
- `entry/postfix/`: same inputs at `aba44783`. `G_SELF`, `G_MISSING`,
  `G_GEN_FWD`, `G_CYCLE` and the post-statement controls now reject with
  `generated_*` reason codes; `D_ARITH`/`C_JSON_WRAP`/`D_WRAP_ENUM`/
  `D_CTS_DEC` reject `tidb_default_expression`; the verbatim/folded
  positives (`D_UUID`, `D_UUID_DATE`, `D_JSON_INT`, `D_WRAP_VBIN`,
  `D_LOWER_VC`, `D_CTS_BIG`, `D_ARITH_SET`, `D_PARENS`, `D_CASE`,
  `D_CONSTANT`) pass; MySQL opposite controls (`M_ARITH`, `M_CASE`)
  unchanged.

## Native probe evidence (TiDB 8.5.0, live catalog reads)

- `native-probe/native_gen*.txt`: four-anchor generated-reference premise
  probes (self/missing/order/cycle/literal/qualifier cells) — the R1-1
  classification basis.
- `native-probe/native_default*_run.txt`: setter shape, grammar, and
  allowlist probes — bare/wrapped/CASE/arithmetic/temporal/literal forms
  plus MySQL opposite controls.
- `native-probe/native_*sweep*.txt`: target-base sweeps pinning the
  outcome tables: uuid() verbatim on 15 non-1101 bases; json_* verbatim
  cells; `((uuid()))`/`((lower('x')))` fold cells; `(1+2)` fold cells;
  `(current_timestamp)` fold cells including the BIGINT/DOUBLE numeric
  form and the TIME part; NULL-catalog accept cells on DATE-family
  targets.
- `native-probe/native_sweeps.sh`: the canonical re-runnable sweep script
  (uses the throwaway `r1probe` schema; never touches `golden`).
- `native_default3b_run_clean.txt` replaces a first capture whose helper
  quoting corrupted the SQL (`x27` artifacts); the clean re-run is the
  authoritative record for those cells.

## Offline gates at HEAD

- `gates/gates-final.txt`: `go test ./...` all packages ok;
  `go test -race` on the three touched packages ok;
  `make ddl-golden-validator-test` — 86 constructed T07 checks +
  contract suite, zero failures; `make sql-corpus-gates`,
  `make ddl-inventory-gate`, and `make build` (all three binaries,
  PostgreSQL tags) pass; `scripts/t07_contract.py` reports 207 required
  cases / 36 cli / 163 metadata with the frozen digest intact;
  three-level doc check OK.
- Fixture credentials enter Docker only through `T07_MYSQL_PASSWORD` /
  `DS_T04_GOLDEN_PW` environment variables; no values in argv or
  artifacts.

## Boundaries

- Constructed validator checks are not product or native acceptance.
- The sweeps bound only the probed cells; unobserved callees, composite
  shapes, and target bases stay bounded gaps by contract.
- This evidence does not close issues, merge, release, or claim external
  reviewer/CI acceptance; it supports human review of the milestone
  branch.
