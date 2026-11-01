# Full T18 Landing Review 1

Review base: previously reviewed rework sources (all saved SHA256 identities
verified) to the 85-path pause snapshot (all current identities verified).
Incremental scope: 51 changed paths, 5870 diff lines. Unchanged previously
reviewed paths were not reviewed again. Lead-authored contract, matrix,
seed manifest and privacy checks retain their independently authored oracles.

## Standards

Layering, private normalized models, shared policy-independent assessment,
immutable cloning, focused parser adaptation and L2 updates are consistent
with repository rules. No L3 header requirement applies. No unrelated project
code is changed. The new query module is a real internal seam, not another
generic schema executor.

One documentation defect: all seven owner:T18 inventory rows remain at old
evidence, including mysql.alter-view and mysql.create-view-check-option still
marked parser_unsupported. Corpus's reported 727/727 is not inventory closure.
Update only those seven rows, preserve historical evidence as history, add
current semantic dimensions and stable refs, and keep TiDB ALTER vendor_not_supported.

Minor wording: public rule/config/ADR descriptions say the blanket rule
evaluates MySQL/TiDB ALTER; TiDB ALTER is actually a vendor boundary and is not
evaluated as a supported view definition. Say MySQL ALTER, and retain the
separate TiDB boundary explanation.

## Spec: Blocking Static Findings Requiring Real Red Tests

1. Bounded query work is not closed over root attribution.
   viewQueryBuilder shares CTE query pointers through its lexical scope map.
   viewQueryFirstRoot recursively walks those shared graphs without a visited
   set. Zero-physical-source CTEs that double references form a small DAG with
   exponential traversal. evaluate() calls that helper after bounded child
   evaluation, even when the child has exhausted its budget. This bypasses
   the advertised intrinsic limits. Reproduce at the SDK seam with 36 CTE
   levels, two references per level, no physical source, all rules disabled;
   use a bounded Go test process timeout, never an unbounded timing run.
   Keep the helper signature, add per-walk identity-based memo/visited
   protection, preserve deterministic first physical root, and keep the
   existing intrinsic incomplete boundary without invented external gaps.

2. The simple SELECT path silently discards ORDER BY validation and accepts
   unaudited LIMIT/inline window constructs as fully modeled.
   needsViewQuery does not route ORDER BY/LIMIT; the simple extractor neither
   validates ORDER BY references nor rejects LIMIT. Inline WindowFuncExpr is
   ignored by both expression visitors, while only named WindowSpecs trigger
   a boundary. For example ALTER VIEW v AS SELECT id FROM t ORDER BY missing_col
   can publish a replacement definition rather than preserving proven-refused
   prior state. MERGE/check conclusions can also be fabricated for inline
   windows or LIMIT. Route ORDER BY to the existing structured query path and
   validate named references/aliases there. Keep LIMIT and window evaluation
   a narrow explicit unaudited-query-shape boundary rather than building an
   optimizer/updatability engine. Positional ordering not currently modeled
   should likewise remain an explicit capability boundary, not fake metadata.
   Preserve authored reference locations and test current plus successor state.

3. TiDB duplicate DROP without IF EXISTS loses a certain refusal when the
   common target is unvouched. DROP VIEW v,v must fail whether v is absent,
   a table, or a view (the first drop then makes the second fail). Current
   assessDropView only recognizes the second occurrence as absent when the
   first fate is proven ViewDropRemove. An unknown first target therefore
   yields only a gap and no target finding. Assert the duplicate occurrence's
   refusal independently of the retained existence gap; keep the current
   ordered partial effects. The same duplicate path also overwrites a proven
   not-view postcondition with generic unknown: an unknown first v produces
   ViewDropNotView, but its duplicate is adjudicated from the original nil
   binding and publishes ViewDropUnknown/NotView=false. Account for the
   previous occurrence's fate in the shared adjudicator, so v remains proven
   not-view after both conditional and nonconditional duplicate lists when
   no earlier uncertain prefix can prevent reaching it. IF EXISTS is a separate
   control and must not acquire the unconditional duplicate finding.

4. Account-boundary scanning stops on a legal unquoted keyword account name.
   parseViewHeader scans the named definer until any VIEW or SQL word, including
   the first username token. The cached MySQL 8.0.46 grammar includes VIEW_SYM
   among identifier keywords. ALTER DEFINER=view SQL SECURITY INVOKER VIEW v
   AS SELECT id FROM t is consequently sent back to the unsupported base ALTER
   parser instead of the typed adapter. Do not treat a username/host token as a
   clause merely because it spells VIEW/SQL. Select a clause boundary only
   after the existing structured SHOW GRANTS account production validates its
   nonempty preceding account prefix. Keep CURRENT_USER handling, malformed
   account controls, operation identity and raw source offsets unchanged.

## Questions Cleared, Not Findings

- ALTER omitted DEFINER inheritance is correct: cached versioned MySQL
  sql_view.cc fill_defined_view_parts sets lex->definer from the old view
  before create_default_definer can run. Do not change the frozen contract
  based on the later misleading comment.
- Canonical Golden compose_cleanup already uses down -v. Do not attribute
  the user's orphan anonymous volumes to this runner or add another cleanup
  framework. No pipeline cleanup change is needed.
- General updatability inference was explicitly deferred. Withholding an
  unobserved derived view updatability attribute is not itself a defect.
- No confirmed fake denylist metadata gap from intrinsic truncation was
  found on the actual projection paths; do not add speculative guards.

## Verification Status

Static review is complete, but these newly found paths do not yet have
executed red/green receipts. Existing full-package receipts do not cover them.
No final acceptance, frozen product SHA, full Golden, commit or push yet.
