# T26-SERVER-CREATE-CONTRACT-01 — CREATE SERVER first-slice semantic contract

Status: PROPOSED / Reviewer pending. Type: docs-only contract
convergence. Baseline: `9e21ace9` (tree `9a6efbd6`), branch
`milestone/mysql-tidb-ddl-completion`. Parent: T26 / issue #105 under
issue #79.

This contract defines what the first audited `CREATE SERVER` slice
consumes, what each conclusion rests on, and how one adjudication
drives the public result and the ordered post-state. It recommends
ONE first-implementation domain; it does not implement anything.

Approved boundaries carried in unchanged from the accepted DROP slice
(`9dad37b8`, tree `eb544128`, R2 PASS): conservative identity
relations, zero-row = unknown (never absent), positive-only
direct-global-grant privilege witness, `observed_account_new_session`
scope, no future-execution claims.

## A. Syntax and version support table

Official basis: MySQL 8.0 Reference Manual §15.1.18 CREATE SERVER
(dev.mysql.com/doc/refman/8.0/en/create-server.html, fetched this
round). The documented grammar:

```
CREATE SERVER server_name
    FOREIGN DATA WRAPPER wrapper_name
    OPTIONS (option [, option] ...)
option: HOST char | DATABASE char | USER char | PASSWORD char
      | SOCKET char | OWNER char | PORT numeric
```

Documented semantics the contract consumes:

- `SUPER` privilege required.
- `server_name`: global (no database qualifier), max 64 characters,
  longer names **silently truncated**, case-insensitive, may be a
  quoted string.
- `wrapper_name`: an identifier, may be single-quoted.
- Character option literals: UTF-8, max 64 chars (silently truncated),
  default empty string. Numeric literals: 0–9999, default 0.
- `OWNER` is currently not applied (documented no-op).
- USER/HOST/DATABASE are described as mandatory **for establishing a
  connection** — advisory; the statement itself stores blank/zero
  defaults. Not a statement-level refusal source.
- Implicit commit; not written to the binary log.
- **No `IF NOT EXISTS` form exists in this grammar.**

Four-way table — database syntax vs parser recognition vs current
product vs proposed first slice:

| form | MySQL 8.0 grammar | parser today | product today | first slice |
|---|---|---|---|---|
| identifier name (`s1`, `` `s1` ``) | yes | recognized | unaudited boundary | **audited** |
| string-literal name (`'s1'`) | yes | recognized (NameIdent=false) | unaudited boundary | stays unaudited — decoded catalog identity unprovable (same as DROP) |
| name spelling >64 runes | truncated to 64 | recognized | unaudited | stays unaudited — truncation makes the effective catalog identity unproven (decoded length also unproven without full string decode) |
| `FOREIGN DATA WRAPPER mysql` (ASCII fold-equal) | `mysql` is the FEDERATED wrapper | recognized | unaudited | **audited** |
| other wrapper spellings | grammar accepts any identifier | recognized | unaudited | stays unaudited — no provable wrapper registry exists; an unregistered wrapper's runtime outcome is unproven |
| wrapper as string literal | doc allows single-quoted | recognized | unaudited | stays unaudited (same literal-identity reasoning as the name) — actually see note* |
| OPTIONS ≥1 documented member | yes | enforced (whole-list parse or no claim) | unaudited | **audited** |
| unknown OPTIONS key | not in grammar | unclaimed → base parser diagnostic | parse error (exit 2) | unchanged — parser boundary |
| duplicate OPTIONS key | grammar admits the list | recorded in `DuplicateKeys` | unaudited | stays audited but `server.create_options` owed — effective option set unproven, never publishes OrderedPresent |
| missing USER/HOST/DATABASE | stores defaults; advisory warning only | parsed fine | unaudited | audited; notice-level completeness note, not a premise blocker |
| PORT outside 0–9999 | documented bound | parsed (digits only, range unchecked today) | unaudited | out-of-range numeric → `server.create_options` owed (registered range check is a proposed implementation step) |
| `IF NOT EXISTS` | **does not exist** | falls through → base parser | parse error or other claim | unchanged — no convergent arm exists for CREATE |
| trailing tokens | — | unclaimed → base parser | parse error (exit 2) | unchanged |

*The recognizer calls `mysqlServerName` for the wrapper, which admits
string-literal spellings; the doc says the wrapper "may be quoted with
single quotation marks", i.e. a literal is grammatical. Recommendation:
first slice requires the wrapper to be the identifier/literal spelling
that ASCII-folds to `mysql`; other wrappers stay on the named unaudited
boundary — not a parse failure, because the parse itself is valid.
This is a coverage-boundary narrowing, not a grammar claim.

TiDB: the whole family stays the vendor boundary — unchanged.

## B. CREATE privilege consumption contract

Required privilege: `SUPER` — identical to DROP SERVER (same manual
requirement). The positive-witness contract is therefore the same one
already approved: direct global `SUPER`/`ALL PRIVILEGES` on `*.*` read
verbatim from `SHOW GRANTS FOR CURRENT_USER()` on the provider's single
acquired `*sql.Conn`, gated behind `observed_account_new_session`,
positive-only, version-domain checked.

Recommended supply: **reuse the same request-level raw observation and
the same loader hook** (`LoadMySQLServerDropPrivilege` — the read is
operation-agnostic grants evidence, not drop-specific). What stays
distinct per the work order:

- A CREATE statement's own premise projection is operation-specific:
  proposed `ServerCreatePrivilege` (unproven / consumable /
  suppressed) derived from the same raw witness and the same ordered
  scope — never copied from a satisfied `server.drop_privilege` fact.
- The owed fact key is distinct: proposed `server.create_privilege`,
  alongside existing `server.drop_privilege`. A finding names the
  operation whose premise is owed.
- A prior DROP's `conditional_audit` marker never counts as the
  CREATE's own privilege evidence; each statement's premise projects
  independently, and the marker's conditional-source propagation
  applies symmetrically (see D).
- Scope terminators: unchanged list (authored GRANT/REVOKE operations;
  `set_role`/`set_default_role`/`grant_role`/`revoke_role`/
  `grant_proxy` boundary features). They end the CREATE premise's
  range identically — same mechanism, no new vocabulary.
- Loader gate: the single-shot read currently fires when the batch
  names ≥1 audited identifier-form DROP SERVER. Proposed widening:
  fire when ≥1 audited identifier-form DROP **or CREATE** SERVER is
  present. One observation still serves all audited family statements
  in the batch (never re-read per statement).
- Loader/provider seam unchanged — no new public hook. The
  `observed_account_new_session` mode already admits this family.

Nothing here widens the witness's claim: it vouches only for direct
global grants observed on the read connection — not pooled-session
effective privileges, not the future executor, not role augmentation
(deferred), not denial.

## C. Existence, success premise, and adjudication order

The existence answers a CREATE consumes, in the approved order
(same six-step shape as DROP):

1. whole-batch contamination or channel taint → bound unvouched /
   batch_effects, no resolver access;
2. relation scan over entries and recorded writes → barrier;
3. ordered channel answers first on a provably-same identity:
   `OrderedAbsent` (from a prior proven drop/convergent arm) proves
   the slot free; `OrderedPresent` (new, see D) proves it occupied;
4. provider resolver read (`mysql.servers`) only when the channel
   cannot answer: row → proven present; **successful zero rows →
   unknown, never absent** (grant-scoped visibility; unchanged);
   identity unproven → read unissued;
5. provider query/scan/iteration error → propagates bounded, never a
   state value;
6. barrier/taint shadows every catalog answer.

Consequence worth stating plainly: **a first-statement CREATE can
never be a proven create.** The slot-free premise needs proven
absence, and the only proven-absence source is a published
`OrderedAbsent` — a zero-row catalog read does not qualify. A lone
`CREATE SERVER s1` therefore always owes `server.existence` (or reads
present → refusal, or barrier). This is the same honesty the DROP
slice applies to absence.

Audited-expected vs database-actual: a "proven create" arm means the
audit model's expected creation under proven premises (slot provably
free, privilege witnessed, input valid). It is NOT a claim the
database executed it — offline audit never executes, and FEDERATED's
availability, option semantics, and runtime errors stay outside what
the arm vouches for. The refusal arm is the mirror: provably-occupied
slot + plain create is a logical impossibility proven by the evidence
alone — it claims no native error number and no MySQL error ordering.

## D. Shared adjudication and post-state table

Proposed new types (names follow the DROP pattern; marked NEW):

- `ServerCreateArm` (NEW): `Unproven`, `ProvenCreate`,
  `TerminalRefusal`, `Barrier` — no convergent arm (no `IF NOT
  EXISTS` exists), no guarded arm.
- `ServerCreateAdjudication` (NEW): `Privilege`, `Arm`, `Owed`,
  `PublishPresent`, `ConditionalSource`.
- `ServerCreatePrivilege` (NEW): `Unproven` / `Consumable` /
  `Suppressed` — same projection mechanism as DROP's.
- `ServerState.OrderedPresent`, `ServerState.OrderedPresentConditional`
  (NEW): the ordered present answer; `json:"-"`, never serialized;
  mutually exclusive with `OrderedAbsent`.
- Facts: existing `server.existence`, `server.batch_effects`;
  NEW `server.create_privilege`, `server.create_options`.
- Outcome tokens on findings: NEW `conditional_create`,
  `refused_present`; reused `blocked_batch_effects`, `unverified`.
- `conditional_audit=observed_account_new_session` metadata: same
  marker, emitted on a statement's own conditional arm and on arms
  consuming a conditionally-published ordered state (inherited
  conditional-source propagation, symmetric with DROP).

Truth table (each row is one complete adjudication; "same" = provably
same identity under the approved relation contract):

| # | input domain | ordered/catalog answer | create privilege | options | arm | owed (exact) | finding level | outcome | coverage | ordered post-state | resolver |
|---|---|---|---|---|---|---|---|---|---|---|---|
| 1 | valid | `OrderedAbsent` same | consumable | clean | ProvenCreate | ∅ | notice | conditional_create | complete | publish `OrderedPresent` (+Conditional) | ordered answered; no read |
| 2 | valid | `OrderedPresent` same | any | any | TerminalRefusal | ∅ | blocker | refused_present | complete | present stands | no read |
| 3 | valid | catalog present (proven row) | any | any | TerminalRefusal | ∅ | blocker | refused_present | complete | present stands | 1 read |
| 4 | valid | zero-row unknown | consumable | clean | Unproven | {server.existence} | notice | unverified | unverified | entry barrier | 1 read |
| 5 | valid | zero-row unknown | unproven | any | Unproven | {server.existence, server.create_privilege} | notice | unverified | unverified | entry barrier | 1 read |
| 6 | valid | zero-row unknown | suppressed | any | Unproven | {server.existence, server.create_privilege} | notice | unverified | unverified | entry barrier | 1 read |
| 7 | valid | `OrderedAbsent` same | unproven | any | Unproven | {server.create_privilege} | notice | unverified | unverified | entry barrier (absent shadowed — undischarged effect) | no read |
| 8 | valid | unread (no provider/mode) | unproven | any | Unproven | {server.existence, server.create_privilege} | notice | unverified | unverified | entry barrier | 0 reads |
| 9 | valid | any | any | duplicate keys or invalid range | Unproven | {server.create_options} (+ others as rows dictate) | notice | unverified | unverified | entry barrier | per channel |
| 10 | any | barrier/taint | any | any | Barrier | {server.batch_effects} | notice | blocked_batch_effects | unverified | entry barrier | no read |

Rows 1–2 keep public output and post-state on the same adjudication:
no arm publishes a present the findings don't also show, and no
finding claims a create the post-state didn't publish. Row 7 is the
work order's "unknown CREATE effect shadows prior OrderedAbsent":
an unproven CREATE arm is an undischarged unknown effect on the
identity — the earlier absent is cleared to the barrier shape
(identical to the DROP contract's non-publish arms).

`OrderedPresent` definition (required by the work order):

- **Production**: only `ProvenCreate`, only on a provably-same
  identity, only on a clean channel. `OrderedPresent=true` implies
  `Exists`-as-ordered-answer only, Status never promoted by the
  channel, Read never set by the channel.
- **Consumption**: same position as `OrderedAbsent` — inside
  `bindMySQLServer`, after contamination/taint/barrier checks, before
  any provider or cache access; consumed by later CREATEs (refusal)
  and later DROPs (proven-present pre-state).
- **Conditional source**: `OrderedPresentConditional` = publishing
  arm's conditional source; consumers keep `conditional_audit` on
  refusal/drop arms resting on it.
- **Invalidation/replacement**: mutually exclusive with `OrderedAbsent`
  on one entry — a proven drop publish replaces present→absent; a
  proven create publish replaces absent→present. Every non-publish arm
  or unaudited/unknown effect clears the state to the barrier shape
  (absent=false, present=false, barrier=true, publishedArm reset).
- **No unconditional production exists** today — the only publisher is
  the conditional arm; the field exists so the contract stays honest
  if an unconditional arm ever lands.

Aggregate: statement coverage complete on zero-gap arms
(ProvenCreate/TerminalRefusal), unverified under any owed fact;
aggregate = min over statements; verdict = reject on blocker,
review while gaps stand, pass when every statement is clean — the
existing `report.Aggregate` rules, unchanged.

## E. Identity relations and cross-statement composition

Relation contract unchanged: byte-equal or ASCII fold-equal within the
registered ASCII-CI subset → `serverRelSame`; everything else
unproven; `serverRelDistinct` keeps its empty provable domain. The
manual's "case-insensitive" is a catalog property, not a license to
widen the relation domain — CREATE inherits the same conservatism.
Barrier/taint checks still precede every provider/cache/ordered
consumption.

Composition chains (each row is the contract's expected behavior):

1. **conditional DROP → CREATE → DROP (same name, consumable
   privilege throughout)**: drop1 publishes conditional
   `OrderedAbsent`; create consumes it → ProvenCreate → publishes
   conditional-source `OrderedPresent` (resolver: 0 new reads);
   drop2 consumes `OrderedPresent` as proven-present pre-state +
   consumable → proven drop arm → publishes `OrderedAbsent`
   conditional again (0 new reads). Markers: every statement carries
   `conditional_audit` — drop2's is source-inherited.
2. **CREATE → CREATE (same name)**: create1 owes existence (zero-row
   unknown or unread) → Unproven → entry barrier; create2's barrier
   scan hits the same-identity barrier → `blocked_batch_effects`,
   owes exactly `server.batch_effects`, 0 resolver reads. (If create1
   had proven-published present, create2 → refused_present.)
3. **valid OrderedAbsent → unknown-effect CREATE → DROP**: an
   unaudited literal-name CREATE taints the whole channel and clears
   every absent (existing apply path); an audited-but-unproven CREATE
   arm on the same identity clears that entry's absent → barrier;
   the later DROP reads `blocked_batch_effects`, owes
   `server.batch_effects`, 0 resolver reads.
4. **barrier/taint → CREATE**: tainted channel → create binds
   unvouched (batch_effects arm, no read); its own post-state records
   the undischarged effect — it never publishes present.
5. **non-same-relation interleave** (`s1` drop published absent,
   `S1` byte-equal→same; `s2`/`Ｓ１` fold-unproven): a CREATE on a
   relation-unproven name barriers against the recorded state (the
   scan treats unproven relation as barrier before any read).
6. **scope terminator before CREATE**: `REVOKE …` or `SET DEFAULT
   ROLE` between the witness and the CREATE → privilege suppressed →
   `server.create_privilege` owed even when a conditional
   `OrderedAbsent` stands (row 7 shape); the terminator never claims
   a grant was revoked.

If the future implementation extends DROP consumers to read
`OrderedPresent` (drop arm input) or extends bind/apply, that is a
new compatibility change with its own regression points — not a
re-verification of the DROP PASS.

## F. Public output and privacy

Unchanged redaction: `ServerDeclaration.PublicSQL()` already emits
`CREATE SERVER <redacted>`; the parser never stores option literal
values (keys + value kinds only); server name, wrapper, option keys,
account, and grant lines stay internal. New findings carry the fixed
outcome token, the bounded owed-fact keys, `conditional_audit`, level,
message, and explanation — worded like the narrowed DROP texts:
`refused_present` states only the impossibility the evidence proves
(no native error number); `conditional_create` states the modeled
creation under the conditional witness; `blocked_batch_effects` and
`unverified` reuse existing wording patterns.

The test plan checks emitted finding fields (message / explanation /
metadata) for protected content — server identity, OPTIONS payload
fragments, account, grant lines — per actual exit, the same shape as
the accepted vertical privacy check. It is not registered as a
whole-serialized-result redaction proof.

## G. Compatibility and implementation mapping

Read-only source review; nothing implemented. Actual touch points a
future execution order would touch:

- parse/declaration: `internal/infrastructure/parser/tidb/mysql_server.go`
  already claims the whole form and records NameIdent/Wrapper/
  Options/DuplicateKeys/NameSpellingRuneCount — the first-slice gates
  (>64 runes, non-`mysql` wrapper, literal wrapper) are proposed
  coverage-boundary conditions, not new grammar.
- coverage boundary: `internal/infrastructure/parser/tidb/coverage_boundary.go`
  — the `unaudited` pin narrows to the accepted slice.
- spec: `internal/domain/spec/server.go` (`OrderedPresent`,
  `OrderedPresentConditional`), new `server_create_premise.go`
  (`ServerCreateArm`/`Adjudication`/`Privilege`,
  `ServerFactCreatePrivilege`, `ServerFactCreateOptions`,
  `AdjudicateServerCreate`).
- application: `metadata.go` (adjudication attach for `ServerKindCreate`),
  `server_drop_premise.go` (loader gate widened to the audited family;
  create-premise projection), `batch_state_server_srs.go`
  (`OrderedPresent` consumption position), `batch_state_server.go`
  (create apply arm + publish/invalidation; `batchServerEntry`
  gains `present` or a state pair).
- rule surface: `internal/domain/rule/ddl/mysql_server_rules.go` — a
  create-arm sibling rule or a widened family rule reading the same
  adjudication; outcome tokens above.
- SDK/CLI: no new public API proposed — `PrivilegeAuditMode`, the
  loader hook, and `Request` already admit the family; outcome values
  are metadata tokens, not API. `testdata/ddl-golden/T26.json`
  create-server cases move from `unaudited` to audited expectations
  in the future implementation candidate — registered here as a
  future manifest change, never a retroactive edit.

Current boundary to state: every `create_server`/`alter_server`
statement is `unaudited` today (`boundary: unaudited`, object
`server`) — including inside `mysql-server-mixed-batch`. Those
expectations change under the future candidate, not under this
contract.

## Verification plan — PLAN / UNEXECUTED (no execution this round)

| case | premises | expected | layer |
|---|---|---|---|
| lone ident create, no mode | zero-row or unread | unverified; owes {existence, create_privilege}; entry barrier; 0–1 resolver reads | public `Audit` |
| conditional drop → create → drop | consumable | conditional_create → OrderedPresent(conditional) → conditional_drop again; resolver total 1 (first drop's) | public `Audit`, query counts |
| catalog present + consumable | proven row | refused_present blocker; reject; owes ∅; 1 read | public `Audit` |
| OrderedPresent → plain create | published | refused_present; 0 new reads | public `Audit` |
| OrderedAbsent → create, unproven privilege | suppressed or no mode | owes exactly {create_privilege}; absent shadowed to barrier | public `Audit` |
| zero-row + consumable | unknown | owes exactly {existence}; unverified | public `Audit` |
| duplicate OPTIONS key | consumable + absent | owes exactly {create_options}; no publish | public `Audit` |
| literal name / >64 runes / non-mysql wrapper | any | stays `unaudited` boundary; channel taint or write record unchanged | public `Audit` |
| tainted channel → create | prior literal write | blocked_batch_effects; 0 reads | public `Audit` |
| terminator → create | witness + REVOKE | owes {existence, create_privilege} (suppressed) | public `Audit` |
| non-same-relation create | prior same-bucket writes | barrier before resolver | public `Audit` |
| privacy | any | finding fields carry no name/wrapper/options/account/grant text | public `Audit` field scan |
| defaults | no mode, no provider | legacy unverified shape; no loader call | public `Audit` |

Query-count layers to assert separately: privilege loader (≤1 per
request), server resolver reads per statement, catalog SQL counts —
only levels with direct assertions get registered as covered.

## Actual checks run this round

- `git status --short` on tracked files: clean (reported in receipt).
- `git diff --check`: pending at commit time — clean expected.
- Source-review consistency: every code reference above verified
  against the baseline tree by reading the named files.
- Official-source consistency: section A checked against the manual
  text quoted at fetch time.
- No `go test`, build, CLI, wrapper, native SQL, Docker, or database
  operation was run or claimed.

## Remaining questions

None blocking. Registered non-goals: ALTER SERVER, SRS, ALTER
INSTANCE, literal-name audit, TiDB semantics, wrapper registry beyond
`mysql`, full string-literal decode, `serverRelDistinct` extension —
all stay deferred exactly as the DROP contract left them.
