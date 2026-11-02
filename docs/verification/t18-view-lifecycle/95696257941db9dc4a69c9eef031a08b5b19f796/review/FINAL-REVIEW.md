# T18-R1-tail2 review handoff

Product: `95696257941db9dc4a69c9eef031a08b5b19f796`

Red baseline: `abe88235e00c53c174d719fe270209b73b82f659`

## Fixed issues

- View `Updatable` now uses one shared tri-state proof for CHECK OPTION and later view-state publication: a proven non-updatable source no longer vetoes an independently proven component, and an updatable source does not prove every requested output.
- Qualified output projection now requires the source view's known outputs to supply the requested field. A known missing output disproves updatable publication; unknown supply stays unpublished as `view.updatable`.
- MySQL 8.0.46 `lex->query_tables` reuse semantics are narrowed to the unique top-level physical source plus later global physical-chain reuse. Derived/CTE/view first nodes do not inherit base-table reuse semantics, while base-table reuse through CTE/derived/view leaves is disproof.
- Published catalog `IS_UPDATABLE` remains authoritative when known; parsed source shape fills gaps without fabricating true/false.

## Golden

- T16: 60/60 cases, 832 assertions.
- T17: 52/52 cases, 834 assertions.
- T18: 150/150 cases, 3,778 assertions.

## Boundaries

- No T19 work, PR, tag, release, deployment, merge-to-main, or reviewer-acceptance claim.
- No credential values are archived; only credential environment variable names and sanitized command shapes.
- Failed credential/setup Golden attempts are preserved under `attempts/` and are not semantic evidence.
- Companion/internal projections are separate from Golden public counts.
