# T06-A13-BASELINE evidence index (out-of-repo, uncommitted)

Built from evidence HEAD 84e37e32 (v0.511.2-0.20261016161622-84e37e32d694+dirty).
+dirty attributed to external .mcp.json edit + untracked tool dirs/restored run dir; no snapshot taken at build time.

- build-info.txt — build cmd, go version -m module line, binary sha256
- rules-list.json — real `deltascope rules list --format json` catalog (381 rules)
- catalog-defaults.txt — the five targets' real defaults (enabled=true, level=warning; forbid param defaults true per boolParam fallback)
- policies/ — 11 generated isolation policies (all-other-rules-disabled + per-rule ON/FALSE + P_OFF)
- M.sql / S.sql — the two fixed single-line inputs
- runs/*.argv|stdout|stderr|rc — 32 valid offline CLI runs (16/dialect)
- probe/ — sibling module github.com/Fanduzi/DeltaScope/probe-t06a13 + replace → checkout; probe-output.txt captured AST+spec facts
