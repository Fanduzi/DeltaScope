# Decision: Official DDL acceptance inventory and four-version golden baseline

Date: 2026-09-22
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (issue #79, task T02/#81)
Related commits: (this task's commit on milestone/mysql-tidb-ddl-completion)
Related tests: `TestDDLInventoryContract`, `scripts/test_ddl_golden.py`, `make ddl-golden TASK=T02 ARTIFACT_DIR=...`
Related docs: `testdata/ddl-inventory/inventory.yaml`, `testdata/ddl-golden/T02.json`, `docs/dev/testing.md`, `docker/README.md`, `docs/research/2026-09-21-mysql-tidb-ddl-official-baseline.md`

## Context

Issue #79 requires the milestone's DDL denominator to come from versioned
official documentation, not from the count of already-implemented rules
(586/586 fixture metrics cannot reveal DDL that never entered a fixture).
Task T02 (#81) therefore had to establish two things before any semantic work:
a reproducible real-database baseline proving fixture legality on the exact
target versions, and an official acceptance inventory where every row is
classified and owned.

## Decision

1. **Four pinned anchors in a dedicated compose file.** `docker/ddl-golden-compose.yaml`
   pins `mysql:5.7.44`, `mysql:8.0.46`, `mysql:8.4.10`, `pingcap/tidb:v8.5.0`.
   The pre-existing builtin matrix pins TiDB `v8.5.7`; a separate file was
   chosen over re-tagging it because the builtin matrix serves different
   evidence and its pin is deliberate. `mysql:5.7.44` ships amd64 only, so it
   runs under `platform: linux/amd64` emulation — recorded as an environment
   fact, not hidden. No host ports are published; access goes through
   `docker exec`/the compose network, and the runner owns deterministic
   `down -v` cleanup on every path.
2. **`make ddl-golden TASK=<id> ARTIFACT_DIR=<dir>` as a thin runner.**
   `scripts/ddl_golden.py` reads a per-task manifest (`testdata/ddl-golden/`),
   builds the checkout CLI into the artifact dir, brings the anchors up,
   executes per-statement database cases with live `information_schema`
   assertions, runs syntax-negative cases (concrete `errno`/`syntax` class
   required), runs CLI static-audit cases with a temporary all-rules-off
   policy, then writes and self-validates `artifact.json`. The validator
   fails — never skips — on missing anchors, unreachable/unhealthy/
   version-mismatched databases, unexecuted required cases, stale binaries,
   absent expected fields, zero cases, or hand-written PASS records; external
   blockers are recorded as `external_blocker` violations in a real artifact.
3. **`testdata/ddl-inventory/inventory.yaml` as the machine-checkable
   denominator.** Each row binds a statement family/subaction to product
   versions where it exists, verified official sources, observed current
   status, preserved objects/semantics, acceptance dimensions, and an owner.
   Five mutually exclusive statuses: `semantically_checked` (feature-specific
   check + cited evidence), `generic_notice` (only generic/`*.notice`
   findings fire), `parse_only` (silent pass), `parser_unsupported`
   (parser_error on a vendor-valid form), `vendor_not_supported` (product
   boundary). `TestDDLInventoryContract` enforces ID uniqueness, required
   fields, per-version sources, status vocabulary, owner assignment, and
   concrete evidence refs for `semantically_checked`.
4. **Owners are real milestone tasks; genuine gaps get a real issue.**
   Rows map to T01–T31 where they fit. One real gap — TiDB
   instance-management DDL (e.g. `ALTER INSTANCE RELOAD TLS`) — fit no
   existing task, so issue #111 (task T32) was created under #79, blocked by
   #82 and required by #110 closure, instead of silently expanding an
   unrelated owner. Owners must resolve to a declared task with a real issue
   number; a proposal without an issue is not an owner.
5. **MySQL 5.7 official source is the frozen Oracle mirror.**
   `https://dev.mysql.com/doc/refman/5.7/en/sql-data-definition-statements.html`
   silently redirects to the MySQL 9.7 manual; the inventory's verified 5.7
   source is `docs.oracle.com/cd/E17952_01/mysql-5.7-en/...`.

## Rationale

- Database execution and CLI static audit prove different things (fixture
  legality vs. parser/audit behavior); the runner keeps them as distinct case
  kinds with separate assertions instead of one blended result.
- Statuses must not substitute for each other: a `.notice` finding is not a
  semantic check, a silent pass is not support, and a vendor boundary is not
  a to-do item. Probe evidence (2026-09-22 CLI runs) distinguishes these
  empirically per row.
- A validator that can be tricked is worse than none: the contract suite
  proves every rejection path fails, including required-DB-unreachable,
  required-case-missing, deleted metadata-query records, failed metadata
  return codes, non-JSON CLI stdout, and artifact-internal expected/actual
  tampering.
- The inventory counts rows honestly: 113 rows, most incomplete — the
  milestone denominator now exists without claiming unimplemented coverage,
  and `required_row_ids` makes row deletion a gate failure.

## Public Contract

- `make ddl-golden TASK=T02 ARTIFACT_DIR=<dir>` produces a validated
  `artifact.json` with top-level `task_id`, `head_sha`, `generated_at`,
  `cli` (`path`, `sha256`, `build` incl. `cgo_enabled`/`go_version`/
  `head_sha`), `policy_profile`, `required_case_ids`, `cases`,
  `executed_count`, and `cleanup`. Per-case fields are `case_id`, `kind`,
  `expected`, `actual`, `assertions`, `status`; raw evidence differs by kind:
  - `db_ddl`: `anchor`, `input_sql`, `actual.database`
    (`product`/`image`/`image_digest`/`container`/`reachable`/`version`),
    `actual.steps[]` (`name`/`sql`/`rc`/`stdout`/`stderr`/`verify[]` with
    per-query `assert`/`sql`/`rc`/`output`/`stderr`)
  - `db_syntax_negative`: `anchor`, `input_sql`, `actual.rc`/`stdout`/
    `stderr`/`error_class`
  - `cli_audit`: `dialect`, `input_sql`, `policy_profile`, `command`,
    `actual.exit`/`stdout`/`stderr`/`parsed`
- The validator recomputes expectations from the task **manifest** and
  results from **raw evidence** (`actual.steps[].verify[]` rc/output, raw
  `stderr`/`stdout`, reparsed CLI stdout) — artifact-recorded `expected`,
  `parsed`, and `assertions` are corroborative only and any disagreement with
  the manifest-derived expectation or raw evidence is a tamper failure.
- `make ddl-golden-validator-test` and `make ddl-inventory-gate` run offline.
- The inventory YAML schema (statuses enum, `file:`/`gate:`/`missing:` ref
  kinds, `owners` bound to real milestone issues, `required_row_ids`
  tamper-evident denominator) is the contract later tasks extend.

## Deferred / Out Of Scope

- No audit-rule changes, parser upgrades, or new semantic checks — this task
  establishes the denominator and baseline only.
- No `target_version`, `coverage.status`, Prospective Schema State, or
  stored-body analysis (later tasks T03–T05, T27–T30).
- Task T32 (issue #111) is an owner of record, not an implementation.
- PostgreSQL rows are out of scope (separate milestone coverage model).

## Verification Evidence

- `make ddl-golden TASK=T02 ARTIFACT_DIR=/tmp/ddl-golden`: 10 cases, 57
  assertions, PASS — 4 anchors × (CREATE+metadata, ALTER+metadata,
  DROP+metadata), 4 syntax-negative cases (errno 1064), 2 CLI audits
  (verdict pass, exit 0, 3 statements).
- `make ddl-golden-validator-test`: 18/18 cases — every rejection path fails,
  including deleted metadata-query records, failed verify return codes,
  non-JSON CLI stdout, recorded-parsed vs raw-stdout disagreement, and
  artifact-internal expected tampering.
- `make ddl-inventory-gate`: 113 rows, all classified and assigned; the
  row set equals `required_row_ids` exactly (verified by truncating rows to
  one and observing gate failure).
- Server-reported versions: 5.7.44, 8.0.46, 8.4.10, 8.0.11-TiDB-v8.5.0.

## Consequences

- Future tasks must extend the inventory rows (status, evidence refs) rather
  than invent parallel tracking; `semantically_checked` requires concrete
  file/gate evidence by gate rule.
- New golden-path task manifests must keep the same case-kind contract; the
  validator stays strict on anchors, binaries, and executed sets.
- The 5.7 mirror URL is the only trustworthy 5.7 source — future versioned
  sources must be checked for silent redirects before use.
- If a milestone task retires a row's `parse_only`/`generic_notice` status,
  the evidence refs must be updated in the same change.

## Links

- Commits: this task's commit; #80 correction at `1768f05`
- Tests: `internal/application/audit/ddl_inventory_contract_test.go`, `scripts/test_ddl_golden.py`
- Docs: `docs/dev/testing.md`, `testdata/ddl-inventory/inventory.yaml`, `docker/README.md`, `scripts/README.md`
