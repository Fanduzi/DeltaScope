# Config Reference

DeltaScope uses a YAML policy file to tune rule enablement and thresholds without changing the binary.

## Operator Commands

```bash
deltascope config init > deltascope.yaml
deltascope config lint --file ./deltascope.yaml
deltascope config show-default
```

## File Sources

- generated from `deltascope config init`
- checked-in example: `configs/deltascope.example.yaml`
- loaded at runtime with `--config`

## Guidance

- generate once, then commit the policy file used by CI
- lint config changes before rollout
- treat the checked-in example as documentation, not the only supported config source
