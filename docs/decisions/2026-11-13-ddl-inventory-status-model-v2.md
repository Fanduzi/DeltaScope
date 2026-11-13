# Decision: official DDL inventory status model v2 — parser/audit/applicability separation (issue #81 R3)

Date: 2026-11-13
Status: Accepted
Related milestone/version: issue #81 T02 R3 under parent issue #79
Related commits: T02 R3 inventory-model commit (this change set)
Related tests: `TestDDLInventoryContract`, `TestDDLInventoryDenominatorLocked`, `TestDDLInventoryAggregateDerivation` (`internal/application/audit/ddl_inventory_contract_test.go`)
Related docs: `testdata/ddl-inventory/README.md`, `testdata/ddl-inventory/matrix.json`

## Context

The v1 inventory stored a single row-level `status`. Reviewer R2 finding:
one scalar collapses three independent questions — can the parser read the
form, what does the auditor do with it, and on which documented/product
versions does that answer hold. Collapsed rows could not express the T26
version-gated domains (SERVER audited only on registered MySQL 8.0.x), the
T25 promotions (tablespace family `parser_error` → finding-covered), or
mixed per-subaction behavior (`ALTER TABLE` options: audited, silent, and
notice-only subactions on one row).

## Decision

- Inventory schema v2 separates three axes per row: `parser_status`
  (`recognized`/`partial`/`unrecognized`), `audit_status`
  (`audited`/`notice_only`/`explicit_unsupported`/`silent`/
  `not_applicable`/`mixed`), and `applicability` — one cell per anchor
  recording `official_presence`
  (`documented`/`not_documented`/`not_assessed`), `product_scope`
  (`registered`/`version_gated`/`vendor_boundary`/`not_applicable`/
  `unknown`), cell-level parser/audit status, and `evidence` refs.
  Evidence is mandatory on every version cell.
- `subaction_status` records are required where subactions differ
  meaningfully (five underexpressive rows at migration:
  `mysql.alter-table.options`, `mysql.alter-table.online-ddl`,
  `tidb.alter-table.online-options`, `tidb.flashback-recover`,
  `tidb.alter-instance`, plus `mysql.alter-instance`). Each record has a
  stable `subaction_id`, parser/audit status, and complete cells.
- Row `status` is a derived aggregate, never authored independently.
  Frozen derivation: all registered applicable cells audited →
  `semantically_checked`; all notice-only → `generic_notice`; all
  explicit-unsupported → `explicit_unsupported`; all silent →
  `parse_only`; all relevant documented forms parser-unrecognized →
  `parser_unsupported`; all relevant product cells vendor-boundary →
  `vendor_not_supported`; mixed registered states → `mixed_coverage`.
  Two precedence rules: out-of-domain
  `version_gated`/`vendor_boundary`/`not_applicable` cells never downgrade
  a fully audited registered domain; parser-unrecognized documented forms
  take precedence over mixed registered states when deriving
  `parser_unsupported`.
- The gate enforces: 114 required row IDs (triple-checked against
  `required_rows.txt`), no duplicates, owners/sources/parser status/audit
  status present and inside declared vocabularies, every declared version
  covered by a cell, every version cell carrying evidence, complete
  subaction cells where records exist, and aggregate status matching the
  frozen derivation.
- `testdata/ddl-inventory/matrix.json` is generated from the inventory by
  `scripts/gen_ddl_inventory_matrix.py` — one authoritative source, no
  hand-maintained matrix.
- Version-domain boundaries are preserved, not widened: SERVER remains a
  registered MySQL 8.0.x domain; SRS remains observed-8.0.4+ with 8.4
  unregistered and 5.7 a boundary; `ALTER INSTANCE ROTATE` vs `RELOAD
  TLS` keep independent applicability; `CREATE LOGFILE GROUP` records an
  NDB engine boundary without inferring support from InnoDB tablespace
  behavior; `tidb.alter-instance` records current explicit-unsupported
  behavior while T32/#111 retains ownership.

## Public contract

No runtime/API surface changes — this is a test-data contract plus its
gate. `inventory.yaml` v2 is additive over v1 keys, so historical scripts
reading `id`/`status`/`versions`/`owner` still parse.

## Verification

`make ddl-inventory-gate` green on the migrated file: 114 rows,
distribution `semantically_checked:75, generic_notice:14,
parser_unsupported:9, vendor_not_supported:11, explicit_unsupported:3,
parse_only:1, mixed_coverage:1`; `TestDDLInventoryAggregateDerivation`
pins the frozen rules on synthetic cells.
