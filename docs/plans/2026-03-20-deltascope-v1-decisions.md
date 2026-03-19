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

## Open Tracking

- Future decision: whether policy params should remain `map[string]any` or move to a stronger typed value model once real config loading and rule evaluation start to expose pain points.
- Future decision: how much parser-specific location/detail should be preserved inside `StatementSpec` without leaking TiDB AST concerns into the domain.
