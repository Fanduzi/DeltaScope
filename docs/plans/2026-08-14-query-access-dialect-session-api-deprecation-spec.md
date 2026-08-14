# Spec: Deprecate Dialect-Specific Query Access Session APIs

## Status

Proposed. This document defines a source-compatible documentation change for
GitHub issue #3. It does not authorize implementation, merging, release, or
future API removal.

## Problem

DeltaScope now exposes one product-neutral online Query Access boundary:
`OnlineQueryAccessSession`, `NewOnlineQueryAccessSessionFromConn`, and
`AnalyzeOnlineQueryAccessWithSession`. Official CLI and HTTP paths use that
boundary, and it routes the existing MySQL, TiDB, and PostgreSQL proof engines
from identity observed on the caller-owned connection.

The older public SDK surface remains available:

- `PostgreSQLQueryAccessSession`;
- `NewPostgreSQLQueryAccessSessionFromConn`;
- `AnalyzePostgreSQLQueryAccessWithSession`;
- `MySQLTiDBQueryAccessSession`;
- `NewMySQLTiDBQueryAccessSessionFromConn`;
- `AnalyzeMySQLTiDBQueryAccessWithSession`.

Keeping two recommended ways to perform the same online analysis makes the
public API and documentation harder to navigate. Removing the old surface now
would be an unnecessary source break. Public GitHub code search found no
external references, but that is not evidence about private consumers.

## Objective

Make the unified online entry the only recommended SDK path while preserving
the complete runtime and source-compatibility contract of the six older public
identifiers. Give Go tooling and users a clear migration path without setting
an automatic removal date.

## Required Contract

1. Add canonical Go `Deprecated:` documentation to the two dialect-specific
   session types and four dialect-specific constructor/analyzer functions.
   Every build-specific declaration of a deprecated PostgreSQL function must
   carry the same notice.
2. Each notice names the direct replacement:
   `OnlineQueryAccessSession`, `NewOnlineQueryAccessSessionFromConn`, or
   `AnalyzeOnlineQueryAccessWithSession` as appropriate.
3. Do not deprecate the dialect-specific error sentinels. Existing callers
   still need them while they use the compatibility APIs, and the unified API
   has a different bounded error vocabulary.
4. Do not change implementation, validation order, error identity or text,
   wrapping, result values, build-tag behavior, caller connection ownership,
   no-execution behavior, or no-leak behavior of any old or unified API.
5. Do not add runtime warnings, logs, telemetry, counters, feature flags, or
   new errors. Deprecation is communicated only through Go documentation and
   maintained user documentation.
6. Current SDK documentation and examples use the unified constructor and
   analyzer as the canonical path. The migration example leaves
   `QueryAccessRequest.Dialect` empty so observed server identity selects the
   route; a non-empty dialect remains an optional matching constraint under
   the Accepted unified-entry contract.
7. Keep one concise compatibility/migration section. It identifies all six
   deprecated names, shows the replacement sequence, explains that callers
   must migrate from dialect-specific errors to `ErrOnlineQueryAccess...`
   sentinels using `errors.Is`, and states that the caller still owns the
   `*sql.Conn`.
8. Historical ADRs, changelogs, and release notes remain historical evidence
   and are not rewritten. Version bumps, CHANGELOG entries, and release notes
   belong to the next release-preparation milestone.
9. Existing dialect-specific API tests remain. At minimum they continue to pin
   source availability in tagged and untagged builds, old error behavior,
   validation priority, result equivalence, caller ownership, no-execution,
   and no-leak boundaries.
10. GitHub issue #4 may later consolidate duplicated behavior matrices, but it
    may not remove the minimum old-API compatibility contract while the names
    remain exported.
11. There is no scheduled removal version. Removal requires a separate GitHub
    issue and Proposed ADR that explicitly evaluates a breaking release,
    current public usage, migration cost, and compatibility policy. This
    decision does not pre-authorize that removal.
12. GitHub issue #3 closes only after implementation and documentation are
    merged, this ADR is Accepted, and the required remote CI for the merged SHA
    succeeds.

## Documentation Scope

The implementation updates only current guidance:

- `pkg/deltascope/README.md`;
- `docs/reference/query-access-analysis.md`;
- `docs/reference/query-access-analysis_zh.md`;
- `docs/recipe/query-platform-access-analysis.md`;
- `docs/recipe/query-platform-access-analysis_zh.md`.

Other current documentation may be updated only if implementation review finds
that it actively recommends a dialect-specific entry. Historical records are
excluded.

## Explicit Non-Goals

- No API deletion, rename, forwarding rewrite, or new compatibility wrapper.
- No deprecation of dialect-specific error sentinels.
- No change to supported products, versions, profiles, SQL shapes, proof
  engines, results, requirements, reason codes, or public JSON.
- No CLI, HTTP, MCP, authorization, registry, TLS, credential, or connection
  lifecycle change.
- No runtime warning or deprecation detection mechanism.
- No test deletion or broad test-matrix consolidation.
- No version bump, release note, package publication, or removal deadline.

## Acceptance Evidence

The ADR may change to Accepted only after all of the following exist:

- Go documentation inspection confirms all six public identifiers are marked
  with canonical `Deprecated:` notices and both PostgreSQL build variants show
  the notice.
- Current EN/ZH reference and recipe documentation plus the SDK README present
  the unified entry as canonical and contain one accurate migration section.
- Existing old-API characterization suites pass unchanged in default and
  PostgreSQL-tagged builds; focused unified-entry equivalence tests also pass.
- Public API comparison confirms no exported identifier was removed or had its
  signature changed.
- Default and PostgreSQL-tagged tests, affected race tests, builds, vet, lint,
  Query Access corpus, PostgreSQL gates, formatting, three-level documentation,
  decision-record, module-tidy, and diff checks pass.
- An independent read-only review reports no P0, P1, or P2 finding and confirms
  that deprecation did not change runtime, privacy, transport, or proof
  behavior.
