# Native probes — T10-R6-tail

No new native probe was run this round.

- Concern (C) is pinned by a **partial injected catalog fixture** in
  `provider_t10r6_test.go` (`PRIMARY`/`primary` folded CLUSTERED rows
  with a NULL sibling, both row orders). No native TiDB NULL-coexistence
  observation is claimed — the fixture models the read shape only.
- Concerns (A) and (B) are ordered-state and rule-layer semantics; their
  catalog premises were already natively anchored by the R6 probe set
  (`../6cec4e6c…/native-probe/r6-attribution-probe.log` on the TiDB
  v8.5.0 anchor): same-sequence candidates, reversed KCU sequences,
  CLUSTERED disambiguation, `pk2`-renamed objects. Those results were
  not rerun.
- Open native boundary carried forward unchanged: MySQL's PK-less
  member-named-`primary` reservation cell remains unprobed.
