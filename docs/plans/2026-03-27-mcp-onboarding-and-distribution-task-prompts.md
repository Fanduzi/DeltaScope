# MCP Onboarding And Distribution Task Prompts

> For task-by-task implementation and review of the `MCP Onboarding And Distribution` milestone.
> Every prompt assumes work happens inside `/Users/fan/GolangProjects/DeltaScope`.

## Global Rules

- Keep `deltascope-mcp` as the canonical MCP server implementation.
- Do not fork or reimplement the MCP tool contract inside the npm package.
- Treat the npm package as a launcher/bootstrap layer only.
- Preserve native binary workflows for CI, automation, and non-Node users.
- All onboarding examples must be copy-paste friendly and reflect real command names and flags.
- Claude Code and Codex are top-priority examples, but not the only audience.
- Always include a generic manual stdio configuration example.
- Always document both direct connection and `connection_ref`.
- `connection_ref` docs must include the actual `connections.yaml` shape.
- Keep English and Chinese docs aligned.
- Use tests or dry-run verification for any launcher resolution, cache, or packaging behavior.
- Return files changed, tests run, status, and commit hash for every task.

## Milestone Focus

- npm launcher package `@fanduzi/deltascope-mcp`
- release-binary download and cache bootstrap
- Claude Code onboarding
- Codex onboarding
- generic MCP client stdio configuration
- native binary fallback path
- dedicated MCP quick-start and usage docs
- one coherent product story across launcher, binary, and connection configuration

## Task Intent

### Task 1: Planning Artifacts

- Save the approved design, implementation plan, and task prompts in English and Chinese.
- Keep the milestone centered on onboarding and distribution UX, not new server internals.

### Task 2: Launcher Scaffold

- Add the minimum npm package structure needed to ship a bootstrap executable.
- Keep the launcher small and easy to reason about.

### Task 3: Release Resolution And Cache

- Download the right DeltaScope binary for the local platform.
- Cache by version and platform so repeated runs stay fast.

### Task 4: Stdio Execution

- Spawn the real `deltascope-mcp` binary and forward stdio cleanly.
- Preserve MCP behavior instead of adding launcher-specific semantics.

### Task 5: Publish Contract

- Make the npm package versioning and release relationship explicit.
- Keep the package contract honest about being a launcher over GitHub Release assets.

### Task 6: Dedicated MCP Guide

- Write one guide that a new user can actually follow end-to-end.
- Include Claude Code, Codex, generic stdio config, direct connection, and `connection_ref`.

### Task 7: README Restructure

- Keep README quick-start friendly.
- Link to the dedicated MCP guide instead of duplicating all details everywhere.

### Task 8: Verification

- Prove launcher behavior with tests and package dry runs.
- Keep the native Go test suite green.

### Task 9: Release-Readiness Handoff

- Make the milestone shippable.
- Leave a clear release checklist for the npm launcher and updated MCP docs.
