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
- `cli_cases`: named CLI audit cases (`id`, `dialect`, `sql`, optional
  `args`, optional per-case `policy` profile selection, `expect`). Besides
  the base keys, `expect` may pin `coverage`/`statement_coverage`,
  `unsupported_features`/`unsupported_entries` (with exact bounded
  `metadata` maps), `statement_sql` identity, `evidence_gaps`/
  `evidence_gap_entries` (`index`/`rule_id`/`reason_code` plus exact
  `required_facts`), `finding_entries`/`finding_metadata`/
  `finding_locations`, and `fail_on_triggered`.
- `policy`: optional isolated-policy declaration. `policy.enable` maps rule
  IDs to `{enabled, level, params}`; the runner asserts each enabled rule
  exists in the live `rules list` catalog, renders the isolated profile
  under `policy_profile`'s name plus a generated `all-rules-disabled`
  profile, and records sha256 per policy file into the artifact. Per-case
  `policy` selects which generated profile a case pins. `policy.profiles`
  names additional isolated profiles; a `profiles` key equal to
  `policy_profile` or `all-rules-disabled` fails the run instead of
  silently redefining the default.
- `metadata_cases`: live-database audit cases. Each binds `anchor`,
  `dialect`, `sql`, a `connect` block (`password` travels only via
  `--password-env`/`--password-file` and is never recorded), ordered
  `setup` steps (with `verify` metadata assertions), the same `expect`
  vocabulary as `cli_cases`, `post_verify` assertions proving the audited
  object was not mutated, and `teardown` steps.
- `error_cases`: CLI invocations expected to fail before producing an audit
  result (for example a real connection refusal). `expect` pins `exit` and
  `stderr_contains` markers only; stdout is never parsed as a result. An
  anchored error case (`anchor` key) additionally records the live observed
  banner into `version_evidence`, and the validator requires it — a
  product/version mismatch can never be laundered into a connection failure
  or lack its observed identity proof (#83 T04-B).
- `metadata_cases` may declare `instance_facts` (`innodb_page_size`,
  `tidb_max_index_length`, `sql_require_primary_key`,
  `sql_generate_invisible_primary_key`,
  `show_gipk_in_create_table_and_information_schema`): the runner live-reads
  each declared fact from the anchor (`SHOW VARIABLES` for MySQL page size
  and the GIPK variables, `SHOW CONFIG` for TiDB `max-index-length`) and the
  validator rejects deleted, emptied, or tampered values — the manifest can
  never substitute for live evidence. Task-scoped auxiliary anchors (e.g.
  `mysql84-4k`, `tidb85-12288`) pin the fact in compose instead of the
  manifest so live reads prove it. GIPK facts prove a server-generated
  invisible primary key was not in play, so a driver-side CREATE cannot
  launder a server-added PK into input-declared evidence (T06-A1).
- `required_case_ids`: exact case IDs that must appear as executed in the
  artifact; any missing required case is a violation.

## Locked baseline

`anchors-baseline.json` is checked in **independently of any task manifest**
and pins the milestone denominator: the four anchors (`mysql:5.7.44`,
`mysql:8.0.46`, `mysql:8.4.10`, `pingcap/tidb:v8.5.0`) and the required CLI
dialects (`mysql`, `tidb`). The runner refuses a manifest that drops or
rewrites a baseline anchor, and the validator requires every baseline anchor's
`db_ddl`/`syntax_negative` cases and every baseline dialect's `cli_audit` case
to have executed — so shrinking a manifest and artifact together still fails.
Editing the baseline is a visible review diff, never a silent shrink.

Database steps and CLI audit are deliberately separate evidence: real
execution proves fixture legality and live metadata; CLI audit proves parser
and audit behavior. Neither substitutes for the other. The artifact validator
recomputes expectations from the manifest and results from raw evidence —
it rejects zero/missing/unexecuted cases, stale binaries, version mismatches,
external blockers, deleted or failed metadata-query records, non-JSON CLI
stdout, parsed/stdout disagreement, artifact-internal expected tampering,
missing or mis-attributed `evidence_gaps` entries (including a gap smuggled
into `findings`), tampered policy files or `enabled_rules`, leaked fixture
passwords in recorded commands, tampered metadata `setup`/`post_verify`/
`teardown` records, and manifests/artifacts shrunk below the locked baseline —
proven offline by `make ddl-golden-validator-test` (`scripts/test_ddl_golden.py`).
