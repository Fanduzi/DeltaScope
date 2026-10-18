# T06-A17 pairing table: AST → formatted text → spec.Column / catalog row → snapshot

Baseline: `a08190969237404b280cfcc8d2cec8c01a082316` (HEAD == origin/workfree).
Parser: `github.com/pingcap/tidb/pkg/parser v0.0.0-20260622124716-26db2d85d6cd` (locked in go.mod).
Probe: `probe_main.go.txt`, binary sha256 `edc51c36…98354`, go1.26.1 darwin/arm64.
All runs rc=0. MySQL-dialect and TiDB-dialect extraction outputs are byte-identical
(same adapter + parser; dialect is not consulted by `extractColumn`). Provider runs for
both dialect labels produced identical output (same `parseColumnType` normalizer).

## N1 DECIMAL (extract/N1-mysql.json)

| declared | AST flen | AST dec | CompactStr/String | Restore | spec.Type | spec.Len |
|---|---|---|---|---|---|---|
| `DECIMAL(12,2)` | 12 | 2 | `decimal(12,2)` | `DECIMAL(12,2)` | `decimal(12,2)` | 12 |
| `DECIMAL(12,3)` | 12 | 3 | `decimal(12,3)` | `DECIMAL(12,3)` | `decimal(12,3)` | 12 |
| `DECIMAL(12,0)` | 12 | 0 | `decimal(12,0)` | `DECIMAL(12,0)` | `decimal(12,0)` | 12 |
| `DECIMAL(12)` | 12 | -1 | `decimal(12,0)` | `DECIMAL(12)` | `decimal(12,0)` | 12 |
| `DECIMAL` | -1 | -1 | `decimal(10,0)` | `DECIMAL` | `decimal(10,0)` | -1 |

- `(12,2)` vs `(12,3)`: distinguishable in Type.
- `(12,0)` vs `(12)`: AST sentinels (0 vs -1) and Restore distinguish; **String/CompactStr/spec.Type merge both to `decimal(12,0)`** (upstream substitutes omitted scale).
- bare `DECIMAL`: AST -1/-1 vs Type `decimal(10,0)` + Len -1 — phantom default `(10,0)` in Type text.

## N2 FLOAT/DOUBLE (extract/N2-mysql.json)

| declared | AST flen | AST dec | String | Restore | spec.Type | spec.Len |
|---|---|---|---|---|---|---|
| `FLOAT(10,2)` | 10 | 2 | `float(10,2)` | `FLOAT(10,2)` | `float(10,2)` | 10 |
| `FLOAT(10,3)` | 10 | 3 | `float(10,3)` | `FLOAT(10,3)` | `float(10,3)` | 10 |
| `FLOAT(10,0)` | 10 | 0 | `float` | `FLOAT(10,0)` | `float` | 10 |
| `FLOAT` | -1 | -1 | `float` | `FLOAT` | `float` | -1 |
| `DOUBLE(16,2)` | 16 | 2 | `double(16,2)` | `DOUBLE(16,2)` | `double(16,2)` | 16 |
| `DOUBLE(16,4)` | 16 | 4 | `double(16,4)` | `DOUBLE(16,4)` | `double(16,4)` | 16 |
| `DOUBLE(16,0)` | 16 | 0 | `double` | `DOUBLE(16,0)` | `double` | 16 |
| `DOUBLE` | -1 | -1 | `double` | `DOUBLE` | `double` | -1 |

- `(10,2)` vs `(10,3)` / `(16,2)` vs `(16,4)`: distinguishable in Type.
- `FLOAT(10,0)` vs `FLOAT`: Type identical (`float`), still distinguishable via Len 10 vs -1 — but the explicit `(M,0)` parameter text is elided upstream; Restore preserves it.
- `FLOAT(10)` single-arg (precision-in-bits form) not in fixture; not claimed.

## L1 ENUM/SET (extract/L1-mysql.json)

| declared | AST GetElems | String/CompactStr | spec.Type |
|---|---|---|---|
| `ENUM('Ready','Closed')` | `['Ready','Closed']` | `enum('Ready','Closed')` | `enum('ready','closed')` |
| `ENUM('ready','closed')` | `['ready','closed']` | `enum('ready','closed')` | `enum('ready','closed')` |
| `ENUM('Closed','Ready')` | `['Closed','Ready']` | `enum('Closed','Ready')` | `enum('closed','ready')` |
| `SET('Read','Write')` | `['Read','Write']` | `set('Read','Write')` | `set('read','write')` |
| `SET('read','write')` | `['read','write']` | `set('read','write')` | `set('read','write')` |
| `SET('Write','Read')` | `['Write','Read']` | `set('Write','Read')` | `set('write','read')` |

- Member **order preserved** at every layer; member **case destroyed** by `strings.ToLower(col.Tp.String())` in `extractColumn` — `e_upper` vs `e_lower` merge to identical spec.Type. Real content change (member literals are values, case-sensitive under bin collations).

## L2 edge members (extract/L2-mysql.json)

| declared | AST GetElems (hex) | spec.Type |
|---|---|---|
| `ENUM('','A,B','A)B','O''Reilly')` | `['','A,B','A)B',"O'Reilly"]` (`41 2c42`…; `''` escaped quote decoded to `'`) | `enum('','a,b','a)b','o''reilly')` |
| `SET('Read','A)B','O''Reilly')` | `['Read','A)B',"O'Reilly"]` | `set('read','a)b','o''reilly')` |

- Structural fidelity of members (empty string, comma, `)`, escaped `'`) survives ToLower — only case is folded.

## C controls (extract/C-mysql.json)

| declared | String | spec.Type | spec.Len | Unsigned |
|---|---|---|---|---|
| `INT` | `int(11)` | `int(11)` | -1 | false |
| `VARCHAR(8)` | `varchar(8)` | `varchar(8)` | 8 | false |
| `BIGINT UNSIGNED` | `bigint(20) UNSIGNED` | `bigint(20) unsigned` | -1 | true |

Unchanged per contract (phantom display width `int(11)` with Len -1 is pre-existing accepted behavior; `unsigned` flag captured in dedicated field).

## Provider (provider/provider-{mysql,tidb}.json; constructed catalog rows)

Query ledger (identical both labels): `information_schema.tables` QueryRow → `information_schema.columns` (ordered by ordinal_position) → `information_schema.statistics`.

| input column_type | out Type | out Len | distinguishable? |
|---|---|---|---|
| `decimal(12,2)` / `decimal(12,3)` / `decimal(12,0)` | `decimal` | 12 | **merged — scale dropped** |
| `float(10,2)` / `float(10,3)` / `float(10,0)` | `float` | 10 | **merged — second param dropped** |
| `double(16,2)` / `double(16,4)` / `double(16,0)` | `double` | 16 | **merged** |
| `enum('Ready','Closed')` / `enum('ready','closed')` / `enum('Closed','Ready')` / `enum('','A,B','A)B','O''Reilly')` | `enum` | 0 | **merged — all members dropped** |
| `set('Read','Write')` / `set('read','write')` / `set('Write','Read')` | `set` | 0 | **merged** |
| `int` / `varchar(8)` / `bigint unsigned` | `int` / `varchar` / `bigint` | 0 / 8 / 0 | consistent (`unsigned` → dedicated field) |

Provider input rows are constructed fixtures; this proves normalization behavior, not live-server output.

## AuditSQL boundary (audit/audit-*.json, all-rules-disabled policy, no provider)

All 10 runs: verdict `pass`, coverage `complete`, 1 statement `ddl`, 0 findings, no evidence gaps, no unsupported, no diagnostics, `err` nil. Public `StatementResult` does not expose column detail (`has_column_detail=false`) — boundary confirms representation is internal; findings neither prove nor disprove fidelity.
