# Decision: Table lifecycle, temporary namespaces, conditional clauses, and rename ordering (MySQL/TiDB)

Date: 2026-11-09 — milestone decision-record convention: decision dates
are assigned per task slot and run ahead of commit dates; neighboring
records follow the same convention (e.g. 2026-11-08 committed 2026-10-29).
Status: Accepted
Related milestone/version: mysql-tidb-ddl-completion (issue #79, task T16 under #95)
Related commits: this task's commits on the workfree line after `74a344dc`
Related tests: `internal/domain/rule/ddl/lifecycle_t16_rules_test.go`,
the updated `internal/application/audit` ordered-state suites
(`batch_state_a*_test.go`, `batch_state_t07*` controls), corpus fixtures
under `testdata/sql-corpus/{mysql,tidb}/ddl/metadata/`, and
`make ddl-golden TASK=T16 ARTIFACT_DIR=...` against `testdata/ddl-golden/T16.json`
Related docs: `docs/work-plans/t16-lifecycle-temporary-conditional/CONTRACT.md`,
native probe logs `docs/work-plans/t16-lifecycle-temporary-conditional/probes/`
(`p16_lifecycle_matrix.log`, `p16_tail_supplement.log`,
`p16_r1_selfpair_lastwrite.log`),
`testdata/ddl-inventory/inventory.yaml` (thirteen T16 rows), module README
updates in the same change

## Context

Issue #95 closes the last owned DDL surface under #79: table lifecycle
forms whose native semantics were either unaudited (local/global
temporary tables, `CREATE TABLE ... LIKE`, CTAS, `IF [NOT] EXISTS`
conditional clauses, multi-target DROP) or modeled by T05-era
conservative fallbacks (non-audited → tombstone) that predated native
probe evidence. The p16 probe matrix froze the semantics on MySQL
5.7.44 / 8.0.46 / 8.4.10 and TiDB v8.5.0 (368 probe labels plus focused
continuation probes), and exposed three places where the older model
was outright wrong rather than merely conservative:

- `RENAME TABLE` pairs are evaluated **left-to-right on an evolving
  working set** — a destination an earlier pair vacates is free when a
  later pair claims it; a destination occupied at its pair's evaluation
  moment blocks even if a later pair would have freed it. Any refused
  pair rolls the whole statement back atomically on all anchors.
- Multi-target `DROP TABLE` atomicity is **version- and dialect-bound**:
  MySQL ≥8.0 vetoes atomically on a missing or duplicate name, MySQL
  5.7 drops resolvable members anyway (sequential partial), TiDB is
  sequential for the persistent catalog while its temporary slot is
  veto-shaped (the temp object drops only when no sibling vetoes).
- Duplicate names in one DROP list are a parse-time `1066` refusal on
  every MySQL anchor (nothing executes) but sequential on TiDB (the
  first occurrence lands, the second reports `1051`).

## Decision

1. **Two disjoint identity namespaces on the ordered state.**
   `batchState` keeps `entries` (persistent catalog) and `tempEntries`
   (session-local temporary) as separate slots keyed by the same
   (schema, table) identity. `bindTarget`/`bindSource`/
   `bindRenameEndpoint` in `batch_state_namespace.go` resolve each
   declared identity to the namespace the operation provably reads:
   explicit `TEMPORARY` binds the session slot, TiDB `GLOBAL TEMPORARY`
   binds the catalog (probed as a persistent object), plain `CREATE`
   lands on the catalog even under a temp shadow, and LIKE/CTAS
   sources, DROP, TRUNCATE, and RENAME endpoints bind temp-first where
   the engine proves it (MySQL standalone `RENAME TABLE` is
   catalog-oriented and ignores the temp namespace; TiDB rename is
   temp-first and refuses 8200 on a local-temporary endpoint).
2. **Per-statement bound projections.** `Metadata.TargetTables`
   publishes one `TargetBinding` per `TableTargets()` identity
   (`Snapshot` nil = unvouched, `Ambiguous` never conflated with
   absence, `Snapshot.Temporary` marks the namespace the binding
   resolved); `Metadata.SourceTable`/`Metadata.SourceTables` carry the
   LIKE/CTAS source bindings with nil entries for unvouched sources.
   Bindings are computed before `state.apply` and degrade to unvouched
   once the state contaminates — they never leak a cached answer.
3. **Probed refusals keep the exact pre-state.** T16 replaces the
   T05-era "non-audited → tombstone" fallback for every cell the probe
   matrix proves refused: 1050/1146 rename endpoint refusals, 1701
   TRUNCATE-on-inbound-FK, TiDB local-temporary 8200 (ALTER, RENAME)
   and 8006 (LIKE source), TiDB CTAS 1105, TiDB `ON COMMIT PRESERVE
   ROWS`, and MySQL `GLOBAL TEMPORARY` syntax refusals. Cells the
   probes did not decide (e.g. DROP on an FK-referenced parent)
   keep the prior conservative invalidation.
4. **Sequential rename work-set, atomic publication.** `applyRenameTable`
   evaluates pairs left-to-right against an evolving work set where the
   LAST write to an identity decides its state at each pair's evaluation
   moment — a destination an earlier pair produced and a middle pair then
   consumed is absent again, and a destination an earlier pair vacated is
   free for a later pair (`RENAME a→b, b→c, d→b` migrates; `RENAME a→b,
   b→c, b→a` refuses 1146 on the consumed source and rolls back). One
   shared adjudicator (`spec.RenamePairClasses`) feeds both the state
   transition and the rule layer (`renamePairVerdicts`), so findings,
   gaps, and state conclusions never diverge; a refused or unproven pair
   rolls the whole statement back atomically. ALTER-form self-pairs
   (`ALTER TABLE a RENAME TO a`) are a probed no-op exempted at the
   endpoint-class level, while standalone `RENAME TABLE a TO a` keeps its
   probed 1050 destination refusal — the exemption is scoped to the
   ALTER syntax only.
5. **Version-gated multi-DROP fates.** `dropTargetFate` resolves each
   member against the version fact: MySQL ≥8.0 atomic veto, 5.7
   sequential, TiDB sequential-persistent + veto-shaped-temp, duplicate
   lists parse-refused on MySQL vs sequential on TiDB. The temporary arm
   mirrors the persistent arm's three states — veto capability alone
   never freezes a resolvable temp object (a fully resolvable list still
   drops it; a proven-missing sibling vetoes; an unvouched sibling
   leaves the fate unproven). An unvouched version leaves the fate
   undecidable rather than guessing atomicity.
6. **Conditional clauses arm existence premises.** `IF NOT EXISTS`
   suppresses the create-collision finding only when the target is
   proven present (the no-op is probed); `IF EXISTS` on DROP/TRUNCATE
   proves absent-target no-ops. An unvouched premise still owes its
   evidence gap — the clause changes the finding, never the honesty of
   the answer.
7. **New T16 rules (all default-on, blocker).**
   `ddl.table.drop.exists.require`, `ddl.table.truncate.exists.require`,
   `ddl.table.like.source.exists.require`,
   `ddl.table.create_as.source.exists.require` (MySQL-only — TiDB CTAS
   is a vendor boundary), `ddl.table.rename.source.exists.require`,
   `ddl.table.rename.dest.exists.forbid`,
   `ddl.table.truncate.inbound_fk` (probed 1701, both dialects), and the
   TiDB-only `ddl.tidb.local_temporary.{alter,rename,like}` refusals.
   Rule applicability excludes dialects where the premise cannot fire.
8. **Inbound-FK truthfulness.** `TableSnapshot.ReferencedBy` carries
   only batch-derived inbound edges; providers never load reverse FK
   edges, so provider snapshots and incomplete entries carry
   `ReferencedByUnknown` and the truncate rule owes a
   `target_table.inbound_fk` gap instead of reading empty as safe.
9. **Deferred boundaries stay explicit.** `FLASHBACK TABLE|DATABASE|
   CLUSTER` and `RECOVER TABLE` keep their T03 unsupported-entry rail
   (dynamic GC/snapshot facts are never simulated); TiDB `ON COMMIT
   PRESERVE ROWS` and MySQL `GLOBAL TEMPORARY` keep `create_table.
   temporary.global[.preserve]` vendor-boundary aspects; the
   preserve-refusal check is create-only so `DROP GLOBAL TEMPORARY`
   models as an ordinary catalog drop.

## Consequences

- The ordered state now answers every named identity in multi-target
  statements, and the metadata layer projects the same answers rules
  consume — closing the class where ordered state and findings could
  disagree about which object a statement touched.
- Frozen contracts updated: T05-era tombstone-on-unaudited cells that
  native probes later proved refused now preserve exact pre-state;
  obsolete tests asserting the old contract were rewritten against the
  probed matrix and recorded in the T16 CONTRACT.
- `semantically_checked` covers all thirteen T16 inventory rows except
  `tidb.flashback-recover`, which stays an explicit unaudited boundary
  by contract.

## Verification

- `go test ./internal/...` — all internal packages green, including the
  updated ordered-state suites and the T16 rule/corpus tests.
- `make ddl-golden TASK=T16 ARTIFACT_DIR=artifacts/t16` — 55/55 cases,
  739 assertions, `[ddl-golden] PASS` on all four anchors; artifact at
  `artifacts/t16/T16/artifact.json`. The R1 remediation adds five cases
  (multi-DROP temp+persistent resolvable on 8.x, offline temp+unvouched
  sibling, both last-write rename counter-examples, ALTER self-pair
  no-op) — post-R1 denominator 60 cases; the original 55/739 artifact is
  preserved untouched.
- Native evidence: `docs/work-plans/t16-lifecycle-temporary-conditional/
  probes/p16_lifecycle_matrix.log` +
  `p16_tail_supplement.log` (368 labels + continuation probes) +
  `p16_r1_selfpair_lastwrite.log` (standalone self-pair = 1050, ALTER
  self-pair = no-op, produced-then-consumed source = 1146 with atomic
  rollback, produced-then-vacated destination = free).
- `make ddl-inventory-gate` — thirteen T16 rows carry concrete
  `file:` evidence refs and updated status.

## Alternatives considered

- **Pre-evaluating all rename destinations** (the first
  implementation): contradicted by the native rotation probes
  (`t→u, u→v` blocks; `u→v, t→u` succeeds) — replaced with the
  sequential work-set model.
- **Boolean produced/consumed identity sets** (the shipped T16
  model): cannot express "last write wins" — a produced-then-consumed
  identity read as still produced (fabricating a phantom destination's
  shape and refusing a legal successor pair), fixed by the shared
  left-to-right `RenamePairClasses` adjudicator.
- **Freezing the temp arm on veto capability alone** (the shipped
  T16 model): kept a resolvable temp object on every atomic-mode
  multi-DROP even when no sibling could veto — replaced with the same
  three-state fate the persistent arm computes.
- **Treating TiDB multi-DROP as atomic**: contradicted by the
  continuation probes (resolvable members drop past missing siblings;
  the temp slot alone is veto-shaped).
- **Loading inbound FK edges from the provider**: providers expose no
  reverse-edge query; batch-derived edges plus the `ReferencedByUnknown`
  marker keeps the honest boundary instead of fabricating completeness.
