# DeltaScope Roadmap

This roadmap tracks near-term engineering milestones and explicit follow-up work that should remain visible in the repository.

It is not a promise of exhaustive SQL grammar support. DeltaScope continues to prioritize tested, auditable, offline-first coverage over broad syntax claims.

## Latest Completed Milestone: v0.26.0 PostgreSQL CREATE TABLE Unsupported Boundary Pack

**Goal:** tighten the PostgreSQL `CREATE TABLE` unsupported boundary contract at the extractor level, backed by corpus cases and surface parity tests.

The milestone answers a practical boundary-confidence question: which PostgreSQL `CREATE TABLE` forms are explicitly outside the supported surface, and how is that contract verified?

### Completed Scope

- Extractor-level `unsupportedStatement` return for `generated_as_identity`, `generated_column`, `exclusion_constraint`, and `partitioning`.
- PostgreSQL corpus cases lock each boundary with precise `.expected.yaml` assertions.
- Surface parity tests across CLI, HTTP, MCP, and `pkg/deltascope` verify the unsupported contract on every transport.
- Docs distinguish unsupported boundaries from supported `v0.23.0` / `v0.24.0` create-table semantics.

### Boundary Contracts

| Feature | Extractor Tag |
|---------|---------------|
| Identity columns (`GENERATED ... AS IDENTITY`) | `generated_as_identity` |
| Generated stored columns (`GENERATED ALWAYS AS ... STORED`) | `generated_column` |
| Exclusion constraints (`EXCLUDE USING`) | `exclusion_constraint` |
| Partitioned tables (`PARTITION BY`) | `partitioning` |

## Next Follow-up

Schema-qualified foreign-key references remain a decision point:

- If preserving schema requires a shared contract expansion, run impact analysis on `spec.Constraint` first.
- If the impact is not clearly low, defer `ReferencedSchema` to a later milestone rather than mixing it into boundary tightening.

Additional potential follow-up (not committed):

- `ALTER TABLE ... GENERATED` boundary coverage.
