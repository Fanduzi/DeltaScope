# T26-SERVER-DROP-PREMISE-01 — acceptance (docs-only contract PASS)

PASS registered by reviewer ruling `T26-SERVER-DROP-PREMISE-01 /
R3` on candidate `cfd34782`; all of R2-A, R2-B, R2-C, S1, S2, and the
PLAN synchronisation items are closed. This file records the closeout
required by work order `T26-SERVER-DROP-PREMISE-01-CLOSEOUT-01`.

## A. PASS scope

Only the approved documentation contract is registered — no product
capability:

- `observed_account_new_session` explicit conditional-audit mode;
- the registered positive-proof domain of direct global-grant
  witnesses (`SUPER` or version-checked `ALL PRIVILEGES` on `*.*`);
- three outcomes only: positive witness / unproven / bounded error;
- target binding = this request's selected provider source;
- request-level immutable raw evidence with independent per-statement
  consumable projections;
- the unified public error boundary at the application's consumption
  point (fixed text + `Unwrap()` cause, independent of each loader
  implementation's own wrapping);
- the consumption-activation condition: rule output, shared
  adjudication, and required post-state must be authorised together;
- the approved convergent arm, terminal-refusal priority, and
  read-error separation;
- the corresponding future verification plan (all PLAN/UNEXECUTED).

## B. Identity and chain

Documentation chain: `244aace5` → `e42eeafe` → `be729d61` →
`cfd34782`.

- `244aace5`: archive baseline of the prior product phase
  (T26-SERVER-DROP-01 archive);
- `e42eeafe`: original document candidate (CHANGES REQUIRED, R1-A..F);
- `be729d61`: R1 document revision (CHANGES REQUIRED,
  R2-A/B/C + S1/S2);
- `cfd34782`: the approved R2 document revision —
  commit `cfd34782b94d12342a021b846d51f41a5682d2e3`,
  tree `d8e41edc5e65b1c55b8051beb0e611449ab210d4`,
  parent `be729d61e1b6e52ffb0e68e06c90436d60e1a199`.

Preserved identities: product/test acceptance `2ee906d5`; that
phase's archive `244aace5`; approved semantics contract `b8a67e7b`.
Neither `cfd34782` nor this closeout HEAD is a product-tested
identity — the approved object is a documentation contract only.

## C. Closure basis

- Prior-round items stayed closed: explicit principal/context mode
  with unselected-default gap retention; observed-account grant vs
  request applicability vs unobserved future execution kept distinct;
  positive-only direct global proof domain; no denial inference, no
  role augmentation, no offline privilege bundle; immutable raw
  evidence + per-statement projections; real semantic classification
  driving ordered scope termination; activation requires rule +
  shared adjudication + post-state together; everything stays
  PLAN/UNEXECUTED.
- R2-A: consumer matrix corrected — the unknown-`IF EXISTS`
  convergent arm is registered separately (existence still unproven,
  post-state convergent on the two arms' joint result), terminal
  refusal priority restored for valid OrderedAbsent + plain DROP,
  guarded `IF EXISTS` registered, and genuine read failures singled
  out as bounded errors. Pointer: `implementation-plan.md` §4.
- R2-B: unified public error boundary fixed at the application's
  optional-loader call site — fixed public text, `Unwrap()` cause,
  enrich error chain; loader-internal wrapping stays inside the
  boundary. Pointers: `implementation-plan.md` §1 Error boundary;
  `source-excerpts.md` B.6, C.4.
- R2-C: `SET ROLE`/`GRANT`/`REVOKE` official semantics split into
  three registered facts plus the `set-role.html` reference; the
  range-termination policy stays a model policy, not a doc claim.
  Pointer: `source-excerpts.md` §D.
- S1: the shared `Metadata.Instance` carrier may hold raw evidence;
  per-statement consumption is decided by the independent projection
  — no gap cleared on the shared witness alone. Pointer:
  `source-excerpts.md` C.2.
- S2: target binding defined as evidence from this request's
  selected provider — no standalone "target matches", no instance-ID
  comparison claimed. Pointers: `execution-premise-contract.md` §2
  condition 2; `implementation-plan.md` §2 data flow.
- Verification plan synchronised (matrix observations, future
  application-consumption case, unified error-boundary marker test)
  — all PLAN/UNEXECUTED, nothing run. Pointer:
  `implementation-plan.md` §5.

## D. Roles and evidence boundary

- Worker performed local read-only source verification, read-only
  official-document checks, document revision, and local commits.
- Reviewer performed material review of submitted receipts and
  read-only official-document checks; the reviewer did not
  independently read the worktree, verify Git objects, or execute
  project commands.
- This phase produced no tests, builds, CLI probes, or database runs;
  every future implementation and verification item is
  PLAN/UNEXECUTED.

## E. Capabilities not activated

This PASS does not implement or activate:

- the request mode field or a production privilege loader;
- production consumption of `server.drop_privilege`;
- `OrderedAbsent` representation or publication;
- role augmentation, negative privilege proof, or offline privilege
  bundles;
- CREATE/ALTER SERVER lifecycle, SRS semantics, or T27.

Any later product work order must explicitly authorise consumption
and cover supply, binding, per-statement premises, the shared
adjudication, rule output, and post-state in one change. This PASS
and the closeout archive do not constitute that authorisation.
