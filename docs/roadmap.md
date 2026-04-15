# DeltaScope Roadmap

This roadmap tracks near-term engineering milestones and explicit follow-up work that should remain visible in the repository.

It is not a promise of exhaustive SQL grammar support. DeltaScope continues to prioritize tested, auditable, offline-first coverage over broad syntax claims.

## Latest Completed Milestone: v0.32.0 PostgreSQL Boundary Support-Readiness Gate

**Goal:** produce an evidence-backed decision about PostgreSQL generated/identity support readiness, documenting stable AST facts and recommending the next milestone direction.

### Completed Scope

- Characterization tests in `internal/infrastructure/parser/postgresql/parser_test.go` documenting stable AST facts: `GeneratedWhen` encoding (`"a"` / `"d"`), `CONSTR_IDENTITY` / `CONSTR_GENERATED` types, identity sequence option shape, and AST consistency between `CREATE TABLE` and `ALTER TABLE ADD COLUMN`.
- Decision report at `docs/plans/reports/2026-04-14-v0.32.0-pg-boundary-support-readiness-report.md` with complete unsupported boundary inventory, AST fact coverage table, shared contract decision, and v0.33.0 recommendation.
- No production code, extractor, spec, rule, or policy changes.

### Key Design Decisions

- Decision gate only — not a feature release.
- DeltaScope is ready for narrow fact preservation, not full generated/identity semantic support.
- Recommended v0.33.0 fields: `GeneratedWhen` (string) and `IsIdentity` (bool) on `spec.Column` as `omitempty` additions.
- Deferred: generated expression rendering, identity sequence option normalization, rule behavior changes, ALTER TABLE state transitions.

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

## Next Milestone: v0.33.0 PostgreSQL Generated/Identity Fact Preservation Pack

**Goal:** preserve narrow generated/identity facts in the shared DDL contract while keeping generated expressions, identity sequence options, and ALTER TABLE state transitions unsupported.

### Candidate Scope

- Add `GeneratedWhen` (string, `omitempty`) and `IsIdentity` (bool, `omitempty`) to `spec.Column`.
- Start with `CREATE TABLE` facts; include `ALTER TABLE ADD COLUMN` only if the field model is identical.
- Add PostgreSQL corpus YAML entries for generated/identity test cases.
- Update CLI, HTTP, MCP, and `pkg/deltascope` surface tests to validate new optional fields.

### Explicit Non-Goals

- Generated expression rendering
- Identity sequence option normalization
- Generated/identity rule behavior
- ALTER TABLE state-transition support (SET GENERATED, DROP EXPRESSION, DROP IDENTITY)
- Removing existing unsupported boundaries before fact preservation is proven
- Changing any MySQL/TiDB behavior

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
