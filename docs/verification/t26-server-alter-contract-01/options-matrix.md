# OPTIONS attribute matrix (A2–A4)

Vocabulary: `HOST | DATABASE | USER | PASSWORD | SOCKET | OWNER`
(char literals), `PORT` (numeric literal, documented 0–9999).

## Recognizer-level facts `[SRC]`

| shape | outcome |
|---|---|
| `USER 'u'` (known key, quoted literal) | admitted; `ValueKind=char` |
| `USER ''` | admitted — empty char literal parses (`quotedTokenClosed` only checks quoting) |
| `PORT 3307` | admitted; `RangeState=in-range` |
| `PORT 10000` | admitted; `RangeState=out-of-range` → owed `server.alter_options` |
| `PORT '3306'` | unclaimed (numeric key requires digit literal) |
| `PORT 99999999999999999999999` | unclaimed — overflow stays a parse rejection |
| `FOO 'x'` (unknown key) | unclaimed → base-parser diagnostic |
| `OPTIONS ()` | unclaimed — empty list rejected |
| `USER 'a', USER 'b'` | admitted; `DuplicateKeys=[user]` → owed `server.alter_options` |
| `HOST x` (unquoted value) | unclaimed — char members require a string literal |

## Native semantics `[DOC]`/`[PEND]`

| question | status |
|---|---|
| which attributes alterable | all seven keys adjust `mysql.servers` columns; no immutable option documented `[DOC]` |
| empty-string values | `USER ''`/`PASSWORD ''`/`HOST ''` — parser admits; native acceptance + stored effect `[PEND 80-3]` |
| duplicate keys | manual silent on ALTER-side outcome; last-wins vs error `[PEND 80-2]` — product already treats dirt as owed-fact only, never a refusal claim |
| unknown key | grammar-level rejection expected (7-key vocab) — error code `[PEND 80-8]` |
| NULL as a value | grammar has no NULL member — `USER NULL` unclaims (not a literal) `[SRC]` |
| PORT=0 | inside documented bound — admitted clean `[SRC]`; native acceptance `[PEND 80-4]` |
