# Batch-effect / ordered-channel invalidation for ALTER (A10)

## Accepted behavior `[SRC]`

Within `applyServerStatement`:

- `PublishPresent` arm → republish `OrderedPresent` on the same key,
  `presentConditional = ConditionalSource`, clear absent/arm/
  publishedIndex/barrier, then `serverInvalidateOthers` retires
  entries whose relation to this identity stayed unproven.
- Every non-publish arm (A1 barrier, A2 refusal, A4 unproven,
  nil-adjudication) → **no-op**: no entry minted, no barrier, no
  `serverWrite`. ALTER can never change tracked presence, so unknown
  arms cannot shadow it — the preservation theorem means there is
  nothing for an invalidation to invalidate.
- Boundary/literal-name ALTER (outside the audited slice): the
  family's accepted write/taint rules apply unchanged — identifier
  name records `serverWrite` + relation barriers, literal name taints
  the whole channel.

## Same-key and relation semantics

Conservative identity relations (accepted family contract): the
channel keys on the authored identity; case-fold/collation
equivalence of server names is *unproven* — a differently-cased
spelling is an unproven-relation entry, retired only when this
identity's proven write lands (`serverInvalidateOthers`). A later
statement keyed to an unproven-relation name can never be proved
distinct — it stays conservatively related.

## What a failed ALTER leaves behind

On A4 (premises owed), the ordered channel republishes presence only
if a presence source answered — otherwise literally nothing changes.
The audited slice has no "partial post-state" mode: premises are
all-or-nothing per statement.
