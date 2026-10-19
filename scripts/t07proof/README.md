# T07 supplemental source/provider proof

Collects one fixed empty-table fixture per exact anchor to prove private
advanced-column facts through the real parser and MySQL-compatible provider.
It is not part of T07's 207 Golden identities or T06's 496 identities.

## Files

| File | Responsibility |
|---|---|
| main.go | Fixed source declarations, complete ordered catalog/provider facts, version/session identity, LIMIT-0 fields, environment-only connection, exclusive evidence writes and owned cleanup |
| ledger.go | Records all real query/exec/prepared/provider operations without DSNs, credentials or bound parameter values |
| main_test.go | Independent complete fact controls and corruptions for missing/duplicate/reordered/type/dependency/known/provenance/default facts plus exclusive JSON-write errors |

## Interface

Build from the repository at the tested code SHA:

```sh
go build -o /external/proof/native-probe ./scripts/t07proof
```

Run with `T07_USER` and `T07_PASS` supplied through the environment:

```sh
/external/proof/native-probe /external/proof/native-facts mysql80
```

Anchors are exactly mysql57/5.7.44, mysql80/8.0.46, mysql84/8.4.10 and
tidb85/8.5.0 on the existing fixed loopback ports. The `golden` fixture database
must already exist. Every anchor output directory must be new; no evidence is
overwritten. No arbitrary SQL input, expression evaluation, INSERT, user table,
query planning or credential/config discovery interface exists.

The independent fixture shapes contain 8 columns on MySQL5.7, 12 on MySQL8.0/
8.4, and 9 on TiDB8.5.0. All contain ordinary/null/empty-string controls,
STORED/VIRTUAL dependencies, literal nonreference and UUID column identity.
Supported anchors apply the independently observed ALTER UUID default. Modern
MySQL additionally contains a column-referencing default, JSON_OBJECT default
and ordinary/generated invisible columns. TiDB uses its CREATE JSON positive,
not a copied MySQL arithmetic default. Differences are selected by fixed
anchor identity, never by catching a failed DDL and silently dropping columns.

## Output and invariants

- Raw catalog rows retain original labels/bytes plus generation/default HEX.
- Typed projections record private known bits, references, storage and effective
  visibility; no new product response fields are introduced.
- Checks require exact complete names/order/count, expected type forms,
  independent reference/storage/default/visibility facts, empty-string
  HasDefault and real catalog attributes. Equality of two producer snapshots
  alone is not a proof.
- SELECT-star and explicit hidden fields use LIMIT 0 and must return no rows.
- The query ledger includes the one real LoadTableSnapshot and all provider SQL.
- A successful prior COUNT=0 is mandatory before CREATE. DROP runs only after
  this process successfully creates its fixture. Primary/cleanup/query/scan/
  iteration/write errors remain nonzero; final COUNT is separately recorded.
- Lifecycle completion and direct process rc, not summary.json alone, establish
  collection completion. Logs/projections do not replace the Golden native
  success/rejection and policy-result oracles.

## Exports

- `go build ./scripts/t07proof`: standalone fixed-anchor fact collector.
- `go test ./scripts/t07proof -run T07Proof`: constructed strict-fact and
  evidence-writer controls, not native/database acceptance.

## Dependencies

Uses existing application Parse/Extract, domain spec facts, real mysql metadata
provider and repository-pinned go-sql-driver/mysql. It does not alter product
transport behavior or depend on an unpinned external library.

## Update rule

Keep the source headers, this manifest and scripts/README.md synchronized.
Frozen fact expectations are authored independently, never rewritten from a
failing actual. Archive raw failed attempts instead of overwriting them.
