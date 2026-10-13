# T06-A10-R2: observation typing and SHOW CREATE head boundary

Code under test: `42b16022efe2d71e99a76965846f338ef141462b`
(`test(ddl): bound T06-A10 observation typing and SHOW CREATE head (#85)`)

Evidence-only closeout on top of the R1 record-set fix (`0693a3df`,
evidence `1fd97e9d`). The artifact re-validated is the **original**
292-case run at `bffcc261`; its `head_sha` is unchanged and not
relabeled to the R2 validator SHA.

## Residuals reproduced and fixed (full `validate_artifact` entry)

**Residual 1 — non-string stdout crashed the validator.** The R1 helper
called `(stdout or "").splitlines()`: a truthy non-string value (`1`,
`true`, `["bad"]`, `{"bad":"value"}`) raised `AttributeError`, exiting
the whole validate run — an exception, not a contract rejection.
`_t06_a10_obs_record` now checks `isinstance(stdout, str)` first and
returns a failure string naming the received type. No `str()` coercion,
no default fill, no broad exception swallow. `None`/missing/empty stay
invalid observations (`None` → "not a string", `""` → "produced no
output"); real string records are untouched.

**Residual 2 — SHOW CREATE head only checked a prefix.** `ddl.startswith
("CREATE TABLE `t`")` accepted `CREATE TABLE `t`.`u` (...)`,
`CREATE TABLE `t``shadow` (...)`, and bare `CREATE TABLE `t`` — each
returned `artifact valid` pre-fix. The record must now match
`CREATE TABLE `t`[ \t]+\(`: the complete `t` token continuing straight
into the column-definition paren. All 20 real anchor records satisfy the
fixed shape; the body stays opaque (TiDB comments and options unparsed).

## Directory layout

| Path | Contents |
| --- | --- |
| `red-pre-fix/` | argv/stdout/stderr/rc for the same mutated artifacts validated by `ddl_golden.py` extracted verbatim from parent `1fd97e9d` (`git show`): `control` green; `r1-alloc-int`, `r1-raw-bool`, `r1-show-dict` exit 1 with AttributeError tracebacks (crash, not rejection); `r2-qualified`, `r2-shadow`, `r2-truncated` wrongly accepted as `artifact valid`. Mutated artifacts differ from the original only in one step's `stdout`. |
| `green-post-fix/` | Same seven mutations under the R2 validator: control stays valid; all six corruptions return clean `[ddl-golden][FAIL]` contract failures. |
| `gates/` | argv/stdout/stderr/rc for the four required gates at the committed SHA. |
| `identity/` | `source.txt` (validator/product/artifact triple attribution, pre-fix extractor note, worktree state); `hashes.txt` (sha256 of changed scripts, README, ADR, the pre-fix validator copy, original artifact, original binary — referenced by hash, not copied). |

## Gate summary

| Gate | Result |
| --- | --- |
| `python3 -m py_compile` (both scripts) | rc=0 |
| `validate --artifact /tmp/ds-t06-a10-proof/T06/artifact.json` | rc=0, `artifact valid` — original `bffcc261` artifact, binary check enabled on the original binary |
| `make ddl-golden-validator-test` | rc=0, 466 contract cases, 0 failures (445 + 21 R2) |
| `make decision-record-gate` | rc=0 |

No Docker started, no Golden rerun, no product rebuild, no `T06.json`/
profile/case changes — the pre-fix and post-fix runs share the same
original artifact and differ only in the validator source.

## Honest-labeling notes

- Red runs are crashes (`AttributeError`) or wrongly-accepted forgeries
  under the parent validator — recorded as such, not paraphrased as
  contract rejections.
- The mutated artifacts' forged stdout values are constructed test
  inputs, never presented as real database output.
- 21 new controls: five non-string values (`int`, `bool`, `list`,
  `dict`, `None`) plus empty stdout on each of the three observation
  steps = 18, plus three forged SHOW CREATE heads. Prior 445 controls
  unchanged.
