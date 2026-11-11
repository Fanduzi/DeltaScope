# T26-SRS-DROP-CONTRACT-01 — MySQL `DROP SPATIAL REFERENCE SYSTEM` audit contract (R1)

Status: **R1 — docs-only contract under review**
Work order: `T26-SRS-DROP-CONTRACT-01 / R1` (AUTHORIZED, docs-only)
Baseline: commit `f129afd8adefee5ffca0431021dc9c4ecfc39acb` · tree `609345c30e3788469397e697d6efa858a8b6611f` · branch `milestone/mysql-tidb-ddl-completion`
Scope: `DROP SPATIAL REFERENCE SYSTEM [IF EXISTS] <SRID>` under MySQL 8.0.x. First-slice targets are the refusal, missing-fact, and boundary branches plus in-batch fact-invalidation rules. Successful DROP, proven-absent, and OrderedAbsent/OrderedPresent publication are **not** contracted this slice — no SRS ordered channel is proposed.

Files delivered: this README plus `case-matrix.md` (section E). No `.go`, policy, registry, corpus, golden, or historical evidence was changed.

---

## A — Baseline capability inventory

Minimal chain on the baseline (all paths/line numbers verbatim in `baseline-source-excerpts.txt`, delivered as an archive artifact):

```
parse (adapter)          parseMySQLSpatialReference → mysqlSRSDrop      mysql_srs.go:47–116, 263–272
  SRID decode            mysqlSRID — digits-only uint32                 mysql_srs.go:122–131
extraction               typed declaration bound as own object family   extractor.go:123–135
qualification            applyCoverageBoundary — MySQL: unaudited impl. coverage_boundary.go:83–103
                          boundary, TiDB: vendor boundary (CURRENT)
enrich qualification     G2 version gate covers SERVER create/alter     metadata.go:248–266
                          only — no SRS enrich-time gate (CURRENT)
bind / cache             bindMySQLSRS → mysqlSRSResolver = objectCache  batch_state_server_srs.go:129–150
                          per-request seam                              metadata.go:353–371
provider reads           loadMySQLSRSObject — version() probe,          provider_server_srs.go:129–199
                          existence + dependency reads                  provider_server_srs.go:259–286
metadata                 Metadata.SpatialReference — json:"-" internal  spec/metadata.go:114, 772–778
public boundary / rules  Unsupported → result.Unsupported; no SRS rules evaluate.go:27–54
ordered apply            Unsupported SRS → return nil (no-op,           batch_state.go:457–465
                          no contamination)
privilege supply         statementsNeedMySQLServerPrivilege — SERVER    server_drop_premise.go:26–37
                          family only; SRS never triggers L
```

Per-item status (CURRENT = baseline implements it; PROPOSED = this contract selects it for a later execution order; UNREACHABLE/DEFERRED = cannot or will not be built):

1. **SRID extraction, identity, public skeleton — CURRENT.** `mysqlSRID` accepts digits-only `ParseUint(…,10,32)`; negative, signed, non-numeric, and overflowing spellings are never claimed and fall to the base grammar diagnostic (`mysql_srs.go:122–131`). `SRID` is `uint32`, always a provable identity (`spec/srs.go:22–39`, `SRIDName` at 175–177). `PublicSQL()` renders the fixed redacted skeleton `DROP SPATIAL REFERENCE SYSTEM [IF EXISTS] <srid>` (`spec/srs.go:183–208`). SRID `0` parses and is claimed — it is a semantic arm, not a parse failure.
2. **bindMySQLSRS / objectCache — CURRENT.** Bind runs for **every** MySQL-dialect SRS statement, including ones already marked `Unsupported` (`metadata.go:676–687`). Cache key is `(type=mysql_spatial_reference_system, name=SRIDName, qualifiers)` once per enrich call (`metadata.go:332–364`). Contaminated batches return the bound state unread with `UnreadReason` left empty (`batch_state_server_srs.go:132–134`) — asymmetric with SERVER, which stamps `MetadataUnreadBatchEffects` (§F bind seam registers the PROPOSED symmetry fix). Resolver/loader failures propagate as `metadataObjectReadError` (`metadata.go:1021–1056`).
3. **SpatialReferenceSystemState semantics — CURRENT.** `Read`/`UnreadReason`/`Exists`/`CatalogName`/`NameConflict`/`OrgConflict`/`Dependencies` (`spec/srs.go:96–129`). Zero-row answers are never promoted: existence zero-row stays unknown (`provider_server_srs.go:153–159`); `SRSDependencyFact.Complete` and `SRSConflictFact.Complete` are always false (`spec/srs.go:138–146`, `164–171`). `Status` never reports `not_found`.
4. **Existence / dependency SQL firing — CURRENT.** `select version()` first (`provider_server_srs.go:94–107`); proven pre-8.0 (`Product=mysql`, `ValidatedRange`, `Major<8`) → `version_inapplicable`, no further reads; proven 8.0+ **or unproven** version → existence read *and* dependency read both fire — the dependency read does not depend on the existence answer (`provider_server_srs.go:190–198`). NAME/ORGANIZATION conflict reads fire only on provable request qualifiers, which the statement path never sends (nil facts always for DROP). The historical record "pre-8.0 skips dedicated reads; unproven version still issues reads and issued-query failures propagate" (`t26-semantics-01` c-decision-table row 13) is **confirmed verbatim against baseline source**.
5. **Contamination / ordered write handling — CURRENT, partial.** Whole-batch contamination exits the bind before the resolver (`batch_state_server_srs.go:132`). There is **no** SRS ordered channel and **no** staleness model: the shared objectCache answer from the first bind is reused verbatim by later same-SRID statements even if an intervening statement changed (or could have changed) the facts — this is the gap §D closes contractually. SRS own apply is a proven no-op on tracked channels (`batch_state.go:457–465`).
6. **Privilege supply — CURRENT: none for SRS.** `statementsNeedMySQLServerPrivilege` matches `Server.Kind ∈ {drop,create,alter}` only (`server_drop_premise.go:26–37`); the loader never fires for SRS (`metadata.go:318–329`). No SRS-consumable privilege witness exists.
7. **Historical SRS decision table (`t26-semantics-01/c-decision-table.md` §C.2, 33 rows) — input material, unapproved.** Rows this contract **supersedes/selects**: rows 26 (`srs-drop-dep`), 27 (`srs-drop-depunknown`), 30 (`srs-drop-unknown`) are adopted as the `refused_dependency` and `unproven` arms below with renamed rule IDs. Row 6 (`srs-srid0`) is adopted as `refused_srid0` for the DROP verb. Row 7 (`srs-srid-reserved`) is adopted as a static advisory, see §C rule 4. Rows 25, 28, 29 (`srs-drop-ok`, `srs-drop-absent`, `srs-drop-ie-absent`) are **UNREACHABLE** this slice (no proven-empty, no proven-absent, no ordered channel). Rows 15–24 (CREATE/INE/REPLACE families) are **DEFERRED** — out of this contract's scope; boundary-statement invalidation of their *effects* is handled in §D1. Rows 31–33 (conflict reads provider-capable-but-unwired, ordered channel) stay unwired / deferred.

## B — Facts, version, and privilege contract

### B1 — Audit admission table (PROPOSED token `MySQLSRSDropAuditedDomain`)

Admission decides audited-vs-boundary from the **resolved** version — the same input the accepted SERVER G2 gate consumes — never from pin-presence alone:

| resolved product | resolved version | statement form | admission |
|---|---|---|---|
| TiDB | any | any SRS statement | `vendor` boundary (CURRENT, parser-level) |
| MySQL | 8.0.x | `SRSKindDrop` recognized (plain or `IF EXISTS`, any uint32 SRID incl. 0) | **audited** → adjudication (PROPOSED) |
| MySQL | 8.0.x | `SRSKindCreate`/`SRSKindCreateOrReplace` | unaudited boundary `object=spatial_reference_system` (unchanged) |
| MySQL | ≠8.0.x resolved (5.7.x, 8.4.x, other) | any SRS statement | unaudited boundary `gate:"version"` (PROPOSED enrich gate, mirrors SERVER) |
| MySQL | unresolved (no pin, unparseable/absent banner) | any SRS statement | unaudited boundary `gate:"version"` (PROPOSED) |
| — | — | unclaimed input (negative/overflow/non-numeric SRID, trailing tokens, `IF NOT EXISTS` on DROP, doubled guards) | never a statement → base grammar diagnostic (CURRENT) |
| — | — | version conflicts | `ErrInvalidTargetVersion` (malformed pin), `ErrDialectProductMismatch` (observed product ≠ dialect), `ErrTargetVersionMismatch` (pin ≠ observed) — typed errors before enrichment (CURRENT, `metadata.go:152–160`) |

`MySQLSRSDropAuditedDomain(version)` = `Product=mysql && Major=8 && Minor=0` (PROPOSED spec helper, single definition site — 8.0 manual semantics only; 8.4 is a later slice with its own evidence).

### B2 — Provider-read applicability table (all CURRENT behavior)

Reads are keyed on the **observed** server version inside the provider and on the **batch state** inside bind — independent of admission: a version-gated unaudited statement still binds and reads on MySQL today (`metadata.go:676` has no `Unsupported` check). Pin-presence never causes a read; non-admission never suppresses one.

| condition | R (resolver forward) | `Qother` version() | `Qexist` | `Qdeps` | notes |
|---|---|---|---|---|---|
| dialect ≠ MySQL (TiDB vendor boundary) | 0 | 0 | 0 | 0 | `bindMySQLSRS` exits before resolver |
| whole-batch contaminated | 0 | 0 | 0 | 0 | bind exits; state unread, `UnreadReason` empty (PROPOSED: `batch_effects`) |
| providerless / resolver nil | 0 | 0 | 0 | 0 | `objectResolverFor` → nil → unread state, no reason token (CURRENT) |
| cache hit (same SRID resolved earlier in this call) | 0 | 0 | 0 | 0 | objectCache once-per-(type,name,qualifiers) |
| resolve forward, proven pre-8.0 observed | 1 | 1 | 0 | 0 | `version_inapplicable` unread on every fact |
| resolve forward, proven 8.0+ or unproven observed | 1 | 1 | 1 | 1 | deps fires regardless of existence answer |
| resolve forward, version() read failed | 1 | 1(attempted) | 0 | 0 | error propagates — typed, never a state |
| resolve forward, existence read failed | 1 | 1 | 1(attempted) | 0 | error propagates before deps read |
| resolve forward, deps read failed | 1 | 1 | 1 | 1(attempted) | error propagates |
| conflict reads (name/org) | 0 | 0 | — | — | never fire: DROP carries no qualifiers (CURRENT + PROPOSED-unchanged) |
| `L` (privilege loader) | — | — | — | — | **always 0 for SRS** — the SERVER loader's trigger set never includes SRS; mixed fixtures attribute L to the SERVER statement |

`R` counts `objectResolverFor` forwards (cache misses), never SQL; one forward issues 2–3 SQL on the success path. Counts in `case-matrix.md` are **contract expectations derived from source**, not measurements.

### Fact vocabulary (bind output consumed by adjudication)

| fact state | producer | meaning |
|---|---|---|
| `unread` | `Read=false`, `UnreadReason` empty | providerless / resolver nil / contaminated / dialect-mismatch — the contract keeps these distinct (never a generic "no data") |
| `unread` | `Read=false`, `UnreadReason=version_inapplicable` | proven pre-8.0 catalog cannot hold the object |
| `existence unknown` | `Read=true`, `Exists=false` | zero-row answer — **never proven-absent** (privilege-filtered catalog, no completeness contract) |
| `proven present` | `Read=true`, `Exists=true`, `Status=confirmed`, `CatalogName` set | catalog row witness |
| `positive dependency witness` | `Dependencies≠nil`, `len(Columns)>0` | ≥1 visible geometry column declares this SRID — proves the 3716 refusal **and** proves the SRID exists |
| `deps empty, unproven` | `Dependencies≠nil`, `len(Columns)=0`, `Complete=false` | zero-row answer — **never "no dependencies"** |
| `deps unread` | `Dependencies=nil` | read never issued (pre-8.0, providerless, contaminated, earlier failure) |
| `facts unconsumable` | §D stale/consumption flags | a batch write (or possible write) outranks the cached answer |
| `read failed` | resolver/load error | typed error path — never converted into a state or gap |

PROPOSED inputs the adjudication adds beyond the CURRENT bound state: `contaminated` (whole-batch flag, already on batchState), `srsExistenceStale`/`srsDepsStale` (per-SRID / batch-wide consumption flags from §D — new PROPOSED `batchState` members), `decl.SRID`, `decl.IfExists`, `decl.SRID ∈ reserved ranges` (static), `resolvedVersion` for admission.

### Hard constraints (frozen)

- Catalog zero-row → never proven-absent. Dependency zero-row with `Complete=false` → never proven-empty.
- **Existence unknown + positive dependency witness → the `refused_dependency` arm refuses.** Preconditions: (a) facts consumable (no contamination/stale flag on this SRID), (b) the witness is a real issued read (`Dependencies≠nil` with rows), (c) `Columns[].SRID` echoes the target SRID. Rationale: the 3716 refusal is enforced on the dependency check regardless of what the existence read returned; the dep rows also imply presence, so `srs.existence` is discharged by the witness itself.
- Unproven-privilege + everything else: `srs.drop.privilege` stays owed on every non-refusal arm. **Privilege premise token `SRSDropPrivilege` is PROPOSED with exactly one reachable state — `unproven` — this slice** (no loader exists; the SERVER consumable premise is not an SRS proof and is not widened).
- Inputs with no lawful producer (proven-absent existence, proven-empty-complete deps) are **model/unit-layer only** — never wrapped as enrich/provider witnesses.
- Missing external fact and unimplemented capability are separate gap tokens; no catch-all.

### Privilege basis (official)

MySQL 8.0 Reference Manual §15.1.31 `DROP SPATIAL REFERENCE SYSTEM` — *"This statement removes a spatial reference system (SRS) definition from the data dictionary. **It requires the `SUPER` privilege.**"* — URL `https://dev.mysql.com/doc/refman/8.0/en/drop-spatial-reference-system.html`, verified 2026-11-15 (this session fetch). Additional fixed semantics from the same page: absent SRID → error unless `IF EXISTS` (then warning); column dependency → `ERROR 3716 (SR005)`; SRID 0 is a valid SRID but cannot be used with `DROP SPATIAL REFERENCE SYSTEM`; reserved ranges `[0,32767]`/`[60000000,69999999]`/`[2000000000,2147483647]` produce a warning on use, and dropping system SRSs is discouraged (not prohibited). Metadata readability (the audit's catalog reads) does **not** imply the audited account holds `SUPER` — the premise stays unproven until a lawful observation path exists (deferred; PROPOSED future slice may reuse the `SUPER`-token grant-line witness shape only under its own work order).

## C — Unique adjudication table and public output

Normative arm table — mutually exclusive, evaluated top-down. `audited` admission is a precondition; boundary rows never reach this table. Rule evaluation and apply consume **one** adjudicated result (`Metadata.SRSDrop`, PROPOSED — no parallel recomputation).

| arm | entry predicate | facts used | outcome | findings | owed | conditional | statement coverage | modeled post-state |
|---|---|---|---|---|---|---|---|---|
| **B — `blocked`** | `contaminated` or `srsExistenceStale[SRID]` | none (unconsumable) | review | none | `srs.batch_effects` | n/a | unverified | SRID stays stale; facts unconsumed |
| **S0 — `refused_srid0`** | `decl.SRID == 0` | static declaration | reject | 1 blocker `ddl.mysql.srs.drop.srid0` | — | n/a | complete | provably unchanged; facts consumable |
| **S1 — `refused_dependency`** | `Dependencies≠nil && len(Columns)>0` (and each row's `SRID` matches) | positive dep witness | reject | 1 blocker `ddl.mysql.srs.drop.dependency` | — | n/a | complete | provably unchanged (3716-class refusal); facts consumable |
| **S2 — `unproven`** | everything else | unread/unknown/empty-incomplete per fact | review | none | one token per unproven required fact: `srs.existence` (unread or `Exists=false`), `srs.column_dependencies` (nil or empty-incomplete **or `srsDepsStale`**), `srs.drop.privilege` (always, this slice) | `if_exists` authored flag rides finding metadata only — it never discharges a gap or proves absent | unverified | unmodeled effect → set `srsExistenceStale[SRID]` **and** `srsDepsStale` |
| reserved advisory | co-fires on B/S2 only | static `SRID ∈ reserved` | — | +1 notice `ddl.mysql.srs.drop.reserved` | — | — | — | — |

Frozen sub-rulings:

1. **Does a dependency refusal close other owed?** Yes — the 3716-class refusal is proven by the witness alone; `srs.existence` and `srs.drop.privilege` are moot for a statement that provably cannot execute. Owed list on refusal arms is empty. (Same closure rule for S0 — a static unconditional refusal owes nothing.)
2. **"Cannot succeed" vs "predict native's first error".** Refusal arms promise only that the statement cannot succeed — never which server error fires first; manual/worklog ordering is not a sequencing oracle.
3. **`IF EXISTS` scope.** It addresses only the absent-object error, which has no lawful producer this slice — so `IF EXISTS` changes **nothing** in the arm table except the carried `if_exists` metadata flag. It does not suppress SRID 0, does not suppress dependency refusal, does not discharge existence, and cannot alone prove absent.
4. **Reserved range is advisory, not prohibition.** `ddl.mysql.srs.drop.reserved` is a static notice — it never makes a statement complete by itself and is **suppressed on refusal arms** (a statement that cannot execute has no reserved-drop to warn about). Advisory text must not claim the native engine emitted a warning — it states the documented range membership only.
5. **Stale deps are per-fact, not per-statement.** `srsDepsStale` demotes only the dependency fact inside S2 — existence may still adjudicate present; `srsExistenceStale[SRID]` demotes the whole statement into B.

Public output (all CURRENT mechanisms reused; new tokens listed separately):

- SQL: `DROP SPATIAL REFERENCE SYSTEM [IF EXISTS] <decimal SRID>` — `SpatialReferenceDeclaration.PublicSQL()`, unchanged. The SRID is numeric and carries no authored identity.
- Findings carry rule_id/level/message only; bounded metadata allowed: `srid` (numeric), `if_exists` (bool), `dependency_witness_count` (int). **Never** exposed: dependency schema/table/column names, `CatalogName`, WKT/description text, account/grant text, provider error payloads.
- Issued-read failures stay on the existing bounded error path: `metadataObjectReadError`/`serverPrivilegeReadError`-shaped (fixed public text, `Unwrap` preserves cause) — a failed metadata query is a typed error, never a success-with-gap.
- New PROPOSED tokens (single naming): rule IDs `ddl.mysql.srs.drop.srid0`, `ddl.mysql.srs.drop.dependency`, `ddl.mysql.srs.drop.reserved`; fact tokens `srs.existence`, `srs.column_dependencies`, `srs.drop.privilege`, `srs.batch_effects`; spec types `SRSDropAdjudication{Arm, Owed}`, `SRSDropArm*`, `SRSDropPrivilege{Unproven…}`; helper `MySQLSRSDropAuditedDomain`; `Metadata.SRSDrop`. Reused: `UnsupportedDetail` gates, `MetadataUnread*` vocabulary, coverage mechanics (`gaps→unverified` lowering), PublicSQL.
- Aggregate `coverage`/`verdict` asserted only in full-request-observed matrix rows; per-statement finding counts are per-statement, never batch totals.

## D — In-batch ordering and fact invalidation

`objectCache` is a per-enrich-call memo of **pre-state reads**; it is not lifecycle knowledge. Consumption validity is adjudicated per fact before any cached or provider answer is used.

**D1 — same SRID:**

| prior statement | modeled effect | effect on later same-SRID reads |
|---|---|---|
| audited DROP, non-refusal arm (S2) | unknown (may have executed) | `srsExistenceStale[SRID]=true` + `srsDepsStale=true` → next DROP hits arm **B** |
| audited DROP, refusal arm (S0/S1) | provably unchanged | none — facts stay consumable; a repeated identical DROP refuses identically |
| boundary CREATE / CREATE OR REPLACE / DROP (unaudited, version-gated) | unmodeled — may have written | same as unknown-effect DROP → arm **B** |
| arm **B** outcome | unchanged stale | stays **B** (stale is monotonic within the batch) |

A refused statement never mints staleness; a non-refused one always does. No absent/publication path exists — nothing publishes `OrderedAbsent`/`OrderedPresent` for SRS this slice.

**D2 — different SRID:** identity is numeric equality of the authored `uint32`; `100001` and `4326` are distinct objects. No name folding, no relation lattice. `srsExistenceStale` is keyed per-SRID; `srsDepsStale` is batch-wide (a geometry-column change can target any SRID — see D3).

**D3 — table/schema writes vs the dependency witness:** the `ST_GEOMETRY_COLUMNS` read is pre-state; any statement whose modeled **or unmodeled** effect could change the column→SRID binding set invalidates every deps witness. Invalidation predicate `spatialTouching(stmt)` = true, **except** a statement provably spatial-inert — audited with a complete authored column/action inventory containing no spatial-typed column create/add/change/drop and no table drop/rename/truncate (e.g., `CREATE TABLE t (i INT)` with the full column list declared, `ALTER TABLE t ENGINE=InnoDB`, pure option/migration-free operations, non-table statements). `DROP SCHEMA`/`ALTER SCHEMA` are touching (a dropped schema removes its tables); `CREATE SCHEMA` is inert (holds no columns). Everything unclassifiable defaults to touching — conservative. Re-querying cannot recover validity: the database never executed this batch's earlier DDL, so a fresh read would still be pre-state.

**D4 — contamination and isolation:** the failure-record ordering uses authored `(Line, Column)` position (`metadata.go:398–403`); contamination set before a statement suppresses its resolver call (R=0) and persists across enrich calls on the same `batchState` (per-call cache cannot launder it — the flag lives on state, `metadata.go:705–707`). SRS's own apply never rewrites accepted SERVER state or unrelated table/schema state — invalidation arrives from *other* statements' applies, attributed there, not to the SRS statement. A refused SRS write adds no taint; an unmodeled-effect SRS write marks only its own SRID — never whole-batch contamination.

## E — Acceptance matrix and probe plan

`case-matrix.md` carries 12 case families (E1–E12) with per-row: case ID, observation layer, full SQL/sequence, dialect/product/version, pin, provider/cache/batch state, provider response, rule config, expected adjudication + public fields + state effect, and independent read counts `R/Qexist/Qdeps/Qother/L`. Three `PLAN/UNEXECUTED` CLI probes (single `DROP SPATIAL REFERENCE SYSTEM 100001` under pin `8.0.36`, no pin, malformed `8.0.x`) are registered with full config text, exact commands, separated stdout/stderr, immediate-rc script, and per-field expectations — this contract only plans them.

## F — Execution seam table (for the follow-on execution order; nothing implemented here)

| seam | change needed | why | reusable | explicitly unchanged |
|---|---|---|---|---|
| spec | `SRSDropAdjudication`, `SRSDropArm*` (blocked/refused_srid0/refused_dependency/unproven), `SRSDropPrivilege`, `MySQLSRSDropAuditedDomain`, `AdjudicateSRS Drop`, `Metadata.SRSDrop` | one adjudication truth feeding rule+apply | `ServerDropAdjudication` shape, `PublicSQL`, `MetadataUnread*` | `SpatialReferenceSystemState` fields, `Complete` contract |
| qualification | `mysqlSRSAuditedDropForm` + enrich G2 for `SRSKindDrop` only | move audited DROP out of the implementation boundary | SERVER G2 gate mechanics (`metadata.go:248–266`) | CREATE/OR REPLACE stay boundary; parser claims |
| bind | consume `contaminated`+stale flags; PROPOSED stamp `UnreadReason=batch_effects` on contaminated bind for symmetry | honest unread reasons | `bindMySQLSRS`, objectCache | resolver signature, lookup qualifiers |
| provider | **none** | reads already bounded | `loadMySQLSRSObject` | no new queries, no completeness claims |
| ordered invalidation | `batchState.srsExistenceStale map[uint32]bool`, `srsDepsStale bool`; SRS own apply stays no-op on tracked channels but sets stale flags per arm; `spatialTouching` predicate wires table/schema applies | §D rules | `fidelityTableSnapshot`-style test witnesses for isolation tests | no SRS ordered channel this slice |
| rules | `ddl.mysql.srs.drop.{srid0,dependency,reserved}` | arm→findings | `mysqlDropServerRule` pattern | existing rule set |
| coverage | arm→coverage mapping (complete/unverified); gap lowering reuse | §C | existing mechanics | boundary coverage path |
| policy | register three rule IDs + default levels | config surface | `deltascope.example.yaml` pattern | — |
| tests | unit truth-table + premise/invalidate/enrich/public/vertical cases per matrix | E families | t26 provider/fixture/hook seams | snapshot helpers already accepted |
| docs | module README rows + ADR | three-level discipline | existing formats | — |

### Static checks actually run this round

`git status`/`git log`/`git rev-parse` (identity), baseline source reads (above), one official-doc fetch (URL + date in §B), `git diff --check` on the delivered diff — outputs in `static-checks.txt` (archive artifact). No tests, builds, probes, golden, or native rounds — per the work order.

### Exclusions

SRS CREATE/OR REPLACE and `IF NOT EXISTS` adjudication; ALTER INSTANCE; T27; SRS ordered channel / post-state publication; privilege loaders for SRS; proven-absent / proven-empty consumption; WKT legality; NAME/ORGANIZATION conflict binding; native oracle work (registered as future evidence needs, never assumed).
