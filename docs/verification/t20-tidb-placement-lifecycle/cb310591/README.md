# T20-R3 second-review corrections

Bound to product commit `cb3105915e6dfa6b008e7eefd27207c726516bc1`
(parent `c2119240aaef6e141d4b01e5e3c4f1cda9ab0654`, chain
`… → 9f4f148b → f457df42 → 2405d09a → c2119240 → cb310591`).

## Scope

Third product commit. The independent two-axis review of
`9f4f148b..2405d09a` returned no hard standards violations and confirmed
all four prior fix claims, but found four remaining premise-precision
defects on the same surfaces:

1. **`IF EXISTS` disarms only the existence premise.** Native 8241
   refuses a proven-bound policy under `DROP PLACEMENT POLICY IF EXISTS`
   too — `PlacementDropOutcome` already modeled that, and `dropFinding`
   already fired the blocker, but `exists.require`'s `EvidenceGaps`
   returned nil for every `IF EXISTS` drop. Now reports with identical
   attribution: unvouched catalog → `[catalog, bound_targets]`, proven
   catalog + unread bound set → `[bound_targets]`, proven-absent no-op
   → silent.
2. **Table-level `= DEFAULT` is an unbind, not a binding.** The warn
   rule's scan stopped at a statement-level DEFAULT and never reached
   named partition binds (`CREATE TABLE ... PLACEMENT POLICY=DEFAULT
   PARTITION (p0 PLACEMENT POLICY=p)`), while `exists.require`'s shared
   scan saw them — divergent rules on one statement. Both rules now use
   `firstNamedPlacementBinding`, which skips DEFAULT and keeps looking;
   the duplication the standards axis flagged is gone.
3. **`CREATE OR REPLACE` over an unvouched catalog.** The create lands
   and warns, but whether bound objects were rewritten is undecidable —
   warn now owes `placement_policy.catalog` rather than staying silent
   on the premise. `exists.require` stays disarmed (OR REPLACE cannot
   refuse).
4. **Fresh create over a tombstone** seeds `refs={}` + `refsKnown`
   (proven-empty bound set — nothing could bind to a policy that did
   not exist), removing an over-gap on a subsequent DROP. OR REPLACE
   over a present entry keeps refs (bindings persist — probed).

Plus precision nits: the `IF EXISTS` no-op drop no longer claims it
"removes a topology policy" (warn fires only on `exists` proven true),
`bindFinding` parameter order normalized to `bindOutcome`, inventory
evidence punctuation corrected.

Test cells K–N in `batch_state_t20r3_test.go` cover every new arm at the
public `AuditSQL` path.

## Golden

- `green/T20/T20/artifact.json` — 25/25 cases, 286 assertions, rc=0,
  `head_sha=cb3105915e6d`. No oracle change was needed — the fixes only
  affect paths no golden case exercises (IF EXISTS drops, OR REPLACE,
  table-level DEFAULT beside partition binds).

## Gates (all rc=0, receipts under `gates/`)

vet, race (audit+rule+spec+policy), full `go test ./...`, sql-corpus,
inventory contract, coverage catalog (postgresql tag), parser
unsupported + diagnostics, docs-examples, decision-record, golden
validator — all green. Lint identity comparison vs the `2405d09a`
baseline: scoped 18→18 (+0/-0), full 33→33 (+0/-0), same commands and
normalization.
