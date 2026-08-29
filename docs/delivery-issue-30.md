# Delivery evidence for issue #30

Verified on 2026-08-29 for [issue #30](https://github.com/Fanduzi/DeltaScope/issues/30).

## Revisions and delivery

- Base: `6ad79bf89db5e8e11ff60bdcaf7e5fba43d68e8f`
- Candidate: `ea75e56021550c07b2c7330fdf4af7ed12ab5e3a`
- Merged revision: `ea75e56021550c07b2c7330fdf4af7ed12ab5e3a`
- `origin/main` at the candidate delivery checkpoint: `ea75e56021550c07b2c7330fdf4af7ed12ab5e3a`
- Merge type: the candidate was already committed on local `main`; no merge commit was created. Remote `main` was fast-forwarded by a normal push.
- Push range: `6ad79bf89db5e8e11ff60bdcaf7e5fba43d68e8f..ea75e56021550c07b2c7330fdf4af7ed12ab5e3a`

## Candidate gates

All commands ran from `/Users/fan/GolangProjects/DeltaScope` at candidate SHA `ea75e56021550c07b2c7330fdf4af7ed12ab5e3a`.

- `make test`: PASS.
- `make build`: PASS; built `deltascope`, `deltascope-server`, and `deltascope-mcp`.
- `make lint`: PASS; `golangci-lint` reported 0 issues.
- `PYTHONPATH=scripts python3 scripts/test_verify_docs_examples.py`: PASS; 81 tests ran.
- `VERSION=v0.490.0 make docs-example-gates`: PASS; `docs-examples: PASS`.
- `git diff --check 6ad79bf89db5e8e11ff60bdcaf7e5fba43d68e8f...ea75e56021550c07b2c7330fdf4af7ed12ab5e3a`: PASS.

## Review

The `/code-review` skill ran independent Standards and Spec reviews. Their terminal verdicts were clean, with 0 unresolved P1 findings and 0 unresolved P2 findings.

## CI and E2E

- [Lint run 33222691153](https://github.com/Fanduzi/DeltaScope/actions/runs/33222691153): `golangci-lint` completed successfully for candidate SHA `ea75e56021550c07b2c7330fdf4af7ed12ab5e3a`.
- [CLI TLS E2E run 33222691203](https://github.com/Fanduzi/DeltaScope/actions/runs/33222691203): `CLI TLS E2E (MySQL 8.4 + PostgreSQL 17)` completed successfully for the same candidate SHA.
- E2E command: `make test-e2e-cli-tls`.
- E2E totals: 12 total, 12 passed, 0 failed, 0 skipped.
- E2E serving directory: `/home/runner/work/DeltaScope/DeltaScope`.
- E2E serving SHA: `ea75e56021550c07b2c7330fdf4af7ed12ab5e3a`, proven by the checkout log and run metadata.
- `Landing Page` was not triggered because the candidate did not change `docs/landing/**`; it is path-filtered and is not required for this delivery.

## Preservation and cleanup

The allowed user-owned dirty-path whitelist was `.agents/`, `.debug-journal.md`, `.opencode/`, `.pi-subagents/`, `.qoder/`, and `docs/quality/`. These paths did not overlap the candidate diff and remained untouched. No branch, worktree, container, service, or user file was deleted or cleaned up. Existing unrelated branches and worktrees were intentionally preserved.

A decision record was not required because this delivery corrects documentation for an existing CLI contract and does not change runtime behavior, public output shape, architecture, privacy boundaries, or cross-surface behavior.
