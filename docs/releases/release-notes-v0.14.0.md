# DeltaScope v0.14.0 Release Notes

Release date: 2026-04-03

## Overview

DeltaScope `v0.14.0` adds conservative DML impact estimation for `UPDATE` and `DELETE`. The engine can now attach a statement-level `impact` object with estimated rows, estimated ratio, risk level, confidence, source, reason codes, and notes across CLI, HTTP, MCP, and the public Go API.

## What's Changed

### DML Impact Estimation

DeltaScope now produces additive DML impact facts before rule evaluation:

- `shape` mode estimates risk from SQL structure alone
- `metadata` mode refines the estimate with read-only table statistics
- statement results expose:
  - `estimated_rows`
  - `estimated_ratio`
  - `risk_level`
  - `confidence`
  - `source`
  - `reason_codes`
  - `notes`

### New DML Impact Rules

This release adds a new DML impact rule family:

- `dml.impact.estimate`
- `dml.impact.rows.max_count`
- `dml.impact.ratio.max_percent`

These rules let teams gate risky DML by conservative estimated row count or affected-table ratio without executing the DML itself.

### Output And Documentation Updates

The shared `impact` contract now appears consistently in:

- Markdown output
- CLI JSON
- HTTP responses
- MCP structured results
- `pkg/deltascope`

The reference docs, capability matrix, and module READMEs were updated accordingly.

### Build And Release Pipeline Hardening

This release also includes release-path improvements shipped since `v0.13.1`:

- default local Go binaries now build with `CGO_ENABLED=0`
- CI verifies Linux binaries are statically linked
- release automation uses `goreleaser-action@v7`
- a manual release smoke workflow was added

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
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.14.0/install.sh | \
  DELTASCOPE_VERSION=v0.14.0 sh
```

## Compatibility

No breaking changes. `v0.14.0` is a backward-compatible feature release on top of `v0.13.1`.
