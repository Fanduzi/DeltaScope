# DeltaScope v0.9.0 Release Notes

## Overview

DeltaScope `v0.9.0` introduces Homebrew Cask distribution and a Claude Code Skill for inline SQL review, making DeltaScope easier to install and use inside AI coding sessions.

## Highlights

- **Homebrew Cask** — install DeltaScope on macOS with a single `brew` command
- **Claude Code Skill** — review SQL directly in Claude Code, Codex, Cursor and 40+ AI agents via `npx skills`

## What's New

### Homebrew Cask distribution

DeltaScope is now available as a Homebrew Cask:

```bash
brew tap Fanduzi/deltascope
brew install --cask deltascope
```

This is the recommended install path for macOS users. The Cask is published automatically to [Fanduzi/homebrew-deltascope](https://github.com/Fanduzi/homebrew-deltascope) on each release via GoReleaser.

### Claude Code Skill — `deltascope-review`

A new Claude Code Skill lets you audit SQL snippets or migration files without leaving your AI coding session:

```bash
# Install via npx skills (Claude Code, Codex, Cursor and 40+ agents)
npx skills add Fanduzi/DeltaScope --skill deltascope-review -a claude-code
```

Then in any Claude Code session:

```
/deltascope-review
```

Paste a SQL snippet or point to a file — Claude writes it to a temp file (avoiding shell-escaping issues with backticks and quotes), runs `deltascope audit`, and returns a structured finding report with fix suggestions.

See [skills/README.md](../../skills/README.md) for full setup and usage.

## Install / Upgrade

**macOS (recommended):**

```bash
brew tap Fanduzi/deltascope
brew install --cask deltascope
```

**Linux / manual:**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.9.0/install.sh | \
  DELTASCOPE_VERSION=v0.9.0 sh
```

**MCP launcher (no install required):**

```bash
npx -y @fanduzi/deltascope-mcp --version
```

## Compatibility

- Supported native targets: `darwin`, `linux`
- Supported architectures: `amd64`, `arm64`
- Supported database dialects: `MySQL`, `TiDB`
- Claude Code Skill requires: local `deltascope` binary (brew or manual install)
