# DeltaScope v0.12.0 Release Notes

## Overview

DeltaScope `v0.12.0` adds structured naming governance for schema objects. This release keeps the existing offline-first audit path and stable rule contract, then lets teams express naming conventions for tables, columns, indexes, and explicitly named constraints through policy configuration instead of ad hoc review.

## What's Changed

### Structured Naming Governance

You can now enforce naming requirements for `CREATE TABLE` schema objects with built-in rules that support:

- `prefix`
- `suffix`
- `contains`

This governance layer applies to:

- table names
- column names
- index names
- explicitly named constraints

The release keeps the existing identifier legality checks in place, so naming governance complements pattern validation instead of replacing it.

### Policy-Aware Constraint Coverage

Constraint naming governance follows the same policy model as the rest of DeltaScope:

- foreign key naming checks only apply when foreign keys are allowed by policy
- the shipped default `ddl.table.foreign_key.forbid` baseline still suppresses foreign key naming governance unless teams explicitly opt in

### Docs, Examples, and Coverage

Release-facing materials now show naming governance as a first-class workflow:

- config examples demonstrate how to require naming prefixes, suffixes, and contains rules
- landing and README surfaces now point to `v0.12.0`
- application-layer and CLI coverage now includes config-driven naming governance findings end to end

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
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.12.0/install.sh | \
  DELTASCOPE_VERSION=v0.12.0 sh
```

## Compatibility

No breaking changes. `v0.12.0` adds configurable naming governance on top of the existing audit contract, while preserving the stable CLI, HTTP, MCP, and Go library surfaces.
