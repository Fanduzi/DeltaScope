# Design: Release Candidate Provenance Enforcement

Date: 2026-07-29
Status: Proposed

## Decision Drivers

- A release tag must be traceable to a reviewed, gate-approved commit.
- Local convenience checks are insufficient because `git tag` can be run
  directly.
- The tag-triggered workflow is the last safe point before release assets and
  downstream publications exist.
- The design must preserve the current tag-driven release and non-destructive
  recovery contracts.

## Model

```
reviewed candidate -- only .release-candidate --> RC commit -- annotated tag --> release.yml
        |                                               |                         |
        +-- candidate_sha ------------------------------+                         +-- post-tag gate
```

The RC commit is intentionally separate from the reviewed candidate. Its
`candidate_sha` records the parent commit, avoiding a self-referential commit
SHA. The existing pre-tag and post-tag scripts remain the source of truth for
validating this relation.

## Local Mutation Boundary

Introduce one repository-owned release orchestrator, for example
`scripts/release_from_candidate.sh`, with an explicit `--dry-run` mode.

For a non-dry run it performs, in order:

1. Verify `main`, tracked-clean state, remote freshness, release collision
   checks, and the requested version.
2. Run the full post-merge release gates required by the current release
   process.
3. Run `make pretag-candidate-gate VERSION=<version>`.
4. Create one annotated tag at `HEAD`.
5. Run `make posttag-candidate-gate VERSION=<version>`.
6. Push `main` without tags, then push only the new tag ref.

The command stops on the first failure. If a failure happens after local tag
creation and before push, it reports the local state and requires an explicit
operator decision; it performs no automatic cleanup.

`--dry-run` executes read-only preflight, selected release gates, and the
pre-tag gate. It does not create a temporary tag because the post-tag script is
already unit-tested against temporary repositories; dry-run must not mutate
repository refs.

The release handoff continues to require the `go-release` skill. That skill or
its operator instructions invoke this repository-owned command rather than
reproducing tag and push steps by hand.

## Workflow Enforcement Boundary

Add an early provenance job to `.github/workflows/release.yml`, triggered by
the existing `v*` tag push. It checks out the exact tag and runs
`make posttag-candidate-gate VERSION="${GITHUB_REF_NAME}"`.

The job must use `fetch-depth: 0`, fetch `origin/main` explicitly, and pass a
resolved remote-tracking ref such as `refs/remotes/origin/main` to the
post-tag verifier. The verifier interface must accept that ref explicitly
(with `main` retained as the local default) and fail when it cannot resolve.
This avoids relying on a local `main` branch in the detached tag checkout.
The job sets job-level `permissions: contents: read`; it does not inherit the
workflow's write capability.

All artifact build and publishing jobs must depend on this job, directly or
through the existing first release job. The job must run before GoReleaser,
GitHub Release creation, checksum generation, Homebrew publication, and npm
publication. It consumes no secrets and performs no external mutation.

For the current workflow topology, `release-linux` is the first job that can
create release assets, so it must declare `needs: provenance`; every other
release or publisher job is already transitively downstream of
`release-linux`. The checker must not encode that topology as a permanent
assumption. It must parse `needs` edges, identify jobs containing GoReleaser,
GitHub Release mutation, npm publication, or Homebrew cask mutation, and
prove each such job is reachable only after `provenance` succeeds.

`release-recover.yml` is deliberately out of scope: it does not create a tag
or build assets. Its existing asset/release preflight remains the recovery
boundary. A later decision may add provenance requirements to recovery after
an explicit compatibility policy for historical tags is approved.

## Failure Semantics

| Condition | Local orchestrator | Tag workflow |
|---|---|---|
| Missing or malformed RC file | Stop before tag | Fail before artifacts |
| Candidate mismatch or extra RC files | Stop before tag | Fail before artifacts |
| Existing local/remote tag | Stop before tag | Not applicable |
| Hand-created invalid tag | Cannot prevent directly | Fail before artifacts |
| Post-tag gate failure before push | Preserve local state; report | Not applicable |
| Push failure | Preserve refs; report | Not applicable |

## Security and Compatibility Boundaries

- `.release-candidate` is provenance metadata, not an authorization token or
  a replacement for human approval.
- Candidate provenance applies to releases created after this milestone. The
  v0.460.0 exception remains documented and immutable.
- No release workflow may use the absence of provenance metadata as a reason
  to silently continue or to repair a tag.
