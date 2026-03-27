# MCP Onboarding And Distribution Design

## Goal

Define the next DeltaScope MCP milestone so `deltascope-mcp` is not only a working MCP server, but also a product that users can discover, install, configure, and connect to mainstream MCP clients with minimal friction.

## Context

DeltaScope `v0.7.0` already ships:

- the official `deltascope-mcp` stdio server
- stable MCP tools: `audit_sql`, `describe_rule`, `list_rules`, `get_capabilities`
- metadata-aware MCP support for direct connections and `connection_ref`
- GoReleaser assets and installer support
- real stdio + MySQL/TiDB end-to-end validation

The current gap is not server correctness. The current gap is onboarding and distribution UX.

Today, a new user still has to understand too much:

- how to install `deltascope-mcp`
- how to wire it into Claude Code, Codex, or other MCP clients
- when to use direct connection vs `connection_ref`
- what `connections.yaml` should look like
- how to override the config path

That is materially worse than the current MCP market baseline where users often copy one `npx -y ...` command and connect immediately.

## Problem Statement

DeltaScope has an official MCP server, but it does not yet feel like a mainstream MCP product. The project needs one coherent onboarding and distribution model that makes three paths obvious:

1. zero-friction recommended path for mainstream MCP clients
2. manual stdio configuration path for clients that do not have helper commands
3. direct binary path for CI, local automation, and operators who want full control

## Non-Goals

This milestone does not:

- redesign the `deltascope-mcp` tool contract
- add new audit rules or new metadata providers
- replace GoReleaser-based binary publishing
- add hosted transport, remote auth, or SaaS infrastructure
- rewrite the MCP server in Node.js
- add every MCP client-specific document in one release

## Approaches Considered

### Approach A: Documentation-Only Improvement

Keep binary distribution exactly as-is and only improve README and recipe docs.

Pros:

- fastest path
- no new release pipeline surface

Cons:

- still worse than the `npx -y ...` onboarding pattern users now expect
- users still need prior installation and PATH setup
- client examples remain less copy-paste friendly

### Approach B: Native Binary-Only Product Surface

Treat `deltascope-mcp` as a binary-first MCP server and optimize only native install paths.

Pros:

- operationally simple
- keeps one artifact model

Cons:

- poor match for current MCP client onboarding norms
- weaker fit for Claude Code / Codex one-liner setup
- discoverability remains lower than npm-based MCP servers

### Approach C: Hybrid Distribution Model

Keep `deltascope-mcp` as the canonical Go stdio server, continue shipping release binaries, and add an npm launcher package that downloads, caches, and executes the correct DeltaScope binary for the user platform.

Pros:

- matches mainstream MCP onboarding expectations
- preserves the existing Go implementation and release assets
- keeps binary-first workflows available for CI and operators
- allows README examples that feel comparable to `npx -y @upstash/context7-mcp`

Cons:

- introduces a second distribution surface
- requires version-selection and cache behavior design
- adds docs and verification work across Node + Go release surfaces

## Recommendation

Choose Approach C.

DeltaScope should present one product, not one transport plus a bag of setup instructions. The right model is:

- canonical server runtime: `deltascope-mcp`
- recommended user entrypoint: `npx -y @fanduzi/deltascope-mcp`
- fallback/manual entrypoint: native `deltascope-mcp`

This aligns with current MCP user expectations without forcing a rewrite of the server itself.

## Design

### 1. Product Definition

The MCP product surface now has two entry styles for one server:

- **recommended**: npm launcher package `@fanduzi/deltascope-mcp`
- **canonical runtime**: native `deltascope-mcp` binary

The launcher is not a second MCP implementation. It is a bootstrap layer that resolves and executes the real DeltaScope binary.

### 2. Distribution Model

#### Canonical Runtime

The real server remains the Go binary:

- `deltascope-mcp`

It stays the source of truth for:

- tool schema
- connection handling
- metadata-aware behavior
- release validation

#### Launcher Package

The npm launcher should:

- detect the host OS and architecture
- resolve a DeltaScope version
- download the matching release asset from GitHub Releases
- cache the unpacked binary locally
- spawn `deltascope-mcp` and bridge stdio transparently

The launcher should not:

- reimplement MCP tools
- parse SQL itself
- add product semantics that differ from the native binary

#### Version Model

Recommended behavior:

- default: launcher uses the latest compatible DeltaScope release
- override: allow explicit version pinning through an environment variable or launcher flag
- cache key: release version + platform + architecture

This keeps local onboarding simple while preserving reproducibility for advanced users.

### 3. Onboarding Paths

The project should explicitly document four user paths.

#### Path A: Claude Code

Primary example:

```bash
claude mcp add --scope user deltascope -- npx -y @fanduzi/deltascope-mcp
```

#### Path B: Codex

Primary example:

```bash
codex mcp add deltascope -- npx -y @fanduzi/deltascope-mcp
```

#### Path C: Other Mainstream MCP Clients

Document a generic stdio configuration that can be adapted to clients with TOML/JSON config files.

Example:

```toml
[mcp_servers.deltascope]
command = "npx"
args = ["-y", "@fanduzi/deltascope-mcp"]
startup_timeout_sec = 20
```

#### Path D: Manual Native Binary

Document the direct binary path for users who want:

- CI integration
- air-gapped or controlled environments
- no Node.js dependency

Example:

```toml
[mcp_servers.deltascope]
command = "deltascope-mcp"
args = []
startup_timeout_sec = 20
```

### 4. Configuration Story

The documentation should treat configuration as two supported modes, not one advanced mode and one hidden fallback.

#### Direct Connection

Recommended for:

- quick tests
- ephemeral sessions
- one-off metadata-aware calls

Users pass connection data directly in the MCP tool input.

#### `connection_ref`

Recommended for:

- repeated usage
- safer secret handling
- named environment profiles

The default config file remains:

- `~/.config/deltascope/connections.yaml`

The docs must include a minimal, copy-paste-ready example:

```yaml
connections:
  prod_readonly:
    host: 127.0.0.1
    port: 3306
    user: app_readonly
    password_env: MYSQL_PASSWORD
    schema: app
```

The docs must also show how to override the path:

```bash
deltascope-mcp -connections-path /path/to/connections.yaml
```

and through MCP client config:

```toml
[mcp_servers.deltascope]
command = "deltascope-mcp"
args = ["-connections-path", "/path/to/connections.yaml"]
startup_timeout_sec = 20
```

### 5. Documentation Architecture

The docs should split by user intent.

#### Root README

Keep only a short MCP quick start:

- what DeltaScope MCP is
- one Claude Code example
- one Codex example
- one link to the dedicated MCP guide

#### Dedicated MCP Guide

Create a dedicated guide for real onboarding, not just contract reference.

Recommended file names:

- `docs/recipe/use-deltascope-mcp.md`
- `docs/recipe/use-deltascope-mcp.zh-CN.md`

This guide should cover:

- install and prerequisites
- Claude Code
- Codex
- generic TOML/JSON stdio config
- native binary path
- direct connection
- `connection_ref`
- `connections.yaml`
- common failures and recovery steps

#### Existing AI-Agent Recipe

`docs/recipe/use-with-ai-agents.md` should remain focused on agent workflow and DeltaScope semantics. It should link to the dedicated MCP guide instead of trying to carry all onboarding content itself.

### 6. Success Criteria

This milestone is successful when:

- a new user can copy one Claude Code command and connect DeltaScope over MCP
- a new user can copy one Codex command and connect DeltaScope over MCP
- a user without helper CLI support can configure the server through a generic stdio config example
- a user can choose direct connection or `connection_ref` without guessing
- the docs clearly show the minimum viable `connections.yaml`
- the npm launcher and native binary are presented as one coherent product story

## Risks

### Distribution Drift

The launcher can drift from the native binary release contract if version resolution or asset naming is not kept explicit.

Mitigation:

- use one documented asset naming contract
- add launcher verification against real releases

### Documentation Fragmentation

If README, MCP guide, and agent guide all try to explain everything, they will drift.

Mitigation:

- give each doc one job
- keep README short
- keep the dedicated MCP guide as the primary onboarding source

### Over-Optimizing For One Client

Writing only for Claude Code or only for Codex would weaken the product.

Mitigation:

- always include generic stdio configuration
- treat Claude and Codex as top examples, not the entire audience

## Decision

The next MCP milestone should focus on onboarding and distribution UX, not on new server internals. DeltaScope should adopt a hybrid model: the Go binary remains canonical, while an npm launcher becomes the recommended user-facing entrypoint. Documentation should be reorganized around copy-paste onboarding for mainstream MCP clients, plus clear manual configuration and connection management guidance.
