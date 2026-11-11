# T26-SERVER-DROP-PREMISE-01 — execution-premise contract (R1)

Contract for the last unwired premise of the accepted DROP SERVER
slice: `server.drop_privilege`. R1 revision: the recommended supply
narrows to the single `observed_account_new_session` conditional-audit
mode with direct global-grant positive witness only — three outcomes
(positive witness / unproven / bounded error), nothing else.

## 1. Core answers

**Q1 — is the expected execution principal represented?** Not by any
existing request entry. `Request` (internal `service.go:40`, public
`pkg/deltascope/audit.go:101`) carries no executor/principal/account
field; `TargetIdentity` binds `DeploymentEvidence` to an *instance*,
not an account (source-excerpts A.1, A.2). This contract therefore
proposes one minimal explicit request entry — the semantic mode
`observed_account_new_session` (Go field/type naming per project
style, documented here only):

> 本次条件审计以 provider 连接观测到的 `CURRENT_USER()` 账号为预期执行主体；
> 预期执行环境为同一目标上、在本次观测后建立的新执行会话。
> 结果以所依赖的直接全局授权在观测后至执行会话加载权限时仍然适用为条件。

Three things stay strictly separated:

1. actually observed: the account, the target, the direct global-grant
   positive witness — read on one actual connection;
2. the request's explicitly selected audit applicability condition —
   the mode above, which states *whose* audit this is and *which*
   execution context it conditions on;
3. the unobserved future: this tool does not observe future execution.

What the mode must **not** claim: that the future actual executor was
verified; that the current pooled/provider session holds effective
`SUPER`; that the system created a new execution session; or that the
system guarantees a future session loads privileges per this snapshot.
When the mode is not selected, `server.drop_privilege` stays owed even
if the reader's grants carry `SUPER`; likewise when principal/target
binding is missing, the context does not cover the statement, or the
evidence mismatches.

Why a *new* session framing, not "same session": `SUPER` is a static
global privilege — official semantics say changes to static global
privileges do not take effect on already-connected clients
(privilege-changes doc). A grant snapshot therefore cannot prove that
an *existing* connection currently holds it (counter-example: the
connection was established before `SUPER` was granted elsewhere).
And a dedicated `*sql.Conn` from `DB.Conn()` may be a pooled existing
connection — not a fresh physical one — so "the reader session's
effective privileges" is itself unprovable from this observation. The
honest condition is: a *new* session established after the observation
loads the account's then-current grants — the witness stays valid
while that grant remains applied.

**Q2 — which relationships are code-guaranteed?**
- `SHOW GRANTS FOR CURRENT_USER()` on one connection provably answers
  that connection's own account's base grants (doc + B.2) — guaranteed
  only for the account *the provider authenticates as*.
- reader ↔ declared executor: the mode above *is* the declaration; no
  pre-existing field binds them.
- reader ↔ future actual executor: unproven — the mode is an audit
  applicability condition, not an observation.

**Q3 — can the T24 premise-bundle be reused as-is?** No. Reusable
mechanics only: `loadMySQLGrantLines` + `collectMySQLGlobalPrivileges`
(`ON *.*`, exact-token), the dedicated-session read discipline, and
propagate-don't-launder errors. Not reusable as-is: the whole
`LoadMySQLEffectivePrivileges` orchestration — its `USING` branch
swallows `augErr` into a partial result (R1-C finding), and its
completeness bit reflects role-channel readability, which is out of
this slice's scope since role augmentation is deferred; the
`LoadMySQLResourceGroupFacts` bundle (unrelated channels, RG family
gate); `MySQLReadOnlyExempt` (answers `SUPER||CONNECTION_ADMIN||ALL`,
a different question — `CONNECTION_ADMIN` does not satisfy DROP
SERVER's `SUPER`); `DeploymentEvidence` (T24 fields only, nested in
the RG gate — offline privilege bundle wiring is deferred, not
proposed for this slice).

**Q4 — minimal evidence source for SUPER?** The base channel alone:
`SHOW GRANTS FOR CURRENT_USER()` on the provider connection,
filtered to `ON *.*` lines, exact-token `SUPER` — or `ALL PRIVILEGES`
only under a registered proof that the global `ALL PRIVILEGES` set on
the applicable version contains `SUPER` (a static set; see §4 domain).
No `CURRENT_ROLE()`/`USING`, no denial inference, no offline bundle,
no general account/role/privilege engine — all deferred, with no
"reviewer picks later" branch kept.

**Q5 — missing / mismatched / out-of-scope / read-failure:** three
outcomes only — §3 table.

**Q6 — seams and the consumption condition:** implementation-plan.md;
consumption requires the explicit mode selected *and* all §2
conditions *and* the per-statement scope projection covering the
statement.

## 2. Consumption contract for privilege evidence

A `server.drop_privilege` proof is consumable only when ALL of:

1. **Source** — the base `SHOW GRANTS FOR CURRENT_USER()` read on the
   provider's own connection; `CURRENT_USER()` and `SHOW GRANTS` come
   from the *same* actual connection. No caller/policy boolean is ever
   a proof — no bare `privilege=true` trust switch.
2. **Target binding** — the connection's own audited target, by
   construction of the read.
3. **Principal & context binding** — the `observed_account_new_session`
   mode selected by the request, binding the observed account as the
   expected principal for a post-observation new execution session on
   the same target. Missing selection, unbound principal, or a
   context that does not cover the statement → not consumable even
   with a positive grant.
4. **Product/version/format applicability** — MySQL dialect only, and
   the resolved version must fall inside the registered domain in §4;
   targets outside stay unproven — no inference from approximate
   syntax.
5. **Scope & timeliness** — one read produces request-level immutable
   raw evidence; an ordered scan computes the applicable range; each
   statement gets its own consumable projection. An authored
   `SET ROLE` / `GRANT` / `REVOKE` earlier in the batch conservatively
   ends the range for later statements — a *scope policy of this
   model*, not a claim that each such statement immediately changes an
   existing session's static `SUPER`.
6. **Immutability & merge** — the raw observation is immutable;
   per-statement projections are derived, never rewritten by later
   statements; no in-place mutation of shared `Known` flags; nothing
   cached across requests.
7. **Independence** — discharges only `server.drop_privilege`; never
   proves existence, never widens the name-relation domain, never
   clears `server.batch_effects`, entry barriers, or channel taint.
8. **External visibility** — internal evidence keeps
   principal/target/source/request scope; public output states only
   the fixed conditional-audit scope — no account names, grant text,
   role names, or connection details in findings, gaps, errors, or
   diagnostics.

## 3. Adjudication table (implementation contract — not run results)

| case | input | consumable? | result | effect | seam (PLAN) |
| --- | --- | --- | --- | --- | --- |
| A. valid positive | base grant read carries `SUPER` (or proven `ALL PRIVILEGES`) on `*.*`; mode selected; principal+target+context+version bound; statement inside scope | yes | consumable positive witness for the declared conditional audit | `server.drop_privilege` discharged at the shared adjudication | unit + stub lines |
| B. no evidence | no provider, provider lacks the loader, mode not selected, no read issued | no | unproven | gap stays | unit |
| C. principal/context unbound | positive grant exists but the mode is undeclared, the principal is unbound, or the context does not cover the statement | no | unproven | gap stays — even with reader `SUPER` | unit — first-implementation must-verify, not deferred |
| D. applicability mismatch | non-MySQL dialect, version outside the registered domain, grant format outside the parsed domain | no | not applied | gap stays | unit |
| E. no positive witness | read succeeds, no `SUPER`/proven-`ALL` token on `*.*`; or unrecognized representation | no | unproven — **no** `Known=true,false`, **no** denial adjudication | gap stays | unit |
| F. out of ordered scope | earlier authored `SET ROLE`/`GRANT`/`REVOKE` (or the unsupported `set_role`/`grant_role`/`revoke_role`/`grant_proxy` boundary features) ends the range | no | suppressed for that statement | gap stays; raw evidence and earlier bindings unchanged | unit — ordered scan |
| G. read failure | driver/scan/rows.Err/permission/cancellation on the required channel | n/a — error, not evidence | bounded error, `errors.Is/As` reachable | error — never a gap, never a denial, never cached | unit + provider-driver |

Deferred (registered, not implementable branches this slice):
`CURRENT_ROLE()`/`USING` augmentation; proven-denial; offline
`DeploymentEvidence` privilege member; generic account/role/privilege
computation. The `grantsComplete`-style completeness bit is **not**
carried — a base-only read never claims a complete account privilege
picture, and the new DROP consumer has no negative-proof condition.

## 4. Version/product applicability domain (registered)

- **In domain**: MySQL dialect where the resolved version is inside
  the audited surface — MySQL 8.0.x — and the observed `SHOW GRANTS
  FOR CURRENT_USER()` line format `GRANT <privs> ON *.* TO …` holds
  per the official statement reference. `ALL PRIVILEGES` counts as a
  `SUPER` witness only under the registered static-set proof for that
  version (on MySQL 8.0 the documented static global `ALL` list
  includes `SUPER`; this must be registered as a version-checked
  constant, not a substring heuristic).
- **Out of domain**: TiDB (vendor boundary — unchanged), PostgreSQL,
  unresolved version, or a grant-line format not matching the parsed
  domain — all unproven; no inference from approximate syntax.
