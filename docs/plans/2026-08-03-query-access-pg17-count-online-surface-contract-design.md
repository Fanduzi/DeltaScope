# Design: PG17 `COUNT(1)` Online Surface Contract

## Status

Proposed. This design validates and, only with evidence, publishes existing
transport delegation. It does not introduce a second PostgreSQL proof engine.

## Existing Facts

The online CLI path in `internal/interfaces/cli/query_access.go` and the HTTP
path in `internal/interfaces/http/query_access.go` both identify the connected
server through `internal/application/online`, obtain a pinned `*sql.Conn`, and
construct a `PostgreSQLQueryAccessSession` when the product is PostgreSQL. Both
then call `AnalyzePostgreSQLQueryAccessWithSession`.

That implementation fact is necessary but not sufficient for a product
contract. This milestone treats it as a candidate path that must be verified
under each transport's ownership and disclosure boundary.

## Target Flow

```text
CLI online flags                     HTTP connection_id
        |                                      |
local direct connection              operator registry + authorization
        |                                      |
        +---------- online.OpenSession --------+
                              |
                   one pinned PostgreSQL 17 connection
                              |
       NewPostgreSQLQueryAccessSessionFromConn
                              |
       AnalyzePostgreSQLQueryAccessWithSession
                              |
    exact COUNT(1) catalog-bound proof + strict requirements
                              |
        read_only + admissible + [app.orders/read_table]
```

The lower half is the accepted SDK proof. The upper halves are transport-owned:
CLI closes its local connection; HTTP resolves and authorizes only configured
connections, then closes its request-owned session. Neither accepts an
analysis profile as a promotion mechanism.

## Invariants

- The same `*sql.Conn` is used for identity, metadata/catalog work, and the
  PG17 session proof. A pooled or replacement connection is not acceptable.
- User SQL remains parser input. Database operations are limited to existing
  fixed liveness, identity, relation/column metadata, and catalog probes.
- The shared PG17 proof remains the sole authority for `COUNT(1)`. CLI and HTTP
  must not special-case literal text, copy catalog rules, or fabricate a type
  OID.
- The generic `COUNT(column)` path continues to use its normal catalog result;
  its shared manifest entry must not require dedicated `COUNT(1)` aggregate
  facts.
- Offline paths never open a connection. They preserve the current
  `indeterminate` result for `COUNT(1)`.
- HTTP request validation and authorization happen before secret resolution or
  dialing. Transport errors map to existing bounded public errors.

## Test Design

Use a task-owned PostgreSQL 17 Docker fixture based on the repository's
committed PG E2E compose/init files. Do not rely on an already-running shared
container or assert its health as proof.

For each online transport, test the exact positive query, exact table-only
requirement, and an exclusion matrix covering at least relationless,
non-`1` literal, cast, parameter, modifier, join, and unqualified relation.
Then prove:

- CLI offline and HTTP without `connection_id` remain indeterminate.
- HTTP rejects a `profile` together with `connection_id` and continues to
  reject direct endpoint/credential input.
- HTTP authorization failures do not dial and do not disclose configuration.
- The shared-session recording-driver proof remains required, but it cannot
  substitute for adapter-level evidence. CLI online and HTTP
  `connection_id` paths each require an observable transport-level test seam.
  The seam may be a test-only injected opener/dialer, recording driver, or
  controlled proxy chosen by the implementation, provided it observes database
  operations before and after the shared session boundary without defining a
  new production API contract.
- For each successful online path, the transport-level test observes at least
  one known fixed identity/catalog probe and proves that the submitted SQL's
  unique marker, `EXPLAIN`, and prepare operations never reach the driver or
  proxy. The fixed-probe observation prevents a no-execution assertion from
  passing vacuously.
- For HTTP rejected or unauthorized `connection_id` paths, the
  transport-level test asserts zero dial/open-session operations and no
  leakage of connection configuration or credentials.
- These adapter-level CLI and HTTP proofs, together with the shared-session
  proof, are mandatory before this ADR may change from Proposed to Accepted.
- Marker values are absent from output and diagnostics; HTTP access-log tests
  additionally assert a matching request log exists before scanning it.

## Rejected Alternatives

### Declare CLI and HTTP supported from call-graph inspection

Rejected. Connection lifecycle, build tags, configuration authorization,
bounded error mapping, and logs are transport behavior, not call-graph facts.

### Add a CLI or HTTP `COUNT(1)` flag

Rejected. It would create a caller-controlled proof switch and duplicate the
existing session-bound proof. Connected-server identity and the exact parsed
statement already define the safe boundary.

### Broaden the PG17 literal parser gate

Rejected. This milestone validates transport parity only. The accepted SDK
envelope remains deliberately narrow.
