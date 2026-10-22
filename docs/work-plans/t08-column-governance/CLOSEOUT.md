# T08 closeout index — #87 acceptance mapping

Status: T08 implementation delivered and self-verified through R1/R2/R3;
external Reviewer acceptance is NOT claimed by this index. It maps each #87
acceptance line to the shipped implementation, frozen roles, proof artifacts,
and code/evidence SHAs. It does not close the issue and does not re-run
anything — all artifacts below are the SHA-bound originals.

## Delivered SHAs

| Piece | Commit | Notes |
|---|---|---|
| T08 implementation (position, multi-action state, effective defaults) | `9a18e143` | code |
| T08 frozen manifest + independent contract oracle + validator | `6fb8e438` | test |
| T08 evidence at `6fb8e438` | `493f8d47` | evidence-only |
| T08 review pass 1 (TiDB AI-removal tombstone, accumulated AFTER set) | `9c4fe175` | code |
| T08 evidence at `9c4fe175` — 172/172, 4240 assertions | `4762fb44` | evidence-only |
| T08 review pass 2 (duplicate-apply guard, produced-set correctness) | `c983fc9d` | code |
| T08 delivery closeout evidence at `c983fc9d` | `18f34abb`, `c8bd6da4` | evidence-only |
| 11-cell oracle migration to the frozen T08 contract | `3b35fe30` | test |
| Oracle-closure evidence at `3b35fe30` (T03/T04/T05 green) | `922771cd` | evidence-only |
| T08-R1 three-state comparable-identity comparison | `1d2c2fa4` | code |
| T08-R1 evidence (red capture, 5-manifest golden, 16-cell probe) | `23364f09` | evidence-only |
| T08-R2 canonical-integer numeric domain | `fb16cf6d` | code |
| T08-R2 evidence (red capture, 3-manifest golden, 8-cell probe) | `ff2364ec` | evidence-only |
| T08-R2 review pin-cells + §5.4-R2 equal-bytes clarification | `27a4355b` | test+docs |
| T08-R2 review run logs | `docs/verification/t08/27a4355b…/review/` | evidence-only |
| T08-R3 year-normalized literals leave the comparable domain | `7e251a67` | code |
| T08-R3 bounded YEAR probe (pre-fix red + post-fix green) | `docs/verification/t08/7e251a67…/native-probe/` | evidence-only |

## #87 acceptance → evidence

1. **A1 — ALTER COLUMN SET/DROP DEFAULT silent path + consistent
   explicit-attribute policy with MODIFY/CHANGE** —
   `internal/application/audit/batch_state*` setter publication and
   `ddl.alter.column.explicit_*`/`ddl.column.default.require` separation
   (§5.5: an omitted DEFAULT clause never counts as an authored default
   operation); cells in `testdata/ddl-golden/T08.json`, real-entry
   `audit_t08r1_default_comparison_test.go`, state-chain tests.

2. **A2 — narrowing, type-family, signed/unsigned, NULL, auto_increment,
   position, multi-column and mixed actions; missing prior-value evidence
   explicitly unverified** —
   `internal/domain/rule/ddl/alter_compatibility_rules.go` (type family,
   width, signedness, nullability, auto_increment gates all fact-gated;
   unknown source type never enters the checks), `metadata_rules.go`
   position members, `batch_state` multi-action ordering. Frozen oracle:
   `testdata/ddl-golden/T08.json` + `roles.json`.

3. **A3 — metadata-aware existence/compatibility covers present,
   confirmed-absent, and missing evidence; offline unknown is never
   absent** — provider-sourced snapshots, `unknown_table_state`/
   `missing_source_column`/`incomplete_source_column` gap vocabulary,
   one-load-per-table call ledgers pinned in tests.

4. **A4 — per-action source/target identity and declared position
   preserved; source→target updates feed later checks** — `spec.Column`
   transition pairs, CHANGE old-name read, ordered-state publication of
   column position (`FIRST`/`AFTER` accumulated-set semantics), position
   oracle cells including the 11 migrated pre-T08 cells.

5. **A5 — four anchors with post-change catalog oracle; default and
   custom policy plus PostgreSQL regression** —
   `docs/verification/t08/<sha>/` evidence sets: golden runs against
   mysql:5.7.44/8.0.46/8.4.10 and pingcap/tidb:v8.5.0 with pre/post
   catalog reads; default/all-off/isolated policies; PG unit gates
   unaffected.

## How the rounds correspond

- **Position oracle migration** (`3b35fe30`): 11 pre-T08 oracle cells
  re-bound to the frozen contract — recorded under §"Old-golden oracle
  migration" in EXPECTED-UPGRADES and proven in `922771cd`.
- **R1** (`1d2c2fa4`): the shared three-state `defaultComparison` —
  presence flips and class switches determined, literal identity via
  `DefaultLiteral` kind+value, expression identity via known fingerprints,
  unproven identities bounded gaps. 16-cell native probe + 5-manifest
  golden at `23364f09`.
- **R2** (`fb16cf6d`): `numeric` kind marks the value domain, not a shared
  encoding — different-text pairs convict only inside the canonical-
  integer domain. 8-cell probe + 3-manifest golden at `ff2364ec`.
- **R2 review** (`27a4355b`): pinned the intentional equal-bytes
  short-circuit (scale-0 canonical cell, declared-decimal bound,
  type-finding division); test+docs only.
- **R3** (`7e251a67`): the canonical-integer domain is proven only where
  neither column type normalizes the literal on storage — MySQL `YEAR`
  maps `DEFAULT 1`→`2001`, so a different-text numeric pair touching a
  year column is `unproven_default_identity`, not a conviction. Native
  pre-fix false-positive capture and post-fix bounded-gap proof at
  `docs/verification/t08/7e251a67…/native-probe/`; TiDB 8.5.0 stores the
  declared integer verbatim (boundary is MySQL-family-specific).

## Frozen knowledge boundaries (accepted, not defects)

- `unproven_default_identity` is an honest bounded gap for known-but-
  incomparable literal pairs; it never fabricates a fact deficit or a
  determined change.
- Numeric-literal identity is provable only as canonical-integer text on
  non-normalizing column types. DECIMAL scale text (`1.00`), leading
  zeros, signs, declared decimal literals, and `year`-column integer text
  all stay unproven. All-numeric-types-safety is NOT claimed: the
  proven domain is INT-family and scale-0 stored integers as tested.
- YEAR arithmetic is not implemented; pairs needing year conversion are
  honest gaps. A different-text year-vs-non-year pair is also unproven
  (provider-final vs declaration-raw text is not distinguishable in the
  fact shape).
- Equal recorded bytes are same unconditionally — byte identity needs no
  encoding reasoning.
- The T06-A4 source boundary stands: the catalog cannot recover which
  source spell produced a stored NULL-able default.

## Scope not committed

- No numeric normalizer/evaluator, no year conversion, no widening of
  the comparable domain without a proven cross-source encoding.
- Index/constraint/TTL governance belongs to T09/#88; T08 artifacts are
  not reused as T09 evidence.
