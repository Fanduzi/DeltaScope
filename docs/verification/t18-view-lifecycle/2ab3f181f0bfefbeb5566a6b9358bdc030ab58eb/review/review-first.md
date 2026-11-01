# Lead Review Of First Slice

Reviewed full tracked diff (47 paths) and all seven new implementation/test
files against base 7f67c5de, both standards and spec. No commit yet.
Native facts remain frozen; do not rerun/rederive them.

First artifact independently checked: 16 cases,163 assertions, no duplicates,
required/executed exact, every assertion boolean true; cleanup recorded.
Frozen validation data: green/lead-first-artifact-check.json.
Three first-Golden runs all rc0; prior two preserved due subsequent code changes,
not failed oracles. Their paths are in green/golden-{1,2}-preserved-path.txt.

Standards: layering, public seam, cloning, L2 and adapter fixture changes are
focused. BASE TABLE additions match new observed column. No unrelated cleanup.
Spec: first ALTER path works, but not ready to freeze/commit. Full scope remains.

Consolidated first-scope corrections before sibling expansion:
1. EvidenceGaps globally suppresses every check's missing facts whenever ANY
   target/source is invalid. Independent target gap + source blocker must coexist.
2. Header accepts arbitrary tokWord identifiers (123,1e3,SELECT). Validate real
   identifier production, retaining typed ALTER identity/source spans.
3. Single-source column binding ignores explicit column schema and allows
   original table qualifier even after a different alias. Must refuse provably
   invalid SELECT binding rather than publish success or call it unsupported.
4. Uncertain ALTER leaves stale Updatable, explicit Algorithm and changed
   DefinerPrincipal/Kind. Withhold only changed attributes; preserve omitted
   inherited security/algorithm and existence/kind where justified.
5. Attribute-only VIEWS empty result after known view is normal missing fields;
   do not turn that into relation absence or actual provider failure. True errors
   (including errors returned by custom Provider, even ErrNoRows) stay errors.

Other known remaining-scope work (not first-path closure):
- Recursive validation currently checks only dependency existence/kind, not
  each external definition's referenced columns. Fix across full dependency graph.
- Only plain one-source projection modeled; full task must support literals,
  WHERE, stars, lists, aggregate/check/algorithm, joins/CTE scoped dependencies,
  replacement/drop/vendor differences. No final no-op rule stubs.
- Catalogue applies six new IDs to both dialects; precise final scope is
  target/source/security both; algorithm/check MySQL; ignored-option TiDB.
- Corpus helper currently defaults new IDs to three dialects: update those
  scopes, then add real paired coverage. Do not exempt/defer new rules to pass.
- Restore old plain SELECT1 controls when supported; document parser promotions
  and retain stable historical IDs/artifacts.
- Findings need safe object/role metadata and original site locations.

Primary grammar checked: MySQL8.0.46 user_ident_or_text permits omitted @host;
installed parser Username/StringName also permits StringName alone (host "%").
parser.Keywords public table has Word/Reserved/Section; do not guess API names.
