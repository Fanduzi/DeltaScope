# T06-A1-R1 evidence — validator identity binding (#85)

Validator code under test: `4e51cbe0d17b035ac950f16176b8cea7f2dacaa4`
(`fix(ddl-golden): bind T06-A1 case ids to frozen proof roles (#85)`).
Artifact under re-validation: `head_sha=06de7475a4f702fac8940d4b6b23c254b18a36a8`
—the original 32-case artifact is NOT rewritten; its evidence lives in the
sibling `06de7475…/` directory and is referenced, not copied.

## What the defect was

A required `case_id` was only a name: `t06_a1_artifact_failures` picked the
frozen spec by full ID, while generic dispatch re-selected by the record's
self-declared `kind`/`cli_case`. A passing mysql84 record could occupy the
tidb85 slot; an offline `cli_audit` record could occupy a `cli_metadata`
slot — with all 32 required IDs present.

## The fix

`t06_a1_identity_map()` freezes `case_id → kind / local id / dialect /
anchor / policy profile / input SQL` from `t06_a1_contract()`;
`t06_a1_identity_failures()` binds every executed record to it before
kind-based dispatch and rejects duplicate executed ids;
`t06_a1_manifest_failures` additionally rejects duplicated or unfrozen
manifest-declared ids.

## Layout

- `reproduce.py` — the full-entry rebinding check (imports the repo's real
  `ddl_golden.validate_artifact` with `verify_binary=True`; control first,
  then in-memory mutations; rc1 = bad evidence accepted, rc2 = environment
  failure). The reviewer's referenced `reproduce_full_validator.py` was not
  delivered with the work order; this script performs the specified check
  through the same complete entry.
- `red/` — run against a detached worktree at `0d083b89` (pre-fix
  validator): rc=1, six mutations accepted (mysql84→mysql57,
  mysql84→tidb85, offline demotion single, both profile swaps, duplicate
  case_id). Captured stdout/stderr/rc/rebinding.json.
- `green/` — same script against the fixed tree: rc=0, all eight mutations
  rejected with `identity` failures; plus `validate` (full validator entry)
  on the original artifact: `artifact valid`, artifact `head_sha` stays
  `06de7475`.
- `gates/` — captured argv/stdout/stderr/rc: validator contract
  (244 cases, 0 failures — 10 new R1 mutations), three-level doc check,
  decision-record gate, Python syntax check, task diff scope, HEAD/status.

## Boundaries

No production code, manifest, oracle, or product JSON changed; Go/PG gates
and the T02–T05 database suites are reused per the work order (validator +
tests + docs only). No database re-run was needed or performed.
