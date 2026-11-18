# Decision: PostgreSQL existing-column generation transition evidence

Date: 2026-11-18
Status: Accepted
Related milestone: F09 / `milestone/pg-alter-semantics` (D5)
Related commits: research baseline `aa7f8872`; D4 history closeout `a2e3a7ab`; D5 implementation pending

## Context

D2 governs CREATE TABLE / ADD COLUMN declarations, not existing-column changes.
At `aa7f8872`, 45 PG17.7 shapes were compared with 90 actual offline/metadata-aware
CLI audits and independent transactional native execution. ADD IDENTITY could
pass despite native 55000/22023/42601 refusals. SET options, RESTART and SET
GENERATED share a lossy `set_generated` projection; guards and duplicate modes
are lost. A normalized-spec probe proves six pairs collapse despite different
native meanings. Existing DEFAULT/NOT NULL blanket forbids are policy, not
proof of the target's generation state or the legality of a transition.

The operator explicitly approved all four defaults below and the conservative
stage boundary. This approval does not complete F09 or authorize expression,
permission, business-data or general PostgreSQL prospective-state analysis.

## Decision

Preserve authored transition facts separately from legacy projections: guards,
mode counts, ordered duplicate-preserving options, exact signed 64-bit literals,
and unmodeled members. Never recover them by reading SQL text or accepting the
first/last conflicting member as a unique declaration.

Load an exact-name PG column directory separate from legacy `Column` fields.
Observe identity/generated flags, actual builtin type identity, NOT NULL,
DEFAULT existence and PK membership. Bind an identity sequence through its
catalog dependency, never by guessing its name or interpreting a default.
Only stable `pg_sequence` definition parameters and its actual integer type
are carried; no issuance/current/cache-consumption state is read.

Catalog proof is limited to observed PG17 and ordinary persistent tables that
are neither inheritance participants nor partition members/roots. A missing,
ambiguous, wrongly bound or stale fact is unknown, not absence or permission.
Withhold proof after relevant preceding DDL, opaque/parser failures, identity
changes, or earlier subcommands. Do not simulate a PostgreSQL post-state.

## Rationale

A blanket forbid cannot establish target legality, and lossy legacy maps cannot
separate a guarded no-op from a refusal or START from RESTART. Typed authored
facts and a separate catalog directory preserve these distinctions without
changing existing output or guessing schema state. Builtin type OIDs avoid
mistaking a domain/custom spelling for a supported integer. Dependency binding
survives sequence rename and keeps its type independent of the column's type.
Stable definitions allow bounded checks without reading issuance state; explicit
gaps are safer than silently promoting missing runtime/data facts to success.

## Public Contract

Four independently configurable, enabled-by-default, parameterless rules:

| ID | Default | Bounded predicate |
|---|---|---|
| `ddl.pg.alter.column.generation.transition.valid` | blocker | Proven invalid target/precondition for ADD/SET/DROP IDENTITY, identity options/RESTART, DROP EXPRESSION, and DEFAULT/NOT NULL generation/PK conflicts |
| `ddl.pg.alter.identity.sequence_options.valid` | blocker | Proven duplicate/conflicting options or modes, zero increment, nonpositive cache, integer-type/effective bound/start/restart violations |
| `ddl.pg.alter.identity.restart.warn` | warning | Explicit RESTART value-reuse/concurrent-blocking risk; SET START alone is not RESTART |
| `ddl.pg.alter.identity.cycle.warn` | warning | Explicit applicable CYCLE in ADD IDENTITY / SET options; NO CYCLE and omission are silent |

ADD IDENTITY requires a proven ordinary, non-generated/non-identity integer
column already NOT NULL and without DEFAULT. SET identity/options/RESTART
requires an identity target. DROP IDENTITY / EXPRESSION requires the matching
kind unless its parsed IF EXISTS makes a proven absent-kind operation a no-op.
The guard never hides a missing column. DEFAULT on identity/stored-generated
columns and dropping NOT NULL on identity/PK columns are proven refusals.
Nullable SET NOT NULL retains a data-proof gap rather than assuming no NULLs.

ADD reuses D2's bounded new-sequence declaration checks. SET uses observed
stable parameters rather than CREATE defaults. Bounds edits without explicit
RESTART retain a current-value gap when no stronger proven refusal resolves
them: PG17 `init_params` checks both recorded START and current value. Bare
RESTART uses recorded START. SET START changes that recorded value but neither
restarts nor advances the sequence.

Known-valid cases emit no blocker. Known guarded no-ops emit no new target
error. Unknown/ambiguous/unmodeled/stale cases use the existing unverified,
at-least-review evidence-gap protocol, not fabricated blockers. Proven
statement-local invalid options can be rejected without fabricating a catalog.

Old blanket forbids and independent rules remain unchanged; silence of a D5
rule does not promise overall pass or executable SQL. D2 CREATE/ADD COLUMN
predicates remain separate, with no duplicate D5 match. Public JSON gains no
fact envelope: catalog and authored evidence are excluded from serialization.
New finding payloads are bounded action/identity/reason fields, never raw
expressions, provider errors, accounts or connection details.

## Deferred / Out Of Scope

The operator accepted these as stage boundaries, not permanent deferrals or
whole-row support: business-data validation, sequence issuance position and
actual value collision, inheritance/partition semantics, permissions,
expression analysis, and general sequential execution. Other F09 dimensions
remain in the original five-row/40-declaration ledger. No parser expansion,
dependency major upgrade or default-off substitute is authorized here.

## Verification Evidence

Research (historical, not final D5 delivery):
`/tmp/ds-pg-identity-scope-aa7f8872/evidence.json`,
`/tmp/ds-pg-identity-normalized.log`. Native outcomes: 18 successes, nine 55000,
nine 22023, eight 42601 and one 42P16; separate nullable-data refusal 23502.
Every audit preserves column/sequence/sentinel state; driver-only DDL runs in
transactions and rolls back. An initial DROP IDENTITY probe mistakenly read
the removed sequence afterwards; the corrected full run inspects its catalog
absence and measures the successful command, not that probe's 42P01.

Freeze these **24 additive T37 cases before implementation** (old 94 cases and
profiles remain):

- `T37.cli.column-generation-target-offline`
- `T37.cli.column-generation-options-offline`
- `T37.cli.column-generation-mode-duplicates`
- `T37.cli.column-generation-guards-offline`
- `T37.cli.column-generation-default-null-offline`
- `T37.cli.column-generation-cycle-offline`
- `T37.cli.column-generation-restart-offline`
- `T37.cli.column-generation-start-offline`
- `T37.cli.column-generation-disabled`
- `T37.cli.column-generation-mysql`
- `T37.cli.column-generation-tidb`
- `T37.cli.column-generation-parser-retention`
- `T37.cli.column-generation-unmodeled-controls`
- `T37.cli.column-generation-add-column-d2-isolation`
- `T37.meta.pg-column-generation-add`
- `T37.meta.pg-column-generation-targets`
- `T37.meta.pg-column-generation-options`
- `T37.meta.pg-column-generation-guards-case`
- `T37.meta.pg-column-generation-restart-cycle`
- `T37.meta.pg-column-generation-prior-ddl`
- `T37.meta.pg-column-generation-prior-parser`
- `T37.meta.pg-column-generation-nullable-data`
- `T37.meta.pg-column-generation-sequence-current`
- `T37.meta.pg-column-generation-scope`

Unit, application/SDK/HTTP/MCP, corpus, native catalog/refusal and audit
non-execution controls must precede exact inventory bindings. Final same-HEAD
TASK=all, artifact validation, independent read-only review, FF-only merge,
normal push and exact-SHA CI remain pending; research is not shipping proof.

### Fact seam (before rule implementation)

The model/parser/provider/application/SDK tests pin lossless guards and int64
options, exact lookup and ambiguous/unknown controls, nil/empty cloning, and
JSON exclusion. Query/scan/iteration/close failures publish no partial catalog.
A native PG17.7 provider check additionally proves renamed-sequence dependency
binding, a bigint column with an independently altered smallint sequence,
serial versus identity, domain versus builtin, stored generation/PK flags,
zero-column tables and unsupported table scopes, without changing issuance
state. These checks validate facts, not the pending four-rule delivery.

## Consequences

Keep authored transition declarations, stable definitions and runtime issuance
separate. A later version/table-scope or ordered-state extension needs its own
proof and approval. Only final canonical bindings may update inventory cells;
this fact seam alone changes no row/cell status or original denominator. The
remaining F09 dimensions stay open after bounded D5 delivery.

## Links

- [PG17 ALTER TABLE](https://www.postgresql.org/docs/17/sql-altertable.html)
- [PG17 ALTER SEQUENCE](https://www.postgresql.org/docs/17/sql-altersequence.html)
- [PG17 pg_attribute](https://www.postgresql.org/docs/17/catalog-pg-attribute.html)
- [PG17.7 sequence implementation](https://github.com/postgres/postgres/blob/REL_17_7/src/backend/commands/sequence.c)
- Tests: `internal/domain/spec/postgresql_column_test.go`,
  `internal/infrastructure/parser/postgresql/parser_column_transition_postgresql_tag_test.go`,
  `internal/infrastructure/metadata/postgresql/column_state_test.go`,
  `internal/application/audit/postgresql_column_metadata_test.go`,
  `pkg/deltascope/audit_column_catalog_test.go`
- [D2 declaration contract](2026-11-16-postgresql-generated-identity-governance.md)
- [D4 constraint target contract](2026-11-17-postgresql-constraint-target-evidence.md)
- [Full backlog ledger](../plans/2026-11-17-rule-backlog-ledger.md)
