# T19-R1 closure addendum — `efc77a9c`

Final-SHA verification for T19/#98, bound to
`efc77a9c8479994ebf95238365ecbbf0c0f7f78f`.

## Commit identity

```
efc77a9c docs(readme): sync T19 module READMEs for L2 gate      ← product (final)
3ba1f860 style(catalog): gofmt map alignment                    ← product
e39fc02c test(auditmeta): TiDB probe stubs in t08r1 fixture     ← product (fixture follow-up)
3966020e test(evidence): original T19 archive (binds f00b3a7d)  ← original evidence
f00b3a7d feat(ddl): T19 TiDB TTL lifecycle                      ← original product
4ef016df docs(verification): archive T18-R1-tail2               ← base
```

## Relationship to prior records

- `../f00b3a7d.../` — the original T19 evidence archive, bound to the
  first product commit. Unchanged.
- `../e39fc02c.../` — first R1 pass + the fixture-followup review
  (`review/fixture-followup-review.md`). Its lint-comparison exposed one
  added gofmt diagnostic and its L2 gate exposed three stale module
  READMEs; both were fixed by `3ba1f860`/`efc77a9c` and re-verified here.

## What this addendum proves on `efc77a9c`

- `gates/01–12`: unit, vet, race, sql-corpus, inventory, coverage
  catalog, parser/unsupported contracts, docs examples, decision record,
  committed-range L2, golden validator — all rc=0.
- `gates/13–15`: lint scoped/fullrepo keep rc=1 (accepted inherited
  diagnostics); diagnostic-identity comparison vs the accepted T18
  `abe88235` baseline reports **added=0, removed=0** on both — rc=1 is a
  baseline carry-over, not a green claim.
- `gate/16` + `green/golden-artifact.json`: real-anchor T19 golden —
  23/23 cases, 281 assertions, PASS on head `efc77a9c`;
  `artifact.head_sha == cli.build.head_sha`; CLI sha256
  `473530c3…cacadb`; `required_case_ids == executed_case_ids` (23, no
  duplicates); `compose_down_rc=0`, `residual_containers=[]`.
- `attempts/01-golden-compose-pool/`: preserved failed first attempt
  (host IPAM exhaustion; 0 cases; not a code failure). The accommodation
  — a pre-created project network on explicit `10.203.0.0/16` — was
  removed after the run; no foreign networks/containers/volumes touched.

## Boundary

This archive records verification results. Independent reviewer
acceptance is a separate step and is not claimed here.
