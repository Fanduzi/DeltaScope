# T06-A14-BASELINE evidence index

Evidence HEAD: `a2ea0b6c89d95093c8acac151d4f5df34ccfa5e3` (= origin/workfree at collection)
Binary: `deltascope` sha256 `37a2451cf8d4e4618184dfdb1733d2fc20fd332e86c771173bfa8a1419638cb7` (build-info.txt)

## Contents
- `build-info.txt` — source/build identity, worktree state
- `catalog-defaults.txt` — product defaults for the three rules (+ collation)
- `rules-list.json` — real catalog (381 rules)
- `policies/` — 7 temporary policies (strict/optional per family + all-off)
- `sql/` — 7 matrix inputs + `probe-column-charset.sql` parser control
- `runs/<dialect>-<family>-<role>.{argv,stdout,stderr,rc}` — 36 runs × 4 files
- `probe/` — real parser/extractor probe (go.mod/go.sum/main.go/probe-output.txt; binary rebuilt locally, not committed)

## Run matrix (all real, rc/verdict verified)
| role | mysql | tidb |
|---|---|---|
| A allowed-strict | pass rc=0 | pass rc=0 |
| B denied-strict | reject rc=1, 1 blocker | same |
| C omitted-strict | reject rc=1, 1 blocker | same |
| D omitted-optional | pass rc=0 (loaded=1, applicable=1) | same |
| E denied-optional | reject rc=1, 1 blocker | same |
| F denied-off | pass rc=0, rule_summary absent | same |
