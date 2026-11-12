# Catalog post-state — what ALTER provably changes (A8/A9)

## The documented write set `[DOC]`

"Adjusting any of the options permitted in CREATE SERVER … the
corresponding fields in `mysql.servers` are updated accordingly."
Merge update on the named row: HOST/DB/USERNAME/PASSWORD/PORT/
SOCKET/OWNER columns take the authored values; unmentioned columns
retain stored values. The example updates `USER` alone.

## Presence-preservation theorem (accepted R5 §D, still binding)

The write set is *option columns of the named row*. Presence
(`server_name` row existence) is not in that set: a successful ALTER
can neither create nor delete the row. Therefore:

- proven presence survives every ALTER arm — `PublishPresent`
  republishes `OrderedPresent` even on the unproven arm when a
  presence source answered `[SRC]`;
- `OrderedAbsent` proves the ALTER cannot have landed — the A2
  terminal refusal owes nothing and never reaches privilege/options;
- no arm mints an absent/post-attribute fact — **what success proves
  is bounded to presence-preservation**, never "the options changed"
  (the product selects only `server_name` — it never reads the
  option columns, so it cannot attest to which values landed).

## Post-state provability limit `[PEND]`

"ALTER success ⇒ all resulting attributes proven" is explicitly
frozen false by the work order. The honest model: success (under
proven presence + consumable premise + clean options) proves the
named row persists — option values are deliberately unobserved
(data-minimalisation: `select server_name` only). Verifying an
option actually landed would need a post-state read of the option
columns — registered as a possible future contract, not proposed:
the current slice never claims attribute-level truth. Native
merge-vs-replace confirmation: probe 80-9.
