# Official MCP Server Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** ship an official `deltascope-mcp` stdio server that exposes DeltaScope audit and rule-discovery capabilities over MCP, including both offline and metadata-aware audit flows.

**Architecture:** add a new MCP interface layer on top of the existing shared audit and rule-catalog path. Reuse `pkg/deltascope` result semantics, extract shared metadata-connection preparation where needed, keep the MCP transport thin, and add only a top-level `context` field to successful `audit_sql` responses.

**Tech Stack:** Go, existing DeltaScope application/domain/infrastructure layers, `pkg/deltascope`, rule catalog, metadata provider infrastructure, MCP server library/runtime, Markdown docs, Go testing

---

### Task 1: Save the milestone planning artifacts

**Files:**
- Create: `docs/plans/2026-03-26-official-mcp-server-design.md`
- Create: `docs/plans/2026-03-26-official-mcp-server-implementation.md`
- Create: `docs/plans/2026-03-26-official-mcp-server-task-prompts.md`
- Create: `docs/plans_zh/2026-03-26-official-mcp-server-design.md`
- Create: `docs/plans_zh/2026-03-26-official-mcp-server-implementation.md`
- Create: `docs/plans_zh/2026-03-26-official-mcp-server-task-prompts.md`

**Step 1:** save the approved design, implementation plan, and task prompts in English and Chinese
**Step 2:** review naming, scope, and vocabulary consistency across all six planning files
**Step 3:** commit

### Task 2: Choose and wire the MCP runtime

**Files:**
- Modify/Create: `go.mod`
- Create: `internal/interfaces/mcp/...`
- Create: `cmd/deltascope-mcp/main.go`
- Modify: affected module `README.md` files
- Test: new MCP interface tests

**Step 1:** evaluate the minimal Go MCP runtime/library choice that fits a local stdio server
**Step 2:** add the dependency and create a thin MCP server bootstrap
**Step 3:** expose stdio startup only for the first release
**Step 4:** add focused smoke tests for server boot and tool registration
**Step 5:** commit

### Task 3: Define the shared MCP request, success, and error contracts

**Files:**
- Create/Modify: `internal/interfaces/mcp/...`
- Modify: reference docs and module README files affected by exported contract changes
- Test: contract-focused tests under the MCP package

**Step 1:** write failing tests for `audit_sql` success responses preserving the `v0.6.2` result body plus top-level `context`
**Step 2:** define the MCP-facing request and response types for `audit_sql`, `describe_rule`, and `list_rules`
**Step 3:** define stable structured error codes for request, connection, config, and internal failures
**Step 4:** ensure no success or error payload leaks passwords, full DSNs, or raw connection structs
**Step 5:** run focused tests
**Step 6:** commit

### Task 4: Extract shared metadata connection preparation

**Files:**
- Modify: shared audit or helper packages as needed
- Modify: `internal/interfaces/cli/...`
- Create/Modify: MCP connection helper files under `internal/interfaces/mcp/...`
- Modify: affected module `README.md` files
- Test: focused helper tests

**Step 1:** identify the CLI metadata-preparation logic that should become shared instead of copied
**Step 2:** extract reusable connection-validation, dialect-detection, and schema-resolution helpers
**Step 3:** keep existing CLI behavior stable after the extraction
**Step 4:** run focused tests for both CLI and shared helper behavior
**Step 5:** commit

### Task 5: Implement `connection_ref` and direct connection resolution

**Files:**
- Create/Modify: MCP connection config and resolution files
- Possibly Create: a small connection-config reader package if reuse warrants it
- Modify: docs for connection config format
- Test: connection-config and validation tests

**Step 1:** write failing tests for `connection_ref` lookup, direct `connection` inputs, and mutual-exclusion validation
**Step 2:** implement loading from `~/.config/deltascope/connections.yaml`
**Step 3:** support direct `connection` inputs with `host`, `port`, `socket`, `user`, `schema`, `dialect`, and exactly one password source
**Step 4:** validate `password`, `password_env`, and `password_file` exclusivity clearly
**Step 5:** ensure direct and referenced connections normalize into one shared internal connection shape
**Step 6:** run focused tests
**Step 7:** commit

### Task 6: Implement `audit_sql`

**Files:**
- Modify/Create: `internal/interfaces/mcp/...`
- Modify: any shared request-shaping helpers needed for clean integration
- Test: `audit_sql`-focused interface tests

**Step 1:** write failing tests for offline `audit_sql` success responses
**Step 2:** write failing tests for metadata-aware `audit_sql` responses using both `connection_ref` and direct `connection`
**Step 3:** call the shared DeltaScope audit path directly instead of shelling out to the CLI
**Step 4:** add the additive top-level `context` field with stable mode, dialect, schema, and metadata-source information
**Step 5:** ensure metadata-aware connection failures return structured errors rather than offline fallback
**Step 6:** run focused tests
**Step 7:** commit

### Task 7: Implement `describe_rule` and `list_rules`

**Files:**
- Modify/Create: `internal/interfaces/mcp/...`
- Modify: rule-catalog-related README files if interfaces or discovery semantics need documentation
- Test: MCP rule-tool tests

**Step 1:** write failing tests for rule lookup by `rule_id`
**Step 2:** write failing tests for rule listing with practical filters only
**Step 3:** implement `describe_rule` on top of the shipped rule catalog
**Step 4:** implement `list_rules` with a small, stable filter set rather than a broad query language
**Step 5:** keep returned metadata aligned with the existing rule catalog and public docs
**Step 6:** run focused tests
**Step 7:** commit

### Task 8: Add secret-redaction and error-hardening coverage

**Files:**
- Modify: MCP error and logging helpers
- Modify: connection-resolution code if needed
- Test: redaction, config, and failure-path tests

**Step 1:** write failing tests proving that passwords and DSNs never appear in returned errors
**Step 2:** redact sensitive values in connection and config error paths
**Step 3:** add coverage for missing env vars, unreadable password files, invalid config files, and connection failures
**Step 4:** keep error `code` stable and user-facing `message` actionable
**Step 5:** run focused tests
**Step 6:** commit

### Task 9: Document the official MCP surface

**Files:**
- Modify: `README.md`
- Modify: `README_ZH.md`
- Create/Modify: `docs/reference/...`
- Create/Modify: `docs/recipe/...`
- Modify: `cmd/deltascope/README.md`
- Create/Modify: `cmd/deltascope-mcp/README.md`
- Modify: affected module `README.md` files

**Step 1:** add product-level documentation for `deltascope-mcp`
**Step 2:** document tool inputs, result shape, and error codes
**Step 3:** document `connection_ref`, direct `connection`, and secret-handling guidance
**Step 4:** add examples for offline and metadata-aware MCP calls
**Step 5:** keep English and Chinese docs aligned where applicable
**Step 6:** run link and content sanity checks
**Step 7:** commit

### Task 10: Final verification and milestone closure

**Files:**
- Modify: `docs/plans/2026-03-20-overnight-handoff.md`
- Modify: `docs/plans/2026-03-20-autonomous-progress.md`
- Modify: `docs/plans/2026-03-20-deltascope-v1-decisions.md`
- Modify: any touched module `README.md` files

**Step 1:** run focused MCP package tests, rule-tool tests, and metadata-aware audit tests
**Step 2:** run broader verification such as `go test ./...`
**Step 3:** verify that `audit_sql` success responses preserve the `v0.6.2` audit body and only add top-level `context`
**Step 4:** run three-level-doc validation if required by the current repo workflow
**Step 5:** update handoff, progress, and decisions to record the MCP milestone outcome
**Step 6:** commit
**Step 7:** push
