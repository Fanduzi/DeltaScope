# TiDB 8.5.0 recovery and flashback probes

The DeltaScope CLI was built from `10354d2bf167f0741a6eeb079e72cb981b4d15a8`. TiDB returned `8.0.11-TiDB-v8.5.0`. The TiDB image was `pingcap/tidb@sha256:490861bd4a2fa643dbe76b77ffecafcad66e6a42774e580b82f4ec2429a80e57`. The TiKV fixture used one `pingcap/pd:v8.5.0` and one `pingcap/tikv:v8.5.0`, with all three containers on an isolated bridge and only TiDB published to a loopback port. The run removed the three owned containers and their network.

## Fixture

Use an empty disposable TiKV-backed TiDB instance. Wait for the GC worker to publish a row in `mysql.tidb` with `variable_name = 'tikv_gc_safe_point'`. Run the following setup while retaining the default GC lifetime.

```sql
CREATE DATABASE recovery_fixture;
CREATE DATABASE dropped_fixture;
USE recovery_fixture;
CREATE TABLE flash_src (id INT PRIMARY KEY);
INSERT INTO flash_src VALUES (7);
CREATE TABLE recover_src (id INT PRIMARY KEY);
INSERT INTO recover_src VALUES (13);
CREATE TABLE dropped_fixture.witness (id INT PRIMARY KEY);
INSERT INTO dropped_fixture.witness VALUES (17);
CREATE TABLE job_src (id INT PRIMARY KEY);
INSERT INTO job_src VALUES (31);
DROP TABLE job_src;
CREATE TABLE job_src (id INT PRIMARY KEY);
INSERT INTO job_src VALUES (37);
DROP TABLE job_src;
CREATE TABLE occupied (id INT PRIMARY KEY);
INSERT INTO occupied VALUES (23);
DROP TABLE flash_src;
DROP TABLE recover_src;
DROP DATABASE dropped_fixture;
ADMIN SHOW DDL JOBS 30;
```

Record the smaller of the two `drop table` job IDs for `recovery_fixture.job_src` and the `create table` job ID for `recovery_fixture.occupied`. For each row below, audit the statement before running the native comparison. Use `deltascope audit --dialect tidb --format json --fail-on none --sql` with the authored statement. Run the native cases in table order in the `recovery_fixture` database. Replace `<create-job>` and `<first-drop-job>` with those observed IDs.

| Statement | Native TiDB result | DeltaScope result |
| --- | --- | --- |
| `FLASHBACK TABLE recovery_fixture.flash_src TO occupied` | `1050` / `42S01`, destination already exists. `occupied.id` remains `23`. | Exit 1, `flashback_table` unsupported. |
| `FLASHBACK TABLE recovery_fixture.flash_src TO flash_restored` | Success, `flash_restored.id = 7`. | Exit 1, `flashback_table` unsupported. |
| `RECOVER TABLE recovery_fixture.recover_src` | Success, `recover_src.id = 13`. | Exit 1, `recover_table` unsupported. |
| `RECOVER TABLE recovery_fixture.unknown_src` | `1105` / `HY000`, table absent from DDL history. | Exit 1, `recover_table` unsupported. |
| `FLASHBACK DATABASE dropped_fixture` | Success, `dropped_fixture.witness.id = 17`. | Exit 1, `flashback_database` unsupported. |
| `RECOVER TABLE BY JOB <create-job>` | `1105` / `HY000`, the job is `create table`. | Exit 1, `recover_table` unsupported. |
| `RECOVER TABLE BY JOB <first-drop-job>` | Success, `job_src.id = 31`. | Exit 1, `recover_table` unsupported. |
| `FLASHBACK CLUSTER` | `1064` / `42000`, missing a recovery point. | Exit 2, `parser_error`. |
| `FLASHBACK CLUSTER TO TIMESTAMP '2022-09-21 16:02:50'` | Documented syntax. The probe exercised the audit only. | Exit 1, `flashback_cluster` unsupported. |
| `FLASHBACK CLUSTER TO TSO 445494839813079041` | Documented syntax. The probe exercised the audit only. | Exit 1, `flashback_cluster` unsupported. |

All ten audit results had `incomplete` coverage. The TiDB unistore fixture in T16 exercises the separate missing-safe-point refusal. Its table and database restore statements returned `1105` / `HY000` with `can not get 'tikv_gc_safe_point'`.

The TiKV successes certify the fixture only. A recovery assessment would need the selected DDL job and snapshot, GC and object facts for that instance, plus topology facts for cluster recovery. [TiDB 8.5 recovery documentation](https://docs.pingcap.com/tidb/v8.5/sql-statement-recover-table/) · [Cluster flashback syntax](https://docs.pingcap.com/tidb/v8.5/sql-statement-flashback-cluster/)
