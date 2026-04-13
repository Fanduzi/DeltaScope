# DeltaScope Roadmap

This roadmap tracks near-term engineering milestones and explicit follow-up work that should remain visible in the repository.

It is not a promise of exhaustive SQL grammar support. DeltaScope continues to prioritize tested, auditable, offline-first coverage over broad syntax claims.

## Latest Completed Milestone: v0.27.0 Schema-Qualified Reference Semantics Pack

**Goal:** preserve PostgreSQL schema-qualified referenced-object facts in the shared contract, backed by corpus cases and service-level semantic tests.

The milestone answers a practical semantic-fidelity question: when a PostgreSQL `REFERENCES` clause includes a schema qualifier (`REFERENCES public.users(id)`), does DeltaScope preserve that schema in the shared contract?

### Completed Scope

- Additive `ReferencedSchema` field on `spec.Constraint`: schema-qualified `REFERENCES` facts are now preserved alongside the existing `ReferencedTable` and `ReferencedColumns`.
- PostgreSQL extractor populates `ReferencedSchema` for both named `FOREIGN KEY` and inline `REFERENCES` forms.
- Corpus cases lock schema-qualified reference semantics with precise `.expected.yaml` assertions (`ReferencedSchema = "public"`, `ReferencedTable = "users"`).
- Service-level semantic tests assert schema-qualified reference facts are preserved through the audit pipeline.
- Surface tests verify no contract regression across CLI, HTTP, MCP, and `pkg/deltascope`.
- Current public finding metadata remains unchanged — the shared semantic contract is richer underneath.

### Key Design Decisions

- `ReferencedSchema` is additive; `ReferencedTable` is never concatenated into `"public.users"`.
- This is not a new rule ID, not a new CLI flag, and not full PostgreSQL foreign key support.
- CLI / HTTP / MCP / `pkg/deltascope` do not currently expose `referenced_schema` in finding metadata.

## Previous Milestone: v0.26.0 PostgreSQL CREATE TABLE Unsupported Boundary Pack

Tightened the PostgreSQL `CREATE TABLE` unsupported boundary contract at the extractor level, backed by corpus cases and surface parity tests.

| Feature | Extractor Tag |
|---------|---------------|
| Identity columns (`GENERATED ... AS IDENTITY`) | `generated_as_identity` |
| Generated stored columns (`GENERATED ALWAYS AS ... STORED`) | `generated_column` |
| Exclusion constraints (`EXCLUDE USING`) | `exclusion_constraint` |
| Partitioned tables (`PARTITION BY`) | `partitioning` |

## Next Follow-up

Schema-qualified reference facts are now preserved in the shared contract. Follow-up decisions:

- Decide whether public finding metadata should expose referenced-object schema facts.
- Schema-aware FK policy/rule work remains a future decision point, not a committed milestone.
- `ALTER TABLE ... GENERATED` boundary coverage is a potential follow-up but is not committed.
