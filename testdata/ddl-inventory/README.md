# testdata/ddl-inventory

Machine-checkable official DDL acceptance inventory for the
mysql-tidb-ddl-completion milestone (issue #79). This is the versioned
denominator: what the official documentation actually offers, classified
honestly — not a count of implemented rules.

## Files

| File | Responsibility |
|------|----------------|
| inventory.yaml | The inventory: verified official sources, status vocabulary, owner tasks, proposed tasks, and one row per statement family/subaction scoped to the versions where it exists |

## Row contract

Every row carries: `id` (stable, unique), `product` (`mysql`/`tidb`),
`versions`, `sources`, `family`, `subactions`, `sql_shape`, `prerequisites`,
`status`, `status_evidence`, `targets`, `acceptance` (`dimensions`, `refs`),
`owner`.

Statuses (mutually exclusive, never substitutable):

- `semantically_checked` — feature-specific check exists with concrete
  evidence (`file:`/`gate:` acceptance ref required)
- `generic_notice` — parses but only generic/`*.notice` findings fire
- `parse_only` — parses silently, zero findings
- `parser_unsupported` — parser_error on a vendor-valid form
- `vendor_not_supported` — product boundary, not a to-do item

`acceptance.refs` entries are `file:<repo-path>` (must exist),
`gate:<make-target>`, or `missing:<description>` for honest gaps.

`owner` is a declared milestone task (`owners`) or a justified
`proposed_tasks` entry under #79. Rows owned by future tasks may record
incomplete implementation — they are assigned, not counted as coverage.

## Gate

`make ddl-inventory-gate` runs `TestDDLInventoryContract`
(`internal/application/audit/ddl_inventory_contract_test.go`).

Verified sources: MySQL 5.7 (frozen Oracle mirror — the dev.mysql.com 5.7
URL redirects to 9.7), MySQL 8.0, MySQL 8.4, TiDB release-8.5 docs.
Verification date: 2026-09-22.
