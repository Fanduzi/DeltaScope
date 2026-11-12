# T26-SERVER-ALTER-NATIVE-ORACLE-01 — Phase 0 R1 (native evidence)

Scope: **Phase 0 / native evidence only** — authorized probes on
disposable MySQL 8.0.46 (primary), 8.4.10 (comparison), and 5.7.44
(independent supplement) environments. **No product code modified.**

Baseline: `9cdd3ac5f1fb3777087f7037432ce5a8a78cf866`
(T26-SERVER-ALTER-CONTRACT-01 Phase 0 R1 docs-only delivery,
tree `b0d2e44995bef1cd85a7d33063e064e6aaff0de4`, parent `ff319250`).

## Contents

| path | role |
|---|---|
| `mysql80-native/` | 32 per-case evidence files (J-format), MySQL 8.0.46 |
| `mysql84-native/` | 32 per-case evidence files, MySQL 8.4.10 |
| `mysql57-native/` | 15 per-case evidence files, MySQL 5.7.44 |
| `harness/` | `setup_8x.sql`, `run_case.sh`, `run_matrix_8x.sh` — exact capture machinery |
| `source-facts.md` | evidence → product-path linkage |
| `privilege-visibility.md` | 80-11: write-path vs catalog visibility |
| `privilege-order.md` | 80-7: privilege-before-existence |
| `absent-server.md` | 80-1: absent error + IF EXISTS |
| `merge-poststate.md` | 80-9: merge-vs-replace per-field verdict |
| `options-matrix.md` | 80-2/3/4: dup keys, empty strings, PORT bounds |
| `identity-collation.md` | 80-5/6: collation, case-fold, 64/65-char names |
| `proposed-contract.md` | A1–A12 re-adjudication with NAT evidence |
| `unresolved-items.md` | remaining gaps + stop-condition assessment |
| `manifest-sha256.txt` | four-column digests |

## Capture convention (work order J)

Every case file carries: case_id, executed_at, port, account_identity,
srv_name, literal_sql_target, server_version/config truth, root
pre/post catalog (full option row — sentinel values only),
account-view read probes, verbatim `-v` session transcript (identity +
grants + target + `SHOW WARNINGS` immediately after the target +
`@@warning_count`), a separate single-statement rc-probe for a clean
exit code + error line, stderr, and servers_total.

Each destructive/existence case uses a dedicated `nao_*` server name;
no shared state between cases. Environments are independent docker
containers (`8a826bfa22b1` / `0d0a0f0f2803` / `38251ca4168f`).

## Headline result

**ALTER's write-path existence check uses the true catalog — not the
observer's SELECT-visibility domain.** A SUPER account that cannot
read `mysql.servers` at all (1142 denied) still ALTERs an existing
row, and still receives the true absent-object error (1477) on a
missing row. This is the opposite of the SRS dependency finding
(visibility-scoped enforcement) — the two subsystems use different
rules, exactly why the work order forbade borrowing the SRS result.
