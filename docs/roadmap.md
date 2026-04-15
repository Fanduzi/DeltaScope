# DeltaScope Roadmap

This roadmap tracks near-term engineering milestones and explicit follow-up work that should remain visible in the repository.

It is not a promise of exhaustive SQL grammar support. DeltaScope continues to prioritize tested, auditable, offline-first coverage over broad syntax claims.

## Latest Completed Milestone: v0.34.0 PostgreSQL Generated/Identity Narrow Support Pack

**Goal:** widen PostgreSQL generated/identity support so that narrow definition forms are processed through the normal audit path, without adding full generated-column support, full identity-column support, generated expression evaluation, or state-transition support.

### Completed Scope

- PostgreSQL extractor no longer rejects narrow generated/identity definition forms (`CREATE TABLE` and `ALTER TABLE ADD COLUMN`) at the unsupported boundary.
- Corpus expected outcomes and service-level tests updated to assert supported results for narrow forms.
- Surface tests across CLI, HTTP, MCP, and `pkg/deltascope` switched from unsupported to supported contract assertions.
- Shared facts from v0.33.0 continue flowing: `generated_when`, `is_identity`, `identity_options`.

### Key Design Decisions

- Narrow support only — not full generated-column support, not full identity-column support, not generated expression evaluation.
- `GeneratedExpression` still deferred — no stable expression renderer.
- State-transition forms remain unsupported: `DROP EXPRESSION` (`generated_column`), `SET GENERATED` (`generated_as_identity`), `DROP IDENTITY` (`generated_as_identity`).
- No new rule IDs, CLI flags, or rule behavior changes.

## Previous Milestone: v0.33.0 PostgreSQL Generated/Identity Fact Preservation + Unsupported Metadata Surfacing Pack

**Goal:** preserve narrow generated/identity column facts in the shared DDL contract and surface structured metadata on unsupported generated/identity outcomes.

- `GeneratedWhen`, `IsIdentity`, `IdentityOptions` added to `spec.Column`.
- `Metadata map[string]any` added to `spec.UnsupportedDetail` for structured unsupported outcomes.
- Corpus, service, and surface parity tests lock the new contract.
- No new rule IDs, CLI flags, or rule behavior changes.

## Previous Milestone: v0.32.0 PostgreSQL Boundary Support-Readiness Gate

**Goal:** produce an evidence-backed decision about PostgreSQL generated/identity support readiness, documenting stable AST facts and recommending the next milestone direction.

- Characterization tests documenting stable AST facts in `parser_test.go`.
- Decision report with complete unsupported boundary inventory, AST fact coverage table, and v0.33.0 recommendation.
- No production code, extractor, spec, rule, or policy changes.

## Previous Milestone: v0.31.0 PostgreSQL ALTER TABLE GENERATED Follow-up Pack

**Goal:** map additional PostgreSQL generated/identity `ALTER TABLE` forms to explicit unsupported feature tags, closing the adjacent gap left by `v0.30.0`.

The milestone follows the boundary discipline from `v0.26.0` (`CREATE TABLE`) and `v0.30.0` (`ADD COLUMN`). `v0.31.0` extends the same explicit unsupported contract shape to the remaining generated/identity alteration forms without broadening semantic support.

### Completed Scope

- Locked `ALTER TABLE ... ALTER COLUMN ... DROP EXPRESSION` to explicit unsupported `generated_column`.
- Locked `ALTER TABLE ... ALTER COLUMN ... SET GENERATED ...` to explicit unsupported `generated_as_identity`.
- Locked `ALTER TABLE ... ALTER COLUMN ... DROP IDENTITY` to explicit unsupported `generated_as_identity`.
- Added corpus, service, and CLI / HTTP / MCP / `pkg/deltascope` parity coverage for the same boundary contract.
- Kept the release framed as boundary tightening, not generated-column support, identity-column support, or complete PostgreSQL `ALTER TABLE` support.

### Key Design Decisions

- Reuse existing unsupported feature names (`generated_column`, `generated_as_identity`) from `v0.26.0` and `v0.30.0`.
- Do not add new rule IDs, CLI flags, or public API contracts.
- Keep unsupported behavior explicit at every public surface.
- Do not imply support for generated expressions or identity semantics beyond the locked unsupported outcomes.

## Previous Milestone: v0.30.0 PostgreSQL ALTER TABLE GENERATED Boundary Pack

**Goal:** tighten PostgreSQL `ALTER TABLE ... ADD COLUMN` generated/identity boundaries so generated stored and identity add-column forms become explicit unsupported outcomes instead of accidental supported actions or ordinary add-column fallthrough.

- Locked `ALTER TABLE ... ADD COLUMN ... GENERATED ALWAYS AS (...) STORED` to explicit unsupported `generated_column`.
- Locked `ALTER TABLE ... ADD COLUMN ... GENERATED ALWAYS AS IDENTITY` to explicit unsupported `generated_as_identity`.
- Corpus, service, and CLI / HTTP / MCP / `pkg/deltascope` parity coverage locked this boundary contract.

### Key Design Decisions

- Reuse existing unsupported feature names where semantics already match.
- Do not add new rule IDs, CLI flags, or public API contracts.
- Keep unsupported behavior explicit at every public surface.
- Do not imply support for generated expressions or identity semantics beyond the locked unsupported outcomes.

## Next Follow-up

- Decide whether `GeneratedExpression` should be addressed once `pg_query_go` exposes a stable expression deparse path.
- Decide whether ALTER TABLE state-transition forms should receive metadata surfacing.
- Decide whether MCP surface should expose unsupported metadata directly.

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
- Decide later whether explicit generated/identity unsupported boundaries should ever become real PostgreSQL generated-column or identity-column support.
