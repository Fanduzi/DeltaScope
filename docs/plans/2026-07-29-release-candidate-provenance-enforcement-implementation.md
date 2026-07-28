# Implementation Plan: Release Candidate Provenance Enforcement

Date: 2026-07-29
Status: Proposed

## Scope

Implement release-process enforcement only. Do not modify product behavior or
release version surfaces.

1. Characterize the current pre-tag and post-tag scripts with focused tests.
   Add malformed duplicate-field and full-SHA validation cases if the current
   parsers do not reject them. Preserve the rule that the tag tree, not the
   active worktree, supplies post-tag provenance.
2. Add a single local release orchestrator with `VERSION` input and
   `--dry-run`. Make its mutating sequence pre-tag gate, annotated tag,
   post-tag gate, separate `main` push, then single-tag push. Make every
   failure stop without automatic deletion, retry, or force push.
3. Add an early `release.yml` provenance job that checks out the pushed tag
   with full history, explicitly fetches `origin/main`, and invokes the
   post-tag gate with that resolved ref. Give this job job-level
   `contents: read` permissions. Wire `release-linux` to depend on successful
   provenance verification; preserve or add transitive dependencies for every
   other artifact and publisher job.
4. Add static workflow contract tests that reject a missing provenance job,
   a tag checkout without an explicit main ref, write permissions on the
   provenance job, post-tag verification after GoReleaser, and any mutation
   path without a transitive provenance dependency. Parse the `needs` graph
   and identify mutation jobs by their GoReleaser, GitHub Release, npm, and
   Homebrew commands. Extend the release workflow hygiene gate rather than
   adding a disconnected test target.
5. Add temporary-repository tests for the orchestrator's dry-run and local
   failure states. The tests must prove dry-run creates no tag and performs no
   push, and prove the command rejects missing RC metadata, drift, collisions,
   and dirty tracked state.
6. Update operator documentation and release handoff instructions to require
   the `go-release` skill plus the orchestrator. Document the local-state
   response for failures after tag creation but before push.
7. Run focused shell/Python tests, release workflow hygiene and contract
   gates, decision-record gate, release formatting, `git diff --check`, and a
   GitNexus change-scope check. Run a read-only independent audit against the
   acceptance criteria before moving the ADR to Accepted.

## Required Evidence Before Acceptance

- Captured valid and invalid local candidate-chain tests.
- Workflow contract tests proving post-tag verification is upstream of all
  release mutation jobs.
- A dry-run transcript proving no ref or remote mutation.
- A manual review showing v0.460.0 still fails the future provenance contract
  rather than being retroactively accepted.
- No P0/P1/P2 findings from the independent audit.
