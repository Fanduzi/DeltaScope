# Decision: bounded PostgreSQL generated/identity declaration governance

Date: 2026-11-16
Status: Accepted
Related milestone: `milestone/pg-generated-identity`, T36 / T37
Base: `795b47b07aedc698dad5b788758f4131113de12a`

## Context

The next-rule-family plan ranks PostgreSQL generated/identity work second.
Existing T36/T37 findings are generic column checks, lifecycle notices or
blanket ALTER forbids; they do not check generation declaration legality or
identity sequence cycling. Real PostgreSQL-capable CLI probes confirm that
ADD generated/identity columns and ADD IDENTITY currently pass with notices.

The normalized seam is partial. `GeneratedWhen` and `IdentityOptions` lose
repeated clauses/options and signed values outside int32. Type-name rendering
omits array dimensions, and `NotNull=false` cannot distinguish omission from
explicit NULL. Generated expression bodies are not modeled. New rules must
not consume these projections as complete evidence.

## Decision

**Behavior change approved:** the operator explicitly selected “Approve both,
default-on” for the bounded declaration-validity blocker and explicit identity
CYCLE warning, including new default-policy reject/review outcomes.

Add two PostgreSQL-only, offline rules for authored CREATE TABLE columns and
ALTER TABLE ADD COLUMN declarations:

| Declaration | Rule result |
| --- | --- |
| Proven non-integer/array identity type | `ddl.pg.column.generation.declaration.valid` blocker |
| Identity with explicit NULL | declaration-validity blocker |
| Generated or identity column with an authored DEFAULT | declaration-validity blocker |
| Identity INCREMENT 0, CACHE < 1, invalid MIN/MAX/type bounds or START outside bounds | declaration-validity blocker |
| Repeated generation clauses/options, unknown modes, unmodeled numeric/type facts | explicit attributable evidence gap, never last-wins |
| Unambiguous applicable identity CYCLE | `ddl.pg.column.identity.cycle.warn` warning about potential value reuse |
| NO CYCLE or omitted cycling | no cycling finding |
| ALWAYS versus BY DEFAULT without CYCLE | no cycling finding; mode alone is not a new warning |
| Ordinary columns, MySQL/TiDB, existing-column ADD/SET/DROP IDENTITY | outside these rules |

The parser adapter first preserves internal `PGColumnGeneration` evidence:
counts, first authored mode, explicit NULL, type-width proof, ordered int64
sequence options and unmodeled markers. Array types are proven non-integer;
unresolved/custom type names stay unproven rather than guessed. The existing
legacy fields remain unchanged for compatibility. No raw expression payload
or new public result field is introduced. Column cloning owns the new facts.

## Rationale

PostgreSQL 17 permits only sequence-supported integer types for identity
columns. Generated columns cannot have DEFAULT or identity definitions.
Identity columns imply NOT NULL; explicit contradictory NULL is not omission.
Sequence bounds depend on integer type and increment direction; omitted bounds
use documented defaults, not existing runtime state. CYCLE can reuse values
but does not itself prove duplicates or an execution failure.

Keeping ordered option declarations prevents `CYCLE NO CYCLE` from becoming a
safe last-wins control. Full int64 extraction prevents large authored bounds
from silently disappearing. A bounded declaration check is useful without
pretending to validate generated expressions or existing catalog transitions.

## Public Contract

Both rules are default-enabled with ordinary enabled/level overrides and no
new params. Findings contain bounded table/column identity, action, generation
kind and reason/provenance flags only; no expression SQL or numeric option
payload is copied into new findings/explanations. Existing caller-authored SQL
serialization is unchanged. SDK/CLI/HTTP/MCP share the same rule engine.

An empty finding set is not a certificate that PostgreSQL will execute the
statement: these checks cover only the declared subset. Parser errors,
unsupported entries, unrelated evidence gaps and existing forbids remain.
The audit does not create tables, invoke expressions or advance sequences.

## Deferred / Out Of Scope

Expression immutability, function/operator resolution, generated-column
references/dependencies, partition/LIKE/inheritance semantics, user-defined
base-type resolution, existing-column identity transitions, uniqueness,
sequence issuance/current value/exhaustion forecasts and privileges are not
part of this tranche. Existing-column ADD IDENTITY retains its prior notice
and bounded option contract. No parser upgrade; #149's five PG18 candidates
remain parser errors. No whole-row completion is inferred from header checks.

## Verification Evidence

Before production rules, a real PG-capable CLI and a temporary normalized-model
probe at base `795b47b0` established the gaps above. The extractor seam tests
cover CREATE/ADD COLUMN, both modes, repeated declarations/options, explicit
NULL, array/custom types, signed int64 extremes and overflow. Focused parser
and application tests pass. Clone tests prove source facts are not mutated.

- `go test ./...` and `CGO_ENABLED=1 go test -tags postgresql ./...` pass
  with the two registered default-on rules, new corpus cases and public SDK /
  HTTP / MCP parity tests. The SDK test retains a PG18 parser-error sibling
  alongside the cycling warning; no candidate is silently promoted.
- Rule tests cover native integer defaults/direction, int64 extrema, every
  refusal category, disabled/configured-level paths, nil/wrong-dialect and
  existing-column controls, repeated/unmodeled declarations and evidence gaps.
- New findings are checked for expression/numeric-payload leakage. Existing
  authored SQL behavior remains unchanged.

Golden and inventory bindings are committed as the next focused evidence slice;
only the exact checked declaration dimensions may gain feature coverage.

## Consequences

Default-policy invalid declarations will reject; explicit cycling definitions
will require review. Previously passing ADD COLUMN cases may change verdict.
New evidence gaps may reveal uncertainty hidden by legacy projections. The
bounded checks must not grow into raw-expression matching or account lookup.

## Links

Versioned PostgreSQL 17 manuals rechecked on 2026-11-16; the native anchor is
PostgreSQL 17.7, not an assertion about the latest release or PG18 support:

- [Identity columns](https://www.postgresql.org/docs/17/ddl-identity-columns.html)
- [Generated columns](https://www.postgresql.org/docs/17/ddl-generated-columns.html)
- [CREATE SEQUENCE](https://www.postgresql.org/docs/17/sql-createsequence.html)
- [CREATE TABLE](https://www.postgresql.org/docs/17/sql-createtable.html)
- [Plan](../plans/2026-11-16-next-rule-family-scope.md)
- Tests: `extractor_generation_postgresql_tag_test.go`, `postgresql_generation_clone_test.go`
