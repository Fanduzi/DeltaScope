# Specification: Release Recovery Provenance Enforcement

Date: 2026-07-30
Status: Proposed

## Problem

The normal tag-triggered release workflow now rejects a tag that lacks valid
`.release-candidate` provenance before it creates assets or publishes
downstream packages. `release-recover.yml` is a separate `workflow_dispatch`
path. It can update the Homebrew cask or publish the MCP launcher for an input
version after release preflight, but it does not currently prove that the input
tag satisfies the same provenance contract.

That makes recovery a release mutation path that could act on a hand-created,
malformed, or post-review tag even though the normal release workflow would
reject it.

## Objective

Before any recovery mutation, require the selected release tag to satisfy the
same repository-owned provenance contract as a normal future release tag:

- the tag is annotated;
- the tag target is reachable from the explicitly fetched `origin/main`;
- the tagged tree contains a valid `.release-candidate` file;
- the file version matches the recovery input version;
- the recorded candidate SHA is exactly the tag target parent; and
- only `.release-candidate` changed between the reviewed candidate and the
  tagged RC commit.

## Required Contract

1. The first step of the `preflight` job is a fail-closed dispatch-ref guard:
   the run continues only when `github.ref` is exactly `refs/heads/main`. Any
   other value — including a branch ref or a tag ref supplied to
   `workflow_dispatch` — fails before checkout, release preflight, checksum
   extraction, or any publisher work. GitHub's workflow-dispatch API accepts
   branch or tag refs and executes the workflow definition from that ref, so
   without this guard routine recovery could run under a modified workflow
   definition from a non-main ref and bypass the provenance constraints on
   `main`.
2. `release-recover.yml` checks out the requested input tag with full history,
   explicitly fetches `origin/main`, and invokes
   `posttag-candidate-gate` with `RELEASE_MAIN_REF=refs/remotes/origin/main`.
3. Provenance verification is part of the existing `preflight` job and occurs
   before its release-asset, checksum, Homebrew, or npm state work.
4. After the post-tag candidate gate succeeds, `preflight` resolves the
   verified tag to its peeled commit SHA and exports it as a job output,
   `tag_target_sha`. Verification target and published content are
   inseparable: what was verified is exactly what publishers may consume.
5. `publish-homebrew-cask` and `publish-mcp-launcher-package` check out
   `needs.preflight.outputs.tag_target_sha`. They must not check out the
   workflow default branch, the input tag name, `main`, or any ref that can
   move while the run is in progress. Without this pin, `preflight` could
   verify tag A while a publisher reads cask scripts or npm package content
   from a later `main` commit or a retargeted tag.
6. Every job capable of mutating an external release surface remains directly
   or transitively dependent on successful `preflight`.
7. The provenance check runs for dry-run and non-dry-run recovery. Dry-run
   avoids publisher mutation; it does not weaken admission of the requested
   tag.
8. There is no workflow input, environment switch, or historical-version
   allowlist that bypasses provenance verification.
9. The `preflight` job declares explicit job-level
   `permissions: contents: read`. It must not rely on workflow-level
   permissions, and it needs no `id-token: write`, publish token, or any
   permission that can mutate an external surface.

## Historical Compatibility

This contract applies only to future provenance-valid release tags. Historical
tags without `.release-candidate`, including `v0.460.0`, fail the automated
recovery preflight with a bounded diagnostic. They are not retroactively
rewritten or silently accepted.

An urgent historical recovery requires a separate, explicitly reviewed
incident decision. It is not an input option on the routine recovery workflow.

## Contract Gate Hermeticity

The current `release-recovery-contract-test` runs the live
`release-recovery-preflight` against `RELEASE_RECOVERY_CONTRACT_VERSION ?=
v0.240.0`. Under this contract, `v0.240.0` is a historical non-provenance tag
that must fail recovery admission, so it can no longer serve as a live
positive gate and would leave the gate permanently red or force a weakening
exception. Therefore:

- `release-recovery-contract-test` becomes a hermetic/static contract gate. It
  must not depend on the network, live GitHub Release assets, the npm
  registry, or a future release tag that does not yet exist.
- The positive path builds a future-valid RC chain in a temporary Git fixture
  with local stubs: reviewed candidate commit, valid `.release-candidate`
  file, annotated tag whose target parent is the candidate, and a simulated
  `origin/main` containing the tag target.
- Historical non-provenance tags, including `v0.240.0` and `v0.460.0`, are
  explicit negative cases: the gate proves they fail before any publisher
  stub is reached.
- If `release-recovery-preflight` is retained, it is an explicit operator
  diagnostic (network read-only asset/npm state inspection). It must not be a
  default dependency of the offline contract gate.

## Checker Enforcement Wiring

The recovery provenance structural checker must be invoked by the existing
`scripts/verify_release_workflow_hygiene.sh`, so that it is inherited by
`make release-workflow-hygiene-gates` and, through it, by
`make release-contract-gates`. Adding only a new disconnected Make target is
not acceptable: a checker that no required gate executes enforces nothing.
Tests must prove the wiring exists and must fail if the checker invocation is
removed from the hygiene script.

## Acceptance Criteria

- A valid provenance-tagged version passes recovery preflight before any
  Homebrew or npm publisher runs.
- A recovery dispatch whose `github.ref` is not `refs/heads/main` — whether a
  branch ref or a tag ref — fails at the dispatch-ref guard before checkout,
  checksum, or publisher work.
- The structural checker asserts the dispatch-ref guard exists, requires
  exactly `refs/heads/main`, and is positioned before all external work in
  `preflight`. Fixtures with a missing guard, a wrong guard value, or a guard
  placed after an external command must fail.
- Missing, lightweight, malformed, version-mismatched, parent-mismatched,
  extra-file, or non-main-ancestry tags fail the recovery preflight.
- A static workflow contract test rejects a missing recovery provenance step,
  missing full-history checkout, missing explicit `origin/main` fetch, a
  split environment/command step, or a publisher path that bypasses
  `preflight`.
- The structural checker verifies that `preflight` produces `tag_target_sha`
  from the verified tag after gate success, that the output is propagated,
  and that both publisher jobs consume exactly
  `needs.preflight.outputs.tag_target_sha` in their checkout. It rejects a
  publisher checkout of the workflow default branch, the input tag ref, a
  `main` ref, or any value other than the preflight-produced SHA.
- `release-recovery-contract-test` passes offline: its positive path uses a
  temporary Git fixture with a future-valid RC chain and local publisher
  stubs, and its negative path proves historical non-provenance tags such as
  `v0.240.0` and `v0.460.0` fail before any publisher stub. No network,
  GitHub Release, or npm registry access is required.
- The recovery provenance checker is executed via
  `scripts/verify_release_workflow_hygiene.sh` and therefore by
  `make release-workflow-hygiene-gates` and `make release-contract-gates`; a
  test fails when the checker invocation is removed from that script.
- The checker asserts that the `preflight` job declares job-level
  `permissions: contents: read`, and rejects a write permission or a missing
  job-level declaration.
- Positive and adversarial tests prove the checker reads the recovery workflow
  structurally, not through whole-file substring matching.
- Dry-run recovery performs provenance verification but does not push a cask,
  publish npm, create/move tags, edit releases, or dispatch another workflow.
- Existing Homebrew trust verification remains required and is not weakened.

## Non-Goals

- Do not recover historical non-provenance tags automatically.
- Do not add a privileged bypass, external approval store, cryptographic
  signing, automatic rollback, tag deletion, release deletion, npm unpublish,
  or Homebrew rollback.
- Do not change product code, Query Access behavior, public SDK/CLI/HTTP/MCP
  contracts, version surfaces, or normal tag-release packaging.
