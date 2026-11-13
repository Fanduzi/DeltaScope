# Decision record — DDL inventory subaction-bound evidence contract

Date: 2026-11-13
Scope: `testdata/ddl-inventory/` evidence contract and
`internal/application/audit/ddl_inventory_contract_test.go` gate.

## Decision

Applicability cells nested inside a `subaction_status` record must carry
at least one evidence ref bound to that exact subaction, in addition to
the R4 concrete-ref requirement. Two bound forms are accepted:

1. A committed probe file under `testdata/ddl-inventory/evidence/` named
   `<row-id>.<subaction-slug>.probes.txt` whose header declares
   `row_id: <row-id>` and `subaction_id: <exact-subaction-id>` followed
   by per-anchor `=== version <v> ` sections containing the captured CLI
   output.
2. A `file:<path>#<subaction-id>` selector whose literal string is
   present in the referenced repository file.

A subaction cell that cites a row-level probe, a probe declaring a
different `subaction_id`, a probe for the wrong row, or only unbound
refs fails the gate. A row-level cell citing a subaction-bound probe
also fails. `missing:` entries remain notes, never evidence.

## Rationale

Row-level probes can cover only one member of a declared subaction
family (e.g. `CONVERT TO CHARACTER SET` inside `alter-table.options`),
so aggregate coverage claims borrowed strength from unprobed siblings.
Binding evidence to the subaction makes each declared form carry its own
captured output, keeps honest states (`silent`, `notice_only`,
`audited`, `explicit_unsupported`, parser `unrecognized`) per form, and
lets the combined row+subaction aggregate stay derivable.

## Consequences applied

- `mysql.alter-table.alter-column-default` gained explicit
  `SET DEFAULT` / `DROP DEFAULT` subaction records; both audited
  (dedicated `ddl.alter.set_default` / `ddl.alter.drop_default`
  forbid rules).
- `mysql.alter-table.options` subactions were re-probed individually:
  `ORDER BY`, `CONVERT TO CHARACTER SET`, `FORCE` audited (rebuild /
  charset warnings); `ENABLE/DISABLE KEYS` notice_only; `DEFAULT
  CHARACTER SET`, `ROW_FORMAT`, `AUTO_INCREMENT=`, `COMMENT`, `ENGINE`
  silent under the declared shapes. Row stays `mixed_coverage`.
- `mysql.logfile-group` gained `CREATE/ALTER/DROP LOGFILE GROUP`,
  `ADD UNDOFILE`, `ENGINE=NDB` records, all `explicit_unsupported`
  (bounded engine-boundary `unsupported` output, not silent).
- `tidb.alter-instance` gained an explicit_unsupported record for the
  documented `ALTER INSTANCE ... RELOAD TLS` form family (bounded
  unaudited output). T32 (#111) still owns the instance-management
  boundary.
- `tidb.flashback-recover`: `FLASHBACK CLUSTER` fails parse
  (parser_error) while the other three forms emit bounded unsupported
  output, so the row is `mixed_coverage` (row-level cell uses the
  `mixed` audit marker, matching the `alter-table.options` convention).
- Earlier probe files recorded `unsupported`/`diagnostics` at the wrong
  JSON level (statements[] instead of top-level); all R4 probe files
  were regenerated with the correct extraction.

## Verification

- `make ddl-inventory-gate` — contract + denominator lock + derivation.
- `TestDDLInventoryEvidenceGate` — 9 new negative/positive cases covering
  wrong subaction, row-level borrowing, wrong version, missing bound
  evidence, selector refs, and row-cells citing subaction probes.
- Full regression: `make test`, `go vet ./...`, pg/corpus gates, and the
  live four-anchor golden path unchanged.

## Addendum — R7: ALTER INSTANCE NO ROLLBACK ON ERROR

The reviewer's final cross-check found one declared form family still
under-expressed: `ALTER INSTANCE RELOAD TLS NO ROLLBACK ON ERROR`, which
the T26 contract treats as a semantically distinct variant (distinct
finding name, `no_rollback_on_error` metadata, distinct why/risk prose —
the suppressed failure may drop the TLS surface even on a "successful"
return code).

Applied (inventory-only, no product changes):

- `mysql.alter-instance` gained subaction `RELOAD TLS NO ROLLBACK ON
  ERROR`: mysql57 `not_documented`/`version_gated`/
  `explicit_unsupported` (bounded unaudited output, same boundary as
  ordinary RELOAD TLS — the variant is an 8.0+ form), mysql80/mysql84
  `documented`/`registered`/`audited`. Ordinary RELOAD TLS evidence
  stays separate. Row stays `semantically_checked`.
- `tidb.alter-instance` replaced the broad
  `ALTER INSTANCE ... RELOAD TLS etc.` identifier with two precise
  records: `ALTER INSTANCE RELOAD TLS` and `ALTER INSTANCE RELOAD TLS
  NO ROLLBACK ON ERROR`, both `explicit_unsupported` on tidb85 (parser
  recognized, bounded unaudited output — not executed). Row stays
  `explicit_unsupported`. T32 (#111) remains the boundary owner.
- Probe files follow the frozen header: `row_id`, `subaction_id`,
  `dialect`, `literal SQL`, `binary HEAD`, `generation date`, then
  per-version sections with exit code, raw JSON findings, unsupported,
  diagnostics, coverage, verdict.
- Gate gained R7 negative controls: ordinary↔variant borrow rejection
  on both products and MySQL→TiDB evidence interchange rejection.
