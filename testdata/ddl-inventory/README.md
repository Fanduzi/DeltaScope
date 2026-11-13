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
| matrix.json | Derived view of the inventory (`id`, `product`, `owner`, derived `status`, `parser_status`, `audit_status`, `applicability`, optional `subaction_status`). Regenerate with `python3 scripts/gen_ddl_inventory_matrix.py` after any inventory change; do not hand-edit |
| T06-base-traceability.md | Bounded T06 (#85) traceability table: maps each CREATE TABLE acceptance dimension on `mysql.create-table`/`tidb.create-table` to normalized spec fields, consuming rules, and concrete evidence; records accepted representation limits and other-task boundaries |
| T06-CLOSEOUT.md | Final T06 (#85) technical closeout index (T06技术收口材料齐备): binds every accepted dimension to the traceability rows, the SHA-frozen umbrella evidence, and the owner boundary for deferred dimensions; issue #85 stays open |
| evidence/ | Per-row offline CLI probe captures (`<row-id>.probes.txt`): recorded `deltascope audit` output for the row's `sql_shape` at every declared version (`=== version <v> (target …)` sections). Regenerate by re-running the probe commands recorded in the file header |

## Row contract

Every row carries: `id` (stable, unique), `product` (`mysql`/`tidb`),
`versions`, `sources`, `family`, `subactions`, `sql_shape`, `prerequisites`,
`status` (derived aggregate — never authored independently),
`parser_status`, `audit_status`, `applicability`, optional
`subaction_status`, `status_evidence`, `targets`, `acceptance`
(`dimensions`, `refs`), `owner`.

The v2 status model separates three axes:

- `parser_status`: `recognized` | `partial` | `unrecognized`
- `audit_status`: `audited` | `notice_only` | `explicit_unsupported` |
  `silent` | `not_applicable` | `mixed`
- `applicability`: one cell per anchor carrying `anchor`,
  `version_range`, `official_presence` (`documented` | `not_documented` |
  `not_assessed`), `product_scope` (`registered` | `version_gated` |
  `vendor_boundary` | `not_applicable` | `unknown`), cell-level
  `parser_status`/`audit_status`, and `evidence` refs — evidence is
  required on every version cell.

`subaction_status` records exist where subactions differ meaningfully;
each carries a stable `subaction_id`, its own parser/audit status, and
complete applicability cells with evidence for every declared version.

`status` is derived from the row's applicability cells by frozen rules
(enforced by the gate): all registered applicable cells audited →
`semantically_checked`; all notice-only → `generic_notice`; all
explicit-unsupported → `explicit_unsupported`; all silent →
`parse_only`; all relevant documented forms parser-unrecognized →
`parser_unsupported`; all relevant product cells vendor-boundary →
`vendor_not_supported`; mixed registered audit states → `mixed_coverage`.
Out-of-domain `version_gated`/`vendor_boundary`/`not_applicable` cells do
not downgrade a completely audited registered domain, and
parser-unrecognized documented forms take precedence over mixed
registered states.

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
- `explicit_unsupported` — statement recognized but the product emits
  explicit unsupported/bounded evidence (not audited, not silent)
- `mixed_coverage` — registered applicable cells carry different audit
  states (per-subaction split)

`acceptance.refs` entries are `file:<repo-path>` (must exist),
`gate:<make-target>`, or `missing:<description>` for honest gaps.

Cell `evidence` follows a stricter contract (R4): every cell must carry
at least one concrete `file:`/`gate:` ref — `missing:` entries are notes,
never evidence. `file:` refs must exist. Probe files under `evidence/`
must be row-scoped (basename `<row-id>.probes.txt`) and must contain a
probe section for the cell's own anchor version, so evidence copied from
the wrong row or the wrong version fails the gate. `gate:` refs must
resolve to a committed golden manifest (`gate:T<NN>` →
`testdata/ddl-golden/T<NN>.json`). The aggregate derivation runs over
the union of row and subaction cells, so a `semantically_checked` parent
cannot mask a silent or explicit_unsupported registered subaction.

R6 — subaction-bound evidence: cells inside a `subaction_status` record
must additionally carry at least one ref bound to that exact subaction,
so one subaction cannot borrow another's evidence and row-level probes
cannot mask missing subaction evidence. Bound forms: an `evidence/`
probe file named `<row-id>.<subaction-slug>.probes.txt` whose header
declares `row_id: <row-id>` and `subaction_id: <exact-subaction-id>`
plus per-version `=== version <v> ` sections, or a
`file:<path>#<subaction-id>` selector whose literal is present in the
referenced file. A subaction cell citing a row-level probe or a probe
declaring a different `subaction_id` fails the gate; a row-level cell
citing a subaction-bound probe likewise fails.

Rows owned by future tasks may record
incomplete implementation — they are assigned, not counted as coverage.

## Gate

`make ddl-inventory-gate` runs `TestDDLInventoryContract` (schema,
vocabularies, per-cell evidence, version/subaction coverage, derived
aggregate status), the denominator-lock negative
`TestDDLInventoryDenominatorLocked`, and the derivation-rules unit test
`TestDDLInventoryAggregateDerivation`
(`internal/application/audit/ddl_inventory_contract_test.go`).

Verified sources: MySQL 5.7 (frozen Oracle mirror — the dev.mysql.com 5.7
URL redirects to 9.7), MySQL 8.0, MySQL 8.4, TiDB release-8.5 docs.
Schema version 2 (parser/audit/applicability/subaction separation),
verification date: 2026-11-13.
