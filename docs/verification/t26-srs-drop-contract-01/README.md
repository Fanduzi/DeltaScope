# T26-SRS-DROP-CONTRACT-01 — MySQL `DROP SPATIAL REFERENCE SYSTEM` audit contract (R3)

Status: **R3 — docs-only contract under review**
Work order: `T26-SRS-DROP-CONTRACT-01 / R3` (AUTHORIZED, docs-only)
Baseline: commit `f129afd8adefee5ffca0431021dc9c4ecfc39acb` · tree `609345c30e3788469397e697d6efa858a8b6611f` · branch `milestone/mysql-tidb-ddl-completion`
Scope: `DROP SPATIAL REFERENCE SYSTEM [IF EXISTS] <SRID>` under MySQL. First-slice targets are the refusal, missing-fact, and boundary branches plus in-batch fact-invalidation rules. Successful DROP, proven-absent, and OrderedAbsent/OrderedPresent publication are **not** contracted this slice — no SRS ordered channel is proposed.

Files delivered: this README plus `case-matrix.md` (section E). No `.go`, policy, registry, corpus, golden, or historical evidence was changed.

**R2 → R3 delta (reviewer R2 ruling `CHANGES REQUIRED`, six groups):**
- **R3-A** — canonical owed/writer consistency: S2 owes `{srs.column_dependencies, srs.drop.privilege}` unconditionally (no reachable complete-empty producer) plus `srs.existence` iff existence is unproven; B owes exactly `{srs.batch_effects}` and mints nothing; only S2 and boundary SRS writes mint `srsExistenceStale`.
- **R3-B** — `spatialTouching` narrowed: inert exceptions are only complete non-spatial `CREATE TABLE`, complete engine/option-only `ALTER TABLE`, `CREATE SCHEMA`, and provably non-table kinds; everything else — partial/unmodeled columns, `CREATE … LIKE/AS`, boundary statements carrying table targets — is touching by default. E8.4 now uses `ALTER TABLE t COMPACT TIFLASH REPLICA`, a statically provable vendor boundary that carries a real `Table` target.
- **R3-C** — matrix fully concretized: exact fixtures/state/positions per row, observation-layer labels, `N/A` discipline, and the reserved-range correction — **SRID 4326 ∈ [0,32767] is reserved**, so every B/S2 row on 4326 co-fires exactly one `…reserved` notice.
- **R3-D** — public projection completed: fixed message literals, real JSON paths (`$.statements`, `$.unsupported`, `$.coverage`, `$.verdict`, `$.summary`), `omitempty` absence-vs-zero discipline, `UnsupportedDetail.index` omit-on-zero, version boundary coverage `incomplete`.
- **R3-E** — probes carry complete copyable command blocks with separated stdout/stderr files and immediate `$?` capture; every rc is derived from the real CLI dispatch (`newUserError`→`exitUser=2`, `failOnTriggered`→exit 1 paths, `SilenceErrors/SilenceUsage`), not assumed.
- **R3-F** — excerpt set extended for the newly required seams (version regex/validated series, discovery/preflight path, in-loop failure consumption, apply outer Unsupported branch, COMPACT extraction+boundary, public evaluator, CLI dispatch); R1/R2 static-record errata registered separately without rewriting originals.

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
version facts            ValidateTargetVersion preflight,               version.go / service.go
                          ParseObservedVersion banner → product+series
authored ordering        failureBefore(line,col) — failure records      batch_state.go:3724–3729
                          precede statements by authored position
geometry vocabulary      spatialMemberFamilyType — the eight            advanced_indexes.go:707–712
                          normalized spatial base-type tokens
report surface           statements[].sql/findings/evidence_gaps/        report package / evaluate.go
                          coverage/unsupported; result.verdict/summary
CLI surface              audit --sql/--file/--dialect/--format/          interfaces/cli/audit.go
                          --fail-on/--target-version/--config; rc 0/1/2/3
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

Admission decides audited-vs-boundary from the **resolved** version — the same input the accepted SERVER G2 gate consumes — never from pin-presence alone. **Audit qualification and provider-read applicability are separate axes** (B2): a gated statement may still bind and read, and an admitted statement may read nothing.

| resolved product | resolved version | statement form | admission |
|---|---|---|---|
| TiDB | any | any SRS statement | `vendor` boundary (CURRENT, parser-level) |
| MySQL | 8.0.4+ (validated `Major=8,Minor=0,Patch≥4`) | `SRSKindDrop` recognized (plain or `IF EXISTS`, any uint32 SRID incl. 0) | **audited** → adjudication (PROPOSED) |
| MySQL | 8.0.0–8.0.3 (validated, patch < 4) | `SRSKindDrop` | unaudited boundary `gate:"version"` — the feature was introduced in 8.0.4 (PROPOSED enrich gate, mirrors SERVER) |
| MySQL | ≠8.0 resolved (5.7.x, 8.4.x, other) | `SRSKindDrop` | unaudited boundary `gate:"version"` (PROPOSED) |
| MySQL | unresolved (no pin; banner present but unparseable by `observedVersionPrefix`) | `SRSKindDrop` | unaudited boundary `gate:"version"` (PROPOSED) — **unknown is never converted to "feature absent"**; it is merely not proven ≥8.0.4 |
| MySQL | version discovery **read error** (issued request-layer read failed) | `SRSKindDrop` | typed error propagates on the request path — never a boundary, never a gap (R3-A.4; distinct from a successful-but-unparseable banner) |
| MySQL | any resolved version | `SRSKindCreate`/`SRSKindCreateOrReplace` | unaudited boundary `object=spatial_reference_system` (CURRENT, unchanged — the 8.0.4 gate is DROP-only and does **not** expand CREATE) |
| — | — | unclaimed input (negative/overflow/non-numeric SRID, trailing tokens, `IF NOT EXISTS` on DROP, doubled guards) | never a statement → base grammar diagnostic (CURRENT) |
| — | — | version conflicts | `ErrInvalidTargetVersion` (malformed pin), `ErrDialectProductMismatch` (observed product ≠ dialect), `ErrTargetVersionMismatch` (pin ≠ observed) — typed errors before enrichment (CURRENT, `metadata.go:152–160`) |

`MySQLSRSDropAuditedDomain(version)` = `Product=mysql && ValidatedRange && Major=8 && Minor=0 && Patch>=4` (PROPOSED spec helper, single definition site). **Feature lower bound basis**: the MySQL 8.0.4 release notes document `CREATE`/`DROP SPATIAL REFERENCE SYSTEM` as new in 8.0.4 (`https://dev.mysql.com/doc/relnotes/mysql/8.0/en/news-8-0-4.html`); the 8.0 Reference Manual DROP page (§B privilege basis) carries the semantics. 8.0.0–8.0.3 predate the feature — a proven pre-feature version is a **proven boundary**, kept distinct from an **unproven version** which is also a boundary but for a different reason (not proven ≥, not proven <). Pin presence alone never infers admission or exit-code behavior; the gate consumes the resolved version fact.

### B2 — Provider-read applicability table (all CURRENT behavior)

Reads are keyed on the **observed** server version inside the provider and on the **batch state** inside bind — independent of admission: a version-gated unaudited statement still binds and reads on MySQL today (`metadata.go:676` has no `Unsupported` check). Pin-presence never causes a read; non-admission never suppresses one. An unproven version keeps reads firing: if the provider issues a read and it fails, the failure stays a typed error — never a "feature absent" state.

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

**Valid-dependency predicate (PROPOSED, the only witness S1 may consume).** A bound `Dependencies` fact is a *valid dependency witness* iff **all** hold: (a) `Dependencies ≠ nil` and `len(Columns) > 0`; (b) every row's `Columns[].SRID` equals `decl.SRID`; (c) the fact came from a real issued read this batch (a populated struct minted without a read is not a witness); (d) the fact is currently consumable — `contaminated=false`, `srsExistenceStale[srid]=false`, **`srsDepsStale=false`** (a batch write that may have dropped/renamed/retyped the dependent column invalidates the witness **before** it reaches S1); (e) no earlier same-SRID statement in authored order minted `srsExistenceStale`. A witness invalidated by a preceding table/schema change is **demoted, never deleted and never read as absence**: the statement falls to S2 with `srs.column_dependencies` owed.

PROPOSED inputs the adjudication adds beyond the CURRENT bound state: `contaminated` (whole-batch flag, already on batchState), `srsExistenceStale` (per-SRID consumption flag) / `srsDepsStale` (batch-wide dependency-fact flag) — two new PROPOSED `batchState` members with **disjoint writers** (§D), `decl.SRID`, `decl.IfExists`, `decl.SRID ∈ reserved ranges` (static), `resolvedVersion` for admission.

### Hard constraints (frozen)

- Catalog zero-row → never proven-absent. Dependency zero-row with `Complete=false` → never proven-empty. An invalidated positive witness → never a proven absence and never a refusal.
- **Existence unknown + valid dependency witness → the `refused_dependency` arm refuses.** Rationale: the 3716 refusal is enforced on the dependency check regardless of what the existence read returned; the dep rows also imply presence, so `srs.existence` is discharged by the witness itself.
- **Existence and dependency validity are independent.** A stale dependency flag demotes only the dependency fact — a still-valid positive existence stays consumable. A stale existence flag routes the whole statement to B regardless of dependency state.
- `srs.drop.privilege` is owed on **S2 only** (B owes exactly `srs.batch_effects`; refusals owe nothing). **Privilege premise token `SRSDropPrivilege` is PROPOSED with exactly one reachable state — `unproven` — this slice** (no loader exists; the SERVER consumable premise is not an SRS proof and is not widened).
- No reachable input this slice can prove an empty dependency set or a proven absence: `Complete=false` is structural, so on S2 `srs.column_dependencies` is **always** owed and `srs.drop.privilege` is **always** owed; `srs.existence` joins iff existence stayed unproven.
- Inputs with no lawful producer (proven-absent existence, proven-empty-complete deps) are **model/unit-layer only**, reachable only through the spec adjudication seam — never wrapped as enrich/provider witnesses and never present in the public matrix.
- The owed vocabulary is the fixed token set `{srs.existence, srs.column_dependencies, srs.drop.privilege, srs.batch_effects}` — no capability-gap mechanism is added.

### Privilege basis (official)

MySQL 8.0 Reference Manual §15.1.31 `DROP SPATIAL REFERENCE SYSTEM` — *"This statement removes a spatial reference system (SRS) definition from the data dictionary. **It requires the `SUPER` privilege.**"* — URL `https://dev.mysql.com/doc/refman/8.0/en/drop-spatial-reference-system.html`, fetched this session (worker local date 2026-11-12; the R1 doc misprinted `2026-11-15` — see §F errata). Additional fixed semantics from the same page: absent SRID → error unless `IF EXISTS` (then warning); column dependency → `ERROR 3716 (SR005)`; SRID 0 is a valid SRID but cannot be used with `DROP SPATIAL REFERENCE SYSTEM`; reserved ranges `[0,32767]`/`[60000000,69999999]`/`[2000000000,2147483647]` produce a warning on use, and dropping system SRSs is discouraged (not prohibited). Metadata readability (the audit's catalog reads) does **not** imply the audited account holds `SUPER` — the premise stays unproven until a lawful observation path exists (deferred; PROPOSED future slice may reuse the `SUPER`-token grant-line witness shape only under its own work order).

## C — Unique adjudication table and public output

Normative arm table — mutually exclusive, evaluated top-down. `audited` admission is a precondition; boundary rows never reach this table. Rule evaluation and apply consume **one** adjudicated result (`Metadata.SRSDrop`, PROPOSED — no parallel recomputation).

| arm | entry predicate | facts used | outcome | findings | owed | conditional | statement coverage | modeled post-state |
|---|---|---|---|---|---|---|---|---|
| **B — `blocked`** | `contaminated` or `srsExistenceStale[SRID]` | none (unconsumable) | review | none (+advisory if reserved) | `srs.batch_effects` | n/a | unverified | SRID stays stale; no new flags minted |
| **S0 — `refused_srid0`** | `decl.SRID == 0` | static declaration | reject | 1 blocker `ddl.mysql.srs.drop.srid0` | — | n/a | complete | provably unchanged; facts consumable; no flags |
| **S1 — `refused_dependency`** | **valid-dependency predicate** (§B) | valid positive dep witness | reject | 1 blocker `ddl.mysql.srs.drop.dependency` | — | n/a | complete | provably unchanged (3716-class refusal); facts consumable; no flags |
| **S2 — `unproven`** | everything else | unread/unknown/empty-incomplete/stale per fact | review | none (+advisory if reserved) | **always** `{srs.column_dependencies, srs.drop.privilege}` (no reachable complete-empty/privilege producer), **plus** `srs.existence` iff existence stayed unproven (`Read=false` or `Exists=false`) | `if_exists` is carried only by the public SQL skeleton — it never discharges a gap, marks a finding, or proves absent | unverified | unmodeled effect → set `srsExistenceStale[SRID]` **only** (a DROP SRS never writes geometry columns, so it cannot invalidate dependency facts) |
| reserved advisory | co-fires on B/S2 only | static `SRID ∈ reserved` | — | +1 notice `ddl.mysql.srs.drop.reserved` | — | — | — | — |

Frozen sub-rulings:

1. **Does a dependency refusal close other owed?** Yes — the 3716-class refusal is proven by the witness alone; `srs.existence` and `srs.drop.privilege` are moot for a statement that provably cannot execute. Owed list on refusal arms is empty. (Same closure rule for S0 — a static unconditional refusal owes nothing.)
2. **"Cannot succeed" vs "predict native's first error".** Refusal arms promise only that the statement cannot succeed — never which server error fires first; manual/worklog ordering is not a sequencing oracle.
3. **`IF EXISTS` scope.** It addresses only the absent-object error, which has no lawful producer this slice — so `IF EXISTS` changes **nothing** in the arm table. Its only public carrier is the verbatim `IF EXISTS` in the redacted SQL skeleton; it is never copied into finding metadata. It does not suppress SRID 0, does not suppress dependency refusal, does not discharge existence, and cannot alone prove absent.
4. **Reserved range is advisory, not prohibition.** `ddl.mysql.srs.drop.reserved` is a static notice — it never makes a statement complete by itself and is **suppressed on refusal arms** (a statement that cannot execute has no reserved-drop to warn about). Advisory text must not claim the native engine emitted a warning — it states the documented range membership only.
5. **Stale facts demote per fact, never per statement and never to absence.** `srsDepsStale` demotes only the dependency fact inside S2 — existence may still adjudicate present; `srsExistenceStale[SRID]` demotes the whole statement into B. Neither flag erases the other, and neither converts a witness into a proven no-dependency.

### Frozen public projection (R2-E — exact per-arm output)

JSON field paths are the existing report surface (`report/result.go`, `rule/rule.go`, `spec/statement.go` — baseline shapes, verbatim in `baseline-source-excerpts.txt`): `statements[i].raw_sql`/`normalized_sql` (the redacted skeleton), `statements[i].findings[]` (`.rule_id`, `.level`, `.message`, `.metadata`), `statements[i].evidence_gaps[]` (`.rule_id`, `.reason_code`, `.required_facts[]`), `statements[i].coverage.status`, `result.unsupported[]` (result-level `{feature, reason, metadata}` — **not** per-statement), `result.verdict`, `result.summary`, `result.coverage`. Arm tokens appear only inside finding `.metadata.outcome`; `if_exists` never appears as a metadata key — it is visible verbatim in `raw_sql`/`normalized_sql`.

Rule-role assignment (fixed): `ddl.mysql.srs.drop.dependency` is the adjudicating rule — it emits the S1 blocker finding **and** reports every B/S2 gap (`rule_id` on the gap is always `…dependency`); `ddl.mysql.srs.drop.srid0` emits only the S0 blocker; `ddl.mysql.srs.drop.reserved` emits only the advisory and reports no gaps.

Fixed message literals (bounded static text / bounded numeric template — never authored names):

- `…srid0`: `"dropping spatial reference system SRID 0 is not allowed"`
- `…dependency`: `"spatial reference system <srid> is used by <n> visible geometry column(s)"` (`<n>` = `dependency_witness_count`)
- `…reserved`: `"spatial reference system <srid> is in a reserved SRID range"` — states documented range membership only; never claims the native engine emitted a warning

Serialization discipline (frozen): empty `findings`/`evidence_gaps`/`unsupported` serialize as **key-absent** (`omitempty`), never `[]`; `UnsupportedDetail.index` is omitted when `0`; `coverage` is always present as an object `{status:…}`; `conditional_audit` and every conditional marker are **absent on all arms this slice** (no conditional publication exists).

| arm | `$.statements[i].findings` (exact) | `$.statements[i].evidence_gaps` | `$.statements[i].coverage.status` | `$.unsupported` contribution | `$.verdict` contribution |
|---|---|---|---|---|---|
| B `blocked` | reserved SRID: `[{rule_id:"ddl.mysql.srs.drop.reserved", level:"notice", message:"spatial reference system <srid> is in a reserved SRID range", metadata:{srid:<n>}}]`; else key absent | `[{rule_id:"ddl.mysql.srs.drop.dependency", reason_code:"unknown_srs_state", required_facts:["srs.batch_effects"]}]` | `unverified` | none | review |
| S0 `refused_srid0` | `[{rule_id:"ddl.mysql.srs.drop.srid0", level:"blocker", message:"dropping spatial reference system SRID 0 is not allowed", metadata:{srid:0, outcome:"refused_srid0"}}]` | key absent | `complete` | none | reject |
| S1 `refused_dependency` | `[{rule_id:"ddl.mysql.srs.drop.dependency", level:"blocker", message:"spatial reference system <srid> is used by <n> visible geometry column(s)", metadata:{srid:<n>, outcome:"refused_dependency", dependency_witness_count:<n>}}]` | key absent | `complete` | none | reject |
| S2 `unproven` | reserved SRID: `[{rule_id:"ddl.mysql.srs.drop.reserved", level:"notice", message:"spatial reference system <srid> is in a reserved SRID range", metadata:{srid:<n>}}]`; else key absent | `[{rule_id:"ddl.mysql.srs.drop.dependency", reason_code:"unknown_srs_state", required_facts:[ordered owed — "srs.column_dependencies","srs.drop.privilege" always, "srs.existence" prepended iff unproven]}]` | `unverified` | none | review |
| version gate (MySQL, non-8.0.4+) | key absent | key absent | `incomplete` | `$.unsupported` gains `{feature:"drop_spatial_reference_system", sql:"DROP SPATIAL REFERENCE SYSTEM [IF EXISTS] <srid>", reason:"parsed by the shared parser but not covered by audited semantics", metadata:{boundary:"unaudited", object:"spatial_reference_system", gate:"version"}}`; `index` omitted when 0 | review |
| TiDB vendor boundary (CURRENT) | key absent | key absent | `incomplete` | `{feature:"drop_spatial_reference_system", sql:"<skeleton>", reason:"parsed by the shared parser but outside the supported statement surface for this dialect", metadata:{boundary:"vendor"}}` | review |

**Never serialized**: dependency schema/table/column names, `CatalogName`, WKT/description text, account/grant text, provider error payloads, internal arm/flag names outside `metadata.outcome`. Issued-read failures stay on the existing bounded error path: `metadataObjectReadError`-shaped (fixed public text, `Unwrap` preserves cause) — a failed metadata query is a typed error, never a success-with-gap.

- New PROPOSED tokens (single naming): rule IDs `ddl.mysql.srs.drop.srid0`, `ddl.mysql.srs.drop.dependency`, `ddl.mysql.srs.drop.reserved`; gap `reason_code` `unknown_srs_state`; fact tokens `srs.existence`, `srs.column_dependencies`, `srs.drop.privilege`, `srs.batch_effects`; spec types `SRSDropAdjudication{Arm, Owed}`, `SRSDropArm*`, `SRSDropPrivilege{Unproven…}`; helper `MySQLSRSDropAuditedDomain`; `Metadata.SRSDrop`. Reused: `UnsupportedDetail` gates, `MetadataUnread*` vocabulary, coverage mechanics (`gaps→unverified` lowering), PublicSQL.
- Aggregate `coverage`/`verdict` asserted only in full-request-observed matrix rows; per-statement finding counts are per-statement, never batch totals.

## D — In-batch ordering and fact invalidation

`objectCache` is a per-enrich-call memo of **pre-state reads**; it is not lifecycle knowledge. Consumption validity is adjudicated per fact **before** any cached or provider answer is used: `contaminated` → `srsExistenceStale[srid]` → `srsDepsStale` → then the bind/cache answer. A read issued *during* the current statement's own bind is still only a **pre-DDL database** read — re-querying cannot prove the post-DDL simulated state.

The two flags have **disjoint writers** (R2-D fix):

| writer class | `srsExistenceStale[srid]` | `srsDepsStale` |
|---|---|---|
| audited `SRSKindDrop` on S2 (uncertain effect) for `srid` | set for that `srid` | not set |
| boundary/unsupported SRS lifecycle write (`CREATE`, `CREATE OR REPLACE`, `DROP` — any SRID-bearing form that did not audit) | set for that statement's `srid` | not set |
| `spatialTouching` table/schema write (enumeration below) | not set | set (batch-wide) |
| refusal arms S0/S1, arm B | never | never |

Rationale: a DROP/CREATE/boundary SRS statement writes only the SRS registry — it never creates, drops, or modifies a geometry column, so it cannot invalidate a dependency witness. A table/schema write writes only geometry-column space — it never deletes an SRS catalog row, so existence stays consumable. Flags are monotonic within the batch (never cleared) and live on `batchState`, persisting across enrich calls.

**D1 — same SRID:**

| prior statement | modeled effect | effect on later same-SRID reads |
|---|---|---|
| audited DROP, non-refusal arm (S2) | unknown (may have executed) | `srsExistenceStale[SRID]=true` → next DROP hits arm **B** |
| audited DROP, refusal arm (S0/S1) | provably unchanged | none — facts stay consumable; a repeated identical DROP refuses identically |
| boundary CREATE / CREATE OR REPLACE / DROP (unaudited, version-gated) | unmodeled — may have written | `srsExistenceStale[SRID]=true` → arm **B** |
| arm **B** outcome | unchanged stale | stays **B** (stale is monotonic within the batch) |

A refused statement never mints staleness; a non-refused or boundary one always does (existence only). No absent/publication path exists — nothing publishes `OrderedAbsent`/`OrderedPresent` for SRS this slice.

**D2 — different SRID:** identity is numeric SRID only — no SERVER-style name folding. A write to SRID `a` sets `srsExistenceStale[a]` and nothing else: SRID `b` keeps every consumable fact (its own existence, the shared dependency space *unless* a table/schema write also intervened). `srsDepsStale` is batch-wide by design: dependency space is a single shared catalog across all SRIDs, so a table/schema write demotes witnesses for every SRID — per-SRID dependency invalidation is not modeled.

**D3 — `spatialTouching` closed enumeration (R3-B narrowing).** A statement is dependency-invalidating iff it matches a touching class below. The inert set is the **narrow explicit list** — there is no "anything unmatched is inert" fallback: a statement whose table/schema effect is unmodeled, partial, or unknown defaults to **touching**.

| class | verdict | basis |
|---|---|---|
| `DROP TABLE`, `TRUNCATE TABLE`, `RENAME TABLE` | touching | removes/renames dependent columns outright |
| `ALTER TABLE` carrying **any** action outside the inert option set — column-identity/type actions (`add_column`, `drop_column`, `change_column`, `modify_column`, `rename_column`, `alter_column` setters), `rename_table`, index/constraint/partition actions, or an unmodeled/partial action list | touching | may drop, retype, or rename the dependent geometry column — or the action set cannot be proven complete |
| `ALTER TABLE` whose **complete** modeled action list contains only `engine` / table-option members | inert | provably no column mutation (the action list is fully modeled and every member is storage-layer only) |
| `CREATE TABLE` with a **complete modeled column list** containing **zero** spatial-family base types (`geometry`, `point`, `linestring`, `polygon`, `multipoint`, `multilinestring`, `multipolygon`, `geometrycollection` — `advanced_indexes.go:707–712`) and no `SRID` column clause | inert | authored column list provably adds no dependent column |
| `CREATE TABLE` with **any** spatial-family column or `SRID` clause, `CREATE … LIKE`, `CREATE … AS SELECT`, or any partial/unmodeled column set (CTAS `ColumnsTailUnknown`, LIKE-source columns unvouched) | touching | may mint a dependency — completeness cannot be assumed |
| `DROP SCHEMA`, `ALTER SCHEMA`, schema `RENAME`-class writes | touching | removes/renames the schema containing dependents |
| `CREATE SCHEMA` | inert | an empty container provably holds no dependent column |
| audited statements whose modeled kind provably carries no table/schema effect — index/view/SERVER/resource-group/sequence/placement/instance families | inert | their modeled effect space cannot write geometry columns |
| boundary/unmodeled statement whose normalized model **carries table targets** (`TableTargets()≠∅` — e.g. `ALTER TABLE t COMPACT TIFLASH REPLICA`, a vendor boundary under MySQL per `coverage_boundary.go:189–194` with `Table` populated at `extractor.go:813–832`) | touching | model-provable target + unmodeled effect — no SQL-text guessing |
| boundary/unmodeled statement with `TableTargets()=∅` but a table-affecting modeled kind (feature/kind identifies a table write the extractor did not normalize) | touching | identification is model-level (kind/feature), never text inspection |
| statement with no table targets and a provably non-table kind | inert | nothing to touch |
| parser failure record before the statement | touching-by-contamination | whole-batch `contaminated` (§E9) — stronger than stale |

Non-table statements never override SRS lifecycle writes: an SRS boundary write still sets `srsExistenceStale` even in a batch that otherwise touches nothing.

**D4 — sequencing and contamination:**

- Authored order governs: a failure record at `(line,col)` contaminates every statement **after** it by `failureBefore` (`batch_state.go:3724–3729`), not by list position; the authored position of each record is preserved in the failure record.
- A trailing failure with **no** later statement still sets `contaminated` for subsequent enrich calls on the same `batchState` (the R2-accepted tail sweep, `metadata.go:705–707`).
- Cache hits do not launder staleness: the second bind of a same-SRID statement sees `srsExistenceStale` **before** consulting the memoized answer (arm B exits at bind; reads=0).
- A stale dependency flag plus a still-valid positive existence → S2 owed `{srs.column_dependencies, srs.drop.privilege}` — existence stays consumed, dependency stays owed, never a false absence.

## E — Case matrix

`case-matrix.md` — uniquely expanded rows over the 12 use-case families, shared concrete fixtures, and three `PLAN/UNEXECUTED` CLI probes with complete commands, separated streams, and exact expected results.

## F — Execution seams, static checks, errata

Baseline excerpts delivered in `baseline-source-excerpts.txt` (R3 set — each excerpt carries the baseline commit identity, path, and verbatim line numbers; only the seams this contract's claims actually need are included, no blanket "all references attached" claim):

| seam | path : lines |
|---|---|
| whole-input DROP claim + SRID decode | `internal/infrastructure/parser/tidb/mysql_srs.go:47–131, 263–272` |
| extraction → typed declaration (Operation/ObjectType/Name) | `internal/infrastructure/parser/tidb/extractor.go:123–135` |
| coverage boundary classification (SRS implementation/vendor) | `internal/infrastructure/parser/tidb/coverage_boundary.go:83–103` |
| COMPACT vendor boundary under MySQL | `internal/infrastructure/parser/tidb/coverage_boundary.go:189–194` |
| COMPACT extraction populating `Table` | `internal/infrastructure/parser/tidb/extractor.go:813–832` |
| `TableTargets` accessor | `internal/domain/spec/ddl.go:184–197` |
| `ALTER` action facts (`spec.Alter` members) | `internal/domain/spec/ddl.go:944–978` |
| geometry family vocabulary | `internal/domain/spec/advanced_indexes.go:707–712` |
| G2 enrich version gate (SERVER today) | `internal/application/audit/metadata.go:243–266` |
| resolvedVersion source + stamped metadata | `internal/application/audit/metadata.go:155–175` |
| version pin preflight + observed resolution | `internal/application/audit/service.go:274–310`; `internal/domain/spec/version.go:45–46, 63–75, 115–160` |
| enrich loop: failure consumption + bind → apply ordering + tail sweep | `internal/application/audit/metadata.go:376–430, 690–710` |
| `bindMySQLSRS` + contaminated early-exit | `internal/application/audit/batch_state_server_srs.go:129–150` |
| `objectResolverFor` + per-call objectCache | `internal/application/audit/metadata.go:332–371` |
| SRS loader: version() probe, read firing | `internal/infrastructure/metadata/mysql/provider_server_srs.go:94–107, 129–199` |
| existence + dependency SQL bodies | `internal/infrastructure/metadata/mysql/provider_server_srs.go:259–286` |
| `SpatialReferenceSystemState` + `Complete=false` | `internal/domain/spec/srs.go:96–171` |
| `SRIDName` + redacted `PublicSQL` | `internal/domain/spec/srs.go:175–208` |
| `Metadata.SpatialReference` member | `internal/domain/spec/metadata.go:110–120, 772–778` |
| unsupported SRS apply = no-op (outer branch) | `internal/application/audit/batch_state.go:457–465` |
| `failureBefore` authored ordering | `internal/application/audit/batch_state.go:3724–3729` |
| privilege loader trigger (SERVER-only) | `internal/application/audit/server_drop_premise.go:26–37`; `metadata.go:318–329` |
| bounded error wrapper + `Unwrap` | `internal/application/audit/metadata.go:1021–1056` |
| unsupported → `$.unsupported` projection + verdict aggregation | `internal/application/audit/evaluate.go:27–54`; `internal/domain/report/result.go:176–190` |
| report JSON shapes (StatementResult/Result/Finding/EvidenceGap/UnsupportedDetail) | `internal/domain/report/result.go:27–46, 85–120`; `internal/domain/rule/rule.go:35–77`; `internal/domain/spec/statement.go:100–106` |
| CLI flags, policy enablement, `newUserError`, `failOnTriggered`, exit dispatch | `internal/interfaces/cli/audit.go:38–42, 60–70, 99–103, 184–204, 586–600`; `internal/interfaces/cli/cli.go:13–60`; `internal/interfaces/cli/root.go:46–47` |

Static checks executed this round (records in the archive): `git status`/`diff` scope checks, `git diff --check`, file byte counts, SHA-256 sums — each logged with its actual command and immediate rc. **Not executed (docs-only order):** `go test`, `go build`, `go vet`, `gofmt`, any CLI invocation, any SQL/native execution.

**Errata register (records preserved, never rewritten):**
- **E-1 (doc date):** the R1 §B privilege-basis line printed `verified 2026-11-15`; the actual session fetch ran on the worker's local date **2026-11-12**. Corrected above; the R1 archive keeps its original record unmodified.
- **E-2 (R1 grep rc=1):** the R1 `static-checks.txt` grep-pipeline record ending `rc=1` is a normal no-match exit (pipeline semantics), not a failed read. Registered, not rewritten.
- **E-3 (R1 `git status --porcelain` rc=1):** the same R1 record separately logs a `git status --porcelain` line at `rc=1`, which the no-match explanation cannot account for. The exact command that produced it is not recoverable from the record; it is registered here as an **unexplained/mislabeled historical capture** — left as-is, not speculated upon.
- **E-4 (R2 `.go` count label):** R2's `static-checks.txt` recorded `git diff --name-only … | grep -c '\.go$' || true; echo "…grep rc=$?"` — the `0` output is `grep -c`'s real no-match count and the printed `rc` was the `|| true` composite's `0`, not grep's own `1`. The label is corrected here; the `.go changed=0` fact stands independently on the diff itself.
