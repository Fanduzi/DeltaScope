# Official MCP Server Design

## Goal

Ship an official `deltascope-mcp` server so AI agents can call DeltaScope through a stable MCP tool surface for both offline and metadata-aware SQL audit workflows.

## Context

DeltaScope `v0.6.2` already has the core pieces needed for an MCP surface:

- a stable audit engine shared by CLI, HTTP, and `pkg/deltascope`
- explainable audit results with `summary`, `explanation`, `why`, `risk`, and `suggestion`
- optional metadata-aware auditing through a public `MetadataProvider`
- stable rule discovery concepts through the shipped rule catalog

The current gap is not audit correctness. The current gap is official agent integration.

Today, agent users can wrap the CLI manually, but that still leaves several product problems unsolved:

- no official MCP tool contract
- no first-party error taxonomy for agent callers
- no first-party way to choose metadata-aware connections
- no shared story for connection security, password handling, and result context

## Non-Goals

This milestone does not:

- add a hosted service, auth, or multi-tenancy
- add new DDL or DML rules
- replace the existing CLI or HTTP products
- add broad agent-framework adapters beyond MCP
- add streamable HTTP/SSE transport in the first release
- redesign DeltaScope's core audit result model

## Approaches Considered

### Approach A: MCP Wrapper Around The Existing CLI

Run `deltascope audit` and `deltascope rules ...` as subprocesses from the MCP server.

Pros:

- fastest first prototype
- minimal new business logic

Cons:

- ties MCP behavior to CLI process orchestration and exit-code handling
- weaker typed error mapping
- more awkward metadata-aware connection plumbing
- adds long-term maintenance friction for an official surface

### Approach B: MCP Server Directly On Top Of The Shared Go Library

Build the MCP server as a thin adapter over `pkg/deltascope`, the rule catalog, and shared application helpers.

Pros:

- preserves a clean transport boundary
- reuses the stable shared audit path directly
- better error control and testability
- strongest foundation for an official product surface

Cons:

- slightly more implementation work than a CLI wrapper
- requires a small amount of new MCP-specific wiring

### Approach C: Extend The Existing HTTP Service To Speak MCP

Teach `deltascope-server` to serve HTTP APIs and MCP from the same process.

Pros:

- fewer binaries
- one network-oriented service entrypoint

Cons:

- blurs two separate product surfaces too early
- pushes transport concerns into the wrong milestone
- increases scope with little first-release value

## Recommendation

Choose Approach B.

DeltaScope already has a clear library-first architecture. The official MCP server should become another thin interface layer, not a subprocess shell and not an overloaded HTTP service. That keeps the milestone focused on a stable transport adapter and lets the shared audit contracts remain the source of truth.

## Design

### 1. Product Definition

The first official MCP release should ship as a local stdio server:

- binary: `deltascope-mcp`
- scope: local agent integration
- runtime model: stateless request handling within one long-running process

The MCP server becomes the fourth formal product surface beside:

- `deltascope` CLI
- `deltascope-server`
- `pkg/deltascope`
- `deltascope-mcp`

### 2. Tool Surface

The first release should expose three required tools and one optional helper.

Required:

- `audit_sql`
- `describe_rule`
- `list_rules`

Optional but recommended:

- `get_capabilities`

#### `audit_sql`

Primary audit tool for agents.

Inputs:

- `sql` (required)
- `dialect` (optional)
- `config_path` (optional)
- `connection_ref` (optional)
- `connection` (optional)

Rules:

- `connection_ref` and `connection` are mutually exclusive
- when neither is provided, the audit runs offline
- when either is provided, the audit runs metadata-aware

#### `describe_rule`

Returns the shipped rule metadata for a single `rule_id`, suitable for follow-up explanation or remediation by an agent.

#### `list_rules`

Returns shipped rules with optional filters such as statement kind, level, metadata awareness, or keyword search.

#### `get_capabilities`

Returns a concise capability summary for agents that want to reason about supported dialects, modes, and public surfaces before invoking the other tools.

### 3. Result Shape

The MCP server should preserve the existing `v0.6.2` audit result body and add only one top-level additive field: `context`.

Recommended `audit_sql` success shape:

- existing top-level fields remain unchanged:
  - `verdict`
  - `summary`
  - `statements`
  - `global_findings`
  - `explanation`
- add:
  - `context`

`context` should describe how the result was produced without duplicating the audit facts:

- `mode`: `offline` or `metadata-aware`
- `dialect`: final effective dialect
- `dialect_source`: `default`, `request`, `detected`, or `connection`
- `schema`: final effective schema when available
- `schema_source`: `none`, `request`, `connection`, or `inferred`
- `metadata_source`: `none`, `connection_ref`, or `direct`

Important constraints:

- keep `summary`, statement-level `explanation`, and finding-level `explanation` intact
- keep `why`, `risk`, and `suggestion` intact
- do not create a second MCP-only explanation model
- do not leak connection parameters into `context`

### 4. Error Model

MCP failures should return a structured error body instead of partial audit output.

Recommended stable error codes:

- `bad_request`
- `connection_invalid`
- `connection_failed`
- `config_invalid`
- `internal_error`

Rules:

- callers may rely on `code`
- callers must not parse `message`
- metadata-aware requests must not silently fall back to offline mode on connection failure

### 5. Metadata-Aware Connection Design

The MCP server should support two connection-input styles.

#### Recommended style: `connection_ref`

The user provides a named connection reference, and the server resolves the real connection details from a local config file.

Recommended local file:

- `~/.config/deltascope/connections.yaml`

Shape:

- `connections.<name>` maps to a connection definition

This lets users say things like "use `prod_readonly`" without exposing secrets in prompts.

#### Direct style: `connection`

The user provides connection details inline for local or ad-hoc use.

Recommended fields:

- `host`
- `port`
- `socket` (optional)
- `user`
- `schema` (optional)
- `dialect` (optional)
- exactly one password source:
  - `password`
  - `password_env`
  - `password_file`

### 6. Secret Handling

The official MCP server must allow direct `password` input for usability, but it should not treat inline secrets as the preferred operational model.

Recommended guidance order:

1. `connection_ref`
2. `password_env`
3. `password_file`
4. `password`

Required safeguards:

- never echo passwords or full DSNs in logs, errors, or tool results
- validate that only one password source is present
- allow config-file cleartext passwords only as a convenience path, not as the recommended default

### 7. Shared Code Reuse

This milestone should reuse existing audit and metadata logic wherever practical.

Expected new structure:

- `cmd/deltascope-mcp`
  - process entrypoint
- `internal/interfaces/mcp`
  - MCP server wiring
  - tool schemas
  - request validation
  - result and error shaping
- shared helper extraction where useful from CLI metadata-preparation code

The MCP layer should not duplicate:

- rule evaluation
- result aggregation
- public explanation shaping
- metadata provider behavior

### 8. Documentation Strategy

This milestone should document:

- how to configure MCP with `deltascope-mcp`
- how `audit_sql` chooses offline versus metadata-aware mode
- how `connection_ref` and direct `connection` work
- the result `context` fields
- secret-handling recommendations for prompts and local config files

## Acceptance Criteria

This milestone is complete when:

1. `deltascope-mcp` runs as an MCP stdio server.
2. `audit_sql`, `describe_rule`, and `list_rules` are available through MCP.
3. `audit_sql` supports offline, `connection_ref`, and direct `connection` modes.
4. Metadata-aware runs reuse the shared DeltaScope audit path rather than the CLI subprocess path.
5. MCP success responses preserve the `v0.6.2` audit result structure and add only a top-level `context`.
6. Passwords and DSNs are never exposed in logs, returned payloads, or error messages.
7. Metadata-aware connection failures return structured errors and do not degrade silently to offline mode.
8. English and Chinese docs explain setup, tool usage, connection handling, and result interpretation.
