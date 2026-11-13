# Decision: ddl-golden evidence must not serialize database credentials (issue #81 R3/R4)

Date: 2026-11-13
Status: Accepted
Related milestone/version: issue #81 T02 R3 and R4 under parent issue #79
Related commits: T02 R3 runner/evidence-privacy commit; T02 R4 evidence-hardening commit
Related tests: `scripts/test_ddl_golden.py` (credential-scan contract rows), `make ddl-golden-validator-test`
Related docs: `testdata/ddl-golden/T02.json`–`T25.json` (`client_option_file` anchor spec), `docker/ddl-golden-compose.yaml`

## Context

The R2 Golden Runner embedded the disposable database password in the
manifest `exec_client` argv (`mysql -uroot -p<fixture-password> …`), which the runner
then serialized verbatim into `artifact.json` — nine plaintext
occurrences in the R2 evidence archive. R3 moved the password into a
provisioned client option file, but the literal value `root` still sat
in committed manifests, the compose file, connect specs, and commit
diffs inside review archives. Even fixture credentials must not appear
in evidence: archives are attached to reviews and may outlive the
fixture.

## Decision

- The database password is an **ephemeral per-run secret**: the runner
  generates `RUN_SECRET` (`secrets.token_hex`) at `cmd_run` start, never
  stores it on disk, and injects it into compose as
  `DDL_GOLDEN_MYSQL_ROOT_PASSWORD` via the process environment. The
  compose file declares `MYSQL_ROOT_PASSWORD:
  ${DDL_GOLDEN_MYSQL_ROOT_PASSWORD:?…}` so `compose up` fails closed if
  the secret is absent; no literal password exists in the compose file.
- Healthchecks carry no credentials (`mysqladmin ping -h 127.0.0.1
  --silent`).
- `exec_client` argv carries `--defaults-extra-file=<container path>` as
  the **first** client option (MySQL rejects it later in argv) plus
  non-secret flags only; the password never appears in argv.
- Each anchor that needs a password declares `client_option_file`
  (`user`, `password_source: ephemeral-run-secret`, `container_path`).
  At run time the runner writes the option file to a host temp dir
  (`0700` dir, `0600` file), `docker cp`s it into the client container,
  chmods `0600`, deletes the host copy immediately, and removes the
  container copy during cleanup **before** `compose down`. Provisioning
  runs after anchor-identity proof and before fixture SQL, preserving
  the R2 identity-before-fixture order.
- `connect` and `env` manifest specs likewise use
  `password_source: ephemeral-run-secret` or the
  `${DDL_GOLDEN_RUN_SECRET}` sentinel, resolved to `RUN_SECRET` at
  execution time and never serialized. Recorded evidence keeps only
  non-secret credential metadata (user, password_source).
- `credential_leak_findings` scans argv, stdout, stderr, setup/cleanup
  logs, and the serialized artifact for `-p<secret>`,
  `--password=<secret>`, bare `--password`, `password=<secret>`,
  `MYSQL_PWD=<secret>`, known fixture password forms, and the live
  `RUN_SECRET` value itself. The run records a `credential_scan` object;
  the validator independently re-scans the artifact excluding the scan
  record itself and fails closed on any finding or on a missing scan
  record. Evidence archives additionally carry a post-pack archive-wide
  scan result, and commit diffs embedded in archives are redacted of any
  historical literal passwords.
- Legitimate name-only flags (`--password-env`, `--password-file`) do not
  match the leak patterns, so future sanitized variants stay usable.

## Public contract

Test-harness contract only — no runtime/API surface. The frozen
`T06_ANCHORS` contract fixture was re-blessed for the new anchor shape;
all golden manifests moved to `password_source`/`client_option_file`
wiring so no future run can serialize the credential.

## Verification

Live `make ddl-golden TASK=T02` green after the change (10 cases, 77
assertions); fresh `artifact.json` contains zero credential occurrences,
`credential_scan.findings == []`, option files observed `0600` in
containers and removed before teardown; validator contract suite clean
including fail-closed credential cases; the evidence archive's
post-pack credential scan reports zero leaks across all members.
