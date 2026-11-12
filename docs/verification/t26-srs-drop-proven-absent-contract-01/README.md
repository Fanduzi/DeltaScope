# T26-SRS-DROP-PROVEN-ABSENT-CONTRACT-01 — R4 revision (docs-only contract)

Candidate contract for promoting a clean zero-observed-row
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
| R1 candidate | `aceaa48d0ef358e71aae89909cda8193aa8cd24b` (preserved) |
| R2 candidate | `23cea286844774274cfaf602a6c8c4973e8c046c` (preserved) |
| R3 candidate | `46a1f4728aa681a9777e788cd49185274630d5ab`, tree `b160bffd34c0ff3b16aa7e6f917388adbcb2ec23` (preserved) |
| this round | `docs/verification/t26-srs-drop-proven-absent-contract-01/{README.md,case-matrix.md}` — the only two paths touched |
| semantic base | SRS DROP contract `d68f4ab7`; SRS DROP execution `128d4595`; SRS privilege execution CLOSED/R3-ACCEPTED `56bcf05f` |
| bound evidence | privilege exec R3 archive 352,624 B / `f7667e08…3393b` + supplement 26,297 B / `cdc54363…d500a1`; PA R1 23,751 B / `bab8e289…fbd7`; PA R2 71,349 B / `0a06aa37…404d`; PA R3 67,191 B / `264cd0ed…adc5` |

R3 retained: authored-SRID ValidAbsence conjunction; single-Qo
D-support/D-proof sharing with registered version-retention
touchpoint; depsStale = loss of S1 eligibility then A/S2 adjudication;
B sources limited to contamination and same-SRID existence stale;
B → S0 → S1(valid witness) → A → S2; A arms mint/clear nothing, no
OrderedAbsent; S2-only privilege notice; A-IE✓ marker carried by its
own `absent_noop` finding; the 11-node historical completeness proof;
same-enrich cache, double-enrich changed-second-answer, PA-7 two
observation layers, reserved guard and the surviving policy rows.

## 1. R4-A — supply predicate and version data flow

### A1. Clean-zero-observed-row supply predicate (restored)

A delivered snapshot may carry `Absence=true` only when ALL of:

1. `QueryContext` succeeded (no query error);
2. **zero observed rows** — `Next()` never returned `true` for this
   result (a clean `Next()==false` AFTER a positive row is only EOF,
   not a zero-row answer);
3. `rows.Err() == nil` after iteration;
4. the checked `rows.Close()` returned nil;
5. the same provider Qo parse satisfies D-proof (§A2).

A positive row sets `Exists=true`, `Absence=false`. Query / scan /
iteration / cancellation / close errors propagate on the registered
error path — an error whose value equals `sql.ErrNoRows` never
supplies absence. A dependency read failure delivers no partial
snapshot. The distinguishing rows (clean zero in-domain; QueryContext
error = `sql.ErrNoRows`; iteration error = `sql.ErrNoRows`; generic
query/read error; cancellation; existence-OK-then-deps-error) and the
positive-row control remain registered as PA-1/PA-4g/PA-4h/PA-9
rows. Go 1.26.1 stdlib decisive source stays archived verbatim.

### A2. D-qualify sources — actual resolvedVersion per entry path

D-qualify consumes the actual `resolvedVersion`; how it is produced
differs by entry:

| entry | production of `resolvedVersion` | decisive source |
| --- | --- | --- |
| providerless + pin | `request.Provider == nil` short-circuit passes `request.TargetVersion` directly — the pin-derived identity (source=target) IS the resolved version | `metadata.go:111` → `enrichOrderedStatements(..., request.TargetVersion, ...)` at `metadata.go:138` |
| providerless, no pin | same branch with `TargetVersion=nil` → resolvedVersion=nil → domain fails → boundary | `metadata.go:111-138` |
| provider-present | `LoadInstanceFacts` → `ParseObservedVersion(banner)` → product check vs dialect (`ErrDialectProductMismatch`) → optional pin compare (`ErrTargetVersionMismatch`) → `resolvedVersion = observed` — in THIS branch observed is the authoritative input and a pin never substitutes for it | `metadata.go:141-163` |

Domain predicate: `spec.MySQLSRSDropAuditedDomain(version)` — the
function at `srs_drop.go:111` (`version != nil && Product==mysql &&
ValidatedRange && Major==8 && Minor==0 && Patch>=4`). The
`SRSDropAdjudication` struct at `srs_drop.go:93` is NOT the domain
predicate; earlier citation of the struct location is corrected here.

D-support / D-proof share the one provider Qo parse
(`mysqlSRSCatalogSupport`, `provider_server_srs.go:94-107`):
`ParseObservedVersion(banner)` nil / `!ValidatedRange` /
`Product != mysql` → `(supported=false, proven=false)`; else
`Major>=8` → supported, proven. The parsed provider version is not
currently retained — registered touchpoint (UNEXECUTED): return it
from the Qo call so D-proof (`Major==8 && Minor==0 && Patch>=4`)
evaluates at the same call point. Qo stays a single
`select version()`; request version never substitutes provider proof.

A request pin in the providerless path is a legitimate admission
input for D-qualify, but it is never a provider proof input: PA-1a
(providerless, pin in-domain) still yields S2 `[E,D,P]` because no
existence read exists to carry `Absence`.

### A3. ValidAbsence (retained) + invalid-absence discriminants

```
ValidAbsence(state, authoredSRID)  ⇔
    state != nil && state.SRID == authoredSRID
 && state.Read && !state.Exists && state.Absence
```

`state.SRID` (`srs.go:98`) is consumed identity — the baseline request
cache key does not validate the returned snapshot's SRID.

Registered invalid-absence discriminants (all PLAN/unit unless noted):
- foreign SRID: authored `100001`, returned `SRID=100002, Read, ¬Exists,
  Absence` → not A; see PA-4e (spec-unit) and PA-4e2 (enrich-apply +
  public companion via a deliberate custom `ObjectResolver` at the bind
  seam — a real mysqlmeta provider is not claimed to produce the
  mismatch).
- `Absence=true && Exists=true` (restored): contradictory pair —
  ValidAbsence conjunction fails on `!Exists`; the `Exists=true` present
  witness governs → S2 owed `[D]`; `Absence` ignored.
- `Read=false, Exists=false, Absence=true`: read never happened →
  existence unknown → S2 owed `[E,D]`; `Absence` ignored.

## 2. Frozen adjudication & state model (retained)

- First-match **B → S0 → S1 → A → S2**; valid non-stale positive deps
  keep S1; `srsDepsStale` loses S1 consumption eligibility only.
- A-plain `refused_absent` `[]` no P; A-IE✓ `absent_noop` `[]` P
  consumed; A-IE? `absent_conditional` `[P]` P unconsumed; S2 keeps its
  four combinations; privilege notice stays S2-only; new gap reason
  `unproven_privilege` carries `[srs.drop.privilege]`.
- `refused_absent` is this audit model's refusal basis — no native
  check-ordering commitment.
- Absent arms mint/clear no stale channel, publish nothing ordered,
  keep per-enrich objectCache; loader gate stays independent of arm.

## 3. Canonical public projection — frozen literals (R4-D2)

Applies to a **single, clean, non-reserved SRS statement, default
policy**. Multi-statement rows keep statement-level projection and
batch aggregate as separate observations. RuleID/level/message/
metadata live ONLY here; matrix rows reference this section.

**A-plain** — findings=1, gaps=0, statement+aggregate coverage
complete, verdict reject, no `conditional_audit`:

```json
{"rule_id": "ddl.mysql.srs.drop.absent",
 "level": "blocker",
 "message": "spatial reference system 100001 does not exist",
 "metadata": {"srid": 100001, "outcome": "refused_absent"}}
```

`metadata.srid` serialises as a JSON number (in-memory `uint32`);
metadata key set is exactly `{srid, outcome}`.

**A-IE✓** — findings=1, gaps=0, complete, verdict pass:

```json
{"rule_id": "ddl.mysql.srs.drop.absent_noop",
 "level": "notice",
 "message": "spatial reference system 100001 does not exist; IF EXISTS drop is a no-op",
 "metadata": {"srid": 100001,
              "outcome": "absent_noop",
              "conditional_audit": "observed_account_new_session"}}
```

**A-IE?** — findings=0; exactly one gap; coverage unverified, verdict
review; no `conditional_audit`, no public internal outcome:

```json
{"rule_id": "ddl.mysql.srs.drop.dependency",
 "reason_code": "unproven_privilege",
 "required_facts": ["srs.drop.privilege"]}
```

### Real JSON paths and field-existence rules (baseline-bound)

Public skeleton (`pkg/deltascope/audit.go:251-289`,
`report/result.go:80-118` — minimal excerpts in
`baseline-source-additions.txt`):

```json
{"verdict": "...", "coverage": {...}, "summary": {...},
 "statements": [{"index": 0, "kind": "...",
   "findings": [ ...finding objects... ],
   "evidence_gaps": [ ...gap objects... ],
   "coverage": {...}}]}
```

- `statements[i].index` has NO omitempty — always present, `0`
  included; `findings`/`evidence_gaps` carry `omitempty` — an empty
  slice is OMITTED entirely (absent key, never `[]` or `null`).
- `finding.statement_index` is `omitempty` — omitted at index 0.
- The marker is the metadata key `conditional_audit`: a missing key
  differs from `null`/empty-string; no null marker is ever emitted.
- `metadata` is `map[string]any` `omitempty` — key set and JSON types
  are exactly the literals above; `srid` is a JSON number.
- gap object = exactly `rule_id`/`reason_code`/`required_facts`
  (`required_facts` omitempty — present here, ordered `[P]`).
- No change to accepted serializer behaviour is proposed.

Reserved / policy variants (complete): `4326` A-plain emits NO
reserved advisory; `4326` A-IE✓ / A-IE? each co-fire exactly one
membership-only `ddl.mysql.srs.drop.reserved` notice carrying only its
established `{srid}` metadata — no outcome/marker added; the S2
privilege notice never extends to A. PA-10d: `absent` rule disabled on
a non-reserved plain absent → findings=0, gaps=0, complete, pass,
internal adjudication/own-apply unchanged. PA-10e: `absent_noop`
level→warning on IF EXISTS → warning → review, marker preserved.

## 4. Errata register

| item | correction |
| --- | --- |
| R3 %q git-log capture | accepted as THIS round's capture; the inference "it proves R1/R2 actual argv" is withdrawn — R1/R2 argv originals were not retained → reported/uncaptured; no reconstruction, no rerun |
| `gofmt -l internal/ pkg/` | occurred in the R2 round — "R2 run / R3 registered": R2's static-checks.txt holds the capture (non-empty: `mysql_t26r5_test.go`, pre-existing, unrelated); R3 registered it as a beyond-docs-only overstep; no R3 run, no rerun, no rc→empty inference |
| history-node classification arithmetic | corrected ledger: introducing 1, copyright-year 5 (2021/2022/2024/2025/2026), GPL/licence 2, clang-format 1, include-paths 1, merge 1 = 11 |
| completeness vs admission | the 11-node proof covers content up to the bound HEAD `9fc8d600` (fetch date); the `Patch>=4` numeric admission is a separate frozen policy — it does not by itself reject future 8.0 patch releases, and no allowlist or future-version evidence is added |
| git-log display line | R1/R2 static-checks echo lines lose shell quoting — registered, originals untouched |
| earlier attribution errors | foreign-SRID cache-key claim withdrawn (R3); "permission-filtered dependency view" conflict theory deleted (R3); domain-predicate citation corrected to `srs_drop.go:111` (this round) |

## 5. Status vocabulary (R4-E5)

- **CONTRACT READY**: supply predicate, identity binding, arm table,
  state and observation expectations are unique — submitted for
  independent ruling.
- **IMPLEMENTED/VERIFIED**: **NO** — everything remains PLAN /
  UNEXECUTED.
- **EXECUTION AUTHORIZED**: **NO** — awaits a separate execution work
  order.
- Registered implementation backlog (explicit, not loop-reasons):
  explicit-`Rows` provider read, provider-version retention for
  D-proof, `Absence` carrier + `ValidAbsence` conjunction, `IfExists`
  into `AdjudicateSRSDrop`, the two new rules + reserved arm guard +
  `unproven_privilege` reason + registry/default/example/policy
  touchpoints.
- Remaining contract items: none beyond this round's registered
  backlog — each work-order item is answered in `case-matrix.md`.

## 6. Out of scope (unchanged)

Ordered publication, present-object DROP success, SRS CREATE auditing,
general dependency-completeness, ALTER INSTANCE, T27, version
allowlists, future-version evidence. Dependency reads stay
witness-only. No push/merge/tag/release/close.
