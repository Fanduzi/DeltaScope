# T06-A15-R1: declared ROW_FORMAT=COMPRESSED identity fix

Code under test: `700e90e4fb2c9a789156f739932727f4e3cfb020`
(`fix(ddl): T06-A15-R1 keep declared ROW_FORMAT=COMPRESSED identity over instance defaults`)

Buggy base (red-state): `32a325c35539bec163677382db587666b0023029`
Evidence parent: `0a4c19d65bd6f7c1273bd2f6f82b702ce2dca9bd`
Frozen T06 denominator: **496 cases — unchanged** (no manifest/profile/case edits).

## The single root cause

`resolveRowSizeFacts` recognized only `DYNAMIC`, `COMPACT`, and
`REDUNDANT` as declared row formats. The extractor already emits
`Options["row_format"] == "COMPRESSED"` verbatim, so a declared
COMPRESSED fell into the instance-default path:

- no provider / nil / empty instance → fabricated
  `missing_instance_fact` gap for `instance.innodb_default_row_format`;
- instance default `DYNAMIC` → declared identity silently replaced;
- instance default `COMPACT`/`REDUNDANT` → the declared format entered
  the wrong format-specific payload branch (8448 > 8126/8000
  pseudo-findings the declaration never authorized).

## The minimal fix

One word in the declared-row-format `switch` plus the comment: the
recognized declared InnoDB set is now exactly
`{DYNAMIC, COMPACT, REDUNDANT, COMPRESSED}`. Declaration precedence over
instance defaults is preserved; `COMPRESSED` enters no payload branch —
no compressed-storage model, no formula/threshold change, no new
instance-default value. `DEFAULT`/absent/unknown declarations still
resolve through `instance.innodb_default_row_format` and still gap when
that field is empty. PostgreSQL gate and index-key-length decisions are
untouched.

## What is proven

- **Red state on the buggy base** (`red-green/red-run.log`): final R1
  test files dropped onto unmodified `32a325c3` — 39 `--- FAIL` lines
  showing all three failure modes (`declared COMPRESSED lost to ""`,
  `lost to "DYNAMIC"/"COMPACT"/"REDUNDANT"`, wrong-branch payload
  findings) while the real parser/extractor test and opposite-format
  controls still pass.
- **Green state** (`red-green/green-run.log`): the same suite at
  `700e90e4` — 70 `--- PASS`, 0 fail, both dialects, across
  offline/nil/empty/DYNAMIC/COMPACT/REDUNDANT instance shapes.
- **Internal identity assertions**:
  `resolveRowSizeFacts` returns `rowFormat == "COMPRESSED"`,
  `rowFormatKnown == true`; the over-limit finding carries
  `metadata.row_format="COMPRESSED"`, `estimated=65536`, `limit=65535`.
- **Real-entry integration**: `Parse` → `Extract` → `AuditSQL` with the
  frozen `t06-a15-row-size-isolated` profile (only
  `ddl.table.row_size.max_bytes.require`, blocker, required=true) and a
  controlled provider returning constructed instance facts — provider
  call ledger pinned (facts + absent-table snapshot), declared/provider
  inputs unmutated.
- **Contracted inputs** (`red-green/sql/`): `S_SMALL` (declaration-complete
  pass under every instance shape), `S_OVER` (single total-estimate
  blocker keeping the COMPRESSED identity), plus the in-test `S_BRANCH`
  (11 × utf8mb4 VARCHAR(192) = 8470 total / 8448 capped — never enters
  the COMPACT/REDUNDANT branch) and `S_NO_CHARSET` (only
  `instance.default_charset` in `required_facts`).
- **Opposite-format controls**: declared COMPACT/REDUNDANT still produce
  their 8126/8000 payload findings at 8448; DYNAMIC stays clean;
  `DEFAULT` + instance COMPACT still enters the COMPACT branch;
  `DEFAULT`/omitted + no instance still gap — the fix cannot pass by
  suppressing all format logic.

## Frozen Golden regression at this SHA

`make ddl-golden TASK=T06 ARTIFACT_DIR=/tmp/ds-t06-a15r1-golden` —
**496/496 cases, 11,772 assertions**, four frozen anchors
(mysql:5.7.44, mysql:8.0.46, mysql:8.4.10, pingcap/tidb:v8.5.0), at
head `700e90e4`. `validate --artifact` → `artifact valid`. The artifact,
cases, and generated policies are mirrored in this directory; the
runner binary stays at `/tmp/ds-t06-a15r1-golden/T06/bin/deltascope`
(sha256 in `identity/hashes.txt`). No frozen case result changed.

## Supplemental CLI runs (not Golden cases)

`red-green/runs/`: four real CLI audits on the `700e90e4` binary,
`--config on.yaml` (the committed A15 isolated profile), no provider,
`--fail-on blocker`, JSON output — original argv/stdout/stderr/rc kept:

| case | dialect | verdict | rc |
|------|---------|---------|----|
| S_SMALL | mysql | pass | 0 |
| S_SMALL | tidb | pass | 0 |
| S_OVER | mysql | reject (total estimate 65536 > 65535, `row_format=COMPRESSED`) | 1 |
| S_OVER | tidb | reject | 1 |

## Gates at `700e90e4` (`gates/`)

| gate | result |
|------|--------|
| `go test ... -run T06A15 -v` (domain + application) | rc=0 |
| `make ddl-golden-validator-test` | 728 checks, 0 failures |
| `python3 scripts/ddl_golden.py validate --artifact …` | `artifact valid` |
| `make ddl-golden TASK=T06` | 496/496, 11772 assertions |
| `GOFLAGS=-count=1 make test` | rc=0 |
| `pg-unit-test-gates`, `sql-corpus-gates`, `ddl-inventory-gate`, `ddl-coverage-catalog-test`, `docs-example-gates`, `decision-record-gate` | rc=0 |
| `go test ./pkg/deltascope ./internal/interfaces/http -run T06A15 -v` | rc=0 |
| index-key-length regression (`IndexKey\|indexKey\|KeyLength`) | rc=0 |
| gofmt / three-level-doc / golangci-lint (touched packages) | clean / OK / 0 issues |

## Boundaries kept

Instance defaults are trusted verbatim only when the declaration is
`DEFAULT`, absent, or unrecognized; `COMPRESSED` is never added to any
instance-default collection and no server config was read or fabricated.
Estimation formulas, 65535/8126/8000 thresholds, PG behavior, index
decisions, catalog classification, parser/spec/provider/state
machine/transport, and the 24 original A15 roles are unchanged. The
directory name `700e90e4` is the *tested code* SHA; the evidence itself
lives in a separate evidence-only commit that is its child (`a0819096`).
Neither is the bug report's reviewed SHA — self-verification is not
reviewer acceptance.

## Working tree honesty

`.mcp.json` carries an unrelated external modification and was never
staged. Untracked tool/recovery directories (`.agents/`, `.codex/`,
`.cursor/`, `.devin/`, `.grok/`, `.opencode/`, `.pi-subagents/`,
`.qoder/`, `.debug-journal.md`, `docs/quality/*.html`) belong to other
tooling and are untouched. Committed tree is clean apart from this
task's files; the whole working tree is not clean.
