# Release Recovery

When a release partially fails, use this guide to determine the correct recovery path.

## Workflows

| Workflow | Trigger | Purpose |
|----------|---------|---------|
| `release.yml` | Tag push (`v*`) | Full release: build binaries, upload assets, publish Homebrew cask, publish npm |
| `release-recover.yml` | Manual `workflow_dispatch` | Downstream-only recovery: re-publish Homebrew cask and/or npm package |

## Failure Matrix

| Failure point | Recovery action |
|---------------|-----------------|
| Failure **before** GitHub Release exists (e.g. build error, test gate failure) | Fix the problem and rerun the full `release.yml` workflow by re-pushing the tag |
| GitHub Release assets exist, **Homebrew** failed | Run `release-recover.yml` with `recover_homebrew=true`, `recover_npm=false` |
| GitHub Release assets exist, Homebrew verified, **npm** failed | Run `release-recover.yml` with `recover_homebrew=false`, `recover_npm=true` |
| GitHub Release assets exist, **both** Homebrew and npm failed | Run `release-recover.yml` with both enabled (defaults) |
| Assets **missing or wrong** | Stop. This requires an explicit tag/release replacement decision. Delete the release and tag only after confirming the replacement plan. |

## Dispatching the Recovery Workflow

```bash
gh workflow run release-recover.yml \
  --repo Fanduzi/DeltaScope \
  --ref main \
  -f version=v0.230.0 \
  -f recover_homebrew=true \
  -f verify_homebrew=true \
  -f recover_npm=false
```

## What the Recovery Workflow Does

- Downloads existing GitHub Release assets as source of truth
- Re-renders and re-publishes the Homebrew cask (idempotent: succeeds if already up to date)
- Re-publishes the npm launcher package (idempotent: skips if version already exists)
- Verifies Homebrew cask install on macOS

## What the Recovery Workflow Does NOT Do

- Does not rebuild binaries or invoke GoReleaser
- Does not upload, replace, or delete release assets
- Does not create, delete, or move tags
- Does not overwrite existing npm package versions

## Warnings

- **Do not rerun the full `release.yml` workflow after assets exist.** The existing-release guard will block it. Even without the guard, GoReleaser would fail with `422 already_exists`.
- **Do not delete a tag or release unless you have made an explicit replacement decision.** Deleting a published release is destructive and cannot be undone.
- **Do not overwrite npm package versions.** npm does not allow re-publishing the same version. If the published package is wrong, a new patch version is required.

## Required Secrets

| Secret | Purpose |
|--------|---------|
| `HOMEBREW_TAP_TOKEN` | Write access to `Fanduzi/homebrew-deltascope` |
| `NPM_TOKEN` | npm publish for `@fanduzi/deltascope-mcp` |
| `GITHUB_TOKEN` | Read access to release assets during preflight (auto-provisioned by Actions, must be explicitly wired as `GH_TOKEN` env var) |

## Preflight Check

Before dispatching recovery, run the preflight locally to verify release asset state:

```bash
VERSION=v0.230.0 make release-recovery-preflight
```

This validates that the GitHub Release exists, has the expected 9 assets, checksums are consistent, and reports npm package state.

### Preflight Auth Wiring

The preflight step uses `gh release download` and `gh release view`, which require GitHub CLI authentication. The workflow wires this via `GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}` in the preflight step environment. This env var was missing in v0.241.0 and added in `0e3f31c` (shipped in v0.242.0). The contract test gate (`make release-recovery-contract-test`) statically verifies that this wiring is present, preventing silent regressions.

## Dry-Run Recovery Validation

The recovery workflow accepts a `dry_run` input (default: `true`) that exercises all non-destructive recovery logic without mutating downstream state.

### What dry-run validates

- Release asset existence and checksum extraction
- Homebrew cask render and syntax verification
- Homebrew tap clone and cask diff (`Homebrew cask would be updated` or `already up to date`)
- npm package state check (`present` or `absent`)

### What dry-run does not do

- Does not push to the Homebrew tap
- Does not run `brew install`
- Does not run `npm publish`
- Does not mutate tags, releases, or assets

### Safe dry-run dispatch

```bash
gh workflow run release-recover.yml \
  --repo Fanduzi/DeltaScope \
  --ref main \
  -f version=v0.240.0 \
  -f dry_run=true \
  -f recover_homebrew=true \
  -f verify_homebrew=true \
  -f recover_npm=true
```

### Real recovery

Real recovery requires explicit `dry_run=false` and should only be used when:

- Release assets exist and are correct
- A known downstream failure has been confirmed (Homebrew or npm)
- The operator intends to push/publish

```bash
gh workflow run release-recover.yml \
  --repo Fanduzi/DeltaScope \
  --ref main \
  -f version=v0.240.0 \
  -f dry_run=false \
  -f recover_homebrew=true \
  -f verify_homebrew=true \
  -f recover_npm=true
```
