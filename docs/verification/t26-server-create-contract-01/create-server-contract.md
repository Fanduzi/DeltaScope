# T26-SERVER-CREATE-CONTRACT-01 — CREATE SERVER first-slice semantic contract

Status: PROPOSED / Reviewer pending — R3 targeted-closure revision
after CHANGES REQUIRED on candidates `3ec5f2c3` (R1) and `e59f2452`
(R2). Type: docs-only contract convergence. This-round baseline:
`e59f2452` (tree `3537f2c9`), branch
`milestone/mysql-tidb-ddl-completion`. Parent: T26 / issue #105 under
issue #79.

This contract defines ONE recommended first-implementation domain for
audited MySQL `CREATE SERVER`: the activation predicate, the exact
evidence each arm consumes, one mutually-exclusive adjudication, and
the unique ordered post-state per arm. It implements nothing; a
future independently-issued execution order activates it.

Approved boundaries carried in unchanged from the accepted DROP slice
(`9dad37b8`, tree `eb544128`, R2 PASS): conservative identity
relations, zero-row = unknown (never absent), positive-only
direct-global-grant privilege witness, `observed_account_new_session`
scope, no future-execution claims.

## A. Syntax, version support, and the activation predicate

Official basis: MySQL 8.0 Reference Manual §15.1.18 CREATE SERVER
(dev.mysql.com/doc/refman/8.0/en/create-server.html, fetched this
round). Documented grammar and semantics the contract consumes:

- `CREATE SERVER server_name FOREIGN DATA WRAPPER wrapper_name
  OPTIONS (option [, option] ...)`; seven documented keys —
  HOST/DATABASE/USER/PASSWORD/SOCKET/OWNER = character literal,
  PORT = numeric literal. **No `IF NOT EXISTS` form exists.**
- `SUPER` privilege required; creates a row in `mysql.servers`;
  global, unqualified name.
- `server_name`: max 64 characters, longer silently truncated,
  case-insensitive, may be a quoted string.
- `wrapper_name`: an identifier; the manual also shows it may be
  quoted with single quotation marks.
- Character literals: UTF-8, max 64, silently truncated, default ''.
  Numeric literals: documented bound 0–9999, default 0.
- `OWNER` is currently not applied (documented no-op).
- USER/HOST/DATABASE described as mandatory **for establishing a
  connection** — the statement itself stores blank/zero defaults.
- Implicit commit; not written to the binary log.

**Activation predicate — the single gate into audited semantics.** A
`CREATE SERVER` statement enters the audited slice iff ALL of the
following hold, each with a named exit:

| # | gate | evidence source | failure exit |
|---|---|---|---|
| G1 | `dialect == MySQL` | request dialect | non-MySQL → vendor boundary (TiDB unchanged) |
| G2 | resolved target version ∈ registered MySQL 8.0.x proof domain | the same `resolvedVersion` the privilege witness already checks | version unknown / conflicting / out-of-domain → `unaudited` boundary, metadata adds `gate: "version"` |
| G3 | `decl.NameIdent` (identifier-form name, incl. backquoted) | parser declaration | string-literal spelling → `unaudited` boundary |
| G4 | `decl.NameSpellingRuneCount <= 64` | parser declaration | longer spelling → `unaudited` boundary (silent truncation makes the effective catalog identity unprovable; decoded length is unproven without full string decode) |
| G5 | wrapper token is identifier-form AND its spelling ASCII-fold-equals `mysql` | parser declaration + NEW internal fact `WrapperIdent` (registered in G-facts below) | string-literal wrapper spelling or any non-`mysql` spelling → `unaudited` boundary — a coverage decision, never a parse rejection |
| G6 | statement reached the recognizer whole | existing parser boundary | unknown OPTIONS key, empty option list, malformed member, trailing tokens → base-parser diagnostic (exit 2), unchanged |

G2 details and **qualification timing** — the version gate sits in
the second of two phases, because the parser-level boundary has no
version input:

- Phase 1, parse time (`applyCoverageBoundary`, dialect + parsed node
  only): shape gates G1, G3, G4, G5, G6 evaluate; failures mark
  `unaudited` immediately.
- Phase 2, enrich entry (before the loader gate, before per-statement
  binding): `resolvedVersion` is already fixed on `batchState` — set
  at `enrichOrderedStatements` entry from the observed banner when a
  provider exists, else from the caller's `TargetVersion` pin.
  Three version conflicts are typed early errors that never reach
  this gate: `ErrInvalidTargetVersion` (malformed pin),
  `ErrDialectProductMismatch` (observed product ≠ dialect),
  `ErrTargetVersionMismatch` (pin ≠ observed banner). An unparseable
  banner stays missing (`resolvedVersion` nil) — it reaches G2 and
  fails it. Phase 2 marks every shape-passing but version-failing
  CREATE `unaudited` with `gate:"version"`, **before** the family
  loader gate evaluates — so a version-failed statement can never
  trigger the loader, and the gate's `Unsupported == nil` test keeps
  its existing meaning: final qualification complete. There is no
  "skipped as unaudited, qualified later" path: qualification is
  complete before the loader decision consumes it.
- G2 therefore fails deterministically on: `resolvedVersion` nil or
  outside the registered 8.0.x domain — *outside the supported
  statement surface*, not unproven-in-domain. Conflicting inputs
  exited earlier as typed errors, so no input can be both "in-domain"
  and "version-conflicted".

Entering the domain is unconditional: **once the predicate holds, the
statement is audited.** Privilege mode and provider presence decide
only which evidence is available:

- **no mode** → the conditional privilege observation is not
  consumed; `server.create_privilege` stays owed (premise unproven).
  Catalog/existence reads still proceed under the provider gate.
- **no provider** → no resolver and no loader: existence stays unread
  (`UnreadReason`), premise unproven.
- **provider present, no mode** → existence read proceeds under its
  own gate; the premise stays unproven because the mode was never
  selected.
- **provider + mode** → both evidence paths available.

Public-output consequence registered honestly: an in-domain CREATE
on the default path moves from today's `unaudited` boundary to an
audited `unverified` finding. That IS a public output change of the
future candidate — the phrases "legacy unverified shape" and
"default behavior fully unchanged" are removed; the migration is
registered here and mirrored in the ADR, the index README, and the
future manifest note. `ALTER SERVER` stays on the boundary — outside
this slice entirely.

Four-way surface table:

| form | MySQL 8.0 grammar | parser today | product today | first slice |
|---|---|---|---|---|
| identifier name, ≤64 runes | yes | recognized | unaudited | **audited** |
| string-literal name | yes | recognized | unaudited | stays `unaudited` (G3) |
| name spelling >64 runes | silently truncated | recognized | unaudited | stays `unaudited` (G4) |
| identifier-form wrapper fold `mysql` | `mysql` is the documented FEDERATED wrapper | recognized | unaudited | **audited** (G5) |
| string-literal wrapper spelling | manual shows single-quoted wrapper allowed | recognized | unaudited | stays `unaudited` (G5) — identifier quoting ≠ string-literal quoting; no expansion into string decoding |
| other wrapper spellings | any identifier accepted | recognized | unaudited | stays `unaudited` (G5) — no provable wrapper registry exists |
| OPTIONS ≥1 documented member | yes | enforced | unaudited | **audited** |
| unknown OPTIONS key | not in grammar | unclaimed | parse error (exit 2) | unchanged — parser boundary |
| duplicate OPTIONS key | grammar admits the list | recorded | unaudited | audited; `server.create_options` owed |
| PORT out of documented range | bound 0–9999 | digits parse (range unchecked today) | unaudited | audited; `server.create_options` owed via NEW `RangeState` fact |
| numeric literal unprovable (e.g. digit run overflowing uint64) | — | today `ParseUint` fails → whole OPTIONS unclaimed | parse error (exit 2) | unchanged at parser boundary — registered, NOT silently classified clean |
| missing USER/HOST/DATABASE | stores defaults; advisory for connecting | parsed | unaudited | audited; audit-model completeness **notice** — not a refusal, not an owed fact, no claim of a native MySQL warning |

## B. Privilege premise — supply, projection, and the owed fact

Official requirement: `SUPER`. The first slice **reuses the
already-approved request-level observation** unchanged:

- `observed_account_new_session` mode; loader
  `LoadMySQLServerDropPrivilege` (`CURRENT_USER()` then
  `SHOW GRANTS FOR CURRENT_USER()` on one `*sql.Conn`);
  witness = bound account + version in domain + verbatim
  `SUPER`/`ALL PRIVILEGES` on `*.*`; positive-only; never a denial;
  never pooled/future-session effective privileges.
- Loader gate widens once — proposed predicate
  `statementsNeedMySQLServerPrivilege`: fires when ≥1 audited
  identifier-form DROP **or CREATE** SERVER is present. Still at most
  ONE raw observation per request, shared by the family. A
  CREATE-only request newly triggers the hook: registered as **new
  call behavior** (a request that never called the loader now does,
  with its bounded error exit reachable); "no new Go methods/params"
  is NOT claimed to mean "behavior unchanged".
- Per-statement projection is operation-specific: proposed
  `ServerCreatePrivilege` {Unproven, Consumable, Suppressed} derived
  from the same raw witness and the same ordered scope. A satisfied
  `server.drop_privilege` is never borrowed; a prior drop's
  `conditional_audit` marker never substitutes for the create's own
  premise. The owed fact key is `server.create_privilege`.
- Scope terminators: identical list (authored GRANT/REVOKE;
  `set_role`/`set_default_role`/`grant_role`/`revoke_role`/
  `grant_proxy`). They end the create premise's range for later
  statements — suppression never claims a grant was revoked.

## C. Existence answers and consumption order

CREATE consumes the same accepted six-step order (bind position,
`bindMySQLServer`):

1. whole-batch contamination → bound unvouched, early return (no
   resolver access, no unread reason);
2. channel taint or relation-scan barrier → bound
   `UnreadReason = server.batch_effects`, early return — steps 1 and
   2 are the two distinct existing exits, reused verbatim.
   The create adjudication distinguishes them on two separate
   internal inputs, not on the bound `ServerState` alone: the
   whole-batch flag reaches the arm table directly as
   `batchState.contaminated` (the adjudication runs on `batchState`,
   the same access the ordered apply already has — it is NOT
   projected through `UnreadReason`, which contamination never sets);
   the step-2 exits reach it through the bound
   `UnreadReason = server.batch_effects` field;
3. ordered channel answers on a provably-same identity — today only
   `OrderedAbsent`; the slice adds `OrderedPresent` as the third
   consumed answer (proposed merged ordered-state scan, entry point
   named in G);
4. provider resolver read (`mysql.servers`) only when the channel
   cannot answer and the name is a provable identifier: row → proven
   present; **successful zero rows → unknown, never absent**;
5. provider query/scan/iteration error → propagates bounded — never a
   state value, never an owed fact, never absent/present;
6. loader/resolver failures take the bounded error exit
   (`serverPrivilegeReadError` /
   object-read boundary patterns) — they never degrade into state.

Direct consequence: a *first-statement* create can never be a proven
create — the slot-free premise needs proven absence, and zero catalog
rows are unknown, never absent. `OrderedAbsent` is the only
proven-absence source and the only existing publisher of it is
conditional — stated plainly so no future arm quietly widens
"absent".

## D. Adjudication — priority-ordered, mutually exclusive, exact owed facts

NEW proposed vocabulary: `ServerCreateArm`, `ServerCreateAdjudication`
(`Privilege`, `Arm`, `Owed`, `PublishPresent`, `ConditionalSource`),
`ServerCreatePrivilege`, facts `server.create_privilege` /
`server.create_options` alongside existing `server.existence` /
`server.batch_effects`; outcome tokens `conditional_create` /
`refused_present` / `blocked_batch_effects` / `unverified`.

**Branch order — first match wins, exactly one arm per input:**

- **A1 Barrier** — contamination, channel taint, or relation-scan
  barrier (step 1–2 exits). Owed = exactly `{server.batch_effects}`.
  Short-circuit semantics: the channel's undischarged unknown effect
  shadows every premise — the row asserts nothing about privilege,
  options, or existence having been proven; it is CREATE's own
  short-circuit rule, not an unverified claim that it equals DROP's
  mapping in all combinations.
- **A2 TerminalRefusal** — proven presence on the provably-same
  identity: ordered `OrderedPresent` OR a proven catalog row. Owed =
  `∅`; blocker; `refused_present`. The impossibility is proven by
  occupancy alone — privilege and options never reach the question;
  no native error number or MySQL error ordering is claimed.
- **A3 ProvenCreate** — publish arm: `OrderedAbsent` on the
  provably-same identity AND consumable premise AND clean options.
  Owed = `∅`; notice; `conditional_create`; publishes
  `OrderedPresent` (+ conditional source).
- **A4 Unproven** — everything else in-domain. Owed = the exact set
  formula, no open-ended "others":
  - `server.existence` iff no consumable proven-absence source
    answered the slot (i.e. the ordered channel did not return
    `OrderedAbsent`-same — zero-row unknown, unread, or no answer);
  - `server.create_privilege` iff the statement's own premise ≠
    consumable (unproven or suppressed);
  - `server.create_options` iff options state ≠ clean (duplicate
    keys, `RangeState` out-of-range or unproven).

**Worked rows** (the formula applied; — = not reached):

| row | channel | ordered answer | catalog | premise | options | arm | owed (exact) | outcome | level | post-state | resolver |
|---|---|---|---|---|---|---|---|---|---|---|---|
| 1 | contaminated | — | — | — | — | Barrier | {batch_effects} | blocked_batch_effects | notice | no publish; entry unchanged | 0 |
| 2 | taint/rel-barrier | — | no read | any | any | Barrier | {batch_effects} | blocked_batch_effects | notice | entry barrier | 0 |
| 3 | clean | OrderedPresent-same | — | any | any | TerminalRefusal | ∅ | refused_present | blocker | OrderedPresent stands (consumed source) | 0 |
| 4 | clean | OrderedAbsent-same | — | consumable | clean | ProvenCreate | ∅ | conditional_create | notice | publish OrderedPresent+cond | 0 (ordered answered) |
| 5 | clean | OrderedAbsent-same | — | unproven | clean | Unproven | {create_privilege} | unverified | notice | absent→barrier | 0 |
| 6 | clean | OrderedAbsent-same | — | suppressed | clean | Unproven | {create_privilege} | unverified | notice | absent→barrier | 0 |
| 7 | clean | OrderedAbsent-same | — | consumable | dirty | Unproven | {create_options} | unverified | notice | absent→barrier | 0 |
| 8 | clean | OrderedAbsent-same | — | unproven/suppressed | dirty | Unproven | {create_privilege, create_options} | unverified | notice | absent→barrier | 0 |
| 9 | clean | none | proven present | any | any | TerminalRefusal | ∅ | refused_present | blocker | no publish — channel unchanged (NOT an OrderedPresent publisher) | 1 |
| 10 | clean | none | zero-row unknown | consumable | clean | Unproven | {existence} | unverified | notice | entry barrier | 1 |
| 11 | clean | none | zero-row unknown | non-consumable | clean | Unproven | {existence, create_privilege} | unverified | notice | entry barrier | 1 |
| 12 | clean | none | zero-row unknown | consumable | dirty | Unproven | {existence, create_options} | unverified | notice | entry barrier | 1 |
| 13 | clean | none | zero-row unknown | non-consumable | dirty | Unproven | {existence, create_privilege, create_options} | unverified | notice | entry barrier | 1 |
| 14 | clean | none | unread (no resolver / no provider) | consumable | clean | Unproven | {existence} | unverified | notice | entry barrier | 0 |
| 15 | clean | none | unread | non-consumable | any | Unproven | {existence, create_privilege} (+ create_options iff dirty) | unverified | notice | entry barrier | 0 |
| 16 | clean | none | resolver/loader error | — | — | error exit | — | — | — | bounded error propagates; no arm | — |

**Unique post-state per arm** (removes the old "present stands" vs
"non-publish clears" conflict):

- ProvenCreate → publishes `OrderedPresent=true` +
  `OrderedPresentConditional` on the provably-same entry; clears that
  entry's `absent` (mutually exclusive) and `barrier`.
- TerminalRefusal (ordered source) → the consumed `OrderedPresent`
  **stands untouched** — the refusal re-confirmed occupancy; no
  publish, no barrier.
- TerminalRefusal (catalog source) → **no publish, no barrier** —
  the channel records nothing new; the only OrderedPresent publisher
  is ProvenCreate, so a catalog-present refusal never mints an
  ordered present. Later statements re-derive via their own bind.
- Unproven → `entry.barrier=true`, `entry.absent=false`,
  `entry.present=false` — undischarged unknown effect; on a
  provably-same `OrderedAbsent`/`OrderedPresent` identity the ordered
  state is invalidated (the create may have succeeded).
- Barrier, **contaminated-batch source** → no entry change at all:
  `applyServerStatement` already returns before touching entries on
  `s.contaminated`, and the whole-batch guard keeps blocking every
  later consumption — clearing entries here would falsely claim a
  settled effect where none was adjudicated.
- Barrier, non-contaminated source (taint or relation scan) →
  `entry.barrier=true`, `entry.absent=false`, `entry.present=false`
  — the channel's undischarged unknown effect on that identity.

**Invalidation scopes, spelled out:** (a) per-identity — an Unproven
or unaudited-boundary create on a provably-same identity clears that
entry's ordered state to barrier; (b) relation-unproven — any entry
or recorded write whose name relates unprovenly to the authored name
forces the *reading* statement's barrier and, at apply, barriers the
related entries (existing `resolveServerBarrier` + write loop,
unchanged); (c) channel taint — an unaudited literal-name create
taints the whole channel and clears every entry's ordered state
(existing literal-write path, extended to `present`). A positive
privilege witness never clears an existing barrier — barriers are
monotonic within the batch.

**`OrderedPresent` / `OrderedPresentConditional`** (NEW proposed
first-slice fields — not implemented; `json:"-"`, never serialized):

- Production: only ProvenCreate, provably-same identity, clean
  channel. The only existing OrderedAbsent publisher is conditional,
  so every OrderedPresent published today carries
  `OrderedPresentConditional=true`; the unconditional variant is a
  field-honest placeholder, not a claim a producer exists.
- Consumption: inside `bindMySQLServer` at the same position as
  `OrderedAbsent` — after the step-1/2 exits, before provider/cache —
  by later creates (refusal input) and later drops (proven-present
  existence answer). Consumption does not fake catalog `Exists`,
  `Read`, or `Status`; consumers read the ordered field directly,
  same pattern as `OrderedAbsent`. The DROP-side consumption is a new
  behavior of the future CREATE candidate — never a backfill into the
  accepted DROP PASS.
- Invalidation/replacement: mutually exclusive with `OrderedAbsent`;
  a proven drop publish replaces present→absent; a proven create
  publish replaces absent→present; every Unproven/Barrier arm and
  every unaudited/unknown effect clears it to the barrier shape.
- Production→consumption path: create adjudication sets
  `PublishPresent` + `ConditionalSource` → apply writes proposed
  entry fields `present`/`presentConditional` (siblings of
  `absent`/`publishedArm`) → bind returns
  `ServerState{OrderedPresent, OrderedPresentConditional}` → later
  adjudications read it → finding/outcome/post-state derive from
  that one adjudication.

**`conditional_audit` projection — deterministic:** the marker is
emitted iff (a) the arm's own consumable premise fed the conclusion,
OR (b) the arm consumed an ordered state carrying conditional source.
Both → one marker. Neither → no marker. A barrier-shadowed ordered
source is never consumed, so it never projects a marker. The marker
never discharges the statement's own owed premise and never claims
the database executed anything.

**Coverage and verdict projection** (rule enabled, documented default
levels — the public projection the worked rows imply):

| arm | statement coverage | verdict contribution |
|---|---|---|
| ProvenCreate | complete | adds no blocker and no evidence-gap floor |
| TerminalRefusal | complete | one blocker → batch reject under default level |
| Unproven | unverified | evidence-gap floor → review |
| Barrier | unverified | evidence-gap floor → review |

Statement coverage is complete on zero-gap arms and `unverified`
under any owed fact. Aggregate coverage and the whole-batch verdict
use the **existing aggregation rules unchanged** — aggregate = min
over statements; verdict = reject on any blocker, review while gaps
stand, pass only when every statement is clean. A single CREATE's
projection never substitutes for the batch result: a contamination
fixture, for instance, may carry other parse failures or uncovered
statements whose own coverage/verdict contributions aggregate in —
the batch outcome derives from the actual fixture, not from this
row alone.

## E. Identity relations and cross-statement composition

Relation contract unchanged: byte-equal or ASCII-fold-equal within
the registered ASCII-CI subset ⇒ `serverRelSame`; everything else
`serverRelUnproven`; `serverRelDistinct` keeps its empty provable
domain. The manual's "case-insensitive" is a catalog property, not
license to widen the relation domain.

Composition chains (expected behavior under this contract):

1. **conditional DROP → CREATE → DROP** (same name, consumable
   premise throughout): drop1 publishes conditional `OrderedAbsent`;
   create consumes it → ProvenCreate → publishes
   `OrderedPresent`+conditional (0 new resolver reads); drop2 consumes
   `OrderedPresent` as its proven-present existence answer + own
   consumable premise → proven drop arm → publishes conditional
   `OrderedAbsent` again. All three carry `conditional_audit`; drop2's
   marker has BOTH sources (own premise + inherited), so a separate
   chain (chain 6) proves inheritance alone.
2. **CREATE → CREATE** (same name): create1 owes existence → Unproven
   → entry barrier; create2's relation scan hits the same-identity
   barrier → Barrier arm, owed exactly `{server.batch_effects}`, 0
   resolver reads. (Had create1 published, create2 → refused_present.)
3. **valid OrderedAbsent → unaudited literal-name CREATE → DROP**:
   the literal write taints the whole channel and clears every
   entry's ordered state; the DROP binds batch-effects unread →
   Barrier arm, 0 reads.
4. **valid OrderedAbsent → audited-unproven CREATE (same identity)
   → DROP**: the unproven create clears the entry's absent to
   barrier; the DROP reads `blocked_batch_effects`. Unknown CREATE
   therefore invalidates the prior `OrderedAbsent` — the work-order
   rule.
5. **channel taint → audited CREATE**: tainted channel → Barrier
   arm; the create never publishes.
6. **conditional OrderedPresent consumed by a premise-suppressed
   statement**: `OrderedPresent`-same + own premise suppressed →
   refusal arm is still TerminalRefusal (premise never reaches the
   question) and the marker comes solely from the inherited
   conditional source — the targeted case proving inheritance works
   without the statement's own conditional premise. Same shape for a
   DROP consuming conditional `OrderedPresent` while its own premise
   is unproven: `OrderedPresent` supplies the proven-present existence
   answer, so the drop lands Unproven owing exactly
   `{server.drop_privilege}`, carries the inherited marker, and
   publishes nothing. Registered as the targeted plan row.
7. **non-same-relation interleave**: an audited create whose name
   relates unprovenly to recorded entries/writes barriers before any
   ordered or catalog consumption.
8. **scope terminator before CREATE**: `REVOKE …`/`SET DEFAULT ROLE`
   between witness and create → premise suppressed →
   `server.create_privilege` owed even with a conditional
   `OrderedAbsent` standing (row 6); the terminator never claims a
   grant was revoked.

## F. Public output, privacy, and the exit map

Redaction unchanged: `PublicSQL()` already emits
`CREATE SERVER <redacted>`; the parser stores option keys + value
kinds only — never literal values; name, wrapper, option text,
account, and grant lines stay internal. New findings carry the fixed
outcome token, exact owed-fact keys, `conditional_audit`, level,
message, explanation — refusal states only the proven impossibility;
`conditional_create` states the modeled creation under the
conditional witness.

Registered public-output changes of the future candidate (NOT claimed
"unchanged"): in-domain create moves `unaudited`→audited; new outcome
tokens and fact keys appear in finding metadata; coverage/verdict
derive per the table; CREATE-only requests newly trigger the loader
hook; `T26.json` `create_server`/`mixed-batch` cells migrate under
the future candidate's policy — expectations are derived from this
adjudication table only under rule-enablement premises the candidate
declares (the table assumes the server rule is enabled and configured
levels are the documented defaults; a custom policy remaps levels
exactly as it does today — adjudication, not policy, owns the arm).

Exit → mechanism → verification map:

| exit | reused mechanism / new seam | verification |
|---|---|---|
| raw_sql / normalized_sql | `PublicSQL()` redaction — already emits `CREATE SERVER <redacted>` | existing golden `statement_sql` assertions + future migrated cells |
| finding message/explanation/metadata | rule projection off the shared adjudication (create-arm sibling in `mysql_server_rules.go`) | future targeted unit + vertical tests, PLAN |
| unsupported / diagnostic | existing coverage-boundary pin (`unaudited` + gate metadata) | existing boundary tests extended, PLAN |
| bounded error content / identity | `serverPrivilegeReadError` + object-read boundary patterns | existing `errors.As`/cause assertions extended to the create arm, PLAN |
| CLI actual output | `ddl_golden.py` runner over `T26.json` under declared policy | future candidate's manifest cells, PLAN |
| privacy scope | finding-field scan (name/wrapper/options/account/grant absent from emitted fields), per actual exit — honestly scoped, NOT a whole-serialized-result proof | future targeted assertion, PLAN |

## G. Internal facts and implementation mapping

Facts consumed from parser input — each with provenance:

- `Name`, `NameIdent`, `NameSpellingRuneCount`: existing fields —
  G3/G4 gates.
- `Wrapper`: existing internal field (spelling retained internally,
  never serialized) — G5 fold check.
- NEW proposed `WrapperIdent` (bool): records the wrapper token's
  quoting category — identifier-form vs string-literal — at parse
  time while the token category is still visible; G5 gate depends on
  it. Assignment point: `mysqlServerCreate` where the wrapper token
  is scanned. Not implemented this round.
- `Options []ServerOption`, `DuplicateKeys`: existing — option
  presence, kinds, duplicates.
- NEW proposed per-option `RangeState` (enum
  `in-range|out-of-range|unproven`) for numeric options: computed at
  parse time while the literal is still visible — in-range
  0≤v≤9999; out-of-range when digits parse but exceed the bound;
  unproven when the digit run cannot be proven within range (e.g.
  length/overflow beyond uint64 — never silently clean). Stored on
  the option entry alongside `ValueKind`; no raw value is retained
  or emitted. Assignment point: the OPTIONS member loop in
  `mysqlServerCreate`, beside the existing `ParseUint` check.
- "clean options" = every present key documented (grammar-guaranteed)
  AND every numeric `RangeState == in-range` AND `DuplicateKeys`
  empty — and nothing more. Remote connectivity, FEDERATED
  availability, and character-option semantics are NOT asserted by
  "clean"; the publish arm never claims them.
- Missing USER/HOST/DATABASE → audit-model completeness **notice**
  (policy of this audit slice), not an owed fact and not a claimed
  native warning. Official fact: the statement stores defaults;
  product today: nothing; proposed: notice-level annotation.

Files a future execution order touches (registered seams):

- `internal/infrastructure/parser/tidb/mysql_server.go` — proposed
  `WrapperIdent` + `RangeState` derivation points (parse-time only).
- `internal/infrastructure/parser/tidb/coverage_boundary.go` —
  `unaudited` pin narrows to the G1–G6 domain.
- `internal/domain/spec/server.go` — `OrderedPresent`,
  `OrderedPresentConditional`, `RangeState`/`WrapperIdent` fields.
- new `internal/domain/spec/server_create_premise.go` — arm/
  adjudication/privilege types, `ServerFactCreatePrivilege`,
  `ServerFactCreateOptions`, `AdjudicateServerCreate`.
- `internal/application/audit/server_drop_premise.go` — loader gate
  widened to the family; create premise projection.
- `internal/application/audit/batch_state_server_srs.go` — merged
  ordered-state scan consuming `OrderedPresent` at the existing
  OrderedAbsent position.
- `internal/application/audit/batch_state_server.go` —
  `batchServerEntry.present`/`presentConditional`; create apply arm;
  invalidation scopes above.
- `internal/domain/rule/ddl/mysql_server_rules.go` — create-arm rule
  surface; outcome/fact tokens.
- `testdata/ddl-golden/T26.json` — `create_server`/`mixed-batch`
  cells migrate under the future candidate's declared policy.
- No new public API proposed (mode, loader hook, `Request` already
  admit the family); new call behavior registered in R2-5 terms.

## Verification plan — PLAN / UNEXECUTED (no execution this round)

Grouped per the R2 order; query-count layers asserted separately
(loader ≤1/request, resolver reads/statement, catalog SQL counts) —
only levels with direct assertions register as covered:

A. **Gate/version rows**: in-domain + provider + no-mode → row 11
   shape — fixed premises: clean channel, no ordered answer,
   successful zero-row catalog, OPTIONS clean → owed exactly
   {existence, create_privilege}, resolver fires once; a provider
   returning a proven row instead lands refusal (row 9).
   in-domain + no-provider → row 15 shape (0 reads). Version rows:
   unknown (`resolvedVersion` nil) / out-of-domain → `unaudited` +
   `gate:"version"`; conflict inputs are typed early errors
   (`ErrInvalidTargetVersion` / `ErrDialectProductMismatch` /
   `ErrTargetVersionMismatch`) and never reach the gate; non-MySQL →
   vendor boundary.
B. **Wrapper forms**: identifier `mysql`/`'mysql'`-literal/other →
   audited / unaudited / unaudited respectively.
C. **OPTIONS**: PORT 0/3306/9999 in-range → clean; 10000 →
   out-of-range → owed `{create_options}` — fixed premises for the
   single-fact outcome: same-identity valid `OrderedAbsent` +
   consumable premise (otherwise the formula owes more);
   overflow-length digit run → parser boundary (unchanged);
   duplicate documented key → `{create_options}` under the same
   fixed premises; missing USER/HOST/DATABASE → completeness
   notice, arm unaffected.
D. **Dirty-vs-refusal precedence**: proven-present + dirty →
   refusal (owed ∅, blocker); unknown + unproven + dirty → all three
   facts owed.
E. **Premise edges**: OrderedAbsent + suppressed →
   `{create_privilege}` only — OPTIONS clean fixed; unread +
   consumable → `{existence}` only — OPTIONS clean fixed.
F. **Refusal post-state**: OrderedPresent-sourced refusal →
   `OrderedPresent` stands; later CREATE/DROP consume it
   (refusal/proven-present) with inherited marker. Catalog-sourced
   refusal → no publish and no cached present: a later statement
   re-derives via its own resolver read — fixed premise: no ordered
   state exists for that identity, so the resolver count shows the
   fresh read.
G. **Inherited-marker edges**: conditional `OrderedPresent` consumed
   by a suppressed/unproven-premise CREATE (refusal, marker solely
   inherited) and by a DROP (Unproven `{server.drop_privilege}`,
   inherited marker, no publish); marker never discharges own premise.
H. **Channel mechanics**: DROP→CREATE→DROP chain; unknown audited
   create invalidating prior `OrderedAbsent`; relation-unproven
   barrier; literal-create channel taint; explicit contaminated-batch
   path → Barrier arm via `batchState.contaminated` (bound unvouched,
   no `UnreadReason`), entry untouched, aggregate derives from the
   actual fixture's other statements.
I. **Loader behavior**: CREATE-only request → loader fires once —
   fixed premises: mode enabled, provider hook available, version +
   shape qualification passed; DROP+CREATE batch → still one
   observation; loader error → bounded error exit.
J. **Default-path registration**: in-domain create with no
   mode/provider asserts `unverified` + exact owed set (NOT the old
   `unaudited`) — the registered public change.

## Actual checks run this round (R3)

- `git status --short` on tracked files: clean at each commit.
- `git diff --check`: executed clean on R1 (`3ec5f2c3`) and R2
  (`e59f2452`) candidates; re-run clean on the R3 candidate at
  commit.
- Source-consistency: every code reference above re-verified against
  the baseline tree (including `bindMySQLServer`'s two distinct early
  exits — contamination → bound unvouched, taint/barrier →
  `MetadataUnreadBatchEffects` — which the CREATE mapping reuses
  verbatim; the only proposed new bind behavior is the merged
  `OrderedPresent` consumption scan).
- Official-source consistency: manual re-checked this round —
  SUPER, quoted wrapper allowed, PORT 0–9999, no `IF NOT EXISTS`,
  USER/HOST/DATABASE phrased as connection requirements (the
  completeness notice is therefore registered as audit-model policy,
  not a claimed native warning).

## Remaining questions

None blocking. Registered non-goals: ALTER SERVER, SRS, ALTER
INSTANCE, literal-name audit, TiDB semantics, non-`mysql` wrappers,
full string-literal decode, `serverRelDistinct` extension — deferred
exactly as the DROP contract left them.
