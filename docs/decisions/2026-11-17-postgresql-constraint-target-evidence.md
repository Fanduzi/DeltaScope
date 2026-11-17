# Decision: PostgreSQL constraint-target facts and conservative freshness

Date: 2026-11-17
Status: Accepted
Family: F09 (partial tranche), canonical T37
Delivery base: `8aa9e3c88a6441c0df0273d03ecf0efcddb6c5d6`

## Context

The preceding D3 slice governs authored PK/FK migration risk, not constraint
identity or kind. Fifteen PostgreSQL 17.7 native shapes and 30 real offline/live
CLI probes show missing/wrong-kind VALIDATE, wrong-kind ALTER CONSTRAINT and
unguarded missing DROP still receive notices rather than target checks.
Native refusals are 42704 (missing), 42809 (wrong kind), 23514/23503 (data) and
2BP01 (dependency). These categories must not be conflated.

The PG provider's legacy `Constraints` contains PK facts only. PostgreSQL does
not participate in the MySQL/TiDB ordered prospective-state pass. Reusing an
old catalog after ADD/DROP would falsely reject ADD→VALIDATE or trust the
removed member after DROP→VALIDATE. Case-folded/dot-concatenated metadata keys
also cannot distinguish quoted PG identities reliably.

## Decision and approved behavior

The operator explicitly approved these **default-enabled blocker** contracts:

| Rule | Bounded check |
| --- | --- |
| `ddl.pg.alter.validate_constraint.target.require` | Proven target exists and is FK/CHECK |
| `ddl.pg.alter.constraint_deferrability.target.require` | Proven target exists and is FK |
| `ddl.pg.alter.drop_constraint.target.require` | Proven missing target refuses unless authored IF EXISTS |

Unknown/incomplete/ambiguous facts owe evidence gaps, not guessed absence or
a blocker. This can raise offline pass to review. Proven already-validated
VALIDATE and proven absent DROP IF EXISTS stop receiving misleading old
scan/removal notices; unknown cases use conditional wording. Existing FK/PK
forbids and NOT VALID→VALIDATE follow-up remain independent. The shared
DROP→PK matcher must compare PostgreSQL constraint names exactly, so a
quoted unique `"CK"` is not misidentified as a distinct primary key `ck`;
MySQL/TiDB case-folding and genuine PK forbids remain unchanged.

### Fact seam

`TableSnapshot.PGConstraints` is a separate, nonserialized directory of names,
`pg_constraint.contype` and `convalidated`. The legacy PK projection remains
unchanged. Target-kind conclusions are scoped to an observed stable PostgreSQL
17 banner from the existing `InstanceFacts.Version` read. Missing/out-of-range
version facts owe `instance.version` gaps. The public `VersionIdentity`
contract deliberately remains MySQL/TiDB-only; it is not widened, and a caller
`target_version` cannot substitute for the observed server fact. nil means unobserved/stale; non-nil empty means observed empty.
The provider publishes only after a successful exact schema/table-bound read,
collects no expressions or definitions and preserves every row. Query/scan/
iteration/close errors propagate rather than producing an empty catalog.

`FindPGConstraint` compares decoded names exactly. Duplicate names (including
identical duplicate records), unnamed members or an unknown target kind keep
proof unowned; no last-wins or case-insensitive fallback. Snapshot cloning
preserves nil/empty distinctions and never aliases mutable member slices.

DROP CONSTRAINT retains AST `if_exists=true|false` and `cascade=true|false`.
Omission and RESTRICT share the same false behavior; no parser widening or
raw SQL inference is needed. PG metadata resolves an explicit DDL schema
before the request fallback and caches by an exact `(schema, table)` tuple.

### Accepted stage boundary

This tranche **does not enable general PostgreSQL ordered execution**.
Related prior table DDL withholds later private constraint directories.
Opaque/unsupported/parser-failure content, omitted identities, non-table DDL
whose effects are not modeled here, and effects that may reach another
identity/dependent table conservatively end freshness more broadly. Provider snapshots and earlier statements remain immutable.
Same-statement preceding ALTER actions must likewise withhold later target
conclusions in the rule assessment; no post-state is invented.

This stage boundary was explicitly accepted. It is not a permanent deferral,
whole-family completion or an excuse to treat a missing fact as supported.
The other F09 dimensions remain queued. Target proof does not certify data,
permissions, dependency removals, index eligibility, inheritance/partition
operations, transaction execution or successful native execution. Transaction
wrappers and the existing #149 candidates keep their unsupported/parser-error
records; no parser release is required or adopted here.

## Public and privacy contract

New directory/freshness facts stay out of serialized metadata. The public Go
`PGConstraintState` alias lets SDK metadata providers supply the bounded
name/native-kind/validated tuple; SDK cloning preserves owned member slices
and the nil/known-empty distinction, without adding JSON envelope fields. New findings
may expose bounded table/schema/constraint identity, action, target kind and
proof outcome, never expressions, raw provider errors or unrelated payloads.
SDK, CLI, HTTP and MCP consume one shared rule/assessment path. No database
mutation, data scan or execution simulation occurs during audit.

## Frozen evidence roles before rule implementation

Keep all 79 existing T37 required IDs/profiles. Add these 15 exact cases;
expectations come from the contract/native oracle, not measured finding totals:

- `T37.cli.constraint-target-validate-offline`
- `T37.cli.constraint-target-deferrability-offline`
- `T37.cli.constraint-target-drop-offline`
- `T37.cli.constraint-target-drop-guard-offline`
- `T37.cli.constraint-target-disabled`
- `T37.cli.constraint-target-mysql`
- `T37.cli.constraint-target-tidb`
- `T37.cli.constraint-target-parser-retention`
- `T37.meta.pg-constraint-target-validate`
- `T37.meta.pg-constraint-target-deferrability`
- `T37.meta.pg-constraint-target-drop`
- `T37.meta.pg-constraint-target-prior-ddl`
- `T37.meta.pg-constraint-target-check-data`
- `T37.meta.pg-constraint-target-fk-data`
- `T37.meta.pg-constraint-target-prior-parser`

Native cases distinguish missing/wrong-kind, known existing/no-op, exact
quoted-name selection, stale prior-DDL/parser facts and independent data
refusals. Driver-only mutation follows audit non-execution checks; catalog
and sentinel controls precede ownership-guarded cleanup. Data refusals are
boundaries, not successful target-legality certification. Canonical manifests,
corpus and exact inventory bindings are delivered with the rule tranche.

## Verification at the fact-preservation stage

- Spec tests pin unknown/empty/exact-case/duplicate/unmodeled bindings and JSON
  privacy. Provider tests pin complete/empty reads and query/scan/iteration
  error propagation without publishing partial facts.
- Application tests pin exact schema/case/dot-safe caches, immutable prefix/
  provider snapshots, prior DDL/opaque/parser-failure invalidation and clone
  nil/empty proof. Parser tests pin guard/CASCADE/RESTRICT and quoted names.
- The real CLI is rebuilt with PostgreSQL support; the 15-shape/30-CLI native
  research loop replays on the changed provider without mutating fixtures.

At this stage, `go build ./...`, `go test ./...`, PostgreSQL-tagged full
suite, default/PG lint and inventory/corpus/coverage-catalog gates pass. The
unchanged 79-case T37 baseline passes with 836 assertions. These focused
pre-commit checks are not borrowed as final-HEAD aggregate evidence.

The rule implementation now registers the approved blocker trio and optional
old-notice no-op suppression. Domain/application/SDK/HTTP/MCP tests pin shared
blocker, silent and unverified-gap arms; corpus covers known invalid targets,
known no-ops, exact-case selection, prior DDL, unknown facts, disabled rules
and non-PG controls. A real 15-shape/30-CLI replay confirms six target refusals
become blockers, guarded absence is silent and ADD/DROP successors retain
catalog gaps; independent 23514/23503 data refusals remain outside target proof.
The shared PK matcher regression is reproduced red before the exact-PG-name
fix and green afterward, retaining MySQL/TiDB behavior. Before the rule
implementation commit, full build/default+PG tests, inventory/corpus/catalog
gates, default+PG lint and formatting pass with the new rules enabled.

These are implementation/focused checks, not final single-HEAD aggregate
readiness or shipped new rule behavior. The frozen canonical/native cases
and exact inventory bindings still belong to the evidence tranche; the full
required delivery chain must pass before merge/push.
The final rule/evidence delivery must independently pass the full goal gates.

## Sources and links

Verified 2026-11-17. Native anchor remains PostgreSQL 17.7; parser remains
pg_query_go/v6.2.2 / libpg_query 17. No dependency/target-version upgrade.

- [PostgreSQL 17 ALTER TABLE](https://www.postgresql.org/docs/17/sql-altertable.html)
- [PostgreSQL 17 pg_constraint](https://www.postgresql.org/docs/17/catalog-pg-constraint.html)
- [17.7 tablecmds.c](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)
- [D3 decision](2026-11-17-postgresql-alter-constraint-build-risk.md)
- [Program ledger](../plans/2026-11-17-rule-backlog-ledger.md)
