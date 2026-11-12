# T26-SRS-DROP-PROVEN-ABSENT-CONTRACT-01 — R3 revision (docs-only contract)

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
| R1 candidate | `aceaa48d0ef358e71aae89909cda8193aa8cd24b` (preserved) |
| R2 candidate | `23cea286844774274cfaf602a6c8c4973e8c046c`, tree `065039ee0d3371b222f7e0416852854d5128bd37` (preserved) |
| this round | `docs/verification/t26-srs-drop-proven-absent-contract-01/{README.md,case-matrix.md}` — the only two paths touched |
| semantic base | SRS DROP contract `d68f4ab7`; SRS DROP execution `128d4595`; SRS privilege contract `2f7231b5`; SRS privilege execution CLOSED/R3-ACCEPTED `56bcf05f` |
| bound evidence | `t26-srs-drop-privilege-execution-r3.tar.gz` 352,624 B / `f7667e08…3393b` + supplement 26,297 B / `cdc54363…d500a1`; R1 archive 23,751 B / `bab8e289…fbd7`; R2 archive 71,349 B / `0a06aa37…404d` |

R2 retained: explicit-Rows clean-zero supply direction; the
QueryContext/iteration-error vs clean-EOF distinction; dependency error
delivering no partial snapshot; order B → S0 → S1(valid witness) → A →
S2; the three absent-arm projections; absent arms minting/clearing no
stale and publishing no OrderedAbsent; per-enrich cache lifecycle with
the changed-second-answer discriminant; the four immutable pins; and
the verified repo originals, diffs and member manifest.

## 1. R3-A — target identity, version data flow, supply proof

### A1. ValidAbsence — authored-SRID bound

```
ValidAbsence(state, authoredSRID)  ⇔
    state != nil
 && state.SRID == authoredSRID
 && state.Read
 && !state.Exists
 && state.Absence
```

`state.SRID` (`internal/domain/spec/srs.go:98`) is part of the consumed
identity: the baseline request key does **not** validate the returned
snapshot's `state.SRID`, so a resolver that answered a different SRID
must be caught by the consumer-side conjunction — the earlier
"foreign-SRID snapshot fails the cache key" attribution is withdrawn.

Registered resolver/bind negative row (unit-constructed, layer: unit —
a real mysqlmeta provider is not claimed to produce this mismatch):
authored `100001`; returned state `SRID=100002, Read=true,
Exists=false, Absence=true`; no valid positive deps; `P=consumable`;
`IF EXISTS=true`. Expected: does NOT enter A; the bounded input yields
S2 with owed `[E,D]` (existence unknown as bound, deps unwitnessed —
privilege itself consumable, so P absent from owed), the existing
S2-only privilege notice fires, and own-apply mints `srsExistenceStale`
for the authored `100001` only.

### A2. Version sources — corrected table

| domain | decided at | version input | membership |
| --- | --- | --- | --- |
| D-qualify (audit admission) | version gate, `metadata.go:274-287` | `resolvedVersion` — the request's **observed** banner parse; a pin is compared against observed and mismatch exits via `ErrTargetVersionMismatch` (`metadata.go:141-163`); pin is never substituted for observed | `MySQLSRSDropAuditedDomain`: MySQL, ValidatedRange, 8.0.x patch≥4 (`srs_drop.go:89-101`) |
| D-support (catalog readability) | `mysqlSRSCatalogSupport`, `provider_server_srs.go:94-107` | the provider's own `select version()` banner (Qo, one query) | proven banner, `Major>=8` |
| D-proof (negative-fact completeness) | `loadMySQLSRSObject` existence-read call point | the **same** provider Qo parse from D-support — one observation, two judgements | proven && `Major==8 && Minor==0 && Patch>=4` |

Today `mysqlSRSCatalogSupport` returns only `supported/proven/error`;
the parsed banner version is not retained. Registered touchpoint
(UNEXECUTED): keep or return the parsed provider version from that Qo
call so D-proof can be evaluated at the same call point — Qo stays a
single `select version()` per resolve; the request version never
substitutes for provider proof.

### A3. depsStale wording + conflict reachability — corrected

`srsDepsStale` semantics: a demoted dependency witness **loses S1
consumption eligibility; the input is then adjudicated by A or S2 per
their own conditions**. B's two established sources stay
contamination and same-SRID existence-stale — dependency staleness is
never a B cause.

Conflict reachability (existence zero + positive deps at the real
provider layer): explained solely by the two reads being independent
observations at different instants — a concurrent DDL between them can
legitimately produce it. The withdrawn alternative ("the privilege-
filtered dependency view could show a row the complete existence view
missed") is deleted: the existence view is a bare projection, and that
mechanism cannot produce the contradiction.

### A4. Whole-range proof — auditable per-node evidence

`official-evidence-r3.txt` + `official-raw/history/` carry the
auditable basis: the file was fetched at **every one of the 11
history commits** on the 8.0 branch (endpoint bound: branch HEAD
`9fc8d60003b4eee32767c1e30942f222c3508c31`, whose file equals the
newest history node `e08842d61c29…`). Per-commit constructor
extraction and token normalisation (exact pipeline disclosed in the
evidence file) yields **one identical digest `4d86eb9a…9edde` at all
11 nodes**, including merge commit `711dfe7…` (compared directly at
the merge commit itself) and the clang-format commit `4491eaf7…`.

Three things are thereby separated: (i) the four immutable pins agree
(direct byte/normalised comparison); (ii) the audited historical
range is invariant — every release on the 8.0 branch, hence the whole
audited 8.0.4+ domain, ships this identical definition; (iii) version
admission remains its own gate — future or unverified versions never
inherit this proof. Claim boundary: the 8.0 series only; nothing is
asserted about 8.4+/9.x.

## 2. R3-A3 supplement — supply predicate (unchanged, retained)

Explicit `*Rows` flow, design only: `QueryContext` error → propagate;
`Next()` iteration → `rows.Err()` non-nil → propagate; `Next()==false
&& Err()==nil && Close()==nil` → clean zero-row triple → `Absence`
iff in D-proof. Go 1.26.1 stdlib decisive source archived verbatim
(`sql.go:3486` Row.Scan order; `3029` Rows.Next; `3136` Rows.Err).
Driver-returned `sql.ErrNoRows` lands on the error path, never the
zero-row branch. Registered distinguishing rows 1–6 unchanged.
Registered touchpoint: `provider_server_srs.go:146-169`
QueryRowContext → explicit Rows; partial helper state is never a
published fact — only the snapshot returned to
`resolveObjectSnapshot`/`bindMySQLSRS` counts.

## 3. Frozen adjudication & state model (R2-retained, unchanged)

- First-match **B → S0 → S1 → A → S2**; S1 requires a valid non-stale
  positive dependency witness; absent arms refuse/consume exactly as
  the table in §2 of the R2 record (refused_absent / absent_noop /
  absent_conditional; P consumed only on A-IE✓; privilege notice stays
  S2-only; new gap reason `unproven_privilege` carries `[P]`).
- Absent arms mint/clear nothing; per-enrich objectCache; L gate
  independent of arm; no OrderedAbsent.
- `refused_absent` is this audit model's refusal basis — no native
  check-ordering commitment.

## 4. Canonical public projection (R2-retained, unchanged)

| arm | findings | gaps | stmt/agg coverage | verdict | conditional_audit |
| --- | --- | --- | --- | --- | --- |
| A-plain | 1 `absent` blocker | 0 | complete | reject | absent |
| A-IE✓ | 1 `absent_noop` notice | 0 | complete | pass | `="observed_account_new_session"` |
| A-IE? | 0 | 1 dependency-owned gap `unproven_privilege`, ordered `[srs.drop.privilege]` | unverified | review | absent |

Serialisation skeleton frozen on the real path: `srid` = in-memory
`uint32` → JSON number; `outcome` exact strings `refused_absent` /
`absent_noop`; `conditional_audit` JSON string; gap object = exactly
`rule_id`/`reason_code`/`required_facts`; empty slices follow existing
omitempty shape. Reserved guard: `reserved` co-fires on A-IE✓/A-IE?
(membership-only metadata), excluded on A-plain; S2 privilege notice
never extends to A. Registered touchpoints: reserved-rule arm guard,
`unproven_privilege` reason, new rules + registry/default/example,
`IfExists` into `AdjudicateSRSDrop`, provider explicit-Rows + retained
provider version (A2), `Absence`/`SRID`-bound ValidAbsence in spec.

## 5. Errata register

| item | correction |
| --- | --- |
| R1 static-checks `git log --format=%H %s -5` display line | display text lost shell quoting; actual argv was `git log "--format=%H %s" -5` (single-argument format) — corrected capture + argv statement in R3 supplement; R1/R2 logs untouched |
| R2 static-checks `gofmt -l internal/ pkg/` | run this round — registered as a beyond-docs-only read-only check with NON-empty output (`mysql_t26r5_test.go`, pre-existing, unrelated); no rc-implies-empty inference |
| `official-evidence-r2.txt` branch-file SHA | transcription error: written `dd1fd25f…569e2c3` (66 chars); correct `dd1fd25fbce5622c1409a9d242210845038d1ab9a210f07bd82c773743b569e2` |
| "intro + ten mechanical" | arithmetic: 11 commits = introducing + 10 mechanical (copyright×6? — corrected ledger in evidence: copyright-year ×4, GPL×3, clang-format×1, include-paths×1, merge×1) |
| normalised constructor digest `de945460…`/`4d86eb9a…` | exact pipeline now disclosed in evidence (awk range + whitespace normalisation + punct strip + sha256); raw-file SHAs and direct comparisons retained as the primary record |
| D-qualify source position | corrected table in §A2 (resolvedVersion/observed vs provider Qo — no invented third read) |
| PA-6c mixed-dialect + PA-7/PA-8/PA-9/PA-10 witness defects | rewritten in `case-matrix.md` R3 |
| over-broad completion claims ("全矩阵已闭合" style) | withdrawn — see §6 status vocabulary |

## 6. Status vocabulary (R3-F)

- **CONTRACT READY**: supply predicate, identity binding, branches,
  state and observation expectations are unique — submitted for
  independent ruling.
- **IMPLEMENTED/VERIFIED**: **NO** — everything remains PLAN /
  UNEXECUTED; the explicit-Rows provider change, ValidAbsence binding,
  arm plumbing and rule registrations are registered touchpoints only.
- **EXECUTION AUTHORIZED**: **NO** — awaits a separate execution work
  order after this contract's acceptance.
- Remaining open item (explicit): the registered provider read-path
  change and its distinguishing rows; nothing else is claimed open or
  closed beyond this list.

## 7. Out of scope (unchanged)

Ordered publication, present-object DROP success, SRS CREATE auditing,
general dependency-completeness, ALTER INSTANCE, T27. Dependency reads
stay witness-only forever under this contract. No push/merge/tag/
release/close.
