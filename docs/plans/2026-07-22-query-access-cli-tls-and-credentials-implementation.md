# Query Access CLI TLS and Credential Boundary Implementation Plan

**Status:** Proposed
**Date:** 2026-07-22
**Baseline:** `main@abb06eb`

## Preconditions

- Work on one milestone branch from current local `main`.
- Keep all task commits focused and do not push, tag, or release.
- Read the specification, design, and Proposed decision record before Task 1.
- Run GitNexus impact analysis before editing every production symbol. Stop and report any HIGH or CRITICAL impact.
- Use an untracked temporary mirror under `.omo/plans/` if Momus requires that path; never commit the mirror.

## Task 1: Characterize Current CLI Boundary

Add focused tests covering both `audit` and `query-access analyze`:

- `--password` and `-p` are absent from help and rejected as unknown flags;
- password sources are mutually exclusive;
- `--tls-mode` accepts only `disabled` and `enabled`;
- `--tls-ca-file` requires enabled TLS;
- enabled TLS rejects Unix sockets;
- malformed or unreadable CA files return bounded errors;
- default options preserve non-TLS direct online behavior.

Update the Proposed ADR only if characterization discovers behavior that contradicts it.

**Gate:** focused CLI tests.

## Task 2: Normalize CLI Connection Inputs

Introduce or extend a shared CLI connection-options function. It must:

- resolve one password source only after flags validate;
- parse a custom CA file into `*x509.CertPool`;
- keep raw secret values and source paths out of returned public errors;
- retain PostgreSQL `--database` separately;
- reject invalid TLS/socket combinations before opening a connection.

Do not add a plaintext password flag or a fallback alias.

**Gate:** focused CLI and online application tests, plus `go vet ./...`.

## Task 3: Thread TLS Through Both Online Commands

Wire normalized TLS settings to:

- Query Access online `SessionConfig`;
- audit metadata connection configuration;
- MySQL/TiDB per-connection connector configuration;
- PostgreSQL verified TLS configuration.

Preserve direct `tls.Config` propagation. Do not call `RegisterTLSConfig`. Ensure `InterpolateParams` remains a typed driver field, not a session SQL parameter.

**Gate:** adapter unit tests for TLS root pools, server name, PostgreSQL database mapping, and bounded error mapping.

## Task 4: Remove Plaintext CLI Password Input

Remove `--password` and `-p` from both command definitions, usage text, examples, docs, and tests. Retain only `--password-env`, `--password-file`, and `--ask-password`.

Search the repository for stale examples and exact flag registrations. The migration is intentionally breaking; do not add a temporary compatibility switch.

**Gate:** CLI help tests and repository search assertions.

## Task 5: Docker TLS CLI E2E

Extend the TLS fixture and scripts to run the compiled CLI inside the fixture network.

Required no-skip cases:

1. MySQL 8.4 audit succeeds with trusted CA.
2. MySQL 8.4 Query Access succeeds with a proven aggregate.
3. PostgreSQL 17 audit succeeds with trusted CA.
4. PostgreSQL 17 Query Access succeeds with `SELECT count(id) FROM app.users`.
5. For each dialect and command, untrusted CA fails with a bounded error.
6. For each dialect and command, hostname mismatch fails with a bounded error.
7. Submitted Query Access SQL never reaches a driver.

Make cleanup unconditional:

```sh
trap cleanup EXIT INT TERM
docker compose -f docker/tls-e2e/docker-compose.yaml down -v --remove-orphans
docker compose -f docker/tls-e2e/docker-compose.yaml up -d --wait mysql-tls postgresql-tls
make test-e2e-cli-tls
```

The actual compose service names may differ; update the plan and test target together if they do.

**Gate:** Docker E2E reports all required tests passed and zero skips. Verify no fixture container, network, volume, generated certificate, or private key remains after cleanup.

## Task 6: Documentation and Public Contract

Update English and Chinese CLI/reference documentation:

- show secure password sources only;
- show TLS disabled/enabled examples;
- explain CA roots and hostname checking in plain language;
- state that online Query Access analyzes metadata only and does not execute submitted SQL;
- distinguish direct CLI connections from HTTP `connection_id` registry connections;
- retain default offline behavior and non-goals precisely.

Update the decision record with concise evidence links. Do not weaken accepted HTTP or SDK boundaries.

**Gate:** documentation example and release-format checks.

## Task 7: Adversarial Review and Acceptance

Before changing the decision record to `Accepted`:

1. Have Oracle review the implementation/diff for TLS validation, credential leakage, error boundaries, SQL execution, and HTTP regressions.
2. Have Momus review the implementation plan through an untracked `.omo/plans/` mirror.
3. Use OMO `review-work` or an equivalent independent diff review. Do not treat a passing test suite as a review.
4. Fix every P0, P1, and P2 finding. Re-run the review after fixes.
5. Run GitNexus `detect_changes` before each commit and verify affected flows are limited to expected CLI/online metadata paths.
6. Record actual E2E commands, zero-skip results, and residual P3 risks in the ADR.

## Final Gates

Run at minimum:

```sh
go test ./... -count=1
go test -tags postgresql ./... -count=1
go test -race ./internal/interfaces/cli/... ./internal/application/online/... ./pkg/deltascope/... -count=1
go build ./...
go build -tags postgresql ./...
go vet ./...
go vet -tags postgresql ./...
golangci-lint run ./...
make query-access-corpus-gates
make pg-unit-test-gates
make test-e2e-http-tls
make test-e2e-cli-tls
make decision-record-gate
make release-gofmt-gate
npm test --prefix packages/deltascope-mcp
git diff --check
go mod tidy && git diff --exit-code go.mod go.sum
```

No release is part of this milestone. Do not push, tag, publish, trigger a workflow, rebase, reset, amend, or mutate stashes.
