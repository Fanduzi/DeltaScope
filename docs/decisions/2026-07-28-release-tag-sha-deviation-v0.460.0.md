# Decision: Release Tag SHA Deviation Record — v0.460.0

Date: 2026-07-28
Status: Accepted
Related milestone/version: v0.460.0
Related commits: d04dc78 (tagged), fc21c0d (reviewed candidate)
Related tests: scripts/test_verify_pretag_candidate.sh (11 negative/positive tests), scripts/verify_pretag_candidate.sh (pre-tag gate), scripts/verify_release_tag_candidate.sh (post-tag gate)
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

Add a two-gate design backed by an auditable `.release-candidate` file:

**`.release-candidate` file** (committed on the release branch during prep):
Contains `version:` and `candidate_sha:` (the reviewed commit's SHA). This is
the single source of truth. The executor cannot substitute an arbitrary SHA —
the gate reads from the file, and an env var override is rejected if it conflicts.

**Pre-tag gate** (`scripts/verify_pretag_candidate.sh`, `make pretag-candidate-gate`):
Runs BEFORE `git tag`. Fails closed if `.release-candidate` is missing or has
empty fields. Verifies:
1. `candidate_sha == HEAD^` — the reviewed commit is HEAD's parent
2. `HEAD^..HEAD` only changed `.release-candidate` — no unreviewed files
3. HEAD is on `main`
4. Tag does not already exist
5. Working tree is clean

**Post-tag gate** (`scripts/verify_release_tag_candidate.sh`, `make posttag-candidate-gate`):
Runs AFTER `git tag`. Verifies the tag is annotated, the target is on `main`,
and the target's parent matches the approved candidate from the file.

**Tests** (`make pretag-candidate-test`): 11 automated negative/positive cases
covering missing file, version mismatch, HEAD drift, non-main branch, existing
tag, dirty tree, conflicting env var, empty SHA, extra files in RC commit, and
success paths.

The pre-tag gate is the enforcement point. The post-tag gate is confirmation.
The `.release-candidate` file makes the approved SHA auditable — it's committed
on the release branch and cannot be retroactively changed without a new commit.

## Rationale

**Why not move the tag?**
The release workflow, GitHub Release, npm, Homebrew, and binary smoke all
completed successfully with `d04dc78`. Moving the tag would require deleting
the GitHub Release, unpublishing npm, and re-triggering the entire workflow —
a higher-risk operation than documenting the deviation.

**Why the `.release-candidate` file instead of an env var?**
An env var (`RELEASE_CANDIDATE_SHA=$(git rev-parse HEAD)`) is self-proving —
the executor can set it to whatever HEAD is, defeating the guard. The file is
committed on the release branch during prep. Its content is auditable (anyone
can read it). The gate rejects env var overrides that disagree with the file.

**Why `candidate_sha == HEAD^` instead of `HEAD`?**
The `.release-candidate` file is committed on top of the reviewed candidate.
The file records the reviewed commit's SHA (HEAD^), not its own commit SHA.
This avoids the circular dependency where a commit's SHA depends on its tree
(which contains the file with the SHA).

## Public Contract

After this decision:
- v0.460.0 is released at `d04dc78` (not the reviewed `fc21c0d`).
- The difference is a single-line JS syntax fix in `docs/landing/index.html`
  (adding `items: {` opening brace). No product code, manifest, test, or
  public contract changed.
- `.release-candidate` is committed on the release branch with the reviewed
  candidate's SHA. The pre-tag gate (`make pretag-candidate-gate`) fails closed
  if the file is missing, has empty fields, or if `HEAD^` doesn't match.
- The post-tag gate (`make posttag-candidate-gate`) confirms the tag target's
  parent matches the file.

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
- Pre-tag gate test suite: 11/11 pass (T1-T8 negative, T9-T11 positive).

## Consequences

- During release prep, the last commit on the release branch MUST be the
  `.release-candidate` file with `candidate_sha` set to the reviewed commit's
  SHA (the parent).
- Before tagging, run `make pretag-candidate-gate VERSION=vX.Y.Z`. This reads
  the file and verifies the chain. No env var needed.
- After tagging, run `make posttag-candidate-gate VERSION=vX.Y.Z` to confirm.
- If post-merge fixes are needed: commit the fix on the release branch, update
  `.release-candidate` with the new parent SHA, re-run gates, ff-merge again,
  then tag. Do not commit fixes directly on main after the ff-merge.

## Links

- Commits: d04dc78 (tagged), fc21c0d (reviewed), d04dc78-fc21c0d diff is 1 line
- Tests: scripts/test_verify_pretag_candidate.sh (11 cases), scripts/verify_pretag_candidate.sh, scripts/verify_release_tag_candidate.sh
- Docs: docs/roadmap.md
