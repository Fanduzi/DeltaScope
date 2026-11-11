# T26-SERVER-DROP-PREMISE-01 — index

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
| this round | docs-only increment under `docs/verification/t26-server-drop-premise-01/` |

Source excerpts cite the baseline commit `244aace5`; the audited source
content is unchanged since `5de2da5e` for every excerpted file (the
`5de2da5e..2ee906d5` increment touched one test file only, and
`2ee906d5..244aace5` touched documentation only).

## Files

| file | content |
| --- | --- |
| README.md | this index — identity, scope, status |
| source-excerpts.md | verbatim source closures answering the six required questions, each excerpt annotated verbatim / derived / proposed |
| execution-premise-contract.md | the principal model, the single recommended evidence source, the consumability contract, and the A–G adjudication table |
| implementation-plan.md | the single recommended implementation path — seams, files, functions, and the minimal (unexecuted) verification plan plus OrderedAbsent entry conditions |

## Status

Draft for reviewer adjudication. Nothing in this directory asserts a
product capability that does not exist: the executing principal is
**not** bound at any request entry today, and `server.drop_privilege`
stays owed on every audited DROP SERVER path until an approved
implementation wires a positive witness.
