# Design: Release Recovery Provenance Enforcement

Date: 2026-07-30
Status: Proposed

## Decision Drivers

- Recovery can mutate Homebrew and npm without rebuilding release artifacts.
- The selected version is supplied by a human through `workflow_dispatch`.
- Provenance must be evaluated against a remote-tracking main ref, not the
  workflow's default checkout branch.
- Historical releases must not gain an undocumented compatibility bypass.
- The protection must survive future workflow topology changes.

## Model

```
workflow_dispatch(version)
          |
          v
release-recover preflight
  | checkout refs/tags/version, fetch-depth: 0
  | fetch origin main
  | posttag-candidate-gate(RELEASE_MAIN_REF=origin/main)
          |
          +-- fail: no checksum, Homebrew, or npm work
          |
          v
existing recovery checks and outputs
          |
          +-------------------+
          v                   v
publish-homebrew-cask   publish-mcp-launcher-package
```

The recovery preflight reuses `scripts/verify_release_tag_candidate.sh`; it
does not reimplement `.release-candidate` parsing. The recovery workflow must
make the requested tag available locally, fetch remote main explicitly, and
run the verifier against that trusted ref.

## Workflow Boundary

The `preflight` job is the single admission boundary. It must:

1. check out `refs/tags/${{ inputs.version }}` with `fetch-depth: 0`;
2. fetch `origin/main`;
3. set `RELEASE_MAIN_REF=refs/remotes/origin/main` on the same step that runs
   `make posttag-candidate-gate VERSION="${{ inputs.version }}"`; and
4. retain read-only repository permissions.

Existing publisher jobs already depend on `preflight`. That dependency remains
the enforcement edge for Homebrew cask mutation and npm publication. The
recovery provenance checker must parse the `needs` graph and rediscover
publisher jobs from their commands instead of hard-coding a short job list.

The checker must inspect `release-recover.yml` independently from the normal
release checker. Shared parsing helpers are acceptable only when they preserve
per-step ownership: an environment declaration on one step must not satisfy a
post-tag command on another step.

## Compatibility and Failure Semantics

| Condition | Recovery result |
| --- | --- |
| Valid future RC tag on `origin/main` | Continue to existing recovery preflight |
| Tag lacks `.release-candidate` | Fail before checksums or publishers |
| Candidate/file/version/parent mismatch | Fail before checksums or publishers |
| Tag not reachable from `origin/main` | Fail before checksums or publishers |
| `v0.460.0` historical tag | Fail as non-provenance; no implicit exception |
| Dry-run with valid tag | Verify, then preserve existing no-publish behavior |
| Dry-run with invalid tag | Fail; dry-run is not an admission bypass |

The workflow performs no auto-repair. A failed recovery leaves remote tags,
GitHub releases, npm, and Homebrew unchanged. The operator receives the
bounded gate diagnostic and must use a separately approved incident process
for historical recovery.

## Security Boundaries

- `.release-candidate` is provenance metadata, not authorization.
- `origin/main` is an explicit trust anchor for the workflow run.
- A dispatcher-provided version is untrusted until the tag verifier accepts
  it; it must not select a publisher path independently.
- No test may dispatch a real workflow or mutate GitHub, npm, Homebrew, tags,
  or releases. Contract tests use static workflow fixtures; behavior tests use
  temporary Git repositories and local stubs only.

## Observability

The gate emits the existing bounded post-tag diagnostics. Documentation must
state that a provenance failure is expected for historical non-provenance tags
and is not proof that their published artifacts are invalid.
