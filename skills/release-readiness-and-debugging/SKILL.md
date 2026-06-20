---
name: release-readiness-and-debugging
description: "Use when preparing, releasing, post-release verifying, or debugging DeltaScope releases, GitHub Actions release runs, release assets, npm/Homebrew publishing, version surfaces, or recovery decisions."
---

# DeltaScope Release Readiness and Debugging

Use this skill for DeltaScope release readiness audits, release execution support, post-release checkpoints, and failed release diagnosis.

This skill is a guardrail. It does not replace `go-release`. When the user explicitly instructs a release, call and follow `go-release` first, then apply the stricter checks here.

## Core Rules

- Do not push, tag, publish, dispatch workflows, run `go-release`, or run `release-recover` unless the user explicitly instructs release or recovery.
- For release execution, require `go-release` and `no-co-author`.
- Never add AI attribution. Scan commits, tag annotation, and release body for `Co-authored-by|Generated-by|Claude|Codex|Anthropic|🤖|generated with`.
- Preserve existing tags/releases. Never move/delete tags or releases unless explicitly approved as a recovery action.
- Keep task reports factual: distinguish local audit, release execution, and post-release verification.
- If a release fails, stop and diagnose before recovery. Do not run `release-recover` reflexively.

## Pre-Release Readiness Audit

Run this when the user asks whether a DeltaScope version is ready to release.

1. Verify git state:
   - branch is `main`
   - `HEAD` is the intended release commit
   - `origin/main` is behind by the expected release commits
   - no tag points at `HEAD`
   - working tree has no tracked changes
   - previous release tag dereferences to the expected commit
2. Verify unpublished target:
   - no local or remote `vX.Y.Z*` tag
   - `gh release view vX.Y.Z` is not found
   - `npm view @fanduzi/deltascope-mcp@X.Y.Z version` is not found
3. Scan release commits for AI attribution.
4. Verify release surfaces:
   - `pkg/deltascope/version.go`
   - `packages/deltascope-mcp/package.json`
   - README install pins
   - release notes EN/ZH
   - changelog, releases index, roadmap, landing page
   - `scripts/verify_release_consistency.py`
5. Run the version's release gates:
   ```bash
   python3 scripts/test_verify_release_consistency.py
   VERSION=vX.Y.Z python3 scripts/verify_release_consistency.py
   PYTHONPATH=scripts python3 scripts/test_verify_docs_examples.py
   VERSION=vX.Y.Z python3 scripts/verify_docs_examples.py
   make docs-example-gates VERSION=vX.Y.Z
   make release-version-surface-gates VERSION=vX.Y.Z
   make release-surface-gates VERSION=vX.Y.Z
   make release-contract-gates VERSION=vX.Y.Z
   make release-test-gates
   make release-consistency-test
   make decision-record-gate
   make release-gofmt-gate
   make lint-landing
   git diff --check
   go mod tidy && git diff --exit-code go.mod go.sum
   go build ./...
   go build -tags postgresql ./...
   go vet ./...
   go vet -tags postgresql ./...
   go test ./... -count=1
   go test ./... -count=1 -tags postgresql
   npm test --prefix packages/deltascope-mcp
   make build
   ```
6. Smoke binary versions:
   ```bash
   bin/deltascope --version
   bin/deltascope-server --version
   bin/deltascope-mcp -version
   ```

If any gate fails, report the exact failing command and do not release.

## Release Execution

Only when the user explicitly says to release:

1. Use `go-release` as the release workflow.
2. Re-run or confirm the pre-release gates.
3. Push `main` only after gates pass.
4. Create an annotated tag `vX.Y.Z` at the audited commit with a minimal annotation like `Release vX.Y.Z`.
5. Push only the intended tag. Do not use broad `--tags`.
6. Wait for the tag-triggered release workflow.
7. Verify all expected jobs succeed:
   - `release-linux`
   - `release-linux-arm64`
   - `release-macos-amd64`
   - `release-macos-arm64`
   - `publish-homebrew-cask`
   - `verify-homebrew-cask-install`
   - `publish-mcp-launcher-package`

Do not manually publish npm or Homebrew unless the release workflow explicitly requires it and the user approves.

## Post-Release Checkpoint

Run this after a release is reported complete.

1. Git refs:
   - local `main` = `origin/main` = `vX.Y.Z^{}`
   - previous release tag unchanged
   - tag at `HEAD` includes `vX.Y.Z`
   - `git log vX.Y.Z..HEAD` is empty
2. GitHub Release:
   - `tagName` and `name` match
   - draft and prerelease are false
   - target commitish is `main`
   - expected 9 assets are present
3. Asset verification:
   ```bash
   VERSION=vX.Y.Z python3 scripts/verify_release_assets.py
   ```
4. npm:
   ```bash
   npm view @fanduzi/deltascope-mcp@X.Y.Z version
   npm view @fanduzi/deltascope-mcp dist-tags.latest
   ```
5. Homebrew:
   - cask version matches
   - darwin amd64/arm64 URLs point to the release assets
   - sha256 values are present
   - install verification job succeeded
6. Local binary smoke:
   ```bash
   bin/deltascope --version
   bin/deltascope-server --version
   bin/deltascope-mcp -version
   ```
7. Scan commits, tag annotation, and release body for AI attribution.
8. Confirm working tree has no tracked changes.

## Debugging Failed Releases

When a release workflow, asset check, npm publish, or Homebrew publish fails:

1. Identify the failed run, job, and step.
2. Confirm whether the tag exists locally and remotely.
3. Confirm whether the GitHub Release exists and whether assets are partial.
4. Confirm npm and Homebrew state separately.
5. Classify failure:
   - pre-tag local gate failure
   - tag pushed but workflow failed before release creation
   - partial GitHub Release/assets
   - npm publish failed
   - Homebrew publish or install verification failed
6. Recommend the smallest safe next action. Do not mutate state until the user approves.

Use `release-recover` only for a real recovery case, only after diagnosis, and only with explicit user approval.

## Report Format

For readiness audits, report:

- git state
- unpublished target checks
- commit table and AI attribution scan
- diff scope by category
- version/feature facts
- full gate table
- decision record status
- final conclusion

For release reports, report:

- skills used
- pre-release state
- gates
- push and tag results
- GitHub Actions run and jobs
- GitHub Release assets
- asset checksum verification
- npm and Homebrew verification
- binary smoke
- final refs
- safety notes

For post-release checkpoints, keep the report shorter and state clearly whether recovery is needed.
