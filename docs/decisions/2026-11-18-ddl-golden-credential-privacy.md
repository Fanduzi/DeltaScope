# Decision: ddl-golden evidence must not serialize database credentials (issue #81 R3)

Date: 2026-11-18
Status: Accepted
Related milestone/version: issue #81 T02 R3 under parent issue #79
Related commits: T02 R3 runner/evidence-privacy commit (this change set)
Related tests: `scripts/test_ddl_golden.py` (credential-scan contract rows), `make ddl-golden-validator-test`
Related docs: `testdata/ddl-golden/T02.json`–`T06.json` (`client_option_file` anchor spec)

## Context

The R2 Golden Runner embedded the disposable database password in the
manifest `exec_client` argv (`mysql -uroot -proot …`), which the runner
then serialized verbatim into `artifact.json` — nine plaintext
occurrences in the R2 evidence archive. Even fixture credentials must not
appear in evidence: archives are attached to reviews and may outlive the
fixture.

## Decision

- `exec_client` argv carries `--defaults-extra-file=<container path>` as
  the **first** client option (MySQL rejects it later in argv) plus
  non-secret flags only; the password never appears in argv.
- Each anchor that needs a password declares `client_option_file`
  (`user`, `password`, `container_path`). At run time the runner writes
  the option file to a host temp dir (`0700` dir, `0600` file), `docker
  cp`s it into the client container, chmods `0600`, deletes the host copy
  immediately, and removes the container copy during cleanup **before**
  `compose down`. Provisioning runs after anchor-identity proof and
  before fixture SQL, preserving the R2 identity-before-fixture order.
- The option-file path is recorded in evidence; its contents never are.
- `credential_leak_findings` scans argv, stdout, stderr, setup/cleanup
  logs, and the serialized artifact for `-p<secret>`,
  `--password=<secret>`, bare `--password`, `password=<secret>`,
  `MYSQL_PWD=<secret>`, and known fixture password forms. The run records
  a `credential_scan` object (surfaces + findings); the validator
  independently re-scans the artifact excluding the scan record itself
  and fails closed on any finding or on a missing scan record.
- Legitimate name-only flags (`--password-env`, `--password-file`) do not
  match the leak patterns, so future sanitized variants stay usable.

## Public contract

Test-harness contract only — no runtime/API surface. The frozen
`T06_ANCHORS` contract fixture was re-blessed for the new anchor shape;
all golden manifests (T02–T06) moved to option-file wiring so no future
run can serialize the credential.

## Verification

Live `make ddl-golden TASK=T02` green after the change (10 cases, 77
assertions); fresh `artifact.json` contains zero credential occurrences,
`credential_scan.findings == []`, option files observed `0600` in
containers and removed before teardown; validator contract suite
857 cases clean including fail-closed credential cases.
