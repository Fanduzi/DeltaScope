# T26-SERVER-ALTER-CONTRACT-01 — Phase 0 R1 (semantic re-audit)

Scope: **Phase 0 only** — source audit, existing-evidence
organisation, and a proposed contract for audited MySQL
`ALTER SERVER … OPTIONS (...)`. **No product implementation.**

This is a re-examination of an area that already went through
contract (R5 ACCEPTED, `93945c01`) and execution (R6) rounds: the
audited identifier-form ALTER slice exists in product today. This
Phase 0 asks the deeper questions the earlier contract deferred:
what ALTER can/can't change natively, what success actually proves,
and whether the accepted semantics survive the deeper questions —
not how to make coverage look better.

Baseline: `ff319250263c223cbbaf0e04f9f94a2d7a1d4fd1`
(tree `a4dfd700abc65d1afca9a0f239a10f9ebd14d05e`, parent
`e4ca8f0effadd533bb07497d358cc8e9141852a1`, tracked tree clean).
HEAD is the docs-only SRS deferral closeout — zero product diff on
top of the SRS-version R4 candidate `e4ca8f0e`.

## Contents

| file | role |
|---|---|
| `prior-contract-r5.md` | the accepted R5 contract text (renamed from README.md, content verbatim — historical reference) |
| `case-matrix.md` | the R5 case matrix (committed historical artifact) |
| `source-facts.md` | current product path end-to-end (parser → boundary → bind → adjudication → apply → rule) |
| `parser-ast-mapping.md` | what the recognizer claims/rejects and which facts it derives |
| `mysql57-behavior.md` | MySQL 5.7.44 evidence status + proposed probes |
| `mysql80-behavior.md` | MySQL 8.0.46 evidence status + proposed probes |
| `mysql84-behavior.md` | MySQL 8.4.10 evidence status + proposed probes |
| `options-matrix.md` | the seven-key OPTIONS vocabulary: modifiability, empty values, duplicates, unknown keys, range |
| `privilege-analysis.md` | SUPER premise, privilege-vs-existence check order, observation mechanics |
| `catalog-poststate.md` | what ALTER provably changes in `mysql.servers` — the presence-preservation theorem |
| `batch-invalidation.md` | batch-effect rules for ALTER in the ordered channel |
| `proposed-contract.md` | A1–A12 clause adjudication + proposed contract |
| `unresolved-items.md` | open questions, proposed probe matrix, stop-condition exposure |
| `manifest-sha256.txt` | four-column digests |

## Evidence posture (work-order C)

Existing evidence is organised; **no new probe was run**. The prior
R5 contract was explicitly docs-only (manual-derived semantics); no
ALTER-specific native probe exists in any accepted archive. Every
behavioural claim below is labelled `[DOC]` (manual-derived),
`[SRC]` (source-verified this round), `[NAT]` (previously captured
native evidence), or `[PEND]` (proposed probe, not run).
