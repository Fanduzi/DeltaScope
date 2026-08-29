# Decision: MySQL/TiDB DML Target-Table Existence Blocker

Date: 2026-08-30
Status: Accepted
Related milestone/version: issue #46
Related commits:
- [DML target-table existence implementation](https://github.com/Fanduzi/DeltaScope/commit/bb6a95b0671973adaf7409ec4aacd5690b5f18a0)
- [Metadata schema-scope correction](https://github.com/Fanduzi/DeltaScope/commit/18adebf8fb5330192f40c1f69a0bd7e5ea23ecc8)
- [DML mutation-target extraction correction](https://github.com/Fanduzi/DeltaScope/commit/761dc7e487e53def75f53720611280c0e6aee258)
- [DML mutation-target expectation synchronization](https://github.com/Fanduzi/DeltaScope/commit/6e9381869d4f9071a69a3d1d4adc162848b35abe)
Related tests:
- `internal/domain/rule/dml/metadata_rules_test.go`
- `internal/application/audit/dml_table_existence_test.go`
- `pkg/deltascope/audit_dml_table_existence_test.go`
- `internal/interfaces/cli/audit_dml_table_existence_test.go`
- `internal/interfaces/http/audit_dml_table_existence_test.go`
- `internal/interfaces/mcp/audit_dml_table_existence_test.go`
- `testdata/sql-corpus/{mysql,tidb}/dml/metadata/`
Related docs:
- `docs/reference/rules.md`
- `docs/reference/config.md`
- `docs/reference/audit-capability-matrix.md`

## Context

Metadata-aware MySQL/TiDB `INSERT`, `UPDATE`, and `DELETE` audits loaded a
target-table snapshot, but the DML registry had no rule that consumed a
provider-confirmed `Exists: false` result. A missing target therefore passed
cleanly. DDL existence rules already established the repository's snapshot
boundary, while offline audits intentionally have no live existence evidence.

## Decision

Ship one default-enabled blocker rule: `dml.table.exists.require`.

The rule applies only to MySQL/TiDB DML operations and emits one stable finding
when `Metadata.TargetTable` is non-nil and `Exists` is false. Existing tables,
nil snapshots, schema-only metadata, offline requests, and PostgreSQL requests
produce no finding. Provider errors are returned by metadata enrichment and
are never converted into an absence finding.

For MySQL/TiDB DML, a qualified mutation target supplies the metadata lookup
schema; an unqualified target uses the schema resolved by the metadata
preparation flow. Snapshot caching is keyed by schema and table. INSERT target
extraction, explicit multi-table DELETE targets, and assignment-qualified
UPDATE targets are preserved. The rule evaluates one resolved target and fails
closed for ambiguous multi-target mutations. Existing DDL and PostgreSQL
metadata schema selection remains unchanged. `INSERT ... SELECT` source tables
are not checked.

## Rationale

The existing `TableSnapshot` contract already distinguishes a provider-confirmed
missing relation (`Exists: false`) from unavailable metadata (`nil` snapshot or
an error). Reusing it avoids a second metadata status type and keeps the rule
logic in the shared DML registry. A single rule ID keeps findings, policy,
catalog discovery, and every public surface aligned.

The explicit MySQL/TiDB dialect gate is required because the stable `dml.*`
ID family otherwise defaults to common catalog scope, while PostgreSQL DML
existence expansion is outside this decision.

## Public Contract

- `dml.table.exists.require` is enabled by default at `blocker` level and has no parameters.
- Missing-target findings use the stable `rule_id`, `level`, `message`, `suggestion`, and bounded `metadata.table` / `metadata.exists` fields.
- The rule is discoverable through the rule catalog and config inventories as a MySQL/TiDB metadata-aware DML rule.
- SDK, CLI, HTTP, and MCP audit results expose the same rule ID and finding shape.
- Offline audits do not claim that a target exists or is absent; metadata lookup failures retain the existing metadata/connection error path.

## Deferred / Out Of Scope

- PostgreSQL DML target-table existence checks
- Changes to DDL existence rules
- Existence checks for source tables inside `INSERT ... SELECT`
- Offline catalog guesses or file-based metadata snapshots
- New public provider methods or rule parameters

## Verification Evidence

Focused domain, application, SDK, CLI, HTTP, and MCP tests cover missing and
existing targets, offline no-claim behavior, qualified/unqualified schema
selection, source-table exclusion, and provider error propagation. MySQL/TiDB
metadata corpus fixtures cover INSERT/UPDATE/DELETE missing targets and an
existing base table for both dialects. The SQL corpus inventory reports
`policy_rule_ids=372`, `covered_rule_dialect_targets=584`, and `coverage_percent=100.0`.
`make test`, PostgreSQL-tagged unit tests, race tests, vet, targeted lint, docs
examples, and DDL census checks pass. Docker-backed live metadata suites are
wired with the same cases and remain to be run under the repository's
serialized Docker orchestration gate.

## Consequences

Future MySQL/TiDB DML metadata rules should treat a nil snapshot as unknown and
must preserve provider errors. Changes to target extraction or schema
resolution must retain the target/source distinction and update the corpus,
catalog, capability matrix, and public-surface contracts together. A future
PostgreSQL or offline existence decision should use a separate ADR rather than
silently widening this rule.

## Links

- Issue: https://github.com/Fanduzi/DeltaScope/issues/46
- Commits: [bb6a95b0671973adaf7409ec4aacd5690b5f18a0](https://github.com/Fanduzi/DeltaScope/commit/bb6a95b0671973adaf7409ec4aacd5690b5f18a0), [18adebf8fb5330192f40c1f69a0bd7e5ea23ecc8](https://github.com/Fanduzi/DeltaScope/commit/18adebf8fb5330192f40c1f69a0bd7e5ea23ecc8), [761dc7e487e53def75f53720611280c0e6aee258](https://github.com/Fanduzi/DeltaScope/commit/761dc7e487e53def75f53720611280c0e6aee258), [6e9381869d4f9071a69a3d1d4adc162848b35abe](https://github.com/Fanduzi/DeltaScope/commit/6e9381869d4f9071a69a3d1d4adc162848b35abe)
- Tests: `internal/infrastructure/parser/tidb/extractor_test.go`, `internal/application/audit/dml_table_existence_test.go`, `pkg/deltascope/audit_dml_table_existence_test.go`, `internal/interfaces/{cli,http,mcp}/audit_dml_table_existence_test.go`
- Docs: `docs/reference/{rules,config,audit-capability-matrix}.md` and their Chinese counterparts
