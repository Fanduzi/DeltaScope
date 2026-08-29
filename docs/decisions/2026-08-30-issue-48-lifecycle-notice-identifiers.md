# Decision: Render normalized identifiers in MySQL/TiDB lifecycle notices

Date: 2026-08-30
Status: Accepted
Related milestone/version: v0.490.0 / issue #48
Related commits: issue #48 implementation commit
Related tests:
  `pkg/deltascope/audit_ddl_lifecycle_mysql_test.go`
  `internal/domain/rule/ddl/`
Related docs:
  `internal/domain/rule/ddl/README.md`
  `pkg/deltascope/README.md`

## Context

The shared MySQL/TiDB lifecycle rule stores notice templates with `%q` but only
formats `DDL.ObjectName`. The TiDB extractor represents standalone `RENAME TABLE`
with `DDL.Table` and standalone `CREATE INDEX` with a nested `DDL.Alter` index
definition, leaving `ObjectName` empty. The unresolved template then propagates
through the shared finding into every output surface.

## Decision

Derive a message-only identifier in the shared lifecycle rule. Prefer
`DDL.ObjectName`; use the normalized qualified source table for `RENAME TABLE`,
the nested index definition for `CREATE INDEX`, and the old nested index name
for `DROP INDEX`. Format `%q` templates from those extracted values. If a
template has no available identifier, format an empty value so an unresolved
format verb cannot reach a renderer. Keep the existing metadata source and
shape unchanged.

## Rationale

The shared rule is the earliest common owner of the defect and covers all
renderers without parser or transport changes. Using normalized spec fields
preserves the existing bounded quoting behavior and avoids raw SQL, credentials,
or source-specific quoting bytes.

## Public Contract

MySQL/TiDB lifecycle findings name their extracted table or index identifier and
do not expose an unresolved `%q`. Rule IDs, levels, verdicts, statement indexes,
locations, and renderer-specific envelopes remain unchanged. JSON, Markdown,
CI, SARIF, GitLab, HTTP, MCP, and SDK consumers receive the same corrected
`Finding.Message`.

## Deferred / Out Of Scope

- No parser contract or source-quoting preservation change.
- No rule-ID, level, verdict, location, or wording redesign.
- No `ALTER TABLE ADD INDEX` reclassification; that remains issue #49.

## Verification Evidence

- Public SDK regression covers schema-qualified and quoted identifiers across
  multiple statements and asserts IDs, notice levels, locations, and no `%q`.
- `go test ./pkg/deltascope -run TestAuditMySQLDDLNoticeMessagesUseNormalizedIdentifiers -count=1`
- `go test ./internal/domain/rule/ddl -count=1`
- `make sql-corpus-gates`
- `make ddl-census-report`
- `make ddl-coverage-catalog-test`
- `make test`

## Consequences

Future standalone lifecycle extractors must populate the normalized spec fields
used by the shared message-identity fallback. Renderer changes are unnecessary
for this class of finding because they consume the shared message directly.

## Links

- Issue: https://github.com/Fanduzi/DeltaScope/issues/48
- Tests: `pkg/deltascope/audit_ddl_lifecycle_mysql_test.go`
- Code: `internal/domain/rule/ddl/mysql_tidb_lifecycle_rules.go`
