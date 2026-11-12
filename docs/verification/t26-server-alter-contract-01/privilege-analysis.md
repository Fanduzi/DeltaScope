# Privilege analysis (A6)

## The premise today `[SRC]`

`server.alter_privilege` is ALTER's own premise token — never
borrowed from `server.drop_privilege`/`server.create_privilege`.
The shared observation `LoadMySQLServerDropPrivilege` runs once per
request on a dedicated conn: `select current_user()` +
`show grants for current_user()`; `MySQLServerDropPrivilegeWitness`
consumes it — bound account + in-domain version + exact-token
`SUPER` (or `ALL PRIVILEGES` under the registered 8.0.x static-set
proof) on a direct `ON *.*` line. Non-global scopes, role lines,
near-tokens never substitute.

## Declared executor semantics

`PrivilegeAuditModeObservedAccountNewSession` binds the observed
`CURRENT_USER()` as the **expected** execution principal — caller-
declared conditional, never verified (R2 of the dependency-absence
round just re-confirmed this is the only principal-binding pattern
in the architecture).

## Check order — privilege vs existence `[PEND]`

Manual documents SUPER as required; whether MySQL checks the grant
before or after the row-existence check is not evidenced. The SRS
precedent (3631 SUPER precedes dependency logic) suggests privilege
first, but that is inference — probe 80-7 proposes the discriminating
case: USAGE-only user, absent name — if the answer is 1227-class
privilege error, the order is privilege→existence; if the absent-
object error surfaces, the order reverses. Not asserted.

## Boundary

`serverAlterPrivilegePremise` can be `Suppressed` when an earlier
authored grant/role statement ended the applicable range — the
premise stays owed, never silently satisfied.
