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

## Preflight Check

Before dispatching recovery, run the preflight locally to verify release asset state:

```bash
VERSION=v0.230.0 make release-recovery-preflight
```

This validates that the GitHub Release exists, has the expected 9 assets, checksums are consistent, and reports npm package state.
