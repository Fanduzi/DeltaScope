# DeltaScope v0.36.1 Release Notes

## Summary

DeltaScope v0.36.1 is a SQL corpus coverage and release-confidence patch. It does not add new audit rules or widen parser support. Instead, it makes the existing supported-rule corpus contract explicit, visible, and enforced in release validation.

## What Changed

### SQL Corpus Coverage Gate

`make sql-corpus-gates` now verifies that every currently supported `rule_id × dialect` surface has at least one SQL corpus case.

The contract is intentionally narrower than “every policy key on every dialect”. It tracks the current stable extractor/rule support surface and avoids over-claiming PostgreSQL coverage where extractor facts are not yet available.

### SQL Corpus Inventory Report

`make sql-corpus-report` prints the current supported-rule coverage inventory, including:

- shipped policy rule count
- supported `rule_id × dialect` target count
- covered target count
- coverage percentage
- corpus fixture counts by dialect
- deferred rule surfaces

Current inventory at release time:

| Metric | Value |
|--------|-------|
| Policy rule IDs | 156 |
| Supported rule/dialect targets | 340 |
| Covered rule/dialect targets | 340 |
| Coverage | 100.0% |
| Corpus expected YAML files | 49 |
| MySQL fixtures | 13 |
| TiDB fixtures | 11 |
| PostgreSQL fixtures | 25 |
| Deferred rule IDs | 2 |

### Release Gate Integration

`release-test-gates` now runs the SQL corpus coverage gate. The release-smoke workflow also includes an explicit supported-rule corpus coverage step.

## What Did Not Change

- No new audit rule IDs.
- No parser support widening.
- No spec contract widening.
- No MySQL, TiDB, or PostgreSQL behavior changes.
- No new CLI flags.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/Fanduzi/DeltaScope/v0.36.1/install.sh | \
  DELTASCOPE_VERSION=v0.36.1 sh
```

## Next Follow-up

The next product-capability work should focus on real PostgreSQL support-surface gaps rather than more corpus plumbing, especially PostgreSQL primary-key facts and PostgreSQL `ALTER TABLE ... ADD CONSTRAINT ... UNIQUE` index-like facts.
