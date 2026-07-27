# Decision: Homebrew Trust Workflow Contract Test

Date: 2026-07-27
Status: Accepted
Related milestone/version: patch-sized release-engineering follow-up
Related commits: (will be filled after commit)
Related tests: scripts/test_verify_release_workflow_hygiene.py
Related docs: docs/dev/testing.md, scripts/README.md, docs/roadmap.md

## Context

The `verify-homebrew-cask-install` job in both `release.yml` and `release-recover.yml` performs a real Homebrew install from the published tap to verify the cask works correctly. This job includes a critical `brew trust --cask fanduzi/deltascope/deltascope` command that must execute before `brew install --cask deltascope`.

Without a static contract test, future workflow edits could silently:
- Remove the trust command
- Move it to a different job
- Reorder it after the install command
- Change the cask name

Simple substring searches would pass even if the trust command appeared in comments, prose, or unrelated jobs.

## Decision

Add a structural contract checker (`scripts/verify_release_workflow_hygiene.py`) that:
- Reads both workflow files
- Locates the `verify-homebrew-cask-install` job by YAML structure
- Extracts run commands from that specific job only
- Verifies the exact trust command exists before the exact install command
- Reports failures with file and line context

Integrate this checker into the existing `release-workflow-hygiene-gates` target via the shell wrapper.

## Rationale

**Why structural parsing over substring search?**
- Substring search cannot distinguish between commands in the correct job vs. comments, prose, or other jobs
- Structural parsing validates job ownership and command ordering
- Prevents false positives from partial matches or misplaced commands

**Why stdlib-only YAML parsing?**
- Avoids introducing PyYAML dependency
- Matches existing precedent in `verify_docs_examples.py`
- Sufficient for the narrow contract being enforced
- No risk of YAML execution or side effects

**Why integrate into existing gate?**
- `release-workflow-hygiene-gates` already checks `release.yml` for hygiene violations
- Already included in `release-contract-gates`
- No new Makefile target needed
- Single entrypoint for workflow contract verification

## Public Contract

After this decision:
- Both `release.yml` and `release-recover.yml` must contain `brew trust --cask fanduzi/deltascope/deltascope` before `brew install --cask deltascope` in the `verify-homebrew-cask-install` job
- `make release-workflow-hygiene-gates` enforces this contract
- The checker is static: it reads files only, never executes workflows or commands
- Unit tests cover both positive and negative scenarios

## Deferred / Out Of Scope

- **Not a full YAML validator.** The checker only parses the narrow structure needed for the trust/install contract.
- **Not a workflow executor.** The checker never runs Homebrew, Docker, npm, or any external command.
- **Not a secret accessor.** The checker never reads GitHub secrets or environment variables.
- **Not a product feature.** This is CI/release hygiene only.
- **Not a general workflow linter.** Only the trust/install sequence is checked.

## Verification Evidence

- `python3 scripts/test_verify_release_workflow_hygiene.py` - 16 tests covering:
  - Valid workflows pass
  - Missing trust command fails
  - Trust after install fails
  - Wrong cask name fails
  - Trust in wrong job fails
  - Missing verify job fails
  - Trust only in comments fails
  - Checker safety (no command execution)
- `bash scripts/verify_release_workflow_hygiene.sh` - passes with real workflows
- `make release-workflow-hygiene-gates` - passes (integrated into existing gate)

## Consequences

- Future workflow edits that break the trust/install contract will fail CI
- The checker must be updated if the verification job name changes
- The checker must be updated if the exact trust or install commands change
- Unit tests should be updated if new negative scenarios are identified

## Links

- Commits: (will be filled after commit)
- Tests: scripts/test_verify_release_workflow_hygiene.py
- Docs: docs/dev/testing.md, scripts/README.md, docs/roadmap.md
