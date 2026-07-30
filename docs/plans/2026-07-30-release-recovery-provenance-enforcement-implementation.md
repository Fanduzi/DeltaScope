# Implementation Plan: Release Recovery Provenance Enforcement

Date: 2026-07-30
Status: Proposed

## Scope

Implement a provenance admission boundary for the existing
`release-recover.yml` workflow. Do not modify product code or run an actual
recovery.

1. Characterize the current recovery workflow and preflight contract. Record
   its external mutation jobs, their `needs` edges, and the historical-tag
   behavior that will become fail-closed. Include the current live
   `release-recovery-contract-test` dependency on
   `RELEASE_RECOVERY_CONTRACT_VERSION ?= v0.240.0`, which the new fail-closed
   rule invalidates as a positive case.
2. Extend `release-recover.yml` preflight to check out the requested tag with
   complete history, fetch `origin/main`, and run
   `posttag-candidate-gate` with `RELEASE_MAIN_REF` on the same step.
   Position this before checksum extraction and all publisher work. After the
   gate succeeds, resolve the verified tag's peeled commit SHA and export it
   as the `tag_target_sha` job output. Declare explicit job-level
   `permissions: contents: read` on `preflight`; it needs no
   `id-token: write`, publish token, or external mutation permission.
3. Pin both publishers to the verified target: `publish-homebrew-cask` and
   `publish-mcp-launcher-package` check out
   `needs.preflight.outputs.tag_target_sha` instead of the workflow default
   branch. No publisher may check out the input tag name, `main`, or any
   movable ref.
4. Add or extend a stdlib-only structural checker for `release-recover.yml`.
   It must verify per-step checkout depth, fetch, environment, command order,
   and graph reachability from preflight to each Homebrew/npm mutation path.
   It must also verify that `preflight` produces `tag_target_sha` after gate
   success, that both publisher jobs consume exactly
   `needs.preflight.outputs.tag_target_sha` in their checkout (rejecting
   default checkout, input tag ref, `main` ref, or any non-preflight SHA),
   and that `preflight` declares job-level `permissions: contents: read`
   (rejecting write permissions or a missing job-level declaration).
5. Wire the checker into `scripts/verify_release_workflow_hygiene.sh` so it
   runs under `make release-workflow-hygiene-gates` and therefore
   `make release-contract-gates`. Do not rely on a new disconnected Make
   target. Add a mutating wiring test that fails when the checker invocation
   is removed from the hygiene script.
6. Add positive and adversarial fixtures. Cover missing provenance, wrong
   checkout ref, shallow checkout, omitted fetch, environment split across
   steps, missing post-tag gate, post-tag gate after preflight work, a
   Homebrew publisher bypass, an npm publisher bypass, a missing or
   unconsumed `tag_target_sha`, a publisher checking out a default branch or
   input tag ref, and a `preflight` job with write or absent job-level
   permissions.
7. Rework `release-recovery-contract-test` into a hermetic/static contract
   gate. Its positive path builds a future-valid RC chain in a temporary Git
   fixture with local publisher stubs; its negative path proves historical
   non-provenance tags (`v0.240.0`, `v0.460.0`) fail before any publisher
   stub. Remove the gate's dependency on the network, live GitHub Release
   assets, the npm registry, and `RELEASE_RECOVERY_CONTRACT_VERSION` as a
   live positive input. Keep `release-recovery-preflight`, if retained, as an
   explicitly documented operator diagnostic that the offline gate does not
   depend on by default.
8. Add focused tests for the recovery preflight invocation. Prove that a valid
   future RC tag is accepted and missing/malformed/historical provenance is
   rejected before publisher commands. Use temporary repositories or local
   stubs; never dispatch the real workflow.
9. Update operator and testing documentation. State that routine recovery is
   only available for future provenance-valid tags, that dry-run still checks
   provenance, that historical recovery needs a separate incident decision,
   and that `release-recovery-preflight` is an operator diagnostic, not a
   contract gate.
10. Run the focused shell/Python tests, recovery workflow hygiene/contract
    gates, decision-record gate, release formatting, `git diff --check`, and
    a GitNexus compare-scope check. Obtain a read-only independent audit
    before changing the ADR to Accepted.

## Required Evidence Before Acceptance

- A static DAG proof that no Homebrew or npm mutation job can run without
  successful recovery preflight provenance.
- Checker evidence that `preflight` emits `tag_target_sha` and that both
  publisher jobs check out exactly `needs.preflight.outputs.tag_target_sha`,
  with negative fixtures for default-branch, input-tag-ref, `main`-ref, and
  non-preflight-SHA publisher checkouts.
- Positive and negative per-step parser fixtures, including env/run split and
  shallow-checkout failures.
- An offline run of the reworked `release-recovery-contract-test` proving the
  fixture-based future-valid RC chain passes and that `v0.240.0` and
  `v0.460.0` fail before any publisher stub, with no network, GitHub
  Release, or npm registry access.
- Wiring evidence that the recovery provenance checker runs inside
  `scripts/verify_release_workflow_hygiene.sh` under
  `make release-workflow-hygiene-gates` and `make release-contract-gates`,
  and that removing the invocation makes the wiring test fail.
- Checker evidence that `preflight` job-level `permissions: contents: read`
  is asserted, with negative fixtures for write or missing declarations.
- Temporary-repository evidence that invalid or historical tags fail before
  any publisher stub is reached.
- Evidence that dry-run verifies provenance and remains non-mutating.
- A review confirming that `v0.460.0` remains a historical exception and is
  not silently allowed through routine recovery.
- No P0/P1/P2 findings from independent audit.
