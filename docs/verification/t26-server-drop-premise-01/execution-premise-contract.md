# T26-SERVER-DROP-PREMISE-01 — execution-premise contract

Contract for the last unwired premise of the accepted DROP SERVER
slice: `server.drop_privilege`. Answers the six required questions,
fixes the single recommended evidence source, and defines when a
privilege fact may discharge the gap — everything else stays unproven.

## 1. Core answers

**Q1 — is the expected execution principal represented?** No.
`Request` (internal `service.go:40`, public `pkg/deltascope/audit.go:101`)
carries no executor/principal/account field; `TargetIdentity` binds
`DeploymentEvidence` to an *instance*, not an account (source-excerpts
A.1, A.2). No executor/principal/session type exists (A.3). The only
existing principal model is the T24 bundle convention: "the
connection's own account, which is the subject that would execute the
audited lifecycle statements" (B.1) — a declared audit applicability
condition, not a verified binding.

**Q2 — which relationships are code-guaranteed?**
- `SHOW GRANTS FOR CURRENT_USER()` on the provider session provably
  answers the *metadata reader's own account* (doc + B.2) — code-guaranteed.
- reader == declared executor: **no declared executor exists** — nothing
  to compare.
- reader == future actual executor: **unproven** — an audit-time read
  cannot observe future execution. The honest contract phrase is the
  T24 one: the audit's applicability condition is that the audited
  statements would execute under the same session/account the provider
  reads on. Under any caller that connects the provider with one
  account and executes the audited SQL with another, the privilege
  fact does not apply — that is a caller-side contract, not an
  enforceable code binding today.

**Q3 — can the T24 premise-bundle be reused as-is?** No, with a
narrow gap. Reusable mechanics: `loadMySQLEffectivePrivileges` (B.2 —
base `CURRENT_USER` grants + `CURRENT_ROLE()`-driven `USING`
augmentation + positive-only completeness degrade), the
`collectMySQLGlobalPrivileges` `ON *.*` exact-token filter (B.3), the
dedicated-session read discipline and propagate-don't-launder error
contract (B.1, B.5). Not reusable as-is: `LoadMySQLResourceGroupFacts`
bundles unrelated channels (read-only vars, occupancy, feature probe)
behind a resource-group family gate (B.5, B.6); `MySQLReadOnlyExempt`
answers `SUPER||CONNECTION_ADMIN||ALL PRIVILEGES` — a different
question than DROP SERVER's exact `SUPER` (B.3); `DeploymentEvidence`
carries only T24 members and fires only inside the resource-group gate
(B.4, B.5). **Missing capability**: a family-scoped privilege supply —
same grant channels, narrower surface, its own fact slot.

**Q4 — minimal evidence source for SUPER?** The recommended source is
the provider session's effective-grant read (B.2 mechanics):
`SHOW GRANTS FOR CURRENT_USER()` plus `USING` per `CURRENT_ROLE()`-reported
active role, filtered to global `*.*` scope, exact-token `SUPER` or
`ALL PRIVILEGES`. Positive witness only: a present grant proves the
premise; an absent/unreadable grant never proves denial unless the
complete set was read. It observes the executing-subject assumption of
B.1 — the same dedicated session the catalog reads run on.

**Q5 — missing / mismatched / out-of-scope / read-failure:** see §3
table — gap stays owed for every non-positive outcome; read errors
propagate through the existing bounded chain, never degrade into gap
or refusal.

**Q6 — which seams must change, and under what condition may a proof
be consumed?** See implementation-plan.md §2–§3; consumption requires
all of §2's binding conditions simultaneously.

## 2. Consumption contract for privilege evidence

A `server.drop_privilege` proof is consumable only when ALL of:

1. **Source** — the evidence comes from the provider session's own
   effective-grant channels (base `CURRENT_USER` + active-role `USING`),
   read once per request on the dedicated connection, or — proposal
   only — a caller-sourced bundle passing the full
   `DeploymentEvidence`-style strict binding (B.4). A bare
   caller/policy boolean is never a proof.
2. **Target binding** — provider reads land on the connection's own
   audited target by construction; a caller bundle must additionally
   bind to the observed `@@server_uuid` online or the declared
   `TargetIdentity` offline, exactly as B.4 adjudicates. One target's
   proof never covers another.
3. **Principal binding** — the proof names the executing subject as
   the provider session's `CURRENT_USER`. It discharges the gap only
   under the audit applicability condition "audited statements
   execute under this same session/account" (B.1 convention). When a
   future request-level executor declaration exists and does not match
   the read subject, the proof does not apply — gap stays.
4. **Product/version applicability** — MySQL dialect only; the grant
   channels used are stable across the supported 8.x surface
   (`CURRENT_ROLE`/`USING` degrade to positive-only where unavailable,
   matching B.2). The contract does not interpret grant formats outside
   the parsed `ON *.*` token set.
5. **Scope & timeliness** — request-scoped (batch-level) fact: one
   read covers the request's statements under the same-session
   condition. An authored `SET ROLE`, `GRANT`, or `REVOKE` inside the
   batch would change the premise for later statements — today those
   are unaudited forms; if a batch contains any of them, the
   request-scoped privilege fact must be withheld for subsequent
   statements (barrier-style suppression), never extrapolated.
6. **Immutability & merge** — fills only still-unproven slots,
   per-request, never cached across requests, never overwrites a
   proven answer (same discipline as `mergeMySQLResourceGroupFacts`,
   B.4-adjacent). Prior statement bindings stay untouched.
7. **Independence** — the privilege fact discharges only
   `server.drop_privilege`. It never proves existence, never widens
   the name-relation proof domain, and never clears
   `server.batch_effects`, entry barriers, or channel taint.
8. **External visibility** — public output keeps the fixed
   fact/reason vocabulary (`server.drop_privilege` present or
   discharged). Account names, grant text, role names, and binding
   detail stay internal evidence — never echoed into findings, gaps,
   errors, or diagnostics.

## 3. Adjudication table (implementation contract, not run results)

| case | input fact | consumable? | binding result | gap/error effect | verification seam (PLAN) |
| --- | --- | --- | --- | --- | --- |
| A. valid positive | complete or partial grant read carrying `SUPER`/`ALL PRIVILEGES` on `*.*`, same session, target matches | yes | `MySQLDropPrivilege=true, Known=true` (proposal name) | `server.drop_privilege` discharged | unit: fact adjudication; provider-driver: stub grant lines |
| B. no evidence | no provider, provider lacks the loader, offline without a binding bundle | no | slot unproven | gap stays | unit: unvouched path |
| C. reader/executor mismatch | reader grant proven, but a future declared executor differs from the read subject | no | proof recorded, not applied | gap stays | unit: declared-mismatch case (post-introduction) |
| D. target/applicability mismatch | caller bundle for another target/version/product | no | bundle ignored whole | gap stays | unit: binding-contract matrix (reuse B.4 shape) |
| E. no positive witness / unhandled form | complete read, no `SUPER` — proven denial; or partial read, no `SUPER` — stays unproven; role line unparseable — skipped | denial: yes for refusal adjudication; partial: no | `Known=true, false` only on complete read; otherwise unproven | denial → refuses with the existing premise-refusal shape; unproven → gap stays | unit: complete/partial matrices |
| F. scope not covering | batch contains unaudited `SET ROLE`/`GRANT`/`REVOKE` before this statement | no | request fact withheld for that statement | gap stays | unit: ordered suppression (barrier analogy) |
| G. read failure | driver/scan/permission error on the grant channels | n/a — error, not evidence | propagates bounded (`fmt.Errorf` chain / adapter-bounded text), `errors.Is/As` reachable | error, never gap, never cached | unit + provider-driver: error injection |

Notes:

- E's proven-denial arm exists only when the *complete* effective set
  was read (`grantsComplete` in B.2); a partial read never denies.
- F is conservative withholding — the premise may still hold, but the
  batch's own authored statements can change it; per-statement
  re-proof is out of this slice's scope.
- G follows the premise-bundle error precedent (B.5): propagate with
  the causal chain; public adapters keep their existing bounded-text
  boundary.
