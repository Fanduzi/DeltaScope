# Lead Incremental Review: T18 Full-Review Landing

Scope: all 22 paths in green/full-review-delta.json, all 1516 patch lines,
against the verified 85-path pause snapshot. The three contract/matrix/ADR
changes authored concurrently by the lead are included. This is an
incremental review after the earlier full product review, not a claim that
the full Golden or final gates have run.

## Production Verdict

The four production corrections are accepted. No new P1/P2 production
defect is identified in this landing.

- First-root attribution memoizes query identity, preserving deterministic
  physical-root selection while terminating on cycles and shared empty DAGs.
  The real 30-level public Audit test times out after five seconds before
  the fix, with the helper on the stack, and passes the same command after.
  The separate 36-level parser-depth control is not substituted for that red.
- SELECT ORDER BY reaches structured source/alias binding. LIMIT, positional
  ordering and inline windows retain intrinsic capability boundaries; an
  unresolved ALTER withholds mutable facts rather than preserving stale ones.
- TiDB repeated targets consume earlier remove/not-view effects. Duplicate
  refusal without IF EXISTS survives an unvouched initial target, while the
  independent gap and justified not-view post-state remain distinguishable.
- Definer clause detection validates the account prefix before selecting
  VIEW or SQL SECURITY. A dangling @ is rejected rather than normalized by
  the SHOW GRANTS grammar into an invented host.

Seven inventory owners preserve their identities and historical dated
evidence. MySQL ALTER and bare CHECK promotions are explicit; TiDB ALTER
remains a vendor boundary. English/Chinese policy descriptions agree.
The HTTP/MCP tests drive actual public Audit and actual transport encoding
with fake metadata only. Their six modes verify private-value omission,
normal missing-definition gaps, and non-swallowed bounded provider errors.
They are unit/transport proofs, not native database observations.

## Consolidated Completion Items

Set-result ORDER BY has an explicit boundary for every ordering form.
The previous named-ORDER prose was ambiguous about that scope. The lead
clarifies it to individual SELECT scopes and records named set-result
ordering as deferred query modeling, not a vendor refusal or parser error.
Add named set-order controls for both dialects and explicit current-statement
coverage assertions in the new public/transport tests before closing review.

The lead-owned planned manifest now has 149 unique required IDs, including
two named set-order controls; every previous 147 case object remains unchanged.
It is still a plan. No execution count or assertion total is inferred.

## Receipt Qualification

The first focused green attempt failed. It remains failure evidence:
unsupported diagnostics and the independent security warning required more
accurate test assertions; a dangling-definer-host acceptance was a real
additional grammar defect and received the production guard. Later focused
passes do not overwrite those receipts. Missing-fixture inspection rc1 is
absence/environment evidence, not a product regression.

The seed Golden remains 16 cases / 163 assertions only. Full T18,
historical T16/T17, final-SHA gates, lint identity comparison, product/evidence
commits and remote delivery have not yet been completed by this review.
