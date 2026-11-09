# T24 fixture SQL / sessions / accounts (R11 frozen templates)

Names derive from `RUN_ID` (must satisfy MySQL's 64-byte group-name
limit); every object is individually registered — never prefix-cleaned.
Every new object is observed first; a same-name non-run object stops the
run — never DROP it to "clean up".

## Accounts / roles (≤4 accounts, ≤2 roles, created only after authorization)

| principal | grants | purpose |
|---|---|---|
| `t24_admin_<run>` | `RESOURCE_GROUP_ADMIN` direct | audit+execute subject (positive privilege arm) |
| `t24_role_user_<run>` + role `t24_rg_role_<run>` | role carries `RESOURCE_GROUP_ADMIN`; account granted role | activated-role arm; separate "role not activated" input |
| `t24_nopriv_<run>` | none beyond USAGE | negative control — no RESOURCE_GROUP_ADMIN |
| `t24_actor_<run>` | none beyond USAGE | occupancy-experiment session owner only |

- Privileges listed verbatim from the provider's actual queries; no
  unrelated SUPER/PROCESS/CONNECTION_ADMIN/ALL unless a frozen negative
  control needs it as its own dimension — then it is a separate line on
  the authorization card.
- The bootstrap account only creates/grants/restores this run's
  principals; never a real corporate account or previously exposed
  credential. Short-lived fixture credentials are generated at native
  time, never stored.
- Product metadata connection and native execution connection use the
  SAME expected principal/target/activated-role state per arm — a
  `SET ROLE` on an external connection does not mean the product's new
  connection activated the role.

## Groups (server-scoped — no schema isolation)

| group | config | use |
|---|---|---|
| `t24fx_rg` | TYPE USER, real allowed VCPU id(s), priority 0 | first-path lifecycle |
| `t24fx_def` | TYPE USER, VCPU `0,2` (or real ids), priority 5, DISABLE | catalog-definition readback on mysql84 |
| `t24fx_occ` | TYPE USER, priority 0, scheduling-neutral | occupancy/DISABLE/DROP FORCE arm |
| `t24fx_ghost` | absent | ALTER-on-absent negative control |
| negative cells | illegal declarations, reserved `usr_default`/`sys_default` | read-only negative controls — never ALTER builtins |
| `TYPE SYSTEM` control | definition only | never assign system threads to it |

## Occupancy experiment (≤2 run-owned short-lived client sessions)

- Actor runs `SET RESOURCE GROUP t24fx_occ` on its own connection —
  never `SET RESOURCE GROUP ... FOR <thread_id>` on other threads; the
  statement is fixture setup only, not a product-supported semantic.
- Record `CONNECTION_ID()` and the corresponding
  `performance_schema.threads.THREAD_ID` — the two ids are distinct;
  never conflate them.
- Actor stays connected until the target assertions finish — a
  disconnected actor cannot serve as busy-group evidence.
- Independent control connection + the formal provider observe:
  occupied DROP w/o FORCE refused + definition retained; DISABLE w/o
  FORCE leaves actor in-group; DISABLE FORCE returns actor to default;
  DROP FORCE removes definition + thread reassignment.
- Only registered run groups/actors; an unregistered thread inside a
  run group stops FORCE operations and is reported.
- Sessions carry a hard deadline + controlled close path; no
  `SLEEP()`-as-infinite-occupancy. `KILL` only on an identity-verified
  run session, only if listed on the card; self-exit is preferred.

## Read-only experiment (this run's fresh 8.0/8.4 instances only, one at a time)

- Scope: `GLOBAL read_only`, `GLOBAL super_read_only`, plus SESSION role
  state where a privilege arm explicitly needs it.
- Read both originals first; keep the restore plan and a rescue
  connection. Verify the four frozen arms: writable / plain read-only
  without exemption / exempt / super_read_only ordinary-client refuse.
- Restore by actual original values in dependency order (not a blanket
  OFF), then readback-confirm both variables. `SET rc=0` without
  readback records only rc-level restore evidence; a rescue connection
  alone never proves restore succeeded.
- No `SET PERSIST`, no startup-config persistence, no thread-pool /
  PFS-capacity / unrelated variables.
