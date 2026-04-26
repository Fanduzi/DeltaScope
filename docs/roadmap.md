# DeltaScope Roadmap

This roadmap tracks near-term engineering milestones and explicit follow-up work that should remain visible in the repository.

It is not a promise of exhaustive SQL grammar support. DeltaScope continues to prioritize tested, auditable, offline-first coverage over broad syntax claims.

## Latest Completed Milestone: v0.45.0 GitLab CI Integration Pack

**Goal:** add GitLab Code Quality report output so merge-request pipelines can surface SQL audit findings as inline code-quality annotations without any post-processing.

### Completed Scope

- `--format gitlab-codequality` CLI flag produces a JSON array matching the GitLab Code Quality report contract.
- Each DeltaScope finding maps to a Code Quality entry with `check_name`, `description`, `severity`, `fingerprint`, and `location` fields.
- File paths from `--file` propagate to `location.path`; inline SQL uses the audit input filename.
- Contract characterization tests lock the required JSON shape and semantic field guarantees.
- Unit tests cover the renderer with zero findings, single finding, and multiple finding cases.
- `make release-gitlab-codequality-smoke` gate validates the built CLI binary against the contract in the release pipeline.
- `make release-contract-gates` now includes the GitLab Code Quality smoke.
- Recipe: Using DeltaScope in GitLab CI with step-by-step `.gitlab-ci.yml` setup.
- CLI reference updated with `--format` flag documentation.
- Audit capability matrix updated to list GitLab Code Quality as a supported output format.

### Key Design Decisions

- No parser, spec, domain-rule, or policy changes.
- No HTTP, MCP, or `pkg/deltascope` production-code changes.
- No new dependencies.
- GitLab Code Quality renderer is a pure output adapter in `internal/infrastructure/output/gitlabcodequality`.

## Previous Milestone: v0.44.0 Release Contract Hardening Pack

**Goal:** harden the release contract surface with unified gates, binary version smoke, default policy dialect isolation smoke, and archive verification so every tagged release passes a deterministic pre-publish check.

### Completed Scope

- Unified `make release-contract-gates VERSION=vX.Y.Z` combining version surface, binary version smoke, default policy dialect isolation, and archive verification.
- `scripts/verify_release_version_surfaces.sh` — verifies source constants, npm package, README install pins, release notes, and landing DOM/JS i18n against the tagged version.
- `scripts/verify_release_version.sh` — builds all three binaries with ldflags and asserts CLI, server, and MCP version output.
- `scripts/verify_release_dialect_hygiene.sh` — asserts PostgreSQL audits do not emit MySQL/TiDB-only rule IDs and MySQL/TiDB audits do not emit PostgreSQL-only rule IDs.
- `scripts/verify_release_archive.sh` — builds cross-compiled archive, verifies filename contract, checksum, binary version, and dialect hygiene against extracted binary.
- Default policy dialect hygiene e2e test added to the release test gate suite.
- Release workflow wired to run `release-contract-gates` before publish.
- Local go-release skill checklist updated with gate verification steps.

### Key Design Decisions

- No production Go code changes — all gates are scripts and tests.
- No new rule IDs, parser features, or public API contracts.
- Dialect hygiene gate tests at the binary level, not the unit level.
- Archive verification uses the cross-compiled Linux amd64 binary to match the release artifact.

## Previous Milestone: v0.43.0 Default Policy Dialect Hygiene Pack

**Goal:** make the shipped default policy respect the user-selected SQL dialect across MySQL, TiDB, and PostgreSQL, so that PostgreSQL audits never emit MySQL/TiDB-only rule IDs or remediation text and MySQL/TiDB audits never emit PostgreSQL-only rule IDs.

### Completed Scope

- Default policy isolates rules by `--dialect` across MySQL, TiDB, and PostgreSQL.
- PostgreSQL audits skip MySQL-family rules: engine allowlist, charset allowlist, row format allowlist, auto_increment init value, unsigned/auto_increment/not_null primary-key requirements, partition forbid, create_as/create_like forbid, column charset/collation allowlists, column charset_collation match, change_column/modify_column forbid, and the `ON UPDATE CURRENT_TIMESTAMP` audit-column suggestion.
- MySQL/TiDB audits exclude all `ddl.pg.*` rules and PostgreSQL-only dialect-gated rules.
- Isolation is enforced at the rule `AppliesTo` gate level, not by post-filtering reports.
- Service-level tests assert cross-dialect rule ID isolation and remediation text isolation.
- SQL corpus PostgreSQL probe includes a negative `exclude:` block listing MySQL-family rules that must never appear.

### Key Design Decisions

- No new rule IDs, parser features, or public API contracts.
- Isolation at the `AppliesTo` gate level, not report-level filtering, so rule accounting and `Applicable` counts remain accurate.
- No live schema validation, cross-database tracking, or MySQL/TiDB behavior changes beyond dialect isolation.
- PostgreSQL type canonicalization (`pg_catalog.int8` normalization) remains out of scope for this milestone.

## Previous Milestone: v0.42.0 PostgreSQL NOT VALID Constraint Validation Pairing Pack

**Goal:** add a PostgreSQL-only global audit rule that warns when a named `ALTER TABLE ... ADD CONSTRAINT ... NOT VALID` CHECK or FOREIGN KEY constraint is not followed by a later matching `ALTER TABLE ... VALIDATE CONSTRAINT ...` statement in the same audited SQL batch.

### Completed Scope

- Added PostgreSQL-only GlobalRule `ddl.pg.alter.not_valid_constraint.validate.require` with default level `warning`.
- The rule applies to named CHECK and FOREIGN KEY `NOT VALID` constraint additions and matches later validation using the same schema + table + constraint name.
- A later matching `VALIDATE CONSTRAINT` suppresses the warning; earlier validation or mismatched table/schema/name does not.
- CLI, HTTP, MCP, and `pkg/deltascope` surfaces expose the result as a global finding.
- SQL corpus expected outcomes and service-level tests lock the rule contract.
- Docker-backed PostgreSQL e2e covers the release-facing user path.

### Key Design Decisions

- This is not first-time `VALIDATE CONSTRAINT` parser support; the statement was already supported and auditable.
- No live database validation-state lookup or cross-file / cross-deployment tracking.
- Unnamed constraints are skipped rather than guessed.
- No CHECK expression correctness validation, FK referenced-table correctness validation, MySQL/TiDB behavior changes, or new public API contract.

## Previous Milestone: v0.41.0 PostgreSQL ALTER TABLE CHECK Fact Support Pack

**Goal:** preserve statement-local foreign key facts for approved PostgreSQL `ALTER TABLE ... ADD CONSTRAINT FOREIGN KEY` forms, allowing existing FK rules to produce findings across all product surfaces.

### Completed Scope

- PostgreSQL `ALTER TABLE ... ADD CONSTRAINT FOREIGN KEY` statement-local FK facts: named and unnamed FK forms now preserve local columns, referenced table, referenced columns, and referenced schema (for schema-qualified references) through the PostgreSQL extractor.
- The `DDL.Constraints` projection allows existing FK rules to trigger on ALTER TABLE FK additions.
- Rules now covering PostgreSQL `ALTER TABLE ... ADD CONSTRAINT ... FOREIGN KEY`:
  - `ddl.table.foreign_key.forbid` — flags foreign key constraints as forbidden under the default policy.
  - `ddl.pg.table.foreign_key.cross_schema.advisory` — emits a notice when owning and referenced schemas differ.
- CLI, HTTP, MCP, and `pkg/deltascope` surfaces produce explicit `rule_id` findings.
- Corpus expected outcomes and service-level tests lock PostgreSQL ALTER TABLE FK fact extraction and rule coverage.
- Docker-backed PostgreSQL CLI e2e covers `ddl.table.foreign_key.forbid`.

### Key Design Decisions

- No new rule IDs — existing FK rules cover ALTER TABLE FK additions through extended applicability and the `DDL.Constraints` projection.
- No live schema FK existence validation — statement-local facts only.
- No deferrable constraint support, MATCH FULL policy expansion, or full constraint/index parity claim.
- No MySQL/TiDB behavior changes.

## Previous Milestone: v0.39.0 PostgreSQL ALTER TABLE Constraint Fact Support Pack

**Goal:** preserve statement-local primary-key and unique constraint facts for approved `ALTER TABLE ... ADD CONSTRAINT` forms, allowing existing primary-key and unique/index rules to produce findings across all product surfaces.

### Completed Scope

- PostgreSQL `ALTER TABLE ... ADD CONSTRAINT` primary-key and unique constraint facts: inline, named, and unnamed forms now preserve statement-local constraint metadata through the PostgreSQL extractor and rule projection helpers.
- Rules now covering PostgreSQL `ALTER TABLE ... ADD CONSTRAINT`:
  - `ddl.table.primary_key.bigint.require` — flags PostgreSQL primary-key columns that are not BIGINT.
  - `ddl.table.primary_key.columns.max_count` — flags PostgreSQL composite primary keys that exceed the configured column limit.
  - `ddl.alter.add_index.unique.prefix.require` — flags unique constraint names that do not start with the required prefix.
- CLI, HTTP, MCP, and `pkg/deltascope` surfaces produce explicit `rule_id` findings.
- Corpus expected outcomes and service-level tests lock PostgreSQL constraint rule coverage.
- Docker-backed PostgreSQL CLI e2e covers `ddl.alter.add_index.unique.prefix.require`.

### Key Design Decisions

- No new rule IDs — existing rules cover approved forms through extended applicability and projection helpers.
- Approved forms only — foreign keys, check constraints, exclusion constraints, deferrability, validation lifecycle, partial/expression index semantics, operator classes, and live schema reconstruction remain out of scope.
- No MySQL/TiDB behavior changes.

## Previous Milestone: v0.38.0 PostgreSQL Unique/Index Rule Coverage

**Goal:** extend PostgreSQL unique/index audit coverage for statement-local unique constraints and simple btree `CREATE INDEX` forms. Existing index rules now produce findings for the approved PostgreSQL forms, with corpus, public-surface, and Docker-backed e2e coverage.

### Completed Scope

- Standalone PostgreSQL `CREATE INDEX`, `CREATE UNIQUE INDEX`, and `CREATE INDEX CONCURRENTLY` statements now trigger existing generic index rules for approved btree forms.
- Rules now covering PostgreSQL standalone `CREATE INDEX`:
  - `ddl.index.secondary.prefix.require` — flags secondary index names that do not start with the required prefix.
  - `ddl.index.unique.prefix.require` — flags unique index names that do not start with the required prefix.
  - `ddl.index.columns.max_count` — flags indexes that exceed the configured column limit.
- CLI, HTTP, MCP, and `pkg/deltascope` surfaces produce explicit `rule_id` findings.
- Corpus expected outcomes and service-level tests lock PostgreSQL index rule coverage.
- Docker-backed PostgreSQL CLI e2e covers `ddl.index.unique.prefix.require`.

### Key Design Decisions

- No new rule IDs — existing generic index rules now cover standalone `CREATE INDEX` through extended applicability gates.
- Approved forms only — partial indexes, expression indexes, INCLUDE, operator classes, non-btree access methods, and NULLS NOT DISTINCT remain out of scope.
- No live schema index introspection.
- No MySQL/TiDB behavior changes.

## Previous Milestone: v0.37.0 PostgreSQL Primary Key Fact Support Pack

**Goal:** add PostgreSQL `CREATE TABLE` primary-key fact support so that inline, table-level, named, and composite primary-key declarations populate DeltaScope's normalized primary-key contract, allowing existing primary-key rules to audit PostgreSQL `CREATE TABLE` statements.

### Completed Scope

- PostgreSQL extractor populates shared `DDL.PrimaryKey` for `CREATE TABLE` inline (`id bigint PRIMARY KEY`), table-level (`PRIMARY KEY (id)`), named (`CONSTRAINT users_pkey PRIMARY KEY (id)`), and composite (`PRIMARY KEY (a, b)`) forms.
- Primary-key columns are treated as effectively `NOT NULL` for PostgreSQL.
- Existing primary-key rules now apply to PostgreSQL:
  - `ddl.table.primary_key.bigint.require` — flags non-BIGINT primary-key columns.
  - `ddl.table.primary_key.columns.max_count` — flags composite primary keys exceeding the column limit.
- CLI, HTTP, MCP, and `pkg/deltascope` surfaces produce explicit `rule_id` findings.
- Corpus expected outcomes and service-level tests lock PostgreSQL primary-key facts and rule coverage.

### Key Design Decisions

- Primary-key fact support for `CREATE TABLE` only — not `ALTER TABLE ADD PRIMARY KEY`, not full PostgreSQL index support, not live schema primary-key introspection.
- No new primary-key rule IDs — existing rules now cover PostgreSQL through shared extractor facts.
- `ddl.table.primary_key.not_null.require` does not produce a stable negative case for PostgreSQL because PK columns are treated as effectively NOT NULL.
- No MySQL/TiDB behavior changes.

## Previous Milestone: v0.36.1 SQL Corpus Coverage Patch

**Goal:** make the existing supported-rule corpus contract explicit, visible, and enforced in release validation.

- `make sql-corpus-gates` verifies every currently supported `rule_id × dialect` surface has at least one SQL corpus case.
- `make sql-corpus-report` prints the current supported-rule coverage inventory.
- Release gates now include SQL corpus coverage verification.

## Previous Milestone: v0.36.0 PostgreSQL Generated/Identity Rule Coverage Pack

**Goal:** extend PostgreSQL DDL rule coverage to the generated/identity state-transition forms that became supported in v0.35.0, so those forms produce explicit `rule_id` findings instead of passing silently.

### Completed Scope

- Three new PostgreSQL-only forbid rules registered:
  - `ddl.alter.drop_expression.forbid` — flags `ALTER TABLE ... ALTER COLUMN ... DROP EXPRESSION`
  - `ddl.alter.set_generated.forbid` — flags `ALTER TABLE ... ALTER COLUMN ... SET GENERATED ...`
  - `ddl.alter.drop_identity.forbid` — flags `ALTER TABLE ... ALTER COLUMN ... DROP IDENTITY`
- Rules use existing `newForbiddenAlterActionRule` with PostgreSQL-only dialect allowlist.
- CLI, HTTP, MCP, and `pkg/deltascope` surfaces produce explicit `rule_id` findings.
- No parser support widening, no spec contract widening, no new spec fields.

### Key Design Decisions

- Rule coverage only — not parser support widening, not spec contract widening, not generated expression evaluation, not complete PostgreSQL sequence semantics.
- `GeneratedExpression` still deferred — no stable expression renderer.
- No MySQL/TiDB behavior changes.

## Previous Milestone: v0.35.0 PostgreSQL Generated/Identity State-Transition Pack

**Goal:** support PostgreSQL generated/identity state-transition forms through the normal audit path, without adding full generated-column lifecycle support, generated expression evaluation, complete PostgreSQL sequence semantics, or new rule IDs.

### Completed Scope

- PostgreSQL extractor no longer rejects state-transition forms at the unsupported boundary.
- Supported forms: `DROP EXPRESSION`, `SET GENERATED ALWAYS`, `SET GENERATED BY DEFAULT`, `DROP IDENTITY`.
- Normalized contract: `drop_expression`, `set_generated` with `generated_when` (`"a"` / `"d"`), `drop_identity`.
- Corpus expected outcomes and service-level tests updated to assert supported results.
- Surface tests across CLI, HTTP, MCP, and `pkg/deltascope` switched from unsupported to supported contract assertions.

### Key Design Decisions

- State-transition support only — not full generated-column lifecycle support, not generated expression evaluation, not complete PostgreSQL sequence semantics.
- `GeneratedExpression` still deferred — no stable expression renderer.
- No new rule IDs, CLI flags, or rule behavior changes.
- No MySQL/TiDB behavior changes.

## Previous Milestone: v0.34.0 PostgreSQL Generated/Identity Narrow Support Pack

**Goal:** widen PostgreSQL generated/identity support so that narrow definition forms are processed through the normal audit path.

- PostgreSQL extractor no longer rejects narrow generated/identity definition forms (`CREATE TABLE` and `ALTER TABLE ADD COLUMN`) at the unsupported boundary.
- Shared facts from v0.33.0 continue flowing: `generated_when`, `is_identity`, `identity_options`.
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
- Decide whether remaining PostgreSQL generated/identity edge cases need coverage.
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
