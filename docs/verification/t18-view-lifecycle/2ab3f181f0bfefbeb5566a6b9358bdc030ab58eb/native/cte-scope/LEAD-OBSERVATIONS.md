# Lead Checked CTE Scope Observations

The initial attempt had no canonical fixtures and executed no target SQL.
Its preflight is preserved separately at the path recorded in
`../cte-scope-failed-path.txt`. That is environment evidence only.

The exact lead-authored script was then mechanically executed once against
healthy task fixtures. Lead checked every target rc/error and catalog row.
No product oracle or replacement query was delegated.

- MySQL5.7 rejects all five CTE statements with1064.
- MySQL8.0/8.4 accept unused CTEs with missing tables or missing columns.
  Stored VIEW_DEFINITION removes those unused CTEs.
- TiDB8.5 rejects unused missing table1146 and unused missing column1054.
- MySQL8.0/8.4/TiDB8.5 accept a nonrecursive CTE c whose definition reads the
  existing persistent base table c; that inner reference is physical.
- All three CTE-capable anchors reject the forward reference later_c with1146.
  They accept an explicit CTE output rename c(x), with x available to the outer
  SELECT. Stored definitions preserve the explicit CTE column list.

Every setup/catalog/cleanup succeeded; all remaining-schema counts were0.
These are native semantic observations, not CLI or companion assertions.
