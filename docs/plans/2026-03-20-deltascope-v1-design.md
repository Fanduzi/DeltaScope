# DeltaScope v1 Design

## Goal

Build `DeltaScope` as a new SQL review engine for MySQL and TiDB. It must preserve the core DDL/DML auditing value of `gAudit`, but be a full rewrite with a cleaner architecture, stronger rule model, and a path toward HTTP API and MCP delivery.

## Product Scope

### Version Roadmap

- **v1:** Go library + CLI
- **v2:** HTTP API service
- **v3:** MCP Server

### v1 Boundaries

- Offline static auditing only
- No required live database connection
- Input is SQL text plus optional YAML config
- Default config is built in when no config file is provided
- Supported dialects are **MySQL + TiDB**

### Non-Goals for v1

- No online metadata lookup
- No table/column existence checks against a real database
- No `EXPLAIN`-based affected row estimation
- No schema snapshot input model

## Key Product Principles

- `DeltaScope` must become a **strict superset** of `gAudit` audit capability over time
- v1 should already cover the main offline-static rule set from `gAudit`
- Architecture quality matters more than cloning old interfaces
- Rules are first-class product features, not scattered conditionals
- Results must be stable for AI agents, CI, and future services

## Architecture

`DeltaScope` uses a DDD-leaning architecture with a unified domain review model.

### Layers

- `interfaces` for CLI now, HTTP/MCP later
- `application` for use-case orchestration
- `domain` for core audit concepts and logic
- `infrastructure` for parser/config/output adapters

### Dependency Direction

`interfaces -> application -> domain <- infrastructure`

The domain layer must not depend on Cobra, Viper, or TiDB parser AST types.

## Domain Model

The core domain object is a unified `StatementSpec`.

### `StatementSpec`

Each parsed SQL statement is transformed into a normalized domain model before rule evaluation. Rules never operate directly on parser AST nodes.

Expected fields:

- statement kind
- dialect
- raw SQL
- normalized SQL
- source location if available
- parser warnings if available

Optional sub-structures attached as needed:

- `TableSpec`
- `ColumnSpec[]`
- `IndexSpec[]`
- `ConstraintSpec[]`
- `AlterAction[]`
- `DMLSpec`
- `ObjectRefs`

## Audit Pipeline

The v1 pipeline is:

1. Load built-in defaults
2. Optionally load YAML config overrides
3. Parse SQL into AST
4. Convert AST into `StatementSpec`
5. Select applicable rules
6. Evaluate rules
7. Aggregate findings and verdict
8. Render Markdown or JSON output

## Rules and Policy

### Rule Engine

Each rule is an independent unit with:

- stable `rule_id`
- statement applicability
- policy-driven configuration
- findings output

### Rule ID Scheme

Rule IDs use dotted names, for example:

- `ddl.table.comment.require`
- `ddl.table.primary_key.require`
- `ddl.column.varchar.max_length`
- `ddl.index.secondary.prefix.require`
- `dml.where.require`
- `dml.limit.forbid`

### Policy Format

Use one YAML config file with grouped top-level sections such as:

- `app`
- `parser`
- `rules`
- `output`

Rules are configured by rule ID, for example:

```yaml
rules:
  ddl.table.comment.require:
    enabled: true
    level: warning

  ddl.table.name.max_length:
    enabled: true
    value: 64
    level: blocker

  dml.where.require:
    enabled: true
    level: blocker
```

## Result Model

### Finding Levels

- `blocker`
- `warning`
- `notice`

### Verdict

- `reject` if any `blocker`
- `review` if no blocker but at least one `warning`
- `pass` otherwise

### Finding Shape

Each finding should support:

- `rule_id`
- `level`
- `message`
- `statement_index`
- `statement_kind`
- `location`
- `suggestion`
- `metadata`

### Result Shape

Top-level result contains:

- statement results
- global findings
- summary counts
- final verdict

Each statement result contains its own findings.

## CLI Design

### Main Command

- `deltascope audit`

### Input Modes

- `--sql`
- `--file`
- `stdin`

### Key Flags

- `--config`
- `--dialect`
- `--format`
- `--fail-on`
- `--quiet`

### Output

- default format: `markdown`
- required optional format: `json`

### Exit Codes

- `0`: success and below fail threshold
- `1`: audit completed but failed threshold
- `2`: user/input/config error
- `3`: internal/runtime error

## Config and Tooling Choices

- CLI framework: `cobra`
- config system: `viper`
- config format: `YAML`
- config hot reload: designed in from v1, most useful for future long-running modes

For v1 CLI, config watchers should not distort the one-shot command model. The architecture should support config watching later without forcing unnecessary runtime complexity now.

## v1 Rule Scope

### Tier 1: Required in v1

Main offline-static rules derived from `gAudit` and improved in structure:

- DDL table naming, comment, charset, engine
- primary key presence and shape
- audit columns
- column naming, comment, type and default constraints
- type allow/forbid rules
- index naming, count, redundancy and duplication checks
- alter/drop/rename restrictions
- create table as/like, foreign key, partition, view toggles
- DML where/limit/order by/subquery/join-on rules
- insert row count rules
- replace/on-duplicate/insert-select restrictions

### Tier 2: Later Offline Enhancements

- richer type compatibility checks
- deeper alter-action modeling
- stronger index length/prefix inference
- better location spans and fix suggestions
- better dialect nuance

### Tier 3: Future Online Capabilities

- existence checks for tables/columns/indexes
- explain-based affected-row estimation
- live row-count constraints for drop/truncate
- live version-aware rule decisions

## Repository Structure Direction

Planned structure:

- `cmd/deltascope`
- `internal/interfaces`
- `internal/application`
- `internal/domain`
- `internal/infrastructure`
- `pkg/deltascope`

Within `internal/domain`, expected focus areas:

- `spec`
- `rule`
- `policy`
- `report`

Within `internal/infrastructure`, expected adapters:

- `parser/tidb`
- `config/viper`
- `output/markdown`
- `output/json`

## Quality Bar

- The rewrite must not mirror `gAudit` package layout or checker style
- Domain rules must stay decoupled from parser AST
- Output contracts must be stable for AI-agent consumption
- v1 should be structured for reuse by CLI, HTTP API, and MCP without re-architecting core audit logic
