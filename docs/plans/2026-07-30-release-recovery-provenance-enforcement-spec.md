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

1. `release-recover.yml` checks out the requested input tag with full history,
   explicitly fetches `origin/main`, and invokes
   `posttag-candidate-gate` with `RELEASE_MAIN_REF=refs/remotes/origin/main`.
2. Provenance verification is part of the existing `preflight` job and occurs
   before its release-asset, checksum, Homebrew, or npm state work.
3. Every job capable of mutating an external release surface remains directly
   or transitively dependent on successful `preflight`.
4. The provenance check runs for dry-run and non-dry-run recovery. Dry-run
   avoids publisher mutation; it does not weaken admission of the requested
   tag.
5. There is no workflow input, environment switch, or historical-version
   allowlist that bypasses provenance verification.

## Historical Compatibility

This contract applies only to future provenance-valid release tags. Historical
tags without `.release-candidate`, including `v0.460.0`, fail the automated
recovery preflight with a bounded diagnostic. They are not retroactively
rewritten or silently accepted.

An urgent historical recovery requires a separate, explicitly reviewed
incident decision. It is not an input option on the routine recovery workflow.

## Acceptance Criteria

- A valid provenance-tagged version passes recovery preflight before any
  Homebrew or npm publisher runs.
- Missing, lightweight, malformed, version-mismatched, parent-mismatched,
  extra-file, or non-main-ancestry tags fail the recovery preflight.
- A static workflow contract test rejects a missing recovery provenance step,
  missing full-history checkout, missing explicit `origin/main` fetch, a
  split environment/command step, or a publisher path that bypasses
  `preflight`.
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
