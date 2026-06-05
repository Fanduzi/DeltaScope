# DeltaScope Roadmap

This roadmap tracks near-term engineering milestones and explicit follow-up work that should remain visible in the repository.

It is not a promise of exhaustive SQL grammar support. DeltaScope continues to prioritize tested, auditable, offline-first coverage over broad syntax claims.

## Latest Completed Milestone: v0.270.0 DDL Coverage Catalog

**Goal:** create a consolidated DDL coverage catalog listing every DDL form DeltaScope has verified across MySQL, TiDB, and PostgreSQL. The catalog is machine-readable JSON generated from census data, with bilingual user documentation and a drift gate. Does not add parser support, SQL audit rules, fallback parser, reduce parser_error counts, or change SQL audit behavior.

### Completed Scope

- Machine-readable catalog: `docs/reference/ddl-coverage-catalog.json` — 400 entries (MySQL 61, TiDB 54, PostgreSQL 285).
- Classification vocabulary: `finding_covered`, `normalized_silent`, `unsupported_boundary`, `parser_error`, `unclassified`.
- 18 entries with `guidance_code=parser_upgrade_candidate`, including all 10 required candidates.
- Drift gate: `make ddl-coverage-catalog-test` keeps catalog JSON aligned with census tests.
- English documentation: `docs/reference/ddl-coverage.md`.
- Chinese documentation: `docs/reference/ddl-coverage.zh-CN.md` (full parity).
- No-leak guarantee: catalog contains no raw SQL, parser near-text, object names, or user payload.
- Decision record: `docs/decisions/2026-06-05-v0.270.0-ddl-coverage-catalog.md`.

### Non-Goals

- Not new parser support.
- Not new SQL audit rules.
- Not fallback parser implementation.
- Not reduced parser_error counts.
- Not full MySQL/TiDB/PostgreSQL DDL support.
- Not dialect parity.
- Not SQL audit behavior changes of any kind.

## Previous Milestone: v0.260.0 Unsupported Diagnostics Guidance Codes

**Goal:** add `guidance_code` and `evidence_ref` to parser_error diagnostics so users can tell which unsupported boundary a statement falls into and find detailed evidence documentation. Does not add parser support, SQL audit rules, fallback parser, reduce parser_error counts, or change SQL audit behavior.

### Completed Scope

- `spec.Diagnostic.GuidanceCode` and `spec.Diagnostic.EvidenceRef` optional fields on diagnostics.
- Guidance classification logic: `classifyParserErrorGuidance(sql, dialect)` for safe, fixed-keyword matching.
- First verified parser-upgrade candidates: MySQL `ALTER VIEW`, PostgreSQL `DROP SUBSCRIPTION ... WITH (drop_slot = true)`.
- Public surface parity: SDK, CLI JSON, CLI text (markdown and quiet), HTTP, MCP.
- No-leak guarantee: guidance fields never contain raw SQL, parser near-text, object names, or user payload.
- Decision record: `docs/decisions/2026-06-04-v0.260.0-unsupported-diagnostics-guidance-codes.md`.

### Non-Goals

- Not new parser support.
- Not new SQL audit rules.
- Not fallback parser implementation.
- Not reduced parser_error counts.
- Not full MySQL/TiDB/PostgreSQL DDL support.
- Not dialect parity.
- Not SQL audit behavior changes of any kind.

## Previous Milestone: v0.251.0 Release Tag Annotation Guard

**Goal:** add a release tag annotation guard that prevents lightweight git tags from passing through the release pipeline. Does not change SQL audit behavior, parser support, audit rules, parser_error counts, or public output shapes.

### Completed Scope

- Tag annotation verification script (`scripts/verify_release_tag_annotation.py`) and offline tests.
- `make release-tag-annotation-test` and `make release-tag-annotation-gate VERSION=vX.Y.Z` Makefile targets.
- Release workflow guard: early step in `release-linux` job rejects lightweight tags.
- Operator docs in `docs/dev/testing.md` and `docs/dev/release-recovery.md`.
- Decision record: `docs/decisions/2026-06-03-v0.251.0-release-tag-annotation-guard.md`.

### Non-Goals

- Not new parser support.
- Not new SQL audit rules.
- Not fallback parser implementation.
- Not reduced parser_error counts.
- Not full MySQL/TiDB/PostgreSQL DDL support.
- Not dialect parity.
- Not SQL audit behavior changes of any kind.

**Goal:** make the `release-recover.yml` workflow default-safe by adding a `dry_run` input (default `true`). In dry-run mode, the workflow exercises the full recovery preflight pipeline without executing destructive or publishing side effects. Real recovery requires explicitly setting `dry_run=false`. Does not change SQL audit behavior, parser support, audit rules, or parser_error counts.

### Completed Scope

- Default-safe `dry_run` input on `release-recover.yml` (default `true`).
- Dry-run verification scope: release asset/checksum preflight, Homebrew cask render/verify with tap clone/diff, npm package state check.
- Dry-run suppresses: Homebrew tap push, Homebrew install verification, npm publish, GitHub release upload/delete, git tag operations.
- `make release-recovery-contract-test` target verifying dry-run default and contract gates.
- Decision record: `docs/decisions/2026-05-31-v0.241.0-release-recovery-dry-run-patch.md`.

### Non-Goals

- Not new parser support.
- Not new SQL audit rules.
- Not fallback parser implementation.
- Not reduced parser_error counts.
- Not SQL audit behavior changes of any kind.

## Previous Milestone: v0.240.0 Release Workflow Idempotency & Recovery Hardening

**Goal:** harden the release workflow against partial downstream failures and accidental reruns. Adds release asset and npm package preflight verification helpers, a manual `release-recover.yml` workflow for operator-driven recovery, and an idempotency guard in the full release workflow. Does not change SQL audit behavior, parser support, audit rules, or parser_error counts.

### Completed Scope

- Release recovery preflight helpers (`scripts/verify_release_assets.py`, `scripts/verify_npm_package_state.sh`): read-only verification of release assets, checksums, and npm package state.
- Manual release recovery workflow (`.github/workflows/release-recover.yml`): operator-triggered downstream job recovery without re-running GoReleaser.
- Full release idempotency guard (`.github/workflows/release.yml`): prevents rerunning GoReleaser after release assets already exist.
- Operator docs for release recovery procedures (`docs/dev/release-recovery.md`).
- Decision record: `docs/decisions/2026-05-30-v0.240.0-release-workflow-idempotency-recovery-hardening.md`.

### Non-Goals

- Not new parser support.
- Not new SQL audit rules.
- Not fallback parser implementation.
- Not reduced parser_error counts.
- Not SQL audit behavior changes of any kind.

## Previous Milestone: v0.230.0 Unsupported Diagnostics Evidence

**Goal:** introduce structured diagnostics evidence for parser-error and unsupported-statement outcomes across all four product surfaces (SDK, CLI, HTTP, MCP). Every statement that DeltaScope cannot fully audit now carries a `spec.Diagnostic` with `classification`, `reason`, `action_hint`, `audited`, and `dialect` fields. Does not add new parser support, new SQL audit rules, fallback parser implementation, or reduce parser_error counts.

### Completed Scope

- `spec.Diagnostic` structured diagnostic with `classification`, `reason`, `action_hint`, `audited`, `dialect` fields.
- `report.Result.Diagnostics` diagnostics array on the public result.
- Parser-error diagnostic: `classification=parser_error`, standardized reason and action hint.
- Unsupported-statement diagnostic: `classification=unsupported_statement`, standardized reason and action hint.
- Public surface coverage: SDK, CLI, HTTP, MCP.
- No raw SQL or payload copied into diagnostics.

### Parser-Error Counts (unchanged)

| Dialect | Parser Error |
|---------|:------------:|
| MySQL | 15 |
| TiDB | 9 |
| PostgreSQL | 5 |
| **Total** | **29** |

### DDL Coverage Census (unchanged)

| Dialect | Total | Finding | Silent | Unsupported | Parser Error |
|---------|------:|--------:|-------:|:-----------:|:------------:|
| MySQL | 61 | 46 | 0 | 0 | 15 |
| TiDB | 54 | 45 | 0 | 0 | 9 |
| PostgreSQL (consolidated tracked-case) | 285 | 274 | 6 | 0 | 5 |

### Unchanged Metrics

- SQL corpus: **582/582**, **100.0%**, **245 YAML** fixture files.
- PostgreSQL ALTER TABLE rule count: **32** (unchanged).
- PostgreSQL consolidated DDL census: **285/274/6/0/5/0** (unchanged).

### Key Design Decisions

- Not new parser support.
- Not new SQL audit rules.
- Not fallback parser implementation.
- Not reduced parser_error counts.
- Not full MySQL/TiDB/PostgreSQL DDL support.
- Not dialect parity.

## Previous Milestone: v0.220.0 Parser-Error Unsupported Contract Hardening

**Goal:** harden the public unsupported-contract diagnostic across SDK, CLI, HTTP, and MCP so parser-error statements emit a standardized diagnostic instead of raw parser fragments or forbidden payloads. Does not add new parser support, new SQL audit rules, fallback parser implementation, or reduce parser_error counts.

### Completed Scope

- Standardized diagnostic: `statement was not audited because the selected dialect parser could not parse it; no audit findings were inferred`.
- Raw parser `near ...` fragments and tracked forbidden payloads removed from public output.
- PostgreSQL capability boundary error preserved.
- Public surface coverage: SDK, CLI, HTTP, MCP.

### Parser-Error Counts (unchanged)

| Dialect | Parser Error |
|---------|:------------:|
| MySQL | 15 |
| TiDB | 9 |
| PostgreSQL | 5 |
| **Total** | **29** |

### DDL Coverage Census (unchanged)

| Dialect | Total | Finding | Silent | Unsupported | Parser Error |
|---------|------:|--------:|-------:|:-----------:|:------------:|
| MySQL | 61 | 46 | 0 | 0 | 15 |
| TiDB | 54 | 45 | 0 | 0 | 9 |
| PostgreSQL (consolidated tracked-case) | 285 | 274 | 6 | 0 | 5 |

### Unchanged Metrics

- SQL corpus: **582/582**, **100.0%**, **245 YAML** fixture files.
- PostgreSQL ALTER TABLE rule count: **32** (unchanged).
- PostgreSQL consolidated DDL census: **285/274/6/0/5/0** (unchanged).

### Key Design Decisions

- Not new parser support.
- Not new SQL audit rules.
- Not fallback parser implementation.
- Not reduced parser_error counts.
- Not full MySQL/TiDB/PostgreSQL DDL support.
- Not dialect parity.

## Previous Milestone: v0.210.0 Cross-Dialect Parser-Error Feasibility Census

**Goal:** promote all previously normalized-silent MySQL and TiDB DDL forms to finding-covered status through a 28-rule DDL notice pack. Does not add new parser support, change PostgreSQL audit behavior, or claim full DDL coverage.

### Completed Scope

- 28 new DDL notice rules covering MySQL/TiDB lifecycle DDL events, ALTER TABLE action notices, TiDB placement policy/sequence lifecycle, and MySQL DROP RESOURCE GROUP.
- MySQL normalized_silent dropped from 25 to 0; finding_covered rose from 21 to 46.
- TiDB normalized_silent dropped from 27 to 0; finding_covered rose from 18 to 45.
- SQL corpus: **582/582**, **100.0%**, **245 YAML** fixture files.
- Decision record: `docs/decisions/2026-05-27-v0.200.0-mysql-tidb-ddl-normalized-silent-coverage.md`.

### DDL Coverage Census

| Dialect | Total | Finding | Silent | Unsupported | Parser Error |
|---------|------:|--------:|-------:|:-----------:|:------------:|
| MySQL | 61 | 46 | 0 | 0 | 15 |
| TiDB | 54 | 45 | 0 | 0 | 9 |
| PostgreSQL (consolidated tracked-case) | 285 | 274 | 6 | 0 | 5 |

### Key Design Decisions

- Not new parser support.
- Not full MySQL/TiDB/PostgreSQL DDL support.
- Not dialect parity.
- Not runtime/catalog validation.
- Not a parser-error fix release.

## Previous Milestone: v0.190.0 Cross-Dialect DDL Coverage Census Consolidation

**Goal:** establish a consolidated DDL coverage census across MySQL, TiDB, and PostgreSQL, providing machine-verifiable coverage inventory data to inform future MySQL/TiDB DDL rule prioritization. Does not add new SQL audit rules, parser support, or product behavior.

### Completed Scope

- `make ddl-census-report` prints tracked DDL coverage census for MySQL, TiDB, and PostgreSQL.
- MySQL/TiDB tracked DDL census expanded from 18 to 61/54 forms covering 10 DDL families each.
- PostgreSQL tracked DDL census consolidated from 5 source-specific tests into a single consolidated test.
- Decision record: `docs/decisions/2026-05-26-v0.190.0-cross-dialect-ddl-census-consolidation.md`.

### DDL Coverage Census

| Dialect | Total | Finding | Silent | Unsupported | Parser Error |
|---------|------:|--------:|-------:|:-----------:|:------------:|
| MySQL | 61 | 21 | 25 | 0 | 15 |
| TiDB | 54 | 18 | 27 | 0 | 9 |
| PostgreSQL (consolidated tracked-case) | 285 | 274 | 6 | 0 | 5 |

### Future Direction

v0.190.0 census data was used to prioritize MySQL/TiDB DDL rule implementation. Top candidates promoted to finding_covered in v0.200.0.

### Unchanged Metrics

- PostgreSQL ALTER TABLE rule count: **32** (unchanged).
- Residual census: **66/60/2/0/4/0** (unchanged).
- SQL corpus: **535/535**, **100.0%**, **243 YAML files** (unchanged).

### Key Design Decisions

- No new SQL audit rules, parser support, or product behavior.
- Not full MySQL/TiDB/PostgreSQL DDL support.
- Not dialect parity.
- Not runtime/catalog validation.

## Previous Milestone: v0.180.0 Release Surface Consistency Gates

**Goal:** add a release surface consistency checker to the release gate pipeline, validating that release-domain facts are consistent across all release surfaces without adding new SQL parser support, audit rules, or product behavior.

### Completed Scope

- Release surface consistency checker (`scripts/verify_release_consistency.py`) validates consistency across landing page, release notes EN/ZH, changelog, roadmap, rules reference, and capability matrix.
- `make release-version-surface-gates` runs both the existing shell version surface checker and the new Python release semantic consistency checker.
- `make release-contract-gates` inherits the new checker through dependency.
- New `make release-consistency-test` target runs the consistency checker's test suite.
- Decision record: `docs/decisions/2026-05-25-v0.180.0-release-surface-consistency-gates.md`.

### Unchanged Metrics

- PostgreSQL ALTER TABLE rule count: **32** (unchanged).
- Residual census: **66/60/2/0/4/0** (unchanged).
- SQL corpus: **535/535**, **100.0%**, **243 YAML files** (unchanged).

### Key Design Decisions

- No new SQL parser support, audit rules, or product behavior.
- Not full PostgreSQL ALTER TABLE support.
- Not PostgreSQL 18 parser support.
- Not runtime/live validation.
- Not rewrite duration estimate.
- Not v1.0/stable API contract claim.

## Previous Milestone: v0.170.0 PostgreSQL ALTER TABLE Final Parseable Boundary Rules

**Goal:** add 4 new PostgreSQL-only ALTER TABLE notice rules covering final parseable boundaries (SET EXPRESSION, ADD IDENTITY, ADD EXCLUSION CONSTRAINT, ALL IN TABLESPACE), advancing the PostgreSQL ALTER TABLE residual census to 60 of 66 forms `finding_covered`.

### Completed Scope

- 4 new PostgreSQL-only ALTER TABLE final parseable boundary rules:
  - `ddl.pg.alter.set_expression.notice` — `ALTER TABLE ... ALTER COLUMN ... SET EXPRESSION` sets a generated column expression.
  - `ddl.pg.alter.add_identity.notice` — `ALTER TABLE ... ALTER COLUMN ... ADD GENERATED ... AS IDENTITY` adds identity to an existing column.
  - `ddl.pg.alter.add_exclusion_constraint.notice` — `ALTER TABLE ... ADD CONSTRAINT ... EXCLUDE USING` adds an exclusion constraint.
  - `ddl.pg.alter.move_all_tablespace.notice` — `ALTER TABLE ALL IN TABLESPACE ... SET TABLESPACE ...` moves all tables in a tablespace.
- PostgreSQL ALTER TABLE residual census: `finding_covered` 56 → 60, `unsupported_boundary` 4 → 0, `normalized_silent` 2 (unchanged), `parser_error` 4 (unchanged).
- SQL corpus: 535/535 supported rule-dialect targets (100%), 243 expected YAML files.
- No-leak contract: findings do not emit expression body, sequence options, exclusion operators/predicates, or raw SQL.

### Key Design Decisions

- No full PostgreSQL ALTER TABLE support claim. Final parseable boundary families are covered; remaining 2 forms are `normalized_silent`.
- No PostgreSQL 18 parser support (4 `parser_error` forms remain blocked on `pg_query_go v7`).
- No runtime/live validation.
- No rewrite duration estimate.
- No v1.0/stable API contract claim.

## Previous Milestone: v0.160.0 PostgreSQL ALTER TABLE Constraint Deferrability Rules

**Goal:** add 4 new PostgreSQL-only ALTER TABLE rules covering table relationship operations (INHERIT and OF type), advancing the PostgreSQL ALTER TABLE residual census to 54 of 66 forms `finding_covered`.

### Completed Scope

- 4 new PostgreSQL-only ALTER TABLE table relationship rules:
  - `ddl.pg.alter.add_inherit.notice` — `ALTER TABLE ... INHERIT` adds a parent table relationship.
  - `ddl.pg.alter.drop_inherit.notice` — `ALTER TABLE ... NO INHERIT` removes a parent table relationship.
  - `ddl.pg.alter.add_of_type.notice` — `ALTER TABLE ... OF type_name` binds the table to a typed table type.
  - `ddl.pg.alter.drop_of_type.notice` — `ALTER TABLE ... NOT OF` removes the typed table binding.
- PostgreSQL ALTER TABLE residual census: `finding_covered` 50 → 54, `unsupported_boundary` 11 → 7.
- SQL corpus: 529/529 supported rule-dialect targets (100%), 237 expected YAML files.
- No-leak contract: table relationship findings do not emit parent table names, typed table type names, or live schema validation claims.

### Key Design Decisions

- No full PostgreSQL ALTER TABLE support claim. Selected table relationship families are covered; remaining 7 `unsupported_boundary` forms deferred.
- No live catalog validation.
- No rewrite duration estimate.
- No runtime behavior validation.
- No DCL expansion.
- No v1.0/stable API contract claim.

## Previous Milestone: v0.140.0 PostgreSQL ALTER TABLE Bounded Residual Pack

**Goal:** add 8 new PostgreSQL-only ALTER TABLE rules covering column attribute mutations (5) and cluster/detach-finalize operations (3), advancing the PostgreSQL ALTER TABLE residual census to 50 of 66 forms `finding_covered`.

### Completed Scope

- 8 new PostgreSQL-only ALTER TABLE rules:
  - Column attributes (5): `ddl.pg.alter.set_column_statistics.notice`, `ddl.pg.alter.set_column_options.notice`, `ddl.pg.alter.reset_column_options.notice`, `ddl.pg.alter.set_column_storage.notice`, `ddl.pg.alter.set_column_compression.notice`.
  - Cluster / detach-finalize (3): `ddl.pg.alter.cluster_on.notice`, `ddl.pg.alter.set_without_cluster.notice`, `ddl.pg.alter.detach_partition_finalize.notice`.
- PostgreSQL ALTER TABLE residual census: `finding_covered` 40 → 50, `unsupported_boundary` 21 → 11.
- SQL corpus: 334 policy rule IDs, 525/525 supported rule-dialect targets (100%), 233 expected YAML files.
- No-leak contract: column attribute findings do not emit raw SQL, option names or values, storage parameter names, compression method names, or compression_kind; cluster/detach-finalize findings do not emit partition bounds, catalog index names, or live validation claims.

### Key Design Decisions

- No full PostgreSQL ALTER TABLE support claim. Selected bounded residual families are covered; remaining 11 `unsupported_boundary` forms deferred.
- No live catalog validation.
- No rewrite duration estimate.
- No runtime behavior validation.
- No DCL expansion.
- No v1.0/stable API contract claim.

## Previous Milestone: v0.130.0 PostgreSQL ALTER TABLE Residual Coverage / Semantics Depth

**Goal:** deepen PostgreSQL ALTER TABLE residual coverage with 10 new rules across storage/layout, trigger/rule residual, and reloptions semantic families.

### Completed Scope

- 10 new PostgreSQL ALTER TABLE residual rules:
  - Storage/layout (2): `ddl.pg.alter.set_tablespace.notice`, `ddl.pg.alter.set_access_method.warn`.
  - Trigger/rule residual (6): `ddl.pg.alter.enable_replica_trigger.notice`, `ddl.pg.alter.enable_always_trigger.notice`, `ddl.pg.alter.enable_rule.notice`, `ddl.pg.alter.disable_rule.warn`, `ddl.pg.alter.enable_replica_rule.notice`, `ddl.pg.alter.enable_always_rule.notice`.
  - Reloptions (2): `ddl.pg.alter.set_reloptions.warn`, `ddl.pg.alter.reset_reloptions.notice`.
- PostgreSQL ALTER TABLE residual census: `finding_covered` 29 → 40, `unsupported_boundary` 32 → 21.
- SQL corpus: 326 policy rule IDs, 517/517 supported rule-dialect targets (100%), 225 expected YAML files.
- No-leak contract: storage/layout findings do not emit raw tablespace or access method names; trigger/rule residual findings do not emit trigger function names, trigger body, rule body, or rule query text; reloptions findings do not emit option names or values.

### Key Design Decisions

- No full PostgreSQL ALTER TABLE support claim. Selected residual families are covered; remaining `unsupported_boundary` forms deferred.
- No live catalog validation.
- No rewrite duration estimate.
- No runtime behavior validation.
- No DCL expansion.
- No v1.0/stable API contract claim.

## Previous Milestone: v0.120.0 PostgreSQL Migration-Safety Semantic Metadata

**Goal:** enrich PostgreSQL migration-safety findings with bounded semantic metadata for CREATE INDEX, ADD COLUMN, and ALTER COLUMN TYPE operations, without emitting raw SQL text.

### Completed Scope

- CREATE INDEX findings include bounded index shape metadata: `index_kind`, `access_method`, `column_count`, `included_column_count`, `has_predicate`, `has_expression_keys`, `expression_count`.
- ADD COLUMN findings include default semantics metadata: `not_null`, `has_default`, `default_kind`.
- ALTER COLUMN TYPE findings include USING clause metadata: `has_using`.
- SDK/CLI/HTTP/MCP parity tests cover all metadata across four public surfaces.
- No-leak contract: no predicate SQL, expression SQL, default SQL, or USING SQL text emitted.

### Key Design Decisions

- No full PostgreSQL DDL support claim. Bounded metadata enrichment for selected migration-safety findings.
- No full expression analysis. Presence/count metadata only; SQL text never emitted.
- No function volatility/immutability analysis. `default_kind` classifies AST node shape.
- No live database/catalog validation.
- No DCL/permission expansion.
- No v1.0/stable API contract claim.

## Previous Milestone: v0.110.0 PostgreSQL Deferred Boundary Identity

**Goal:** promote two previously deferred PostgreSQL DDL boundary forms — CREATE TRANSFORM and CREATE ACCESS METHOD — to supported lifecycle findings with bounded identity, completing the long-tail census at 57/57 `finding_covered`.

### Completed Scope

- 2 promoted PostgreSQL DDL boundary forms: CREATE TRANSFORM (bounded identity: `type@language`) and CREATE ACCESS METHOD (bounded identity: `amname`).
- Long-tail census: 57 of 57 forms `finding_covered` (0 `unsupported_boundary`).
- Payload safety: no handler, function, body, query, definition, or options payloads emitted.
- SQL corpus: 316 policy rule IDs, 507/507 supported rule-dialect targets (100%), 215 expected YAML fixtures.
- Public surface coverage: 2 rules x SDK/CLI/HTTP/MCP = 8 assertions.

### Key Design Decisions

- No full PostgreSQL DDL support claim. Selected long-tail object lifecycle families are covered.
- CREATE TRANSFORM and CREATE ACCESS METHOD promoted (v0.110.0): bounded identity (type@language, amname) avoids handler/function payload.
- No DCL/permission expansion.
- No live DDL execution or migration outcome validation.
- No v1.0/stable API contract claim.

## Previous Milestone: v0.90.0 PostgreSQL Metadata-Aware Object Validation

**Goal:** enrich selected PostgreSQL lifecycle rule findings with live object metadata during metadata-aware audit. DeltaScope resolves non-table PostgreSQL objects (types, domains, extensions, sequences, materialized views, schemas, foreign servers, user mappings, publications, comments) through `pg_catalog` queries and projects safe attributes into findings.

### Completed Scope

- Object metadata resolution pipeline: `ObjectSnapshot` with `Status` (`confirmed`, `not_found`, `unavailable`), `Exists`, `Schema`, `Type`, `Name`, and safe `Attributes`.
- `ResolveObject` interface on PostgreSQL metadata provider. MySQL/TiDB returns `unavailable` (no behavior change).
- CLI, HTTP, and MCP adapters forward `ResolveObject` so all three transports surface object metadata.
- Metadata-aware finding enrichment for 10 supported PostgreSQL lifecycle rules (`drop_schema`, `drop_type`, `drop_domain`, `drop_extension`, `drop_sequence`, `drop_materialized_view`, `drop_publication`, `drop_foreign_server`, `drop_user_mapping`, `comment_on`).
- Safe attribute projection: dual blacklist/whitelist ensures only 8 safe keys (`type_kind`, `extension_version`, `enabled`, `server`, `foreign_data_wrapper`, `target_type`, `has_options`, `table`) reach findings.
- Docker-backed E2E test suite: 12 cases covering confirmed/not_found across 10 object types with privacy assertions.
- Schema-qualified name parsing fix for `DROP DOMAIN/TYPE` and `COMMENT ON`/`SECURITY LABEL` with qualified names.

### Key Design Decisions

- No new rule IDs. v0.90.0 enriches existing findings, does not add rules.
- Dual blacklist/whitelist attribute filtering prevents sensitive value leakage.
- MySQL/TiDB object metadata resolution returns `unavailable` — zero behavior change on non-PG dialects.
- No overclaim of full PostgreSQL DDL grammar coverage.
- DeltaScope does not execute migrations.

## Previous Milestone: v0.80.0 Selected PostgreSQL Non-Permission DDL Deep Coverage

**Goal:** extend DeltaScope's PostgreSQL coverage to selected non-permission DDL deep coverage families beyond v0.70.0: composite type attribute operations, extension member mutation, publication/subscription lifecycle, foreign object lifecycle, annotation lifecycle (COMMENT ON, SECURITY LABEL), and event trigger/rewrite rule lifecycle.

### Completed Scope

- Composite type attribute lifecycle (4 rules): `ADD ATTRIBUTE`, `DROP ATTRIBUTE`, `ALTER ATTRIBUTE TYPE`, `RENAME ATTRIBUTE`.
- Extension member lifecycle (2 rules): `ADD TABLE`, `DROP TABLE`.
- Publication/subscription lifecycle (7 rules): `CREATE/ALTER/DROP PUBLICATION`, `CREATE/ALTER/DROP SUBSCRIPTION`, `ALTER SUBSCRIPTION DISABLE`.
- Foreign object lifecycle (12 rules): `CREATE/ALTER/DROP` for `FOREIGN TABLE`, `SERVER`, `USER MAPPING`, `FOREIGN DATA WRAPPER`.
- Annotation lifecycle (4 rules): `COMMENT ON`, `COMMENT ON IS NULL`, `SECURITY LABEL`, `SECURITY LABEL IS NULL`.
- Event trigger/rewrite rule lifecycle (7 rules): `CREATE/ALTER/DROP EVENT TRIGGER` (including `DISABLE`), `CREATE/ALTER/DROP RULE`.
- 36 new PostgreSQL-only DDL lifecycle rules across 6 families.
- PostgreSQL DDL deep coverage census: 38/39 forms `finding_covered`, 1 `parser_error` (`DROP SUBSCRIPTION ... WITH (drop_slot)`).
- SQL corpus: 278 policy rules, 469/469 supported targets, 100% coverage.
- Public surface coverage verified across SDK, CLI, HTTP, MCP for all 36 rules.
- Privacy boundary: corpus expected outputs do not contain connection strings, foreign option values, comment text, security label text, trigger/rule bodies.

### Key Design Decisions

- No overclaim of full PostgreSQL DDL grammar coverage.
- `DROP SUBSCRIPTION ... WITH (drop_slot = true)` remains deferred (parser error).
- DCL/permissions remain out of scope except pre-existing v0.60.0 table-level DCL support.
- DeltaScope does not execute migrations.

## Previous Milestone: v0.70.0 Selected PostgreSQL Non-Permission DDL Lifecycle

**Goal:** extend DeltaScope's PostgreSQL coverage to selected non-permission DDL lifecycle families: RLS/Policy, Trigger, Function/Procedure, Advanced View, and selected ALTER object lifecycle (schema, index, materialized view).

### Completed Scope

- RLS/Policy lifecycle rules: `CREATE POLICY`, `ALTER POLICY`, `DROP POLICY`, `ENABLE/DISABLE/FORCE/NO FORCE ROW LEVEL SECURITY`.
- Trigger lifecycle rules: `CREATE TRIGGER`, `CREATE CONSTRAINT TRIGGER`, `DROP TRIGGER`.
- Function/Procedure lifecycle rules: `CREATE FUNCTION`, `CREATE OR REPLACE FUNCTION`, `CREATE FUNCTION ... SECURITY DEFINER`, `DROP FUNCTION`, `CREATE PROCEDURE`, `DROP PROCEDURE`.
- Advanced View lifecycle rules: `CREATE OR REPLACE VIEW`, `CREATE TEMP VIEW`, `CREATE VIEW ... WITH CHECK OPTION`, `DROP VIEW ... CASCADE`, `ALTER VIEW ... RENAME TO`, `ALTER VIEW ... SET SCHEMA`.
- Selected ALTER object lifecycle rules: `ALTER SCHEMA ... RENAME TO`, `ALTER SCHEMA ... OWNER TO`, `ALTER INDEX ... RENAME TO`, `ALTER INDEX ... SET TABLESPACE`, `ALTER MATERIALIZED VIEW ... RENAME TO`, `ALTER MATERIALIZED VIEW ... SET SCHEMA`.
- 34 new PostgreSQL audit rules across 5 lifecycle families.
- PostgreSQL DDL completion census: 31 selected non-permission DDL forms `finding_covered`.
- SQL corpus: 242 policy rules, 433/433 supported targets, 100% coverage.
- Public surface coverage verified across SDK, CLI, HTTP, and MCP for all new rules.
- Standalone `ALTER INDEX` metadata resolution path via `IndexOwnerResolver` for PostgreSQL metadata-aware audit.

### Key Design Decisions

- `pgObjectLifecycleRule` pattern introduced with `newPGObjectLifecycleRuleWithAction` supporting operation + action + objectType matching for lifecycle rules.
- Census scope is limited to 31 selected non-permission DDL forms; many PG DDL families remain intentionally deferred.
- Standalone `ALTER INDEX` operations resolved through `IndexOwnerResolver` interface without MySQL/TiDB semantic expansion.
- No overclaim of full PostgreSQL DDL grammar coverage.
- DeltaScope does not execute migrations.

## Previous Milestone: v0.64.0 Cross-Dialect DDL Parity

**Goal:** build a cross-dialect DDL coverage census and close the first parity gap with database/schema lifecycle audit rules for MySQL, TiDB, and PostgreSQL.

### Completed Scope

- Cross-dialect DDL coverage census: representative DDL forms classified across MySQL, TiDB, and PostgreSQL.
- MySQL/TiDB database/schema lifecycle normalization: `CREATE DATABASE`, `CREATE DATABASE IF NOT EXISTS`, `CREATE SCHEMA`, `DROP DATABASE`, `DROP DATABASE IF EXISTS`, `DROP SCHEMA` now pass through the audit pipeline instead of passing silently.
- PostgreSQL `CREATE SCHEMA` normalization: `CREATE SCHEMA` and `CREATE SCHEMA IF NOT EXISTS` now produce an explicit notice.
- Three new audit rules: `ddl.database.create.notice` (MySQL/TiDB, notice), `ddl.database.drop.warn` (MySQL/TiDB, warning), `ddl.pg.create_schema.notice` (PostgreSQL, notice).
- Default policy entries for all three new rules.
- SQL corpus: 214 policy rules, 405/405 supported targets, 100% coverage.
- Public surface coverage verified across CLI, HTTP, MCP, and SDK.
- AST characterization tests for database/schema lifecycle forms.

### Key Design Decisions

- Reuse `create_schema` / `drop_schema` operations for MySQL/TiDB database/schema synonyms with `ObjectType = "database"` to distinguish from PostgreSQL `ObjectType = "schema"`.
- Existing PostgreSQL `DROP SCHEMA` rules (`ddl.pg.drop_schema.advisory`, `ddl.pg.drop_schema.cascade.warn`) are unchanged.
- No shared `ddl.schema.drop.*` rule to avoid duplicate findings or policy churn for existing PG users.
- MySQL/TiDB `CREATE DATABASE ... CHARACTER SET`/`COLLATE` facts preserved but not governed.
- PostgreSQL `CREATE SCHEMA AUTHORIZATION` and nested schema creation remain unsupported/deferred.
- DeltaScope does not execute migrations.

## Prior Milestone: v0.63.0 Adoption & Runtime Config Pack

**Goal:** expose metadata connect timeout publicly across CLI, HTTP, and MCP surfaces; add optional runtime configuration for server and MCP operations; and improve adoption documentation — without changing SQL rule behavior, parser coverage, or public SDK contracts.

### Completed Scope

- Runtime config loader (`internal/infrastructure/runtimeconfig`): YAML-based logging and metadata defaults for server and MCP (`-runtime-config` flag).
- Runtime config logging: `level`, `format`, `output`, `file`, and `rotation` (max size, max age, max backups, compress).
- Runtime config metadata: `metadata.connect_timeout` default for server and MCP metadata connections.
- CLI metadata connect timeout: `--metadata-connect-timeout` flag.
- HTTP metadata connect timeout: `connection.connect_timeout` in the JSON request body.
- MCP metadata connect timeout: `connect_timeout` on direct and named connection inputs.
- Timeout precedence: request-level `connect_timeout` > runtime `metadata.connect_timeout` > opener default; empty or `0s` means unset/default.
- MCP stdout logging remains forbidden to protect the stdio protocol.
- Public surface coverage verified across CLI, HTTP, MCP, and SDK.
- Full E2E matrix: CLI/HTTP/MCP x MySQL/TiDB/PostgreSQL.
- SQL corpus 400/400 targets and fixture execution verified.
- Runtime config and metadata timeout examples in adoption docs.
- Adoption docs: SQL audit MCP server and TiDB schema change audit pages.

### Key Design Decisions

- v0.63.0 is an adoption and runtime config release: no new SQL rule IDs, parser features, or public SDK contract changes.
- Runtime config is separate from policy config (`--config`) and currently applies to `deltascope-server` and `deltascope-mcp`, not ordinary CLI logging.
- SDK `deltascope.Request` does NOT have `MetadataConnectTimeout`; SDK callers that pass their own `MetadataProvider` manage connection behavior themselves.
- MySQL, TiDB, and PostgreSQL rule semantics remain unchanged.
- Parser cache remains deferred.
- DeltaScope does not execute migrations and is not a database proxy or runtime query firewall.

## Previous Milestone: v0.62.0 Logging & Maintainability Pack

**Goal:** add structured logging for server and MCP services, log file rotation, metadata connect timeout, and improve code maintainability by splitting large files — without changing rule semantics, parser coverage, or public APIs.

### Completed Scope

- Structured logging foundation with server and MCP logging flags (`-log-output`, `-log-level`, `-log-file`).
- Log file rotation support with configurable max size, max age, max backups, and compress options.
- Internal 5s default connect timeout for metadata-aware MySQL/TiDB and PostgreSQL openers; `OpenDBContext` propagates caller cancellation; `auditmeta.Prepare(ctx)` respects caller context.
- Log file permissions restricted to owner-only (`0750` for directories, `0600` for files).
- `defaults.go` split into 5 files by rule category; `extractor.go` split into 7 files by statement type.
- Parser benchmark coverage for hot paths.
- Context propagation fixes in boundary error wrapping and impact estimation.

### Key Design Decisions

- v0.62.0 is a logging and maintainability release: no new rule IDs, parser features, or public API changes.
- MySQL, TiDB, and PostgreSQL rule semantics remain unchanged.
- Release asset naming and install workflows remain unchanged.

## Previous Milestone: v0.61.0 Quality & Reliability Pack

**Goal:** improve runtime reliability, static-analysis discipline, context propagation, test throughput, and hot-path performance without changing rule semantics, parser coverage, public APIs, or release asset contracts.

### Completed Scope

- Database connection pool lifecycle hardened to prevent connection leaks under metadata-aware audit workloads.
- MCP server panic recovery added across tool handler, server handler, and process-level boundaries.
- golangci-lint v2 integration added with 15 active linters and 903 code-quality issues fixed.
- `context.Context` timeout and cancellation propagation improved across audit layers.
- 1522 tests made parallel where safe to reduce feedback time.
- Hot-path performance optimized with slice preallocation, `strings.Builder` string assembly, and markdown renderer builder reuse.
- Codebase maintainability improved so all files are now under 800 lines.

### Key Design Decisions

- v0.61.0 is a quality release: no new rule IDs, parser features, public API changes, or audit behavior changes.
- MySQL, TiDB, and PostgreSQL rule semantics remain unchanged.
- Release asset naming and install workflows remain unchanged.
- Quality work stays visible in release docs and landing copy, while capability matrices remain versioned by audit coverage milestones.

## Previous Milestone: v0.60.0 PostgreSQL Table Privilege DCL Pack

**Goal:** normalize PostgreSQL table-level privilege DCL narrow support through the audit pipeline, adding four PostgreSQL-only findings for offline migration review while keeping `ALL TABLES IN SCHEMA`, sequence privileges, role membership GRANT/REVOKE, and `ALTER DEFAULT PRIVILEGES` explicitly deferred and not performing live validation.

### Completed Scope

- 8 PostgreSQL table-level privilege DCL forms normalized through the audit pipeline: `GRANT ... ON TABLE` (single/multiple privileges, single/multiple grantees, schema-qualified table), `GRANT ALL PRIVILEGES ON TABLE`, `REVOKE ... ON TABLE` (single/multiple privileges, single/multiple grantees), `REVOKE ... ON TABLE ... CASCADE`.
- Four new PostgreSQL-only rules: `ddl.pg.grant.table_privilege.notice` (notice), `ddl.pg.grant.table_privilege.all.warn` (warning), `ddl.pg.revoke.table_privilege.notice` (notice), `ddl.pg.revoke.table_privilege.cascade.warn` (warning).
- `GRANT ALL PRIVILEGES ON TABLE` intentionally triggers both `ddl.pg.grant.table_privilege.notice` and `ddl.pg.grant.table_privilege.all.warn`. `REVOKE ... ON TABLE ... CASCADE` intentionally triggers both `ddl.pg.revoke.table_privilege.notice` and `ddl.pg.revoke.table_privilege.cascade.warn`.
- Corpus fixtures covering all four new rules.
- Service-level tests through `AuditSQL` for representative table privilege DCL variants.
- Public surface tests across all four surfaces: `pkg/deltascope` Audit, CLI Execute, HTTP handler, and MCP audit_sql tool.
- AST census tests documenting stable parser facts for all table privilege DCL forms.

### Key Design Decisions

- DeltaScope does not perform live validation of any kind for table privileges — no grantee/role existence checks, no table/object existence checks, no grantor permission checks, no effective privilege computation, no role inheritance resolution, no ownership verification, and no RLS/policy evaluation.
- `ALL TABLES IN SCHEMA`, sequence privileges, role membership GRANT/REVOKE, and `ALTER DEFAULT PRIVILEGES` are explicitly unsupported/deferred.
- This is narrow table-level privilege DCL support — not broad governance or admin DCL support.
- No MySQL/TiDB behavior changes.
- No default policy changes beyond the four new PostgreSQL-only rule entries.

## Previous Milestone: v0.59.0 PostgreSQL Extension Lifecycle Pack

**Goal:** normalize PostgreSQL extension lifecycle narrow support through the audit pipeline, adding six PostgreSQL-only findings for offline migration review while keeping extension member mutation explicitly deferred and not performing live validation.

### Completed Scope

- 11 PostgreSQL extension lifecycle forms normalized through the audit pipeline: `CREATE EXTENSION` (including `IF NOT EXISTS`, `WITH SCHEMA`, `WITH VERSION`, `CASCADE`), `ALTER EXTENSION` (`UPDATE`, `UPDATE TO`, `SET SCHEMA`), `DROP EXTENSION` (including `IF EXISTS`, `CASCADE`).
- Six new PostgreSQL-only rules: `ddl.pg.create_extension.notice` (notice), `ddl.pg.create_extension.cascade.warn` (warning), `ddl.pg.alter_extension.update.notice` (notice), `ddl.pg.alter_extension.set_schema.notice` (notice), `ddl.pg.drop_extension.advisory` (warning), `ddl.pg.drop_extension.cascade.warn` (warning).
- `CREATE EXTENSION ... CASCADE` intentionally triggers both `ddl.pg.create_extension.notice` and `ddl.pg.create_extension.cascade.warn`. `DROP EXTENSION ... CASCADE` intentionally triggers both `ddl.pg.drop_extension.advisory` and `ddl.pg.drop_extension.cascade.warn`.
- Corpus fixtures covering all six new rules.
- Service-level tests through `AuditSQL` for representative extension lifecycle variants.
- Public surface tests across all four surfaces: `pkg/deltascope` Audit, CLI Execute, HTTP handler, and MCP audit_sql tool.
- AST census tests documenting stable parser facts for all extension lifecycle forms.

### Key Design Decisions

- DeltaScope does not perform live validation of extension availability, installed packages, version compatibility, or dependency graphs.
- DeltaScope does not model full PostgreSQL extension system semantics.
- This is narrow extension lifecycle support — not broad governance or admin DDL support.
- Extension member mutation (`ALTER EXTENSION ... ADD/DROP TABLE`) remains explicitly deferred as `alter_extension_add_member`, `alter_extension_drop_member`.
- No MySQL/TiDB behavior changes.
- No default policy changes beyond the six new PostgreSQL-only rule entries.

## Previous Milestone: v0.58.0 PostgreSQL Composite Type Lifecycle Pack

**Goal:** normalize PostgreSQL composite type lifecycle narrow support through the audit pipeline, adding three PostgreSQL-only findings for offline migration review while keeping attribute-level operations explicitly deferred and reusing existing DROP TYPE rules.

### Completed Scope

- Five PostgreSQL composite type lifecycle forms normalized through the audit pipeline: `CREATE TYPE ... AS (...)` (including schema-qualified and collation-annotated forms), `ALTER TYPE ... RENAME TO`, `ALTER TYPE ... SET SCHEMA`.
- Three new PostgreSQL-only rules: `ddl.pg.create_type.composite.notice` (notice), `ddl.pg.alter_type.composite_rename.notice` (notice), `ddl.pg.alter_type.composite_set_schema.notice` (notice).
- `DROP TYPE` for composite types reuses existing v0.55.0 rules (`ddl.pg.drop_type.advisory`, `ddl.pg.drop_type.cascade.warn`) — no new composite-specific DROP TYPE rule.
- Corpus fixtures covering all three new rules.
- Service-level tests through `AuditSQL` for representative composite lifecycle variants.
- Public surface tests across all four surfaces: `pkg/deltascope` Audit, CLI Execute, HTTP handler, and MCP audit_sql tool.
- AST census tests documenting stable parser facts for all composite type lifecycle forms.

### Key Design Decisions

- DeltaScope does not perform live dependency validation on composite types.
- DeltaScope does not model full PostgreSQL type system semantics.
- This is narrow composite type lifecycle support — not complete PostgreSQL type system support.
- Collation annotations in composite type attribute definitions are recognized structurally but not interpreted.
- Attribute-level operations (`ADD ATTRIBUTE`, `DROP ATTRIBUTE`, `ALTER ATTRIBUTE ... TYPE`, `RENAME ATTRIBUTE`) remain explicitly deferred as `alter_type_add_attribute`, `alter_type_drop_attribute`, `alter_type_alter_attribute_type`, `alter_type_rename_attribute`.
- No MySQL/TiDB behavior changes.
- No default policy changes beyond the three new PostgreSQL-only rule entries.

## Previous Milestone: v0.57.0 PostgreSQL Domain Lifecycle Pack

**Goal:** normalize PostgreSQL domain lifecycle DDL through the audit pipeline, adding seven PostgreSQL-only findings for offline migration review while keeping `CHECK`/`DEFAULT` expression rendering out of scope and composite types as an explicit unsupported boundary.

### Completed Scope

- 15 PostgreSQL domain lifecycle forms normalized through the audit pipeline: `CREATE DOMAIN`, `ALTER DOMAIN` (SET/DROP DEFAULT, SET/DROP NOT NULL, ADD/DROP/VALIDATE CONSTRAINT, RENAME TO), `DROP DOMAIN`, `DROP DOMAIN IF EXISTS ... CASCADE`.
- Seven new PostgreSQL-only rules: `ddl.pg.create_domain.notice` (notice), `ddl.pg.alter_domain.constraint.notice` (notice), `ddl.pg.alter_domain.default.notice` (notice), `ddl.pg.alter_domain.not_null.notice` (notice), `ddl.pg.alter_domain.rename.notice` (notice), `ddl.pg.drop_domain.advisory` (warning), `ddl.pg.drop_domain.cascade.warn` (warning).
- Corpus fixtures covering all seven new rules.
- Service-level tests through `AuditSQL` for 12 domain lifecycle variants.
- Public surface tests across all four surfaces: `pkg/deltascope` Audit, CLI Execute, HTTP handler, and MCP audit_sql tool.
- AST census tests documenting stable parser facts for all 15 domain lifecycle forms.

### Key Design Decisions

- DeltaScope does not render `CHECK` or `DEFAULT` expression text. Rules emit boolean facts (`has_check`, `has_default`, `not_null`) and constraint names, but never the expression body.
- DeltaScope does not perform live dependency validation on domains.
- `CREATE TYPE ... AS (...)` composite types were added in v0.58.0.
- No MySQL/TiDB behavior changes.
- No default policy changes beyond the seven new PostgreSQL-only rule entries.

## Previous Milestone: v0.56.0 PostgreSQL ALTER TABLE Remaining Grammar Pack

**Goal:** normalize PostgreSQL ALTER TABLE logged-state transitions and capture ALTER COLUMN TYPE USING metadata, adding two PostgreSQL-only findings for logged-state review while keeping SET TABLESPACE as an explicit unsupported boundary.

### Completed Scope

- Two PostgreSQL ALTER TABLE logged-state forms normalized through the audit pipeline: `ALTER TABLE ... SET LOGGED`, `ALTER TABLE ... SET UNLOGGED`.
- Two new PostgreSQL-only rules: `ddl.pg.alter.set_logged.notice` (notice) and `ddl.pg.alter.set_unlogged.notice` (notice).
- ALTER COLUMN TYPE USING metadata: the USING expression is now captured in normalized alter metadata.
- Corpus fixtures covering both new rules.
- Service-level tests through `AuditSQL` for logged-state variants.
- Public surface tests across all four surfaces: `pkg/deltascope` Audit, CLI Execute, HTTP handler, and MCP audit_sql tool.
- AST census tests documenting stable parser facts for logged-state forms.

### Key Design Decisions

- DeltaScope does not verify whether the target table is currently logged or unlogged.
- DeltaScope does not evaluate WAL or replication implications of logged-state transitions.
- This is not full PostgreSQL ALTER TABLE grammar support.
- SET TABLESPACE remains explicit unsupported as `alter_set_tablespace`.
- No MySQL/TiDB behavior changes.
- No default policy changes beyond the two new PostgreSQL-only rule entries.

## Previous Milestone: v0.55.0 PostgreSQL Type Lifecycle Pack

**Goal:** normalize PostgreSQL enum type creation, enum value additions, and type drops through the audit pipeline, adding five PostgreSQL-only findings for migration review while keeping composite types and domains as explicit unsupported boundaries.

### Completed Scope

- Seven PostgreSQL type lifecycle forms normalized through the audit pipeline: `CREATE TYPE ... AS ENUM`, `ALTER TYPE ... ADD VALUE`, `ALTER TYPE ... ADD VALUE IF NOT EXISTS`, `ALTER TYPE ... ADD VALUE ... BEFORE`, `ALTER TYPE ... ADD VALUE ... AFTER`, `DROP TYPE`, `DROP TYPE IF EXISTS ... CASCADE`.
- Five new PostgreSQL-only rules: `ddl.pg.create_type.enum.notice` (notice), `ddl.pg.alter_type.add_value.advisory` (warning), `ddl.pg.alter_type.add_value.position.notice` (notice), `ddl.pg.drop_type.advisory` (warning), `ddl.pg.drop_type.cascade.warn` (warning).
- Corpus fixtures covering all five new rules.
- Service-level tests through `AuditSQL` for type lifecycle variants.
- Public surface tests across all four surfaces: `pkg/deltascope` Audit, CLI Execute, HTTP handler, and MCP audit_sql tool.
- AST census tests documenting stable parser facts for all seven supported forms and two deferred forms.

### Key Design Decisions

- DeltaScope does not inspect live dependent objects.
- DeltaScope does not validate whether enum values are already used by data or application code.
- DeltaScope does not model full PostgreSQL type system semantics.
- This is not full PostgreSQL type lifecycle support.
- Composite types remain explicit unsupported as `create_type_composite`.
- Domains were added in v0.57.0.
- No MySQL/TiDB behavior changes.
- No default policy changes beyond the five new PostgreSQL-only rule entries.

## Previous Milestone: v0.54.0 PostgreSQL ALTER TABLE Residual Coverage Pack

**Goal:** close the remaining high-value PostgreSQL ALTER TABLE residual coverage around trigger-scope operations and replica identity configuration, normalizing eight previously deferred forms and adding three PostgreSQL-only replica identity rules.

### Completed Scope

- Eight PostgreSQL ALTER TABLE residual forms now normalized through the audit pipeline: `ENABLE TRIGGER ALL`, `ENABLE TRIGGER USER`, `DISABLE TRIGGER ALL`, `DISABLE TRIGGER USER`, `REPLICA IDENTITY DEFAULT`, `REPLICA IDENTITY FULL`, `REPLICA IDENTITY NOTHING`, `REPLICA IDENTITY USING INDEX ...`.
- Three new PostgreSQL-only rules: `ddl.pg.alter.replica_identity_full.warn` (warning), `ddl.pg.alter.replica_identity_nothing.warn` (warning), `ddl.pg.alter.replica_identity_using_index.notice` (notice).
- Trigger-scope forms reuse existing `ddl.pg.alter.enable_trigger.notice` and `ddl.pg.alter.disable_trigger.warn` rules.
- `REPLICA IDENTITY DEFAULT` is normalized and intentionally silent.
- Corpus fixtures covering all three new rules.
- Service-level tests through `AuditSQL` for all variants.
- Public surface tests across all four surfaces: `pkg/deltascope` Audit, CLI Execute, HTTP handler, and MCP audit_sql tool.
- AST census tests documenting stable parser facts for all eight residual forms.

### Key Design Decisions

- DeltaScope does not inspect live trigger state or validate trigger definitions or functions.
- DeltaScope does not verify whether `REPLICA IDENTITY USING INDEX` names a valid, unique, or non-partial index.
- This is not full PostgreSQL ALTER TABLE grammar support.
- No MySQL/TiDB behavior changes.
- No default policy changes beyond the three new PostgreSQL-only rule entries.

## Previous Milestone: v0.53.0 PostgreSQL REFRESH MATERIALIZED VIEW Pack

**Goal:** normalize all four `REFRESH MATERIALIZED VIEW` variants through the audit pipeline and add two PostgreSQL-only rules warning on non-concurrent refreshes and surfacing `WITH NO DATA` refreshes.

### Completed Scope

- Two new PostgreSQL-only rules: `ddl.pg.refresh_materialized_view.concurrently.warn` (warning) and `ddl.pg.refresh_materialized_view.no_data.notice` (notice).
- Parser/extractor normalization for all four refresh variants (basic, `CONCURRENTLY`, `WITH DATA`, `WITH NO DATA`).
- Corpus fixtures covering both rules' trigger forms.
- Service-level tests through `AuditSQL` for all four refresh variants.
- Public surface tests across all four surfaces.
- AST census tests documenting stable parser facts for all four refresh variants.

### Key Design Decisions

- This is not live unique-index validation for `CONCURRENTLY`. DeltaScope does not verify whether a unique index exists on the materialized view.
- No query, cost, or dependency analysis is performed on the underlying view query.
- No MySQL/TiDB behavior changes.
- No default policy changes beyond the two new PostgreSQL-only rule entries.

## Previous Milestone: v0.52.0 PostgreSQL ALTER TABLE Unsupported-Action Pack

**Goal:** normalize six previously unsupported PostgreSQL ALTER TABLE actions and add six PostgreSQL-only rules providing actionable findings for schema moves, ownership changes, named trigger toggles, and partition attach/detach operations.

### Completed Scope

- Six new PostgreSQL-only rules: `ddl.pg.alter.set_schema.advisory` (notice), `ddl.pg.alter.owner.advisory` (notice), `ddl.pg.alter.enable_trigger.notice` (notice), `ddl.pg.alter.disable_trigger.warn` (warning), `ddl.pg.alter.attach_partition.advisory` (notice), `ddl.pg.alter.detach_partition.warn` (warning).
- Parser/extractor normalization for all six ALTER TABLE action types.
- Corpus fixtures covering each rule's trigger forms.
- Service-level tests through `AuditSQL` for all six rules.
- Public surface tests across all four surfaces.
- AST census tests documenting stable parser facts for each action type.

### Key Design Decisions

- This is not full PostgreSQL ALTER TABLE grammar support. Remaining ALTER TABLE sub-commands remain explicit boundaries.
- Partition bound semantic analysis is not performed.
- No MySQL/TiDB behavior changes.
- No default policy changes beyond the six new PostgreSQL-only rule entries.

## Previous Milestone: v0.51.0 PostgreSQL ALTER TABLE Coverage Pack

**Goal:** extend PostgreSQL ALTER TABLE audit coverage with three gap-fill rules covering the most common ALTER TABLE safety patterns beyond the existing migration-safety and object lifecycle rule families.

### Completed Scope

- Three new PostgreSQL-only rules: `ddl.pg.alter.drop_column.advisory`, `ddl.pg.alter.validate_constraint.advisory`, `ddl.pg.alter.add_column.nullable.notice`.
- Corpus fixtures for each rule's trigger forms.
- Service-level tests through `AuditSQL` for all three rules.
- Public surface tests across all four surfaces: `pkg/deltascope` Audit, CLI Execute, HTTP handler, and MCP audit_sql tool.

### Key Design Decisions

- This is not full PostgreSQL ALTER TABLE coverage. Remaining ALTER TABLE sub-commands remain explicit boundaries.
- No MySQL/TiDB behavior changes.
- No default policy changes beyond the three new PostgreSQL-only rule entries.

## Previous Milestone: v0.50.0 PostgreSQL Object Lifecycle DDL Pack

**Goal:** normalize PostgreSQL schema, sequence, and materialized view lifecycle DDL through the audit pipeline, and add nine PostgreSQL-only rules covering cascade drops, sequence cycling, and sequence restarts.

### Completed Scope

- PostgreSQL object lifecycle DDL normalization: `CREATE SCHEMA`, `DROP SCHEMA`, `CREATE SEQUENCE`, `ALTER SEQUENCE`, `DROP SEQUENCE`, `CREATE MATERIALIZED VIEW`, `DROP MATERIALIZED VIEW`.
- Nine new PostgreSQL-only rules covering advisory notices for drops and warnings for cascades, sequence cycling, and sequence restarts.
- Service-level tests for all lifecycle operations through `AuditSQL`.
- Corpus fixtures for schema, sequence, and materialized view lifecycle forms.
- Public surface tests across all four surfaces: `pkg/deltascope` Audit, CLI Execute, HTTP handler, and MCP audit_sql tool.

### Key Design Decisions

- `REFRESH MATERIALIZED VIEW` remains unsupported/deferred.
- This is not full PostgreSQL object lifecycle coverage. Remaining unsupported DDL forms (triggers, functions, etc.) remain explicit boundaries.
- No MySQL/TiDB behavior changes.
- No default policy changes beyond the nine new PostgreSQL-only rule entries.

## Previous Milestone: v0.49.0 PostgreSQL Advanced CREATE INDEX Normalization Pack

**Goal:** normalize PostgreSQL partial indexes, expression indexes, INCLUDE covering indexes, and non-btree access methods through the audit pipeline instead of returning unsupported, reducing unsupported-explicit from 22 to 18.

### Completed Scope

- PostgreSQL extractor no longer returns unsupported for partial, expression, INCLUDE, or non-btree `CREATE INDEX` variants.
- `spec.Index` extended with five new fields: `AccessMethod`, `IncludedColumns`, `HasPredicate`, `HasExpressionKeys`, `ExpressionCount`.
- Existing `ddl.pg.create_index.concurrently.require` rule now fires for the newly normalized forms.
- Service-level tests for all five advanced index forms through `AuditSQL`.
- Corpus fixtures for partial, expression, INCLUDE, and GIN index forms.
- Public surface tests across all four surfaces: `pkg/deltascope` Audit, CLI Execute, HTTP handler, and MCP audit_sql tool.

### Key Design Decisions

- No new rule IDs — the existing `ddl.pg.create_index.concurrently.require` rule now covers the newly normalized forms.
- No default policy changes.
- No MySQL/TiDB behavior changes.
- No predicate SQL or expression SQL semantic analysis — DeltaScope preserves coarse presence/count flags only.
- Public response types do not expose full internal `spec.Index` advanced fields yet (future surface extension).
- Remaining 18 unsupported PG DDL forms remain explicit boundaries.

### Census Movement (v0.48 → v0.49)

| Metric | v0.48 | v0.49 |
|--------|-------|-------|
| finding-covered | 31 | 35 |
| unsupported-explicit | 22 | 18 |
| classified DDL | 34 | 38 |
| normalized | 34 | 38 |
| corpus-covered | 19/56 | 23/56 |

## Previous Milestone: v0.48.0 PostgreSQL DDL Coverage Census & Gap Closure Pack

**Goal:** systematically audit 56 representative PostgreSQL DDL forms through the full pipeline, identify coverage gaps, and close them with new PostgreSQL-only rules, an extractor fix, and expanded SQL corpus coverage.

### Completed Scope

- Census characterization: 56 representative PostgreSQL DDL forms audited through the full pipeline (parse → enrich → evaluate → report).
- Census inventory locked: total 56, parseable 56, classified 34, normalized 34, finding-covered 31, normalized-silent-pass 3, unsupported-explicit 22, parser-error 0.
- Four new PostgreSQL-only DDL rules:
  - `ddl.pg.drop_index.advisory` — notice when `DROP INDEX` removes an index.
  - `ddl.pg.alter.add_column.non_null_no_default.warn` — warning when `ALTER TABLE ADD COLUMN` adds `NOT NULL` without `DEFAULT`.
  - `ddl.pg.alter.add_unique_constraint.concurrent_index.advisory` — notice suggesting concurrent index creation for `ALTER TABLE ADD UNIQUE CONSTRAINT`.
  - `ddl.pg.alter.drop_constraint.advisory` — notice when `ALTER TABLE DROP CONSTRAINT` removes a CHECK, UNIQUE, or FOREIGN KEY constraint.
- PostgreSQL extractor fix: `CONSTR_NOTNULL` and `CONSTR_DEFAULT` on `ALTER TABLE ADD COLUMN` now populate column `NotNull` and `Default` fields.
- SQL corpus expanded with new PostgreSQL DDL finding cases covering all four new rules.

### Key Design Decisions

- No new MySQL or TiDB rule IDs, parser features, or policy changes.
- `CREATE INDEX CONCURRENTLY`, `ALTER TABLE VALIDATE CONSTRAINT`, and `ALTER TABLE DROP COLUMN` remain normalized silent pass for this milestone.
- `hasColumnConstraint` is an internal helper, not a public export.
- No public API contract changes.

## Previous Milestone: v0.47.0 Source Location Fidelity Pack

**Goal:** make CI renderers (GitHub Actions, SARIF, GitLab Code Quality) carry the original file path and statement-start line number for each finding, so inline annotations point at the exact SQL statement instead of the first line of the migration file.

### Completed Scope

- Progressive source mapper that scans the original SQL buffer forward, matches each `RawSQL` text, and counts newlines — replacing the previous statement-index fallback.
- `Finding.Location` populated from statement location in the evaluation layer (only when the rule does not already provide a custom location).
- GitHub Actions output emits `file=<path>,line=N,col=N` with correct statement-start line; `file=` omitted when no `--file` path.
- SARIF output includes `artifactLocation.uri`, `startLine`, `startColumn` per result; `artifactLocation` omitted when no `--file` path.
- GitLab Code Quality output carries correct statement-start line number in `location.lines.begin`.
- Dedicated unit tests for the progressive source mapper (multi-line, repeated-statement, blank-line, fallback cases).
- Public API tests for MySQL, TiDB, and PostgreSQL source location fidelity.
- CLI integration tests for TiDB SARIF and TiDB GitLab Code Quality.
- HTTP and MCP integration tests for structured `location` in findings.
- `make release-source-location-smoke` gate validates cross-renderer source location fidelity.

### Key Design Decisions

- No new rule IDs, parser features, or policy changes.
- No domain logic changes beyond statement location propagation.
- No HTTP/MCP transport protocol changes beyond auto-serialized `location`.

## Previous Milestone: v0.46.0 Homebrew Verification Hygiene Pack

**Goal:** clean the Homebrew cask install verification path so successful release workflows no longer show misleading Homebrew tap/cask unavailable error annotations.

### Completed Scope

- Replaced noisy tolerated cleanup (`|| true`) with conditional cleanup probes in the `verify-homebrew-cask-install` release workflow job.
- Added `make release-workflow-hygiene-gates` static gate that enforces conditional cleanup probes, lowercase tap names, and rejects tolerated failure patterns.
- Documented the Homebrew verification hygiene contract in developer testing docs.

### Key Design Decisions

- No SQL audit behavior, parser, rule, or policy changes.
- No formatter, HTTP, MCP, or `pkg/deltascope` production-code changes.
- No release asset naming or npm launcher behavior changes.

## Previous Milestone: v0.45.0 GitLab CI Integration Pack

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

## Future Direction

Areas that may be addressed in future milestones (no dates committed):

- Remaining PostgreSQL ALTER TABLE grammar branches (e.g., `SET TABLESPACE`).
- PostgreSQL governance/admin DDL (`CREATE ROLE`, `GRANT`/`REVOKE` for non-table objects, `ALTER DEFAULT PRIVILEGES`).
- Trigger lifecycle parity across MySQL, TiDB, and PostgreSQL (deferred from v0.64.0).
- Routine/function/procedure lifecycle parity (deferred from v0.64.0).
- Event lifecycle support (deferred from v0.64.0).
- Database privilege/DCL parity (deferred from v0.64.0).
- Parser cache (deferred from v0.63.0).
- Possible SDK public metadata opener (deferred from v0.63.0).
