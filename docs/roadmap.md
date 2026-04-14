# DeltaScope Roadmap

This roadmap tracks near-term engineering milestones and explicit follow-up work that should remain visible in the repository.

It is not a promise of exhaustive SQL grammar support. DeltaScope continues to prioritize tested, auditable, offline-first coverage over broad syntax claims.

## Latest Completed Milestone: v0.29.0 Schema-Aware FK Policy Pack

**Goal:** use explicit PostgreSQL schema-qualified FK facts for one narrow policy decision: a notice-level cross-schema advisory when both schemas are explicit and different.

The milestone answers the next practical product question after `v0.28.0`: should the referenced-object metadata surface participate in policy at all? The answer is yes, but only for explicit cross-schema foreign keys.

### Completed Scope

- Added the PostgreSQL-only notice rule `ddl.pg.table.foreign_key.cross_schema.advisory`.
- The rule fires only when the owning table schema is explicit, the referenced schema is explicit, and those schemas differ.
- Same-schema foreign keys do not trigger the advisory.
- Bare references such as `REFERENCES users(id)` remain schema unknown; DeltaScope does not infer `public` and does not model PostgreSQL `search_path` semantics.
- Finding metadata can include `table_schema`, `referenced_schema`, `referenced_table`, and `referenced_columns`; `referenced_table` remains normalized as `"users"`, never `"auth.users"`.
- Surface tests lock the advisory contract across CLI, HTTP, MCP, and `pkg/deltascope`.

### Key Design Decisions

- This is the first schema-aware FK policy step, not full PostgreSQL foreign key support.
- The rule is advisory and notice-level; it adds context without replacing `ddl.table.foreign_key.forbid`.
- Cross-schema detection depends only on explicit SQL facts, not live metadata or search-path inference.
- The metadata representation stays normalized: schema and table remain separate fields.

## Previous Milestone: v0.27.0 Schema-Qualified Reference Semantics Pack

**Goal:** preserve PostgreSQL schema-qualified referenced-object facts in the shared contract, backed by corpus cases and service-level semantic tests.

- Additive `ReferencedSchema` field on `spec.Constraint`: schema-qualified `REFERENCES` facts are now preserved alongside the existing `ReferencedTable` and `ReferencedColumns`.
- PostgreSQL extractor populates `ReferencedSchema` for both named `FOREIGN KEY` and inline `REFERENCES` forms.
- Corpus cases lock schema-qualified reference semantics with precise `.expected.yaml` assertions.
- `ReferencedSchema` is additive; `ReferencedTable` is never concatenated into `"public.users"`.

## Previous Milestone: v0.26.0 PostgreSQL CREATE TABLE Unsupported Boundary Pack

Tightened the PostgreSQL `CREATE TABLE` unsupported boundary contract at the extractor level, backed by corpus cases and surface parity tests.

| Feature | Extractor Tag |
|---------|---------------|
| Identity columns (`GENERATED ... AS IDENTITY`) | `generated_as_identity` |
| Generated stored columns (`GENERATED ALWAYS AS ... STORED`) | `generated_column` |
| Exclusion constraints (`EXCLUDE USING`) | `exclusion_constraint` |
| Partitioned tables (`PARTITION BY`) | `partitioning` |

## Additional Follow-up

- Decide whether schema-aware FK policy should expand beyond the explicit cross-schema advisory shipped in `v0.29.0`.
- `ALTER TABLE ... GENERATED` boundary coverage remains a visible follow-up, but not a committed milestone.
