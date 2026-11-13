# Decision: ddl-golden anchor image identity is proven, not recorded (issue #81 R2)

Date: 2026-11-17
Status: Accepted
Related milestone/version: issue #81 T02 R2 under parent issue #79
Related commits: T02 R2 runner/evidence commit (this change set)
Related tests: `scripts/test_ddl_golden.py` (identity + post-state contract rows)
Related docs: `testdata/ddl-golden/anchors-baseline.json`, `docker/ddl-golden-compose.yaml`

## Context

The T02 Golden Runner previously *recorded* the repo digest each anchor
happened to resolve at run time. Reviewer R2 finding: a recorded digest is
not proof that the run used the pre-approved image — a re-pointed or
missing tag, a failed `docker image inspect`, or a container created from
a different image could all degrade silently to PASS.

## Decision

- `testdata/ddl-golden/anchors-baseline.json` locks, per anchor, the
  expected repo digest (`expected_image_digest`) and the expected
  `platform` (os/arch for the pinned host class — mysql57 is the pinned
  `linux/amd64` exception on arm64 hosts). Editing this file is a visible
  review diff, as with all baseline shrink-protection.
- `docker/ddl-golden-compose.yaml` uses digest-qualified image references
  for the four anchors (and the TiDB mysql client), so compose resolution
  itself cannot drift to a newer tag contents.
- `scripts/ddl_golden.py` proves identity at run level in
  `prove_anchor_identity`, after containers are healthy but **before any
  fixture SQL executes**: the manifest anchor's resolved RepoDigests must
  contain the locked digest, `docker image inspect` must return a
  non-empty image ID and platform matching the lock, and the running
  container's `.Image` must equal that image ID. Any failure is an
  `external_blocker` — the run stops rather than degrading.
- Run-level facts are merged into every case's `actual.database` record
  and re-checked by the offline validator (locked-digest match, non-empty
  image ID/platform, container image equality, locked platform match) so
  fabricated artifacts fail the same checks the live run enforces.
- The T02 syntax-negative case gains a manifest-declared
  `post_state_check`: the fixture object is proven absent before the
  broken statement and absent again after it, with recorded argv/rc/
  stdout/stderr for both probes.

## Deferred

- `platform` locks are the host-observed values for the pinned review
  host; a different-arch machine running the golden suite reports a
  visible blocker rather than silently passing (baseline edit = review
  diff).
- Inventory status-model migration (R2 §4.3) is a separate review
  artifact; no schema change ships until the reviewer approves it.
