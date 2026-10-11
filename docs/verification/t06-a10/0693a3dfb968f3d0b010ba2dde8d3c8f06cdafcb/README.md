# T06-A10-R1: observation record-set validation

Code under test: `0693a3dfb968f3d0b010ba2dde8d3c8f06cdafcb`
(`test(ddl): bind T06-A10 observations to exact record-set shape (#85)`)

This evidence commit is the direct child of the validator fix. The
artifact re-validated here is the **original** 292-case run at
`bffcc261` — its `head_sha` stays `bffcc261`, unchanged and not
relabeled to the R1 validator SHA.

## What was fixed

`t06_a10_artifact_failures` previously checked only the **last non-empty
line** of the allocator and raw EXTRA stdout, and only required
`"CREATE TABLE"` to appear anywhere in the SHOW CREATE stdout. A
wrong-target or malformed row ahead of the real payload, duplicate data
records, or a header-only result could therefore pass as valid
observations.

Now each observation must be its step's **fixed client header plus
exactly one data record**:

- allocator: header `observation`, then exactly one
  `allocator|golden|t|(null|value:<digits>)` record — verbatim observed,
  never numerically asserted against the declared init value;
- raw EXTRA: header = the literal GROUP_CONCAT select expression, then
  exactly one record whose members name the declared columns in ordinal
  order with the role-frozen HEX (existing member/hex checks unchanged);
- SHOW CREATE: header `Table\tCreate Table`, then exactly one record
  naming object `t` and opening `CREATE TABLE `t`` — the DDL body stays
  an opaque observation (TiDB version comments and table options are not
  parsed or re-asserted).

Header equality is exact; an arbitrary first line cannot masquerade as a
header; every post-header line counts as a record (no blank-line
filtering hides damage); malformed shapes produce failure strings, never
exceptions.

## Directory layout

| Path | Contents |
| --- | --- |
| `gates/` | argv/stdout/stderr/rc originals for every gate below. |
| `identity/` | `source.txt` (validator SHA vs tested-artifact SHA dual attribution, worktree state, Python version); `hashes.txt` (sha256 of the two changed scripts, README, ADR, the original artifact, and the original product binary — referenced by hash, not copied). |

## Gate summary

| Gate | Result |
| --- | --- |
| `python3 -m py_compile scripts/ddl_golden.py scripts/test_ddl_golden.py` | rc=0 |
| `python3 scripts/ddl_golden.py validate --artifact /tmp/ds-t06-a10-proof/T06/artifact.json` | rc=0, `artifact valid` — original `bffcc261` artifact, binary check enabled against the original run's binary |
| `make ddl-golden-validator-test` | rc=0, 445 contract cases, 0 failures (422 prior + 23 R1) |
| `make decision-record-gate` | rc=0 |

No Docker started, no Golden rerun, no product rebuild — all four
database anchors' raw records already satisfy the tightened shape, which
the original artifact's successful validation confirms.

## R1 mutation coverage (all through the full `validate_artifact` entry)

- **allocator**: duplicate identical rows, conflicting values,
  wrong-target row ahead of the real row, malformed row ahead of the
  real row, header-only result, wrong table, missing header — all
  rejected; legal `null`, `value:0`, `value:8`, `value:9` single records
  all accepted.
- **raw EXTRA**: duplicate payload, wrong-member record ahead of the
  real record, header-only, swapped member order, phantom member,
  missing header — all rejected; empty-HEX members (pk-no-auto,
  composite) remain valid records, never treated as missing.
- **SHOW CREATE**: object column naming `u`, DDL head opening `u` under
  object `t`, bare `CREATE TABLE` keyword row, header-only, duplicate
  records, missing header — all rejected.

## Allocator observation recap (unchanged from the original run)

The per-role catalog observations recorded at `bffcc261` remain
accurate: mysql80/mysql84 `pk-auto` → `null`; the three MySQL anchors
report `value:8`/`value:9` for the explicit-init roles; TiDB 8.5 reports
`value:0` for its auto-increment roles. These are catalog observations
from that run — not runtime ID guarantees and not upgraded to oracles by
this round.
