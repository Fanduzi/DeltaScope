# T26-SEMANTICS-01 — acceptance record

Reviewer ruling: **APPROVED — T26-SEMANTICS-01 documentation and
first-slice contract accepted** (issued in the T26 review thread after
the R4 concentrated review).

## Accepted object

| item | identity |
|---|---|
| accepted document commit | `b8a67e7b5f7e62a825adace06113a233240d28df` |
| document tree | `787092acc65b51268ff08ab75902545ebcfb7ef4` |
| parent | `30e6afb82fce6c24fdd84800b6c273c21ee16bf6` |
| tested product (unchanged) | `7c2c1f23d41960ef5f8d5efeb51c7fd166eb9a8e` |

Scope of the acceptance: the semantic-contract convergence and the
first-slice (`DROP SERVER`, MySQL, `NameIdent` forms) design — NOT a
product verification. The documents carry no new product PASS or
native PASS.

## What the accepted contract establishes

- `DROP SERVER` consumes no connection-payload correctness or remote
  reachability; `server.drop_privilege` is a bounded gap on every
  non-terminal-refusal drop arm → `review` / `unverified`.
- No proven `absent` is published from `present`/`unknown` — an
  audited drop with unproven privilege leaves an unproven barrier
  shadowing the audit-time catalog present.
- Ordered channel: six-step `resolveServer` — whole-batch
  contamination → `serverChannelTainted` → relation scan over all
  related entries AND write records → barrier return → valid ordered
  proof → provider/cache. Barriers never call the resolver; a
  provably-same hit never short-circuits the scan; recorded unaudited
  writes are unknown effects, never consumable facts.
- `R(A,B)` minimal conservative contract: byte-identical →
  provably-same; registered SERVER ASCII case-insensitive subset +
  ASCII-fold-equal → provably-same; everything else → unproven.
  `provably-distinct` is a registered class with an empty current
  provable domain.
- Transition table: proven absent is preserved under refusal / guard /
  privilege-failure arms; only an actual write relation or channel
  taint invalidates it.
- SRS rows 4/5 register named implementation boundaries for missing
  decode capability — not same-origin external-fact gaps; the global
  merge contract keeps single-check judgments distinct from
  statement-level outcomes.
- Provider snapshot and prior statements' bound metadata are
  immutable; ordered entries are channel-owned views.

## Explicitly not accepted / still open

- SRS 33 rows remain design proposals with their `proposal`/`PENDING`
  statuses — none approved by this acceptance.
- The `OrderedAbsent` representation, the successful `absent` publish
  path, and every arm consuming ordered-absent (`D6`/`D7` `[dep]`) —
  future premise-supply-stage contract only.
- The privilege/grants supply for `server.drop_privilege` — future
  stage; not implemented.
- §E verification plan — a design plan (21 cases, PLAN/UNEXECUTED);
  implementation results bind to a new product candidate separately.
- CREATE/ALTER SERVER audit semantics, SRS semantics, ALTER INSTANCE
  expansion, native verification — all still open.

## Registered evidence boundary (accepted cost)

Under the minimal conservative `R(A,B)` contract, successors after a
SERVER write may receive `server.batch_effects` on objects that could
actually have distinct names — relation proof must come from a future
scoped identity basis, not from fold keys or heuristics.

## Round ledger

| round | document commit | ruling |
|---|---|---|
| initial | `8fc066571ab7946a8b42f79137fd832d065fc960` | CHANGES REQUIRED (R1) |
| R1 revision | `5640954ee760d92011677a8b37add64579b81ab5` | CHANGES REQUIRED (R2) |
| R2 revision | `ac5433373b8d2e04e5d400a618d8dad669c4a3b9` | CHANGES REQUIRED (R3) |
| R3 revision | `30e6afb82fce6c24fdd84800b6c273c21ee16bf6` | CHANGES REQUIRED (R4) |
| R4 revision | `b8a67e7b5f7e62a825adace06113a233240d28df` | **APPROVED** |

Next stage: `T26-SERVER-DROP-01` — implement the converged minimal
slice. Historical records (`16eba8a0` R6, `7c2c1f23` provider round,
`dfa7852e` archive) keep their identities; nothing above rewrites them.
