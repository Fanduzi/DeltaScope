# Decision: Release Candidate Provenance Enforcement

Date: 2026-07-29
Status: Accepted
Related commits: 14128e1, 55ea429, c42d26a, e117c20, 6bbaffb
Related decisions: docs/decisions/2026-07-28-release-tag-sha-deviation-v0.460.0.md
Related tests: scripts/test_verify_pretag_candidate.sh, scripts/test_verify_release_tag_candidate.sh, scripts/test_release_from_candidate.sh, scripts/test_verify_release_workflow_provenance.py, scripts/test_verify_release_workflow_provenance_negative.py

## Context

v0.460.0 exposed a release-process deviation: a syntax fix was committed to
`main` after the reviewed release-prep candidate, and the release tag pointed
to that later commit. The release remains published and is not to be changed,
but the deviation showed that a local candidate check alone does not stop a
hand-created tag from triggering the release workflow.

The repository now has candidate-chain checks based on `.release-candidate`.
They validate an RC commit whose only change is provenance metadata recording
its reviewed parent. The remaining gap is orchestration: the checks are not
yet a mandatory local release path or an early tag-workflow dependency.

## Decision

Adopt a two-boundary provenance model for future releases:

1. A repository-owned local release orchestrator is the documented mutating
   path. It runs the pre-tag gate, creates the annotated tag, runs the
   post-tag gate, then pushes `main` and the tag separately.
2. The tag-triggered release workflow runs the post-tag gate before any
   artifact build or downstream publication, using an explicitly fetched
   `origin/main` ref in the detached tag checkout. The provenance job has
   job-level `contents: read` permissions, and every release mutation job is
   transitively dependent on that result.

The `.release-candidate` RC file remains the provenance source of truth. It is
not an approval mechanism; it makes the reviewed-commit assertion inspectable
and mechanically testable.

## Rationale

The local boundary prevents normal operator mistakes before a tag exists. The
workflow boundary contains manual bypasses, because a malformed or unrelated
tag cannot produce release artifacts. Keeping tag creation local preserves the
existing tag-driven workflow and avoids adding a privileged workflow-dispatch
release path.

The workflow check must use a remote-tracking main ref, not assume a local
`main` branch exists after Actions checks out a tag. Dependency validation is
graph-based so future workflow topology changes cannot silently introduce an
independent publisher path.

The design does not auto-delete a local tag or rewrite an existing release.
Those actions need explicit human judgment because they can invalidate public
artifacts and downstream package state.

## Public and Operational Contract

- Future release tags must target an RC commit that contains a valid
  `.release-candidate` file.
- A tag that lacks valid provenance fails before artifact creation or
  publication.
- Operators use the `go-release` skill and the repository release
  orchestrator; hand-written tag/push sequences are unsupported.
- v0.460.0 remains a documented historical exception. This decision does not
  move its tag, alter its assets, or claim that it satisfies the new contract.

## Deferred / Out of Scope

- Applying provenance checks to the existing recovery workflow or historical
  release tags.
- Cryptographic signing or external approval storage for candidate metadata.
- Automated tag deletion, release deletion, reruns, npm unpublish, or
  Homebrew rollback.
- Product-code, parser, Query Access, CLI, HTTP, MCP, or version-surface
  changes.

## Acceptance Evidence

The specification and implementation landed as five focused commits
(`14128e1`, `55ea429`, `c42d26a`, `e117c20`, `6bbaffb`) that were
fast-forward merged and pushed to `main`.

- Focused checks: 69 passing — pre-tag gate 16, post-tag gate 14,
  orchestrator 21, provenance contract 7, adversarial 11.
- Gates: `release-workflow-hygiene-gates`, `decision-record-gate`,
  `release-gofmt-gate`, and `git diff --check` all passed.
- Independent review found no P0/P1/P2 issues.
- T12 exercised a bare-remote `pre-receive` hook and proved
  `refs/heads/main` is pushed before `refs/tags/v0.1.0`, confirming the
  orchestrator's main-then-tag push ordering.

v0.460.0 remains a documented historical exception; nothing above claims it
satisfies this contract.
