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

## Sources

- [PostgreSQL 17 ALTER TABLE](https://www.postgresql.org/docs/17/sql-altertable.html)
- [PostgreSQL 17 identifier syntax](https://www.postgresql.org/docs/17/sql-syntax-lexical.html)
- [PostgreSQL 17 system columns](https://www.postgresql.org/docs/17/ddl-system-columns.html)
- [PostgreSQL 17.7 collision and rename implementation](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/tablecmds.c)
- [D7 target catalog contract](2026-11-18-postgresql-column-storage-compression-targets.md)
