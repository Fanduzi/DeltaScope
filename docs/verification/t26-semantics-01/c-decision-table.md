# T26-SEMANTICS-01 §C — consolidated semantic decision table (R2 revision)

Availability vocabulary (per premise):

- `proven` — the statement path obtains the fact today (declaration
  field or wired provider read).
- `unknown` — obtainable in principle but the answer stays unproven.
- `unproven` — bound but never read, or read with an undecidable
  answer; distinct from `unbound`.
- `unbound` — nothing bound at the metadata member level
  (`Metadata.Server == nil` — e.g., the bind never ran); **distinct**
  from a non-nil bound state that is merely unvouched (see ladder).
- `no-capability` — neither the declaration nor any accepted fact can
  supply it (WKT grammar, decoded-length measure, full option values).
- `ordered` — supplied only by the same-adjudication ordered channel
  (G-src.4), never by the catalog.

**R2-A ruling applied.** `server.connection_semantics` is ruled
*inapplicable to DROP* — the drop removes the server definition and
never consumes HOST/USER/PASSWORD runtime correctness or remote
reachability, so no exclusion is needed on DROP rows (it stays a
CREATE/ALTER-side proposal, unapproved here). `server.execution_premises`
as a coverage-neutral umbrella is **rejected**: the documented SUPER
requirement for DROP SERVER is a real execution premise, the SERVER
statement path wires no privilege proof, and R4 point 3 makes a needed-
but-missing external fact a **bounded gap + `unverified`**, not an
exemption. Scope note: the absence is per-path — the T24 premise bundle
does include grant/permission reads; only the SERVER path lacks one
(this round adds no privilege collection).

Consequences for every DROP row below:

- provable `server.existence` still drives findings;
- the drop's privilege premise is unproven → bounded gap
  `server.drop_privilege` → `coverage=unverified`;
- no new proven `absent` is published from a `present`/`unknown`
  pre-state — a possibly-executed drop leaves an **unknown barrier**
  on the identity (see C.1b): the catalog's audit-time `present` is
  shadowed, not promoted, and `absent` is not claimed;
- a still-valid known fact is NOT cleared by "this statement may not
  have run" — e.g., an ordered-proven absent keeps its reject/no-op
  arms, whose consumption requires no new write.

`complete` cells appear only on refusal arms whose proof needs nothing
missing (R4 point 6) — no unconditional success exists under the
current fact set.

SERVER name identity basis (catalog read vs ordered channel — R2-B):

- The **catalog read** issues `WHERE server_name = ?` on the verbatim
  decoded identifier form — the catalog column's own documented
  case-insensitive collation does the compare (exact per-version
  collation stays the registered PEND, `r2-metadata-facts.md` §1 —
  G-src.3).
- The **objectCache request key** stays the verbatim decoded name —
  accepted in the provider round, unchanged.
- The **ordered channel** keys entries by `canonical(name)` = ASCII
  letter-case fold of the decoded name. Scoped basis (no universal
  collation claim): the documented rule for `server_name` is
  case-insensitive (R1 A.5 + R2 §1 — G-src.2/3), and within the
  documented CI collations for this column ASCII letter-case variants
  name the same object. Relations outside that sourced envelope —
  non-ASCII names, or names differing in more than ASCII letter case
  — are **relation-unproven**, not provably-distinct (C.1b).

Five-state existence ladder (R1-C.3, corrected bound shapes):

| state | bound shape on `Metadata.Server` | usable as |
|---|---|---|
| unbound | `Metadata.Server == nil` — bind never ran | existence unproven |
| bound-unvouched | non-nil `{Name, Read:false}` with **empty** `UnreadReason` — contaminated batch / no resolver | unproven |
| not-read | non-nil `Read=false` + `UnreadReason` (`identity_unproven` / `version_inapplicable`) | unproven |
| read-unknown | `Read=true` + `Exists=false` — query ran, zero rows, answer unknown | unproven — **`Read=true` is not existence-known** |
| proven-present | `Read=true` + `Exists=true` | `present` premise |
| ordered-absent | `OrderedAbsent=true` (same-adjudication channel only; currently unreachable — D.4) | `absent` premise |

## C.1 SERVER family — 16 units + ordered-channel contract C17–C22

| # | unit | form / trigger | premises consumed | availability | adjudication | diagnostics / coverage | modeled post-state |
|---|---|---|---|---|---|---|---|
| C1 | srv-identity-kind | any SERVER form | decl `Kind`/`Name`/`NameIdent` | proven | identity base | coverage base | — |
| C2 | srv-dialect-gate | TiDB any SERVER | dialect | proven | vendor boundary | `unsupported`+`boundary=vendor`+diag+incomplete | unchanged |
| C3 | srv-drop-present | `DROP SERVER n`, ident | decl `drop`/`NameIdent`; existence proven-present; **privilege unproven** | mixed | notice `[p] ddl.mysql.server.drop.notice` + gap `server.drop_privilege` | review / unverified | `servers[canonical]=unproven-barrier` — shadows catalog present; no absent published (premise-dependent, D.4) |
| C4 | srv-drop-unknown | same, existence unproven | same + existence unproven | unknown+unproven | notice + gaps `server.existence`,`server.drop_privilege` | review / unverified | unproven-barrier |
| C5 | srv-drop-ie-unknown | `DROP SERVER IF EXISTS n`, existence unproven | `IfExists`+ | unknown+unproven | notice + gaps `server.existence`,`server.drop_privilege` | review / unverified | unproven-barrier — the present-arm may not have executed, so no convergent absent (C.4) |
| C6 | srv-drop-unbound | drop, string-literal name | decl `NameIdent=false` | unbound | `unaudited` boundary (ruling G-1) | `unsupported`+`unaudited`+incomplete | unchanged + channel taint (unprovable write target, C20) |
| C7 | srv-drop-absent | drop on ordered-proven-absent | ordered `OrderedAbsent` | ordered-only (currently unreachable — premise-dependent) | **two arms**: (a) plain DROP → reject `[p] drop.absent` — refusal proven by absent alone, no privilege needed; (b) `IF EXISTS` → guard no-op + gap `server.drop_privilege` | (a) reject / complete; (b) review / unverified | (a) absent stands; (b) absent stands — a no-op/refusal can't recreate |
| C8 | srv-create-dup | `CREATE SERVER n`, present | decl `create`; present | proven (premise set — slice excludes create) | reject `[p] ddl.mysql.server.create.dup` — design row, not in slice | reject / complete | unchanged |
| C9 | srv-create-unknown | create, unproven | same | unknown | review + gap | unverified | unchanged |
| C10 | srv-create-absent | create on proven-absent | absent premise | **catalog-unreachable** (zero-row=unknown); ordered-absent *would be* reachable from an own-batch proven drop — but no absent publishes under current capability, and create is outside this slice | registered, not dropped | — | — |
| C11 | srv-alter-present | `ALTER SERVER n`, present | decl `alter`; present | proven (slice excludes alter) | notice `[p] alter.notice` — design row | review / unverified (`server.alter_privilege` premise same shape) | `present` + opaque revision marker — design row only |
| C12 | srv-alter-unknown | alter, unproven | same | unknown | review + gap | unverified | unchanged |
| C13 | srv-alter-absent | alter on ordered-absent | ordered `OrderedAbsent` | ordered-only (unreachable) | reject `[p] alter.absent` | reject / complete | unchanged |
| C14 | srv-options-static | dup keys; PORT; truncation | decl facts | basis unapproved [PEND] | proposal only | — | unchanged |
| C15 | srv-name-truncation | decoded >64 | decoded length | no-capability [PEND] | deferred | — | unchanged |
| C16 | srv-connection-semantics | HOST/USER/PASSWORD payloads | — | no-capability by design | named aspect — **ruled inapplicable to DROP** (removed from DROP premises); stays a CREATE/ALTER-side proposal, unapproved | named aspect record | unchanged |

### C.1b Ordered-channel contract (R2-B unified algorithm)

`resolveServer(name)` evaluates in one fixed order — "an entry exists"
never short-circuits the taint checks:

1. `s.contaminated` (whole-batch) → bound-unvouched, no reads.
2. `serverChannelTainted` → bound-unvouched (channel overlay lost).
3. Entry check by relation (`R(W, L)` — write/tracked identity vs the
   looked-up name, symmetric): an entry or prior write
   **provably-same** as the lookup → consume it; **provably-distinct**
   → ignore; **unproven** → the lookup's overlay is unproven →
   unproven-barrier outcome (no promotion).
4. A valid entry (`orderedAbsent` — proven, uninvalidated) wins over
   the catalog.
5. Else the provider read fires via the verbatim objectCache seam —
   once per verbatim name — and the answer binds (present or
   read-unknown).
6. A lookup with a prior **unproven-relation write** gets
   `server.batch_effects` — the stale catalog value may be
   superseded by an undischarged modeled write, so it is shadowed,
   not returned as a clean answer.

Relation classification `R(A,B)` (sourced, conservative):

| relation | condition | handling |
|---|---|---|
| provably-same | byte-identical, or ASCII-envelope letter-case variants under the documented CI rule | same object — use/invalidate the matching entry |
| provably-distinct | ASCII-envelope names differing in something other than letter case | different objects — unaffected |
| unproven | anything else (non-ASCII, non-letter-case differences like trailing space) | neither used nor ignored — produces an unproven barrier |

Writes leave records regardless of prior entries (R2-B.5): every
unaudited SERVER write appends its identity to the channel's write
list; a later lookup with an unproven relation to ANY recorded write
gets the barrier — "no prior entry" is not proof of no prior effect.
`serverChannelTainted` is set by an unprovable-identity write
(string-literal name): every subsequent SERVER statement fails step 3.

Entry provenance & immutability (R2-B.6): the ordered projection never
mutates the cached provider snapshot or prior statements' metadata —
entries are channel-owned views. `OrderedAbsent` marks only a valid,
proven, uninvalidated ordered derivation; a tainted or barred identity
carries no `OrderedAbsent`.

| # | unit | contract |
|---|---|---|
| C17 | srv-ordered-key | `serverEntries` keyed by `canonical(name)` (ASCII case-fold, scoped basis above); objectCache key verbatim; `NameIdent=false` names have no canonical identity |
| C18 | srv-ordered-algorithm | the six-step `resolveServer` order above |
| C19 | srv-ordered-publish | a *proven* drop publishes `servers[canonical]=absent`+`OrderedAbsent` — **currently unreachable** (privilege premise unwired); an unproven-outcome drop publishes the unproven barrier instead |
| C20 | srv-ordered-invalidate | unaudited SERVER write → per-entry relation: same → mark unproven-barrier; unproven → mark unproven-barrier; distinct → untouched; unprovable identity → channel taint; every write also lands in the write list |
| C21 | srv-ordered-contamination | `s.contaminated` suppresses the channel; channel taint never spreads to table/schema state |
| C22 | srv-ordered-unknown-barrier | an unproven-outcome drop on a provably-same identity converts its entry to the unproven barrier (catalog present may be stale — the statement may have deleted the object) |

## C.2 SRS family — the 33-row table, corrected (R2-C)

**Statement-level merge contract** (global, new per R2-C.3):

- each row is a single-check judgment — the statement verdict /
  coverage merges separately;
- a terminal refusal with proven applicability and no PENDING
  dependency may stop demanding unrelated premises;
- a warning or a single compliance arm cannot alone yield
  statement-level `complete`;
- an uncovered promised capability → named implementation boundary
  (never a bounded gap — the capability was never claimed);
- a missing external fact on a supported check → bounded gap;
- PENDING-dependent results stay undecided; a shared post-state
  requires independent per-branch derivation.

| row | unit | premises | avail | single-check verdict (proposal) | diagnostic / coverage | post-state |
|---|---|---|---|---|---|---|
| 1 | srs-parse-invalid | grammar | proven | parser_error | parser_error / — | unchanged |
| 2 | srs-dup-attr | `DuplicateAttrs` | proven | `[p] srs.attr.duplicate` reject | reject / per-check complete | unchanged |
| 3 | srs-missing-attr | `HasName`/`HasDefinition`=false | proven | `[p] srs.attr.missing` reject — absent attrs are preserved facts (empty attr list still recognized); only missing attr *values* fall back unclaimed | reject / per-check complete | unchanged |
| 4 | srs-len-violate | byte len of unquoted spelling | partial | compliance arm: `byte_len ≤ cap` → chars≤bytes≤cap → compliant (per-check); **violation arm → `no-capability`**: escapes shorten the decoded form (e.g., `\\\\`), so even single-byte spelling can't prove the decoded length; registered until a decode measure exists — no decode/SQL-mode/payload expansion this round | per-check: pass / complete, else boundary+gap | unchanged |
| 5 | srs-newline-def | `DefHasNewline` | positive-witness | `true` → `[p] srs.def.newline` reject; `false` → unproven-clean | reject / complete on true; review+gap `srs.def_newline_unproven` / unverified on false | unchanged |
| 6 | srs-srid0 | `SRID==0` | proven | `[p] srs.srid0` reject | reject / complete | unchanged |
| 7 | srs-srid-reserved | reserved table | proven | warning `[p] srs.reserved_range.warn` — **single check; cannot alone yield statement complete** | review / per-check | unchanged |
| 8 | srs-org-pair | bare ORGANIZATION | grammar-unreachable | unclaimed → base grammar diagnostic | parser_error / — | unchanged |
| 9 | srs-def-malformed | WKT | no-capability | named implementation boundary | boundary / incomplete | unchanged |
| 10 | srs-def-axis | WKT+version≥8.0.13 | no-capability | named implementation boundary | boundary / incomplete | unchanged |
| 11 | srs-def-unknown-method | WKT method set | no-capability + **PENDING ORACLE** | undecided | boundary / — | unchanged |
| 12 | srs-version-57 | proven <8.0 | proven when loaded | vendor-ish boundary | boundary / incomplete | unchanged |
| 13 | srs-version-unknown | version unproven | unknown | **per-check proposal** — corrected provider contract: proven-pre-8.0 → dedicated reads skipped + `version_inapplicable` unread; version unproven → reads STILL ISSUED, issued-query failures propagate; proven-8.0+ → normal reads. Declaration-fact checks are computable on any version; native SRS semantic *applicability* per check stays version-scoped | per-check / unverified where skipped | unchanged |
| 14 | srs-tidb | dialect | proven | vendor boundary | boundary / incomplete | unchanged |
| 15 | srs-create | `srs.existence` | wired; absent catalog-unreachable | per-arm rows 16–17 | per-arm | `srs[srid]=present` only on a reachable proven create — none exists |
| 16 | srs-create-dup | present | proven | `[p] srs.create.dup` reject | reject / complete | unchanged |
| 17 | srs-create-unknown | unproven | unknown | review + gap `srs.existence` | review / unverified | unchanged |
| 18 | srs-ine-absent | INE + absent | unreachable both channels (no SRS ordered channel; create can't supply absent) | registered unreachable | — | — |
| 19 | srs-ine-present | INE + present | proven | proposal — **stays undecided while row 20 ordering is PENDING**; a no-op/complete claim is not derivable yet | undecided / — | unchanged |
| 20 | srs-ine-pend | INE ordering | **PENDING ORACLE** | undecided | — | — |
| 21 | srs-replace-absent | OR REPLACE + absent | PENDING + catalog-unreachable | undecided | — | — |
| 22 | srs-replace-undep | present + deps proven-empty | proven-empty unreachable | registered unreachable | — | — |
| 23 | srs-replace-dep | present + deps present | proven | `[p] srs.replace.dep` reject | reject / complete | unchanged |
| 24 | srs-replace-depunknown | deps unproven | unknown | review + gap `srs.column_dependencies` | review / unverified | unchanged |
| 25 | srs-drop-ok | present + deps proven-empty | proven-empty unreachable | three arms: (a) proven-empty → success (unreachable); (b) deps present → refusal (3716-class — a positive dep witness never supports drop-ok or absent post); (c) deps unproven → review+gap; no independent warning ships this row | per-arm | unchanged |
| 26 | srs-drop-dep | present + deps present | proven | `[p] srs.drop.dep` reject | reject / complete | unchanged |
| 27 | srs-drop-depunknown | deps unproven | unknown | review + gap `srs.column_dependencies` | review / unverified | unchanged |
| 28 | srs-drop-absent | proven-absent | unreachable (no SRS ordered channel) | registered unreachable | — | — |
| 29 | srs-drop-ie-absent | DROP IF EXISTS + absent | unreachable | registered unreachable | — | — |
| 30 | srs-drop-unknown | unproven | unknown | review + gap `srs.existence` | review / unverified | unchanged |
| 31 | srs-name-conflict | `srs_name` qualifier | provider-capable, statement path unwired | **named implementation boundary** (uncovered capability ≠ missing external fact; ruling G-2 keeps provider-only) | boundary / incomplete | unchanged |
| 32 | srs-org-conflict | (org,id) pair | provider-capable, unwired | named implementation boundary | boundary / incomplete | unchanged |
| 33 | srs-batch-order | ordered `srs[srid]` | no-capability | deferred family work | boundary | unchanged |

Priority note: no priority column is carried — check precedence is an
implementation detail, not the pending INE ordering oracle (row 20).

## C.3 ALTER INSTANCE — registered boundary, remaining deps only

| unit | status | remaining dependency |
|---|---|---|
| `alter_instance.rotate_innodb_master_key` | shipped named boundary (MySQL-only; TiDB vendor) | native oracle only |
| `alter_instance.reload_tls` (+ `no_rollback_on_error` member) | shipped precise feature + bounded member metadata — T26-owned (G-8) | same |
| generic `alter_instance` + surplus E1–E5 | sibling-task registry (DDL-ALTER-INSTANCE-EXT) minus T26-owned members | instance-premise capability + per-form ownership |

## C.4 Guard-clause contract — complete branch derivation (R2-A.4)

For `DROP SERVER IF EXISTS n` with existence unproven, every necessary
arm:

| modeled arm | transition | modeled post-state |
|---|---|---|
| n present, drop executes | delete | absent |
| n present, drop fails execution premise (SUPER unproven) | refused at runtime | **present — the convergent absent is broken here** |
| n absent | guard no-op (or guard-time refusal — either way) | absent |

Since the privilege premise is unproven, arm 2 keeps `present` possible
→ the merge does NOT converge → post-state = unproven barrier +
gaps `server.existence` + `server.drop_privilege` → unverified. Had the
privilege premise been proven, arms 1+3 converge to absent (the
`applyCreateSchema` `if_not_exists` convergent-arm precedent, G-src.6)
— registered as the future-premise path, not consumable today.

`IF NOT EXISTS` on CREATE keeps the same shape and stays registered for
the create slice.
