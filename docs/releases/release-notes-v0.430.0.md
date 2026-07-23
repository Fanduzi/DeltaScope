# DeltaScope v0.430.0 Release Notes

## Summary - Secure Direct CLI TLS and Credential Hygiene

v0.430.0 adds an opt-in secure TLS mode for CLI direct connections for `audit` and `query-access analyze`. When TLS is enabled, the CLI validates the full certificate chain and exact hostname; these checks cannot be disabled. Plaintext `--password` and `-p` flags are removed. The only supported password sources are `--password-env`, `--password-file`, and `--ask-password`. CLI `--database` support is added for PostgreSQL target selection in both `audit` and `query-access analyze`. Query Access submitted SQL is not executed.

Default offline SDK, CLI, and HTTP behavior and MCP Query Access availability remain unchanged.

## What Changed

- CLI `audit` and `query-access analyze` direct connections support `--tls-mode enabled` with mandatory certificate-chain and hostname validation. `--tls-ca-file` is optional; when absent, system trust roots are used. `--tls-mode disabled` (default) intentionally uses no TLS.
- Plaintext `--password` and `-p` flags are removed with no compatibility switch. Supported password sources are `--password-env` (environment variable name), `--password-file` (file path), and `--ask-password` (interactive prompt).
- CLI `--database` flag added for PostgreSQL target selection in `audit` and `query-access analyze`.
- Query Access submitted SQL is analyzed but not executed. No SQL execution path is introduced.
- Decision record: `docs/decisions/2026-07-22-query-access-cli-tls-and-credentials.md` (Accepted; Related milestone/version: v0.430.0).

## What Stayed the Same

- Default offline SDK, CLI, and HTTP audit behavior is unchanged. No default path automatically enables TLS or changes credential handling.
- Query Access emits static requirements only. It does not authenticate callers, evaluate grants, enforce RLS, mask columns, auto-grant privileges, rewrite SQL, or guarantee a later execution snapshot.
- MCP tools remain `audit_sql`, `describe_rule`, `list_rules`, and `get_capabilities` only. No Query Access tool is added.
- The audit rule catalog and default audit behavior are unchanged. `level` remains the public audit priority field; no severity field is introduced.
- Query Access results do not include raw SQL, literals, function names, DSNs, credentials, driver errors, session data, endpoint addresses, or secrets.
- HTTP connection model and API key allowlists are unchanged from v0.420.0.

## Non-Goals

- Not SQL execution or data-returning APIs.
- Not arbitrary password submission. Only `--password-env`, `--password-file`, and `--ask-password` are accepted.
- Not database grant, role, RLS, or session-authorization evaluation. Not masking, rewrite, or execution-snapshot guarantees.
- Not SQL-mode attestation, arbitrary functions, UDFs, or broad function-name allowlists.
- Not an MCP Query Access tool.
- No severity field is added, and the registered audit rule catalog is unchanged.

## Rule Catalog Facts

The registered audit rule catalog is unchanged from v0.420.0. This release changes the direct CLI connection boundary: TLS, credential sources, and PostgreSQL target-database selection.

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

- `docs/decisions/2026-07-22-query-access-cli-tls-and-credentials.md` (this release)
- `docs/decisions/2026-07-20-query-access-online-connection-registry.md` (v0.420.0)
- MySQL/TiDB builtin semantic manifests (v0.410.0): `docs/decisions/2026-07-18-query-access-mysql-tidb-builtin-semantic-manifests.md`
- Common pure-effect Query Access (v0.400.0): `docs/decisions/2026-07-16-query-access-common-pure-effects.md`
- Trusted PostgreSQL SDK (v0.390.0): `docs/decisions/2026-07-12-query-access-pure-read-admissibility.md`
- Query Access foundation (v0.380.0): `docs/decisions/2026-07-11-query-access-analysis-foundation.md`
