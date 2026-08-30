# v0.510.2 Delivery Evidence

Date: 2026-08-31

## Git provenance

- Release base: `11926452300b6fd3d42cbe316696d8689cd5c6ed` (`v0.510.1` RC/tag commit).
- Independently reviewed candidate: `92df5f6ee8045480c3b8babb3f2e722be5ebcf88`.
- Final Standards and Spec verdicts: PASS; P1 0, P2 0, P3 0 after fixing the swapped v0.500.0/v0.490.0 landing bindings and class-token validator coverage.
- RC-only commit: `dd1abeed2e245cf9c1df281fa376c4a4a38123d2`; its only change is `.release-candidate`.
- Annotated tag object `74b1a8483c533a62d8feb06ec9dc81ecb61bfa85` peels to `dd1abeed2e245cf9c1df281fa376c4a4a38123d2`.
- Local `main` and `origin/main` resolved to `dd1abeed2e245cf9c1df281fa376c4a4a38123d2` after the release push. The release branch was fast-forwarded to `main`; no additional release cleanup was performed.

The root worktree retained only the pre-existing untracked whitelist: `.agents/`, `.debug-journal.md`, `.opencode/`, `.pi-subagents/`, `.qoder/`, and `docs/quality/`.

## Candidate gates and review

The reviewed candidate passed the final Standards and Spec reviews with no remaining P1, P2, or P3 findings. The landing-page binding correction and class-token validator coverage were included before the final PASS verdicts.

The release evidence gate passed:

```text
VERSION=v0.510.2 python3 scripts/verify_release_assets.py
```

It reported 9/9 expected assets and passing checksums. No commit contains AI attribution or a `Co-Authored-By` trailer.

## Published release

- GitHub Release: [v0.510.2](https://github.com/Fanduzi/DeltaScope/releases/tag/v0.510.2), non-draft and non-prerelease.
- Release workflow: [33320693864](https://github.com/Fanduzi/DeltaScope/actions/runs/33320693864), completed successfully.
- Successful jobs: provenance `99282017630`; release-linux `99282036339`; release-macos-amd64 `99283565829`; release-macos-arm64 `99283565898`; release-linux-arm64 `99283565913`; publish-homebrew-cask `99284333633`; publish-mcp-launcher-package `99284333636`; verify-homebrew-cask-install `99284360128`.

Archive SHA-256 values:

| Archive | SHA-256 |
| --- | --- |
| `deltascope_0.510.2_linux_amd64.tar.gz` | `dea562a93718b175c2103a597a15664dcbf7552ec98dc6de545d0a3e4caee9f7` |
| `deltascope_0.510.2_linux_arm64.tar.gz` | `06049f8f7e80fbb7a2ba04f39d4da6edb0533542b88a5ef63755b2ea2a3b3068` |
| `deltascope_0.510.2_darwin_amd64.tar.gz` | `c5af622f9f4af3db5fbb66c9f629161c53982896c5fe76b6939c8eb49437a2e3` |
| `deltascope_0.510.2_darwin_arm64.tar.gz` | `37ebc66a6bfa34c786257d0c5c721e3d12632c93ee24a1288aa27795acd644a9` |

## Package channels

- npm `@fanduzi/deltascope-mcp@0.510.2` is the `latest` dist-tag; version `0.510.2`, engines `>=24`, integrity `sha512-6eyhekiedc7MmmaEWGR1tY6z1OONIoO3itbCMYtfEdhOyjrE+lztR6K13MFhQb7jfi3xIetmEOx2bdtGFmT/FQ==`, and shasum `c2c5752b40a17f6ce2932d577859f5a91c58bac9`.
- A real pinned `npx` cache-miss smoke check verified the checksum, launched darwin-arm64, and returned exit 0 with `v0.510.2` output. `@latest` was a cache hit and also returned exit 0 with `v0.510.2` output.
- `help` and `-help` returned exit 2 with the same native usage output, except for npm's single-hyphen warning.
- Homebrew tap commit [`f367fda20f486a1d9593785eb008396233b24a4d`](https://github.com/Fanduzi/homebrew-deltascope/commit/f367fda20f486a1d9593785eb008396233b24a4d) was published, and install verification passed.

## Main CI and issue acceptance

CI for release commit `dd1abeed2e245cf9c1df281fa376c4a4a38123d2` completed successfully:

- [Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/33320432480)
- [Landing Page](https://github.com/Fanduzi/DeltaScope/actions/runs/33320432485)
- [CLI TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/33320432625)

This fulfills issue #63 acceptance. The issue remains open. No decision record was required because this release evidence introduces no new boundary.
