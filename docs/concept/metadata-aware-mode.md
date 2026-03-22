# Metadata-Aware Mode

Metadata-aware mode adds live instance and schema facts to the same audit flow used offline.

## What It Adds

- instance facts such as engine version, default charset, and selected InnoDB settings
- target-table snapshots for existence, compatibility, and lifecycle checks
- schema inference when the target object resolves uniquely

## What It Does Not Do

- It does not replace offline evaluation with a second rules engine.
- It does not guess a schema when object resolution is ambiguous.
- It does not silently hide missing metadata requirements.

## When To Use It

- Pre-migration checks against an existing production-like schema
- Alter-table compatibility checks that need current object state
- Drop/truncate guards that depend on table existence or instance facts
