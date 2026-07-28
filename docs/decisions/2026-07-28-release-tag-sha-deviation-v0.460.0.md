# Decision: Release Tag SHA Deviation Record — v0.460.0

Date: 2026-07-28
Status: Accepted
Related milestone/version: v0.460.0
Related commits: d04dc78 (tagged), fc21c0d (reviewed candidate)
Related tests: scripts/verify_pretag_candidate.sh (pre-tag gate), scripts/verify_release_tag_candidate.sh (post-tag gate)
Related docs: docs/roadmap.md

## Context

DeltaScope v0.460.0 release prep reviewed and approved candidate `fc21c0d` on
branch `release/v0.460.0-prep`. The release execution fast-forwarded `main` to
`fc21c0d`, but then discovered a JS syntax error in `docs/landing/index.html`
(the `items: {` opening brace was missing from the EN i18n data block, causing
`lint-landing` to fail).

The fix was committed as `d04dc78` directly on `main` after the ff-merge. The
tag `v0.460.0` was then created pointing at `d04dc78` instead of the reviewed
candidate `fc21c0d`.

This violates the release constraint: the annotated tag must point at the exact
commit SHA that was reviewed and gate-approved as the release candidate.

## Decision

Record this as a documented release process exception for v0.460.0. The release
is valid and all artifacts are verified. The tag will not be moved.

Add two gates to prevent recurrence:

1. **Pre-tag gate** (`scripts/verify_pretag_candidate.sh`, `make pretag-candidate-gate`):
   runs BEFORE `git tag`. `RELEASE_CANDIDATE_SHA` is **mandatory** — omission
   fails closed. Verifies HEAD == candidate SHA, HEAD is on `main`, tag does
   not yet exist, and working tree is clean.

2. **Post-tag gate** (`scripts/verify_release_tag_candidate.sh`, `make posttag-candidate-gate`):
   runs AFTER `git tag`. Verifies the tag is annotated, the peeled target is
   on `main`, and (when `RELEASE_CANDIDATE_SHA` is set) the target matches the
   candidate exactly.

Both gates are wired into the Makefile. The pre-tag gate is the enforcement
point; the post-tag gate is a confirmation step.

## Rationale

**Why not move the tag?**
The release workflow, GitHub Release, npm, Homebrew, and binary smoke all
completed successfully with `d04dc78`. Moving the tag would require deleting
the GitHub Release, unpublishing npm, and re-triggering the entire workflow —
a higher-risk operation than documenting the deviation.

**Why two gates instead of one?**
The pre-tag gate prevents the incorrect tag from being created. The post-tag
gate confirms the tag landed correctly. Together they form a verify-then-confirm
pair. The pre-tag gate fails closed on missing `RELEASE_CANDIDATE_SHA`; the
post-tag gate is backward-compatible for workflows that only need annotation
and ancestry checks.

## Public Contract

After this decision:
- v0.460.0 is released at `d04dc78` (not the reviewed `fc21c0d`).
- The difference is a single-line JS syntax fix in `docs/landing/index.html`
  (adding `items: {` opening brace). No product code, manifest, test, or
  public contract changed.
- Future releases MUST set `RELEASE_CANDIDATE_SHA` before tagging. The pre-tag
  gate (`make pretag-candidate-gate`) fails closed if it is omitted. The
  post-tag gate (`make posttag-candidate-gate`) enforces the match when set.

## Deferred / Out Of Scope

- **Not a CI-enforced gate.** The guard runs locally. CI integration into
  `release.yml` would require storing the candidate SHA as a workflow input
  or artifact, which is a larger change deferred to a future milestone.
- **Not retroactive.** Past releases are not affected.

## Verification Evidence

- v0.460.0 GitHub Release: 9 assets, checksums match, not draft/prerelease.
- npm `@fanduzi/deltascope-mcp@0.460.0`: published, `dist-tags.latest = 0.460.0`.
- Homebrew cask: version `0.460.0`, URL and sha256 correct.
- Binary smoke: `deltascope --version` / `deltascope-server --version` /
  `deltascope-mcp --version` all report `v0.460.0`.
- AI attribution scan: clean across commits, tag annotation, and release body.
- `d04dc78` vs `fc21c0d` diff: only `items: {` line in landing page i18n JS.

## Consequences

- Release operators MUST export `RELEASE_CANDIDATE_SHA=<full-sha>` and run
  `make pretag-candidate-gate VERSION=vX.Y.Z` before `git tag`. Omission of
  the env var is a hard error, not a skip.
- After tagging, run `make posttag-candidate-gate VERSION=vX.Y.Z` to confirm.
- If post-merge fixes are needed, the correct sequence is: commit the fix on
  the release branch, re-run gates, fast-forward main again, re-run the pre-tag
  gate with the new HEAD SHA, then tag. Do not commit fixes directly on main
  after the ff-merge.

## Links

- Commits: d04dc78 (tagged), fc21c0d (reviewed), d04dc78-fc21c0d diff is 1 line
- Tests: scripts/verify_pretag_candidate.sh, scripts/verify_release_tag_candidate.sh
- Docs: docs/roadmap.md
