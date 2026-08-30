# Decision: npm Publication Independent of Homebrew Channel

Date: 2026-08-30
Status: Accepted
Related milestone/version: Issue #54
Related commits: (will be filled after commit)
Related tests: scripts/test_verify_release_workflow_provenance.py, scripts/test_verify_release_workflow_provenance_negative.py
Related docs: docs/dev/release-recovery.md, scripts/README.md

## Context

v0.500.0 published GitHub Release assets while `@fanduzi/deltascope-mcp` latest
remained `0.490.0`. The documented unpinned `npx -y @fanduzi/deltascope-mcp`
path therefore served an older MCP launcher than the GitHub tarball.

Triage on issue #54 classified this as a release-channel blocker, not a
product-source regression. Immediate package recovery still requires human npm
Trusted Publisher / package permission work. The durable defect in the
tag-triggered workflow was that `publish-mcp-launcher-package` listed
`verify-homebrew-cask-install` in `needs`, so a Homebrew publish or install
verification failure skipped npm for that tag.

## Decision

Keep Homebrew publish and Homebrew install verification as independent release
jobs. `publish-mcp-launcher-package` in `.github/workflows/release.yml` waits
only on the platform-build jobs (`release-linux`, `release-linux-arm64`,
`release-macos-arm64`, `release-macos-amd64`) and, transitively, on
`provenance`. It does not wait on `publish-homebrew-cask` or
`verify-homebrew-cask-install`.

The recovery workflow remains independently selectable per channel. The
existing release-workflow provenance DAG checker enforces this split so a
later workflow edit cannot quietly recouple npm to Homebrew.

## Rationale

GitHub Actions skips a job when a needed job fails. Coupling npm to Homebrew
verification therefore drops one public install path because a different
install path failed. The two channels share GitHub Release assets, not each
other's tap or registry credentials. Provenance and platform builds remain
prerequisites because npm publication still consumes the tagged, asset-backed
release, not because Homebrew succeeded.

Leaving npm blocked on Homebrew would repeat the v0.500.0 channel split on
every future tap or cask-install failure.

## Public Contract

- A future `v*` tag publishes `@fanduzi/deltascope-mcp` even if Homebrew cask
  publish or install verification fails, provided provenance and platform
  builds succeed and npm Trusted Publishing is authorized.
- Homebrew publish and install verification continue to run; they no longer
  gate npm.
- npm publication still uses `npm publish --access public --provenance`.
- `make release-workflow-hygiene-gates` rejects a workflow graph that puts
  Homebrew jobs on the npm path or drops the platform-build / provenance
  prerequisites. The live `release.yml` invocation of
  `scripts/test_verify_release_workflow_provenance.py` is the regression gate.

## Deferred / Out Of Scope

- Publishing `@fanduzi/deltascope-mcp@0.500.0` or any later version from this
  change. That remains a human npm auth recovery.
- Changing npm Trusted Publisher configuration, `NPM_TOKEN`, or other secrets.
- Pinning README / skill / `mcp.json` examples to a specific npm version.
- Replacing, moving, or republishing the existing v0.500.0 GitHub Release.
- Product, parser, CLI, HTTP, MCP source, or version-surface changes.

## Verification Evidence

- `python3 scripts/test_verify_release_workflow_provenance.py` — live
  `release.yml` graph: npm job exists, directly needs the four platform-build
  jobs, is downstream of provenance, and is not downstream of Homebrew jobs.
- `python3 scripts/test_verify_release_workflow_provenance_negative.py` —
  existing adversarial fixtures, with the positive-control tail matching the
  decoupled npm needs list.
- `make release-workflow-hygiene-gates` — load-bearing gate invocation.

## Consequences

- A failed Homebrew job no longer implies npm was skipped. Operators should
  inspect each channel separately and recover only the failed one.
- The provenance DAG checker must be updated if npm job names or required
  platform jobs change.
- Historical v0.500.0 npm absence is not repaired by this workflow change.

## Links

- Commits: (will be filled after commit)
- Tests: scripts/test_verify_release_workflow_provenance.py, scripts/test_verify_release_workflow_provenance_negative.py
- Docs: docs/dev/release-recovery.md, scripts/README.md
- Issue: https://github.com/Fanduzi/DeltaScope/issues/54
