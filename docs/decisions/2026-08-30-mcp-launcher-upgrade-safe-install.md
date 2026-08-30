# Decision: Upgrade-Safe MCP Launcher Install

Date: 2026-08-30
Status: Accepted
Related milestone/version: Issue #61
Related commits: Issue #61 implementation commit
Related tests: `scripts/test_verify_docs_examples.py`, `make docs-example-gates`
Related docs: `README.md`, `README_ZH.md`, `docs/recipe/use-deltascope-mcp.md`, `docs/recipe/use-deltascope-mcp.zh-CN.md`, `packages/deltascope-mcp/README.md`, `docs/landing/`, `.mcp.json`

## Context

The npm MCP launcher is a public installation path. An unversioned npm exec
request can reuse cached package metadata, leaving a user on an older launcher
even after a newer package is published. The active canonical docs and repo
configuration need one upgrade-safe install contract without changing the
launcher runtime or its supported Node version.

## Decision

Use the exact launcher command
`npx -y --prefer-online @fanduzi/deltascope-mcp@latest` on every active
canonical launcher surface. In structured MCP configuration, represent the
same command as `command: "npx"` with args `-y`, `--prefer-online`, and
`@fanduzi/deltascope-mcp@latest`.

Enforce the contract with the curated static docs guard in
`scripts/verify_docs_examples.py`. The guard covers the tracked English and
Chinese root READMEs, current MCP recipes, npm package README, current landing
pages, and repo-root `.mcp.json`. Historical release notes and decision text
remain outside this active-surface check.

## Rationale

`--prefer-online` asks npm to refresh package metadata, while `@latest`
selects the published latest dist-tag. Together they address stale npm exec
resolution with the smallest change to the documented public path. Keeping
the native launcher behavior, package engine requirement, and version override
environment unchanged avoids changing the binary download or runtime
contracts.

## Public Contract

- Active canonical install examples use exactly
  `npx -y --prefer-online @fanduzi/deltascope-mcp@latest`.
- The repo-root MCP config launches `npx` with the matching three arguments.
- Historical release notes and decision records are retained as written.
- The npm launcher continues to require Node.js 24 or newer and its runtime
  implementation is unchanged.

## Deferred / Out Of Scope

- Rewriting historical release notes or historical decision records.
- Changing `packages/deltascope-mcp` engine metadata or launcher runtime code.
- Adding an old-release fallback or making the launcher resolve a custom
  version through a second public path.
- Replacing the curated repository guard with a broad, unbounded documentation
  crawler.

## Verification Evidence

The active-surface guard fails closed on an unpinned invocation and validates
the structured `.mcp.json` command/args. The repository regression suite passes
83 tests, `make docs-example-gates VERSION=v0.510.1` passes, the npm launcher
suite passes 15 tests, `make test` passes, `make build` passes, and the
decision-record and landing syntax gates pass.

## Consequences

Future active launcher examples must preserve both online metadata refresh and
the `latest` dist-tag. Any intentionally different launcher contract must add
or update a decision record and the curated guard in the same change.

## Links

- Commits: Issue #61 implementation commit
- Tests: `scripts/test_verify_docs_examples.py`, `make docs-example-gates`
- Docs: `README.md`, `README_ZH.md`, `docs/recipe/use-deltascope-mcp.md`, `docs/recipe/use-deltascope-mcp.zh-CN.md`, `.mcp.json`
