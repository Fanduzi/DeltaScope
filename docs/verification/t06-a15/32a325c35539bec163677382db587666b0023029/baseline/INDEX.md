# T06-A15-BASELINE evidence index

Scope: `ddl.table.row_size.max_bytes.require` — instance-fact premise,
65535 threshold, and charset-source reconciliation. Evidence only; zero
repo modifications, commits, or pushes. No database was contacted; every
AuditSQL request ran against a constructed MetadataProvider (synthetic
input, never a real server observation — `version.source:"observed"` in
results is stamped by ParseObservedVersion on the *injected* fact string).

## Identity

- Evidence HEAD: `737175980afb38759097a9971cc54a922fea0983`
  (= `origin/workfree` at collection time; the A14 evidence commit)
- Branch: `milestone/mysql-tidb-ddl-completion`
- Binary: `./deltascope` — sha256
  `f8ce3864cf60cf100d2203499a83f633056fca98b8e8b7c3a7af24739d11b919`,
  built `CGO_ENABLED=0 go build ./cmd/deltascope` at the evidence HEAD
  (see `build-info.txt`, `go-version-m.txt`; `+dirty` in the module line
  reflects the external `.mcp.json` modification and untracked tool dirs —
  no build-moment snapshot was captured, so attribution is limited to
  "worktree contained external changes")
- `rules-list.json` — real catalog, 381 rules; the row_size entry ships
  `enabled=true, level=blocker` (params `required=true` per
  `defaults_ddl.go`; catalog output does not echo params)

## Layout

- `policies/` — `p-on.yaml` (row_size only, blocker, `required=true`),
  `p-not-required.yaml` (same, `required=false`), `p-off.yaml` (the full
  real catalog with everything disabled — same shape used for the A13/A14
  all-rules-disabled control)
- `sql/` — the five fixed inputs L_MINUS / L_EQUAL / L_PLUS /
  OTHER_ENGINE / COLUMN_OVERRIDE (one statement each, trailing semicolon)
- `runs/` — 8 offline CLI runs × {argv,stdout,stderr,rc} —
  `{mysql|tidb}-{L_MINUS,L_PLUS}-{p-on|p-not-required|p-off}`; stderr is
  empty in all 8
- `probe/` — Go module `github.com/Fanduzi/DeltaScope/probe-t06a15` with
  `replace =>` the checkout (module path under the repo prefix so internal
  packages resolve); `main.go` is the collector; `probe-output.txt` the
  raw capture (FIELD PROBE: 5 SQL × 2 dialects AST+spec fields; AUDITSQL
  PROBE: 14 requests × {result JSON, provider call ledger});
  `probe-t06a15` binary kept alongside for review (not committed anywhere)
- `probe/p-on.yaml` — the policy the probe passed as `ConfigPath`

## Result summary (full results in runs/ and probe/probe-output.txt)

8 offline CLI runs — no provider, `Metadata.Instance` never exists:

| run | verdict | coverage | findings | gaps | rule_summary |
|---|---|---|---|---|---|
| L_MINUS/L_PLUS × p-on × mysql/tidb | pass rc=0 | complete | 0 | 0 | `{loaded:1, applicable:1}` |
| L_PLUS × p-not-required | pass rc=0 | complete | 0 | 0 | `{loaded:1, applicable:0}` |
| L_PLUS × p-off | pass rc=0 | complete | 0 | 0 | key absent |

14 controlled AuditSQL requests (provider ledger: exactly
`LoadInstanceFacts` once + `LoadTableSnapshot(schema=golden table=t)`
once per request; no second reads):

| request | result |
|---|---|
| L_MINUS / L_EQUAL × F_FULL | pass, 0 findings (estimated 65534 / 65535 — the `>` boundary) |
| L_PLUS × F_FULL | reject: `estimated row size exceeds 65535 bytes (65536)` blocker, meta {table:t, charset:latin1, row_format:DYNAMIC, estimated:65536, limit:65535}, loc 1:1 |
| L_PLUS × F_EMPTY | **identical reject** (all inputs declared → instance fallbacks unreachable); `result.version` absent (zero facts → no observed identity) |
| L_PLUS × F_NIL | **pass, 0 findings** — silent skip; `{1,1}` + complete indistinguishable from an evaluated pass |
| OTHER_ENGINE × F_FULL | pass, 0 findings — declared MyISAM skips before estimation even with full facts |
| COLUMN_OVERRIDE × F_FULL | reject: estimated **65538** (16384×4+2 — column `utf8mb4` beat table `latin1`); `metadata.charset` still records the **table** charset `latin1` |

mysql/tidb identical for every request; both dialects produce the same
findings/verdicts/ledgers.
