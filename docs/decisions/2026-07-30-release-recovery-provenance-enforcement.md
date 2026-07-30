# Decision: Release Recovery Provenance Enforcement

Date: 2026-07-30
Status: Accepted
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

Constrain the dispatch execution ref: the first step of `preflight` is a
fail-closed guard that requires `github.ref` to be exactly
`refs/heads/main` and fails before checkout, checksum, or any external work.
GitHub's workflow-dispatch API accepts branch and tag refs and runs the
workflow definition from the dispatched ref; the guard prevents routine
recovery from running under a non-main workflow definition that could omit
the provenance constraints reviewed on `main`.

Bind verification to publication: after the gate succeeds, `preflight`
resolves the verified tag to its peeled commit SHA and exports it as the
`tag_target_sha` job output. `publish-homebrew-cask` and
`publish-mcp-launcher-package` check out
`needs.preflight.outputs.tag_target_sha`, never the workflow default branch,
the input tag name, or any ref that can move during the run.

Make the recovery contract gate hermetic: `release-recovery-contract-test`
becomes a static/offline gate whose positive path is a future-valid RC chain
in a temporary Git fixture with local stubs, and whose negative path proves
historical non-provenance tags such as `v0.240.0` and `v0.460.0` fail before
any publisher stub. `RELEASE_RECOVERY_CONTRACT_VERSION ?= v0.240.0` stops
serving as a live positive gate input. `release-recovery-preflight`, if
retained, is an explicit read-only operator diagnostic, not a default
dependency of the offline contract gate.

Enforce through existing required gates: the recovery provenance structural
checker is invoked by `scripts/verify_release_workflow_hygiene.sh` and is
therefore inherited by `make release-workflow-hygiene-gates` and
`make release-contract-gates`. A disconnected Make target is not an
acceptable enforcement point. The checker also asserts that `preflight`
declares explicit job-level `permissions: contents: read`; preflight needs no
`id-token: write`, publish token, or external mutation permission.

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

Pinning publishers to the preflight-resolved SHA closes a
verify-then-publish gap: without it, preflight could verify tag A while a
publisher checks out the workflow default branch or a retargeted tag and
publishes cask or npm content that was never verified.

The dispatch-ref guard closes a definition-substitution gap: a recovery run
dispatched on a branch or tag ref would execute that ref's copy of
`release-recover.yml`, which may lack the provenance admission entirely.
Requiring `refs/heads/main` keeps every routine recovery on the reviewed
workflow definition. The guard lives in that definition, so branch protection
on `main` and Actions workflow-dispatch permissions remain operational trust
prerequisites rather than something this guard replaces.

The contract gate must be hermetic because the fail-closed rule makes
`v0.240.0` — the current live positive input — correctly fail; keeping it as
a live positive gate would leave the gate permanently red or force a
weakening exception, and no provenance-valid release tag exists yet to
replace it. Fixture-based positives and explicit historical negatives keep
the gate green, deterministic, and network-free.

Wiring the checker into `scripts/verify_release_workflow_hygiene.sh` makes it
a real gate: a checker reachable only through a disconnected Make target that
no required gate runs would enforce nothing. Explicit job-level
`contents: read` on `preflight` keeps the admission boundary least-privileged
and independent of workflow-level permission drift.

Keeping historical exceptions outside the routine workflow avoids a permanent
allowlist that could be expanded accidentally. It also keeps the existing
v0.460.0 exception accurate: published artifacts remain valid, but that tag
does not satisfy a policy introduced later.

## Public and Operational Contract

- Routine recovery accepts only future tags that satisfy release-candidate
  provenance and are reachable from `origin/main`.
- Routine recovery runs only when dispatched on `refs/heads/main`; a
  branch-ref or tag-ref dispatch fails at the ref guard before any external
  work.
- Provenance failure occurs before Homebrew cask mutation or npm publication.
- Publisher jobs consume only content at the preflight-verified
  `tag_target_sha`; no publisher checks out a default branch, input tag ref,
  or movable ref.
- `dry_run` prevents publisher mutation but still requires valid provenance.
- No workflow input bypasses provenance. Historical recovery needs a separate
  incident decision.
- `release-recovery-contract-test` is offline and deterministic; historical
  tags `v0.240.0` and `v0.460.0` are its documented negative cases.
- The recovery provenance checker runs inside
  `make release-workflow-hygiene-gates` and `make release-contract-gates`
  via `scripts/verify_release_workflow_hygiene.sh`.
- The `preflight` job holds job-level `permissions: contents: read` only.
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
- Structural tests prove the dispatch-ref guard requires exactly
  `refs/heads/main` and precedes all external work; missing-guard,
  wrong-guard, guard-after-external-command, and branch-ref/tag-ref dispatch
  fixtures fail.
- Structural tests prove `preflight` produces `tag_target_sha` from the
  verified tag and that both publisher jobs check out exactly
  `needs.preflight.outputs.tag_target_sha`; default-branch, input-tag-ref,
  `main`-ref, and non-preflight-SHA checkouts are rejected.
- Adversarial fixtures prove missing checkout depth, explicit fetch, same-step
  environment, post-tag gate, or dependency edges fail the contract check.
- The reworked `release-recovery-contract-test` passes offline with a
  fixture-based future-valid RC chain and proves `v0.240.0` and `v0.460.0`
  fail before any publisher stub, without network, GitHub Release, or npm
  registry access.
- Wiring tests prove the checker runs via
  `scripts/verify_release_workflow_hygiene.sh` under
  `make release-workflow-hygiene-gates` and `make release-contract-gates`,
  and fail when that invocation is removed.
- Structural tests prove `preflight` declares job-level
  `permissions: contents: read` and reject write or missing declarations.
- Focused behavior tests prove invalid and historical tags stop before
  publisher stubs, while a valid future candidate chain passes.
- Dry-run evidence proves no remote mutation while retaining provenance
  verification.
- Independent review finds no P0/P1/P2 issues before this decision moves to
  Accepted.
