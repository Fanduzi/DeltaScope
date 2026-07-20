# Design: Online Query Access Connection Registry and Common Scalar Effects

## Goal

Provide one safe online-analysis boundary for HTTP audit, HTTP Query Access,
CLI Query Access, and the existing session SDKs. HTTP selects only an
operator-managed connection. CLI keeps local direct credentials. All online
paths share server identity, metadata resolution, semantic proof, and bounded
error handling.

## Configuration and Authorization

Extend `runtimeconfig.Config` with named metadata connections and named API-key
identities. Unknown YAML fields remain startup errors.

```text
RuntimeConfig
  Logging
  Metadata
    ConnectTimeout
    Connections[]
      ID, Dialect, Host, Port, User, Schema
      PasswordEnv xor PasswordFile
      TLSMode
      Purposes[]
      AllowedAPIKeyIDs[]
  HTTP
    Auth
      Enabled
      Keys[]
        ID, SecretEnv xor SecretFile
```

Startup validation rejects duplicate IDs, invalid/empty endpoint fields,
unsupported dialect/purpose/TLS values, multiple secret sources, unknown
allowlisted key IDs, and malformed key/connection definitions. Secret values
are resolved only from sources named by operator configuration, never by HTTP
input.

The existing raw-key middleware is extended to map a valid key to an internal
principal ID. It stores only that ID in request context. The registry authorizer
checks:

```text
connection exists
AND requested purpose is enabled
AND (authentication disabled OR principal is explicitly allowed)
```

With authentication enabled, an empty allowlist denies access. With it disabled,
the operator has chosen trusted self-hosted mode and all configured connections
are usable.

## Connection Lifecycle

### HTTP

The HTTP handler resolves `connection_id`, authorizes its purpose, opens a
request-owned handle, obtains one pinned `*sql.Conn`, and passes it to the
shared session factory. It closes the connection and handle on success,
cancellation, timeout, or failure.

The decoder contains no direct connection fields. A legacy `connection` object
is rejected before secret resolution or dialing. `/v1/audit` and
`/v1/query-access/analyze` use the same resolver and error boundary.

### CLI and SDK

CLI opens and closes a local direct connection using its existing safe password
flags. The caller-owned SDK keeps its ownership model: the library never closes
the caller's connection. Both use the same session factory as HTTP, rather than
duplicating version parsing or metadata construction.

## Session Identity

Introduce an internal immutable `ServerIdentity` containing product family and
parsed major/minor series. It never appears in results, logs, or public errors.

The session factory runs only fixed identity queries on the pinned connection:

- MySQL/TiDB use a fixed version query and a parser that distinguishes TiDB from
  MySQL.
- PostgreSQL reuses/extends its pinned-session version fact and verifies the
  existing PG17 range before trusted promotion.

Identity maps to an internal target: MySQL 5.7/8.0/8.4, TiDB 8.5, or PostgreSQL
17. Query failure, unknown product/fork, malformed version, unsupported series,
or dialect disagreement returns a public sentinel error with a fixed message.
The observed version, endpoint, DSN, SQL mode, and driver error are never shown.

No SQL-mode query is needed. The parser/native-form gate admits only canonical
forms independent of `IGNORE_SPACE` and rejects spacing/comment variants.

## Analysis Paths

```text
offline SDK / CLI / HTTP
  -> parser and extraction only
  -> function-bearing query remains indeterminate

online CLI
  -> direct local connection -> shared session factory -> analysis

online HTTP
  -> connection_id authorization -> shared session factory -> analysis
```

The online factory and metadata adapters run fixed identity/catalog queries
only. User SQL is parser input. Driver-backed tests must fail if the submitted
SQL, `EXPLAIN`, or a prepare operation reaches the database driver.

## Scalar Effect Proof

Extend the current candidate representation, not a parallel function detector.
Candidates retain canonical form, quote/qualification state, arity, direct
column operands, clause location, modifiers, nesting, casts, literals, and
unsupported traversal state.

The collector must cover projection, `WHERE`, `JOIN ON`, `GROUP BY`, `HAVING`,
and `ORDER BY`, or produce a fail-closed unsupported marker. A manifest/catalog
proof entry is keyed by engine target, canonical native form, call class, arity,
operand shape, and permitted locations.

PostgreSQL retains its catalog identity, type binding, and PG17 manifest. MySQL
and TiDB retain their profile-specific canonical-native semantic proof. Each
direct input is added to strict requirements; `COALESCE(a, b)` requires both
columns. Any literal, parameter, cast, arithmetic expression, nested call,
subquery, modifier, or unknown candidate prevents promotion.

## Error, Logging, and Privacy Boundary

Create one shared online-operation error classifier for audit and Query Access.
It maps configuration, authorization, connection, identity, and unsupported
server failures to stable bounded messages/codes. HTTP never writes `err.Error()`
for an online failure.

Access logs contain only safe operational fields such as route, status,
duration, and request ID. They do not contain bodies, API keys, SQL, credentials,
secret-source names, file paths, endpoints, version strings, catalog SQL, or
driver errors. SDK errors preserve `errors.Is` sentinels; CLI prints bounded
setup failures without connection details.

## Rejected Alternatives

- **Redact direct HTTP inputs only:** rejected because callers could still cause
  server-side secret reads and outbound dials.
- **Require an enterprise secret manager:** rejected. Operator-owned environment
  variables and permission-restricted files support small self-hosted setups.
- **Trust caller-selected profiles:** rejected because they can mislabel a live
  instance.
- **Run submitted SQL or EXPLAIN:** rejected because Query Access is analysis,
  not SQL execution.

## Consequences

The HTTP input contract intentionally breaks: callers migrate from direct
connection objects to administrator-provisioned IDs. This is justified because
the old contract cannot enforce a sound credential and outbound-network boundary.
The new configuration model, startup validation, and operational examples are
therefore first-class public server documentation.
