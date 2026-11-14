# Decision: keep `/*T![ttl] ... */` visible to the MySQL audit surface

Date: 2026-11-22 — milestone decision-record convention: decision dates
reflect the commit date, not the issue-open date.
Status: Accepted
Related milestone/version: MySQL/TiDB DDL semantic acceptance (issue #79, task T19 / issue #98)
Related commits: this commit; see also `7bf6e257` (T20-R2 feature-comment blanking) and `f00b3a7d` (T19 TTL lifecycle)

## Context

MySQL has no TiDB feature-comment syntax. The server treats `/*T! ... */`
as an ordinary comment and ignores its contents, while the shared TiDB
parser resolves it into AST nodes. T20-R2 introduced
`blankTiDBFeatureComments` so MySQL placement comments such as
`/*T![placement] PLACEMENT POLICY = p */` stay an honest silent no-op
instead of being audited as real configuration.

That same blanking erased `/*T![ttl] TTL_ENABLE='ON' */` before the parser
saw it. Under T19 a TTL clause is an authored automatic-deletion
configuration; letting it vanish into a silent pass would conceal exactly
the deletion-risk surface the milestone must expose, and broke the frozen
`mysql-comment-noop` vendor-boundary expectation (exit 1, `review`,
one bounded diagnostic, one `alter_table.table_option.ttl.*` unsupported
entry, `coverage=incomplete`).

## Decision

`blankTiDBFeatureComments` preserves `/*T![ttl] ... */` regions verbatim
(case-insensitive tag match); every other `/*T!` family stays blanked.
On MySQL the surviving TTL clause resolves as a feature-comment TTL
declaration and is reported as a vendor boundary: recognized-but-inert,
unsupported entry `alter_table.table_option.ttl.*`, bounded diagnostic,
`verdict=review`, `coverage=incomplete`. On TiDB the comment executes and
the declaration flows through the normal TTL outcomes unchanged.

## Rationale

Placement semantics under MySQL are only ever a no-op, so blanking is the
honest model there. TTL is different: ignoring it does not merely skip an
unused option — an operator reading `TTL_ENABLE='ON'` inside a comment can
reasonably believe automatic deletion is armed. Surfacing the clause as a
vendor boundary preserves the distinction between "server ignored it" and
"auditor never saw it", without fabricating a finding the server would
never act on.

## Public Contract

- `/*T![ttl]` is the only `/*T!` family guaranteed to survive MySQL
  blanking; all other feature comments remain same-width whitespace.
- Byte offsets, line numbers, `RawSQL`, and extraction offsets are
  unchanged — preserved regions occupy their original bytes.
- Consumers can rely on `unsupported` entries with feature
  `alter_table.table_option.ttl.*` (ALTER) or `create_table.option.ttl.*`
  (CREATE) plus one diagnostic per entry on MySQL dialects, and on the
  normal TTL outcomes on TiDB.

## Deferred / Out Of Scope

- No attempt to model MySQL-ignored TTL in prospective state beyond the
  existing unproven/evidence-gap paths in `batch_state_ttl.go`.
- Other `/*T![tag]` families keep T20 semantics; promoting another family
  requires its own decision record.

## Verification Evidence

- `TestT19MySQLTTLFeatureCommentVendorBoundary` — RED before the fix,
  GREEN after: vendor boundary on MySQL, executable evidence on TiDB,
  placement comment still ignored, ordinary comments never fabricate TTL.
- Golden `testdata/ddl-golden/T19.json` cases `cli.mysql-comment-noop`
  and `meta.mysql80-comment-noop`: 25/25 cases, 321 assertions on the
  final HEAD.
- `make test`, `make pg-unit-test-gates`, `go vet ./...` clean.

## Consequences

- `mysql_feature_comments.go` now carries a tag-family allowlist; future
  `/*T!` families default to blanked and must opt in deliberately.
- The T20 placement no-op contract is untouched and remains covered by
  T20 golden cases.

## Links

- Issue: <https://github.com/Fanduzi/DeltaScope/issues/98>
- Code: `internal/application/audit/mysql_feature_comments.go`
- Tests: `internal/application/audit/batch_state_t19_test.go`
- Prior decision: `docs/decisions/2026-11-11-tidb-ttl-lifecycle.md`
