# DeltaScope v0.9.1 Release Notes

## Overview

DeltaScope `v0.9.1` is a patch release that fixes the CI release pipeline broken in v0.9.0, ensuring the Homebrew Cask and GitHub Release artifacts are properly published.

## What's Fixed

### CI release pipeline (`npm publish --dry-run` false failure)

The `Verify MCP launcher package contract` step in the release workflow called `npm publish --dry-run`, which fails with an error when the version has already been published to the npm registry — even in dry-run mode. This caused the v0.9.0 CI run to fail after tagging, which meant GoReleaser never ran and the Homebrew Cask was not updated.

Fixed by removing the redundant `npm publish --dry-run` call. Package contents are already validated by `npm pack --dry-run`.

## Install / Upgrade

**macOS (recommended):**

```bash
brew tap Fanduzi/deltascope
brew install --cask deltascope
```

**Linux / manual:**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.9.1/install.sh | \
  DELTASCOPE_VERSION=v0.9.1 sh
```

**MCP launcher (no install required):**

```bash
npx -y @fanduzi/deltascope-mcp --version
```

## Compatibility

No behavior changes. All compatibility notes from [v0.9.0](release-notes-v0.9.0.md) apply.
