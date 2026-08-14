# Decision: Deprecate Dialect-Specific Query Access Session APIs

- Date: 2026-08-14
- Status: Proposed
- Related: [Unified online analysis entry](2026-08-12-query-access-online-analysis-entry.md), [API lifecycle issue #3](https://github.com/Fanduzi/DeltaScope/issues/3), [test ownership issue #4](https://github.com/Fanduzi/DeltaScope/issues/4)
- Spec: `docs/plans/2026-08-14-query-access-dialect-session-api-deprecation-spec.md`
- Design: `docs/plans/2026-08-14-query-access-dialect-session-api-deprecation-design.md`
- Implementation: `docs/plans/2026-08-14-query-access-dialect-session-api-deprecation-implementation.md`

## Context

The Accepted unified online Query Access entry now routes MySQL, TiDB, and
PostgreSQL proof from identity observed on a caller-owned `*sql.Conn`. CLI and
HTTP use that entry. The older dialect-specific session types, constructors,
and analyzers remain exported and behavior-compatible, but no longer represent
a distinct capability or preferred integration path.

Keeping both paths equally recommended increases public surface and
documentation cost. Immediate removal would break source compatibility. Public
GitHub code search found only this repository's references, but cannot establish
whether private consumers exist. DeltaScope also has no explicit policy that
makes pre-1.0 status sufficient reason for an unannounced source break.

## Proposed Decision

Soft-deprecate these public identifiers using Go's canonical `Deprecated:`
documentation convention:

- `PostgreSQLQueryAccessSession`;
- `NewPostgreSQLQueryAccessSessionFromConn`;
- `AnalyzePostgreSQLQueryAccessWithSession`;
- `MySQLTiDBQueryAccessSession`;
- `NewMySQLTiDBQueryAccessSessionFromConn`;
- `AnalyzeMySQLTiDBQueryAccessWithSession`.

Name `OnlineQueryAccessSession`, `NewOnlineQueryAccessSessionFromConn`, and
`AnalyzeOnlineQueryAccessWithSession` as their replacements. Apply equivalent
function notices in PostgreSQL tagged and untagged source files.

Do not deprecate the old error sentinels. Do not change any old function body,
signature, validation order, error identity/text, result, build stub,
connection-ownership rule, proof behavior, no-execution guarantee, or no-leak
boundary. Do not emit runtime warnings.

Make the unified entry the canonical path in current SDK, reference, and recipe
documentation. Retain one concise migration section that explains the new
generic errors and caller-owned connection semantics. Historical ADRs,
changelogs, and release notes remain unchanged.

Set no removal version. Any future removal requires a separate issue and ADR
that explicitly treats it as a breaking change and re-evaluates usage and
migration cost.

## Rationale

Go's native deprecation marker is sufficient: it reaches `go doc`, package
documentation, editors, and language tooling without adding runtime behavior or
maintenance machinery. The unified entry is already the reviewed owner of
identity-derived routing, so continuing to teach product-specific wrappers
would preserve accidental complexity rather than a useful choice.

Compatibility is cheaper than guessing. Keeping the old implementation and
errors intact lets consumers migrate deliberately. Avoiding a removal date
prevents this documentation decision from silently authorizing a future source
break without current evidence.

## Contract

- Six old public identifiers remain exported and source-compatible but carry
  canonical deprecation notices.
- Unified online session construction and analysis are the only recommended
  SDK path.
- Old error sentinels and all old runtime behavior remain supported while the
  compatibility APIs exist.
- No runtime warning, log, telemetry, new error, output field, SQL capability,
  transport behavior, or connection lifecycle changes.
- Current EN/ZH documentation gives one migration path; historical records are
  not rewritten.
- Existing compatibility tests remain. Issue #4 may reduce duplicated behavior
  matrices later but must retain the minimum old-API contract.
- Deprecation has no automatic removal deadline.

## Consequences

Positive consequences:

- New SDK users see one online Query Access entry.
- Existing users receive standard tooling guidance without a source or runtime
  break.
- Documentation and future capability work have one canonical public seam.

Costs and limitations:

- Six compatibility identifiers and their minimum tests remain maintained.
- Consumers must update error handling because unified sentinels are not aliases
  for dialect-specific errors.
- The public surface does not become smaller until a separately approved
  breaking change.

## Alternatives Rejected

- Retain both paths indefinitely as equally recommended: rejected because they
  expose the same capability and increase navigation and maintenance cost.
- Remove the old APIs now: rejected because private usage is unknown and the
  source break has no demonstrated necessity.
- Announce a fixed removal version: rejected because it would be an arbitrary
  deadline without usage or migration evidence.
- Deprecate old errors too: rejected because compatibility callers still need
  to match them correctly.
- Add runtime warnings: rejected because documentation guidance must not alter
  library output or observability.
- Delete duplicate tests now: rejected because issue #4 owns that evidence
  review separately.

## Deferred Scope

- Any physical removal or rename of the six deprecated identifiers.
- A project-wide breaking-release or semantic-versioning policy.
- Consolidation of repeated SDK/CLI/HTTP behavior matrices under issue #4.
- New Query Access capabilities, products, versions, SQL shapes, outputs, or
  transports.
- Version bump, CHANGELOG, release notes, tag, release, or publication.

## Acceptance Evidence

This record remains Proposed until implementation and independent review prove:

- all six identifiers and both PostgreSQL build variants carry correct native
  Go deprecation notices;
- current EN/ZH guidance is unified-entry-first and migration instructions are
  accurate;
- exported signatures and all old runtime/error/privacy contracts are
  unchanged;
- default/tagged tests and repository gates pass with no API, transport,
  capability, or release-surface expansion; and
- no unresolved P0, P1, or P2 finding remains.
