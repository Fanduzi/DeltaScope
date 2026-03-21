# DeltaScope Audit Completion Design

## Goal

Bring DeltaScope to a capability-complete audit milestone: keep the current offline-first flow, add optional metadata-aware enhancement, and close the remaining important audit gaps with a formal capability matrix as the acceptance source of truth.

## Success Criteria

- DeltaScope still runs fully offline by default.
- When connection details are supplied, DeltaScope can enrich review decisions with instance facts and object metadata.
- Remaining important DDL and DML audit gaps are either:
  - implemented,
  - replaced by a stronger equivalent,
  - or explicitly deferred with a documented reason.
- The repository contains a capability matrix that marks every important audit ability as covered, replaced, deferred, or intentionally out of scope.
- Public docs are upgraded to match the maturity of the product:
  - `README.md`
  - `README_ZH.md`
  - `CHANGELOG.md`
  - `SECURITY.md`

## Why This Milestone Exists

The project already has a stable library API, CLI, HTTP service, stronger create-table coverage, and a growing alter-table semantic model. The remaining gap is not basic plumbing; it is audit completeness. This milestone turns the project from “strong in many areas” into “provably complete against a defined capability matrix.”

## Recommended Direction

Use one audit engine with optional metadata providers instead of creating separate offline and online code paths.

### Core principle

Online enhancement adds facts, not a second workflow.

That means:

- without metadata access, the engine evaluates SQL and policy only
- with metadata access, the same engine evaluates SQL, policy, instance facts, and table snapshots

## Metadata Model

### Instance facts

These are server-level facts that influence compatibility and risk decisions:

- `version`
- `character_set_database`
- `innodb_large_prefix`
- `innodb_default_row_format`
- `innodb_adaptive_hash_index`

### Object facts

These are table-level facts used by existence and compatibility rules:

- whether the target table exists
- current table snapshot
- current column definitions
- current index definitions
- current primary-key definition

## Proposed Shape

- `internal/domain/spec`
  - extend metadata-aware audit facts and snapshot types
- `internal/domain/rule`
  - keep rules consuming normalized facts, not raw database clients
- `internal/application/audit`
  - orchestrate offline-only and optional metadata-enriched execution
- `internal/infrastructure/...`
  - add metadata provider implementations for MySQL and TiDB

## Capability Matrix

The capability matrix is a milestone artifact, not a nice-to-have appendix.

It should:

- list every important audit ability in a stable document
- mark each row as:
  - covered
  - replaced/enhanced
  - deferred
  - intentionally out of scope
- point to the rule IDs or modules that implement each capability

This matrix becomes the acceptance checklist for the milestone.

## Scope

### In scope

- capability matrix
- optional metadata provider abstraction
- instance facts loading
- target-table snapshot loading
- object existence checks
- deeper alter compatibility checks
- remaining important DDL/DML audit gaps
- documentation and release-surface upgrade

### Out of scope

- MCP server
- auth and bearer-token middleware
- explain-plan-driven risk analysis
- full-database inspection/crawling
- UI work

## Expected Outcome

After this milestone, DeltaScope should have one coherent answer to “what can it audit?” and that answer should be backed by code, tests, and a capability matrix instead of intuition.
