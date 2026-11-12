# Unresolved items — Phase 0 open questions for contract ruling

1. **V1** — Should request-level and provider-level version observation share one parsed identity, or remain two independent reads?
2. **V2** — Does `gate:"version"` need a sub-reason distinguishing `unproven` (nil) from `out_of_series` (non-nil, !ValidatedRange)?
3. **V4** — Named `srs.version_applicability` premise fact vs marker-level gap: which vocabulary does the reviewer want?
4. **V6** — Should provider read gating tighten from `Major>=8` to `>=8.0.3` (ST_SRS catalog existence floor)?
5. **V7** — The 8.4 read/admit asymmetry: suppress reads for out-of-domain versions, or document as deliberate?
6. **V8** — Central fork: should `!proven` suppress SRS catalog reads (fail-closed) or keep issuing them (current: propagate real errors)?
7. **V10** — Is a third provider unread reason (`version_unproven`) warranted alongside `version_inapplicable`?
8. **V11** — Exact metadata key/projection for the version-gap sub-reason.
9. **V12** — Should version-gated SRS writes mint `srsDepsStale` (currently family-exempt under D3-0)?
10. **Scope** — 8.4 audited-domain widening is explicitly out of this work order; flagged only as a documented asymmetry.

No new native probes were run for this Phase 0; all findings are source review at baseline `41ea7895` plus provenance-cited accepted archives.
