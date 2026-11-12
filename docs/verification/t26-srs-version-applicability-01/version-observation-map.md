# Version observation map — SRS family

How a version fact reaches each SRS surface on baseline `41ea7895`. Source review only.

## Observation sources

| # | Source | Code | Produces |
|---|---|---|---|
| O1 | Provider banner `select version()` at request level | `metadata.go:161-176` → `spec.ParseObservedVersion` | `resolvedVersion` on statement `Metadata.Version`; nil on absent/unparseable banner |
| O2 | Provider banner inside the SRS catalog loader | `provider_server_srs.go:106-118` `mysqlSRSCatalogSupport` | `(supported, proven, *VersionIdentity)` — decides read issuance only |
| O3 | Caller `target_version` pin | `spec.ParseTargetVersion` (`version.go:82-106`) | Never substitutes for O1/O2; pin/observed mismatch → `ErrTargetVersionMismatch` |

O1 and O2 parse the same banner grammar but are independent reads: O1 gates **audit applicability**, O2 gates **read applicability**. Nothing reconciles their answers when they disagree (they cannot disagree on banner content, only on how each consumer interprets it).

## Flow to each consumer

```
select version() ──O1──► resolvedVersion ──► MySQLSRSDropAuditedDomain ──► Unsupported{gate:"version"} (metadata.go:293-305)
        │                     │
        │                     ├─► Metadata.Version on every enriched statement (metadata.go:199-201 area)
        │                     ├─► orderedSRSDropEligible (batch_state_server_srs.go:236)
        │                     └─► srsDrop/CreatePrivilegePremise → MySQLServerDropPrivilegeDomain (server_drop_premise.go:218)
        │
        └──O2──► mysqlSRSCatalogSupport ──► proven&&!supported → skip all SRS reads + version_inapplicable
                                    └─► otherwise (incl. !proven) → existence/conflict/deps reads issue
```

## Per-feature version dependence

| Feature | Reads gated by | Audited domain | Boundary result |
|---|---|---|---|
| SRS existence (all kinds) | O2 (`Major>=8` when proven) | O1 8.0.4+ | read may still fire under boundary |
| SRS NAME conflict (CREATE family) | O2 + decode-complete qualifier (I6/I7) | O1 8.0.4+ | qualifier supplied by bind regardless of version; provider decides |
| SRS ORGANIZATION conflict | O2 + composite decode-complete | O1 8.0.4+ | same |
| SRS column dependencies | O2 | O1 8.0.4+ | read fires when resolver reached |
| Ordered-absent channel | O1 audited domain inside `orderedSRSDropEligible` | 8.0.4+ | ineligible → full resolver path |
| Adjudication (both kinds) | — | O1 via `Unsupported==nil` | no arm, no apply mints |
| Boundary stale mint | — | — | `mintSRSExistenceStale` always (batch_state.go:535) |

## Cross-check answers to the work order

- **C4 decode markers**: `NameDecodeComplete`/`OrgDecodeComplete` are qualifier preconditions set at parse; nothing version-related upgrades or downgrades them. Verified unchanged at `batch_state_server_srs.go:204-210`.
- **Guard correction**: `SRSCreateArmOutcomeUnproven` is adjudication output; adjudication never runs for boundary statements, so unknown version can never reach `exists_noop` either — both arms are unreachable off-domain. No regression vector.
- **Parser recognition ≠ applicability**: the parser declares SRS under both dialects; `bindMySQLSRS` returns early for non-MySQL dialect (:137-139). TiDB never issues SRS reads through this path.
