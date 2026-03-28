# DeltaScope Skills

Claude Code skills that wrap DeltaScope for use inside AI coding sessions.

## deltascope-review

Review SQL snippets or files for safety and quality issues using DeltaScope.

### Install

**Option 1 — `npx skills` (recommended, works with Claude Code, Codex, Cursor and 40+ AI agents)**

```bash
npx skills add Fanduzi/DeltaScope --skill deltascope-review -a claude-code
```

Install globally (available across all projects):

```bash
npx skills add Fanduzi/DeltaScope --skill deltascope-review -a claude-code -g
```

**Option 2 — `curl`**

```bash
mkdir -p ~/.claude/skills/deltascope-review
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/main/skills/deltascope-review/SKILL.md \
  -o ~/.claude/skills/deltascope-review/SKILL.md
```

### Requirement

The skill calls the local `deltascope` binary. Install it first:

```bash
# macOS
brew tap Fanduzi/deltascope
brew install --cask deltascope

# Linux / manual
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/main/install.sh | sh
```

### Usage

In any Claude Code session, invoke the skill:

```
/deltascope-review
```

Then either:
- Paste a SQL snippet and ask Claude to review it
- Provide a file path: `review ./migrations/20260328_add_users.sql`

### What it checks

DeltaScope runs offline against the [shipped rule set](../docs/reference/README.md). Findings are classified as `blocker`, `warning`, or `notice`. Claude interprets the JSON output and suggests fixes.

### Example

```
/deltascope-review

Review this migration:
ALTER TABLE orders ADD COLUMN status VARCHAR(32) NOT NULL;
```

Claude will write the SQL to a temp file, run `deltascope audit`, and return a structured finding report with suggested fixes.
