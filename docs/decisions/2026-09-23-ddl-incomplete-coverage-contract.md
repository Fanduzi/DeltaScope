# Decision: Retained incomplete-coverage contract for recognized-but-unaudited DDL

Date: 2026-09-23
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (issue #79, task T03/#82)
Related commits: this task's commit on milestone/mysql-tidb-ddl-completion
Related tests: `internal/application/audit/coverage_t03_test.go`, `internal/interfaces/{cli,http,mcp}/audit_coverage_t03_test.go`, `make ddl-golden TASK=T03 ARTIFACT_DIR=...`
Related docs: `testdata/ddl-golden/T03.json`, `docs/decisions/2026-09-22-ddl-inventory-and-golden-baseline.md`

## Context

The milestone denominator (T02/#81) made silent passes visible: DDL the
parser recognized but rules never audited previously completed as `pass`
with no evidence. Under the all-rules-off policy, `CREATE SEQUENCE
golden_seq START WITH 1;` under MySQL exited 0 with verdict `pass` — MySQL
has no SEQUENCE feature (vendor boundary), yet the result claimed a clean
audit. Task T03 (#82) requires recognized-but-unaudited statements to
return an explicit incomplete audit result instead of silently passing.

## Decision

1. **Coverage is a capability fact on every result scope.**
   `report.Coverage{Status}` and the public `deltascope.Coverage` carry
   `complete` | `unverified` | `incomplete` on both `StatementResult` and
   `Result`. `Aggregate()` rolls statement coverage up (`incomplete` >
   `unverified` > `complete`). Coverage is computed from extraction facts
   and cannot be changed by policy configuration: disabling every finding
   rule never turns an incomplete statement complete. `unverified` is
   reserved for #83 (missing metadata/version evidence) and is not emitted
   by this task.
2. **Recognized-but-unsupported statements are retained, not dropped.**
   `EvaluateStatements` keeps them as top-level `StatementResult` entries
   with `coverage.status=incomplete`, preserving index, raw/normalized SQL,
   and source locations; they are excluded from rule evaluation while
   supported siblings still audit normally. This changes the prior
   PostgreSQL contract where unsupported statements were absent from
   `statements`; the retained contract is now uniform across dialects.
3. **Shared semantic classification, one place.** Boundary decisions live
   in `internal/infrastructure/parser/tidb/coverage_boundary.go`
   (parser-attached statement-level boundaries) and
   `internal/application/audit/coverage.go` (aspect gaps: unhandled ALTER
   sub-actions, unextracted options, non-audited constraint/index kinds).
   The MySQL/TiDB audited-aspect whitelist applies only to those dialects;
   PostgreSQL keeps its existing statement-level mechanism. CLI, HTTP, and
   MCP contain no boundary logic — they map the shared result and error.
4. **Vendor boundaries are product facts, not gaps.** MySQL `CREATE
   SEQUENCE`/`DROP SEQUENCE`, placement policies, and TiDB-side procedures,
   resource groups, and flashback/recovery/admin forms classify as
   `unsupported_statement` with stable feature IDs and bounded fixed
   reasons — not as parser errors and not as silently-passed statements.
   The same treatment covers every parser-recognized mutating or
   administrative statement the extractor does not model (DCL role/proxy
   lifecycle, stats/load/import, region/topology operations, session
   admin, procedure bodies): read-only queries and transaction/session
   statements stay outside the audit surface, but anything that mutates
   server or schema state without audited semantics reports incomplete.
   Inventory `status_evidence` records the observed classification per row.
5. **Verdict floor only.** Incomplete coverage floors `pass` to `review`;
   existing `review` and `reject` are never downgraded. Parser-error
   semantics are unchanged: genuine syntax failures keep `parser_error`
   diagnostics and CLI exit 2, and can coexist with unsupported diagnostics
   in one mixed batch.
6. **Golden runner gains `cli_cases` without weakening anchors.**
   `testdata/ddl-golden/T03.json` adds manifest `cli_cases` asserting
   expected exit codes and recomputed `coverage`/`unsupported` evidence
   from real CLI JSON across all four pinned anchors (5.7.44, 8.0.46,
   8.4.10, TiDB v8.5.0). The validator recomputes from raw stdout; recorded
   expected/parsed fields are corroborative only.

## Rationale

- A retained statement result preserves statement identity, order, and
  source locations — dropping unsupported statements would silently
  renumber the batch and lose the audit trail the milestone exists to
  expose.
- One shared classifier prevents three transport copies of the boundary
  from drifting; adapters only translate the result.
- Vendor boundaries differ from unaudited aspects: `CREATE SEQUENCE` under
  MySQL can never be audited (the product has no such feature), while a
  TiDB partition sub-action may be promoted by later tasks (#92, #101).
  Feature IDs name the deferred owner explicitly.
- The floor cannot weaken real findings: `reject` outranks the incomplete
  floor so a batch with a blocker finding stays `reject`.

## Public Contract

- `Result.coverage.status` and `StatementResult.coverage.status` are
  additive JSON fields with values `complete`/`unverified`/`incomplete`.
- Unsupported statements appear in `statements[]` with their original
  index, `raw_sql`, `normalized_sql`, empty findings, and
  `coverage.status=incomplete`; `Result.unsupported[]` carries
  `{index, feature, sql, reason}` per detail.
- SDK: `Audit` returns `ErrUnsupportedStatement` (wrappable via
  `errors.Is`) together with the populated partial `Result`.
- CLI: exit 1 for incomplete coverage even with `--fail-on none`; exit 2
  stays reserved for parser/user errors; JSON/markdown still render the
  partial result.
- HTTP: `400` with the partial result, coverage, diagnostics, and
  unsupported evidence in the error envelope.
- MCP: `isError=true` with the same structured partial result.
- Unsupported diagnostics never contain raw SQL fragments beyond the
  statement text field, passwords, parser internals, or option values.

## Deferred / Out Of Scope

- `coverage.status=unverified` (metadata/version evidence missing) is
  reserved for #83.
- Promoting deferred boundaries to audited semantics belongs to their
  owner tasks (e.g. #92 partition sub-actions, #100 sequences, #101
  resource groups, #103 MySQL resource groups); their expected fixtures
  and inventory `status_evidence` must be updated when they land.
- PostgreSQL boundary semantics beyond the retained-statement contract
  are unchanged.

## Verification Evidence

- `make ddl-golden TASK=T03 ARTIFACT_DIR=/tmp/ddl-golden`: 10 cases, 62
  assertions, PASS on all four anchors — `cli_cases` prove MySQL
  `CREATE SEQUENCE` + `ALTER TABLE ... ADD COLUMN` exits 1 with
  `coverage.status=incomplete`, bounded `create_sequence` evidence, and a
  `review` verdict under the all-rules-off policy.
- `make ddl-golden TASK=T02 ARTIFACT_DIR=/tmp/ddl-golden`: 10 cases, 57
  assertions, PASS — baseline behavior unregressed.
- `make ddl-golden-validator-test`: 24 contract cases — `cli_cases`
  coverage downgrade and wrong-feature tampering are rejected.
- `make ddl-inventory-gate`, `make sql-corpus-gates`, `make test`, and
  `make pg-unit-test-gates` all pass; PostgreSQL-tagged suites verify the
  uniform retained-statement contract.
- Focused transport tests: CLI exit 1 vs exit 2, HTTP 400, MCP
  `isError=true`, mixed reject>review, and no-leak assertions.

## Consequences

- Any future `statements[]` consumer must tolerate incomplete entries with
  empty findings; `summary.statements` counts retained unsupported
  statements.
- Promoting a boundary row to audited semantics requires updating its
  feature-ID expectation, corpus fixtures, census classifications, the
  coverage catalog, and inventory `status_evidence` in the same change.
- New parser-recognized statement types must be classified in
  `coverage_boundary.go` (boundary) or `coverage.go` (aspect gap) rather
  than letting them fall through silently.

## Links

- Commits: this task's commit; prerequisites #80 `1768f05`, #81 `b2956b3`
- Tests: `internal/application/audit/coverage_t03_test.go`, `internal/application/audit/coverage.go`, `internal/infrastructure/parser/tidb/coverage_boundary.go`, `internal/interfaces/{cli,http,mcp}/audit_coverage_t03_test.go`, `pkg/deltascope/audit_unsupported_verdict_floor_postgresql_tag_test.go`
- Docs: `testdata/ddl-golden/T03.json`, `testdata/ddl-inventory/inventory.yaml`, `docs/decisions/2026-09-22-ddl-inventory-and-golden-baseline.md`
