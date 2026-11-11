# T26-SERVER-DROP-PREMISE-01 — implementation plan (R1, all PLAN/UNEXECUTED)

Single recommended path only — no alternatives offered, no branches
deferred to reviewer choice. Nothing in this file is executed product
work; the R1 work order authorises docs-only.

## 1. The one recommended path

`observed_account_new_session` conditional supply:

- **Request entry** (proposal, naming per project style): an explicit
  mode field on the audit request declaring the semantic mode
  defined in execution-premise-contract.md §1 — observed
  `CURRENT_USER()` account as expected principal; expected execution
  context = a new session on the same target established after the
  observation; result conditioned on the relied-upon direct global
  grant still applying at permission-load time in that session.
  When the field is absent the mode is not selected and
  `server.drop_privilege` stays owed unconditionally.
- **Loader** (new, MySQL provider): on one dedicated `*sql.Conn`
  obtained via `DB.Conn(ctx)` (same acquisition/release discipline as
  `provider_resource_group_facts.go:36-41` — `conn.Close()` deferred),
  run `SELECT CURRENT_USER()` then `SHOW GRANTS FOR CURRENT_USER()`
  on that same actual connection; collect `ON *.*` lines via the
  existing `loadMySQLGrantLines`/`collectMySQLGlobalPrivileges`
  mechanism; positive witness iff exact-token `SUPER`, or `ALL
  PRIVILEGES` under the registered version-checked static-set proof.
- **Gate**: the loader is invoked only for an audited MySQL
  identifier-form DROP SERVER *and* the explicit mode selected —
  never for CREATE/ALTER SERVER, TiDB statements, or other families;
  no widening of reads for unrelated channels.
- **Error boundary**: one path, two layers. The complete public
  guarantee lives at the *application's* consumption boundary —
  at the call site where the application invokes the optional
  privilege loader, a non-nil error is uniformly wrapped with a
  fixed public text (fixed-prefix wrap that never re-splices the
  raw cause text) while `Unwrap()` preserves the cause, then enters
  the enrich error-return chain. Public bounding therefore never
  depends on every provider/SDK/custom loader implementation
  wrapping its own errors. The built-in MySQL loader may keep its
  own internal wrapping (fixed `Error()` + `Unwrap()` cause), but
  that is belt-and-braces inside the boundary — the application-side
  wrap is what guarantees the public text. `fmt.Errorf("...: %w",
  err)` alone does not bound public text, and the existing
  `ObjectResolver` wrap is not assumed to cover a session-based
  privilege loader. Query/scan/`rows.Err`/permission/cancellation
  failures all propagate as the bounded error; `errors.Is/As` stays
  reachable to the cause.

## 2. Data flow (deterministic, per R1-D)

```
request-level immutable raw evidence
    (observed account + ON *.* grant lines + target + source +
     request scope — never mutated)
        │
        ▼
explicit principal/target/context binding
    (mode selected? observed account bound? evidence's target
     source is this request's selected provider? context covers
     this batch?)
        │
        ▼
ordered statement scan → applicability range
    (walk statements in order; the range ends at the first authored
     statement that could change grants/roles — §3 markers)
        │
        ▼
per-statement consumable projection
    (each audited DROP SERVER gets its own projection:
     positive / unproven / suppressed — derived, never shared-
     mutable; later statements never rewrite earlier bindings)
        │
        ▼
shared DROP adjudication → rule/apply projections
    (one adjudication drives gap set, Message, Explanation,
     coverage, verdict, and ordered post-state — §4)
```

## 3. Ordered-scope classification seam (actual parser shapes)

Verified markers available today — no raw-SQL substring matching:

- `*ast.GrantStmt` → extracted `spec.DDL` with
  `Operation = spec.DDLOperationGrant` (`extractor.go:218,1971`).
- `*ast.RevokeStmt` → `Operation = spec.DDLOperationRevoke`
  (`extractor.go:220,2029`).
- `*ast.SetRoleStmt` → coverage-boundary feature `"set_role"`
  (`coverage_boundary.go:374-375`) — an unsupported entry, not a DDL.
- Related boundary features also terminating the range:
  `"grant_role"` (coverage_boundary.go:367), `"revoke_role"` (:369),
  `"grant_proxy"` (:365), `"set_default_role"` (:372).

Policy (registered): any of these authored forms earlier in the batch
conservatively ends the applicable range for subsequent statements.
This is the model's scope policy — it does **not** claim each such
statement immediately changes an existing session's static `SUPER`.
Whether an extracted `grant`/`revoke` DDL actually names `SUPER` is
not inspected in this slice; the termination trigger is the operation
kind alone — a deliberate over-approximation that stays on the
unproven side.

`Metadata.Instance` binding: enrichment attaches `InstanceFacts` per
statement (`metadata.go:153,172,184`), so each audited statement
sees the request-level evidence; the projection adds the per-statement
scope verdict on top — `Known`-style flags on the shared struct are
never mutated in place to retroactively change earlier bindings.
Minimal new seam required (proposal only): a per-statement derived
projection value produced by the ordered scan, consumed by the
adjudication — not a mutable field on shared `InstanceFacts`.

## 4. Consumption activation gate and shared adjudication (R1-E)

This round implements none of: privilege supply, gap consumption, or
OrderedAbsent. Future activation requires a separately authorised
product work order covering **all of** in one change:

- rule layer: gap set, finding `Message`, `Explanation`;
- apply layer: the shared adjudication and the ordered post-state;
- per-statement premise projections and their public-result
  applicability.

Forbidden shape: changing `missingServerFacts` alone to discharge the
last gap while `applyServerStatement` still publishes a barrier solely
on "privilege unwired" — the two must move together under one
adjudication.

The only activation chain kept:

```
raw positive witness → binding/scope → per-statement premise
    → same adjudication → rule/apply projections
```

Future consumer matrix (all rows derivable from the approved
b8a67e7b contract — no semantic redesign). For each row the matrix
names: the pre-state existence knownness, the premises the
adjudication actually needs, the resulting gap, and the post-state
direction — the shared adjudication computes gaps from owed
premises, never from a fixed per-row rule.

| pre-state | privilege premise | adjudication direction |
| --- | --- | --- |
| proven-present + consumable privilege | positive | strongest arm — proven-drop post-state publication becomes reachable only if every other premise of that arm also holds |
| existence unknown, plain DROP + consumable privilege | positive | existence stays unproven — privilege never proves existence; the unknown-existence arm's remaining premises stay owed |
| existence unknown, IF EXISTS + consumable privilege | positive | convergent arm per the approved contract: with identity proven, no blocking taint/barrier, and that arm's other premises met, both branches publish absent (present → drop → absent; absent → guard no-op → absent); pre-state existence itself stays unproven — the post-state converges on the two arms' joint result, not on a proven existence |
| batch-effects barrier + consumable privilege | positive | barrier stays — privilege never clears `batch_effects` |
| valid OrderedAbsent + plain DROP | not required for this arm | terminal refusal — the refusal adjudication is already complete; no separate privilege premise is owed for it; absent preserved |
| valid OrderedAbsent + IF EXISTS | per guarded arm | absent preserved under the guard; whether the privilege premise is still owed follows the guarded arm's own premises |
| no applicable terminal refusal + premise needed | unproven / suppressed | gap stays — current accepted behaviour |
| any + genuine read failure | error, not evidence | bounded error per G — never laundered into a gap, and terminal refusal never swallows an already-raised read failure |

Notes on derivation:

- A privilege witness never counts as existence proof, never clears
  `batch_effects`, never widens `R(A,B)`; an unproven existence does
  not by itself veto an approved convergent post-state either — the
  arm's registered premises decide.
- `coverage`/`verdict` derive from remaining gaps + the shared
  adjudication + existing aggregation rules — never a fixed
  "unchanged by privilege" claim.
- Two post-state outcomes stay distinct: publishing absent under a
  proven-drop or approved convergent arm with sufficient premises,
  versus preserving an already-published absent under
  refusal/guard.

Approving this supply document does not itself authorise consumption
activation or OrderedAbsent; no intermediate deliverable that produces
fields or queries with no consumer will be created.

## 5. Verification plan (PLAN/UNEXECUTED — nothing run this round)

- unit: A–G rows of the contract table, including C (mode unselected /
  principal unbound / context not covering → gap even with reader
  `SUPER`), E (successful empty / unrecognised representation →
  unproven, never `Known=true,false`, never denial), F (ordered-scope
  suppression with immutable earlier bindings), G (bounded error with
  `errors.Is/As` reachable);
- shared-adjudication matrix observations:
  - unknown `IF EXISTS` post-state convergence under the full
    applicable premises;
  - valid absent + plain DROP → terminal-refusal priority;
  - valid absent + `IF EXISTS` → state preservation and the guarded
    arm's privilege premise;
  - unproven/suppressed separated from genuine read errors;
  each checked item-by-item across gap, adjudication, post-state,
  and coverage derivation;
- one future application-layer consumption case: once a later work
  order authorises consumption, drive the shared adjudication with
  a lawful source and binding, and observe rule output, gap set,
  statement/aggregate coverage and verdict, plus successor-statement
  post-state bindings — supply-layer A–G unit checks do not
  substitute for this observation;
- unified error boundary: an optional-loader implementation returns
  a cause carrying a sensitive marker; public text stays fixed and
  marker-free while `errors.Is/As` still reaches the cause — no
  per-adapter repeated matrix needed;
- provider-driver tests for read-failure propagation;
- no denial-finding tests exist — denial adjudication does not exist;
- manifest/native/live-DB: out of scope this slice.
