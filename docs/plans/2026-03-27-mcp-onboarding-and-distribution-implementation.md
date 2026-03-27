# MCP Onboarding And Distribution Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** ship a complete MCP onboarding and distribution experience for DeltaScope, including an npm launcher, mainstream client setup docs, and manual configuration guidance that makes `deltascope-mcp` feel like a first-class MCP product.

**Architecture:** keep `deltascope-mcp` as the canonical Go stdio server, add a thin npm launcher that downloads and caches release binaries, and restructure documentation so README points to one dedicated MCP onboarding guide with client-specific and manual configuration paths.

**Tech Stack:** Go release artifacts, GoReleaser, GitHub Releases, npm package tooling, Node.js launcher code, Markdown docs, shell verification, MCP client configuration examples

---

### Task 1: Save the planning artifacts

**Files:**
- Create: `docs/plans/2026-03-27-mcp-onboarding-and-distribution-design.md`
- Create: `docs/plans/2026-03-27-mcp-onboarding-and-distribution-implementation.md`
- Create: `docs/plans/2026-03-27-mcp-onboarding-and-distribution-task-prompts.md`
- Create: `docs/plans_zh/2026-03-27-mcp-onboarding-and-distribution-design.md`
- Create: `docs/plans_zh/2026-03-27-mcp-onboarding-and-distribution-implementation.md`
- Create: `docs/plans_zh/2026-03-27-mcp-onboarding-and-distribution-task-prompts.md`

- [ ] **Step 1: Save the approved bilingual planning set**

Write the six planning documents with aligned scope, naming, and milestone language.

- [ ] **Step 2: Review the planning set for drift**

Check that all six files describe the same milestone: npm launcher, mainstream MCP onboarding, and manual configuration support.

- [ ] **Step 3: Commit the planning artifacts**

Run:

```bash
git add docs/plans/2026-03-27-mcp-onboarding-and-distribution-*.md docs/plans_zh/2026-03-27-mcp-onboarding-and-distribution-*.md
git commit -m "docs: plan MCP onboarding distribution milestone"
```

Expected: one commit containing only the new planning files.

### Task 2: Create the npm launcher package scaffold

**Files:**
- Create: `packages/deltascope-mcp/package.json`
- Create: `packages/deltascope-mcp/README.md`
- Create: `packages/deltascope-mcp/bin/deltascope-mcp.js`
- Create: `packages/deltascope-mcp/lib/...`
- Modify: root docs or module READMEs affected by the new package tree
- Test: launcher smoke tests

- [ ] **Step 1: Write the failing launcher smoke test**

Define a test that proves the launcher resolves a release asset URL, downloads a binary into a cache directory, and spawns it with stdio.

- [ ] **Step 2: Create the package scaffold**

Add a package named `@fanduzi/deltascope-mcp` with one executable entrypoint.

- [ ] **Step 3: Implement minimal command passthrough**

Keep the launcher responsible only for bootstrap, cache, and process execution.

- [ ] **Step 4: Run focused launcher tests**

Run:

```bash
npm test --prefix packages/deltascope-mcp
```

Expected: launcher bootstrap tests pass.

- [ ] **Step 5: Commit**

```bash
git add packages/deltascope-mcp
git commit -m "feat: add npm MCP launcher scaffold"
```

### Task 3: Implement release resolution, download, and cache behavior

**Files:**
- Modify: `packages/deltascope-mcp/bin/deltascope-mcp.js`
- Create/Modify: `packages/deltascope-mcp/lib/platform.js`
- Create/Modify: `packages/deltascope-mcp/lib/releases.js`
- Create/Modify: `packages/deltascope-mcp/lib/cache.js`
- Test: launcher resolution tests

- [ ] **Step 1: Write failing tests for version and platform resolution**

Cover:

- default latest release resolution
- explicit version override
- asset naming for darwin/linux and amd64/arm64
- cache hits vs cache misses

- [ ] **Step 2: Implement platform normalization**

Map Node host facts to the DeltaScope asset naming contract.

- [ ] **Step 3: Implement release download and unpack**

Download the matching archive from GitHub Releases and unpack the native `deltascope-mcp` binary into a deterministic cache path.

- [ ] **Step 4: Implement cache reuse**

Ensure repeated runs do not redownload the same binary unless the version changes.

- [ ] **Step 5: Run focused tests**

Run:

```bash
npm test --prefix packages/deltascope-mcp
```

Expected: release-resolution and cache tests pass.

- [ ] **Step 6: Commit**

```bash
git add packages/deltascope-mcp
git commit -m "feat: implement MCP launcher download cache"
```

### Task 4: Add launcher execution and stdio bridging verification

**Files:**
- Modify: `packages/deltascope-mcp/bin/deltascope-mcp.js`
- Modify/Create: launcher test files
- Test: end-to-end launcher execution test

- [ ] **Step 1: Write the failing launcher execution test**

Mock or fixture a real `deltascope-mcp` binary invocation contract and assert the launcher forwards stdio and exit status transparently.

- [ ] **Step 2: Implement process spawning**

Spawn the cached native binary with inherited stdin, stdout, and stderr semantics that preserve MCP stdio behavior.

- [ ] **Step 3: Add argument passthrough**

Ensure the launcher can pass flags like `-connections-path`.

- [ ] **Step 4: Run focused tests**

Run:

```bash
npm test --prefix packages/deltascope-mcp
```

Expected: stdio bridging tests pass.

- [ ] **Step 5: Commit**

```bash
git add packages/deltascope-mcp
git commit -m "feat: bridge stdio through npm MCP launcher"
```

### Task 5: Wire package publishing and release documentation

**Files:**
- Modify: release docs
- Modify/Create: package publish workflow/config if needed
- Modify: root README release/install sections
- Modify: package README
- Test: publish-config sanity checks

- [ ] **Step 1: Define the package publish contract**

Document package name, versioning relationship to DeltaScope, and whether package versions mirror release tags exactly.

- [ ] **Step 2: Add publish/config files**

Add only the minimal package metadata and workflow wiring required to publish the launcher reliably.

- [ ] **Step 3: Keep the release contract explicit**

Document that the npm launcher is a bootstrap layer over GitHub Release binaries, not a separate MCP implementation.

- [ ] **Step 4: Run sanity checks**

Run:

```bash
npm pack --dry-run --prefix packages/deltascope-mcp
```

Expected: the package bundles only the intended launcher files.

- [ ] **Step 5: Commit**

```bash
git add packages/deltascope-mcp README.md docs
git commit -m "build: define MCP launcher publish contract"
```

### Task 6: Create the dedicated MCP onboarding guide

**Files:**
- Create: `docs/recipe/use-deltascope-mcp.md`
- Create: `docs/recipe/use-deltascope-mcp.zh-CN.md`
- Modify: `docs/recipe/README.md` if needed
- Modify: MCP-related cross-links in existing docs

- [ ] **Step 1: Draft the English guide**

Cover:

- what DeltaScope MCP is
- recommended `npx` path
- native binary fallback
- Claude Code setup
- Codex setup
- generic stdio TOML/JSON config
- direct connection example
- `connection_ref` example
- minimal `connections.yaml`
- common failures

- [ ] **Step 2: Draft the Chinese guide**

Keep the same structure and examples aligned with the English guide.

- [ ] **Step 3: Add copy-paste-ready examples**

Include exact snippets for:

```bash
claude mcp add --scope user deltascope -- npx -y @fanduzi/deltascope-mcp
codex mcp add deltascope -- npx -y @fanduzi/deltascope-mcp
```

and generic stdio config examples.

- [ ] **Step 4: Run content sanity review**

Check that the docs reflect the actual MCP contract and `connection_ref` behavior already implemented in the server.

- [ ] **Step 5: Commit**

```bash
git add docs/recipe/use-deltascope-mcp.md docs/recipe/use-deltascope-mcp.zh-CN.md docs/recipe
git commit -m "docs: add dedicated MCP onboarding guide"
```

### Task 7: Restructure README and existing MCP references

**Files:**
- Modify: `README.md`
- Modify: `README_ZH.md`
- Modify: `docs/recipe/use-with-ai-agents.md`
- Modify: `docs/recipe/use-with-ai-agents.zh-CN.md`
- Modify: `cmd/deltascope-mcp/README.md`

- [ ] **Step 1: Write the failing doc checklist**

List the required README outcomes:

- README contains a short MCP quick start
- README links to the dedicated MCP guide
- the agent recipe links out instead of duplicating onboarding details
- `cmd/deltascope-mcp/README.md` stays command-focused

- [ ] **Step 2: Add MCP quick start to the root README files**

Keep the section short and copy-paste friendly.

- [ ] **Step 3: Trim and relink existing MCP prose**

Make `use-with-ai-agents` focus on agent workflow semantics, not full onboarding duplication.

- [ ] **Step 4: Run doc sanity checks**

Read the English and Chinese README MCP sections side by side and confirm the entrypoints match.

- [ ] **Step 5: Commit**

```bash
git add README.md README_ZH.md docs/recipe/use-with-ai-agents.md docs/recipe/use-with-ai-agents.zh-CN.md cmd/deltascope-mcp/README.md
git commit -m "docs: streamline MCP quick start entrypoints"
```

### Task 8: Add verification for launcher and onboarding contract

**Files:**
- Modify/Create: launcher tests
- Modify/Create: docs validation notes
- Possibly Modify: CI workflows if launcher verification should run automatically

- [ ] **Step 1: Define the verification matrix**

Cover:

- launcher unit tests
- dry-run package bundling
- native Go test suite still green
- docs examples reflect real command names and flags

- [ ] **Step 2: Add the missing automated checks**

Add only the minimum CI or local test commands needed to keep the launcher release path trustworthy.

- [ ] **Step 3: Run the combined verification set**

Run:

```bash
npm test --prefix packages/deltascope-mcp
npm pack --dry-run --prefix packages/deltascope-mcp
go test ./...
```

Expected: all pass without changing the MCP server contract.

- [ ] **Step 4: Commit**

```bash
git add packages/deltascope-mcp .github docs
git commit -m "test: verify MCP launcher onboarding contract"
```

### Task 9: Final review and release-readiness handoff

**Files:**
- Modify: release notes or future milestone notes if needed
- Modify: affected docs indexes/READMEs
- Modify: any remaining MCP onboarding references

- [ ] **Step 1: Re-read the milestone against the design**

Confirm the deliverable now covers:

- npm launcher
- Claude Code
- Codex
- generic MCP clients
- manual native binary setup
- direct connection
- `connection_ref`

- [ ] **Step 2: Run final verification**

Run:

```bash
npm test --prefix packages/deltascope-mcp
npm pack --dry-run --prefix packages/deltascope-mcp
go test ./...
```

Expected: final green state for both launcher and native server.

- [ ] **Step 3: Prepare release follow-up**

Record what the next release must include: launcher publish, updated README, dedicated MCP guide, and verified onboarding snippets.

- [ ] **Step 4: Commit**

```bash
git add .
git commit -m "docs: close MCP onboarding distribution milestone"
```
