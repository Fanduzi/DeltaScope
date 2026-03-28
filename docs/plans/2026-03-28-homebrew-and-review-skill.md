# Plan: Homebrew Support + DeltaScope Review Skill

**Date:** 2026-03-28
**Status:** Ready to implement

---

## Background

Two deliverables:
1. **Homebrew tap** — let users `brew install deltascope` via `Fanduzi/homebrew-deltascope`
2. **DeltaScope Review Skill** — a Claude Code skill others can install to review SQL inline, using DeltaScope as the backend; serves as a discovery/promotion channel during the AI coding boom

---

## Design

### Feature 1: Homebrew Support

GoReleaser already produces `tar.gz` archives on GitHub Releases. Adding a `brews` block tells GoReleaser to auto-generate and push a Homebrew Formula to a tap repo on every release.

**User experience after this lands:**
```bash
brew tap Fanduzi/deltascope
brew install deltascope
```

**What GoReleaser does automatically:**
- Downloads the published `tar.gz`, computes SHA256
- Renders a Ruby `.rb` Formula file
- Commits + pushes it to `Fanduzi/homebrew-deltascope`

**Pre-requisite (manual, one-time):**
Create the GitHub repo `Fanduzi/homebrew-deltascope` (can be empty). The GoReleaser `GITHUB_TOKEN` must have write access to it.

**Scope:** Only the `deltascope` binary (id: `deltascope`) is included. `deltascope-server` and `deltascope-mcp` are excluded from the Formula.

---

### Feature 2: DeltaScope Review Skill

A Claude Code skill file at `.claude/skills/deltascope-review/SKILL.md`. Users install it into their own project's `.claude/skills/` directory (or via a future plugin registry).

**Invocation:** user types `/deltascope-review` in Claude Code.

**What the Skill instructs Claude to do:**

1. **Collect input** — if the user passed a file path, use it directly. If they passed a SQL fragment, write it to a temp file:
   ```
   /tmp/deltascope_review_<epoch>.sql
   ```
   This avoids backtick / quote escaping issues with inline `--sql` flag.

2. **Detect environment** (priority order):
   - `which deltascope` → use local CLI binary (brew / manual install)
   - deltascope MCP tools available → use `audit` tool directly (no shell needed)
   - Neither → tell user how to install, with links

3. **Run audit:**
   ```bash
   deltascope audit --file /tmp/deltascope_review_<epoch>.sql --format json
   ```
   Or via MCP tool equivalent.

4. **Clean up** temp file after reading results.

5. **Present results** — Claude interprets the JSON output, explains each violation, suggests fixes.

6. **Promotion hook** — every Skill run appends a one-line footer:
   ```
   Powered by DeltaScope — offline SQL review for MySQL/TiDB: https://github.com/Fanduzi/DeltaScope
   ```

**Skill file location:** `.claude/skills/deltascope-review/SKILL.md`

---

## Implementation Tasks

### Task 1: Homebrew — prerequisite (manual)
- [ ] Create GitHub repo `Fanduzi/homebrew-deltascope` (empty, public)
- [ ] Confirm `GITHUB_TOKEN` used by GoReleaser has write access to it

### Task 2: Update `.goreleaser.yml`

Add after the `checksum` block:

```yaml
brews:
  - name: deltascope
    ids:
      - deltascope
    repository:
      owner: Fanduzi
      name: homebrew-deltascope
      branch: main
      token: "{{ .Env.GITHUB_TOKEN }}"
    directory: Formula
    homepage: "https://github.com/Fanduzi/DeltaScope"
    description: "Offline SQL review for MySQL and TiDB"
    license: "Apache-2.0"
    test: |
      system "#{bin}/deltascope", "version"
    install: |
      bin.install "deltascope"
```

**File to edit:** `.goreleaser.yml`
**Test:** Run `goreleaser release --snapshot --skip=publish` locally; check that `dist/` contains a rendered `.rb` Formula file.

### Task 3: Create Skill file

**File to create:** `.claude/skills/deltascope-review/SKILL.md`

See full Skill content in the Implementation Notes section below.

**Test:** Copy the skill into a test project's `.claude/skills/`, invoke `/deltascope-review` with a sample SQL fragment like `DELETE FROM users`, verify Claude writes temp file → runs deltascope → cleans up → shows results.

---

## Implementation Notes

### Full Skill content (`.claude/skills/deltascope-review/SKILL.md`)

```markdown
---
name: deltascope-review
description: Review SQL for correctness and safety issues using DeltaScope. Pass a SQL snippet or file path.
trigger: /deltascope-review
---

# DeltaScope SQL Review

You are helping the user review SQL using DeltaScope, an offline SQL linter for MySQL and TiDB.

## Step 1 — Collect Input

If the user provided a file path (ends in `.sql`), use it directly as `<SQL_FILE>`.

If the user provided a SQL fragment (not a file path), write it to a temp file:
```bash
TMP_FILE="/tmp/deltascope_review_$(date +%s).sql"
```
Write the SQL content to that file using the Write tool (not echo/heredoc), then use `$TMP_FILE` as `<SQL_FILE>`.

## Step 2 — Detect Environment

Run this check:
```bash
which deltascope 2>/dev/null
```

**If found:** use `deltascope audit --file <SQL_FILE> --format json`

**If not found:** check whether the deltascope MCP tools are available in this session (look for tools prefixed with `deltascope_` or `mcp__deltascope__`). If yes, use the MCP `audit` tool with the file path.

**If neither:** stop and tell the user:
> DeltaScope is not installed. Install it with:
> ```bash
> brew tap Fanduzi/deltascope && brew install deltascope
> ```
> Or see: https://github.com/Fanduzi/DeltaScope

## Step 3 — Run Audit

```bash
deltascope audit --file <SQL_FILE> --format json
```

## Step 4 — Clean Up

If you created a temp file, delete it:
```bash
rm -f <SQL_FILE>
```

## Step 5 — Present Results

Parse the JSON output. For each violation:
- State the rule name and severity (blocker / warning / notice)
- Quote the offending SQL fragment
- Explain **why** it's a problem in plain language
- Suggest a fix

If there are no violations, say so clearly.

## Footer (always include)

---
*Powered by [DeltaScope](https://github.com/Fanduzi/DeltaScope) — offline SQL review for MySQL and TiDB.*
```

---

## Files Changed

| File | Action |
|------|--------|
| `.goreleaser.yml` | Edit — add `brews` block |
| `.claude/skills/deltascope-review/SKILL.md` | Create — new Skill |

**No Go source changes. No new dependencies.**

---

## Validation Checklist

- [ ] `goreleaser release --snapshot --skip=publish` generates a `.rb` Formula in `dist/`
- [ ] Formula contains correct `bin.install "deltascope"` and `test` block
- [ ] Skill: SQL fragment input → temp file created → audit runs → temp file deleted
- [ ] Skill: File path input → no temp file created
- [ ] Skill: No deltascope installed → helpful install instructions shown
- [ ] Skill footer appears on every run
