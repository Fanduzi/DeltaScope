# T18 View Lifecycle And Security: Review Candidate

Issue: #97 under #79. Execution date: 2026-11-01.
Status: ready for independent review, not independent acceptance, merge or CI certification.

## Fixed Identity

- Accepted task base: `7f67c5de42fb6096d9c80f51e2d0f217eb34e528`.
- Runtime product: `879c49f58d8df02120875fffc901dc2d672210f0`.
- Final product, including test/doc compatibility repair:
  `2ab3f181f0bfefbeb5566a6b9358bdc030ab58eb`.
- Evidence identity: the appended commit containing this README; no self-referential SHA.
- Branch: `milestone/mysql-tidb-ddl-completion`.
- Task diff: accepted base through final product, 96 owned paths.
- Milestone scope is separate: see the recorded `origin/main...HEAD` path list.
  Recorded main is `9a28cc99c38770e6e40738515994f864166419f9`.
- Protected remote milestone: `d0c7cf3373167db7065d3027088a43c56ed6dbf5`.

The required [decision record](../../../decisions/2026-11-01-ddl-view-lifecycle.md)
is committed with the product. The
[contract](../../../work-plans/t18-view-lifecycle/CONTRACT.md) and
[dimension matrix](../../../work-plans/t18-view-lifecycle/MATRIX.md) describe
semantic premises and intentional boundaries. L3 headers are retired; affected
L2 module READMEs and committed-range documentation checks are included.

## Seven Inventory Rows

| Identity | Completed scope |
| --- | --- |
| mysql.create-view | CREATE/OR REPLACE, target kind/existence, source bindings, columns, dependencies, security and algorithm declarations, policy-independent state |
| mysql.create-view-check-option | Bare/LOCAL/CASCADED declaration, applicable updatability/refusal premises, inherited-algorithm gaps and distinct private check scopes |
| mysql.alter-view | Typed MySQL syntax, original identity/locations, inherited attributes, new/retired dependencies, exact refusal and uncertainty state |
| mysql.drop-view | Every ordered target, IF EXISTS, duplicates, temporary/persistent identity and 5.7 partial versus 8.x atomic effects |
| tidb.create-view | CREATE/OR REPLACE, dependency/security facts, CTE scope and explicit ignored-option warnings without enforcement claims |
| tidb.alter-view | Recognized vendor boundary, not supported policy evaluation or fabricated execution; CREATE OR REPLACE remains distinct |
| tidb.drop-view | Ordered partial effects, missing-target continuation, wrong-kind stopping, IF EXISTS, duplicate and unknown-prefix outcomes |

The six semantic surfaces and one vendor boundary retain all seven inventory
identities. Corpus, generated/embedded catalogs, defaults, example config and
English/Chinese public wording are synchronized. MySQL ALTER and bare CHECK
promotions preserve invalid SELECT and malformed-header controls.

## Final-SHA Evidence

All formal results here identify final product `2ab3f181f0bfefbeb5566a6b9358bdc030ab58eb`.

| Formal Golden | Cases | CLI/native assertions |
| --- | ---: | ---: |
| T16 regression | 60/60 | 832 |
| T17 regression | 52/52 | 834 |
| T18 complete suite | 149/149 | 3755 |

Required/executed sets match exactly, IDs are unique, every recorded assertion
passes, cleanup returns 0 and no task container remains. T16/T17 manifests are
byte-identical to the accepted base. T18 manifest SHA256:
`fef55d43bff8f0aa90e06fc55655afedbd6bf448a06e5ecfb2edee5d423ad3ac`.
Final CLI SHA256:
`419abd2721401ddc64d1003ddb5ed0fda71eb23c347c53447fa79b5bd7841820`.
Final T18 artifact SHA256:
`eb6ff447aa2a81f49ce031a113e80782cecfb6448de010ce52dbcd45c53a1f6e`.

The initial ALTER VIEW vertical slice passed 16 cases / 163 assertions; that
seed result alone was not completion. The full suite contains four native DDL
cases, four native syntax-negative cases, 28 offline CLI cases and 113
metadata-aware CLI cases. Companion Go assertions are a separate layer and are
not added to 3755. Private snapshots and successor state are checked through
the real parse/extract/enrich path, not inferred from native catalog queries.

All 23 gate entries ran with matching before/after final product identity:
21 returned rc0. The two exact lint commands returned rc1; comparison returned
rc0 with all 22 scoped / 39 fullrepo inherited diagnostic identities unchanged.
There are no new or removed identities. Lint is not claimed warning-free.

Full Go tests, vet, fresh internal race, canonical PG confidence, SQL/Query
Access corpus, inventory/catalog, Golden validator, parser/unsupported,
HTTP/MCP privacy, docs/examples, ADR, effective committed-range L2 and scoped
formatting checks pass. PG confidence actually runs CLI, HTTP and MCP E2E:
the CLI completes all 17 sequential cases; verbose HTTP/MCP E2E has no failed
or skipped tests. Tagged unit output is non-verbose, so its per-test skip
count is not claimed. See `gates/` and the lead-checked captures for exact
commands, output, return codes, source identity and artifact fingerprints.

## Behavior And Evidence Layers

The motivating valid MySQL ALTER VIEW no longer yields parser_error. It keeps
target v, source t, request-schema binding, original SQL/index/locations and
the correct conditional successor dependencies. Independent refusal retains
the prior object; unknown mutable effects withhold affected facts. Old provider
snapshots do not resurrect changed objects, definitions or dropped schemas.

Four native anchors are observed, not assumed: MySQL 5.7.44, 8.0.46,
8.4.10 and `8.0.11-TiDB-v8.5.0`. Their image digests and real SQL/catalog/
warning/cleanup receipts are retained. Probes use dedicated disposable objects;
no historical system-schema probe is rerun. Only the original T17 Golden's
explicitly authorized system-schema cases run in its original disposable fixture.

Public SDK/CLI tests assert exact findings/gaps, levels, locations, current
coverage, verdict and exit behavior. Companion tests assert private projection,
old/new dependency ownership and immutable snapshots. HTTP/MCP known-definition,
normal-unavailable and provider-error modes keep private sentinels out of
rendered output/logs. Real provider failures remain bounded errors with original
cause/cancellation identity; they do not become successful unavailable reads.

Declarations and catalog values never prove account existence, sufficient
privileges or database authorization. No submitted SELECT is executed by the
product. Query Access's existing view-backed promotion barrier remains.
LIMIT, inline windows, unmodeled positional ordering and all set-result ORDER
BY forms remain explicit query-model boundaries. Individual SELECT named ORDER
BY references/aliases bind structurally. General expression-value evaluation,
authorization, arbitrary query execution and T19 are not introduced.

## Failed Attempts And Preservation

Developmental red/green, replay provenance, native observations, WIP runs and
final-product runs are separately labeled. The first complete WIP run failed
three authoring errors: two incorrect vendor-feature labels and a denylist
fixture using an unsupported wildcard. Corrections retain the semantic outcomes
and positive privacy blocker; the original failed artifact/source are retained.
The first fixed-product broad gate attempt also failed stale CLI catalog tests,
a six-column proof fake driver and omitted auditmeta L2 synchronization. The
appended test/doc repair changes neither runtime logic nor historical evidence.
No failed receipt or original artifact is overwritten.

The 12 historical foreign roots, guidance worktree, historical T09-T17 artifacts,
17 protected running containers and 14 preserved volumes remain. Container
identity/running state/named mounts are checked. Foreign path listings match
the baseline; no historical content-byte hash baseline exists, so byte-for-byte
foreign-content preservation is not asserted. Canonical PG and Golden fixtures
are cleaned. The normal ignored PG-built `bin/deltascope` is not archived.

Publication authority is workfree-only normal fast-forward. No force push,
history rewrite, remote milestone/main update, PR, merge, issue closure,
release, deployment or T19 work is authorized by this evidence package.
Post-publication remote readback is a separate read-only phase; successful
publication is not the independent Reviewer's acceptance.
