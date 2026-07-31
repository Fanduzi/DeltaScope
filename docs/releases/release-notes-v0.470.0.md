# DeltaScope v0.470.0 Release Notes

## Summary - Release Recovery Provenance Enforcement

v0.470.0 hardens the manually dispatched release-recovery workflow so it enforces the same release-candidate provenance contract as the normal tag-triggered release workflow. Routine recovery now accepts only future annotated tags whose release-candidate provenance is valid and whose peeled tag target commit is reachable from the trusted `origin/main` reference. Recovery dispatch runs only on `refs/heads/main`, publisher jobs check out only the preflight-verified peeled tag target SHA, and the recovery contract gate is hermetic with historical tags `v0.240.0` and `v0.460.0` as documented fail-closed negatives.

This is a release-engineering change only. It does not change SQL audit behavior, Query Access, the SDK, CLI, HTTP, or MCP surfaces, and it does not alter any existing release tag, GitHub Release, npm package, or Homebrew cask.

## What Changed

### Recovery Provenance Admission

Recovery preflight validates the requested input tag through the existing post-tag candidate gate before checksum extraction or any Homebrew or npm mutation. The workflow checks out the requested tag with full history, fetches `origin/main`, and passes `refs/remotes/origin/main` as the verifier's explicit trusted main reference. Routine recovery therefore accepts only future provenance-valid annotated tags; historical tags without `.release-candidate` provenance, including `v0.460.0`, fail closed. No workflow input bypasses provenance, and `dry_run` still requires valid provenance.

### Main-Only Dispatch-Ref Guard

The first step of recovery `preflight` is an isolated, fail-closed guard that requires `github.ref` to be exactly `refs/heads/main` and fails before checkout, checksum extraction, or any external work. A branch-ref or tag-ref dispatch fails at the guard, so routine recovery always runs the reviewed workflow definition from `main`. The structural checker requires the canonical guard shape and rejects nested dead-code exits and else-branch inverted guards.

### Verified Peeled SHA Publisher Pinning

After the provenance gate succeeds, `preflight` resolves the verified tag to its peeled commit SHA and exports it as the `tag_target_sha` job output. `publish-homebrew-cask` and `publish-mcp-launcher-package` check out `needs.preflight.outputs.tag_target_sha` only — never the workflow default branch, the input tag name, or any ref that can move while the run is in flight. This closes the verify-then-publish gap between preflight verification and publisher checkout.

### Hermetic Recovery Contract Gate

`release-recovery-contract-test` is now a static, offline, deterministic gate. Its positive path is a future-valid release-candidate chain built in a temporary Git fixture with local stubs; its negative path proves that historical non-provenance tags `v0.240.0` and `v0.460.0` fail before any publisher stub. The gate needs no network, GitHub Release, or npm registry access, and `v0.240.0` no longer serves as a live positive gate input.

### Structural Checker and Hygiene Wiring

The recovery provenance structural checker is invoked by `scripts/verify_release_workflow_hygiene.sh` and is therefore inherited by `make release-workflow-hygiene-gates` and `make release-contract-gates`. The checker also asserts that recovery `preflight` declares explicit job-level `permissions: contents: read` and holds no publish or external mutation permission.

## What Stayed the Same

- SQL audit behavior, the registered audit rule catalog, and default audit output are unchanged. `level` remains the public audit priority field; no severity field is introduced.
- Query Access behavior on every surface is unchanged: default offline SDK, CLI, and HTTP stay fail-closed, and MCP still has no Query Access tool.
- MCP tools remain `audit_sql`, `describe_rule`, `list_rules`, and `get_capabilities` only.
- Existing release tags, GitHub Releases, npm packages, and Homebrew casks are untouched. Published `v0.460.0` artifacts remain valid; that tag simply does not satisfy a policy introduced later.
- The normal tag-triggered release workflow keeps its existing release-candidate provenance enforcement.

## Non-Goals

- Not historical recovery automation. Recovering a historical tag without `.release-candidate` provenance is an incident decision outside this routine workflow and requires separate review.
- Not a generic emergency bypass mechanism, version-based override, or allowlist.
- Not cryptographic signing or external approval storage.
- Not automated tag, release, package, or cask rollback or deletion.
- Not a product behavior change: no SQL audit, parser, rule, Query Access, SDK, CLI, HTTP, or MCP semantic change.
- Not a change to any published artifact or existing tag.

## Rule Catalog Facts

The registered audit rule catalog is unchanged from v0.460.0. This release changes release-recovery workflow enforcement only.

| Metric | Count |
|--------|------:|
| Total rules | **371** |
| blocker | 72 |
| warning | 142 |
| notice | 157 |

| Dialect scope | Rules |
|---------------|------:|
| common | 177 |
| postgresql | 191 |
| mysql | 1 |
| tidb | 2 |

| Statement kind | Rules |
|----------------|------:|
| ddl | 361 |
| dml | 10 |

## Unchanged Metrics

- SQL corpus: **582/582**, **100.0%**, **247** YAML fixture files.
- PostgreSQL ALTER TABLE config entries: **53**.
- DDL coverage catalog: **400** entries (mysql 61, tidb 54, postgresql 285, parser_upgrade_candidate 18).

## Decision Records

- `docs/decisions/2026-07-30-release-recovery-provenance-enforcement.md` (this release)
- `docs/decisions/2026-07-29-release-candidate-provenance-enforcement.md` (related: normal release workflow provenance)
- `docs/decisions/2026-07-28-query-access-relationless-literal-selects.md` (v0.460.0)
