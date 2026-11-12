# Unresolved items + stop-condition exposure

## Open questions (all proposed-probe scope, none blocking)

1. **80-1** absent-server native error class — deferred since R5 P5;
   the A2 refusal rests on ordered-absent proof only, so this is
   oracle-grade optional evidence.
2. **80-2** duplicate-key native resolution (last-wins vs error) —
   `[PEND]` since R2. Product treatment (owed-fact only) is correct
   either way.
3. **80-3** empty-string option acceptance — parser admits `''`;
   native outcome unknown.
4. **80-4** PORT boundary behavior (`0`/`9999`/`10000`/quoted/negative).
5. **80-5** `mysql.servers.Server_name` collation + case-fold of
   ALTER identity — affects only whether "differently-cased" entries
   could ever be proven same; current conservative treatment stands
   regardless.
6. **80-7** privilege-vs-existence check order.
7. **80-9** merge-vs-replace post-state confirmation.
8. **57-1..5** whole 5.7 matrix — 5.7 page was never fetched; every
   5.7 claim is family-inference until probed.
9. **80-11** grant-scoped `mysql.servers` visibility vs the ALTER
   check — does a hidden-from-observer row still refuse the ALTER?
   (Same visibility-scoping question the SRS dependency round
   answered YES for the catalog view — but for writes, does MySQL's
   existence check also use the caller-visible domain? If yes, an
   ALTER could land on a row the observer can't see — materially
   identical hazard to the dependency finding.)

## Stop-condition assessment (work order G)

| condition | triggered? |
|---|---|
| parser grammar change needed | NO — recognizer already claims the documented shape |
| generic SERVER lifecycle needed | NO — family reuse stands |
| ALTER identity/post-state unprovable | NO — presence is provable on the same bounded read; option-attribute truth is deliberately unclaimed (not a defect) |
| must change accepted CREATE/DROP semantics | NO |
| generic privilege supply contract change | NO — shared observation reused as-is |
| widen FEDERATED/TiDB scope | NO |

No stop condition triggered. Phase 0 recommends HOLD → the existing
accepted contract stands; the `[PEND]` items are candidate future
probe orders, not defects.
