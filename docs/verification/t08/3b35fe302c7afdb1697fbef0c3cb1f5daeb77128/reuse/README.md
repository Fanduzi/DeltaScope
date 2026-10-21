# Reused evidence — revalidation under the updated validator

These artifacts were produced at frozen product SHA
`c983fc9d2e8f887981a5f031b1d55b699d711e84` and are reused unchanged (their
`head_sha` stays `c983fc9d`; binaries are the original bytes, sha256
`42c468d5…0798a4d` for the closeout regression binaries and the recorded
T08 binary). Each was re-validated with the **new** validator at proof SHA
`3b35fe302c7afdb1697fbef0c3cb1f5daeb77128` — binary check ON (the recorded
`/private/tmp/t08-closeout-evidence/…` binaries still exist and hash-match).

| Artifact | head_sha | New-validator result | Log |
|---|---|---|---|
| T06 closeout regression (496/496) | c983fc9d | artifact valid, rc=0 | `T06-revalidate.log` |
| T07 closeout regression (207/207) | c983fc9d | artifact valid, rc=0 | `T07-revalidate.log` |
| T08 golden (172/172) | c983fc9d | artifact valid, rc=0 | `T08-revalidate.log` |

The T06/T07/T08 manifests are byte-identical between `c983fc9d` and
`3b35fe30` (task-range diff: only T03/T04/T05 manifests moved), so the new
validator is re-running the same contract against the same evidence.

T02 continues to reuse the accepted original at `aba44783` (10/10), as
recorded in the delivery-closeout supplement — not re-executed this round.
