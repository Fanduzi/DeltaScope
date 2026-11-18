# Decision: PostgreSQL replica identity index attachment

Date: 2026-11-18
Status: Accepted
Milestone: F09 / `milestone/pg-alter-semantics` (D9)
Product baseline: `8cff67d49058b339f5cea96f85f03a2e548f9418`

## Decision

Enable `ddl.pg.alter.replica_identity_using_index.valid` at blocker level
with no parameters. It applies to PostgreSQL `ALTER TABLE … REPLICA IDENTITY
USING INDEX`. Use a fresh exact schema-qualified binding with actual PostgreSQL
17 evidence and an ordinary, persistent, non-typed table. An unqualified name,
three-part identity, unsupported kind, altered batch state or unobserved index
stays unverified. A proven absent guarded table has no index premise.

The provider resolves the named relation in the target table's namespace. The
candidate must be an index of that exact relation, with a uniqueness-capable
access method, unique and immediate enforcement, no expression or predicate,
and positive NOT NULL key attributes. For the access-method premise, the
qualified built-in btree property is evidence. Other methods may override the
property callback, so it cannot prove their native capability. Included
attributes are outside the key
check. Read only bounded catalog flags and exact identity. Neither predicates,
index expressions, OIDs nor provider errors belong in findings. Gaps use
`unproven_replica_identity_index`.

Keep the existing notice with wording about the requested configuration. The
existing DEFAULT, FULL and NOTHING behavior stays in place. Assignment and
replication usability are separate. The user approved the attachment blocker
and selected this slice without a runtime availability warning. Invalid or
unready indexes can still receive the replica-identity flag and that flag alone
does not certify logical decoding. The runtime dimension remains in the queue.

## Baseline evidence

At `cca065cd`, an independent PG17.7 probe ran 19 cases with 9 successful
assignments/mode changes and 10 exact refusals. A second probe paired six native
shapes with 12 real CLI audits. Four native refusals still received audit pass
with the old notice. Both probes checked row/catalog/guard state and owned
cleanup. The INCLUDE control succeeds with nullable included columns, while a
nullable key refuses even with `NULLS NOT DISTINCT`. An invalid and unready
unique index left by a failed concurrent build accepts assignment and retains
both flags. Research artifacts: `/tmp/ds-pg-f09-rid-research-cca065cd/evidence.json`
and `/tmp/ds-pg-f09-rid-cli-cca065cd/evidence.json`.

## Fact-only verification

The targeted reader was exercised through the new PG17.7 CLI on six native
paired shapes (12 offline/live audits). All catalog reads completed. Audit
behavior remains the D8 baseline until the rule commit.

## Frozen T37 additions

- `T37.cli.replica-index-offline`
- `T37.cli.replica-index-disabled`
- `T37.cli.replica-index-other-modes`
- `T37.meta.pg-replica-index-eligible`
- `T37.meta.pg-replica-index-binding`
- `T37.meta.pg-replica-index-structure`
- `T37.meta.pg-replica-index-keys`
- `T37.meta.pg-replica-index-invalid-build`
- `T37.meta.pg-replica-index-freshness`
- `T37.meta.pg-replica-index-scope`
- `T37.meta.pg-replica-index-wrong-owner`
- `T37.meta.pg-replica-index-wrong-kind`
- `T37.meta.pg-replica-index-deferred`
- `T37.meta.pg-replica-index-expression`
- `T37.meta.pg-replica-index-partial`
- `T37.meta.pg-replica-index-guard-absent`

The six additional request-level controls keep each assessment fresh. Earlier
DDL in a single request deliberately withholds later index observations.

Each comparison retains its separate native refusal and no-audit-execution
oracle. Use actual catalog identity, row/file preservation and a sequence guard.
The existing case IDs and profiles are preserved.

## Evidence bindings

The focused T37 replay passed 189/189 with 3922 assertions. All 173 prior
cases and profiles are preserved. The 16 additions use one isolated policy
profile and 31 native execution controls (15 successful statements and 16
exact refusals). The invalid-build test independently verifies that the initial
failed build left an invalid and unready unique index, and that native
assignment marked it without changing eligibility for replication. The first
replays identified fixture issues (punctuation in unquoted schema names and a
setup command issued through `psql -c`). Fixture corrections retained every
required ID and native assertion. The final SHA still requires full verification.

D9 changes only the original F09 REPLICA IDENTITY evidence cell. Remaining
runtime usability, broader relation kinds, other table options, dependencies
and permissions stay in the queue.

## Sources

- [PostgreSQL 17 ALTER TABLE](https://www.postgresql.org/docs/17/sql-altertable.html)
- [PostgreSQL 17 pg_index](https://www.postgresql.org/docs/17/catalog-pg-index.html)
- [PostgreSQL 17.7 attachment checks](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)
- [PostgreSQL 17.7 replica identity index selection](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/utils/cache/relcache.c)
