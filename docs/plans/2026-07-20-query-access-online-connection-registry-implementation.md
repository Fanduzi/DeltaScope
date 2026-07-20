# Implementation Plan: Online Query Access Connection Registry and Common Scalar Effects

## Objective

Implement the named HTTP connection boundary, online CLI/HTTP Query Access,
same-connection server identity validation, and a narrowly proven common scalar
function subset. Start from current local `main` on one milestone branch; use
focused commits and do not push, release, rebase, reset, amend, or mutate stashes
without explicit authorization.

## Global Safety Rules

1. HTTP accepts `connection_id` only. It never accepts direct endpoint,
   credential, secret-source, socket, TLS, profile, or version fields.
2. CLI retains direct local connection flags and closes connections it creates.
3. SDK, CLI, and HTTP share one session factory; identity and metadata use the
   same pinned connection.
4. Submitted SQL never reaches the database driver. Only fixed identity and
   metadata/catalog queries are allowed.
5. Unsupported SQL is `indeterminate`; configuration, authorization,
   connection, and identity failures are bounded errors.
6. No public output, error, or log leaks SQL, literals, API keys, passwords,
   secret-source names, file paths, endpoints, versions, DSNs, driver errors,
   manifests, candidates, session facts, or `severity`.
7. Run GitNexus impact analysis before every code-symbol edit. Stop and report
   any HIGH/CRITICAL impact. Before each commit run `gitnexus_detect_changes`.
8. Update relevant module READMEs and file headers with exported API/config
   changes. The decision remains `Proposed` until final acceptance evidence.

## Task 1: Commit the Proposed Boundary

Commit the spec, design, implementation plan, and Proposed decision record.
Add configuration examples that use placeholders only.

Verification: `make decision-record-gate`, docs-example gate, `git diff --check`.

Commit: `docs: define online query access connection boundary`.

## Task 2: Build the Runtime Connection Registry

Extend runtime configuration with named metadata connections and named API-key
identities. Implement strict startup validation for IDs, purpose values,
dialects, TLS modes, endpoints, secret-source exclusivity, and connection/key
allowlists. Resolve secrets only from operator configuration.

Extend HTTP authentication so a validated key maps to an internal principal ID.
Add registry authorization for `audit` and `query_access` purposes.

Verification:

- runtime-config matrix tests for valid multi-database input and every invalid
  configuration;
- auth/principal tests, allowlist denial tests, and auth-disabled tests;
- tests proving raw secrets and source names do not enter public errors/logs.

Commit: `feat(server): add named metadata connection registry`.

## Task 3: Migrate HTTP Audit and Repair its Error Boundary

Replace `/v1/audit` direct `connection` input with `connection_id`. Reject every
legacy direct field before secret lookup or dialing. Authorize the selected
connection for `audit`, then open a request-owned connection from its registry
definition.

Create a shared bounded online-error mapper. Replace audit's `err.Error()`
connection response path; preserve only existing independently safe diagnostics.

Verification:

- injected DSN/password/host/file-path/environment/raw-SQL/driver-error markers
  are absent from HTTP responses and logs;
- every legacy direct field is rejected and triggers neither secret resolution
  nor a dial;
- unknown connection, wrong purpose, unauthorized key, invalid key, and
  auth-disabled behavior are covered;
- migrate existing audit Docker E2E to configuration-backed IDs;
- CLI audit direct-connection behavior remains unchanged.

Commit: `fix(http): replace direct metadata connections with registry`.

## Task 4: Create the Shared Online Session Factory

Implement an internal factory that receives a pinned `*sql.Conn`, captures
server identity, creates the correct metadata resolver, and selects an internal
capability target. It must accept MySQL 5.7/8.0/8.4, TiDB 8.5, and PostgreSQL
17 only. It returns bounded sentinel errors for unavailable, unknown,
unsupported, malformed, or mismatched identity.

Delete caller-selected profile promotion from online paths. Deprecate the
offline SDK profile field, remove CLI `--profile`, and reject HTTP online
`profile`. An explicit dialect is an assertion only and must match identity.

Verification:

- version-parser tests for supported series, unknown products/forks, malformed
  strings, explicit dialect disagreement, and no-leak errors;
- driver tests prove one pinned connection serves identity and metadata;
- driver tests fail if submitted SQL, `EXPLAIN`, or prepare reaches the driver;
- PostgreSQL build-tag and PG17 trusted-path regression tests.

Commit: `feat(queryaccess): derive online capability from server identity`.

## Task 5: Add Online CLI and HTTP Query Access

CLI uses its existing local connection flags to build a short-lived session and
render the existing public result. With no flags it preserves offline behavior.
HTTP uses optional `connection_id`; without it it preserves offline behavior,
and with it it authorizes `query_access`, opens a request-owned session, and
uses the shared factory.

Both paths close their owned resources on cancellation/failure and reuse the
shared error/no-leak rules. Neither path executes the submitted SQL.

Verification:

- CLI tests for offline parity, ask-password, cleanup, identity errors, and
  secret-free stdout/stderr;
- HTTP tests for authorization, purpose isolation, cancellation, legacy-field
  rejection, safe errors/logs, and offline parity;
- Docker CLI/server E2E for MySQL 5.7/8.0/8.4, TiDB 8.5, and PostgreSQL 17;
- MCP surface contract remains unchanged.

Commit: `feat(queryaccess): add online CLI and HTTP analysis`.

## Task 6: Collect Common Scalar Candidates

Extend existing parser/application candidates for `LOWER`, `UPPER`, `LENGTH`,
`CHAR_LENGTH`, `ABS`, `CEIL`, `FLOOR`, `COALESCE`, `NULLIF`, and dialect-native
`IFNULL`. Preserve canonical/quoted/qualified/spacing facts, direct operand
shape, clause location, nesting, casts, literals, parameters, and unsupported
traversal state.

Cover projection, `WHERE`, `JOIN ON`, `GROUP BY`, `HAVING`, and `ORDER BY`, or
emit a fail-closed unsupported marker. All direct input columns must be strict
requirements. No literal, cast, nested, arithmetic, subquery, modifier, or
unknown form may promote.

Verification: parser/application matrices for every clause and negative form,
corpus fixtures for full dependencies, and aggregate/window/function-free
regressions.

Commit: `feat(queryaccess): collect common scalar effect candidates`.

## Task 7: Prove and Enable Scalar Effects Per Dialect

Extend PostgreSQL 17's catalog-bound manifest and MySQL/TiDB's versioned native
semantic manifests only for independently proven entries. PostgreSQL keeps OID,
type, and session binding; MySQL/TiDB keep native-form/profile proof. No dialect
inherits another dialect's entries; `IFNULL` never implies PostgreSQL support.

Each positive result must assert `read_only + admissible` and exact strict
requirements through SDK, CLI, and HTTP online paths. Each unproven row remains
indeterminate for that engine/version only.

Verification:

- primary-source ledger entry and immutable-manifest/candidate-binding tests for
  every shipped entry;
- non-skippable Docker E2E matrix for MySQL 5.7/8.0/8.4, TiDB 8.5, PostgreSQL
  17, CLI, and HTTP;
- no-leak tests for successes and all bounded-error paths.

Commit: `feat(queryaccess): admit proven common scalar effects` or focused
defer commits for entries lacking proof.

## Task 8: Documentation, Review, and Acceptance

Update SDK README, CLI/HTTP documentation, runtime configuration reference,
and EN/ZH Query Access guides in plain language. State that HTTP uses
administrator-configured IDs, CLI owns local direct credentials, and neither
path executes user SQL.

Run Oracle security/code review and Momus plan/diff review. If Momus requires
`.omo/plans/`, create only an untracked review mirror. Resolve P1/P2 findings
before acceptance.

Required final gates:

- `go test ./... -count=1`
- `go test -tags postgresql ./... -count=1`
- `go test -race ./internal/domain/queryaccess/... ./internal/application/queryaccess/... ./internal/interfaces/http/... ./pkg/deltascope/... -count=1`
- `go build ./...` and `go build -tags postgresql ./...`
- `go vet ./...` and `go vet -tags postgresql ./...`
- `golangci-lint run ./...`
- `make query-access-corpus-gates`
- `make pg-unit-test-gates`
- all required Docker CLI/HTTP/SDK integration suites
- `make decision-record-gate`
- `make release-gofmt-gate`
- `npm test --prefix packages/deltascope-mcp`
- `git diff --check`
- `go mod tidy && git diff --exit-code go.mod go.sum`

Before each commit run `gitnexus_detect_changes`; after the final commit refresh
the GitNexus index, preserving embeddings if present. Change the decision to
`Accepted` only with actual final evidence.
