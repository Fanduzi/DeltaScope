# DeltaScope v0.9.2 Release Notes

## Overview

DeltaScope `v0.9.2` is a patch release with documentation and AI agent skill improvements. No binary changes.

## What's Changed

### AI Agent Skill: multi-platform install guidance

When the `deltascope` binary is not found, the skill now provides OS-specific install commands instead of a generic link:

- **macOS** — Homebrew (recommended): `brew tap Fanduzi/deltascope && brew install --cask deltascope`
- **Linux** — curl installer to `~/.local/bin`, no sudo required
- **Windows** — PowerShell one-liner that downloads the latest release from GitHub

All commands are shown to the user for review — nothing is installed silently.

### AI Agent Skill: keep up to date with `npx skills update`

`skills/README.md`, `README.md`, and `README_ZH.md` now document the `npx skills update` command so users know how to pull the latest skill revision after install.

### Documentation: AI Agent Skill section improvements

- Restored Quick Start examples and Release Contract section
- Renamed "Skill" section to "AI Agent Skill" for clarity
- Updated `skills/README.md` to reflect support for universal AI agents (Claude Code, Codex, Cursor, and 40+ others)

## Install / Upgrade

**macOS (recommended):**

```bash
brew tap Fanduzi/deltascope
brew install --cask deltascope
```

Or upgrade:

```bash
brew upgrade --cask deltascope
```

**Linux:**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.9.2/install.sh | \
  DELTASCOPE_VERSION=v0.9.2 sh
```

**MCP launcher (no install required):**

```bash
npx -y @fanduzi/deltascope-mcp --version
```

## Compatibility

No behavior changes. All compatibility notes from [v0.9.1](release-notes-v0.9.1.md) apply.
