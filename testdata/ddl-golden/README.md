# testdata/ddl-golden

Per-task manifests for the milestone DDL golden-path runner
(`make ddl-golden TASK=<id> ARTIFACT_DIR=<dir>`, `scripts/ddl_golden.py`).

## Manifest contract

- `task_id`: milestone task id (e.g. `T02`)
- `issue`: originating GitHub issue URL
- `policy_profile`: name recorded into the artifact for the isolated policy
  used by CLI cases (a temporary all-rules-off profile; the repository
  default policy is never modified)
- `description`: human-readable scope
- `anchors`: required database anchors keyed by name. Each anchor declares
  its compose `service`/`container`, pinned `image`, `product`,
  `version_contains` (matched against the server-reported version — a
  mismatch fails the run), target `database`, `exec_client` argv, and
  `needs_database_create`. A missing, unreachable, or unhealthy anchor fails
  the run; it is never skipped.
- `ddl_steps`: ordered DDL steps. Each step has `name`, `sql`, `expect_rc`,
  and `verify` assertions executed as live `information_schema` queries with
  expected scalar results (e.g. table exists, primary key exists, column
  exists, table absent).
- `syntax_negative`: `sql` that must fail for a syntax reason. `expect`
  carries `rc_nonzero`, `error_class` (e.g. `1064`), and `forbidden_markers`
  (connectivity/permission/missing-schema/collision markers that would
  disqualify the negative).
- `cli_audit`: `dialects`, the same `sql` batch, and `expect` on `exit`,
  `verdict`, `statements`, `findings`, `diagnostics`, `unsupported`.
- `required_case_ids`: exact case IDs that must appear as executed in the
  artifact; any missing required case is a violation.

Database steps and CLI audit are deliberately separate evidence: real
execution proves fixture legality and live metadata; CLI audit proves parser
and audit behavior. Neither substitutes for the other. The artifact validator
rejects zero/missing/unexecuted cases, stale binaries, version mismatches,
external blockers, and hand-written PASS records — proven offline by
`make ddl-golden-validator-test` (`scripts/test_ddl_golden.py`).
