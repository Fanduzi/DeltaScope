# MySQL 8.0 applicability — SRS family

## Registered domain

`MySQLSRSDropAuditedDomain` = MySQL + ValidatedRange + 8.0.x + patch≥4 (`srs_drop.go:135-140`). The CREATE family inherits it verbatim (frozen F-2).

## Sub-ranges

| Range | Audited? | Provider reads? | Notes |
|---|---|---|---|
| 8.0.0–8.0.3 | **No** (patch<4) | **Yes** (`Major>=8`) | ST_SRS introduced 8.0.3; the provider gate never checks minor.patch — a proven 8.0.0–8.0.2 issues queries against a catalog surface that may not exist → query error propagates as hard failure, not bounded gap. Divergence case. |
| 8.0.4+ | Yes | Yes | Full slice: existence, decode-gated conflicts, deps, ordered channel, adjudication, apply mints. |
| 8.0.46 | Yes | Yes | Native-probed (prior accepted rounds). The generalization 8.0.4+→all-8.0.x is a documented registered boundary, not per-patch probe coverage. |

## Evidence base

- Accepted: SRS DROP R5, SRS CREATE R5, guard correction R3, conflict identity R2 — all executed against 8.0.46.
- Parser-recognition-only range: the parser accepts SRS syntax regardless of server version; offline/unknown audits prove nothing about applicability.
- Patch-specific risk: 8.0.0–8.0.2 lacks the SRS catalog — the only MySQL 8.0 sub-range where provider reads can hit a missing table.

## Contract relevance

V6 records the domain as registered (8.0.4+), notes the 8.0.0–8.0.3 provider-read divergence as unresolved, and keeps patch-level claims bounded to the 8.0.46 anchor.
