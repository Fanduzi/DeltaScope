# Analysis and frozen inputs

`FINAL-FACTS.json` is the lead-checked factual source for the entrypoints.
The original and corrected manifests, seed, authorship receipts and correction
rationale are retained without changing their bytes.

`check_t18_artifact.py` and the two `check_history_artifacts` versions account
for recorded artifacts. `compare_lint.py` and `test_compare_lint.py` compare
lint identities and exercise the comparator. `t18_full_golden_author.py` and
the archived earlier author are the original generator sources. These helpers
refer to their recorded execution paths; archived copies do not rebind them.
`view-lint-before.go.txt` is the original Go source snapshot renamed as text.
