# Own-apply and batch effects under version variance — SRS family

## Write paths and their version sensitivity

| Write | Minted by | Version gate |
|---|---|---|
| `srsExistencePresent[SRID]` | `applySRSCreateStatement` create/replace arms (`batch_state_srs.go:157-160`) | Adjudication only runs when `Unsupported==nil` → unreachable under boundary. **Never minted on unknown/out-of-domain version.** |
| `srsOrdered[SRID]` proven-absence entry | `applySRSStatement` A-arms (:105-112) | Same — unreachable off-domain. |
| `srsExistenceStale[SRID]` | adjudicated unproven arms AND **any boundary SRS write** (`batch_state.go:535`) | Boundary mint is version-independent — fires on gated CREATE/OR REPLACE/DROP alike. |
| `entry.barrier` flip | `mintSRSExistenceStale` | Follows the stale mint. |
| `srsDepsStale` (batch-wide) | `spatialTouching` | SRS family exits at D3-0 — never minted by SRS writes themselves. |

## Consumption under version variance

- `orderedSRSDropEligible` requires `MySQLSRSDropAuditedDomain(s.resolvedVersion)` — ordered channel is closed under unknown version; falls through to the resolver path, which still reads.
- `srsExistencePresent` short-circuit at bind (:160-164) is version-blind but unreachable: it can only have been minted by an audited statement earlier in the same batch (same resolvedVersion).
- Later statements on a different version premise than earlier ones cannot occur — `resolvedVersion` is per-enrich-call, not per-statement.

## Net assessment

Version-unknown writes are fail-closed on the batch channel (stale mint, barrier flip, no publication) and fail-open on the read channel (queries still issue). The asymmetry is the honest current state; the contract proposes keeping the batch side unchanged and deciding the read side deliberately (V8, V12).
