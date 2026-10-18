# T06-A17 declaration matrix — declared SQL → AST → spec.Column

Real parser/extractor output under the fix (`declaredColumnType`), asserted by
`internal/infrastructure/parser/tidb/extractor_t06a17_test.go` on both mysql and
tidb dialect paths (adapter output identical across dialects).

| declaration | spec.Column.Type | spec.Column.Length | notes |
|---|---|---|---|
| `DECIMAL(12,2)` | `decimal(12,2)` | 12 | scale kept |
| `DECIMAL(12,3)` | `decimal(12,3)` | 12 | scale kept |
| `DECIMAL(12,0)` | `decimal(12,0)` | 12 | explicit 0 kept (was elided by String()) |
| `DECIMAL(12)` | `decimal(12)` | 12 | no phantom `,0` |
| `DECIMAL` | `decimal` | -1 | sentinels unspecified; no phantom `(10,0)` |
| `FLOAT(10,2)` | `float(10,2)` | 10 | |
| `FLOAT(10,3)` | `float(10,3)` | 10 | |
| `FLOAT(10,0)` | `float(10,0)` | 10 | explicit 0 kept |
| `FLOAT` | `float` | -1 | |
| `DOUBLE(16,2)` | `double(16,2)` | 16 | |
| `DOUBLE(16,4)` | `double(16,4)` | 16 | |
| `DOUBLE(16,0)` | `double(16,0)` | 16 | explicit 0 kept |
| `DOUBLE` | `double` | -1 | |
| `ENUM('Ready','Closed')` | `enum('Ready','Closed')` | -1 | member case kept |
| `ENUM('ready','closed')` | `enum('ready','closed')` | -1 | distinct from above |
| `ENUM('Closed','Ready')` | `enum('Closed','Ready')` | -1 | order kept |
| `SET('Read','Write')` | `set('Read','Write')` | -1 | |
| `SET('read','write')` | `set('read','write')` | -1 | |
| `SET('Write','Read')` | `set('Write','Read')` | -1 | |
| `ENUM('','A,B','A)B','O''Reilly')` | `enum('','A,B','A)B','O''Reilly')` | -1 | 4 members, no split/truncate |
| `SET('Read','A)B','O''Reilly')` | `set('Read','A)B','O''Reilly')` | -1 | |
| `FLOAT(10)` | `float` | -1 | parser normalizes p≤24 → FLOAT; p text not recoverable (AST fact) |
| `FLOAT(30)` | `double` | -1 | parser normalizes p≥25 → DOUBLE; source `float` keyword not recoverable |
| `INT` | `int(11)` | 11 | non-target: existing rendering kept |
| `VARCHAR(8)` | `varchar(8)` | 8 | non-target |
| `BIGINT UNSIGNED` | `bigint(20) unsigned` | 20 | non-target |

Backslash member control: declaration member `'a\\b'` (SQL bytes `a`, `\`, `\`,
`b`) decodes to the two-character member `a\b`; `declaredColumnType` re-escapes
the backslash so the rendered `enum('a\\b','plain')` re-parses to identical
bytes. `TestT06A17MemberRoundTrip` re-parses every produced Type string inside
a `CREATE TABLE x (c <type>)` wrapper and asserts identical member bytes and
order.

Unsigned/zerofill/charset attributes on the five types are carried by the same
`FieldType.Restore` call (e.g. `DECIMAL(12,2) UNSIGNED` → `decimal(12,2) unsigned`).
