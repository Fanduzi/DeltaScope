# DeltaScope v1 Decision Log

## Purpose

This file records implementation-time decisions, tradeoffs, and issues encountered while building `DeltaScope` v1 without synchronous user feedback.

## Decision 1: v1 delivery shape

- Decision: build `library + CLI` first, postpone HTTP API and MCP to later versions.
- Why: the first target user is AI coding agents running locally; offline consumption matters more than long-running service shape.

## Decision 2: v1 runtime model

- Decision: v1 is offline-only and does not require a live database connection.
- Why: local developer and agent environments often cannot reach production metadata, and deterministic offline auditing is more useful than partial online behavior.

## Decision 3: configuration model

- Decision: use one YAML configuration file, loaded with Viper, grouped by domain and keyed by rule ID.
- Why: this is human-editable, compatible with Cobra/Viper, and scales better than flat all-caps config flags.

## Decision 4: architecture style

- Decision: use a DDD-leaning structure with `interfaces -> application -> domain <- infrastructure`.
- Why: the rewrite should avoid `gAudit`'s checker-centric structure and keep parser/config/CLI concerns out of the core domain.

## Decision 5: audit model

- Decision: rules evaluate a unified domain model rooted at `StatementSpec` instead of parser AST types.
- Why: this keeps rules parser-neutral and creates a better foundation for future HTTP API and MCP reuse.

## Decision 6: result contract

- Decision: findings use levels `blocker`, `warning`, and `notice`; final verdict is `reject`, `review`, or `pass`.
- Why: these names better express governance outcomes than generic runtime severity terms.

## Decision 7: documentation contract

- Decision: adopt `three-level-doc` as an active development gate.
- Why: the repository is new, so adding L1/L2/L3 structure now is cheaper than retrofitting later.

## Decision 8: Task 2 domain follow-up

- Problem: initial Task 2 implementation modeled policy params too narrowly, used ambiguous result naming for global findings, and left statement kind/dialect as raw strings.
- Decision:
  - generalize rule params to `Params map[string]any`
  - rename result-level findings to `GlobalFindings`
  - introduce typed `Kind` and `Dialect`
- Why: these are foundational types that Task 3+ will depend on; it is cheaper to fix them now than after config loading and API wiring are built.

## Decision 9: Viper rule-ID loading

- Problem: rule IDs such as `dml.where.require` use dots, but Viper treats dots as nested-path delimiters by default.
- Decision: create the Viper instance with `viper.NewWithOptions(viper.KeyDelimiter("::"))` so dotted rule IDs remain literal map keys.
- Why: the config design is intentionally rule-ID keyed. Preserving dotted IDs is more important than using Viper's default nested key behavior in this loader.

## Decision 10: kind modeling stays in `statement.go`

- Problem: Task 4 originally mentioned `internal/domain/spec/kind.go`, but Task 2 follow-up had already introduced `Kind` and `Dialect` in `statement.go`.
- Decision: keep `Kind` and `Dialect` in `statement.go` and avoid creating a duplicate `kind.go`.
- Why: creating a second file for the same types would add churn without improving ownership, and Task 4 only needs to consume the existing domain types for classification.

## Decision 11: parser module and Go version

- Problem: the current latest `github.com/pingcap/tidb/pkg/parser` module requires Go 1.25, and its driver import path uses `pkg/parser/test_driver` instead of the older `pkg/types/parser_driver` path seen in older codebases.
- Decision:
  - depend on `github.com/pingcap/tidb/pkg/parser` directly
  - import `github.com/pingcap/tidb/pkg/parser/test_driver`
  - accept the module `go` version moving to `1.25`
- Why: this keeps DeltaScope aligned with the current parser module boundary rather than copying older TiDB integration details from `gAudit`.

## Decision 12: application-owned parse result

- Problem: returning the infrastructure parser result directly from `internal/application/audit` let TiDB AST-bearing types leak through the application contract.
- Decision:
  - keep raw AST nodes inside the application package only
  - expose `ParsedSQL` and `ParsedStatement` as application-owned types
  - keep the AST node as an unexported field for upcoming extraction work
- Why: Task 5 still needs access to raw parser nodes, but the DDD-leaning boundary should not expose infrastructure result types or AST symbols outside the application flow.

## Decision 13: extraction stays in `internal/application/audit`

- Problem: the original Task 5 plan listed `internal/infrastructure/parser/tidb/extractor.go`, but Task 4 had already moved AST-bearing parsed statements behind an application-owned contract.
- Decision:
  - implement extraction in `internal/application/audit`
  - keep infrastructure limited to parsing only
  - avoid a second extractor layer in `internal/infrastructure/parser/tidb`
- Why: extraction consumes hidden AST carried by application-owned parsed statements and produces domain `Statement` values. Putting it in infrastructure would duplicate boundary work and weaken the application/domain seam.

## Decision 14: first-pass extraction keeps DDL constraints and DML join shape separate

- Problem: a naive first-pass extractor mislabeled primary keys and other constraints as generic indexes and could not distinguish "no join" from "join without ON".
- Decision:
  - model `PrimaryKey` separately from `Indexes`
  - preserve other non-index constraints in `DDL.Constraints`
  - add `HasJoin` alongside `HasJoinOn`
  - allow unknown-but-parseable statements to flow through extraction without hard failure
- Why: upcoming DDL/DML rules need these distinctions, and the extraction layer should not hard-code policy decisions by rejecting parseable statements too early.

## Decision 15: registry enforces rule IDs

- Problem: rule IDs are central to config and output, but a naive registry can treat them as advisory and allow duplicate or mismatched identifiers to slip through.
- Decision:
  - reject empty rule IDs at registration time
  - reject duplicate statement/global rule IDs at registration time
  - stamp empty finding rule IDs from the registered rule ID
  - reject findings that claim a different rule ID than the registered rule
  - keep report-flow integration coverage in `internal/application/audit`, not in domain/rule tests
- Why: Task 7/8 will add many real rules, and rule ID correctness must be enforced by the engine boundary rather than by discipline alone.

## Decision 16: first DDL rule batch stays focused on create-table structure

- Problem: Task 7 could sprawl into many shallow checks, especially around audit columns and alter restrictions that need richer extracted metadata than v1 currently exposes.
- Decision:
  - implement the first DDL batch only for `CREATE TABLE` statements
  - ship four high-signal rules first: table comment required, table name max length, primary key required, and primary key max column count
  - keep audit-column and alter restrictions for later batches after the extractor captures enough shape safely
- Why: this proves the rule architecture with meaningful coverage, stays aligned with `gAudit`'s core value, and avoids writing brittle rules against metadata the domain model does not yet expose.

## Decision 17: DML rules need explicit operation semantics

- Problem: the original `spec.DML` shape exposed flags like `HasWhere` and `InsertRows`, but it did not explicitly identify whether a statement was an `INSERT`, `UPDATE`, or `DELETE`.
- Decision:
  - add `spec.DMLOperation`
  - store the operation on every extracted DML statement
  - use operation-aware applicability checks in DML rules instead of inferring behavior from incidental fields
- Why: Tier-1 DML rules differ sharply between mutation statements and insert-family statements. Encoding the operation in the domain model is cleaner and safer than relying on heuristics such as `InsertRows > 0`.

## Decision 18: rename insert-select metadata before the public API exists

- Problem: the earlier `spec.DML.IsSelectInto` field name was misleading because it represented `INSERT ... SELECT`, not `SELECT ... INTO`.
- Decision:
  - rename the field to `IsInsertSelect`
  - add `HasOnDuplicate`
  - keep subquery detection statement-oriented while reserving `IsInsertSelect` for the dedicated insert-select rule
- Why: this is still an internal domain model, so fixing the naming now avoids leaking a confusing term into the eventual public API and output contracts.

## Decision 19: overnight flow should not block on asynchronous reviewer turnaround

- Problem: the overnight implementation run still follows the subagent-driven review model, but reviewer turnaround is occasionally slower than the coding loop and should not stall independent next tasks for hours.
- Decision:
  - keep formal reviewer checkpoints in place
  - allow the controller to continue into the next independent task after local verification when the previous task has no known blocking defect
  - record that handoff explicitly in this log and reconcile any later reviewer findings before merge
- Why: the user asked for unattended overnight progress. Waiting idle on reviewer latency would waste the available development window without improving code quality.

## Decision 20: the public package owns its result contract

- Problem: exposing `internal/domain/*` result types directly from `pkg/deltascope` would couple external consumers to internal domain refactors and make the public API less stable than it appears.
- Decision:
  - keep `pkg/deltascope` request/result types separate from the internal domain packages
  - map internal report and finding types into public equivalents at the package boundary
- Why: v1 is library-first. A small, stable public contract is more valuable than saving a thin mapping layer.

## Decision 21: omitted public dialect defaults to MySQL

- Problem: most first-pass consumers of the library and future CLI will target MySQL syntax, and forcing an explicit dialect on every inline audit call adds friction without adding much safety.
- Decision:
  - treat an empty public dialect as MySQL
  - still reject unknown non-empty dialect values
- Why: this keeps the happy path short for common local usage while preserving strict validation when callers do set a dialect explicitly.

## Decision 22: Task 7 acceptance is by completed DDL batch, not by exhausting all planned DDL concerns

- Problem: the original Task 7 prompt listed audit columns, column constraints, index constraints, and alter restrictions in the same batch, but the current extracted DDL model only supports a safe first slice of that space without brittle rule logic.
- Decision:
  - treat the existing `CREATE TABLE`-focused DDL rule set as the completed first Task 7 batch
  - keep the remaining DDL concerns as explicit follow-up rule batches instead of forcing shallow implementations into the same commit
  - preserve this decision in the log so later reviewers and morning handoff can distinguish intentional batching from silent omission
- Why: v1 still needs broader DDL coverage, but squeezing every planned concern into one early batch would either overfit the current extractor or lower rule quality. Recording the split makes the tradeoff explicit and keeps overnight development moving.

## Decision 23: v1 public API stays request-driven and config-path based

- Problem: Task 9 needs a stable public library surface, but exposing internal policy/domain types would leak implementation details into the package that CLI, HTTP API, and MCP should all sit above.
- Decision:
  - expose a single public entrypoint `Audit(ctx, request)`
  - keep the public request shape minimal: SQL text, dialect, and optional config path
  - keep policy loading inside the application service rather than accepting internal policy structs from callers
- Why: this keeps the public surface narrow and stable while still covering the concrete v1 need for default-policy audits plus file-based overrides.

## Decision 24: renderers consume domain report results, not public package results

- Problem: Task 10 needs Markdown and JSON renderers, but wiring infrastructure to the public `pkg/deltascope` types would invert the intended dependency direction and couple renderers to the library wrapper.
- Decision:
  - implement renderers against `internal/domain/report.Result`
  - keep the public package as a conversion layer above the application service
  - let CLI and future adapters choose whether to call the internal service directly or the public package, but keep renderer dependencies pointed inward
- Why: output formatting belongs in infrastructure, but it should still consume the core result contract rather than a higher-level wrapper. This preserves the DDD-leaning dependency direction while keeping the public API free to evolve as a thin facade.

## Decision 25: public finding levels are typed, not free-form strings

- Problem: Task 9 originally exposed finding severities as raw strings in `pkg/deltascope`, which weakened the public contract even though the domain already had a closed severity set.
- Decision:
  - expose a public `Level` type in `pkg/deltascope`
  - map internal rule levels into that type at the package boundary
- Why: this keeps the library API more explicit and stable without leaking the internal domain package itself.

## Decision 26: `config init` renders YAML from the default policy model

- Problem: Task 11 needs `deltascope config init` to emit a usable YAML template, but the CLI package cannot reliably embed or read `configs/deltascope.example.yaml` by relative filesystem path at runtime.
- Decision:
  - generate the template from `internal/domain/policy.Default()`
  - render the rules in deterministic sorted order
- Why: this keeps the command self-contained, avoids runtime path assumptions, and guarantees the emitted template stays aligned with the real default rule set.

## Decision 27: audit threshold failures return exit code 1 without extra stderr noise

- Problem: in CLI usage, a non-zero exit caused by findings reaching `--fail-on` is a normal audit outcome, not a tool/runtime failure. Emitting an extra stderr error line would make scripts and agents harder to integrate cleanly.
- Decision:
  - keep the rendered audit result on stdout
  - return exit code `1` when the configured finding threshold is reached
  - avoid printing an additional stderr error for that case
- Why: this preserves the agreed exit-code contract while keeping successful audit output machine-friendly for automation.

## Decision 28: unreadable or invalid CLI config files are user errors

- Problem: the final review found that bad `--config` input was being reported as an internal/runtime failure even though the user supplied the bad path or malformed file.
- Decision:
  - classify Viper config lookup and parse failures as CLI user errors
  - keep those failures on stderr with exit code `2`
- Why: this matches the documented exit-code contract and keeps automation from misclassifying bad user input as a tool failure.

## Decision 29: `--quiet` switches Markdown output to a flat finding list

- Problem: the first CLI cut defined `--quiet` but left it behaviorally inert, which made the flag misleading.
- Decision:
  - keep JSON output unchanged under `--quiet`
  - make Markdown quiet mode emit a flat per-finding list without the full report wrapper
  - emit `pass` when the audit has no findings
- Why: this gives the flag a minimal but meaningful machine-friendly behavior without adding a second full renderer stack.

## Decision 30: second DDL expansion starts with column governance, not alter restrictions

- Problem: the next DDL milestone needs to close more of the gap with `gAudit`, but the current extracted model still lacks safe, detailed alter/index metadata for a strong second batch.
- Decision:
  - prioritize column-governance rules first
  - expand the DDL column spec with offline facts such as length, nullability, defaults, and current-timestamp semantics
  - defer alter restrictions and deeper index rules to a later batch after the extracted model is richer
- Why: column rules add high-value offline coverage immediately and can be implemented safely against the existing create-table shape without inventing brittle metadata.

## Decision 31: audit-column detection stays pattern-based and name-agnostic

- Problem: `gAudit`-style audit requirements care about created/updated timestamp behavior, but hard-coding exact column names would make the rule less portable across schemas.
- Decision:
  - treat audit columns as semantic patterns, not fixed names
  - require one time-like column with `DEFAULT CURRENT_TIMESTAMP`
  - require one time-like column with both `DEFAULT CURRENT_TIMESTAMP` and `ON UPDATE CURRENT_TIMESTAMP`
- Why: this preserves the governance intent while keeping the rule useful for teams that use different audit-column names.

## Decision 32: create-table index rules need typed index metadata

- Problem: the next DDL gap after column governance is index policy, but the domain model originally preserved only index names and columns.
- Decision:
  - add `spec.IndexKind`
  - classify extracted indexes as `primary`, `secondary`, `unique`, or `fulltext`
  - keep rules consuming only domain `Index` values, not AST constraint types
- Why: prefix and duplicate-index checks need stable semantic classification, and this is the smallest model expansion that unlocks them cleanly.

## Decision 33: exact duplicate-index detection is a safe first step, not full redundancy analysis

## Decision 34: metadata-aware audit stays on one engine path

- Problem: deeper audit coverage now needs live instance facts and table snapshots, but introducing a separate "online mode" pipeline would split rule behavior and make CLI/HTTP harder to reason about.
- Decision:
  - keep one audit engine
  - let metadata-aware mode enrich `spec.Statement.Metadata` with schema, instance facts, and target-table snapshots
  - let rules opt in to those facts when present and skip honestly when they are absent
- Why: this preserves the offline-first contract while still enabling stronger checks without forking the architecture.

## Decision 35: object-scope governance uses explicit denylist params

- Problem: the acceptance matrix still had DB/table blocklist gaps for both DDL and DML.
- Decision:
  - add `ddl.table.denylist.forbid` and `dml.table.denylist.forbid`
  - support `schemas`, `tables`, and `qualified_tables` params
  - keep the shipped defaults empty so the rules are available but inert until configured
- Why: this closes the protected-table governance gap without inventing a second policy subsystem.

## Decision 36: metadata-backed alter option compatibility should be explicit and narrow

- Problem: alter-column compatibility existed, but `ALTER TABLE ...` option changes still had no metadata-backed comparison against the current schema.
- Decision:
  - add `ddl.alter.table_option.compatibility.require`
  - compare explicit `engine`, `charset`, `collation`, `row_format`, and `auto_increment` option changes against the current snapshot only
- Why: this covers a real audit gap while staying honest about what the snapshot can actually prove.

## Decision 37: alter-added redundancy should reuse create-table logic

- Problem: added indexes in `ALTER TABLE` already had width/duplicate checks, but deeper left-prefix and unique-overlap redundancy still lagged behind create-table.
- Decision:
  - reuse the existing create-table redundant-index rules
  - project `snapshot.Indexes + alter-added indexes` into one temporary lifecycle view before evaluation
- Why: this keeps redundancy semantics consistent across create-table and alter-table without writing a second algorithm.

## Decision 38: sizing checks are rough preflight guards, not exact storage simulators

- Problem: the remaining matrix gap required row-size and index-size checks using instance facts, but exact engine/runtime simulation would be disproportionate and fragile.
- Decision:
  - add metadata-backed rough guards for row size and index key length
  - require instance facts for charset/default-row-format/large-prefix context
  - document them as conservative preflight checks rather than exact execution predictions
- Why: this closes the baseline coverage gap honestly and gives users actionable signals without pretending to solve full storage-engine analysis.

- Problem: broader redundant-index analysis is valuable, but the current offline model can only judge exact duplicate signatures safely without inventing optimizer or live-schema semantics.
- Decision:
  - ship exact duplicate-index detection first for create-table rules
  - reuse that same exact-signature logic for alter-added indexes
  - keep broader left-prefix/redundancy analysis as a later milestone
- Why: exact duplicates are high-signal and honest with current parser-neutral facts; broader redundancy needs richer semantics and more careful false-positive control.

## Decision 34: source-aware alter facts must stay explicit, not inferred

- Problem: `MODIFY COLUMN` and `CHANGE COLUMN` include a full target definition, but that does not prove which semantics were explicitly changed by the statement.
- Decision:
  - keep `AlterColumn.Change` limited to statement-local explicit touches for nullability, default, and auto-increment
  - do not label target type or unsigned shape as explicit change facts
  - let downstream rules inspect target `Definition` separately when they need target-side policy checks
- Why: this keeps the source-aware alter model honest and prevents downstream policy from overclaiming source-to-target truth that offline extraction does not have.

## Decision 35: target-side alter rules stay target-side

- Problem: early alter rule naming drifted toward "compatibility" language even when the implementation only judged the target type family.
- Decision:
  - keep target-type-family rules explicitly named as allowlists
  - use explicit-change forbid rules only for semantics the statement clearly spells out
  - postpone true source-to-target compatibility rules until the model can support them honestly
- Why: stable rule IDs are part of the product surface; they should describe real behavior, not hoped-for future semantics.

## Decision 36: alter-added index lifecycle checks should wrap create-table index rules

- Problem: Milestone 3 needed more alter-index governance, but copying create-table index rule bodies into alter-specific rules would create drift immediately.
- Decision:
  - project alter-added index payloads into temporary parser-neutral index lists
  - reuse existing create-table prefix, width, and exact-duplicate index rule bodies through wrappers
  - only register the alter-added width/duplicate rules when their policies are explicitly enabled
- Why: this keeps alter-index governance consistent with create-table behavior, avoids duplicate rule logic, and stays honest about what is or is not in the default shipped policy.

- Problem: `gAudit` has broader redundant-index concerns, but complete redundancy analysis needs expression, prefix-length, and left-prefix semantics that DeltaScope does not extract yet.
- Decision:
  - implement only exact duplicate detection for now
  - compare index kind plus ordered indexed columns
  - defer broader redundant-index analysis to a later batch
- Why: exact duplicates are high-signal and safe to detect offline, while broader redundancy logic would be easy to overclaim with the current model.

## Decision 37: alter restrictions land before richer alter modeling

- Problem: `gAudit` covers several alter-related governance switches, but DeltaScope's current alter model only exposes normalized actions plus a single related name.
- Decision:
  - ship action-level alter forbid rules now
  - keep them coarse and policy-driven
  - defer richer type/existence analysis until the alter model grows beyond `Action + Name`
- Why: this captures meaningful offline governance immediately without pretending the current model can safely answer deeper alter questions.

## Decision 38: modify-column stays allowed by default, but rename/change do not

- Problem: blocking every alter action by default would make DeltaScope too noisy for common iterative schema work, but leaving rename-style operations open would miss several high-risk patterns that `gAudit` already guards.
- Decision:
  - default-forbid `drop_primary_key`, `rename_table`, `rename_column`, and `change_column`
  - default-allow `drop_column`, `drop_index`, and `modify_column` while still making them policy-addressable
- Why: this keeps the default policy strict on risky structural rewrites without turning all alter workflows into immediate blockers.

## Decision 39: create-table option rules justify a few shape booleans in `spec.DDL`

- Problem: several `gAudit` table-level rules depend on whether a create-table statement uses `LIKE`, `AS SELECT`, or partitioning, but the domain model originally had no way to express those shapes.
- Decision:
  - add `HasReferTable`, `HasSelect`, and `HasPartition` to `spec.DDL`
  - keep them specific to create-table shape rather than introducing a larger object-kind hierarchy
- Why: these booleans unlock several high-value offline rules with minimal model growth and keep parser details out of the rule layer.

## Decision 40: table engine/charset rules stay allowlist-based in v1

- Problem: `gAudit` carries richer charset recommendation semantics, but DeltaScope's current offline model only preserves the explicit option values, not recommendation metadata.
- Decision:
  - implement engine and charset rules as strict allowlists
  - require explicit values to be present and belong to the configured list
  - defer recommendation-style guidance to a later batch if needed
- Why: allowlists are simple, explicit, and safe for offline enforcement. They cover the most important governance behavior now without inventing premature policy complexity.

## Decision 41: primary-key semantic rules target the single-column convention first

- Problem: teams often expect more than "has a primary key"; they expect a single bigint unsigned auto-increment primary key. But composite keys complicate those semantics.
- Decision:
  - add semantic PK rules for bigint, unsigned, auto-increment, and not-null
  - apply bigint/unsigned/auto-increment rules only when the normalized primary key has exactly one column
  - keep `not_null` checking valid for every primary-key column
- Why: this captures the dominant convention cleanly without inventing misleading semantics for composite keys.

## Decision 42: richer alter modeling keeps one canonical subject name per alter record

- Problem: early rich-alter drafts let `Alter.Name` compete with rename/change payload fields, which would force downstream rules to guess which name is authoritative.
- Decision:
  - keep `Alter.Name` as the canonical subject identifier for every normalized alter record
  - use `AlterColumn.OldName` / `AlterIndex.OldName` only for rename-or-change history
  - carry target column/index shape in `Definition` instead of duplicating create-table field sets
- Why: this keeps the domain contract lean, parser-neutral, and predictable for later rule matching.

## Decision 43: multi-column alter specs normalize into one alter record per semantic target

- Problem: TiDB AST can encode one `ALTER TABLE ... ADD (...)` spec that adds multiple columns, but the rule layer expects one `spec.Alter` per semantic action target.
- Decision:
  - fan out multi-column add specs during extraction
  - emit one normalized `spec.Alter` per added column, each with canonical `Name` and `AlterColumn.Definition`
  - keep non-index `ADD CONSTRAINT` payloads out of `AlterIndex`
- Why: this avoids silent data loss, keeps the rule surface uniform, and prevents application-layer AST quirks from leaking into the domain contract.

## Decision 44: alter target-type rules must describe allowlists, not compatibility

- Problem: the first semantic alter rule names used `compatible.require`, but the implementation only checked whether the extracted target column type fell into a conservative allowed family set.
- Decision:
  - rename those rules to `ddl.alter.modify_column.target_type_family.allowlist` and `ddl.alter.change_column.target_type_family.allowlist`
  - keep their behavior explicitly target-side and offline-conservative
  - document that `ddl.alter.change_column.forbid` remains the stricter default gate unless a team intentionally relaxes it
- Why: honest rule IDs and docs matter more than aspirational naming. The current model does not prove source-to-target compatibility, so the exported surface must not imply that it does.

## Decision 45: alter-added index rules should reuse create-table index governance by projection

- Problem: Milestone 2 needs offline governance for indexes introduced by `ALTER TABLE ... ADD CONSTRAINT`, but copying create-table prefix rule bodies into alter-specific rules would create drift.
- Decision:
  - project alter-added index payloads into a temporary parser-neutral `DDL.Indexes` list
  - reuse the existing create-table index prefix rule constructor and behavior
  - keep Task 5 scoped to alter-added unique/secondary/fulltext prefix checks only
- Why: projection keeps logic reuse clean, avoids AST leakage, and narrows the new alter surface to behavior the current domain model can support honestly.

## Decision 46: source-aware alter facts must only encode what the statement explicitly proves

- Problem: early Milestone 3 drafts tried to mark `TouchesType` and `TouchesUnsigned` on every `MODIFY COLUMN` / `CHANGE COLUMN`, but those syntaxes always carry a full target definition even when the actual intended change may only be nullability or default-related.
- Decision:
  - keep `AlterColumn.Change` for explicit statement-local touches only
  - remove overclaiming flags that would pretend the statement proved more than it actually does
  - preserve target type and unsigned shape on `AlterColumn.Definition`, but do not label them as explicit touched facts
- Why: downstream rules must be able to trust the semantics of every flag. When the model cannot honestly prove a change relation, it should expose target shape only and leave the comparison decision to a later, more explicit layer.

## Decision 47: rename intent is inferred from names, not a second change flag

- Problem: an early Milestone 3 Task 1 draft added a `Renames` flag under `AlterColumnChange`, even though rename intent was already derivable from `OldName` plus `Definition.Name`.
- Decision:
  - remove the duplicate rename flag
  - keep rename inference as a derived fact from the existing name fields
- Why: one source of truth is enough. Duplicating rename intent inside the domain model would force later rules to choose which representation to trust.

## Decision 48: Milestone 4 pins breadth-first create-table rule IDs before behavior

- Problem: the remaining create-table work is breadth-oriented, and later tasks need stable rule IDs before implementation starts so config, docs, and tests do not churn mid-milestone.
- Decision:
  - pin Milestone 4 rule IDs up front for four create-table families:
    - identifier and keyword governance
    - wider type-family plus charset/collation governance
    - deeper redundant-index analysis
    - remaining create-table object-shape coverage
  - keep these IDs in `internal/domain/rule/ddl/common.go` even before their rule bodies exist
  - document them explicitly as planned Milestone 4 surface, not already-shipped behavior
- Why: create-table is now mostly a coverage-completion problem, so naming stability matters more than squeezing out one more pre-implementation abstraction.

## Decision 49: remaining create-table naming stays literal and family-first

- Problem: Milestone 4 will add many breadth rules quickly, and vague names would make it harder to tell whether a rule is a hard forbid, a pattern requirement, an allowlist, or a redundancy heuristic.
- Decision:
  - keep identifier legality rules under `*.name.pattern.require`
  - keep reserved-word governance under `*.name.keyword.forbid`
  - keep type-family restrictions literal, for example `ddl.column.blob_text.forbid`, `ddl.column.json.forbid`, and `ddl.column.timestamp.forbid`
  - keep charset/collation rules literal about whether they are allowlists or pair-coherence checks, for example `ddl.column.charset.allowlist` and `ddl.column.charset_collation.match.require`
  - keep create-table-only object-shape additions literal, for example `ddl.table.row_format.allowlist` and `ddl.table.auto_increment.init_value.require`
  - keep deeper redundant-index rules explicit about the heuristic they apply, for example `ddl.index.redundant_left_prefix.forbid`
- Why: the existing rule surface already uses family-first names such as `*.allowlist`, `*.forbid`, and `*.max_length`. Extending that style is clearer than introducing more abstract policy names now.

## Decision 50: unnamed secondary indexes must stay unnamed through extraction

- Problem: MySQL/TiDB allow `KEY (col)` syntax without an explicit secondary-index name, but an early extractor normalized unnamed indexes to synthetic names such as `key`, which hid a real governance concern.
- Decision:
  - keep unnamed non-primary indexes as empty names in the extracted domain model
  - reserve `"primary"` only for the primary-key synthetic identifier
- Why: identifier and keyword governance should evaluate what the statement actually declared, not a synthetic placeholder invented by extraction.

## Decision 51: column collation must be extracted from column options, not only field-type metadata

- Problem: TiDB parser preserves explicit column `COLLATE` clauses as `ColumnOptionCollate`, while `FieldType.GetCollate()` stayed empty in the relevant `CREATE TABLE` shapes.
- Decision:
  - keep using `FieldType` for explicit column charset extraction
  - additionally read `ColumnOptionCollate` during extraction for explicit column collation
- Why: without this split extraction path, DeltaScope would silently miss explicit collation overrides and any downstream charset/collation governance would be incomplete.

## Decision 52: row-format allowlist rules only apply when row format is explicitly set

- Problem: using the generic table-option allowlist behavior for `ROW_FORMAT` made every create-table statement without an explicit row-format clause look invalid, which is stricter than the intended offline check.
- Decision:
  - add `require_explicit` support to table-option allowlist rules
  - keep engine and charset rules requiring explicit values by default
  - configure `ddl.table.row_format.allowlist` with `require_explicit: false`
- Why: `ROW_FORMAT` governance is about restricting explicit row-format choices, not forcing every statement to spell out a row format when the engine default is acceptable.

## Decision 53: the HTTP adapter should reuse the public audit contract directly

- Problem: the service milestone needs JSON request/response handling, but mapping HTTP responses from deeper internal types would create a second external contract beside `pkg/deltascope`.
- Decision:
  - build the HTTP adapter on top of `pkg/deltascope.Audit`
  - return the public `pkg/deltascope.Result` JSON shape directly from `POST /v1/audit`
- Why: the library contract is already the intended stable external surface. Reusing it keeps CLI, library, and HTTP output aligned and reduces adapter-specific drift.

## Decision 54: service config hot-reload is implemented as reload-on-request

- Problem: Milestone 5 needs config-backed long-running service behavior, but the existing audit flow already reloads policy from disk for every invocation when a config path is supplied.
- Decision:
  - validate the configured policy path once at server startup
  - keep each HTTP audit request calling the same config-loading path used by the CLI/library
  - document that file updates take effect on the next request without a watcher-specific in-memory cache
- Why: this preserves one policy-loading path, avoids another long-running config subsystem, and still delivers immediate config reload behavior for a small service.

## Decision 55: the capability matrix is the acceptance source of truth for Audit Completion

- Problem: the next milestone is about audit completeness, which is too easy to judge by intuition instead of by a stable checklist.
- Decision:
  - maintain a dedicated audit capability matrix document
  - mark each important capability as covered, enhanced, gap, or deferred
  - drive follow-up rule work from matrix gaps instead of ad-hoc intuition
- Why: this keeps the milestone measurable and makes future claims about audit completeness auditable.

## Decision 56: metadata-aware access stays on `deltascope audit`, not a second online command

- Problem: CLI completion needed live metadata access, but splitting that into a second top-level online command would fork help, examples, errors, and long-term adapter behavior.
- Decision:
  - keep one `deltascope audit` command
  - enter metadata-aware mode only when connection flags are supplied
  - auto-detect dialect online, infer schema when safe, and fail honestly on ambiguity
- Why: this keeps one coherent audit UX and matches the single-engine architecture used by the application layer.

## Decision 57: shipped rule catalog entries are generated from default-policy rule IDs plus explanation templates

- Problem: the CLI needed `rules list/show/search`, but hand-maintaining a second manually enumerated shipped-rule list would drift from the real default policy surface quickly.
- Decision:
  - derive the catalog entry set from `policy.Default().Rules`
  - attach explanation-oriented metadata such as summaries, examples, config snippets, and remediation hints through catalog templates
  - keep rule execution and rule explanation linked only by `rule_id`
- Why: this keeps the catalog complete for the shipped surface while avoiding a second fragile source of truth for which rules actually ship.

## Decision 58: metadata-aware CLI live smoke stays in Docker-backed e2e, not `go test ./...`

- Problem: the metadata-aware CLI path was feature-complete, but there was still real risk around live MySQL/TiDB connectivity, dialect auto-detect, schema inference, and metadata-backed findings that unit tests and fake providers could not fully retire.
- Decision:
  - add a Docker Compose-backed shell e2e layer that exercises only the shipped CLI against real MySQL and TiDB containers
  - keep that layer behind explicit `make test-e2e-cli*` entrypoints instead of folding it into `go test ./...`
  - use deterministic fixture schemas to prove unique-schema inference, ambiguity failures, qualified-schema SQL, existence checks, and one instance-fact-backed sizing path on both engines
- Why: this gives the project credible live proof for the metadata-aware CLI surface without slowing the default Go test loop or coupling public behavior checks to internal test doubles.

## Open Tracking

- Future decision: whether policy params should remain `map[string]any` or move to a stronger typed value model once real config loading and rule evaluation start to expose pain points.
- Future decision: how much parser-specific location/detail should be preserved inside `StatementSpec` without leaking TiDB AST concerns into the domain.
