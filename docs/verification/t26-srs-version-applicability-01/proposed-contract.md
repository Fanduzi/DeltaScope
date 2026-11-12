# Proposed contract V1–V12 — SRS version applicability (Phase 0, unfrozen)

Each clause: source evidence → current implementation → observed behavior → proposed rule → unresolved boundary. **Nothing here is implemented; this is a proposal for reviewer adjudication.**

## V1 — Trusted version-observation source

- **Evidence**: `metadata.go:161-176`; `provider_server_srs.go:106-118`; `version.go:132-162`.
- **Current**: two independent `select version()` parses — request-level `resolvedVersion` (audit gate) and provider-internal `mysqlSRSCatalogSupport` (read gate). Caller `target_version` never substitutes; mismatches are typed errors.
- **Observed**: both consume the same banner grammar but can degrade independently; O1-nil + O2-ok (or vice versa) is possible.
- **Rule**: the provider-observed banner is the sole authoritative version fact for both gates; a caller pin may only *conflict-check*, never substitute.
- **Unresolved**: whether the two observation points should share one parsed identity (single-parse contract) or stay independent.

## V2 — Definition of unknown version

- **Evidence**: `MySQLSRSDropAuditedDomain` (`srs_drop.go:135-140`); `version.go:82-162`.
- **Current**: `resolvedVersion==nil` and `ValidatedRange==false` both fail the domain predicate identically — "unknown" and "proven-but-unvalidated" are not distinguished at the gate.
- **Observed**: unparseable banner and proven 9.x produce the same boundary marker.
- **Rule**: unknown = `nil` (no fact) OR `!ValidatedRange` (fact outside evidence series); both must stay non-applicable, but the marker/gap should keep the two cases distinguishable.
- **Unresolved**: whether the marker needs a reason sub-field (`unproven` vs `out_of_series`).

## V3 — Static-only checking scope

- **Evidence**: parser markers (`NameDecodeComplete`, `OrgDecodeComplete`), `SRID` u32 decode, `HasName/HasOrganization/HasIdentifiedBy` flags, reserved-range `SRSDropInReservedRange`.
- **Current**: parser-proven facts (SRID validity, attribute completeness, decode-complete flags, IF guards, contradictory forms, reserved-range notices) are computed version-blind.
- **Observed**: boundary statements still carry complete static evidence.
- **Rule**: static checks must run under any version premise including nil; no static fact may be suppressed by version gates.
- **Unresolved**: none — current behavior already satisfies it; codify to prevent regression.

## V4 — Version-dependent premise scope

- **Evidence**: `bindMySQLSRS` qualifier supply (:198-210); provider loader reads; `srs_drop_premise.go`.
- **Current**: existence, NAME/ORG conflicts, dependencies, drop/create privilege premises are all version-dependent facts; no named `srs.version_applicability` premise exists — the gap is a statement-level marker.
- **Observed**: under unknown version the premises are neither discharged nor explicitly owed — the statement exits before adjudication enumerates them.
- **Rule**: enumerate the version-dependent premise set per feature; a `srs.version_applicability` named fact (or equivalent marker field) should carry the bounded gap when applicability is undecidable.
- **Unresolved**: exact fact-id naming and whether it lands in `Owed` vocabulary or stays marker-level.

## V5 — MySQL 5.7 boundary

- **Evidence**: mysql57-boundary.md; provider :121-131.
- **Current**: proven 5.7 → application boundary + provider `version_inapplicable` + zero reads. Both gates agree.
- **Observed**: consistent, dedicated unread reason, no silent fallthrough.
- **Rule**: proven 5.7.x = determined inapplicable — never unknown, never generic; reads must not fire.
- **Unresolved**: none for 5.7 proper; the 5.7-shaped geometry-column SRID surface remains out of scope.

## V6 — MySQL 8.0 applicability

- **Evidence**: mysql80-applicability.md; `srs_drop.go:135-140`.
- **Current**: audited domain = 8.0.4+; provider reads on `Major>=8` → proven 8.0.0–8.0.3 takes boundary **yet reads still fire** against a surface that exists only from 8.0.3.
- **Observed**: on 8.0.0–8.0.2 a query error propagates as hard failure; on 8.0.3 reads succeed but never adjudicate.
- **Rule**: audited domain stays registered at 8.0.4+; patch-level claims anchored to 8.0.46 probe, not generalized.
- **Unresolved**: whether provider's `Major>=8` should be tightened to `>=8.0.3` (catalog existence floor) — flagged for execution-order decision.

## V7 — MySQL 8.4 applicability

- **Evidence**: mysql84-applicability.md.
- **Current**: 8.4.x → application boundary (`Minor==0` required) but provider-supported (`Major>=8`) → reads fire, never adjudicate.
- **Observed**: real catalog facts land in `Metadata.SpatialReference` on a statement the audit reports as outside its domain.
- **Rule**: 8.4 remains boundary under the current registered domain; any widening is a separate explicit work order.
- **Unresolved**: the read/admit asymmetry on 8.4 — suppress reads for out-of-domain versions, or accept and document?

## V8 — Unverified MySQL versions

- **Evidence**: unknown-version-behavior.md.
- **Current**: nil/unvalidated → boundary at O1, reads still issue at O2.
- **Observed**: unknown version can produce a hard provider error (e.g. truly-old server) rather than a bounded gap.
- **Rule**: unknown must not imply applicability — satisfied today on the audit side; read-side behavior (fire vs suppress) must be a deliberate contract choice, and errors must propagate rather than mint facts.
- **Unresolved**: read-suppression under `!proven` is the open design fork — this is the central deferred decision.

## V9 — TiDB / vendor boundary

- **Evidence**: `bindMySQLSRS` :137-139; `VersionProductForDialect`; ParseObservedVersion TiDB marker handling.
- **Current**: TiDB banner → `Product=tidb` → fails MySQL gates; TiDB dialect → early bind return, zero reads; vendor boundary is dialect-driven.
- **Observed**: no MySQL semantics inherit through shared grammar.
- **Rule**: vendor separation must remain dialect/product-based, never grammar-recognition-based; TiDB SRS claims stay unaudited vendor boundary.
- **Unresolved**: none observed — codify as invariant.

## V10 — Provider version-observation errors

- **Evidence**: provider-version-error.md.
- **Current**: `!proven` ⇒ reads issue and errors propagate; `proven&&!supported` ⇒ dedicated `version_inapplicable`; read errors always propagate.
- **Observed**: "observation failed" is not distinguished from "observed but unvalidated" in any surfaced reason.
- **Rule**: observation errors stay distinct from both known-inapplicable and unknown; their propagation must not launder into empty witness sets.
- **Unresolved**: whether a third unread reason (`version_unproven`) is warranted at provider level.

## V11 — Public finding / evidence-gap mapping

- **Evidence**: `UnsupportedDetail` marker at `metadata.go:293-305`; `MetadataUnreadVersionInapplicable`; rule-side boundary rendering.
- **Current**: version-gated statements surface as unaudited boundary with `gate:"version"`; proven-inapplicable additionally carries the provider unread reason in bound state.
- **Observed**: consumer sees "unaudited (version)" — sufficient for boundary honesty, but cannot distinguish sub-causes (5.7 vs 8.4 vs unknown vs unvalidated).
- **Rule**: the public map must keep gate:"version" but should project the proven/unknown/inapplicable distinction into the gap detail.
- **Unresolved**: exact metadata key/name for the sub-reason.

## V12 — Batch and own-apply effects for unknown version

- **Evidence**: own-apply-version-analysis.md; `batch_state.go:520-536`.
- **Current**: boundary write mints `srsExistenceStale` + barrier flip (fail-closed); no present mint, no ordered entry, no adjudication.
- **Observed**: later same-SRID statements degrade to `batch_effects` — correct and honest.
- **Rule**: keep fail-closed mint semantics; unknown-version writes must never mint positive facts and must never suppress the stale flag.
- **Unresolved**: whether a version-gated write should also mark `srsDepsStale` (currently D3-0 exempts the whole family).
