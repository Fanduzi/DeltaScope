# Decision: T06-A1 CREATE TABLE primary-key golden proof path

Date: 2026-10-06
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (#79), task T06 / issue #85
Related commits: T06-A1 implementation commit on milestone/mysql-tidb-ddl-completion
Related tests: `internal/application/audit/batch_state_t06a1_test.go`, `pkg/deltascope/audit_create_table_pk_t06a1_test.go`, `internal/interfaces/http/audit_create_table_pk_t06a1_test.go`, `scripts/test_ddl_golden.py` T06 contract group
Related docs: `testdata/ddl-golden/T06.json`, `testdata/ddl-inventory/T06-base-traceability.md`, `docs/decisions/2026-08-30-mysql-tidb-column-primary-key-extraction.md`

## Context

Issue #85 asks for CREATE TABLE base semantics with evidence that separates
"the database accepts this DDL" from "the team's policy accepts this DDL".
The A0 baseline showed `ddl.table.primary_key.require` already behaves
correctly offline (no-PK → blocker/reject/exit 1; inline and table-level PK →
pass; rule off → pass), so there was no product defect to fix. The missing
piece was a standardized, replayable proof entry: a T06 golden manifest, live
four-anchor execution evidence, and validator contract checks so corrupted or
shrunken evidence cannot pass.

## Decision

Introduce `testdata/ddl-golden/T06.json` as the T06-A1 proof entry with
exactly 32 cases: the 10 locked baseline cases (unchanged T02 SQL), 10
offline CLI controls (five shapes × mysql/tidb), and 12 anchored metadata
cases (no-PK / inline-PK / table-level-PK × mysql57/mysql80/mysql84/tidb85).
Each anchored case follows the fixed sequence: confirm absence → product
audit → confirm the product did not create the table → test-driver executes
the same CREATE → query real column/PK structure → teardown and confirm
absence. The validator gained a T06-specific contract that re-derives the
frozen oracle from the manifest and inspects raw stdout, so a paired
manifest/artifact shrink or a laundered result is rejected.

No production code changed: the rule, extractor, spec, coverage, state layer,
and transports are untouched. `instance_facts` gained three MySQL 8.0/8.4
GIPK reads (`sql_require_primary_key`,
`sql_generate_invisible_primary_key`,
`show_gipk_in_create_table_and_information_schema`) recorded per anchored
case.

## Rationale

- A policy `reject` must never substitute for native execution: every no-PK
  case runs the driver-side CREATE anyway and asserts success plus the real
  absence of a PRIMARY KEY, so legality and policy stay independent claims.
- MySQL 8.0/8.4 can auto-generate an invisible primary key
  (`sql_generate_invisible_primary_key`). Recording the live GIPK facts
  prevents a server-generated key from being mistaken for an input-declared
  PK.
- Freezing the oracle inside the runner (`t06_a1_contract`) and re-deriving
  expectations at validation time keeps the manifest/artifact pair honest —
  the same pattern established by the T05-A3/A4/A6 frozen contracts.

## Public Contract

Consumers can rely on: `ddl.table.primary_key.require` with
`required:true` produces exactly one blocker finding
(`primary key is required` / `add an explicit PRIMARY KEY declaration`,
`metadata.table`, location 1:1), verdict `reject`, coverage `complete`, CLI
exit 1 under `--fail-on blocker`; inline `PRIMARY KEY` and table-level
`PRIMARY KEY (...)` are equivalent declarations; `UNIQUE KEY` alone never
satisfies the rule; disabling the rule or setting `required:false` yields
`pass`. The created-table PK fact is carried into follower statements'
prospective pre-state (T05 layer).

## Deferred / Out Of Scope

This slice proves one dimension of `mysql.create-table`/`tidb.create-table`.
Column type-family matrices, version-conditional forms, LIKE/SELECT
lifecycle (T16), full table-option coverage (T15), CHECK semantics (T12),
index families (T09/T10), generated columns (T07), and TiDB-only options
(T19/T20/T23) remain with their owning tasks. `semantically_checked` on the
two inventory rows predates this slice and is unchanged; #85 stays open.

## Revision T06-A1-R1: identity binding (2026-10-06)

Review found that a required `case_id` was only a name: the T06-specific
checks selected the frozen spec by full ID, while the generic validator
dispatched on the record's self-declared `kind` and re-selected the manifest
spec by self-declared `cli_case`. A passing mysql84 record could therefore
occupy the `t06-tidb85-no-pk` slot, and an offline `cli_audit` record could
occupy a `cli_metadata` slot, with the required set still complete.

The validator now builds one frozen map from `t06_a1_contract` —
`case_id → kind / local id / dialect / anchor / policy profile / input SQL` —
and binds every executed record to it **before** kind-based dispatch, plus
rejects duplicate executed ids and duplicated or unfrozen manifest
declarations. An ID being present is not proof the right anchor and role
ran; the original `06de7475` artifact remains valid evidence — the artifact's
`head_sha` is unchanged and validator-fix commits are recorded separately.
Verified by the full validator entry on the original artifact plus 10 new
contract mutations (244 total).

## Verification Evidence

- `python3 scripts/test_ddl_golden.py` — 234 contract cases, 0 failures,
  including 28 T06-A1 mutations (policy path/sha tamper, product-side
  CREATE claimed rejected, removed absence re-check, native CREATE failure
  laundered, PK structure erased/renamed, member query deleted on both
  sides, no-PK column recorded NOT NULL, teardown removed, driver step
  skipped, connectivity stderr laundered, failed cleanup accepted, missing
  GIPK facts, finding/statement/summary/coverage tampering).
- `go test ./internal/application/audit -run T06A1` — five controls × two
  dialects, UNIQUE control, normalization, pre-state consumption.
- `go test ./pkg/deltascope ./internal/interfaces/http -run T06A1` — SDK and
  HTTP representative results.
- `make ddl-golden TASK=T06` against the four locked anchors (evidence
  recorded with the task evidence commit).

## Consequences

- Future CREATE TABLE slices must extend `T06.json` or add sibling manifests
  through the same contract path; the frozen oracle cannot be weakened
  without a visible diff in `ddl_golden.py`.
- GIPK handling is now a recorded fixture fact; any slice that asserts
  "table has PK" on MySQL 8.0/8.4 must account for server-generated keys.
- The traceability table (`T06-base-traceability.md`) is the honest status
  ledger for the two inventory rows and must be updated when later slices
  prove additional dimensions.

## Links

- Tests: `internal/application/audit/batch_state_t06a1_test.go`,
  `pkg/deltascope/audit_create_table_pk_t06a1_test.go`,
  `internal/interfaces/http/audit_create_table_pk_t06a1_test.go`
- Docs: `testdata/ddl-golden/T06.json`,
  `testdata/ddl-inventory/T06-base-traceability.md`,
  `testdata/ddl-golden/README.md`, `scripts/README.md`
