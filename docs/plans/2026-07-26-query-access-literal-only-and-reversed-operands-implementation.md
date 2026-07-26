# Implementation Plan: Literal-Only and Reversed Operand Proofs

Date: 2026-07-26
Status: Proposed
Baseline: `main@d2c4d91`
Spec: `docs/plans/2026-07-26-query-access-literal-only-and-reversed-operands-spec.md`
Design: `docs/plans/2026-07-26-query-access-literal-only-and-reversed-operands-design.md`
Decision: `docs/decisions/2026-07-26-query-access-literal-only-and-reversed-operands.md`

## Operating Rules

- Create one milestone branch from the current local `main`; do not implement
  from `draft/query-access-literal-only-operands`.
- Run GitNexus impact analysis before editing each existing production symbol
  and run `gitnexus_detect_changes` before every commit.
- Keep the ADR Proposed until the evidence and independent review gates below
  complete. Do not merge, push, tag, or release without explicit approval.
- Stop rather than broadening the contract if parser facts differ across the
  supported MySQL/TiDB profiles.

## Phase 1: Characterize Current Boundaries

1. Record parser candidates for every proposed positive and negative SQL form.
2. Add parser tests for exact operand vectors: `[const]`, `[const,column]`,
   `[const,const]`, and rejected `[column,const,column]`, nested, cast,
   parameter, and expression cases.
3. Add regression tests proving current v0.440.0 `[column,const]` behavior is
   unchanged before altering semantic matching.

Stop if the parser cannot distinguish a proposed positive shape from a
deferred neighbor without inspecting literal values.

## Phase 2: Make Manifest Matching Exact

1. Inspect impacts for `BuiltinSemanticEntry`,
   `candidateOperandKindsMatch`, `candidateOperandRefsShape`, and
   `phase1FunctionEligible`.
2. Strengthen manifest validation so fixed-arity entries require
   `len(OperandKinds) == Arity`, and variable-arity entries cannot accidentally
   encode a new literal form through tail repetition.
3. Add exact entries per supported profile for the approved shapes only.
4. Add unit tests for every entry and negative tests for wrong position, wrong
   arity, repeated-tail, and literal-only aggregate without a relation.

Commit this mechanical manifest/matcher step separately after focused unit
tests and `gitnexus_detect_changes` confirm only expected proof symbols move.

## Phase 3: Preserve Physical Requirements

1. Update Phase 1 eligibility only as required by the exact entries.
2. Verify relation collection supplies `read_table` for all positive SQL.
3. Verify a direct column in either binary position produces exactly one
   `read_column` requirement for its resolved physical column.
4. Verify literal-only scalars and `COUNT(1)` produce no synthetic column
   requirement and never an empty-requirement admission.
5. Add corpus fixtures for one literal-only scalar, `COUNT(1)`, and each
   reversed binary form, plus deferred neighbor fixtures.

Stop if a positive result lacks a resolved table requirement or if a literal is
serialized as a requirement.

## Phase 4: Public Surface Evidence

Start the existing MySQL/TiDB Docker matrix with
`docker/query-access-builtin-compose.yaml`. Run public, not internal-only,
promotion tests for MySQL 5.7.44, 8.0.46, 8.4.10, and TiDB 8.5.7.

For SDK, CLI, and HTTP, verify each admitted family produces the exact
requirements and `read_only + admissible`. Verify every default/offline path
remains `indeterminate`. HTTP tests must use an authorized configured
`connection_id`; CLI tests must use its online connection path.

Add no-leak tests with unique literal markers. Check SDK serialization, CLI
stdout/stderr, HTTP response JSON, and HTTP access logs. Also retain bounded
connection-failure no-leak coverage.

## Phase 5: Negative and Compatibility Evidence

Add explicit regressions for:

- PostgreSQL online and all offline surfaces.
- Relationless literal-only queries.
- `COALESCE` arity one and arity three or more.
- Reversed and all-constant forms not explicitly listed in the manifest.
- Nested calls, casts, parameters, expressions, quoted/qualified calls,
  unqualified relations, views, CTEs, and unsupported profile/server identity.
- Existing v0.440.0 `[column,const]` forms.

## Phase 6: Review and Acceptance

1. Update user-facing Query Access documentation only after behavior is proven.
2. Update the ADR with concise evidence links, exact supported SQL shapes, and
   explicit deferrals. Do not paste task logs.
3. Put a temporary copy of this implementation plan under `.omo/plans/` only
   if Momus requires it; remove that mirror before committing or reporting.
4. Obtain an Oracle diff audit covering proof widening, public surfaces,
   Docker evidence, and no-leak tests.
5. Obtain a Momus `[OKAY]` review of this plan. Resolve every P0/P1/P2 before
   setting the ADR to Accepted.

## Required Gates

Run focused tests while developing, then run at minimum:

```text
go test ./... -count=1
go test -tags postgresql ./... -count=1
go test -race ./internal/domain/queryaccess/... ./internal/application/queryaccess/... ./internal/interfaces/cli/... ./internal/interfaces/http/... ./pkg/deltascope/... -count=1
go build ./...
go build -tags postgresql ./...
go vet ./...
go vet -tags postgresql ./...
golangci-lint run ./...
make query-access-corpus-gates
make pg-unit-test-gates
make test-e2e-http-tls
make test-e2e-cli-tls
make docs-example-gates VERSION=v0.440.0
make decision-record-gate
make release-gofmt-gate
npm test --prefix packages/deltascope-mcp
git diff --check
go mod tidy && git diff --exit-code go.mod go.sum
```

The Docker matrix used in Phase 4 must be brought down with volumes and
orphans removed, then checked for residual project containers, networks, and
volumes. Report commands actually run, not planned commands.
