# T18 Dimension Oracle

Frozen before sibling implementation, 2026-11-01. This is an expectation map,
not a claim that its rows have executed. The executable T18 JSON grows by
vertical slices; its first 16 required case IDs remain stable when extended.

## Review Hardening Regressions

These are required public/companion regressions, not completed-run claims:

- A shared zero-physical-source CTE DAG exhausts a bounded intrinsic budget,
  including with all rules off; root attribution cannot evade the budget.
- Named ORDER BY references and aliases in individual SELECT scopes bind
  correctly. A missing column produces the current source refusal and preserves the prior view for a
  successor; LIMIT, inline windows and unmodeled positional ordering remain
  explicit capability boundaries without fabricated metadata gaps. All
  set-result ORDER BY forms, including named outputs, retain that boundary.
- TiDB repeated unknown targets retain certain nonconditional refusal and
  justified not-view postconditions, separately from IF EXISTS and unknown
  prefix effects. No TiDB ALTER support is implied.
- A valid named definer spelled VIEW remains an account identity rather than
  a header delimiter; quoted and CURRENT_USER controls preserve their distinct
  declaration semantics.
- T18-R1 schema premise: a proven dropped schema refuses CREATE VIEW and never
  publishes the target; a never-touched schema remains unknown rather than
  absent, and source facts outside that schema survive.
- T18-R1 CHECK OPTION statics: computed-only projections and outer joins are
  proven non-updatable; direct, mixed, and inner-join controls remain valid.
- T18-R1-tail updatable proof: TEMPTABLE, static non-updatable shape, a
  non-mergeable UNDEFINED result, or a proven non-updatable source proves
  `Updatable=false`; unknown algorithm/source updatability never publishes
  `true`; a proven false dominates unrelated unknown premises.
- T18-R1 ALTER algorithm: omitted or explicit UNDEFINED inherits a proven
  prior MERGE, non-mergeable replacement falls back to final UNDEFINED, and
  the warning/state use one verdict.
- T18-R1-tail unresolved ALTER: an option attribute survives only when the
  proven-failure and possible-success values agree; differing values become
  unknown. A proven refusal still preserves the exact prior value, and a
  proven success publishes the final value.
- T18-R1 correlated scopes: nearest outer match wins, outer fallback works,
  true same-scope ambiguity still refuses, and unknown columns stay gaps.

## Profiles And Fixtures

- A: allow view lifecycle, enable target/source validity and security review.
- B: A plus existing create/drop blanket forbids (the default-policy behavior
  also has an independent public SDK check).
- T/S: isolate target/source validity respectively; all unrelated rules off.
- C: isolate CHECK OPTION applicability.
- G: isolate algorithm review.
- I: isolate TiDB ignored-option review.
- P: existing table denylist, including protected non-first targets and
  direct/transitive dependencies.
- O: all policy rules disabled; intrinsic boundaries must remain visible.
- L: selected subject rule at notice instead of its default level.

Unless varied: schema `g`, base tables `t(id INT)` and `u(id INT)`, view `v`
defined over `u`, view `w` defined over `t`; known-present/absent/unknown are
separate provider facts. Catalog security is DEFINER. Live versions are observed;
offline versions pass through the existing target-version input parser.

Complete/review with warnings exits 0 at blocker threshold. A blocker exits 1;
gaps alone exit 0 at blocker and 1 at warning. Recognized unsupported exits 1
even at none. Parser failure exits 2; wholly unparseable input retains empty
Verdict and zero statements. Every row also checks statement index and original
SQL. Clause/body-specific findings point to their original authored site;
statement-wide or inherited-property findings use the statement start. A
transitive dependency problem points to its authored root reference, never a
private external definition's coordinates or text.

## Parsing And Declarations

| Dimension | Oracle |
| --- | --- |
| CREATE VIEW, four anchors | supported; target and physical sources have distinct roles |
| CREATE OR REPLACE, present/absent view | supported; reset creation defaults, replace only successful-branch edges |
| ALTER VIEW, three MySQL anchors | supported; new definition, inherited definer/security and omitted/UNDEFINED algorithm |
| ALTER VIEW, TiDB 8.5 | confirmed vendor boundary; no fabricated successful mutation |
| DROP VIEW single/multiple/IF EXISTS | retain every authored target in source order, including repeated names |
| Explicit output column list | retain exact names; known count mismatch and duplicate labels refuse |
| DEFINER omitted/CURRENT_USER/named | distinct declaration kinds; named identity never proves account/rights |
| SQL SECURITY omitted/DEFINER/INVOKER | preserve presence and effective inheritance separately |
| CHECK OPTION omitted | MySQL declaration none, not the AST's default CASCADED |
| Bare CHECK OPTION | MySQL CASCADED; TiDB 8.5 confirmed invalid vendor form |
| LOCAL/CASCADED | both syntactically supported; do not equate their recursive enforcement scopes |
| ALGORITHM omitted/UNDEFINED/MERGE/TEMPTABLE | preserve presence, requested setting, inherited/recorded setting and execution-guarantee limits separately |
| Qualified/quoted target and source | keep identity boundaries and original locations; quoted dots are not schema separators |
| Comments/string literals containing clause words | not header productions; no keyword rewrite or false explicitness |
| Wrong SELECT, extra tokens, duplicated/reordered clauses | real parser_error, never a successful adapted statement |
| Valid sibling around bad statement | retained with original index/location and incomplete aggregate coverage |
| CTE, derived table, joins/subqueries/set operands | physical sources only; no CTE/alias object invention |

## Targets And Conditional State

| Input/premise | Current-statement oracle | Post-state oracle |
| --- | --- | --- |
| Plain CREATE, absent target and valid sources | A: security warning only; complete/review | present view, justified names/dependencies |
| Plain CREATE, present view or table | target.valid blocker; independent checks retained | exact prior object and attributes |
| OR REPLACE, present view | no target violation | old dependencies retired; new ones active |
| OR REPLACE, absent target | no target violation | created view |
| OR REPLACE, target is table | target.valid blocker | table unchanged |
| OR REPLACE, target kind/existence unvouched | narrow target gap, not an absent finding | no invented present view |
| ALTER, present view | A: effective-security warning as applicable | replacement definition with correct inherited attributes |
| ALTER, absent target | target.valid blocker | absent stays absent |
| ALTER, target is table | target.valid blocker | table unchanged |
| ALTER, unvouched target | target gap on this statement | affected facts withheld |
| ALTER, target definition invisible but kind/security known | no old-body prerequisite invented | new known definition may replace old unknown body |
| Unknown target plus proven missing source | target gap cannot hide source.valid blocker | proven refusal preserves exact prior target |
| Known refused shape plus other unknown options/facts | independent refusal remains a finding | no success publication |
| B instead of A | additional blanket blocker | same policy-independent state |
| O or L instead of enabled default level | no subject finding, or configured notice | same state, refusal and ownership |
| Same-name temp table shadows view DDL target | target is persistent relation | temp identity survives CREATE/ALTER/DROP VIEW |
| DROP DATABASE then recreate same schema | no provider resurrection | old views/edges absent in recreated namespace |
| Unrelated view after one unknown effect | analyze independently | no whole-batch contamination from a bounded view effect |

The DROP matrix in CONTRACT is normative. In particular:

- 5.7 bad/missing targets do not preserve other successfully dropped views.
- 8.0/8.4 proven atomic refusal preserves all prior target shapes.
- TiDB missing targets continue; wrong-kind targets stop, including IF EXISTS.
- TiDB unknown-prefix stopping possibilities make only affected suffix facts
  unknown. They do not erase a proven earlier prefix effect.
- An unknown MySQL version reports a current version gap when differing
  partial/atomic effects matter, not only a gap on a following statement.
- A narrower post-DROP "not a view" proof cannot become relation absence.
- P checks a protected target in any list position, regardless of native
  execution stopping or the blanket forbid also firing.

## Dependencies And Columns

| Dimension | Required proof |
| --- | --- |
| Known source table or view | current ordered existence/kind, not target-name metadata reused as source |
| Known absent source | source.valid blocker, never source-unverified |
| Unvouched source | current source.valid gap with `table.exists` |
| Present source, kind unobserved | `relation.kind` gap, not assumed table |
| External source view, missing/invisible definition | view remains present; `view.definition` gap |
| Returned definition that cannot be analyzed | unknown dependency set, not empty set or source absence |
| External unqualified dependencies without binding provenance | do not guess the original creator's current database |
| Source created earlier | consume batch-derived relation and columns |
| Source renamed/dropped earlier | old view body does not silently rebind or refresh from provider |
| Referenced source column dropped | dependent view is invalid, not absent |
| Source column modified | do not republish stale column attributes/types |
| Source ADD COLUMN after SELECT-star view | frozen view output list does not grow |
| Invisible source columns and SELECT star | do not silently include hidden fields |
| Unknown star expansion at creation | later metadata cannot invent its earlier expansion |
| OR REPLACE/ALTER old edge `u`, new edge `t` | dropping `u` no longer invalidates view; dropping `t` does |
| Direct/transitive self-reference | proven invalid definition preserves prior target |
| Dependency depth/size exhaustion | explicit current incomplete boundary; unrelated objects remain usable |
| Provider snapshot and earlier projection ownership | every relevant map/slice/view payload stays unchanged |
| Failed resolver/provider read | fail loudly, preserve cause/cancellation, never cache failure |

## Safety And Disclosure

| Check | Oracle |
| --- | --- |
| Effective DEFINER security | one security review warning; no authorization assertion |
| Explicitly named principal, including INVOKER | at most one security/ownership warning; never principal details |
| Effective INVOKER with no named declaration | no DEFINER-context warning |
| Omitted ALTER security, prior field unavailable | `view.security` gap, not default-to-DEFINER |
| MySQL TEMPTABLE | algorithm warning; published `Updatable=false`; a later CHECK OPTION refuses |
| MySQL MERGE plus aggregate | warning and fallback setting, not invented MERGE |
| MySQL CHECK plus known non-updatable definition | check_option.valid blocker; precise refusal state |
| MySQL CHECK plus missing applicability premises | narrow current check gap |
| Nested LOCAL/CASCADED | distinct private check scopes; no evaluation of WHERE predicate values |
| TiDB explicit algorithm/check options | one ignored-option warning per requested feature; settings are not enforcement proof |
| Normal NULL definition response | gap, not error or absence |
| Permission/timeout/cancellation/query failure | real safe-presented error with original cause, not normal Unavailable |
| Protected dependency reachable through external view | blocker on necessary authored identity, hidden object name omitted |
| Synthetic external-body/principal/literal/error sentinels | absent from findings/gaps/diagnostics/logs and public metadata |
| Caller-authored raw SQL | unchanged only where the existing report explicitly discloses it; do not copy it into other fields |
| Query Access over a view | existing view-backed promotion barrier remains; no new privilege/execution capability |

## Evidence Layers

Formal Golden assertions cover actual CLI/database behavior and fixed-ref
identity. SDK tests cover policy/default/custom-level/error results. Companion
tests cover the same SQL's internal ordered projections and immutability.
Native receipt matrices validate dialect/version distinctions, not product
projections. Existing historical artifacts remain at their original SHAs.
Archived replay source snapshots use a non-compiled suffix such as `.go.txt`;
the manifest records old-to-new paths and identical hashes so `go list ./...`
never discovers evidence as product packages. Append actual final
case/assertion totals only after a valid final-SHA run.
