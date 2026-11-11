# Build & invocation record — T26-SERVER-DROP-01 R1/R2 verification

Retained records only; items not retained are marked explicitly. The
full heredoc wrapper was executed via `python3 - <<'EOF'` and was
**not** saved to disk — the fragment below is a faithful reconstruction
of the actual function calls and argv, not an original script.

## Build

Executed on the working tree immediately before commit `5de2da5e`,
while that tree contained all 7 R1 file modifications; those files were
then committed unchanged, so the binary's source content corresponds to
the `5de2da5e` tree. The binary was **not** rebuilt on a clean
`5de2da5e` checkout — this record registers the build as it happened.

```
mkdir -p /tmp/t26-r1 && CGO_ENABLED=0 go build -o /tmp/t26-r1/deltascope ./cmd/deltascope
```

Binary path at run time: `/tmp/t26-r1/deltascope` (37,051,858 bytes).

## Manifest/profile consumption (runner's own functions)

The full `ddl_golden.py run` path cannot select individual units and
requires Docker for `db_*` cases; only the three named `cli_cases` were
executed, through the runner's own `make_policies` +
`execute_cli_case` functions — the same manifest/policy-consumption
path the full runner uses for CLI cases. Reconstructed invocation
fragment (actual calls and arguments as executed):

```python
import json, sys, importlib.util
from pathlib import Path
sys.path.insert(0, "scripts")
spec = importlib.util.spec_from_file_location("dg", "scripts/ddl_golden.py")
dg = importlib.util.module_from_spec(spec); spec.loader.exec_module(dg)

manifest = json.loads(Path("testdata/ddl-golden/T26.json").read_text())
work = Path("/tmp/t26-r1")
policies = dg.make_policies("/tmp/t26-r1/deltascope", work, manifest)
# -> golden-policy-server-drop.yaml (ddl.mysql.server.drop.notice enabled)
# -> golden-policy-all-off.yaml

wanted = ["mysql-drop-server", "mysql-drop-server-if-exists", "mysql-server-mixed-batch"]
for spec_case in manifest["cli_cases"]:
    if spec_case["id"] in wanted:
        case = dg.execute_cli_case(manifest, "/tmp/t26-r1/deltascope", policies, spec_case)
```

Each case's real argv is recorded verbatim in
`manifest-cli-results.json` under `command` (e.g.
`/tmp/t26-r1/deltascope audit --dialect mysql --sql "DROP SERVER golden_srv" --config /tmp/t26-r1/golden-policy-server-drop.yaml --format json`).

## Targeted verdict run (bound to `2ee906d5`)

```
go test -count=1 -run '^TestT26DropServerBarrierPublicShape$' -v ./internal/application/audit
```

## Not retained (registered honestly)

- The heredoc wrapper script text itself (invoked via stdin, not saved).
- The browser delivery script used to post receipts (generated per run).
- No `5de2da5e`-clean-checkout rebuild exists; no re-run of the three
  packages or three CLI units on `2ee906d5` (reviewer-authorized: prior
  records stay bound to `5de2da5e`).
