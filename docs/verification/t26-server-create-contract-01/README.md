# T26-SERVER-CREATE-CONTRACT-01

Docs-only contract convergence for the MySQL `CREATE SERVER` slice —
T26 continuation, issue #105, parent #79. No product, test, or manifest
change exists in this directory's commit.

Status: **PROPOSED / Reviewer pending** — R2 revision after CHANGES
REQUIRED on candidate `3ec5f2c3`; a recommended implementation
contract for a future independently-issued execution order.

## Identity

| object | value |
|---|---|
| work order | T26-SERVER-CREATE-CONTRACT-01 (R2 docs-only revision) |
| R1 candidate | `3ec5f2c3` (tree `9b8ba8ec`, parent `9e21ace9`) — preserved |
| this-round baseline | `3ec5f2c3` |
| branch | `milestone/mysql-tidb-ddl-completion` |
| accepted predecessor | product `9dad37b8` (tree `eb544128`, R2 PASS) |
| contract doc | [create-server-contract.md](create-server-contract.md) |
| decision record | `docs/decisions/2026-11-12-mysql-create-server-contract.md` |

## Recommended first slice (summary — the contract doc is authoritative)

- **Activation predicate**: MySQL dialect + resolved version in the
  registered 8.0.x proof domain + identifier-form name +
  `NameSpellingRuneCount <= 64` + identifier-form wrapper
  ASCII-fold-equal `mysql` + whole-form recognizer claim. In-domain ⇒
  audited; mode/provider decide evidence availability only — an
  in-domain create with no mode/provider is audited `unverified`,
  never `unaudited` (a registered public output change).
- **Stays unaudited** (named exits, never parse rejections):
  string-literal names, >64-rune spellings, string-literal or
  non-`mysql` wrappers, version unknown/conflict/out-of-domain,
  non-MySQL dialects. `ALTER SERVER` untouched.
- **Privilege**: same approved `SUPER` observation reused; CREATE has
  its own `ServerCreatePrivilege` projection and owed fact
  `server.create_privilege` — never borrowed from a satisfied
  `server.drop_privilege`. Loader gate widens once to the family
  (registered new call behavior, still one observation per request).
- **Adjudication** (priority-ordered, first match wins): `Barrier`
  `{server.batch_effects}` → `TerminalRefusal` (provably-occupied
  slot; blocker, `refused_present`, owed ∅) → `ProvenCreate`
  (`OrderedAbsent`-same + consumable + clean options; publishes
  `OrderedPresent`, `conditional_create`) → `Unproven` with the exact
  owed-set formula `{existence?, create_privilege?, create_options?}`.
  No convergent/guarded arm — no `IF NOT EXISTS` exists.
- **Post-state**: per-arm unique action — refusal never publishes
  (ordered-sourced leaves `OrderedPresent` standing; catalog-sourced
  records nothing), Unproven/Barrier clear ordered state to the
  barrier shape, so an unknown CREATE invalidates a prior
  `OrderedAbsent` on its provably-same identity and a literal-name
  unaudited create taints the whole channel.
- **Internal facts (proposed)**: per-numeric-option `RangeState`
  (`in-range|out-of-range|unproven`, computed at parse time) and
  `WrapperIdent` (quoting category); "clean options" = documented
  keys + in-range numerics + no duplicates — nothing more claimed.
- **Honesty invariants carried over**: zero-row catalog = unknown,
  never absent; unproven relation = barrier, never distinct; public
  output fully redacted; no claim the database executed or refused
  anything — every conclusion is the audit model's adjudicated
  projection.

## Checks actually run this round

- `git status --short` (tracked): clean at each commit.
- `git diff --check`: executed clean on R1 candidate `3ec5f2c3`;
  re-run clean on the R2 candidate at commit.
- Source-consistency review: every code path cited in the contract was
  read at the baseline tree, including `bindMySQLServer`'s two
  distinct early exits (contamination → bound unvouched; taint/barrier
  → `MetadataUnreadBatchEffects`), which the CREATE mapping reuses
  verbatim.
- Official-source consistency: syntax/semantics table re-checked
  against the MySQL 8.0 manual; USER/HOST/DATABASE registered as an
  audit-model completeness notice, not a claimed native warning.

All product validation stays **PLAN / UNEXECUTED** — no `go test`,
build, CLI, wrapper, native SQL, Docker, or database operation ran or
is claimed. Historical evidence identities (`9dad37b8`, `bb62dc38`,
`9e21ace9`, NATIVE-01/NATIVE-02 archive references) are preserved
unchanged; nothing here relabels earlier results.
