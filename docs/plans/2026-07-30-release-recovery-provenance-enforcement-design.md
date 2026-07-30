# Design: Release Recovery Provenance Enforcement

Date: 2026-07-30
Status: Proposed

## Decision Drivers

- Recovery can mutate Homebrew and npm without rebuilding release artifacts.
- The selected version is supplied by a human through `workflow_dispatch`.
- Provenance must be evaluated against a remote-tracking main ref, not the
  workflow's default checkout branch.
- Historical releases must not gain an undocumented compatibility bypass.
- The protection must survive future workflow topology changes.

## Model

```
workflow_dispatch(version)
          |
          v
release-recover preflight
  | checkout refs/tags/version, fetch-depth: 0
  | fetch origin main
  | posttag-candidate-gate(RELEASE_MAIN_REF=origin/main)
  | output tag_target_sha = peeled commit SHA of the verified tag
          |
          +-- fail: no checksum, Homebrew, or npm work
          |
          v
existing recovery checks and outputs
          |
          +------------------------------+
          v                              v
publish-homebrew-cask            publish-mcp-launcher-package
  checkout tag_target_sha          checkout tag_target_sha
```

The recovery preflight reuses `scripts/verify_release_tag_candidate.sh`; it
does not reimplement `.release-candidate` parsing. The recovery workflow must
make the requested tag available locally, fetch remote main explicitly, and
run the verifier against that trusted ref.

## Workflow Boundary

The `preflight` job is the single admission boundary. It must:

1. check out `refs/tags/${{ inputs.version }}` with `fetch-depth: 0`;
2. fetch `origin/main`;
3. set `RELEASE_MAIN_REF=refs/remotes/origin/main` on the same step that runs
   `make posttag-candidate-gate VERSION="${{ inputs.version }}"`;
4. after the gate succeeds, resolve the verified tag to its peeled commit SHA
   and export it as the job output `tag_target_sha`; and
5. declare explicit job-level `permissions: contents: read` instead of
   relying on workflow-level permissions. Preflight only reads repository
   content; it needs no `id-token: write`, publish token, or any external
   mutation permission.

Existing publisher jobs already depend on `preflight`. That dependency remains
the enforcement edge for Homebrew cask mutation and npm publication. In
addition, `publish-homebrew-cask` and `publish-mcp-launcher-package` must
check out `needs.preflight.outputs.tag_target_sha`, never the workflow
default branch, the input tag name, or any ref that can move during the run.
This binds the verified target to the published content: without the pin,
preflight could verify tag A while a publisher renders the cask or packs npm
content from a later `main` commit or a retargeted tag. The recovery
provenance checker must verify that the output is produced from the verified
tag, propagated through job outputs, and consumed exactly by both publisher
checkouts; it rejects a default checkout, an input tag ref, a `main` ref, or
any non-preflight SHA. The checker must also parse the `needs` graph and
rediscover publisher jobs from their commands instead of hard-coding a short
job list, and it must assert the exact job-level `contents: read` permission
on `preflight`, rejecting write permissions or a missing job-level
declaration.

The checker must inspect `release-recover.yml` independently from the normal
release checker. Shared parsing helpers are acceptable only when they preserve
per-step ownership: an environment declaration on one step must not satisfy a
post-tag command on another step.

## Gate Wiring

The recovery provenance checker is not a standalone convenience target. It
must be invoked by the existing `scripts/verify_release_workflow_hygiene.sh`,
which `make release-workflow-hygiene-gates` runs and
`make release-contract-gates` composes. A disconnected Make target that no
required gate executes would leave the contract unenforced. A mutating test
must prove the wiring: temporarily removing the checker invocation from the
hygiene script has to make the wiring test fail.

## Contract Gate Hermeticity

`RELEASE_RECOVERY_CONTRACT_VERSION ?= v0.240.0` currently drives a live
`release-recovery-preflight` inside `release-recovery-contract-test`. That
tag predates `.release-candidate` provenance, so under this design it must
fail admission and can no longer act as the gate's live positive case.
Resolution:

- `release-recovery-contract-test` becomes a hermetic/static contract gate
  with no dependency on the network, live GitHub Release assets, the npm
  registry, or a future release tag that does not exist yet.
- The positive path constructs a future-valid RC chain in a temporary Git
  fixture with local stubs: a reviewed candidate commit, a valid
  `.release-candidate`, an RC commit whose only change is that file, an
  annotated tag targeting it, and a simulated `origin/main` ref.
- Historical non-provenance tags such as `v0.240.0` and `v0.460.0` are
  explicit negative cases that must fail before any publisher stub runs.
- `release-recovery-preflight`, if retained, is an explicit operator
  diagnostic for live asset/npm state inspection. It stays read-only and is
  not a default dependency of the offline contract gate.

## Compatibility and Failure Semantics

| Condition | Recovery result |
| --- | --- |
| Valid future RC tag on `origin/main` | Continue to existing recovery preflight |
| Tag lacks `.release-candidate` | Fail before checksums or publishers |
| Candidate/file/version/parent mismatch | Fail before checksums or publishers |
| Tag not reachable from `origin/main` | Fail before checksums or publishers |
| `v0.240.0` or `v0.460.0` historical tag | Fail as non-provenance; no implicit exception |
| Dry-run with valid tag | Verify, then preserve existing no-publish behavior |
| Dry-run with invalid tag | Fail; dry-run is not an admission bypass |

The workflow performs no auto-repair. A failed recovery leaves remote tags,
GitHub releases, npm, and Homebrew unchanged. The operator receives the
bounded gate diagnostic and must use a separately approved incident process
for historical recovery.

## Security Boundaries

- `.release-candidate` is provenance metadata, not authorization.
- `origin/main` is an explicit trust anchor for the workflow run.
- A dispatcher-provided version is untrusted until the tag verifier accepts
  it; it must not select a publisher path independently.
- No test may dispatch a real workflow or mutate GitHub, npm, Homebrew, tags,
  or releases. Contract tests use static workflow fixtures; behavior tests use
  temporary Git repositories and local stubs only.

## Observability

The gate emits the existing bounded post-tag diagnostics. Documentation must
state that a provenance failure is expected for historical non-provenance tags
and is not proof that their published artifacts are invalid.
