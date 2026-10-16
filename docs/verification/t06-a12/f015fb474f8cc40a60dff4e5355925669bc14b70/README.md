# T06-A12-R1: strict `rule_summary` shape in the artifact validator

Validator code under test: `f015fb474f8cc40a60dff4e5355925669bc14b70`
(`test(ddl): strict rule_summary shape in T06-A12 checks (T06-A12-R1)`).

Product/artifact provenance: the validated golden artifact is the original
360-case run bound to code SHA `1a5dd90de898a59590ef8a19aa068c4b5e8ce7f8`
(`product-artifact-sha.txt`) — reused verbatim, never re-executed. The
binary at its recorded `cli.path` was restored from the preserved original
(sha256 `e298498a9e33eded04a84246f6cac8f42e4563988aa39a57521317df980e95e7`,
matching `cli.sha256` byte-for-byte) together with the recorded policy
files, so validation runs with the binary check enabled against true
hashes. This evidence HEAD is the validator SHA, not the product SHA.

## Root cause

`t06_a12_artifact_failures` read `parsed.get("rule_summary")` and compared
with `==`. Three real gaps, all reproduced on the untouched 360-case
artifact before the fix:

- `T06.cli.t06-a12-mysql-disabled` with `"rule_summary": null` added —
  **wrongly accepted** (`red-pre-fix/disabled-summary-null.rc` rc=0,
  "artifact valid"): a present key with a null value read identically to
  a missing key.
- `T06.cli.t06-a12-mysql-required-false` with `{"loaded":true,
  "applicable":false}` or `{"loaded":1.0,"applicable":0.0}` —
  **wrongly accepted** (rc=0 both): Python `True == 1` and `1.0 == 1`.
- `T06.cli.t06-a12-mysql-time-exempt` (and anchored
  `T06.meta.t06-a12-mysql84-time-exempt`) with `rule_summary` replaced by
  `1`, `true`, `"bad"`, or `[1]` — **AttributeError crash** (rc=1,
  traceback in stderr, `t06_a12_artifact_failures` line 5937): an
  exception escape, never a contract rejection. The fabricated summary
  also escaped the generic `rule_summary_loaded`/`rule_summary_applicable`
  reads for the same reason.

These are proof-contract failures, not product findings — the bad
summaries were constructed evidence; the product never emitted them.

## Fix

`scripts/ddl_golden.py` only (zero product code, zero case/SQL/profile/oracle
changes):

- "Absent" = the key does not exist on the top-level JSON object;
  `null`/`{}`/`[]`/`false` under a present key all reject.
- Enabled roles require `rule_summary` to be a JSON object containing
  both `loaded` and `applicable`, each a strict JSON integer
  (`type(value) is int`) equal to the frozen per-role value
  (1/1 for the six isolated roles, 1/0 for `required-false`).
- The two generic reads in `cli_case_expect_checks` now go through
  `rule_summary_int_count`, which returns `None` for non-dict summaries
  or non-`int` values — malformed evidence reports as a normal failure
  row instead of crashing or coercing.

## Post-fix results (`green-post-fix/`)

- control (unchanged original artifact): rc=0, `[ddl-golden] artifact valid`.
- All eight mutations: rc=1 with `[ddl-golden][FAIL]` contract messages —
  key-absence, object-shape, missing-key, and integer-type rejections, plus
  the guarded generic count rows (`loaded=None`) for non-object summaries.
  No exceptions, no tracebacks.

## Gates at `f015fb47` (`gates/`)

- `make ddl-golden-validator-test` — rc=0, 622 contract cases, 0 failures
  (35 new A12-R1 controls: disabled-key presence forgeries, missing/null/
  non-object summaries, missing counters, bool/float/string/null counts on
  both keys, wrong per-role integer combos, and the same tampering on an
  anchored metadata role — proving the check is not cli-audit-only).
- `python3 scripts/ddl_golden.py validate --artifact <1a5dd90d artifact>` —
  rc=0, `[ddl-golden] artifact valid`, binary check enabled.
- `python3 -m py_compile scripts/ddl_golden.py scripts/test_ddl_golden.py`
  — rc=0.
- `make decision-record-gate` — rc=0.
- `task-range-diff.txt` — `git diff 2a7b2ddf..f015fb47` (the fix commit).

Reused unchanged: the 360-case golden artifact, the four-anchor driver
records, the product binary and generated policies, and all Go/PG/SDK/HTTP
gates — no product or query behavior changed this round.
