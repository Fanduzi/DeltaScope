# Decision: Model View Definitions In The Existing Ordered Relation State

Date: 2026-11-01 (actual execution date, not a milestone slot date)
Status: Proposed
Issue: [#97](https://github.com/Fanduzi/DeltaScope/issues/97)
Parent: [#79](https://github.com/Fanduzi/DeltaScope/issues/79)
Contract: [T18](../work-plans/t18-view-lifecycle/CONTRACT.md)

## Context

The locked TiDB parser rejects vendor-valid MySQL ALTER VIEW and bare CHECK
OPTION. Its CREATE VIEW AST also defaults omitted clauses, including CHECK
OPTION, so AST enum values alone cannot prove an authored declaration.
The prior extractor recorded selected clauses as unaudited but discarded
definition dependencies. Ordered table projections did not distinguish views.

Views and tables share a persistent namespace, but view DDL ignores session
temporary-table shadowing while its SELECT sources do not. Reusing a generic
table transition without object-kind and role facts can mutate the wrong
object. Re-reading provider facts after a batch mutation can resurrect retired
dependencies or views from a dropped/recreated schema.

The four anchors additionally disagree on failed multi-target DROP VIEW:
MySQL 5.7 has partial effects, MySQL 8.x is atomic, and TiDB has ordered
prefix effects with a wrong-kind stop. This cannot inherit DROP TABLE's table.

## Decision

Use the existing ordered relation entries, namespace selection and provider
ownership. Add an observed relation kind and private view payload rather than
another schema executor, authoritative view cache, or transport-specific model.
Keep view absence separate from relation absence: a table may survive a view
DROP IF EXISTS. Preserve immutable pre-statement projections and publish only
the applicable conditional post-state.

Extract direct static dependencies through the existing structured query AST
analysis. Bind unqualified sources to the request schema, freeze star expansion
at definition time, and revalidate dependency/column premises through current
ordered entries. Replacement retires old edges only on the successful branch.
Missing definitions, missing source objects and provider errors are distinct.
No submitted SELECT or arbitrary expression value is evaluated.

Lexical CTE references share query identities, so every graph consumer,
including root attribution after a bounded assessment, must avoid repeated
expansion. Intrinsic exhaustion remains distinct from a missing catalog fact.
Named ordering references within individual SELECT scopes use the structured
binding assessment; set-result ORDER BY (including named outputs), LIMIT,
inline windows and unmodeled positional ordering retain explicit capability
boundaries rather than silently publishing successful view facts. Repeated
TiDB targets consume prior same-name effects while keeping not-view separate
from relation absence.

Use one policy-independent assessment for rule evidence and publication.
Independent native refusal dominates missing facts. Policy levels or blanket
forbids do not choose the modeled database effect. DROP VIEW uses its own
version/product transition table and preserves partial effects where observed.

Keep the locked parser dependency. Extend only the documented view productions
with a bounded typed grammar adapter, delegating the unchanged SELECT production
to the existing parser. Preserve statement operation and original source spans;
do not turn ALTER text into CREATE text. Ordinary supported CREATE continues
to use its real AST, with explicitness recovered from structured header tokens.
Malformed syntax remains parser_error; confirmed TiDB syntax boundaries remain
vendor boundaries rather than simulated support.

## Security And Public Contract

Separate authored declarations, observed catalog fields, required extra facts
and database authorization. A named definer is not account-existence evidence;
DEFINER/INVOKER risk descriptions do not grant or prove privileges. CHECK OPTION
and ALGORITHM consume the version-specific semantics in the contract, not merely
their strings. Recorded TiDB algorithm/check settings do not prove enforcement.

View definitions, principals and hidden transitive dependencies stay private.
New view-read errors use a bounded presentation while preserving their cause
and cancellation identity. Actual query failures remain errors and are not
cached as Unavailable; a normally returned invisible/NULL definition is a gap.
Findings and gaps identify only necessary authored objects and locations.

The shared Audit/SDK seam owns these semantics. CLI/HTTP/MCP retain their
existing parser-error, unsupported, coverage and threshold contracts. The
existing view-create blanket also governs MySQL ALTER definition changes;
PostgreSQL's existing operation behavior does not change. The new view semantic
RuleIDs and configurable default levels are frozen in the T18 contract.

Query Access keeps its existing view-backed promotion barrier. Reusing static
AST extraction does not expand its execution, privilege resolution or admission.

## Alternatives

- Treat valid ALTER as permanently unsupported: does not meet the accepted
  MySQL scope or resolve discarded dependencies.
- Rewrite ALTER into CREATE: loses statement identity and cannot establish
  declaration/source fidelity.
- Fork or broadly upgrade the parser: introduces a larger shared compatibility
  surface than these bounded productions require.
- Use a separate view state/cache: permits disagreement about shared names and
  schema invalidation, and can reload stale provider facts.
- Treat all DROP lists as atomic: contradicts the fixed 5.7 and TiDB receipts.

## Verification

The contract requires a real baseline public/CLI behavior red, first-path
Golden, then the complete seven-row dimension matrix. Native probes run only
fixed synthetic business DDL in disposable four-anchor fixtures. Internal
companion projection proofs are counted separately from CLI/database assertions.
Final gates include Query Access/privacy/transport, T16/T17 and PostgreSQL
regression coverage as well as the T18 Golden at the frozen product SHA.
Independent T18 acceptance is a later reviewer decision, not this ADR status.

## Boundaries

No database execution, query-result inference, authorization engine, account or
privilege changes, full query optimizer, arbitrary stored-routine expansion,
unrelated DDL family, release operation or T19 work is introduced. Four anchors
are bounded observations, not a claim about every patch or server configuration.
