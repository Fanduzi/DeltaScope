# DeltaScope v0.440.0 Release Notes

## Summary - CLI TLS E2E as a CI and Release Gate

v0.440.0 promotes the CLI TLS end-to-end suite (`make test-e2e-cli-tls`) to an enforced gate. It now runs on every pull request and push to `main` through a dedicated GitHub Actions workflow, and it is composed into `make release-test-gates` so a release cannot be published without it passing. The suite uses Compose-assigned dynamic host ports and unique project names so it is safe to run in parallel, applies a fail-closed Docker policy, and is backed by a resource-cleanup regression harness. This is a testing and release-infrastructure release only.

No product, audit, or Query Access behavior changes in this release. Default offline SDK, CLI, and HTTP behavior and MCP Query Access availability remain unchanged from v0.430.0.

## What Changed

- The CLI TLS E2E suite (`make test-e2e-cli-tls`) runs as a required GitHub Actions gate on pull requests and pushes to `main`, and is composed into `make release-test-gates` invoked by the release workflow.
- The TLS fixtures use Compose-assigned dynamic host ports with unique Compose project names and container-name overrides, so runs do not collide with other services or parallel runs.
- A fail-closed Docker policy is enforced: in CI the suite fails if Docker is unavailable, and `--docker-optional` is rejected in CI or when the required flag is set.
- Fixture lifecycle is hardened: Compose teardown plus explicit leftover container/network/volume checks and temporary workspace removal, verified by a dedicated cleanup regression harness (`make test-e2e-cli-tls-regression`).
- The MySQL TLS fixtures use a TCP + TLS readiness healthcheck and readable server keys so the gate reflects real `TCP+TLS` connectivity on Linux CI instead of a Unix-socket false positive.
- Decision record: `docs/decisions/2026-07-22-query-access-cli-tls-and-credentials.md` (Accepted; updated with CI/release gate evidence for v0.440.0).

## What Stayed the Same

- Default offline SDK, CLI, and HTTP audit behavior is unchanged. No default path automatically enables TLS or changes credential handling.
- CLI TLS mode, credential sources, and PostgreSQL `--database` selection are unchanged from v0.430.0. Query Access submitted SQL is still analyzed but not executed.
- Query Access emits static requirements only. It does not authenticate callers, evaluate grants, enforce RLS, mask columns, auto-grant privileges, rewrite SQL, or guarantee a later execution snapshot.
- MCP tools remain `audit_sql`, `describe_rule`, `list_rules`, and `get_capabilities` only. No Query Access tool is added.
- The audit rule catalog and default audit behavior are unchanged. `level` remains the public audit priority field; no severity field is introduced.
- Query Access results do not include raw SQL, literals, function names, DSNs, credentials, driver errors, session data, endpoint addresses, or secrets.

## Non-Goals

- Not a new product feature. This release changes CI and release gating and test infrastructure only.
- Not a change to Query Access semantics, provable pure-function ranges, or literal-operand admissibility.
- Not SQL execution or data-returning APIs.
- Not database grant, role, RLS, or session-authorization evaluation. Not masking, rewrite, or execution-snapshot guarantees.
- Not an MCP Query Access tool.
- No severity field is added, and the registered audit rule catalog is unchanged.

## Rule Catalog Facts

The registered audit rule catalog is unchanged from v0.430.0. This release changes CI and release gating only.

| Metric | Count |
|--------|------:|
| Total rules | **371** |
| blocker | 72 |
| warning | 142 |
| notice | 157 |

| Dialect scope | Rules |
|---------------|------:|
| common | 177 |
| postgresql | 191 |
| mysql | 1 |
| tidb | 2 |

| Statement kind | Rules |
|----------------|------:|
| ddl | 361 |
| dml | 10 |

## Unchanged Metrics

- SQL corpus: **582/582**, **100.0%**, **247** YAML fixture files.
- PostgreSQL ALTER TABLE config entries: **53**.
- DDL coverage catalog: **400** entries (mysql 61, tidb 54, postgresql 285, parser_upgrade_candidate 18).

## Decision Records

- `docs/decisions/2026-07-22-query-access-cli-tls-and-credentials.md` (this release; updated with CI/release gate evidence)
- `docs/decisions/2026-07-20-query-access-online-connection-registry.md` (v0.420.0)
- MySQL/TiDB builtin semantic manifests (v0.410.0): `docs/decisions/2026-07-18-query-access-mysql-tidb-builtin-semantic-manifests.md`
- Common pure-effect Query Access (v0.400.0): `docs/decisions/2026-07-16-query-access-common-pure-effects.md`
- Trusted PostgreSQL SDK (v0.390.0): `docs/decisions/2026-07-12-query-access-pure-read-admissibility.md`
- Query Access foundation (v0.380.0): `docs/decisions/2026-07-11-query-access-analysis-foundation.md`
