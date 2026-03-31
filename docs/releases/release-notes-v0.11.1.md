# DeltaScope v0.11.1 Release Notes

## Overview

DeltaScope `v0.11.1` focuses on first-run usability. This patch release makes installation choices clearer, improves terminal onboarding in the CLI, and aligns the homepage and reference docs with the actual install and rule-list experience.

## What's Changed

### Install Experience: Homebrew-first on macOS

The top-level install guidance now makes the platform split explicit:

- **macOS:** install with Homebrew
- **Linux / other environments:** use the portable `install.sh`

This guidance is now consistent across the README, landing page hero, and release-facing docs.

### Portable Installer: Safer, More Predictable Defaults

`install.sh` now improves operator control during interactive installs:

- Defaults to installing only `deltascope`
- Prompts interactive users to choose which binaries to install
- Prompts for the install directory before copying binaries
- Prints an install summary before downloading artifacts
- Warns before invoking `sudo`
- Skips `sudo` entirely when already running as `root`
- Preserves compatibility for older releases that do not publish `deltascope-mcp`

### CLI: Better First-Run Terminal UX

Two CLI flows are now clearer in direct terminal use:

- `deltascope audit` prints:

```text
Waiting for SQL from stdin. Press Ctrl+D to finish.
```

  before reading pasted SQL from an interactive terminal.

- `deltascope rules list` and `deltascope rules search` now render rules as an aligned ASCII table instead of a Markdown bullet list.

Example:

```text
# DeltaScope Rules

RULE ID                     LEVEL    KIND  SUMMARY
--------------------------  -------  ----  ----------------------------------------
ddl.table.comment.require  warning  ddl   Require DDL table comment require
dml.where.require          blocker  dml   Require DML where require
```

### Documentation Alignment

English and Chinese reference docs now match the current CLI output contract for rule discovery commands.

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

**Linux / other environments:**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.11.1/install.sh | \
  DELTASCOPE_VERSION=v0.11.1 sh
```

## Compatibility

No breaking changes. This release improves install and CLI usability without changing the core audit contract, shipped rule IDs, or API surfaces.
