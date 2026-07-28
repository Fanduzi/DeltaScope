# Specification: Release Candidate Provenance Enforcement

Date: 2026-07-29
Status: Proposed

## Problem

The v0.460.0 tag included a landing-page fix committed after the reviewed
release candidate. The existing candidate gates can detect this shape, but a
human operator can still create and push a tag without calling them. A
tag-triggered workflow must therefore reject invalid provenance before it
builds assets or publishes downstream packages.

## Objective

For every new release tag, establish a mechanical chain from the reviewed
candidate commit to the tagged commit, and require that chain at both the
local mutation boundary and the tag-triggered workflow boundary.

## Definitions

- **Reviewed candidate**: the release-prep commit that has passed the approved
  review and release gates.
- **RC commit**: one commit directly on top of the reviewed candidate. It
  changes only `.release-candidate`, which records the version and reviewed
  candidate SHA.
- **Release tag target**: the RC commit. It is the only commit a new release
  tag may point to.

## Required Contract

1. `.release-candidate` is committed on the release branch as the final RC
   commit and contains exactly one release version and one full candidate SHA.
2. The recorded SHA equals the parent of the RC commit; the RC commit changes
   only `.release-candidate`.
3. A standard local release command runs the existing pre-tag gate, creates an
   annotated tag only after that gate passes, runs the post-tag gate, then
   pushes `main` and the tag separately.
4. `release.yml` runs the post-tag gate before GoReleaser, asset upload,
   Homebrew publication, or npm publication. A tag that bypassed the local
   command cannot publish release artifacts.
   The provenance job must fetch `origin/main` and pass that explicit ref to
   the verifier; a detached tag checkout must never assume a local `main`
   branch exists. The job has `contents: read` permissions only.
5. A dry-run performs no tag, push, release, publish, workflow dispatch, or
   remote mutation. It may run read-only preflight and verification commands.
6. Any failed gate stops the current operation. The implementation must not
   delete, move, recreate, or force-push a tag as automatic recovery.

## Acceptance Criteria

- A valid candidate chain passes local pre-tag, local post-tag, and the early
  workflow provenance check.
- Missing, malformed, version-mismatched, parent-mismatched, extra-file, or
  already-existing-tag candidates fail before a local tag is created.
- A manually created invalid annotated tag reaches `release.yml` but fails the
  provenance job before any build or publisher job starts.
- The orchestrator dry-run leaves local refs, remotes, GitHub Releases, npm,
  Homebrew, and workflows unchanged.
- The release workflow contract test proves that the workflow check runs
  before every artifact or publisher path, using the workflow `needs` graph
  rather than a fixed list of job names.

## Non-Goals

- Do not move, retag, or reinterpret historical releases, including v0.460.0.
- Do not add automatic tag deletion, release deletion, workflow reruns, npm
  unpublish, or Homebrew rollback behavior.
- Do not alter Go product code, SQL analysis, public SDK/CLI/HTTP/MCP
  contracts, release version surfaces, or GoReleaser packaging semantics.
- Do not claim that a local shell command can prevent a user from manually
  running Git. The workflow gate is the artifact-publication enforcement
  boundary.
