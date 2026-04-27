# DeltaScope v0.46.0 Release Notes

## Summary

v0.46.0 cleans the Homebrew cask install verification path in the release workflow. Successful release runs should no longer show misleading Homebrew tap/cask unavailable error annotations. Real Homebrew install, version, and audit failures still block the release.

## Changed

- The `verify-homebrew-cask-install` job in the release workflow now uses conditional cleanup probes (`if brew list --cask deltascope`) instead of tolerated failure fallbacks (`|| true`). On fresh CI runners, cleanup is skipped silently; on re-runs, it removes the previous cask and tap before reinstalling.
- Added `make release-workflow-hygiene-gates` static gate that enforces conditional cleanup probes, lowercase tap names, and rejects tolerated failure patterns in the release workflow. This gate is included in `make release-contract-gates`.
- Documented the Homebrew verification hygiene contract in developer testing docs.

## Release Workflow Hygiene

The release workflow runs a real Homebrew install from the published tap on macOS. It verifies:

- The cask installs from `fanduzi/deltascope`
- `deltascope --version` contains the release tag
- The binary includes PostgreSQL support
- A PostgreSQL audit smoke passes

Before v0.46.0, the cleanup step used `brew uninstall --cask deltascope || true` and `brew untap Fanduzi/deltascope || true`. On fresh runners these commands emit errors to stderr, and GitHub Actions promotes stderr to error annotations even when the exit code is caught. The v0.46.0 fix replaces unconditional cleanup with conditional probes so no spurious annotations appear.

## Verification

- `make release-contract-gates VERSION=v0.46.0` — all gates pass
- `make release-workflow-hygiene-gates` — static gate passes
- `make release-test-gates` — all tests pass

## Non-Goals

- No SQL audit behavior changes.
- No parser, rule, or policy changes.
- No formatter changes.
- No release asset naming changes.
- No npm launcher behavior changes.

## Install

**macOS (recommended):**

```bash
brew tap fanduzi/deltascope
brew install --cask deltascope
```

**Generic installer:**

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.46.0/install.sh | \
  DELTASCOPE_VERSION=v0.46.0 sh
```
