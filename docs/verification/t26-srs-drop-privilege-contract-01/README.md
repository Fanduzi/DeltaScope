# T26-SRS-DROP-PRIVILEGE-CONTRACT-01 — R1 candidate (docs-only contract)

Candidate contract for the `srs.drop.privilege` premise of the audited
MySQL `DROP SPATIAL REFERENCE SYSTEM [IF EXISTS] <uint32>` slice
(issue #105, T26). This round is documentation only: it defines when an
observed account's privilege evidence may independently discharge the
premise `P = srs.drop.privilege` for one statement, and freezes the
bounded public projection of that consumption. **No product code, test
code, policy, registry, corpus, or golden change is made or implied;
every execution item below is PLAN / UNEXECUTED.**

## Identity

| item | identity |
| --- | --- |
| baseline commit | `0c6004baf3d5104c0f261c4b7a63cf8ebf6f53b4` |
| baseline tree | `8faf5d24bd2681810ace55cc3b7b4445544400cc` |
| baseline parent | `128d459520c17660efce37e867fa7b779a9e55e3` |
| semantic base | `d68f4ab7` SRS DROP contract R10 + accepted parser type-subtree supplement + SRS DROP execution accepted at `128d4595` (archive `t26-srs-drop-execution-r5.tar.gz`, SHA-256 `31de65e4…4530c`) |
| closeout increment | `128d4595..0c6004ba` — two files, +3/−2, docs-only (`docs/decisions/2026-11-17-mysql-drop-srs-audited-slice.md`, `docs/verification/t26-srs-drop-execution-01/README.md`); no `.go` change |
| this round | `docs/verification/t26-srs-drop-privilege-contract-01/{README.md,case-matrix.md}` — the only new paths |

Contract acceptance is not implementation authorization. Activation of
premise consumption, the conditional public marker, and every
PLAN/UNEXECUTED row in `case-matrix.md` requires a separate execution
work order.

## 1. Baseline behavior (verified, not proposed)

All citations below bind baseline `0c6004ba`; verbatim excerpts are in
`baseline-source-excerpts.txt`.

- `P = srs.drop.privilege` is an owed-fact token registered as
  `spec.SRSFactDropPrivilege` (`internal/domain/spec/srs_drop.go:29-32`).
  It is owed on the S2 arm today on every qualified input.
- `spec.SRSDropPrivilege` exists with exactly one reachable value,
  `SRSDropPrivilegeUnproven` (`srs_drop.go:35-46`); the shared
  adjudication `spec.SRSDropAdjudication.Privilege` is already a field
  (`srs_drop.go:78-87`) and `AdjudicateSRSDrop` hard-codes it
  (`srs_drop.go:128-129`).
- `AdjudicateSRSDrop(state, srid, depsStale)` walks first-match
  B → S0 → S1 → S2 (`srs_drop.go:128-155`): B owes `[srs.batch_effects]`,
  S0/S1 owe nothing, S2 owes `srs.existence` (only while unproven) then
  `srs.column_dependencies`, `srs.drop.privilege`.
- Adjudication attaches at
  `internal/application/audit/metadata.go:715-721` for
  `Kind==SRSKindDrop && Unsupported==nil` statements after
  `bindMySQLSRS` (`internal/application/audit/batch_state_server_srs.go:129-166`).
- Public projection: `ddl.mysql.srs.drop.srid0` (S0 blocker),
  `ddl.mysql.srs.drop.dependency` (S1 blocker + `unknown_srs_state` gap
  carrier publishing `adjudication.Owed` verbatim on B/S2),
  `ddl.mysql.srs.drop.reserved` (notice on B/S2 when
  `SRSDropInReservedRange`, metadata exactly `{srid}`)
  (`internal/domain/rule/ddl/mysql_srs_rules.go`; rule IDs at
  `internal/domain/rule/ddl/common.go:137-139`; registered enabled at
  `configs/deltascope.example.yaml:1187-1197`). `rule.EvidenceGap` has
  only `ReasonCode` and `RequiredFacts` — no metadata carrier.
- SRS audited domain `MySQLSRSDropAuditedDomain` = resolved MySQL
  8.0.x, `ValidatedRange`, patch ≥ 4 (`srs_drop.go:89-101`); enforced by
  the phase-2 version gate at `metadata.go:274-287`, which mints
  `Unsupported{boundary:"unaudited",object:"spatial_reference_system",gate:"version"}`
  **before** the privilege loader gate (`metadata.go:341`) evaluates.

## 2. MySQL 8.0 authority

Official documentation, MySQL 8.0 Reference Manual §15.1.31 "DROP
SPATIAL REFERENCE SYSTEM Statement":
`https://dev.mysql.com/doc/refman/8.0/en/drop-spatial-reference-system.html`,
accessed 2026-11-12. The page states:

> This statement removes a spatial reference system (SRS) definition
> from the data dictionary. **It requires the `SUPER` privilege.**
> (`privileges-provided.html#priv_super`)

The page also documents: `srid` is a 32-bit unsigned integer; absent
SRID errors unless `IF EXISTS` (then a warning); a used SRID refuses
with error 3716 (`SR005`). The registered positive-witness slice for
this contract is therefore **account-bound direct-global `SUPER`** —
the documented grant — plus the registered `ALL PRIVILEGES` static-set
proof (§3.2). Nothing else counts: no role closure, no dynamic
privileges, no schema-scoped grants, no grant grammar extension.

## 3. Premise states, producer, and consumer ownership

### 3.1 Frozen state table

| state | inputs | producer | consumer scope | public impact |
| --- | --- | --- | --- | --- |
| `unproven` | any of: mode not selected; loader absent on provider; observation nil; empty `Account` (principal unbound); resolved version outside the witness domain; grant lines without exact-token `SUPER`/`ALL PRIVILEGES` on `*.*`; loader read issued but returned no positive witness | request-level, derived once at load | premise read by the SRS adjudication of every audited drop statement | `P` stays owed on S2; no conditional marker anywhere |
| `consumable` | all of: a loaded observation; bound account; in-domain version; positive `*.*` witness; the statement's premise projected while the positional scope is still open | the statement's own premise projection over the request witness | only this statement's adjudication | S2 owed set drops `P` → `[D]` or `[E,D]`; the SRS privilege-premise notice fires (§5) |
| `suppressed` (suppressed-by-scope) | a positive request witness exists, but the positional scope terminator minted `serverPrivilegeScopeEnded` before this statement | per-statement projection over the same witness + scope flag | later statements only; completed statements keep their snapshot | identical public shape to `unproven` on this statement (`P` owed, no marker); prefix statements are unaffected |

`suppressed` is the `ServerDropPrivilegeSuppressed` equivalent already
registered for the SERVER family (`internal/domain/spec/server_drop_premise.go:77-80`).
The execution phase adds `SRSDropPrivilegeConsumable` and
`SRSDropPrivilegeSuppressed` constants to the existing
`spec.SRSDropPrivilege` enum — it does not replace it.

### 3.2 Producer chain (actual, traced)

1. Request mode — `request.PrivilegeAuditMode` (fallback
   `request.Metadata.PrivilegeAuditMode`) is validated against the
   closed vocabulary `{"", "observed_account_new_session"}` at
   `internal/application/audit/service.go:115-123`; any other value is
   the typed preflight error `ErrInvalidPrivilegeMode`
   (`service.go:37-40`), returned before policy load, parsing, or any
   provider read — a zero-precondition-read exit.
2. Version resolution — the resolved `*spec.VersionIdentity` reaches
   `enrichStatements` (`metadata.go:232-234`) where the phase-2 gate
   marks out-of-domain statements `Unsupported` first
   (`metadata.go:248-289`, SRS branch `274-287`).
3. Observation gate — `metadata.go:341-352`: `dialect==MySQL &&
   request.PrivilegeAuditMode==observed_account_new_session &&
   statementsNeedMySQLServerPrivilege(statements)` selects the loader.
   The loader runs **once, before the per-statement loop**
   (`metadata.go:397`), so loading precedes every arm selection; a
   final B/S0/S1 arm never implies `L=0`.
4. Raw observation — `MySQLServerDropPrivilegeLoader`
   (`metadata.go:65-67`) →
   `mysqlmeta.Provider.LoadMySQLServerDropPrivilege`
   (`internal/infrastructure/metadata/mysql/provider_server_drop_privilege.go:30-71`):
   one dedicated `*sql.Conn` reads `select current_user()` (rowset —
   empty answer is distinct from a failed query) then
   `show grants for current_user()` via `loadMySQLGrantLines`
   (`provider_resource_group_facts.go:218`). Every connect/query/scan/
   iterate/close error wraps and propagates; non-MySQL answers
   `(nil, nil)`.
5. Immutable storage — the result lands on
   `batchState.serverDropPrivilege` (`batch_state.go:171-175`) as
   request-level raw evidence (`Account`, `GrantLines`, both
   `json:"-"`), and the once-derived verdict
   `spec.MySQLServerDropPrivilegeWitness(observation, resolvedVersion)`
   lands on `serverDropPrivilegeWitness` (`batch_state.go:176-182`;
   compute at `metadata.go:350`).
6. Witness predicate — `MySQLServerDropPrivilegeWitness`
   (`server_drop_premise.go:240-247`): non-empty bound account +
   `MySQLServerDropPrivilegeDomain` (resolved MySQL 8.0.x,
   `server_drop_premise.go:218-222`) + `SUPER` — or `ALL PRIVILEGES`
   under `MySQLServerDropAllImpliesSuper` (`server_drop_premise.go:228-230`)
   — in the exact-token `*.*` set folded by
   `CollectMySQLGlobalPrivileges` (`internal/domain/spec/mysql_grants.go:19-32`;
   regex `^GRANT\s+(.+?)\s+ON\s+(\S+)\s+TO\s+`, scope must equal `*.*`,
   comma-split upper-cased exact tokens; role-assignment lines have no
   `ON` clause and never match).

**`ALL PRIVILEGES` reuse basis**: the parser path is
`CollectMySQLGlobalPrivileges` + the version-checked constant
`MySQLServerDropAllImpliesSuper`, a registered static-set proof that on
in-domain MySQL 8.0.x the documented global `ALL PRIVILEGES` list
contains `SUPER` — not a substring heuristic. The SRS audited domain
(8.0.4+) is a strict subset of the proof domain (8.0.x), so the proof
remains valid wherever an SRS premise can be consumed. This contract
reuses the proof unchanged; it does not expand it.

### 3.3 Independent SRS derivation (no borrowing)

The execution phase adds one projection on `batchState` — proposed
name `srsDropPrivilegePremise()` — shaped exactly like the SERVER
sibling (`server_drop_premise.go:69-77`):

```
if !s.serverDropPrivilegeWitness → unproven
if s.serverPrivilegeScopeEnded   → suppressed
                                 → consumable
```

The SRS premise reads only the shared request-level raw evidence and
the shared scope flag. It must never read: a SERVER statement's
`ServerDropAdjudication`/`Privilege`, the SERVER finding's
`conditional_audit` marker, any SERVER arm outcome, the SRS catalog
existence/dependency answer (catalog success is not privilege
evidence), or another SRS statement's premise/adjudication. Shared
request-level immutable raw observation is the approved seam; per-family
premise projections are independent derivations over it — the same
separation the SERVER drop/create/alter siblings already keep
(`server_drop_premise.go:89-128`).

Because the SRS premise only attaches inside the audited slice — the
version gate already guarantees `MySQLSRSDropAuditedDomain
(resolvedVersion)` — and the witness verdict already binds the wider
8.0.x domain, no second version check is needed at premise time; the
contract records that the narrower 8.0.4+ domain is implied by slice
membership, not re-queried.

### 3.4 Extension points (minimal, identified — not implemented)

| point | file (baseline) | change |
| --- | --- | --- |
| premise enum | `internal/domain/spec/srs_drop.go:39-46` | add `SRSDropPrivilegeConsumable`, `SRSDropPrivilegeSuppressed` |
| adjudication signature | `srs_drop.go:128` | accept the statement's `SRSDropPrivilege` premise (mirrors `AdjudicateServerDrop`'s parameter at `server_drop_premise.go:147`) |
| premise projection | new sibling of `internal/application/audit/server_drop_premise.go` | `(*batchState).srsDropPrivilegePremise()` |
| loader qualification | `server_drop_premise.go:26-37` (`statementsNeedMySQLServerPrivilege`) | extend coverage to audited SRS drops (`DDL.SpatialReference != nil && Kind==SRSKindDrop && Unsupported==nil`), e.g. a composed `statementsNeedMySQLPrivilege` gate — qualification runs after the version gate, so `Unsupported==nil` already encodes version failure |
| adjudication attach | `metadata.go:715-721` | pass `state.srsDropPrivilegePremise()` into `AdjudicateSRSDrop` |
| public notice | new rule beside `internal/domain/rule/ddl/mysql_srs_rules.go` | §5 carrier + registry/config row |

No grant grammar, role closure, dynamic-privilege inference, or general
privilege-calculation framework is added.

## 4. Frozen adjudication and owed-fact table

Priority order stays `B → S0 → S1 → S2` exactly. `P` consumption is an
owed-set difference on the S2 arm only — it never changes an arm, never
clears stale state, never suppresses reads, and never rewrites a
completed statement's snapshot.

| arm | condition | owed (exact order) | P consumed |
| --- | --- | --- | --- |
| B | `UnreadReason == MetadataUnreadBatchEffects` (contamination or same-SRID `srsExistenceStale`) | `[srs.batch_effects]` | no |
| S0 | authored SRID 0 (plain or `IF EXISTS`) | `[]` | no |
| S1 | valid positive dependency witness, clean channel, `!depsStale` | `[]` | no |
| S2 | presence proven, P unavailable (unproven or suppressed) | `[srs.column_dependencies, srs.drop.privilege]` | no |
| S2 | presence unknown, P unavailable | `[srs.existence, srs.column_dependencies, srs.drop.privilege]` | no |
| S2 | presence proven, P consumable | `[srs.column_dependencies]` | yes |
| S2 | presence unknown, P consumable | `[srs.existence, srs.column_dependencies]` | yes |

`RequiredFacts` order is verbatim `adjudication.Owed` — the dependency
rule publishes it unchanged (`mysql_srs_rules.go:141-148`). Consumption
of `P` does not erase or alter stale/unknown existence or dependency
evidence (`srs.existence`/`srs.column_dependencies` stay owed exactly as
before), does not skip the SRS existence/dependency reads (the
adjudication still consumes `Metadata.SpatialReference` bound by
`bindMySQLSRS`), does not clear `srsExistenceStale`/`srsDepsStale`, and
does not alter `applySRSStatement` (`batch_state_srs.go:24-34` — the S2
arm still mints `srsExistenceStale[srid]` for later statements).

## 5. Selected conditional public projection

**Selected carrier: a dedicated SRS privilege-premise notice rule.**
Rationale: `rule.EvidenceGap` has no metadata carrier (only
`ReasonCode`/`RequiredFacts`), so the marker cannot ride the gap; and
`ddl.mysql.srs.drop.reserved` must stay membership-only
(`{srid}`-only metadata, suppressed on refusal arms), so it cannot
absorb the marker either.

Frozen public contract (all PLAN / UNEXECUTED):

| property | frozen value |
| --- | --- |
| RuleID | `ddl.mysql.srs.drop.privilege` |
| emission condition | audited SRS drop statement (`srsDropStatement`) AND `adjudication != nil` AND `adjudication.Arm == spec.SRSDropArmUnproven` (S2) AND `adjudication.Privilege == spec.SRSDropPrivilegeConsumable` — only the arm that actually consumed `P` |
| level | `notice` default via `configuredLevel(cfg, rule.LevelNotice)` — configurable like sibling rules |
| default enabled | yes; registered row beside the three existing SRS rules in `configs/deltascope.example.yaml` and the defaults registry |
| fixed text | `spatial reference system <srid> drop privilege premise discharged under the observed_account_new_session conditional audit` |
| metadata | exactly two keys: `"srid"` = `uint32` authored SRID; `"conditional_audit"` = the Go string `"observed_account_new_session"` |
| JSON path | `$.statements[i].findings[j].metadata.conditional_audit`; the key is present only on consuming statements — absent, `null`, non-string, `""`, or any other token never equals the marker |
| findings per SRS statement | S2+consumed+non-reserved → exactly 1 (this notice); S2+consumed+reserved → exactly 2 (notice + reserved advisory); S2+not-consumed+reserved → exactly 1 (reserved); S2+not-consumed+non-reserved → 0; B/S0/S1 → unchanged by this rule |
| gaps per SRS statement | exactly 1 on B/S2 via `ddl.mysql.srs.drop.dependency`, reason `unknown_srs_state`, `RequiredFacts` = owed order above |
| coverage / verdict under consumption | S2 stays unverified → statement coverage `unverified`, aggregate verdict `review` — `P` consumption never upgrades coverage or claims success |

Three distinct predicates stay distinct and each appears in the matrix:
(i) the request carries a legal observation; (ii) this statement's
premise is consumable; (iii) this arm actually consumed `P`. Only (iii)
emits the marker.

Unique projections, frozen:

- reserved vs non-reserved differ by the reserved advisory's presence;
- `P` consumed vs not consumed differ by (a) the notice's presence and
  (b) the owed set (`[D]`/`[E,D]` vs `[D,P]`/`[E,D,P]`).

The notice is never a DROP-success proof and never a
catalog-completeness proof — the S2 arm still owes `D` (and `E` when
applicable), coverage stays `unverified`, and the text states only that
the premise was discharged under the declared conditional scope.

**Rule-disabled / level-rewritten behavior**: disabling
`ddl.mysql.srs.drop.privilege` suppresses only the notice — the
adjudication's premise verdict and the reduced owed set are spec truth,
not output policy; the gap's `RequiredFacts` still show `P` discharged.
A level rewrite changes only the finding level. Premise validity never
silently tracks rule config.

**Conditionality cannot be represented**: `P` consumption is decided by
the premise, not by the marker — if the notice rule were disabled the
consumption still stands and `RequiredFacts` exposes it. There is no
fallback that mutates the reserved advisory or the gap.

**Suppression preservation**: S0/S1 keep advisory suppression; the
notice emits on no refusal arm and never on B.

**Leakage**: the notice's metadata carries `srid` and the literal
marker only — no account, grant text, privilege payload, internal SQL,
or session data; both raw fields are `json:"-"` (`server_drop_premise.go:52-62`).
Scan plan: (a) match the legal key/value exactly; (b) separately scan
the serialized statement for account/grant-shaped strings — both scans
are PLAN matrix rows.

## 6. Request loading order, scope lifetime, mixed-family ownership

### 6.1 Actual order (bound to source)

`service.go` request validation (incl. `ErrInvalidPrivilegeMode`)
→ parse → `enrichOrderedStatements`/`enrichStatements` (`metadata.go:219/232`)
→ phase-2 version gates (`metadata.go:248-289`)
→ placement/catalog request loads
→ **privilege observation gate + load** (`metadata.go:341-352`)
→ resolver cache wiring (`metadata.go:355-394`)
→ per-statement loop (`metadata.go:397-733`): contamination check
(421-426) → scope-terminator mint (433-435) → family bind incl.
`bindMySQLSRS` (699-709) → shared adjudication (715-721) →
`afterBindObserve` → `apply` (727) → `afterApplyObserve`
→ rule projection/public report (outside this loop).

Loading therefore precedes per-statement arm selection — recorded
explicitly: `L` (loader invocations, 0 or 1 per request) must never be
inferred `0` from a B/S0/S1 outcome.

### 6.2 SRS-only observed-mode entry

Baseline: an SRS-only request never triggers the gate
(`statementsNeedMySQLServerPrivilege` is SERVER-family-only), so `L=0`
and every premise stays `unproven`. Proposed increment: extend the
qualification to cover audited SRS drops so an SRS-only
`observed_account_new_session` request enters the same single-load path;
the predicate runs after version gating, so only statements that would
be adjudicated qualify the read.

### 6.3 Scope termination and lifetime

Positional mechanism (existing): inside the statement loop,
`serverPrivilegeScopeTerminator(statement)` (metadata.go:433;
`server_drop_premise.go:49-63`) mints `serverPrivilegeScopeEnded`
before the current statement's bind — `DDLOperationGrant` /
`DDLOperationRevoke` plus boundary features
`{set_role, set_default_role, grant_role, revoke_role, grant_proxy}`.
After termination: every later statement's premise projects
`suppressed`; an authored later `GRANT` does **not** restore a new
positive observation (no un-end path — the flag is monotonic within the
request); completed statements keep their already-attached
premise/adjudication snapshots — nothing is rewritten retroactively.

Required scope witness (matrix row P7):

```sql
DROP SPATIAL REFERENCE SYSTEM 100001;
REVOKE SELECT ON *.* FROM 'r1'@'%';
DROP SPATIAL REFERENCE SYSTEM 100002;
```

Two distinct SRIDs isolate privilege-scope suppression from the
same-SRID existence-stale barrier (a same-SRID repeat lands on B via
`srsExistenceStale`, which would mask the premise question — it is kept
only as a separate barrier regression row).

### 6.4 Mixed SERVER/SRS requests

One raw observation load (`L=1`) serves both families; each derives its
own premise projection; each emits its own family marker
(`conditional_audit` on the SERVER notice shape vs the §5 SRS notice).
Neither family reads the other's adjudication, marker, or outcome —
shared ownership lives only at the request-level raw evidence.

## 7. Reads and error semantics

Reads vocabulary (unchanged from the accepted execution round):

| counter | meaning | source seam |
| --- | --- | --- |
| `R` | SRS resolver forwards (`mysqlSRSResolver` → cachedObjectResolver → provider `ResolveObject`) | `metadata.go:393`, `batch_state_server_srs.go:151-161` |
| `Qexist` | `information_schema.st_spatial_reference_systems` existence query | `provider_server_srs.go` |
| `Qdeps` | `information_schema.st_geometry_columns` dependency query | `provider_server_srs.go:262` |
| `Qo` | loader-internal `select version()` | provider version channel |
| `L` | privilege-observation loader invocations (0 or 1 per request) | `metadata.go:341-352` |
| `Vd` | request `LoadInstanceFacts` method invocations | service/request layer |

Sub-counts are available on demand: the loader issues exactly two
statements — `select current_user()` then `show grants for
current_user()` (`provider_server_drop_privilege.go:46-65`) — so a
per-statement split is recordable without new seams. Failed calls count
as attempts. `P` consumption adds **zero** new catalog reads: the
premise is projection over already-loaded evidence, and the SRS
existence/dependency reads still issue exactly as today (`P` is never a
read gate). `Vd` and `Qo` counts are unchanged by this contract.

Privilege read error: a failed `LoadMySQLServerDropPrivilege` wraps in
`serverPrivilegeReadError` ("server drop privilege observation
failed", `Unwrap` preserved, `server_drop_premise.go:141-154`) and
returns from `enrichStatements` (`metadata.go:345-348`) — processing
terminates; no bind, adjudication, or apply is entered for any
statement; the bounded public error reaches the caller with the
`errors.Is` cause intact. A real issued-read error is **never**
downgraded into an ordinary `P` evidence gap. Distinct states the
matrix keeps separate: mode off / loader absent → `unproven` (no
error); loader present but observation fails → typed error exit;
observation ok, no positive witness → `unproven`; witness ok, scope
ended → `suppressed`.

The `serverPrivilegeReadError` public text names the SERVER family
because the shared observation was first consumed there; for an
SRS-only request the same registered error chain applies (the
observation is request-level). A family-neutral rename is logged as a
bounded deferral (§9), not silently changed here — the existing SERVER
contract froze this text.

Boundary catalog-bind behavior is preserved exactly: B-arm unread
markers and read suppression follow `bindMySQLSRS`'s existing order
(`batch_state_server_srs.go:129-166`); this contract claims no "zero
reads" for any boundary — reads follow the actual resolver path.

## 8. Differences from previous scope

| # | delta vs baseline `0c6004ba` |
| --- | --- |
| D1 | `srs.drop.privilege` becomes independently consumable on the S2 arm for account-bound direct-global `SUPER` (+ registered `ALL` static-set proof); baseline owed `[D,P]`/`[E,D,P]` may become `[D]`/`[E,D]` |
| D2 | loader qualification extended to SRS-only observed-mode requests (baseline: SERVER family only) |
| D3 | new notice rule `ddl.mysql.srs.drop.privilege` carrying `{srid, conditional_audit}` on consuming S2 statements |
| D4 | `spec.SRSDropPrivilege` gains `Consumable`/`Suppressed`; `AdjudicateSRSDrop` gains a premise parameter; SRS premise projection added |

Everything else — arm priority, S0/S1 suppression, reserved-advisory
membership, read order/counts, stale-flag mechanics, error chain —
stays the baseline contract verbatim. No already-accepted document is
rewritten.

## 9. Execution wiring checklist (forward, PLAN / UNEXECUTED)

1. `spec`: extend `SRSDropPrivilege` with `Consumable`/`Suppressed`;
   change `AdjudicateSRSDrop` to take the premise and fold it into the
   S2 owed set per §4; keep arm/classification untouched.
2. `audit`: add `srsDropPrivilegePremise()`; extend the loader gate
   predicate for audited SRS drops; pass the premise at the
   `metadata.go:719` attach site.
3. `rule/ddl`: add `ddl.mysql.srs.drop.privilege` (§5 freeze); register
   in `common.go`, defaults registry, and `configs/deltascope.example.yaml`.
4. Matrix P1–P10 executed at the layers each row states (request/API
   where the row says so — no enrich-only substitution for API rows).
5. Serialization/leakage scans: legal key/value match plus
   sensitive-value scan on consuming statements.
6. Module READMEs + decision record updated with the execution change.

## 10. Deferral table

| deferred item | why | own prerequisites (non-circular) |
| --- | --- | --- |
| dependency completeness (proven-complete `st_geometry_columns` visibility) | no approved completeness contract exists; a zero-row answer stays unproven | a registered completeness proof + its own contract round |
| proven-absent catalog evidence | a zero-row existence answer is "read, unknown" — privilege-filtered visibility forbids proven-absent | a completeness contract for the SRS catalog |
| successful / conditional DROP arm | this contract only discharges `P` on the unproven arm; a `conditional_drop`-style SRS arm would need existence + dependency-completeness + post-state modeling of its own | deferred prerequisites above plus a dedicated arm contract — does not depend on this contract's acceptance |
| ordered publication / downstream consumers | no SRS ordered channel entries exist yet (family has no OrderedPresent/Absent equivalents this round) | an ordered-channel contract for the SRS family |
| family-neutral privilege-error rename | existing frozen SERVER public text; cosmetic only | separate bounded decision |
| grant grammar, role closure, dynamic privileges, non-`*.*` scopes | outside the registered positive-witness slice | each is its own contract if ever needed |
| `rename_user` as scope terminator | already excluded from the registered terminator list by the SERVER contract | re-evaluation only under a new contract |

## 11. Validation performed for this contract

Static only: `git` identity/status/diff checks, source reading of every
cited symbol at baseline `0c6004ba`, official MySQL 8.0 documentation
lookup. No tests, builds, vet, CLI, SQL/native, Docker, or golden
execution ran. Raw captures live in `static-checks.txt`;
`baseline-source-excerpts.txt` carries the verbatim decisive chain.
