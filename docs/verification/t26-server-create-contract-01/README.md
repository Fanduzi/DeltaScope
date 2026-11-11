# T26-SERVER-CREATE-CONTRACT-01

Docs-only contract convergence for the MySQL `CREATE SERVER` slice —
T26 continuation, issue #105, parent #79. No product, test, or manifest
change exists in this directory's commit.

Status: **PROPOSED / Reviewer pending** — a recommended implementation
contract for a future independently-issued execution order.

## Identity

| object | value |
|---|---|
| work order | T26-SERVER-CREATE-CONTRACT-01 |
| baseline | `9e21ace9` (tree `9a6efbd6`, the accepted DROP archive tip) |
| branch | `milestone/mysql-tidb-ddl-completion` |
| accepted predecessor | product `9dad37b8` (tree `eb544128`, R2 PASS) |
| contract doc | [create-server-contract.md](create-server-contract.md) |
| decision record | `docs/decisions/2026-11-12-mysql-create-server-contract.md` |

## Recommended first slice (summary — the contract doc is authoritative)

- **Domain**: MySQL 8.0.x, identifier-form `CREATE SERVER`, authored
  spelling ≤64 runes, wrapper ASCII-fold-equal `mysql`, OPTIONS ≥1
  documented member; duplicate keys / out-of-range PORT stay audited
  but owe `server.create_options`.
- **Stays unaudited**: string-literal names, >64-rune spellings,
  non-`mysql` / literal wrapper spellings (no provable wrapper
  registry), and every non-MySQL dialect.
- **Privilege**: same `SUPER` positive-witness observation already
  approved (single request-level read); CREATE gets its own premise
  projection and its own fact key `server.create_privilege` — never
  borrowed from a satisfied `server.drop_privilege`.
- **Arms**: `ProvenCreate` (conditional `OrderedAbsent` same-identity +
  consumable + clean options → publishes `OrderedPresent`),
  `TerminalRefusal` (provably-occupied slot + plain create),
  `Unproven` (owed facts), `Barrier` (`blocked_batch_effects`). No
  convergent or guarded arm — the grammar has no `IF NOT EXISTS`.
- **Post-state**: new `OrderedPresent` / `OrderedPresentConditional`,
  mutually exclusive with `OrderedAbsent`, produced only by
  `ProvenCreate`, consumed before any provider/cache read, cleared to
  barrier shape by every undischarged unknown effect — including an
  unproven CREATE arm, so a stale `OrderedAbsent` can never survive a
  CREATE whose outcome stayed owed.
- **Honesty invariants carried over**: zero-row catalog = unknown,
  never absent; unproven relation = barrier, never distinct; public
  output fully redacted; no claim the database actually executed or
  refused anything — every conclusion is the audit model's adjudicated
  projection.

## Checks actually run this round

- `git status --short` (tracked): clean before commit.
- `git diff --check`: clean.
- Source-consistency review: every code path cited in the contract was
  read at the baseline tree.
- Official-source consistency: the syntax/semantics table was checked
  against the MySQL 8.0 manual text fetched this round.

All product validation stays **PLAN / UNEXECUTED** — no `go test`,
build, CLI, wrapper, native SQL, Docker, or database operation ran or
is claimed. Historical evidence identities (`9dad37b8`, `bb62dc38`,
`9e21ace9`, NATIVE-01/NATIVE-02 archive references) are preserved
unchanged; nothing here relabels earlier results.
