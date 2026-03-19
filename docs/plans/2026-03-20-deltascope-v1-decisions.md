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

## Open Tracking

- Future decision: whether policy params should remain `map[string]any` or move to a stronger typed value model once real config loading and rule evaluation start to expose pain points.
- Future decision: how much parser-specific location/detail should be preserved inside `StatementSpec` without leaking TiDB AST concerns into the domain.
