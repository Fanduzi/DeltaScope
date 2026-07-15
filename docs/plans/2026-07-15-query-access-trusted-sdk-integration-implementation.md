# Implementation Plan: Query Access Trusted PostgreSQL SDK Integration

Date: 2026-07-15
Status: Proposed follow-on plan
Depends on: pure-read branch at `ca64b49` and the accompanying design
Rule: do not start implementation until the design receives human approval

## Global Safety Rules

- Keep the pure-read decision record `Proposed` until every implementation and
  E2E acceptance criterion is satisfied.
- Do not rebase, merge, push, tag, release, or touch `stash@{0}` in this work.
- Do not change the default SDK, CLI, HTTP, MCP, MySQL, or TiDB path.
- Do not expose a resolver, session, trust policy, catalog fact, OID, manifest,
  SQL, literal, credential, DSN, or `severity` through public JSON.
- Do not widen the PG17 manifest or replace exact identity checks with syntax,
  name, namespace, or volatility-class trust.
- Before each production-symbol edit, run GitNexus impact analysis. Stop for a
  HIGH or CRITICAL result until direct callers and processes have a test plan.
  Run `gitnexus_detect_changes` before each commit.

## Task 1: Clarify Existing Contracts

Scope:

- Correct `PathEpoch` documentation: it is a stable non-zero marker for one
  capture pair; database, role, version, and session are compared field-wise,
  and search path is compared separately.
- Link this design and plan from the pure-read decision record.
- Record the current proof gateway as internal/test-verified, not publicly
  reachable through the default SDK, CLI, HTTP, or MCP paths.
- Correct every decision-record statement that calls the current trusted path
  public before this plan can change status to `Accepted`.

Acceptance:

- No production behavior changes.
- `make decision-record-gate` and `git diff --check` pass.

Commit: `docs: plan trusted query access SDK integration`

## Task 2: Opaque Caller-Owned Session API

Scope:

- Add a public `PostgreSQLQueryAccessSession` wrapper and
  `NewPostgreSQLQueryAccessSessionFromConn(*sql.Conn)`.
- Keep its pinned-session field unexported and provide no JSON-marshalable
  fields.
- The wrapper never closes its caller-owned connection; it must document and
  test this ownership rule.
- Define bounded errors for nil/closed session and non-PostgreSQL dialect.
- Do not add a `*sql.DB` constructor or a session field to
  `QueryAccessRequest`.

Tests:

- Nil connection/session and wrong dialect reject without driver text.
- A caller can query and then close its connection after successful analysis.
- An already-closed caller connection fails boundedly and never panics.
- Reflection/JSON tests prove the wrapper cannot appear in public result JSON.
- Existing default SDK API remains source- and behavior-compatible.

## Task 3: Same-Connection Metadata and Proof Adapters

Scope:

- Add a `*sql.Conn`-backed query-access relation/column metadata resolver. It
  must not contain a `*sql.DB` field or use pooled lookup helpers.
- Wire this resolver, the atomic effect resolver, and the PG17 trust policy
  behind a private SDK helper that creates `NewTrustedService`.
- Preserve the single application proof gateway; do not reproduce its checks in
  `pkg/deltascope`.
- Reject a non-nil `QueryAccessRequest.SchemaResolver` on this trusted path;
  never ignore it silently and never combine its facts with session-bound facts.

Tests:

- A recording integration adapter captures `pg_backend_pid()` for relation
  metadata, type OID, and identity lookups and proves all equal the supplied
  caller connection's backend PID.
- Schema-qualified `count(*)` and base-column comparison reach the application
  service through the helper.
- No default service path starts using trusted dependencies.
- A caller-supplied schema resolver is rejected before analysis and cannot
  influence requirements or admission.

## Task 4: Public Trusted SDK Function

Scope:

- Add `AnalyzePostgreSQLQueryAccessWithSession` as designed.
- Validate dialect/session before internal construction and map expected
  catalog/proof failures to bounded `indeterminate` results.
- Add GoDoc spelling out static-only semantics and caller execution ownership.

Tests:

- Strict and projection-only preserve classification and differ only in
  requirements/warnings.
- Schema-qualified PG17 `count(*)` and an exact supported comparison become
  `admissible` only with complete metadata/proof.
- Unqualified relations, views, wildcard failure, literals, parameters, casts,
  UDFs, non-manifest effects, incomplete metadata, and resolver failures remain
  `indeterminate`.
- JSON has no proof internals, SQL/literals, credentials, or `severity`.
- Success and bounded-error tests use sentinel OID, session-binding, manifest,
  DSN, password, catalog-SQL, and SQL-literal values and assert none appear in
  JSON or public error text.

## Task 5: PG17 Docker E2E

Scope:

- Extend the PostgreSQL integration suite to call the public SDK function with a
  caller-owned `*sql.Conn` from the PG17 compose database.
- Verify the test owns connection close; DeltaScope must not close it.

Required cases:

- `SELECT count(*) FROM app.users` is `read_only` + `admissible`.
- A supported schema-qualified column-column comparison is admissible.
- Unqualified relation, view, `TABLESAMPLE`, and unsupported/non-manifest
  effect remain indeterminate.
- Changed `search_path` or role/context must make promotion fail closed.
- Call only `NewPostgreSQLQueryAccessSessionFromConn` and
  `AnalyzePostgreSQLQueryAccessWithSession`; do not construct internal services
  or resolvers in the public SDK E2E.
- Assert metadata, type, and identity lookup backend PIDs all equal the
  caller-owned connection's PID.
- Assert the caller can still query and close that connection afterward.

Commands:

```bash
docker compose -f docker/pg-e2e-compose.yaml up -d --wait
make pg-trusted-sdk-e2e-gate
```

`pg-trusted-sdk-e2e-gate` must start PG17 with `docker compose ... up -d --wait`
and fail when its required public SDK test is skipped, absent, or fails. Existing
internal integration tests do not satisfy this acceptance target. If Docker is
unavailable or the public E2E fails, stop. Do not call the decision accepted or
characterize it as pre-existing.

## Task 6: Cross-Surface Non-Regression and Docs

Scope:

- Add exact default-surface non-regression tests for
  `SELECT count(*) FROM app.users` and
  `SELECT u.id FROM app.users u JOIN app.orders o ON u.id = o.user_id`:
  default SDK, CLI, and HTTP must remain `indeterminate` and must not invoke a
  trusted resolver. MCP remains four tools with no Query Access tool.
- Update `pkg/deltascope/README.md` and EN/ZH reference docs: trusted PG17
  promotion is SDK-only and requires a caller-owned connection.
- Do not change release surfaces unless the milestone later becomes a release
  candidate.

## Task 7: Final Independent Audit and Decision

Required independent review questions:

1. Can any public request bypass the application proof gateway or inject trust
   facts/context?
2. Can metadata, identity, and type facts come from different connections?
3. Can an unqualified relation, view, unsupported AST, or incomplete strict
   requirement become admissible?
4. Does public JSON leak internal data?
5. Does the SDK claim to authorize or guarantee later execution?

Required gates:

```bash
go test ./... -count=1
go test -tags postgresql ./... -count=1
go test -race ./internal/domain/queryaccess/... ./internal/application/queryaccess/... ./pkg/deltascope/... -count=1
go build ./...
go build -tags postgresql ./...
go vet ./...
go vet -tags postgresql ./...
golangci-lint run ./...
make query-access-corpus-gates
make pg-unit-test-gates
make decision-record-gate
make release-gofmt-gate
npm test --prefix packages/deltascope-mcp
git diff --check
go mod tidy && git diff --exit-code go.mod go.sum
```

Only after all gates, public SDK E2E, and independent audit pass may the
decision record first be updated to describe the delivered SDK-only public
boundary, then change from `Proposed` to `Accepted` in the same acceptance
commit.

## Integration Note

The pure-read branch forked before local `main@a1a68da`; it cannot be
fast-forward merged. Do not resolve that here. After the final audit, choose a
separate, reviewed rebase or merge strategy and re-run the full gate set.
