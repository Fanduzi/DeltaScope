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
  provider; L/R/S are declared N/A at that layer.

**Shared defaults** — one concrete fixture; every row inherits it
unless its own `fixture`/`mode`/layer fields declare an override.
Rows tagged `unit`, `off`, `providerless`, `suppressed`, `error`,
or `boundary` explicitly override the corresponding fields; no row
silently changes arm under these defaults.

- `dialect = mysql`; observed product `mysql`; observed version
  `8.0.36`; **no pin** unless a row declares one; `resolvedVersion`
  is therefore `8.0.36` (in-domain) unless a row overrides.
- privilege mode `observed_account_new_session` with a positive,
  in-scope, direct-global `SUPER` witness ⇒ premise `consumable`
  unless a row declares `off`/`suppressed`/`unproven`.
- healthy loader and provider; one `mysql.servers` resolver wired
  through the per-request `objectCache`; ordered channel and cache
  clean at batch entry.
- `ddl.mysql.server.alter.notice` enabled at `notice`; sibling
  rules under their accepted defaults.
- loader ≤1 per enrich call at entry; R at the `ResolveObject`
  forward seam; S at the provider SQL seam; R never implies S.
- **public finding shape** (per the fixed §G P2 projection, **per
  audited ALTER statement**, not per batch): every arm emits the
  main finding `object=server, operation=alter_server,
  outcome=<arm token>`, `conditional_audit` only when
  `Metadata.ServerAlter.ConditionalAudit` (token
  `observed_account_new_session`); A3/A4 additionally emit the
  independent `notice=connection_members` advisory (no outcome,
  no marker, no gap). Per audited ALTER: A3/A4 → 2 findings,
  A1/A2 → 1; a boundary statement → the fixed `unaudited`
  skeleton. A multi-statement batch's total findings are the sum
  over statements.

**Per-statement public-output scope** — batches are enriched and
applied in full fixture order; REVOKE and failure records still
drive suppression/contamination/state transitions as registered.
Unless a row declares otherwise, its public assertions cover only
the **SERVER records it lists** — findings, level, metadata, exact
owed/gaps, conditional marker, statement coverage, and any
applicable SQL redaction / `unaudited` boundary skeleton. Public
output of non-SERVER statements and failure records is **out of
scope** for the row (they still produce output; it is just not
asserted). Whole-request finding totals, aggregate coverage, and
verdict are asserted only on rows that declare them; elsewhere
they are N/A — N/A means *not observed here*, never "zero output".
The "batch findings are the sum of per-statement findings" note
applies only to rows that exercise full-request public
projection.

**Observation layers** — five named layers; a row asserts fields
only at the layer(s) it exercises and marks the rest N/A:

| layer | rows |
|---|---|
| adjudication-unit (seeded inputs; L/R/S, public projection, aggregate, rc all N/A) | AM-1.3, AM-2.4, AM-2.5 |
| parser/extraction (unclaimed input; everything downstream N/A — enrich/apply, projection, aggregate, rc) | AM-3.5 |
| enrich-apply + per-statement public scope (CLI rc N/A) | all rows not listed elsewhere |
| request/API surface (version gates, typed errors; rc N/A — no CLI command asserted) | AM-9.1b, AM-9.3a–e |
| CLI (exact rc asserted) | AM-9.1a, AM-9.2, CLI probe plan |
| enrich-apply only (bounded error at the named seam; public/aggregate/rc N/A) | AM-9.4, AM-9.5, AM-10.3 |

## Group 1 — single ALTER: present / ordered-absent / catalog-unknown

| id | SQL | fixture | pre-state | mode | arm → row | expected (exact) | observation points |
|---|---|---|---|---|---|---|---|
| AM-1.1 | `ALTER SERVER s1 OPTIONS (USER 'u')` | catalog present | none | observed, consumable | A3 → R9 | owed ∅; outcome `conditional_alter`; marker `conditional_audit` set (own premise); coverage complete; verdict contribution none; publish `OrderedPresent` flag=false (catalog source) | L=1 R=1 S=1; resolver args `{Type:mysql_server, Name:s1, name_form:identifier}` |
| AM-1.2 | same SQL | catalog present | none | off | A4 → R10 | owed exactly `{server.alter_privilege}`; outcome `unverified`; no marker; coverage unverified; verdict review; publish `OrderedPresent` flag=false | L=0 R=1 S=1 |
| AM-1.3 | seeded unit input: `ServerState{OrderedAbsent:true, OrderedAbsentConditional:true}`, decl `s` ident-form, premise unproven, options clean | n/a — no provider call | OrderedAbsent on s, flag true | seeded unproven | A2 → R3 | owed ∅; outcome `refused_absent`; blocker; `ConditionalAudit=true` **solely inherited** (ConditionalSource=consumed flag); coverage complete; verdict reject; absent stands | L/R/S n/a (unit layer) |
| AM-1.4 | `ALTER SERVER s9 OPTIONS (USER 'u')` | catalog zero | none | observed, consumable | A4 → R12 | owed exactly `{server.existence}`; `unverified`; marker set (own consumable); coverage unverified; **no publish, no barrier, no entry** | L=1 R=1 S=1 |
| AM-1.5 | same SQL | catalog zero | none | off | A4 → R13 | owed `{server.existence, server.alter_privilege}`; `unverified`; no marker; no publish/barrier/entry | L=0 R=1 S=1 |
| AM-1.6 | same SQL | none (no provider) — **enrich-layer fixture**: `resolvedVersion=8.0.36` supplied directly (this row does not exercise version resolution) | none | observed requested, loader absent | A4 → R16 | owed `{existence, alter_privilege}`; `unverified`; no marker; none published | L=0 R=0 S=0 |

## Group 2 — own ALTER privilege: consumable / unproven / suppressed / inherited-marker-only

| id | SQL | fixture | premise | arm → row | expected | observation |
|---|---|---|---|---|---|---|
| AM-2.1 | `ALTER SERVER s1 OPTIONS (USER 'u')` | present | consumable | A3 → R9 | owed ∅; `conditional_alter`; marker via own premise | L=1 R=1 S=1 |
| AM-2.2 | same | present | unproven (mode off) | A4 → R10 | owed `{alter_privilege}` only; no marker; `unverified` | L=0 R=1 S=1 |
| AM-2.3 | `REVOKE SELECT ON *.* FROM 'r1'@'%'; ALTER SERVER s1 OPTIONS (USER 'u')` | present; observed; positive SUPER witness in scope before the terminator | suppressed | A4 → R10 | owed `{alter_privilege}` only — suppression is owed, never a denial; terminator emits no grant-revoked claim. **Frozen per-statement projection on the ALTER record**: A4 → 2 notice findings (main `unverified` + `connection_members` advisory); owed exactly `{server.alter_privilege}`; no conditional marker; statement coverage `unverified`. REVOKE's own public output is out of scope | L=1 (loader loads once at enrich entry, before the statement loop — the terminator suppresses only the projection) R=1 S=1 |
| AM-2.4 | **seeded adjudication-unit input**: `ServerState{OrderedAbsent:true, OrderedAbsentConditional:true}` on `s`, decl `ALTER SERVER s OPTIONS (USER 'u')` ident-form, own premise `unproven`, options clean | unit layer — no SQL, no provider | seeded unproven | A2 → R3 | owed ∅; `PublishPresent=false`; `ConditionalSource=true` (consumed flag); `ConditionalAudit=true` — marker **solely inherited**, proves inheritance without an own consumable premise; `refused_absent`; absent stands | L/R/S: N/A (unit layer) |
| AM-2.5 | **seeded adjudication-unit input**: `ServerState{OrderedPresent:true, OrderedPresentConditional:true}` on `s`, decl `ALTER SERVER s OPTIONS (USER 'u')` ident-form, own premise `unproven`, options clean | unit layer — no SQL, no provider | seeded unproven | A4 → R5 | owed exactly `{server.alter_privilege}`; `PublishPresent=true`; `ConditionalSource=true` (consumed flag); `ConditionalAudit=true`; re-published flag = consumed flag (`true`, never laundered unconditional) | L/R/S: N/A (unit layer) |
| AM-2.4i | integration chain (premise **suppressed**, not unproven): `DROP SERVER s; REVOKE SELECT ON *.* FROM 'r1'@'%'; ALTER SERVER s OPTIONS (USER 'u')` | `s` present; observed, consumable; terminator precedes stmt3 | stmt3 suppressed | stmt3 A2 → R3 | stmt1 proven drop → `OrderedAbsent` flag=true; stmt2 scope terminator; stmt3 `refused_absent`, owed ∅, marker = inherited conditional absent (suppressed premise feeds no own marker). **Frozen per-statement projection on stmt3**: A2 → 1 blocker finding `refused_absent`; owed ∅; `conditional_audit` = inherited conditional absent; statement coverage `complete`; no advisory | stmt1 R=1 S=1; stmt3 R=0 (ordered answered); total L=1 R=1 S=1 |
| AM-2.5i | integration chain (premise **suppressed**): `DROP SERVER s; CREATE SERVER s FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', DATABASE 'd', USER 'u'); REVOKE SELECT ON *.* FROM 'r1'@'%'; ALTER SERVER s OPTIONS (USER 'u')` | `s` present; observed, consumable | stmt4 suppressed | stmt4 A4 → R5 | stmt1 proven drop → conditional absent; stmt2 consumes absent → ProvenCreate publishes `OrderedPresent` flag=true; stmt3 terminator; stmt4 owed `{alter_privilege}`, marker = inherited flag, re-published flag = consumed `true`. **Frozen per-statement projection on stmt4**: A4 → 2 notice findings (main `unverified` + advisory); owed exactly `{server.alter_privilege}`; `conditional_audit` = inherited conditional present; statement coverage `unverified` | stmt1 R=1 S=1; stmt2/4 R=0 (ordered answered); total L=1 R=1 S=1 |

Note on the dropped cross-call claim: the earlier draft asserted a
mode-off *tail* enrich call on a shared `batchState` makes the own
premise unproven. That property is **not statically established**
for a clean tail (the bind excerpt does not cover enrich-entry
privilege-state handling), so it is removed; the seeded units carry
the unproven-premise assertions and the integration chains carry
accurately-labelled `suppressed` premises instead.

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
| AM-4.2 | `DROP SERVER s; ALTER SERVER s OPTIONS (USER 'u')` | catalog present on `s`; mode **off** (the drop is unproven) | stmt2 A1 → R2 | stmt1 unproven drop → entry `s` `barrier=true` (the excerpt-supported accepted behavior; no additional write-record claim is made for this audited DROP); stmt2 relation same → `blocked_batch_effects`, owed `{batch_effects}`, no marker; **A1 no-op** — the barrier entry stands, nothing is created/modified/cleared | stmt1 L=0 R=1 S=1; stmt2 R=0 S=0 (barrier exit before resolver); total L=0 R=1 S=1 |
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
| AM-6.6 | `ALTER SERVER s1 OPTIONS (USER 'u', USER 'v'); CREATE SERVER s1 FOREIGN DATA WRAPPER mysql OPTIONS (HOST 'h', DATABASE 'd', USER 'u')` | present, consumable; stmt1 authored list dirty (duplicate key) | stmt1 A4 → R11: owed `{alter_options}`, `ConditionalAudit=true` (own consumable), `PublishPresent=true` with **flag=false** (catalog source — audit conditionality ≠ presence conditionality); stmt2 CREATE → TerminalRefusal `refused_present`, marker = inherited flag=false — **no** conditional marker surfaces on the refusal. Aggregate: coverage = min(stmt1 `unverified`, stmt2 `complete`) = `unverified`; verdict = `reject` (stmt2 blocker) | total R=1 S=1; snapshot asserts `Metadata.Server` **and** `Metadata.ServerAlter` `{Owed, PublishPresent, ConditionalSource=false, ConditionalAudit=true}` for stmt1 before apply, after own apply, and after stmt2's apply |

## Group 7 — names: same / ASCII alias / provable-distinct / unproven relation; boundary ALTER write records

| id | SQL | fixture | expected | observation |
|---|---|---|---|---|
| AM-7.1 | `ALTER SERVER s1 OPTIONS (USER 'u'); DROP SERVER S1` (ASCII fold alias) | present, consumable | stmt2 consumes re-published `OrderedPresent` (fold-bucket `s1`) → proven-present existence answer; `conditional_drop` if own premise consumable | total R=1 S=1 |
| AM-7.2 | `ALTER SERVER s9 OPTIONS (USER 'u'); DROP SERVER S9` | zero, off | no publish → stmt2 fresh bind: cache key `S9` ≠ `s9` → fresh resolver call → zero-row → Unproven | total R=2 S=2 — alias read not deduped (verbatim cache) |
| AM-7.3 | `ALTER SERVER sé OPTIONS (USER 'u'); DROP SERVER se` | (a) `sé` present, `se` zero; (b) `sé` zero, `se` zero; mode observed consumable; channel clean at entry | non-ASCII names: audited ALTER (ident form, shape-qualified); stmt2 is a plain DROP (no `IF EXISTS`) described in the accepted DROP contract's own vocabulary. (a) `sé` proven present → publishes `OrderedPresent` flag=false on bucket `sé`; `se` relates unprovenly to the `sé` entry → stmt2 = DROP's batch-effects-blocked branch: `blocked_batch_effects`, owed exactly `{server.batch_effects}`, **`conditional_audit=observed_account_new_session` — marker from the DROP's own still-consumable premise** (the blocked bind does not consume an ordered answer, so nothing is inherited and nothing is suppressed), statement coverage `unverified`; post-state per accepted DROP apply: **new entry `se` `{absent=false, publishedArm=0, present=false, presentConditional=false, barrier=true}`** — the channel is NOT unchanged: the stored `sé` presence remains, but the fresh `se` barrier now shadows later binds (an optional stmt3 `ALTER SERVER sé OPTIONS (USER 'v')` lands A1 `blocked_batch_effects` at the relation scan before ever reaching the `sé` ordered answer — static derivation, same fixture, still R=0/S=0). (b) `sé` zero → **no publish, no write record, no entry** → `se` binds its own catalog answer (zero → DROP unknown branch: owed exactly `{server.existence}` — the consumable premise discharges `drop_privilege`; marker set via own consumable premise; statement coverage `unverified`; post-state per the accepted unproven-DROP rule: entry `se` `barrier=true`) | (a) stmt1 R=1 S=1, stmt2 R=0 S=0 → total L=1 R=1 S=1; (b) total L=1 R=2 S=2 — proves audited ALTER mints no shadowing record |
| AM-7.4 | `ALTER SERVER 's9' OPTIONS (USER 'u'); DROP SERVER s9` then variant `ALTER SERVER sssssssssssssssssssssssssssssssssssssssssssssssssssssssssssssssss OPTIONS (USER 'u'); DROP SERVER s9` | `s9` zero; mode off | unchanged accepted E14: literal name → channel taint → stmt2 `blocked_batch_effects`. Variant: 65-rune ident-form boundary → binds (catalog `zero` for the 65-rune name, no entry minted) then apply records a `serverWrite`; stmt2's relation scan hits that write record (relation unproven) → `blocked_batch_effects` | literal variant: stmt1 R=1 S=0 (provider declines), stmt2 R=0 → total R=1 S=0; 65-rune variant: stmt1 R=1 S=1 (boundary ident-form still binds and reads), stmt2 R=0 → total R=1 S=1 |
| AM-7.5 | `ALTER SERVER s1 OPTIONS (USER 'u'); ALTER SERVER 'x' OPTIONS (USER 'v'); DROP SERVER S1` | `s1` present; mode observed consumable | stmt1 A3 → R9 publishes `OrderedPresent` flag=false on `s1`; stmt2 literal-name boundary: bind issues the provider-facing resolver request (`{Name:'x'}` no qualifier) — provider declines without SQL (`identity_unproven`); apply taints the whole channel → stmt1's OrderedPresent cleared; stmt3 `blocked_batch_effects`, owed `{batch_effects}` | stmt1 R=1 S=1; stmt2 R=1 S=0; stmt3 R=0 S=0 → **total R=2 S=1**; L=1 |
| AM-7.6 | `ALTER SERVER sé OPTIONS (USER 'u'); ALTER SERVER se OPTIONS (USER 'v'); DROP SERVER sé` | `sé` present; mode observed consumable; ordered channel clean at entry | stmt1 A3 → R9 publishes `OrderedPresent` flag=false on `sé`; stmt2 relation-scan hits unproven `sé` entry → A1 `blocked_batch_effects`, owed `{batch_effects}` — **channel untouched: no `se` entry, no write record, no barrier minted, `sé` presence stands**; stmt3 still consumes the original `sé` presence → proven drop | stmt1 R=1 S=1; stmt2 R=0 S=0 (barrier exit); stmt3 R=0 (ordered answered); total **L=1 R=1 S=1** — the A1 no-side-effect discriminator |

## Group 8 — contamination: observed prefix / mode-off tail, separate counts

| id | SQL / calls | fixture | expected | observation |
|---|---|---|---|---|
| AM-8.1 | call1 (mode observed): `ALTER SERVER s1 OPTIONS (USER 'u'); GARBAGE(` → records [stmt1 ALTER, failure record]; call2 (mode **off**, same `batchState`): `ALTER SERVER s1 OPTIONS (HOST 'h')` | `s1` present; shared batchState; snapshot `state.contaminated` false→true across the call boundary | call1: stmt1 A3 → R9 (`conditional_alter`, publishes `OrderedPresent` flag=false); failure record sets `contaminated=true`; call2: stmt enters contaminated → A1 → R1 bound unvouched, owed `{batch_effects}`, no marker, **no publish/no-op on the channel** (stmt1's OrderedPresent stands — contamination blocks consumption, it does not clear) | call1 L=1 R=1 S=1; call2 **loader delta 0 attributed to mode-off tail** (the loader gate evaluates mode at enrich entry — contamination is not the cause), resolver delta 0 attributed to contamination; total L=1 R=1 S=1; assert `Metadata.Server`/`Metadata.ServerAlter` snapshots pre/post both calls; public assertions cover the two ALTER records only — the failure record's own diagnostic output and any cross-call aggregate are N/A |
| AM-8.2 | single batch (mode observed): `GARBAGE(; ALTER SERVER s1 OPTIONS (USER 'u')` → records [failure record#1, stmt#2 ALTER] | `s1` present | loader gate runs at enrich entry **before** the failure record is processed → L=1 (the audited-shape ALTER still triggers the gate); record#1 sets `contaminated`; stmt#2 A1 → R1, owed `{batch_effects}`, zero reads | total L=1 R=0 S=0; statement numbering: failure=record#1, ALTER=stmt#2 |

## Group 9 — version / preflight / provider-error termination positions

| id | input | fixture | expected | observation |
|---|---|---|---|---|
| AM-9.1a | `ALTER SERVER s OPTIONS (USER 'u')`, no `--target-version`, no provider (providerless CLI request) | providerless | `unaudited` + `gate:"version"`; findings=0, gaps=0; statement and aggregate coverage `incomplete`; verdict `review`; rc=1; **identical to the frozen probe-2 expectation below** — same fixed providerless configuration, same exact result | L=0 R=0 S=0 |
| AM-9.1b | `ALTER SERVER s1 OPTIONS (USER 'u')`, no `--target-version`, provider present but observed version out-of-domain (`5.7.44`) | `s1` present | `unaudited` + `gate:"version"`; loader never fires; **catalog read still fires** — decided keep-boundary-bind (README §E): `bindMySQLServer` has no `Unsupported` gate, the gated ALTER walks the six-step order; the answer is never consumed | L=0 R=1 S=1 |
| AM-9.2 | `ALTER SERVER s OPTIONS (USER 'u')`, `--target-version 8.0.x` (malformed) | any | typed `ErrInvalidTargetVersion` preflight, rc 2; zero audit reads | preflight precedes enrich — L=0 R=0 S=0 |
| AM-9.3a | `ALTER SERVER s1 OPTIONS (USER 'u')`, `--target-version 5.7.44` + observed version `5.7.44` (matching — no mismatch possible) | provider present, observed `5.7.44` | pin parses, pin matches observed, `resolvedVersion` out-of-domain → `unaudited` + `gate:"version"`; same keep-boundary-bind read behavior as AM-9.1b | L=0 R=1 S=1 |
| AM-9.3b | `ALTER SERVER s OPTIONS (USER 'u')`, dialect `tidb`, no pin, providerless, mode off | providerless | vendor boundary — `unaudited`, product not supported on this dialect (TiDB grammar lacks the family); never reaches the MySQL resolver, no version observation consumed | L=0 R=0 S=0 |
| AM-9.3c | `ALTER SERVER s1 OPTIONS (USER 'u')`, no pin, provider observed in-domain version `8.0.36` | provider present, observed consumable | `resolvedVersion` = observed → G2 passes → **audited**: lands A3 → R9; the pin is not the gate | L=1 R=1 S=1 |
| AM-9.3d | `ALTER SERVER s OPTIONS (USER 'u')`, `--dialect mysql` + observed product `postgres` | provider observed | typed `ErrDialectProductMismatch`, early error before any arm | audit-layer L=0 R=0 S=0 — the version observation itself is not billed into L/R/S |
| AM-9.3e | `ALTER SERVER s OPTIONS (USER 'u')`, `--target-version 8.0.36` + observed version `8.4.0` | provider observed | typed `ErrTargetVersionMismatch`, early error | audit-layer L=0 R=0 S=0 |
| AM-9.4 | catalog `error` on audited `ALTER SERVER s1 OPTIONS (USER 'u')` | `s1` fixture; mode observed consumable; sentinel injected at the **`ResolveObject` forward seam** (the request is issued, the resolver raises before issuing SQL) | resolver request counted, no SQL issued: R=1, S=0; bounded error propagates and `errors.Is` reaches the sentinel; the audit exits at that statement's bind — no arm, no gap substitution; public-layer output fields N/A at this layer (the provider-SQL-seam variant, if exercised separately, would be R=1/S=1) | L=1 R=1 S=0 |
| AM-9.5 | loader error with ALTER-only request `ALTER SERVER s1 OPTIONS (USER 'u')` | `s1` fixture; observed mode; in-domain version; loader raises the sentinel at enrich entry | loader attempted once (L=1) at enrich entry before the statement loop → bounded privilege-read error (`serverPrivilegeReadError` class) terminates enrich; no statement reaches bind → R=0 S=0; public output fields N/A at this layer; error is **not** degraded to an evidence gap | L=1 R=0 S=0 |

**Registered CLI probe plan** (future execution order — PLAN /
UNEXECUTED; nothing below has been run this contract round). This
is the canonical location for the recipe; the README's §E table
cross-references these exact expectations.

Planned policy file `policy-alter.yaml` (document content only —
the file is not created or modified by this contract):

```yaml
rules:
  ddl.mysql.server.alter.notice:
    enabled: true
    level: notice
    params:
```

Planned commands (written into the future execution plan; **not run
this round**):

```bash
# 1) pinned in-domain
./deltascope audit --dialect mysql --target-version 8.0.36 \
  --config policy-alter.yaml --format json \
  --sql "ALTER SERVER s OPTIONS (USER 'u')" \
  > alter-indomain.json 2> alter-indomain.err
alter_indomain_rc=$?

# 2) no pin, no other version source
./deltascope audit --dialect mysql \
  --config policy-alter.yaml --format json \
  --sql "ALTER SERVER s OPTIONS (USER 'u')" \
  > alter-nopin.out 2> alter-nopin.err
alter_nopin_rc=$?

# 3) malformed pin
./deltascope audit --dialect mysql --target-version "8.0.x" \
  --config policy-alter.yaml --format json \
  --sql "ALTER SERVER s OPTIONS (USER 'u')" \
  > alter-badpin.out 2> alter-badpin.err
alter_badpin_rc=$?
```

Plan conditions: providerless — all DSN/provider-wiring arguments
omitted; privilege mode off — no privilege observation configured;
single clean ALTER statement; the explicit notice policy above;
stdout/stderr captured separately and each rc captured immediately;
no unconfirmed `--no-metadata` or privilege CLI flag is assumed.

Frozen expectations — valid only under the fixed configuration
above, never generalised into a pin⇒rc rule:

| probe | expected (exact) | rc |
|---|---|---|
| pin `8.0.36` | A4; owed exactly `{server.existence, server.alter_privilege}`; main finding + `connection_members` advisory = **2 notice findings**; main `outcome=unverified`, no conditional marker; statement and aggregate coverage `unverified`; verdict `review`; L/R/S = 0/0/0 | 0 |
| no pin, no other version source | `unaudited` + `gate:"version"` boundary; findings=0, gaps=0; statement and aggregate coverage `incomplete`; verdict `review`; L/R/S = 0/0/0 | 1 |
| malformed pin `8.0.x` | typed `ErrInvalidTargetVersion` preflight error; no audit result produced; audit-layer L/R/S = 0/0/0 | 2 |

## Group 10 — public output redaction, metadata immutability, unrelated objects

| id | SQL | fixture | expected |
|---|---|---|---|
| AM-10.1 | any audited ALTER | — | `raw_sql`/`normalized_sql` = `ALTER SERVER <redacted>`; finding carries no name, option keys/values, account, grant text; unsupported boundary entries carry the fixed skeleton |
| AM-10.2 | `ALTER SERVER s1 OPTIONS (USER 'u'); DROP SERVER s1` | `s1` present, consumable | snapshot plan: `Metadata.Server` **and** `Metadata.ServerAlter` checked for stmt1 — `Owed`, `PublishPresent`, `ConditionalSource`, `ConditionalAudit` — before apply, after own apply, and after stmt2's apply; bound state never mutates retroactively; stmt2 consumes stmt1's published `OrderedPresent` |
| AM-10.3 | `ALTER SERVER s1 OPTIONS (USER 'u'); CREATE TABLE t(a int)` | `s1` present; mode observed consumable → stmt1 A3 → R9; table `t` per its own catalog answer (zero) | enrich-apply layer; public fields N/A. stmt1: snapshot table/schema state **immediately before vs after stmt1's own apply** → unchanged (audited ALTER touches only the server channel; `OrderedPresent` on `s1` published). stmt2 `CREATE TABLE` has its own model effect (table `t` entry per the accepted table contract — its own resolver/catalog fixture, counted separately and out of this row's L/R/S which are server-channel counts only). After stmt2: server channel still holds exactly the `s1` entry — server state never spreads to table/schema state. L=1 R=1 S=1 (stmt1 only; stmt2 contributes no server-channel reads) |
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
