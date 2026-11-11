# T26-SERVER-ALTER-CONTRACT-01 — implementation acceptance matrix

Status: **PLAN / UNEXECUTED** — every row is a design-to-execute case
for the future ALTER execution order. Nothing below has been run;
no row asserts PASS. Existing tests are cited as static design basis
only. Row IDs are stable (`AM-<group>.<n>`); adjudication row IDs
(`R1`–`R17`) refer to the README's worked-row table; arm/flag values
refer to the README §D canonical assignment table.

Fixture vocabulary (shared with the accepted slices):

- **modes**: `observed` = `observed_account_new_session` privilege
  mode selected; `off` = mode never selected (premise unproven);
  `none` = no provider at all (no loader, no resolver).
- **premise**: `consumable` = positive SUPER witness in scope;
  `suppressed` = witness exists but a scope terminator preceded;
  `unproven` = no witness / no mode.
- **catalog stub**: `present` = one `mysql.servers` row for the
  verbatim name; `zero` = clean zero-row answer; `error` = resolver
  raises the injected sentinel. Catalog rows are per verbatim name;
  names not listed answer `zero`.
- **counts**: L = privilege-loader calls (≤1/request, fires once at
  enrich entry before the statement loop), R = provider-facing
  resolver requests (per `ResolveObject` forward seam), S = catalog
  SQL queries (`mysql.servers` reads). `setup`/`tail`/`total` are
  counted independently; a call counted in a phase stays counted in
  the total.
- **two-phase enrich** (`prefix`/`tail`): two real `enrichStatements`
  calls sharing one `batchState`, the R3-B mechanism — mode is
  selected per call, snapshots are taken across the call boundary.
  A single SQL batch is one call.
- `s`/`s1`/`s9`/`sé`/`se` are fixture identities: `s1` catalog
  `present`, `s9` catalog `zero`, `sé`/`se` non-ASCII pair per-case,
  `s` per-case.
- **seeded unit input**: `ServerAlterAdjudication` unit-level rows
  seed `ServerState`/`ServerDecl`/`Premise` directly — no SQL, no
  provider; S is declared N/A at that layer.

## Group 1 — single ALTER: present / ordered-absent / catalog-unknown

| id | SQL | fixture | pre-state | mode | arm → row | expected (exact) | observation points |
|---|---|---|---|---|---|---|---|
| AM-1.1 | `ALTER SERVER s1 OPTIONS (USER 'u')` | catalog present | none | observed, consumable | A3 → R9 | owed ∅; outcome `conditional_alter`; marker `conditional_audit` set (own premise); coverage complete; verdict contribution none; publish `OrderedPresent` flag=false (catalog source) | L=1 R=1 S=1; resolver args `{Type:mysql_server, Name:s1, name_form:identifier}` |
| AM-1.2 | same SQL | catalog present | none | off | A4 → R10 | owed exactly `{server.alter_privilege}`; outcome `unverified`; no marker; coverage unverified; verdict review; publish `OrderedPresent` flag=false | L=0 R=1 S=1 |
| AM-1.3 | seeded unit input: `ServerState{OrderedAbsent:true, OrderedAbsentConditional:true}`, decl `s` ident-form, premise unproven, options clean | n/a — no provider call | OrderedAbsent on s, flag true | seeded unproven | A2 → R3 | owed ∅; outcome `refused_absent`; blocker; `ConditionalAudit=true` **solely inherited** (ConditionalSource=consumed flag); coverage complete; verdict reject; absent stands | L/R/S n/a (unit layer) |
| AM-1.4 | `ALTER SERVER s9 OPTIONS (USER 'u')` | catalog zero | none | observed, consumable | A4 → R12 | owed exactly `{server.existence}`; `unverified`; marker set (own consumable); coverage unverified; **no publish, no barrier, no entry** | L=1 R=1 S=1 |
| AM-1.5 | same SQL | catalog zero | none | off | A4 → R13 | owed `{server.existence, server.alter_privilege}`; `unverified`; no marker; no publish/barrier/entry | L=0 R=1 S=1 |
| AM-1.6 | same SQL | none (no provider) | none | observed requested, loader absent | A4 → R16 | owed `{existence, alter_privilege}`; `unverified`; no marker; none published | L=0 R=0 S=0 |

## Group 2 — own ALTER privilege: consumable / unproven / suppressed / inherited-marker-only

| id | SQL | fixture | premise | arm → row | expected | observation |
|---|---|---|---|---|---|---|
| AM-2.1 | `ALTER SERVER s1 OPTIONS (USER 'u')` | present | consumable | A3 → R9 | owed ∅; `conditional_alter`; marker via own premise | L=1 R=1 S=1 |
| AM-2.2 | same | present | unproven (mode off) | A4 → R10 | owed `{alter_privilege}` only; no marker; `unverified` | L=0 R=1 S=1 |
| AM-2.3 | `REVOKE SELECT ON *.* FROM 'r1'@'%'; ALTER SERVER s1 OPTIONS (USER 'u')` | present; observed; positive SUPER witness in scope before the terminator | suppressed | A4 → R10 | owed `{alter_privilege}` only — suppression is owed, never a denial; terminator emits no grant-revoked claim | L=1 (loader loads once at enrich entry, before the statement loop — the terminator suppresses only the projection) R=1 S=1 |
| AM-2.4 | two-phase: prefix call `DROP SERVER s` (mode observed, consumable; catalog present) → proven drop publishes `OrderedAbsent` flag=true; tail call `ALTER SERVER s OPTIONS (USER 'u')` (mode **off** — the reason the own premise is unproven) | present; shared batchState | prefix consumable / tail unproven | tail A2 → R3 | marker set **solely from inherited** conditional absent — proves inheritance without own premise; owed ∅; `refused_absent`; absent stands | prefix L=1 R=1 S=1; tail L=0 R=0 S=0 (mode-off → no loader delta; A2 → no resolver); total L=1 R=1 S=1 |
| AM-2.5 | two-phase: prefix call `CREATE SERVER s FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', DATABASE 'd', USER 'u')` (mode observed, consumable; catalog **zero** → ProvenCreate publishes `OrderedPresent` flag=true); tail call `ALTER SERVER s OPTIONS (USER 'u')` (mode off) | zero; shared batchState | prefix consumable / tail unproven | tail A4 → R5 | owed `{alter_privilege}`; marker = inherited `OrderedPresentConditional`; re-published flag = consumed flag (never laundered unconditional) | prefix L=1 R=1 S=1; tail L=0 R=0 S=0; total L=1 R=1 S=1 |

Both AM-2.4/2.5 also register a **seeded unit-level variant**: seed
`OrderedAbsent{flag:true}` / `OrderedPresent{flag:true}` on the
shared identity directly with premise unproven — same arm and flag
assertions, S declared N/A. Either form is an accepted witness; the
two-phase form additionally proves the mode-off tail is what makes
the own premise unproven, not an unexplained fixture flip.

## Group 3 — OPTIONS: legal partial update / dirty static input / insufficient semantics

| id | SQL | fixture | arm → row | expected | observation |
|---|---|---|---|---|---|
| AM-3.1 | `ALTER SERVER s1 OPTIONS (USER 'u', PORT 3306)` | present, consumable | A3 → R9 | clean authored list → owed ∅; partial-update advisory per decided P2 (separate finding under `ddl.mysql.server.alter.notice`, fixed text in README §G, adds no gap) | L=1 R=1 S=1 |
| AM-3.2 | `ALTER SERVER s1 OPTIONS (USER 'u', USER 'v')` (duplicate key) | present, consumable | A4 → R11 | owed exactly `{server.alter_options}`; `unverified`; still publishes `OrderedPresent` flag=false | L=1 R=1 S=1 |
| AM-3.3 | `ALTER SERVER s1 OPTIONS (PORT 10000)` | present, consumable | A4 → R11 | `RangeState=out-of-range` → owed `{alter_options}`; advisory still emitted (A4 in scope for P2); publish flag=false | L=1 R=1 S=1 |
| AM-3.4 | `ALTER SERVER s1 OPTIONS (PORT 9999)` | present, consumable | A3 → R9 | in-bound → clean; owed ∅ | L=1 R=1 S=1 |
| AM-3.5 | `ALTER SERVER s1 OPTIONS ()` / `OPTIONS (FOO 'x')` / `OPTIONS (PORT '3306')` | — | not claimed | unclaimed → base-parser diagnostic (exit-2 class), **never** a named boundary and never `alter_options` owed | parse rejects |
| AM-3.6 | `ALTER SERVER s1 OPTIONS (PASSWORD 'pw-sentinel-455', HOST 'host-sentinel-77')` | present, consumable | A3 → R9 | owed ∅; public output contains no option keys/values and neither `pw-sentinel-455` nor `host-sentinel-77` appears anywhere — redaction check with distinguishable sentinels | finding metadata has no payload |

## Group 4 — DROP → ALTER: proven-absent consumption and post-state

| id | SQL | fixture | arm → row | expected | observation |
|---|---|---|---|---|---|
| AM-4.1 | `DROP SERVER s; ALTER SERVER s OPTIONS (USER 'u'); CREATE SERVER s FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', DATABASE 'd', USER 'u')` | catalog present on `s`; observed consumable | stmt2 A2 → R3 | stmt1 proven drop → `OrderedAbsent` flag=true; stmt2 `refused_absent`, blocker, marker = inherited conditional, absent stands; stmt3 still consumes `OrderedAbsent` → proven-absent path (create arm per its own contract) | stmt1 L=1 R=1 S=1; stmt2 R+=0 S+=0 (ordered answered); stmt3 R+=0 (ordered answer stands); total L=1 R=1 S=1 |
| AM-4.2 | `DROP SERVER s; ALTER SERVER s OPTIONS (USER 'u')` | catalog present on `s`; mode **off** (the drop is unproven → barrier publication) | stmt2 A1 → R2 | stmt1 unproven drop → `serverWrite` + entry `s` barrier=true; stmt2 relation same → `blocked_batch_effects`, owed `{batch_effects}`, no marker; **A1 no-op** — the barrier entry stands, nothing is created/modified/cleared | stmt1 L=0 R=1 S=1; stmt2 R=0 S=0 (barrier exit before resolver); total L=0 R=1 S=1 |
| AM-4.3 | `DROP SERVER 'lit9'; ALTER SERVER s1 OPTIONS (USER 'u')` | `s1` present; mode off | stmt2 A1 → R2 (channel-taint exit) | stmt1 literal-name boundary write → whole-channel taint; stmt2 `blocked_batch_effects`; **zero** resolver requests for stmt2 — taint precedes the entry scan | stmt1 L=0 R=1 (literal resolver request issued, provider declines, S=0); stmt2 R=0 S=0; total L=0 R=1 S=0 |

## Group 5 — DROP → CREATE → ALTER → DROP: full conditional chain

| id | SQL | fixture | expected | observation |
|---|---|---|---|---|
| AM-5.1 | `DROP SERVER s; CREATE SERVER s FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', DATABASE 'd', USER 'u'); ALTER SERVER s OPTIONS (USER 'v'); DROP SERVER s;` | catalog present on `s`; observed consumable throughout | stmt1 proven drop → `OrderedAbsent` flag=true; stmt2 ProvenCreate consuming ordered absent → publishes `OrderedPresent` flag=true (conditional); stmt3 A3 → R4: `conditional_alter`, marker inherited, re-publishes `OrderedPresent` flag=consumed flag=true; stmt4 proven drop consuming the re-published present → publishes `OrderedAbsent` again | stmt1 R=1 S=1 (its catalog read); stmt2–4 R+=0 — all ordered-answered; total **L=1 R=1 S=1**; every statement carries `conditional_audit` |

## Group 6 — ALTER → ALTER and ALTER → CREATE/DROP: what the prior statement preserves

| id | SQL | fixture | expected | observation |
|---|---|---|---|---|
| AM-6.1 | `ALTER SERVER s1 OPTIONS (USER 'u'); ALTER SERVER s1 OPTIONS (HOST 'h')` | present, consumable | stmt1 A3 → R9 (catalog source, publishes `OrderedPresent` flag=false); stmt2 A3 → R4 (consumes `OrderedPresent`-same, re-publishes flag=false); each `conditional_alter`; presence affirmed twice | stmt1 R=1 S=1; stmt2 R=0 — ordered answered; total L=1 R=1 S=1 |
| AM-6.2 | `ALTER SERVER s9 OPTIONS (USER 'u'); ALTER SERVER s9 OPTIONS (HOST 'h')` | zero, off | both A4 → R13 `{existence, alter_privilege}`; **no entry created after either** — second statement re-reads the cached zero answer | total R=1 S=1 (objectCache dedupes verbatim `s9`) |
| AM-6.3 | `ALTER SERVER s1 OPTIONS (USER 'u'); CREATE SERVER s1 FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', DATABASE 'd', USER 'u')` | present, consumable | stmt2 CREATE: `OrderedPresent`-same → TerminalRefusal `refused_present`, marker = inherited flag=false (catalog-source publish is unconditional) — the ALTER's own `ConditionalAudit` never leaches into presence | total R=1 S=1 |
| AM-6.4 | `ALTER SERVER s9 OPTIONS (USER 'u'); CREATE SERVER s9 FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', DATABASE 'd', USER 'u')` | zero, off | stmt1 no publish/barrier/entry → stmt2 binds fresh zero answer → Unproven `{existence, create_privilege}` (clean options) | total R=1 S=1 (cache reuse) — proves non-barriered ALTER does not shadow later statements |
| AM-6.5 | `ALTER SERVER s9 OPTIONS (USER 'u'); DROP SERVER s9` | zero, off | stmt2 binds zero-row unknown → Unproven `{existence, drop_privilege}` — the ALTER neither created presence nor barriered | total R=1 S=1 |
| AM-6.6 | `ALTER SERVER s1 OPTIONS (USER 'u', USER 'v'); CREATE SERVER s1 FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', DATABASE 'd', USER 'u')` | present, consumable; stmt1 authored list dirty (duplicate key) | stmt1 A4 → R11: owed `{alter_options}`, `ConditionalAudit=true` (own consumable), `PublishPresent=true` with **flag=false** (catalog source — audit conditionality ≠ presence conditionality); stmt2 CREATE → TerminalRefusal `refused_present`, marker = inherited flag=false — **no** conditional marker surfaces on the refusal | total R=1 S=1; snapshot asserts `Metadata.ServerAlter.{Owed, PublishPresent, ConditionalSource=false, ConditionalAudit=true}` before/after own apply and after stmt2's apply |

## Group 7 — names: same / ASCII alias / provable-distinct / unproven relation; boundary ALTER write records

| id | SQL | fixture | expected | observation |
|---|---|---|---|---|
| AM-7.1 | `ALTER SERVER s1 OPTIONS (USER 'u'); DROP SERVER S1` (ASCII fold alias) | present, consumable | stmt2 consumes re-published `OrderedPresent` (fold-bucket `s1`) → proven-present existence answer; `conditional_drop` if own premise consumable | total R=1 S=1 |
| AM-7.2 | `ALTER SERVER s9 OPTIONS (USER 'u'); DROP SERVER S9` | zero, off | no publish → stmt2 fresh bind: cache key `S9` ≠ `s9` → fresh resolver call → zero-row → Unproven | total R=2 S=2 — alias read not deduped (verbatim cache) |
| AM-7.3 | `ALTER SERVER sé OPTIONS (USER 'u'); DROP SERVER se` | (a) `sé` present, `se` zero; (b) `sé` zero, `se` zero; mode observed consumable | non-ASCII names: audited ALTER (ident form, shape-qualified). (a) `sé` proven present → publishes `OrderedPresent` on bucket `sé`; `se` relates unprovenly to the `sé` entry → stmt2 A1 `blocked_batch_effects`, owed `{batch_effects}` — unproven relation still barriers **reads**. (b) `sé` zero → **no publish, no write record, no entry** → `se` binds its own catalog answer (zero → Unproven `{existence, drop_privilege}`) | (a) stmt1 R=1 S=1, stmt2 R=0 (barrier exit) → total R=1 S=1; (b) total R=2 S=2 — proves audited ALTER mints no shadowing record |
| AM-7.4 | `ALTER SERVER 's9' OPTIONS (USER 'u'); DROP SERVER s9` then variant `ALTER SERVER sssssssssssssssssssssssssssssssssssssssssssssssssssssssssssssssss OPTIONS (USER 'u'); DROP SERVER s9` | `s9` zero; mode off | unchanged accepted E14: literal name → channel taint → stmt2 `blocked_batch_effects`. Variant: 65-rune ident-form boundary → binds (catalog `zero` for the 65-rune name, no entry minted) then apply records a `serverWrite`; stmt2's relation scan hits that write record (relation unproven) → `blocked_batch_effects` | literal variant: stmt1 R=1 S=0 (provider declines), stmt2 R=0 → total R=1 S=0; 65-rune variant: stmt1 R=1 S=1 (boundary ident-form still binds and reads), stmt2 R=0 → total R=1 S=1 |
| AM-7.5 | `ALTER SERVER s1 OPTIONS (USER 'u'); ALTER SERVER 'x' OPTIONS (USER 'v'); DROP SERVER S1` | `s1` present; mode observed consumable | stmt1 A3 → R9 publishes `OrderedPresent` flag=false on `s1`; stmt2 literal-name boundary: bind issues the provider-facing resolver request (`{Name:'x'}` no qualifier) — provider declines without SQL (`identity_unproven`); apply taints the whole channel → stmt1's OrderedPresent cleared; stmt3 `blocked_batch_effects`, owed `{batch_effects}` | stmt1 R=1 S=1; stmt2 R=1 S=0; stmt3 R=0 S=0 → **total R=2 S=1**; L=1 |
| AM-7.6 | `ALTER SERVER sé OPTIONS (USER 'u'); ALTER SERVER se OPTIONS (USER 'v'); DROP SERVER sé` | `sé` present; mode observed consumable; ordered channel clean at entry | stmt1 A3 → R9 publishes `OrderedPresent` flag=false on `sé`; stmt2 relation-scan hits unproven `sé` entry → A1 `blocked_batch_effects`, owed `{batch_effects}` — **channel untouched: no `se` entry, no write record, no barrier minted, `sé` presence stands**; stmt3 still consumes the original `sé` presence → proven drop | stmt1 R=1 S=1; stmt2 R=0 S=0 (barrier exit); stmt3 R=0 (ordered answered); total **L=1 R=1 S=1** — the A1 no-side-effect discriminator |

## Group 8 — contamination: observed prefix / mode-off tail, separate counts

| id | SQL / calls | fixture | expected | observation |
|---|---|---|---|---|
| AM-8.1 | call1 (mode observed): `ALTER SERVER s1 OPTIONS (USER 'u'); GARBAGE(` → records [stmt1 ALTER, failure record]; call2 (mode **off**, same `batchState`): `ALTER SERVER s1 OPTIONS (HOST 'h')` | `s1` present; shared batchState; snapshot `state.contaminated` false→true across the call boundary | call1: stmt1 A3 → R9 (`conditional_alter`, publishes `OrderedPresent` flag=false); failure record sets `contaminated=true`; call2: stmt enters contaminated → A1 → R1 bound unvouched, owed `{batch_effects}`, no marker, **no publish/no-op on the channel** (stmt1's OrderedPresent stands — contamination blocks consumption, it does not clear) | call1 L=1 R=1 S=1; call2 **loader delta 0 attributed to mode-off tail** (the loader gate evaluates mode at enrich entry — contamination is not the cause), resolver delta 0 attributed to contamination; total L=1 R=1 S=1; assert `Metadata.Server`/`Metadata.ServerAlter` snapshots pre/post both calls |
| AM-8.2 | single batch (mode observed): `GARBAGE(; ALTER SERVER s1 OPTIONS (USER 'u')` → records [failure record#1, stmt#2 ALTER] | `s1` present | loader gate runs at enrich entry **before** the failure record is processed → L=1 (the audited-shape ALTER still triggers the gate); record#1 sets `contaminated`; stmt#2 A1 → R1, owed `{batch_effects}`, zero reads | total L=1 R=0 S=0; statement numbering: failure=record#1, ALTER=stmt#2 |

## Group 9 — version / preflight / provider-error termination positions

| id | input | fixture | expected | observation |
|---|---|---|---|---|
| AM-9.1a | audited-shape ALTER, no `--target-version`, no provider (providerless CLI request) | providerless | `unaudited` + `gate:"version"`, coverage incomplete; loader never fires; **no** catalog read possible (nothing to read with) | L=0 R=0 S=0; verdict per existing mapping (incomplete ⇒ not pass) |
| AM-9.1b | audited-shape ALTER, no `--target-version`, provider present but observed version out-of-domain (`5.7.44`) | `s1` present | `unaudited` + `gate:"version"`; loader never fires; **catalog read still fires** — decided keep-boundary-bind (README §E): `bindMySQLServer` has no `Unsupported` gate, the gated ALTER walks the six-step order; the answer is never consumed | L=0 R=1 S=1 |
| AM-9.2 | same, `--target-version 8.0.x` (malformed) | any | typed `ErrInvalidTargetVersion` preflight, rc 2; zero audit reads | preflight precedes enrich — L=0 R=0 S=0 |
| AM-9.3a | same, `--target-version 5.7.44` (resolved, out-of-domain) | provider present | `unaudited` + `gate:"version"`; same keep-boundary-bind read behavior as AM-9.1b | L=0 R=1 S=1 |
| AM-9.3b | same, dialect `tidb` | any | vendor boundary — `unaudited`, product not supported; never reaches the MySQL resolver | L=0 R=0 S=0 |
| AM-9.3c | no pin, provider observed in-domain version (e.g. banner `8.0.36`) | provider present, observed consumable | `resolvedVersion` = observed → G2 passes → **audited**: `ALTER SERVER s1 OPTIONS (USER 'u')` lands A3 → R9; the pin is not the gate | L=1 R=1 S=1 |
| AM-9.3d | MySQL request + observed non-MySQL product | provider observed product `postgres` | typed `ErrDialectProductMismatch`, early error before any arm | audit-layer L=0 R=0 S=0 — the version observation itself is not billed into L/R/S |
| AM-9.3e | `--target-version 8.0.36` + observed version `8.4.0` | provider observed | typed `ErrTargetVersionMismatch`, early error | audit-layer L=0 R=0 S=0 |
| AM-9.4 | catalog `error` on audited ALTER | injected sentinel | bounded error propagates; `errors.Is` reaches sentinel; public text bounded (no name/grant/payload); **not** degraded to a gap | error exit, no arm |
| AM-9.5 | loader error with ALTER-only request | injected loader sentinel | bounded `serverPrivilegeReadError`-class exit | propagates |

**Registered CLI probe plan** (future execution order — none run in
this contract round): providerless `--no-metadata` shape, privilege
mode off, single clean `ALTER SERVER s OPTIONS (USER 'u')`, policy
enabling `ddl.mysql.server.alter.notice` at notice. Expected: pin
`8.0.36` ⇒ A4, owed `{server.existence, server.alter_privilege}`,
no marker, `unverified`, verdict review, rc per that policy's
existing mapping; no pin ⇒ AM-9.1a version boundary; malformed
`8.0.x` ⇒ AM-9.2 typed preflight. No generalised pin⇒rc rule is
asserted — A2's blocker and other policies map by existing rules.

## Group 10 — public output redaction, metadata immutability, unrelated objects

| id | SQL | fixture | expected |
|---|---|---|---|
| AM-10.1 | any audited ALTER | — | `raw_sql`/`normalized_sql` = `ALTER SERVER <redacted>`; finding carries no name, option keys/values, account, grant text; unsupported boundary entries carry the fixed skeleton |
| AM-10.2 | `ALTER SERVER s1 OPTIONS (USER 'u'); DROP SERVER s1` | `s1` present, consumable | snapshot plan: `Metadata.Server` **and** `Metadata.ServerAlter` checked for stmt1 — `Owed`, `PublishPresent`, `ConditionalSource`, `ConditionalAudit` — before apply, after own apply, and after stmt2's apply; bound state never mutates retroactively; stmt2 consumes stmt1's published `OrderedPresent` |
| AM-10.3 | `ALTER SERVER s1 OPTIONS (USER 'u'); CREATE TABLE t(a int)` | `s1` present | tracked table model untouched; server channel never spreads to table/schema state |
| AM-10.4 | `ALTER SERVER 'lit9' OPTIONS (USER 'u'); DROP SERVER s9` | provider present; `s9` zero; ordered channel clean at entry; mode off | literal name → `unaudited` boundary + whole-channel taint at apply; no `alter_server` audited finding; resolver request issued for `lit9` but provider declines without SQL (`identity_unproven`) → R=1 S=0; stmt2 `blocked_batch_effects` R=0 — consistent with AM-7.5's literal branch |

## Cross-cutting expected invariants

1. `server.alter_privilege` is **never** discharged by a sibling's
   `drop_privilege`/`create_privilege` premise or marker.
2. `conditional_audit` appears iff own consumable premise fed the
   arm OR a conditional ordered source was consumed — never both-
   counted, never on A1.
3. Owed sets are exact: `{existence}` / `{alter_privilege}` /
   `{alter_options}` / `{batch_effects}` / unions — no open-ended
   "and more".
4. An audited ALTER introduces **no new presence barrier and no
   write record on any arm** — A1 is a uniform no-op that only
   reports the pre-existing shadow; it can still owe
   `server.batch_effects`.
5. Boundary ALTERs keep the accepted write-record + taint
   semantics untouched (E14 `[cur]` preserved).
6. Zero-row catalog is unknown on every row — never absent.
7. Verbatim cache key and ordered identity relation stay separate
   (AM-7.2: alias misses the cache and still binds its own answer).
8. Counts are per-layer honest: L, R, S are never inferred from
   each other (R3-A's independent resolver counter precedent);
   setup/tail/total phases are independently accounted.
9. `PublishPresent=true` ⇒ published `OrderedPresentConditional`
   equals `ConditionalSource`, never `ConditionalAudit` (AM-6.6 is
   the discriminator).
10. `Metadata.ServerAlter` is the single adjudication consumed by
    both rule projection and ordered apply — no second truth.

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
