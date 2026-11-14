# DeltaScope T11/#90 — accepted evidence (R4)

- **T11 ACCEPTED** — unified PK/UNIQUE/FK CREATE+ALTER checks; Reviewer final technical verdict at frozen HEAD.
- Frozen identity: HEAD `9d2c439c46f6faed434e91c8b54ebc82abbf0787` / tree `bcec2a29997276ca8d3a173da15ce7cb4c66ab4c` / parent `d6af124165ff05ba1ca18948a7aa20c8f03df58f` (local milestone branch `milestone/mysql-tidb-ddl-completion`).
- R4 archive: `t11-unified-constraints-evidence-r4.tar.gz` — SHA-256 `724a43dcf898331f06dddf166d7431b6a16c50d166dfcd409cdfe73c66cee604`, 373,003 bytes, manifest 244 rows.
- Evidence chain: R1 baseline audit → R2 transport parity + test reconciliation → R3 per-version standards corrections + frozen-source bindings → R4 SET DEFAULT final verdict + matrix errata + complete source excerpt + HTTP binary re-binding.
- Golden: 40/40 cases, 518 assertions, all anchors.
- Four-surface parity: CLI/SDK/HTTP/MCP identical under same SQL + isolated policy.
- SET DEFAULT boundary: declaration landed on MySQL 8.0.46 / 8.4.10 / TiDB v8.5.0; rejected on 5.7.44 (ERROR 1215); official-doc divergence recorded per version — T11 audits declared actions only, not runtime DML behavior.
- Proof boundary: R4 TiDB probe recorded its image digest at probe time; the three new MySQL probes carry version+native results (digest binding post-hoc only — see `native/r4-mysql-image-digests-posthoc.txt` inside the archive).
- The local milestone branch leads the remote by 317 commits; this evidence branch is NOT a product release — no product code is published here.
