# MySQL DROP SPATIAL REFERENCE SYSTEM ordered absence publication

## Status

Submitted — pending reviewer ruling
(T26-SRS-DROP-ORDERED-PUBLICATION-EXECUTION-01 R2, candidate
`fc872115`; R1 `5f9e8439` was returned CHANGES REQUIRED and is
preserved as historical evidence).

## Decision

A catalog-certified `DROP ... SRS` absence arm (absent_refused /
absent_noop / absent_conditional — any A arm whose bound state
satisfies `ValidAbsence`) publishes a per-SRID ordered entry into
`batchState.srsOrdered` at own-apply. A later same-batch audited drop
on the same authored SRID consumes an active entry through the bind's
ordered branch: `OrderedAbsentConsumed` supplies the A slot while the
deps-only direct reader (`MySQLSRSDependencyReader`, an optional
provider capability re-judged per enrich call) issues fresh Qo+Qdeps
— the existence query and objectCache are skipped. The entry is
internal provenance only: `OrderedAbsent` is `json:"-"`, no catalog
fact (`Read`/`Exists`/`Absence`) is fabricated, and public output is
unchanged.

Barrier + stale semantics: a same-SRID unproven arm or boundary write
mints `srsExistenceStale` and flips `entry.barrier` — provenance kept
(`barrier ⇒ stale` invariant). Contamination shadows consumption;
`srsDepsStale` demotes the witness only; REVOKE ends the privilege
premise, not the fact-level entry; inert operations do nothing;
absent the deps-reader capability the entry is never consumed and the
full resolver/objectCache path governs (warm cache answers with
zero-read cache-hit, cold cache may change the arm).

## Rationale

Same-batch DROP statements must see the earlier proven absence without
re-reading existence, while the dependency witness — the only fact
that can change arm — still re-reads fresh on every hit. Separating
the ordered fact (entry) from the fresh witness (deps read) keeps S1
outranking ordered A exactly as catalog S1 outranks catalog A.

## Public contract

No serialized shape change. `statements[i].metadata` never carries
`OrderedAbsent`; findings, gaps, coverage, and verdict derive from the
same adjudication.

## Deferred

Successful-deletion-driven publication, ordered-present, dependency
completeness, SRS CREATE auditing, ALTER INSTANCE.

## Evidence

- `docs/verification/t26-srs-drop-ordered-publication-execution-01/README.md`
- Contract: `docs/verification/t26-srs-drop-ordered-publication-contract-01/`
- Tests: `internal/application/audit/t26_srs_ordered{,_int}_test.go`,
  `internal/infrastructure/metadata/mysql/provider_t26_srs_deps_reader_test.go`
