# Source facts — current ALTER SERVER product path (baseline ff319250)

All `[SRC]` — traced this round, nothing modified.

## Parse — `internal/infrastructure/parser/tidb/mysql_server.go`

- `parseMySQLServer` dispatches `alter server` → `mysqlServerAlter`
  (L180): claims `ALTER SERVER <name> OPTIONS (<members>)` whole-input;
  anything extra/malformed → false → base parser owns diagnostics.
- Name via `mysqlServerName`: bare word (reserved-word/pure-digit
  rejected), backquoted ident, or string literal. Only identifier
  forms mark `NameIdent=true` (provable catalog identity); string
  literal keeps the spelling but `NameIdent=false`.
- `mysqlServerOptions` (L226): `OPTIONS (` + ≥1 members + `)`, each
  `KEY literal`; the seven-key map `host|database|user|password|
  socket|owner` = char literal, `port` = unsigned digits.
  - unknown key → whole claim fails (unclaimed → base diagnostic);
  - empty list / dangling comma / missing value → unclaimed;
  - duplicate key → appended to `DuplicateKeys` — recorded, not
    rejected;
  - PORT digits parse → `RangeState=in-range` or `out-of-range`
    (>9999); overflow digit run → unclaimed (never `unproven`);
  - char members require `tokStr` + closed quotes — **empty string
    `''` parses fine**; unquoted/identifier values → unclaimed.
- Values are opaque: only key + value-kind + count recorded
  (`spec.ServerOption{Key, ValueKind, RangeState}`) — no literal
  payload reaches the declaration (privacy contract, A11).
- `NameSpellingRuneCount` = rune count of unquoted spelling — lexical
  measure only, NOT a proven 64-char verdict (decode gap registered).
- Fabricated node: `ast.AlterResourceGroupStmt` carrier +
  `DDLOperationAlterServer` + `ServerDeclaration{Kind:"alter"}`.

## Boundary — `coverage_boundary.go` `mysqlServerAuditedForm`

- ALTER admitted iff `NameIdent && NameSpellingRuneCount <= 64`.
  No wrapper gate (ALTER has no wrapper clause). String-literal name
  or >64-rune spelling → not the audited form → `Unsupported`.
- Non-MySQL dialect → vendor boundary before any of this.

## Version gate — `metadata.go` (enrich entry)

- Shape-passing CREATE/ALTER with `resolvedVersion` outside the
  registered MySQL 8.0.x domain → `Unsupported{boundary:"unaudited",
  gate:"version"}` — before the family loader, so a version-failed
  ALTER can never trigger the privilege observation.
- Conflicting version inputs (malformed pin, product/pin mismatch)
  exit earlier as typed errors.

## Privilege premise — `server_drop_premise.go` / metadata wiring

- ALTER has its own projection `serverAlterPrivilegePremise()` —
  `ServerAlterPrivilege{Unproven|Consumable|Suppressed}` from the
  request-level `LoadMySQLServerDropPrivilege` observation (one
  dedicated conn: `select current_user()` + `show grants for
  current_user()`), gated on `PrivilegeAuditMode=
  observed_account_new_session` + in-domain version + positive
  witness (SUPER or ALL→SUPER static-set on a direct `ON *.*` line).
- Never borrowed: `server.drop_privilege`/`server.create_privilege`
  satisfaction doesn't feed ALTER.

## Bind — `bindMySQLServer` (batch_state_server_srs.go)

- Ordered-channel exits first (contamination → unvouched; taint/
  relation barrier → `UnreadReason=batch_effects`).
- Clean channel + `NameIdent` → ONE existence read via resolver:
  `select server_name from mysql.servers where server_name = ?`
  (`loadMySQLServerObject`, provider_server_srs.go:55). Only
  `server_name` selected — connection columns never read (data
  minimalisation). Row → `Exists`, `Status=confirmed`; zero rows →
  `Read=true`, `Exists` unset = **unknown, never absent** (grant-
  scoped visibility, no completeness contract); errors propagate.
- `NameIdent=false` → `UnreadReason=identity_unproven`, no SQL.

## Adjudication — `spec.AdjudicateServerAlter` (server_alter_premise.go)

Arm order A1→A2→A3→A4 (first-match), inputs: `state *ServerState`,
`privilege`, `optionsClean`, `contaminated`:
- **A1 Barrier**: `contaminated` or `UnreadReason=batch_effects` →
  owes `{server.batch_effects}`, mints nothing.
- **A2 TerminalRefusal**: `OrderedAbsent` on the identity → owed ∅;
  privilege/options never reached; conditional source kept.
- **A3 ProvenAlter**: `OrderedPresent || (Read && Exists)` +
  consumable + `optionsClean` → `PublishPresent=true`, owed ∅.
- **A4 Unproven**: else — owed = `{existence?} + {alter_privilege?}
  + {alter_options?}` per which premises failed; `PublishPresent`
  still set when presence was proven (preservation republish).
- `OptionsClean()` = no `DuplicateKeys` AND every numeric member
  `in-range`. Dirt is owed-fact only, never a refusal claim.

## Apply — `batch_state_server.go` (presence-preserving)

- `Kind=alter && NameIdent`: `PublishPresent` → republish
  `OrderedPresent` (`presentConditional = ConditionalSource`),
  `serverInvalidateOthers` retires unproven-relation entries.
- Every other arm: **no-op** — no entry, no barrier, no write mint.
- Boundary/literal-name ALTER: unchanged accepted behavior — write
  record + relation barriers / channel taint.

## Rule — `ddl.mysql.server.alter.notice` (mysql_server_rules.go:516)

- Projects the adjudication: findings/gaps/messages read the owed
  set; `conditional_audit` marker follows `ConditionalAudit`.
- `PublicSQL` = `ALTER SERVER <redacted>` — fixed redaction, option
  values never appear anywhere.
