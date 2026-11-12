# SRS CREATE guard correction — IF NOT EXISTS outcome vs privilege

## Status

Submitted — pending reviewer ruling
(T26-SRS-CREATE-GUARD-CORRECTION-01, AUTHORIZED; over the frozen
guard oracle T26-SRS-CREATE-GUARD-ORACLE-01 Phase 0 R3 and the frozen
G5 two-layer validation order on MySQL 8.0.46 / 8.4.10).

## Decision

`CREATE SPATIAL REFERENCE SYSTEM IF NOT EXISTS` publishes the
`exists_noop` marker arm only when the statement's own privilege
premise is consumable. A proven-present SRID under an unproven or
suppressed premise lands on the new `outcome_unproven` arm — the
catalog fact (object present) and the statement outcome (guarded
no-op reached?) are separated forever; the second is never inferred
from the first.

Rationale: the frozen oracle proved the native check order places the
privilege gate BEFORE the `IF NOT EXISTS` existence shortcut
(`priv-exists`: USAGE-only user + present target → ERROR 3631, never
the 3713 warning). The previously accepted arm therefore overclaimed:
`target present + privilege unknown` cannot prove the no-op arm fired.

`SRSCreateArmOutcomeUnproven` owes exactly `srs.create.privilege`
(post-guard conflict checks are natively shadowed by the existence
guard — they are never evaluated on either the 3631 or the 3713
branch, so no conflict premise is owed) and its own-apply writes
nothing: no `srsExistencePresent` success publication, no stale mint
that would overwrite the catalog-proven presence — both the 3631 and
the 3713 native branches leave the catalog unchanged, so the
proven-present fact stays consumable by later batch statements.

No new finding identifier was created — contract D's existing
vocabulary suffices: the public surface carries only the
`unproven_privilege` evidence gap (required fact
`srs.create.privilege`) and omits the `exists_noop` notice and any
Warning-3713 text. The gap carrier narrows to `unproven_privilege`
because the arm's sole debt is the privilege premise.

## Regression invariants (frozen scope B/F)

Plain `CREATE` (`refused_existence`), `OR REPLACE` (replace arm),
SRS DROP, SERVER, ALTER INSTANCE, T25, the shared parser, the G5
validation order itself, and the TiDB SRS vendor boundary are all
unchanged — only the `IF NOT EXISTS`-on-present arm split moved.

Corpus: `ddl.mysql.srs.create.exists_noop` joins the deferred-coverage
list — it can only fire under a consumable privilege premise, which
the providerless corpus fixture can never supply (same contract as
`ddl.mysql.srs.create.privilege`).

## Verification

- `TestT26SRSCreateGuardCorrection*` — E1 arm grid (present/absent/
  unknown × proven/unknown/suppressed/missing-observation privilege),
  E2 own-apply (zero present/stale mints, fresh re-read on the next
  bind, following DROP unblocked), public JSON (no `exists_noop`
  finding / no 3713 text / `unproven_privilege` gap retained).
- `go test ./...`, `go vet ./...`, `gofmt` — clean (the pre-existing
  `mysql_t26r5_test.go` drift untouched).
- Real CLI JSON matrix: `cli01..cli08` under
  `docs/verification/t26-srs-create-guard-correction-01/`.
