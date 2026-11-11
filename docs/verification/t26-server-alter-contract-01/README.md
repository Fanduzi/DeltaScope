# T26-SERVER-ALTER-CONTRACT-01 — ALTER SERVER first-slice candidate contract

Status: **ISSUED — candidate contract, docs-only (R5 revision)**.
Every adjudication below is a *proposed* contract awaiting Reviewer
acceptance; nothing here is implemented, verified, or approved.
This document implements no code, runs no test, and changes no
accepted DROP/CREATE behavior. Frozen from earlier rounds: A1
uniform no-op; the canonical flag-assignment table; P1–P7
adjudicated (§G); keep-boundary-bind; seeded/suppressed witnesses;
fixed public projections; the CLI probe recipe. R5 closes the last
matrix rulings: per-statement public-output scope is declared, the
five observation layers are named per row, and Group 9 rows carry
their exact layer and rc applicability.
Parent: T26 / issue #105 under issue #79. Baseline: `28152dfd`
(tree `8b17cf3c`, parent `2afe8ecb`) on
`milestone/mysql-tidb-ddl-completion` — the accepted
T26-SERVER-CREATE-EXECUTION-01 R3 candidate `2afe8ecb` plus its docs
closeout.

The contract defines ONE recommended first-implementation domain for
audited MySQL `ALTER SERVER`: the activation predicate, the premises
each arm consumes, one mutually-exclusive adjudication, the unique
ordered post-state per arm, and the exact public projection. It
continues the accepted DROP (`9dad37b8`) and CREATE (`2afe8ecb`)
slices and reuses their approved boundaries unchanged: conservative
identity relations, zero-row catalog = unknown (never absent),
positive-only direct-global-grant privilege witness,
`observed_account_new_session` scope, no future-execution claims.

## A. Syntax, official semantics, and the activation predicate

Official basis: MySQL 8.0 Reference Manual §15.1.8 ALTER SERVER
(dev.mysql.com/doc/refman/8.0/en/alter-server.html, fetched this
round). The whole documented grammar:

```
ALTER SERVER server_name OPTIONS (option [, option] ...)
```

Documented semantics the contract consumes:

- "Alters the server information for *server_name*, **adjusting any
  of the options permitted in the CREATE SERVER statement**. The
  corresponding fields in the `mysql.servers` table are updated
  accordingly." The example updates `USER` alone — **partial update
  (merge), not wholesale replacement**.
- Requires `SUPER` — the same privilege the family's approved
  observation witnesses.
- `ALTER SERVER` causes an implicit commit and is not written to the
  binary log — no audit-model consequence (this engine models no
  transactions and no binlog surface); registered as documented-but-
  unmodeled facts.
- `server_name` spelling rules are CREATE SERVER's: ≤64 characters
  (longer silently truncated), case-insensitive, may be quoted.
- `ALTER` declares **no `FOREIGN DATA WRAPPER` clause** and no guard
  form — no `IF EXISTS`/`IF NOT EXISTS` exists for it.
- The option vocabulary is CREATE's: HOST/DATABASE/USER/PASSWORD/
  SOCKET/OWNER char literals; PORT numeric in 0–9999.

**Current product path** (all source-verified on baseline `28152dfd`,
not history):

| step | code seam | current ALTER behavior |
|---|---|---|
| parse | `internal/infrastructure/parser/tidb/mysql_server.go:180 mysqlServerAlter` | claims the whole form; produces `DDLOperationAlterServer` + `ServerDeclaration{Kind:"alter"}`; OPTIONS members parsed by the same `mysqlServerOptions` (7 keys, value kinds, `DuplicateKeys`, `RangeState`) |
| phase-1 shape gate | `coverage_boundary.go:475 mysqlServerAuditedForm` | `default: return false` — every ALTER takes `Unsupported{Feature:"alter_server", boundary:"unaudited", object:"server"}` on MySQL; `boundary:"vendor"` on TiDB |
| G2 version gate | `metadata.go:247-263` | gated on `Kind == ServerKindCreate` only — ALTER never reaches this gate today (it is already unsupported) |
| privilege trigger | `server_drop_premise.go:24 statementsNeedMySQLServerPrivilege` | counts `ServerKindDrop`/`ServerKindCreate` only — ALTER never triggers the loader |
| bind | `metadata.go:619` → `batch_state_server_srs.go:56 bindMySQLServer` | **runs for every SERVER kind including boundary ALTER** — the ordered-channel exits apply (contamination → unvouched; taint/relation barrier → `UnreadReason=batch_effects`); on a clean channel an identifier-form ALTER *does* issue one `mysql.servers` catalog read today; a string-literal name issues a resolver request that the provider declines without SQL (`identity_unproven`) |
| adjudication | `metadata.go:634/647` | only `ServerKindDrop`/`ServerKindCreate` attach — ALTER carries none |
| ordered apply | `batch_state_server.go:184 applyServerStatement` | `Unsupported != nil` → identifier name records a `serverWrite` + barriers every non-distinct-relation entry; literal name taints the whole channel. `Kind == "alter"` has no audited block — no publish, ever |

Registered current-behavior facts the candidate must respect or
explicitly migrate: the boundary ALTER still binds (and can fire a
catalog read) because `bindMySQLServer` has no `Unsupported` gate;
the write record on boundary ident-form ALTER is the accepted
E14 `[cur]` behavior — both are preserved unless an arm below
changes them.

**Activation predicate** — the single gate into audited semantics.
An `ALTER SERVER` statement enters the audited slice iff ALL hold,
each with its named exit:

| # | gate | evidence source | failure exit |
|---|---|---|---|
| G1 | `dialect == MySQL` | request dialect | non-MySQL → vendor boundary (TiDB unchanged) |
| G2 | resolved target version ∈ registered MySQL 8.0.x proof domain | same `resolvedVersion` the privilege witness checks (`state.resolvedVersion`, fixed at enrich entry before the loader gate) | version unknown or out-of-domain → `unaudited` + `gate:"version"`; conflicting inputs (malformed pin, product mismatch, pin/observed mismatch) exit earlier as typed errors and never reach this gate |
| G3 | `decl.NameIdent` (identifier-form name) | parser declaration | string-literal spelling → `unaudited` boundary; channel-taint write at apply (unchanged accepted behavior) |
| G4 | `decl.NameSpellingRuneCount <= 64` | parser declaration | longer spelling → `unaudited` boundary (silent truncation makes the effective catalog identity unprovable) |
| G5 | `OPTIONS (...)` whole-list parse with documented members only | parser claim | unknown key, empty list, malformed member, trailing tokens → unclaimed → base-parser diagnostic (unchanged) |
| G6 | phase-2 ordering | enrich entry | G2 executes before the family loader gate and before per-statement binding; a version-failed ALTER can never trigger the loader (same contract as CREATE) |

Two-phase timing identical to CREATE: shape gates at
`applyCoverageBoundary` (parse time, no version input); G2 at enrich
entry on `batchState.resolvedVersion`. Entering the domain is
unconditional — mode/provider decide evidence availability only.

**Wrapper question, decided:** ALTER declares no wrapper. The
candidate keeps **no own wrapper premise**: (a) the partial-update
effect is wrapper-agnostic — the presence-preservation proof in §D
holds for any stored row; (b) the bound existence read selects only
`server_name` — no wrapper evidence exists to consume and none is
proposed (widening the read's selected columns would enlarge the
payload surface for no consumer); (c) public text must stay
wrapper-agnostic — findings never assert the stored row's wrapper.
CREATE's authored-wrapper condition is not inherited as "already
satisfied": it simply has no counterpart on this verb.

**Boundaries kept** (each with its reason class):

| form | exit | class |
|---|---|---|
| TiDB `ALTER SERVER` | vendor boundary | product not supported on this dialect (TiDB grammar lacks the family) |
| string-literal name | `unaudited` + channel taint write | audit evidence: unprovable catalog identity |
| spelling >64 runes | `unaudited` | audit evidence: truncation makes effective identity unprovable |
| version unknown/out-of-domain | `unaudited` + `gate:"version"` | audit evidence: no in-domain proof |
| malformed OPTIONS / unknown key / empty list | base-parser diagnostic | native grammar: not the documented shape |
| duplicate keys, PORT out-of-range | **audited**, owes `server.alter_options` | grammar admits the list; semantics stay adjudicatable |

## B. Own privilege premise — `server.alter_privilege`

Official requirement: `SUPER` (§15.1.8). The candidate **reuses the
approved request-level observation unchanged**: the loader
`MySQLServerDropPrivilegeLoader` (`CURRENT_USER()` then
`SHOW GRANTS FOR CURRENT_USER()` on one `*sql.Conn`) yields a
verb-agnostic raw fact — the bound account's verbatim `SUPER`/`ALL
PRIVILEGES` on a direct global `*.*` grant under
`observed_account_new_session`, positive-only, in-domain only, never
a denial. The loader's historical name carries "Drop"; the observed
evidence is not verb-specific — registered so no reader mistakes the
name for the fact.

Per-statement projection is operation-specific, mirroring
`ServerCreatePrivilege`: proposed `ServerAlterPrivilege`
{Unproven, Consumable, Suppressed} over the same raw witness and the
same positional scope. Owed fact key: `server.alter_privilege`. A
satisfied `server.drop_privilege` or `server.create_privilege` on a
sibling statement is never borrowed; a prior statement's
`conditional_audit` marker never substitutes for this statement's
own premise; the marker and the premise remain distinct facts.

Registered new call behavior (execution-order note, not a claim this
doc changes it): `statementsNeedMySQLServerPrivilege` widens to
`ServerKindAlter` — an ALTER-only request newly triggers the one
observation per request. Scope terminators are the identical list
(authored GRANT/REVOKE; `set_role`/`set_default_role`/`grant_role`/
`revoke_role`/`grant_proxy`) — suppression never claims a grant was
revoked.

## C. OPTIONS — partial-update semantics and typed facts

Doc fact: "adjusting any of the options" + "the corresponding fields
… are updated accordingly" + the single-option example ⇒ **merge,
not replace**. Consequences, per work order:

- **Undeclared options**: keep their stored values — which this
  audit never reads (`loadMySQLServerObject` selects `server_name`
  only). They stay unknown; the model never infers stored values,
  defaults, or "unchanged = known".
- **Typed facts can adjudicate only authored members**: every key
  documented (grammar-guaranteed), `PORT` `RangeState` in/out-of-
  range, `DuplicateKeys` recorded. Value kinds are enforced at
  parse; a non-literal or wrongly-kinded member stays unclaimed.
- **Duplicate keys / PORT range**: sufficient evidence for the
  *owed* fact `server.alter_options` (same shape as
  `server.create_options`), **not** for a refusal claim — the
  manual does not state duplicate-key or out-of-range outcomes for
  ALTER; they are adjudicated owed, never asserted native errors.
- **Length/truncation**: char literals ≤64 silent truncation is a
  stored-value property; values never enter the model — nothing to
  adjudicate there.
- **Dirty OPTIONS post-state**: a dirty authored list does not
  change the preservation theorem — the write either lands (row
  still present) or is refused (row still present); options dirt
  never deletes. It marks the *statement's own* OPTIONS premise
  unproven, nothing else.
- **Connection advisory**: CREATE's `ConnectionMembersComplete`
  notice does **not** transfer — for ALTER the authored list is
  intentionally partial and the stored set is unread, so "omitted
  members store blank defaults" is false here. **Decided** scoped
  advisory (§G P2, A3/A4 arms only). Fixed public projection — two
  findings, using the existing metadata structure, no new schema:
  the main finding carries `object=server`,
  `operation=alter_server`, `outcome=` the arm's token, and
  `conditional_audit` projected only from
  `Metadata.ServerAlter.ConditionalAudit` (true ⇒ the existing
  `observed_account_new_session` token); the independent advisory
  finding carries `object=server`, `operation=alter_server`,
  `notice=connection_members` with the fixed text "ALTER SERVER
  specifies a partial options update; stored and resulting
  connection member sets are unverified." at the rule's configured
  level — no execution outcome, no `conditional_audit`, no owed
  fact or gap, no authored option keys/values. Emitted under
  `ddl.mysql.server.alter.notice`; A1/A2 never emit it; the
  internal arm is not itself the public discriminator between the
  two findings.
- **No expansion**: credentials, coordinates, and raw OPTIONS
  payloads stay out of both internal storage and public output —
  the parser already records keys and value kinds only.

**"Opaque revision marker" rejected** (work order C): the historical
C11 row imagined `present + opaque revision marker`. No consumer
exists: the ordered channel tracks presence only, and presence is
preserved unconditionally (§D). A revision marker would record
"options may have changed" — already the exact semantics of the
Unproven arm's owed set — and would mint a field with no reader.
Historical design is not a necessity proof; the field is not
proposed.

## D. Adjudication — priority-ordered, mutually exclusive

Proposed vocabulary (mirrors the accepted CREATE file):
`ServerAlterArm`, `ServerAlterAdjudication` {Privilege, Arm, Owed,
PublishPresent, ConditionalSource, ConditionalAudit},
`ServerAlterPrivilege`; facts `server.alter_privilege` /
`server.alter_options` alongside reused `server.existence` /
`server.batch_effects`; outcome tokens `conditional_alter` (new) /
`refused_absent`, `blocked_batch_effects`, `unverified` (reused).

**Canonical assignment table** — every adjudication field's value
per arm, the single truth rule evaluation and ordered apply both
read (`Metadata.ServerAlter`); nothing downstream recomputes a
different premise or provenance:

| arm | Owed | PublishPresent | ConditionalSource | ConditionalAudit | published `OrderedPresentConditional` |
|---|---|---|---|---|---|
| A1 Barrier | exactly `{server.batch_effects}` | `false` | `false` | `false` | — |
| A2 TerminalRefusal | `∅` | `false` | = consumed `OrderedAbsentConditional` | = `ConditionalSource` | — |
| A3 ProvenAlter | `∅` | `true` | = consumed `OrderedPresentConditional`; `false` when the source is the catalog row | `true` | = `ConditionalSource` |
| A4 Unproven | exact formula below | `true` iff a proven-present source exists | = consumed `OrderedPresentConditional`; `false` for catalog/no ordered source | `ConditionalSource \|\| own privilege consumable` | = `ConditionalSource` |

When `PublishPresent=true`, the published
`OrderedPresentConditional` equals `ConditionalSource` — never
`ConditionalAudit`: a statement's own conditional premise marks the
*audit conclusion* conditional, it does not make the *presence fact*
conditional. Catalog-sourced presence publishes `flag=false` even
when `ConditionalAudit=true` (A4 with own consumable premise — the
case that proves the two conditionality axes stay separate).

**Branch order — first match wins:**

- **ALT-A1 Barrier** — one of three distinct bound sources:
  whole-batch contamination (`batchState.contaminated` reaching the
  arm table directly, never via UnreadReason), channel taint, or a
  relation-scan barrier (the latter two bound as
  `UnreadReason = server.batch_effects`). All three are the same
  uniform **ordered-channel no-op**: Owed = exactly
  `{server.batch_effects}`; `PublishPresent=false`,
  `ConditionalSource=false`, `ConditionalAudit=false`. The arm
  **reports** existing uncertainty — it creates or modifies no
  entry, clears no prior `OrderedPresent`/`OrderedAbsent`, records
  no `serverWrite`, taint, or barrier, and rewrites no existing
  publication or provenance. Pre-existing channel state stands
  untouched. The bound snapshot/adjudication may record this
  statement's refused-read reason (`UnreadReason`), nothing more.
  A relation-scan barrier needs no provably-same entry to exist —
  an *unproven*-relation entry or write triggers it; the A1 arm
  must not assume a same-name barrier entry already stands, and
  must not mint one.
- **ALT-A2 TerminalRefusal** — `OrderedAbsent` on the provably-same
  identity. Owed = `∅`; blocker; `refused_absent`. The impossibility
  is proven by the ordered absent alone — privilege and options
  never reach the question; no native error is claimed.
  `ConditionalSource`/`ConditionalAudit` = the consumed
  `OrderedAbsentConditional` (a conditional absent consumed keeps
  the conclusion conditional; an unconditional absent gives none).
- **ALT-A3 ProvenAlter** — proven presence on the provably-same
  identity (ordered `OrderedPresent` OR catalog `Read && Exists`)
  AND consumable premise AND clean options. Owed = `∅`; notice;
  `conditional_alter`; `ConditionalAudit = true` (own consumable
  always feeds this arm); `ConditionalSource` = the ordered source's
  flag when the ordered channel answered.
- **ALT-A4 Unproven** — everything else in-domain. Owed = the exact
  formula, no open "others":
  - `server.existence` iff no proven-presence answer exists
    (ordered `OrderedPresent` absent AND no proven catalog row —
    zero-row, unread, or unbound). `OrderedAbsent` inputs never
    reach A4 (A2 took them).
  - `server.alter_privilege` iff premise ≠ consumable.
  - `server.alter_options` iff authored OPTIONS ≠ clean.
  `ConditionalSource` = `OrderedPresentConditional` when the ordered
  channel supplied the presence answer; `ConditionalAudit` =
  `ConditionalSource || consumable`.

**Presence-preservation theorem** (the divergence the order asks to
prove, not borrow — derived inside the declared presence-only model,
not a native-execution proof):

The official semantics scope the statement's effect to option
fields: "alters the server information for *server_name*, adjusting
any of the options … the corresponding fields in the `mysql.servers`
table are updated accordingly." The documented write set is the
named row's option columns — field values only. Within this engine's
presence-only ordered model, a write that updates option fields
changes **no tracked fact**: it cannot turn a present row absent
(the update targets the row, it does not remove it) and it cannot
turn an absent row present (updating fields of the named row does
not mint the row — the verb's own definition presupposes it). The
claim is therefore model-level: every adjudicable outcome —
executed, refused, or left unknown — preserves the pre-state
existence value. It does not rest on "the grammar shows no
INSERT/DELETE keywords" as a claim about the engine's internal
implementation paths; it rests on the documented field-update
semantics combined with the model tracking presence only.
Corollaries:

1. **Merge proof**: proven presence is republishable on every arm
   whose source proves it — catalog `Read && Exists` or consumed
   `OrderedPresent`. The published fact needs no premise: presence
   survives *whether or not* the alter ran, so an unproven-premise
   ALTER on proven present still publishes `OrderedPresent`
   unconditionally (catalog source) or with the consumed flag
   (ordered source).
2. **An audited ALTER introduces no new presence barrier**: a
   barrier records an undischarged unknown *effect on tracked
   state*. The channel tracks presence only; ALTER provably cannot
   change presence. An unproven ALTER on unknown existence leaves
   unknown unknown — nothing was made *more* unknown, no catalog
   answer goes stale (zero-row stays valid — ALTER never inserts).
   So A4 on unknown pre-state publishes nothing, barriers nothing,
   and creates no entry. This is the deliberate divergence from
   CREATE/DROP's unproven→barrier rule: there the write may have
   *deleted or created*, shadowing the read; here it provably
   cannot. (A1 still *owes* `server.batch_effects` when
   pre-existing state shadows the premises — "owes a gap" and
   "introduces a barrier" are different facts.)
3. **Consumed ordered answers stay valid**: `OrderedAbsent` consumed
   (A2) stands; `OrderedPresent` consumed (A3/A4) stands — the
   refusal/publish leaves the entry's ordered state untouched or
   re-affirms it.
4. **Contamination/taint/barrier sources still shadow**: a channel
   already carrying an undischarged effect keeps it — A1's own
   premises stay shadowed; the theorem protects *existence facts*,
   not premise shadowing.

**Worked rows** (premise `P` = own alter privilege; `O` = authored
options clean; — = not reached; resolver counts assume clean channel
and miss-cache first touch; loader ≤1/request):

| row | channel | ordered answer | catalog | P | O | arm | owed | outcome | level | post-state | resolver |
|---|---|---|---|---|---|---|---|---|---|---|---|
| 1 | contaminated | — | — | — | — | Barrier | {batch_effects} | blocked_batch_effects | notice | no mutation | 0 |
| 2 | taint/rel-barrier | — | no read | any | any | Barrier | {batch_effects} | blocked_batch_effects | notice | **no-op** — pre-existing shadowing state stands; nothing created/modified | 0 |
| 3 | clean | OrderedAbsent-same | — | any | any | TerminalRefusal | ∅ | refused_absent | blocker | absent stands (entry untouched) | 0 |
| 4 | clean | OrderedPresent-same | — | consumable | clean | ProvenAlter | ∅ | conditional_alter | notice | OrderedPresent re-published; flag inherited | 0 |
| 5 | clean | OrderedPresent-same | — | unproven | clean | Unproven | {alter_privilege} | unverified | notice | OrderedPresent re-published; flag inherited | 0 |
| 6 | clean | OrderedPresent-same | — | suppressed | clean | Unproven | {alter_privilege} | unverified | notice | same | 0 |
| 7 | clean | OrderedPresent-same | — | consumable | dirty | Unproven | {alter_options} | unverified | notice | same | 0 |
| 8 | clean | OrderedPresent-same | — | non-consumable | dirty | Unproven | {alter_privilege, alter_options} | unverified | notice | same | 0 |
| 9 | clean | none | proven present | consumable | clean | ProvenAlter | ∅ | conditional_alter | notice | publish OrderedPresent, unconditional flag=false | 1 |
| 10 | clean | none | proven present | non-consumable | any | Unproven | {alter_privilege} (+ alter_options iff dirty) | unverified | notice | publish OrderedPresent, unconditional | 1 |
| 11 | clean | none | proven present | consumable | dirty | Unproven | {alter_options} | unverified | notice | publish OrderedPresent, unconditional | 1 |
| 12 | clean | none | zero-row unknown | consumable | clean | Unproven | {existence} | unverified | notice | **no publish, no barrier, no entry** | 1 |
| 13 | clean | none | zero-row unknown | non-consumable | clean | Unproven | {existence, alter_privilege} | unverified | notice | none | 1 |
| 14 | clean | none | zero-row unknown | any | dirty | Unproven | {existence, alter_options} (+ alter_privilege iff non-consumable) | unverified | notice | none | 1 |
| 15 | clean | none | unread (no provider) | consumable | clean | Unproven | {existence} | unverified | notice | none | 0 — adjudication-unit input; public no-provider lands row 16 |
| 16 | clean | none | unread (no provider) | non-consumable | any | Unproven | {existence, alter_privilege} (+ alter_options iff dirty) | unverified | notice | none | 0 |
| 17 | clean | none | resolver/loader error | — | — | error exit | — | — | — | bounded error propagates; no arm | — |

Rows 10–11 register the candidate's sharpest divergence: an unproven
ALTER on proven presence *still re-publishes* `OrderedPresent` —
the presence fact never depended on the premise, and the
preservation theorem proves the write cannot remove it. Republish
is **decided** (§G P1), and its cost is registered honestly: a
re-published catalog presence is consumed by same-name *and*
ASCII-fold-alias statements, and — under the unchanged relation
scan — a new `present` entry can additionally throw
`server.batch_effects` at later *unproven-relation* names (AM-7.3
demonstrates). Republish vs. leave-the-entry therefore differ in
later gaps, coverage, and read counts — not only in fold-alias
reach — and the contract accepts that as the priced consequence of
keeping presence provable.

**Premise discharge on known-refusal branches** (work order D):
on A2 the impossibility is proven by ordered absent alone — nothing
else is owed, not even a dirty authored OPTIONS list; on A1 every
premise stays shadowed and only `server.batch_effects` is owed.
Rule evaluation and ordered apply consume the **same** adjudication
record attached at bind (proposed `Metadata.ServerAlter`); no second
truth is recomputed downstream.

## E. Identity relations, transitions, and historical reconciliation

Relation contract unchanged: byte-equal or ASCII-fold-equal within
the registered ASCII subset ⇒ same; else unproven; distinct stays
empty. "Case-insensitive" is a catalog property, not license to
widen the relation domain.

**State-transition table (candidate, per arm):**

| arm | pre-state on provably-same identity | modeled transition |
|---|---|---|
| Barrier (all three sources) | any | **uniform no-op** — no entry created or modified, no ordered answer cleared, no write record, taint, or barrier minted; the shadowing state that produced the bound `UnreadReason`/contamination already stands |
| TerminalRefusal | `absent` entry | untouched — absent stands; `publishedArm` preserved |
| ProvenAlter | ordered-source entry | `present=true`, `presentConditional` = consumed `OrderedPresentConditional`, `absent/barrier=false` |
| ProvenAlter | catalog source, no entry | create entry: `present=true`, `presentConditional=false` |
| Unproven (ordered present source) | ordered-source entry | re-publish: `present=true`, `presentConditional` = consumed flag (inherited, never laundered) |
| Unproven (catalog present source) | catalog source, no entry | create entry: `present=true`, `presentConditional=false` — unconditional presence |
| Unproven (unknown source) | any | **no publish, no barrier, no entry** — preservation theorem |
| boundary ALTER (all forms) | any | **unchanged accepted behavior**: ident name → `serverWrite` + barrier on non-distinct entries; literal name → whole-channel taint |

**C11–C13 / E14 reconciliation** (historical design rows re-judged
against the accepted baseline):

| row | historical claim | verdict under this contract |
|---|---|---|
| C11 srv-alter-present | notice + `present` + opaque revision marker | superseded by A3/A4 — presence re-published, no revision marker (rejected, §C) |
| C12 srv-alter-unknown | review + gap, post-state unchanged | A4-on-unknown — "unchanged" sharpened: no publish, no barrier, no entry (preservation theorem) |
| C13 srv-alter-absent | reject `[p] alter.absent`, **"unreachable"** | **now reachable** — accepted DROP publishes `OrderedAbsent` (proven/convergent arms); A2 is the live refusal arm |
| E14 `ALTER; DROP` | boundary write record → stmt2 barrier | unchanged `[cur]` — boundary ALTER keeps write-record semantics; new audited-ALTER rows in case matrix §7 |

**Composition consequences** (work order D examples):

- `DROP s` (proven) → `ALTER SERVER s` → A2 refusal, `refused_absent`,
  complete, reject; the absent stands for a later `CREATE SERVER s`.
- `CREATE s` (proven) → `ALTER s` → A3 if consumable+clean
  (`conditional_alter`, inherited conditional source) → `DROP s`
  still consumes `OrderedPresent` (re-published by the alter).
- `ALTER s` first (catalog present) → `DROP S` — the fold-alias hits
  the re-published `OrderedPresent` → the drop reads proven presence
  without a second catalog read (objectCache already held the row
  anyway; the publish additionally answers the alias at channel
  level). Same entry, other side: `ALTER sé` (catalog present) →
  `ALTER se` — `se` hits the relation-scan barrier (`serverRel`
  unproven) and owes `server.batch_effects`; without the publish `se`
  would have taken its own zero-row read and owed `existence`. Both
  directions are the priced P1 consequence.
- `ALTER s` unknown → `ALTER s` again → both A4-on-unknown; no
  entries created; each may fire its own first-touch catalog read
  (objectCache deduplicates same verbatim name).

**Public output** (per arm; rule enabled, documented default levels;
aggregation rules unchanged — aggregate coverage = min, verdict =
reject on any blocker, review while gaps stand, pass when clean):

| arm | outcome token | level | owed | marker | stmt coverage | verdict contribution | loader | resolver | catalog SQL |
|---|---|---|---|---|---|---|---|---|---|
| Barrier | blocked_batch_effects | notice | {batch_effects} | none | unverified | gap floor → review | ≤1 (request-shared) | 0 | 0 |
| TerminalRefusal | refused_absent | blocker | ∅ | OrderedAbsentConditional | complete | blocker → reject | ≤1 | 0 | 0 |
| ProvenAlter | conditional_alter | notice | ∅ | always + inherited flag | complete | none | ≤1 | 0 or 1 (source) | 0 or 1 |
| Unproven | unverified | notice | exact formula | conditional iff source-flag or consumable | unverified | gap floor → review | ≤1 | 0 or 1 | 0 or 1 |

CLI exits (future execution probes — registered plan, none run
here). Version resolution is request-level and precedes every arm;
the cases that must stay distinct:

| input | resolution | class | audit-layer L/R/S |
|---|---|---|---|
| pin `8.0.36` (providerless) | in-domain | arms apply; single clean ALTER under the declared policy → A4 unverified (no loader, no provider) | 0/0/0 — no provider to read |
| no pin, no provider, no other version source | unknown | `unaudited` + `gate:"version"` boundary | 0/0/0 — providerless has nothing to read |
| no pin, provider observed in-domain | resolved from banner | arms apply — the pin is not what gates | per arm |
| pin out-of-domain (e.g. `5.7.44`) | resolved, outside 8.0.x | `unaudited` + `gate:"version"` boundary | bind may still issue reads — registered below |
| TiDB dialect | vendor boundary | `unaudited`, product not supported | bind does not reach the MySQL resolver |
| malformed pin `8.0.x` | — | typed `ErrInvalidTargetVersion` preflight, rc 2 | 0/0/0 — preflight precedes enrich |
| MySQL request + observed non-MySQL product | — | typed `ErrDialectProductMismatch` | 0/0/0 |
| pin + observed version disagree | — | typed `ErrTargetVersionMismatch` | 0/0/0 |

The last two mismatches terminate before any arm; audit-layer
loader/resolver/SQL counts are zero and the version observation
itself is not billed into L/R/S. For the **version boundary
keep-vs-short-circuit** question this contract chooses: **keep the
existing boundary bind behavior** — `bindMySQLServer` has no
`Unsupported` predicate today, so a version-gated audited-shape
ALTER still walks the six-step order and issues its catalog read
(provider present ⇒ R=1/S=1; the read's result is never used for
the boundary statement). The `R=1/S=1` figure holds under its
stated preconditions only: clean channel, no ordered answer,
cache miss, identifier-form name, and a provider that actually
executes the query — the channel guards and the cache can still
suppress the read on other inputs; the keep-boundary-bind
decision stands. Introducing an earlier exit would change
read counts and possibly error propagation and needs its own
contract ruling. `rc` follows the existing verdict→exit mapping
per request and policy — the accepted CREATE CLI evidence recorded
review ⇒ rc 0 under its declared notice-level policy and typed
preflight ⇒ rc 2; this contract asserts no generalized
"pin ⇒ rc 0 / no pin ⇒ rc 1" rule: A2's blocker, multi-statement
aggregates, and other policies map by the existing rules,
unchanged.

Three-probe plan (future execution order — registered, none run
here). The canonical recipe — `policy-alter.yaml` content, the
three exact commands with separated stdout/stderr and immediate
rc capture, and the plan conditions — lives in
`case-matrix.md` Group 9 ("Registered CLI probe plan", PLAN /
UNEXECUTED). Its frozen expectations under that fixed
configuration only:

| probe | expected | rc |
|---|---|---|
| pin `8.0.36` | A4; owed `{server.existence, server.alter_privilege}`; main + advisory = 2 notice findings; main `unverified`, no marker; verdict review; L/R/S 0/0/0 | 0 |
| no pin, no other version source | `unaudited` + `gate:"version"`; findings=0, gaps=0; coverage `incomplete`; verdict review; L/R/S 0/0/0 | 1 |
| malformed pin `8.0.x` | typed `ErrInvalidTargetVersion` preflight; no audit result; L/R/S 0/0/0 | 2 |

No new CLI flags; no gap rewrites the accepted preflight
semantics; the numbers are not a generalised pin⇒rc rule.

## F. Registration — proposed seams (execution order implements)

Files a future ALTER execution order would touch (registered, not
implemented — `.go` changed = 0 in this doc round):

- `internal/infrastructure/parser/tidb/coverage_boundary.go` —
  `mysqlServerAuditedForm`: admit `ServerKindAlter` under
  NameIdent ∧ ≤64-rune (G3/G4); ALTER has no wrapper gate.
- `internal/application/audit/metadata.go` — extend the phase-2 G2
  predicate to `ServerKindAlter`; attach `Metadata.ServerAlter`
  adjudication for audited ident-form ALTER.
- new `internal/domain/spec/server_alter_premise.go` —
  `ServerAlterPrivilege`/`ServerAlterArm`/`ServerAlterAdjudication`,
  `ServerFactAlterPrivilege`, `ServerFactAlterOptions`,
  `AdjudicateServerAlter`.
- `internal/domain/spec/metadata.go` — `Metadata.ServerAlter` field.
- `internal/application/audit/server_drop_premise.go` —
  `statementsNeedMySQLServerPrivilege` widen to alter kind;
  `serverAlterPrivilegePremise()` projection.
- `internal/application/audit/batch_state_server.go` —
  `ServerKindAlter` apply block implementing §E transitions.
- `internal/application/audit/batch_state_server_srs.go` —
  **no change required** (R2-C static-seam verdict): the bound
  baseline excerpt (`r2-bind-excerpt.txt`, recorded as non-repo
  evidence) shows `bindMySQLServer` is kind-agnostic — it reads
  `decl.Name`/`decl.NameIdent` only and walks the same six-step
  order (contamination → taint → relation barrier → ordered
  absent/present consumption → resolver fallback) for every
  `DDL.Server != nil`. An audited ALTER inherits exactly the
  ordered-consumption behavior the contract relies on. The kind
  predicates that do need extension live elsewhere (the four
  entries above plus the loader gate below).
- `internal/domain/rule/ddl/mysql_server_rules.go` —
  `mysqlAlterServerRule` (`ddl.mysql.server.alter.notice`),
  outcome tokens, ALTER-scoped advisory decision (§C).
- rule registry + `configs/deltascope.example.yaml` + default policy
  — register the new rule ID exactly as the create sibling did.
- `pkg/deltascope` — no public-API change anticipated (same
  request/mode/loader surfaces).

No new public API; no new CLI flags; no new provider method — the
loader, resolver, cache, and ordered channel are all reused.

**Bind invariant** (R2-C): before consuming objectCache/provider
presence, an audited ALTER has already passed the same ordered
guards the channel defines — whole-batch contamination exit,
channel taint, and the write/entry relation scan on
`decl.NameIdent`. It never bypasses a prior `OrderedAbsent`,
`OrderedPresent`, or unresolved shadowing effect to reach the
catalog. Evidence: `r2-bind-excerpt.txt` (baseline `28152dfd`,
line-numbered verbatim excerpts of `bindMySQLServer`, the
`metadata.go` call sites, `mysqlServerAuditedForm`,
`applyServerStatement`, the loader gate, and the provider-side
name-form decline; SHA-256 in the R2 receipt).

## G. Decisions — P1–P7 adjudicated (R2)

All seven items are now **decided**, not open; this table is the
decision record. Deferred-evidence notes are marked where they
apply — none is a contract blocker.

| # | decision | ruling |
|---|---|---|
| P1 | Republish `OrderedPresent` on proven-presence arms | **adopted** — presence survives the write under the preservation theorem. Priced consequences registered in §D/§E: same-name and fold-alias reuse, plus relation-scan `batch_effects` thrown at unproven-related names; gaps, coverage, and read counts can differ — not only fold-alias reach |
| P2 | ALTER-scoped `connection_members` advisory | **adopted, A3/A4 only** — fixed public projection (README §C): independent advisory finding `object=server, operation=alter_server, notice=connection_members` with fixed text "ALTER SERVER specifies a partial options update; stored and resulting connection member sets are unverified." at the configured level; no outcome/marker/gap on the advisory; main finding projects `conditional_audit` only from `Metadata.ServerAlter.ConditionalAudit`; A1/A2 never emit it |
| P3 | `OrderedPresentConditional` on ordered-source re-publish | **inherit the consumed flag verbatim** — never launder to unconditional |
| P4 | Audited-ALTER write record | **none** — audited ALTER records no `serverWrite` on any arm, A1 included; boundary ALTER keeps the accepted write-record/taint behavior |
| P5 | Native absent-case error class for `ALTER` on a missing row | **not claimed** — the A2 refusal rests on the ordered-absent proof only; native error class remains optional oracle evidence **deferred** to a future execution order |
| P6 | `server.alter_options` on duplicate-key / PORT-range dirt | **owed-fact only** — the manual is silent on ALTER-side outcomes; no refusal or native-error claim; native observation **deferred**, same treatment as `server.create_options` |
| P7 | Own `wrapper` premise for ALTER | **none** — no authored wrapper clause exists on this verb; public wording stays wrapper-agnostic |

Nothing under §G remains PEND for this contract slice.

## H. Non-goals

Literal-name audit, TiDB semantics, string-literal decode widening,
`serverRelDistinct` extension, stored-options content reads, remote
connectivity, FEDERATED availability, native binlog/implicit-commit
modeling, the rejected opaque revision marker, SRS, ALTER INSTANCE,
T27. Historical failures and evidence stay untouched; this doc
asserts no test, build, probe, native, golden, or Docker run.

## Checks run this round

`git status`, `git rev-parse` on baseline; source reads of every
cited seam on `28152dfd` (parser, coverage gate, bind, apply,
privilege premise, rules); MySQL 8.0 manual fetch for §15.1.8 and
§15.1.18. **This work order ran no project tests, builds, or
runtime probes** — by the order's own constraint (docs-only).
