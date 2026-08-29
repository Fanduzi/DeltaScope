# Decision: Set the Source-Readiness Target to 85

Date: 2026-08-30
Status: Accepted
Related issue: [#53](https://github.com/Fanduzi/DeltaScope/issues/53)
Tested commit: [`23c02fc09e0e3e6682114f944077e94a79d4e847`](https://github.com/Fanduzi/DeltaScope/commit/23c02fc09e0e3e6682114f944077e94a79d4e847)

## Decision

Use **85/100 source readiness** as the milestone target. This is a statement
about the named source commit, not about the currently published package.

The source milestone scores **85/100** after the DBA, application-development,
and CI dogfood described below. Published `v0.490.0` remains **58/100** because
it predates these changes. No release-readiness or installation recommendation
is made until a later published release is built and verified from this work.

## Ordered Scope

The target was implemented as a finite dependency-ordered set of existing
issues rather than adding behavior to the meta issue:

1. Baseline correctness and delivery: #38, #52, #42, #31, #32, #35, #40.
2. Input and cross-surface contracts: #44, #45, #36, #48, #41.
3. Parser, Query Access, and PostgreSQL boundaries: #43, #49, #39, #34,
   #33, #37.
4. Metadata-aware MySQL/TiDB rules: #46, then #47.
5. Representative corpus expansion: #50 after #47/#49, and #51 after #43.

The implementation dependencies are now satisfied in the tested milestone
commit. Docker-backed gates remain globally serialized because their Compose
projects and container names are shared.

## Fresh Dogfood Evidence

### MySQL DBA

- The production-shaped `BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY`
  table with `InnoDB` and `utf8mb4` returned exit 0, one statement, and zero
  `ddl.table.primary_key.require` findings.
- The MySQL corpus now has 32 SQL files and separate fixtures for realistic
  table options, JSON/generated columns, RANGE/LIST partitioning, ALTER
  `ALGORITHM`/`LOCK`, MODIFY nullability, ADD INDEX, and UTF-8 BOM input.
- Live MySQL and TiDB CLI, HTTP, and MCP metadata suites passed missing/existing
  DML target checks and MODIFY restatement/transition checks.

### Application Developer

- Explicit empty Query Access SQL returned exit 3 immediately with
  `SQL input must not be empty`; stdin was not consulted.
- Query Access rejects audit-only `--format` with exit 3 instead of silently
  ignoring it. This is the accepted flag-ownership contract from #36.
- A migration with a parser error in the middle returned exit 2 while retaining
  two valid statement results and one located diagnostic.
- A valid file with one leading UTF-8 BOM returned exit 0 and one statement.
- PostgreSQL Query Access advertises its PG17 boundary, and the live PG17 CLI,
  HTTP, and MCP confidence suites passed database/schema selection and plan,
  existence, and object checks.

### CI and Corpus

- The pull-request workflow contract test confirms the stable MySQL/TiDB SQL
  corpus gate is required without a step or job guard.
- `make sql-corpus-gates`, both Query Access corpus gates, `make test`,
  `make lint`, `make build`, `make docs-example-gates`, npm launcher tests,
  release-surface gates, and the full PostgreSQL confidence gate passed.
- PostgreSQL has 230 SQL corpus files, including concurrent index/materialized
  view forms, modern CREATE TABLE shapes, and mixed partial migrations.

## Score Boundary

The 85 score reflects a usable source milestone with the reproduced product
holes closed and representative gates running. It is not an SLA or an
automated metric, and it does not replace the separate published score: the
public CLI and npm package still report `v0.490.0`.

The published score may be raised only after the repository release workflow
produces a new version from an exact reviewed commit and its assets, checksums,
install paths, and binary version smoke tests pass. That action requires a
separate authorized release using the repository `go-release` skill.

## Deferred / Out of Scope

- Publishing, tagging, pushing, or changing the current release in this task.
- Treating the representative corpus as official grammar coverage.
- Converting the subjective readiness score into a CI quality metric or SLA.
- Duplicating implementation already owned by issues #31 through #52.

## Verification Commands

- `make test`
- `make lint`
- `make build`
- `make sql-corpus-gates`
- `make query-access-corpus-gates`
- `make pg-confidence-gates`
- `make docs-example-gates`
- `make release-surface-gates`
- `npm test --prefix packages/deltascope-mcp`
- `python3 -m unittest scripts/test_pr_workflow_contract.py`
