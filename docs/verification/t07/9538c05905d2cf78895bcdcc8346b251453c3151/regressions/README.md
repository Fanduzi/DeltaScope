# Regression reuse index — T07-R2 @ 9538c059

| Task | Source | Why |
|---|---|---|
| T06 | `regressions/T06/` (this directory) | Re-run at 9538c059: 496/496, 11772 assertions. |
| T02 | `../aba4478328875790e39ed6aeaae0ad67c6d5a996/regressions/T02/` | Unreachable: no `SET DEFAULT` fixture in T02; R2 changed only the TiDB `ALTER COLUMN SET DEFAULT` adjudication, `ExpressionFacts.CallCount`, and the ordered-state gate consuming it. |
| T03 | `../aba4478328875790e39ed6aeaae0ad67c6d5a996/regressions/T03/` | Same — no `SET DEFAULT` input. |
| T04 | `../aba4478328875790e39ed6aeaae0ad67c6d5a996/regressions/T04/` | Same — no `SET DEFAULT` input. |
| T05 | `../aba4478328875790e39ed6aeaae0ad67c6d5a996/regressions/T05/` | Same — no `SET DEFAULT` input. |

Reachability check: `grep -c "SET DEFAULT" testdata/ddl-golden/T0*.json`
reports 0 for T02–T06 and 48 for T07; the R2 diff touches no other
shared path those tasks consume. T06 was still re-run at this head as a
conservative regression over the shared parser/extractor surface.
