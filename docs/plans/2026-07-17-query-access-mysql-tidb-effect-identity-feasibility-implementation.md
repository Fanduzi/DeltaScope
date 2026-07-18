# Implementation Plan: Query Access MySQL/TiDB Builtin Effect Identity Feasibility

Date: 2026-07-17
Status: Proposed
Branch: `query-access-mysql-tidb-effect-identity-feasibility`
Baseline: v0.400.0 (`e01d7e8`)
Decision: `docs/decisions/2026-07-17-query-access-mysql-tidb-effect-identity-feasibility.md`

## Global Rules

- Work only on this milestone branch/worktree. Make one focused commit per
  task. Do not rebase, merge, push, tag, release, publish, trigger a workflow,
  or alter a published version.
- Read the decision, specification, design, existing pure-effect decision, and
  current code before each task. A task report is not a substitute for source
  review.
- Before editing a production symbol, run GitNexus upstream impact analysis.
  Stop and report HIGH/CRITICAL blast radius before editing. Before each
  commit, run GitNexus detect-changes; if the installed CLI lacks it, record
  that limitation and review the exact staged diff plus GitNexus status.
- Use a base characterization or adversarial reproduction for every behavior
  claim. A green new test alone does not prove that the prior boundary existed.
- No candidate name, parser spelling, `DETERMINISTIC`, schema name, or vendor
  prose may become a trust shortcut. No resolver may return `Trusted`.
- A GO/DEFER/KILL disposition is per dialect. MySQL evidence cannot promote
  TiDB, and TiDB evidence cannot promote MySQL.
- A failed proof leaves the public result fail-closed with bounded reasons. Do
  not add raw SQL, credentials, driver errors, identity facts, session data, or
  `severity` to public results/errors.
- A kill result is complete work for that dialect. Do not force production code
  merely to make the milestone appear symmetric.

## Task 1: Freeze the Current Contract

Add characterization tests and corpus fixtures for MySQL and TiDB, in both
`strict` and `projection_only` modes:

- `COUNT(*)`, `COUNT(column)`, direct-column `SUM/AVG/MIN/MAX`, grouped
  aggregate, and `ROW_NUMBER/RANK/DENSE_RANK` with direct partition/order;
- stored/UDF-looking names, qualified calls, parameter/literal/NULL/cast,
  `DISTINCT`, aggregate-local order, unsupported modifiers, explicit frames,
  nested expression operands, wildcard, ambiguity, views, CTE/derived input,
  locking read, and multi-statement cases;
- no-leak assertions for results, errors, corpus values, and reason ordering.

Expected baseline: every MySQL/TiDB function-bearing candidate remains
`indeterminate` with `unknown_function_effect`; existing function-free
admissible MySQL/TiDB SELECT behavior remains unchanged. Record exact strict
requirements and projection-only warnings for every case.

Gates: focused parser/application/domain/SDK tests, `make
query-access-corpus-gates`, `go test ./... -count=1`, and `git diff --check`.

## Task 2: Complete Dependency and AST Coverage

Audit `internal/infrastructure/parser/tidb/query_access.go`, the application
extractor, and existing corpus against every expression-bearing location used
by the candidate matrix. Test direct and nested effects in projection, JOIN,
WHERE, GROUP BY, HAVING, ORDER BY, LIMIT/OFFSET, VALUES, set operations, CTEs,
subqueries, aggregate arguments/modifiers, and window definitions.

Fix only an observed completeness gap. Unhandled grammar must produce an
existing or newly bounded fail-closed reason, never silent `read_only`.
Verify strict requirements include all true dependencies; projection-only must
retain its inference-risk behavior. Do not implement identity resolution or
promotion in this task.

Required negative regression: a hidden function in a derived/subquery/order
position cannot be admitted on either dialect.

## Task 3: MySQL 8.4 Identity Feasibility Ledger

Start `docker compose -f docker/cli-e2e-compose.yaml up -d --wait` and execute
real MySQL 8.4 probes on a caller-owned `*sql.Conn`. Add executable tests where
possible under `internal/infrastructure/metadata/mysql/`; test code must not
become promotion code.

The probes MUST be live Docker integration tests (build tag `integration`),
NOT tautological static struct-literal self-assertions. The tests open real
`*sql.Conn` connections, assert server-returned evidence, and FAIL if the
Docker server returns materially different evidence. They do not self-assert
hardcoded struct literals.

Investigate and record:

1. Actual server version/build, current database, SQL mode, and all discovered
   context settings that affect resolution.
2. Normal and qualified calls for each candidate.
3. Stored function creation/use, builtin-like naming attempts, `DETERMINISTIC`
   declarations, UDF/plugin metadata where accessible, and ambiguity/shadowing
   outcomes. A privilege or syntax failure is evidence, not proof.
4. Whether any server facility returns a unique resolved builtin identity and
   effective type facts rather than echoing the SQL name.
5. Whether information-schema relation metadata, column types, identity, and
   initial/final context can be read on one connection with a defensible
   consistent-read boundary.

KILL predicate (corrected): KILL holds when live probes establish that the
ONLY available identity root is a forbidden name-based model (name, parser
token, `DETERMINISTIC`, schema, or vendor promise) OR required
identity/dependency/context facts cannot be bound. `DETERMINISTIC`, rejected
`CREATE FUNCTION count`, schema qualification, parser spelling, and server
documentation are supporting negative evidence only — never a KILL necessary
condition and never an identity root. Do not claim KILL merely because
selected catalog tables lack OIDs.

Write the MySQL ledger in the decision/design. Classify GO, DEFER, or KILL
against the explicit kill criteria. If the result is not GO, commit the
evidence and stop MySQL promotion tasks.

## Task 4: TiDB 8.5 Identity Feasibility Ledger

Repeat Task 3 against the TiDB 8.5 Docker service. Do not copy the MySQL
disposition. Record TiDB-specific server/version/compatibility facts,
function-resolution behavior, stored/UDF/plugin boundaries, and any identity
or type facts. The probes must be independent live Docker integration tests
that assert server-returned evidence.

Add the TiDB ledger and independent GO/DEFER/KILL result. If it is not GO,
commit evidence and stop TiDB promotion tasks.

Real E2E is required for Tasks 3 and 4. A Docker startup failure may block the
task only after the documented compose command was attempted and its output is
captured. Never label a skipped probe as proof.

## Task 5: Conditional Session and Metadata Foundation

Run only for a dialect with a GO disposition. Before editing, ask Oracle for a
read-only security review of the proposed connection/context model and ask
Momus to review the task plan's executable QA scenarios.

Implement a dialect-specific opaque, caller-owned connection session only if
live proof is required. It must:

- accept a non-nil live `*sql.Conn` and bounded `context.Context`;
- preserve caller ownership and never close the connection;
- create relation metadata, type, identity, and context adapters from that one
  connection, with no `*sql.DB` field or fallback;
- use a researched consistent-read boundary and validate initial/final context;
- reject external resolvers and mismatched dialects; and
- keep build-tag public symbol parity and bounded no-leak errors.

Tests must prove caller post-analysis usability/closure, already-closed
connection handling, same backend/session evidence where the server exposes
it, no `*sql.DB` fallback, context drift fail-close, and public JSON/error
privacy.

## Task 6: Candidate Facts, Binding, and Proof Gateway

Run only for each GO dialect. Extend internal candidates and the application
proof gateway with the exact fact tuple established by its ledger. Reuse the
single final admission recomputation; do not add a dialect-specific shortcut.

Validate independently for every candidate:

- candidate ordinal closure and duplicate/missing facts;
- bounded kind/arity/modifier eligibility;
- direct physical operand provenance and type binding;
- initial/final context compatibility and fact pins;
- complete strict requirements; and
- full application-owned manifest tuple membership.

Add adversarial tests for swapped facts, same-name/different-identity facts,
cross-connection/context drift, stale server version, partial batches,
non-manifest facts, resolver errors, coercion, and no-leak output. One proven
candidate must not clear another candidate's reason.

## Task 7: Per-Dialect Manifest and Opt-in SDK E2E

Run only for each GO dialect. Add the smallest audited manifest subset and no
more than the exact direct-column/count/ranking shapes proven by the ledger.
For each entry document semantic audit, server identity tuple, version range,
negative near-misses, and requirement completeness.

Docker E2E must call only the new public caller-owned session API. It must
prove positive `read_only + admissible` cases and exact physical requirements,
then prove every negative boundary remains indeterminate. Default SDK, CLI,
HTTP, and MCP must remain unchanged and fail-closed/no-tool.

If no dialect reached GO, this task is intentionally skipped and the final
contract states that all MySQL/TiDB function-bearing queries remain deferred.

## Task 8: Corpus, Documentation, and Decision Evidence

For every actually promoted dialect, add corpus positive/negative fixtures,
strict versus projection-only assertions, deterministic bounded reason checks,
and SDK no-leak tests. Do not change corpus expectations for a deferred
dialect except to lock its existing fail-closed behavior.

Update EN/ZH Query Access reference and recipe docs with the exact
dialect/version/surface matrix. State that `admissible` is static dependency
analysis, not authorization, grants, RLS, masking, rewrite, or execution
snapshot equivalence. MCP remains without a Query Access tool.

Update the decision evidence with the actual GO/DEFER/KILL outcomes. It stays
`Proposed` until Task 9 completes.

## Task 9: Independent Final Audit and Acceptance

Before accepting, request a read-only Oracle security/design audit and Momus
adversarial diff review. Require explicit answers to:

1. Can a name, UDF, stored function, plugin, qualifier, type/coercion, or
   session mode bypass identity proof?
2. Can metadata/type/identity facts be mixed across connections or snapshots?
3. Are strict requirements complete for every newly admitted shape?
4. Can partial success or an unrelated candidate clear a reason?
5. Did default SDK, CLI, HTTP, MCP, PostgreSQL, or existing function-free
   MySQL/TiDB behavior drift?
6. Do public JSON/errors leak proof or connection data?

Fix every P1/P2 finding, rerun the relevant E2E and full gates, then repeat
review until no P1/P2 finding remains. Only then set the decision to
`Accepted`; otherwise leave it `Proposed` with evidence.

Required final gates:

- `docker compose -f docker/cli-e2e-compose.yaml up -d --wait`
- `go test ./... -count=1`
- `go test -tags postgresql ./... -count=1`
- `go test -race ./internal/domain/queryaccess/... ./internal/application/queryaccess/... ./pkg/deltascope/... -count=1`
- `go build ./...` and `go build -tags postgresql ./...`
- `go vet ./...` and `go vet -tags postgresql ./...`
- `golangci-lint run ./...`
- `make query-access-corpus-gates`
- `make pg-unit-test-gates`
- `make decision-record-gate`
- `make release-gofmt-gate`
- `npm test --prefix packages/deltascope-mcp`
- `git diff --check`
- `go mod tidy && git diff --exit-code go.mod go.sum`

Also run every MySQL/TiDB Docker E2E introduced by the milestone. A required
E2E skip or unavailable server is a blocker for a GO claim.

## Release Boundary

This plan does not choose a release version or authorize push, tag, publish,
workflow dispatch, Homebrew, or npm activity. A future release decision must
name the exact dialect/version/candidate matrix that actually passed proof. A
DEFER/KILL-only milestone does not justify a release feature claim.
