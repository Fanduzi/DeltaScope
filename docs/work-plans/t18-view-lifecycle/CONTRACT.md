# T18 View Lifecycle And Security Contract

Frozen: 2026-11-01 (actual execution date). Issue: #97; parent: #79.
Base: `7f67c5de42fb6096d9c80f51e2d0f217eb34e528`.
T17/#96 is independently accepted. T18 does not reopen its B1/B2/B3 decisions.

## Scope And Proof Seams

The seven inventory owners are `mysql.create-view`,
`mysql.create-view-check-option`, `mysql.alter-view`, `mysql.drop-view`,
`tidb.create-view`, `tidb.alter-view`, and `tidb.drop-view`.
CREATE, OR REPLACE, ALTER, DROP, ordered multi-target lists, IF EXISTS,
column lists, qualified/quoted identities, DEFINER, SQL SECURITY, CHECK
OPTION and ALGORITHM are separate dimensions, not a seven-row completion count.

Primary seams: public AuditSQL/SDK and the real CLI driven by
`make ddl-golden TASK=T18 ARTIFACT_DIR=<independent-directory>`.
Companion proofs run the same SQL through real parse/extract/enrich to inspect
internal projections. They are not CLI assertions or native catalog assertions.
Parser/helper tests localize defects; they do not replace these seams.

First path: observed MySQL 8.0.46, a dedicated schema with `t(id INT)`,
`u(id INT)`, and `v` initially selecting `u`.
Input: `ALTER VIEW v AS SELECT id FROM t;`.
The isolated allow profile disables the two view blanket forbids and enables
the semantic checks below. Expect one statement, original SQL/index/location,
target `v`, direct dependency `t`, no parser diagnostic or unsupported item,
complete coverage, one warning from `ddl.view.security.definer.warn`,
Verdict `review`, and CLI exit 0 with `--fail-on blocker`.
A subsequent view reading `v` must use `t`, not retired dependency `u`.
Default policy additionally produces the view-definition blanket blocker.
Invalid SELECT remains parser_error. Known-absent source is a blocker;
unvouched source is a current-statement evidence gap, not an absent finding.

## Query Shape And Graph Budget Qualifications

Within an individual SELECT, named ORDER BY references and output aliases use
the same structured source binding assessment as projections and filters.
A proven invalid reference refuses publication; it does not retire an
existing view definition.
Set-result ORDER BY, including named output ordering, is not modeled in T18.
LIMIT, inline window functions and unmodeled positional ordering also retain a
narrow `view_query_shape` capability boundary. They must not become complete
merely because their AST parses, or become caller-resolvable metadata gaps.
An uncertain ALTER withholds its mutable definition and output columns.

Lexical CTE references can share query identities. Attribution, evaluation and
freezing must all remain bounded over that graph; an attribution helper cannot
expand a shared zero-source DAG exponentially after evaluation has stopped.
Exhaustion remains intrinsic and visible with all policy rules disabled.

TiDB duplicate DROP targets consume prior same-name effects. Without IF EXISTS,
a repeated target provides a refusal independently of unknown initial existence.
Retain independent evidence gaps and justified not-view postconditions; do not
mistake not-view for absent. IF EXISTS does not acquire an unconditional
duplicate refusal. An earlier uncertain prefix can still leave later effects
unverified.

A named definer token that spells VIEW is not itself a clause delimiter.
The account production must validate before selecting the following VIEW/SQL
boundary. These qualifications freeze required regressions, not executed
acceptance evidence.

## Primary Evidence And Vendor Boundaries

Exact anchors: MySQL 5.7.44, 8.0.46, 8.4.10, TiDB v8.5.0.
MySQL's versioned `sql/sql_yacc.yy` and `sql/sql_view.cc` are authoritative
grammar/transition sources; TiDB's `release-8.5` CREATE/DROP VIEW manuals are
the documentation source. Live receipts must accompany version-dependent
claims. The latest unversioned MySQL manual is not evidence for these anchors.

The initial fixed native scripts are `/tmp/t18_native_probe.py`,
`/tmp/t18_native_extra.py`, and `/tmp/t18_native_namespace.py`;
archive their unmodified source and raw receipts
with the final evidence. They create only dedicated business objects inside
the canonical disposable fixture containers. No system-schema DDL, user
queries, account creation, or privilege changes are part of these probes.

Verified native distinctions:

- All three MySQL anchors accept ALTER VIEW and bare/LOCAL/CASCADED CHECK
  OPTION. TiDB rejects ALTER VIEW and the bare form with 1064; its documented
  LOCAL/CASCADED forms parse but do not enforce view updates.
- TiDB accepts ALGORITHM declarations and records them, but the documented
  execution capability is MERGE-only. Recorded text is not proof of execution
  algorithm or updatability.
- MySQL rejects CHECK OPTION on TEMPTABLE/aggregate non-updatable views
  (1368); MERGE with an aggregate succeeds with a warning and records
  UNDEFINED. Never publish the requested MERGE as the observed algorithm.
- MySQL ALTER inherits omitted DEFINER and SQL SECURITY, and inherits the
  old algorithm for omitted or explicitly UNDEFINED ALGORITHM. Omitted CHECK
  OPTION becomes NONE. OR REPLACE instead resets omitted clauses to creation
  defaults. Missing prior attributes remain missing, without losing existence.
- Unqualified sources bind to the request's current schema, not the
  qualified target view's schema. All four anchors verified this distinction.
- Temporary sources (1352), missing source columns (1054), unequal explicit
  column counts (1353), duplicate output names (1060), self-reference, and
  invalid source views refuse creation. A known refusal preserves the prior
  target; it does not retire an old view definition.
- TABLE and VIEW share the persistent relation namespace. A table cannot be
  replaced/altered as a view. DROP TABLE and ALTER TABLE cannot mutate a view;
  DROP TABLE IF EXISTS on a view is a no-op.
- Additional dedicated-object controls confirm mixed DROP TABLE lists leave
  views intact: without IF EXISTS, MySQL 5.7/TiDB remove named base tables
  despite the view-name error, while MySQL 8.0/8.4 preserve the whole list.
  IF EXISTS removes the base tables and skips views on all four anchors.
  This does not replace the distinct DROP VIEW rules below.
- RENAME TABLE does not update a stored view's old source reference. A source
  MODIFY can change a view's observed column type. Base-column defaults,
  nullability and AUTO_INCREMENT are not uniformly view properties across
  these products; conditional derived output names must not clone the whole
  source column. Unknown type facts are withheld rather than stale.
- Nonrecursive CTE definitions are accepted by MySQL 8.0/8.4 and TiDB 8.5;
  MySQL 5.7 rejects that syntax. Named definers without a host and unquoted
  dotted hosts are accepted; reserved/numeric username atoms are rejected.
  Exact follow-up native scripts and receipts remain separate from product
  projections and must be archived unchanged.
- CTE name resolution is lexical: a nonrecursive CTE definition can read a
  same-named persistent base table, and a forward CTE name is not visible.
  MySQL 8.0/8.4 ignore unused CTE bodies for source/column validity and omit
  them from stored definitions; TiDB validates them, rejecting missing sources
  or columns even when unused. Required dependencies and metadata reads follow
  that proven vendor distinction, not a global CTE-name filter.

### Ordered DROP VIEW

Do not reuse DROP TABLE's atomicity.

| Anchor | Missing target, no IF EXISTS | Base-table target | IF EXISTS + base table |
| --- | --- | --- | --- |
| MySQL 5.7.44 | error; existing named views still dropped | error; existing named views still dropped | skip table; drop views |
| MySQL 8.0.46/8.4.10 | atomic refusal; all targets unchanged | atomic refusal; all targets unchanged | skip table; drop views |
| TiDB 8.5.0 | report missing; continue dropping named views | stop at table; prefix effects persist, suffix unchanged | same stopping behavior |

Duplicate DROP VIEW names refuse before effects on MySQL (1066); TiDB processes
them in order (the first drop persists, the second missing name reports 1051).
Rules inspect all named targets even when native execution stops earlier.
An unknown target may stop TiDB's suffix: merge both possibilities rather than
pretend the suffix certainly executes. Unknown MySQL version must merge 5.7
and 8.x effects wherever their outcomes differ.

## Normalized Facts And Ownership

Reuse the existing ordered relation entries and persistent/temp namespace;
do not add another generic schema executor or a second authoritative cache.
Add `RelationKind` and a private view payload to `TableSnapshot`. Empty kind
means unobserved, not table. Batch-created tables explicitly have kind table.
View payloads distinguish provider-observed catalog fields from authored
declaration facts and conditional batch-derived fields.

View DDL targets always use the persistent namespace, even under a same-name
session temporary table. Query sources remain temp-first; using a temporary
source refuses view definition. A DROP VIEW must not drop the temporary table.
All four anchors verified CREATE/DROP shadowing; ALTER shadowing is valid on
the three MySQL anchors and retains TiDB's ALTER boundary.

`DDL.View` retains OR REPLACE, explicit column names, algorithm/security/check
values and presence flags, and definer kind. Named principal details and
definition SQL are private, never public finding metadata. Direct dependencies
are extracted from structured query AST; CTE and derived aliases are excluded.
Dependencies are schema-bound once at definition time. Preserve source order
and independently clone every collection handed to another statement.

Freeze SELECT-star expansion against the creation/replacement-time source
columns, excluding invisible columns. Later ADD COLUMN must not enlarge an
existing view. Later DROP of a referenced column makes a dependent definition
invalid, not absent. The four anchors' star/add/drop control confirms this.
Never refresh a batch-owned view from an older provider snapshot to repair it.

Each view statement gets immutable ordered target/source projections. A
present view does not imply its definition is visible or valid. A nonempty
external definition that cannot be analyzed has unknown dependencies, never
an invented empty dependency set. A missing definition is not a missing view.
Only named columns with a supported static derivation may be published;
arbitrary expression values and query results are never inferred.

The view-specific negative fact "not a view" may survive DROP IF EXISTS even
when the shared relation kind/existence remains unknown (a table could remain).
Do not publish relation absence from that narrower fact.

Revalidate dependencies through current ordered entries on every use:
table/view creation, modification, rename, deletion, schema drop/recreation,
and view replacement must not resurrect old provider/cache facts. New
definitions retire old dependency edges only on the applicable successful
branch. Missing/invalid/recursive sources refuse; missing evidence withholds
only affected conclusions. Unrelated objects remain analyzable.
Bound external dependency traversal to depth 32 and 256 distinct relations;
exhaustion is an explicit incomplete analysis boundary, not silent completion
or a metadata gap that a caller could resolve by supplying the same facts.
This boundary invalidates only the affected view, not unrelated batch objects.

## Shared Assessment And Rules

One policy-independent view assessment supplies both rules and state
publication. Any independent proven refusal dominates missing evidence.
Policy blockers are not native refusal and never decide publication.

Reuse `ddl.view.create.forbid` for CREATE/OR REPLACE and MySQL/TiDB ALTER
definition changes; default blocker, configured level respected. PostgreSQL's
existing operation contract is unchanged. Reuse `ddl.view.drop.forbid` and
`ddl.table.denylist.forbid` (all targets and view dependencies; existing
`tables` parameter). Hidden transitive identities must not be disclosed.

Freeze these new IDs (no parameters; configured levels override defaults):

| RuleID | Default | Meaning |
| --- | --- | --- |
| `ddl.view.target.valid` | blocker | duplicate CREATE, missing/wrong-kind ALTER/DROP, duplicate MySQL DROP targets; ordered per-target checks |
| `ddl.view.source.valid` | blocker | missing/invalid/temporary/recursive source or a provably invalid column binding |
| `ddl.view.security.definer.warn` | warning | effective DEFINER security or an explicitly named principal needs separate privilege/ownership review; never account/authorization proof |
| `ddl.view.algorithm.warn` | warning | requested TEMPTABLE update restriction or known MERGE fallback on MySQL |
| `ddl.view.check_option.valid` | blocker | CHECK OPTION's statically proven MySQL updatability prerequisite is violated |
| `ddl.view.options.ignored.warn` | warning | TiDB CHECK OPTION/ALGORITHM declarations cannot provide requested enforcement/algorithm guarantees |

Gap reason remains the existing bounded `unknown_table_state`; RequiredFacts
use the narrow missing names `table.exists`, `relation.kind`,
`view.definition`, `view.dependencies`, `view.columns`, `view.security`,
`view.algorithm`, `view.updatable`, or `database.version`, as applicable.
Gaps belong to the enabled applicable RuleID and the current statement.
Disabled rules remove their findings/gaps, never intrinsic vendor/parse
boundaries or policy-independent state effects. No findings is not completion.
Name-based denylist checks need their own dependency gap when an external
view's unknown definition/dependencies prevents completing that enabled check;
they do not require unrelated column facts. Gap review floors are not finding
counts. Checks requiring a prior view attribute are not applicable to a
proven absent/wrong-kind ALTER target, unless an authored clause independently
creates a declaration-level check.

The new rule/dialect pairs are precise: target/source/security apply to MySQL
and TiDB; algorithm/check apply to MySQL only; ignored-option applies to TiDB
only. No new rule remains a final placeholder or gets a corpus exemption.

## Parsing And Privacy

The locked parser is `26db2d85d6cd`. It has no ALTER VIEW production and
requires an explicit CHECK OPTION modifier; its CREATE defaults collapse
omission and explicit defaults. Use existing structured AST wherever possible.
A bounded view grammar adapter may parse the documented header productions
and delegate the untouched SELECT production to the locked parser. It must
produce a typed view statement with its own operation identity and original
source spans. Never rewrite ALTER to CREATE, replace keywords, use regex
recognition, or mistake text inside comments/strings for clauses.
The adapter must consume the whole statement, reject extra/duplicate/reordered
clauses, and preserve quoted identifiers, original SQL, indexes and locations.
Shared parser dependency upgrades require a separate compatibility decision.

TiDB ALTER and bare CHECK OPTION are explicit confirmed vendor syntax
boundaries, not supported semantics. Truly malformed SELECT/header syntax is
parser_error on both products. A real boundary does not become complete merely
because blanket policy is disabled.

Query Access remains view-backed/indeterminate under its existing promotion
barrier. Static DDL dependency extraction adds neither execution nor permission
admission. DEFINER declarations, catalog security fields, risk explanations
and runtime authorization are four different facts.

Actual metadata errors propagate with their cause/cancellation identity intact;
failed reads are not cached. New view reads wrap untrusted error text in a
bounded safe error presentation, not a normal Unavailable response. Normal
NULL/invisible/unprovided definitions remain fact gaps. No privilege-error
swallowing or new authorization classification is introduced.
For the attribute-only VIEWS lookup after TABLES has identified a view, a
successful empty row set (`sql.ErrNoRows` from QueryRow.Scan) means attributes
were not provided, not a failed query or relation absence. This normalization
does not apply to errors returned by the provider/resolver interface itself.

Findings/gaps/diagnostics/logs expose only necessary authored object identities
and source locations. Never emit external definition text, literal sentinels,
hidden dependency names, named-definer principals, or raw provider error text.
Use synthetic sentinels at public SDK/CLI and transport privacy seams.

## Completion And Delivery

Advance first path red -> green before extending sibling dimensions. Permanent
public behavior tests must compile on the accepted baseline; a compiler failure
from new fields is not the red proof. Freeze every added oracle before observing
product output. Native catalog assertions and internal projection assertions
are separately identified.

After the full seven-row dimension matrix: review the complete diff, synchronize
inventory/corpus/catalog/rules/default policy/config/ADR/L2/EN-ZH contracts, then
freeze the product SHA. Run final Go test/vet/internal race, PG unit/confidence,
Query Access and privacy/transport regressions, corpus/inventory/catalog/parser
and unsupported/docs/ADR/committed-range L2, and T18 Golden on that SHA.
Run T16/T17 regressions and expand historical regression only for shared impact;
retain historical manifests/artifacts and explain supported-boundary upgrades.
Compare exact scoped/full lint diagnostic identities against 22/39, not counts.

Append separate product and evidence commits. Preserve all history, external
files/worktrees/containers. Publish only an ordinary fast-forward via
`git push origin HEAD:refs/heads/workfree` after fixing the delivery SHA.
Read back refs, parent chain and fixed-ref evidence files. Never move milestone,
force, create a PR, merge, close issues, release, deploy, or enter T19.
End at the T18 independent-review candidate, not accepted/merged/CI-green.
