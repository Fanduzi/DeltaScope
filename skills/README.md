# DeltaScope Skills

Universal AI agent skills that wrap DeltaScope for inline SQL review inside AI coding sessions. Works with Claude Code, Codex, Cursor, and 40+ AI agents.

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

**Keep up to date:**

```bash
npx skills update
```

**Option 2 — `curl`**

```bash
mkdir -p ~/.claude/skills/deltascope-review
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/main/skills/deltascope-review/SKILL.md \
  -o ~/.claude/skills/deltascope-review/SKILL.md
```

### Requirement

The skill calls the local `deltascope` binary. Install it first:

**macOS (recommended):**
```bash
brew tap Fanduzi/deltascope && brew install --cask deltascope
```

**Linux (installs to `~/.local/bin`, no sudo needed):**
```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/main/install.sh | DELTASCOPE_INSTALL_DIR="$HOME/.local/bin" sh
```
Ensure `~/.local/bin` is in your `PATH` after install.

Native release installs are currently documented for `darwin` and `linux` only. If you are on Windows, use WSL or another supported environment for the local `deltascope` binary.

### Usage

In any supported AI session (Claude Code, Codex, Cursor, etc.), invoke the skill:

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

The AI agent will write the SQL to a temp file, run `deltascope audit`, and return a structured finding report with suggested fixes.
