# Decision: SRS dependency absence stays unproven — observer-scoped catalog reads never discharge the DROP dependency premise

Date: 2026-11-13
Status: Accepted (reviewer-frozen, Option C)
Related milestone/version: issue #105 / milestone T26 (T26-SRS-DEPENDENCY-ABSENCE-01)
Related commits: none — zero product changes
Related docs: docs/verification/t26-srs-dependency-absence-01/ (Phase 0 R1+R2 evidence, archives t26-srs-dependency-absence-phase0-r1.tar.gz 3510e9b1 and -r2.tar.gz 59266e19)

## Context

The SRS DROP adjudication reads `information_schema.st_geometry_columns WHERE srs_id = ?` on the R4 pinned connection. A positive witness row refuses the drop (S1 arm); `SRSDependencyFact.Complete` was hardcoded false, so a zero-row read could never prove dependency absence and always owed `srs.column_dependencies` (S2). Phase 0 investigated under which conditions a clean zero-row read could be promoted to a complete-empty dependency fact.

## Findings (native-verified on MySQL 8.0.46 and 8.4.10)

- The catalog view is the *exact* native enforcement surface for the executing account: visible dependency rows block with ERROR 3716; an invisible dependency does NOT block — the drop lands and leaves a dangling SRID reference (`priv-visibility-scoped-drop.txt`).
- A resolvable view column is itself a blocking dependency even when its base table is invisible to the account; an orphaned view contributes neither row nor block.
- Visibility is **session-state-bound**, not just account-bound: `CURRENT_ROLE() = NONE` hides a role-granted dependency and the drop lands; `SET ROLE` or `activate_all_roles_on_login` exposes it and the same drop refuses 3716 — the identical account, two different sessions, two different verdicts.
- Under proxy authentication `USER() != CURRENT_USER()`; both catalog and enforcement follow `CURRENT_USER()` (the effective account).
- The audited DROP's future executor is architecturally absent from the request model — no field names it; the only existing binding is the caller-declared conditional `PrivilegeAuditModeObservedAccountNewSession` ("expected", never verified).

## Decision

**Option C selected by the reviewer.** `Complete` remains unconditionally false; a clean zero-row dependency read stays observer-scoped evidence and never discharges `srs.column_dependencies`. No `VisibilityScope` field, no `Complete=true`, no `PublishAbsent`, no ordered dependency-absence publication, no SHOW GRANTS read added. D1–D3, D6–D9, D11 frozen; D4/D5/D10 not authorized; D12 deferred.

## Rationale

A zero-row read proves only that *the observing session's* account sees no blocking dependency. Promotion to absence would require the audited statement's execution principal — and its session visibility state — which the request model cannot observe or verify. Keeping the owed gap is the honest outcome, not a query-capability limit.

## Re-open conditions (future Option A work order only)

Caller-declared expected-execution-principal field on the request, explicitly conditional; the declaration must cover CURRENT_USER, active roles, role inheritance, proxying, and session visibility; grant observation and the dependency query share one pinned connection; documented fallback on declaration mismatch. Even then `Complete` may only claim the declared execution *visibility class*, never instance-wide absence.

## Public Contract

- `srs.column_dependencies` is owed on every SRS DROP whose premise bundle lacks a positive dependency witness — permanently, until a separate authorised work order changes the principal model.
- Positive witness rows (non-empty `Columns`, all SRID matching) remain the only consumable dependency evidence — the S1 refusal arm is unchanged.

## Deferred / Out Of Scope

- Observer-scoped completeness reporting (Option B) rejected: a `Complete` flag that never discharges would invite future misuse.
- `PublishAbsent` / ordered dependency publication: not authorised, deferred to any future Option A contract.
- Generalising the visibility findings beyond MySQL 8.0.46/8.4.10 or beyond the probed object kinds: explicitly not claimed.
