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

Recovery dispatch **must use `--ref main`**. The workflow's first preflight step fails closed unless the dispatch ref is exactly `refs/heads/main`, because a branch or tag dispatch would execute an unreviewed workflow definition.

```bash
gh workflow run release-recover.yml \
  --repo Fanduzi/DeltaScope \
  --ref main \
  -f version=v0.461.0 \
  -f recover_homebrew=true \
  -f verify_homebrew=true \
  -f recover_npm=false
```

## Provenance Admission

Before any downstream work, the recovery preflight proves the input tag is a legitimate release candidate:

1. Fail-closed dispatch-ref guard (`refs/heads/main` only), before checkout or any network work
2. Full-history checkout of the input tag; the tag must be annotated
3. `.release-candidate` is read from the tagged commit and the candidate chain is verified against fetched `origin/main` (`make posttag-candidate-gate` with `RELEASE_MAIN_REF=refs/remotes/origin/main`)
4. On success, the peeled tag target SHA is exported as `tag_target_sha`; the Homebrew and npm publisher jobs check out exactly that SHA — never the default branch, `main`, the input tag ref, or any other movable ref

**Historical tags predating the candidate provenance contract (for example `v0.240.0` and `v0.460.0`) fail admission by design.** There is no bypass or allowlist. Recovering such a release requires an explicit incident decision (typically a new patch release with a valid candidate chain), not a gate exception. Dry-run dispatches go through the same admission — `dry_run=true` does not skip provenance verification.

## What the Recovery Workflow Does

- Verifies provenance admission first (dispatch-ref guard, annotated tag, candidate chain against `origin/main`)
- Downloads existing GitHub Release assets as source of truth
- Re-renders and re-publishes the Homebrew cask from the verified tag target SHA (idempotent: succeeds if already up to date)
- Re-publishes the npm launcher package from the verified tag target SHA (idempotent: skips if version already exists)
- Verifies Homebrew cask install on macOS

## What the Recovery Workflow Does NOT Do

- Does not rebuild binaries or invoke GoReleaser
- Does not upload, replace, or delete release assets
- Does not create, delete, or move tags
- Does not overwrite existing npm package versions

## Warnings

- **Do not rerun the full `release.yml` workflow after assets exist.** The existing-release guard will block it. Even without the guard, GoReleaser would fail with `422 already_exists`.
- **Do not delete a tag or release unless you have made an explicit replacement decision.** Deleting a published release is destructive and cannot be undone.
- **Release tags must be annotated.** The release workflow guard rejects lightweight tags. If you discover a lightweight tag post-push, do not move it — decide whether a new patch release with an annotated tag is warranted. Verify locally with `VERSION=vX.Y.Z make release-tag-annotation-gate`.
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
VERSION=v0.461.0 make release-recovery-preflight
```

This validates that the GitHub Release exists, has the expected 9 assets, checksums are consistent, and reports npm package state. This is a read-only operator diagnostic — it is not part of `make release-recovery-contract-test`, which is hermetic and needs no network or existing release.

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
  -f version=v0.461.0 \
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
  -f version=v0.461.0 \
  -f dry_run=false \
  -f recover_homebrew=true \
  -f verify_homebrew=true \
  -f recover_npm=true
```
