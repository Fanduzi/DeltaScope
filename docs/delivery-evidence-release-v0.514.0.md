# v0.514.0 Delivery Evidence

Date: 2026-11-16
Status: merged/pushed/CI green for the release and Homebrew repair described below.

## Git provenance

- Starting local main: `83b3fcc662d2578d254a57ce8d944557e403a7de`.
- Starting origin/main and retained golden baseline: `6a4f14e7aa82289c39f11a10fc04cb0fe92f41d2`.
- Previous release: `v0.513.0`, `bff6519113f4e7419a861e711aed300c9151d2da`.
- Reviewed release-preparation candidate: `8069406798e4c437f0ea9da9e2d53b4f2764f37b`.
- Approval/tag target: `eec3bca5c1f56f17419ab7b4ef6be48ca252e709`; its only change from the candidate is `.release-candidate`.
- Annotated tag object: `bf97a84673b4aec77343ccad375f6775e9378347`.
- Local main fast-forwarded from `83b3fcc6` to `eec3bca5`; the verified orchestrator pushed `6a4f14e7..eec3bca5` and then only tag `v0.514.0`.
- Homebrew renderer repair: `4f3abfcef34b5908f6168a7b7c6558213dfd29cf`, fast-forwarded to main and normally pushed as `eec3bca5..4f3abfce`.
- At the post-repair verification snapshot, local HEAD and origin/main both equalled `4f3abfcef34b5908f6168a7b7c6558213dfd29cf`. This evidence is a subsequent documentation-only commit, not a claim that the tag includes the repair.
- Homebrew tap base: `140dfa6e312a0baf02ba81c0908b7c596fc97f65`; repaired main and origin/main: `82220b08aded794dd4d559c7e095332ff8d843f7`.

No force push, tag movement, archive replacement, or npm overwrite was performed. Commits in the delivered range and the tap repair contain no AI attribution or co-author trailers.

## Candidate gates and review

The `go-release` and `no-co-author` skills were followed. Release preparation was reviewed in a separate read-only phase by the same agent; no independent-agent review is claimed. The implementation review supplied with the task was the existing baseline. The release-surface review found no unresolved P0/P1/P2 issues. The later live install failure is recorded separately below rather than concealed by that review result.

Passed commands:

- `go build ./...`
- `go test ./... -count=1`
- `CGO_ENABLED=1 go test -tags postgresql ./... -count=1`
- `make release-test-gates VERSION=v0.514.0`: default/tagged tests, SQL corpus gates, **12/12 CLI TLS E2E cases**, **20/20 npm launcher tests**, zero npm skips.
- `make release-contract-gates VERSION=v0.514.0`: all version/surface/binary/dialect/output/workflow/gofmt/launcher gates.
- `make lint-landing`; `goreleaser check`; `git diff --exit-code go.mod go.sum`; `git diff --check`.
- Module README checker, both working and staged forms; tracked Go sources produced empty `gofmt -l` output.
- `bash scripts/verify_pretag_candidate.sh v0.514.0`.
- `bash scripts/release_from_candidate.sh v0.514.0 --dry-run`, followed by `bash scripts/release_from_candidate.sh v0.514.0`: full gates repeated, annotated tag created only after the pre-tag gate, post-tag provenance passed, then main/tag pushed separately.
- `RELEASE_MAIN_REF=refs/remotes/origin/main make posttag-candidate-gate VERSION=v0.514.0`: passed after the repair advanced main.

Fresh Docker golden evidence at candidate `8069406798e4c437f0ea9da9e2d53b4f2764f37b`, served by the CLI built from `/Users/fan/GolangProjects/DeltaScope-v0.514.0`:

| Command | Executed / passed | Assertions | Failures / skipped |
| --- | --- | --- | --- |
| `make ddl-golden TASK=T54 ARTIFACT_DIR=/tmp/deltascope-v0.514.0-golden` | 32 / 32 | 351 | 0 / 0 |
| `make ddl-golden TASK=T25 ARTIFACT_DIR=/tmp/deltascope-v0.514.0-golden` | 52 / 52 | 784 | 0 / 0 |

`python3 scripts/ddl_golden.py validate --task all --artifact /tmp/ddl-golden-milestone/all/artifact.json` also passed. That retained aggregate is **50 tasks, 3361/3361 cases, 59,206 assertions at `6a4f14e7`**, not a release-tag rerun.

## Published release and incident

[Release v0.514.0](https://github.com/Fanduzi/DeltaScope/releases/tag/v0.514.0) is non-draft and non-prerelease, with exactly **9/9 expected assets**.

[Release workflow 43812771712](https://github.com/Fanduzi/DeltaScope/actions/runs/43812771712), attempt 2, concluded **success**. All eight jobs are successful: provenance, release-linux, release-linux-arm64, release-macos-arm64, release-macos-amd64, publish-mcp-launcher-package, publish-homebrew-cask, and verify-homebrew-cask-install.

Attempt 1 published successfully but failed its Homebrew install verifier because the renamed `postflight_steps` body still used legacy Ruby accessors. The failure was reproduced with the real local Homebrew loader. The committed renderer/contract fix passed build, default and PostgreSQL-tagged tests, release-contract gates, positive and four legacy-regression checks, and real cask loading. The published tap received exactly the two-line declarative-body correction, matching the reviewed renderer and existing release checksums. `gh run rerun 43812771712 --failed` reran only the install verifier; successful publisher jobs were not rerun. The tag and all release assets remained unchanged.

Main CI is also green:

- Tag/approval main: [Landing Page 43812764982](https://github.com/Fanduzi/DeltaScope/actions/runs/43812764982), [Lint 43812764808](https://github.com/Fanduzi/DeltaScope/actions/runs/43812764808), [CLI TLS E2E 43812764810](https://github.com/Fanduzi/DeltaScope/actions/runs/43812764810).
- Repair main: [Lint 43815843111](https://github.com/Fanduzi/DeltaScope/actions/runs/43815843111), [CLI TLS E2E 43815843126](https://github.com/Fanduzi/DeltaScope/actions/runs/43815843126).

## Assets and package channels

`VERSION=v0.514.0 make release-recovery-preflight` passed the 9-asset/checksum-reference gate and reported the npm version present. All four archives were additionally downloaded and hashed against their sidecars:

| Archive | SHA-256 |
| --- | --- |
| darwin_amd64 | `2d394f6aad3c3d4afe58d549f98926f52c3467ceef381ed7f203b4e7bed78654` |
| darwin_arm64 | `1b58a8fbf183cf3921ba5571641130e48b087633dc1dbf35f14e5910d3f04525` |
| linux_amd64 | `96910d9f0a7019a67503095f8b5312777f742c1cd80284e50eaa99de1985f8e5` |
| linux_arm64 | `aef6c4aeee6723b48390f14baa7f2004fb1dd9c35924396b883517e676825d9c` |

- `scripts/verify_release_archive.sh` passed against the published darwin_arm64 archive: CLI/server/MCP versions, PostgreSQL smoke, archive shape, and dialect isolation. Other architectures were built/verified in their successful CI jobs; they were not executed on the local ARM64 host.
- npm registry query proved `@fanduzi/deltascope-mcp` `latest` = `0.514.0`.
- The repaired Homebrew tap is published at `82220b08aded794dd4d559c7e095332ff8d843f7`, and the real macOS install verifier passed.
- An HTTPS fetch of `https://deltascope.pages.dev/` proved spotlight `v0.514.0` and exactly three historical cards: `v0.513.0`, `v0.512.0`, `v0.511.1`. No browser visual inspection is claimed.

## Preservation and decision records

The clean release worktree was used for edits. The root's pre-existing untracked skills/tool directories, debug journal, artifacts, verification directories, quality HTML, `package-consistency-check.py`, and `reuse-ledger.txt` were left untouched. Full `git status --porcelain=v1 -uall` snapshots compared byte-identical before work, before/after main integration, and after release verification. Existing worktrees/services/evidence archives were not cleaned up.

The PostgreSQL metadata and tablespace evidence decisions were already committed with the implementation. Release preparation introduced no new product boundary, so those records suffice. The non-obvious Homebrew incident repair required and includes [its own decision record](decisions/2026-11-16-homebrew-declarative-postflight-repair.md). No tracker issue was closed as part of this release.

Detailed local logs and downloaded assets are retained in `/tmp/deltascope-v0.514.0-evidence/`; golden artifacts are under `/tmp/deltascope-v0.514.0-golden/`. They supplement, rather than replace, this tracked record.
