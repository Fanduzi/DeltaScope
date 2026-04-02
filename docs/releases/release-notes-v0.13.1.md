# DeltaScope v0.13.1 Release Notes

## Overview

DeltaScope `v0.13.1` is a patch release for the landing page. It fixes a JavaScript syntax error introduced in `v0.13.0` so the homepage can render and switch localized content correctly again.

## What's Changed

### Landing Page JavaScript Hotfix

The landing page examples for DDL and CI output embedded SQL strings inside the inline i18n configuration object. In `v0.13.0`, those strings included unescaped single quotes, which caused browsers to fail with:

- `Uncaught SyntaxError: Unexpected string`

This patch release escapes the affected SQL examples in both English and Chinese content blocks so the homepage script parses correctly again.

### No Product Contract Changes

This release does not change:

- the CLI audit behavior
- the HTTP metadata-aware audit contract
- the MCP launcher behavior
- the public Go API

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
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.13.1/install.sh | \
  DELTASCOPE_VERSION=v0.13.1 sh
```

## Compatibility

No breaking changes. `v0.13.1` is a documentation and website hotfix release on top of `v0.13.0`.
