# Documentation Remediation Plan Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix the highest-impact DeltaScope documentation inaccuracies and tighten first-use verification across CLI, MCP, and skill docs.

**Architecture:** This plan is documentation-only. Start with factual corrections that are directly contradicted by code, then align version/release metadata across root and module docs, then patch architecture docs and first-success verification steps. Keep edits minimal, exact, and co-located with the affected doc surfaces.

**Tech Stack:** Markdown, Go source references, npm package metadata, shell install script

---

### Task 1: Fix root README factual errors

**Files:**
- Modify: `README.md:100-139`
- Verify against: `pkg/deltascope/audit.go:114-125`
- Verify against: `internal/interfaces/cli/rules.go:17-61`

**Step 1: Update the JSON example field name**

Change the finding example in `README.md` from:

```json
{
  "level": "warning",
  "rule": "ddl.alter.drop.column",
  "message": "dropping column `age` is destructive and cannot be undone"
}
```

To:

```json
{
  "level": "warning",
  "rule_id": "ddl.alter.drop.column",
  "message": "dropping column `age` is destructive and cannot be undone"
}
```

**Step 2: Update the rules listing command**

Change the shipped-rules example in `README.md` from:

```bash
deltascope rules
```

To:

```bash
deltascope rules list
```

**Step 3: Re-read the edited section**

Read `README.md:100-139` and confirm the snippet now matches the public JSON contract and CLI subcommand structure.

**Step 4: Spot-check against code**

Re-read:
- `pkg/deltascope/audit.go:114-125`
- `internal/interfaces/cli/rules.go:17-61`

Expected: field remains `rule_id`; command group remains `rules list`.

**Step 5: Commit**

```bash
git add README.md
git commit -m "docs: fix root readme audit examples"
```

---

### Task 2: Align release/version references across root docs

**Files:**
- Modify: `README.md:12,25-44`
- Modify: `docs/releases/README.md`
- Verify against: `pkg/deltascope/version.go:8-19`
- Verify against: `packages/deltascope-mcp/package.json:1-34`
- Verify against: `docs/releases/release-notes-v0.9.0.md`
- Verify against: `docs/releases/release-notes-v0.9.1.md`
- Verify against: `docs/releases/release-notes-v0.9.2.md`

**Step 1: Decide the documentation versioning rule**

Before editing, choose one rule and apply it consistently:

- **Option A:** treat `v0.9.2` as current and update README / release index / module notes to match
- **Option B:** use version-agnostic wording where possible and avoid claiming strict parity until code metadata is reconciled

Do not invent a new policy. Use only what can be defended from the repository state.

**Step 2: Update the root README release-notes link**

If the current release docs should point to the latest available notes, update the release-notes badge/link in `README.md` so it no longer points at the stale `v0.9.1` page when newer notes exist in-repo.

**Step 3: Update the release contract wording if needed**

If `README.md` claims npm and Go release tags are versioned identically, verify that claim against:
- `pkg/deltascope/version.go`
- `packages/deltascope-mcp/package.json`

Then either:
- make the wording strictly true, or
- remove/soften the claim so it does not overstate current repo alignment.

**Step 4: Refresh the release index**

Update `docs/releases/README.md` to include at least:
- `v0.9.0`
- `v0.9.0 中文版`
- `v0.9.1`
- `v0.9.1 中文版`
- `v0.9.2`
- `v0.9.2 中文版`

Keep the existing older entries.

**Step 5: Re-read the edited files**

Read:
- `README.md`
- `docs/releases/README.md`

Expected: no stale “current release” references remain in those two docs.

**Step 6: Commit**

```bash
git add README.md docs/releases/README.md
git commit -m "docs: align release references"
```

---

### Task 3: Update stale module README version notes

**Files:**
- Modify: `cmd/deltascope-server/README.md:11-16`
- Modify: `cmd/deltascope-mcp/README.md:13-20`
- Modify: `pkg/deltascope/README.md:41-47`
- Verify against: `pkg/deltascope/version.go:8-19`

**Step 1: Replace stale `v0.7.0` statements**

Update the module README notes that currently say source builds default to `v0.7.0`.

Use one of these approaches consistently:
- replace with the actual current default from `pkg/deltascope/version.go`, or
- reword to “defaults to the repository’s current `DefaultVersion`” if you want to reduce future drift.

**Step 2: Keep the module scope narrow**

Do not rewrite unrelated README sections. Only fix the version-related notes that are now inaccurate.

**Step 3: Re-read all three module READMEs**

Read:
- `cmd/deltascope-server/README.md`
- `cmd/deltascope-mcp/README.md`
- `pkg/deltascope/README.md`

Expected: no remaining references to `v0.7.0` unless a deliberate historical note is clearly labeled as historical.

**Step 4: Commit**

```bash
git add cmd/deltascope-server/README.md cmd/deltascope-mcp/README.md pkg/deltascope/README.md
git commit -m "docs: refresh module version notes"
```

---

### Task 4: Patch architecture docs to include MCP consistently

**Files:**
- Modify: `docs/concept/architecture.md:18-65,114-123`
- Modify: `docs/dev/architecture.md:7-47`
- Verify against: `cmd/deltascope-mcp/main.go`
- Verify against: `internal/interfaces/mcp/server.go`

**Step 1: Update the shared audit flow diagram**

In `docs/concept/architecture.md`, update the surface box from:

```text
| CLI / HTTP / Library |
```

To include MCP explicitly.

Also update the output/result section so MCP is represented where appropriate.

**Step 2: Update the layer-boundary block**

In `docs/concept/architecture.md`, update the layer list so it includes:
- `cmd/deltascope-mcp`
- `internal/interfaces/mcp`

**Step 3: Update the implementation architecture diagram**

In `docs/dev/architecture.md`, update the ASCII diagram so it includes:
- `cmd/deltascope-mcp`
- `internal/interfaces/mcp`

Also update the practical-boundaries bullet that currently says `internal/interfaces` owns CLI and HTTP translation only.

**Step 4: Re-read both architecture docs**

Read:
- `docs/concept/architecture.md`
- `docs/dev/architecture.md`

Expected: both docs describe MCP as part of the actual current implementation, not as an omitted or implicit surface.

**Step 5: Commit**

```bash
git add docs/concept/architecture.md docs/dev/architecture.md
git commit -m "docs: include mcp in architecture docs"
```

---

### Task 5: Add first-success verification to CLI docs

**Files:**
- Modify: `README.md:17-45`
- Optional verify against: `internal/interfaces/cli/version.go`
- Optional verify against: `cmd/deltascope/main.go`

**Step 1: Add a minimal verification command after install**

Add a short verification subsection in the root README install area:

```bash
deltascope version
```

**Step 2: Add expected success text**

Document the expected outcome in one sentence, for example:
- prints the DeltaScope semantic version and exits successfully

Do not add troubleshooting trees. Keep it short.

**Step 3: Re-read the install section**

Read the updated install/quick-start area and confirm the verification step appears before longer usage examples.

**Step 4: Commit**

```bash
git add README.md
git commit -m "docs: add cli install verification"
```

---

### Task 6: Add first-success verification to MCP docs

**Files:**
- Modify: `README.md:153-169`
- Modify: `docs/recipe/use-deltascope-mcp.md:22-70`
- Verify against: `internal/interfaces/mcp/server.go`

**Step 1: Add a minimal MCP success check to the root README**

After the MCP registration example, add a short note explaining how the user knows setup worked.

Keep it concrete and surface-level, e.g. the client should show the DeltaScope server and expose these tools:
- `audit_sql`
- `describe_rule`
- `list_rules`
- `get_capabilities`

**Step 2: Add the same verification concept to the recipe doc**

In `docs/recipe/use-deltascope-mcp.md`, add a short “Verify setup” subsection immediately after the launcher/native setup examples.

The subsection should tell the user what to check in the client after registration, without expanding into strategy or product messaging.

**Step 3: Re-read the modified MCP docs**

Read:
- `README.md`
- `docs/recipe/use-deltascope-mcp.md`

Expected: a new user can tell whether the MCP server is actually wired up.

**Step 4: Commit**

```bash
git add README.md docs/recipe/use-deltascope-mcp.md
git commit -m "docs: add mcp setup verification"
```

---

### Task 7: Add first-success verification to skill docs

**Files:**
- Modify: `README.md:171-201`
- Modify: `skills/README.md:37-69`
- Verify against: `skills/deltascope-review/SKILL.md`

**Step 1: Add a minimal skill verification note to the root README**

After the `/deltascope-review` usage example, add one short sentence describing the expected successful behavior, e.g. that the skill accepts SQL or a file path and returns DeltaScope findings.

**Step 2: Add a minimal verification step to `skills/README.md`**

After the install requirement section, add a short verification step such as:

```bash
deltascope version
```

Then keep the existing `/deltascope-review` invocation as the first end-to-end success action.

**Step 3: Check platform wording for consistency**

Review the Windows subsection in `skills/README.md` against:
- `install.sh`
- `packages/deltascope-mcp/lib/releases.js`
- `README.md`

If Windows is not an officially supported platform in the root install/runtime docs, reword the skill README so it does not imply parity without qualification.

**Step 4: Re-read the skill docs**

Read:
- `README.md`
- `skills/README.md`

Expected: the skill surface has a minimal success check and does not overstate platform support.

**Step 5: Commit**

```bash
git add README.md skills/README.md
git commit -m "docs: add skill verification guidance"
```

---

### Task 8: Run the documentation verification loop

**Files:**
- Review: all files changed by Tasks 1-7

**Step 1: Read all changed docs once more**

Re-read every modified file and check only for these failure modes:
- command names do not match code
- JSON field names do not match code
- version references conflict across docs
- MCP is omitted from architecture where it should be explicit
- verification steps are missing or vague

**Step 2: Run targeted grep checks**

Run commands or searches that confirm:
- no stale `"rule":` example remains in the root README
- no stale `deltascope rules` example remains where `rules list` is intended
- no stale `v0.7.0` note remains in the updated module docs

**Step 3: Review diff for scope control**

Confirm the diff only changes:
- factual doc corrections
- release index updates
- small verification additions
- architecture text/diagram alignment

No unrelated copy-edit churn.

**Step 4: Commit final cleanup if needed**

```bash
git add README.md docs/releases/README.md docs/concept/architecture.md docs/dev/architecture.md docs/recipe/use-deltascope-mcp.md skills/README.md cmd/deltascope-server/README.md cmd/deltascope-mcp/README.md pkg/deltascope/README.md
git commit -m "docs: complete remediation pass"
```
