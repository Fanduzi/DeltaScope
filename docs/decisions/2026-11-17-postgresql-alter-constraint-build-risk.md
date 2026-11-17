# Decision: PostgreSQL ALTER constraint build/validation risk

Date: 2026-11-17
Status: Accepted
Family: F09 (partial slice), canonical T37
Base: `8de713b818cb20d199c5e0172ae2607708f46b91`

## Context

F09 has five inventory rows and 40 authored dimensions. All 62 current T37 CLI
inputs were probed with the real PostgreSQL-capable CLI before implementation.
ADD PRIMARY KEY is silent; FK has a blanket forbid but no specific initial
validation-scan warning. Existing CHECK NOT VALID and same-batch VALIDATE
follow-up rules already cover distinct dimensions and must not be duplicated.

The extractor retains FK `not_valid=true|false`, but loses the PRIMARY KEY /
UNIQUE `USING INDEX` distinction. Consequently the existing UNIQUE advisory
incorrectly says it builds an index even when it explicitly reuses one.
The first commit preserves an AST-derived `using_index=true|false` option for
these two constraint kinds, rather than inferring reuse from an empty column
list. No parser widening, raw index payload or new public result field is
required.

## Decision

**Behavior change approved:** the operator explicitly approved this batch,
including both new warnings enabled by default and the corrected UNIQUE
USING INDEX silent arm. Ordinary enabled/level overrides remain available.

| Declaration | Finding from this slice |
| --- | --- |
| ALTER ADD FOREIGN KEY without NOT VALID | `ddl.pg.alter.add_foreign_key.not_valid.require`, warning |
| ALTER ADD FOREIGN KEY with NOT VALID | no finding from that rule; existing follow-up check remains |
| ALTER ADD PRIMARY KEY without USING INDEX | `ddl.pg.alter.add_primary_key.concurrent_index.warn`, warning |
| ALTER ADD PRIMARY KEY with USING INDEX | no finding from that rule; not index-legality proof |
| ALTER ADD UNIQUE with USING INDEX | existing `ddl.pg.alter.add_unique_constraint.concurrent_index.advisory` is silent |
| ALTER ADD UNIQUE without USING INDEX | existing advisory remains at its configured level; lock explanation corrected |
| Missing/unowned relevant normalized option | new rules owe an explicit bounded declaration fact; never guess from absence |
| CREATE constraints, unrelated ALTER actions, MySQL/TiDB | new rules do not apply |

These are authored migration-risk warnings, not claims of successful execution,
actual scan duration, measured locking or metadata-backed target legality.
The FK forbid is independent: if enabled it can still reject, and this warning
does not relax it. NOT VALID is not a general safe override: PostgreSQL 17 does
not allow it for an FK on a partitioned table, so remediation must state the
applicability limit rather than prescribe invalid syntax universally.

## Rationale

PostgreSQL 17 ADD FOREIGN KEY normally validates existing rows; NOT VALID
skips that initial scan but still enforces subsequent writes. ADD FOREIGN KEY
requires SHARE ROW EXCLUSIVE on both referencing and referenced tables.
Most other ADD constraint forms, including regular PK/UNIQUE index builds,
require ACCESS EXCLUSIVE unless another documented subform applies.

ADD PRIMARY KEY/UNIQUE USING INDEX attaches an existing eligible unique btree
index. Eligibility, table partitioning and possible NOT NULL verification
still matter; silence from the build-risk rule does not establish them.
Prebuilding an index CONCURRENTLY is an alternative to review, not an action
the audit executes or a promise of lock-free attachment.

## Public Contract

Only bounded table/schema/constraint identity, constraint kind, action and the
relevant boolean declaration fact enter new findings. No predicate/default
expression, account, provider error or SQL payload is copied to new finding or
explanation fields. Existing caller-authored SQL behavior is unchanged.
SDK/CLI/HTTP/MCP share the rule registry; no transport-specific analysis.

The original UNIQUE rule ID/default level remain stable. Reused-index forms
stop receiving its misleading build notice; plain forms keep it. New rules
emit no synthetic index-existence or constraint-state conclusion.

## Deferred / Out Of Scope

F09 remains open. Provider constraints currently include only PK facts and do
not preserve constraint kind/validated status for VALIDATE or ALTER CONSTRAINT
adjudication. Existing-column identity options/RESTART also collapse into a
coarse set_generated action. Those gaps remain queued for separately approved
fact preservation and rules; they are not accepted product deferrals or whole
family completion from this slice.

No privilege resolution, existing index eligibility query, constraint
existence/type/state resolution, data scan, actual ALTER execution, runtime
sequence analysis or PG18 parser promotion. The five #149 candidates remain
on their existing parser-error boundaries.

## Verification Evidence

- Before rules: real CLI probes cover all current T37 CLI shapes. A temporary
  normalized-model probe proves FK NOT VALID is retained but USING INDEX,
  DROP constraint guards/CASCADE and identity option/RESTART details are lost.
- `parser_constraint_build_postgresql_tag_test.go` binds the reusable-index
  marker to the actual AST, including plain, reused, unnamed and quoted cases;
  FK/CHECK do not acquire an irrelevant marker.
- Evidence for the new rule/default contract is supplied by the implementation
  and native/corpus commits; this seam record alone claims no inventory
  promotion or final delivery readiness.

## Consequences

Eligible previously-pass PK migrations now review. Allowing FK through policy
no longer hides the initial validation trade-off. USING INDEX can remain silent
without being described as safe or executable. Other F09 dimensions must still
receive their own exact evidence and disposition before the family closes.

## Links

Official PostgreSQL 17 material checked on 2026-11-17; native target remains
17.7, parser remains pg_query_go/v6.2.2 / libpg_query 17:

- [ALTER TABLE](https://www.postgresql.org/docs/17/sql-altertable.html)
- [pg_constraint](https://www.postgresql.org/docs/17/catalog-pg-constraint.html)
- [Constraints](https://www.postgresql.org/docs/17/ddl-constraints.html)
- [Program ledger](../plans/2026-11-17-rule-backlog-ledger.md)
