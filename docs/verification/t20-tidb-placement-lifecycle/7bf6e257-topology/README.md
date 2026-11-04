# T20/#99 topology round — real PD/TiKV verification bound to product `7bf6e257332b5fd0a632cd881af8487f5dbc9a0b`

Round scope: execute the deferred real-topology prerequisite verification under
this round's one-shot authorization (1 PD + 3 TiKV + 1 TiDB, all `v8.5.0`).
No product source changed; the frozen `7bf6e257` binary performed all
product-side checks. Prior A/B/P1/P2/P4 fixes are regression-checked only.

## Fixture identity (`probes/00-fixture-compose.yaml.txt`, `10-cluster-identity.txt`)

- Compose project `deltascope-t20-topo`, file `docker/ddl-t20-topo-compose.yaml`,
  dedicated network `deltascope-t20-topo` on explicit subnet `10.211.0.0/24`
  (verified free: 10.203/16 golden, 10.210/16 docker_default, 10.254.x vm-worker
  all occupied by foreign resources — none touched).
- Images (all arm64/linux, digests in receipt): `pingcap/pd:v8.5.0`,
  `pingcap/tikv:v8.5.0`, `pingcap/tidb:v8.5.0`, client `mysql:8.4.10`.
- Loopback-only publishes: 24100→TiDB 4000, 24180→TiDB 10080. PD/TiKV ports
  internal only.
- **Real-cluster proof**: TiDB argv `--store=tikv --path=pd0:2379`; PD
  cluster_id `7704189008621554081`, member `pd0` health=true, binary v8.5.0;
  three TiKV stores (PD-side IDs 1, 4, 5) all `Up` at `tikv{1,2,3}:20160`;
  `SHOW PLACEMENT LABELS` returns `region=[t20rev]`, `zone=[z1,z2,z3]`,
  `host=[h1,h2,h3]` — the labels are synthetic single-host logical labels,
  NOT evidence of multi-datacenter or hardware fault tolerance.
  `tidb_placement_mode=STRICT`.

## Matrix results (`probes/20-topology-matrix.txt`, `21-topology-matrix-c2.txt`)

| case | input | native result | layer proved |
|---|---|---|---|
| A | `p_sat PRIMARY_REGION="t20rev" REGIONS="t20rev"`, `ALTER TABLE t20topo.t PLACEMENT POLICY=p_sat` | bind rc=0, `SCHEDULING_STATE=SCHEDULED` (real scheduling, not unistore PENDING); `PLACEMENT POLICY=DEFAULT` unbind lands | static definition + catalog binding + real schedule completion |
| B | `ALTER TABLE t20topo.t PLACEMENT POLICY=p_no_such` (proven absent from catalog) | `ERROR 8239 Unknown placement policy`, prior `p_keep` binding preserved | proven-missing refusal + pre-state retention |
| C1 | `p_unsat PRIMARY_REGION="t20-noreg" REGIONS="t20-noreg"` (no store carries the label) | `ERROR 8243 [PD:placement:ErrRuleContent] ... can not match any store` — **refused at bind** | label-unsatisfiable = DDL-time refusal |
| C2 | `p_5f FOLLOWERS=5` (3 stores exist) | bind rc=0, catalog binding lands, `SCHEDULING_STATE=PENDING` persists ≥15s | replica-count unsatisfiable = accepted binding + genuine pending, explained by independent store facts (3 < 5) — PENDING never auto-means either outcome |
| D | product CLI `7bf6e257` against `127.0.0.1:24100` (`probes/22`, `23`) | existing policy → `ddl.tidb.placement.warn` fires, coverage complete; missing policy → no finding (matches frozen `bind-missing-refused` oracle); provider errors still fail-loud | product reads catalog only; topology satisfaction is delegated via the finding suggestion, never claimed |
| D-offline | same SQL, no connection (`probes/24`) | `ddl.tidb.placement.warn` gap `required_facts=['placement_policy.catalog']`, coverage unverified | honest gap when catalog facts not provided |

Cleanup: fixture objects dropped, `compose down -v` removed all containers,
the dedicated network, and (none existed) volumes — zero residual
(`probes/90-teardown.txt`). Foreign `vm-worker-*`, `controlhub-*`,
`binlog-*`, `deltascope-ddl-golden_*` resources untouched.

## A/B regression recheck (read-only)

`go test ./internal/application/audit -run 'TestT20R2|TestT20MixedAlter|
TestT20AlterPolicy|TestT20Schema|TestT20Catalog'` — green on `b24f7331`
worktree (product bytes still `7bf6e257`). The committed blanking lexer and
`schemaPostAttrs` tri-state were re-read and match the reviewed semantics.

## Verdict

The last missing prerequisite is now proven on a real cluster: catalog binding,
bind-time topology refusal (8243), genuine PENDING for unsatisfied replicas,
and real SCHEDULED completion are all distinguished with independent store
facts. Combined with the closed A/B layer this is the T20 final independent
review candidate. Original 25-case/255-assertion golden at `7bf6e257` stands;
this round adds companion native+product receipts, counted separately.
