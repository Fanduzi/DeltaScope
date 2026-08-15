# Design: Consolidate Query Access Test Ownership

## Status

Proposed. This design removes duplicate evidence by assigning each behavior to
one owner. It does not reduce the product contract or required delivery gates.

## Existing Shape

The unified online entry removed product routing from CLI and HTTP production
code, but the migration deliberately retained the old tests. The result is four
overlapping evidence layers:

```text
deprecated dialect SDK APIs  -> full semantic/live matrices
unified SDK API              -> full semantic/live matrices
CLI                          -> semantic matrices + CLI contracts
HTTP                         -> semantic matrices + HTTP contracts
```

The duplication helped prove migration equivalence. After the unified entry and
soft deprecation have landed, it no longer represents independent behavior.
However, tests that observe different trust boundaries or public sinks are not
duplicates even when they submit the same SQL.

## Target Shape

```text
unified SDK
  owns: exhaustive product/profile/shape/result/probe behavior

deprecated SDK compatibility
  owns: old source/error/priority/ownership contract + one case per target

CLI
  owns: flags/session/close/exit/stdout/stderr/no-leak + real route smoke

HTTP
  owns: registry/auth/zero-dial/status/body/log/no-leak + real route smoke

MCP
  owns: proof that Query Access remains absent
```

No shared matrix package sits between these owners. The unified SDK is already
the behavioral seam; another abstraction would move test data without reducing
conceptual duplication.

## Semantic Ownership

The unified SDK matrix is exhaustive across supported identity targets. It
owns exact analysis output and every SQL-shape boundary. New shapes are added
there once.

Transport smoke is deliberately shallow but real. CLI and HTTP each connect
through their actual configuration path to MySQL 8.4, TiDB 8.5, and PostgreSQL
17. For each family, one admissible and one fail-closed statement verifies that
the transport passes SQL/default schema/mode correctly and serializes the
result. MySQL 5.7 and 8.0 remain SDK-live targets because transport wiring no
longer varies by MySQL server series.

A new product family or driver adds one smoke to each supported transport. A
new version within an existing driver/configuration family adds only SDK
coverage unless its transport configuration differs.

## Trust and Privacy Exceptions

Two tests with similar SQL are independent when they guard different trust
boundaries:

- PostgreSQL `COUNT(2)` guards the syntax envelope;
- PostgreSQL `COUNT(1)` against a foreign table guards relation-kind trust;
- CLI marker tests guard stdout/stderr;
- HTTP marker tests guard response JSON and synchronized access logs;
- HTTP unauthorized and unknown IDs guard authorization-before-dial;
- SDK recording tests guard complete driver probe/no-execution behavior.

Therefore each transport retains the PostgreSQL syntax negative, foreign-table
negative, and offline/default regression. Each external sink retains its own
success and failure no-leak evidence.

## Recording-Driver Split

The SDK recording suite owns the detailed sequence and content class of
identity, relation, column, and function catalog probes. It proves submitted SQL
is never executed, prepared, or explained.

CLI and HTTP recording tests shrink to adapter contracts. They prove the
transport opens and closes its session, delegates the pinned connection to the
unified API, maps bounded failures, and does not execute submitted SQL,
`EXPLAIN`, or prepare. They do not repeat every excluded shape or catalog row.

## Deprecated API Compatibility

Soft deprecation did not remove old behavior. The compatibility suite retains:

- six exported names and their deprecation notices;
- tagged and untagged PostgreSQL declarations/stubs;
- exact old errors and combined-invalid-input priority;
- caller-owned connection behavior;
- one unified-versus-old equivalence case for MySQL 5.7, 8.0, 8.4, TiDB 8.5,
  and PostgreSQL 17.

The old APIs share private proof cores with the unified entry, so their full SQL
shape matrices add little protection once each target still proves routing and
equivalence. The unified matrix owns semantic breadth.

## Deletion Method

Build the ledger before deleting. Every candidate row has two columns of
replacement evidence: semantic owner and observable-boundary owner. A purely
SDK behavior needs only the first; transport, compatibility, privacy, or
lifecycle behavior needs both.

Prefer reducing cited files in place. This preserves paths referenced by
Accepted ADRs and makes the remaining ownership obvious. If a file has no
remaining responsibility, its deletion requires a follow-up note in each ADR
that cites it, linking this decision and the replacement test.

Do not create an automated checker for duplication. Test names and SQL strings
are not stable policy identifiers, and a grep-based gate could reward renaming
rather than ownership.

## Mutation Evidence

Coverage counts cannot prove that the retained test observes the intended
boundary. Before final review, make and restore five temporary mutations:

1. promote or reject one unified SDK shape incorrectly;
2. map one CLI admission to the wrong exit code;
3. allow an unauthorized HTTP request to reach the opener;
4. treat a PostgreSQL foreign table as a physical base table;
5. corrupt only the target stored by one deprecated MySQL/TiDB wrapper after
   identity derivation, leaving the unified route and shared profile mapper
   unchanged.

For each mutation, run the narrow retained test and record the expected RED
failure. The fifth probe must not mutate the shared target-to-profile helper:
that could make unified and legacy paths wrong in the same way and leave an
equivalence assertion green. Restore the file immediately, verify a clean diff
for the mutation, and do not commit a mutation harness.

## Documentation

`docs/dev/testing.md` gains the durable ownership matrix and future-change
rules. Affected package/module READMEs describe what their remaining tests own.
This spec and implementation plan retain the deletion ledger because it is
milestone evidence, not permanent policy.

`CONTEXT.md` does not change: semantic matrix, transport evidence, and deletion
ledger are test-architecture terms, not DeltaScope domain language.

## Alternatives Rejected

### Keep all duplicated matrices

Rejected because the unified seam has landed and duplication now multiplies
maintenance and Docker cost without adding an independent observation point.

### Share one generated matrix across packages

Rejected because it creates a test framework, couples transport tests to SDK
fixtures, and hides rather than removes duplicate responsibilities.

### Keep only SDK tests

Rejected because SDK tests cannot observe CLI streams, HTTP authorization,
status/body/logging, or transport lifecycle.

### Keep one generic fail-closed transport case

Rejected because syntax-envelope and foreign-table failures guard different
trust boundaries.

### Keep one deprecated API target

Rejected because target-to-profile mapping remains part of the exported old
API behavior for four MySQL/TiDB targets plus PG17.

### Use coverage percentage as the gate

Rejected because line coverage does not prove ownership, privacy sinks, or
fail-closed behavior. Ledger mapping and mutation probes are stronger evidence.

## Consequences

The suite becomes smaller and future semantic work changes one exhaustive
matrix. Transport and compatibility suites remain intentionally repetitive at
their distinct boundaries. The cleanup pays a one-time evidence cost: ledger,
mutation probes, full Docker gates, and ADR reconciliation.
