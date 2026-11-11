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
- **Error boundary**: the loader returns a typed error whose
  `Error()` text is a fixed bounded message carrying no underlying
  sensitive content, with `Unwrap()` preserving the cause so
  `errors.Is/As` still reach it. Query/scan/`rows.Err`/permission/
  cancellation failures on the required channel all propagate as this
  bounded error. `fmt.Errorf("...: %w", err)` alone does not bound
  public text — the wrapper lives at the loader's return edge, and
  the existing `ObjectResolver` wrap is not assumed to cover a
  session-based privilege loader; the plan must name the concrete
  seam where enrichment surfaces it (metadata enrich failure path,
  same class as existing `Metadata.*` read errors).

## 2. Data flow (deterministic, per R1-D)

```
request-level immutable raw evidence
    (observed account + ON *.* grant lines + target + source +
     request scope — never mutated)
        │
        ▼
explicit principal/target/context binding
    (mode selected? observed account bound? target matches?
     context covers this batch?)
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
b8a67e7b contract — no semantic redesign):

| pre-state | privilege premise | adjudication direction |
| --- | --- | --- |
| proven-present + consumable privilege | positive | strongest arm — proven-drop post-state publication becomes reachable only if every other premise of that arm also holds |
| existence unknown, plain DROP + consumable privilege | positive | existence gap still owed — privilege does not prove existence; outcome stays on the unknown-existence arm |
| existence unknown, IF EXISTS + consumable privilege | positive | same — IF EXISTS arm unaffected by privilege witness alone |
| batch-effects barrier + consumable privilege | positive | barrier stays — privilege never clears `batch_effects` |
| any + privilege unproven / suppressed / error | — | gap stays (or bounded error) — current accepted behaviour |
| existing valid OrderedAbsent under refusal/guard | any | absent state preserved under refusal/guard — never republished by privilege evidence |

`coverage`/`verdict` are derived from remaining gaps + the shared
adjudication + existing aggregation rules — never a fixed
"unchanged by privilege" claim. Two distinct post-state outcomes stay
separate: publishing absent under a proven-drop/convergent arm with
sufficient premises, versus preserving an already-published absent
under refusal/guard.

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
- provider-driver tests for read-failure propagation;
- no denial-finding tests exist — denial adjudication does not exist;
- manifest/native/live-DB: out of scope this slice.
