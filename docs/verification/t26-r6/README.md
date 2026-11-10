# T26-R6 evidence package

Scope: #105 / T26 — R6 work order: unterminated block-comment closure
fix + offline evidence closure. OFFLINE ONLY: no native execution, no
resource ops, no SERVER/SRS provider or lifecycle work.

| file | contents |
|---|---|
| `r6-stage-acceptance.md` | reviewer final ruling (APPROVE — SCOPED OFFLINE PHASE), accepted identity, recorded results, deferred scope |
| `BUILD-IDENTITY.txt` | full build record — CGO, tags, go version, platform, complete command |
| `stage-summary.json` | independent offline stage result — 23/23 cases pass |
| `stage.log` | stage harness run log |
| `scan_record.txt` | per-file marker scan classification — 25 clean, 0 hit, 0 error + reach proof |
| `SHA256SUMS.txt` / `INPUT-HASHES.txt` | archive identity bindings — binary, manifest, policy, harness, per-file |
| `r6_red_recapture.log` | labeled red-state recapture (old compilable combination; not the original run) |
| `r6_green_full.log` | green-state test output on the accepted candidate |
| `r6_cli_pair.log` | CLI unclosed-vs-closed comparison — parser_error path vs drop_server boundary |
| `r6_suite.log` | affected-package + full-repository run log |

The 23 per-case JSON files, `testdata/ddl-golden/T26.json`, and the
stage `policy.yaml` were delivered to the reviewer verbatim; source
copies remain in the independent stage archive (`/tmp/t26stage/r6run`,
ephemeral).

No R3–R5 historical raw record was rewritten by this directory.
