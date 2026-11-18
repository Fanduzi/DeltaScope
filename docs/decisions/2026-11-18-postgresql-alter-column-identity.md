# Decision: PostgreSQL ALTER column identity and existence guards

Date: 2026-11-18
Status: Accepted
Milestone: F09 / `milestone/pg-alter-semantics` (D8)
Product baseline: `b966f73c76352507a8dcf03b01063e0f43255564`
Research HEAD: `3d8a5b87db34c0fe3fa388a89fbd926100af9c54`

## Decision

Reuse the default blocker levels, parameters and IDs of
`ddl.alter.add_column.exists.forbid`, `ddl.alter.rename_column.exists.require`,
`ddl.alter.rename_column.target.exists.forbid` and `ddl.table.exists.alter.require`
for the approved PostgreSQL target checks. The existing rename forbid and
independent declaration policies still apply. Findings identify the action,
target names and reason. Guard and relation observations stay private in JSON.

On observed PG17 with an exact fresh ordinary persistent column catalog, ADD
rejects a duplicate user name without a column guard. An observed existing user
column with `IF NOT EXISTS` settles the target check silently and suppresses the
ADD notices that describe an actual addition. A real system column causes a name
collision regardless of the column guard. RENAME rejects a missing or inapplicable
system source and an occupied destination, including a rename to the same exact
name. Quoted identifiers keep their decoded case and spaces. The destination
check requires a proven applicable source. Unknown relation, column, scope or
version premises remain evidence gaps.

The table guard belongs to the target relation, separately from the ADD column
guard. A system-catalog observation over all relation kinds can prove a named
relation present or absent. A filtered table snapshot's `Exists` value alone
cannot make that distinction. True absence of the qualified relation under
`ALTER TABLE IF EXISTS` settles existence silently. For unqualified names, a
schema-local absence does not prove search-path-wide absence. An observed object
of another kind discharges the existence question only; child action eligibility
remains independent. Earlier DDL can create implicit relations such as indexes
or sequences, so it invalidates the relation observation even when the earlier
statement's primary table differs. Column checks reuse the conservative existing
pre-action freshness rules. Catalog proof never simulates a post-state or changes
the session search path.

## Evidence

On PostgreSQL 17.7, 49 statement shapes produced 22 successes and 27 exact
refusals, with 196 CLI audits split between default and isolated policies,
offline and metadata-aware. The owned driver checked attributes, data, view text,
relation files and guard state before and after each audit and native rollback.
Evidence: `/tmp/ds-pg-d8-column-identity-scope-v3-3d8a5b87/evidence.json`.
The isolated profile exposed duplicate ADD, exact RENAME, same-name conflict,
quoted-space false absence and prior-DDL stale snapshot failures independently of
the existing blanket rename forbid. Raw AST probes independently distinguish
ADD's column guard and both ALTER/RENAME table guards that normalized identically
in the baseline. A guarded system-name collision remains SQLSTATE `42701`.
Sequence, index and view fixtures distinguish filtered collection misses from
relation absence. Column-level `RENAME COLUMN IF EXISTS` remains `42601`.

## Frozen T37 additions

The existing 156 IDs and profiles remain. The following additive IDs cover the
new proof without extending table-kind, expression or dependency scope:

- `T37.cli.column-identity-add-offline`
- `T37.cli.column-identity-rename-offline`
- `T37.cli.column-identity-table-guard-offline`
- `T37.cli.column-identity-disabled`
- `T37.cli.column-identity-rename-column-guard-parser`
- `T37.meta.pg-column-identity-add-duplicates`
- `T37.meta.pg-column-identity-add-guard`
- `T37.meta.pg-column-identity-add-system`
- `T37.meta.pg-column-identity-rename-source`
- `T37.meta.pg-column-identity-rename-destination`
- `T37.meta.pg-column-identity-rename-quoted-spaces`
- `T37.meta.pg-column-identity-table-guard-absent`
- `T37.meta.pg-column-identity-table-guard-other-kind`
- `T37.meta.pg-column-identity-table-guard-unqualified`
- `T37.meta.pg-column-identity-prior-ddl`
- `T37.meta.pg-column-identity-prior-parser`
- `T37.meta.pg-column-identity-scope`

TYPE / USING conversion, dependencies, permissions, table kind eligibility,
inheritance/partition semantics, general prospective order, and the remaining
F09 dimensions stay in the ledger. A column-name result does not certify those
independent premises.

## Canonical evidence and compatibility

The initial implementation changed five existing T37 expectations because the
ADD and RENAME rules now report missing live facts offline. The original 156
cases retained their SQL, profiles and finding assertions, and passed with
2794 assertions. The T36 live partitioned ADD still reports its existing notice
and now carries an explicit ordinary-table evidence gap.

The first additive T37 replay at `c94a15da` with evidence edits passed 171/173.
Two multi-statement fixtures expected later relation observations to remain fresh.
The expectations now pin the additional gaps and retain the same native oracles.
The corrected replay passed 173/173 with 3368 assertions. The 17 new cases
include 32 native steps (17 successes and 15 exact SQLSTATE refusals). Each live
case compares relation/file identities, attributes, an exact payload and a
sequence position before native execution. The prior-DDL case also proves that
native DROP can execute ahead of ADD even when ADD was written first.

The column inventory row stays mixed; only the RENAME COLUMN child and anchor
classification moves from notice-only to mixed. Existing IDs, profiles and
inventory denominators remain attached to their original scopes.

## Typed tables found in read-only review

The separately reviewed `cca065cd` CLI reported complete coverage for three
ADD/RENAME statements on `CREATE TABLE … OF`. Native PG17.7 refused each with
`42809` before a column guard could take effect. This exposed a gap in the
scope check because a typed relation can be persistent, `relkind=r` and have
no inheritance edges. D8 now gates on the system-qualified `reloftype`
observation as well. Prior column-rule scope continues to use its existing
contract. The existing T37 scope case verifies the three native refusals and
the preserved type binding and payload. The focused candidate replay passed
173/173, 3380 assertions and 35 native steps (17 successes and 18 refusals).

The `cca065cd` full runner was stopped before further edits. Its exit 143 is
an interruption. Cleanup first failed because Compose needs an interpolation
value. The retry used an ephemeral value, removed the owned project, and
confirmed all 16 original container identities. The final candidate needs a
fresh full run and a separate review.

## Exact relation binding found in read-only review

At `96b4e3bd` the live provider found the requested schema, but the extractor
had discarded the database component of a three-part name. A cross-database
qualified guarded ADD reported complete coverage even though PG17.7 refused
it with `0A000`. This slice leaves database-qualified statements unverified
under `target_table.pg_database`, since schema-local evidence has no database
binding. The AST keeps this distinction privately. The all-space quoted table
name also now reaches its exact relation and column reads, so the original
native `42701` duplicate has a matching target finding. The member query uses
system-qualified comparisons when binding scope, namespace and attributes.

The previous full runner was stopped before editing (exit 143); owned cleanup
returned zero with no residual containers. The two augmented T37 controls keep
all existing IDs and prior oracles. The focused replay passed 173/173 with 3390
assertions and 37 native steps: 17 successes and 20 exact refusals.

## Metadata preparation without a schema flag

A separate real CLI review at `1fb0372b` found that metadata preparation
rejected a guarded unqualified missing relation before rules ran. It also
trimmed an all-whitespace quoted table prior to schema discovery. PostgreSQL
DDL targets now remain exact at preparation, and a guarded schema inference
miss reaches the normal evidence gaps. An unguarded miss retains the existing
schema-hint error. The existing T37 unqualified absence and quoted-name cases
now exercise the actual no-schema CLI path with the same native oracles.
The targeted replay again passed 173/173 and 3390 assertions.
The `1fb0372b` all run stopped before edits (exit 143), followed by owned-only
cleanup with no residual containers and the original 16 container identities
preserved.

## Delivered verification

D8 shipped at `8cff67d49058b339f5cea96f85f03a2e548f9418` from base
`b966f73c76352507a8dcf03b01063e0f43255564` via fast-forward and normal push.
The final canonical run passed 50 tasks, 3505/3505 cases and 62429 assertions,
with zero failures or skips. Every task and CLI build used the shipped SHA.
Artifact validation and all owned cleanup passed. T36 passed 59/59 with 660
assertions. T37 passed 173/173 with 3390 assertions, including 37 new native
steps (17 successes and 20 precise refusals). The independent read-only
review, candidate gates and merged-root gates passed. The original workspace,
protected release worktree and 16 unrelated container identities remained
intact. The last task changed seven paths and the milestone changed 59.

- [Go, PostgreSQL and lint CI](https://github.com/Fanduzi/DeltaScope/actions/runs/44360977800)
- [CLI TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/44360977946)
- [Pages](https://github.com/Fanduzi/DeltaScope/runs/135002167971)

Final artifact: `/tmp/ddl-golden-pg-column-identity-final-8cff67d4/all/artifact.json`.
Task counts and CI refer to the shipped product SHA. The three interrupted
candidates remain historical. Other F09 dimensions stay open.

## Sources

- [PostgreSQL 17 ALTER TABLE](https://www.postgresql.org/docs/17/sql-altertable.html)
- [PostgreSQL 17 identifier syntax](https://www.postgresql.org/docs/17/sql-syntax-lexical.html)
- [PostgreSQL 17 system columns](https://www.postgresql.org/docs/17/ddl-system-columns.html)
- [PostgreSQL 17.7 collision and rename implementation](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)
- [D7 target catalog contract](2026-11-18-postgresql-column-storage-compression-targets.md)
