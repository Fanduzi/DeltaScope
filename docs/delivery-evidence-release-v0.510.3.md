# v0.510.3 Delivery Evidence

Date: 2026-08-31

## Git provenance

- Release base and previous tag: `v0.510.2` at `dd1abeed2e245cf9c1df281fa376c4a4a38123d2`.
- Issue #64 implementation commits: `57a508f48722cc448b208d0c5ff64cec1f4c65f4` and file-header fix `e5a24eb33b77573ea348cd647a360bbed31844b1`.
- Independently reviewed release candidate: `0cc05c06babb1843b5f2b4ec9b42bfbad53e811f`.
- Final code and release Standards/Spec verdicts: PASS; P1 0, P2 0, P3 0.
- Approval/tag commit: `ec33a917fe8c528c23060ca463531f21e150ccc4`; its only change is `.release-candidate`.
- Annotated tag object `1972949eae62733904801bb555653e3c8f1c38ea` peels to `ec33a917fe8c528c23060ca463531f21e150ccc4`.
- The release branch fast-forwarded `main` over `dd1abeed2e245cf9c1df281fa376c4a4a38123d2..ec33a917fe8c528c23060ca463531f21e150ccc4`; local `main` and `origin/main` resolved to `ec33a917fe8c528c23060ca463531f21e150ccc4` after the release push.

The root worktree retained only the pre-existing untracked whitelist: `.agents/`, `.debug-journal.md`, `.opencode/`, `.pi-subagents/`, `.qoder/`, and `docs/quality/`. No cleanup was performed; existing branches, worktrees, and services were intentionally preserved.

## Candidate gates and review

The local release dry-run and real release-orchestrator gates passed, including 12 CLI TLS E2E tests and 15 npm launcher tests. The published Go SDK `CommandTransport` smoke checks against both `@fanduzi/deltascope-mcp@0.510.3` and `@latest` exited 0 and returned `get_capabilities` with `connection.connect_timeout` present.

The release evidence gate passed:

```text
VERSION=v0.510.3 python3 scripts/verify_release_assets.py
```

It reported exactly 9 expected assets (9/9) and passing checksums. No commit in the release range contains AI attribution or a `Co-Authored-By` trailer.

## Published release

- GitHub Release: [v0.510.3](https://github.com/Fanduzi/DeltaScope/releases/tag/v0.510.3), non-draft and non-prerelease.
- Release workflow: [33348174567](https://github.com/Fanduzi/DeltaScope/actions/runs/33348174567), completed successfully.
- Successful jobs: provenance `99356226002`; release-linux `99356244990`; release-macos-amd64 `99357835665`; release-macos-arm64 `99357835684`; release-linux-arm64 `99357835689`; publish-homebrew-cask `99358665087`; publish-mcp-launcher-package `99358665106`; verify-homebrew-cask-install `99358686842`.

Archive SHA-256 values:

| Archive | SHA-256 |
| --- | --- |
| `deltascope_0.510.3_linux_amd64.tar.gz` | `ad2e5218231ee1d943d273c3f50bd4e8b51740d19ca558421b41402533b533b8` |
| `deltascope_0.510.3_linux_arm64.tar.gz` | `63e78bd37ee890eb071c95b39ac11ba5dcd47a33abe6baa3f4b067aa9151556b` |
| `deltascope_0.510.3_darwin_amd64.tar.gz` | `e957ad081f3fd2a2c20d7c64e798fe0a9d92a9bd126a3410e16d4fbe7acfdf29` |
| `deltascope_0.510.3_darwin_arm64.tar.gz` | `e8ffd2e9c50992c353984134ba643559006ed7689b6cff55fc7e9e07b235d9a2` |

## Package channels

- npm `@fanduzi/deltascope-mcp@0.510.3` is the `latest` dist-tag; version `0.510.3`, engines `>=24`, integrity `sha512-lgSl9FcrAMM5OfKDvIGR0MHDHhgFI257MxR3w4J/ds4JaU8OQC3XAQO4qc9KFYTg0skCjVJXil4WZfGVZZH4iQ==`, shasum `5f15ef747c70f34cf2da08b0d5c9dce83009ebe0`, and gitHead `ec33a91`.
- Homebrew tap commit [`6b3c25ccd7daa29f7253770e4fcb26f7894ea7c9`](https://github.com/Fanduzi/homebrew-deltascope/commit/6b3c25ccd7daa29f7253770e4fcb26f7894ea7c9) was published, and install verification passed.

## Main CI and issue acceptance

CI for the implementation and approval commits completed successfully:

- Code main: [Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/33347161415) and [CLI TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/33347161375).
- Approval main: [Landing Page](https://github.com/Fanduzi/DeltaScope/actions/runs/33347761722), [Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/33347761724), and [CLI TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/33347761704).

This fulfills issue #64 acceptance. The issue remains open. The existing decision record [2026-08-31-mcp-connect-timeout-capability.md](decisions/2026-08-31-mcp-connect-timeout-capability.md) is required and sufficient; no new decision record is required for release execution because it introduces no new boundary.
