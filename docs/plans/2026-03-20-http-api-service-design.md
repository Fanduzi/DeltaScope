# DeltaScope HTTP API Service Design

## Goal

Expose the existing library-first audit engine as a long-running HTTP service without disturbing the core parser-neutral audit flow.

## Current State

`DeltaScope` already has:

- stable library entrypoint through `pkg/deltascope`
- stable policy loading and config initialization
- Markdown and JSON output renderers
- a CLI-first workflow

The next service layer should adapt those capabilities, not re-implement them.

## Recommended Direction

Build a thin HTTP adapter that depends on the same application/library core used by the CLI.

### Required service responsibilities

- accept SQL and audit options over HTTP
- load and hot-reload config through the existing Viper-backed path
- return structured JSON results
- expose health and version endpoints

### Keep out of scope

- auth and multi-tenancy
- live database metadata checks
- distributed policy management
- MCP protocol handling

## Proposed Shape

- `internal/interfaces/http`
  - request binding
  - response rendering
  - error mapping
- `internal/application`
  - reuse the same service/use-case layer already called by CLI
- `cmd/deltascope-server`
  - thin process entrypoint

## Expected Outcome

After this milestone, `DeltaScope` should run as a small service that exposes the same offline audit engine over JSON APIs. That should make the MCP-server milestone an adapter problem, not a core-audit rewrite.
