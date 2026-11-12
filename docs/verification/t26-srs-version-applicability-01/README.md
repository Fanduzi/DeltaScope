# T26-SRS-VERSION-APPLICABILITY-01 — Phase 0 package

Baseline: `41ea7895a1df62df924fe4f3576bbf54f72c5576` (tree `b14f910e`, parent `9ac5330c`), tracked working tree clean.

Scope: source review + applicability matrix + V1–V12 contract proposal. **No product implementation, no new native probes, no grammar/provider/contract changes.**

## Contents

| File | Content |
|---|---|
| source-facts.md | Full version-fact trace: O1/O2 observation points, two-gate divergence, per-surface behavior, own-apply writes |
| version-observation-map.md | Observation sources, flow to each consumer, per-feature dependence table, cross-check answers |
| mysql57-boundary.md | Proven-5.7 cell: both gates agree — boundary + zero reads + `version_inapplicable` |
| mysql80-applicability.md | Registered 8.0.4+ domain; 8.0.0–8.0.3 provider-read divergence; probe anchor 8.0.46 |
| mysql84-applicability.md | 8.4 = boundary at application, supported at provider — the read/admit asymmetry |
| unknown-version-behavior.md | Nil/unvalidated paths: boundary statements, reads still issue, fail-closed stale mint |
| provider-version-error.md | Observation-error taxonomy; distinctness of inapplicable vs unproven vs read-failure |
| own-apply-version-analysis.md | Per-write version sensitivity table; batch/ordered consumption under version variance |
| proposed-contract.md | V1–V12 clauses with evidence → implementation → behavior → rule → unresolved |
| unresolved-items.md | 10 open questions deferred to the contract ruling |
| mysql-collation-boundary.md | REUSED verbatim from T26-SRS-CONFLICT-IDENTITY-01 Phase 0 R2 accepted archive (provenance in reuse-ledger.txt): SRS catalog identity/collation boundary |

## Headline findings

- **Two gates disagree on three cells**: 8.4.x, 8.0.0–8.0.3, and unknown/unvalidated versions all take the audited-domain boundary at the application layer yet still issue real catalog reads at the provider layer.
- **5.7 is the consistent cell**: proven 5.7 → boundary + zero reads + dedicated `version_inapplicable`.
- **Fail-closed batch side**: boundary SRS writes mint `srsExistenceStale` and flip ordered barriers; no positive facts are ever minted off-domain.
- **No `srs.version_applicability` fact exists** — the version gap is a statement-level `gate:"version"` marker; C3's named-premise request is contract item V4/V11.
