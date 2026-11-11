# T26-SERVER-ALTER-CONTRACT-01 — implementation acceptance matrix

Status: **PLAN / UNEXECUTED** — every row is a design-to-execute case
for the future ALTER execution order. Nothing below has been run;
no row asserts PASS. Existing tests are cited as static design basis
only. Row IDs are stable (`AM-<group>.<n>`); adjudication row IDs
(`R1`–`R17`) refer to the README's worked-row table.

Fixture vocabulary (shared with the accepted slices):

- **modes**: `observed` = `observed_account_new_session` privilege
  mode selected; `off` = mode never selected (premise unproven);
  `none` = no provider at all (no loader, no resolver).
- **premise**: `consumable` = positive SUPER witness in scope;
  `suppressed` = witness exists but a scope terminator preceded;
  `unproven` = no witness / no mode.
- **catalog stub**: `present` = one `mysql.servers` row for the
  verbatim name; `zero` = clean zero-row answer; `error` = resolver
  raises the injected sentinel.
- **counts**: L = privilege-loader calls (≤1/request), R =
  provider-facing resolver requests (per `ResolveObject` forward
  seam), S = catalog SQL queries (`mysql.servers` reads).
- `s`/`s1`/`s9` are fixture identities: `s1` catalog-`present`,
  `s9` catalog-`zero`, `s` per-case.

## Group 1 — single ALTER: present / ordered-absent / catalog-unknown

| id | SQL | fixture | pre-state | mode | arm → row | expected (exact) | observation points |
|---|---|---|---|---|---|---|---|
| AM-1.1 | `ALTER SERVER s1 OPTIONS (USER 'u')` | catalog present | none | observed, consumable | A3 → R9 | owed ∅; outcome `conditional_alter`; marker `conditional_audit` set (own premise); coverage complete; verdict contribution none; publish `OrderedPresent` unconditional (catalog source → flag false) | L=1 R=1 S=1; resolver args `{Type:mysql_server, Name:s1, name_form:identifier}` |
| AM-1.2 | same SQL | catalog present | none | off | A4 → R10 | owed exactly `{server.alter_privilege}`; outcome `unverified`; no marker; coverage unverified; verdict review; publish `OrderedPresent` unconditional | L=0 R=1 S=1 |
| AM-1.3 | `ALTER SERVER s OPTIONS (USER 'u')` after proven `DROP SERVER s` (own consumable premise) | any | OrderedAbsent on s, conditional flag true | observed, consumable | A2 → R3 | owed ∅; outcome `refused_absent`; blocker; marker set (inherited conditional absent); coverage complete; verdict reject; absent stands | L=1 R=0 S=0 — ordered answered |
| AM-1.4 | `ALTER SERVER s9 OPTIONS (USER 'u')` | catalog zero | none | observed, consumable | A4 → R12 | owed exactly `{server.existence}`; `unverified`; marker set (own consumable); coverage unverified; **no publish, no barrier, no entry** | L=1 R=1 S=1 |
| AM-1.5 | same SQL | catalog zero | none | off | A4 → R13 | owed `{server.existence, server.alter_privilege}`; `unverified`; no marker; no publish/barrier/entry | L=0 R=1 S=1 |
| AM-1.6 | same SQL | none (no provider) | none | observed requested, loader absent | A4 → R16 | owed `{existence, alter_privilege}`; `unverified`; no marker; none published | L=0 R=0 S=0 |

## Group 2 — own ALTER privilege: consumable / unproven / suppressed / inherited-marker-only

| id | SQL | fixture | premise | arm → row | expected | observation |
|---|---|---|---|---|---|---|
| AM-2.1 | `ALTER SERVER s1 OPTIONS (USER 'u')` | present | consumable | A3 → R9 | owed ∅; `conditional_alter`; marker via own premise | L=1 R=1 S=1 |
| AM-2.2 | same | present | unproven (mode off) | A4 → R10 | owed `{alter_privilege}` only; no marker; `unverified` | L=0 R=1 S=1 |
| AM-2.3 | `REVOKE …; ALTER SERVER s1 OPTIONS (USER 'u')` (scope terminator before ALTER) | present | suppressed | A4 → R10 | owed `{alter_privilege}` only — suppression is owed, never a denial; terminator emits no grant-revoked claim | L=1 (loader loads once at enrich entry, before the statement loop — the terminator suppresses only the projection) R=1 S=1 |
| AM-2.4 | `DROP SERVER s` (proven, conditional publish) → `ALTER SERVER s OPTIONS (USER 'u')` with own premise unproven | any | unproven | A2 → R3 | marker set **solely from inherited** conditional absent — proves inheritance without own premise; owed ∅; `refused_absent` | L=1 R=0 S=0 |
| AM-2.5 | `CREATE SERVER s` (ProvenCreate, conditional publish) → `ALTER SERVER s OPTIONS (USER 'u')` with own premise unproven, options clean | any | unproven | A4 → R5 | owed `{alter_privilege}`; marker = inherited `OrderedPresentConditional`; `OrderedPresent` re-published flag inherited; never laundered unconditional | L=1 R=0 S=0 |

## Group 3 — OPTIONS: legal partial update / dirty static input / insufficient semantics

| id | SQL | fixture | arm → row | expected | observation |
|---|---|---|---|---|---|
| AM-3.1 | `ALTER SERVER s1 OPTIONS (USER 'u', PORT 3306)` | present, consumable | A3 → R9 | clean authored list → owed ∅; partial-update advisory `notice: connection_members` per candidate P2 (separate finding, adds no gap) | L=1 R=1 S=1 |
| AM-3.2 | `ALTER SERVER s1 OPTIONS (USER 'u', USER 'v')` (duplicate key) | present, consumable | A4 → R11 | owed exactly `{server.alter_options}`; `unverified`; still publishes `OrderedPresent` | S=1 |
| AM-3.3 | `ALTER SERVER s1 OPTIONS (PORT 10000)` | present, consumable | A4 → R11 | `RangeState=out-of-range` → owed `{alter_options}` | S=1 |
| AM-3.4 | `ALTER SERVER s1 OPTIONS (PORT 9999)` | present, consumable | A3 → R9 | in-bound → clean; owed ∅ | S=1 |
| AM-3.5 | `ALTER SERVER s1 OPTIONS ()` / `OPTIONS (FOO 'x')` / `OPTIONS (PORT '3306')` | — | not claimed | unclaimed → base-parser diagnostic (exit-2 class), **never** a named boundary and never `alter_options` owed | parse rejects |
| AM-3.6 | `ALTER SERVER s1 OPTIONS (PASSWORD 'p', HOST 'h')` | present, consumable | A3 → R9 | owed ∅; public output contains no option keys/values/`p`/`h` — redaction check | finding metadata has no payload |

## Group 4 — DROP → ALTER: proven-absent consumption and post-state

| id | SQL | fixture | arm → row | expected | observation |
|---|---|---|---|---|---|
| AM-4.1 | `DROP SERVER s` (proven drop) `; ALTER SERVER s OPTIONS (USER 'u')` | any | stmt2 A2 → R3 | `refused_absent`, blocker, marker = inherited conditional, absent stands; stmt3 `CREATE SERVER s` still sees `OrderedAbsent` | L=1 R=1(drop's read) S=1; ALTER resolver R+=0 |
| AM-4.2 | `DROP SERVER s` (unproven → barrier publish) `; ALTER SERVER s OPTIONS (USER 'u')` | any | stmt2 A1 → R2 | `blocked_batch_effects`, owed `{batch_effects}`, no marker, entry barrier stands | R=0 for stmt2 (barrier exit) |
| AM-4.3 | `DROP SERVER 'lit'` (literal, taints channel) `; ALTER SERVER s1 OPTIONS (USER 'u')` | present | stmt2 A1 → R2 (channel-taint exit) | `blocked_batch_effects`; **zero** resolver requests for stmt2 — taint precedes entry scan | R=0 |

## Group 5 — CREATE → ALTER → DROP: full conditional chain

| id | SQL | fixture | expected | observation |
|---|---|---|---|---|
| AM-5.1 | `CREATE SERVER s FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', DATABASE 'd', USER 'u')` on OrderedAbsent (proven drop) → `ALTER SERVER s OPTIONS (USER 'v')` → `DROP SERVER s` | zero catalog; observed consumable | stmt1 ProvenCreate publishes OrderedPresent+conditional; stmt2 A3 → R4: `conditional_alter`, inherited source flag, re-publishes flag inherited; stmt3 proven drop consuming re-published present → publishes OrderedAbsent again | total R=1 S=1 (drop1's read); all statements carry `conditional_audit` |

## Group 6 — ALTER → ALTER and ALTER → CREATE/DROP: what the prior statement preserves

| id | SQL | fixture | expected | observation |
|---|---|---|---|---|
| AM-6.1 | `ALTER SERVER s1 OPTIONS (USER 'u'); ALTER SERVER s1 OPTIONS (HOST 'h')` | present, consumable | both A3 → R4 after stmt1 publish; stmt2 consumes `OrderedPresent`-same (0 resolver for stmt2); each `conditional_alter`; presence affirmed twice | R=1 total (first touch only — ordered answers stmt2) S=1 |
| AM-6.2 | `ALTER SERVER s9 OPTIONS (USER 'u'); ALTER SERVER s9 OPTIONS (HOST 'h')` | zero, off | both A4 → R13 `{existence, alter_privilege}`; **no entry created after either** — second statement re-reads the cached zero answer | R=1 (objectCache dedupes verbatim) S=1 |
| AM-6.3 | `ALTER SERVER s1 OPTIONS (USER 'u'); CREATE SERVER s1 FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h')` | present, consumable | stmt2 CREATE: `OrderedPresent`-same → TerminalRefusal `refused_present`, marker = inherited conditional=false (catalog-source publish is unconditional) | R=1 S=1 for both statements combined |
| AM-6.4 | `ALTER SERVER s9 OPTIONS (USER 'u'); CREATE SERVER s9 FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h')` | zero, off | stmt1 no publish/barrier/entry → stmt2 binds fresh zero answer → Unproven `{existence, create_privilege, create_options?}` per clean options → `{existence, create_privilege}` | R=1 S=1 (cache reuse) — proves non-barriered ALTER does not shadow later statements |
| AM-6.5 | `ALTER SERVER s9 OPTIONS (USER 'u'); DROP SERVER s9` | zero, off | stmt2 binds zero-row unknown → Unproven `{existence, drop_privilege}` — the ALTER neither created presence nor barriered | R=1 S=1 |

## Group 7 — names: same / ASCII alias / provable-distinct / unproven relation; boundary ALTER write records

| id | SQL | fixture | expected | observation |
|---|---|---|---|---|
| AM-7.1 | `ALTER SERVER s1 OPTIONS (USER 'u'); DROP SERVER S1` (ASCII fold alias) | present, consumable | stmt2 consumes re-published `OrderedPresent` (fold-bucket `s1`) → proven-present existence answer; `conditional_drop` if own premise consumable | R=1 S=1 total |
| AM-7.2 | `ALTER SERVER s9 OPTIONS (USER 'u'); DROP SERVER S9` | zero, off | no publish → stmt2 fresh bind: cache key `S9` ≠ `s9` → fresh resolver call → zero-row → Unproven | R=2 S=2 — alias read not deduped (verbatim cache) |
| AM-7.3 | `ALTER SERVER sé OPTIONS (USER 'u'); DROP SERVER se` | (a) sé present, (b) sé zero | non-ASCII names: audited ALTER (ident form, shape-qualified). (a) sé proven present → publish `OrderedPresent` on bucket `sé`; `se` relates unprovenly to the `sé` entry → stmt2 A1 `blocked_batch_effects` — the unproven relation still barriers **reads**, exactly like family semantics. (b) sé zero → **no publish, no write record, no entry** (the audited-ALTER divergence this case exists to pin) → `se` binds its own catalog answer, not a barrier | (a) R=1 S=1, stmt2 R=0 (barrier exit); (b) R=2 S=2 — proves audited ALTER mints no shadowing record |
| AM-7.4 | `ALTER SERVER 's9' OPTIONS (USER 'u'); DROP SERVER s9` (literal-name boundary form) | any | unchanged accepted E14: literal write → channel taint → stmt2 `blocked_batch_effects`. Second variant: `ALTER SERVER <65-rune ident> OPTIONS (...); DROP SERVER s9` → ident-form boundary write record + unproven-relation barrier | R=0 stmt2 |
| AM-7.5 | audited `ALTER SERVER s1 OPTIONS (USER 'u')` then literal `ALTER SERVER 'x' OPTIONS (USER 'v')` then `DROP SERVER S1` | present | literal write taints whole channel → clears s1's OrderedPresent; stmt3 `blocked_batch_effects` | R=1 S=1 |

## Group 8 — contamination: observed prefix / mode-off tail, separate counts

| id | SQL | fixture | expected | observation |
|---|---|---|---|---|
| AM-8.1 | `ALTER SERVER s1 OPTIONS (USER 'u'); GARBAGE(; ALTER SERVER s1 OPTIONS (HOST 'h')` | present; observed prefix, tail mode-off | stmt1 A3 (L=1 R=1 S=1); parse-failure record → `contaminated`; stmt2 A1 → R1: bound unvouched, owed `{batch_effects}`, no marker, **no resolver delta**; loader delta 0 attributed to **mode-off tail**, never to contamination suppression | R stays 1; S stays 1; `state.contaminated` asserted false→true across the real `enrichStatements` boundary (snapshot-based, mirrors R3-B) |
| AM-8.2 | `GARBAGE(; ALTER SERVER s1 OPTIONS (USER 'u')` | any | stmt1 A1 contaminated; zero reads; entry untouched | R=0 S=0 |

## Group 9 — version / preflight / provider-error termination positions

| id | input | expected | observation |
|---|---|---|---|
| AM-9.1 | audited-shape ALTER, `--target-version` absent | `unaudited` + `gate:"version"`, coverage incomplete, rc 1; loader never fires (gate precedes loader gate); **catalog object read still fires** under the current `bindMySQLServer` path — it has no `Unsupported` gate, same as a version-failed CREATE today (registered current-behavior fact, §A table; whether the execution order keeps or short-circuits it is an implementation decision, not a correctness one — the read's answer is never consumed by a boundary statement) | phase-2 gate precedes loader |
| AM-9.2 | same, `--target-version 8.0.x` (malformed) | typed `ErrInvalidTargetVersion` preflight, rc 2; zero audit reads | preflight precedes enrich |
| AM-9.3 | same, `--target-version 5.7.44` or TiDB product | `unaudited`+`gate:"version"` (5.7) / vendor boundary (TiDB dialect) | two distinct exits |
| AM-9.4 | catalog `error` on audited ALTER | bounded error propagates; `errors.Is` reaches sentinel; public text bounded (no name/grant/payload); **not** degraded to a gap | error exit, no arm |
| AM-9.5 | loader error with ALTER-only request | bounded `serverPrivilegeReadError`-class exit | propagates |

## Group 10 — public output redaction, metadata immutability, unrelated objects

| id | SQL | fixture | expected |
|---|---|---|---|
| AM-10.1 | any audited ALTER | — | `raw_sql`/`normalized_sql` = `ALTER SERVER <redacted>`; finding carries no name, option keys/values, account, grant text; unsupported boundary entries carry the fixed skeleton |
| AM-10.2 | `ALTER SERVER s1 OPTIONS (USER 'u'); DROP SERVER s1` | present | stmt1's bound `Metadata.Server` unchanged after stmt2 (channel views never mutate prior bound state); stmt2 sees stmt1's published `OrderedPresent` |
| AM-10.3 | `ALTER SERVER s1 OPTIONS (USER 'u'); CREATE TABLE t(a int)` | present | tracked table model untouched; server channel never spreads to table/schema state |
| AM-10.4 | `ALTER SERVER 'lit-name' OPTIONS (USER 'u')` | — | literal name → `unaudited` boundary + channel taint at apply; no `alter_server` audited finding; resolver request issued but provider declines without SQL (`identity_unproven`) |

## Cross-cutting expected invariants

1. `server.alter_privilege` is **never** discharged by a sibling's
   `drop_privilege`/`create_privilege` premise or marker.
2. `conditional_audit` appears iff own consumable premise fed the
   arm OR a conditional ordered source was consumed — never both-
   counted, never on A1.
3. Owed sets are exact: `{existence}` / `{alter_privilege}` /
   `{alter_options}` / `{batch_effects}` / unions — no open-ended
   "and more".
4. Presence facts are never invalidated by an audited ALTER —
   barrier publication only on A1 (channel already shadowed).
5. Boundary ALTERs keep the accepted write-record + taint
   semantics untouched (E14 `[cur]` preserved).
6. Zero-row catalog is unknown on every row — never absent.
7. Verbatim cache key and ordered identity relation stay separate
   (AM-7.2: alias misses the cache and still binds its own answer).
8. Counts are per-layer honest: L, R, S are never inferred from
   each other (R3-A's independent resolver counter precedent).

## Deferred-to-execution notes

- The execution order names the rule ID (`ddl.mysql.server.alter.
  notice` candidate), the `ServerAlterArm`/`AdjudicateServerAlter`
  signatures, and `Metadata.ServerAlter` — all proposed here, none
  implemented.
- Golden cells (`T26.json` `alter_server` entries) migrate under the
  future candidate's declared policy — expectations derive from the
  README's worked rows only under enabled-rule premises.
- Native/oracle rows (P5, P6 minimal evidence) may attach to a later
  native evidence round; the contract does not require them.
