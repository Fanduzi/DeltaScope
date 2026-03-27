# Official MCP Server Task Prompts

> For task-by-task implementation and review of the `Official MCP Server` milestone.
> Every prompt assumes work happens inside `/Users/fan/GolangProjects/DeltaScope`.

## Global Rules

- Keep the shared audit engine as the source of truth.
- Do not shell out to `deltascope` for MCP audit execution unless a task explicitly revisits that decision.
- Preserve the `v0.6.2` audit result body and add only a top-level `context` field for MCP `audit_sql` success responses.
- Keep `summary`, `explanation`, `why`, `risk`, and `suggestion` intact.
- Treat metadata-aware requests as explicit intent; connection failure must not silently downgrade to offline mode.
- Allow direct `password` input, but never expose passwords or full DSNs in logs, errors, docs examples, or tool results.
- Use TDD for every non-trivial MCP contract, connection, or transport change.
- Keep `three-level-doc` as a hard gate.
- Return files changed, tests run, status, and commit hash for every task.

## Milestone Focus

- official `deltascope-mcp` stdio server
- stable MCP tool schemas
- offline and metadata-aware `audit_sql`
- `connection_ref` plus direct `connection` support
- secret-safe connection handling
- rule discovery through `describe_rule` and `list_rules`
- docs that teach agent users how to set up and use the MCP server

## Task Intent

### Task 1: Planning Artifacts

- Save the approved design, implementation plan, and task prompts in English and Chinese.
- Keep naming and scope aligned with `Official MCP Server`.

### Task 2: MCP Runtime Bootstrap

- Add the minimal MCP runtime needed for a local stdio server.
- Keep the transport layer thin and easy to test.

### Task 3: MCP Contracts

- Define the tool request, success, and error shapes before filling in business logic.
- Success results must preserve DeltaScope's existing audit semantics.

### Task 4: Shared Metadata Helpers

- Extract reusable metadata-preparation logic instead of duplicating CLI code.
- Keep the existing CLI path behaviorally stable after extraction.

### Task 5: Connection Resolution

- Support both `connection_ref` and direct `connection`.
- Enforce mutual exclusion and password-source validation clearly.

### Task 6: `audit_sql`

- Reuse the shared DeltaScope audit path directly.
- Return metadata-aware context honestly and never silently downgrade to offline mode.

### Task 7: Rule Tools

- Make `describe_rule` and `list_rules` useful enough for agent follow-up reasoning.
- Keep the filter set small, stable, and well documented.

### Task 8: Secret Safety

- Prove with tests that passwords and DSNs are never leaked.
- Keep error codes stable and redact aggressively.

### Task 9: Docs

- Teach users how to wire the MCP server and choose between offline, `connection_ref`, and direct connections.
- Keep English and Chinese docs aligned.

### Task 10: Closure

- Re-run focused and broad verification.
- Record the milestone in handoff/progress/decision docs.
- Leave the MCP milestone in a shippable state.
