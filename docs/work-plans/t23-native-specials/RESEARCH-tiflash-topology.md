# T23 — real TiDB+TiKV+TiFlash fixture research (v8.5.0, linux/arm64)

Research date: 2026-11-08 UTC. Scope: a single-node, real (non-emulated)
TiDB + TiKV + PD + TiFlash docker-compose fixture, all images pinned to
`v8.5.0` on `linux/arm64`. Primary sources only: `pingcap/docs` release-8.5
branch, upstream source at the `v8.5.0` tag (`pingcap/tidb`, `tikv/tikv`,
`pingcap/pd`, `pingcap/tiflash`), `pingcap/tidb-docker-compose`, and the
Docker Hub public registry API (`hub.docker.com/v2` + `registry-1.docker.io`
anonymous pull). No blog posts or third-party tutorials were used. Claims
that could not be verified from these sources are marked **UNVERIFIED**.

`pingcap/tidb-docker-compose` is stale (its compose files target
`latest`/`nightly` and pre-date the v8.5 config layout), so it is cited only
for entrypoint/flag patterns that are independently confirmed against
v8.5.0 source. Its old `[application] runAsDaemon = true` and two-file
`tiflash.toml`/`tiflash-learner.toml` wiring must not be copied verbatim —
see §2.3.

## 1. Docker Hub tags and digests

All four `v8.5.0` tags exist, are `active`, and are multi-arch manifest
lists (`application/vnd.docker.distribution.manifest.list.v2+json`) with
both `linux/amd64` and **`linux/arm64`** entries. No component is missing
arm64. Retrieved via
`https://hub.docker.com/v2/repositories/pingcap/<img>/tags/v8.5.0` and
confirmed with anonymous-pull manifests from `registry-1.docker.io` on
2026-11-08; all four tags were pushed 2024-12-19.

| image | manifest-list digest (`docker pull pingcap/<img>:v8.5.0`) | linux/arm64 image-manifest digest | arm64 compressed size |
|---|---|---|---|
| `pingcap/pd:v8.5.0` | `sha256:5cf66a73894ca652dd640e2ae72bc0bff6211ec55efbe95fbf45f98807e26af0` | `sha256:f3da901fee79981b5eec50eca75d22eb3be76d62a499a9eb78bc880a601c4fc2` | 219,882,876 B (~210 MiB) |
| `pingcap/tikv:v8.5.0` | `sha256:0524a2070bbfe3fef1331589113344f10050e6b821c2c0a4b79108b4b535f824` | `sha256:188fc8eaf9d7f11abd405b4b5b7bef2f5d7cc6319cb6e49e05d0f6aac42eb734` | 806,011,503 B (~769 MiB) |
| `pingcap/tiflash:v8.5.0` | `sha256:bb0b83a84e6276b20dba2466beb02d74d546c04e1ad9b5b5e17c7830ddfa412a` | `sha256:1c85abb9c8ace54878d0cfdb2922ffda3b75ff3b8115d1d941ce9ef7acd444ad` | 696,844,391 B (~665 MiB) |
| `pingcap/tidb:v8.5.0` | `sha256:490861bd4a2fa643dbe76b77ffecafcad66e6a42774e580b82f4ec2429a80e57` | `sha256:9777fb617f3f81d5a5fec6fcf311616da2b1d9f0d1103d86952374f53421856e` | 212,210,022 B (~202 MiB) |

amd64 image-manifest digests for completeness: pd
`sha256:51570618f3c4307f73efac23634f40acf42f314b23bd5afae903b3c413e51346`,
tikv `sha256:eed41be14384a2123b5ffabffae7e8322a027ea94d6482646e1d3a1d0fd8bbac`,
tiflash `sha256:93019aaff214a04d80dda6caf8873e6b564a9c9991211a1c7b7919e3bd1cb9a3`,
tidb `sha256:5dea0d93bf399ed583a762113fc1ead528d92f2b6dff68c0ca45a3d523d41e2b`.
The `size`/`full_size` fields in the Hub API response are summed **compressed**
layer sizes, not on-disk uncompressed size.

Image config blobs (fetched from `registry-1.docker.io`, arm64 manifest →
config blob) give the entrypoints:

- `pingcap/pd:v8.5.0` — `Entrypoint: ["/pd-server"]`, `ExposedPorts: 2379, 2380`.
- `pingcap/tikv:v8.5.0` — `Entrypoint: ["/tikv-server"]`, `ExposedPorts: 20160`;
  env `MALLOC_CONF=prof:true,prof_active:false`.
- `pingcap/tiflash:v8.5.0` — `Entrypoint: ["/tiflash/tiflash", "server"]` (two
  argv elements; `server` is the ClickHouse-style subcommand), no
  `ExposedPorts`, env `LD_LIBRARY_PATH=/tiflash`.
- `pingcap/tidb:v8.5.0` — `Entrypoint: ["/tidb-server"]`, `ExposedPorts: 4000`.

Therefore a compose `command:` list is appended to each entrypoint as
process arguments (e.g. tiflash gets `/tiflash/tiflash server
--config-file=/tiflash.toml`).

## 2. Minimal single-node topology

Canonical flag sets are taken from each component's v8.5.0 source, cross-
checked against `pingcap/tidb-docker-compose` `docker-compose.yml` /
`docker-compose-tiflash-nightly.yml`
(<https://github.com/pingcap/tidb-docker-compose/tree/master>).

### 2.1 PD (`/pd-server`)

Flags registered in `pingcap/pd@v8.5.0`, `cmd/pd-server/main.go`,
`addFlags()` (lines ~163–184): `--name`, `--data-dir`,
`--client-urls`, `--peer-urls`, `--advertise-client-urls`,
`--advertise-peer-urls`, `--initial-cluster`, `--join`, `--config`,
`-L/--log-level`, `--log-file`, `--metrics-addr`, `--force-new-cluster`,
`--cacert/--cert/--key`, `--config-check`, `-V/--version`.

Minimal 1-node cluster command (pattern verified against
`docker-compose-tiflash-nightly.yml`):

```yaml
command:
  - --name=pd0
  - --data-dir=/data
  - --client-urls=http://0.0.0.0:2379
  - --peer-urls=http://0.0.0.0:2380
  - --advertise-client-urls=http://pd0:2379
  - --advertise-peer-urls=http://pd0:2380
  - --initial-cluster=pd0=http://pd0:2380
  - --config=/pd.toml          # optional; needed for replication.max-replicas
```

`initial-cluster-state` defaults to `"new"`
(`server/config/config.go:192`, `defaultInitialClusterState =
embed.ClusterStateFlagNew`), so no flag is needed. `--config` accepts a
TOML file; `[replication] max-replicas = 1` belongs there — see §3.

### 2.2 TiKV (`/tikv-server`)

Flags registered in `tikv/tikv@v8.5.0`, `cmd/tikv-server/src/main.rs`
(lines ~29–160): `--config`, `--addr`, `--advertise-addr`,
`--status-addr`, `--advertise-status-addr`, `--data-dir`, `--capacity`,
`--pd-endpoints` (aliases `--pd`, `--pd-endpoint`), `--labels`,
`--log-level`, `--log-file`, `--config-check`, `--config-info`,
`--metrics-addr`, `--print-sample-config`.

Minimal single-store command:

```yaml
command:
  - --addr=0.0.0.0:20160
  - --advertise-addr=tikv:20160
  - --status-addr=0.0.0.0:20180
  - --advertise-status-addr=tikv:20180
  - --data-dir=/data
  - --pd=pd0:2379            # alias of --pd-endpoints; comma-separated list
```

Defaults from `src/server/config.rs` v8.5.0: `DEFAULT_LISTENING_ADDR =
"127.0.0.1:20160"` (line 22), `DEFAULT_STATUS_ADDR = "127.0.0.1:20180"`
(line 24). The PD client defaults to `endpoints:
["127.0.0.1:2379"]` (`components/pd_client/src/config.rs:46`), so `--pd`
is not schema-required but must be set to the real PD DNS name inside
compose. `--advertise-addr`/`--advertise-status-addr` must be the
container's DNS-reachable name so PD and TiFlash can dial it.

### 2.3 TiFlash (`/tiflash/tiflash server --config-file <toml>`)

The image entrypoint is `/tiflash/tiflash server`. The `server` subcommand
loads a TOML config; `server --config-file` is the canonical flag
(`docs/release-8.5/tiflash/tiflash-command-line-flags.md`: *"You must
specify the configuration file"*). `BaseDaemon::defineOptions`
(`pingcap/tiflash@v8.5.0`, `libs/libdaemon/src/BaseDaemon.cpp:1198-1202`)
registers `config-file` (`-C`); the header comment
(`libs/libdaemon/include/daemon/BaseDaemon.h:47`) also documents
`--config` as an alias, which is what the old
`docker-compose-tiflash-nightly.yml` (`command: --config=/tiflash.toml`)
uses. Default config path is `config.toml` in the working directory
(`BaseDaemon.cpp:702`).

`application.runAsDaemon` defaults to `false` (`BaseDaemon.cpp:755`), so
the process stays in the foreground — correct for a container. The
nightly-era config's `[application] runAsDaemon = true` daemonizes the
process (parent exits); do **not** copy that key into the fixture.

Layout in v8.5.0 — the raft proxy ("learner") is **embedded** in the
tiflash binary, not a separate process. `dbms/src/Server/Server.cpp:285-336`
(`TiFlashProxyConfig`) shows:

- the proxy only starts when `raft.pd_addr` is set (`Server.cpp:297-305`;
  otherwise it logs `TiFlash Proxy will not start because `raft.pd_addr`
  is not configured.`);
- `raft.pd_addr` is forwarded to the proxy as `--pd-endpoints`
  (`Server.cpp:314`);
- every `flash.proxy.*` key becomes a `--key value` proxy argument
  (`Server.cpp:299-312`);
- `engine-addr` defaults to `flash.service_addr` = `0.0.0.0:3930`
  (`Server.cpp:318`, `dbms/src/Server/RaftConfigParser.cpp:38`);
- `flash.proxy.config = "/tiflash-learner.toml"` passes a second TOML to
  the proxy. The separate `tiflash-learner.toml` file still exists and is
  documented (`docs/release-8.5/tiflash/tiflash-configuration.md:492+`:
  *"The parameters in tiflash-learner.toml are basically the same as those
  in TiKV"*); it is **optional** in v8.5.0 — proxy `addr`/`status-addr`/
  `data-dir`/`labels` can be set inline under `[flash.proxy]` (TiFlash's
  own CI compose `tests/docker/tiflash-dt.yaml` mounts both
  `/config.toml` and `/proxy.toml`).

Minimal `tiflash.toml` for the fixture:

```toml
listen_host = "0.0.0.0"
path = "/data/tiflash"            # or [storage.main] dir = ["/data/tiflash"]

[flash]
service_addr = "0.0.0.0:3930"

[flash.proxy]
addr = "0.0.0.0:20170"
advertise-addr = "tiflash:20170"
status-addr = "0.0.0.0:20292"
advertise-status-addr = "tiflash:20292"
engine-addr = "tiflash:3930"
data-dir = "/data/flash"

[raft]
pd_addr = "pd0:2379"

[status]
metrics_port = 8234

[logger]
log = "/logs/tiflash.log"
errorlog = "/logs/tiflash_error.log"
level = "info"
```

Ports for v8.5.0 (`docs/release-8.5/hardware-and-software-requirements.md`
network-port table, ~line 251-255, plus `tiflash-configuration.md`):

| port | component | source of default |
|---|---|---|
| `3930` | TiFlash RAFT + coprocessor (gRPC, incl. MPP) — `flash.service_addr` | `RaftConfigParser.cpp:38`, docs port table |
| `20170` | TiFlash Proxy service — `flash.proxy.addr` | `tiflash-configuration.md` (`"127.0.0.1:20170"`), docs port table |
| `20292` | TiFlash Proxy metrics/status — `flash.proxy.status-addr` | `tiflash-configuration.md` (`"127.0.0.1:20292"`), docs port table |
| `8234` | TiFlash Prometheus metrics — `status.metrics_port` | `dbms/src/Core/Defines.h:103` (`DEFAULT_METRICS_PORT 8234`), `dbms/src/Server/MetricsPrometheus.cpp:261-269` |
| `9000` | TiFlash TCP service — `tcp_port` | `tiflash-configuration.md`: default 9000 **but disabled when unset since v7.1.0**; `Server.cpp:623` binds it only `if (config.has("tcp_port"))`. Do not configure it. |
| `http_port` | legacy ClickHouse HTTP | In v8.5.0 `Server.cpp` no HTTP server is created — `http_params` at line 550 is dead code and no `config.getInt("http_port")` call exists. The key is silently accepted but nothing listens. |

### 2.4 TiDB (`/tidb-server`)

Flags registered in `pingcap/tidb@v8.5.0`, `cmd/tidb-server/main.go`,
`initFlagSet()` (lines ~193-256), with defaults:

- `--store` — default `unistore`; registered values `tikv`, `mocktikv`,
  `unistore` (`main.go:201`, `registerStores` at ~391-397). Use
  `--store=tikv`.
- `--path` — default `/tmp/tidb`; for `tikv` this is the PD endpoint list
  (`main.go:202`; `createStoreDDLOwnerMgrAndDomain` builds
  `tikv://<path>` at ~400-406, `pkg/store/driver/tikv_driver.go:127-131`
  parses it via `config.ParsePath` from `client-go/v2/config` into etcd
  addrs). Use `--path=pd0:2379` (comma-separated for multiple PDs).
- `--advertise-address` — TiDB's own address registered into cluster
  topology; set to the compose DNS name (`tidb`). Flag defined
  `main.go:97`, default `""`.
- `-P` / `--host` — SQL port `4000` (default) / bind host `0.0.0.0`.
- `--status` — HTTP status port, default `10080` (`main.go:227-228`),
  `--status-host` default `0.0.0.0`.
- `--config` — TOML config file path (`main.go:196`); needed for
  `alter-primary-key` (§5), `performance.enforce-mpp`, etc.

Minimal command (matches `docker-compose-tiflash-nightly.yml`):

```yaml
command:
  - --store=tikv
  - --path=pd0:2379
  - --advertise-address=tidb
  - --status=10080
  - --config=/tidb.toml        # optional
```

### 2.5 Ordering and readiness signals

Compose `depends_on` in the official examples orders `pd0 → tikv → tidb`,
with `tiflash` depending on `tikv` and `tidb`
(`docker-compose-tiflash-nightly.yml`). Functionally TiFlash needs PD +
TiKV (it joins as a raft-learner store of the TiKV cluster and pulls
replica data from TiKV); starting it after tidb also lets the DDL-owner
poll loop find the store immediately. Note `depends_on` alone waits only
for container start, not service readiness — healthchecks are required
for a deterministic fixture.

Verified readiness signals (v8.5.0 source):

- **PD**: `GET http://pd0:2379/pd/api/v1/health` → JSON array of members
  with `"health": true` (`pingcap/pd@v8.5.0`, `server/api/health.go:51-73`,
  route registered `server/api/router.go:334`; deprecated alias `/health`
  at `router.go:386-387`; `/ping` also registered). `pd-ctl health` wraps
  this API (`tools/pd-ctl/pdctl/command/health_command.go:24-38`).
  `pd-ctl store` shows registered TiKV/TiFlash stores
  (`tools/pd-ctl/pdctl/command/store_command.go`).
- **TiKV**: `GET http://tikv:20180/status` → HTTP 200 empty body once the
  status server is up (`tikv/tikv@v8.5.0`,
  `src/server/status_server/mod.rs:676`). `/metrics`, `/config`,
  `/engine_type` also served (lines 657-716).
- **TiDB**: `GET http://tidb:10080/status` → JSON
  `{connections, version, git_hash}` (`pingcap/tidb@v8.5.0`,
  `pkg/server/http_status.go:211`); final gate is a real MySQL handshake
  on `tidb:4000`.
- **TiFlash**: `GET http://tiflash:8234/metrics` (metrics exposer bound on
  `status.metrics_port`, `MetricsPrometheus.cpp:261-292`); proxy status
  server on `:20292` serves `/metrics` (docs port table) and
  `/tiflash/sync-status/keyspace/<ks>/table/<id>` — the exact endpoint
  TiDB calls to compute replica progress
  (`pingcap/tidb@v8.5.0`, `pkg/store/helper/helper.go:879-885`, consumed
  in `pkg/domain/infosync/tiflash_manager.go:94-99`). Whether the proxy
  status server also answers a plain `/status` like TiKV's is
  **UNVERIFIED** (the proxy is a TiKV-derived fork, but no primary source
  was confirmed). The authoritative end-to-end gate is
  `information_schema.tiflash_replica.AVAILABLE = 1` after
  `SET TIFLASH REPLICA` (§4). The TiFlash store itself appears in PD with
  label `engine=tiflash` — the proxy is launched with
  `--engine-label tiflash` (`dbms/src/Core/TiFlashDisaggregatedMode.h:21`,
  `DEF_PROXY_LABEL`; wired in `Server.cpp:321`).

## 3. PD scheduling under a single TiKV store

Defaults in `pingcap/pd@v8.5.0`, `pkg/schedule/config/config.go`:

- `replication.max-replicas` default `3` — `DefaultMaxReplicas = 3`
  (line 31), applied by `Adjust` (line 659).
- `replication.location-labels` default `[]` — `defaultLocationLabels`
  (line 84; `Adjust` line 666-667).
- `replication.enable-placement-rules` default **true** —
  `defaultEnablePlacementRules = true` (~line 66; `Adjust` line 660-662).
- `strictly-match-label` default false; `isolation-level` default "".

Interaction (important): `ReplicationConfig.EnablePlacementRules`'s own
comment says *"When PlacementRules feature is enabled. MaxReplicas,
LocationLabels and IsolationLabels are not used any more."*
(`config.go:613`). Two supported ways still make `max-replicas=1`
effective for a fresh fixture:

1. **Config file at bootstrap** — PD creates the default placement rule
   from `maxReplicas` when the rule manager initializes:
   `pkg/schedule/placement/rule_manager.go:82-127`, `Initialize(maxReplica,
   ...)` creates rule `pd/default` with `Role: Voter, Count: maxReplica`
   (lines 123-128). The official tiflash example sets both keys —
   `pingcap/tidb-docker-compose@master:config/pd-nightly-tiflash.toml`:
   ```toml
   [replication]
   enable-placement-rules = true
   max-replicas = 1
   ```
2. **Runtime** — `pd-ctl config set max-replicas 1` (the `set` subcommand,
   `tools/pd-ctl/pdctl/command/config_command.go:167-170`) or
   `POST /pd/api/v1/config/replication` (`server/api/config.go:426-441`).
   `server/server.go:1058-1125` (`SetReplicationConfig`): when placement
   rules are enabled, a max-replicas/location-labels/isolation-level
   change is propagated to the **default rule** (`rule.Count =
   int(cfg.MaxReplicas)` via `SetRule`) only if the default rule still
   exists and still matches the previous config values; otherwise it
   returns *"cannot update MaxReplicas, LocationLabels or IsolationLevel
   when placement rules feature is enabled and not only default rule
   exists, please update rule instead"*. TiFlash replica rules added by
   `SET TIFLASH REPLICA` are learner rules in a separate rule group
   (`placement.TiFlashRuleGroupID`) and do not count as "extra default
   rules" for this check.
   `pd-ctl config placement-rules` subcommands edit rules directly
   (`config_command.go:499+`).

Behavior with `max-replicas=3` (default) and exactly 1 TiKV store: every
region's voter rule wants 3 peers; PD can place only one, so all regions
permanently carry 2 missing peers (region health `miss_peer` /
insufficient replicas). The raft group still elects its single voter and
serves reads/writes — the cluster works for a smoke test but every
health/dashboard surface reports under-replication and any TiKV loss is
fatal. `max-replicas=1` removes that noise, which is why the official
tiflash example pins it. (Derived from `DefaultMaxReplicas`,
`RuleManager.Initialize`, and standard raft semantics — the
"cluster still serves traffic" part is reasoning, not a docs quote;
**UNVERIFIED** as an explicit statement in docs.)

## 4. `information_schema.tiflash_replica` semantics (v8.5.0)

Columns defined in `pingcap/tidb@v8.5.0`, `pkg/infoschema/tables.go:1199-1207`
(`tableTableTiFlashReplicaCols`): `TABLE_SCHEMA varchar(64)`, `TABLE_NAME
varchar(64)`, `TABLE_ID bigint`, `REPLICA_COUNT bigint`, `LOCATION_LABELS
varchar(64)`, `AVAILABLE tinyint`, `PROGRESS double`. Population:
`pkg/executor/infoschema_reader.go:2697-2743`
(`dataForTableTiFlashReplica`).

- `TABLE_SCHEMA`/`TABLE_NAME`/`TABLE_ID`: table identity; `TABLE_ID` is
  the cluster-internal table ID.
- `REPLICA_COUNT` / `LOCATION_LABELS`: copied verbatim from
  `tbl.TiFlashReplica.Count` / `tbl.TiFlashReplica.LocationLabels` (the
  meta written by `ALTER TABLE ... SET TIFLASH REPLICA n [LOCATION LABELS
  'l1','l2']` — note the docs warn multi-value `LOCATION LABELS` parses
  incorrectly: `create-tiflash-replicas.md:291-292`, *"do not use LOCATION
  LABELS"*).
- `PROGRESS`: computed, not stored. `infosync.MustGetTiFlashProgress` →
  `calculateTiFlashProgress`
  (`pkg/domain/infosync/tiflash_manager.go:119-148`):
  `progress = tiflashPeerCount / (regionCount * replicaCount)`, where
  `regionCount` comes from PD (`/stats/region`) and `tiflashPeerCount` is
  the number of TiFlash peers without replication lag, collected per
  store via `GET <store-status-addr>/tiflash/sync-status/...`
  (`helper.go:879-885`). Capped at 1 when PD balancing overshoots
  (`tiflash_manager.go:141-144`); truncated to 2 decimals
  (`infoschema_reader.go:2727`). Partitioned tables average per-partition
  progress (`infoschema_reader.go:2711-2719`). Docs describe it as
  replication progress in `[0, 1]` with *"1 means at least one replica is
  replicated"* (`create-tiflash-replicas.md:72`) — the source shows the
  exact ratio above.
- `AVAILABLE`: persisted meta flag (`tbl.TiFlashReplica.Available`,
  `infoschema_reader.go:2734`). It is flipped to `true` by the DDL
  owner's background poll when the computed progress reaches `1`
  (`pkg/ddl/ddl_tiflash_api.go:484-543`: `avail := progress == 1` →
  `UpdateTableReplicaInfo`). Per docs it is the planner's gate:
  `AVAILABLE=1` ⇒ optimizer may push to TiFlash, `0` ⇒ never
  (`information-schema/information-schema-tiflash-replica.md`), and
  *"Once the value of this field becomes 1 (available), it will not
  change anymore"* — matching `PollAvailableTableProgress`
  (`ddl_tiflash_api.go:353-432`), which only refreshes the progress cache
  for already-available tables and never sets `Available=false` there
  (it is recalculated when DDL changes the replica count;
  `create-tiflash-replicas.md:71`). Because `PROGRESS` is recomputed on
  demand, a transient `AVAILABLE=1` + `PROGRESS<1` state is possible
  after balancing/region growth — docs: queries pushed to TiFlash then
  risk timeout waiting for replication (`information-schema-tiflash-replica.md`).

`replica_count > #tiflash stores` is refused **at DDL statement time**,
not stalled: `checkTiFlashReplicaCount`
(`pkg/ddl/executor.go:3803-3811`) errors
`"the tiflash replica count: %d should be less than the total tiflash
server count: %d"` when `replicaCount > tiflashStoreCnt` (count == stores
is allowed; same check re-run in the DDL worker,
`pkg/ddl/table.go:1150-1157`). The store count comes from PD's store
list filtered to `ServerType == "tiflash"`
(`pkg/infoschema/tables.go:2178-2193`, `GetTiFlashStoreCount`), so with
zero TiFlash nodes even `REPLICA 1` errors out.

Verifying a SELECT ran on TiFlash:

- `EXPLAIN`/`EXPLAIN ANALYZE` `task` column: `cop[tiflash]` /
  `batchCop[tiflash]` for coprocessor/batch-cop scans and
  `mpp[tiflash]` for MPP fragments (with `ExchangeSender`/
  `ExchangeReceiver` operators) — `docs/release-8.5/explain-mpp.md`
  (lines ~48-57, 95-104 show `batchCop[tiflash]`, `cop[tiflash]`) and
  `tiflash/use-tiflash-mpp-mode.md` (lines ~99-109 show `mpp[tiflash]`,
  ~155-174 `batchCop[tiflash]`).
- `tidb_isolation_read_engines` (SESSION, default `tikv,tiflash,tidb`)
  restricts engines reads may use — `system-variables.md:3696+`. Setting
  it to `tiflash` forces TiFlash reads when a replica is available.
- `tidb_allow_mpp` (SESSION|GLOBAL, default `ON`) and `tidb_enforce_mpp`
  (SESSION, default `OFF`, seeded from `performance.enforce-mpp`):
  `enforce_mpp=1` makes the optimizer ignore cost and pick MPP —
  `system-variables.md:1105-1130, 3026-3041`. If blocked by missing
  replica it returns warning `MPP mode may be blocked because there
  aren't tiflash replicas of table ...` (`use-tiflash-mpp-mode.md:73-86`).

The replica mechanism itself: `SET TIFLASH REPLICA n` writes a PD
placement rule `Role: Learner`, `Count: n`, constraint `engine in
(tiflash)` over the table's key range — `makeBaseRule`,
`pkg/domain/infosync/tiflash_manager.go:334-348` — which is why placement
rules must stay enabled and why the TiFlash peer is a raft learner
(doesn't vote). `locationLabels` become the rule's label constraints.

## 5. `alter-primary-key` (v8.5.0)

- TOML key: **`alter-primary-key`, top-level** in `tidb.toml` (not
  `[security]` or any section) — `Config.AlterPrimaryKey`,
  `pkg/config/config.go:214-215`. Default **`false`** (`config.go:917`).
- Doc status: `docs/release-8.5/tidb-configuration-file.md` —
  `### alter-primary-key (Deprecated)`, *"takes effect only when the
  value of `@tidb_enable_clustered_index` is `INT_ONLY`"*; to add/remove
  a PK use `NONCLUSTERED` at create-table time. With `false` (default),
  `ADD/DROP PRIMARY KEY` is unsupported for existing tables.
- `tidb_enable_clustered_index`: SESSION|GLOBAL enum
  `OFF`/`ON`/`INT_ONLY`, **default `ON`**
  (`pkg/sessionctx/variable/sysvar.go:2390`); setting `INT_ONLY` emits a
  deprecation warning (`errWarnDeprecatedSyntax`, sysvar.go:2391-2395).
- Decision point in source: `ShouldBuildClusteredIndex`
  (`pkg/ddl/create_table.go:1581-1595`). When the statement does not
  explicitly say `CLUSTERED`/`NONCLUSTERED`:
  `ON` → clustered; `INT_ONLY` → `!AlterPrimaryKey && isSingleIntPK`;
  otherwise (`OFF`) → nonclustered. An explicit keyword wins
  (`opt.PrimaryKeyTp == PrimaryKeyTypeClustered`). Single-int-PK clustered
  tables get `PKIsHandle`, other clustered tables `IsCommonHandle`
  (`create_table.go:1276-1284`).

For `CREATE TABLE t (id BIGINT PRIMARY KEY AUTO_RANDOM)` — AUTO_RANDOM
requires the PK to be **clustered** (BIGINT, first PK column):
`setTableAutoRandomBits`, `create_table.go:772-799`, errors
`ErrInvalidAutoRandom` → *"auto_random is only supported on the tables
with clustered primary key"* (`pkg/meta/autoid/errors.go:37`) when
neither `PKIsHandle` nor `IsCommonHandle`; docs `auto-random.md:208`
(*"when the attribute of the primary key is NONCLUSTERED, AUTO_RANDOM is
not supported even on the integer primary key"*). Matrix:

| `tidb_enable_clustered_index` | `alter-primary-key` | PK type | AUTO_RANDOM result |
|---|---|---|---|
| `ON` (default) | any | clustered (PKIsHandle) | OK |
| `OFF` | any | nonclustered | error |
| `INT_ONLY` | `false` (default) | clustered | OK |
| `INT_ONLY` | `true` | nonclustered | error |
| explicit `PRIMARY KEY ... CLUSTERED` | any | clustered | OK |
| explicit `PRIMARY KEY ... NONCLUSTERED` | any | nonclustered | error |

`SHOW CONFIG` visibility: `alter-primary-key` **is** visible.
`SHOW CONFIG` → `fetchShowClusterConfigs` → `fetchClusterConfig`
(`pkg/executor/show.go:1492-1499`, `memtable_reader.go:179-228`): for a
`tidb` node it GETs `http://<statusAddr>/config`
(`http_status.go:244-246` returns `config.GetGlobalConfig()`), flattens
the JSON, and drops only keys matching `ContainHiddenConfig` —
`removedConfig` + `hideConfig` lists (`config.go:1109-1149, 1483-1486`)
do not contain `alter-primary-key`. There is **no** `tidb_alter_primary_key`
system variable (absent from `sysvar.go`; it is config-file only).

## 6. Resource floor per component (fixture, not production)

Documented **development/test** minimums
(`docs/release-8.5/hardware-and-software-requirements.md`, ~line 182-189)
are per-*machine*, not per-container: TiDB 8c/16 GiB; PD 4c/8 GiB; TiKV
8c/32 GiB (3 instances); TiFlash 32c/64 GiB. These are far above what a
smoke fixture needs; there is **no documented container minimum** —
fixture sizing below is reasoning from upstream defaults, marked
**UNVERIFIED** as a docs claim:

- Disk footprint: *"TiFlash occupies about 700 MB space and other
  components (such as PD, TiDB, and TiKV) occupy about 200 MB space
  respectively"* on the deployment server (same doc, ~line 298). Image
  pull sizes in §1 (compressed): pd ~210 MiB, tikv ~769 MiB, tiflash
  ~665 MiB, tidb ~202 MiB arm64.
- TiKV's RocksDB block cache defaults to 45% of total memory
  (`BLOCK_CACHE_RATE = 0.45`, `tikv/tikv@v8.5.0:src/config/mod.rs:97`;
  30% for raftstore-v2, line 100), so a small container limit simply
  shrinks the cache — a 1.5–4 GiB tikv container is a sane floor.
- TiFlash's higher baseline is architectural: a C++ ClickHouse-derived
  engine plus an embedded TiKV-fork raft proxy plus `mark_cache_size`
  (default 1 GiB, `config-template.toml`) — 4–8 GiB is a sane floor;
  **UNVERIFIED** beyond the "highest documented component requirement"
  and footprint facts above.
- PD and TiDB are pure-Go and modest: ~512 MiB–1 GiB pd, ~1–2 GiB tidb
  (**UNVERIFIED** — no docs minimum below the dev/test table).
- arm64 gate: TiFlash on ARM64 requires ARMv8 with `asimd`+`crc32`
  (`hardware-and-software-requirements.md` ~line 197) — satisfied by
  Apple Silicon / Linux aarch64 hosts.

## 7. `ALTER TABLE ... COMPACT TIFLASH REPLICA` (v8.5.0)

Executor: `pingcap/tidb@v8.5.0`, `pkg/executor/compact_table.go`;
planner `pkg/planner/core/planbuilder.go:5830-5848`; builder
`pkg/executor/builder.go:5640-5676`. Requires `ALTER` privilege and a
tikv-compatible store (`"compact tiflash replica can only run with tikv
compatible storage"`, builder.go:5648-5651).

- **Table without TiFlash replica** (`TiFlashReplica == nil || Count ==
  0`): statement returns after appending warning `compact skipped: no
  tiflash replica in the table` (`compact_table.go:86-91`) — warning
  no-op, zero TiFlash RPCs.
- **Table with a real replica**: `doCompact` spawns one `storeCompactTask`
  **per alive TiFlash store** (all stores with `ServerType == "tiflash"`
  from `GetStoreServerInfo`, `compact_table.go:50-62` — not only stores
  holding that table's peers), which compacts each physical table
  (table + partitions, or the `PARTITION` list) via serial `CmdCompact`
  RPCs (`tikvrpc.Request{Type: CmdCompact, StoreTp: TiFlash}` carrying
  `LogicalTableId`, `PhysicalTableId`, incremental `StartKey`,
  `compact_table.go:244-260`), paging on `CompactedEndKey` until
  `HasRemaining == false` (lines 311-333); each RPC may take up to 60 min
  (line 44). The statement blocks until all replicas finish
  (`sql-statement-alter-table-compact.md`: *"finished and returned when
  all replicas of a table are compacted"*; `KILL` can interrupt).
- Failure modes are surfaced as **statement warnings**, not errors
  (`_ = g.Wait()` — errors intentionally discarded, line 114):
  RPC failure → `compact on store %s failed: %v` (stops that store);
  `ErrCompactInProgress` → warning + cancels **all** stores' tasks
  (lines 269-281); `ErrTooManyPendingTasks` → `store is too busy`
  (stops that store, lines 283-290); `ErrPhysicalTableNotExist` →
  silent skip of that partition (lines 292-301); other →
  `internal error` warning.
- So yes — with a real replica the statement performs actual incremental
  compaction RPCs against every live TiFlash store; it is observably
  different from the no-replica warning no-op. Effect can be verified via
  `INFORMATION_SCHEMA.TIFLASH_TABLES` (`TOTAL_DELTA_ROWS` →
  `TOTAL_STABLE_ROWS`), per the doc's worked example
  (`sql-statement-alter-table-compact.md` tail).
- Version/deprecation status: since **v6.2.0** the `TIFLASH REPLICA`
  keywords are optional — bare `ALTER TABLE t COMPACT` is equivalent and
  still TiFlash-only (`sql-statement-alter-table-compact.md`, synopsis;
  same text in release-6.5/7.5/8.5 docs). The statement is **not**
  deprecated in release-8.5 docs and emits no deprecation warning in
  v8.5.0 source (checked `planbuilder.go:5830+`, `builder.go:5640+`,
  `compact_table.go`). It is supported only for TiFlash replicas —
  `COMPACT TIKV REPLICA` is rejected in the builder (`"compact %v replica
  is not supported"`, builder.go:5641-5644). TiCDC does not replicate it
  (doc compatibility section).

## Appendix — primary sources used

| fact | source |
|---|---|
| image digests/arm64 manifests, entrypoints, exposed ports, env | `https://hub.docker.com/v2/repositories/pingcap/{pd,tikv,tiflash,tidb}/tags/v8.5.0`; `registry-1.docker.io/v2/pingcap/<img>/manifests/...` + config blobs (anonymous token via `auth.docker.io`) |
| compose flag patterns | `github.com/pingcap/tidb-docker-compose@master`: `docker-compose.yml`, `docker-compose-tiflash-nightly.yml`, `config/pd-nightly-tiflash.toml`, `config/tiflash-nightly.toml`, `config/tiflash-learner-nightly.toml` |
| PD flags, defaults, health API, replication config, runtime rule update, pd-ctl | `pingcap/pd@v8.5.0`: `cmd/pd-server/main.go`, `server/config/config.go`, `pkg/schedule/config/config.go`, `pkg/schedule/placement/rule_manager.go`, `server/config/persist_options.go`, `server/server.go:1058+`, `server/api/{router,health,config}.go`, `tools/pd-ctl/pdctl/command/{health,config,store}_command.go` |
| TiKV flags, port/status defaults, pd endpoint default, `/status`, block-cache rate | `tikv/tikv@v8.5.0`: `cmd/tikv-server/src/main.rs`, `src/server/config.rs`, `src/server/status_server/mod.rs`, `components/pd_client/src/config.rs`, `src/config/mod.rs` |
| TiFlash subcommand/config handling, proxy embedding, ports, metrics, engine label | `pingcap/tiflash@v8.5.0`: `dbms/src/Server/{main,Server,RaftConfigParser,MetricsPrometheus,FlashGrpcServerHolder}.cpp`, `dbms/src/Core/{Defines.h,TiFlashDisaggregatedMode.h}`, `libs/libdaemon/{src/BaseDaemon.cpp,include/daemon/BaseDaemon.h}`, `etc/config-template.toml`, `tests/docker/{tiflash-dt.yaml,config/tiflash_dt.toml,config/proxy.toml}` |
| TiDB flags, store/path wiring, `/status`+`/config`, apk config, sysvar, clustered-index + auto_random checks, tiflash_replica + replica-count check + progress, compact executor, SHOW CONFIG path, tiflash placement rule, sync-status endpoint | `pingcap/tidb@v8.5.0`: `cmd/tidb-server/main.go`, `pkg/config/{config,config_util}.go`, `pkg/store/{driver/tikv_driver,helper/helper}.go`, `pkg/server/http_status.go`, `pkg/sessionctx/variable/{sysvar,session}.go`, `pkg/ddl/{create_table,table,executor,ddl_tiflash_api}.go`, `pkg/meta/autoid/errors.go`, `pkg/infoschema/tables.go`, `pkg/executor/{infoschema_reader,memtable_reader,show,builder,compact_table}.go`, `pkg/planner/core/planbuilder.go`, `pkg/domain/infosync/{info,tiflash_manager}.go` |
| documented ports, hardware mins, config keys, replica semantics, COMPACT status, EXPLAIN task values, auto_random/clustered-index/deprecation docs | `pingcap/docs@release-8.5`: `hardware-and-software-requirements.md`, `tiflash/{tiflash-configuration,tiflash-command-line-flags,create-tiflash-replicas,use-tiflash-mpp-mode}.md`, `information-schema/information-schema-tiflash-replica.md`, `tidb-configuration-file.md`, `system-variables.md`, `auto-random.md`, `clustered-indexes.md`, `sql-statements/sql-statement-alter-table-compact.md`, `explain-mpp.md` |
