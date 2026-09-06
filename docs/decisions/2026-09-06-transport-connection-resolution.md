# Decision: One Transport Connection Resolution Path, Stop Before Open

- Date: 2026-09-06
- Status: Accepted
- Related: architecture review `docs/quality/architecture-review-2026-09-06.html`
- Related decisions:
  - `2026-08-12-query-access-online-analysis-entry.md`
  - `2026-08-17-cli-metadata-connection-exit-mapping.md`
  - `2026-08-30-cli-tls-metadata-error-categories.md`
  - `2026-08-30-cli-connection-error-categories.md`
  - `2026-08-30-cli-mysql-tidb-database-schema-alias.md`
  - `2026-08-30-query-access-mysql-tidb-schema-binding.md`
  - `2026-08-30-cli-postgresql-default-port.md`
- Related glossary: `CONTEXT.md` (Transport Connection Resolution, Connection Failure Class)

## Context

CLI, HTTP, and MCP each assemble database connections for metadata-aware Audit
and, on CLI and HTTP, for an Online Query Access Session. Password, TLS,
timeout, PostgreSQL default port, and MySQL/TiDB catalog alias leak across
those assemblers. Shared helpers cover host/socket/password only. MCP has no
TLS fields. Connection-failure mapping is four tables; CLI TLS phrases, HTTP
status codes, and MCP string matching already drift.

`CONTEXT.md` previously defined Transport Connection Resolution as CLI- or
HTTP-owned setup for an Online Query Access Session only. That did not match
the files that keep changing, especially CLI audit.

Audit opens a `*sql.DB` pool. Query Access pins one `*sql.Conn`. Those open
models are not interchangeable. Existing CLI TLS and connection-refusal
decisions freeze per-surface user-facing sentences and leave HTTP, MCP, and
Query Access mappings outside those contracts.

## Decision

Transport Connection Resolution is one shared path for metadata-aware Audit and
for an Online Query Access Session. CLI, HTTP, and MCP all use it.

The path stops before open. It owns input validation, password source
resolution, TLS configuration shape, timeout, PostgreSQL omitted-port default,
MySQL/TiDB catalog alias, and Connection Failure Class. It does not open or
close the connection.

Opening stays two existing paths:

- metadata-aware Audit uses the current pool opener (`auditmeta.Prepare`)
- Online Query Access uses the current pinned-session opener (`online.OpenSession`)

CLI's extra metadata `OpenClient` is leftover around `auditmeta.Prepare`. HTTP
and MCP already use the default opener. The CLI fork should go away rather than
become a third opener.

Connection Failure Class is shared. Each surface keeps its current user-facing
words, exit codes, and HTTP or MCP codes. This decision does not unify CLI TLS
phrases with HTTP or MCP.

MySQL/TiDB `database` and `schema` remain catalog aliases on this path for
both Audit and Query Access. PostgreSQL database and schema stay independent.

MCP stays on this path for Audit connections only. MCP still has no Query
Access tool. This decision does not add TLS fields to the MCP public contract.
The shared path must accept TLS configuration so a later MCP TLS change can
use it.

## Rationale

The recurring bugs are in the work before open, not in Audit versus Query
Access analysis. One path for that work gives one place to fix TLS, timeout,
catalog alias, and failure class. Two openers remain because a pool cannot
satisfy Query Access same-session proof, and pinning every Audit metadata load
is the wrong default.

Unifying user-facing sentences would reopen accepted CLI contracts. Sharing
only the failure class keeps those contracts and still stops four independent
classifiers.

Deferring MCP TLS avoids bundling a public MCP contract change into a
structure fix. Omitting TLS from the shared path would recreate today's hole
the next time MCP grows a TLS field.

## Public Contract

Unchanged by this decision:

- CLI metadata and Query Access error sentences and exits already accepted in
  the related CLI decisions
- HTTP status and code mapping
- MCP tool names, Audit-only Query Access absence, and current MCP connection
  fields (still no TLS)
- SDK Online Query Access Session construction from a caller-owned `*sql.Conn`
- no host, port, user, database, schema, DSN, password, driver text, path, or
  version in portable connection-failure output

## Deferred / Out Of Scope

- Adding TLS fields to the MCP public connection contract
- Unifying CLI, HTTP, and MCP user-facing failure sentences
- Merging the Audit pool opener with the Query Access pinned opener
- A single opener for all products
- Dual Observed Server Identity probes after `OpenSession` (separate leftover)
- SQL `--sql`/`--file`/stdin twins between `audit` and `query-access`
- An MCP Query Access tool

## Alternatives Rejected

- Keep Transport Connection Resolution as Query-Access-only: leaves CLI audit,
  the hottest connection file, outside the path that is supposed to own it.
- Two named connection modules, one for Audit and one for Query Access:
  repeats today's split under new names.
- Put both openers inside one connection module: mixes two ownership models
  and hides the pool-versus-pin constraint.
- Unify public failure sentences across surfaces: contradicts accepted CLI
  TLS and refusal contracts.
- Add MCP TLS in the same change: a public contract expansion, not required
  to make the shared path real.

## Verification Evidence

Implementation lives in `internal/application/connresolve`. `Resolve` owns
password, TLS shape, timeout, PostgreSQL omitted-port default, and MySQL/TiDB
catalog binding, and does not open. `Classify` is the shared Connection Failure
Class. CLI maps those classes to existing sentences; HTTP and MCP map open
failures through `Classify` then keep their existing codes. CLI metadata open
delegates to `auditmeta.OpenClient`. Unknown-dialect first-open catalog hint
lives in `auditmeta`, with PostgreSQL fallback using the original database.

Retained public-contract tests include `TestAuditConnectionRefusedExitsRuntime`,
`TestAuditTLSFailureCategories`, `TestPrepareUsesMySQLCompatibleDatabaseAsSchemaAlias`,
and `TestQueryAccessOnlineBindsMySQLTiDBSchema`. Module tests:
`TestBindMySQLTiDBCatalog`, `TestClassify`, `TestApplyPostgreSQLDefaultPort`.
