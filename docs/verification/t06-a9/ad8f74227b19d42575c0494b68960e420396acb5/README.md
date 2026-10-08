# T06-A9-R1 evidence: strict finding identity and EXTRA whitelist

Tested code commit: `ad8f74227b19d42575c0494b68960e420396acb5`
(`test(ddl): tighten T06-A9 finding identity and EXTRA whitelist oracle (#85)`).
This directory is the evidence-only commit on top of it — the evidence
HEAD is not the tested HEAD. **Zero production code changed**; the commit
modifies only `scripts/ddl_golden.py` (validator + A9 oracle query),
`scripts/test_ddl_golden.py` (contract mutations), the A9 EXTRA query
texts inside `testdata/ddl-golden/T06.json`, the A9 ADR, and
`scripts/README.md`.

The parent evidence `docs/verification/t06-a9/b9af1238…/` stays as the
historical record of the original run — its old query text and old SHA
are preserved unmodified, not relabeled as proof of the strict oracle.

## What R1 fixed

1. **Finding identity (R1).** The multi-finding canon used
   `entry.get("statement_kind", "ddl")` on *actual* output, so a recorded
   finding with no `statement_kind` was silently treated as `"ddl"`.
   Expected manifest entries keep their compact shorthand, but actual
   findings are now field-strict: `statement_kind` must exist and equal
   `"ddl"`, `location` must be a real object with integer
   `line`/`column`, `metadata` must be a real map, and a present
   `statement_index` must agree with its enclosing statement (the
   omitempty-absent zero stays legal). Multiset comparison still
   normalizes ordering while preserving multiplicity and the
   message↔`metadata.kind` pairing.
2. **EXTRA oracle (R2).** `t06_a9_extra_normalized` previously deleted
   `default_generated`, `(0)`, and `()` as arbitrary substrings, which
   accepted malformed texts (`DEFAULT_GENERATED()`,
   `on update curr()ent_timestamp`, `DEFAULT_GENERATEDDEFAULT_GENERATED`,
   `()`, `on update CURRENT_TIMESTAMP(0)()`,
   `on update current_default_generatedtimestamp`). The query now folds
   the complete lower-cased EXTRA text through a SQL `CASE`: created
   accepts `''`/`'default_generated'`, updated accepts exactly the six
   frozen zero-precision `on update current_timestamp` spellings, and
   everything else — SQL NULL included — maps to the sentinel
   `<unrecognized-extra>`, which satisfies neither role. No substring
   deletion, token removal, or whitespace merging. The identical `CASE`
   expression also runs as a read-only literal control on every anchor,
   so the shipped query itself proves the legality matrix.

## Contents

- `artifact.json` — new formal `make ddl-golden TASK=T06` artifact at
  ad8f7422: **256/256 cases, 6179 assertions** (+16 vs A9: one
  extra-whitelist literal control per anchored A9 case), cleanup rc=0.
- `cases/` — all 256 per-case records, unchanged denominator and case
  identities; the A9 anchored records now carry the strict `CASE`
  structure queries, the literal control rows (rc 0, frozen concat
  output), and the unchanged raw `COLUMN_DEFAULT`/`EXTRA` observation
  step.
- `policies/`, `T06-manifest.json`, `anchors.json`, `build-identity.txt`
  — generated policies (incl. `t06-a9-audit-columns-isolated`), the
  frozen manifest at ad8f7422, per-anchor identity, and the
  `go version -m` build identity + sha256 (matches artifact;
  `vcs.modified=true` comes from the pre-existing untracked environment
  files in the worktree).
- `r1-controls/` — red/green proof through the **full** `validate_artifact`
  entry point, plus the read-only scalar EXTRA demonstration:
  - `r1-kind-drop-red.log` — *pre-fix* validator: the unmutated real
    artifact validates clean (control), and the artifact with
    `statement_kind` deleted from one finding
    (`T06.cli.t06-a9-mysql-missing-both`) and both findings
    (`T06.cli.t06-a9-tidb-missing-both`) was wrongly reported
    `artifact valid` — the defect.
  - `kind-dropped-cases.json` — the mutated case bodies (stdout
    reserialized from the mutated parsed payload so no parsed/stdout
    drift pre-empts the finding-identity check).
  - `r1-kind-drop-green.log` — *post-fix* validator on the same mutated
    artifact: rc=1 with explicit `statement_kind must be present and
    'ddl'` failures and the multiset mismatch. The same run also lists
    the 16 expected `frozen oracle changed` failures — the old artifact
    still carries the legacy REPLACE query text, which correctly no
    longer matches the tightened contract.
  - `r2-extra-scalar.sql` / `r2-extra-scalar.log` — read-only scalar
    SELECTs executed on the `mysql84` compose fixture (mysql:8.4.10):
    the legacy REPLACE expression and the new `CASE` expression applied
    to the same 17 literals + NULL. The old expression folds every
    malformed string into a passing value; the new expression emits the
    sentinel for all of them. These are constructed literals proving the
    query text — no database catalog produced such EXTRA values, and no
    abnormal table was created.
- `gates/` — argv/stdout/stderr/real-rc for `py_compile`,
  `ddl-golden-validator-test` (385 contract cases: 375 existing + 10 new
  R1/R2 mutations), `validate` on this artifact (binary check enabled),
  `decision-record-gate`, the task-range diff
  (`b9af1238..ad8f7422`), and the formal `ddl-golden` run (observed
  summary tail; full per-case streams live in `cases/`).

## Boundaries

- Denominator frozen at 256; the 228 pre-A9 cases and the A9 inputs,
  policies, product expectations, and role definitions are unchanged —
  only the A9 EXTRA structure queries (and their +1 literal-control row)
  were tightened.
- The validator suite's purpose set is preserved; count went 375 → 385.
- No new business cases, no production code, no new rules or gap types.
- The old A9 artifact under `b9af1238…/` keeps its historical oracle and
  SHA; it is not evidence for the strict query.
