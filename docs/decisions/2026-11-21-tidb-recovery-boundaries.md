# TiDB 8.5 recovery and flashback evidence

Status Accepted

## Decision

Keep table, database and cluster recovery on the existing unsupported rail. The audit can identify recovery statements and report that their semantics are outside coverage. Recovery success depends on TiDB's current DDL history, the snapshot, garbage collection, target occupancy and topology. Reading a live table directory does not establish that the source is recoverable. The auditor does not execute recovery to obtain those facts.

`FLASHBACK CLUSTER TO TIMESTAMP` and `FLASHBACK CLUSTER TO TSO` parse and produce `flashback_cluster` unsupported entries. The previous inventory used `FLASHBACK CLUSTER` without a recovery point. Both TiDB 8.5.0 and the bound parser reject that incomplete form. The inventory records the recognized forms and retains the bare statement as a negative control.

## Verification

The canonical T16 fixtures cover the bound parser and read-only audit results. The unistore native cases fail for a missing `tikv_gc_safe_point` and retain the original fixtures. A separate disposable TiDB 8.5.0 TiKV/PD fixture with a real safe point confirms an actual restore, destination collision, absent history and job-type refusal. No fixture executes the cluster-wide recovery forms.

- [T16 recovery boundary cases](../../testdata/ddl-golden/T16.json)
- [Native TiDB 8.5.0 procedure and results](../../testdata/ddl-inventory/evidence/tidb.flashback-recover.native.md)
- [FLASHBACK TABLE](https://docs.pingcap.com/tidb/v8.5/sql-statement-flashback-table/)
- [FLASHBACK DATABASE](https://docs.pingcap.com/tidb/v8.5/sql-statement-flashback-database/)
- [FLASHBACK CLUSTER](https://docs.pingcap.com/tidb/v8.5/sql-statement-flashback-cluster/)
- [RECOVER TABLE](https://docs.pingcap.com/tidb/v8.5/sql-statement-recover-table/)

T16 preview passed 66 cases and 974 assertions. The four recovery cells bind six canonical case references and the native annotation.
