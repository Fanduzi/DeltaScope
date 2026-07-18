# Design: MySQL/TiDB Builtin Semantic Manifests

## Goal

Allow a small, auditable subset of native MySQL and TiDB aggregate/window calls
to contribute to `read_only + admissible`, without treating a parsed function
name as proof and without weakening PostgreSQL's catalog-bound proof model.

The first supported target must include MySQL 5.7. MySQL 8.0, MySQL 8.4, and
TiDB 8.5 are independent targets, not aliases. No implementation may claim a
target/version has support until its row in the evidence ledger and its Docker
tests are complete.

## Non-Goals

- Proving arbitrary functions, stored functions, loadable/plugin UDFs, or custom
  SQL extensions.
- Trusting a name, a volatility class, an `EXPLAIN` string, a server version
  string, or a caller-supplied manifest by itself.
- Expanding PostgreSQL's trusted SDK boundary or changing its catalog/OID proof.
- Grant, RLS, masking, rewrite, execution-snapshot, or authorization analysis.
- Turning default SDK/CLI/HTTP into live database clients or adding an MCP Query
  Access tool.

## Target Profile Contract

Query Access receives an explicit optional analysis profile:

```text
empty       Preserve current behavior. Function-bearing MySQL/TiDB SQL is indeterminate.
mysql-5.7   Use only the immutable MySQL 5.7 semantic manifest.
mysql-8.0   Use only the immutable MySQL 8.0 semantic manifest.
mysql-8.4   Use only the immutable MySQL 8.4 semantic manifest.
tidb-8.5    Use only the immutable TiDB 8.5 semantic manifest.
```

The value must be a closed public enum or validated constants. It is an analysis
compatibility target, not a claim about a connected server. A PostgreSQL request
cannot select a MySQL/TiDB profile; a MySQL request cannot select `tidb-8.5`.
Invalid values and mismatches return bounded validation errors. There is never a
fallback to another profile or to name-based proof.

An empty profile preserves the existing public contract. This avoids silently
promoting previously indeterminate MySQL/TiDB queries in SDK, CLI, or HTTP.

## Proof Model

The manifest is an immutable program-owned table. It is keyed by:

```text
(dialect, target profile, canonical native function, call class, argument shape,
 window shape)
```

An entry contains at least:

```text
Dialect and profile
Canonical native spelling
Call class: aggregate or window
Permitted arity and argument shape
Permitted window feature subset
Semantic class: pure query evaluation
Primary documentation references
Evidence identifier/version and explicit negative boundaries
```

The application never accepts a manifest from a caller. Constructors deep-copy
any internal input, and accessors return copies. Tests must demonstrate that a
mutated source or returned slice/map cannot alter admission.

The proof root is the conjunction below, not any individual part:

1. Exact opt-in profile and dialect.
2. Immutable manifest entry for that profile.
3. Parser-native-form facts establishing the documented native syntax.
4. Complete candidate collection: every candidate in the statement is proven.
5. Complete strict base-table dependency requirements.

## Native-Form Facts

The parser must carry enough information for the semantic gateway to distinguish
a documented native call from a superficially identical identifier. Candidate
facts must not be reduced to a lowercased name before this decision is made.

Required facts include:

```text
Candidate ordinal and kind
Original spelling/classification as represented by the parser
Whether the call is qualified or quoted
Whether special-function syntax is canonical for the active grammar
Argument count and operand kind per argument
Aggregate modifiers: DISTINCT, FILTER, local ORDER BY
Window modifiers: OVER present, named window, partition/order expressions, frame
Nested function/cast/subquery/literal/parameter presence
Source-form ambiguity marker where parser facts cannot establish native form
```

Phase 1 only admits an unquoted, unqualified, parser-established canonical native
form. For MySQL functions affected by special-function parsing, forms whose
meaning may depend on unknown `IGNORE_SPACE` state are marked ambiguous and stay
indeterminate. The exact rule must be backed by MySQL 5.7 and each supported 8.x
profile's live parser probes. Parser behavior that cannot be represented faithfully
is a fail-closed reason, not an assumption.

## Admission Pipeline

```text
SQL request
  -> dialect parser and effect/dependency extraction
  -> candidate closure / unsupported traversal barrier
  -> strict relation and column metadata resolution
  -> profile validation
  -> per-candidate native-form validation
  -> immutable semantic-manifest lookup
  -> complete requirements check
  -> all-proven decision
  -> read_only + admissible, otherwise indeterminate
```

The existing classifier first rejects write/DDL statements. The semantic gateway
is only reachable for MySQL/TiDB read candidates with a non-empty valid profile.
Any candidate that is unknown, malformed, unsupported, or incomplete retains the
existing bounded function reason and forces `indeterminate`.

PostgreSQL continues through its existing controlled session, atomic catalog,
identity, type-pinning, and manifest gateway. There is no shared fallback that
could let a MySQL/TiDB profile affect PostgreSQL.

## Candidate Closure

The collector must enumerate effects in every expression-bearing location before
the gateway evaluates eligibility. Required locations include projection,
`WHERE`, `HAVING`, `GROUP BY`, `ORDER BY`, `DISTINCT ON` where applicable,
`LIMIT/OFFSET`, join conditions, derived tables, CTEs, scalar subqueries, set
operations, aggregate `FILTER`/local ordering, window partition/order/frame,
and nested expressions.

The collector must emit an explicit unsupported/unknown candidate when it cannot
prove traversal completeness. A boolean `hasFunctionCall` is not sufficient for
admission because it cannot show which calls or modifiers were present.

## Strict Dependencies

For each potentially admitted candidate, every argument, partition key, and order
key must resolve to a physical base-table column. Existing strict requirements
remain authoritative. The semantic gateway rejects candidate admission when any
of the following is present:

- an unqualified relation or unresolved/ambiguous relation/column;
- a view, CTE, derived output, wildcard, or missing metadata;
- a literal, parameter, `NULL`, cast, nested expression/function, or subquery
  operand;
- an aggregate modifier not explicitly allowed;
- a window frame, named window, or feature not explicitly allowed;
- a strict dependency omitted by traversal.

Projection-only mode must not receive stricter function promotion than strict
mode. If a profile feature requires strict dependencies, projection-only returns
indeterminate with a bounded reason rather than silently dropping dependencies.

## Initial Manifest Scope

All entries are provisional until Task 2 evidence is accepted.

| Target | Aggregate candidates | Window candidates |
|---|---|---|
| MySQL 5.7 | `COUNT(*)`; direct-column `COUNT`, `SUM`, `AVG`, `MIN`, `MAX` | none |
| MySQL 8.0 | same aggregate subset if separately evidenced | direct-column `ROW_NUMBER`, `RANK`, `DENSE_RANK` only if separately evidenced |
| MySQL 8.4 | same aggregate subset if separately evidenced | direct-column `ROW_NUMBER`, `RANK`, `DENSE_RANK` only if separately evidenced |
| TiDB 8.5 | independently evidenced subset only | independently evidenced subset only |

No `FILTER`, `DISTINCT`, explicit frame, named window, aggregate-local `ORDER BY`,
ordered-set operation, nested operand, or cast is in phase 1.

## Evidence Architecture

Every manifest entry needs two independent evidence classes:

1. Primary language documentation, captured in a versioned ledger with stable
   URLs, cited syntax, semantic conclusion, and known boundary.
2. Live Docker probes against the exact image used by the profile. A test must
   connect to the running server, assert version, execute positive native calls,
   execute all negative syntax/collision/qualification/SQL-mode probes that bear
   on the entry, and fail if the service is unavailable in required acceptance
   mode. Static structs are evidence snapshots, not executable proof.

The Docker matrix is MySQL 5.7, MySQL 8.0, MySQL 8.4, and TiDB 8.5. Each server
has its own probes and ledger row; observations may not be inferred across rows.
An unsupported or contradictory probe blocks the entry, not merely the test.

## Cross-Surface and Privacy Rules

SDK, CLI, and HTTP share the application result. They must agree for identical
SQL/profile inputs. A profile is public request configuration, but results and
errors must not expose internal manifest identifiers, parser facts, raw SQL,
literals, function names, session information, DSNs, credentials, driver errors,
effect candidates, catalog data, or `severity`.

MCP remains without a Query Access tool. Existing function-free MySQL/TiDB
admissibility and all PostgreSQL behavior must remain unchanged when the profile
is absent.

## Rejected Alternatives

### Keep all functions indeterminate forever

This is safe but unnecessarily excludes canonical language-defined read-only
aggregates. It does not match the product requirement for ordinary `COUNT`,
`SUM`, and narrow ranking-window analysis.

### OID/catalog identity only

Appropriate for PostgreSQL's richer catalogs but unavailable for the MySQL/TiDB
builtin surface. It cannot distinguish a fixed native syntax contract from an
arbitrary executable object.

### Runtime server version plus a name allowlist

Rejected. It makes default surfaces connect to databases, trusts unbounded server
claims, does not establish call form, and risks version/fork drift.

### Generic per-dialect function list

Rejected. It loses version and syntax semantics and cannot represent MySQL's
special-function parsing boundaries.

## Rollout and Rollback

The profile is opt-in. A discovered issue can remove an entry or profile in a
patch release; profile users become indeterminate rather than incorrectly
admissible. The empty-profile default remains unchanged, which makes rollback
bounded and preserves existing users' behavior.
