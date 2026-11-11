# Decision: MySQL CREATE SERVER first-slice contract — shared adjudication + OrderedPresent (T26-SERVER-CREATE-CONTRACT-01)

Date: 2026-11-12
Status: **PROPOSED / Reviewer pending.** This record is the contract
document for a docs-only convergence order; it recommends ONE
first-implementation domain and records no accepted product behavior.
Nothing in `internal/`, `pkg/`, or `testdata/` changed under it.
Related milestone/version: issue #105 T26 under parent issue #79
Related commits: docs-only candidate on baseline `9e21ace9`
Related docs: `docs/verification/t26-server-create-contract-01/`
(authoritative contract), `docs/decisions/2026-11-16-mysql-drop-server-conditional-privilege.md`
(accepted DROP sibling)

## Context

`CREATE SERVER` is the other half of the T26 server-lifecycle story.
The parser already recognizes the whole identifier/literal form
(`mysql_server.go` claims `CREATE SERVER name FOREIGN DATA WRAPPER
wrapper OPTIONS (…)`) and the product deliberately pins every create —
regardless of shape — to the `unaudited` boundary: identifier-form
statements record a batch write, literal-name statements taint the
channel, and neither acquires semantics. T26-SERVER-DROP-EXECUTION-01
(accepted `9dad37b8`, tree `eb544128`) proved the DROP sibling's
mechanism: one request-level positive-witness privilege observation,
one shared per-statement adjudication, and an ordered channel
(`OrderedAbsent`) that a later statement consumes before any provider
read. A create audit needs the same honesty — a catalog read's zero
rows still mean unknown, never a free slot — plus a post-state the
drop contract never had: proven *presence* the ordered channel can
hand to later creates and drops.

## Decision

Recommended first-implementation domain — MySQL 8.0.x only, and only:

- identifier-form `CREATE SERVER` whose authored spelling stays ≤64
  runes (the catalog silently truncates longer names; a >64-rune
  spelling's effective identity is unprovable);
- wrapper spelled so it ASCII-folds to `mysql` (the only wrapper the
  manual documents; there is no provable wrapper registry, so other
  spellings keep the unaudited boundary — never a parse rejection);
- OPTIONS with ≥1 documented member; every key/value type is already
  parser-enforced. Duplicate keys or a numeric literal outside the
  documented 0–9999 bound keep the statement audited but owe
  `server.create_options` — the effective option set is unproven, so
  the create can never publish. Missing `USER`/`HOST`/`DATABASE`
  members are a notice-level completeness note, not a refusal source
  (the manual treats them as mandatory for *connecting*, while the
  statement itself stores blank/zero defaults).
- `OWNER` is a documented no-op: it contributes to option-presence
  facts only and never gates the arm.

Privilege premise: CREATE requires the same `SUPER` as DROP, so the
first slice **reuses the already-approved request-level observation**
(`observed_account_new_session`; `LoadMySQLServerDropPrivilege`
loader; same terminator vocabulary). What stays distinct per the work
order: a per-statement `ServerCreatePrivilege` projection
(unproven/consumable/suppressed) and its own fact key
`server.create_privilege` — a satisfied `server.drop_privilege` on a
sibling statement is never borrowed, and a prior drop's
`conditional_audit` marker never substitutes for the create's own
premise. The loader gate widens once: one observation per request for
the whole audited family, never per statement.

Adjudication (one shared table per audited statement; NEW proposed
vocabulary marked):

- `ServerCreateArm`: `ProvenCreate` — a provably-same conditional
  `OrderedAbsent` (the only proven-absence source) + consumable
  privilege + clean options → publishes `OrderedPresent` and owes
  nothing; `TerminalRefusal` — provably-occupied slot (ordered
  `OrderedPresent` or a proven catalog row) + plain create, a logical
  impossibility proven by the evidence alone — blocker, owes nothing,
  claims no native error number and no MySQL error ordering;
  `Unproven` — owed `{server.existence}`, `{server.create_privilege}`,
  `{server.create_options}` in the combinations the premises
  actually lack; `Barrier` — `blocked_batch_effects`, owes
  `{server.batch_effects}`.
- No convergent or guarded arm exists: the MySQL 8.0 grammar has no
  `IF NOT EXISTS` for `CREATE SERVER`.
- Direct consequence: a *first-statement* create can never be a
  proven create — the slot-free premise needs proven absence, and
  zero catalog rows are unknown by the approved contract. This is
  stated plainly so no future arm quietly widens "absent".
- Outcome tokens: NEW `conditional_create`, `refused_present`; reused
  `blocked_batch_effects`, `unverified`. `conditional_audit` marks a
  statement's own conditional arm and arms consuming a
  conditionally-published ordered state, symmetric with DROP.

`OrderedPresent` / `OrderedPresentConditional` (NEW, `json:"-"`,
never serialized):

- produced only by `ProvenCreate` on a provably-same identity on a
  clean channel; mutually exclusive with `OrderedAbsent` — a proven
  drop publish replaces present→absent, a proven create publish
  replaces absent→present;
- consumed inside `bindMySQLServer` at the same position
  `OrderedAbsent` sits — after contamination/taint/barrier, before
  provider or cache — by later creates (refusal) and later drops
  (proven-present pre-state);
- cleared to the barrier shape by every non-publish arm and every
  unaudited/unknown effect — the work order's rule "an unknown CREATE
  must invalidate a prior `OrderedAbsent`" falls out of the same
  apply path the drop contract already uses;
- `OrderedPresentConditional` carries the publishing arm's
  conditional source; consumers inherit `conditional_audit` from it.
  No unconditional publisher exists today; the field is honest about
  that rather than implying one.

Identity relations unchanged: byte-equal or ASCII fold-equal within
the registered ASCII-CI subset ⇒ same; everything else unproven;
`serverRelDistinct` keeps its empty provable domain. The manual's
"case-insensitive" is a catalog property, not license to widen the
relation domain — CREATE inherits the same conservatism, and barriers
still precede every provider/cache/ordered consumption.

## Public Contract

- No new public API proposed: `PrivilegeAuditMode`,
  `MySQLServerDropPrivilegeProvider`, and `Request` already admit the
  family. Outcome tokens are finding metadata, not API.
- Public output never carries the server name, wrapper, option
  keys/values, observed account, or grant text;
  `ServerDeclaration.PublicSQL()` already emits
  `CREATE SERVER <redacted>`; the parser stores option keys and value
  *kinds* only — never literal values.
- Default (no mode, no provider) behavior is unchanged: the create
  stays a boundary statement exactly as today — the legacy shape is
  verified field-by-field against the current manifest expectations
  in the future candidate, not relabeled here.
- `testdata/ddl-golden/T26.json` `create_server` / `mixed-batch`
  expectations move from `unaudited` to audited **in the future
  implementation candidate** — registered here, never retroactively
  edited.

## Deferred / Out Of Scope

- ALTER SERVER, SRS adjudication, `ALTER INSTANCE`; literal-name
  create audit; non-`mysql` wrappers; `serverRelDistinct` extension —
  unchanged deferred list, same reasons as the DROP sibling.
- FEDERATED availability, remote connectivity, native error numbers,
  and any claim the database actually executed or refused a create —
  every arm is the audit model's adjudicated projection only.
- Product implementation itself: this contract authorizes nothing; a
  separate execution order must activate it.
- No push, merge, release, or issue close.

## Verification Evidence

Docs-only round. Actual checks: `git status --short` clean on tracked
files at baseline `9e21ace9`; `git diff --check` clean at commit time;
source-consistency review of every cited file against the baseline
tree; syntax/semantic table checked against the MySQL 8.0 Reference
Manual §15.1.18 text fetched this round. No `go test`, build, CLI,
wrapper, native SQL, Docker, or database operation ran or is claimed.
The full per-case verification matrix lives in
`docs/verification/t26-server-create-contract-01/create-server-contract.md`
under PLAN / UNEXECUTED.

## Consequences

- If accepted, the future execution order touches:
  `coverage_boundary.go` (narrow the unaudited pin to the accepted
  slice), `spec/server.go` + new `server_create_premise.go`,
  `metadata.go` (adjudication attach), `server_drop_premise.go`
  (family-wide loader gate + create premise projection),
  `batch_state_server*.go` (`OrderedPresent` produce/consume/
  invalidate; `batchServerEntry` gains the present leg),
  `mysql_server_rules.go` (create-arm rule surface), and
  `T26.json` expectations — each change carrying its own regression
  point, none re-verifying the DROP PASS.
- A later create arm that consumes `OrderedPresent` as proven-present
  input (or a drop doing so) is a new compatibility surface with its
  own verification, not a re-check of the R2 PASS.
- `OrderedPresent` is channel-internal; serializing it externally is
  a contract break, same as `OrderedAbsent`.

## Links

- Contract: `docs/verification/t26-server-create-contract-01/create-server-contract.md`
- Index: `docs/verification/t26-server-create-contract-01/README.md`
- Sibling accepted record: `docs/decisions/2026-11-16-mysql-drop-server-conditional-privilege.md`
- Official source: https://dev.mysql.com/doc/refman/8.0/en/create-server.html
