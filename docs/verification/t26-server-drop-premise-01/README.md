# T26-SERVER-DROP-PREMISE-01 — index (R1)

Docs-only premise round (issue #105): the execution-principal binding
and the `server.drop_privilege` supply contract for the already
accepted MySQL identifier-form `DROP SERVER` slice. No product, test,
policy, or manifest changes this round; no test runs, CLI probes, or
native verification — every forward item is marked PLAN / UNEXECUTED.

## Identity

| item | identity |
| --- | --- |
| baseline commit | `244aace52f319abd1fe0ebaed01144c2afd1bd5a` |
| baseline tree | `93224bd3f69862c71f494a37f4f1ac5864b2e008` |
| baseline parent | `2ee906d55cdc3916e5169f090eaf701d17a3bb9d` |
| accepted product/test | `2ee906d55cdc3916e5169f090eaf701d17a3bb9d` (tree `7c6ea98c…`, parent `5de2da5e…`) — T26-SERVER-DROP-01 PASS |
| semantics contract | `docs/verification/t26-semantics-01` at `b8a67e7b5f7e62a825adace06113a233240d28df` |
| R0 candidate | `e42eeafe33edb5b7bc32c2f4412f670f2864d87e` — CHANGES REQUIRED (R1-A..F) |
| this round | R1 docs-only revision of the same four files, new commit with parent `e42eeafe` |

Source excerpts cite the baseline commit `244aace5`; the audited source
content is unchanged since `5de2da5e` for every excerpted file (the
`5de2da5e..2ee906d5` increment touched one test file only, and
`2ee906d5..244aace5` touched documentation only).

## Files

| file | content |
| --- | --- |
| README.md | this index — identity, scope, status |
| source-excerpts.md | verbatim source closures answering the six required questions — excerpts annotated verbatim / derived / doc-derived / proposed; includes the completed token loop, the session acquire/release points, and the ordered-scope classification seam |
| execution-premise-contract.md | the `observed_account_new_session` conditional mode, the single recommended evidence source, the eight-condition consumability contract, and the A–G adjudication table (positive witness / unproven / bounded error only — no denial) |
| implementation-plan.md | the single recommended path — request entry, loader, gate, error boundary, deterministic data flow, ordered-scope markers, the consumption-activation gate, and the (unexecuted) verification plan |

## Status

R1 revision for reviewer adjudication. Nothing in this directory
asserts a product capability that does not exist: the executing
principal is **not** bound at any request entry today, and
`server.drop_privilege` stays owed on every audited DROP SERVER path
until an approved implementation wires a positive witness. Document
approval alone authorises neither consumption activation nor
OrderedAbsent — those require a separate product work order.
