# T16-R1 evidence bundle — reviewer-remediation @034e9859

This bundle closes the T16-R1 remediation round under #95: the four
divergences the independent review found against `7985c774` are fixed
(`034e9859`), and this directory records the gates and the re-executed
golden suite at that exact head. The original T16 evidence bundle at
`../7985c774f8c83e5a93facb4106923a5581495062/` is preserved untouched —
55/739 stays the historical record.

## Commits covered

| Commit | Content |
|---|---|
| `034e9859` | `fix(ddl): T16-R1 — multi-DROP temp veto, rename last-write, self-pair, inbound-FK truth` — 16 files: shared `spec.RenamePairClasses` last-write adjudicator consumed by `applyRenameTable` and `renamePairVerdicts`, per-pair shape binding (`pairShapes`), three-state temp-arm `dropTargetFate`, ALTER-vs-standalone self-pair distinction, provider `ReferencedByUnknown` marking + gap suppression when a proven edge answers, CONTRACT/ADR/probes-README/module-README sync, new probe log, manifest +5 cases. |

## Findings and their dispositions

| # | Reviewer finding | Disposition | Proof |
|---|---|---|---|
| F1 | `dropTargetFate` 8.x temp arm unconditionally `Kept` — conflated veto capability with an actual veto | Temp arm mirrors the persistent three-state fate: `!vetoPossible → Dropped`, `anyMissing → Kept`, else `Unproven`; TiDB keeps missing-before-unvouched ordering | probe `mysql80::drop-multi-temp-and-persistent` (both dropped); unit `TestBatchStateT16R1DropMultiTemp*`; golden `t16-mysql80-drop-multi-temp-resolvable` + `t16-cli-drop-multi-temp-unvouched` |
| F2 | Boolean `produced`/`consumed` sets lost last-write ordering | `spec.RenamePairClasses` evaluates every endpoint at its own pair position over an evolving disposition map; consumed-then-produced identities re-present, produced-then-consumed re-absent; each pair's bound shape captured per-position | native `p16_r1_selfpair_lastwrite.log` (`a→b,b→c,d→b` migrates; `a→b,b→c,b→a` refuses 1146 + rollback); unit `TestBatchStateT16R1Rename*` + rule-layer `TestT16R1RenameLastWriteAndSelfPair`; golden `t16-mysql80-rename-lastwrite-{free,rollback}` |
| F3 | `ALTER TABLE a RENAME TO a` fired a false destination blocker | `renameEndpointSelfPair` class scoped to ALTER-form pairs only; standalone `RENAME TABLE a TO a` keeps the destination-occupied verdict | native probe: standalone self-pair = ERROR 1050, ALTER self-pair = rc 0 no-op (both recorded in `p16_r1_selfpair_lastwrite.log`); rule tests cover present/absent/unknown/temp/cross-schema; golden `t16-mysql80-alter-rename-self-noop` |
| F4 | Contract vs implementation: provider inbound-FK completeness | Adjudicated: provider loads outbound edges only → provider snapshots can never prove an empty inbound set. Provider-loaded present shapes carry `ReferencedByUnknown`; TRUNCATE blocks on proven batch edges, owes `target_table.inbound_fk` on a bare unknown set, and suppresses the gap when a proven edge already answers 1701. Post-state stays existence/member-faithful and withholds volatile counter claims either way | `resolve()` marks the flag on the bound clone (provider object untouched); unit `TestBatchStateT16R1Provider{TruncateGapsInboundFK,ParentBatchEdgeBlocksNoGap},BatchParentProvenEmptyClean`; golden re-expected `truncate-suite` + `drop-duplicate` |

## Golden line at this head

`T16_MYSQL_PASSWORD=root python3 ./scripts/ddl_golden.py run --task T16
--artifact-dir artifacts/t16r1` → **60/60 cases, 832 assertions, PASS**,
`head_sha == cli.build.head_sha == 034e9859`, cleanup `compose_down_rc=0`,
zero residual containers (`golden/T16.artifact.json`, `golden/T16-run.log`).

Denominator reconciliation vs the original 55/739:

- +4 metadata cases: `t16-mysql80-drop-multi-temp-resolvable`,
  `t16-mysql80-rename-lastwrite-free`, `t16-mysql80-rename-lastwrite-rollback`,
  `t16-mysql80-alter-rename-self-noop`
- +1 cli case: `t16-cli-drop-multi-temp-unvouched`
- 2 re-expected cases under the honest F4 contract (provider-loaded
  TRUNCATE owes the inbound-FK gap): `t16-mysql80-truncate-suite`,
  `t16-mysql80-drop-duplicate`

Cross-suite regression check was done by manifest analysis: the only
consumer of `ReferencedByUnknown` is `ddl.table.truncate.inbound_fk`,
which exists solely in the `t16-lifecycle` policy profile — no other
suite enables it, so T09–T15 manifests cannot change (verified: T13's
`TRUNCATE TABLE` cases run under `t13r1-truncate-rows`, which enables
`max_count` only).

## Gates collected

See `gates/INDEX.md`: `go test ./... -count=1`, `go vet ./...`, race on
`./internal/...`, sql-corpus, inventory, coverage-catalog, parser-error
contract, unsupported-diagnostics, decision-record, docs-example, both
PostgreSQL gate packs, three-level-doc (worktree + committed-range), and
the lint baseline comparison — all green except the two lint runs whose
findings are 100% pre-existing baseline (zero new items).

## Native probe supplement

`docs/work-plans/t16-lifecycle-temporary-conditional/probes/p16_r1_selfpair_lastwrite.log`
— disposable `mysql:8.0` container (verified 8.0.46), closed the two cells
the original matrix never probed: standalone self-pair = 1050 (the README
row for the tail supplement previously claimed this cell; the log never
contained it — corrected) and the exact error of a consumed rename source
(1146 + atomic rollback).

## Boundaries not re-verified this round

- MySQL `DROP a, a` (1066) still records no finding/diagnostic — state is
  correct; the notice gap stays a documented non-blocker.
- TiDB temp-endpoint RENAME still over-reports (8200 + dest.exists) —
  accepted behavior.
- `docker pull` through the default credential store hangs on this
  machine (`docker-credential-desktop` stalls); pulls used an isolated
  `DOCKER_CONFIG`. Environment-only, unrelated to product.
