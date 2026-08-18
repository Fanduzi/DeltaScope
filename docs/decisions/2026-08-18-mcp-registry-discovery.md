# Decision: Add MCP Registry and Repository Discovery Metadata

- Date: 2026-08-18
- Status: Proposed
- Pull request: [#27](https://github.com/Fanduzi/DeltaScope/pull/27)

## Context

DeltaScope already publishes the `@fanduzi/deltascope-mcp` stdio launcher, but
the repository has no official MCP Registry metadata or repository-root MCP
configuration for catalogs that discover one automatically.

The official registry also verifies the namespace against an ownership marker
in the published npm package. The current `0.480.0` package predates that
marker, so repository metadata can land now while registry publication must
wait for the next release.

## Decision

- Reserve `io.github.fanduzi/deltascope` as the registry name in `server.json`
  and the npm package's `mcpName`.
- Add a repository-root `.mcp.json` that launches the existing package with
  `npx -y @fanduzi/deltascope-mcp` over stdio.
- Keep registry and npm package versions synchronized through the existing
  release-version-surface gate.
- Defer `mcp-publisher publish` until a newly published npm package contains
  the ownership marker.

## Boundaries

This metadata does not add an MCP tool, change transport behavior, modify audit
results, publish a package, or register the server. Registry login and publish
remain explicit release operations.

## Acceptance Criteria

This decision remains Proposed until the official publisher validates
`server.json`, npm/docs/release gates pass, the refreshed pull-request CI is
green, and review finds no unresolved P0, P1, or P2.
