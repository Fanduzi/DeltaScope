# Decision: Query Access Literal-Only and Reversed Operand Shapes

- Date: 2026-07-26
- Status: Accepted
- Related: [literal operand support](2026-07-25-query-access-literal-operand-support.md), [builtin semantic manifests](2026-07-18-query-access-mysql-tidb-builtin-semantic-manifests.md)
- Related milestone/version: v0.450.0
- Spec: `docs/plans/2026-07-26-query-access-literal-only-and-reversed-operands-spec.md`
- Design: `docs/plans/2026-07-26-query-access-literal-only-and-reversed-operands-design.md`
- Implementation: `docs/plans/2026-07-26-query-access-literal-only-and-reversed-operands-implementation.md`

## Context

v0.440.0 added online MySQL/TiDB proof for three exact mixed shapes:
`COALESCE(column, const)`, `NULLIF(column, const)`, and
`IFNULL(column, const)`. It intentionally leaves literal-only calls,
`COUNT(1)`, and reversed mixed positions indeterminate.

Those deferred shapes need a separate decision because they stress the core
meaning of a Query Access requirement. A literal does not identify a physical
column, while a base relation still represents a table read. A broad change
that merely treats constants like columns, or accepts every constant anywhere,
would make the manifest a name allowlist instead of a bounded proof.

## Proposed Decision

If the implementation evidence is completed, extend only the MySQL/TiDB online
manifest with finite, exact operand vectors for:

- Unary pure scalars with `[const]`.
- `COUNT(const)` over a schema-qualified resolved physical base relation.
- `COALESCE`, `NULLIF`, and `IFNULL` with `[const,column]` and
  `[const,const]`, each at exactly two operands.

The existing `[column,const]` forms remain unchanged. Every newly admitted
shape must be explicit per dialect/profile and must match arity and operand
position exactly. No new variable-arity tail expansion is part of this
decision.

## Requirement Model

- A resolved physical base relation contributes `read_table`.
- A direct physical column contributes `read_column` plus its table read.
- A literal contributes no table or column requirement.
- This proposal requires at least one resolved physical base relation. It does
  not introduce `admissible` results with an empty requirements list.

For example, `SELECT NULLIF('x', name) FROM app.users` would require
`app.users` and `app.users.name`; `SELECT COUNT(1) FROM app.orders` would
require only `app.orders`.

## Consequences

The change can make a small set of common, non-writing online queries usable
when their table and column dependencies are fully known. It does not execute
the query or inspect query results. It does not infer authorization, grants,
RLS, masking, SQL mode, or runtime behavior.

The following remain deferred: relationless literal-only `SELECT`, PostgreSQL
literal operands, `COALESCE` with more than two operands, parameters, casts,
nested functions, expressions, UDFs, quoted/qualified calls, and all default
offline surfaces.

## Alternatives Considered

### Keep All Deferred

This is the lowest-risk option but leaves straightforward online reads such as
`COUNT(1) FROM app.orders` unnecessarily unusable.

### Accept Constants Wherever Columns Are Accepted

Rejected. It loses position-specific proof and would silently admit shapes
that have no parser, manifest, requirement, or live-evidence contract.

### Admit Relationless Literal-Only Queries with Empty Requirements

Deferred. It changes the current strict physical-relation proof model and
requires a separate product decision about the meaning and use of an empty
requirement set.

## Acceptance Evidence

### Architecture

The MySQL/TiDB path bypasses `ValidatePhase1PureEffectCandidates` entirely. In `service.go`, the MySQL/TiDB builtin gateway (`proveBuiltinSemantics`) runs directly on candidates without Phase-1 eligibility filtering. The PostgreSQL path (`resolveAndProveEffects`) calls `ValidatePhase1PureEffectCandidates` which rejects literal-only operands. This preserves the PostgreSQL boundary while enabling literal-only shapes for MySQL/TiDB.

### Admitted Shapes (15 total)

| Category | Shapes | OperandKinds |
|----------|--------|--------------|
| Unary literal-only | LOWER, UPPER, LENGTH, CHAR_LENGTH, ABS, CEIL, CEILING, FLOOR | `[const]` |
| Aggregate literal | COUNT(1) | `[const]` |
| Reversed binary | COALESCE, NULLIF, IFNULL | `[const, column]` |
| All-constant binary | COALESCE, NULLIF, IFNULL | `[const, const]` |

Each shape is admitted across MySQL 5.7, 8.0, 8.4, and TiDB 8.5 profiles (60 profile-shape combinations).

### Manifest Validator

`validateBuiltinSemanticEntry` now rejects malformed fixed-arity entries:
- `len(OperandKinds) != Arity` when `MinArity == 0` and `Arity > 0`
- Arity-0 entries with non-star operand kinds

Regression test: `TestBuiltinSemanticManifest_RejectsInvalidEntries` covers both cases.

### E2E Test Matrix

**SDK** (`pkg/deltascope/query_access_session_mysql_tidb_live_e2e_test.go`):
- `TestLiveProfile_AssertsVersionAndAdmitsAggregates` runs across all 4 profiles
- 15 admitted probes: 9 literal-only + 3 reversed + 3 all-constant
- 7 negative probes: relationless, arity-1, arity-3, nested, cast, param, unknown
- Each probe verified with exact requirements and no-leak assertions

**CLI** (`internal/interfaces/cli/query_access_e2e_mixed_literal_test.go`):
- `TestQueryAccessOnline_MixedLiteralScalars` runs across all 4 profiles
- 18 online probes: 3 existing [column,const] + 8 literal-only + 1 COUNT + 3 reversed + 3 all-constant
- Each probe verified with exit code, classification, admission, exact requirements, and no-leak
- Same 18 probes also run through `offline_indeterminate` path asserting exit code 2 and indeterminate classification

**HTTP** (`internal/interfaces/http/query_access_e2e_mixed_literal_test.go`):
- `TestQueryAccessOnline_MixedLiteralScalars` runs across all 4 profiles
- 18 online admitted probes: 3 existing [column,const] + 8 literal-only + 1 COUNT + 3 reversed + 3 all-constant
- 2 failure probes for credential no-leak verification
- Each probe verified with HTTP 200, classification, admission, exact requirements, and no-leak
- Same 18 probes also run through `default_path_indeterminate` path asserting indeterminate classification

### Evidence Maintenance (2026-08-15)

The test names above record the original matrix, not current retention. Issue
#12 removed the CLI `TestQueryAccessOnline_MixedLiteralScalars` matrix because
the unified SDK owns its product/profile/shape semantics. The retained CLI
transport evidence is `TestQueryAccessOnline_BuiltBinaryTransportSmoke` for
MySQL 8.4 and TiDB 8.5 real-route admitted/fail-closed behavior plus
`TestQueryAccessOnline_DefaultOffline`; at that point the HTTP matrix remained
unchanged.

Issue #13 then removed the duplicate HTTP matrix. `TestQueryAccessOnline_TransportSmoke`
retains MySQL 8.4 and TiDB 8.5 real-route admitted/fail-closed status/body,
requirement/reason, request-ID/access-log, and no-leak evidence;
`TestQueryAccessOnline_DefaultOffline` retains the HTTP offline boundary.

Issue #14 records the authoritative deleted-declaration reconciliation in the
[implementation ledger](../plans/2026-08-15-query-access-test-ownership-consolidation-implementation.md):
`TestLiveProfile_AssertsVersionAndAdmitsAggregates` was renamed and retained as
`TestLiveUnifiedSession_AssertsVersionAndSemanticMatrix`; the deleted CLI and
HTTP matrices remain semantically owned by the unified default and four-target
live SDK matrices, with the named CLI and HTTP tests retained only as transport
smoke owners.

### Verification Commands

```bash
# Full suite
go test ./... -count=1

# PostgreSQL-tagged
go test -tags postgresql ./... -count=1

# Race detection
go test -race ./internal/domain/queryaccess/... ./internal/application/queryaccess/... ./internal/interfaces/cli/... ./internal/interfaces/http/... ./pkg/deltascope/... -count=1

# Build and vet
go build ./... && go build -tags postgresql ./...
go vet ./... && go vet -tags postgresql ./...

# Corpus and lint
make query-access-corpus-gates
golangci-lint run ./...

# Documentation and formatting
make decision-record-gate
make release-gofmt-gate

# Git hygiene
git diff --check
go mod tidy && git diff --exit-code go.mod go.sum
```

### Deferred Shapes

- Relationless literal-only `SELECT` (no FROM clause)
- `COALESCE` with 3+ operands
- PostgreSQL literal operands
- Nested expressions, casts, parameters, UDFs, quoted/qualified calls
