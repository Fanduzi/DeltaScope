# T06-A11-R1: bound the raw COLUMN_TYPE HEX field shape

Validator under test: `30baf74da346ee6ba1efbcbc66d5b68d4bcafda7`
(`test(ddl): bound T06-A11 COLUMN_TYPE hex field shape (T06-A11-R1)`)

Evidence-only closeout on top of the A11 proof (`4dc51cbd`, evidence
`451f54ca`). The artifact re-validated is the **original** 324-case run
whose `head_sha` is `4dc51cbd` — unchanged and not relabeled to the R1
validator SHA.

## Root cause

`t06_a11_artifact_failures` decoded the observed `HEX(COLUMN_TYPE)` field
with `bytes.fromhex(...)`, which silently skips ASCII whitespace. A
corrupted field containing literal spaces or tabs still decoded into the
role whitelist — the record shape contract "complete, consecutive
two-digit hex pairs" was not actually enforced.

## Fix (proof code only, no product change)

`re.fullmatch(r"(?:[0-9A-Fa-f]{2})+", column_type_hex)` is required
before decoding. Non-empty, any letter case, no stripping, no
re-encoding; the byte-empty `EXTRA` field is unaffected (presence
mandatory, emptiness legal). The whitespace inside a legal type like
`bigint(20) unsigned` arrives as encoded byte `20`, not literal space —
rejected whitespace means whitespace *in the hex field*, never the
encoded `20` byte.

## Residuals reproduced and fixed (full `validate` entry, binary check on)

Mutation target: `T06.meta.t06-a11-mysql57-unsigned-pass`, step
`column type raw observation`, only the COLUMN_TYPE hex field replaced in
a verbatim copy of the original artifact — header, nine fields, trailing
empty EXTRA, SQL, step names, rc, policy, and all other records
untouched.

| Variant (H = `626967696E742832302920756E7369676E6564`) | Pre-fix | Post-fix |
| --- | --- | --- |
| control (H verbatim) | `artifact valid` rc=0 | `artifact valid` rc=0 |
| pairs joined by space | **wrongly `artifact valid`** | rc=1 contract rejection |
| leading space | **wrongly `artifact valid`** | rc=1 contract rejection |
| trailing space | **wrongly `artifact valid`** | rc=1 contract rejection |
| tab between first pairs | **wrongly `artifact valid`** | rc=1 contract rejection |
| leading tab | **wrongly `artifact valid`** | rc=1 contract rejection |
| trailing tab | **wrongly `artifact valid`** | rc=1 contract rejection |

Pre-fix runs used the `ddl_golden.py` extracted verbatim from
`451f54ca` (pre-fix HEAD) under `/tmp/ds-t06-a11-r1/oldval/`; post-fix
runs used the committed R1 validator. Both directions ran the complete
`validate` command against the real artifact copy — no helper-only test.

## Directory layout

| Path | Contents |
| --- | --- |
| `red-pre-fix/` | argv/stdout/stderr/rc for control + six mutations under the pre-fix validator — control valid, all six wrongly `artifact valid`. |
| `green-post-fix/` | Same seven files under the R1 validator: control valid; all six return clean `[ddl-golden][FAIL]` contract failures naming the corrupted hex. |
| `gates/` | argv/stdout/stderr/rc for the required gates at the R1 SHA. |
| `identity/` | `hashes.txt` (blob ids of changed files, original artifact, original binary). |
| `task-range-diff.txt` | `5330912f..HEAD` and `451f54ca..HEAD` file lists. |

## Gate summary (at `30baf74d`)

| Gate | Result |
| --- | --- |
| `make ddl-golden-validator-test` | rc=0 — **554 contract cases, 0 failures** (507 preserved + 47 new R1 controls: 32 whitelist-form sweeps, upper/mixed-case hex, 6 whitespace corruptions, empty/odd/non-hex/0x-prefix/non-ASCII shapes, encoded trailing space, wrong signedness) |
| `ddl_golden.py validate --artifact <original>` | rc=0 — `artifact valid`, binary check enabled on the original binary (`sha256 3b3986fe…`) |
| `python3 -m py_compile` both scripts | rc=0 |
| `make decision-record-gate` | rc=0 (see note below) |

## Honest-labeling notes

- Pre-fix runs are recorded as **wrongly accepted**, not paraphrased as
  pass — they were genuine validator gaps, not product defects.
- Mutated artifacts are constructed test inputs, never real database
  output; the database was not touched this round (explicitly reused:
  the 324-case run, product binary, and four-anchor originals from
  `4dc51cbd`).
- The decision-record gate again reported the SIGPIPE-shortcut branch
  (`trigger paths hit but no trigger keywords`) on the large milestone
  diff — a pre-existing script quirk; the R1 ADR section is appended to
  `docs/decisions/2026-10-14-ddl-primary-key-bigint-unsigned-proof.md`.
- No Golden rerun, no Docker, no CLI rebuild, no manifest/profile/case
  changes, no `T06.json` edit, no Go changes this round.
