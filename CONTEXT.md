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
A caller-chosen finding-level bar for process exit. Unresolved Audit Evidence
Gaps have warning-equivalent weight without becoming findings. It is not Verdict.
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

## DDL Coverage

**DDL Semantic Coverage**:
The extent to which a DDL form's relevant objects and options are understood and
its applicable checks have evidence. Parsing success or a generic notice alone
does not establish semantic coverage.
_Avoid_: parser coverage, finding count, fixture count

**Target Database Version**:
The database version against which an audit's version-dependent conclusions are
assessed. An unspecified target leaves those conclusions undetermined.
_Avoid_: parser version, latest version, DeltaScope version

**Version Identity**:
The canonical product/version fact an audit resolves for version-dependent
rules: `product`, canonical `version`, components, `source`, and
`validated_range`. Offline it comes from the strict `target_version` input;
online the provider-observed identity is authoritative — its product is
derived from the banner itself (a `8.0.11-TiDB-v8.5.0` banner is
`tidb/8.5.0`, never MySQL `8.0.11`), and a caller `target_version` may only
constrain it (a version or product mismatch is an input error, never an
override). The raw provider banner stays internal.
_Avoid_: server banner text, connection-reported version string, caller
dialect, caller guess

**Instance Fact**:
A normalized live-server configuration fact carried with explicit known bits
(`instance.innodb_page_size`, `instance.innodb_large_prefix_enabled`,
`instance.tidb_max_index_length`, ...). Zero, absent, or unparseable provider
values stay unknown — they are never defaulted into evidence. A fact that can
change a rule's outcome but was not observed produces an audit evidence gap.
_Avoid_: server default, assumed configuration, version fact

**Validated Version Series**:
The product/version series this milestone verified against versioned
documentation and live anchors: MySQL 5.7.x, 8.0.x, 8.4.x, and TiDB 8.5.x.
A syntactically valid version outside the series is not an input error — it
produces bounded version evidence gaps on version-dependent rules.
_Avoid_: supported version, newest version, parser-supported version

**Audit Evidence Gap**:
Missing facts needed to resolve an enabled, applicable audit check. It leaves
that conclusion unverified even when the operation's semantics are understood.
_Avoid_: policy violation, parser error, unsupported syntax

**Prospective Schema State**:
The schema facts implied by preceding operations in an audited batch, conditional
on their successful execution. Unknown effects leave affected facts unknown.
_Avoid_: live database state, executed migration, guaranteed final schema

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
