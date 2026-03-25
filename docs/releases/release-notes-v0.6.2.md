# DeltaScope v0.6.2 Release Notes

## Overview

DeltaScope `v0.6.2` focuses on explainable audit results across the public product surface. It adds structured explanations to findings and aggregate results, aligns CLI/HTTP/library outputs, and updates the English and Chinese documentation so the shipped contracts match runtime behavior.

## Highlights

- Structured result explanations for audit summaries, statements, and findings
- Stable public/library and HTTP output updates for explainable findings
- Markdown renderer upgrades for richer CLI output
- Bilingual doc refresh aligned with current runtime contracts
- Patch release that keeps the existing installation and release flow intact

## What’s New

### Explainable audit results

- audit results now include aggregate `explanation` blocks at the result and statement levels
- findings can now carry structured explanation fields such as `summary`, `why`, `risk`, and `suggestion`
- metadata-aware findings can now expose metadata-availability notes through structured explanation metadata

### Public surface alignment

- `pkg/deltascope` now maps the richer explanation model into the stable public API
- the HTTP adapter now returns the updated audit result shape consistently
- the Markdown CLI renderer now prints explanation details directly in human-facing output

### Rule catalog and discoverability

- shipped rule catalog entries now carry richer explanation-oriented metadata
- `rules list`, `rules show`, and related documentation now reflect the current output contract more accurately
- rule examples and metadata-aware notes were aligned with the cataloged runtime output

### Documentation refresh

- English and Chinese README, recipe, and reference pages were updated together
- docs now reflect `omitempty` behavior, verdict semantics, explanation fields, and localized cross-links consistently
- release-facing install snippets now point at `v0.6.2`

## Install / Upgrade

Install the latest release:

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/main/install.sh | sh
```

Install this exact release:

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.6.2/install.sh | \
  DELTASCOPE_VERSION=v0.6.2 sh
```

## Compatibility

- Supported OS targets: `darwin`, `linux`
- Supported architectures: `amd64`, `arm64`
- Supported database dialects: `MySQL`, `TiDB`

## Known Limitations

- metadata-aware live checks still depend on live schema access and are not available in offline mode
- release examples document current shipped behavior and may evolve again in future minor releases as the public surface expands
- MCP Server remains outside this release line
