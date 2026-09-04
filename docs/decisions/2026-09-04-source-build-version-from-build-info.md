# Decision: Report VCS Build Info for Untagged Binaries

Date: 2026-09-04
Status: Accepted
Related milestone/version: issue #70
Related commits:
Related tests: `pkg/deltascope/version_test.go`, `internal/interfaces/cli/version_test.go`
Related docs: `pkg/deltascope/README.md`, `docs/reference/cli.md`

## Context

`DefaultVersion` is bumped to the last release tag (currently `v0.510.3`).
GoReleaser ldflags inject that tag into CLI, server, and MCP `Version` only
for tagged release archives. A binary built from source or
`go install ...@main` after further commits still printed `DefaultVersion`,
so it claimed the last shipped release as if it were that release.

## Decision

Prefer `runtime/debug.ReadBuildInfo` for the public version string.

- Tagged and pseudo-version module builds report that module version.
- Devel source builds report `devel-<rev>` or `devel-<rev>-dirty` from
  `vcs.revision` / `vcs.modified`.
- `DefaultVersion` is used only when build information is absent.
- Release ldflags still override `Version` to the release tag, so tagged
  archives keep printing the release version.

CLI, HTTP `-version` / `GET /version`, and MCP `-version` share this helper.
Library consumers who need the release baseline keep using `DefaultVersion`.

## Rationale

Go already embeds module version and VCS metadata in the binary. Using that
information distinguishes `go install @main`, a dirty local `go build`, and a
release archive without parsing GoReleaser internals or changing npm/Homebrew
version surfaces.

Keeping `DefaultVersion` as a constant fallback preserves the release-surface
gate that aligns the constant with the tag, while stopping untagged binaries
from treating that constant as their identity.

## Public Contract

- Release ldflags / tagged archives still print the release tag.
- Untagged, `(devel)`, and pseudo-version builds do not print the last
  release tag as the sole version.
- `ReportedVersion()` is the exported helper for this process version.
- `DefaultVersion` remains the last-release constant and the fallback when
  build info is missing.

## Deferred / Out Of Scope

- Changing npm or Homebrew version strings to Go pseudo-versions.
- Rebranding Official Distribution versus source builds beyond version text.
- Replacing GoReleaser ldflags with VCS ldflags on tagged releases.

## Verification Evidence

- `pkg/deltascope/version_test.go` covers absent build info, tagged versions,
  pseudo-versions, devel revision/dirty, dependency lookup, and the live test
  binary.
- `internal/interfaces/cli/version_test.go` covers ldflags override versus
  empty `Version` on `version` and `--version`.
- Server and MCP entrypoint tests cover the same override/fallback seam.

## Consequences

Source-build version strings are no longer a stable `vX.Y.Z`. Scripts that
compared `deltascope --version` to `DefaultVersion` on untagged binaries must
accept module pseudo-versions or `devel-*`. Release archives are unchanged.

## Links

- Commits:
- Tests: `pkg/deltascope/version_test.go`, `internal/interfaces/cli/version_test.go`, `cmd/deltascope-server/main_test.go`, `cmd/deltascope-mcp/main_test.go`
- Docs: `pkg/deltascope/README.md`, `docs/reference/cli.md`, `docs/reference/cli.zh-CN.md`
