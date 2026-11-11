# T26-SEMANTICS-01 §C — consolidated semantic decision table (R1 revision)

Availability vocabulary (per premise):

- `proven` — the statement path obtains the fact today (declaration
  field or wired provider read).
- `unknown` — obtainable in principle but the answer stays unproven.
- `unbound` — the statement path binds nothing (provider absent,
  contaminated channel, identity unprovable, fact not requested).
- `no-capability` — neither the declaration nor any accepted fact can
  supply it (WKT grammar, decoded-length measure, full option values).
- `ordered` — supplied only by the same-adjudication ordered channel
  (G-src.4), never by the catalog.

Premise itemization per success row (R1-A): every success conclusion
lists its **declaration facts**, its **provider/ordered facts**, its
**named exclusions**, and what happens when a premise is unproven.
Two named exclusions apply to every SERVER lifecycle row and are
registered per-row rather than assumed:

| exclusion | basis | status | consequence if the registration is rejected |
|---|---|---|---|
| `server.connection_semantics` — runtime correctness of HOST/USER/ PASSWORD/etc. payloads | R1 A.4 registered the audited row as "identity + findings + ordered state for the definition row" with connection semantics a named unaudited aspect (`[proposed]` evidence record, not a finding) — G-src.2 | proposal pending approval | the aspect becomes an R4-point-2 named implementation boundary → `coverage=incomplete` on every SERVER row |
| `server.execution_premises` — runtime execution environment (e.g., the documented SUPER requirement for DROP SERVER), engine/network reachability | no fact supplies it and no earlier contract explicitly excluded runtime privileges; the precedent is only implicit (every existing family adjudicates statement-level lifecycle semantics, never runtime grants) | **registered as a gap candidate — PENDING RULING** | every success arm emits bounded gap `server.execution_premises` → `coverage=unverified`; no unconditional success is claimed anywhere in this table meanwhile |

Until the `server.execution_premises` disposition lands, **no row in
this table emits an unconditional success**. Every `complete` cell below
is written *conditional on the registered exclusions*; the modeled
post-state is "the statement executes as adjudicated", matching the
existing family's scoped complete (R4 point 1 scopes complete to the
promised surface — G-src.1).

SERVER name identity basis (catalog read vs ordered channel — R1-B):

- The **catalog read** issues `WHERE server_name = ?` on the verbatim
  decoded identifier form — matching is the catalog column's own
  collation (documented case-insensitive; exact per-version collation
  stays the registered PEND in `r2-metadata-facts.md` §1 — G-src.3).
- The **objectCache request key** stays the verbatim decoded name —
  accepted in the provider round, unchanged.
- The **ordered channel** keys entries by canonical identity:
  ASCII case-fold of the decoded name. Basis: R1 A.5 registered
  `state.servers` keyed case-insensitive (G-src.2); R2 §1 registered
  the documented case-insensitive identity rule (G-src.3); ASCII fold
  is the provable kernel of any case-insensitive collation. Names
  outside the ASCII-fold envelope can't be proven distinct from a
  prior write under an unproven collation — the bounded
  `server.batch_effects` gap covers that residual (C18).

Five-state existence ladder (R1-C.3) — `server.existence` resolves to
exactly one of:

| state | bound shape | usable as |
|---|---|---|
| unbound | no `ServerState` bound (provider absent, contaminated channel) | `server.existence` unproven → gap |
| not-read | `Read=false` + `UnreadReason` (identity_unproven / version inapplicable) | unproven → gap |
| read-unknown | `Read=true` + `Exists=false` (zero rows ran, answer unknown) | unproven → gap — **`Read=true` is not existence-known** |
| proven-present | `Read=true` + `Exists=true` | `present` premise |
| ordered-absent | `OrderedAbsent=true` (same-adjudication channel only) | `absent` premise — catalog can never supply this |

## C.1 SERVER family — 16 units + ordered-channel contract C17–C22

| # | unit | form / trigger | premises consumed (decl / provider-ordered / exclusions) | availability | adjudication | diagnostics / coverage | post-state |
|---|---|---|---|---|---|---|---|
| C1 | srv-identity-kind | any SERVER form | decl: `Kind`, `Name`, `NameIdent` | proven | identity base — not a verdict | coverage base | — |
| C2 | srv-dialect-gate | TiDB any SERVER | dialect | proven | vendor boundary | `unsupported` + `boundary=vendor` + diag + incomplete | unchanged |
| C3 | srv-drop-present | `DROP SERVER n`, ident name | decl: `Kind=drop`, `NameIdent`; provider: proven-present; exclusions: C16 + `server.execution_premises` (PENDING) | proven | notice `[proposed] ddl.mysql.server.drop.notice` | review / complete-conditional (see exclusion table) | `servers[canonical(name)]=absent` (modeled) |
| C4 | srv-drop-unknown | same, existence unproven (any unproven ladder state) | decl: `Kind=drop`, `NameIdent`; provider: unproven | unknown | review + bounded gap `server.existence` | review / unverified | unchanged — unknown propagates |
| C5 | srv-drop-ie-unknown | `DROP SERVER IF EXISTS n`, existence unproven | decl: `Kind=drop`, `IfExists`, `NameIdent`; provider: unproven | unknown | review + bounded gap `server.existence` | review / unverified | **conditional `absent`** — derived per arm in C.4: present→delete→absent; absent→no-op→absent; both modeled arms converge, so the modeled post-state is absent *conditional on the statement executing under the registered exclusions*; the existence premise itself stays an honest gap |
| C6 | srv-drop-unbound | drop, string-literal name | decl: `NameIdent=false` | unbound | named boundary kept — `unaudited` (ruling G-1: accepted) | `unsupported` + `unaudited` + incomplete | unchanged — and the literal name is an unprovable write: it taints the ordered channel (C20) |
| C7 | srv-drop-absent-reject | drop on ordered-proven-absent | ordered: `OrderedAbsent` (C18/C19) | ordered-only — catalog zero-row is unknown, never absent (G-src.3) | **two arms — no unified reject**: (a) plain `DROP SERVER` → reject `[proposed] ddl.mysql.server.drop.absent`; (b) `DROP SERVER IF EXISTS` → guard no-op → notice-level success | (a) reject / complete-conditional; (b) review / complete-conditional | (a) unchanged — proven refusal keeps terminal pre-state (R4 point 6); (b) unchanged — already absent |
| C8 | srv-create-dup | `CREATE SERVER n`, proven-present | decl: `Kind=create`; provider: present | proven | reject `[proposed] ddl.mysql.server.create.dup` | reject / complete-conditional | unchanged |
| C9 | srv-create-unknown | create, existence unproven | same | unknown | review + gap `server.existence` | review / unverified | unchanged |
| C10 | srv-create-absent-allow | create on proven-absent | proven-absent | unreachable (no completeness contract; no create arm publishes absent) | n/a — registered unreachable | — | — |
| C11 | srv-alter-present | `ALTER SERVER n OPTIONS(...)`, present | decl: `Kind=alter`; provider: present | proven | notice `[proposed] ddl.mysql.server.alter.notice` | review / complete-conditional | `servers[canonical]=present` + opaque options-revision marker |
| C12 | srv-alter-unknown | alter, existence unproven | same | unknown | review + gap `server.existence` | review / unverified | unchanged |
| C13 | srv-alter-absent-reject | alter on ordered-proven-absent | ordered `OrderedAbsent` | ordered-only | reject `[proposed] ddl.mysql.server.alter.absent` | reject / complete-conditional | unchanged |
| C14 | srv-options-static | dup OPTION keys; PORT range; truncation | decl: `Options`, `DuplicateKeys`, spelling counts | decl proven; **adjudication basis unapproved [PEND]** | proposal only | — | unchanged |
| C15 | srv-name-truncation | decoded name >64 silent truncation | decoded length | **no-capability** for literal spellings; ident-form names decoded but basis unverified [PEND] | deferred — no `EffectiveName` derivation | — | unchanged |
| C16 | srv-connection-semantics | HOST/USER/PASSWORD payload correctness | — | no-capability by design | named unaudited aspect — evidence record, never a finding | **coverage-neutral only inside the registered surface** — R1 A.4 scoped the audited row to "identity + findings + ordered state" (G-src.2); outside that scope the aspect is a named exclusion, not an unchecked check. Basis is R1 A.4 only — not a general exemption; pending approval like `server.execution_premises` | unchanged |

### C.1b Ordered-channel contract units (new this revision — R1-B)

| # | unit | contract |
|---|---|---|
| C17 | srv-ordered-key | `serverEntries` keyed by `canonical(name)` = ASCII case-fold of the verbatim decoded identifier; objectCache request key stays verbatim (never folded); `decl.NameIdent=false` names have no canonical identity |
| C18 | srv-ordered-overlay | `resolveServer(name)`: (a) canonical entry exists → batch-proven state wins over the catalog; (b) `serverChannelTainted` or entry `tainted` → bound unvouched state — **no catalog read is promoted**; (c) name outside the ASCII-fold envelope AND any prior in-batch SERVER write exists whose canonical key is not byte-identical → bounded gap `server.batch_effects` (alias can't be discharged under the PEND collation); (d) else → provider read via the verbatim objectCache seam, once per verbatim name |
| C19 | srv-ordered-publish | only same-adjudication outcomes publish: proven drop → `servers[canonical]=absent` + `OrderedAbsent=true`; rejected/no-op arms keep the entry |
| C20 | srv-ordered-invalidate | a recognized-but-unaudited SERVER write (CREATE/ALTER boundary forms, literal-name DROP) never publishes but **taints**: provably-same canonical key → that entry marked `tainted` (shadows catalog — never deleted back to a clean read); identity unprovable (literal name) → `serverChannelTainted` — every subsequent SERVER statement's overlay is unproven; provably-distinct keys → unaffected |
| C21 | srv-ordered-contamination | `s.contaminated` (whole-batch) suppresses the channel entirely — same as today's `bindMySQLServer` unvouched path; channel taint is family-scoped and never touches table/schema tracked state |
| C22 | srv-ordered-read | a tainted entry or channel yields the unvouched ladder state (existence unproven → `server.batch_effects` gap on the consuming row), not a fresh catalog read — the audit-time snapshot was already superseded by an unproven modeled write |

`OrderedAbsent` representation (R1-C.4, G-4 pending field design):
`OrderedAbsent=true` ⇒ `Exists=false`, `Status` ≠ `not_found` (that
vocabulary stays catalog-only), `Read` never set by the ordered channel
(no read happened — the channel must not fabricate one);
mutually exclusive with `Exists=true`. Rule and `apply` consume one
adjudicated existence: `OrderedAbsent` → absent; else `Read && Exists`
→ present; else unknown. Provenance validity: an entry is valid until
invalidated (C20) or the channel taints (C21).

## C.2 SRS family — the 33-row R2 table, corrected (R1-D)

Row identity preserved from `r2-srs-decision-table.md`. All rows remain
**unapproved design cells**; PENDING cells stay PENDING. New columns:
proposed verdict / priority / diagnostic-coverage / post-state.

| row | unit | premises | avail | proposed verdict | priority | diagnostic / coverage | post-state |
|---|---|---|---|---|---|---|---|
| 1 | srs-parse-invalid | grammar | proven | parser_error diag (existing path) | 0 | parser_error / incomplete | unchanged |
| 2 | srs-dup-attr | `DuplicateAttrs` | proven | `[p] srs.attr.duplicate` blocker | 1 | reject / complete | unchanged |
| 3 | srs-missing-attr | `HasName`/`HasDefinition`=false | proven | `[p] srs.attr.missing` blocker — **row stands**: recognizer does NOT reject absent attributes (empty attr list → all `Has*` false, statement still recognized); only a missing attr *value* falls back unclaimed | 1 | reject / complete | unchanged |
| 4 | srs-len-violate | `DefinitionByteLen` etc. — **byte length of unquoted spelling** | proven for **compliance** arm only | (a) `byte_len ≤ cap` → compliant proven; (b) `byte_len > cap` AND content provably single-byte → violation; (c) else → bounded gap `srs.attr_length_unproven` — official caps are in *characters*, bytes ≥ chars only proves the compliant arm | 1 | per-arm: reject / complete, or review+gap / unverified | unchanged |
| 5 | srs-newline-def | `DefHasNewline` — literal CR/LF in unquoted spelling | proven **as positive witness only** | `true` → `[p] srs.def.newline` violation proven; `false` → **not** proven clean (escaped `\n`/`\r` decode to newlines — capability gap registered) | 1 | reject / complete on true; review+gap `srs.def_newline_unproven` / unverified on false | unchanged |
| 6 | srs-srid0 | `SRID==0` | proven | `[p] srs.srid0` blocker | 1 | reject / complete | unchanged |
| 7 | srs-srid-reserved | `SRID` + reserved table | proven | warning `[p] srs.reserved_range.warn` | 1 | review / complete | unchanged |
| 8 | srs-org-pair-incomplete | `HasOrganization` w/o `HasIdentifiedBy` | grammar-unreachable | subsumed — bare-ORG statements are **unclaimed** (recognizer `return false` → base grammar diagnostic), not recognizer-rejected; semantic arm unreachable | 1 | parser_error / incomplete | unchanged |
| 9 | srs-def-malformed | WKT parse | no-capability | named implementation boundary | 2 | boundary / incomplete | unchanged |
| 10 | srs-def-axis | WKT + version≥8.0.13 | no-capability | named implementation boundary | 2 | boundary / incomplete | unchanged |
| 11 | srs-def-unknown-method | WKT method set | no-capability + **PENDING ORACLE** | PENDING — no deterministic result | — | boundary / incomplete | unchanged |
| 12 | srs-version-57 | proven <8.0 | proven when provider loaded | vendor-ish boundary per row | 0 | boundary / incomplete | unchanged |
| 13 | srs-version-unknown | version unproven | unknown | **proposal per-check, not blanket static-only**: each static check states its own version applicability (SRID domain/reserved/attr-shape apply on any version; axis check already no-capability; existence/deps reads carry `version_inapplicable` unread state) — version-dependent checks skipped with named gap | — | per-check / unverified for skipped checks | unchanged |
| 14 | srs-tidb | dialect | proven | shipped vendor boundary | 0 | boundary / incomplete | unchanged |
| 15 | srs-create | `srs.existence` | wired; **absent arm catalog-unreachable** | dup arm (16) reject; unknown arm (17) review+gap; allow arm unreachable | 2 | per-arm | `srs[srid]=present` only on a reachable proven create — none exists this round |
| 16 | srs-create-dup | present | proven | reject `[p] srs.create.dup` | 2 | reject / complete | unchanged |
| 17 | srs-create-unknown | unproven | unknown | review + gap `srs.existence` | 2 | review / unverified | unchanged |
| 18 | srs-ine-absent | INE + absent | **unreachable both channels** — corrected: no SRS ordered channel exists (row 33); CREATE can never supply absent (G-src.4); absent arm registered unreachable pending a proven transition design | 2 | — | — |
| 19 | srs-ine-present | INE + present | proven | notice/no-op arm; ordering vs validation = row 20 PENDING | 2 | review / complete-conditional | unchanged |
| 20 | srs-ine-pend | INE ordering | **PENDING ORACLE** | no deterministic result or post-state | — | — | — |
| 21 | srs-replace-absent | OR REPLACE + absent | **PENDING ORACLE** + catalog-unreachable | provisional only | — | — | — |
| 22 | srs-replace-undep | present + deps proven-absent | proven-absent **unreachable** (witness-positive: `Complete=false` keeps empty sets unproven) | allow arm unreachable — registered, not relaxed | 2 | — | — |
| 23 | srs-replace-dep | present + deps present | proven | reject `[p] srs.replace.dep` | 2 | reject / complete | unchanged |
| 24 | srs-replace-depunknown | deps unproven | unknown | review + gap `srs.column_dependencies` | 2 | review / unverified | unchanged |
| 25 | srs-drop-ok | present + deps proven-empty | proven-empty unreachable | **split into three arms**: (a) deps proven-empty → success arm — currently unreachable (Complete=false); (b) deps present → refusal arm (native 3716 — a positive dep witness can never support drop-ok or an absent post-state); (c) deps unproven → review+gap. An independent warning that coexists with a refusal is undefined this round — if introduced later it must state whether it attaches beside the blocker or replaces it; no such warning ships from this row | 2 | per-arm | (a) `srs[srid]=absent` unreachable; (b) unchanged; (c) unchanged |
| 26 | srs-drop-dep | present + deps present | proven | reject `[p] srs.drop.dep` (3716-class refusal) | 2 | reject / complete | unchanged |
| 27 | srs-drop-depunknown | deps unproven | unknown | review + gap `srs.column_dependencies` | 2 | review / unverified | unchanged |
| 28 | srs-drop-absent | proven-absent | **unreachable both channels** (no SRS ordered channel) | reject arm ordered-only — registered unreachable | 2 | — | — |
| 29 | srs-drop-ie-absent | DROP IF EXISTS + absent | same | no-op arm ordered-only — registered unreachable | 2 | — | — |
| 30 | srs-drop-unknown | unproven | unknown | review + gap `srs.existence` | 2 | review / unverified | unchanged |
| 31 | srs-name-conflict | `srs_name` qualifier | provider-capable, **statement path unwired** (authored NAME is a string-literal spelling — ruling G-2: stays provider-only) | PENDING trusted-identity basis | 3 | gap until wired | unchanged |
| 32 | srs-org-conflict | (org,id) pair | provider-capable, unwired | PENDING same basis | 3 | gap until wired | unchanged |
| 33 | srs-batch-order | ordered `srs[srid]` | no-capability — no SRS ordered channel | deferred family work | 3 | boundary | unchanged |

## C.3 ALTER INSTANCE — registered boundary, remaining deps only

| unit | status | remaining dependency |
|---|---|---|
| `alter_instance.rotate_innodb_master_key` | shipped named boundary (MySQL-only; TiDB vendor) | native oracle only — not expanded this order |
| `alter_instance.reload_tls` (+ `no_rollback_on_error` member) | shipped precise feature + bounded member metadata — stays T26-owned (ruling G-8: not moved to sibling scope) | same |
| generic `alter_instance` + surplus E1–E5 (REDO_LOG, BINLOG key, KEYRING, FOR CHANNEL, NRB modifier) | sibling-task registry (DDL-ALTER-INSTANCE-EXT) — minus the T26-owned members above | instance-premise capability (keyring/component) + per-form ownership — registered deps only |

## C.4 Guard-clause contract — per-branch derivation (R1-C.2)

`IF EXISTS`/`IF NOT EXISTS` is never a premise of prior existence —
but the post-state is derived per branch, not asserted by slogan. For
`DROP SERVER IF EXISTS n` with existence unproven and the registered
exclusions in force:

| modeled arm | transition | modeled post-state |
|---|---|---|
| n present at execution | delete runs | `absent` |
| n absent at execution | guard no-op | `absent` |

Common conclusion: both modeled arms publish `absent` — the
convergent-arm precedent (`applyCreateSchema` `if_not_exists` arm,
G-src.6) permits publishing the converged post-state without proving
which arm ran. What stays unproven and reported: the existence premise
itself → bounded gap `server.existence` + `coverage=unverified`; the
execution success is scoped to the exclusions — if
`server.execution_premises` becomes a gap instead, the same row emits
that gap and the converged post-state is withheld (a possibly-unrun
statement leaves no provable post-state — R4 point 7).

The independent premises that could block the merge, explicitly:

1. execution-environment premise unproven (PENDING exclusion
   disposition) → post-state withheld;
2. channel tainted (`server.batch_effects` unproven — the identity may
   have been touched by an unproven write) → post-state withheld;
3. name identity unprovable (`NameIdent=false`) → boundary row C6, not
   this row.

`IF NOT EXISTS` on CREATE follows the same shape (present→no-op→present;
absent→create→present) — registered for the create slice, not this one.
