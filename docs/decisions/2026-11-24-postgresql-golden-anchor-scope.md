# Decision: postgresql golden anchor uses manifest scopes and trust auth (issue #115 T34)

Date: 2026-11-24
Status: Accepted
Related milestone/version: issue #115 T34 (#117) under the PostgreSQL DDL acceptance milestone
Related commits: T34 runner/compose/baseline/manifest commit (this change set)
Related tests: `scripts/test_ddl_golden.py` (pg-scope contract rows), `testdata/ddl-golden/T34.json` run artifact (5 cases / 45 assertions)
Related docs: `testdata/ddl-golden/anchors-baseline.json`, `docker/ddl-golden-compose.yaml`

## Context

The golden runner's baseline was a flat contract: every manifest had to
declare the four mysql/tidb anchors and execute mysql+tidb CLI dialect
cases, because the file was written for the single-product-line
milestone (#79). Adding PostgreSQL forced three decisions:

- PG manifests must not inherit the mysql/tidb anchor denominator — a
  postgresql task proving all four mysql anchors too would blur the
  milestone's denominator and double every PG run's lifecycle cost.
- The baseline's shrink-protection must extend to scoped sets, not just
  the flat set, or a scoped anchor could silently degrade.
- PG fixture execution needs a client contract: psql flags, error
  classification (sqlstate, not mysql errno), and a credential model.

## Decision

- `anchors-baseline.json` gains `manifest_scopes.postgresql`:
  `required_anchors: {postgres17}` + `required_cli_dialects:
  ["postgresql"]`. A manifest opts in by declaring
  `anchor_scope: "postgresql"`; everything else keeps the legacy flat
  set. An unknown scope name is a hard failure in
  `baseline_manifest_failures` / `baseline_executed_failures`, never a
  silent fallback — a typo'd scope must not widen or narrow the
  denominator.
- `prove_anchor_identity` merges the scoped required anchors into its
  locked-digest lookup, so postgres17 pins with the same force as the
  flat anchors: `postgres@sha256:20064937…` (index digest) +
  `linux/arm64` platform.
- `docker/ddl-golden-compose.yaml` declares `postgres17` as
  `postgres:17.7` (tag pull so the manifest tag resolves locally) with
  `POSTGRES_HOST_AUTH_METHOD=trust`. Trust auth mirrors the tidb85
  root/empty-password model: the disposable fixture container carries no
  credential, so nothing credential-shaped can leak into serialized
  evidence. `manifest_credential_failures` hard-fails
  `client_option_file`/`exec_password_env` on postgresql anchors —
  a half-wired secret channel must fail loud.
- `psql` fixture client contract: `exec_client` carries
  `-X -v ON_ERROR_STOP=1 -v VERBOSITY=verbose`; anchors declare
  `exec_flag`/ `exec_silent_flag` (`-c` / `-tAXc`) and
  `error_class_pattern` (`ERROR:\s+(\w+):` extracting the sqlstate, e.g.
  `42601`). Runtime and validator extract the error class through the
  same anchor-declared pattern so recorded and re-derived values cannot
  diverge.
- `mysql_exec`/`mysql_exec_argv` renamed `db_exec`/`db_exec_argv` — the
  function already dispatched on the anchor's declared client; the name
  now says so.
- Deployment evidence probes keyed by product: postgresql uses
  `pg_control_system().system_identifier` as the immutable cluster
  identity (its `@@server_uuid` equivalent) and `version()`.
- `connect.database` maps to `--database` in CLI argv (PostgreSQL keeps
  database and schema distinct); the argv-binding validator mirrors the
  same derivation.
- `build_cli` builds with `CGO_ENABLED=1 -tags postgresql`
  unconditionally: `pg_query_go` links libpg_query through cgo, and
  TASK=all interleaves all dialects against one binary.
- `read_instance_fact` supports `pg.<guc>` facts via `SHOW <guc>` with a
  strict identifier charset.
- `required_all_tasks` gains `T34`; the smoke manifest proves the full
  contract (db DDL steps + sqlstate negative + feature-rule isolation +
  live metadata audit over TCP at `127.0.0.1:25432`).

## Deferred

- `platform` locks remain host-observed for the pinned review host
  (linux/arm64); same visible-blocker behavior on other arch machines.
- postgres17 is the only PG anchor. Multi-version PG acceptance (16/18)
  is out of scope for this milestone — the parser anchors on
  libpg_query 17.
- Password-bearing PG connect paths (`.pgpass`, `--password-env` on a
  credentialed PG) are intentionally unsupported by the golden
  contract — trust auth is the fixture model; adding one later is a
  baseline + credential-gate diff.
- Per-statement transaction semantics: `psql -c` wraps multi-statement
  strings in one implicit transaction; manifests keep `ddl_steps` and
  `execute` steps single-statement or use `via: stdin` (batch mode) so
  non-transactional DDL (e.g. CREATE DATABASE, CREATE INDEX
  CONCURRENTLY) keeps its honest rc.

## Verification evidence

- `python3 scripts/ddl_golden.py run --task T34 --artifact-dir <dir>`:
  5 cases, 45 assertions, PASS; `validate` → artifact valid.
- `python3 scripts/test_ddl_golden.py`: 896 contract cases, 0 failures
  (6 new pg-scope rows).
- T02 mysql regression: 10 cases, 77 assertions, PASS — the scoped
  machinery is inert for non-scoped manifests.
- Inventory contract gate unchanged: `go test
  ./internal/application/audit -run TestDDLInventoryContract` passes.
