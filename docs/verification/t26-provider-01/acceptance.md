# T26-PROVIDER-01 — acceptance record (SCOPED ACCEPTED, offline provider round)

Status: **SCOPED ACCEPTED** by independent reviewer (work order
"T26-PROVIDER-01 — 最后一处 provider 名字保真修正与出口证据闭环",
ruling issued after the R3 correction receipt). This document archives
the final ruling on the T26 / #105 MySQL SERVER/SRS metadata-provider
round. The tested snapshot is frozen; this file is archival only and
does not describe a re-execution. T26/#105 overall remains **OPEN** —
WKT, lifecycle, post-state, SQL-mode decode, and native verification
stay out of scope.

## 1. Accepted product identity

- commit: `7c2c1f23d41960ef5f8d5efeb51c7fd166eb9a8e`
- tree: `cca8af6914cd1a5754d0724c5ac45cb9c1e8bdaa`
- parent: `f1ddab3d5330747031970396fc545adc209fb2e4`
- Continuous chain (unchanged, no history rewrite):

  `1e130f1d` (T26-R6 archive) → `81051e45` (provider round impl) →
  `4cc16ff6` (issued-query error propagation) → `471d3d66` (public
  error-boundary evidence + comment sync) → `f1ddab3d` (cache identity
  + bounded resolver error) → `7c2c1f23` (verbatim decoded name in
  zero-row server state).

- If this archive is committed: the tested/accepted object stays
  `7c2c1f23`; the documentation commit is registered separately with
  its own commit/tree/parent (see §8).

## 2. Accepted capability scope (accurate registration)

Provider-level fact capabilities (5):

1. `mysql_server` existence — bounded `mysql.servers` read, issued only
   when `name_form=identifier` qualifies the request.
2. `mysql_spatial_reference_system` existence by provable numeric SRID.
3. SRS `SRS_NAME` conflict witness (request-qualified).
4. SRS `ORGANIZATION` + `ORGANIZATION_COORDSYS_ID` pair conflict witness
   (request-qualified).
5. SRS column dependency witnesses via `ST_GEOMETRY_COLUMNS.SRS_ID`
   (privilege-filtered; positive rows are witness sets, zero rows stay
   unknown — no approved completeness contract).

Statement-path consumption (3): SERVER existence, SRS existence, SRS
dependencies — bound through `bindMySQLServer`/`bindMySQLSRS` during
enrichment onto `Metadata.Server` / `Metadata.SpatialReference`.

Provider-only (not statement-path active): SRS NAME conflict and SRS
ORG pair conflict reads — the application binder does not pass those
qualifiers, so these stay unused by the current statement path.

## 3. Closed issues (as ruled)

| # | Issue | Fix commit | Evidence |
|---|---|---|---|
| 1 | Issued-query errors (MySQL 1054/1109/1146) downgraded to `catalog_unproven` on unproven versions | `4cc16ff6` | downgrade paths removed; propagation tests red→green |
| 2 | `mysql_server` cache key folded/trimmed decoded name (` s1`/`s1` aliased) | `f1ddab3d` | `newObjectLookupKey` keeps verbatim name for `mysql_server` |
| 3 | Qualifier set serialization ambiguous (`name=value;` aliased values carrying `=`/`;`) | `f1ddab3d` | length-prefixed `%d:%s=%d:%s;` encoding, sorted keys |
| 4 | `bindMySQLServer` fallback + `loadMySQLServerObject` trimmed the decoded name | `f1ddab3d` / `7c2c1f23` | `decl.Name` / `request.Name` verbatim in unvouched and zero-row states |
| 5 | Provider/driver/authored payload could reach public error text | `f1ddab3d` | `metadataObjectReadError`: fixed public text `"metadata object read failed"`, `Unwrap()` preserves cause; both seams wrapped (`resolveObjectSnapshot`, `objectResolverFor` fallback) |
| 6 | application/SDK/HTTP boundary regressions | `f1ddab3d` | sentinel isolation tests at all three surfaces; `errors.Is` chain kept |
| 7 | CLI/MCP exit source chain | — (source evidence only) | `mcp_exit.go.txt` in this archive |

## 4. This round's provider zero-row RED/GREEN

- RED (labeled: new assertion on `f1ddab3d` product code — verbatim
  console transcript, `red_zero_row.log`):

  ```
  provider_t26_test.go:114: server state name = "ghost", want verbatim decoded " ghost" — zero rows never normalize identity
  ```

- GREEN (`-count=1`, on `7c2c1f23`; `green_provider_t26.log`,
  `green_packages.log`): all 18 T26 provider tests PASS;
  `internal/infrastructure/metadata/mysql` and
  `internal/application/audit` packages PASS.

## 5. Run-record attribution (evidence form per record)

| Record | Candidate | Scope | Form |
|---|---|---|---|
| Full-repo 39 pkg / 0 FAIL | `4cc16ff6` | whole repo | raw log (previously delivered) |
| audit/SDK/adapter boundary runs | `f1ddab3d` | affected packages | raw `-count=1` output (previously delivered) |
| R2 five-cell red | `471d3d66` product + new assertions | audit/pkg/http `-run T26` | verbatim recapture, `red_r2_recapture.log` — labeled, not the original run |
| Provider zero-row red | `f1ddab3d` product + new assertion | single test | verbatim transcript, `red_zero_row.log` |
| Provider T26 + affected packages green | `7c2c1f23` | provider pkg + audit pkg | raw `-count=1` logs, this archive |
| MCP/CLI exit chain | `7c2c1f23` source | adapters | source derivation, `mcp_exit.go.txt` |

The original first RED run on `81051e45` was not saved to disk — that
absence registration stands; `red_r2_recapture.log` keeps its recapture
identity. The duplicated R2 receipt delivery is registered as a
transport incident, not a new candidate or new verification.

## 6. Record corrections applied (per reviewer §三)

1. `errors.As` is evaluated over the whole cause chain — the earlier
   note's phrasing "wrapper is not that type" was wrong as written;
   `mapAuditToolError` reaching `bad_request` stands only for
   `metadataObjectReadError`'s actual membership set (no
   `*auditmeta.Error` anywhere in the chain, no sentinel `errors.Is`
   hit, no keyword substring in the fixed text).
2. `provider_t26_test.go` top-level `TestT26*` count is **18** (source
   enumeration, §9); the earlier "16" figure is withdrawn. Excerpts
   and bracketed tallies are labeled excerpt/summary; raw logs keep
   original form; explanatory text is separate.
3. `request.Name` as the issued query argument is registered as
   **source evidence** (parameter binding at
   `provider_server_srs.go:55-59`); the runtime assertion added this
   round is registered as zero-row **return-state** name fidelity only.
4. Corrected historical REDs registered by actual first `t.Fatal`
   output: qualifier regression is a key-level comparison
   (`distinct qualifier sets share one cache identity`), not a call
   count; SERVER cache red's first fatal is the resolver-call count
   (`resolver calls = [...], want two distinct identities`) — the name
   assertions never executed in that run; application boundary red is
   "new bounded assertion fails on old product text"
   (`public error text echoes provider payload: t26-provider-marker-7f3a`).

## 7. Historical attribution preserved

`4cc16ff6`, `f1ddab3d`, and `7c2c1f23` records stay bound to their own
commits. No historical RED was rewritten or relabeled.

## 8. Archive commit identity

This directory is documentation-only; the tested product object stays
`7c2c1f23`. The archive commit's own commit/tree/parent is registered
in the closing review receipt for this round, not inside this file
(the hash cannot pre-exist inside the commit that creates it).

## 9. Archive contents

See `README.md` in this directory for the file index and per-file
description.
