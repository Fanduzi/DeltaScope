# T26-SRS-DROP-PROVEN-ABSENT-CONTRACT-01 — R2 revision (docs-only contract)

Candidate contract for promoting a clean zero-row
`ST_SPATIAL_REFERENCE_SYSTEMS` answer to a proven-absence fact for the
audited MySQL `DROP SPATIAL REFERENCE SYSTEM [IF EXISTS] <uint32>`
slice (issue #105, T26). **Docs-only: no product code, test, policy,
registry, corpus, or golden change is made or implied. Every matrix,
test, probe, and native item below is PLAN / UNEXECUTED. Acceptance is
not execution authorization.**

## Identity

| item | identity |
| --- | --- |
| baseline commit | `56bcf05ff3c9ce087b088eeaba04093d51a97070` |
| baseline tree | `46436c9e63626b493eff8f15249aa5d29876889b` |
| R1 candidate | `aceaa48d0ef358e71aae89909cda8193aa8cd24b` (preserved, not amended) |
| this round | `docs/verification/t26-srs-drop-proven-absent-contract-01/{README.md,case-matrix.md}` — the only two paths touched |
| semantic base | SRS DROP contract `d68f4ab7`; SRS DROP execution `128d4595`; SRS privilege contract `2f7231b5`; SRS privilege execution CLOSED/R3-ACCEPTED `56bcf05f` |
| bound evidence | `t26-srs-drop-privilege-execution-r3.tar.gz` 352,624 B / `f7667e08…3393b` + supplement 26,297 B / `cdc54363…d500a1`; R1 archive `t26-srs-drop-proven-absent-contract-r1.tar.gz` 23,751 B / `bab8e289…fbd7` |

R1 was returned CHANGES REQUIRED (six groups). R2 keeps the preserved
design direction the ruling explicitly retained — independent zero-value
-safe absence carrier, plain/IF EXISTS split, independent SRS privilege
projection, S2-only privilege notice, absent arms mint no stale and
publish no OrderedAbsent — and rewrites the supply predicate, the
conflict ordering, the cache/read model, the matrix, and the evidence
records per the R2 work order.

## 1. R2-A — lawful supply of the negative fact

### A1. Three separate domains — never conflated

| domain | decided at | version source | membership rule |
| --- | --- | --- | --- |
| **D-qualify** (request/audit qualification) | version gate, `metadata.go:274-287` | request-observed version (`spec.ParseObservedVersion` of the target banner, `metadata.go:141-163`) | `MySQLSRSDropAuditedDomain`: MySQL, ValidatedRange, `Major==8 && Minor==0 && Patch>=4` (`srs_drop.go:89-101`) |
| **D-support** (catalog readability) | `mysqlSRSCatalogSupport`, `provider_server_srs.go:94-107` | provider's own `select version()` result (Qo read) at that call | banner parses to ValidatedRange MySQL, `Major>=8` |
| **D-proof** (completeness proof for negative fact) | inside `loadMySQLSRSObject`, before the existence read may mint `Absence` | the **same provider banner** `mysqlSRSCatalogSupport` already produced at that call point — NOT the request version | proven && banner version satisfies `Major==8 && Minor==0 && Patch>=4` |

Rules: `proven && supported` alone NEVER supplies absence. Absence is
supplied only inside D-proof at the provider call point, judged on the
provider's own observed banner. Outside D-proof no absence is minted —
the statement then falls back to whatever the existing admission path
decides (S2 if qualified and unknown, or boundary/unsupported when the
request gate applies). The three domains use three distinct observation
points; the request version and the provider banner are separate reads
of the same target and are never interchanged.

### A2. Official proof — decisive verbatim evidence

`official-evidence-r2.txt` (out-of-repo supplement) carries the full,
contiguous, boundary-delimited view definition plus per-source identity:

| source | immutable identity | file bytes / SHA-256 | fetch date |
| --- | --- | --- | --- |
| tag `mysql-8.0.4` | commit `577fbcf856cd693434d554545d499559bcdd3695` | 2,152 B / `8e5a631c…eb56` | 2026-11-12 |
| tag `mysql-8.0.24` | commit `3e90d07c3578e4da39dc1bce73559bbdf655c28c` | 2,184 B / `6a831b8b…5874` | 2026-11-12 |
| tag `mysql-8.0.36` | commit `49ef33f7edadef3ae04665e73d1babd40179a4f1` | 2,184 B / `426d1d0e…42333` | 2026-11-12 |
| `8.0` branch HEAD | commit `9fc8d60003b4eee32767c1e30942f222c3508c31` (resolved via `repos/mysql/mysql-server/branches/8.0`) | 2,235 B / `dd1fd25f…569e2` | 2026-11-12 |

Verbatim definition (branch-HEAD pin, whitespace-normalised, all four
pins token-identical — normalised SHA-256 `de945460…713c` on the
constructor body):

```
m_target_def.add_field(FIELD_SRS_NAME, "SRS_NAME", "name");
m_target_def.add_field(FIELD_SRS_ID, "SRS_ID", "id");
m_target_def.add_field(FIELD_ORGANIZATION, "ORGANIZATION", "organization");
m_target_def.add_field(FIELD_ORGANIZATION_COORDSYS_ID, "ORGANIZATION_COORDSYS_ID", "organization_coordsys_id");
m_target_def.add_field(FIELD_DEFINITION, "DEFINITION", "definition");
m_target_def.add_field(FIELD_DESCRIPTION, "DESCRIPTION", "description");
m_target_def.add_from("mysql.st_spatial_reference_systems");
```

No `add_where`, no access predicate — a bare projection. Byte-level
diffs between pins are mechanical only (copyright year, brace style,
namespace comments), shown verbatim in the supplement.

**Coverage-scale basis for the whole 8.0.4+ claim:** the file's commit
history on the `8.0` branch (`repos/mysql/mysql-server/commits?path=
sql/dd/impl/system_views/st_spatial_reference_systems.cc&sha=8.0`,
fetched 2026-11-12) contains exactly 11 commits: the 2017-04-22
introducing commit (WL#9495 schema tables) plus ten mechanical commits
(copyright headers ×6, GPL clause ×2, clang-format ×1, include-paths
×1, licence exception ×1). **No commit changes the view's fields,
source table, or adds a predicate since introduction** — the identical
definition therefore spans every 8.0.x release on the branch, not just
the sampled tags. This, not tag sampling alone, is the coverage basis.

Contrast: `st_geometry_columns.cc` (pin 9fc8d600) selects
`add_from("INFORMATION_SCHEMA.COLUMNS cols")` + LEFT JOIN +
`add_where(DATA_TYPE IN …)` — filtered view; dependency surface stays
witness-only. WL#9059 F-3.1 makes the I_S view the sole read channel;
Refman 28.3.36 is corroboration only.

**Ruling proposed:** inside D-proof (provider banner: MySQL,
ValidatedRange, 8.0.x, patch ≥ 4), a clean zero-row `WHERE srs_id = ?`
answer is complete absence evidence at the observation instant. The
claim now rests on the branch-wide unmodified history, bounded to the
8.0 series; nothing is asserted about 8.4+/9.x or unproven banners.

### A3. Clean-zero-row predicate — rewritten

`Row.Scan() == sql.ErrNoRows` does NOT prove a clean zero-row read.
Decisive Go 1.26.1 stdlib source (`database/sql/sql.go`):

- `(*Row).Scan` (line 3486): returns `r.err` first (pre-stored query
  error), then `if !r.rows.Next() { if err := r.rows.Err(); err != nil
  { return err }; return ErrNoRows }` — a driver that surfaces
  `sql.ErrNoRows` *as an error* returns the identical value through
  `r.err` or `rows.Err()`, indistinguishable from a genuine zero-row
  result by value comparison alone.
- `(*Rows).Next` (line 3029) / `(*Rows).Err` (line 3136): the
  structural discriminant available is `Next()==false && Err()==nil` —
  a driver-reported `ErrNoRows` lands in `Err() != nil` and never
  reaches the zero-row branch.

Registered supply predicate (explicit `*Rows` flow — design only,
UNEXECUTED):

```
rows, err := p.db.QueryContext(ctx, qExist, int64(srid))
if err != nil              { propagate; Absence never set }
defer-free close check:
for rows.Next()          { scan one row → Exists=true, Read=true }
if rows.Err() != nil     { propagate; Absence never set }
if !sawRow               { Read=true; Absence=true when in D-proof }
if err := rows.Close()   { err → propagate }
```

`Next()==false && Err()==nil && Close()==nil` is the observable
triple; no sentinel value is trusted. Planned distinguishing rows
(register now, execute later):

| # | condition | supply |
| --- | --- | --- |
| 1 | in-domain clean zero rows (`Next`=false, `Err`=nil, `Close`=nil) | `Absence` set iff in D-proof |
| 2 | `QueryContext` error that happens to equal `sql.ErrNoRows` | propagate; never Absence |
| 3 | iteration error that equals `sql.ErrNoRows` (`Err()`) | propagate; never Absence |
| 4 | generic query/read error | propagate |
| 5 | ctx cancellation mid-iteration | propagates via `Err()` |
| 6 | existence succeeded (zero or row) then dependency read fails | propagate; no snapshot delivered to bind — partial local state is never a published fact |

Existing bounded-error / `errors.Is` conventions stay; the ErrNoRows
sentinel keeps its meaning only as a value, never as a path proof. A
locally assigned struct field inside a helper is NOT a delivered fact:
only the snapshot returned to `resolveObjectSnapshot`/`bindMySQLSRS`
counts as published. Registered touchpoint for a future execution
round: `provider_server_srs.go:146-169` (QueryRowContext → explicit
Rows), plus `Row.Err()` precheck is insufficient alone because
`rows.Err()` — not `r.err` — is where iteration errors surface.

### A4. ValidAbsence invariants

Consumer-visible validity: `ValidAbsence(state)` ⇔ `state != nil &&
state.Read && !state.Exists && state.Absence` — the three-field
conjunction is the ONLY consumable shape; the bool alone is never
interpreted.

Producer contract (single in-tree producer, `loadMySQLSRSObject`):
`Absence=true` is set only at the explicit-Rows clean-zero branch,
only when D-proof holds for the provider banner at that call point,
for the exact SRID queried. Provenance is enforced producer-side; the
consumer does not re-derive it — the contract's trust boundary is that
only the in-tree provider sets the bit, and third-party resolvers that
never set it keep `Absence=false` (zero value) → unknown, which is the
only honest claim available: the consumer validates field consistency,
not provenance.

Registered invalid/conflict inputs (absence not consumable; remaining
facts continue per existing adjudication):

| input | treatment |
| --- | --- |
| `Absence && !Read` | invalid — `Absence` ignored; state reads as unknown |
| `Absence && Exists` | invalid — `Exists` is the positive fact; `Absence` ignored |
| snapshot carrying `Absence` for a different SRID | cannot exist: bind resolves by `(type,name=SRIDName())`; a foreign-SRID snapshot fails the key and is never bound |
| providerless / non-`ObjectResolver` provider | `Unavailable` snapshot; `Absence` never set |
| `!proven` banner, or banner outside D-proof | `Absence` never set even on clean zero rows |

Cache: `objectCache` is created per `enrichOrderedStatements` call
(`metadata.go:356-397`) and memoises the whole snapshot — including a
legally established `Absence` — for the rest of THAT enrich only. The
next enrich builds a fresh cache and re-issues reads; `batchState`
sharing across enriches does NOT share `objectCache`. No cross-call
persistence, no re-read repair, no new barrier. No claim is made that
the two `p.db` reads run on one physical connection or one consistency
snapshot — they are separate observations at separate instants.

## 2. R2-B — frozen first-match table: B → S0 → S1 → A → S2

Frozen order (per ruling): **B → S0 → S1 (valid, non-stale positive
dependency witness) → A (valid absence) → S2**. A positive, un-demoted
dependency witness always wins over absence — the conservative
refusal. A `srsDepsStale`-demoted witness is not a valid positive and
does not reach S1 (it enters adjudication as the B arm's debt only).

| arm | entry predicate | outcome | owed | P consumed | conditional marker | own-apply mint |
|---|---|---|---|---|---|---|
| B | `UnreadReason==batch_effects` | blocked | `[srs.batch_effects]` | no | none | none |
| S0 | `srid==0` | refused_srid0 blocker | `[]` | no | none | none |
| S1 | valid positive deps witness (non-stale channel, clean read) | refused_dependency blocker | `[]` | no | none | none |
| A-plain | `ValidAbsence && !IfExists` | refused_absent blocker | `[]` | no | none | none |
| A-IE✓ | `ValidAbsence && IfExists && P==consumable` | absent_noop notice | `[]` | **yes** | `conditional_audit="observed_account_new_session"` | none |
| A-IE? | `ValidAbsence && IfExists && P!=consumable` | absent_conditional | `[srs.drop.privilege]` | no | none | none |
| S2 | rest | unproven | per matrix | on consumable | `conditional_audit` | mint `srsExistenceStale[srid]` |

- **B1.** `refused_absent` is THIS audit model's refusal basis
  ("the authored SRID is proven absent"); no claim is made about which
  native error the server returns first when P is unproven — the
  ER_SRS_NOT_FOUND/need-super ordering is outside the bounded
  commitment (WL#9059 F-2.x listed as corroboration, observation scope
  = documentation, not a probed server).
- **B2.** Within the A branch only, absence is the complete answer to
  D: the DD invariant (a column cannot reference a nonexistent SRID)
  means the dependency debt is discharged. This is a debt adjudication
  inside the absent arm — NOT a general dependency-completeness
  carrier, and it proves nothing about present objects.
- **B3 conflict rows** (distinguishing, all PLAN/UNEXECUTED): absence +
  valid positive witness + plain DROP → **S1 refused_dependency**;
  absence + valid positive witness + IF EXISTS + consumable → **S1
  refused_dependency** (R1 wrongly routed to absent_noop); demoted
  (depsStale) witness + absence → A arms (the demoted witness is not
  valid). A real provider-layer conflict IS reachable: existence and
  dependency are two separate `p.db` observations without a shared
  snapshot — a concurrent DDL between the two reads, or a
  privilege-filtered dependency view that shows a row the existence
  scan missed, produces "existence zero + positive deps" at the real
  read layer; at least one such planned row uses the real provider
  path (matrix PA-4).
- **B4.** `ddl.mysql.srs.drop.privilege` stays S2-only; the A-IE✓
  marker lives on its own `absent_noop` finding. SRS keeps its
  independent premise projection.
- **B5.** `observed_account_new_session` semantics unchanged;
  absent_noop claims only "warning + no dictionary write under that
  observed condition", never execution success.

## 3. R2-C — cache, read vector, state (corrected)

- **C1.** `objectCache` scope is one `enrichOrderedStatements` call.
  Same-enrich second bind of the same SRID → cache hit → zero
  additional R/Qexist/Qdeps/Qo on that bind. The NEXT enrich call
  builds a new cache → fresh reads; `batchState` sharing ≠ cache
  sharing. A legally established `Absence` persists for that enrich
  only.
- **C2.** B-arm short-circuit (`MetadataUnreadBatchEffects`) returns
  before the resolver → a real zero-catalog-read result; the L loader
  gate evaluates independently of the final arm and never back-infers
  from it.
- **C3.** Read deltas are specified per-statement AND per-call:
  R (resolver forwards), Qexist/Qdeps (catalog queries), Qo
  (`select version()`), L (privilege loader calls); request layer
  also counts Vd (version discovery); internal layer Vd=N/A.
- **C4.** Absent arms mint/clear nothing: `srsExistenceStale` and
  `srsDepsStale` unchanged; unrelated SRID/SERVER/table/schema
  untouched; no OrderedAbsent publication. State-preservation rows
  carry non-empty unrelated stale sets and `depsStale=true` controls —
  empty/false alone proves nothing.
- **C5.** Double-enrich plan row uses a DIFFERENT second catalog answer
  (first call: existence zero + clean deps → absent; second call:
  existence row present) so the new-cache re-read is a distinguishable
  observable — not just asserted.

## 4. R2-E — canonical public projection (clean, non-reserved, single
SRS statement, defaults enabled)

| arm | findings | gaps | stmt/agg coverage | verdict | conditional_audit |
| --- | --- | --- | --- | --- | --- |
| A-plain | 1 × `ddl.mysql.srs.drop.absent` blocker | 0 | complete | reject | absent |
| A-IE✓ | 1 × `ddl.mysql.srs.drop.absent_noop` notice | 0 | complete | pass | `="observed_account_new_session"` |
| A-IE? | 0 | 1 × owner `ddl.mysql.srs.drop.dependency`, `reason_code="unproven_privilege"`, `required_facts=[srs.drop.privilege]` ordered | unverified | review | absent |

Frozen serialization (to be asserted exactly):

- `absent` finding: `RuleID`, `Level=blocker`, message
  `spatial reference system {srid} does not exist`, metadata keys
  exactly `{srid, outcome}` — `srid` emitted as JSON number (in-memory
  `uint32`), `outcome="refused_absent"`.
- `absent_noop` finding: `RuleID`, `Level=notice`, message
  `spatial reference system {srid} does not exist; IF EXISTS drop is a
  no-op`, metadata keys exactly `{srid, outcome, conditional_audit}`;
  `conditional_audit` JSON string.
- Gap object: exactly `rule_id`, `reason_code`, `required_facts` —
  three keys, `required_facts` ordered `[srs.drop.privilege]`; no
  internal outcome is serialised as a public field; empty slices
  follow the existing `omitempty` shape, asserted on real JSON paths.
- reserved 4326 rows: A-plain emits NO reserved advisory; A-IE✓ and
  A-IE? each add exactly one existing membership-only
  `ddl.mysql.srs.drop.reserved` notice — the dual expectation and the
  PA-10a question mark are removed.

Registered future touchpoints for that co-fire and projection:

- `mysql_srs_rules.go` reserved rule: arm guard must include the A
  arms (today it fires on B/S2); new `absent`/`absent_noop` rules;
  gap reason token `unproven_privilege` on the dependency-owned
  carrier; coverage projection unchanged shape.
- `common.go` rule IDs; registry + `configs/deltascope.example.yaml`
  default levels.
- `spec`: `Absence` field; `AdjudicateSRSDrop` gains `IfExists` input
  and the A arm values; `unproven_privilege` reason.
- `provider_server_srs.go:146-169`: QueryRowContext → explicit Rows
  (A3); D-proof check on the provider banner.
- `metadata.go:716-727`: pass `decl.IfExists` into the adjudication
  attach; `bindMySQLSRS` unchanged.
- `batch_state_srs.go`: no change — the `== SRSDropArmUnproven` mint
  guard already excludes A arms.

Policy rows (PLAN): `absent` disabled → A-plain produces no finding
(bound state/premise/own-apply unchanged); `absent_noop` level
rewritten to warning → A-IE✓ finding becomes warning → verdict review
with `conditional_audit` still present. Sentinel plan: distinguishable
`SRS_ZZTEST`/account `zz_acct`/grant/schema/table/column sentinels;
scan scope = the full serialised result JSON.

CLI providerless probes are referenced by identity only (not rerun,
not used as proven-absence positive evidence).

## 5. Errata registered against R1 records

- `static-checks.txt` printed `git log --format=%H %s -5` unquoted —
  as transcribed it cannot produce the shown hash+subject output
  (`%s` splits into a second arg and fails as an ambiguous revision);
  a corrected real capture ships in the R2 supplement.
- The trailing search-command lines had no retained output/rc; the
  `sed -n ranges listed …` line was a placeholder — all marked
  REPORTED/UNCAPTURED, not re-executed history.
- R1 `official-evidence.md` was a summary without the contiguous view
  definition, and claimed a whole-range coverage basis from three
  tags plus an unbound branch — corrected in R2-A2.
- R1 README C5/PA-6a cache wording implied cross-enrich persistence —
  corrected in R2-C.
- R1 PA-B ordered A before S1 — reversed per R2 ruling; the
  contradiction row is now S1-frozen with a real provider-layer
  sequence planned.
- R1 PA-1a/PA-6c/PA-7/PA-8c/PA-8d/PA-9/PA-10a witness defects —
  rewritten in `case-matrix.md` R2.
- `EXECUTION READY` resets to **NO** (see §7).

## 6. Out of scope (unchanged)

Ordered publication, present-object DROP success, SRS CREATE, ALTER
INSTANCE, T27. Native check ordering is corroborating documentation
only, never a bounded commitment. Dependency reads stay witness-only
forever under this contract.

## 7. EXECUTION READY: NO

Precise reasons: (1) the supply predicate requires the registered
explicit-Rows provider change before absence can be produced by the
real chain — `Row.Scan()==ErrNoRows` alone cannot supply it; (2) that
change is a registered touchpoint, not yet implemented or verified;
(3) therefore no execution work order can currently yield unique
expected results. The remaining items — domains, ordering, state and
projections — are closed by this contract; the sole open gate is the
provider read-path change plus its distinguishing rows.
