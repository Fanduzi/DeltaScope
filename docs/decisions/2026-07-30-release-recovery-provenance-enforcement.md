# Decision: Release Recovery Provenance Enforcement

Date: 2026-07-30
Status: Proposed
Related decision: docs/decisions/2026-07-29-release-candidate-provenance-enforcement.md
Related workflows: .github/workflows/release-recover.yml, .github/workflows/release.yml

## Context

The normal release workflow now rejects invalid release-candidate provenance
before it can build assets or publish downstream packages. The manually
dispatched recovery workflow remains an independent mutation path: it can
update Homebrew and publish an absent npm launcher package for an input
version. Its current release preflight validates assets and package state, but
does not prove that the selected tag is an approved release-candidate commit.

This leaves a narrower but material release-integrity gap. An operator could
request recovery for a tag that the normal tag-triggered workflow would reject
for missing or malformed provenance.

## Proposed Decision

Require recovery preflight to validate the selected input tag through the
existing post-tag candidate gate before checksum extraction or any Homebrew or
npm mutation. The workflow will check out the requested tag with full history,
fetch `origin/main`, and pass `refs/remotes/origin/main` as the verifier's
explicit trusted main reference. All recovery publisher jobs remain
transitively downstream of that preflight.

The routine workflow will fail closed for historical tags without
`.release-candidate`, including v0.460.0. It will not expose a version-based
override. A historical recovery is an incident decision outside this routine
workflow and requires separate review.

## Rationale

Reusing the existing post-tag verifier preserves one provenance definition for
normal releases and recovery. Verifying against an explicitly fetched remote
main ref avoids trusting a dispatcher input or an assumed local branch in a
detached checkout. Requiring the same preflight for dry-run prevents dry-run
from becoming an undocumented compatibility or discovery bypass.

Keeping historical exceptions outside the routine workflow avoids a permanent
allowlist that could be expanded accidentally. It also keeps the existing
v0.460.0 exception accurate: published artifacts remain valid, but that tag
does not satisfy a policy introduced later.

## Public and Operational Contract

- Routine recovery accepts only future tags that satisfy release-candidate
  provenance and are reachable from `origin/main`.
- Provenance failure occurs before Homebrew cask mutation or npm publication.
- `dry_run` prevents publisher mutation but still requires valid provenance.
- No workflow input bypasses provenance. Historical recovery needs a separate
  incident decision.
- This decision does not alter existing release tags, GitHub Releases, npm
  packages, Homebrew casks, or product behavior.

## Deferred / Out of Scope

- Retroactively making historical tags provenance-valid.
- Automated tag/release/package/cask rollback or deletion.
- Cryptographic signing, external approval storage, or a generic emergency
  bypass mechanism.
- Product-code, parser, Query Access, SDK, CLI, HTTP, MCP, or version-surface
  changes.

## Acceptance Evidence Required

- Structural workflow tests prove the recovery provenance step is upstream of
  each external publisher path.
- Adversarial fixtures prove missing checkout depth, explicit fetch, same-step
  environment, post-tag gate, or dependency edges fail the contract check.
- Focused behavior tests prove invalid and historical tags stop before
  publisher stubs, while a valid future candidate chain passes.
- Dry-run evidence proves no remote mutation while retaining provenance
  verification.
- Independent review finds no P0/P1/P2 issues before this decision moves to
  Accepted.
