# Decision: Cross-Dialect PostgreSQL Offline DML Impact Contract

Date: 2026-08-30
Status: Accepted
Related issue: [#37](https://github.com/Fanduzi/DeltaScope/issues/37)
Implementation commits:
- `7427512a1435f46658126f2943317b73a4194925` — PostgreSQL normalized predicate extraction, tests, corpus contracts, and impact documentation
- `6f37a43fbce609160e088d03b2e4aad74b4fdfb7` — shared PostgreSQL mutation extraction assembly cleanup

## Context

The shared offline DML impact estimator already treated a normalized
single-table identifier equality as a one-row, low-risk, high-confidence
estimate. MySQL and TiDB predicate adapters emitted that normalized shape, but
the PostgreSQL adapter emitted only `HasWhere` and therefore returned
`shape_unknown` for equivalent `DELETE` and `UPDATE` statements. PostgreSQL
metadata-aware audits already obtain a planner estimate through `EXPLAIN`.

## Decision

The PostgreSQL DML adapter emits the same parser-neutral predicate contract as
the existing MySQL/TiDB heuristic:

- a single-table `id =` literal or parameter equality emits
  `PredicateShapeUniqueEquality`, `LookupColumns=["id"]`,
  `MatchedKeyName="PRIMARY"`, and `MatchedKeyKind=primary`;
- reversed equality operands are accepted;
- the shared offline estimator produces `estimated_rows=1`, low risk, high
  confidence, `source=shape`, and `reason_codes=["pk_equality"]`;
- live PostgreSQL planner output remains `source=plan` and takes precedence
  over the shape estimate.

The adapter remains intentionally bounded. Non-equality, `OR`, range,
missing-`WHERE`, unrecognized-column, and other unproven predicates retain
their existing conservative behavior. MySQL/TiDB production behavior is not
changed.

## Public Contract

Equivalent offline PostgreSQL requests expose the existing statement-level
impact object through the SDK, CLI JSON, HTTP JSON, and MCP structured output.
The object uses the existing fields: `estimated_rows`, `estimated_ratio`,
`risk_level`, `confidence`, `source`, `reason_codes`, and optional `notes`.
No new command, rule, threshold, metadata requirement, or public output field
is introduced.

## Deferred / Out Of Scope

- arbitrary unique-key inference or metadata-free uniqueness claims beyond the
  existing `id` heuristic;
- compound predicates, range selectivity, OR selectivity, casts, functions,
  and other expression-aware row-count estimates;
- executing DML, `EXPLAIN ANALYZE`, or changing PostgreSQL planner behavior;
- changing MySQL/TiDB predicate extraction or impact thresholds;
- adding a separate impact subcommand.

## Verification Evidence

- PostgreSQL parser tests cover literal and `$1` equality, reversed operands,
  non-equality, OR, range, missing-WHERE, and unrecognized-column shapes.
- Application tests cover offline shape output and planner-source precedence.
- Corpus fixtures cover a PostgreSQL `$1` positive case and an OR unknown case.
- Tagged SDK, CLI, HTTP, and MCP tests assert the same positive impact fields.
- Existing MySQL/TiDB corpus and unit tests remain green.

Related implementation files and tests:

- `internal/infrastructure/parser/postgresql/extractor_dml.go`
- `internal/infrastructure/parser/postgresql/extractor_dml_postgresql_tag_test.go`
- `internal/application/audit/impact_postgresql_tag_test.go`
- `internal/application/audit/corpus_postgresql_tag_test.go`
- `pkg/deltascope/audit_postgresql_tag_test.go`
- `internal/interfaces/cli/cli_impact_postgresql_tag_test.go`
- `internal/interfaces/http/audit_impact_postgresql_tag_test.go`
- `internal/interfaces/mcp/audit_impact_postgresql_tag_test.go`
- `testdata/sql-corpus/postgresql/dml/supported/delete_pk_equality.sql`
- `testdata/sql-corpus/postgresql/dml/boundary/delete_or_predicate.sql`

## Consequences

The offline impact contract is now dialect-consistent at the normalized AST
boundary. Future dialect adapters must emit the same bounded shape facts before
the shared estimator is expanded. The explicit boundary keeps unknown shapes
fail-closed while allowing planner-backed PostgreSQL estimates to remain
authoritative.

## Links

- Exact implementation commits:
  - `7427512a1435f46658126f2943317b73a4194925`
  - `6f37a43fbce609160e088d03b2e4aad74b4fdfb7`
- User-facing impact references: `README.md`, `README_ZH.md`,
  `docs/reference/audit-capability-matrix.md`,
  `docs/reference/audit-capability-matrix.zh-CN.md`,
  `docs/reference/cli.md`, `docs/reference/http-api.md`,
  `docs/reference/library.md`, `docs/reference/library.zh-CN.md`
