# DeltaScope

DeltaScope statically determines database-operation and object-access effects
without executing the analyzed SQL.

## Audit

**Verdict**:
The quality judgment of a completed audit: `reject` when blockers exist,
`review` when warnings exist, `pass` otherwise. Notices do not change Verdict.
_Avoid_: success, CI status, exit code, fail-on

**Finding Level**:
blocker, warning, or notice on one finding. Only blocker and warning feed
Verdict.
_Avoid_: severity, priority, exit threshold

**Fail Threshold**:
A caller-chosen finding-count bar for process exit. It is not Verdict.
_Avoid_: verdict, CI verdict

**Rule Catalog**:
The shipped list of rules that discovery surfaces describe. It may include
rules that Default Policy does not enable.
_Avoid_: loaded rules, registry, default policy

**Default Policy**:
The enabled-rule set used when the caller supplies no config. It is a subset
of the Rule Catalog, not the Catalog itself.
_Avoid_: catalog, loaded

**Loaded**:
The statement-rule set actually registered for one audit (`rule_summary.loaded`).
Caller config can Load Catalog rules that Default Policy does not enable.
Suppression can omit Default Policy rules. Loaded is not the Catalog count.
_Avoid_: catalog, default policy

**Suppression**:
A Default Policy rule that stays enabled but is not Loaded because another
rule forbids the subject. Under the shipped baseline,
`ddl.table.foreign_key.forbid` suppresses the three
`ddl.constraint.foreign_key.name.*` rules with reason `fk_forbid`.
_Avoid_: missing, skipped, catalog gap

**Mutation Target**:
The table a DML statement writes. It is not every table named in FROM or JOIN.
_Avoid_: mentioned tables, FROM list

## Connection

**Transport Connection Resolution**:
The shared path that turns caller connection input into a ready-to-open
configuration for metadata-aware Audit and for an Online Query Access Session.
CLI, HTTP, and MCP all use it. It does not open or close the connection.
_Avoid_: Online analysis, Query Access proof, audit-only connection setup,
Query-Access-only connection setup

**Connection Failure Class**:
A bounded category of why a connection could not be used, such as
authentication, refused, timeout, or TLS hostname mismatch. It is not the
sentence a surface prints, and not an HTTP status or process exit.
_Avoid_: driver error text, exit code, HTTP status, stderr phrasing

## Query Access

**Online Query Access Session**:
An opaque wrapper over a caller-owned pinned database connection whose observed
server identity determines the supported dialect and analysis capability.
_Avoid_: Caller-selected online profile, transport connection

**Observed Server Identity**:
The product and capability facts derived from a pinned connection. Transports
that already identified the connection pass that identity into the Online Query
Access Session; they do not probe again. Callers may constrain these facts but
never supply them.
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
