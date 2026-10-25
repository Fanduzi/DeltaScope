# T10-R6 native probe archive

Minimal supplementary TiDB v8.5.0 probes behind the R6 attribution
ordering change — the R5 archive's pk2 rename / visibility / freed-name
cells are reused verbatim and not re-run here.

| File | Description |
|------|-------------|
| r6-attribution-probe.sh | Probe driver: three catalog shapes against `pingcap/tidb:v8.5.0` (compose anchor `deltascope-ddl-golden-tidb85`, host 127.0.0.1:24000, explicit empty password) — a1 same-sequence candidates `aa(id)` + renamed PK `pk2(id)`, a2 reversed-order member `aa(b,a)` vs PK `pk2(a,b)`, a3 single clustered PK object |
| r6-attribution-probe.log | Verbatim capture: a1 shows both folds NON_UNIQUE=0 with `CLUSTERED=NO` and ordered KCU `[id]` (attribution must withhold); a2 shows ordered sequences `[a,b]`/`[b,a]` (reversed never binds); a3 shows the lone PK object row `CLUSTERED=YES` beside member `NO` rows |
