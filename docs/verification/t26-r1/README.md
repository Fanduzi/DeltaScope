# T26-R1 evidence package

Scope: #105 / T26 — MySQL SERVER lifecycle, spatial reference systems,
low-frequency DDL (ALTER INSTANCE). Work order T26-R1 = offline
evidence + baseline + acceptance design only. **No native execution
authorized; none run.**

| file | contents |
|---|---|
| `r1-scope-and-sources.md` | official per-form inventory, MySQL 5.7/8.0/8.4 + TiDB 8.5 matrix, inventory ownership, doc sources, boundary facts, out-of-scope |
| `r1-baseline.md` | current source chain, verbatim offline probe matrix, leak-surface analysis, 153-probe corpus mapping, metadata baseline, no-outbound static evidence |
| `r1-decision-table.md` | proposed (unapproved) truth tables for SERVER/SRS/ALTER INSTANCE, ordered-state model, metadata facts, open rulings |
| `r1-native-plan.md` | draft case matrix, anchors, isolation, teardown — design only |
| `probes/inputs.txt` | 13 offline probe inputs |
| `probes/outputs.txt` | verbatim JSON+stderr for both dialects |

Binary used for probes: sha256
`b68c4dbb23e0452c269f0ecd6698a1d0f36578cbfb59b4f64f25eeb061ef2f11`
(go1.26.1, CGO_ENABLED=0, built on HEAD `4896137f`).

Registered open rulings for the reviewer: ALTER INSTANCE surplus
subactions (inventory lists 2 of ~5 official forms); DEFINITION-text vs
length-only retention; redaction contract for credential-carrying SQL
(`unsupported[].sql`/`raw_sql` echo); `IF NOT EXISTS` validation ordering
(待证); zero-row absence promotion policy; egress-probe authorization.
