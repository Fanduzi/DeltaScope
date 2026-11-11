# T26-SEMANTICS-01 §F — stage-matrix correspondence + open items

## F.1 `cli_*` cells (23/23 verified on `16eba8a0`, R6)

All 23 remain verified against their R6 candidate; none is re-registered
to a later commit. The slice changes expected behavior for exactly two
cells — they must be re-versioned in the implementation work order's
manifest update (new expectations recorded; the R6 pass record stays
bound to `16eba8a0`):

| case id | R6 expectation | post-slice expectation |
|---|---|---|
| `T26.cli.mysql-drop-server` | `unsupported` + `unaudited` boundary + incomplete | audited notice path (or gap when premise unproven) |
| `T26.cli.mysql-drop-server-if-exists` | same | same |

Cells unchanged by the slice (regression guards): `mysql-create-server-*`
(boundary kept), `mysql-alter-server` (boundary kept), `tidb-*` (vendor),
`srs-*`, `alter-instance-*`, `mysql-server-mixed-batch` (mixed-batch
behavior re-verified against the slice's server channel — E12).

## F.2 `db_*` cells — 8 required units, unexecuted

`T26.db.mysql57.ddl`, `T26.db.mysql57.syntax_negative`,
`T26.db.mysql80.ddl`, `T26.db.mysql80.syntax_negative`,
`T26.db.mysql84.ddl`, `T26.db.mysql84.syntax_negative`,
`T26.db.tidb85.ddl`, `T26.db.tidb85.syntax_negative`.

Relation to semantics: these are the native oracle for rows the offline
table cannot settle — INE validation ordering (row 20), OR REPLACE on
absent (row 21), option-duplication semantics (C14), name truncation vs
error (C15), and per-version catalog collation for `server_name`
comparison. They stay unexecuted under this order; no offline stub
result is registered as a DB PASS.

## F.3 Open items (affecting the first slice or semantic adjudication)

| id | item | worker recommendation |
|---|---|---|
| G-1 | Identity-unproven `DROP SERVER 'x'`: boundary vs audited-with-gap | keep `unaudited` boundary (Option A) — the catalog identity premise is structurally unavailable, matching the provider contract that the read must never fire |
| G-2 | SRS NAME/ORG conflict rows 31–32 | needs a wiring ruling + trusted-identity basis: authored NAME/OrgName are string-literal spellings; propose conflict reads qualify only on a provable-identity channel (same rule as `NameIdent`), else stays provider-only — do not fire on spellings |
| G-3 | SRS drop-ok (row 25) deps proven-absent arm | unreachable this round (dependency set is witness-positive, Complete=false stays unproven); warn arm only fires on proven deps — register, don't relax |
| G-4 | Ordered proven-absent representation for `ServerState` | add `OrderedAbsent bool` (explicit provenance) rather than reusing `not_found` — keeps the catalog `not_found` contract permanently unreachable |
| G-5 | `DROP IF EXISTS` on unproven existence — post-state | unchanged + unknown propagates (R4 addendum point 7); never written absent |
| G-6 | `server_name` catalog collation for the existence read | `WHERE server_name = ?` with verbatim decoded name; matching is the catalog column's documented case-insensitive collation — exact per-version collation stays the registered PEND from `r2-metadata-facts.md` §1; not expanded this order |
| G-7 | SRS row 13 (`srs-version-unknown`) | proposal stands: static-only checks still run, version-dependent checks (axis) skipped with bounded gap — needs reviewer ruling (suspended from R6) |
| G-8 | ALTER INSTANCE remaining deps | registered: keyring/component instance-premise capability, per-form ownership of E1–E5 — sibling task, not expanded |
