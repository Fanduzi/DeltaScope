# DeltaScope Roadmap

This roadmap tracks near-term engineering milestones and explicit follow-up work that should remain visible in the repository.

It is not a promise of exhaustive SQL grammar support. DeltaScope continues to prioritize tested, auditable, offline-first coverage over broad syntax claims.

## Latest Completed Milestone: v0.28.0 Referenced-Object Metadata Surface Pack

**Goal:** expose PostgreSQL referenced-object facts that already existed in the shared semantic contract as additive finding metadata on the FK forbid rule, across all four transport surfaces.

The milestone answers a practical product question left open after `v0.27.0`: should CLI / HTTP / MCP / `pkg/deltascope` users be able to see `referenced_schema`, `referenced_table`, and `referenced_columns` directly in the relevant FK-related finding metadata?

### Completed Scope

- Impact analysis confirmed LOW blast radius on the FK forbid metadata builder path.
- Additive metadata widening: `ddl.table.foreign_key.forbid` finding metadata now includes `referenced_schema`, `referenced_table`, and `referenced_columns` when the underlying constraint carries those facts.
- `referenced_table` is never concatenated with `referenced_schema` (e.g., never `"public.users"`).
- Surface tests lock the widened metadata contract across CLI, HTTP, MCP, and `pkg/deltascope`.
- Docs and release surfaces describe metadata widening accurately without implying schema-aware rule support.

### Key Design Decisions

- Metadata widening is additive only; no existing metadata fields changed, no rule IDs changed.
- Conditional emission: `referenced_schema` is omitted when no schema qualifier is present.
- Parser/extractor semantics are unchanged from `v0.27.0`.
- This is not schema-aware FK policy support, not full PostgreSQL foreign key support, and not a new rule family.

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

## Next Milestone: v0.29.0 Schema-Aware FK Policy Pack

**Goal:** decide whether explicit PostgreSQL referenced-object schema facts should start influencing FK policy behavior, and ship only the narrowest schema-aware rule behavior that remains explainable and low-risk.

The milestone follows `v0.27.0` and `v0.28.0` in sequence:

- `v0.27.0` preserved schema-qualified FK facts in the shared semantic contract.
- `v0.28.0` exposed those facts on outward FK forbid finding metadata.
- `v0.29.0` should answer whether explicit schema-qualified FK facts have policy value, not just descriptive value.

### Planned Scope

- Decision gate first: use GitNexus impact analysis to determine whether a narrow schema-aware FK policy path can be introduced with low blast radius.
- Treat bare references such as `REFERENCES users(id)` as schema unknown.
- Do not infer `public`.
- Do not model PostgreSQL `search_path`.
- Prefer one narrow PostgreSQL-specific advisory or policy distinction over a broad new rule pack.
- Preserve the current normalized representation: schema and table remain separate facts.

### Explicit Non-Goals

- Full PostgreSQL foreign key support
- Complete schema-aware PostgreSQL modeling
- Cross-schema validation matrices
- Parser/extractor redesign
- MySQL or TiDB policy changes

## Additional Follow-up

- `ALTER TABLE ... GENERATED` boundary coverage is still a potential follow-up, but not a committed milestone.
