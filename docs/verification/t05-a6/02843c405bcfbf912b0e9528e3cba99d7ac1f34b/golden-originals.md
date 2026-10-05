# T05-A6 golden originals — same run as 02843c40

This file only indexes originals from the successful golden run. It does not change that run's `head_sha`, expectations, or status. The tested code stays `02843c405bcfbf912b0e9528e3cba99d7ac1f34b`. The commit that adds this directory is a later evidence commit.

## Source

The runner recorded:

```text
make ddl-golden TASK=T05 ARTIFACT_DIR=/tmp/ds-t05-a6-golden-02843c405bcfbf912b0e9528e3cba99d7ac1f34b-r2
```

On this machine `/tmp` is `/private/tmp`, so the files were read from:

```text
/private/tmp/ds-t05-a6-golden-02843c405bcfbf912b0e9528e3cba99d7ac1f34b-r2/T05/
```

`gates/golden.argv`, `gates/golden.stdout`, and `gates/golden.rc` (0) are the records already published for that command. They were not copied again.

## What was copied

256 files were copied byte for byte. Each copy's SHA-256 matched its source. `golden-originals.sha256` lists those 256 hashes. The count is files only; `policies/` is the one new directory. `cases/` already existed.

| Archive path | Source path under the run's `T05/` |
|---|---|
| `artifact.json` | `artifact.json` |
| `cases/<case_id>.json` (247 files) | `cases/<case_id>.json` |
| `policies/<filename>.yaml` (8 files) | `<filename>.yaml` |

`artifact.json` SHA-256 is `1856e63ded84fabbbde2471d0ed55c921b54cde99b512753d327c819aafde791`. That is the same value already stored in `identity/artifact.sha256`. `head_sha` inside the artifact is `02843c405bcfbf912b0e9528e3cba99d7ac1f34b`. Local absolute paths in the JSON were left as the run wrote them.

The artifact contains 247 cases, and the 247 case files use those same `case_id` values. Sixteen of the ids contain `t05-a6`. The earlier `cases/a6-digest.txt` is unchanged and is not one of the 247 raw records.

`artifact.json` already holds `container_health` for the four anchors and the TiDB client, plus `cleanup` (`compose_down_rc` 0, empty `residual_containers`). Those objects were not split into extra files.

## Policies

The artifact names these eight files. Their bytes match the `sha256` fields in `artifact.json`.

| Profile | File | SHA-256 |
|---|---|---|
| `all-rules-disabled` | `policies/golden-policy-all-off.yaml` | `57e926645162cf32e47f91c9409b48374e85f8c571f801dc56728cb61859b250` |
| `t04-modify-column-compat-isolated` | `policies/golden-policy.yaml` | `6874b37c7315969d65aa5e6c8cb94b293a454487b4ca3828f4a2d7d409715794` |
| `t04b-key-length-isolated` | `policies/golden-policy-t04b-key-length-isolated.yaml` | `533ce28aa73e9b6bc4822f62888f97c55d6751b3e4cb7b7c3dda3ced68fc14a6` |
| `t05-a4-modify-isolated` | `policies/golden-policy-t05-a4-modify-isolated.yaml` | `c0879f071a11a2dee53cfee32d08a715b3bb3bd1bd15e098b5a1b98338097cf9` |
| `t05-a5-column-identity-isolated` | `policies/golden-policy-t05-a5-column-identity-isolated.yaml` | `b1111973f9d1299ff19ca37d30a115e850bfd90d97b92d65378da504fa6f1374` |
| `t05-a6-drop-column-isolated` | `policies/golden-policy-t05-a6-drop-column-isolated.yaml` | `4c64a312dac2b31b2d412cdf10c2140983d8835027e30e46be7e6d5adaff268b` |
| `t05-drop-recreate-isolated` | `policies/golden-policy-t05-drop-recreate-isolated.yaml` | `ebba386df8c4d306a5368e695e5ac9fecb3f5658b35f52822738ed7ee6e1f2c8` |
| `t05-first-path-isolated` | `policies/golden-policy-t05-first-path-isolated.yaml` | `276d3462a09095525a6fb52411da409f1d1fbd3f3c1101215c8fd4896f6bdf6a` |

The manifest is already at `testdata/ddl-golden/T05.json` in the code commit. Its blob is `e87e040840bb735795b14f615003552e97df48d9`. Its content SHA-256 is `09edfeea6c9a0eb4d056f24c522cfc7bd9db2612171872afc23d2490dd101566`. It was not copied into this directory.

## Left out

`T05/bin/deltascope` was not copied. Its SHA-256 is already in `identity/golden-binary.sha256` (`2c4cd1901a923b0151d7d593725c7b285716f0d4809f5ce4fd89983e36b46956`). That hash was not recomputed for this commit.

The case records name `--password-env DS_T05_GOLDEN_PW` and MySQL's command-line password warning. They do not contain a password value or a DSN. Those bytes were not redacted.

No other file from that run directory was missing.
