# Decision: MySQL CREATE SERVER first-slice contract — shared adjudication + OrderedPresent (T26-SERVER-CREATE-CONTRACT-01)

Date: 2026-11-12
Status: **PROPOSED / Reviewer pending** — R3 targeted-closure
revision after CHANGES REQUIRED on candidates `3ec5f2c3` (R1) and
`e59f2452` (R2). This record is the contract document for a
docs-only convergence order; it recommends ONE first-implementation
domain and records no accepted product behavior. Nothing in
`internal/`, `pkg/`, or `testdata/` changed under it.
Related milestone/version: issue #105 T26 under parent issue #79
Related commits: docs-only candidates `3ec5f2c3` (R1, baseline
`9e21ace9`), `e59f2452` (R2); R3 revision (this change set)
Related docs: `docs/verification/t26-server-create-contract-01/`
(authoritative contract), `docs/decisions/2026-11-16-mysql-drop-server-conditional-privilege.md`
(accepted DROP sibling)

## Context

`CREATE SERVER` is the other half of the T26 server-lifecycle story.
The parser already recognizes the whole identifier/literal form and
the product deliberately pins every create to the `unaudited`
boundary: identifier-form statements record a batch write,
literal-name statements taint the channel, and neither acquires
semantics. The accepted DROP slice (`9dad37b8`, R2 PASS) proved the
mechanism this contract reuses: one request-level positive-witness
privilege observation, one shared per-statement adjudication, and an
ordered channel (`OrderedAbsent`) consumed before any provider read.
A create audit needs the same honesty — zero catalog rows still mean
unknown, never a free slot — plus a post-state the drop contract
never had: proven *presence* the ordered channel can hand to later
creates and drops.

## Decision

**Activation predicate (single gate).** A `CREATE SERVER` enters the
audited slice iff ALL hold: dialect MySQL; resolved target version in
the registered 8.0.x proof domain (version unknown/conflict/out-of-
domain → `unaudited` with `gate:"version"`); `NameIdent` (identifier-
form name); `NameSpellingRuneCount <= 64` (longer spellings stay
`unaudited` — silent truncation makes the effective catalog identity
unprovable); wrapper is **identifier-form** AND ASCII-fold-equals
`mysql` (string-literal wrapper spellings and non-`mysql` wrappers
stay `unaudited` — a coverage decision, never a parse rejection); the
whole-form recognizer already claimed the statement (unknown options
keys / trailing tokens stay parser-diagnostic exits). Qualification
is two-phase: parse-time shape gates (`applyCoverageBoundary`,
dialect + node only) run first; the version gate runs at enrich
entry on `batchState.resolvedVersion` — fixed before the loader gate
and before per-statement binding — so a version-failed statement is
marked `unaudited`/`gate:"version"` before it could ever trigger the
loader; malformed pin / product / pin-vs-observed conflicts exit
earlier as typed errors (`ErrInvalidTargetVersion`,
`ErrDialectProductMismatch`, `ErrTargetVersionMismatch`) and never
reach the gate; an unparseable banner reaches G2 as "unknown" and
fails it. Entering the domain is unconditional: mode and provider
presence decide only which evidence is available — an in-domain
CREATE with no mode/provider is audited `unverified`, NOT
`unaudited`. That is a registered public output change of the future
candidate.

**Privilege premise.** Same `SUPER` requirement as DROP → reuse the
approved request-level observation (`observed_account_new_session`,
`LoadMySQLServerDropPrivilege`, identical terminator vocabulary). The
loader gate widens once to the audited family — a CREATE-only request
newly triggers the hook: registered new call behavior, still at most
one raw observation per request. Per-statement projection is
operation-specific: `ServerCreatePrivilege` and its own owed fact
`server.create_privilege` — a satisfied `server.drop_privilege` is
never borrowed and a prior `conditional_audit` marker never
substitutes.

**Adjudication — priority-ordered, first match wins (NEW):**

- `Barrier` (A1): contamination / taint / relation barrier → owed
  exactly `{server.batch_effects}` — a CREATE-specific
  short-circuit; asserts nothing about privilege, options, or
  existence.
- `TerminalRefusal` (A2): provably-occupied slot — ordered
  `OrderedPresent` or a proven catalog row — + plain create → blocker,
  owed `∅`, `refused_present`; proven by occupancy alone; no native
  error number claimed; privilege/options never reach the question.
- `ProvenCreate` (A3): `OrderedAbsent`-same + consumable premise +
  clean options → owed `∅`, `conditional_create`, publishes
  `OrderedPresent`+conditional source.
- `Unproven` (A4): everything else in-domain → owed = exact set:
  `server.existence` iff no `OrderedAbsent`-same answered; plus
  `server.create_privilege` iff own premise non-consumable; plus
  `server.create_options` iff options non-clean. No open-ended
  "others".

No convergent or guarded arm — the grammar has no `IF NOT EXISTS`. A
first-statement create can never be a proven create: the slot-free
premise needs proven absence, and zero catalog rows are unknown.

**Unique post-state per arm.** ProvenCreate publishes
`OrderedPresent`(+`OrderedPresentConditional`) on the provably-same
entry, clearing its `absent`. Ordered-sourced TerminalRefusal leaves
the consumed `OrderedPresent` standing — no publish, no barrier.
Catalog-sourced TerminalRefusal publishes nothing — the only
OrderedPresent publisher is ProvenCreate, so a catalog-present
refusal never mints an ordered present. Unproven and
non-contaminated Barrier sources (taint / relation scan) clear the
entry's ordered state to the barrier shape — an unproven CREATE
therefore invalidates a prior `OrderedAbsent` on its provably-same
identity. The contaminated-batch Barrier source touches no entry at
all (`applyServerStatement` exits before any entry write; the
whole-batch guard blocks all consumption); the arm reads
`batchState.contaminated` directly — never projected through
`UnreadReason`, which contamination doesn't set. Invalidation
scopes: per-identity (same-relation writes), relation-unproven
(barrier before consumption), channel taint (literal-name unaudited
create clears every entry). A positive privilege witness never
clears a barrier.

**`OrderedPresent` / `OrderedPresentConditional`** (proposed
first-slice fields — not implemented; `json:"-"`, never serialized):
consumed inside `bindMySQLServer` at the existing `OrderedAbsent`
position (merged ordered-state scan), before provider/cache; read by
later creates (refusal input) and later drops (proven-present
existence answer — new DROP consumer behavior of the future CREATE
candidate, never a backfill into the accepted DROP PASS); consumers
read the ordered field directly — catalog `Exists`/`Read`/`Status`
are never faked. The only existing OrderedAbsent publisher is
conditional, so every `OrderedPresent` published today carries the
conditional source; the unconditional variant is a field-honest
placeholder.

**`conditional_audit` projection:** emitted iff the arm's own
consumable premise fed the conclusion OR the arm consumed an ordered
state carrying conditional source; never projected by a
barrier-shadowed (unconsumed) source; never discharges the
statement's own owed premise.

**Coverage/verdict projection** (rule enabled, default levels):
ProvenCreate/TerminalRefusal → statement coverage `complete`
(refusal's blocker drives batch `reject`); Unproven/Barrier →
`unverified` (evidence-gap floor → `review`). Aggregate coverage and
batch verdict use the existing aggregation rules unchanged — a
single CREATE's projection never substitutes for the batch result.

**OPTIONS/wrapper internal evidence (proposed, not implemented).**
`RangeState` per numeric option (`in-range|out-of-range|unproven`)
computed at parse time while the literal is visible — out-of-range
past 0–9999, unproven on unprovable magnitude (never silently clean);
`WrapperIdent` records the wrapper's identifier-vs-literal quoting
category at parse time. "Clean options" = documented keys + all
numeric `RangeState=in-range` + no duplicates — and nothing more;
remote connectivity and FEDERATED availability are never claimed.
Missing USER/HOST/DATABASE → audit-model completeness **notice**
(the manual makes them mandatory for connecting, not for the create),
not an owed fact and not a claimed native warning.

**Identity relations unchanged:** byte/ASCII-fold equal ⇒ same;
everything else unproven; `serverRelDistinct` empty provable domain.
The manual's "case-insensitive" is a catalog property, not license to
widen relations.

## Public Contract

- No new Go methods/params proposed — but public behavior changes are
  registered, not denied: in-domain create moves `unaudited`→audited
  on the default path too; new outcome tokens
  (`conditional_create`/`refused_present`) and fact keys
  (`server.create_privilege`/`server.create_options`) appear in
  finding metadata; CREATE-only requests newly trigger the loader
  hook; coverage/verdict derive per the adjudication table under the
  declared rule-enablement premise (custom policies remap levels as
  today).
- Public output never carries the server name, wrapper, option
  keys/values, observed account, or grant text; `PublicSQL()` already
  emits `CREATE SERVER <redacted>`; the parser stores option keys and
  value kinds only — never literal values.
- `testdata/ddl-golden/T26.json` `create_server`/`mixed-batch` cells
  migrate `unaudited`→audited **in the future implementation
  candidate**, under its declared policy — registered here, never
  retroactively edited. `ALTER SERVER` stays outside this slice.

## Deferred / Out Of Scope

- ALTER SERVER, SRS adjudication, `ALTER INSTANCE`; literal-name
  create audit; non-`mysql`/literal wrappers; `serverRelDistinct`
  extension; full string-literal decode — unchanged deferred list.
- FEDERATED availability, remote connectivity, native error numbers,
  and any claim the database actually executed or refused a create —
  every arm is the audit model's adjudicated projection only.
- Product implementation itself: this contract authorizes nothing; a
  separate execution order must activate it.
- No push, merge, release, or issue close.

## Verification Evidence

Docs-only round. Actual checks: `git status --short` clean on tracked
files at each commit; `git diff --check` executed clean on the R1
candidate (`3ec5f2c3`) and re-run clean on the R2 candidate;
source-consistency review of every cited file against the baseline
tree (including `bindMySQLServer`'s two distinct early exits —
contamination → bound unvouched, taint/barrier →
`MetadataUnreadBatchEffects` — reused verbatim; the only proposed new
bind behavior is the merged `OrderedPresent` consumption scan);
manual re-checked this round (SUPER, quoted wrapper allowed, PORT
0–9999, no `IF NOT EXISTS`, USER/HOST/DATABASE phrased as
connection requirements). No `go test`, build, CLI, wrapper, native
SQL, Docker, or database operation ran or is claimed. The full
per-case verification matrix lives in
`docs/verification/t26-server-create-contract-01/create-server-contract.md`
under PLAN / UNEXECUTED, groups A–J.

## Consequences

- If accepted, the future execution order touches:
  `coverage_boundary.go` (G1–G6 gate replaces the blanket unaudited
  pin), `mysql_server.go` (`WrapperIdent`/`RangeState` parse-time
  derivation), `spec/server.go` + new `server_create_premise.go`,
  `metadata.go` (adjudication attach), `server_drop_premise.go`
  (family-wide loader gate + create premise projection),
  `batch_state_server_srs.go` (merged ordered-state scan),
  `batch_state_server.go` (`present`/`presentConditional` entry
  fields + create apply arm), `mysql_server_rules.go` (create-arm
  surface), and `T26.json` expectations — each change carrying its
  own regression point, none re-verifying the DROP PASS.
- A later create or drop consuming `OrderedPresent` is a new
  compatibility surface with its own verification — not a re-check
  of the R2 PASS.
- `OrderedPresent` is channel-internal; serializing it externally is
  a contract break, same as `OrderedAbsent`.

## Links

- Contract: `docs/verification/t26-server-create-contract-01/create-server-contract.md`
- Index: `docs/verification/t26-server-create-contract-01/README.md`
- Sibling accepted record: `docs/decisions/2026-11-16-mysql-drop-server-conditional-privilege.md`
- Official source: https://dev.mysql.com/doc/refman/8.0/en/create-server.html
