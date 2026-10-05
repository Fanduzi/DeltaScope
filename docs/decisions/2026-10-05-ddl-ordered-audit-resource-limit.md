# Decision: Bounded admission to the MySQL/TiDB ordered audit

Date: 2026-10-05
Status: Frozen design; implementation awaits independent review
Related issues: [#79](https://github.com/Fanduzi/DeltaScope/issues/79), [T05/#84](https://github.com/Fanduzi/DeltaScope/issues/84), [coverage/#82](https://github.com/Fanduzi/DeltaScope/issues/82), [evidence/#83](https://github.com/Fanduzi/DeltaScope/issues/83)
Related commits: the A7 code commit containing this record; the separate evidence commit identifies the exact tested code SHA.

## Context and decision

The ordered migration path saves a separate pre-state for each normalized statement, derives its conditional post-state, then runs impact/planner and rule evaluation. Merely contaminating a batch would neither stop this work nor distinguish analysis not attempted from an ordinary missing-metadata evidence gap.

Admit the first **1024 top-level normalized statements per MySQL/TiDB request** to that ordered path. Statement 1024 is admitted; a resource entry appears only if a 1025th normalized statement exists. All normalized statement kinds consume one slot, including read-only, session and already-unsupported forms. Parser failures stay diagnostics rather than invented statements. ALTER actions and target members are not separate units. The quota is a Planner-selected engineering bound, not a database standard or measured safety limit.

One immutable by-value `auditLimits` value enters the single full `auditWithLimits` core. `AuditSQL` and `Service.Audit`, and therefore SDK/CLI/HTTP/MCP, supply the shared production constant. Internal tests may supply zero or small quotas; zero means zero admission, not unlimited. No public field, flag, environment variable, mutable package-global override, context value, policy rule, or adapter counter is introduced. PostgreSQL never enters the limited ordered pass.

Admission is checked after parse/extract, before target resolution, snapshot/object/index-owner lookup, pre-state cloning and `state.apply`. Pre-batch instance-fact loading is unchanged. A blocked statement gets an internal `Statement.ResourceLimit` marker (`json:"-"`), loses stale target/object/row/impact conclusions, and contaminates ongoing derivation. Its original normalized payload and Unsupported/aspect facts remain. Every blocked successor skips ordered work, StatementRule/EvidenceReporter calls, normal global-rule input and PlanEstimator/impact publication. The already-resolved canonical version may be retained; no stale table or object conclusion is reused. Existing prefix snapshots and results are not mutated. Context cancellation is still checked on skipped paths and before returning.

The `batchState.enrichStatements` method is the actual production loop, not a test reimplementation. Direct tests can inspect its owned state after the cutoff and prove that blocked CREATE, ALTER and INDEX effects were never published, rather than inferring that from a contaminated `preState` returning nil.

## Public contract

Every blocked normalized statement stays individually in `statements` with its kind, raw/normalized SQL, index and original source position, `coverage=incomplete`, no resource-generated finding, and no resource-generated evidence gap. It adds exactly one `unsupported` entry after preserved existing evidence:

```json
{
  "index": 2,
  "feature": "audit.resource_limit",
  "sql": "CREATE INDEX idx_c ON t(c);",
  "reason": "ordered-state statement budget exhausted",
  "metadata": {
    "phase": "ordered_state",
    "resource": "statements",
    "limit": 2,
    "consumed": 2,
    "line": 3,
    "column": 1
  }
}
```

This example uses the internal two-statement test quota. Production `limit` and every blocked production statement's `consumed` are 1024. Index zero keeps the existing `omitempty` behavior. Resource feature/reason are fixed, not a vendor boundary. Reason and metadata never copy SQL, body text, credentials, provider errors or stacks. The preexisting `unsupported[].sql` contract is intentionally unchanged and still carries corresponding `raw_sql`; this is not a no-SQL API.

Resource-only outcomes have at least `review` and aggregate `incomplete`; a prior blocker keeps `reject`. Disabled policy cannot disable this boundary. Application/SDK return partial results plus the existing `ErrUnsupportedStatement` identity. CLI exits 1 even at `--fail-on none`; HTTP uses the existing 400 unsupported envelope; MCP uses the existing `isError=true` envelope. Parser failures anywhere in the already-parsed input retain their diagnostics and higher-priority parser error (CLI exit 2). Real cancellation, provider and connection errors keep their existing channels. Uncalled providers cannot produce errors. Existing optional planner-error fallback behavior is unchanged. Applicable counts only evaluated rules.

## Limits and deferred scope

This admission bound is not a parser-depth, SQL-size, wall-clock, memory or OOM guarantee. Parsing/extraction, normalized input storage and retained result storage can still grow with input. HTTP's 1 MiB body rejection, parser, splitter, connection resolution and cancellation contract are unchanged. Splitting a migration across requests is not automatically equivalent because prior Prospective Schema State does not carry across requests.

A1–A6 and the accepted #82/#83 contracts are preserved. PROCEDURE definition/deletion isolation is a separate, unauthorized successor slice. No precise multi-action execution engine, whole-database dependency discovery, Query Access extension, manifest/runner/validator change, or database run belongs to A7. A7 is not completion or acceptance of #84 or the milestone.

## Verification links and ownership

- Public baseline red/green: `internal/application/audit/resource_limit_public_test.go` uses existing AuditSQL/report types and the 1025-statement short input; the red failure is a runtime assertion, not a missing-type compiler error.
- First-path controls and direct state non-publication: `internal/application/audit/resource_limit_test.go`; supplementary stale-conclusion and blocked-CREATE checks are in `resource_limit_matrix_test.go`.
- Shared semantic, call-stop, error, cancellation, counting and isolation matrix: `internal/application/audit/resource_limit_matrix_test.go`.
- PostgreSQL no/schema/provider and legacy request controls: `internal/application/audit/resource_limit_postgresql_tag_test.go`.
- Thin real entrypoints: `audit_resource_limit_a7_test.go` in `pkg/deltascope` and `internal/interfaces/{cli,http,mcp}`.
- The stdlib-only real-CLI proof and raw command/output/rc records are published separately under `docs/verification/t05-a7/`, keyed by the full tested code SHA. Pre-commit runs and final-SHA runs are labeled separately; evidence HEAD is not tested code HEAD.
- Required final gates are `GOFLAGS=-count=1 make test`, `make pg-unit-test-gates`, `make sql-corpus-gates`, `make ddl-inventory-gate`, `make ddl-coverage-catalog-test`, `make docs-example-gates`, `make decision-record-gate`, focused T05A7 entrypoints, gofmt and actual task-range diff/document checks.
- The prior 247-case T05 Golden and 204-case validator contract are reused by explicit Planner authorization, not reported as A7 reruns. See the [A6 evidence index](../verification/t05-a6/02843c405bcfbf912b0e9528e3cba99d7ac1f34b/README.md) and [originals index](../verification/t05-a6/02843c405bcfbf912b0e9528e3cba99d7ac1f34b/golden-originals.md).

Verification and Standards/Spec self-checks do not constitute the independent Reviewer's acceptance. The delivery stops at ready for human review.
