# Parser → AST → declaration mapping (`[SRC]`)

## Claim boundary

`parseMySQLServer` runs before the base parser. It claims ONLY the
pure whole-input shape; false → the base grammar path owns the
statement and diagnostics. The recognizer never emits partial claims.

```
ALTER SERVER <name> OPTIONS (<member> [, <member>]...)
```

- `<name>`: bare word | backquoted ident | string literal
- `<member>`: `KEY literal` from the 7-key vocabulary
- no guard forms exist — `IF EXISTS`/`IF NOT EXISTS` → unclaimed
- `FOREIGN DATA WRAPPER` is not part of the ALTER grammar — extra
  tokens after `OPTIONS (...)` → unclaimed

## Token → fact mapping

| input element | derived fact | notes |
|---|---|---|
| bare-word name | `Name` + `NameIdent=true` | reserved word or pure digits → rejected upstream, never claimed |
| backquoted name | `Name` + `NameIdent=true` | ```` `` ```` → `` ` `` unescape only = complete ident decode |
| `'name'` / `"name"` | `Name` + `NameIdent=false` | doubled-quote unescape only — backslash/charset-introducer variance keeps catalog identity unprovable |
| option key | `Key` (lowercased) | must be in the 7-key map else whole claim fails |
| char value | `ValueKind="char"` | closed quoted literal required; `''` admitted |
| numeric value | `ValueKind="numeric"` + `RangeState` | unsigned digits; >9999 → `out-of-range`; overflow → unclaimed |
| repeated key | `DuplicateKeys += key` | grammar admits; adjudication owes `server.alter_options` |
| name spelling | `NameSpellingRuneCount` | lexical count only — NOT a proven ≤64 verdict |

## What never survives the recognizer

Option literal payloads (HOST/USER/PASSWORD/…), quote style of
option values, member ordering beyond the recorded list, the raw
value bytes. `PublicSQL` renders `ALTER SERVER <redacted>`.

## Carry node

`ast.AlterResourceGroupStmt{ResourceGroupName}` fabricated carrier +
`mysqlServerStatement.declaration` — the real AST type keeps
classify/coverage paths unchanged.
