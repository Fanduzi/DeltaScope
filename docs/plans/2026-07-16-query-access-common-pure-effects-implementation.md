# Implementation Plan: Query Access Common Pure-Effect Admissibility

Date: 2026-07-16
Status: Proposed
Branch: `query-access-common-pure-effects`
Baseline: v0.390.0 (`d72eeb4`)
Decision: `docs/decisions/2026-07-16-query-access-common-pure-effects.md`

## Global Rules

- Work sequentially on this milestone branch. Make one focused commit per
  task; do not rebase, merge, push, tag, release, or change a published tag.
- Before editing a production symbol, run GitNexus impact analysis and record
  d=1 callers plus a test plan. Before each commit, run detect-changes; if the
  local CLI lacks that command, report the limitation and use available
  GitNexus status plus a reviewed diff.
- Do not start a task from its report. Read current code, this decision/spec,
  and all prior task evidence first.
- Each behavior change needs a base reproduction or characterization test. A
  passing new test alone is not proof that the old boundary was real.
- Perform self-review before requesting Oracle/Momus review. Fix every P1/P2,
  rerun affected gates, and repeat review until no P1/P2 remain. Do not call a
  task ready while a reviewer reports unresolved findings.
- No function name, schema name, volatility class, or parser token may be a
  trust shortcut. No public JSON/error may expose identities, SQL, literals,
  credentials, connection data, or `severity`.
- A kill criterion leaves the affected dialect fail-closed and updates decision
  evidence; it is not an invitation to add a broader allowlist.

## Task 1: Characterize Current Cross-Dialect Boundaries

Add tests and corpus fixtures proving current behavior for MySQL, TiDB, and
PostgreSQL:

- `COUNT(*)`, `COUNT(column)`, `SUM`, `AVG`, `MIN`, `MAX`;
- direct-column `GROUP BY` aggregate;
- `ROW_NUMBER`, `RANK`, `DENSE_RANK` with direct partition/order columns;
- negatives: DISTINCT, FILTER, local aggregate order, frame, named window,
  UDF-looking name, cast, parameter, literal overload, wildcard, ambiguity,
  view, CTE/derived input, and unqualified PostgreSQL relation.

For every case assert classification, admission, reason codes, strict
requirements, projection-only requirements/warnings, and no-leak output. The
baseline must show TiDB-parser MySQL/TiDB function-bearing queries are
indeterminate under `unknown_function_effect`; PostgreSQL default stays
indeterminate, while current public trusted PG17 support is preserved.

Gates: focused parser/application/domain/SDK tests, `make
query-access-corpus-gates`, `go test ./...`, `go test -tags postgresql ./...`,
and `git diff --check`.

## Task 2: Dependency Completeness Audit and Repair

Audit every AST expression location for aggregate/window dependencies in the
TiDB and PostgreSQL parsers. Add shared semantic tests proving strict mode
includes all argument, grouping, having, window partition/order, and output
lineage dependencies, while projection-only emits its existing warning.

Fix only confirmed omissions. Any expression holder that cannot be completely
traversed must emit the existing bounded unsupported/unproven outcome. Do not
promote any function in this task.

Required adversarial cases: nested subquery, set operation, CTE, JOIN, HAVING,
window partition/order, LIMIT/OFFSET, wildcard, ambiguous column, and a hidden
function in each supported expression holder.

## Task 3: Dialect Proof Feasibility Research Gate

This task has no admission promotion. Produce a version-scoped evidence ledger
in decision/design documents and executable research tests where possible.

### PostgreSQL 17

- Probe exact catalog identities and argument/result types for Phase 1
  aggregates/windows on Docker PG17.
- Audit each candidate for row-source-only dependencies and all required
  strict columns.
- Record the precise manifest tuple and negative nearby identities.

### MySQL and TiDB

- Determine supported server/version test environments.
- Research and test builtin identity/shadowing behavior for stored functions,
  UDFs/plugins, qualified names, SQL mode, and version changes.
- Decide whether a bounded static proof is defensible, a session-bound resolver
  is necessary, or the dialect must remain deferred.

Go/no-go: if proof needs a name-only or generic determinism allowlist, stop
that dialect. Do not proceed to its promotion task. The decision remains
`Proposed`.

## Task 4: Internal Candidate Collection

After Task 3 approval, add bounded internal pure-effect candidates for Phase 1
AST forms. Preserve stable AST ordinal, class, modifiers, direct operand
provenance, and dependency-role links. Candidate collection must be complete or
fail closed; it must not appear in a domain/public result.

Add tests for candidate order, every expression holder, direct-column-only
eligibility, excluded modifiers, unknown functions, and no public JSON leak.
Keep current reason codes and admissions unchanged in this task.

## Task 5: Shared Completeness and Per-Candidate Proof Gateway

Implement the application gate that combines candidates, resolved physical
requirements, and dialect proof facts. It must:

- validate each candidate independently;
- reject incomplete batches, duplicate/missing ordinals, mismatched types, and
  foreign facts;
- remove a function-effect reason only for a fully proven complete batch; and
- retain unrelated unresolved/unproven reasons and use existing final admission
  recomputation as the sole producer of `admissible`.

Add adversarial tests for partial success, swapped facts, context drift,
metadata mismatch, projection-only, output ordering, and no public leak.

## Task 6: PostgreSQL PG17 Manifest Expansion

Extend the public trusted SDK path only after Task 3's ledger is approved. Add
audited PG17 manifest entries and exact identity tests for every promoted
aggregate/window builtin. Start with the smallest safe subset; do not bundle
casts or arbitrary scalar functions.

PG17 public SDK Docker E2E must call only
`NewPostgreSQLQueryAccessSessionFromConn` and
`AnalyzePostgreSQLQueryAccessWithSession`. It must prove positive requirements
and all negative boundaries. Default SDK, CLI, HTTP, and MCP remain
fail-closed/no-tool. Run same-connection PID, caller ownership, context drift,
and no-leak tests.

## Task 7: MySQL/TiDB Dialect Implementation or Recorded Deferral

Proceed only for a dialect whose Task 3 proof model passed.

- Implement its bounded provider/policy without importing PostgreSQL catalog
  assumptions.
- Add version-scoped integration evidence when the model requires a live
  connection.
- If a static proof model is accepted, lock exact dialect/version/token
  conditions and shadowing negatives in tests and docs.
- Preserve `unknown_function_effect` for every unproven candidate.
- Do not make CLI/HTTP open a database connection. If connection-bound proof
  is required, design a separate opt-in SDK session path rather than silently
  changing default surfaces.

If research fails, commit only evidence and documented deferral. This is a
successful safety outcome, not a failed implementation task.

## Task 8: Corpus and Cross-Surface Contract

Add canonical positive and negative fixtures for every dialect actually
promoted. The corpus must assert classification/admission, strict versus
projection-only requirements, bounded reasons, deterministic order, and
no-leak. Add SDK/CLI/HTTP parity tests only for surfaces whose proof model is
available; MCP remains explicitly without Query Access.

Update EN/ZH reference and recipe docs with the exact dialect matrix. Never
describe a candidate as supported merely because it is characterized.

## Task 9: Independent Audit and Decision

Run an independent read-only security review that asks:

1. Can a spelling, UDF, overload, schema, volatility, or type coercion bypass
   proof?
2. Are strict requirements complete for every admitted aggregate/window shape?
3. Can one candidate's proof clear another candidate's reason?
4. Can resolver/session/catalog failures or cross-connection facts promote?
5. Did default PostgreSQL/CLI/HTTP/MCP behavior drift?
6. Do public JSON/errors leak proof or connection data?

Required final gates: `go test ./... -count=1`, `go test -tags postgresql
./... -count=1`, race on Query Access/domain/SDK, default and PostgreSQL
build/vet, `golangci-lint run ./...`, `make query-access-corpus-gates`, `make
pg-unit-test-gates`, `make decision-record-gate`, `make release-gofmt-gate`,
`npm test --prefix packages/deltascope-mcp`, `git diff --check`, and `go mod
tidy && git diff --exit-code go.mod go.sum`.

Run Docker-backed gates for every dialect whose promotion claims need a live
server. A skipped or unavailable required E2E is a blocker, not a pass. Change
the decision to `Accepted` only after the audit has no P1/P2 findings and all
claimed dialects satisfy proof and corpus evidence.

## Release Boundary

This plan does not select a version, prepare release surfaces, push, tag, or
publish. A later release decision must state exactly which dialects and Phase 1
candidates shipped. Do not imply cross-dialect parity if a proof feasibility
gate deferred one dialect.
