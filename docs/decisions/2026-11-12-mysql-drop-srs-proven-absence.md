# MySQL DROP SPATIAL REFERENCE SYSTEM: supplier-certified proven absence

Status: **candidate** — T26-SRS-DROP-PROVEN-ABSENT-EXECUTION-01
implementation on `milestone/mysql-tidb-ddl-completion` from the accepted
contract `0fb1b7ff` (issue #105, milestone T26/#105); R1 ruling pending at
the independent reviewer. The contract
(`docs/verification/t26-srs-drop-proven-absent-contract-01`, R1 `aceaa48d`
→ R2 `23cea286` → R3 `46a1f472` → R4 `0fb1b7ff` + evidence supplement
`f206060d`) was accepted separately.

## Decision

A clean zero-row `ST_SPATIAL_REFERENCE_SYSTEMS` answer is no longer only
unknown. The provider supplies `SpatialReferenceSystemState.Absence`
behind an explicit-`Rows` predicate — query succeeded, zero rows
observed, `rows.Err()` and `rows.Close()` clean — and only while the same
`select version()` observation carries the registered completeness proof
(validated MySQL 8.0.4+, the domain where the information-schema view is
a bare unfiltered projection of the data-dictionary table).
`spec.ValidAbsence` binds that certified absence to the statement's own
authored SRID, and `AdjudicateSRSDrop` takes `ifExists` and selects the
frozen first-match order B → S0 → S1 → A → S2: a positive dependency
witness still outranks a simultaneous absence proof.

## Why

`DROP SPATIAL REFERENCE SYSTEM` on a proven-absent SRID refuses natively
(`ER_SP_DOES_NOT_EXIST`) while `IF EXISTS` warns — evidence the audit can
now legally supply, unlike the privilege-filtered
`ST_GEOMETRY_COLUMNS` view where zero rows stay unproven forever. The
`Absence` bit is supplier-certified, never inferred from
`Read && !Exists`: `sql.ErrNoRows` at QueryContext or `rows.Err()` is an
error, an unread or foreign-SRID state never qualifies, and an
unproven/out-of-domain Qo observation leaves the zero-row answer
unknown.

## Public contract

- `ddl.mysql.srs.drop.absent` (default blocker): fires only on
  `absent_refused`, message `spatial reference system <srid> does not
  exist`, metadata exactly `{srid, outcome: "refused_absent"}`.
- `ddl.mysql.srs.drop.absent_noop` (default notice): fires only on
  `absent_noop`, metadata exactly
  `{srid, outcome: "absent_noop", conditional_audit: "observed_account_new_session"}`.
- `ddl.mysql.srs.drop.dependency` gaps: `unproven_privilege` with exactly
  `[srs.drop.privilege]` on `absent_conditional`; `unknown_srs_state`
  keeps every other arm's owed set.
- `ddl.mysql.srs.drop.reserved` co-fires on `blocked`, `unproven`,
  `absent_noop`, and `absent_conditional` — never on `absent_refused`.
- Own-apply: only `SRSDropArmUnproven` mints the authored SRID's
  `srsExistenceStale`; every A arm mints and clears nothing.

## Privacy

No authored name, catalog `srs_name`, dependent schema/table/column
identity, account, or grant text is serialized. A foreign-SRID absence
snapshot is never consumed as this object's absence (PA-4e2).

## Verification

The PA case matrix lands across layers:
`internal/infrastructure/metadata/mysql/provider_t26_srs_absence_test.go`
(supply-predicate discriminants incl. `ErrNoRows` in both positions),
`internal/application/audit/t26_srs_absent_int_test.go` (unit/enrich-apply
rows incl. stale channels, per-enrich cache, REVOKE scoping snapshots),
`internal/application/audit/t26_srs_absent_test.go` (request/public rows
incl. policy rewrites and serialized-JSON sentinel scans). The two
provider-dependent rules defer from providerless corpus coverage exactly
like `ddl.mysql.srs.drop.privilege`; the online/API matrix proves their
public shape.

## Links

- Contract: `docs/verification/t26-srs-drop-proven-absent-contract-01/`
- Execution receipt: `docs/verification/t26-srs-drop-proven-absent-execution-01/`
- Sister records:
  `docs/decisions/2026-11-17-mysql-drop-srs-audited-slice.md`,
  `docs/decisions/2026-11-12-mysql-drop-srs-conditional-privilege.md`
