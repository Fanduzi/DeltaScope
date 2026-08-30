# v0.510.1 Delivery Evidence

Date: 2026-08-30

## Git provenance

- Release base: `10c4d007fc0db492dd35b3136aada97bbc427646` (`v0.510.0` RC/tag commit).
- Independently reviewed candidate: `1970e8def5e6ba25ab66754ed94a734da37243d2`.
- Candidate verdict: APPROVE, P1 0, P2 0.
- RC-only commit: `11926452300b6fd3d42cbe316696d8689cd5c6ed`; its only change is `.release-candidate` with `version: v0.510.1` and the reviewed candidate SHA.
- Local `main`, `origin/main`, and peeled annotated tag `v0.510.1` all resolved to `11926452300b6fd3d42cbe316696d8689cd5c6ed` immediately after the release push.
- The release branch was fast-forwarded into local `main`; the release orchestrator pushed `10c4d007..1192645` to `origin/main` before pushing the tag.
- The release diff contains the deterministic CLI TLS CA test-contract correction, synchronized version/release surfaces, bilingual notes, and the decision record `docs/decisions/2026-08-30-v0.510.1-linux-tls-ca-test-contract.md`. No production Go implementation changed.

The root worktree retained only the pre-existing untracked whitelist: `.agents/`, `.debug-journal.md`, `.opencode/`, `.pi-subagents/`, `.qoder/`, and `docs/quality/`.

## Failure diagnosis and candidate gates

The official `v0.510.0` run [33302045413](https://github.com/Fanduzi/DeltaScope/actions/runs/33302045413) passed provenance and failed in `TestAuditCommandLoadsTLSCAFile` before any GitHub Release or assets were created. The annotated `v0.510.0` tag was preserved and was not moved or reused.

The focused Linux reproduction was red before the fix and green after it:

```text
docker run --rm -v /Users/fan/GolangProjects/DeltaScope:/src -w /src golang:1.26.1-bookworm go test ./internal/interfaces/cli -run '^TestAuditCommandLoadsTLSCAFile$' -count=1
```

The final candidate passed:

- `git diff --check 10c4d007..1970e8d`
- `make test`, `make build`, `go vet ./...`, coverage, and `golangci-lint`
- `make release-contract-gates VERSION=v0.510.1`
- `make release-test-gates VERSION=v0.510.1`, including Docker TLS e2e, SQL corpus, PostgreSQL-tagged, and npm gates
- `make release-version-surface-gates VERSION=v0.510.1`
- `make decision-record-gate`
- three-level documentation loop: `[three-level-doc] OK`
- `make release-from-candidate-dry-run VERSION=v0.510.1`
- `make release-from-candidate VERSION=v0.510.1`

No commit contains an AI attribution or `Co-Authored-By` trailer.

## Published release

- GitHub Release: [v0.510.1](https://github.com/Fanduzi/DeltaScope/releases/tag/v0.510.1), non-draft and non-prerelease.
- Release workflow: [33303809965](https://github.com/Fanduzi/DeltaScope/actions/runs/33303809965), completed successfully at release SHA `11926452300b6fd3d42cbe316696d8689cd5c6ed`.
- Successful jobs: provenance, Linux amd64, Linux arm64, macOS amd64, macOS arm64, Homebrew publish, npm publish, and Homebrew install verification.
- The release contains exactly nine expected assets. All four archive checksum files passed `shasum -a 256 -c`; the native darwin-arm64 CLI reported `deltascope v0.510.1 (mysql, tidb, postgresql)`.

Archive SHA-256 values:

| Archive | SHA-256 |
| --- | --- |
| `deltascope_0.510.1_linux_amd64.tar.gz` | `7dda7e93857f0e89fe757956fbdbd96dd763a570fdad4b7bbe0c0e48665e7f13` |
| `deltascope_0.510.1_linux_arm64.tar.gz` | `e6ac12ddac80dfaaee2d818112c0e10560b070f33f3865b3545dc2f07da95038` |
| `deltascope_0.510.1_darwin_amd64.tar.gz` | `c89747f55f16d082058aa1c5c721ecac8537e2339291675c7f1e10aac055fa79` |
| `deltascope_0.510.1_darwin_arm64.tar.gz` | `665d15f9066b08b1a9563bba0e1258017b86122c1abd7248c33a3560650ecf32` |

The published GitHub Release body contains both the English and Chinese release notes.

## Package channels

- npm `@fanduzi/deltascope-mcp@0.510.1` is the `latest` dist-tag, with SLSA provenance attestation and integrity `sha512-J10AvxM29/H9nsr9NiEucNa07T9UCdgAYklxuptkS61ARDnbrpWZqqE4bH3lOXlVf8H40wKjTdAbWHBAOuamHQ==`.
- Both pinned and unpinned `npx` smoke checks resolved and launched `v0.510.1` on darwin-arm64.
- Homebrew cask commit [`b50fc51739eaaf2ea63f00a607e66d3d10d9594b`](https://github.com/Fanduzi/homebrew-deltascope/commit/b50fc51739eaaf2ea63f00a607e66d3d10d9594b) publishes version `0.510.1` with the verified darwin checksums.
- The workflow's `verify-homebrew-cask-install` job passed the installed binary contract.

## Main CI and cleanup

CI for release commit `11926452300b6fd3d42cbe316696d8689cd5c6ed` completed successfully:

- [Lint](https://github.com/Fanduzi/DeltaScope/actions/runs/33303807822)
- [CLI TLS E2E](https://github.com/Fanduzi/DeltaScope/actions/runs/33303807824)
- [Landing Page](https://github.com/Fanduzi/DeltaScope/actions/runs/33303807842)

The used `release-v0.510.1` herdr tab was closed. The v0.510.0 and v0.510.1 release worktrees and their fully merged local release branches were removed; unrelated worktrees and branches were preserved.
