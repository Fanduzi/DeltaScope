# Design: Consolidate Query Access Proof Orchestration

## Status

Proposed. This design changes internal sequencing only; public and supported
behavior remains fixed.

## Current Shape

`Service.Analyze` currently has three promotion sequences:

```text
ordinary PostgreSQL:
  metadata -> PG proof -> normalize/reclassify/admission -> requirements

exact PostgreSQL COUNT(1):
  metadata -> initial normalize/reclassify/admission -> requirements
           -> completeness gate -> PG proof -> reclassify/admission

MySQL/TiDB builtin:
  metadata -> initial normalize/reclassify/admission -> requirements
           -> builtin proof -> reclassify/admission
```

The split exists because exact `COUNT(1)` and builtin semantics require
Physical Requirement Completeness, while the older ordinary PostgreSQL path
predates that invariant. The predicates confirm that they do not depend on an
intermediate classification, admission, normalized reasons, or warnings.

## Chosen Shape

```text
extract
  -> resolve metadata
  -> apply Promotion Barriers
  -> build requirements
  -> prove for promotion
       -> ordinary PostgreSQL manifest proof, or
       -> exact COUNT(1) completeness + manifest proof, or
       -> MySQL/TiDB builtin semantic proof, or
       -> no-effect applicability rule
  -> normalize reasons once
  -> reclassify once
  -> recompute admission once
  -> sort and validate
```

Add one private file, `proof_orchestration.go`, containing a small unexported
result and the orchestration function. The intended shape is equivalent to:

```go
type promotionProof struct {
	allowsPromotion bool
}
```

Proof-specific reason removal is an internal mutation of the extracted
result, not part of the return value.

The exact names and signature may follow existing package style. The important
contract is that this type carries only what the common pipeline needs. It is
not public, not serialized, and not an extensibility point.

`reclassifyAfterResolution` accepts the single `allowsPromotion` fact rather
than PostgreSQL and variadic builtin proof values. It continues to enforce the
resolver, reason, and unresolved checks. `recomputeAdmission` continues to
preserve an existing rejection before deriving the final admission.

## Applicability Rules

| Path | Proof permits common promotion checks when |
|---|---|
| PostgreSQL ordinary effects | trusted atomic manifest decision is `all_proven` |
| PostgreSQL exact `COUNT(1)` | exact requirement predicate passes and trusted atomic manifest decision is `all_proven` |
| PostgreSQL without applicable proof | never; an indeterminate result remains indeterminate |
| MySQL/TiDB with effect candidates | builtin semantic decision is `all_proven` |
| MySQL/TiDB without effect candidates | proof is not required; ordinary common checks may proceed |

The table describes proof applicability, not final admission. Any remaining
reason, unresolved reference, wildcard, missing resolver, barrier, or write
classification still prevents promotion.

## Reason Ownership

On successful PostgreSQL proof, remove only:

- `unproven_operator_effect`;
- `unproven_function_effect`; and
- `unproven_cast_effect`.

On successful MySQL/TiDB builtin proof, remove only the existing function-call
semantic reasons. Failed or inapplicable proof removes nothing. Final reason
normalization occurs after this operation.

## Behavioral Compatibility

The requirement builder is pure and does not inspect proof state. The existing
proof predicates inspect candidates, metadata, requirements, and session facts,
not intermediate classification or admission. Moving ordinary PostgreSQL proof
after requirement construction therefore adds no catalog interaction and
changes no proof input relevant to its decision.

The refactor must nevertheless preserve observable and operational behavior:

- exact domain result fields and stable ordering;
- exact bounded reason and warning sets;
- catalog probe sequence and same-session atomicity;
- cancellation and closed-session behavior;
- caller ownership of the connection; and
- no execution or disclosure of user SQL, credentials, endpoints, OIDs,
  backend identifiers, or driver errors.

## Testing

Add one table-driven application test for orchestration-owned decisions:

- requirements are present when each proof predicate runs;
- ordinary PostgreSQL and exact `COUNT(1)` success/failure;
- MySQL/TiDB candidate and no-candidate applicability;
- barriers cannot be promoted;
- successful proof removes only its owned reasons; and
- failed proof removes none.

Use existing trusted-service, builtin gateway, recording-driver, unified SDK,
transport, corpus, and Docker-backed tests for end-to-end equivalence. Do not
introduce a source parser, custom checker, test framework, or duplicate matrix.

## Alternatives Rejected

### Introduce a `proofEngine` interface

Rejected because there is one orchestration caller and no independent runtime
implementations requiring polymorphism. A private function is smaller and
keeps the closed product switch visible.

### Preserve three sequencing clocks behind helper functions

Rejected because it shortens `Analyze` without fixing the divergence risk.

### Merge PostgreSQL and builtin proof models

Rejected because their evidence and failure semantics differ. Only the common
promotion input is shared.

### Combine identity-protocol cleanup

Rejected because that changes the trust boundary at the same time as proof
sequencing. It requires a separate audit and decision.

### Skip proof for already rejected statements

Rejected for this milestone because it may change catalog traffic, cancellation
timing, or reason behavior. Optimization requires separate evidence.
