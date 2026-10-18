# T06-A15: row-size field-level fact resolution fix

Code under test: `32a325c35539bec163677382db587666b0023029`
(`fix(ddl): T06-A15 row-size field-level fact resolution (missing_instance_fact gaps)`)

This is a **fix slice** (not proof-only): production code changed.
`ddl.table.row_size.max_bytes.require` previously gated the whole
estimate on `Metadata.Instance != nil` — an object-presence check. The
slice replaces it with field-level resolution: the rule computes from
declared inputs whenever they suffice, consults
`instance.default_charset` / `instance.innodb_default_row_format` only
as per-field fallbacks, and reports a `missing_instance_fact` evidence
gap for genuinely missing required fields instead of silently passing.
MySQL/TiDB only; PostgreSQL keeps the legacy instance-object gate.
`indexKeyLengthRule` and the shared estimate formulas are untouched.

The frozen T06 golden contract grows from 472 to **496 cases** — the
A1–A14 contract is preserved (every pre-A15 case contract-identical;
the file keeps its indent/UTF-8/key-order conventions), all 496 records
executed fresh at `head_sha` `32a325c3`, with 24 new CLI records under
two new isolated profiles plus the reused `all-rules-disabled` control:

- `t06-a15-row-size-isolated` — only the row-size rule at `blocker`,
  `required=true`
- `t06-a15-row-size-not-required-isolated` — same rule at `blocker`,
  `required=false`

## What is proven

**Product (real parser + AuditSQL + CLI, both dialects):** 12 roles ×
2 dialects = 24 offline CLI runs over ten fixed inputs (plus a
`ROW_FORMAT=DEFAULT` supplementary input):

| Role | Input × profile | Expected/actual |
| --- | --- | --- |
| `below` | `VARCHAR(65532)` latin1 DYNAMIC × on | pass rc=0, complete, 0 findings/gaps (estimate 65534) |
| `equal` | `VARCHAR(65533)` × on | pass rc=0, complete (estimate 65535 — strict `>` only) |
| `above` | `VARCHAR(65534)` × on | reject rc=1: 1 blocker, `estimated=65536, charset=latin1, row_format=DYNAMIC, limit=65535` |
| `column-override` | `VARCHAR(16384) CHARACTER SET utf8mb4`, table latin1 × on | reject: `estimated=65538`, `metadata.charset=latin1` keeps the table-level value |
| `column-only-charset` | same column, no table charset × on | reject: `estimated=65538`, `metadata.charset=""` — no gap for an unused table charset |
| `non-innodb-skip` | `ENGINE=MyISAM` × on | pass rc=0, complete, 0/0 — scoped out before any fact lookup |
| `missing-charset` | `ENGINE=InnoDB ROW_FORMAT=DYNAMIC`, no charset anywhere × on | review, unverified, 0 findings, gap `[instance.default_charset]` |
| `missing-format-safe` | `VARCHAR(65532)` latin1, no row_format × on | review, unverified, 0 findings, gap `[instance.innodb_default_row_format]` |
| `missing-format-over` | `VARCHAR(65534)` latin1, no row_format × on | reject, unverified: total blocker (`row_format=""`) **plus** the row-format gap coexist |
| `missing-both` | `ENGINE=InnoDB` only × on | review, unverified, gap `[instance.default_charset, instance.innodb_default_row_format]` in fixed order |
| `not-required` | `L_PLUS` × not-required | pass rc=0, complete, `rule_summary` loaded=1 applicable=0 |
| `disabled` | `L_PLUS` × all-rules-disabled | pass rc=0, `rule_summary` key absent |

Supplementary roles (offline CLI, `red-green/runs/`): `ROW_FORMAT=DEFAULT`
is unresolved — reject + `innodb_default_row_format` gap, never a
fabricated DYNAMIC; gap-only `NO_CHARSET` under `--fail-on`
blocker/none exits 0, under warning/notice exits 1 (existing exit
contract unchanged).

**Red/green (real AuditSQL, compilable at old baseline):**
`red-green/red-run-*.log` show the new tests failing at `73717598` —
offline/F_NIL returned `pass` where the contract requires
reject/review, and the rule lacked `EvidenceReporter`
(`panic: interface conversion … missing method EvidenceGaps`).
`red-green/green-run.log` shows the same inputs passing at the fix.

**Pointer-shape matrix (domain + application tests):** nil, empty, and
full instance objects produce identical findings/gaps on
declaration-complete inputs; partial objects contribute only the fields
they carry; provider sentinel errors and context cancellation keep the
error channel — never become `missing_instance_fact`.

## Baseline reuse (old-HEAD evidence, not re-executed)

`baseline/` carries the publishable T06-A15-BASELINE originals
collected on evidence HEAD `73717598` (8 offline CLI
argv/stdout/stderr/rc records, three policies, rules catalog, the
14-request controlled-provider AuditSQL probe with call ledger, fixed
SQL files). `baseline/binary-sha256.txt` and
`baseline/probe/probe-binary-sha256.txt` record the baseline binary
hashes; the binaries stay under `/tmp/ds-t06-a15-baseline/` outside the
repo. The baseline established that the object-presence gate silently
swallowed evaluation; it was not re-run here.

## Gates (argv/stdout/stderr recorded under `gates/`, all rc=0 unless noted)

| Command | Result |
| --- | --- |
| `go test -count=1 ./internal/domain/rule/ddl ./internal/domain/rule/catalog ./internal/application/audit -run T06A15 -v` | PASS — resolver matrix, catalog classification, audit roles |
| `make ddl-golden-validator-test` | PASS — 728 contract checks, 0 failures (incl. 17 A15 mutations) |
| `make ddl-golden TASK=T06 ARTIFACT_DIR=/tmp/ds-t06-a15-golden` | PASS — 496/496, 11772 assertions, four anchors at head 32a325c3 |
| `python3 scripts/ddl_golden.py validate --artifact …/T06/artifact.json` | PASS — `artifact valid`, binary check on |
| `GOFLAGS=-count=1 make test` | PASS — full repository suite |
| `make pg-unit-test-gates sql-corpus-gates ddl-inventory-gate ddl-coverage-catalog-test docs-example-gates decision-record-gate` | all PASS (06-misc-gates.log) |
| `go test -run T06A15 ./pkg/deltascope ./internal/interfaces/http -v` | PASS — SDK/HTTP seam: reject/review/200 projections |
| `go test ./internal/domain/rule/ddl -run 'IndexKey\|indexKey\|KeyLength' -v` | PASS — index-length matrix untouched (35 checks) |
| `gofmt -l`, three-level-doc check | clean / OK |
| `golangci-lint run` on touched packages | 2 pre-existing baseline goimports issues only (verified identical at `73717598`; zero-diff files) |

## Layout

- `artifact.json`, `cases/` — the full 496-case run at `32a325c3`
- `T06-manifest.json`, `anchors.json` — the frozen contract and pinned
  anchor identities the run executed against
- `policies/` — the 43 generated policy files the run actually used
  (incl. the two `t06-a15-*-isolated` profiles), sha256-bound in the
  artifact
- `gates/` — gate argv/stdout/stderr/rc records
- `red-green/` — old-HEAD red logs, post-fix green log, and the
  post-fix offline CLI matrix (`runs/`: argv/stdout/stderr/rc for 28
  runs across both dialects incl. the fail-on ladder and the
  `ROW_FORMAT=DEFAULT` supplement), with its three policies and eleven
  SQL inputs
- `baseline/` — old-HEAD baseline originals (see above)
- `binary-sha256.txt`, `binary-version.txt`, `build-identity.txt`,
  `go-version.txt`, `head-sha.txt` — source/build identity; the proof
  binaries stay at `/tmp/ds-t06-a15-golden/T06/bin/deltascope` and
  `/tmp/ds-t06-a15-work/deltascope` outside the repo
- `identity/hashes.txt` — artifact/manifest/binary hashes
- `task-range-diff.txt` — `73717598..32a325c3` diffstat
- `worktree-status.txt` — tree state at evidence time (tracked-clean
  except external `.mcp.json`; untracked tool dirs and restored run
  dirs excluded by explicit path staging)

Self-check is not an independent reviewer pass.
