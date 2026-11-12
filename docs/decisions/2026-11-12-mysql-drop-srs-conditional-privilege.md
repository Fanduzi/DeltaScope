# MySQL DROP SPATIAL REFERENCE SYSTEM: conditional privilege discharge

Status: **candidate** — T26-SRS-DROP-PRIVILEGE-EXECUTION-01 implementation
on `milestone/mysql-tidb-ddl-completion` from the accepted contract
`2f7231b59744d6bbafb7150ffe32891002a2f6ab` (issue #105, milestone T26/#105);
R1 ruling pending at the independent reviewer. The contract
(`docs/verification/t26-srs-drop-privilege-contract-01`, R1 `524a1935` →
R2 `1c48d4ec` → R3 `2f7231b5`) was accepted separately.

## Decision

`ddl.mysql.srs.drop.privilege` joins the audited SRS DROP slice: the
`srs.drop.privilege` owed fact is no longer unconditionally unproven.
`AdjudicateSRSDrop` now takes the statement's own three-state premise —
unproven, consumable, or suppressed-by-scope — and only the unproven (S2)
arm carrying a consumable premise removes the fact from `Owed` and emits
the family's own conditional notice.

## Why

`DROP SPATIAL REFERENCE SYSTEM` requires SUPER under MySQL 8.0.x exactly
like the audited SERVER family. The request already loads one immutable
`CURRENT_USER()` + `SHOW GRANTS` observation for the SERVER slice; the SRS
family projects that same witness independently through
`(*batchState).srsDropPrivilegePremise()` — never borrowing a SERVER
adjudication, marker, or sibling SRS premise — so a discharged SERVER
premise can never silently discharge `srs.drop.privilege` (and vice
versa). A scope end without a positive witness stays unproven, not
suppressed.

## Public contract

- Finding: `ddl.mysql.srs.drop.privilege` (default level notice), exactly
  one emission on the S2 arm with a consumable premise — including
  alongside the reserved advisory — with the fixed message
  `spatial reference system <srid> drop privilege premise discharged under
  the observed_account_new_session conditional audit` and exactly
  `{srid, conditional_audit: "observed_account_new_session"}` metadata.
  Blocked/refused/unaudited statements never carry it.
- Gaps: `srs.drop.privilege` leaves the ordered `Owed` set only on S2 +
  consumable; B/S0/S1 may carry the consumable projection without
  consuming it, and a suppressed premise stays owed exactly like unproven.
- Configuration: disabling the rule suppresses only the finding — premise
  derivation, P discharge, loader qualification, `Owed`, and own-apply
  staleness are unchanged; level rewrites ride the existing severity
  aggregation.
- Evidence: the once-per-request loader now also qualifies on an audited
  `SRSKindDrop` statement under `observed_account_new_session`; SERVER and
  SRS statements in one request share the single observation, counted
  separately per family.

## Privacy

The observation (`Account`, `GrantLines`) stays internal: no account,
grant, session, schema, query payload, or provider-sensitive value is
serialized into findings, gaps, diagnostics, or errors. The public marker
is only the closed `observed_account_new_session` token.

## Verification

`internal/application/audit/t26_srs_priv_exec_test.go` (external
`audit_test` package) drives the real product chain — scripted
`database/sql` driver → `mysqlmeta.NewProvider` → product loader and
collector — for the P1–P10 request/API matrix, including the P4h
`GRANT SUPERUSER ON *.*` exact-token discriminator and the
`R/Qexist/Qdeps/Qo/L` + `Vd` counting vector.
`internal/application/audit/t26_srs_priv_exec_int_test.go` observes the
real `batchState` through the bind/apply seams for the internal halves:
stale demotion, double-enrich loader rerun, positional suppression, the
four-cell premise truth table, boundary mint, and sibling independence.

## Links

- Contract: `docs/verification/t26-srs-drop-privilege-contract-01/`
- Sister records: `docs/decisions/2026-11-17-mysql-drop-srs-audited-slice.md`,
  `docs/decisions/2026-11-16-mysql-drop-server-conditional-privilege.md`
