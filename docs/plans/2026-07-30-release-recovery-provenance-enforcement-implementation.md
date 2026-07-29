# Implementation Plan: Release Recovery Provenance Enforcement

Date: 2026-07-30
Status: Proposed

## Scope

Implement a provenance admission boundary for the existing
`release-recover.yml` workflow. Do not modify product code or run an actual
recovery.

1. Characterize the current recovery workflow and preflight contract. Record
   its external mutation jobs, their `needs` edges, and the historical-tag
   behavior that will become fail-closed.
2. Extend `release-recover.yml` preflight to check out the requested tag with
   complete history, fetch `origin/main`, and run
   `posttag-candidate-gate` with `RELEASE_MAIN_REF` on the same step.
   Position this before checksum extraction and all publisher work.
3. Add or extend a stdlib-only structural checker for `release-recover.yml`.
   It must verify per-step checkout depth, fetch, environment, command order,
   preflight permissions, and graph reachability from preflight to each
   Homebrew/npm mutation path.
4. Add positive and adversarial fixtures. Cover missing provenance, wrong
   checkout ref, shallow checkout, omitted fetch, environment split across
   steps, missing post-tag gate, post-tag gate after preflight work, a
   Homebrew publisher bypass, and an npm publisher bypass.
5. Add focused tests for the recovery preflight invocation. Prove that a valid
   future RC tag is accepted and missing/malformed/historical provenance is
   rejected before publisher commands. Use temporary repositories or local
   stubs; never dispatch the real workflow.
6. Update operator and testing documentation. State that routine recovery is
   only available for future provenance-valid tags, that dry-run still checks
   provenance, and that historical recovery needs a separate incident
   decision.
7. Run the focused shell/Python tests, recovery workflow hygiene/contract
   gates, decision-record gate, release formatting, `git diff --check`, and a
   GitNexus compare-scope check. Obtain a read-only independent audit before
   changing the ADR to Accepted.

## Required Evidence Before Acceptance

- A static DAG proof that no Homebrew or npm mutation job can run without
  successful recovery preflight provenance.
- Positive and negative per-step parser fixtures, including env/run split and
  shallow-checkout failures.
- Temporary-repository evidence that invalid or historical tags fail before
  any publisher stub is reached.
- Evidence that dry-run verifies provenance and remains non-mutating.
- A review confirming that `v0.460.0` remains a historical exception and is
  not silently allowed through routine recovery.
- No P0/P1/P2 findings from independent audit.
