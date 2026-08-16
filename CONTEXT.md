# DeltaScope

DeltaScope statically determines database-operation and object-access effects
without executing the analyzed SQL.

## Query Access

**Online Query Access Session**:
An opaque wrapper over a caller-owned pinned database connection whose observed
server identity determines the supported dialect and analysis capability.
_Avoid_: Caller-selected online profile, transport connection

**Transport Connection Resolution**:
The CLI- or HTTP-owned process that selects, authorizes, configures, opens, and
closes the database connection used by an Online Query Access Session.
_Avoid_: Online analysis, Query Access proof

**Observed Server Identity**:
The product and capability facts derived by an Online Query Access Session from
its pinned connection; callers may constrain these facts but never supply them.
_Avoid_: Requested dialect, caller identity

**Online Capability**:
The analysis behavior permitted by an Observed Server Identity and the
capabilities linked into a DeltaScope source build, independent of transport
configuration or authorization. Official DeltaScope release binaries link all
supported MySQL, TiDB, and PostgreSQL capabilities.
_Avoid_: Analysis profile, connection purpose

**Official Distribution**:
The CLI, server, and MCP binaries published by DeltaScope, all built with
PostgreSQL support. A source build without the `postgresql` tag is a supported
compile-time compatibility path, not a distinct official product edition.
_Avoid_: Default build, PostgreSQL-disabled product

**Effect Proof**:
Bounded evidence that every effect candidate relevant to a Query Access result
has semantics permitted by the active online capability.
_Avoid_: Function allowlist, parser trust

**Physical Requirement Completeness**:
The condition that every permission-bearing physical source implied by the SQL
is represented by the result's requirements before admission can be promoted.
_Avoid_: Successful parsing, read-only classification

**Promotion Barrier**:
A fail-closed condition that prevents proof from promoting a Query Access
result, including unresolved, unqualified, view-backed, or write behavior.
_Avoid_: Warning, proof failure

**Proof Applicability**:
Whether an Online Capability requires Effect Proof for the extracted candidates
before an otherwise indeterminate result may be promoted.
_Avoid_: Vacuous proof, empty candidate success
