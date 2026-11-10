# T26-R2 SRS validation scope, internal representation, decision table

> **R4 addendum (issued under T26-R4 — supersedes the R3 addendum's
> completion wording).** The table remains a *target* design, not an
> approved oracle: **no row below is approved semantics yet** — all 33
> numbered rows are unapproved design cells until the SRS implementation
> round adjudicates them against oracle evidence. The R3 sentence "`complete`
> means the checked set was checked" is **replaced** by this contract:
> 1. `complete` is claimable only when every check the adjudication
>    actually requires — and that the supported surface promises — has
>    run;
> 2. a capability the implementation does not cover gets a named
>    implementation boundary / incomplete, never silent omission;
> 3. an external fact the adjudication needs but lacks produces the
>    applicable bounded gap / unverified per the T04 contract;
> 4. provider-query-failure stays an error path — never a gap;
> 5. PENDING ORACLE cells produce no deterministic result or post-state
>    influenced by them;
> 6. a proven refusal may keep its terminal verdict without demanding
>    unrelated later premises;
> 7. an unknown execution outcome is never written as "exact pre-state
>    preserved" — known public facts are kept, unknown affected facts
>    propagate unknown.
> In the current implementation stage every recognized SRS statement
> still emits `coverage=incomplete` + the named implementation boundary —
> no row's findings, gaps, or post-state ship.

> **R3 addendum (issued under T26-R3, implementation stage).** The table
> below is the *target* adjudication design — none of it is implemented
> in R3. R3 ships recognition + named boundaries + output redaction only.
> Status vocabulary for every row below: `S`=static parse fact,
> `V`=version fact, `M`=metadata fact. An `M` premise resolves to exactly
> one of the premise states **proven-present / proven-absent / unknown /
> provider-not-loaded / provider-query-failure** — provider failure is an
> error path, never a gap; `unknown` and `absent` never collapse.
> `partial-coverage` is a coverage *verdict*, not a premise state — an
> unproven premise yields the applicable bounded gap plus that coverage
> outcome, it never resolves as the premise itself.
> Rows marked **PENDING ORACLE** (INE precedence, OR REPLACE absent-arm)
> produce no deterministic post-state until source evidence lands
> ~~every `complete` cell means "the checked set was checked"~~
> *(superseded — the R4 addendum above carries the effective
> `complete` contract)*; unexecuted layers keep their bounded unknowns,
> never a complete claim. In R3 the
> emitted outcome is uniformly `coverage=incomplete` + named
> implementation boundary — no findings, no gaps, no post-state.


Proposal status: **awaiting reviewer ruling**. Ruling 五 applied: internal
representation must carry what promised semantics need; product output
never echoes DEFINITION; length/newline checks alone do NOT suffice;
unverified parts stay named gaps — not repackaged as EvidenceGaps.

## 1. What "complete" promises — layered scope

| layer | scope | evidence class | complete means |
|---|---|---|---|
| L1 SQL outer declaration | verb form (plain/`OR REPLACE`/`IF NOT EXISTS`/`DROP [IF EXISTS]`), SRID literal, attribute presence/duplication, string lengths | **static** (recognizer) | every documented static rule checked: SRID u32+SRID0+reserved-range; NAME nonempty+edge-whitespace+≤80; DEFINITION present+≤4096+no-newline; ORGANIZATION+IDENTIFIED BY pairing+≤256; DESCRIPTION≤2048; no duplicate attributes |
| L2 DEFINITION structure & legality | WKT grammar, GEOGCS 2-AXIS rule (8.0.13+), mandatory projection parameters where defined | **static, partially implementable** | WKT parse succeeds + documented clauses checked; unknown projection methods are NOT auto-refused (docs allow unknown methods — PENDING ORACLE for refusal set) |
| L3 existence & declared-attr conflicts | SRID existence; NAME uniqueness; (ORGANIZATION,IDENTIFIED BY) pair uniqueness | **metadata** (bounded facts) | three-valued facts consumed, gaps named when unproven |
| L4 column dependency | referencing columns in `ST_GEOMETRY_COLUMNS` | **metadata** | dependency fact three-valued; refusal vs proceed vs gap |
| L5 lifecycle & ordered state | create/replace/drop post-state | **ordered-state** | post-state only after the same adjudication row decides |

`coverage=complete` requires L1 fully checked + every metadata premise
the row consumes being proven. L2 partial verification is stated
honestly: "well-formedness checked" vs "definition valid" are different
claims ~~; complete-coverage only claims the checked set~~
*(closing clause superseded — the R4 addendum's seven-point `complete`
contract is the effective definition)*.

## 2. Internal representation proposal

`[proposed] spec.SpatialReferenceDeclaration`:

```go
type SpatialReferenceDeclaration struct {
    Kind          SRSKind            // create | create_or_replace | drop
    SRID          uint32             // authored SRID
    SRIDReserved  bool               // in a documented reserved range
    IfNotExists   bool
    IfExists      bool
    Attrs         SRSAttrSet         // presence + derived facts below
    Duplicates    []string           // duplicated attribute names
}
type SRSAttrSet struct {
    Name         *SRSName      // value retained (identity, ≤80 after check), edge-ws flag
    Definition   *SRSDefinition // see below — controlled internal rep
    Organization *SRSOrg       // name + identified-by pair, pair-conflict key
    Description  *SRSDesc      // length only? — description is inert text, retain-or-drop is a ruling question (propose drop; only len kept)
}
type SRSDefinition struct {
    Raw        string `json:"-"`  // INTERNAL ONLY — serializer-excluded, needed for WKT parse
    WKTValid   *bool           // nil=unchecked
    AxisOK     *bool           // GEOGCS two-axis rule, nil when not GEOGCS/not checked
    Length     int             // ≤4096 check
    HasNewline bool
}
```

- `Raw` is parse-stage-only: consumed by the WKT checker, never assigned
  to any serialized field; the struct is declared inside `spec` with a
  `json:"-"` tag AND the output contract (G-doc) forbids any echo path
  reaching it — defense in depth, not tag-only.
- Alternative if reviewer prefers zero retention: run WKT validation in
  the recognizer and keep only derived booleans + a bounded diagnostic
  code (e.g., `wkt_err_class`). Downside: richer refusal reasons lose
  their basis. **Worker recommendation: keep internal `Raw`, exclude
  from output** — the redaction boundary is at the output edge, not the
  parse edge (R2 ruling 六.1: internal input vs external representation
  separated).
- NAME/ORGANIZATION are identities → retained verbatim (T25 precedent);
  DESCRIPTION is inert → propose drop, keep length only (data
  minimization).

## 3. Verification capability sources

| need | reuse | implement | unresolved |
|---|---|---|---|
| WKT/SRS-def grammar parse | none in repo today — **new implementation needed** (bounded parser for documented clauses) | SRS-def checker recognizing documented WKT subset + documented clause rules | full WKT spec coverage — unknown/unsupported projection methods must produce `unverified`-class uncertainty, never auto-refusal |
| reserved-range check | `spec` numeric helpers | range table from docs | — |
| existence/name-org/dependency | provider pattern from T21/T25 `load*Object` | 3 new bounded readers (`r2-metadata-facts.md`) | visibility/privilege completeness per fact |
| version gate 8.0.13 | `spec.VersionIdentity` + `TablespaceVersionOutcomeFor` precedent | `SRSVersionOutcomeFor` | — |
| INE ordering | — | **PENDING ORACLE** — source read or authorized native probe | all INE rows marked pending |

Explicit non-goals restated: no coordinate transforms, no GIS engine, no
claim that "one fixture accepted" generalizes to all definitions, no
auto-refusal of unrecognized projection methods.

## 4. Complete decision table (supersedes R1 19-row)

Verdict levels: blocker → reject; warning/notice → review (+floor rules).
`cov` = coverage. `post` = ordered post-state (`srs[srid]`). Premise
source: S=static, M=metadata, V=version, O=ordered-pre-state.

| # | row | pre-conds (source) | findings | gaps | verdict | cov | post | branch priority |
|---|---|---|---|---|---|---|---|---|
| 1 | srs-parse-invalid | grammar fail (S) | — | — | — | incomplete (parser_error diag) | unchanged | 0 — precedes all |
| 2 | srs-dup-attr | ≥1 duplicate attr (S) | blocker `[p] srs.attr.duplicate` | — | reject | complete | unchanged | 1 — static violations first |
| 3 | srs-missing-attr | NAME or DEFINITION absent (S) | blocker `[p] srs.attr.missing` | — | reject | complete | unchanged | 1 |
| 4 | srs-len-violate | any cap exceeded (S) | blocker `[p] srs.attr.length` | — | reject | complete | unchanged | 1 |
| 5 | srs-newline-def | DEFINITION has newline (S) | blocker `[p] srs.def.newline` | — | reject | complete | unchanged | 1 |
| 6 | srs-srid0 | SRID=0 (S) | blocker `[p] srs.srid.reserved0` | — | reject | complete | unchanged | 1 |
| 7 | srs-srid-reserved | reserved range ≠0 (S) | warning `[p] srs.srid.reserved` | — | review | complete | unchanged* | 2 — *still proceeds if rest passes: warning coexists with successful create — post=present |
| 8 | srs-org-pair-incomplete | ORGANIZATION w/o IDENTIFIED BY or vice versa (S) | blocker `[p] srs.org.pair` | — | reject | complete | unchanged | 1 |
| 9 | srs-def-malformed | WKT parse fail (S) | blocker `[p] srs.def.malformed` | — | reject | complete | unchanged | 3 — after attr-level checks |
| 10 | srs-def-axis | GEOGCS missing two-axis (8.0.13+) (S+V) | blocker `[p] srs.def.axis` | — | reject | complete | unchanged | 3 — version-floor consumed |
| 11 | srs-def-unknown-method | WKT ok, projection method unrecognized (S) | warning `[p] srs.def.method_unverified` | — | review | **unverified** | unchanged | 3 — uncertainty honest, not refusal |
| 12 | srs-version-57 | dialect=mysql, version<8.0 proven (V) | — | — | — | incomplete + vendor-boundary unsupported entry | unchanged | 0.5 — version boundary precedes semantics |
| 13 | srs-version-unknown | version unproven (V) | — | `srs.version_applicability` | — | unverified | unchanged | 0.5 — cannot adjudicate 8.0-only rules on unknown version? **or** static checks apply anyway? **[RULING NEEDED]** — propose: static-only checks still run (they're version-independent), version-dependent (axis) skipped with gap |
| 14 | srs-tidb | dialect=tidb (V) | — | — | — | incomplete + boundary=vendor | unchanged | 0.5 |
| 15 | srs-create | plain CREATE, existence=absent (M), statics pass | notice `[p] srs.create.notice` | — | review | complete | present | 4 |
| 16 | srs-create-dup | plain CREATE, existence=present (M) | blocker `[p] srs.create.dup` | — | reject | complete | unchanged | 4 |
| 17 | srs-create-unknown | plain CREATE, existence=unknown (M) | notice + gap | `srs.existence` | review | unverified | unchanged | 4 |
| 18 | srs-ine-absent | IF NOT EXISTS, absent (M) | notice | — | review | complete | present | 5 |
| 19 | srs-ine-present | IF NOT EXISTS, present (M) | notice (documented no-op+warning) | — | review | complete | unchanged | 5 |
| 20 | srs-ine-pend | IF NOT EXISTS, existence=unknown (M) | notice + gap | `srs.existence` | review | unverified | unchanged | 5 — **PENDING ORACLE: does validation precede existence check?** affects whether invalid attrs in an INE-statement-on-present-SRID are refused or ignored |
| 21 | srs-replace-absent | OR REPLACE, absent (M) | notice — **PENDING ORACLE**: create-like vs refused | — | review (provisional) | complete | present (provisional) | 5 |
| 22 | srs-replace-undep | OR REPLACE, present, deps proven-absent (M) | notice | — | review | complete | replaced | 6 |
| 23 | srs-replace-dep | OR REPLACE, present, deps proven (M) | blocker `[p] srs.replace.dep` | — | reject | complete | unchanged | 6 |
| 24 | srs-replace-depunknown | OR REPLACE, present, deps unknown (M) | notice + gap | `srs.column_dependencies` | review | unverified | unchanged | 6 |
| 25 | srs-drop-ok | DROP, present, deps proven-absent (M) | warning `[p] srs.drop.warn` | — | review | complete | absent | 6 |
| 26 | srs-drop-dep | DROP, present, deps proven (M) | blocker `[p] srs.drop.dep` | — | reject | complete | unchanged | 6 |
| 27 | srs-drop-depunknown | DROP, present, deps unknown (M) | warning + gap | `srs.column_dependencies` | review | unverified | unchanged | 6 |
| 28 | srs-drop-absent | DROP, absent (M) | blocker `[p] srs.drop.absent` | — | reject | complete | unchanged | 6 |
| 29 | srs-drop-ie-absent | DROP IF EXISTS, absent (M) | notice | — | review | complete | unchanged | 6 |
| 30 | srs-drop-unknown | DROP, existence unknown (M) | warning + gap | `srs.existence` | review | unverified | unchanged | 6 |
| 31 | srs-name-conflict | CREATE, NAME exists on other SRID (M) | blocker `[p] srs.name.conflict` | — | reject | complete | unchanged | 4 — needs `srs.name_org_conflict` fact |
| 32 | srs-org-conflict | CREATE/REPLACE, (org,id) pair exists on other SRID (M) | blocker `[p] srs.org.conflict` | — | reject | complete | unchanged | 4/6 |
| 33 | srs-batch-order | O: prior stmt in batch changed `srs[srid]` | per consuming row | — | per row | per row | per row | O feeds pre-state of rows 15+ |

Rows marked **PENDING ORACLE**: 20, 21 — plus L2 method-recognition
breadth (row 11's refusal set). These stay uncertain until fixed-version
source or authorized native evidence arrives; no short-circuit assumed.

## 5. IF NOT EXISTS ordering — registered

Unproven: whether MySQL validates attributes before the existence
short-circuit. Consequence if validation-first: an INE-create with
malformed DEFINITION on a present SRID would refuse (row 19 becomes
reject). Consequence if existence-first: it no-ops. Both orders are
documented-plausible; the rows stay dual until evidence. Resolution
paths: (a) fixed-version MySQL source read [SRC], (b) one native case in
the authorized plan (plan only).
