# Implementation Plan: MySQL/TiDB Builtin Semantic Manifests

## Objective

Implement version-profiled semantic proof for a deliberately narrow set of
documented native MySQL and TiDB aggregate/window effects. The milestone starts
from `main@9491c5f` on one worktree/branch:
`query-access-mysql-tidb-builtin-semantic-manifest`.

This plan implements no name-only allowlist. A function can become admissible
only through a profile-specific immutable manifest, canonical native-form parser
facts, complete candidate closure, and complete strict dependencies.

## Global Safety Rules

1. Preserve empty-profile behavior exactly: function-bearing MySQL/TiDB queries
   remain `indeterminate` with bounded unknown/unproven effect reasons.
2. Do not change PostgreSQL's controlled-session/catalog proof path, its trusted
   SDK contract, or its default fail-closed surfaces.
3. Do not make default SDK, CLI, or HTTP connect to a database. Profiles are
   static compatibility targets, not live-server claims.
4. No request may inject a manifest, resolver, server version, session, SQL mode,
   function identity, or trusted fact.
5. Any missing parser fact, unsupported AST node, incomplete requirement, unknown
   candidate, profile mismatch, evidence gap, or manifest miss must fail closed.
6. Every code edit begins with GitNexus impact analysis of the symbols changed.
   Stop and report before editing a HIGH or CRITICAL impact target. Before each
   commit run `gitnexus_detect_changes` and verify expected scope/d=1 callers.
7. Use a focused commit for every completed task. Do not push, tag, release,
   publish, rebase, merge, reset, amend, touch stashes, or touch `.omo/` or
   `skills-lock.json`.
8. The new decision record remains `Proposed` until the final audit proves every
   claimed profile entry with executable Docker tests and public-surface coverage.
9. Default SDK/CLI/HTTP profile analysis stays offline and cannot promote a
   function query without complete physical metadata. If promotion is shipped,
   it is available only through an explicit caller-owned MySQL/TiDB `*sql.Conn`
   SDK session that constructs its resolver internally and rejects external
   resolver injection; the profile remains a static compatibility target.

## Acceptance Boundary

The target is not "all common SELECT". The first allowed entries may be only:

- `COUNT(*)`.
- Direct physical base-column `COUNT`, `SUM`, `AVG`, `MIN`, and `MAX`.
- Direct physical base-column `ROW_NUMBER`, `RANK`, and `DENSE_RANK` partition/
  order expressions only where an exact profile has independent proof.

The following always remain indeterminate in this milestone unless a later
decision explicitly expands the scope: no profile, unknown profile/version,
profile/dialect mismatch, quoted/qualified/ambiguous form, stored/UDF call,
literal/parameter/NULL/cast/nested/subquery operands, `FILTER`, `DISTINCT`,
aggregate-local order, named windows, frames, wildcard/ambiguous metadata, views,
CTEs/derived effect inputs, missing metadata, unknown AST traversal, and any query
containing an unproven candidate.

## Task 1: Freeze Baseline and Evidence Ledger Schema

Create the decision record, this design, and a committed evidence-ledger schema.
The ledger is a human-readable primary-source record, not a test pretending to
query Docker. It must include for each proposed entry:

- dialect, profile, Docker image and exact observed version;
- documented syntax/semantics source and retrieval date;
- canonical native-call form and ambiguity rules;
- positive probe IDs and required negative probe IDs;
- strict dependency shape and all excluded modifiers;
- disposition: candidate, supported, deferred, or rejected.

The ledger must distinguish MySQL 5.7, 8.0, 8.4, and TiDB 8.5. It must not infer
TiDB facts from MySQL or newer MySQL facts from 5.7.

Verification:

- `make decision-record-gate`
- Link checks or the repository's docs gate if available.
- `git diff --check`

Commit: `docs: define MySQL TiDB builtin semantic proof boundary`.

## Task 2: Build Reproducible Docker Evidence Matrix

Add or extend compose services and integration tests for MySQL 5.7, MySQL 8.0,
MySQL 8.4, and TiDB 8.5. Use the official/current project images only and isolate
fixtures. Tests must be executable probes, not static self-comparisons.

For each target, tests must:

1. Assert the exact `VERSION()` expected by the profile.
2. Execute each proposed positive canonical builtin form.
3. Probe stored-function/UDF creation and collision behavior relevant to the
   candidate.
4. Probe schema qualification, quoted identifiers, and any special-function
   whitespace/SQL-mode behavior relevant to MySQL's native parsing rules.
5. Prove a negative form is rejected or behaves outside the supported native
   predicate; capture the bounded conclusion, never raw driver output.
6. Clean up fixtures and leave compose state reproducible.

If an image cannot support an entry, mark that entry deferred/rejected. Do not
work around a failed probe by changing expected static values. Update the ledger
only with the observed, reproducible result.

Verification:

- `docker compose -f docker/cli-e2e-compose.yaml up -d --wait` or a new dedicated
  compose file if parallel version support requires it.
- Focused integration command for the live profile probes.
- Repeat the focused command from a clean compose lifecycle.

Commit: `test: add executable MySQL TiDB builtin proof probes`.

## Task 3: Add the Public Analysis Profile Contract

Add a closed public profile type/constants to `pkg/deltascope` and propagate it
to the internal Query Access request. Keep the field optional and default-empty.

Required behavior:

- Empty profile: exact current results.
- A profile can only pair with its owning dialect.
- Unknown value and mismatch: bounded validation error, no fallback.
- Profile data cannot enter public result JSON except as a caller-provided request
  echo only if the existing request contract already exposes it; do not add a
  response echo merely for this feature.
- Non-PostgreSQL builds retain public API/build-tag compatibility.
- README and EN/ZH reference documentation describe target profile semantics and
  the fact that it is not live-server attestation.

Before editing the request types and service orchestration, run GitNexus impact
for those symbols and update all d=1 callers/tests.

Verification:

- Public SDK validation tests for empty, each valid profile, invalid profile, and
  every dialect mismatch.
- CLI and HTTP request decoding/validation tests if their request surfaces expose
  the new field.
- JSON no-leak/error tests with profile-shaped malicious values.
- `go test ./pkg/deltascope/... -count=1`
- `go test ./internal/application/queryaccess/... -count=1`

Commit: `feat: add query access analysis profiles`.

## Task 4: Replace Boolean Function Detection with Candidate Closure

Extend the MySQL/TiDB parser extraction so it creates ordered internal effect
candidates rather than relying only on a boolean function marker. Preserve all
existing reason codes and output behavior until the semantic gateway is enabled.

Candidate facts must retain native-form-relevant data: call kind, spelling/parser
classification, qualification/quoting/ambiguity, arity, operand kinds, aggregate
modifiers, window features, nested effects, and ordinal. The parser must traverse
all expression-bearing statement locations described in the design.

For an unrepresentable node or uncertain native interpretation, emit an explicit
unknown/unsupported candidate or existing bounded unsupported-traversal reason.
Do not silently omit it. Ensure nested functions such as `COUNT(my_udf(col))`
produce both candidates and therefore cannot promote.

Before editing parser collectors, run GitNexus impact for each target. If risk is
HIGH/CRITICAL, report the exact caller/process scope and add regression coverage
before changing behavior.

Verification:

- Parser characterization matrix for every expression location.
- Positive/negative native form facts for canonical, qualified, quoted, and
  spacing/SQL-mode-sensitive forms.
- Candidate ordinal, sort, dedupe, nested closure, and unknown-node tests.
- Existing MySQL/TiDB corpus remains unchanged with empty profile.

Commit: `feat: collect MySQL TiDB function effect candidates`.

## Task 5: Implement Immutable Semantic Manifest and Gateway

Implement an internal immutable manifest keyed by exact dialect/profile/native
form/call shape. The gateway runs only after read classification, candidate
closure, strict dependency resolution, and profile validation.

Gateway requirements:

- Deep-copy constructor and read access; mutation tests are mandatory.
- Validate candidate kind, canonical native-form facts, arity, operand shape,
  aggregate/window modifiers, and profile.
- Require every candidate ordinal to have one decision; duplicate/missing/foreign
  entries fail closed.
- Refuse unknown profile, empty profile, manifest miss, malformed facts, and
  incomplete strict dependencies.
- Call the existing reason/reclassification flow so proof success removes only
  the reasons resolved by all-proven semantic proof; an unrelated reason remains.
- Do not reuse PostgreSQL OID fields, fake OIDs, or `Trusted` booleans.

Add a small internal decision type rather than exposing candidates or manifest
internals in domain/public results. The full manifest must never be caller
mutable or serializable through a public result.

Before editing gateway/service symbols, run GitNexus impact and report any HIGH
or CRITICAL flow. Include adversarial tests for forged profile strings, mutation,
wrong dialect, candidate swaps, missing ordinals, and residual reason codes.

Verification:

- Focused domain/application tests.
- JSON and bounded-error no-leak tests.
- Function-free MySQL/TiDB non-regression tests.

Commit: `feat: prove versioned MySQL TiDB builtin effects`.

## Task 6: Enable MySQL Aggregate Profiles

Populate only the MySQL entries whose Task 2 ledger/probes are complete. Start
with MySQL 5.7. MySQL 8.0 and 8.4 get separate entries and tests even if their
semantics match 5.7; no profile aliases are allowed.

Required positive tests per supported profile:

- `COUNT(*)` over a schema-qualified physical base relation.
- Direct-column `COUNT`, `SUM`, `AVG`, `MIN`, and `MAX` with complete strict
  requirements.

Required negative tests per supported profile:

- unqualified relation;
- schema-qualified/quoted/ambiguous function form;
- stored/UDF and nested function;
- literal, parameter, `NULL`, cast, arithmetic or other nested operand;
- `DISTINCT`, aggregate-local `ORDER BY`, `FILTER`, wildcard/metadata gap;
- mixed query containing one allowed and one unknown candidate.

Every positive session test must assert `read_only + admissible` and exact
physical requirements through the explicit same-connection SDK session. The
corresponding offline SDK/CLI/HTTP tests must assert `indeterminate` when
physical metadata is unavailable, plus no internal proof data in public output.
Every negative test on both paths must assert `indeterminate` and a bounded
reason, not just lack of admission.

Verification:

- Profile-specific Docker E2E against each claimed MySQL image.
- Offline SDK, CLI, and HTTP parity for exact profile input, plus session-only
  SDK promotion coverage.
- `make query-access-corpus-gates`.

Commit: `feat: admit proven MySQL aggregate effects`.

## Task 7: Enable MySQL 8.x Ranking Windows Only If Proven

Use Task 2 evidence to decide MySQL 8.0 and 8.4 independently. Add only direct
`ROW_NUMBER`, `RANK`, and `DENSE_RANK` calls with direct physical partition/order
columns if documentation and parser facts prove them. MySQL 5.7 has no window
entry.

Positive session tests must assert exact partition `window` and order `ordering`
strict usages. Offline SDK/CLI/HTTP tests remain indeterminate without physical
metadata. Negative tests must cover named windows, explicit/default frames where
the parser cannot prove the phase-1 predicate, `FILTER`, `DISTINCT`, nested
expressions, missing order dependencies, and any unproven function in the query.

If either version's live evidence is incomplete, leave that profile's windows
deferred and document the result rather than creating a placeholder manifest.

Verification:

- Exact MySQL 8.0/8.4 Docker profile E2E.
- Parser/application/corpus regression tests.

Commit: `feat: admit proven MySQL ranking windows` or
`docs: defer MySQL ranking windows pending proof`.

## Task 8: Decide and Implement the TiDB 8.5 Subset Independently

Repeat Tasks 4-7 conclusions against TiDB 8.5 only. Do not copy the MySQL
manifest, parser predicate, or documentation conclusion without TiDB-specific
proof. The task can legitimately conclude that TiDB remains deferred.

If entries are enabled, add TiDB-specific manifest rows, Docker probes, positive
and negative corpus cases, and all public-surface assertions. If no entry meets
the proof model, leave all TiDB function candidates indeterminate and update the
decision/ledger with the precise failure point.

Verification:

- TiDB 8.5 Docker profile E2E.
- Function-free TiDB non-regression.
- Cross-dialect test that a MySQL profile cannot affect TiDB and vice versa.

Commit: `feat: admit proven TiDB builtin effects` or
`docs: defer TiDB builtin semantic profiles`.

## Task 9: Corpus, Documentation, and Cross-Surface Contract

Add fixtures for every shipped positive entry and every mandatory negative
boundary. Fixture input must record the profile explicitly so the corpus tests
exercise the same public request semantics as SDK/CLI/HTTP.

Update:

- Query Access EN/ZH reference documentation.
- Public SDK README and any CLI/HTTP request documentation.
- The roadmap: distinguish opt-in profiled support from the still-deferred broad
  default-path MySQL/TiDB function analysis.
- The decision record with actual evidence only; do not paste task logs.

Validate no leaks in successful and failing SDK JSON, CLI stdout/stderr, and HTTP
responses. Include raw SQL, literals, function names, manifest/profile internals,
driver errors, DSNs, credentials, candidates, session/context data, and severity
in the forbidden-marker set where applicable.

Verification:

- Dedicated SDK, CLI, HTTP no-leak/parity tests.
- MCP surface contract test confirms no new tool.
- Docs examples gate and decision-record gate.

Commit: `docs: document profiled MySQL TiDB query access effects`.

## Task 10: Independent Review, Final Audit, and Decision

Before acceptance, conduct an adversarial review with these questions:

1. Can any parsed name, qualified name, quoted identifier, whitespace form, SQL
   mode, or UDF/stored function bypass native-form validation?
2. Does every function-bearing expression position produce a candidate or a
   fail-closed unsupported marker?
3. Can a profile be spoofed, mutated, mismatched, or leaked?
4. Can incomplete strict dependencies, projection-only mode, or a mixed query
   become admissible?
5. Are each dialect/version's claims live and independently executable?
6. Did PostgreSQL, default MySQL/TiDB, function-free existing cases, CLI, HTTP,
   MCP, and release surfaces remain unchanged outside the intended profile path?

Use Oracle for code/security review. For Momus, place a review-only mirror or
pointer under `.omo/plans/` only if its tool requires that path; do not commit
`.omo/`. Record an unavailable tool honestly and perform a documented manual
adversarial audit rather than claiming approval.

The decision may change from `Proposed` to `Accepted` only if every supported
entry has live Docker evidence, all final gates pass without skipped required
profile tests, and review findings P1/P2 are closed. If evidence is incomplete,
retain `Proposed` and merge only a documentation/defer outcome if the product
boundary is still accurate.

Final required gates:

- `go test ./... -count=1`
- `go test -tags postgresql ./... -count=1`
- `go test -race ./internal/domain/queryaccess/... ./internal/application/queryaccess/... ./pkg/deltascope/... -count=1`
- `go build ./...` and `go build -tags postgresql ./...`
- `go vet ./...` and `go vet -tags postgresql ./...`
- `golangci-lint run ./...`
- `make query-access-corpus-gates`
- `make pg-unit-test-gates`
- profile Docker integration commands for every claimed target
- `make decision-record-gate`
- `make release-gofmt-gate`
- `npm test --prefix packages/deltascope-mcp`
- `git diff --check`
- `go mod tidy && git diff --exit-code go.mod go.sum`

Before each commit, run `gitnexus_detect_changes`. Before final reporting, refresh
the GitNexus index, state the exact branch/base/HEAD, distinguish task and
milestone diffs, and list any profile rows still deferred.

## Executable QA Matrix Added After Review

The following commands and binary pass criteria make the task list executable.
Required profile tests fail when their service is unavailable; they do not skip
claimed evidence.

### Docker evidence

- Compose file: `docker/query-access-builtin-compose.yaml`.
- Start: `docker compose -f docker/query-access-builtin-compose.yaml up -d --wait mysql57 mysql80 mysql84 tidb85 tidb85-fixture`.
- Stop/cleanup: `docker compose -f docker/query-access-builtin-compose.yaml down -v --remove-orphans`.
- Probe command: `go test -tags integration ./internal/infrastructure/metadata/mysql -run 'TestBuiltinSemantic(57|80|84|TiDB85)_Live' -count=1 -v`.
- Pass criteria: every selected service returns the exact profile version prefix,
  canonical positive calls succeed, collision/qualification/quote/spacing and
  unsupported-form probes return the expected bounded outcome, and the test
  process exits 0 without `SKIP` output.

### Parser closure

- Command: `go test ./internal/infrastructure/parser/tidb -run 'TestQueryAccess(EffectCandidates|CandidateClosure|NativeForm|UnsupportedTraversal)' -count=1 -v`.
- Pass criteria: candidates are ordered, nested, and complete across projection,
  WHERE, HAVING, GROUP BY, ORDER BY, LIMIT/OFFSET, JOIN, CTE, derived tables,
  subqueries, set operations, aggregate modifiers, and windows; any unsupported
  location emits an explicit fail-closed marker; canonical, qualified, quoted,
  spaced, commented, and ambiguous forms never collapse to a name-only fact.

### Gateway and offline boundary

- Commands: `go test ./internal/application/queryaccess -run 'Test(Builtin|QueryAccessProfile|Offline|StrictDependency|Manifest)' -count=1 -v` and `go test ./pkg/deltascope -run 'Test(AnalyzeQueryAccessProfile|Offline|NoLeak)' -count=1 -v`.
- Pass criteria: empty profile preserves `unknown_function_effect`; invalid or
  mismatched profiles return bounded errors; default profile requests never open
  a connection and remain indeterminate when physical metadata is absent; only
  complete candidate closure plus complete strict physical requirements can
  promote; manifest/source mutation cannot alter admission.

### Cross-surface and session-only promotion

- Commands: `go test ./internal/interfaces/cli -run 'TestQueryAccess(Profile|Offline|NoLeak)' -count=1 -v`, `go test ./internal/interfaces/http -run 'TestHandlerQueryAccess(Profile|Offline|NoLeak)' -count=1 -v`, and `go test ./pkg/deltascope -run 'Test(MySQLTiDBQueryAccessSession|ProfileParity)' -count=1 -v`.
- Pass criteria: SDK/CLI/HTTP produce identical bounded offline results for the
  same SQL/profile, no response/error contains raw SQL, literals, function names,
  candidates, manifest internals, DSNs, credentials, driver errors, session
  details, or `severity`, and the explicit session API alone can promote a
  profile-backed query after same-connection physical metadata is proven.

### Profile-specific regression commands

- MySQL 5.7: `go test ./internal/application/queryaccess -run 'TestMySQL57(Profile|Aggregate|Boundary)' -count=1 -v`.
- MySQL 8.0: `go test ./internal/application/queryaccess -run 'TestMySQL80(Profile|Aggregate|Window|Boundary)' -count=1 -v`.
- MySQL 8.4: `go test ./internal/application/queryaccess -run 'TestMySQL84(Profile|Aggregate|Window|Boundary)' -count=1 -v`.
- TiDB 8.5: `go test ./internal/application/queryaccess -run 'TestTiDB85(Profile|Aggregate|Window|Boundary)' -count=1 -v`.
- Pass criteria: each command asserts only that profile's independent manifest
  rows; deferred rows assert `indeterminate` with bounded reasons rather than
  pretending a neighboring dialect/version proves them.
