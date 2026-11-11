# T26-SEMANTICS-01 §C — consolidated semantic decision table

Availability vocabulary (per premise, per the provider-round contract):

- `proven` — the statement path obtains the fact today (declaration
  field or wired provider read).
- `unknown` — obtainable in principle but the current answer stays
  unproven (e.g., zero-row catalog read).
- `unbound` — the statement path binds nothing (provider absent,
  contaminated batch, identity unprovable, fact not requested).
- `no-capability` — neither the declaration nor any accepted fact can
  supply it (WKT grammar, decoded-length measure, full option values).

Adjudication outcomes use the existing verdict vocabulary:
`reject` (blocker), `review` (warning/notice + findings), `gap` (bounded
EvidenceGap), `unverified` coverage, `complete` coverage (per the R4
seven-point contract), `boundary` (named unaudited/vendor), and ordered
post-state `servers[name]` / `srs[srid]` ∈ {present, absent, unchanged}.
`unchanged` never fabricates; an unknown affected fact propagates
unknown rather than "exact pre-state preserved" (R4 addendum point 7).

SERVER name comparison basis: the existence read issues `WHERE
server_name = ?` with the verbatim decoded name — matching is the
catalog column's own collation (documented case-insensitive;
exact collation per version stays a registered PEND in
`r2-metadata-facts.md` §1). Cache-key comparison is internal plumbing
and is **not** a database identity rule — registered, not reused.

## C.1 SERVER family — 16 units

| # | unit | form / trigger | basis + status | premises (decl / provider) | availability today | adjudication | diagnostics / coverage | post-state |
|---|---|---|---|---|---|---|---|---|
| C1 | srv-identity-kind | any SERVER form | R2-S1, shipped | `decl.Kind`, `decl.Name` | proven | identity base — not a verdict | coverage base | — |
| C2 | srv-dialect-gate | TiDB any SERVER | R2-S2 + R3, shipped | dialect | proven | vendor boundary | `unsupported` + `boundary=vendor` + `unsupported_statement` diag + incomplete | unchanged |
| C3 | srv-drop-present | `DROP SERVER [IF EXISTS] n`, ident name | R1-A.5 + R2-S6 (proposal→first slice) | `decl.Kind=drop`, `decl.NameIdent`, `server.existence` | proven (wired) | allow → notice `[proposed] ddl.mysql.server.drop.notice` | review / complete | `servers[name]=absent` |
| C4 | srv-drop-unknown | same, existence answer unproven | R2-S6 + T04 gap contract | same | unknown | review + gap `server.existence` | review / unverified | unchanged |
| C5 | srv-drop-ie-unknown | `DROP SERVER IF EXISTS n`, existence unproven | R4.7 unknown-propagation | `decl.IfExists`, `server.existence` | unknown | review + gap `server.existence`; guard is not proof of prior absence | review / unverified | unchanged (unknown propagates — never written absent on the affected key) |
| C6 | srv-drop-unbound | drop, string-literal name | R2-S1/identity contract + provider round | `decl.NameIdent=false` | unbound (read never issued) | named boundary kept — propose keeping `unaudited` for identity-unproven drops (Option A; see G-1) | `unsupported` + `unaudited` + incomplete | unchanged |
| C7 | srv-drop-absent-reject | drop on proven-absent | R1-A.5 | proven-absent premise | **unreachable via catalog** (zero-row = unknown; no completeness contract); reachable only via ordered channel (own-batch proven drop) — see C18 | reject `[proposed] ddl.mysql.server.drop.absent` when ordered channel proves absent | reject / complete | unchanged |
| C8 | srv-create-dup | `CREATE SERVER n ...`, existence=present | R2-S6 | `decl.Kind=create`, `server.existence` | proven (wired) | reject `[proposed] ddl.mysql.server.create.dup` | reject / complete | unchanged |
| C9 | srv-create-unknown | create, existence unproven | R2-S6 + T04 | same | unknown | review + gap `server.existence` | review / unverified | unchanged |
| C10 | srv-create-absent-allow | create on proven-absent | R1-A.5 | proven-absent premise | **unreachable via catalog** — same zero-row contract; no proven-allow arm for create this round | n/a — registered unreachable, not dropped silently | — | — |
| C11 | srv-alter-present | `ALTER SERVER n OPTIONS(...)`, present | R2-S7 | `decl.Kind=alter`, `server.existence` | proven (wired) | notice `[proposed] ddl.mysql.server.alter.notice` | review / complete | `servers[name]=present` + opaque options-revision marker (never values) |
| C12 | srv-alter-unknown | alter, existence unproven | R2-S6 | same | unknown | review + gap `server.existence` | review / unverified | unchanged |
| C13 | srv-alter-absent-reject | alter on proven-absent | R1-A.5 | proven-absent | unreachable via catalog (same); ordered-channel reachable | reject `[proposed] ddl.mysql.server.alter.absent` | reject / complete | unchanged |
| C14 | srv-options-static | duplicate OPTION keys; PORT range; char-literal truncation | R2-S4/S5 — **[PEND] oracle** (dup last-wins vs error; range/truncation adjudication unapproved) | `decl.Options`, `decl.DuplicateKeys`, spelling counts | decl facts proven; **adjudication basis unapproved** | proposal only — no verdict this round | — | unchanged |
| C15 | srv-name-truncation | decoded name >64 silent truncation | R2-S5 — needs full decode | decoded length | **no-capability** for literal spellings (`NameSpellingRuneCount` is lexical only); ident-form names are decoded but the truncation-vs-error basis is unverified [PEND] | deferred — `EffectiveName` derivation not implementable yet | — | unchanged |
| C16 | srv-connection-semantics | HOST/USER/PASSWORD/etc. payload correctness | R2-S3/S7 non-goal | — | no-capability (by design) | named unaudited aspect `[proposed] server.connection_semantics` — evidence record, never a finding | named aspect, not coverage loss | unchanged |

## C.2 SRS family — the 33-row R2 table mapped to current facts

Row identity preserved from `r2-srs-decision-table.md` §4. All rows
remain **unapproved design cells**; the R4 addendum's seven-point
`complete` contract is the effective completeness vocabulary. Column
"avail" marks the strictest premise per row.

| row | unit | key premises | avail today | status class | notes |
|---|---|---|---|---|---|
| 1 | srs-parse-invalid | grammar | proven | statics shipped | parser_error path unchanged |
| 2 | srs-dup-attr | `DuplicateAttrs` | proven | **facts sufficient** (static) | verdict basis still unapproved |
| 3 | srs-missing-attr | `Attrs.HasName/HasDefinition` | proven | **facts sufficient** | recognizer already requires NAME+DEFINITION shape; partial-pair semantics subsumed — merge candidate with grammar gate |
| 4 | srs-len-violate | byte-len fields | proven* | facts sufficient with caveat — lengths measured on unquoted spelling; decoded-length for escaped literals needs the registered decode step | DEFINITION ≤4096 byte len available |
| 5 | srs-newline-def | `DefHasNewline` | proven | **facts sufficient** | lexical fact on unquoted text |
| 6 | srs-srid0 | `decl.SRID==0` | proven | **facts sufficient** | recognizer rejects non-u32; =0 check static |
| 7 | srs-srid-reserved | `decl.SRID` + reserved table | proven (docs table implementable) | **facts sufficient** | warning-level proposal stands |
| 8 | srs-org-pair-incomplete | `HasOrganization/HasIdentifiedBy` | proven | **facts sufficient** — recognizer already refuses partial pairs (grammar); row's semantic arm unreachable → merge candidate | |
| 9 | srs-def-malformed | WKT parse | **no-capability** | needs WKT implementation | stays named implementation boundary scope |
| 10 | srs-def-axis | WKT + version≥8.0.13 | **no-capability** (WKT) | needs WKT + version premise | |
| 11 | srs-def-unknown-method | WKT method set | **no-capability** + PENDING ORACLE (refusal set) | needs WKT + oracle | stays uncertain |
| 12 | srs-version-57 | proven <8.0 | proven when provider loaded (`Metadata.Version`) | **facts sufficient** (premise class) | vendor-ish boundary per row |
| 13 | srs-version-unknown | version unproven | unknown | **RULING NEEDED** — suspended from R6; proposal: static-only checks run, version-dependent (axis) skipped with gap | |
| 14 | srs-tidb | dialect | proven | shipped boundary | |
| 15 | srs-create | `srs.existence=absent` needed for allow arm | existence **wired**; absent unreachable via catalog (zero-row) | allow arm unreachable; dup/unknown arms reachable | same zero-row contract as SERVER |
| 16 | srs-create-dup | existence=present | proven | **facts sufficient** | |
| 17 | srs-create-unknown | existence unproven | unknown | **facts sufficient** | |
| 18 | srs-ine-absent | INE + absent | absent unreachable via catalog | allow arm catalog-unreachable; ordered-channel reachable after own-batch create | |
| 19 | srs-ine-present | INE + present | proven | **facts sufficient** | ordering vs validation = row 20 PENDING |
| 20 | srs-ine-pend | INE ordering oracle | **PENDING ORACLE** | needs fixed-version source or authorized native | no deterministic post-state |
| 21 | srs-replace-absent | OR REPLACE + absent | **PENDING ORACLE** + absent catalog-unreachable | provisional only | |
| 22 | srs-replace-undep | present + deps proven-absent | deps=witness-set; proven-absent unreachable (Complete=false stays unproven) | allow arm unreachable this round | |
| 23 | srs-replace-dep | present + deps proven | proven (positive witness rows retained) | **facts sufficient** | |
| 24 | srs-replace-depunknown | deps unproven/empty | unknown | **facts sufficient** | |
| 25 | srs-drop-ok | present + deps proven-absent | dep proven-absent unreachable | warn arm partially reachable — see G-3 | |
| 26 | srs-drop-dep | present + deps proven | proven | **facts sufficient** | |
| 27 | srs-drop-depunknown | deps unproven | unknown | **facts sufficient** | |
| 28 | srs-drop-absent | proven-absent | catalog-unreachable; ordered-channel reachable | reject arm ordered-only | |
| 29 | srs-drop-ie-absent | DROP IF EXISTS + absent | catalog-unreachable | ordered-channel reachable | |
| 30 | srs-drop-unknown | existence unproven | unknown | **facts sufficient** | |
| 31 | srs-name-conflict | `srs.name_org_conflict` (NAME arm) | **provider-only** — statement path carries no `srs_name` qualifier; authored NAME is a string-literal spelling | needs trusted NAME identity + wiring ruling (G-2) | |
| 32 | srs-org-conflict | (org,id) pair read | **provider-only** — OrgID numeric provable, OrgName spelling; pair qualifier unwired | needs trusted ORG identity + wiring ruling (G-2) | |
| 33 | srs-batch-order | ordered `srs[srid]` channel | **no-capability yet** — no SRS registry channel exists | first-slice establishes the pattern for SERVER; SRS channel is later work | |

SRS summary against the four provider capabilities: `srs.existence`
supports rows 15–19/21/28–30; `srs.column_dependencies` supports rows
22–27; `srs_name`/`srs_org` conflict reads are provider-capable but
statement-path unwired (rows 31–32, G-2); WKT rows 9–11 and the ordered
channel row 33 stay no-capability/PENDING.

## C.3 ALTER INSTANCE — registered boundary, remaining deps only

| unit | status | remaining dependency |
|---|---|---|
| `alter_instance.rotate_innodb_master_key` | shipped named boundary (MySQL-only feature; TiDB vendor) | native oracle only — no semantics expanded this order |
| `alter_instance.reload_tls` (+ `no_rollback_on_error` member) | shipped precise feature + bounded member metadata | same |
| generic `alter_instance` + surplus E1–E5 (REDO_LOG, BINLOG key, KEYRING, FOR CHANNEL, NRB modifier) | sibling-task registry (DDL-ALTER-INSTANCE-EXT) | instance-premise capability (keyring/component) + per-form ownership assignment — registered deps only, not expanded |

## C.4 Guard-clause contract (per work order)

`IF EXISTS` / `IF NOT EXISTS` are never premises of prior existence.
Common conclusions across guard branches are listed per-branch above
(C4/C5, rows 18–19/29–30). A guard on an unproven premise yields the
bounded gap of that premise plus the unknown-propagation post-state —
never a rewritten success or refusal.
