# Decision: PostgreSQL existing-column generation transition evidence

Date: 2026-11-18
Status: Accepted
Related milestone: F09 / `milestone/pg-alter-semantics` (D5)
Related commits: research baseline `aa7f8872`; D4 history closeout `a2e3a7ab`; D5 facts `1e8c773d`, rules `23c9f4f9`, native/inventory and final shipping `26dde9ad`

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

Four independently configurable, enabled-by-default, parameterless rules.
Their findings/gaps use the observed PG17 banner, never a caller version pin;
unobserved/out-of-series versions stay gaps. Intrinsic option errors require
that observation but do not fabricate a table catalog.

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
non-execution controls precede exact inventory bindings. Final same-HEAD
TASK=all, validation, independent review, FF-only merge, normal push and exact-SHA
CI are recorded below; research alone is never shipping proof.

### Fact seam (before rule implementation)

The model/parser/provider/application/SDK tests pin lossless guards and int64
options, exact lookup and ambiguous/unknown controls, nil/empty cloning, and
JSON exclusion. Query/scan/iteration/close failures publish no partial catalog.
A native PG17.7 provider check additionally proves renamed-sequence dependency
binding, a bigint column with an independently altered smallint sequence,
serial versus identity, domain versus builtin, stored generation/PK flags,
zero-column tables and unsupported table scopes, without changing issuance
state. These checks validate facts, not the four-rule delivery by themselves.

### Shared rule implementation (candidate, not shipping proof)

All four shared rules, configuration/catalog metadata, six cross-dialect corpus
pairs and application/SDK/HTTP/MCP tests implement the approved contract. A
candidate replay of all 45 native shapes and 90 actual CLI audits preserves the
same native outcomes: illegal ADD/SET/DROP transitions and option conflicts now
reject with observed facts; valid ADD and the large-int64 declaration remain
pass; CYCLE/RESTART risks review; START-only/NO CYCLE/guards produce no new risk
finding. Offline cases retain version/fact gaps. Native-valid ordinary DEFAULT/
NOT NULL cases retain the independent legacy blanket rejects. All audit and
transactional driver rollback state controls hold. Candidate replay evidence
records `1e8c773d` plus its uncommitted rule diff; it is not a clean-HEAD or final
canonical artifact. Build/default+PG tests, default+PG lint, formatting and
inventory/corpus/coverage-catalog gates pass for this candidate. Frozen T37
additions and inventory bindings are described below; this candidate evidence
is distinct from the final same-HEAD delivery evidence.

### Canonical/native and inventory evidence

All frozen 24 cases are implemented without changing old 94 case objects,
required IDs or profiles: T37 **118/118, 1661 assertions** (530 additive).
Ten owned native fixtures contain 62 driver steps, 31 successes and 31 refusals,
with independent catalog/row/sequence/lock receipts after audit non-execution
checks. Refusals include 55000/22023/42601/42P16/42703 and nullable-data 23502.
A bigint column with a renamed smallint sequence proves dependency binding and
actual sequence type; START-only, RESTART, CYCLE, guarded/exact-case targets,
retained stored data, prior-DDL/parser and scope/current-value gaps are separate.
The partition-member ADD refusal is 42P16; parent/child/root/unlogged positive
native forms remain outside the audited stage rather than being mislabeled
unsupported syntax or successful product audit.

Initial fixture failures were assertion-contract errors: gap entries needed
explicit reason codes, and stored ADD's first native refusal was its nullable
precondition. The corrected fixture uses stored NOT NULL and independently
proves its default-expression 55000 refusal. No case or assertion was removed.
Focused artifact `/tmp/ddl-golden-pg-column-native-r3/T37/artifact.json` records
`23c9f4f9` with candidate manifest changes, not final clean-HEAD readiness.

Only six existing column-attribute cells gain exact selectors. SET GENERATED /
DROP EXPRESSION and RESTART become audited for bounded target/risk predicates;
NOT NULL, DEFAULT and identity options stay mixed where data/expression/unread
current-value dimensions are not certified. ADD/DROP IDENTITY keeps audited
status with stronger facts. The parent remains mixed; other attributes, row
counts and the original dimensional denominator are unchanged. Matrix output
is generated from YAML. Final same-HEAD delivery evidence follows.

## Final D5 delivery evidence

D5 shipped at `26dde9adbec574ddaeae5d01b461d15c7fa2065a`, FF-only merged
from `milestone/pg-alter-semantics` and normally pushed from base
`aa7f88723fbc930dacdd451a0a6c772a6617a5a5`. Milestone diff is **66 paths**;
last native/inventory task `23c9f4f9..26dde9ad` is seven. The original full
tracked/untracked status snapshot and unrelated worktrees/services were
preserved; no release, tag, branch deletion or PR/issue closure occurred.

Validated final artifact:
`/tmp/ddl-golden-pg-column-final-26dde9ad/all/artifact.json` — **50 tasks,
3450/3450 cases, 60,695 assertions**, every task at the exact final SHA with
nonzero execution and no failed case/assertion. The first all-run exceeded the
1800-second tool limit after 45 passing tasks; the unchanged HEAD supplied
T51/T52/T53/T54/T58, then `reaggregate` and `validate` passed. No skipped/zero
execution or old-HEAD substitute contributed. T37 is **118/118, 1661 assertions**.

Build, default/PG tests, default/PG lint, formatting and inventory/corpus/catalog
gates passed before/after merge; final artifact validation passed after push.
A separate single-agent read-only phase checked actual code/callers, approved
scope, privacy, silent/gap branches, evidence ownership and Git state with no
unresolved P1/P2. Post-push verification directly matched HEAD/main/origin/main,
the milestone ref and the server main ref, and all five exact-SHA checks.
[Go/PG/lint CI](https://github.com/Fanduzi/DeltaScope/actions/runs/44228829754),
[TLS E2E CI](https://github.com/Fanduzi/DeltaScope/actions/runs/44228829863) and
Cloudflare Pages succeeded.

This records historical D5 delivery, not same-HEAD aggregate evidence for a
later documentation commit. **F09 remains open** for the other original
column/constraint/attribute/options/rename-owner dimensions and wider
state/data/dependency/permission/inheritance/partition boundaries.

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
