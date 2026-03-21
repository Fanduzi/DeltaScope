# DeltaScope CLI Completion Design

## Goal

Make the CLI a complete first-class product surface for DeltaScope: expose every core audit capability that already exists, add direct metadata-aware access with MySQL-like connection ergonomics, and ship the self-explaining commands needed so users and agents no longer need to inspect source code or README files to understand how to use the tool.

## Success Criteria

- The CLI can drive both offline audit and metadata-aware audit.
- Metadata-aware audit does not require hiding connection details inside a config file.
- The CLI exposes enough catalog and config tooling that users can:
  - inspect available rules,
  - inspect one rule in detail,
  - search rules,
  - lint configs,
  - print default config,
  - inspect current product capabilities.
- The command surface, help text, examples, exit semantics, and output modes feel complete rather than provisional.
- After this milestone, there are no major “already implemented underneath but unavailable from CLI” gaps.

## Command Surface

The completed CLI should expose:

- `deltascope audit`
- `deltascope rules list`
- `deltascope rules show <rule-id>`
- `deltascope rules search <keyword>`
- `deltascope config init`
- `deltascope config lint`
- `deltascope config show-default`
- `deltascope capabilities`
- `deltascope version`
- `deltascope --version`

This keeps `audit` as the primary action, uses `rules` and `config` as natural command groups, and keeps `capabilities` and `version` easy to discover at the top level.

## Audit Modes

### Offline mode

When no database connection parameters are supplied, `audit` behaves as it does today:

- uses SQL input from `--sql`, `--file`, or stdin
- uses policy from defaults or `--config`
- uses `--dialect` as the dialect source

### Metadata-aware mode

When any connection parameters are supplied, `audit` should enrich the same audit engine with live instance facts and target-table snapshots.

The CLI should feel familiar to MySQL users:

- `-h, --host`
- `-P, --port`
- `-u, --user`
- `-p, --password`
- `--ask-password`
- `-D, --schema`
- `-S, --socket`

Rules:

- `--password` and `--ask-password` are mutually exclusive.
- `--socket` conflicts with host/port TCP selection.
- If connection parameters are present, DeltaScope enters metadata-aware mode.
- In metadata-aware mode, DeltaScope auto-detects the dialect from instance facts.
- If the user also passes `--dialect`, DeltaScope verifies that it matches the detected dialect and fails on mismatch.

## Schema Resolution

The CLI should not force users to always pass `--schema`.

Resolution order:

1. If `--schema` is given, use it.
2. Otherwise, try to infer the schema by looking up the target object name across schemas.
3. If the target resolves to exactly one schema, use it and surface that decision in output.
4. If the target exists in multiple schemas, fail and ask the user to pass `--schema`.
5. If the target does not exist anywhere, stay honest:
   - `CREATE TABLE` may continue with partial metadata context because the object is expected not to exist yet.
   - statements that require a real target object should fail rather than pretend metadata was available.

## Rule Catalog Metadata

The existing rule engine is execution-oriented. CLI completion needs an explanation-oriented layer on top of it.

Each shipped rule should have stable catalog metadata such as:

- `rule_id`
- summary
- description
- applies-to statement kinds
- default enabled state
- default level
- default params
- metadata-aware flag
- trigger example
- valid example
- config example
- suggestion hint

This metadata should power:

- `rules list`
- `rules show`
- `rules search`
- future docs and tooling

Rule execution and rule description should stay separate concerns linked by `rule_id`.

## Tooling Commands

### `rules list`

Lists the shipped rules with compact catalog fields and basic filters.

Recommended filters:

- `--level`
- `--kind`
- `--enabled-only`

### `rules show <rule-id>`

Shows:

- core rule metadata
- default config
- whether metadata is required
- a trigger example
- a valid example
- a config example
- brief remediation guidance

### `rules search <keyword>`

Searches rule IDs and summaries so users do not need to scan the full list.

### `config lint`

Validates:

- YAML syntax
- unknown rule keys
- invalid param types
- invalid enum values

### `config show-default`

Prints the built-in default config directly to stdout.

### `capabilities`

Prints a concise summary of:

- supported dialects
- supported input forms
- supported output formats
- offline and metadata-aware modes
- supported metadata facts
- exposed product surfaces such as CLI and HTTP

## UX Completion

The milestone is not just new commands; it is also CLI quality.

Required UX improvements:

- stronger help text and examples for every command
- clear online/offline usage examples in `audit --help`
- better connection and schema-resolution errors
- password prompt support without echo
- explicit indication when metadata-aware mode, schema inference, or dialect auto-detection were used
- quiet and JSON output that stay predictable for shell use and agents

## Out Of Scope

- HTTP service enhancements
- MCP server
- full request-file input model
- `config explain`
- advanced credential stores or secret managers

## Expected Outcome

After this milestone, DeltaScope should have a CLI that feels complete, discoverable, and honest. Users should be able to audit SQL, connect to live MySQL/TiDB metadata when needed, inspect the shipped rule catalog, validate configs, and understand the tool’s capabilities without dropping into repository internals.
