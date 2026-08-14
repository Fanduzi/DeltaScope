# Implementation Plan: Deprecate Dialect-Specific Query Access Session APIs

## Status

Proposed implementation plan. It authorizes no code change, merge, push,
release, issue closure, or API removal by itself.

## 1. Establish the Fixed Point

- Start one milestone branch from current `main` and record its full SHA.
- Confirm GitHub issue #3 is open and issue #4 remains the separate test-
  consolidation tracker.
- Use CodeGraph and exact source search to enumerate the two old session types,
  four entry functions, tagged/untagged PostgreSQL declarations, current docs,
  and retained compatibility tests.
- Record public GitHub code-search results as limited evidence only; do not infer
  that private consumers do not exist.

## 2. Characterize Compatibility Before Editing

- Run focused old PostgreSQL and MySQL/TiDB session tests in default and
  PostgreSQL-tagged builds.
- Record signatures, exported names, sentinel identities, validation priority,
  caller connection ownership, result equivalence, build stubs, no-execution,
  and no-leak behavior.
- Run focused unified-versus-old equivalence tests. The intended production
  change is comments only; any behavioral diff is a blocker.

## 3. Add Native Go Deprecation Notices

- Add `Deprecated:` notices to `PostgreSQLQueryAccessSession` and
  `MySQLTiDBQueryAccessSession`.
- Add replacement-specific notices to both constructors and both analyzers.
- Apply matching notices to the tagged and untagged PostgreSQL function
  declarations.
- Leave all function bodies, signatures, types, fields, build constraints,
  errors, and execution paths unchanged.
- Do not mark old error sentinels deprecated.

## 4. Make Unified Guidance Canonical

- Update `pkg/deltascope/README.md` and the current EN/ZH Query Access reference
  and recipe documents to use the unified constructor and analyzer.
- Keep a single concise migration section listing all six deprecated names and
  their replacements.
- Explain request-dialect omission, caller-owned connection lifecycle, and the
  need to migrate `errors.Is` handling to generic online sentinels.
- Do not rewrite historical ADRs, changelogs, release notes, or old milestone
  evidence.
- Synchronize changed L3 headers and L2 README responsibility text only when
  required by the three-level documentation policy.

## 5. Verify the Deprecation Surface

- Inspect `go doc` in default and PostgreSQL-tagged contexts and confirm all six
  public identifiers show canonical deprecation guidance.
- If a regression test is needed, add one minimal source/`go doc` contract test
  for the six markers; do not create a general documentation parser.
- Confirm an exported-API comparison reports no removed or changed signature.
- Confirm source search finds no current canonical example that still recommends
  a dialect-specific constructor or analyzer. Historical records are excluded.

## 6. Run Compatibility and Repository Gates

- Run default and PostgreSQL-tagged full tests plus affected race tests.
- Run build and vet in both configurations, `make lint`, Query Access corpus,
  PostgreSQL unit/confidence gates, and focused online integration evidence.
- Run decision-record, gofmt, three-level documentation, module-tidy, and diff
  checks.
- Verify no CLI, HTTP, MCP, runtime, public JSON, dependency, version, workflow,
  or release-surface file changed.

## 7. Independent Review and ADR Decision

- Obtain an independent read-only Standards and Spec review from the fixed
  milestone base.
- Treat any runtime behavior change, old error change, API removal, misleading
  migration claim, or missing tagged/untagged notice as blocking.
- Keep the ADR Proposed while any P0, P1, or P2 remains.
- After all acceptance evidence passes, update only the ADR status and concise
  evidence in a focused commit.

## 8. Delivery Closure

- Fast-forward local `main` only after human integration approval and rerun the
  required gates on `main`.
- Push only with explicit authorization and verify required CI for the exact
  merged SHA.
- Close issue #3 only after merge, push, green CI, and Accepted ADR evidence.
- Leave issue #4 open and note that future consolidation must retain the
  minimum old-API compatibility suite.
- Do not create a tag, release, package publication, or removal follow-up unless
  separately authorized.

## Suggested Commit Boundaries

1. `docs(queryaccess): propose dialect session API deprecation`
2. `docs(queryaccess): deprecate dialect-specific session APIs`
3. `docs(queryaccess): accept dialect session API deprecation`

The second commit may include a small contract test only if needed to prevent
loss of the Go deprecation markers. No runtime refactor is implied.
