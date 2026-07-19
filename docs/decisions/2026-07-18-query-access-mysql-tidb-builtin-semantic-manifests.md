# Decision: MySQL/TiDB Builtin Semantic Manifests for Query Access

- Date: 2026-07-18
- Status: Accepted
- Baseline: `main@9491c5f`
- Milestone branch: `query-access-mysql-tidb-builtin-semantic-manifest`
- Related: [pure-read admissibility](2026-07-12-query-access-pure-read-admissibility.md), [common pure effects](2026-07-16-query-access-common-pure-effects.md), [MySQL/TiDB feasibility](2026-07-17-query-access-mysql-tidb-effect-identity-feasibility.md)

## Context

Query Access currently makes every MySQL and TiDB function-bearing `SELECT`
indeterminate. The feasibility decision correctly found no PostgreSQL-like catalog
OID identity root for builtins, so it deferred a catalog-identity implementation.
That conclusion must not be interpreted as "native builtins are unprovable".

For a bounded set of documented native syntax, the database language definition
can establish semantics more directly than a catalog entry. For example, the
tested MySQL images reject creation of a stored function named `COUNT`; TiDB 8.5
rejects stored and loadable function creation. The decisive fact is still not a
lowercased function name alone: it is a version-profiled, canonical native call
form with documented semantics and executable regression evidence.

Relevant primary documentation:

- [MySQL 5.7 function name parsing and resolution](https://dev.mysql.com/doc/refman/5.7/en/function-resolution.html)
- [MySQL 5.7 built-in function reference](https://dev.mysql.com/doc/refman/5.7/en/built-in-function-reference.html)
- [MySQL 8.4 aggregate functions](https://dev.mysql.com/doc/refman/8.4/en/aggregate-functions.html)
- [TiDB window functions](https://docs.pingcap.com/tidb/stable/window-functions/)

MySQL's parsing rules also show why a generic allowlist is unsafe. `IGNORE_SPACE`,
quoted identifiers, qualification, comments, and noncanonical spacing can affect
whether a token is parsed as a native special function or a stored function. A
manifest must prove the native form, not infer it from an AST name after that
information has been discarded.

## Decision

Introduce a second, independent proof model for MySQL and TiDB: immutable,
version-profiled builtin semantic manifests. This model is not a catalog identity
substitute and is never used for PostgreSQL.

Initial public target profiles are proposed as:

| Profile | Target | Initial candidate scope |
|---|---|---|
| `mysql-5.7` | MySQL 5.7 | `COUNT`, `SUM`, `AVG`, `MIN`, `MAX` |
| `mysql-8.0` | MySQL 8.0 | MySQL 5.7 scope, then documented window subset if proven |
| `mysql-8.4` | MySQL 8.4 | MySQL 5.7 scope, then documented window subset if proven |
| `tidb-8.5` | TiDB 8.5 | Only entries independently evidenced for TiDB 8.5 |

The final scope of each row is earned by the evidence tasks below. A profile may
ship with fewer entries, including zero. No profile can silently inherit entries
from another dialect or version.

The request carries an optional explicit analysis profile. An empty profile
preserves current behavior: all MySQL/TiDB function-bearing queries remain
indeterminate with `unknown_function_effect`. A non-empty profile is a
caller-declared static compatibility target, not a database connection, server
attestation, authorization decision, or execution-snapshot guarantee. Dialect and
profile mismatch is a bounded request-validation error with no fallback.

An effect can contribute to `read_only + admissible` only if all of these gates
hold:

1. The request selects the exact dialect/version profile owning the manifest.
2. The candidate is an immutable manifest entry backed by primary documentation
   and live Docker probes for that exact server image.
3. Parser facts prove the canonical native call form. Phase 1 rejects quoted or
   schema-qualified names, parser-ambiguous special-function forms, source forms
   whose native interpretation depends on unknown `IGNORE_SPACE`, and any form
   not captured by the parser facts.
4. Every effect candidate in the statement is proven. One unknown function,
   nested call, cast, UDF, or unsupported window feature keeps the whole result
   indeterminate.
5. Strict requirements fully resolve every effect input to physical base-table
   columns. Existing fail-closed boundaries remain in force.

The initial admissible shape is deliberately narrow:

- `COUNT(*)`.
- `COUNT(column)` and `SUM`/`AVG`/`MIN`/`MAX(column)` where the argument is one
  direct, physical base-table column and the profile's entry permits it.
- `ROW_NUMBER`, `RANK`, and `DENSE_RANK` only for profiles whose exact window
  syntax and strict partition/order dependencies are proven. Direct partition and
  order columns are allowed; `FILTER`, `DISTINCT`, frames, named windows,
  aggregate-local ordering, nested expressions, casts, literals, parameters,
  wildcard expansion gaps, views, CTE/derived inputs, unqualified ambiguity, and
  unresolved metadata remain indeterminate.

## Public Contract

The profile is opt-in and bounded. It must be represented by a closed public enum
or equivalent validated constants, not an arbitrary server version string or a
caller-supplied manifest. Invalid values, a profile/dialect mismatch, and a
non-empty external proof/resolver injection are rejected with bounded errors.

The public result remains requirements and bounded reason codes only. It must not
expose a manifest entry, profile implementation details, source text, function
name, parser facts, SQL mode, server connection details, identities, candidates,
DSN, credentials, literals, or `severity`.

Default `AnalyzeQueryAccess`, CLI, and HTTP remain offline; they can use an
explicit static profile but must not open a live database connection. When a
profiled query lacks complete physical metadata on those paths it remains
indeterminate. If promotion is shipped, it is exposed only through a separate
explicit SDK entry point over a caller-owned MySQL/TiDB `*sql.Conn`; that entry
point constructs its same-connection metadata resolver internally and rejects
external resolver injection. MCP still has no Query Access tool. PostgreSQL's
caller-owned connection and catalog proof chain is unchanged.

`admissible` continues to mean static Query Access requirements only. It is not
authorization, grant evaluation, RLS or masking evaluation, query rewrite proof,
or a guarantee that a later execution uses the same database state.

## Rationale

An OID-only rule conflates two different questions:

- whether the current catalog can identify an arbitrary executable object; and
- whether a language-standard, native builtin form is semantically fixed for a
  particular engine/version profile.

Catalog identity remains appropriate for PostgreSQL and arbitrary functions.
Versioned semantic manifests are appropriate only for the latter narrow MySQL/TiDB
case, because their proof root is the documented language contract plus
version-specific executable probes. Neither proof model authorizes arbitrary
user-defined functions.

## Deferred Scope

This decision does not support:

- arbitrary or user-defined functions, stored functions, plugin/loadable UDFs, or
  schema-qualified/quoted calls;
- a generic function-name allowlist, a volatility-only allowlist, or a
  caller-supplied manifest;
- unknown versions, forks, compatibility modes, or SQL modes not captured by the
  phase-1 native-form predicate;
- MySQL/TiDB casts, operators, literals, parameters, nested expressions, `FILTER`,
  `DISTINCT`, frames, named windows, ordered-set behavior, or broad common
  `SELECT` support;
- database connections or runtime identity lookup in default SDK/CLI/HTTP; or
- an MCP Query Access tool.

## Acceptance Evidence

Before this decision can become `Accepted`, the milestone must provide:

1. A primary-source ledger per manifest entry and profile, including version,
   syntax, semantic classification, and negative boundaries.
2. Non-skippable Docker tests for MySQL 5.7, MySQL 8.0, MySQL 8.4, and TiDB 8.5
   where a profile claims support. Tests must probe canonical native parsing and
   all documented collision/qualification/SQL-mode boundaries relevant to the
   entry.
3. Parser characterization that proves native-form facts survive extraction; an
   AST function name alone is insufficient evidence.
4. Corpus and cross-surface tests for positive entries and every fail-closed
   boundary, including no-leak success and bounded-error outputs.
5. PostgreSQL, function-free MySQL/TiDB, default-surface, CLI, HTTP, and MCP
   non-regression evidence.
6. A documented adversarial review of parser ambiguity, profile mismatch,
   collision attempts, incomplete strict requirements, and manifest mutability.

The decision remains `Proposed` until the offline fail-closed boundary and any
session-only promotion boundary are both covered by executable tests. A valid
profile without a session is not evidence of admission; it is evidence that the
closed profile contract was accepted and that the parser/gateway failed closed
without physical requirements.

## Task 2 Evidence Status

The dedicated `docker/query-access-builtin-compose.yaml` matrix was exercised
against independent services pinned to `mysql:5.7.44`, `mysql:8.0.46`,
`mysql:8.4.10`, and `pingcap/tidb:v8.5.7`. Each profile returned its own
version marker and passed the fixture-backed `COUNT(*)`, direct-column
`COUNT`/`SUM`/`AVG`/`MIN`/`MAX` probes. MySQL 8.0 and 8.4 and TiDB 8.5 passed
the canonical ranking-window probe; MySQL 5.7 was explicitly recorded as
window-deferred after its version was verified.

The same profile-specific probes recorded stored-function support for MySQL and
rejection for TiDB, MySQL builtin-like stored-function collision creation in
the documented spaced form, rejection of loadable-UDF creation, rejection of
schema-qualified and quoted builtin forms, and bounded spacing/comment outcomes
under controlled `IGNORE_SPACE`.

## Acceptance Evidence Summary

The production builtin semantic registry has been promoted with the entries
listed below. Each entry is backed by primary documentation, live Docker
probes against the exact server image, parser-native-form facts, complete
candidate closure, strict physical dependency requirements, no-leak coverage,
and cross-surface parity.

| Profile | Supported entries | Live Docker image | Observed VERSION() |
|---|---|---|---|
| `mysql-5.7` | `COUNT(*)`; direct-column `COUNT`, `SUM`, `AVG`, `MIN`, `MAX` | `mysql:5.7.44` | `5.7.44` |
| `mysql-8.0` | `mysql-5.7` scope plus `ROW_NUMBER`, `RANK`, `DENSE_RANK` with direct partition and order columns | `mysql:8.0.46` | `8.0.46` |
| `mysql-8.4` | `mysql-5.7` scope plus `ROW_NUMBER`, `RANK`, `DENSE_RANK` with direct partition and order columns | `mysql:8.4.10` | `8.4.10` |
| `tidb-8.5` | Independently evidenced `mysql-5.7` scope plus `ROW_NUMBER`, `RANK`, `DENSE_RANK` with direct partition and order columns | `pingcap/tidb:v8.5.7` | `8.0.11-TiDB-v8.5.7` |

Ranking-window entries require both `PARTITION BY` and `ORDER BY` clauses
with direct physical base-table columns. This is deliberately stricter than
MySQL's syntax contract (which accepts ranking windows without `ORDER BY`)
to preserve the design's "strict partition/order dependencies" boundary.

The following shapes remain indeterminate under every profile: empty profile
with a function call, quoted call, schema-qualified call, noncanonical
spacing (`COUNT (id)`), `IGNORE_SPACE`-dependent ambiguity, `DISTINCT`,
`FILTER`, aggregate-local `ORDER BY`, named windows, explicit frames, nested
operands, literals, parameters, casts, wildcard metadata gaps, views,
CTEs/derived effect inputs, unqualified relations, missing physical
requirements, unknown functions, and any query containing an unproven
candidate.

The following shapes return bounded validation errors (not indeterminate):
unknown profile value, profile/dialect mismatch, and any non-empty external
proof/resolver injection. A valid profile without the explicit
same-connection SDK session is not a validation error, but it is not evidence
of admission either: the parser/gateway fails closed without physical
requirements and the result remains indeterminate.

The default `Service{}` (no `builtinSemantic` bundle), default SDK
`AnalyzeQueryAccess`, CLI `query-access analyze`, and HTTP
`/v1/query-access/analyze` surfaces remain offline and fail-closed. A valid
profile without the explicit same-connection SDK session
(`AnalyzeMySQLTiDBQueryAccessWithSession`) cannot promote a function query.
MCP still has no Query Access tool. PostgreSQL behavior is unchanged.

Independent review evidence:

- Oracle (read-only security/architecture review, session
  `ses_0884d672affeUoWPeDuu4n9vjn`) confirmed 13 of 14 required checks PASS
  (parser-native-form proof, MySQL special-function parsing and `IGNORE_SPACE`,
  quoted/qualified calls, profile validation, manifest mutability, candidate
  closure, strict dependencies, profile leaks, default behavior, cross-surface
  parity, session-only promotion, PostgreSQL isolation, MCP boundary). The
  14th check was the empty-registry state, which this promotion addresses by
  design. Oracle's final re-review (session `ses_08822ab49ffeb0dmJrAxPnzOOz`)
  found 0 P1 and 2 P2 findings (exact version assertion; decision-record
  accuracy). Both P2 findings were reproduced and fixed.
- Momus plan-critic reviewed the implementation plan and the current diff
  through an untracked temporary mirror under `.omo/plans/` (which is
  gitignored). The initial deep executability review found 3 P1 blockers:
  (1) the plan's Docker start command omitted the `tidb85-fixture` service;
  (2) live probes only executed `ROW_NUMBER()` as live SQL, not `RANK()` and
  `DENSE_RANK()`; (3) no per-entry evidence ledger existed, the corpus runner
  had no profile field, and reference docs had stale "empty registry" claims.
  All 3 P1 findings were reproduced and fixed across commits `b29e729`,
  `f0c3c00`, `9c26a15`, `1758659`, `069d59b`, `636fb50`, and `1938322`. Momus's
  final re-review returned `[OKAY]` with all P1 findings resolved.

## Consequences

The old MySQL/TiDB feasibility record remains `Proposed`; it documents why a
catalog-OID implementation was not built. This decision refines the product
direction rather than changing that evidence retroactively. The production
builtin semantic registry is now populated for the four documented profiles;
the offline fail-closed boundary and the session-only promotion boundary are
both covered by executable tests.
