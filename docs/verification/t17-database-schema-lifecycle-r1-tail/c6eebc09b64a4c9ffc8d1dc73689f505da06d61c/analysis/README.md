# Evidence checks

These lead-authored scripts check frozen delivery artifacts, not runtime
application behavior or an AI evaluation pipeline.

- `t17r1-tail-verify.py`: accepts artifact path, product SHA, and bundle
  directory; verifies the unchanged manifest, real CLI artifact and binary,
  red/green internal projections, and frozen native correspondence.
- `t17r1-tail-compare-lint.py`: accepts a gate-receipt directory; compares
  diagnostic identities with committed R1 baseline `9da9f8e0`.
- `final.*`: actual command/stdout/stderr/rc from the independent lead
  artifact/projection check.

Run from the repository root. The verifier's binary hash check needs the
runner's retained binary at the path recorded in the artifact; rebuilding
or rerunning Golden is a separate operation, not presentation of this
frozen evidence.
