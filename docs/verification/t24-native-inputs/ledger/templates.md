# T24 ledger templates (R11 frozen)

`receipts.json` and its generation-append format are unchanged; these
indexes sit beside it — recomputable read-only views, never a second
ownership system. One attempt per directory; failed inputs, expected,
actual, and raw state are never overwritten; SHA256SUMS + index are
generated after each seal. Restricted originals dir `0700`, secrets
`0600`; committed artifacts carry only the needed sanitized evidence —
no `/proc/*/environ`, no keychain, no full user Docker config.

## RUN_PLAN.json

```json
{
  "task": "T24", "run_id": "<UTC-stamp>", "attempt_id": "attempt-1",
  "fingerprints": {
    "product_head_sha": "<full 40-hex>", "runner_sha256": "<sha>",
    "cli_binary_sha256": "<sha>", "manifest_sha256": "<sha>",
    "policy_sha256": "<sha>", "config_sha256": "<sha>"
  },
  "anchors": {"<anchor>": {"image": "…", "platform": "linux/amd64",
    "digest": "sha256:…", "publish": "127.0.0.1:PORT:3306"}},
  "approved_actions": ["P","I","C","O","G","X","T","R","S","R","V","L"],
  "resource_budget": {"cpu_total": "<=4", "ram_total": "<=12GiB",
    "containers_approx": 5, "probe_concurrency_cap": 6,
    "generations": 1, "data": "new volumes only, exact names+mounts"},
  "required_set": "24 (10 cli + 6 meta + 8 db)",
  "selected_set": ["<case ids executed>"],
  "input_generation": {"collector": "scripts/t24_collect_evidence.py",
    "oracle_source": "frozen contract — never reverse-fit from actual"}
}
```

## evidence-index.jsonl (one row per evidence package)

```json
{"evidence_id":"…","anchor":"mysql80","server_uuid":"…",
 "container_id":"…","process_start":"…","observed_at":"…Z",
 "source_method":"proc-status+sched_getaffinity","source_ref":"raw/…",
 "raw_artifact_sha256":"…","facts":[{"name":"host_vcpus","value":4,
 "knownness":"proven","scope":"this-instance"},
 {"name":"priority_capable","value":true,"knownness":"proven",
 "scope":"this-instance"}],
 "invalidate_on":["restart","rebuild","target-change","cpuset-change"]}
```

## sql-actions.jsonl (one row per executed statement)

```json
{"action_id":"…","case_id":"T24.db.mysql80.ddl","phase":"setup|observe|
execute|verify|restore","target_uuid":"…","principal_alias":"t24_admin",
 "session_id":"…","sql_sha256":"…","sql_text_redacted":"…",
 "rc":0,"sqlstate":"00000","errno":0,"warnings":[],
 "result_artifact":"cases/….json","pre_state_ref":"…","post_state_ref":"…",
 "restore_action_ref":null}
```

Errors are never overwritten by later success — each attempt gets its
own row.

## restore.jsonl (one row per compensated object)

```json
{"object":"GLOBAL super_read_only","original_value":"0",
 "original_evidence_ref":"…","compensation_cmd_ref":"sql-actions#…",
 "compensation_rc":0,"readback":"0",
 "status":"restored|failed|not-run"}
```

Resource ownership comes only from receipts: full container id, network
id, volume name/driver/create info, Mounts + mounted_by, generation,
capture state, per-object S/R/V outcomes. Counts are unique-identity
unions — reobserved objects never masquerade as new; anonymous vs named
volumes listed separately.

## Stop conditions (any failure → keep attempt + raw inputs, stop widening writes, run still-authorized safe restores)

- uuid/version/identity query error mid-run → propagate, no fallback
- unregistered object/thread inside a run group → stop FORCE ops
- restore failure vs container destruction → two separate facts; a
  destroyed instance is never recorded as "variable restored"
- tool-refused deletion → keep it, never manual cleanup outside the
  authorized list
- never rerun-to-green on leftover budget
