# T20-R1 (#99) targeted-acceptance archive — product d2f8cd4e

Bound product commit: `d2f8cd4e87add7b71718fb871140afa165f7a13f`
(`fix(ddl): T20-R1 placement corrections from native v8.5.0 probes`)
Parent chain: `8ce694df` (T20 product) → `fda344dd` (evidence) →
`f1d8dd9b` (inventory) → `d2f8cd4e` (T20-R1 corrections) → this archive.

## What this round proved and fixed

Native oracle: `pingcap/tidb:v8.5.0` (`8.0.11-TiDB-v8.5.0`,
`--store=unistore`, compose project `deltascope-ddl-golden`). Raw probe
transcripts live in `probes/`; MySQL-side comment classification was
probed on `mysql:8.0.46` (`03-mysql-comment.txt`).

1. **ALTER PLACEMENT POLICY replaces, never merges** (P1). Before:
   `PRIMARY_REGION + REGIONS`; after `ALTER ... FOLLOWERS=4`:
   `SHOW CREATE` emits only `FOLLOWERS=4`. The merge implementation and
   the "merged members" test expectation were both wrong; the ordered
   state now replaces `entry.attrs` and the test pins replacement.
2. **Mixed ALTER: placement is a sole schema-change action** (P2).
   Probed on v8.5.0: missing policy refuses the whole statement
   atomically (8239) in either spec order; a structural sibling or a
   second placement spec refuses 8200 ("Unsupported multi schema change
   for alter table placement"); `ALGORITHM`/`LOCK` modifiers combine
   (rc=0). `placement_policy` joined the probed INSTANT-capable set;
   `alterPlacementMultiChange` refuses the 8200 combinations before the
   premise check. Unvouched policy in the combinable form withholds.
3. **Schema binding stamps new tables** (P4). `CREATE TABLE` under a
   proven-bound schema materializes the policy into the new row's
   catalog column; the stamp is a real bound reference that survives a
   later schema `=DEFAULT` and still refuses the drop (8241). Schema
   bind/unbind/undecidable transitions now maintain every proven
   reference set carrying the `schema:` identity — the registry refs,
   not the schema attr projection, are authoritative.
4. **Feature-comment classification** (P5). `/*T![placement]
   PLACEMENT POLICY=<name> */` is a valid binding form (rc=0, binds;
   SHOW CREATE emits it back; missing policy → 8239; works on ALTER
   too). `/*T![placement] PRIMARY_REGION='r1' */` direct options stay a
   1064 parse boundary on TiDB. On MySQL every `/*T![placement]*/`
   form is an ignored ordinary comment (lands rc=0, no binding); the
   product's MySQL dialect still parse-errors the direct-option comment
   and vendor-refuses the policy-reference comment — a conservative
   over-refusal gap, recorded in the ADR and the
   `tidb.create-table-placement-hint` inventory row.
5. **Topology layer stays unmodeled, honestly** (P3). The fixture runs
   `--store=unistore`; `SHOW PLACEMENT LABELS` itself errors
   `1105 pd http client unavailable`, and `SHOW PLACEMENT` reports
   `PENDING` — catalog binding proven, deployment never claimed. A real
   PD/TiKV cluster with labeled stores is the minimum environment for
   the scheduling layer; that remains an environment-blocked acceptance
   item, represented by evidence gaps not by findings.
6. **Privacy contract holds** (P7). Findings carry only bounded fields
   (`rule_id`, `level`, message with policy/target identity,
   `metadata: action|policy|target|target_kind`);
   `Metadata.PlacementPolicies` is `json:"-"` — catalog attrs never
   serialize. Provider errors propagate as wrapped errors, never cached
   as absence and never copied into findings.

## Golden

`green/T20-artifact.json` — 23/23 PASS, 229 assertions,
`head_sha == cli.build.head_sha == d2f8cd4e…`, `compose_down_rc=0`,
`residual_containers=[]`.

`attempts/` keeps three failed runs (`ddl-golden-t20-r1|b|c`): all six
MySQL db/syntax cases returned `rc=1 "required fixture credential
environment is unavailable"` because `T20_MYSQL_PASSWORD` was unset —
the green run passes it as `root` per `docker/ddl-golden-compose.yaml`.
The runner self-ups and self-downs the anchor services
(`compose_cleanup`), which is why containers vanish between runs.

## Gates

`gates/gate-index.json` — all rc recorded per gate.
go-test/vet/race/sql-corpus/ddl-inventory/coverage-catalog/
parser-unsupported/unsupported-diagnostics/docs-examples/
decision-record/l2-range (`57c207a8..d2f8cd4e`)/golden-validator: rc=0.
lint scoped=18, fullrepo=33 → rc=1 inherited baseline; diagnostic
identity vs the T19 baseline: added=0/removed=0 (raw stdout line
counts differ from diagnostic counts; the comparison normalizes
`path:line:col: msg (linter)` → `path|msg (linter)`).
`pg/pg-unit` and `pg/pg-confidence`: PASS on a clean worktree at
d2f8cd4e; confidence needed the same `docker_default` IPAM
accommodation (`--subnet 10.210.0.0/16`) as the previous round.
