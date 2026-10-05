# testdata/ddl-inventory

Machine-checkable official DDL acceptance inventory for the
mysql-tidb-ddl-completion milestone (issue #79). This is the versioned
denominator: what the official documentation actually offers, classified
honestly — not a count of implemented rules.

## Files

| File | Responsibility |
|------|----------------|
| inventory.yaml | The inventory: verified official sources, status vocabulary, owner tasks (real issues), the `required_row_ids` denominator, and one row per statement family/subaction scoped to the versions where it exists |
| required_rows.txt | Locked denominator baseline — one row ID per line, checked in independently of `inventory.yaml`. The gate requires `rows` == `required_row_ids` == this file, so shrinking both YAML fields together still fails |
| T06-base-traceability.md | Bounded T06-A1 (#85) traceability table: maps each CREATE TABLE acceptance dimension on `mysql.create-table`/`tidb.create-table` to normalized spec fields, consuming rules, and concrete evidence; records honest gaps deferred to later T06 slices and other tasks |

## Row contract

Every row carries: `id` (stable, unique), `product` (`mysql`/`tidb`),
`versions`, `sources`, `family`, `subactions`, `sql_shape`, `prerequisites`,
`status`, `status_evidence`, `targets`, `acceptance` (`dimensions`, `refs`),
`owner`.

`required_row_ids` lists every row ID and must equal the row set exactly —
deleting a row without updating the denominator fails the gate. The checked-in
`required_rows.txt` baseline holds the same set outside the editable YAML, and
the gate additionally locks per-product minimums (mysql ≥ 63, tidb ≥ 50) in the
test itself — shrinking any pair of files leaves the third behind and fails.

`owner` must be a declared milestone task with a real GitHub issue number
(`owners` map). A gap with no owning task means a new child issue under #79
must be created first — a proposal without an issue is not an owner.

Statuses (mutually exclusive, never substitutable):

- `semantically_checked` — feature-specific check exists with concrete
  evidence (`file:`/`gate:` acceptance ref required)
- `generic_notice` — parses but only generic/`*.notice` findings fire
- `parse_only` — parses silently, zero findings
- `parser_unsupported` — parser_error on a vendor-valid form
- `vendor_not_supported` — product boundary, not a to-do item

`acceptance.refs` entries are `file:<repo-path>` (must exist),
`gate:<make-target>`, or `missing:<description>` for honest gaps.

Rows owned by future tasks may record
incomplete implementation — they are assigned, not counted as coverage.

## Gate

`make ddl-inventory-gate` runs `TestDDLInventoryContract` plus the
denominator-lock negative `TestDDLInventoryDenominatorLocked`
(`internal/application/audit/ddl_inventory_contract_test.go`).

Verified sources: MySQL 5.7 (frozen Oracle mirror — the dev.mysql.com 5.7
URL redirects to 9.7), MySQL 8.0, MySQL 8.4, TiDB release-8.5 docs.
Verification date: 2026-09-22.
