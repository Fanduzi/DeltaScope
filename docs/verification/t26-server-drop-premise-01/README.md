# T26-SERVER-DROP-PREMISE-01 — index (PASS, docs-only contract acceptance)

Docs-only premise round (issue #105): the execution-principal binding
and the `server.drop_privilege` supply contract for the already
accepted MySQL identifier-form `DROP SERVER` slice — **PASS** as a
documentation contract. No product, test, policy, or manifest changes;
no test runs, CLI probes, or native verification — every forward item
is marked PLAN / UNEXECUTED.

## Identity

| item | identity |
| --- | --- |
| baseline commit | `244aace52f319abd1fe0ebaed01144c2afd1bd5a` |
| baseline tree | `93224bd3f69862c71f494a37f4f1ac5864b2e008` |
| baseline parent | `2ee906d55cdc3916e5169f090eaf701d17a3bb9d` |
| accepted product/test | `2ee906d55cdc3916e5169f090eaf701d17a3bb9d` (tree `7c6ea98c…`, parent `5de2da5e…`) — T26-SERVER-DROP-01 PASS |
| semantics contract | `docs/verification/t26-semantics-01` at `b8a67e7b5f7e62a825adace06113a233240d28df` |
| R0 candidate | `e42eeafe33edb5b7bc32c2f4412f670f2864d87e` — CHANGES REQUIRED (R1-A..F) |
| R1 candidate | `be729d61e1b6e52ffb0e68e06c90436d60e1a199` — CHANGES REQUIRED (R2-A/B/C + S1/S2) |
| approved contract | `cfd34782b94d12342a021b846d51f41a5682d2e3` (tree `d8e41edc…`, parent `be729d61…`) — **PASS** |
| this closeout | docs-only: README status + `acceptance.md`; parent `cfd34782` |

Source excerpts cite the baseline commit `244aace5`; the audited source
content is unchanged since `5de2da5e` for every excerpted file (the
`5de2da5e..2ee906d5` increment touched one test file only, and
`2ee906d5..244aace5` touched documentation only).

## Files

| file | content |
| --- | --- |
| README.md | this index — identity, scope, status |
| acceptance.md | the registered PASS: scope, identity chain, closure basis, role/evidence boundary, and non-activated capabilities |
| source-excerpts.md | verbatim source closures — excerpts annotated verbatim / derived / doc-derived / proposed; includes the completed token loop, session acquire/release, the ordered-scope classification seam, the corrected SET ROLE/GRANT/REVOKE registration, and the unified application-side error boundary |
| execution-premise-contract.md | the `observed_account_new_session` conditional mode, the single recommended evidence source, the eight-condition consumability contract (target binding = this request's selected provider), and the A–G adjudication table (positive witness / unproven / bounded error only — no denial) |
| implementation-plan.md | the single recommended path — request entry, loader, gate, unified application error boundary, deterministic data flow, ordered-scope markers, the consumption-activation gate, the corrected consumer matrix (terminal-refusal priority, IF EXISTS convergent arm, read-error separation), and the (unexecuted) verification plan |

## Status

**T26-SERVER-DROP-PREMISE-01 — PASS (docs-only contract acceptance)**,
object `cfd34782`; acceptance details in `acceptance.md`.

Document approval is not a product capability: the executing principal
is **not** bound at any request entry today, and `server.drop_privilege`
stays owed on every audited DROP SERVER path until an approved
implementation wires a positive witness. Activation of the privilege
supply, gap consumption, shared adjudication, and `OrderedAbsent`
requires a separate, explicit product work order; this closeout HEAD
does not replace the approved contract identity `cfd34782`, and the
R0/R1 `CHANGES REQUIRED` history and their candidate identities stay
preserved.
